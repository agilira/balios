// string_clone_reality_check_test.go: Verifica l'impatto REALE di strings.Clone()
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGILira fragment
// SPDX-License-Identifier: MPL-2.0

package balios

import (
	"fmt"
	"testing"
)

// TestSetNewKeysAllocations misura le allocazioni quando si inseriscono NUOVE chiavi
// (non aggiornamenti di chiavi esistenti come fa TestStoreKeyZeroAllocation)
func TestSetNewKeysAllocations(t *testing.T) {
	cache := NewCache(Config{
		MaxSize: 100000, // Large enough to avoid evictions
	})

	// Measure allocations for Set operations with NEW keys each time
	keyIndex := 0
	allocsBefore := testing.AllocsPerRun(1000, func() {
		// Generate a UNIQUE key each iteration
		key := fmt.Sprintf("unique-key-%d", keyIndex)
		keyIndex++
		cache.Set(key, "test-value")
	})

	t.Logf("Set() with NEW keys: %.2f allocs/op", allocsBefore)

	// With strings.Clone(), we expect:
	// - 1 alloc for strings.Clone() (key copy)
	// - 1 alloc for valueHolder
	// - 1 alloc for fmt.Sprintf (key generation)
	// Total: ~3 allocs/op

	if allocsBefore < 2 {
		t.Logf("✅ Excellent: < 2 allocs/op (strings.Clone() may be optimized out)")
	} else if allocsBefore <= 3.5 {
		t.Logf("✅ Expected: ~3 allocs/op (strings.Clone() + valueHolder + fmt)")
	} else {
		t.Logf("⚠️  Higher than expected: %.2f allocs/op", allocsBefore)
	}
}

// TestSetExistingKeysAllocations misura le allocazioni quando si aggiornano chiavi esistenti
// Con il fix per il type-change bug, ora alloca 1 valueHolder per update
func TestSetExistingKeysAllocations(t *testing.T) {
	cache := NewCache(Config{
		MaxSize: 1000,
	})

	// Pre-populate with keys (using string values)
	for i := 0; i < 100; i++ {
		cache.Set(fmt.Sprintf("key-%d", i), "value")
	}

	// Measure allocations for updates to EXISTING keys
	allocsBefore := testing.AllocsPerRun(1000, func() {
		cache.Set("key-50", "updated-value")
	})

	t.Logf("Set() with EXISTING keys: %.2f allocs/op", allocsBefore)

	// After type-change fix, we allocate 1 new valueHolder per update
	// This prevents atomic.Value panic when types change
	if allocsBefore > 1.5 {
		t.Logf("⚠️  Higher than expected: %.2f allocs/op (expected ~1 for valueHolder)", allocsBefore)
	} else if allocsBefore >= 0.5 {
		t.Logf("✅ Expected: ~1 alloc/op (new valueHolder for type-change safety)")
	} else {
		t.Logf("✅ Excellent: %.2f allocs/op", allocsBefore)
	}
}

// BenchmarkSetNewKeysWithAllocs benchmarks Set() with new unique keys
func BenchmarkSetNewKeysWithAllocs(b *testing.B) {
	cache := NewCache(Config{MaxSize: 1000000})

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Each iteration uses a UNIQUE key
		key := fmt.Sprintf("bench-key-%d", i)
		cache.Set(key, i)
	}
}

// BenchmarkSetExistingKeysWithAllocs benchmarks Set() updates (hot path)
func BenchmarkSetExistingKeysWithAllocs(b *testing.B) {
	cache := NewCache(Config{MaxSize: 10000})

	// Pre-populate with 1000 keys
	for i := 0; i < 1000; i++ {
		cache.Set(fmt.Sprintf("bench-key-%d", i), i)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Reuse existing keys (hot path)
		key := fmt.Sprintf("bench-key-%d", i%1000)
		cache.Set(key, i)
	}
}
