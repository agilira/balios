// race_test.go: comprehensive data race tests for Balios
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGILira library
// SPDX-License-Identifier: MPL-2.0

package balios

import (
	"fmt"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestRaceConditions_ConcurrentSetGet tests for data races during concurrent Set/Get operations
func TestRaceConditions_ConcurrentSetGet(t *testing.T) {
	cache := NewCache(Config{MaxSize: 1000})
	const numGoroutines = 100
	const numOperations = 1000

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Launch concurrent Set/Get operations
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < numOperations; j++ {
				key := strconv.Itoa((goroutineID*numOperations + j) % 100) // Key collision intentional
				value := goroutineID*numOperations + j

				// Mix Set and Get operations
				if j%2 == 0 {
					cache.Set(key, value)
				} else {
					cache.Get(key)
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify cache integrity
	stats := cache.Stats()
	if stats.Size < 0 || stats.Size > 1000 {
		t.Errorf("Cache size corrupted: %d", stats.Size)
	}
}

// TestRaceConditions_ConcurrentSetUpdate tests for data races during concurrent updates of same key
func TestRaceConditions_ConcurrentSetUpdate(t *testing.T) {
	cache := NewCache(Config{MaxSize: 100})
	const numGoroutines = 50
	const numUpdates = 100
	const testKey = "race-test-key"

	var wg sync.WaitGroup
	var successCount int64

	wg.Add(numGoroutines)

	// All goroutines update the same key
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < numUpdates; j++ {
				value := goroutineID*numUpdates + j
				if cache.Set(testKey, value) {
					atomic.AddInt64(&successCount, 1)
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify final state
	finalValue, found := cache.Get(testKey)
	if !found {
		t.Error("Key should exist after concurrent updates")
	}
	if finalValue == nil {
		t.Error("Final value should not be nil")
	}

	// All Sets should succeed for the same key (updates)
	expectedSuccess := int64(numGoroutines * numUpdates)
	if successCount != expectedSuccess {
		t.Errorf("Expected %d successful sets, got %d", expectedSuccess, successCount)
	}
}

// TestRaceConditions_ConcurrentSetDelete tests for data races between Set and Delete operations
func TestRaceConditions_ConcurrentSetDelete(t *testing.T) {
	cache := NewCache(Config{MaxSize: 100})
	const numGoroutines = 50
	const numOperations = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 2) // Half for Set, half for Delete

	keys := make([]string, numOperations)
	for i := 0; i < numOperations; i++ {
		keys[i] = "key-" + strconv.Itoa(i)
	}

	// Setters
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < numOperations; j++ {
				key := keys[j]
				value := goroutineID*numOperations + j
				cache.Set(key, value)
			}
		}(i)
	}

	// Deleters
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < numOperations; j++ {
				key := keys[j]
				cache.Delete(key)
			}
		}(i)
	}

	wg.Wait()

	// Verify cache integrity
	stats := cache.Stats()
	if stats.Size < 0 || stats.Size > 100 {
		t.Errorf("Cache size corrupted: %d", stats.Size)
	}
}

// TestRaceConditions_ConcurrentGetHas tests for data races between Get and Has operations
func TestRaceConditions_ConcurrentGetHas(t *testing.T) {
	cache := NewCache(Config{MaxSize: 100})

	// Pre-populate cache
	for i := 0; i < 50; i++ {
		cache.Set("key-"+strconv.Itoa(i), i)
	}

	const numGoroutines = 50
	const numOperations = 1000

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 2) // Half for Get, half for Has

	// Getters
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()

			for j := 0; j < numOperations; j++ {
				key := "key-" + strconv.Itoa(j%50)
				cache.Get(key)
			}
		}()
	}

	// Has checkers
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()

			for j := 0; j < numOperations; j++ {
				key := "key-" + strconv.Itoa(j%50)
				cache.Has(key)
			}
		}()
	}

	wg.Wait()

	// Verify no corruption occurred
	count := 0
	for i := 0; i < 50; i++ {
		if cache.Has("key-" + strconv.Itoa(i)) {
			count++
		}
	}

	if count != 50 {
		t.Errorf("Expected 50 keys to exist, found %d", count)
	}
}

// TestRaceConditions_ConcurrentEviction tests for data races during eviction
func TestRaceConditions_ConcurrentEviction(t *testing.T) {
	// Small cache to force eviction
	cache := NewCache(Config{MaxSize: 10})
	const numGoroutines = 20
	const numKeys = 100 // Much larger than cache size

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// All goroutines add keys to force eviction
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < numKeys; j++ {
				key := strconv.Itoa(goroutineID*numKeys + j)
				value := goroutineID*numKeys + j
				cache.Set(key, value)
			}
		}(i)
	}

	wg.Wait()

	// Verify cache doesn't exceed capacity and isn't corrupted
	stats := cache.Stats()
	if stats.Size < 0 {
		t.Errorf("Cache size is negative: %d", stats.Size)
	}
	if stats.Size > 10 {
		t.Errorf("Cache size exceeds capacity: %d > 10", stats.Size)
	}
	if stats.Evictions == 0 {
		t.Error("Expected some evictions to occur")
	}
}

// TestRaceConditions_ConcurrentStats tests for data races when accessing stats
func TestRaceConditions_ConcurrentStats(t *testing.T) {
	cache := NewCache(Config{MaxSize: 100})
	const numGoroutines = 50
	const numOperations = 1000

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 2) // Half for operations, half for stats

	// Operation goroutines
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < numOperations; j++ {
				key := strconv.Itoa(j % 50)
				value := goroutineID*numOperations + j

				switch j % 4 {
				case 0:
					cache.Set(key, value)
				case 1:
					cache.Get(key)
				case 2:
					cache.Has(key)
				case 3:
					cache.Delete(key)
				}
			}
		}(i)
	}

	// Stats readers
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()

			for j := 0; j < numOperations; j++ {
				stats := cache.Stats()

				// Basic sanity checks
				if stats.Size < 0 {
					t.Errorf("Negative cache size in stats: %d", stats.Size)
				}
				if stats.Capacity != 100 {
					t.Errorf("Wrong capacity in stats: %d", stats.Capacity)
				}
			}
		}()
	}

	wg.Wait()
}

// TestRaceConditions_ConcurrentClear tests for data races during Clear operations
func TestRaceConditions_ConcurrentClear(t *testing.T) {
	cache := NewCache(Config{MaxSize: 100})
	const numGoroutines = 20
	const numOperations = 50

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 2)

	// Populate cache initially
	for i := 0; i < 50; i++ {
		cache.Set("init-key-"+strconv.Itoa(i), i)
	}

	// Operation goroutines
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < numOperations; j++ {
				key := "key-" + strconv.Itoa(goroutineID*numOperations+j)
				value := goroutineID*numOperations + j

				switch j % 3 {
				case 0:
					cache.Set(key, value)
				case 1:
					cache.Get(key)
				case 2:
					cache.Has(key)
				}
			}
		}(i)
	}

	// Clear goroutines
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()

			for j := 0; j < 5; j++ { // Less frequent clears
				time.Sleep(time.Microsecond * 100)
				cache.Clear()
			}
		}()
	}

	wg.Wait()

	// Final verification
	stats := cache.Stats()
	if stats.Size < 0 {
		t.Errorf("Cache size corrupted after concurrent clear: %d", stats.Size)
	}
}

// TestRaceConditions_SketchConcurrency tests for data races in frequency sketch
func TestRaceConditions_SketchConcurrency(t *testing.T) {
	sketch := newFrequencySketch(1000)
	const numGoroutines = 50
	const numOperations = 10000

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Concurrent increment and estimate operations
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < numOperations; j++ {
				keyHash := uint64(goroutineID*numOperations + j)

				if j%2 == 0 {
					sketch.increment(keyHash)
				} else {
					sketch.estimate(keyHash)
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify sketch is still functional
	testHash := uint64(12345)
	sketch.increment(testHash)
	estimate := sketch.estimate(testHash)

	if estimate == 0 {
		t.Error("Sketch should have recorded the increment")
	}
}

// TestRaceConditions_StringHashConcurrency tests string hash function for race conditions
func TestRaceConditions_StringHashConcurrency(t *testing.T) {
	const numGoroutines = 100
	const numOperations = 10000

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Concurrent hash operations with same and different strings
	testStrings := []string{
		"test1", "test2", "test3", "test4", "test5",
		"concurrent", "hash", "function", "testing", "race",
	}

	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < numOperations; j++ {
				str := testStrings[j%len(testStrings)]
				hash1 := stringHash(str)
				hash2 := stringHash(str)

				// Same string should always produce same hash
				if hash1 != hash2 {
					t.Errorf("Hash inconsistency for string '%s': %d != %d", str, hash1, hash2)
				}
			}
		}(i)
	}

	wg.Wait()
}

// TestRaceConditions_EntryStateConcurrency tests entry state transitions for race conditions
func TestRaceConditions_EntryStateConcurrency(t *testing.T) {
	cache := NewCache(Config{MaxSize: 50})
	const numGoroutines = 30
	const numOperations = 500

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Force entry state transitions by using limited key space
	keys := make([]string, 20) // Smaller than cache size to force updates
	for i := range keys {
		keys[i] = "state-key-" + strconv.Itoa(i)
	}

	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < numOperations; j++ {
				key := keys[j%len(keys)]
				value := goroutineID*numOperations + j

				switch j % 4 {
				case 0:
					cache.Set(key, value) // entryEmpty -> entryPending -> entryValid
				case 1:
					cache.Get(key) // Read entryValid
				case 2:
					cache.Set(key, value+1) // entryValid -> entryPending -> entryValid (update)
				case 3:
					cache.Delete(key) // entryValid -> entryDeleted
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify final state is consistent
	stats := cache.Stats()
	if stats.Size < 0 || stats.Size > 50 {
		t.Errorf("Entry state corruption detected, cache size: %d", stats.Size)
	}
}

// TestRaceConditions_MemoryBarriers tests memory barrier correctness
func TestRaceConditions_MemoryBarriers(t *testing.T) {
	cache := NewCache(Config{MaxSize: 1000})
	const numGoroutines = 10
	const numOperations = 100

	var wg sync.WaitGroup
	var inconsistencies int64

	wg.Add(numGoroutines)

	// Each goroutine uses unique keys to avoid legitimate overwrites
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < numOperations; j++ {
				// Use unique keys per goroutine to test real memory barriers
				// not legitimate concurrent overwrites
				key := fmt.Sprintf("barrier-test-%d-%d", goroutineID, j)
				expectedValue := goroutineID*numOperations + j

				// Set a value
				cache.Set(key, expectedValue)

				// Immediately try to read it - should ALWAYS find the same value
				// since we're the only goroutine writing to this key
				if value, found := cache.Get(key); found {
					if value != expectedValue {
						// This indicates a real memory barrier issue
						atomic.AddInt64(&inconsistencies, 1)
					}
				} else {
					// Key should always be found after Set
					atomic.AddInt64(&inconsistencies, 1)
				}
			}
		}(i)
	}

	wg.Wait()

	// With unique keys, there should be ZERO inconsistencies
	// Any inconsistency indicates a real memory barrier problem
	if inconsistencies > 0 {
		t.Errorf("Memory barrier issues detected: %d inconsistencies with unique keys", inconsistencies)
	}
}

// TestRaceConditions_GoroutineStress applies maximum stress to detect any race conditions
func TestRaceConditions_GoroutineStress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	cache := NewCache(Config{MaxSize: 1000})

	// Use all available CPU cores
	numGoroutines := runtime.GOMAXPROCS(0) * 4
	const numOperations = 50000
	const testDuration = 5 * time.Second

	var wg sync.WaitGroup
	var stopFlag int64

	// Stop after duration
	go func() {
		time.Sleep(testDuration)
		atomic.StoreInt64(&stopFlag, 1)
	}()

	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()
			operationCount := 0

			for atomic.LoadInt64(&stopFlag) == 0 && operationCount < numOperations {
				key := strconv.Itoa(operationCount % 100)
				value := goroutineID*numOperations + operationCount

				switch operationCount % 8 {
				case 0:
					cache.Set(key, value)
				case 1:
					cache.Get(key)
				case 2:
					cache.Has(key)
				case 3:
					cache.Delete(key)
				case 4:
					cache.Stats()
				case 5:
					cache.Set(key+"-alt", value)
				case 6:
					cache.Get(key + "-alt")
				case 7:
					if operationCount%1000 == 0 {
						cache.Clear()
					}
				}

				operationCount++
			}
		}(i)
	}

	wg.Wait()

	// Final verification
	stats := cache.Stats()
	if stats.Size < 0 || stats.Size > 1000 {
		t.Errorf("Cache corrupted under stress: size=%d, capacity=%d", stats.Size, stats.Capacity)
	}

	t.Logf("Stress test completed: final cache size=%d, stats=%+v", stats.Size, stats)
}

// BenchmarkRaceConditions_ConcurrentOps benchmarks concurrent operations to detect performance issues
func BenchmarkRaceConditions_ConcurrentOps(b *testing.B) {
	cache := NewCache(Config{MaxSize: 10000})

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := strconv.Itoa(i % 1000)
			value := i

			switch i % 4 {
			case 0:
				cache.Set(key, value)
			case 1:
				cache.Get(key)
			case 2:
				cache.Has(key)
			case 3:
				cache.Delete(key)
			}
			i++
		}
	})
}
