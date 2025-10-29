// bounded_probing_test.go: tests for bounded linear probing improvement (v1.1.35)
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGILira fragment
// SPDX-License-Identifier: MPL-2.0

package balios

import (
	"fmt"
	"testing"
)

// TestBoundedProbing_NormalCase verifies that bounded probing works correctly
// in normal scenarios (most common case)
func TestBoundedProbing_NormalCase(t *testing.T) {
	cache := NewCache(Config{
		MaxSize: 1000,
	})

	// Insert keys normally
	for i := 0; i < 500; i++ {
		key := fmt.Sprintf("key-%d", i)
		if !cache.Set(key, i) {
			t.Fatalf("Failed to set key %s", key)
		}
	}

	// Verify all keys are retrievable
	for i := 0; i < 500; i++ {
		key := fmt.Sprintf("key-%d", i)
		val, found := cache.Get(key)
		if !found {
			t.Errorf("Key %s not found", key)
		}
		if val != i {
			t.Errorf("Key %s: expected %d, got %v", key, i, val)
		}
	}

	// Verify Has() works
	for i := 0; i < 500; i++ {
		key := fmt.Sprintf("key-%d", i)
		if !cache.Has(key) {
			t.Errorf("Has(%s) returned false", key)
		}
	}

	// Verify Delete() works
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("key-%d", i)
		if !cache.Delete(key) {
			t.Errorf("Failed to delete key %s", key)
		}
	}

	// Verify deleted keys are gone
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("key-%d", i)
		if _, found := cache.Get(key); found {
			t.Errorf("Deleted key %s still found", key)
		}
	}

	t.Logf("✅ Normal bounded probing: %d keys inserted, retrieved, and deleted successfully", 500)
}

// TestBoundedProbing_HighLoadFactor tests behavior at high load factor
// where probe lengths are longer but still bounded
func TestBoundedProbing_HighLoadFactor(t *testing.T) {
	cache := NewCache(Config{
		MaxSize: 1000,
	})

	// Fill to 90% capacity (high load factor)
	insertCount := 900
	for i := 0; i < insertCount; i++ {
		key := fmt.Sprintf("high-load-key-%d", i)
		if !cache.Set(key, i) {
			t.Fatalf("Failed to set key at high load: %s", key)
		}
	}

	if cache.Len() != insertCount {
		t.Fatalf("Expected %d entries, got %d", insertCount, cache.Len())
	}

	// Verify all keys are still retrievable
	missingKeys := 0
	for i := 0; i < insertCount; i++ {
		key := fmt.Sprintf("high-load-key-%d", i)
		if _, found := cache.Get(key); !found {
			missingKeys++
		}
	}

	if missingKeys > 0 {
		t.Logf("⚠️  Missing %d keys at high load (some eviction expected)", missingKeys)
	}

	// Insert more keys - should trigger bounded probing fallback occasionally
	additionalKeys := 200
	successCount := 0
	for i := insertCount; i < insertCount+additionalKeys; i++ {
		key := fmt.Sprintf("high-load-key-%d", i)
		if cache.Set(key, i) {
			successCount++
		}
	}

	t.Logf("✅ High load factor: %d/%d additional keys inserted successfully", successCount, additionalKeys)

	// Cache should not exceed capacity significantly
	finalSize := cache.Len()
	if finalSize > cache.Capacity()+50 {
		t.Errorf("Cache size %d exceeds capacity %d by too much", finalSize, cache.Capacity())
	}
}

// TestBoundedProbing_ConcurrentOps verifies bounded probing under concurrent load
func TestBoundedProbing_ConcurrentOps(t *testing.T) {
	cache := NewCache(Config{
		MaxSize: 10000,
	})

	// Pre-populate to 70% load
	for i := 0; i < 7000; i++ {
		cache.Set(fmt.Sprintf("init-key-%d", i), i)
	}

	// Concurrent operations
	const goroutines = 10
	const opsPerGoroutine = 1000

	errors := make(chan error, goroutines)
	done := make(chan bool, goroutines)

	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer func() { done <- true }()

			for i := 0; i < opsPerGoroutine; i++ {
				key := fmt.Sprintf("concurrent-key-%d-%d", id, i)

				// Set
				if !cache.Set(key, i) {
					// Set can fail at high load, that's okay
					continue
				}

				// Get
				val, found := cache.Get(key)
				if found && val != i {
					errors <- fmt.Errorf("goroutine %d: key %s wrong value: expected %d, got %v", id, key, i, val)
					return
				}

				// Has
				cache.Has(key)

				// Delete (some)
				if i%10 == 0 {
					cache.Delete(key)
				}
			}
		}(g)
	}

	// Wait for completion
	for g := 0; g < goroutines; g++ {
		<-done
	}
	close(errors)

	// Check for errors
	errorCount := 0
	for err := range errors {
		t.Error(err)
		errorCount++
	}

	if errorCount > 0 {
		t.Fatalf("Concurrent bounded probing test failed with %d errors", errorCount)
	}

	t.Logf("✅ Concurrent bounded probing: %d goroutines × %d ops completed successfully", goroutines, opsPerGoroutine)
}

// TestBoundedProbing_EvictionFallback tests that eviction fallback works
// when probe limit is reached
func TestBoundedProbing_EvictionFallback(t *testing.T) {
	// Small cache to trigger fallback more easily
	cache := NewCache(Config{
		MaxSize: 100,
	})

	// Fill cache completely
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("fill-key-%d", i)
		if !cache.Set(key, i) {
			t.Fatalf("Failed to fill cache at key %s", key)
		}
	}

	// Continue inserting - should trigger eviction fallback
	extraInserts := 50
	successCount := 0
	for i := 100; i < 100+extraInserts; i++ {
		key := fmt.Sprintf("overflow-key-%d", i)
		if cache.Set(key, i) {
			successCount++
		}
	}

	// Should have succeeded via eviction fallback
	if successCount == 0 {
		t.Error("Eviction fallback did not work - no keys inserted after capacity reached")
	}

	// Cache size should be near capacity (some evictions happened)
	finalSize := cache.Len()
	if finalSize < 50 || finalSize > 110 {
		t.Errorf("Unexpected cache size after eviction fallback: %d (expected ~100)", finalSize)
	}

	t.Logf("✅ Eviction fallback: %d/%d keys inserted after capacity, final size: %d",
		successCount, extraInserts, finalSize)
}

// TestBoundedProbing_MaxProbeLength verifies maxProbeLength constant is sensible
func TestBoundedProbing_MaxProbeLength(t *testing.T) {
	// This is a compile-time constant check
	if maxProbeLength < 64 {
		t.Errorf("maxProbeLength too small: %d (minimum recommended: 64)", maxProbeLength)
	}
	if maxProbeLength > 256 {
		t.Errorf("maxProbeLength too large: %d (maximum recommended: 256)", maxProbeLength)
	}

	t.Logf("✅ maxProbeLength constant: %d (reasonable bounds)", maxProbeLength)
}

// TestBoundedProbing_SmallCache verifies bounded probing works even with tiny caches
func TestBoundedProbing_SmallCache(t *testing.T) {
	// Cache smaller than maxProbeLength
	cache := NewCache(Config{
		MaxSize: 10,
	})

	// Insert more than capacity
	for i := 0; i < 20; i++ {
		key := fmt.Sprintf("tiny-key-%d", i)
		// May fail, but should not panic or hang
		cache.Set(key, i)
	}

	// Should have some entries (eviction happened)
	size := cache.Len()
	if size == 0 {
		t.Error("Small cache is empty after inserts")
	}
	if size > 15 {
		t.Errorf("Small cache size %d exceeds expected range", size)
	}

	t.Logf("✅ Small cache bounded probing: size=%d after 20 inserts (capacity=10)", size)
}

// TestBoundedProbing_GetAfterEviction verifies Get still works after eviction fallback
func TestBoundedProbing_GetAfterEviction(t *testing.T) {
	cache := NewCache(Config{
		MaxSize: 100,
	})

	// Fill cache
	for i := 0; i < 100; i++ {
		cache.Set(fmt.Sprintf("key-%d", i), i)
	}

	// Insert many more to trigger evictions
	for i := 100; i < 200; i++ {
		cache.Set(fmt.Sprintf("key-%d", i), i)
	}

	// Try to retrieve recent keys (should be present)
	foundCount := 0
	for i := 150; i < 200; i++ {
		key := fmt.Sprintf("key-%d", i)
		if _, found := cache.Get(key); found {
			foundCount++
		}
	}

	// Should find at least some recent keys (eviction is probabilistic with W-TinyLFU)
	if foundCount < 30 {
		t.Errorf("Too few recent keys found after eviction: %d/50", foundCount)
	}

	t.Logf("✅ Get after eviction: %d/50 recent keys still present", foundCount)
}
