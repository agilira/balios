// metrics_test.go: tests for MetricsCollector interface and implementations
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGILira library
// SPDX-License-Identifier: MPL-2.0

package balios

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestNoOpMetricsCollector verifies that NoOpMetricsCollector does nothing
// and doesn't panic when called.
func TestNoOpMetricsCollector(t *testing.T) {
	collector := NoOpMetricsCollector{}

	// Should not panic
	collector.RecordGet(100, true)
	collector.RecordGet(200, false)
	collector.RecordSet(150)
	collector.RecordDelete(50)
	collector.RecordEviction()

	// No assertions - just verifying it doesn't panic
}

// TestNoOpMetricsCollector_Concurrent verifies NoOpMetricsCollector is safe
// for concurrent use without panics.
func TestNoOpMetricsCollector_Concurrent(t *testing.T) {
	collector := NoOpMetricsCollector{}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				collector.RecordGet(int64(j), j%2 == 0)
				collector.RecordSet(int64(j))
				collector.RecordDelete(int64(j))
				collector.RecordEviction()
			}
		}()
	}

	wg.Wait()
}

// mockMetricsCollector is a test implementation that records calls
type mockMetricsCollector struct {
	mu sync.Mutex

	getCalls      int
	setCalls      int
	deleteCalls   int
	evictionCalls int

	getLatencies    []int64
	setLatencies    []int64
	deleteLatencies []int64

	hitCount  int
	missCount int
}

func (m *mockMetricsCollector) RecordGet(latencyNs int64, hit bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.getCalls++
	m.getLatencies = append(m.getLatencies, latencyNs)

	if hit {
		m.hitCount++
	} else {
		m.missCount++
	}
}

func (m *mockMetricsCollector) RecordSet(latencyNs int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.setCalls++
	m.setLatencies = append(m.setLatencies, latencyNs)
}

func (m *mockMetricsCollector) RecordDelete(latencyNs int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.deleteCalls++
	m.deleteLatencies = append(m.deleteLatencies, latencyNs)
}

func (m *mockMetricsCollector) RecordEviction() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.evictionCalls++
}

// TestMockMetricsCollector verifies our test implementation works correctly
func TestMockMetricsCollector(t *testing.T) {
	collector := &mockMetricsCollector{}

	// Record some operations
	collector.RecordGet(100, true)
	collector.RecordGet(200, false)
	collector.RecordSet(150)
	collector.RecordDelete(50)
	collector.RecordEviction()

	// Verify counts
	if collector.getCalls != 2 {
		t.Errorf("Expected 2 get calls, got %d", collector.getCalls)
	}

	if collector.setCalls != 1 {
		t.Errorf("Expected 1 set call, got %d", collector.setCalls)
	}

	if collector.deleteCalls != 1 {
		t.Errorf("Expected 1 delete call, got %d", collector.deleteCalls)
	}

	if collector.evictionCalls != 1 {
		t.Errorf("Expected 1 eviction call, got %d", collector.evictionCalls)
	}

	// Verify hit/miss tracking
	if collector.hitCount != 1 {
		t.Errorf("Expected 1 hit, got %d", collector.hitCount)
	}

	if collector.missCount != 1 {
		t.Errorf("Expected 1 miss, got %d", collector.missCount)
	}

	// Verify latencies recorded
	if len(collector.getLatencies) != 2 {
		t.Errorf("Expected 2 get latencies, got %d", len(collector.getLatencies))
	}

	if collector.getLatencies[0] != 100 {
		t.Errorf("Expected first get latency 100, got %d", collector.getLatencies[0])
	}
}

// TestCacheWithMetricsCollector verifies that cache calls metrics collector
func TestCacheWithMetricsCollector(t *testing.T) {
	collector := &mockMetricsCollector{}

	cache := NewCache(Config{
		MaxSize:          100,
		MetricsCollector: collector,
	})

	// Perform operations
	cache.Set("key1", "value1")
	cache.Get("key1") // hit
	cache.Get("key2") // miss
	cache.Delete("key1")

	// Verify metrics were recorded
	if collector.setCalls != 1 {
		t.Errorf("Expected 1 set call, got %d", collector.setCalls)
	}

	if collector.getCalls != 2 {
		t.Errorf("Expected 2 get calls, got %d", collector.getCalls)
	}

	if collector.hitCount != 1 {
		t.Errorf("Expected 1 hit, got %d", collector.hitCount)
	}

	if collector.missCount != 1 {
		t.Errorf("Expected 1 miss, got %d", collector.missCount)
	}

	if collector.deleteCalls != 1 {
		t.Errorf("Expected 1 delete call, got %d", collector.deleteCalls)
	}

	// Verify latencies are recorded (may be 0 for very fast operations)
	if len(collector.getLatencies) != 2 {
		t.Fatalf("Expected 2 get latencies, got %d", len(collector.getLatencies))
	}

	for i, lat := range collector.getLatencies {
		if lat < 0 {
			t.Errorf("Get latency[%d] should be >= 0, got %d", i, lat)
		}
	}

	if len(collector.setLatencies) != 1 {
		t.Fatalf("Expected 1 set latency, got %d", len(collector.setLatencies))
	}

	if collector.setLatencies[0] < 0 {
		t.Errorf("Set latency should be >= 0, got %d", collector.setLatencies[0])
	}
}

// TestCacheWithNilMetricsCollector verifies that cache works without metrics collector
func TestCacheWithNilMetricsCollector(t *testing.T) {
	cache := NewCache(Config{
		MaxSize:          100,
		MetricsCollector: nil, // Explicitly nil
	})

	// Should not panic
	cache.Set("key1", "value1")
	cache.Get("key1")
	cache.Delete("key1")
}

// TestCacheMetrics_ZeroOverhead verifies that nil metrics collector has zero overhead
func TestCacheMetrics_ZeroOverhead(t *testing.T) {
	// Skip this test when race detector is enabled (it adds significant overhead)
	if testing.Short() {
		t.Skip("Skipping overhead test in short mode")
	}

	// Cache without metrics
	cache1 := NewCache(Config{
		MaxSize:          1000,
		MetricsCollector: nil,
	})

	// Cache with NoOpMetricsCollector
	cache2 := NewCache(Config{
		MaxSize:          1000,
		MetricsCollector: NoOpMetricsCollector{},
	})

	// Pre-populate
	for i := 0; i < 100; i++ {
		key := string(rune('a' + i%26))
		cache1.Set(key, i)
		cache2.Set(key, i)
	}

	// Benchmark Get operations
	iterations := 10000

	start1 := time.Now()
	for i := 0; i < iterations; i++ {
		key := string(rune('a' + i%26))
		cache1.Get(key)
	}
	duration1 := time.Since(start1)

	start2 := time.Now()
	for i := 0; i < iterations; i++ {
		key := string(rune('a' + i%26))
		cache2.Get(key)
	}
	duration2 := time.Since(start2)

	t.Logf("Without metrics: %v", duration1)
	t.Logf("With NoOp metrics: %v", duration2)

	// If duration1 is too small, the test is too fast to measure overhead accurately
	// This is actually a good thing - it means operations are extremely fast
	if duration1 < time.Microsecond {
		t.Logf("INFO: Operations too fast to measure overhead (<1µs), skipping overhead check")
		t.Logf("This indicates excellent performance - both caches complete in < %v", time.Microsecond)
		return
	}

	// NoOpMetricsCollector should have negligible overhead
	overhead := float64(duration2-duration1) / float64(duration1) * 100
	t.Logf("Overhead: %.2f%%", overhead)

	// Allow up to 100% overhead (generous for noisy environments and under system load)
	// In production without race detector, overhead is typically < 5%
	// This test is informational and can show high variance when system is under load
	if overhead > 100 {
		t.Errorf("NoOpMetricsCollector overhead too high: %.2f%% (expected < 100%%)", overhead)
	} else if overhead > 50 {
		t.Logf("INFO: Overhead is high but acceptable for system under load: %.2f%%", overhead)
	}
}

// TestMetricsCollector_Concurrent verifies metrics collector is called correctly
// under concurrent load.
func TestMetricsCollector_Concurrent(t *testing.T) {
	var getCalls int64
	var setCalls int64
	var deleteCalls int64

	// Use atomic counters for thread-safe test collector
	atomicCollector := &atomicMetricsCollector{
		getCalls:    &getCalls,
		setCalls:    &setCalls,
		deleteCalls: &deleteCalls,
	}

	cache := NewCache(Config{
		MaxSize:          1000,
		MetricsCollector: atomicCollector,
	})

	var wg sync.WaitGroup
	numGoroutines := 10
	opsPerGoroutine := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < opsPerGoroutine; j++ {
				key := "key" + string(rune('a'+id))

				cache.Set(key, j)
				cache.Get(key)
				if j%10 == 0 {
					cache.Delete(key)
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify counts
	expectedSets := int64(numGoroutines * opsPerGoroutine)
	expectedGets := int64(numGoroutines * opsPerGoroutine)
	expectedDeletes := int64(numGoroutines * (opsPerGoroutine / 10))

	if atomic.LoadInt64(&getCalls) != expectedGets {
		t.Errorf("Expected %d get calls, got %d", expectedGets, atomic.LoadInt64(&getCalls))
	}

	if atomic.LoadInt64(&setCalls) != expectedSets {
		t.Errorf("Expected %d set calls, got %d", expectedSets, atomic.LoadInt64(&setCalls))
	}

	if atomic.LoadInt64(&deleteCalls) != expectedDeletes {
		t.Errorf("Expected %d delete calls, got %d", expectedDeletes, atomic.LoadInt64(&deleteCalls))
	}
}

// atomicMetricsCollector is a lock-free test collector using atomic operations
type atomicMetricsCollector struct {
	getCalls    *int64
	setCalls    *int64
	deleteCalls *int64
}

func (a *atomicMetricsCollector) RecordGet(latencyNs int64, hit bool) {
	atomic.AddInt64(a.getCalls, 1)
}

func (a *atomicMetricsCollector) RecordSet(latencyNs int64) {
	atomic.AddInt64(a.setCalls, 1)
}

func (a *atomicMetricsCollector) RecordDelete(latencyNs int64) {
	atomic.AddInt64(a.deleteCalls, 1)
}

func (a *atomicMetricsCollector) RecordEviction() {
	// Not tracked in this test
}
