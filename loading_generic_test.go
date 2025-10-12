package balios

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestGenericCache_GetOrLoad_CacheHit verifies generic API with cache hit
func TestGenericCache_GetOrLoad_CacheHit(t *testing.T) {
	cache := NewGenericCache[string, string](Config{MaxSize: 100})

	cache.Set("key1", "cached_value")

	loaderCalled := false
	loader := func() (string, error) {
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

// TestGenericCache_GetOrLoad_CacheMiss verifies generic API with cache miss
func TestGenericCache_GetOrLoad_CacheMiss(t *testing.T) {
	cache := NewGenericCache[int, string](Config{MaxSize: 100})

	loaderCalled := false
	loader := func() (string, error) {
		loaderCalled = true
		return "loaded_value", nil
	}

	value, err := cache.GetOrLoad(42, loader)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if value != "loaded_value" {
		t.Errorf("Expected 'loaded_value', got: %v", value)
	}
	if !loaderCalled {
		t.Error("Loader should be called on cache miss")
	}

	// Verify cached
	cachedValue, found := cache.Get(42)
	if !found {
		t.Error("Value should be cached")
	}
	if cachedValue != "loaded_value" {
		t.Errorf("Expected 'loaded_value', got: %v", cachedValue)
	}
}

// TestGenericCache_GetOrLoad_StructValue verifies generic API with complex types
func TestGenericCache_GetOrLoad_StructValue(t *testing.T) {
	type User struct {
		ID   int
		Name string
	}

	cache := NewGenericCache[int, User](Config{MaxSize: 100})

	loader := func() (User, error) {
		return User{ID: 1, Name: "Alice"}, nil
	}

	value, err := cache.GetOrLoad(1, loader)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if value.ID != 1 || value.Name != "Alice" {
		t.Errorf("Expected User{1, 'Alice'}, got: %+v", value)
	}
}

// TestGenericCache_GetOrLoad_PointerValue verifies generic API with pointer types
func TestGenericCache_GetOrLoad_PointerValue(t *testing.T) {
	type Data struct {
		Value string
	}

	cache := NewGenericCache[string, *Data](Config{MaxSize: 100})

	loader := func() (*Data, error) {
		return &Data{Value: "test"}, nil
	}

	value, err := cache.GetOrLoad("key1", loader)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if value == nil {
		t.Error("Expected non-nil pointer")
		return
	}
	if value.Value != "test" {
		t.Errorf("Expected 'test', got: %v", value.Value)
	}
}

// TestGenericCache_GetOrLoad_Concurrent verifies singleflight with generic API
func TestGenericCache_GetOrLoad_Concurrent(t *testing.T) {
	cache := NewGenericCache[string, string](Config{MaxSize: 100})

	const numGoroutines = 100
	var loaderCallCount int32
	var wg sync.WaitGroup

	loader := func() (string, error) {
		atomic.AddInt32(&loaderCallCount, 1)
		time.Sleep(50 * time.Millisecond)
		return "loaded_value", nil
	}

	wg.Add(numGoroutines)
	results := make([]string, numGoroutines)
	errs := make([]error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(index int) {
			defer wg.Done()
			results[index], errs[index] = cache.GetOrLoad("key1", loader)
		}(i)
	}

	wg.Wait()

	// CRITICAL: Loader called exactly once
	if loaderCallCount != 1 {
		t.Errorf("Expected loader to be called exactly once, got: %d", loaderCallCount)
	}

	for i := 0; i < numGoroutines; i++ {
		if errs[i] != nil {
			t.Errorf("Goroutine %d got error: %v", i, errs[i])
		}
		if results[i] != "loaded_value" {
			t.Errorf("Goroutine %d got wrong value: %v", i, results[i])
		}
	}
}

// TestGenericCache_GetOrLoadWithContext_Timeout verifies context timeout with generic API
func TestGenericCache_GetOrLoadWithContext_Timeout(t *testing.T) {
	cache := NewGenericCache[string, string](Config{MaxSize: 100})

	loader := func(ctx context.Context) (string, error) {
		select {
		case <-time.After(100 * time.Millisecond):
			return "loaded_value", nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	value, err := cache.GetOrLoadWithContext(ctx, "key1", loader)

	if err == nil {
		t.Error("Expected timeout error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected DeadlineExceeded, got: %v", err)
	}
	if value != "" {
		t.Errorf("Expected zero value, got: %v", value)
	}
}

// TestGenericCache_GetOrLoad_LoadError verifies error handling with generic API
func TestGenericCache_GetOrLoad_LoadError(t *testing.T) {
	cache := NewGenericCache[string, string](Config{MaxSize: 100})

	expectedErr := errors.New("load failed")
	loader := func() (string, error) {
		return "", expectedErr
	}

	value, err := cache.GetOrLoad("key1", loader)

	if err == nil {
		t.Error("Expected error from loader")
	}
	if !errors.Is(err, expectedErr) {
		t.Errorf("Expected error to wrap loader error, got: %v", err)
	}
	if value != "" {
		t.Errorf("Expected zero value on error, got: %v", value)
	}

	// Verify error not cached
	_, found := cache.Get("key1")
	if found {
		t.Error("Error should not be cached")
	}
}
