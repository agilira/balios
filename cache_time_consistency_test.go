// cache_time_consistency_test.go: tests for time consistency in cache operations
//
// These tests verify that cache operations use consistent timestamps throughout
// a single operation, preventing edge cases where time changes mid-operation.
//
// Copyright (c) 2025 AGILira - A. Giordano
// SPDX-License-Identifier: MPL-2.0

package balios

import (
	"sync/atomic"
	"testing"
	"time"
)

// mockTimeProvider simulates time progression to detect multiple Now() calls
type mockTimeProvider struct {
	currentTime int64
	callCount   int64
}

func (m *mockTimeProvider) Now() int64 {
	atomic.AddInt64(&m.callCount, 1)
	return atomic.AddInt64(&m.currentTime, 1) // Increment time on each call
}

func (m *mockTimeProvider) ResetCallCount() {
	atomic.StoreInt64(&m.callCount, 0)
}

func (m *mockTimeProvider) GetCallCount() int64 {
	return atomic.LoadInt64(&m.callCount)
}

// TestSingleNowCallInGet verifies Get() calls Now() at most twice per operation
// (once for TTL check, once for metrics latency)
func TestSingleNowCallInGet(t *testing.T) {
	mock := &mockTimeProvider{currentTime: 1000}

	cache := NewCache(Config{
		MaxSize:          100,
		TTL:              time.Hour,
		TimeProvider:     mock,
		MetricsCollector: NoOpMetricsCollector{},
	})
	defer func() {
		if err := cache.Close(); err != nil {
			t.Errorf("Failed to close cache: %v", err)
		}
	}()

	// Set a value
	cache.Set("key1", "value1")
	mock.ResetCallCount()

	// Get should call Now() at most twice (TTL check + metrics end time)
	_, found := cache.Get("key1")
	if !found {
		t.Fatal("Key not found")
	}

	callCount := mock.GetCallCount()
	if callCount > 2 {
		t.Errorf("Get() called Now() %d times, expected at most 2 calls (1 for TTL, 1 for metrics)", callCount)
	} else {
		t.Logf("✓ Get() called Now() %d time(s)", callCount)
	}
}

// TestSingleNowCallInSet verifies Set() calls Now() at most twice per operation
// (once for TTL calculation, once for metrics latency)
func TestSingleNowCallInSet(t *testing.T) {
	mock := &mockTimeProvider{currentTime: 1000}

	cache := NewCache(Config{
		MaxSize:          100,
		TTL:              time.Hour,
		TimeProvider:     mock,
		MetricsCollector: NoOpMetricsCollector{},
	})
	defer func() {
		if err := cache.Close(); err != nil {
			t.Errorf("Failed to close cache: %v", err)
		}
	}()

	mock.ResetCallCount()

	// Set should call Now() at most twice (TTL + metrics)
	cache.Set("key1", "value1")

	callCount := mock.GetCallCount()
	if callCount > 2 {
		t.Errorf("Set() called Now() %d times, expected at most 2 calls (1 for TTL, 1 for metrics)", callCount)
	} else {
		t.Logf("✓ Set() called Now() %d time(s)", callCount)
	}
}

// TestSingleNowCallInHas verifies Has() calls Now() at most once per operation
func TestSingleNowCallInHas(t *testing.T) {
	mock := &mockTimeProvider{currentTime: 1000}

	cache := NewCache(Config{
		MaxSize:          100,
		TTL:              time.Hour,
		TimeProvider:     mock,
		MetricsCollector: NoOpMetricsCollector{},
	})
	defer func() {
		if err := cache.Close(); err != nil {
			t.Errorf("Failed to close cache: %v", err)
		}
	}()

	// Set a value
	cache.Set("key1", "value1")
	mock.ResetCallCount()

	// Has should call Now() at most once (for TTL check)
	exists := cache.Has("key1")
	if !exists {
		t.Fatal("Key should exist")
	}

	callCount := mock.GetCallCount()
	if callCount > 1 {
		t.Errorf("Has() called Now() %d times, expected at most 1 call", callCount)
	} else {
		t.Logf("✓ Has() called Now() %d time(s)", callCount)
	}
}

// TestTimeConsistencyInGet verifies that Get() uses consistent time for all checks
func TestTimeConsistencyInGet(t *testing.T) {
	mock := &mockTimeProvider{currentTime: 1000}

	cache := NewCache(Config{
		MaxSize:      100,
		TTL:          100, // Short TTL: 100 nanoseconds
		TimeProvider: mock,
	})
	defer func() {
		if err := cache.Close(); err != nil {
			t.Errorf("Failed to close cache: %v", err)
		}
	}()

	// Set a value
	cache.Set("key1", "value1")

	// Advance time past TTL
	atomic.StoreInt64(&mock.currentTime, 2000)
	mock.ResetCallCount()

	// Get should detect expiration consistently
	_, found := cache.Get("key1")

	if found {
		t.Error("Key should be expired")
	}

	callCount := mock.GetCallCount()
	t.Logf("Get() (expired key) called Now() %d time(s)", callCount)
}

// TestNoTimeCallsWithoutTTL verifies minimal Now() calls when TTL is disabled
func TestNoTimeCallsWithoutTTL(t *testing.T) {
	mock := &mockTimeProvider{currentTime: 1000}

	cache := NewCache(Config{
		MaxSize:          100,
		TTL:              0, // No TTL
		TimeProvider:     mock,
		MetricsCollector: NoOpMetricsCollector{},
	})
	defer func() {
		if err := cache.Close(); err != nil {
			t.Errorf("Failed to close cache: %v", err)
		}
	}()

	// Set a value
	mock.ResetCallCount()
	cache.Set("key1", "value1")
	setCount := mock.GetCallCount()

	// Get a value
	mock.ResetCallCount()
	cache.Get("key1")
	getCount := mock.GetCallCount()

	t.Logf("Without TTL: Set() called Now() %d times, Get() called Now() %d times",
		setCount, getCount)

	// Without TTL, we expect minimal time calls (maybe 0 for operations, but metrics might call)
	// This test is mainly for documentation
}

// BenchmarkNowCallOverhead measures the overhead of Now() calls
func BenchmarkNowCallOverhead(b *testing.B) {
	b.Run("SystemTime", func(b *testing.B) {
		tp := &systemTimeProvider{}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = tp.Now()
		}
	})

	b.Run("MockTime", func(b *testing.B) {
		tp := &mockTimeProvider{currentTime: 1000}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = tp.Now()
		}
	})
}

// BenchmarkCacheWithTimeConsistency benchmarks cache operations with time provider
func BenchmarkCacheWithTimeConsistency(b *testing.B) {
	b.Run("GetWithTTL", func(b *testing.B) {
		cache := NewCache(Config{
			MaxSize: 10000,
			TTL:     time.Hour,
		})
		defer func() {
			if err := cache.Close(); err != nil {
				b.Errorf("Failed to close cache: %v", err)
			}
		}()

		// Populate cache
		for i := 0; i < 1000; i++ {
			cache.Set(keyForBench(i), i)
		}

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			cache.Get(keyForBench(i % 1000))
		}
	})

	b.Run("GetWithoutTTL", func(b *testing.B) {
		cache := NewCache(Config{
			MaxSize: 10000,
			TTL:     0,
		})
		defer func() {
			if err := cache.Close(); err != nil {
				b.Errorf("Failed to close cache: %v", err)
			}
		}()

		// Populate cache
		for i := 0; i < 1000; i++ {
			cache.Set(keyForBench(i), i)
		}

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			cache.Get(keyForBench(i % 1000))
		}
	})
}

func keyForBench(i int) string {
	return string(rune('a' + (i % 26)))
}
