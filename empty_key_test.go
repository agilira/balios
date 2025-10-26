// empty_key_test.go: tests for empty key validation
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGILira library
// SPDX-License-Identifier: MPL-2.0

package balios

import (
	"context"
	"testing"
	"time"
)

// TestEmptyKeyValidation_Cache tests empty key validation for Cache interface
func TestEmptyKeyValidation_Cache(t *testing.T) {
	cache := NewCache(Config{
		MaxSize: 100,
		TTL:     time.Minute,
	})

	t.Run("Set with empty key returns false", func(t *testing.T) {
		success := cache.Set("", "value")
		if success {
			t.Error("Set with empty key should return false")
		}

		// Verify size is still 0
		if cache.Len() != 0 {
			t.Errorf("Cache size should be 0, got %d", cache.Len())
		}
	})

	t.Run("Get with empty key returns not found", func(t *testing.T) {
		value, found := cache.Get("")
		if found {
			t.Error("Get with empty key should return false")
		}
		if value != nil {
			t.Errorf("Get with empty key should return nil value, got %v", value)
		}
	})

	t.Run("Delete with empty key returns false", func(t *testing.T) {
		success := cache.Delete("")
		if success {
			t.Error("Delete with empty key should return false")
		}
	})

	t.Run("Has with empty key returns false", func(t *testing.T) {
		exists := cache.Has("")
		if exists {
			t.Error("Has with empty key should return false")
		}
	})
}

// TestEmptyKeyValidation_GenericCache tests empty key validation for GenericCache
func TestEmptyKeyValidation_GenericCache(t *testing.T) {
	cache := NewGenericCache[string, string](Config{
		MaxSize: 100,
		TTL:     time.Minute,
	})

	t.Run("Set with empty string key", func(t *testing.T) {
		cache.Set("", "value")

		// Verify it wasn't stored
		if cache.Len() != 0 {
			t.Errorf("Cache size should be 0, got %d", cache.Len())
		}
	})

	t.Run("Get with empty string key", func(t *testing.T) {
		value, found := cache.Get("")
		if found {
			t.Error("Get with empty key should return false")
		}
		if value != "" {
			t.Errorf("Get with empty key should return zero value, got %q", value)
		}
	})

	t.Run("Delete with empty string key", func(t *testing.T) {
		cache.Delete("")
		// Should not panic or error
	})

	t.Run("Has with empty string key", func(t *testing.T) {
		exists := cache.Has("")
		if exists {
			t.Error("Has with empty key should return false")
		}
	})
}

// TestEmptyKeyValidation_GetOrLoad tests empty key validation for GetOrLoad
func TestEmptyKeyValidation_GetOrLoad(t *testing.T) {
	cache := NewCache(Config{
		MaxSize: 100,
		TTL:     time.Minute,
	})

	t.Run("GetOrLoad with empty key returns error", func(t *testing.T) {
		loaderCalled := false
		loader := func() (interface{}, error) {
			loaderCalled = true
			return "value", nil
		}

		value, err := cache.GetOrLoad("", loader)

		// Should return error without calling loader
		if err == nil {
			t.Error("GetOrLoad with empty key should return error")
		}
		if !IsEmptyKey(err) {
			t.Errorf("Error should be ErrCodeEmptyKey, got %v", err)
		}
		if value != nil {
			t.Errorf("Value should be nil, got %v", value)
		}
		if loaderCalled {
			t.Error("Loader should not be called for empty key")
		}
	})

	t.Run("GetOrLoadWithContext with empty key returns error", func(t *testing.T) {
		ctx := context.Background()
		loaderCalled := false
		loader := func(ctx context.Context) (interface{}, error) {
			loaderCalled = true
			return "value", nil
		}

		value, err := cache.GetOrLoadWithContext(ctx, "", loader)

		// Should return error without calling loader
		if err == nil {
			t.Error("GetOrLoadWithContext with empty key should return error")
		}
		if !IsEmptyKey(err) {
			t.Errorf("Error should be ErrCodeEmptyKey, got %v", err)
		}
		if value != nil {
			t.Errorf("Value should be nil, got %v", value)
		}
		if loaderCalled {
			t.Error("Loader should not be called for empty key")
		}
	})
}

// TestEmptyKeyValidation_GenericGetOrLoad tests empty key validation for generic GetOrLoad
func TestEmptyKeyValidation_GenericGetOrLoad(t *testing.T) {
	cache := NewGenericCache[string, string](Config{
		MaxSize: 100,
		TTL:     time.Minute,
	})

	t.Run("GenericCache GetOrLoad with empty key returns error", func(t *testing.T) {
		loaderCalled := false
		loader := func() (string, error) {
			loaderCalled = true
			return "value", nil
		}

		value, err := cache.GetOrLoad("", loader)

		// Should return error without calling loader
		if err == nil {
			t.Error("GetOrLoad with empty key should return error")
		}
		if !IsEmptyKey(err) {
			t.Errorf("Error should be ErrCodeEmptyKey, got %v", err)
		}
		if value != "" {
			t.Errorf("Value should be empty string, got %q", value)
		}
		if loaderCalled {
			t.Error("Loader should not be called for empty key")
		}
	})

	t.Run("GenericCache GetOrLoadWithContext with empty key returns error", func(t *testing.T) {
		ctx := context.Background()
		loaderCalled := false
		loader := func(ctx context.Context) (string, error) {
			loaderCalled = true
			return "value", nil
		}

		value, err := cache.GetOrLoadWithContext(ctx, "", loader)

		// Should return error without calling loader
		if err == nil {
			t.Error("GetOrLoadWithContext with empty key should return error")
		}
		if !IsEmptyKey(err) {
			t.Errorf("Error should be ErrCodeEmptyKey, got %v", err)
		}
		if value != "" {
			t.Errorf("Value should be empty string, got %q", value)
		}
		if loaderCalled {
			t.Error("Loader should not be called for empty key")
		}
	})
}

// TestEmptyKeyDoesNotAffectValidKeys verifies empty key validation doesn't affect valid operations
func TestEmptyKeyDoesNotAffectValidKeys(t *testing.T) {
	cache := NewCache(Config{
		MaxSize: 100,
		TTL:     time.Minute,
	})

	// Set valid key
	if !cache.Set("valid", "value") {
		t.Error("Set with valid key should succeed")
	}

	// Try to set empty key (should fail)
	if cache.Set("", "empty") {
		t.Error("Set with empty key should fail")
	}

	// Verify valid key still exists
	value, found := cache.Get("valid")
	if !found {
		t.Error("Valid key should still exist")
	}
	if value != "value" {
		t.Errorf("Valid key should have correct value, got %v", value)
	}

	// Verify cache size is 1 (only valid key)
	if cache.Len() != 1 {
		t.Errorf("Cache should have 1 entry, got %d", cache.Len())
	}
}

// TestEmptyKeyWithMetrics verifies empty key operations don't affect metrics
func TestEmptyKeyWithMetrics(t *testing.T) {
	cache := NewCache(Config{
		MaxSize: 100,
		TTL:     time.Minute,
	})

	// Get initial stats
	stats := cache.Stats()
	if stats.Sets != 0 || stats.Hits != 0 || stats.Misses != 0 {
		t.Errorf("Initial stats should be zero, got %+v", stats)
	}

	// Operations with empty keys
	cache.Set("", "value")
	cache.Get("")
	cache.Delete("")

	// Verify stats are still zero (empty keys don't count)
	stats = cache.Stats()
	if stats.Sets != 0 {
		t.Errorf("Sets should be 0 after empty key operations, got %d", stats.Sets)
	}
	if stats.Hits != 0 {
		t.Errorf("Hits should be 0 after empty key operations, got %d", stats.Hits)
	}
	// Note: Get with empty key might increment misses in current implementation
	// We could argue either way - for now we accept both behaviors
}
