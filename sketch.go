// sketch.go: core lock-free W-TinyLFU cache implementation
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGILira library
// SPDX-License-Identifier: MPL-2.0

package balios

import (
	"sync/atomic"
	"unsafe"
)

// frequencySketch implements a Count-Min Sketch with 4-bit counters
// for tracking access frequency in W-TinyLFU algorithm.
// This implementation is lock-free and zero-allocation on hot path.
type frequencySketch struct {
	// table stores 4-bit counters packed into uint64 values
	// Each uint64 holds 16 counters (64 bits / 4 bits per counter)
	table []uint64

	// tableMask is used for fast modulo operation (table size must be power of 2)
	tableMask uint64

	// seeds for hash functions (4 different hash functions)
	seed1, seed2, seed3, seed4 uint64

	// sampleSize tracks number of operations for periodic reset
	sampleSize int64

	// resetThreshold defines when to reset counters (aging)
	resetThreshold int64
}

// newFrequencySketch creates a new frequency sketch with the given maximum size.
// The table size is set to the next power of 2 that fits maxSize entries.
func newFrequencySketch(maxSize int) *frequencySketch {
	// Calculate table size as next power of 2
	tableSize := nextPowerOf2(maxSize / 4) // Each uint64 holds 16 counters, so divide by 4 for approximation
	if tableSize < 64 {
		tableSize = 64 // Minimum size
	}

	return &frequencySketch{
		table:          make([]uint64, tableSize),
		tableMask:      uint64(tableSize - 1), // #nosec G115 - tableSize is power of 2, bounded and safe
		seed1:          0x9e3779b97f4a7c15,    // Golden ratio hash seeds
		seed2:          0xbf58476d1ce4e5b9,
		seed3:          0x94d049bb133111eb,
		seed4:          0xbf58476d1ce4e5b7,
		resetThreshold: int64(maxSize * 10), // Reset after 10x maxSize operations
	}
}

// nextPowerOf2 returns the next power of 2 greater than or equal to n.
func nextPowerOf2(n int) int {
	if n <= 1 {
		return 1
	}
	n--
	n |= n >> 1
	n |= n >> 2
	n |= n >> 4
	n |= n >> 8
	n |= n >> 16
	n |= n >> 32
	return n + 1
}

// increment increments the frequency counter for the given key.
// This method is lock-free and allocation-free.
func (s *frequencySketch) increment(keyHash uint64) {
	// Check if we need to reset (aging mechanism)
	if atomic.AddInt64(&s.sampleSize, 1)%s.resetThreshold == 0 {
		s.reset()
	}

	// Get 4 different positions using different hash functions
	pos1 := s.hash1(keyHash) & s.tableMask
	pos2 := s.hash2(keyHash) & s.tableMask
	pos3 := s.hash3(keyHash) & s.tableMask
	pos4 := s.hash4(keyHash) & s.tableMask

	// Calculate sub-positions within each uint64 (0-15)
	subPos1 := (keyHash & 0xF) * 4 // 4 bits per counter
	subPos2 := ((keyHash >> 4) & 0xF) * 4
	subPos3 := ((keyHash >> 8) & 0xF) * 4
	subPos4 := ((keyHash >> 12) & 0xF) * 4

	// Increment counters atomically, with saturation at 15 (4-bit max)
	s.incrementCounter(pos1, subPos1)
	s.incrementCounter(pos2, subPos2)
	s.incrementCounter(pos3, subPos3)
	s.incrementCounter(pos4, subPos4)
}

// incrementCounter atomically increments a 4-bit counter within a uint64.
func (s *frequencySketch) incrementCounter(tablePos, subPos uint64) {
	mask := uint64(0xF) << subPos // 4-bit mask at the right position

	for {
		old := atomic.LoadUint64(&s.table[tablePos])
		counter := (old >> subPos) & 0xF

		// Saturate at 15 (maximum value for 4-bit counter)
		if counter >= 15 {
			return
		}

		// Increment the counter
		new := (old & ^mask) | ((counter + 1) << subPos)

		if atomic.CompareAndSwapUint64(&s.table[tablePos], old, new) {
			return
		}
		// CAS failed, retry
	}
}

// estimate returns the estimated frequency for the given key.
// Returns the minimum of the 4 hash positions (Count-Min Sketch property).
func (s *frequencySketch) estimate(keyHash uint64) uint64 {
	// Get 4 different positions using different hash functions
	pos1 := s.hash1(keyHash) & s.tableMask
	pos2 := s.hash2(keyHash) & s.tableMask
	pos3 := s.hash3(keyHash) & s.tableMask
	pos4 := s.hash4(keyHash) & s.tableMask

	// Calculate sub-positions within each uint64 (0-15)
	subPos1 := (keyHash & 0xF) * 4 // 4 bits per counter
	subPos2 := ((keyHash >> 4) & 0xF) * 4
	subPos3 := ((keyHash >> 8) & 0xF) * 4
	subPos4 := ((keyHash >> 12) & 0xF) * 4

	// Read counters
	count1 := (atomic.LoadUint64(&s.table[pos1]) >> subPos1) & 0xF
	count2 := (atomic.LoadUint64(&s.table[pos2]) >> subPos2) & 0xF
	count3 := (atomic.LoadUint64(&s.table[pos3]) >> subPos3) & 0xF
	count4 := (atomic.LoadUint64(&s.table[pos4]) >> subPos4) & 0xF

	// Return minimum (Count-Min Sketch property)
	return min4(count1, count2, count3, count4)
}

// reset performs aging by halving all counters.
// This prevents counters from becoming stale.
func (s *frequencySketch) reset() {
	for i := range s.table {
		for {
			old := atomic.LoadUint64(&s.table[i])

			// Halve each 4-bit counter
			new := uint64(0)
			for j := 0; j < 16; j++ { // 16 counters per uint64
				shift := uint64(j * 4) // #nosec G115 - j is bounded 0-15, multiplication is safe
				counter := (old >> shift) & 0xF
				new |= (counter >> 1) << shift // Halve and store
			}

			if atomic.CompareAndSwapUint64(&s.table[i], old, new) {
				break
			}
		}
	}
}

// Hash functions using multiplication method for good distribution
func (s *frequencySketch) hash1(key uint64) uint64 {
	return (key * s.seed1) >> 32
}

func (s *frequencySketch) hash2(key uint64) uint64 {
	return (key * s.seed2) >> 32
}

func (s *frequencySketch) hash3(key uint64) uint64 {
	return (key * s.seed3) >> 32
}

func (s *frequencySketch) hash4(key uint64) uint64 {
	return (key * s.seed4) >> 32
}

// min4 returns the minimum of 4 uint64 values.
func min4(a, b, c, d uint64) uint64 {
	min := a
	if b < min {
		min = b
	}
	if c < min {
		min = c
	}
	if d < min {
		min = d
	}
	return min
}

// stringHash computes a 64-bit hash of a string using FNV-1a algorithm.
// This is optimized for performance & zero allocations.
func stringHash(s string) uint64 {
	const (
		fnv64Offset = 14695981039346656037
		fnv64Prime  = 1099511628211
	)

	hash := uint64(fnv64Offset)

	// Use unsafe to avoid allocations when converting string to []byte
	// #nosec G103 - Safe usage: we only read the string data, no writes or pointer arithmetic
	data := unsafe.Slice(unsafe.StringData(s), len(s))

	for _, b := range data {
		hash ^= uint64(b)
		hash *= fnv64Prime
	}

	return hash
}
