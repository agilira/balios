// cache_generic_benchmark_comparison_test.go: fair comparison between generic and non-generic APIs
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGILira library
// SPDX-License-Identifier: MPL-2.0

package balios

import (
	"testing"
)

// BenchmarkCache_StringKey_Set benchmarks non-generic cache with pre-converted string keys
func BenchmarkCache_StringKey_Set(b *testing.B) {
	cache := NewCache(Config{MaxSize: 10000})
	defer func() { _ = cache.Close() }()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Set("key", i)
	}
}

// BenchmarkCache_StringKey_Get benchmarks non-generic cache Get with string key
func BenchmarkCache_StringKey_Get(b *testing.B) {
	cache := NewCache(Config{MaxSize: 10000})
	defer func() { _ = cache.Close() }()
	cache.Set("key", 42)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Get("key")
	}
}

// BenchmarkGenericCache_StringKey_Set benchmarks generic cache with string keys
func BenchmarkGenericCache_StringKey_Set(b *testing.B) {
	cache := NewGenericCache[string, int](Config{MaxSize: 10000})
	defer func() { _ = cache.Close() }()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Set("key", i)
	}
}

// BenchmarkGenericCache_StringKey_Get benchmarks generic cache Get with string key
func BenchmarkGenericCache_StringKey_Get(b *testing.B) {
	cache := NewGenericCache[string, int](Config{MaxSize: 10000})
	defer func() { _ = cache.Close() }()
	cache.Set("key", 42)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Get("key")
	}
}

// BenchmarkGenericCache_IntKey_Set benchmarks generic cache with int keys
func BenchmarkGenericCache_IntKey_Set(b *testing.B) {
	cache := NewGenericCache[int, string](Config{MaxSize: 10000})
	defer func() { _ = cache.Close() }()
	key := 42 // Use same key to avoid eviction overhead
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Set(key, "value")
	}
}

// BenchmarkGenericCache_IntKey_Get benchmarks generic cache Get with int key
func BenchmarkGenericCache_IntKey_Get(b *testing.B) {
	cache := NewGenericCache[int, string](Config{MaxSize: 10000})
	defer func() { _ = cache.Close() }()
	cache.Set(42, "value")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Get(42)
	}
}
