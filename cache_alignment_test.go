// cache_alignment_test.go: tests for struct field alignment safety
//
// These tests verify that atomic 64-bit fields are properly aligned on both
// 32-bit and 64-bit architectures, preventing runtime panics.
//
// Copyright (c) 2025 AGILira - A. Giordano
// SPDX-License-Identifier: MPL-2.0

package balios

import (
	"testing"
	"unsafe"
)

// TestEntryAlignment verifies that all atomic 64-bit fields in entry struct
// are properly aligned to 8-byte boundaries on all architectures.
//
// On 32-bit architectures, atomic operations on misaligned 64-bit values
// will panic. This test ensures the struct layout prevents such issues.
func TestEntryAlignment(t *testing.T) {
	var e entry

	// Get base address of struct
	baseAddr := uintptr(unsafe.Pointer(&e))

	// Check alignment of all atomic 64-bit fields
	tests := []struct {
		name  string
		addr  uintptr
		field string
	}{
		{"version", uintptr(unsafe.Pointer(&e.version)), "version uint64"},
		{"keyLen", uintptr(unsafe.Pointer(&e.keyLen)), "keyLen int64"},
		{"keyHash", uintptr(unsafe.Pointer(&e.keyHash)), "keyHash uint64"},
		{"expireAt", uintptr(unsafe.Pointer(&e.expireAt)), "expireAt int64"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			offset := tt.addr - baseAddr

			// 64-bit fields must be 8-byte aligned
			if offset%8 != 0 {
				t.Errorf("Field %s at offset %d is not 8-byte aligned (offset %% 8 = %d)",
					tt.field, offset, offset%8)
				t.Errorf("This will cause panic on 32-bit architectures!")
			} else {
				t.Logf("✓ Field %s at offset %d is properly aligned", tt.field, offset)
			}
		})
	}
}

// TestEntrySize verifies the entry struct size is as expected
func TestEntrySize(t *testing.T) {
	var e entry
	size := unsafe.Sizeof(e)

	// Log struct size for documentation
	t.Logf("entry struct size: %d bytes", size)

	// On 64-bit: expect reasonable size (not bloated)
	// This is mainly for documentation/monitoring
	if size > 128 {
		t.Logf("Warning: entry struct is large (%d bytes), consider optimization", size)
	}
}

// TestEntryFieldOffsets documents the memory layout of entry struct
func TestEntryFieldOffsets(t *testing.T) {
	var e entry
	baseAddr := uintptr(unsafe.Pointer(&e))

	t.Logf("=== Entry Struct Memory Layout ===")
	t.Logf("Base address: 0x%x", baseAddr)

	offsets := []struct {
		name string
		addr uintptr
		size uintptr
	}{
		{"version", uintptr(unsafe.Pointer(&e.version)), unsafe.Sizeof(e.version)},
		{"keyData", uintptr(unsafe.Pointer(&e.keyData)), unsafe.Sizeof(e.keyData)},
		{"keyLen", uintptr(unsafe.Pointer(&e.keyLen)), unsafe.Sizeof(e.keyLen)},
		{"value", uintptr(unsafe.Pointer(&e.value)), unsafe.Sizeof(e.value)},
		{"keyHash", uintptr(unsafe.Pointer(&e.keyHash)), unsafe.Sizeof(e.keyHash)},
		{"expireAt", uintptr(unsafe.Pointer(&e.expireAt)), unsafe.Sizeof(e.expireAt)},
		{"valid", uintptr(unsafe.Pointer(&e.valid)), unsafe.Sizeof(e.valid)},
	}

	for _, field := range offsets {
		offset := field.addr - baseAddr
		t.Logf("  %-12s: offset=%3d size=%2d align=%s",
			field.name, offset, field.size,
			alignmentStatus(offset))
	}

	t.Logf("Total struct size: %d bytes", unsafe.Sizeof(e))
}

func alignmentStatus(offset uintptr) string {
	if offset%8 == 0 {
		return "8-byte ✓"
	} else if offset%4 == 0 {
		return "4-byte"
	}
	return "misaligned ✗"
}

// BenchmarkEntryAccess ensures alignment doesn't hurt performance
func BenchmarkEntryAccess(b *testing.B) {
	var e entry

	b.Run("WriteAtomic64", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			e.storeKey("benchmark-key")
		}
	})

	b.Run("ReadAtomic64", func(b *testing.B) {
		e.storeKey("benchmark-key")
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = e.loadKey()
		}
	})
}
