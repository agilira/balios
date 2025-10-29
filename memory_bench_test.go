// memory_bench_test.go: benchmarks for memory footprint analysis
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGILira fragment
// SPDX-License-Identifier: MPL-2.0

package balios

import (
	"fmt"
	"runtime"
	"testing"
	"time"
)

// BenchmarkMemoryFootprint_Empty measures memory usage of empty cache
func BenchmarkMemoryFootprint_Empty(b *testing.B) {
	sizes := []int{100, 1000, 10000, 100000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("Size%d", size), func(b *testing.B) {
			// Force GC before measurement
			runtime.GC()
			runtime.GC()

			var m1, m2 runtime.MemStats
			runtime.ReadMemStats(&m1)

			// Create cache
			cache := NewCache(Config{
				MaxSize: size,
			})

			runtime.GC()
			runtime.ReadMemStats(&m2)

			bytesUsed := m2.Alloc - m1.Alloc
			bytesPerEntry := float64(bytesUsed) / float64(size)

			b.ReportMetric(float64(bytesUsed), "bytes")
			b.ReportMetric(bytesPerEntry, "bytes/entry")

			// Keep cache alive
			_ = cache
		})
	}
}

// BenchmarkMemoryFootprint_Populated measures memory usage with data
func BenchmarkMemoryFootprint_Populated(b *testing.B) {
	sizes := []int{100, 1000, 10000, 100000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("Size%d", size), func(b *testing.B) {
			cache := NewCache(Config{
				MaxSize: size,
			})

			// Force GC before measurement
			runtime.GC()
			runtime.GC()

			var m1, m2 runtime.MemStats
			runtime.ReadMemStats(&m1)

			// Populate cache with data
			for i := 0; i < size; i++ {
				key := fmt.Sprintf("key_%d", i)
				value := fmt.Sprintf("value_%d", i)
				cache.Set(key, value)
			}

			runtime.GC()
			runtime.ReadMemStats(&m2)

			bytesUsed := m2.Alloc - m1.Alloc
			bytesPerEntry := float64(bytesUsed) / float64(size)

			b.ReportMetric(float64(bytesUsed), "bytes")
			b.ReportMetric(bytesPerEntry, "bytes/entry")

			// Verify all entries are there
			stats := cache.Stats()
			if stats.Size != size {
				b.Errorf("Expected size %d, got %d", size, stats.Size)
			}
		})
	}
}

// BenchmarkMemoryFootprint_GenericCache measures memory usage of generic cache
func BenchmarkMemoryFootprint_GenericCache(b *testing.B) {
	sizes := []int{100, 1000, 10000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("Size%d", size), func(b *testing.B) {
			// Force GC before measurement
			runtime.GC()
			runtime.GC()

			var m1, m2 runtime.MemStats
			runtime.ReadMemStats(&m1)

			// Create and populate generic cache
			cache := NewGenericCache[string, string](Config{
				MaxSize: size,
			})

			for i := 0; i < size; i++ {
				key := fmt.Sprintf("key_%d", i)
				value := fmt.Sprintf("value_%d", i)
				cache.Set(key, value)
			}

			runtime.GC()
			runtime.ReadMemStats(&m2)

			bytesUsed := m2.Alloc - m1.Alloc
			bytesPerEntry := float64(bytesUsed) / float64(size)

			b.ReportMetric(float64(bytesUsed), "bytes")
			b.ReportMetric(bytesPerEntry, "bytes/entry")
		})
	}
}

// BenchmarkMemoryFootprint_WithTTL measures memory usage with TTL enabled
func BenchmarkMemoryFootprint_WithTTL(b *testing.B) {
	cache := NewCache(Config{
		MaxSize: 10000,
		TTL:     time.Hour,
	})

	// Force GC before measurement
	runtime.GC()
	runtime.GC()

	var m1, m2 runtime.MemStats
	runtime.ReadMemStats(&m1)

	// Populate cache
	for i := 0; i < 10000; i++ {
		key := fmt.Sprintf("key_%d", i)
		value := fmt.Sprintf("value_%d", i)
		cache.Set(key, value)
	}

	runtime.GC()
	runtime.ReadMemStats(&m2)

	bytesUsed := m2.Alloc - m1.Alloc
	bytesPerEntry := float64(bytesUsed) / 10000.0

	b.ReportMetric(float64(bytesUsed), "bytes")
	b.ReportMetric(bytesPerEntry, "bytes/entry")
}

// BenchmarkMemoryFootprint_LargeValues measures memory with large values
func BenchmarkMemoryFootprint_LargeValues(b *testing.B) {
	valueSizes := []int{100, 1000, 10000} // bytes per value

	for _, valueSize := range valueSizes {
		b.Run(fmt.Sprintf("ValueSize%d", valueSize), func(b *testing.B) {
			cache := NewCache(Config{
				MaxSize: 1000,
			})

			// Create large value
			largeValue := make([]byte, valueSize)
			for i := range largeValue {
				largeValue[i] = byte(i % 256)
			}

			// Force GC before measurement
			runtime.GC()
			runtime.GC()

			var m1, m2 runtime.MemStats
			runtime.ReadMemStats(&m1)

			// Populate cache with large values
			for i := 0; i < 1000; i++ {
				key := fmt.Sprintf("key_%d", i)
				cache.Set(key, largeValue)
			}

			runtime.GC()
			runtime.ReadMemStats(&m2)

			bytesUsed := m2.Alloc - m1.Alloc
			bytesPerEntry := float64(bytesUsed) / 1000.0
			overhead := bytesPerEntry - float64(valueSize)

			b.ReportMetric(float64(bytesUsed), "total_bytes")
			b.ReportMetric(bytesPerEntry, "bytes/entry")
			b.ReportMetric(overhead, "overhead/entry")
		})
	}
}

// BenchmarkMemoryFootprint_Comparison compares memory usage with different cache types
func BenchmarkMemoryFootprint_Comparison(b *testing.B) {
	const size = 10000

	b.Run("Cache", func(b *testing.B) {
		runtime.GC()
		runtime.GC()
		var m1, m2 runtime.MemStats
		runtime.ReadMemStats(&m1)

		cache := NewCache(Config{MaxSize: size})
		for i := 0; i < size; i++ {
			cache.Set(fmt.Sprintf("key_%d", i), fmt.Sprintf("value_%d", i))
		}

		runtime.GC()
		runtime.ReadMemStats(&m2)
		b.ReportMetric(float64(m2.Alloc-m1.Alloc)/size, "bytes/entry")
	})

	b.Run("GenericCache", func(b *testing.B) {
		runtime.GC()
		runtime.GC()
		var m1, m2 runtime.MemStats
		runtime.ReadMemStats(&m1)

		cache := NewGenericCache[string, string](Config{MaxSize: size})
		for i := 0; i < size; i++ {
			cache.Set(fmt.Sprintf("key_%d", i), fmt.Sprintf("value_%d", i))
		}

		runtime.GC()
		runtime.ReadMemStats(&m2)
		b.ReportMetric(float64(m2.Alloc-m1.Alloc)/size, "bytes/entry")
	})

	b.Run("NativeMap", func(b *testing.B) {
		runtime.GC()
		runtime.GC()
		var m1, m2 runtime.MemStats
		runtime.ReadMemStats(&m1)

		m := make(map[string]string, size)
		for i := 0; i < size; i++ {
			m[fmt.Sprintf("key_%d", i)] = fmt.Sprintf("value_%d", i)
		}

		runtime.GC()
		runtime.ReadMemStats(&m2)
		b.ReportMetric(float64(m2.Alloc-m1.Alloc)/size, "bytes/entry")
	})
}

// BenchmarkMemoryAllocation_Set measures allocations per Set operation
func BenchmarkMemoryAllocation_Set(b *testing.B) {
	cache := NewCache(Config{
		MaxSize: 100000,
	})

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key_%d", i%10000)
		value := fmt.Sprintf("value_%d", i%10000)
		cache.Set(key, value)
	}
}

// BenchmarkMemoryAllocation_Get measures allocations per Get operation
func BenchmarkMemoryAllocation_Get(b *testing.B) {
	cache := NewCache(Config{
		MaxSize: 100000,
	})

	// Populate cache
	for i := 0; i < 10000; i++ {
		key := fmt.Sprintf("key_%d", i)
		value := fmt.Sprintf("value_%d", i)
		cache.Set(key, value)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key_%d", i%10000)
		cache.Get(key)
	}
}

// BenchmarkMemoryAllocation_GetOrLoad measures allocations per GetOrLoad operation
func BenchmarkMemoryAllocation_GetOrLoad(b *testing.B) {
	cache := NewCache(Config{
		MaxSize: 100000,
	})

	loader := func() (interface{}, error) {
		return "loaded value", nil
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key_%d", i%10000)
		_, _ = cache.GetOrLoad(key, loader)
	}
}

// BenchmarkMemoryPressure_Eviction measures memory during heavy eviction
func BenchmarkMemoryPressure_Eviction(b *testing.B) {
	cache := NewCache(Config{
		MaxSize: 1000, // Small cache to force evictions
	})

	// Force GC before measurement
	runtime.GC()
	runtime.GC()

	var m1, m2 runtime.MemStats
	runtime.ReadMemStats(&m1)

	// Write many more entries than cache size to force evictions
	for i := 0; i < 10000; i++ {
		key := fmt.Sprintf("key_%d", i)
		value := fmt.Sprintf("value_%d", i)
		cache.Set(key, value)
	}

	runtime.GC()
	runtime.ReadMemStats(&m2)

	stats := cache.Stats()
	bytesUsed := m2.Alloc - m1.Alloc
	bytesPerEntry := float64(bytesUsed) / float64(stats.Size)

	b.ReportMetric(float64(bytesUsed), "total_bytes")
	b.ReportMetric(bytesPerEntry, "bytes/entry")
	b.ReportMetric(float64(stats.Evictions), "evictions")

	// Verify cache stayed at max size
	if stats.Size > 1000 {
		b.Errorf("Cache size %d exceeded max size 1000", stats.Size)
	}
}

// TestMemoryLeak_NegativeCache tests that negative cache doesn't leak memory
func TestMemoryLeak_NegativeCache(t *testing.T) {
	cache := NewCache(Config{
		MaxSize:          1000,
		NegativeCacheTTL: 10 * time.Millisecond, // Short TTL
	})

	loader := func() (interface{}, error) {
		return nil, fmt.Errorf("error")
	}

	// Force GC and measure baseline
	runtime.GC()
	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	// Generate many failed loads
	for i := 0; i < 10000; i++ {
		key := fmt.Sprintf("key_%d", i)
		_, _ = cache.GetOrLoad(key, loader)
	}

	// Wait for negative cache to expire
	time.Sleep(20 * time.Millisecond)

	// Force GC and measure after
	runtime.GC()
	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)

	// Memory growth should be minimal (< 1MB)
	// Use int64 to handle potential underflow in GC
	growth := int64(m2.Alloc) - int64(m1.Alloc)
	if growth < 0 {
		// GC may have run, use HeapAlloc instead
		growth = int64(m2.HeapAlloc) - int64(m1.HeapAlloc)
	}

	absGrowth := growth
	if absGrowth < 0 {
		absGrowth = -absGrowth
	}

	if absGrowth > 1024*1024 {
		t.Errorf("Significant memory change detected: %d bytes", growth)
	}

	t.Logf("Memory growth: %d bytes", growth)
}
