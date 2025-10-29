// expiration_test.go: comprehensive tests for cache expiration and cleanup
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGILira fragment
// SPDX-License-Identifier: MPL-2.0

package balios

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestInlineExpiration_OpportunisticCleanup tests that Set() cleans up expired entries
// encountered during linear probing without impacting performance
func TestInlineExpiration_OpportunisticCleanup(t *testing.T) {
	mockTime := &MockTimeProvider{currentTime: 1000000000}

	cache := NewCache(Config{
		MaxSize:      10,
		TTL:          100 * time.Millisecond,
		TimeProvider: mockTime,
	})
	defer func() { _ = cache.Close() }()

	// Fill cache with entries (using consistent type: string)
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("key%d", i)
		cache.Set(key, fmt.Sprintf("value%d", i)) // Use string instead of int
	}

	if cache.Len() != 10 {
		t.Fatalf("Expected 10 entries, got %d", cache.Len())
	}

	// Advance time to expire all entries
	mockTime.Advance(150 * time.Millisecond)

	// Insert new entry - should trigger cleanup of expired entries during linear probing
	cache.Set("new_key", "new_value")

	// Verify new entry is accessible
	value, found := cache.Get("new_key")
	if !found {
		t.Error("New entry should be accessible")
	}
	if value != "new_value" {
		t.Errorf("Expected 'new_value', got %v", value)
	}

	// Old expired entries should not be accessible
	// Accessing them will trigger lazy cleanup
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("key%d", i)
		if _, found := cache.Get(key); found {
			t.Errorf("Expired key %s should not be accessible", key)
		}
	}

	// After all Gets, verify metrics are consistent
	// The exact number depends on linear probing path during Set and subsequent Gets
	stats := cache.Stats()

	// At minimum, the new entry should be present
	if cache.Len() < 1 {
		t.Errorf("Expected at least 1 entry (new_key), got %d", cache.Len())
	}

	// Verify metrics are consistent after cleanup
	if stats.Size != cache.Len() {
		t.Errorf("Stats size (%d) doesn't match Len() (%d)", stats.Size, cache.Len())
	}

	// Verify expirations were tracked
	if stats.Expirations == 0 {
		t.Error("Expected expirations > 0 after accessing expired entries")
	}
}

// TestInlineExpiration_NoTTL verifies zero overhead when TTL is disabled
func TestInlineExpiration_NoTTL(t *testing.T) {
	cache := NewCache(Config{
		MaxSize: 100,
		TTL:     0, // No expiration
	})
	defer func() { _ = cache.Close() }()

	// Fill cache
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("key%d", i)
		cache.Set(key, i)
	}

	// All entries should remain
	if cache.Len() != 100 {
		t.Errorf("Expected 100 entries with no TTL, got %d", cache.Len())
	}

	// Add more entries
	for i := 100; i < 200; i++ {
		key := fmt.Sprintf("key%d", i)
		cache.Set(key, i)
	}

	// Should have exactly MaxSize entries (eviction, not expiration)
	if cache.Len() != 100 {
		t.Errorf("Expected 100 entries after eviction, got %d", cache.Len())
	}

	stats := cache.Stats()
	if stats.Evictions == 0 {
		t.Error("Expected evictions when cache is full with no TTL")
	}
}

// TestInlineExpiration_PartialExpiration tests mixed expired/valid entries
func TestInlineExpiration_PartialExpiration(t *testing.T) {
	mockTime := &MockTimeProvider{currentTime: 1000000000}

	cache := NewCache(Config{
		MaxSize:      20,
		TTL:          100 * time.Millisecond,
		TimeProvider: mockTime,
	})
	defer func() { _ = cache.Close() }()

	// Add first batch
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("old_key%d", i)
		cache.Set(key, i)
	}

	// Advance time to expire first batch
	mockTime.Advance(150 * time.Millisecond)

	// Add second batch (not expired)
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("new_key%d", i)
		cache.Set(key, i)
	}

	// Verify old keys are not accessible
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("old_key%d", i)
		if _, found := cache.Get(key); found {
			t.Errorf("Expired key %s should not be accessible", key)
		}
	}

	// Verify new keys are accessible
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("new_key%d", i)
		value, found := cache.Get(key)
		if !found {
			t.Errorf("New key %s should be accessible", key)
		}
		if value != i {
			t.Errorf("Expected value %d for new_key%d, got %v", i, i, value)
		}
	}
}

// TestInlineExpiration_ConcurrentAccess tests thread safety of inline cleanup
func TestInlineExpiration_ConcurrentAccess(t *testing.T) {
	mockTime := &MockTimeProvider{currentTime: 1000000000}

	cache := NewCache(Config{
		MaxSize:      100,
		TTL:          100 * time.Millisecond,
		TimeProvider: mockTime,
	})
	defer func() { _ = cache.Close() }()

	// Pre-populate with entries that will expire
	for i := 0; i < 50; i++ {
		key := fmt.Sprintf("initial_key%d", i)
		cache.Set(key, i)
	}

	// Expire entries
	mockTime.Advance(150 * time.Millisecond)

	// Concurrent Set operations that will encounter expired entries
	var wg sync.WaitGroup
	const numGoroutines = 20
	const opsPerGoroutine = 50

	wg.Add(numGoroutines)
	for g := 0; g < numGoroutines; g++ {
		go func(goroutineID int) {
			defer wg.Done()
			for i := 0; i < opsPerGoroutine; i++ {
				key := fmt.Sprintf("goroutine%d_key%d", goroutineID, i)
				cache.Set(key, i)
			}
		}(g)
	}

	wg.Wait()

	// Verify cache is consistent
	stats := cache.Stats()
	if stats.Size < 0 {
		t.Errorf("Negative cache size: %d", stats.Size)
	}
	if stats.Size > cache.Capacity() {
		t.Errorf("Cache size (%d) exceeds capacity (%d)", stats.Size, cache.Capacity())
	}

	// Verify old expired entries are not accessible
	for i := 0; i < 50; i++ {
		key := fmt.Sprintf("initial_key%d", i)
		if _, found := cache.Get(key); found {
			t.Errorf("Expired key %s should not be accessible after concurrent operations", key)
		}
	}
}

// TestExpireNow_Basic tests the manual expiration API
func TestExpireNow_Basic(t *testing.T) {
	mockTime := &MockTimeProvider{currentTime: 1000000000}

	cache := NewCache(Config{
		MaxSize:      100,
		TTL:          100 * time.Millisecond,
		TimeProvider: mockTime,
	})
	defer func() { _ = cache.Close() }()

	// Add entries
	for i := 0; i < 50; i++ {
		key := fmt.Sprintf("key%d", i)
		cache.Set(key, i)
	}

	if cache.Len() != 50 {
		t.Fatalf("Expected 50 entries, got %d", cache.Len())
	}

	// Advance time to expire all entries
	mockTime.Advance(150 * time.Millisecond)

	// Manually trigger expiration
	expired := cache.ExpireNow()

	// Should have removed all expired entries
	if expired != 50 {
		t.Errorf("Expected 50 expired entries, got %d", expired)
	}

	if cache.Len() != 0 {
		t.Errorf("Expected cache to be empty after ExpireNow(), got %d entries", cache.Len())
	}

	// Verify entries are truly gone
	for i := 0; i < 50; i++ {
		key := fmt.Sprintf("key%d", i)
		if _, found := cache.Get(key); found {
			t.Errorf("Key %s should be expired and removed", key)
		}
	}
}

// TestExpireNow_PartialExpiration tests ExpireNow with mixed expired/valid entries
func TestExpireNow_PartialExpiration(t *testing.T) {
	mockTime := &MockTimeProvider{currentTime: 1000000000}

	cache := NewCache(Config{
		MaxSize:      100,
		TTL:          100 * time.Millisecond,
		TimeProvider: mockTime,
	})
	defer func() { _ = cache.Close() }()

	// Add first batch
	for i := 0; i < 30; i++ {
		key := fmt.Sprintf("old_key%d", i)
		cache.Set(key, i)
	}

	// Advance time to expire first batch
	mockTime.Advance(150 * time.Millisecond)

	// Add second batch (not expired)
	for i := 0; i < 30; i++ {
		key := fmt.Sprintf("new_key%d", i)
		cache.Set(key, i)
	}

	if cache.Len() != 60 {
		t.Fatalf("Expected 60 entries before ExpireNow(), got %d", cache.Len())
	}

	// Manually expire
	expired := cache.ExpireNow()

	// Should have removed only expired entries
	if expired != 30 {
		t.Errorf("Expected 30 expired entries, got %d", expired)
	}

	if cache.Len() != 30 {
		t.Errorf("Expected 30 entries after ExpireNow(), got %d", cache.Len())
	}

	// Verify old keys are gone
	for i := 0; i < 30; i++ {
		key := fmt.Sprintf("old_key%d", i)
		if _, found := cache.Get(key); found {
			t.Errorf("Expired key %s should be removed", key)
		}
	}

	// Verify new keys remain
	for i := 0; i < 30; i++ {
		key := fmt.Sprintf("new_key%d", i)
		value, found := cache.Get(key)
		if !found {
			t.Errorf("Valid key %s should remain after ExpireNow()", key)
		}
		if value != i {
			t.Errorf("Expected value %d for new_key%d, got %v", i, i, value)
		}
	}
}

// TestExpireNow_NoTTL verifies ExpireNow returns 0 when TTL is disabled
func TestExpireNow_NoTTL(t *testing.T) {
	cache := NewCache(Config{
		MaxSize: 100,
		TTL:     0, // No expiration
	})
	defer func() { _ = cache.Close() }()

	// Add entries
	for i := 0; i < 50; i++ {
		key := fmt.Sprintf("key%d", i)
		cache.Set(key, i)
	}

	// ExpireNow should return 0 (nothing to expire)
	expired := cache.ExpireNow()
	if expired != 0 {
		t.Errorf("Expected 0 expirations with no TTL, got %d", expired)
	}

	// All entries should remain
	if cache.Len() != 50 {
		t.Errorf("Expected 50 entries with no TTL, got %d", cache.Len())
	}
}

// TestExpireNow_EmptyCache tests ExpireNow on empty cache
func TestExpireNow_EmptyCache(t *testing.T) {
	cache := NewCache(Config{
		MaxSize: 100,
		TTL:     time.Hour,
	})
	defer func() { _ = cache.Close() }()

	// ExpireNow on empty cache should return 0
	expired := cache.ExpireNow()
	if expired != 0 {
		t.Errorf("Expected 0 expirations on empty cache, got %d", expired)
	}

	if cache.Len() != 0 {
		t.Errorf("Expected cache to remain empty, got %d entries", cache.Len())
	}
}

// TestExpireNow_ConcurrentSafety tests thread safety of ExpireNow
func TestExpireNow_ConcurrentSafety(t *testing.T) {
	mockTime := &MockTimeProvider{currentTime: 1000000000}

	cache := NewCache(Config{
		MaxSize:      200,
		TTL:          100 * time.Millisecond,
		TimeProvider: mockTime,
	})
	defer func() { _ = cache.Close() }()

	// Add entries
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("key%d", i)
		cache.Set(key, i)
	}

	// Expire half the entries
	mockTime.Advance(150 * time.Millisecond)

	// Add more entries (not expired)
	for i := 100; i < 200; i++ {
		key := fmt.Sprintf("key%d", i)
		cache.Set(key, i)
	}

	// Concurrent ExpireNow + Set/Get operations
	var wg sync.WaitGroup
	var totalExpired int64

	// Multiple ExpireNow calls
	wg.Add(5)
	for i := 0; i < 5; i++ {
		go func() {
			defer wg.Done()
			expired := cache.ExpireNow()
			atomic.AddInt64(&totalExpired, int64(expired))
		}()
	}

	// Concurrent Set operations
	wg.Add(5)
	for i := 0; i < 5; i++ {
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				key := fmt.Sprintf("concurrent_key%d_%d", goroutineID, j)
				cache.Set(key, j)
			}
		}(i)
	}

	// Concurrent Get operations
	wg.Add(5)
	for i := 0; i < 5; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				key := fmt.Sprintf("key%d", j)
				cache.Get(key)
			}
		}()
	}

	wg.Wait()

	// Verify cache consistency
	stats := cache.Stats()
	if stats.Size < 0 {
		t.Errorf("Negative cache size after concurrent ExpireNow: %d", stats.Size)
	}
	if stats.Size > cache.Capacity() {
		t.Errorf("Cache size (%d) exceeds capacity (%d) after concurrent ExpireNow", stats.Size, cache.Capacity())
	}

	// All expired entries should be gone
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("key%d", i)
		if _, found := cache.Get(key); found {
			t.Errorf("Expired key %s should be removed by ExpireNow", key)
		}
	}
}

// TestExpireNow_MetricsTracking tests that expiration metrics are properly recorded
func TestExpireNow_MetricsTracking(t *testing.T) {
	mockTime := &MockTimeProvider{currentTime: 1000000000}

	cache := NewCache(Config{
		MaxSize:      100,
		TTL:          100 * time.Millisecond,
		TimeProvider: mockTime,
	})
	defer func() { _ = cache.Close() }()

	// Add entries
	for i := 0; i < 50; i++ {
		key := fmt.Sprintf("key%d", i)
		cache.Set(key, i)
	}

	// Expire all entries
	mockTime.Advance(150 * time.Millisecond)

	// Get baseline stats
	statsBefore := cache.Stats()

	// ExpireNow
	expired := cache.ExpireNow()
	if expired != 50 {
		t.Fatalf("Expected 50 expirations, got %d", expired)
	}

	// Verify stats are updated
	statsAfter := cache.Stats()

	// Expirations counter should increase
	if statsAfter.Expirations != statsBefore.Expirations+50 {
		t.Errorf("Expected expirations to increase by 50, got before=%d after=%d",
			statsBefore.Expirations, statsAfter.Expirations)
	}

	// Size should decrease
	if statsAfter.Size != 0 {
		t.Errorf("Expected size=0 after expiring all entries, got %d", statsAfter.Size)
	}
}

// BenchmarkInlineExpiration_NoOverhead verifies zero overhead when TTL=0
func BenchmarkInlineExpiration_NoOverhead(b *testing.B) {
	cache := NewCache(Config{
		MaxSize: 10000,
		TTL:     0, // No TTL - should have zero overhead
	})
	defer func() { _ = cache.Close() }()

	// Pre-populate
	for i := 0; i < 5000; i++ {
		key := fmt.Sprintf("key%d", i)
		cache.Set(key, i)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("bench_key%d", i%1000)
		cache.Set(key, i)
	}
}

// BenchmarkInlineExpiration_WithTTL measures overhead of inline cleanup
func BenchmarkInlineExpiration_WithTTL(b *testing.B) {
	mockTime := &MockTimeProvider{currentTime: 1000000000}

	cache := NewCache(Config{
		MaxSize:      10000,
		TTL:          time.Hour,
		TimeProvider: mockTime,
	})
	defer func() { _ = cache.Close() }()

	// Pre-populate with non-expired entries
	for i := 0; i < 5000; i++ {
		key := fmt.Sprintf("key%d", i)
		cache.Set(key, i)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("bench_key%d", i%1000)
		cache.Set(key, i)
	}
}

// BenchmarkExpireNow measures performance of manual expiration
func BenchmarkExpireNow(b *testing.B) {
	mockTime := &MockTimeProvider{currentTime: 1000000000}

	cache := NewCache(Config{
		MaxSize:      10000,
		TTL:          100 * time.Millisecond,
		TimeProvider: mockTime,
	})
	defer func() { _ = cache.Close() }()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		// Populate cache
		for j := 0; j < 1000; j++ {
			key := fmt.Sprintf("key%d_%d", i, j)
			cache.Set(key, j)
		}
		// Expire entries
		mockTime.Advance(150 * time.Millisecond)
		b.StartTimer()

		// Measure ExpireNow
		cache.ExpireNow()
	}
}

// BenchmarkExpireNow_NoExpiredEntries measures overhead when nothing to expire
func BenchmarkExpireNow_NoExpiredEntries(b *testing.B) {
	cache := NewCache(Config{
		MaxSize: 10000,
		TTL:     time.Hour,
	})
	defer func() { _ = cache.Close() }()

	// Populate with valid entries
	for i := 0; i < 5000; i++ {
		key := fmt.Sprintf("key%d", i)
		cache.Set(key, i)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		cache.ExpireNow()
	}
}
