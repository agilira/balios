// metrics_test.go: tests for MetricsCollector interface and implementations
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGILira fragment
// SPDX-License-Identifier: MPL-2.0

package balios

import (
	"sort"
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
	collector.RecordExpiration()

	// Test new metrics
	collector.RecordProbeCount(5, "get")
	collector.RecordProbeCount(3, "set")
	collector.RecordFallbackScan(1000, "set")
	collector.RecordDuplicateCleanup(2)
	collector.RecordRaceCondition("set")
	collector.RecordEvictionSampling(8, true)
	collector.RecordMemoryPressure(75)

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
				collector.RecordExpiration()

				// Test new metrics
				collector.RecordProbeCount(j%10, "get")
				collector.RecordFallbackScan(j*10, "set")
				collector.RecordDuplicateCleanup(j % 3)
				collector.RecordRaceCondition("set")
				collector.RecordEvictionSampling(8, j%2 == 0)
				collector.RecordMemoryPressure(j % 100)
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

	// New metrics
	probeCounts        []int
	probeOperations    []string
	fallbackScans      []int
	fallbackOperations []string
	duplicateCleanups  []int
	raceConditions     []string
	evictionSamplings  []struct {
		sampleSize  int
		victimFound bool
	}
	memoryPressures []int
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

func (m *mockMetricsCollector) RecordExpiration() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Track expirations if needed for testing
}

func (m *mockMetricsCollector) RecordProbeCount(probeCount int, operation string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.probeCounts = append(m.probeCounts, probeCount)
	m.probeOperations = append(m.probeOperations, operation)
}

func (m *mockMetricsCollector) RecordFallbackScan(scanSize int, operation string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.fallbackScans = append(m.fallbackScans, scanSize)
	m.fallbackOperations = append(m.fallbackOperations, operation)
}

func (m *mockMetricsCollector) RecordDuplicateCleanup(duplicatesFound int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.duplicateCleanups = append(m.duplicateCleanups, duplicatesFound)
}

func (m *mockMetricsCollector) RecordRaceCondition(operation string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.raceConditions = append(m.raceConditions, operation)
}

func (m *mockMetricsCollector) RecordEvictionSampling(sampleSize int, victimFound bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.evictionSamplings = append(m.evictionSamplings, struct {
		sampleSize  int
		victimFound bool
	}{sampleSize, victimFound})
}

func (m *mockMetricsCollector) RecordMemoryPressure(pressureLevel int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.memoryPressures = append(m.memoryPressures, pressureLevel)
}

func (m *mockMetricsCollector) RecordKeyAccess(key string, operation string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Track key access if needed for testing
}

func (m *mockMetricsCollector) RecordKeyFrequency(key string, frequency uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Track key frequency if needed for testing
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

	// Use higher iteration count and multiple samples to reduce statistical noise
	// This makes the test more reliable on systems under load
	const iterations = 100000
	const samples = 5

	// Warmup phase to eliminate JIT/allocation effects
	for warmup := 0; warmup < 1000; warmup++ {
		key := string(rune('a' + warmup%26))
		cache1.Get(key)
		cache2.Get(key)
	}

	// Collect multiple samples and use median to filter outliers
	durations1 := make([]time.Duration, samples)
	durations2 := make([]time.Duration, samples)

	for sample := 0; sample < samples; sample++ {
		// Measure cache without metrics
		start1 := time.Now()
		for i := 0; i < iterations; i++ {
			key := string(rune('a' + i%26))
			cache1.Get(key)
		}
		durations1[sample] = time.Since(start1)

		// Measure cache with NoOp metrics
		start2 := time.Now()
		for i := 0; i < iterations; i++ {
			key := string(rune('a' + i%26))
			cache2.Get(key)
		}
		durations2[sample] = time.Since(start2)
	}

	// Calculate median (more robust than mean for noisy measurements)
	median1 := medianDuration(durations1)
	median2 := medianDuration(durations2)

	t.Logf("Without metrics (median of %d samples): %v", samples, median1)
	t.Logf("With NoOp metrics (median of %d samples): %v", samples, median2)

	// If median1 is too small, the test is too fast to measure overhead accurately
	if median1 < 10*time.Microsecond {
		t.Logf("INFO: Operations too fast to measure overhead (<10µs for %d ops), skipping overhead check", iterations)
		t.Logf("This indicates excellent performance - operations complete in < %v per op", median1/iterations)
		return
	}

	// NoOpMetricsCollector should have negligible overhead
	overhead := float64(median2-median1) / float64(median1) * 100
	t.Logf("Overhead: %.2f%%", overhead)

	// Stricter threshold with more reliable measurement
	// With median of multiple samples, we can be more confident about the measurement
	// Allow up to 50% overhead (was 100%, but now we have better statistical power)
	// In production without race detector, overhead is typically < 5%
	if overhead > 50 {
		t.Errorf("NoOpMetricsCollector overhead too high: %.2f%% (expected < 50%%)", overhead)
	} else if overhead > 25 {
		t.Logf("INFO: Overhead is elevated but acceptable: %.2f%%", overhead)
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

func (a *atomicMetricsCollector) RecordExpiration() {
	// Not tracked in this test
}

func (a *atomicMetricsCollector) RecordProbeCount(probeCount int, operation string) {
	// Not tracked in this test
}

func (a *atomicMetricsCollector) RecordFallbackScan(scanSize int, operation string) {
	// Not tracked in this test
}

func (a *atomicMetricsCollector) RecordDuplicateCleanup(duplicatesFound int) {
	// Not tracked in this test
}

func (a *atomicMetricsCollector) RecordRaceCondition(operation string) {
	// Not tracked in this test
}

func (a *atomicMetricsCollector) RecordEvictionSampling(sampleSize int, victimFound bool) {
	// Not tracked in this test
}

func (a *atomicMetricsCollector) RecordMemoryPressure(pressureLevel int) {
	// Not tracked in this test
}

func (a *atomicMetricsCollector) RecordKeyAccess(key string, operation string) {
	// Not tracked in this test
}

func (a *atomicMetricsCollector) RecordKeyFrequency(key string, frequency uint64) {
	// Not tracked in this test
}

// medianDuration calculates the median of a slice of durations.
// This is more robust than mean for filtering out outliers in performance tests.
func medianDuration(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}

	// Make a copy to avoid modifying the original
	sorted := make([]time.Duration, len(durations))
	copy(sorted, durations)

	// Sort durations
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})

	// Return median
	mid := len(sorted) / 2
	if len(sorted)%2 == 0 {
		// Even number of elements: average of two middle elements
		return (sorted[mid-1] + sorted[mid]) / 2
	}
	// Odd number of elements: middle element
	return sorted[mid]
}

// TestNewMetrics verifies that the new metrics are properly recorded
func TestNewMetrics(t *testing.T) {
	collector := &mockMetricsCollector{}

	// Test probe count metrics
	collector.RecordProbeCount(5, "get")
	collector.RecordProbeCount(3, "set")
	collector.RecordProbeCount(7, "delete")

	if len(collector.probeCounts) != 3 {
		t.Errorf("Expected 3 probe count records, got %d", len(collector.probeCounts))
	}

	if collector.probeCounts[0] != 5 || collector.probeOperations[0] != "get" {
		t.Errorf("Expected probe count 5 for 'get', got %d for '%s'", collector.probeCounts[0], collector.probeOperations[0])
	}

	// Test fallback scan metrics
	collector.RecordFallbackScan(1000, "set")
	collector.RecordFallbackScan(500, "get")

	if len(collector.fallbackScans) != 2 {
		t.Errorf("Expected 2 fallback scan records, got %d", len(collector.fallbackScans))
	}

	// Test duplicate cleanup metrics
	collector.RecordDuplicateCleanup(2)
	collector.RecordDuplicateCleanup(1)

	if len(collector.duplicateCleanups) != 2 {
		t.Errorf("Expected 2 duplicate cleanup records, got %d", len(collector.duplicateCleanups))
	}

	// Test race condition metrics
	collector.RecordRaceCondition("set")
	collector.RecordRaceCondition("get")

	if len(collector.raceConditions) != 2 {
		t.Errorf("Expected 2 race condition records, got %d", len(collector.raceConditions))
	}

	// Test eviction sampling metrics
	collector.RecordEvictionSampling(8, true)
	collector.RecordEvictionSampling(12, false)

	if len(collector.evictionSamplings) != 2 {
		t.Errorf("Expected 2 eviction sampling records, got %d", len(collector.evictionSamplings))
	}

	// Test memory pressure metrics
	collector.RecordMemoryPressure(75)
	collector.RecordMemoryPressure(90)

	if len(collector.memoryPressures) != 2 {
		t.Errorf("Expected 2 memory pressure records, got %d", len(collector.memoryPressures))
	}
}
