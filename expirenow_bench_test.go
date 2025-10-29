// expirenow_bench_test.go: Benchmark ExpireNow() to measure O(capacity) overhead
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGILira fragment
// SPDX-License-Identifier: MPL-2.0

package balios

import (
	"fmt"
	"testing"
	"time"
)

// BenchmarkExpireNow_LowLoad simulates "worst case" from code review:
// capacity=1M, size=1K (0.1% load factor)
func BenchmarkExpireNow_LowLoad(b *testing.B) {
	cache := NewCache(Config{
		MaxSize: 1_000_000, // 1M capacity
		TTL:     time.Hour,
	})

	// Populate only 0.1% (1K entries)
	for i := 0; i < 1000; i++ {
		cache.Set(fmt.Sprintf("key-%d", i), i)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		cache.ExpireNow()
	}
}

// BenchmarkExpireNow_MediumLoad tests typical production scenario
// capacity=100K, size=50K (50% load factor)
func BenchmarkExpireNow_MediumLoad(b *testing.B) {
	cache := NewCache(Config{
		MaxSize: 100_000,
		TTL:     time.Hour,
	})

	// Populate 50%
	for i := 0; i < 50_000; i++ {
		cache.Set(fmt.Sprintf("key-%d", i), i)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		cache.ExpireNow()
	}
}

// BenchmarkExpireNow_HighLoad tests cache near capacity
// capacity=100K, size=90K (90% load factor)
func BenchmarkExpireNow_HighLoad(b *testing.B) {
	cache := NewCache(Config{
		MaxSize: 100_000,
		TTL:     time.Hour,
	})

	// Populate 90%
	for i := 0; i < 90_000; i++ {
		cache.Set(fmt.Sprintf("key-%d", i), i)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		cache.ExpireNow()
	}
}

// BenchmarkExpireNow_WithExpiredEntries tests actual expiration work
func BenchmarkExpireNow_WithExpiredEntries(b *testing.B) {
	cache := NewCache(Config{
		MaxSize: 100_000,
		TTL:     50 * time.Millisecond, // Short TTL
	})

	// Populate cache
	for i := 0; i < 10_000; i++ {
		cache.Set(fmt.Sprintf("key-%d", i), i)
	}

	// Wait for entries to expire
	time.Sleep(100 * time.Millisecond)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Re-populate between iterations
		if i%10 == 0 {
			for j := 0; j < 10_000; j++ {
				cache.Set(fmt.Sprintf("key-%d", j), j)
			}
			time.Sleep(100 * time.Millisecond)
		}
		cache.ExpireNow()
	}
}

// BenchmarkExpireNow_NoTTL tests fast-path when TTL disabled
func BenchmarkExpireNow_NoTTL(b *testing.B) {
	cache := NewCache(Config{
		MaxSize: 100_000,
		// No TTL
	})

	// Populate cache
	for i := 0; i < 50_000; i++ {
		cache.Set(fmt.Sprintf("key-%d", i), i)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		cache.ExpireNow() // Should return immediately (0 overhead)
	}
}

// BenchmarkInlineExpiration_vs_ManualExpiration compares inline vs manual
func BenchmarkInlineExpiration_vs_ManualExpiration(b *testing.B) {
	b.Run("Inline", func(b *testing.B) {
		cache := NewCache(Config{
			MaxSize: 10_000,
			TTL:     50 * time.Millisecond,
		})

		// Populate
		for i := 0; i < 1000; i++ {
			cache.Set(fmt.Sprintf("key-%d", i), i)
		}

		time.Sleep(100 * time.Millisecond) // Expire entries

		b.ResetTimer()
		b.ReportAllocs()

		// Get operations will trigger inline expiration
		for i := 0; i < b.N; i++ {
			cache.Get(fmt.Sprintf("key-%d", i%1000))
		}
	})

	b.Run("Manual", func(b *testing.B) {
		cache := NewCache(Config{
			MaxSize: 10_000,
			TTL:     50 * time.Millisecond,
		})

		// Populate
		for i := 0; i < 1000; i++ {
			cache.Set(fmt.Sprintf("key-%d", i), i)
		}

		time.Sleep(100 * time.Millisecond) // Expire entries

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			// Manual expiration
			if i%1000 == 0 { // Amortize cost
				cache.ExpireNow()
			}
			cache.Get(fmt.Sprintf("key-%d", i%1000))
		}
	})
}

// BenchmarkExpireNow_Contended tests ExpireNow under concurrent load
func BenchmarkExpireNow_Contended(b *testing.B) {
	cache := NewCache(Config{
		MaxSize: 100_000,
		TTL:     time.Hour,
	})

	// Populate cache
	for i := 0; i < 50_000; i++ {
		cache.Set(fmt.Sprintf("key-%d", i), i)
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%100 == 0 {
				// 1% of operations are ExpireNow
				cache.ExpireNow()
			} else {
				// 99% are Get/Set
				key := fmt.Sprintf("key-%d", i%50_000)
				if i%5 == 0 {
					cache.Set(key, i)
				} else {
					cache.Get(key)
				}
			}
			i++
		}
	})
}
