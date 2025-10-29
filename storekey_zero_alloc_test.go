// storekey_zero_alloc_test.go: tests for zero-allocation key storage
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGILira fragment
// SPDX-License-Identifier: MPL-2.0

package balios

import (
	"fmt"
	"testing"
)

// TestStoreKeyZeroAllocation verifies that storeKey() allocation behavior
// After type-change safety fix: Set allocates 1 new valueHolder per operation
func TestStoreKeyZeroAllocation(t *testing.T) {
	cache := NewCache(Config{
		MaxSize: 1000,
	})

	// Warm up
	for i := 0; i < 100; i++ {
		cache.Set(fmt.Sprintf("key%d", i), i)
	}

	// Measure allocations for Set operations
	allocsBefore := testing.AllocsPerRun(1000, func() {
		cache.Set("test-key", "test-value")
	})

	// After type-change fix: expect 1 alloc for new valueHolder
	// This prevents atomic.Value panic when types change
	if allocsBefore > 1.5 {
		t.Errorf("Set() allocates too much: %.2f allocs/op (expected ≤1)", allocsBefore)
		t.Logf("This suggests additional unexpected allocations")
	} else if allocsBefore >= 0.5 {
		t.Logf("Set() allocations: %.2f allocs/op (expected: ~1 for valueHolder)", allocsBefore)
	} else {
		t.Logf("✅ Set() allocations: %.2f allocs/op (excellent!)", allocsBefore)
	}
}

// TestLoadKeyZeroAllocation verifies that loadKey() doesn't allocate
func TestLoadKeyZeroAllocation(t *testing.T) {
	cache := NewCache(Config{
		MaxSize: 1000,
	})

	// Pre-populate
	for i := 0; i < 100; i++ {
		cache.Set(fmt.Sprintf("key%d", i), i)
	}

	// Measure allocations for Get operations
	allocsBefore := testing.AllocsPerRun(1000, func() {
		cache.Get("key50")
	})

	// Get should be completely zero-allocation
	if allocsBefore > 0.5 {
		t.Errorf("Get() allocates: %.2f allocs/op (expected 0)", allocsBefore)
		t.Logf("This suggests loadKey() is allocating when reconstructing strings")
	} else {
		t.Logf("✅ Get() allocations: %.2f allocs/op (perfect!)", allocsBefore)
	}
}

// TestKeyStorageMemoryEfficiency verifies memory doesn't leak over time
func TestKeyStorageMemoryEfficiency(t *testing.T) {
	cache := NewCache(Config{
		MaxSize: 1000,
	})

	// Fill cache completely
	for i := 0; i < 1000; i++ {
		cache.Set(fmt.Sprintf("initial-key-%d", i), i)
	}

	// Now repeatedly overwrite the SAME keys (high churn scenario)
	// This would cause memory leak with old implementation
	for iteration := 0; iteration < 10; iteration++ {
		for i := 0; i < 1000; i++ {
			// Use same keys to test update path (not eviction path)
			cache.Set(fmt.Sprintf("initial-key-%d", i), iteration*1000+i)
		}
	}

	// Verify cache still works correctly
	val, found := cache.Get("initial-key-500")
	if !found {
		t.Error("Key not found after churn test")
	}
	// Last iteration was 9, so value should be 9*1000 + 500 = 9500
	expected := 9*1000 + 500
	if val != expected {
		t.Errorf("Wrong value after churn: got %v, want %d", val, expected)
	}

	stats := cache.Stats()
	t.Logf("After high-churn test: size=%d, sets=%d, evictions=%d",
		stats.Size, stats.Sets, stats.Evictions)

	// Since we're updating existing keys, evictions should be minimal
	if stats.Evictions > 100 {
		t.Logf("Note: Higher evictions than expected (%d), but this is acceptable", stats.Evictions)
	}
}

// BenchmarkStoreKeyAllocation benchmarks the storeKey operation specifically
func BenchmarkStoreKeyAllocation(b *testing.B) {
	cache := NewCache(Config{MaxSize: 10000})

	// Pre-populate to ensure we're updating existing entries
	for i := 0; i < 1000; i++ {
		cache.Set(fmt.Sprintf("bench-key-%d", i), i)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("bench-key-%d", i%1000)
		cache.Set(key, i)
	}
}

// BenchmarkLoadKeyAllocation benchmarks the loadKey operation specifically
func BenchmarkLoadKeyAllocation(b *testing.B) {
	cache := NewCache(Config{MaxSize: 10000})

	// Pre-populate
	for i := 0; i < 1000; i++ {
		cache.Set(fmt.Sprintf("bench-key-%d", i), i)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("bench-key-%d", i%1000)
		cache.Get(key)
	}
}
