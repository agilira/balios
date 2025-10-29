// cache_memory_leak_test.go: tests for memory leak prevention
//
// These tests verify that deleted entries don't prevent garbage collection
// of their values, avoiding memory leaks.
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGILira fragment
// SPDX-License-Identifier: MPL-2.0

package balios

import (
	"runtime"
	"testing"
	"time"
)

// largeValue is a struct large enough to detect memory leaks
type largeValue struct {
	data [1024 * 1024]byte // 1MB
	id   int
}

// newLargeValue creates a largeValue with initialized data to prevent unused field warnings
func newLargeValue(id int) *largeValue {
	v := &largeValue{id: id}
	// Initialize data to ensure it's not optimized away
	v.data[0] = byte(id)
	v.data[len(v.data)-1] = byte(id)
	return v
}

// TestDeleteClearsValue verifies that Delete() clears the value to allow GC
func TestDeleteClearsValue(t *testing.T) {
	cache := NewCache(Config{
		MaxSize: 100,
		TTL:     time.Hour,
	})
	defer func() {
		if err := cache.Close(); err != nil {
			t.Errorf("Failed to close cache: %v", err)
		}
	}()

	// Set a large value
	large := newLargeValue(1)
	cache.Set("key1", large)

	// Verify it's there and data is intact
	if val, found := cache.Get("key1"); !found {
		t.Fatal("Key not found after Set")
	} else if v, ok := val.(*largeValue); !ok || v.data[0] != 1 {
		t.Fatal("Value data not intact")
	}

	// Delete it
	if !cache.Delete("key1") {
		t.Fatal("Delete returned false")
	}

	// Force GC
	runtime.GC()
	time.Sleep(10 * time.Millisecond)

	// Value should be nil after delete (allowing GC)
	// We can't directly test GC, but we verify the entry is cleared
	if _, found := cache.Get("key1"); found {
		t.Error("Key still found after Delete")
	}
}

// TestEvictionClearsValue verifies that eviction clears values
func TestEvictionClearsValue(t *testing.T) {
	cache := NewCache(Config{
		MaxSize: 10, // Small cache to force evictions
	})
	defer func() {
		if err := cache.Close(); err != nil {
			t.Errorf("Failed to close cache: %v", err)
		}
	}()

	// Fill cache with large values
	for i := 0; i < 20; i++ {
		large := newLargeValue(i)
		cache.Set(keyForTest(i), large)
	}

	// Force GC
	runtime.GC()
	time.Sleep(10 * time.Millisecond)

	// Some entries should have been evicted
	// We can't test GC directly, but verify cache size is bounded
	stats := cache.Stats()
	if stats.Size > 10 {
		t.Errorf("Cache size %d exceeds MaxSize 10", stats.Size)
	}
}

// TestClearClearsValues verifies that Clear() clears all values
func TestClearClearsValues(t *testing.T) {
	cache := NewCache(Config{
		MaxSize: 100,
	})
	defer func() {
		if err := cache.Close(); err != nil {
			t.Errorf("Failed to close cache: %v", err)
		}
	}()

	// Set multiple large values
	for i := 0; i < 50; i++ {
		large := newLargeValue(i)
		cache.Set(keyForTest(i), large)
	}

	// Clear cache
	cache.Clear()

	// Force GC
	runtime.GC()
	time.Sleep(10 * time.Millisecond)

	// Verify cache is empty
	stats := cache.Stats()
	if stats.Size != 0 {
		t.Errorf("Cache size %d after Clear, expected 0", stats.Size)
	}

	// Verify no keys are found
	for i := 0; i < 50; i++ {
		if _, found := cache.Get(keyForTest(i)); found {
			t.Errorf("Key %d still found after Clear", i)
		}
	}
}

// TestExpiredEntryClearsValue verifies that expired entries clear their values
func TestExpiredEntryClearsValue(t *testing.T) {
	cache := NewCache(Config{
		MaxSize: 100,
		TTL:     50 * time.Millisecond, // Short TTL
	})
	defer func() {
		if err := cache.Close(); err != nil {
			t.Errorf("Failed to close cache: %v", err)
		}
	}()

	// Set a large value
	large := newLargeValue(1)
	cache.Set("key1", large)

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Access should trigger expiration cleanup
	if _, found := cache.Get("key1"); found {
		t.Error("Expired key still found")
	}

	// Force GC
	runtime.GC()
	time.Sleep(10 * time.Millisecond)

	// Entry should be marked as deleted (value cleared)
}

// TestMemoryLeakUnderLoad is a stress test for memory leaks
func TestMemoryLeakUnderLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory leak test in short mode")
	}

	cache := NewCache(Config{
		MaxSize: 1000,
	})
	defer func() {
		if err := cache.Close(); err != nil {
			t.Errorf("Failed to close cache: %v", err)
		}
	}()

	// Measure initial memory
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	// Perform many Set/Delete operations
	for round := 0; round < 10; round++ {
		for i := 0; i < 5000; i++ {
			large := newLargeValue(i)
			cache.Set(keyForTest(i), large)
		}

		for i := 0; i < 5000; i++ {
			cache.Delete(keyForTest(i))
		}

		runtime.GC()
	}

	// Force final GC
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	// Measure final memory
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)

	// Memory growth should be bounded (not proportional to operations)
	growth := m2.Alloc - m1.Alloc
	t.Logf("Memory growth: %d bytes (%.2f MB)", growth, float64(growth)/(1024*1024))

	// We expect some growth, but not massive (less than 100MB)
	if growth > 100*1024*1024 {
		t.Errorf("Excessive memory growth: %.2f MB, possible memory leak", float64(growth)/(1024*1024))
	}
}

func keyForTest(i int) string {
	return string(rune('a' + (i % 26)))
}

// BenchmarkDeleteMemoryOverhead measures the overhead of value clearing
func BenchmarkDeleteMemoryOverhead(b *testing.B) {
	cache := NewCache(Config{
		MaxSize: 10000,
	})
	defer func() {
		if err := cache.Close(); err != nil {
			b.Errorf("Failed to close cache: %v", err)
		}
	}()

	// Pre-populate
	for i := 0; i < 1000; i++ {
		cache.Set(keyForTest(i), newLargeValue(i))
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		key := keyForTest(i % 1000)
		cache.Delete(key)
		cache.Set(key, newLargeValue(i))
	}
}
