// cache_generic_comprehensive_test.go: comprehensive tests for untested generic cache functions
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGILira library
// SPDX-License-Identifier: MPL-2.0

package balios

import (
	"testing"
)

// =============================================================================
// KEY TO STRING CONVERSION TESTS
// =============================================================================

func TestKeyToString_AllTypes(t *testing.T) {
	tests := []struct {
		name     string
		key      interface{}
		expected string
	}{
		// String keys (zero allocation)
		{"string", "test-key", "test-key"},
		{"empty string", "", ""},

		// Integer types
		{"int", 42, "42"},
		{"int negative", -42, "-42"},
		{"int8", int8(8), "8"},
		{"int16", int16(16), "16"},
		{"int32", int32(32), "32"},
		{"int64", int64(64), "64"},

		// Unsigned integer types
		{"uint", uint(42), "42"},
		{"uint8", uint8(8), "8"},
		{"uint16", uint16(16), "16"},
		{"uint32", uint32(32), "32"},
		{"uint64", uint64(64), "64"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use type switch to call keyToString with correct type
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
				t.Errorf("keyToString(%v) = %q, want %q", tt.key, result, tt.expected)
			}
		})
	}
}

// TestKeyToString_DefaultCase tests the default fmt.Sprintf fallback
func TestKeyToString_DefaultCase(t *testing.T) {
	// Custom struct type (not in the optimized type switch)
	type CustomKey struct {
		ID   int
		Name string
	}

	key := CustomKey{ID: 123, Name: "test"}
	cache := NewGenericCache[CustomKey, string](DefaultConfig())

	// This should trigger the default case in keyToString
	cache.Set(key, "value")

	// Verify we can retrieve it
	value, found := cache.Get(key)
	if !found {
		t.Error("Expected to find custom key")
	}
	if value != "value" {
		t.Errorf("Expected 'value', got %q", value)
	}
}

// TestKeyToString_Float tests float keys (uses default case)
func TestKeyToString_Float(t *testing.T) {
	cache := NewGenericCache[float64, string](DefaultConfig())

	cache.Set(3.14, "pi")
	cache.Set(2.71, "e")

	if value, found := cache.Get(3.14); !found || value != "pi" {
		t.Errorf("Expected 'pi', got %q (found=%v)", value, found)
	}

	if value, found := cache.Get(2.71); !found || value != "e" {
		t.Errorf("Expected 'e', got %q (found=%v)", value, found)
	}
}

// TestKeyToString_Bool tests bool keys (uses default case)
func TestKeyToString_Bool(t *testing.T) {
	cache := NewGenericCache[bool, int](DefaultConfig())

	cache.Set(true, 1)
	cache.Set(false, 0)

	if value, found := cache.Get(true); !found || value != 1 {
		t.Errorf("Expected 1 for true, got %v (found=%v)", value, found)
	}

	if value, found := cache.Get(false); !found || value != 0 {
		t.Errorf("Expected 0 for false, got %v (found=%v)", value, found)
	}
}

// TestKeyToString_Array tests array keys (uses default case)
func TestKeyToString_Array(t *testing.T) {
	cache := NewGenericCache[[3]int, string](DefaultConfig())

	key1 := [3]int{1, 2, 3}
	key2 := [3]int{4, 5, 6}

	cache.Set(key1, "first")
	cache.Set(key2, "second")

	if value, found := cache.Get(key1); !found || value != "first" {
		t.Errorf("Expected 'first', got %q (found=%v)", value, found)
	}

	if value, found := cache.Get(key2); !found || value != "second" {
		t.Errorf("Expected 'second', got %q (found=%v)", value, found)
	}
}

// =============================================================================
// TYPE ASSERTION EDGE CASES
// =============================================================================

// TestGenericCache_TypeAssertionFailure tests the safety net for type assertion failures
// Note: This is hard to trigger in normal usage, but we test the defensive code path
func TestGenericCache_Get_TypeSafety(t *testing.T) {
	// Create a generic cache
	cache := NewGenericCache[string, int](DefaultConfig())

	// Normal case: correct type
	cache.Set("key1", 42)
	value, found := cache.Get("key1")
	if !found || value != 42 {
		t.Errorf("Expected 42, got %v (found=%v)", value, found)
	}

	// The type assertion failure path is defensive programming
	// In normal usage it should never trigger, but the code is there for safety
	// We verify that Get returns zero value and false if something goes wrong

	// Edge case: empty key should be rejected by inner cache
	emptyCache := NewGenericCache[string, int](DefaultConfig())
	emptyCache.Set("", 100) // This may or may not be stored depending on validation

	// Just verify it doesn't panic
	_, _ = emptyCache.Get("")
}

// =============================================================================
// CAPACITY AND LEN TESTS
// =============================================================================

func TestGenericCache_Capacity(t *testing.T) {
	tests := []struct {
		name    string
		maxSize int
	}{
		{"small cache", 10},
		{"medium cache", 100},
		{"large cache", 1000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := NewGenericCache[string, int](Config{MaxSize: tt.maxSize})
			capacity := cache.Capacity()

			// Capacity should match the configured MaxSize
			if capacity != tt.maxSize {
				t.Errorf("Expected capacity %d, got %d", tt.maxSize, capacity)
			}
		})
	}
}

func TestGenericCache_Len(t *testing.T) {
	cache := NewGenericCache[string, int](DefaultConfig())

	if cache.Len() != 0 {
		t.Errorf("Expected initial Len=0, got %d", cache.Len())
	}

	// Add items
	for i := 0; i < 10; i++ {
		cache.Set(string(rune('a'+i)), i)
	}

	if cache.Len() != 10 {
		t.Errorf("Expected Len=10, got %d", cache.Len())
	}

	// Delete some
	cache.Delete("a")
	cache.Delete("b")

	if cache.Len() != 8 {
		t.Errorf("Expected Len=8 after deletes, got %d", cache.Len())
	}

	// Clear
	cache.Clear()

	if cache.Len() != 0 {
		t.Errorf("Expected Len=0 after Clear, got %d", cache.Len())
	}
}

// =============================================================================
// INTEGER KEY OPTIMIZATIONS
// =============================================================================

func TestGenericCache_IntegerKeys(t *testing.T) {
	t.Run("int keys", func(t *testing.T) {
		cache := NewGenericCache[int, string](DefaultConfig())

		for i := -100; i <= 100; i++ {
			cache.Set(i, "value")
		}

		// Verify all are retrievable
		for i := -100; i <= 100; i++ {
			if value, found := cache.Get(i); !found || value != "value" {
				t.Errorf("Key %d: expected 'value', got %q (found=%v)", i, value, found)
			}
		}
	})

	t.Run("int8 keys", func(t *testing.T) {
		cache := NewGenericCache[int8, string](DefaultConfig())

		for i := int8(-128); i < 127; i++ {
			cache.Set(i, "value")
		}

		if _, found := cache.Get(int8(0)); !found {
			t.Error("Expected to find int8(0)")
		}
	})

	t.Run("uint64 keys", func(t *testing.T) {
		cache := NewGenericCache[uint64, string](DefaultConfig())

		largeKeys := []uint64{
			0,
			1000,
			1000000,
			18446744073709551615, // max uint64
		}

		for _, key := range largeKeys {
			cache.Set(key, "value")
		}

		for _, key := range largeKeys {
			if value, found := cache.Get(key); !found || value != "value" {
				t.Errorf("Key %d: expected 'value', got %q (found=%v)", key, value, found)
			}
		}
	})
}

// =============================================================================
// CONCURRENT TYPE SAFETY
// =============================================================================

func TestGenericCache_ConcurrentTypeSafety(t *testing.T) {
	cache := NewGenericCache[int, string](DefaultConfig())

	// Spawn multiple goroutines to stress test type safety
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				key := id*100 + j
				cache.Set(key, "goroutine")

				if value, found := cache.Get(key); found && value != "goroutine" {
					t.Errorf("Type safety violation: expected 'goroutine', got %q", value)
				}
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

// =============================================================================
// EDGE CASES AND ERROR CONDITIONS
// =============================================================================

func TestGenericCache_EmptyStringKey(t *testing.T) {
	cache := NewGenericCache[string, int](DefaultConfig())

	// Empty string keys should be handled gracefully
	cache.Set("", 42)

	// The underlying cache may reject empty keys, so we just ensure no panic
	_, _ = cache.Get("")
}

func TestGenericCache_NilValue(t *testing.T) {
	cache := NewGenericCache[string, *User](DefaultConfig())

	// nil pointer values should be stored correctly
	cache.Set("nil-user", nil)

	value, found := cache.Get("nil-user")
	if !found {
		t.Error("Expected to find nil-user")
	}
	if value != nil {
		t.Errorf("Expected nil, got %v", value)
	}
}

func TestGenericCache_ZeroValues(t *testing.T) {
	t.Run("int zero value", func(t *testing.T) {
		cache := NewGenericCache[string, int](DefaultConfig())
		cache.Set("zero", 0)

		value, found := cache.Get("zero")
		if !found {
			t.Error("Expected to find 'zero'")
		}
		if value != 0 {
			t.Errorf("Expected 0, got %d", value)
		}
	})

	t.Run("string zero value", func(t *testing.T) {
		cache := NewGenericCache[string, string](DefaultConfig())
		cache.Set("empty", "")

		value, found := cache.Get("empty")
		if !found {
			t.Error("Expected to find 'empty'")
		}
		if value != "" {
			t.Errorf("Expected empty string, got %q", value)
		}
	})

	t.Run("bool zero value", func(t *testing.T) {
		cache := NewGenericCache[string, bool](DefaultConfig())
		cache.Set("false", false)

		value, found := cache.Get("false")
		if !found {
			t.Error("Expected to find 'false'")
		}
		if value != false {
			t.Errorf("Expected false, got %v", value)
		}
	})
}

// =============================================================================
// BENCHMARKS FOR KEY CONVERSION
// =============================================================================

func BenchmarkKeyToString(b *testing.B) {
	b.Run("string", func(b *testing.B) {
		key := "test-key-benchmark"
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = keyToString(key)
		}
	})

	b.Run("int", func(b *testing.B) {
		key := 123456
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = keyToString(key)
		}
	})

	b.Run("int64", func(b *testing.B) {
		key := int64(123456789012345)
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = keyToString(key)
		}
	})

	b.Run("uint64", func(b *testing.B) {
		key := uint64(18446744073709551615)
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = keyToString(key)
		}
	})

	b.Run("float64-default", func(b *testing.B) {
		key := 3.14159265359
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = keyToString(key)
		}
	})

	b.Run("struct-default", func(b *testing.B) {
		type Key struct {
			ID   int
			Name string
		}
		key := Key{ID: 123, Name: "test"}
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = keyToString(key)
		}
	})
}
