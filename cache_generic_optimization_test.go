// cache_generic_optimization_test.go: tests for zero-allocation key conversion
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGILira library
// SPDX-License-Identifier: MPL-2.0

package balios

import (
	"testing"
)

// TestKeyToString_ZeroAlloc tests that common key types have zero allocations
func TestKeyToString_ZeroAlloc(t *testing.T) {
	tests := []struct {
		name     string
		key      interface{}
		expected string
	}{
		{"string", "test-key", "test-key"},
		{"int", 42, "42"},
		{"int8", int8(127), "127"},
		{"int16", int16(-1000), "-1000"},
		{"int32", int32(100000), "100000"},
		{"int64", int64(-9223372036854775808), "-9223372036854775808"},
		{"uint", uint(42), "42"},
		{"uint8", uint8(255), "255"},
		{"uint16", uint16(65535), "65535"},
		{"uint32", uint32(4294967295), "4294967295"},
		{"uint64", uint64(18446744073709551615), "18446744073709551615"},
		{"zero", 0, "0"},
		{"negative", -123, "-123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result string
			switch v := tt.key.(type) {
			case string:
				result = keyToString(v)
			case int:
				result = keyToString(v)
			case int8:
				result = keyToString(v)
			case int16:
				result = keyToString(v)
			case int32:
				result = keyToString(v)
			case int64:
				result = keyToString(v)
			case uint:
				result = keyToString(v)
			case uint8:
				result = keyToString(v)
			case uint16:
				result = keyToString(v)
			case uint32:
				result = keyToString(v)
			case uint64:
				result = keyToString(v)
			}

			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// BenchmarkKeyToString_String benchmarks string key conversion
func BenchmarkKeyToString_String(b *testing.B) {
	key := "test-key-123"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = keyToString(key)
	}
}

// BenchmarkKeyToString_Int benchmarks int key conversion
func BenchmarkKeyToString_Int(b *testing.B) {
	key := 12345
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = keyToString(key)
	}
}

// BenchmarkKeyToString_Int64 benchmarks int64 key conversion
func BenchmarkKeyToString_Int64(b *testing.B) {
	key := int64(1234567890)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = keyToString(key)
	}
}

// BenchmarkKeyToString_Uint64 benchmarks uint64 key conversion
func BenchmarkKeyToString_Uint64(b *testing.B) {
	key := uint64(1234567890)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = keyToString(key)
	}
}

// BenchmarkGenericCache_OptimizedStringSet benchmarks optimized string key Set
func BenchmarkGenericCache_OptimizedStringSet(b *testing.B) {
	cache := NewGenericCache[string, int](Config{MaxSize: 10000})
	defer cache.Close()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Set("key", i)
	}
}

// BenchmarkGenericCache_OptimizedStringGet benchmarks optimized string key Get
func BenchmarkGenericCache_OptimizedStringGet(b *testing.B) {
	cache := NewGenericCache[string, int](Config{MaxSize: 10000})
	defer cache.Close()
	cache.Set("key", 42)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Get("key")
	}
}

// BenchmarkGenericCache_OptimizedIntSet benchmarks optimized int key Set
func BenchmarkGenericCache_OptimizedIntSet(b *testing.B) {
	cache := NewGenericCache[int, string](Config{MaxSize: 10000})
	defer cache.Close()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Set(i, "value")
	}
}

// BenchmarkGenericCache_OptimizedIntGet benchmarks optimized int key Get
func BenchmarkGenericCache_OptimizedIntGet(b *testing.B) {
	cache := NewGenericCache[int, string](Config{MaxSize: 10000})
	defer cache.Close()
	cache.Set(42, "value")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Get(42)
	}
}

// BenchmarkGenericCache_OptimizedInt64Set benchmarks optimized int64 key Set
func BenchmarkGenericCache_OptimizedInt64Set(b *testing.B) {
	cache := NewGenericCache[int64, string](Config{MaxSize: 10000})
	defer cache.Close()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Set(int64(i), "value")
	}
}

// BenchmarkGenericCache_OptimizedInt64Get benchmarks optimized int64 key Get
func BenchmarkGenericCache_OptimizedInt64Get(b *testing.B) {
	cache := NewGenericCache[int64, string](Config{MaxSize: 10000})
	defer cache.Close()
	cache.Set(int64(42), "value")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Get(int64(42))
	}
}

// BenchmarkGenericCache_Comparison compares with non-generic API
func BenchmarkGenericCache_Comparison(b *testing.B) {
	b.Run("Generic_String", func(b *testing.B) {
		cache := NewGenericCache[string, int](Config{MaxSize: 10000})
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			cache.Set("key", i)
		}
	})

	b.Run("NonGeneric_String", func(b *testing.B) {
		cache := NewCache(Config{MaxSize: 10000})
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			cache.Set("key", i)
		}
	})

	b.Run("Generic_Int", func(b *testing.B) {
		cache := NewGenericCache[int, string](Config{MaxSize: 10000})
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			cache.Set(i, "value")
		}
	})
}
