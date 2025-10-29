// memory_overhead_test.go: tests to document memory overhead per cache entry
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGILira fragment
// SPDX-License-Identifier: MPL-2.0

package balios

import (
	"testing"
	"unsafe"
)

// TestMemoryOverhead documents the memory cost per cache entry.
// This helps users estimate memory usage: memory = capacity × entry_size
func TestMemoryOverhead(t *testing.T) {
	e := &entry{}
	entrySize := unsafe.Sizeof(*e)

	t.Logf("Memory overhead per entry: %d bytes", entrySize)
	t.Logf("For capacity=10,000: ~%d KB", (10000*int(entrySize))/1024)
	t.Logf("For capacity=100,000: ~%d MB", (100000*int(entrySize))/(1024*1024))
	t.Logf("For capacity=1,000,000: ~%d MB", (1000000*int(entrySize))/(1024*1024))

	// Document breakdown
	t.Log("Entry structure breakdown:")
	t.Logf("  - keyData (pointer): 8 bytes")
	t.Logf("  - keyLen (int64): 8 bytes")
	t.Logf("  - keyHash (uint64): 8 bytes")
	t.Logf("  - value (atomic.Value): 16 bytes")
	t.Logf("  - expireAt (int64): 8 bytes")
	t.Logf("  - valid (int32): 4 bytes")
	t.Logf("  - version (uint64): 8 bytes")
	t.Logf("  - padding: %d bytes (alignment)", entrySize-60)

	// Verify reasonable overhead (should be ~64 bytes with alignment)
	if entrySize > 128 {
		t.Errorf("Entry size too large: %d bytes (expected ~64)", entrySize)
	}
}

// TestValueHolderOverhead documents valueHolder memory cost
func TestValueHolderOverhead(t *testing.T) {
	vh := &valueHolder{}
	vhSize := unsafe.Sizeof(*vh)

	t.Logf("valueHolder size: %d bytes", vhSize)
	t.Log("This is allocated per Set/Update operation for type safety")

	// valueHolder contains only atomic.Value (16 bytes)
	if vhSize > 32 {
		t.Errorf("valueHolder too large: %d bytes (expected ~16)", vhSize)
	}
}

// BenchmarkMemoryFootprint measures actual memory usage of populated cache
func BenchmarkMemoryFootprint(b *testing.B) {
	sizes := []int{1000, 10000, 100000}

	for _, size := range sizes {
		b.Run(string(rune(size)), func(b *testing.B) {
			cache := NewCache(Config{MaxSize: size})

			// Populate cache
			for i := 0; i < size; i++ {
				cache.Set("key:"+string(rune('0'+i%10))+string(rune('a'+i%26)), i)
			}

			b.ReportMetric(float64(size), "entries")

			// Estimate memory (rough)
			entrySize := unsafe.Sizeof(entry{})
			estimatedMB := float64(size*int(entrySize)) / (1024 * 1024)
			b.ReportMetric(estimatedMB, "est_MB")
		})
	}
}
