// string_clone_bench_test.go: benchmark strings.Clone overhead
//
// Copyright (c) 2025 AGILira - A. Giordano
// SPDX-License-Identifier: MPL-2.0

package balios

import (
	"strings"
	"testing"
	"unsafe"
)

// Benchmark strings.Clone vs unsafe string reconstruction
// to measure overhead and validate safety-first approach.

// BenchmarkStringClone_Small tests cloning small keys (typical case)
func BenchmarkStringClone_Small(b *testing.B) {
	key := "user:123" // 8 bytes - typical cache key
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = strings.Clone(key)
	}
}

// BenchmarkStringClone_Medium tests cloning medium keys
func BenchmarkStringClone_Medium(b *testing.B) {
	key := "user:session:abc123def456ghi789" // 32 bytes
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = strings.Clone(key)
	}
}

// BenchmarkStringClone_Large tests cloning large keys
func BenchmarkStringClone_Large(b *testing.B) {
	key := strings.Repeat("x", 256) // 256 bytes
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = strings.Clone(key)
	}
}

// BenchmarkStringUnsafe_Small tests unsafe string reconstruction (for comparison)
func BenchmarkStringUnsafe_Small(b *testing.B) {
	key := "user:123"
	data := unsafe.StringData(key)
	length := len(key)
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = unsafe.String((*byte)(unsafe.Pointer(data)), length)
	}
}

// BenchmarkStringUnsafe_Medium tests unsafe string reconstruction
func BenchmarkStringUnsafe_Medium(b *testing.B) {
	key := "user:session:abc123def456ghi789"
	data := unsafe.StringData(key)
	length := len(key)
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = unsafe.String((*byte)(unsafe.Pointer(data)), length)
	}
}

// BenchmarkCacheSet_WithClone benchmarks Set operation (includes strings.Clone)
func BenchmarkCacheSet_WithClone(b *testing.B) {
	cache := NewCache(Config{MaxSize: 10000})
	keys := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		keys[i] = "user:" + string(rune('0'+i%10)) + ":" + string(rune('a'+i%26))
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		cache.Set(keys[i%1000], i)
	}
}

// BenchmarkCacheSet_Parallel tests Set under contention (real-world scenario)
func BenchmarkCacheSet_Parallel(b *testing.B) {
	cache := NewCache(Config{MaxSize: 10000})
	keys := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		keys[i] = "user:" + string(rune('0'+i%10)) + ":" + string(rune('a'+i%26))
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			cache.Set(keys[i%1000], i)
			i++
		}
	})
}

// BenchmarkWorkloadRealistic simulates realistic cache workload (80% read, 20% write)
func BenchmarkWorkloadRealistic(b *testing.B) {
	cache := NewCache(Config{MaxSize: 10000})

	// Pre-populate cache
	for i := 0; i < 1000; i++ {
		key := "key:" + string(rune('0'+i%10)) + string(rune('a'+i%26))
		cache.Set(key, i)
	}

	keys := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		keys[i] = "key:" + string(rune('0'+i%10)) + string(rune('a'+i%26))
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		key := keys[i%1000]
		if i%5 == 0 {
			// 20% writes
			cache.Set(key, i)
		} else {
			// 80% reads
			_, _ = cache.Get(key)
		}
	}
}
