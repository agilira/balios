package balios

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	goerrors "github.com/agilira/go-errors"
)

// TestGetOrLoad_CacheHit verifies that GetOrLoad returns cached value without calling loader
func TestGetOrLoad_CacheHit(t *testing.T) {
	cache := NewCache(Config{MaxSize: 100})

	// Pre-populate cache
	cache.Set("key1", "cached_value")

	loaderCalled := false
	loader := func() (interface{}, error) {
		loaderCalled = true
		return "loaded_value", nil
	}

	value, err := cache.GetOrLoad("key1", loader)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if value != "cached_value" {
		t.Errorf("Expected 'cached_value', got: %v", value)
	}
	if loaderCalled {
		t.Error("Loader should not be called on cache hit")
	}
}

// TestGetOrLoad_CacheMiss_LoadSuccess verifies loader is called on cache miss
func TestGetOrLoad_CacheMiss_LoadSuccess(t *testing.T) {
	cache := NewCache(Config{MaxSize: 100})

	loaderCalled := false
	loader := func() (interface{}, error) {
		loaderCalled = true
		return "loaded_value", nil
	}

	value, err := cache.GetOrLoad("key1", loader)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if value != "loaded_value" {
		t.Errorf("Expected 'loaded_value', got: %v", value)
	}
	if !loaderCalled {
		t.Error("Loader should be called on cache miss")
	}

	// Verify value is now cached
	cachedValue, found := cache.Get("key1")
	if !found {
		t.Error("Value should be cached after load")
	}
	if cachedValue != "loaded_value" {
		t.Errorf("Expected cached 'loaded_value', got: %v", cachedValue)
	}
}

// TestGetOrLoad_CacheMiss_LoadError verifies error handling when loader fails
func TestGetOrLoad_CacheMiss_LoadError(t *testing.T) {
	cache := NewCache(Config{MaxSize: 100})

	expectedErr := errors.New("loader failed")
	loader := func() (interface{}, error) {
		return nil, expectedErr
	}

	value, err := cache.GetOrLoad("key1", loader)

	if err == nil {
		t.Error("Expected error from loader")
	}
	if !errors.Is(err, expectedErr) {
		t.Errorf("Expected error to wrap loader error, got: %v", err)
	}
	if value != nil {
		t.Errorf("Expected nil value on error, got: %v", value)
	}

	// Verify error is NOT cached
	_, found := cache.Get("key1")
	if found {
		t.Error("Error should not be cached")
	}
}

// TestGetOrLoad_Concurrent_Singleflight is the CRITICAL test for cache stampede prevention
// Multiple goroutines request same missing key - loader should be called ONCE
func TestGetOrLoad_Concurrent_Singleflight(t *testing.T) {
	cache := NewCache(Config{MaxSize: 100})

	const numGoroutines = 100
	var loaderCallCount int32
	var wg sync.WaitGroup

	// Slow loader to ensure all goroutines arrive during load
	loader := func() (interface{}, error) {
		atomic.AddInt32(&loaderCallCount, 1)
		time.Sleep(50 * time.Millisecond) // Simulate slow operation
		return "loaded_value", nil
	}

	wg.Add(numGoroutines)
	results := make([]interface{}, numGoroutines)
	errs := make([]error, numGoroutines)

	// Launch concurrent requests for same key
	for i := 0; i < numGoroutines; i++ {
		go func(index int) {
			defer wg.Done()
			results[index], errs[index] = cache.GetOrLoad("key1", loader)
		}(i)
	}

	wg.Wait()

	// CRITICAL: Loader should be called exactly ONCE
	if loaderCallCount != 1 {
		t.Errorf("Expected loader to be called exactly once, got: %d", loaderCallCount)
	}

	// All goroutines should get the same value
	for i := 0; i < numGoroutines; i++ {
		if errs[i] != nil {
			t.Errorf("Goroutine %d got error: %v", i, errs[i])
		}
		if results[i] != "loaded_value" {
			t.Errorf("Goroutine %d got wrong value: %v", i, results[i])
		}
	}
}

// TestGetOrLoadWithContext_Timeout verifies context timeout is respected
func TestGetOrLoadWithContext_Timeout(t *testing.T) {
	cache := NewCache(Config{MaxSize: 100})

	loader := func(ctx context.Context) (interface{}, error) {
		// Simulate slow operation
		select {
		case <-time.After(100 * time.Millisecond):
			return "loaded_value", nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	// Context with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	value, err := cache.GetOrLoadWithContext(ctx, "key1", loader)

	if err == nil {
		t.Error("Expected timeout error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected DeadlineExceeded error, got: %v", err)
	}
	if value != nil {
		t.Errorf("Expected nil value on timeout, got: %v", value)
	}
}

// TestGetOrLoadWithContext_Cancellation verifies context cancellation is respected
func TestGetOrLoadWithContext_Cancellation(t *testing.T) {
	cache := NewCache(Config{MaxSize: 100})

	loader := func(ctx context.Context) (interface{}, error) {
		// Simulate slow operation
		select {
		case <-time.After(100 * time.Millisecond):
			return "loaded_value", nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel context after 10ms
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	value, err := cache.GetOrLoadWithContext(ctx, "key1", loader)

	if err == nil {
		t.Error("Expected cancellation error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected Canceled error, got: %v", err)
	}
	if value != nil {
		t.Errorf("Expected nil value on cancellation, got: %v", value)
	}
}

// TestGetOrLoadWithContext_CacheHit verifies context is not needed on cache hit
func TestGetOrLoadWithContext_CacheHit(t *testing.T) {
	cache := NewCache(Config{MaxSize: 100})

	// Pre-populate cache
	cache.Set("key1", "cached_value")

	loader := func(ctx context.Context) (interface{}, error) {
		t.Error("Loader should not be called on cache hit")
		return "loaded_value", nil
	}

	ctx := context.Background()
	value, err := cache.GetOrLoadWithContext(ctx, "key1", loader)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if value != "cached_value" {
		t.Errorf("Expected 'cached_value', got: %v", value)
	}
}

// TestGetOrLoad_NilLoader verifies proper error handling for nil loader
func TestGetOrLoad_NilLoader(t *testing.T) {
	cache := NewCache(Config{MaxSize: 100})

	value, err := cache.GetOrLoad("key1", nil)

	if err == nil {
		t.Error("Expected error for nil loader")
	}
	if value != nil {
		t.Errorf("Expected nil value, got: %v", value)
	}

	// Should be a specific error code
	var baliosErr *goerrors.Error
	if !errors.As(err, &baliosErr) {
		t.Errorf("Expected *errors.Error, got: %T", err)
	} else if string(baliosErr.Code) != "BALIOS_INVALID_LOADER" {
		t.Errorf("Expected BALIOS_INVALID_LOADER, got: %s", baliosErr.Code)
	}
}

// TestGetOrLoad_LoaderPanic verifies panic recovery
func TestGetOrLoad_LoaderPanic(t *testing.T) {
	cache := NewCache(Config{MaxSize: 100})

	loader := func() (interface{}, error) {
		panic("loader panic!")
	}

	value, err := cache.GetOrLoad("key1", loader)

	if err == nil {
		t.Error("Expected error for panicking loader")
	}
	if value != nil {
		t.Errorf("Expected nil value, got: %v", value)
	}

	// Should be a panic recovered error
	var baliosErr *goerrors.Error
	if !errors.As(err, &baliosErr) {
		t.Errorf("Expected *errors.Error, got: %T", err)
	} else if string(baliosErr.Code) != "BALIOS_PANIC_RECOVERED" {
		t.Errorf("Expected BALIOS_PANIC_RECOVERED, got: %s", baliosErr.Code)
	}
}
