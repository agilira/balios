// negative_cache_leak_test.go: tests for negative cache memory leak issue
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGILira library
// SPDX-License-Identifier: MPL-2.0

package balios

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

// TestNegativeCacheMemoryLeak tests the CRITICAL issue #2:
// negativeCache sync.Map is never cleaned up. Expired errors remain in memory forever.
//
// PROBLEM: When errors are cached with NegativeCacheTTL, they're only deleted
// when explicitly accessed AFTER expiration. If never accessed again, they leak.
//
// SCENARIO: Database down → 10,000 unique keys fail → all cached in negativeCache
// → Database recovered → Those 10,000 entries remain in memory FOREVER
//
// Expected behavior: Background cleanup removes expired entries
func TestNegativeCacheMemoryLeak(t *testing.T) {
	cache := NewCache(Config{
		MaxSize:          1000,
		NegativeCacheTTL: 100 * time.Millisecond, // Short TTL for testing
	})
	defer cache.Clear()

	testErr := errors.New("database connection failed")

	// Simulate many failed loads (e.g., database down)
	const numFailedKeys = 1000
	for i := 0; i < numFailedKeys; i++ {
		key := fmt.Sprintf("failed-key-%d", i)
		_, err := cache.GetOrLoadWithContext(context.Background(), key, func(ctx context.Context) (interface{}, error) {
			return nil, testErr
		})
		if err != testErr {
			t.Errorf("Expected error from loader, got: %v", err)
		}
	}

	// Verify entries are cached
	negativeCount := 0
	cache.(*wtinyLFUCache).negativeCache.Range(func(key, value interface{}) bool {
		negativeCount++
		return true
	})
	t.Logf("Negative cache entries after failures: %d", negativeCount)
	if negativeCount != numFailedKeys {
		t.Errorf("Expected %d negative entries, got %d", numFailedKeys, negativeCount)
	}

	// Wait for TTL to expire
	time.Sleep(150 * time.Millisecond)

	// WITHOUT FIX: Expired entries remain because nobody accesses them
	// WITH FIX: Background cleanup removes them

	// Check if expired entries are still there
	expiredCount := 0
	now := time.Now().UnixNano()
	cache.(*wtinyLFUCache).negativeCache.Range(func(key, value interface{}) bool {
		neg := value.(negativeEntry)
		if now > neg.expireAt {
			expiredCount++
		}
		return true
	})

	t.Logf("Expired entries still in memory: %d", expiredCount)

	// CRITICAL ASSERTION: Expired entries should be cleaned up
	if expiredCount > numFailedKeys/10 { // Allow 10% tolerance
		t.Errorf("MEMORY LEAK: %d expired entries not cleaned up (should be near 0)", expiredCount)
		t.Errorf("This is a memory leak - expired errors remain in sync.Map forever!")
	}
}

// TestNegativeCacheCleanupUnderLoad tests cleanup under continuous load
func TestNegativeCacheCleanupUnderLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	cache := NewCache(Config{
		MaxSize:          1000,
		NegativeCacheTTL: 200 * time.Millisecond,
	})
	defer cache.Clear()

	// Continuously generate errors for different keys
	stopChan := make(chan struct{})
	var errorCount int64 // Use atomic for race-free access

	// Producer: generate errors
	go func() {
		i := 0
		for {
			select {
			case <-stopChan:
				return
			default:
				key := fmt.Sprintf("error-key-%d", i)
				_, _ = cache.GetOrLoadWithContext(context.Background(), key, func(ctx context.Context) (interface{}, error) {
					return nil, errors.New("transient error")
				})
				atomic.AddInt64(&errorCount, 1)
				i++
				time.Sleep(1 * time.Millisecond)
			}
		}
	}()

	// Run for 2 seconds
	time.Sleep(2 * time.Second)
	close(stopChan)
	time.Sleep(100 * time.Millisecond) // Let cleanup run

	t.Logf("Generated %d errors", atomic.LoadInt64(&errorCount))

	// Count entries in negative cache
	currentCount := 0
	cache.(*wtinyLFUCache).negativeCache.Range(func(key, value interface{}) bool {
		currentCount++
		return true
	})

	t.Logf("Negative cache entries: %d", currentCount)

	// With cleanup, entries should be bounded (roughly TTL * rate)
	// Expected: ~200ms TTL * 1000 errors/sec = ~200 entries max
	maxExpected := 500 // Be generous

	if currentCount > maxExpected {
		t.Errorf("Negative cache growing unbounded: %d entries (expected < %d)",
			currentCount, maxExpected)
		t.Errorf("This indicates cleanup is not working properly")
	}
}

// TestNegativeCacheCleanupStopsOnCacheClear tests that cleanup goroutine
// stops properly when cache is cleared
func TestNegativeCacheCleanupStopsOnCacheClear(t *testing.T) {
	runtime.GC()
	time.Sleep(20 * time.Millisecond)
	baseline := runtime.NumGoroutine()

	cache := NewCache(Config{
		MaxSize:          100,
		NegativeCacheTTL: 100 * time.Millisecond,
	})

	// Give cleanup goroutine time to start (if implemented)
	time.Sleep(50 * time.Millisecond)

	// Clear cache (should stop cleanup goroutine)
	cache.Clear()

	// Give time for cleanup to stop
	time.Sleep(100 * time.Millisecond)
	runtime.GC()
	time.Sleep(20 * time.Millisecond)

	final := runtime.NumGoroutine()

	// Should return to baseline (no leaked cleanup goroutine)
	if final > baseline+2 {
		t.Errorf("Cleanup goroutine leak: baseline=%d, final=%d", baseline, final)
	}
}

// TestNegativeCacheExplicitDelete verifies existing behavior:
// expired entries ARE deleted when accessed
func TestNegativeCacheExplicitDelete(t *testing.T) {
	cache := NewCache(Config{
		MaxSize:          100,
		NegativeCacheTTL: 50 * time.Millisecond,
	})
	defer cache.Clear()

	// Generate error
	testErr := errors.New("test error")
	_, err := cache.GetOrLoadWithContext(context.Background(), "test-key", func(ctx context.Context) (interface{}, error) {
		return nil, testErr
	})
	if err != testErr {
		t.Fatalf("Expected error, got: %v", err)
	}

	// Verify it's cached
	_, err = cache.GetOrLoadWithContext(context.Background(), "test-key", func(ctx context.Context) (interface{}, error) {
		t.Error("Loader should not be called (error is cached)")
		return nil, errors.New("should not be called")
	})
	if err != testErr {
		t.Error("Expected cached error")
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Access again - should delete expired entry and call loader
	loaderCalled := false
	_, err = cache.GetOrLoadWithContext(context.Background(), "test-key", func(ctx context.Context) (interface{}, error) {
		loaderCalled = true
		return "success", nil
	})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if !loaderCalled {
		t.Error("Loader should be called after negative cache expiration")
	}

	// Verify entry was deleted
	found := false
	cache.(*wtinyLFUCache).negativeCache.Range(func(key, value interface{}) bool {
		if key == "neg:test-key" {
			found = true
		}
		return true
	})

	if found {
		t.Error("Expired negative entry should be deleted on access")
	}
}

// BenchmarkNegativeCacheWithCleanup benchmarks performance impact of cleanup
func BenchmarkNegativeCacheWithCleanup(b *testing.B) {
	cache := NewCache(Config{
		MaxSize:          10000,
		NegativeCacheTTL: 1 * time.Second,
	})
	defer cache.Clear()

	testErr := errors.New("benchmark error")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("bench-key-%d", i%1000)
		_, _ = cache.GetOrLoadWithContext(context.Background(), key, func(ctx context.Context) (interface{}, error) {
			return nil, testErr
		})
	}
}
