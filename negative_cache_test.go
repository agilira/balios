// negative_cache_test.go: tests for negative caching functionality
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGILira fragment
// SPDX-License-Identifier: MPL-2.0

package balios

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestNegativeCaching_Disabled tests that errors are NOT cached when NegativeCacheTTL is 0
func TestNegativeCaching_Disabled(t *testing.T) {
	cache := NewCache(Config{
		MaxSize:          100,
		NegativeCacheTTL: 0, // Disabled
	})

	callCount := 0
	loader := func() (interface{}, error) {
		callCount++
		return nil, errors.New("load failed")
	}

	// First call - should fail
	_, err := cache.GetOrLoad("key1", loader)
	if err == nil {
		t.Error("Expected error from loader")
	}
	if callCount != 1 {
		t.Errorf("Loader should be called once, got %d", callCount)
	}

	// Second call - should call loader again (no caching)
	_, err = cache.GetOrLoad("key1", loader)
	if err == nil {
		t.Error("Expected error from loader")
	}
	if callCount != 2 {
		t.Errorf("Loader should be called twice (no negative caching), got %d", callCount)
	}
}

// TestNegativeCaching_Enabled tests that errors ARE cached when NegativeCacheTTL > 0
func TestNegativeCaching_Enabled(t *testing.T) {
	cache := NewCache(Config{
		MaxSize:          100,
		NegativeCacheTTL: 100 * time.Millisecond,
	})

	callCount := 0
	expectedErr := errors.New("database unavailable")
	loader := func() (interface{}, error) {
		callCount++
		return nil, expectedErr
	}

	// First call - should fail and cache the error
	_, err := cache.GetOrLoad("key1", loader)
	if err == nil {
		t.Error("Expected error from loader")
	}
	if err.Error() != expectedErr.Error() {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}
	if callCount != 1 {
		t.Errorf("Loader should be called once, got %d", callCount)
	}

	// Second call immediately - should return cached error without calling loader
	_, err = cache.GetOrLoad("key1", loader)
	if err == nil {
		t.Error("Expected cached error")
	}
	if err.Error() != expectedErr.Error() {
		t.Errorf("Expected cached error %v, got %v", expectedErr, err)
	}
	if callCount != 1 {
		t.Errorf("Loader should NOT be called again (negative cache hit), got %d calls", callCount)
	}

	// Third call immediately - still should return cached error
	_, err = cache.GetOrLoad("key1", loader)
	if err == nil {
		t.Error("Expected cached error")
	}
	if callCount != 1 {
		t.Errorf("Loader should still NOT be called, got %d calls", callCount)
	}
}

// TestNegativeCaching_Expiration tests that cached errors expire after NegativeCacheTTL
func TestNegativeCaching_Expiration(t *testing.T) {
	cache := NewCache(Config{
		MaxSize:          100,
		NegativeCacheTTL: 50 * time.Millisecond,
	})

	callCount := 0
	loader := func() (interface{}, error) {
		callCount++
		if callCount == 1 {
			return nil, errors.New("temporary failure")
		}
		// Second call succeeds
		return "success", nil
	}

	// First call - fails
	_, err := cache.GetOrLoad("key1", loader)
	if err == nil {
		t.Error("Expected error from first call")
	}
	if callCount != 1 {
		t.Errorf("Loader should be called once, got %d", callCount)
	}

	// Wait for negative cache to expire
	time.Sleep(60 * time.Millisecond)

	// Second call - after expiration, should call loader again and succeed
	value, err := cache.GetOrLoad("key1", loader)
	if err != nil {
		t.Errorf("Expected success after expiration, got error: %v", err)
	}
	if value != "success" {
		t.Errorf("Expected 'success', got %v", value)
	}
	if callCount != 2 {
		t.Errorf("Loader should be called twice (after expiration), got %d", callCount)
	}

	// Third call - should return cached success value
	value, err = cache.GetOrLoad("key1", loader)
	if err != nil {
		t.Errorf("Expected cached success, got error: %v", err)
	}
	if value != "success" {
		t.Errorf("Expected cached 'success', got %v", value)
	}
	if callCount != 2 {
		t.Errorf("Loader should not be called again (cache hit), got %d calls", callCount)
	}
}

// TestNegativeCaching_WithContext tests negative caching with context
func TestNegativeCaching_WithContext(t *testing.T) {
	cache := NewCache(Config{
		MaxSize:          100,
		NegativeCacheTTL: 100 * time.Millisecond,
	})

	callCount := 0
	expectedErr := errors.New("service unavailable")
	loader := func(ctx context.Context) (interface{}, error) {
		callCount++
		return nil, expectedErr
	}

	ctx := context.Background()

	// First call - should fail and cache the error
	_, err := cache.GetOrLoadWithContext(ctx, "key1", loader)
	if err == nil {
		t.Error("Expected error from loader")
	}
	if err.Error() != expectedErr.Error() {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}
	if callCount != 1 {
		t.Errorf("Loader should be called once, got %d", callCount)
	}

	// Second call - should return cached error
	_, err = cache.GetOrLoadWithContext(ctx, "key1", loader)
	if err == nil {
		t.Error("Expected cached error")
	}
	if err.Error() != expectedErr.Error() {
		t.Errorf("Expected cached error %v, got %v", expectedErr, err)
	}
	if callCount != 1 {
		t.Errorf("Loader should NOT be called again (negative cache hit), got %d calls", callCount)
	}
}

// TestNegativeCaching_DifferentKeys tests that negative cache is per-key
func TestNegativeCaching_DifferentKeys(t *testing.T) {
	cache := NewCache(Config{
		MaxSize:          100,
		NegativeCacheTTL: 100 * time.Millisecond,
	})

	key1Calls := 0
	key2Calls := 0

	loader1 := func() (interface{}, error) {
		key1Calls++
		return nil, errors.New("error for key1")
	}

	loader2 := func() (interface{}, error) {
		key2Calls++
		return nil, errors.New("error for key2")
	}

	// Call key1 - should fail
	_, err := cache.GetOrLoad("key1", loader1)
	if err == nil {
		t.Error("Expected error for key1")
	}
	if key1Calls != 1 {
		t.Errorf("key1 loader should be called once, got %d", key1Calls)
	}

	// Call key2 - should fail independently
	_, err = cache.GetOrLoad("key2", loader2)
	if err == nil {
		t.Error("Expected error for key2")
	}
	if key2Calls != 1 {
		t.Errorf("key2 loader should be called once, got %d", key2Calls)
	}

	// Call key1 again - should return cached error
	_, err = cache.GetOrLoad("key1", loader1)
	if err == nil {
		t.Error("Expected cached error for key1")
	}
	if key1Calls != 1 {
		t.Errorf("key1 loader should NOT be called again, got %d calls", key1Calls)
	}

	// Call key2 again - should return cached error
	_, err = cache.GetOrLoad("key2", loader2)
	if err == nil {
		t.Error("Expected cached error for key2")
	}
	if key2Calls != 1 {
		t.Errorf("key2 loader should NOT be called again, got %d calls", key2Calls)
	}
}

// TestNegativeCaching_SuccessOverridesError tests that successful load overrides negative cache
func TestNegativeCaching_SuccessOverridesError(t *testing.T) {
	cache := NewCache(Config{
		MaxSize:          100,
		NegativeCacheTTL: 1 * time.Second, // Long TTL
	})

	callCount := 0
	loader := func() (interface{}, error) {
		callCount++
		return nil, errors.New("failure")
	}

	// First call - fails and cached
	_, err := cache.GetOrLoad("key1", loader)
	if err == nil {
		t.Error("Expected error")
	}
	if callCount != 1 {
		t.Errorf("Expected 1 call, got %d", callCount)
	}

	// Manually set a successful value
	cache.Set("key1", "manual success")

	// Next GetOrLoad should return the successful value (not the cached error)
	value, err := cache.GetOrLoad("key1", loader)
	if err != nil {
		t.Errorf("Expected success, got error: %v", err)
	}
	if value != "manual success" {
		t.Errorf("Expected 'manual success', got %v", value)
	}
	if callCount != 1 {
		t.Errorf("Loader should not be called again, got %d calls", callCount)
	}
}

// TestNegativeCaching_ConcurrentAccess tests thread safety of negative cache
func TestNegativeCaching_ConcurrentAccess(t *testing.T) {
	cache := NewCache(Config{
		MaxSize:          100,
		NegativeCacheTTL: 100 * time.Millisecond,
	})

	callCount := 0
	loader := func() (interface{}, error) {
		callCount++
		time.Sleep(10 * time.Millisecond) // Simulate slow operation
		return nil, errors.New("failure")
	}

	// Launch multiple goroutines concurrently
	const numGoroutines = 10
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			_, err := cache.GetOrLoad("key1", loader)
			if err == nil {
				t.Error("Expected error")
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Loader should be called only once (singleflight)
	if callCount != 1 {
		t.Errorf("Expected 1 loader call (singleflight), got %d", callCount)
	}

	// Subsequent call should return cached error
	_, err := cache.GetOrLoad("key1", loader)
	if err == nil {
		t.Error("Expected cached error")
	}
	if callCount != 1 {
		t.Errorf("Loader should still be called only once, got %d", callCount)
	}
}
