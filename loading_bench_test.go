// loading_bench_test.go: benchmarks for loading functionality in Balios
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGILira library
// SPDX-License-Identifier: MPL-2.0

package balios

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

// BenchmarkGetOrLoad_CacheHit benchmarks GetOrLoad with cache hit (no loader call)
func BenchmarkGetOrLoad_CacheHit(b *testing.B) {
	cache := NewCache(Config{MaxSize: 10000})

	// Pre-populate cache
	cache.Set("key1", "value1")

	loader := func() (interface{}, error) {
		return "loaded", nil
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = cache.GetOrLoad("key1", loader)
	}
}

// BenchmarkGetOrLoad_CacheMiss benchmarks GetOrLoad with cache miss (fast loader)
func BenchmarkGetOrLoad_CacheMiss(b *testing.B) {
	cache := NewCache(Config{MaxSize: 10000})

	loader := func() (interface{}, error) {
		return "loaded", nil
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key%d", i)
		_, _ = cache.GetOrLoad(key, loader)
	}
}

// BenchmarkGetOrLoad_CacheMiss_SlowLoader benchmarks GetOrLoad with slow loader
func BenchmarkGetOrLoad_CacheMiss_SlowLoader(b *testing.B) {
	cache := NewCache(Config{MaxSize: 10000})

	loader := func() (interface{}, error) {
		time.Sleep(1 * time.Millisecond) // Simulate DB call
		return "loaded", nil
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key%d", i)
		_, _ = cache.GetOrLoad(key, loader)
	}
}

// BenchmarkGetOrLoad_Concurrent_Singleflight benchmarks singleflight effectiveness
// Multiple goroutines request same missing key - measures how many loader calls happen
func BenchmarkGetOrLoad_Concurrent_Singleflight(b *testing.B) {
	cache := NewCache(Config{MaxSize: 10000})

	var loaderCalls int64
	loader := func() (interface{}, error) {
		atomic.AddInt64(&loaderCalls, 1)
		time.Sleep(10 * time.Millisecond) // Slow loader to ensure concurrency
		return "loaded", nil
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = cache.GetOrLoad("hotkey", loader)
		}
	})

	b.ReportMetric(float64(loaderCalls), "loader-calls")
	efficiency := float64(b.N) / float64(loaderCalls)
	b.ReportMetric(efficiency, "efficiency")
}

// BenchmarkGetOrLoadWithContext_CacheHit benchmarks context version with cache hit
func BenchmarkGetOrLoadWithContext_CacheHit(b *testing.B) {
	cache := NewCache(Config{MaxSize: 10000})
	ctx := context.Background()

	// Pre-populate cache
	cache.Set("key1", "value1")

	loader := func(ctx context.Context) (interface{}, error) {
		return "loaded", nil
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = cache.GetOrLoadWithContext(ctx, "key1", loader)
	}
}

// BenchmarkGetOrLoadWithContext_CacheMiss benchmarks context version with cache miss
func BenchmarkGetOrLoadWithContext_CacheMiss(b *testing.B) {
	cache := NewCache(Config{MaxSize: 10000})
	ctx := context.Background()

	loader := func(ctx context.Context) (interface{}, error) {
		return "loaded", nil
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key%d", i)
		_, _ = cache.GetOrLoadWithContext(ctx, key, loader)
	}
}

// BenchmarkGenericCache_GetOrLoad_CacheHit benchmarks generic API with cache hit
func BenchmarkGenericCache_GetOrLoad_CacheHit(b *testing.B) {
	cache := NewGenericCache[string, string](Config{MaxSize: 10000})

	// Pre-populate cache
	cache.Set("key1", "value1")

	loader := func() (string, error) {
		return "loaded", nil
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = cache.GetOrLoad("key1", loader)
	}
}

// BenchmarkGenericCache_GetOrLoad_CacheMiss benchmarks generic API with cache miss
func BenchmarkGenericCache_GetOrLoad_CacheMiss(b *testing.B) {
	cache := NewGenericCache[string, string](Config{MaxSize: 10000})

	loader := func() (string, error) {
		return "loaded", nil
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key%d", i)
		_, _ = cache.GetOrLoad(key, loader)
	}
}

// BenchmarkGetOrLoad_vs_GetSet compares GetOrLoad with manual Get+Set pattern
func BenchmarkGetOrLoad_vs_GetSet(b *testing.B) {
	b.Run("GetOrLoad", func(b *testing.B) {
		cache := NewCache(Config{MaxSize: 10000})
		loader := func() (interface{}, error) {
			return "loaded", nil
		}

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("key%d", i%100)
			_, _ = cache.GetOrLoad(key, loader)
		}
	})

	b.Run("Manual_Get_Set", func(b *testing.B) {
		cache := NewCache(Config{MaxSize: 10000})

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("key%d", i%100)
			if _, found := cache.Get(key); !found {
				cache.Set(key, "loaded")
			}
		}
	})
}
