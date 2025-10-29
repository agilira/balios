// cache_key_lifetime_test.go: tests for key lifetime safety
//
// These tests verify that cached keys remain valid even after garbage collection
// and that the cache doesn't hold dangling pointers to caller's string data.
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGILira fragment
// SPDX-License-Identifier: MPL-2.0

package balios

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestKeyLifetimeAfterGC verifies that keys survive garbage collection.
// This is a critical safety test for the unsafe.Pointer key storage.
func TestKeyLifetimeAfterGC(t *testing.T) {
	cache := NewCache(Config{
		MaxSize: 100,
		TTL:     time.Hour,
	})
	defer func() {
		if err := cache.Close(); err != nil {
			t.Errorf("Failed to close cache: %v", err)
		}
	}()

	// Insert keys with temporary strings that will be GC'd
	const numKeys = 50
	for i := 0; i < numKeys; i++ {
		// Create temporary string that will go out of scope
		key := createTemporaryString(i)
		value := fmt.Sprintf("value-%d", i)

		if !cache.Set(key, value) {
			t.Fatalf("Failed to set key %d", i)
		}
	}

	// Force multiple garbage collections to ensure strings are collected
	for i := 0; i < 5; i++ {
		runtime.GC()
		runtime.Gosched()
		time.Sleep(10 * time.Millisecond)
	}

	// Verify all keys are still retrievable with correct values
	for i := 0; i < numKeys; i++ {
		key := createTemporaryString(i) // Recreate the same key
		value, found := cache.Get(key)

		if !found {
			t.Errorf("Key %d not found after GC", i)
			continue
		}

		expectedValue := fmt.Sprintf("value-%d", i)
		if value != expectedValue {
			t.Errorf("Key %d: expected value %q, got %q", i, expectedValue, value)
		}

		// Also verify the key string itself is correct
		if !cache.Has(key) {
			t.Errorf("Has() returned false for key %d after GC", i)
		}
	}
}

// TestKeyLifetimeWithConcurrentGC tests key lifetime under concurrent operations and GC
func TestKeyLifetimeWithConcurrentGC(t *testing.T) {
	cache := NewCache(Config{
		MaxSize: 1000,
		TTL:     time.Hour,
	})
	defer func() {
		if err := cache.Close(); err != nil {
			t.Errorf("Failed to close cache: %v", err)
		}
	}()

	var wg sync.WaitGroup
	const numGoroutines = 10
	const opsPerGoroutine = 100

	// Concurrent writers
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for i := 0; i < opsPerGoroutine; i++ {
				key := createTemporaryString(goroutineID*opsPerGoroutine + i)
				value := fmt.Sprintf("g%d-v%d", goroutineID, i)
				cache.Set(key, value)
			}
		}(g)
	}

	// Concurrent GC forcer
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			runtime.GC()
			time.Sleep(5 * time.Millisecond)
		}
	}()

	wg.Wait()

	// Final GC
	runtime.GC()
	time.Sleep(50 * time.Millisecond)

	// Verify all keys are still valid
	for g := 0; g < numGoroutines; g++ {
		for i := 0; i < opsPerGoroutine; i++ {
			key := createTemporaryString(g*opsPerGoroutine + i)
			value, found := cache.Get(key)

			if found {
				expectedValue := fmt.Sprintf("g%d-v%d", g, i)
				if value != expectedValue {
					t.Errorf("Goroutine %d, key %d: expected %q, got %q", g, i, expectedValue, value)
				}
			}
		}
	}
}

// TestKeyLifetimeWithStringBuilder tests keys created with strings.Builder
// which may have different memory characteristics
func TestKeyLifetimeWithStringBuilder(t *testing.T) {
	cache := NewCache(Config{
		MaxSize: 100,
		TTL:     time.Hour,
	})
	defer func() {
		if err := cache.Close(); err != nil {
			t.Errorf("Failed to close cache: %v", err)
		}
	}()

	// Create keys using strings.Builder
	for i := 0; i < 50; i++ {
		var builder strings.Builder
		builder.WriteString("prefix-")
		builder.WriteString(fmt.Sprintf("%d", i))
		builder.WriteString("-suffix")

		key := builder.String() // String() returns a new string
		value := fmt.Sprintf("value-%d", i)

		if !cache.Set(key, value) {
			t.Fatalf("Failed to set key %d", i)
		}
	}

	// Force GC
	for i := 0; i < 3; i++ {
		runtime.GC()
		runtime.Gosched()
	}

	// Verify all keys
	for i := 0; i < 50; i++ {
		var builder strings.Builder
		builder.WriteString("prefix-")
		builder.WriteString(fmt.Sprintf("%d", i))
		builder.WriteString("-suffix")

		key := builder.String()
		value, found := cache.Get(key)

		if !found {
			t.Errorf("Key %d not found after GC", i)
			continue
		}

		expectedValue := fmt.Sprintf("value-%d", i)
		if value != expectedValue {
			t.Errorf("Key %d: expected %q, got %q", i, expectedValue, value)
		}
	}
}

// TestKeyLifetimeWithSubstrings tests keys that are substrings
// which may share backing arrays with parent strings
func TestKeyLifetimeWithSubstrings(t *testing.T) {
	cache := NewCache(Config{
		MaxSize: 100,
		TTL:     time.Hour,
	})
	defer func() {
		if err := cache.Close(); err != nil {
			t.Errorf("Failed to close cache: %v", err)
		}
	}()

	// Create keys as substrings
	for i := 0; i < 50; i++ {
		parentString := fmt.Sprintf("XXXXXX-key-%d-YYYYYY", i)
		key := parentString[7 : len(parentString)-7] // Extract "key-N"
		value := fmt.Sprintf("value-%d", i)

		if !cache.Set(key, value) {
			t.Fatalf("Failed to set key %d", i)
		}
	}

	// Force GC to potentially collect parent strings
	for i := 0; i < 5; i++ {
		runtime.GC()
		runtime.Gosched()
		time.Sleep(10 * time.Millisecond)
	}

	// Verify all keys
	for i := 0; i < 50; i++ {
		parentString := fmt.Sprintf("XXXXXX-key-%d-YYYYYY", i)
		key := parentString[7 : len(parentString)-7]
		value, found := cache.Get(key)

		if !found {
			t.Errorf("Key %d not found after GC", i)
			continue
		}

		expectedValue := fmt.Sprintf("value-%d", i)
		if value != expectedValue {
			t.Errorf("Key %d: expected %q, got %q", i, expectedValue, value)
		}
	}
}

// TestKeyLifetimeAfterUpdate verifies that updated keys survive GC
func TestKeyLifetimeAfterUpdate(t *testing.T) {
	cache := NewCache(Config{
		MaxSize: 100,
		TTL:     time.Hour,
	})
	defer func() {
		if err := cache.Close(); err != nil {
			t.Errorf("Failed to close cache: %v", err)
		}
	}()

	const numKeys = 30

	// Initial set
	for i := 0; i < numKeys; i++ {
		key := createTemporaryString(i)
		value := fmt.Sprintf("value-v1-%d", i)
		cache.Set(key, value)
	}

	runtime.GC()

	// Update all keys
	for i := 0; i < numKeys; i++ {
		key := createTemporaryString(i)
		value := fmt.Sprintf("value-v2-%d", i)
		cache.Set(key, value)
	}

	// Force GC
	for i := 0; i < 5; i++ {
		runtime.GC()
		runtime.Gosched()
	}

	// Verify updated values
	for i := 0; i < numKeys; i++ {
		key := createTemporaryString(i)
		value, found := cache.Get(key)

		if !found {
			t.Errorf("Key %d not found after update and GC", i)
			continue
		}

		expectedValue := fmt.Sprintf("value-v2-%d", i)
		if value != expectedValue {
			t.Errorf("Key %d: expected updated value %q, got %q", i, expectedValue, value)
		}
	}
}

// TestKeyCorruptionUnderStress is a stress test to detect memory corruption
func TestKeyCorruptionUnderStress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	cache := NewCache(Config{
		MaxSize: 1000,
		TTL:     time.Hour,
	})
	defer func() {
		if err := cache.Close(); err != nil {
			t.Errorf("Failed to close cache: %v", err)
		}
	}()

	var wg sync.WaitGroup
	const duration = 2 * time.Second
	stopTime := time.Now().Add(duration)

	// Continuous writers
	for g := 0; g < 5; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			i := 0
			for time.Now().Before(stopTime) {
				key := fmt.Sprintf("g%d-k%d", goroutineID, i)
				value := fmt.Sprintf("g%d-v%d", goroutineID, i)
				cache.Set(key, value)
				i++
			}
		}(g)
	}

	// Continuous readers
	for g := 0; g < 5; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			i := 0
			for time.Now().Before(stopTime) {
				key := fmt.Sprintf("g%d-k%d", goroutineID, i%100)
				if value, found := cache.Get(key); found {
					// Verify value is not corrupted
					if str, ok := value.(string); ok {
						if !strings.HasPrefix(str, fmt.Sprintf("g%d-v", goroutineID)) {
							t.Errorf("Corrupted value for key %q: %q", key, str)
						}
					}
				}
				i++
			}
		}(g)
	}

	// Continuous GC pressure
	wg.Add(1)
	go func() {
		defer wg.Done()
		for time.Now().Before(stopTime) {
			runtime.GC()
			time.Sleep(10 * time.Millisecond)
		}
	}()

	wg.Wait()
}

// Helper function to create temporary strings that will go out of scope
// This simulates real-world scenarios where keys are constructed and passed to cache
func createTemporaryString(n int) string {
	// Create string in a way that it might be allocated on stack or collected quickly
	return fmt.Sprintf("key-%d", n)
}

// BenchmarkKeyLifetimeOverhead measures the performance impact of safe key storage
func BenchmarkKeyLifetimeOverhead(b *testing.B) {
	cache := NewCache(Config{
		MaxSize: 10000,
		TTL:     time.Hour,
	})
	defer func() {
		if err := cache.Close(); err != nil {
			b.Errorf("Failed to close cache: %v", err)
		}
	}()

	keys := make([]string, 1000)
	for i := range keys {
		keys[i] = fmt.Sprintf("benchmark-key-%d", i)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		key := keys[i%len(keys)]
		cache.Set(key, i)
		cache.Get(key)
	}
}
