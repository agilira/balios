// sketch_test.go: unit tests and benchmarks for frequency sketch
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGILira fragment
// SPDX-License-Identifier: MPL-2.0

package balios

import (
	"strconv"
	"testing"
)

func TestNewFrequencySketch(t *testing.T) {
	tests := []struct {
		name    string
		maxSize int
		wantMin int // minimum expected table size
	}{
		{"small size", 100, 64},
		{"medium size", 1000, 64},
		{"large size", 10000, 256},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sketch := newFrequencySketch(tt.maxSize)

			if len(sketch.table) < tt.wantMin {
				t.Errorf("table size %d < minimum %d", len(sketch.table), tt.wantMin)
			}

			// Table size should be power of 2
			tableSize := len(sketch.table)
			if tableSize&(tableSize-1) != 0 {
				t.Errorf("table size %d is not power of 2", tableSize)
			}

			// tableMask should be tableSize - 1
			if sketch.tableMask != uint64(tableSize-1) {
				t.Errorf("tableMask %d != %d", sketch.tableMask, tableSize-1)
			}
		})
	}
}

func TestNextPowerOf2(t *testing.T) {
	tests := []struct {
		input    int
		expected int
	}{
		{0, 1},
		{1, 1},
		{2, 2},
		{3, 4},
		{4, 4},
		{5, 8},
		{8, 8},
		{9, 16},
		{15, 16},
		{16, 16},
		{17, 32},
		{1000, 1024},
	}

	for _, tt := range tests {
		t.Run(strconv.Itoa(tt.input), func(t *testing.T) {
			got := nextPowerOf2(tt.input)
			if got != tt.expected {
				t.Errorf("nextPowerOf2(%d) = %d, want %d", tt.input, got, tt.expected)
			}
		})
	}
}

func TestFrequencySketch_IncrementAndEstimate(t *testing.T) {
	sketch := newFrequencySketch(1000)

	// Test single increment
	keyHash := stringHash("test-key")

	// Initial estimate should be 0
	if est := sketch.estimate(keyHash); est != 0 {
		t.Errorf("initial estimate = %d, want 0", est)
	}

	// Increment and check
	sketch.increment(keyHash)
	if est := sketch.estimate(keyHash); est == 0 {
		t.Errorf("estimate after increment = %d, want > 0", est)
	}

	// Multiple increments should increase estimate
	for i := 0; i < 5; i++ {
		sketch.increment(keyHash)
	}

	finalEst := sketch.estimate(keyHash)
	if finalEst == 0 {
		t.Errorf("estimate after multiple increments = %d, want > 0", finalEst)
	}
}

func TestFrequencySketch_SaturationAt15(t *testing.T) {
	sketch := newFrequencySketch(1000)
	keyHash := stringHash("saturation-test")

	// Increment many times to test saturation
	for i := 0; i < 100; i++ {
		sketch.increment(keyHash)
	}

	est := sketch.estimate(keyHash)
	if est > 15 {
		t.Errorf("estimate %d > 15, counters should saturate at 15", est)
	}
}

func TestFrequencySketch_DifferentKeys(t *testing.T) {
	sketch := newFrequencySketch(1000)

	keys := []string{"key1", "key2", "key3", "different-key", "another-one"}
	hashes := make([]uint64, len(keys))

	// Hash all keys
	for i, key := range keys {
		hashes[i] = stringHash(key)
	}

	// Increment each key a different number of times
	for i, hash := range hashes {
		for j := 0; j <= i; j++ {
			sketch.increment(hash)
		}
	}

	// Verify estimates are reasonable
	prevEst := uint64(0)
	for i, hash := range hashes {
		est := sketch.estimate(hash)

		// Each key should have at least some estimate
		if est == 0 && i > 0 {
			t.Errorf("key %d estimate = 0, expected > 0", i)
		}

		// Generally, more increments should give higher or equal estimates
		// (not always true due to Count-Min Sketch properties, but should trend upward)
		if i > 0 && est < prevEst {
			// This is not necessarily an error due to hash collisions,
			// but let's log it for debugging
			t.Logf("key %d estimate %d < previous %d (hash collisions possible)", i, est, prevEst)
		}

		prevEst = est
	}
}

func TestStringHash(t *testing.T) {
	tests := []struct {
		input string
	}{
		{""},
		{"a"},
		{"test"},
		{"hello world"},
		{"this is a longer string for testing"},
		{"unicode: 你好世界"},
	}

	// Test that hash function is deterministic
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			hash1 := stringHash(tt.input)
			hash2 := stringHash(tt.input)

			if hash1 != hash2 {
				t.Errorf("hash not deterministic: %d != %d", hash1, hash2)
			}
		})
	}

	// Test that different strings produce different hashes (usually)
	hash1 := stringHash("string1")
	hash2 := stringHash("string2")

	if hash1 == hash2 {
		t.Logf("collision detected (expected to be rare): %s and %s both hash to %d", "string1", "string2", hash1)
	}
}

func TestFrequencySketch_Reset(t *testing.T) {
	sketch := newFrequencySketch(1000)
	keyHash := stringHash("reset-test")

	// Increment several times
	for i := 0; i < 8; i++ {
		sketch.increment(keyHash)
	}

	estBefore := sketch.estimate(keyHash)
	if estBefore == 0 {
		t.Fatalf("estimate before reset = 0, expected > 0")
	}

	// Force reset
	sketch.reset()

	estAfter := sketch.estimate(keyHash)

	// After reset, estimate should be roughly half (due to halving in reset)
	// Allow some variance due to the nature of the data structure
	if estAfter > estBefore {
		t.Errorf("estimate after reset %d > before reset %d", estAfter, estBefore)
	}
}

func TestMin4(t *testing.T) {
	tests := []struct {
		a, b, c, d uint64
		want       uint64
	}{
		{1, 2, 3, 4, 1},
		{4, 3, 2, 1, 1},
		{2, 1, 4, 3, 1},
		{5, 5, 5, 5, 5},
		{0, 10, 20, 30, 0},
		{15, 14, 13, 12, 12},
	}

	for _, tt := range tests {
		got := min4(tt.a, tt.b, tt.c, tt.d)
		if got != tt.want {
			t.Errorf("min4(%d, %d, %d, %d) = %d, want %d", tt.a, tt.b, tt.c, tt.d, got, tt.want)
		}
	}
}

// Benchmark tests
func BenchmarkFrequencySketch_Increment(b *testing.B) {
	sketch := newFrequencySketch(10000)
	keyHashes := make([]uint64, 1000)

	// Pre-compute hashes to avoid measuring hash time
	for i := range keyHashes {
		keyHashes[i] = stringHash("key" + strconv.Itoa(i))
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		sketch.increment(keyHashes[i%len(keyHashes)])
	}
}

func BenchmarkFrequencySketch_Estimate(b *testing.B) {
	sketch := newFrequencySketch(10000)
	keyHashes := make([]uint64, 1000)

	// Pre-compute hashes and populate sketch
	for i := range keyHashes {
		keyHashes[i] = stringHash("key" + strconv.Itoa(i))
		sketch.increment(keyHashes[i])
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		sketch.estimate(keyHashes[i%len(keyHashes)])
	}
}

func BenchmarkStringHash(b *testing.B) {
	keys := []string{
		"short",
		"medium-length-key",
		"this-is-a-very-long-key-for-testing-hash-performance",
	}

	for _, key := range keys {
		b.Run(key, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				stringHash(key)
			}
		})
	}
}
