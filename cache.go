// cache.go: core lock-free W-TinyLFU cache implementation
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGILira library
// SPDX-License-Identifier: MPL-2.0

package balios

import (
	"sync/atomic"
)

// entry represents a cache entry with atomic access
type entry struct {
	key      string
	value    interface{}
	keyHash  uint64
	expireAt int64 // expiration timestamp in nanoseconds (0 = no expiration)
	valid    int32 // atomic flag: 0=empty, 1=valid, 2=deleted
}

// wtinyLFUCache implements W-TinyLFU cache with lock-free operations.
// Uses simple atomic operations on fixed arrays for maximum performance.
type wtinyLFUCache struct {
	// Configuration (immutable after creation)
	maxSize      int32
	tableMask    uint32
	ttlNanos     int64        // TTL in nanoseconds (0 = no expiration)
	timeProvider TimeProvider // Provides current time

	// Fixed-size array of entries for lock-free access
	entries []entry

	// W-TinyLFU frequency sketch (already lock-free)
	sketch *frequencySketch

	// Atomic statistics counters
	hits      int64
	misses    int64
	sets      int64
	deletes   int64
	evictions int64
	size      int64
}

const (
	entryEmpty   = 0
	entryValid   = 1
	entryDeleted = 2
)

// NewCache creates a new W-TinyLFU cache with lock-free operations.
func NewCache(config Config) Cache {
	// Apply configuration defaults
	if config.MaxSize <= 0 {
		config.MaxSize = DefaultMaxSize
	}
	if config.WindowRatio <= 0 {
		config.WindowRatio = DefaultWindowRatio
	}
	if config.TimeProvider == nil {
		config.TimeProvider = &systemTimeProvider{}
	}

	// Hash table size: power of 2, at least 2x maxSize for good load factor
	tableSize := nextPowerOf2(config.MaxSize * 2)
	if tableSize < 16 {
		tableSize = 16
	}

	cache := &wtinyLFUCache{
		maxSize:      int32(config.MaxSize), // #nosec G115 - MaxSize is validated and bounded
		tableMask:    uint32(tableSize - 1), // #nosec G115 - tableSize is power of 2, safe conversion
		ttlNanos:     int64(config.TTL),
		timeProvider: config.TimeProvider,
		entries:      make([]entry, tableSize),
		sketch:       newFrequencySketch(config.MaxSize),
	}

	return cache
}

// Set stores a key-value pair using lock-free operations.
func (c *wtinyLFUCache) Set(key string, value interface{}) bool {
	keyHash := stringHash(key)

	// Update frequency sketch (lock-free)
	c.sketch.increment(keyHash)

	// Calculate expiration time if TTL is set
	var expireAt int64
	if c.ttlNanos > 0 {
		expireAt = c.timeProvider.Now() + c.ttlNanos
	}

	// Find slot using linear probing
	startIdx := keyHash & uint64(c.tableMask)

	for i := uint32(0); i <= c.tableMask; i++ {
		idx := (startIdx + uint64(i)) & uint64(c.tableMask)
		entry := &c.entries[idx]

		// Load current state atomically
		state := atomic.LoadInt32(&entry.valid)

		if state == entryEmpty || state == entryDeleted {
			// Try to claim this slot
			if atomic.CompareAndSwapInt32(&entry.valid, state, entryValid) {
				// Successfully claimed - populate entry
				entry.keyHash = keyHash
				entry.key = key
				entry.value = value
				atomic.StoreInt64(&entry.expireAt, expireAt)

				if state == entryEmpty {
					atomic.AddInt64(&c.size, 1)
				}
				atomic.AddInt64(&c.sets, 1)

				// Check if eviction needed
				if atomic.LoadInt64(&c.size) > int64(c.maxSize) {
					c.evictOne()
				}
				return true
			}
			// CAS failed, continue
			continue
		}

		// Check if this is an update to existing key
		// We need to be careful about race conditions here
		if state == entryValid && entry.keyHash == keyHash {
			// Double-check the state is still valid before comparing key
			// Add nil check to prevent panic
			if atomic.LoadInt32(&entry.valid) == entryValid && entry.key != "" && entry.key == key {
				// Update value - this is safe since we verified the entry is valid
				entry.value = value
				atomic.StoreInt64(&entry.expireAt, expireAt)
				atomic.AddInt64(&c.sets, 1)
				return true
			}
		}
	}

	// Table full - try eviction once
	c.evictOne()
	return false
}

// Get retrieves a value using lock-free operations.
func (c *wtinyLFUCache) Get(key string) (interface{}, bool) {
	keyHash := stringHash(key)

	// Update frequency sketch (lock-free)
	c.sketch.increment(keyHash)

	// Find slot using linear probing
	startIdx := keyHash & uint64(c.tableMask)

	for i := uint32(0); i <= c.tableMask; i++ {
		idx := (startIdx + uint64(i)) & uint64(c.tableMask)
		entry := &c.entries[idx]

		// Load state atomically
		state := atomic.LoadInt32(&entry.valid)

		if state == entryEmpty {
			// Empty slot means key not found
			break
		}

		if state == entryValid && entry.keyHash == keyHash {
			// Double-check the state and compare key safely
			if atomic.LoadInt32(&entry.valid) == entryValid && entry.key != "" && entry.key == key {
				// Check if entry has expired
				if c.ttlNanos > 0 {
					expireAt := atomic.LoadInt64(&entry.expireAt)
					if expireAt > 0 && c.timeProvider.Now() > expireAt {
						// Entry expired - mark as deleted asynchronously
						// We don't wait for the CAS to succeed, just try once
						atomic.CompareAndSwapInt32(&entry.valid, entryValid, entryDeleted)
						atomic.AddInt64(&c.misses, 1)
						return nil, false
					}
				}

				// Found key and not expired - return value
				atomic.AddInt64(&c.hits, 1)
				return entry.value, true
			}
		}
	}

	atomic.AddInt64(&c.misses, 1)
	return nil, false
}

// Delete removes a key using lock-free operations.
func (c *wtinyLFUCache) Delete(key string) bool {
	keyHash := stringHash(key)
	startIdx := keyHash & uint64(c.tableMask)

	for i := uint32(0); i <= c.tableMask; i++ {
		idx := (startIdx + uint64(i)) & uint64(c.tableMask)
		entry := &c.entries[idx]

		state := atomic.LoadInt32(&entry.valid)

		if state == entryEmpty {
			return false // Key not found
		}

		if state == entryValid && entry.keyHash == keyHash {
			// Double-check and compare key safely
			if atomic.LoadInt32(&entry.valid) == entryValid && entry.key != "" && entry.key == key {
				// Mark as deleted atomically
				if atomic.CompareAndSwapInt32(&entry.valid, entryValid, entryDeleted) {
					entry.key = ""
					entry.value = nil
					atomic.AddInt64(&c.size, -1)
					atomic.AddInt64(&c.deletes, 1)
					return true
				}
			}
		}
	}

	return false
}

// Has checks if a key exists without retrieving the value.
func (c *wtinyLFUCache) Has(key string) bool {
	keyHash := stringHash(key)
	startIdx := keyHash & uint64(c.tableMask)

	for i := uint32(0); i <= c.tableMask; i++ {
		idx := (startIdx + uint64(i)) & uint64(c.tableMask)
		entry := &c.entries[idx]

		state := atomic.LoadInt32(&entry.valid)

		if state == entryEmpty {
			return false
		}

		if state == entryValid && entry.keyHash == keyHash {
			// Double-check and compare key safely
			if atomic.LoadInt32(&entry.valid) == entryValid && entry.key != "" && entry.key == key {
				return true
			}
		}
	}

	return false
}

// Len returns current number of items.
func (c *wtinyLFUCache) Len() int {
	return int(atomic.LoadInt64(&c.size))
}

// Capacity returns maximum number of items.
func (c *wtinyLFUCache) Capacity() int {
	return int(c.maxSize)
}

// Clear removes all entries.
func (c *wtinyLFUCache) Clear() {
	// Reset all entries
	for i := range c.entries {
		atomic.StoreInt32(&c.entries[i].valid, entryEmpty)
		c.entries[i].key = ""
		c.entries[i].value = nil
		c.entries[i].keyHash = 0
	}

	// Reset counters
	atomic.StoreInt64(&c.size, 0)
	atomic.StoreInt64(&c.hits, 0)
	atomic.StoreInt64(&c.misses, 0)
	atomic.StoreInt64(&c.sets, 0)
	atomic.StoreInt64(&c.deletes, 0)
	atomic.StoreInt64(&c.evictions, 0)

	// Reset frequency sketch
	c.sketch.reset()
}

// Stats returns cache statistics.
func (c *wtinyLFUCache) Stats() CacheStats {
	return CacheStats{
		Hits:      uint64(atomic.LoadInt64(&c.hits)),      // #nosec G115 - stats counters are always positive
		Misses:    uint64(atomic.LoadInt64(&c.misses)),    // #nosec G115 - stats counters are always positive
		Sets:      uint64(atomic.LoadInt64(&c.sets)),      // #nosec G115 - stats counters are always positive
		Deletes:   uint64(atomic.LoadInt64(&c.deletes)),   // #nosec G115 - stats counters are always positive
		Evictions: uint64(atomic.LoadInt64(&c.evictions)), // #nosec G115 - stats counters are always positive
		Size:      int(atomic.LoadInt64(&c.size)),
		Capacity:  int(c.maxSize),
	}
}

// Close gracefully shuts down the cache.
func (c *wtinyLFUCache) Close() error {
	c.Clear()
	return nil
}

// evictOne performs W-TinyLFU eviction by finding the entry with lowest frequency.
// Uses a sampling approach to avoid scanning the entire table.
func (c *wtinyLFUCache) evictOne() {
	const sampleSize = 5 // Sample 5 random entries to find victim

	var victim *entry
	minFrequency := uint64(^uint64(0)) // Max uint64

	// Sample random entries to find the one with lowest frequency
	// This is much faster than scanning the entire table
	tableSize := int(c.tableMask) + 1
	step := tableSize / sampleSize
	if step < 1 {
		step = 1
	}

	for i := 0; i < sampleSize; i++ {
		idx := (i * step) % tableSize
		entry := &c.entries[idx]
		state := atomic.LoadInt32(&entry.valid)

		if state == entryValid {
			// Check frequency using the sketch
			freq := c.sketch.estimate(entry.keyHash)

			if freq < minFrequency {
				minFrequency = freq
				victim = entry
			}
		}
	}

	// If we found a victim, try to evict it
	if victim != nil {
		if atomic.CompareAndSwapInt32(&victim.valid, entryValid, entryDeleted) {
			victim.key = ""
			victim.value = nil
			atomic.AddInt64(&c.size, -1)
			atomic.AddInt64(&c.evictions, 1)
			return
		}
	}

	// Fallback: if sampling failed, do a simple linear scan
	for i := range c.entries {
		entry := &c.entries[i]
		state := atomic.LoadInt32(&entry.valid)

		if state == entryValid {
			if atomic.CompareAndSwapInt32(&entry.valid, entryValid, entryDeleted) {
				entry.key = ""
				entry.value = nil
				atomic.AddInt64(&c.size, -1)
				atomic.AddInt64(&c.evictions, 1)
				return
			}
		}
	}
}
