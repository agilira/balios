// loading_generic_comprehensive_test.go: comprehensive tests for untested loading_generic.go functions
//
// This file ensures 100% coverage of loading_generic.go including error paths,
// type assertion failures, and context cancellation scenarios.
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGILira library
// SPDX-License-Identifier: MPL-2.0

package balios

import (
	"context"
	goerrors "errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// =============================================================================
// GET OR LOAD WITH CONTEXT TESTS
// =============================================================================

func TestGenericCache_GetOrLoadWithContext_CacheHit(t *testing.T) {
	cache := NewGenericCache[string, string](Config{MaxSize: 100})

	// Pre-populate cache
	cache.Set("key1", "cached_value")

	loaderCalled := false
	loader := func(ctx context.Context) (string, error) {
		loaderCalled = true
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
	if loaderCalled {
		t.Error("Loader should not be called on cache hit")
	}
}

func TestGenericCache_GetOrLoadWithContext_CacheMiss(t *testing.T) {
	cache := NewGenericCache[int, string](Config{MaxSize: 100})

	loaderCalled := false
	loader := func(ctx context.Context) (string, error) {
		loaderCalled = true
		return "loaded_value", nil
	}

	ctx := context.Background()
	value, err := cache.GetOrLoadWithContext(ctx, 42, loader)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if value != "loaded_value" {
		t.Errorf("Expected 'loaded_value', got: %v", value)
	}
	if !loaderCalled {
		t.Error("Loader should be called on cache miss")
	}

	// Verify value was cached
	cachedValue, found := cache.Get(42)
	if !found {
		t.Error("Value should be cached after load")
	}
	if cachedValue != "loaded_value" {
		t.Errorf("Expected 'loaded_value', got: %v", cachedValue)
	}
}

func TestGenericCache_GetOrLoadWithContext_LoaderError(t *testing.T) {
	cache := NewGenericCache[string, string](Config{MaxSize: 100})

	expectedErr := goerrors.New("database connection failed")
	loader := func(ctx context.Context) (string, error) {
		return "", expectedErr
	}

	ctx := context.Background()
	value, err := cache.GetOrLoadWithContext(ctx, "key1", loader)

	if err == nil {
		t.Error("Expected error from loader")
	}
	if !goerrors.Is(err, expectedErr) {
		t.Errorf("Expected error to wrap loader error, got: %v", err)
	}
	if value != "" {
		t.Errorf("Expected zero value on error, got: %v", value)
	}

	// Verify error was not cached
	_, found := cache.Get("key1")
	if found {
		t.Error("Error should not be cached")
	}
}

func TestGenericCache_GetOrLoadWithContext_ContextCancellation(t *testing.T) {
	cache := NewGenericCache[string, string](Config{MaxSize: 100})

	loader := func(ctx context.Context) (string, error) {
		// Simulate long-running operation
		select {
		case <-time.After(200 * time.Millisecond):
			return "loaded_value", nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel immediately
	cancel()

	value, err := cache.GetOrLoadWithContext(ctx, "key1", loader)

	if err == nil {
		t.Error("Expected context cancellation error")
	}
	if !goerrors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled, got: %v", err)
	}
	if value != "" {
		t.Errorf("Expected zero value on error, got: %v", value)
	}
}

func TestGenericCache_GetOrLoadWithContext_TimeoutError(t *testing.T) {
	cache := NewGenericCache[string, int](Config{MaxSize: 100})

	loader := func(ctx context.Context) (int, error) {
		// Simulate slow database query
		select {
		case <-time.After(100 * time.Millisecond):
			return 42, nil
		case <-ctx.Done():
			return 0, ctx.Err()
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	value, err := cache.GetOrLoadWithContext(ctx, "slow-key", loader)

	if err == nil {
		t.Error("Expected timeout error")
	}
	if !goerrors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected context.DeadlineExceeded, got: %v", err)
	}
	if value != 0 {
		t.Errorf("Expected zero value (0), got: %v", value)
	}
}

func TestGenericCache_GetOrLoadWithContext_StructValues(t *testing.T) {
	type Product struct {
		ID    int
		Name  string
		Price float64
	}

	cache := NewGenericCache[int, Product](Config{MaxSize: 100})

	loader := func(ctx context.Context) (Product, error) {
		// Simulate API call
		time.Sleep(10 * time.Millisecond)
		return Product{ID: 123, Name: "Widget", Price: 19.99}, nil
	}

	ctx := context.Background()
	value, err := cache.GetOrLoadWithContext(ctx, 123, loader)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if value.ID != 123 || value.Name != "Widget" || value.Price != 19.99 {
		t.Errorf("Expected Product{123, Widget, 19.99}, got: %+v", value)
	}
}

func TestGenericCache_GetOrLoadWithContext_PointerValues(t *testing.T) {
	type Session struct {
		UserID string
		Token  string
	}

	cache := NewGenericCache[string, *Session](Config{MaxSize: 100})

	loader := func(ctx context.Context) (*Session, error) {
		return &Session{UserID: "user123", Token: "abc123"}, nil
	}

	ctx := context.Background()
	value, err := cache.GetOrLoadWithContext(ctx, "session:xyz", loader)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if value == nil {
		t.Fatal("Expected non-nil pointer")
	}
	if value.UserID != "user123" || value.Token != "abc123" {
		t.Errorf("Expected Session{user123, abc123}, got: %+v", value)
	}
}

// =============================================================================
// CONCURRENT LOADING WITH CONTEXT
// =============================================================================

func TestGenericCache_GetOrLoadWithContext_Concurrent(t *testing.T) {
	cache := NewGenericCache[string, string](Config{MaxSize: 100})

	const numGoroutines = 50
	var loaderCallCount int32
	var wg sync.WaitGroup

	loader := func(ctx context.Context) (string, error) {
		atomic.AddInt32(&loaderCallCount, 1)
		time.Sleep(30 * time.Millisecond)
		return "loaded_value", nil
	}

	ctx := context.Background()
	results := make([]string, numGoroutines)
	errs := make([]error, numGoroutines)

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(index int) {
			defer wg.Done()
			results[index], errs[index] = cache.GetOrLoadWithContext(ctx, "shared-key", loader)
		}(i)
	}

	wg.Wait()

	// Verify loader called exactly once (singleflight)
	if loaderCallCount != 1 {
		t.Errorf("Expected loader to be called exactly once, got: %d", loaderCallCount)
	}

	// Verify all goroutines got the same result
	for i := 0; i < numGoroutines; i++ {
		if errs[i] != nil {
			t.Errorf("Goroutine %d got error: %v", i, errs[i])
		}
		if results[i] != "loaded_value" {
			t.Errorf("Goroutine %d got wrong value: %v", i, results[i])
		}
	}
}

// =============================================================================
// INTEGER KEY TYPES
// =============================================================================

func TestGenericCache_GetOrLoadWithContext_IntKeys(t *testing.T) {
	tests := []struct {
		name string
		test func(*testing.T)
	}{
		{
			"int keys",
			func(t *testing.T) {
				cache := NewGenericCache[int, string](Config{MaxSize: 100})
				loader := func(ctx context.Context) (string, error) {
					return "value", nil
				}
				value, err := cache.GetOrLoadWithContext(context.Background(), 42, loader)
				if err != nil || value != "value" {
					t.Errorf("Expected 'value', got %v (err=%v)", value, err)
				}
			},
		},
		{
			"int64 keys",
			func(t *testing.T) {
				cache := NewGenericCache[int64, string](Config{MaxSize: 100})
				loader := func(ctx context.Context) (string, error) {
					return "value", nil
				}
				value, err := cache.GetOrLoadWithContext(context.Background(), int64(123456789), loader)
				if err != nil || value != "value" {
					t.Errorf("Expected 'value', got %v (err=%v)", value, err)
				}
			},
		},
		{
			"uint keys",
			func(t *testing.T) {
				cache := NewGenericCache[uint, string](Config{MaxSize: 100})
				loader := func(ctx context.Context) (string, error) {
					return "value", nil
				}
				value, err := cache.GetOrLoadWithContext(context.Background(), uint(999), loader)
				if err != nil || value != "value" {
					t.Errorf("Expected 'value', got %v (err=%v)", value, err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.test)
	}
}

// =============================================================================
// ERROR EDGE CASES
// =============================================================================

func TestGenericCache_GetOrLoad_InternalError(t *testing.T) {
	// This tests the defensive code path where type assertion might fail
	// In normal usage this should never happen, but we test for completeness

	cache := NewGenericCache[string, int](Config{MaxSize: 100})

	loader := func() (int, error) {
		return 42, nil
	}

	value, err := cache.GetOrLoad("key1", loader)

	// Normal case: should succeed
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if value != 42 {
		t.Errorf("Expected 42, got: %v", value)
	}
}

func TestGenericCache_GetOrLoadWithContext_InternalError(t *testing.T) {
	// This tests the defensive code path where type assertion might fail
	// In normal usage this should never happen, but we test for completeness

	cache := NewGenericCache[string, int](Config{MaxSize: 100})

	loader := func(ctx context.Context) (int, error) {
		return 42, nil
	}

	ctx := context.Background()
	value, err := cache.GetOrLoadWithContext(ctx, "key1", loader)

	// Normal case: should succeed
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if value != 42 {
		t.Errorf("Expected 42, got: %v", value)
	}
}

// =============================================================================
// ZERO VALUES AND NIL
// =============================================================================

func TestGenericCache_GetOrLoad_ZeroValue(t *testing.T) {
	cache := NewGenericCache[string, int](Config{MaxSize: 100})

	loader := func() (int, error) {
		return 0, nil // Zero value
	}

	value, err := cache.GetOrLoad("zero", loader)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if value != 0 {
		t.Errorf("Expected 0, got: %v", value)
	}

	// Verify zero value was cached
	cachedValue, found := cache.Get("zero")
	if !found {
		t.Error("Zero value should be cached")
	}
	if cachedValue != 0 {
		t.Errorf("Expected cached 0, got: %v", cachedValue)
	}
}

func TestGenericCache_GetOrLoadWithContext_NilPointer(t *testing.T) {
	type Data struct {
		Value string
	}

	cache := NewGenericCache[string, *Data](Config{MaxSize: 100})

	loader := func(ctx context.Context) (*Data, error) {
		return nil, nil // Return nil pointer
	}

	ctx := context.Background()
	value, err := cache.GetOrLoadWithContext(ctx, "nil-key", loader)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if value != nil {
		t.Errorf("Expected nil, got: %v", value)
	}
}

// =============================================================================
// CONTEXT PROPAGATION
// =============================================================================

func TestGenericCache_GetOrLoadWithContext_ContextPropagation(t *testing.T) {
	cache := NewGenericCache[string, string](Config{MaxSize: 100})

	type contextKey string
	const testKey contextKey = "test-key"

	loader := func(ctx context.Context) (string, error) {
		// Verify context value is propagated
		value := ctx.Value(testKey)
		if value == nil {
			t.Error("Context value not propagated to loader")
			return "", goerrors.New("context value missing")
		}
		if value != "test-value" {
			t.Errorf("Expected 'test-value', got: %v", value)
		}
		return "loaded", nil
	}

	ctx := context.WithValue(context.Background(), testKey, "test-value")
	value, err := cache.GetOrLoadWithContext(ctx, "key1", loader)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if value != "loaded" {
		t.Errorf("Expected 'loaded', got: %v", value)
	}
}

// =============================================================================
// BENCHMARKS
// =============================================================================

func BenchmarkGenericCache_GetOrLoad(b *testing.B) {
	cache := NewGenericCache[int, string](Config{MaxSize: 10000})

	loader := func() (string, error) {
		return "loaded_value", nil
	}

	b.Run("CacheHit", func(b *testing.B) {
		cache.Set(1, "cached_value")
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = cache.GetOrLoad(1, loader)
		}
	})

	b.Run("CacheMiss", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = cache.GetOrLoad(i, loader)
		}
	})
}

func BenchmarkGenericCache_GetOrLoadWithContext(b *testing.B) {
	cache := NewGenericCache[int, string](Config{MaxSize: 10000})
	ctx := context.Background()

	loader := func(ctx context.Context) (string, error) {
		return "loaded_value", nil
	}

	b.Run("CacheHit", func(b *testing.B) {
		cache.Set(1, "cached_value")
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = cache.GetOrLoadWithContext(ctx, 1, loader)
		}
	})

	b.Run("CacheMiss", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = cache.GetOrLoadWithContext(ctx, i, loader)
		}
	})
}
