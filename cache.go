// cache.go: core lock-free W-TinyLFU cache implementation
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGILira library
// SPDX-License-Identifier: MPL-2.0

package balios

import (
	"sync/atomic"
	"unsafe"
)

// entry represents a cache entry with atomic access.
type entry struct {
	keyPtr   unsafe.Pointer // Thread-safe key storage (points to string)
	value    atomic.Value   // Thread-safe value storage
	keyHash  uint64         // Thread-safe hash storage (use atomic operations)
	expireAt int64          // expiration timestamp in nanoseconds (0 = no expiration)
	valid    int32          // atomic flag: 0=empty, 1=valid, 2=deleted, 3=pending
}

// wtinyLFUCache implements W-TinyLFU cache with lock-free operations.
// Uses simple atomic operations on fixed arrays for maximum performance.
type wtinyLFUCache struct {
	// Configuration (immutable after creation)
	maxSize          int32
	tableMask        uint32
	ttlNanos         int64            // TTL in nanoseconds (0 = no expiration)
	timeProvider     TimeProvider     // Provides current time
	metricsCollector MetricsCollector // Collects operation metrics (nil-safe)

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
	entryPending = 3 // Entry being written/updated
)

// Helper functions for atomic key operations
func (e *entry) loadKey() string {
	ptr := atomic.LoadPointer(&e.keyPtr)
	if ptr == nil {
		return ""
	}
	return *(*string)(ptr)
}

func (e *entry) storeKey(key string) {
	if key == "" {
		atomic.StorePointer(&e.keyPtr, nil)
		return
	}
	// Allocate string on heap to ensure it survives function scope
	keyPtr := new(string)
	*keyPtr = key
	atomic.StorePointer(&e.keyPtr, unsafe.Pointer(keyPtr))
}

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
	if config.MetricsCollector == nil {
		config.MetricsCollector = NoOpMetricsCollector{}
	}

	// Hash table size: power of 2, at least 2x maxSize for good load factor
	tableSize := nextPowerOf2(config.MaxSize * 2)
	if tableSize < 16 {
		tableSize = 16
	}

	cache := &wtinyLFUCache{
		maxSize:          int32(config.MaxSize), // #nosec G115 - MaxSize is validated and bounded
		tableMask:        uint32(tableSize - 1), // #nosec G115 - tableSize is power of 2, safe conversion
		ttlNanos:         int64(config.TTL),
		timeProvider:     config.TimeProvider,
		metricsCollector: config.MetricsCollector,
		entries:          make([]entry, tableSize),
		sketch:           newFrequencySketch(config.MaxSize),
	}

	return cache
}

// Set stores a key-value pair using lock-free operations.
func (c *wtinyLFUCache) Set(key string, value interface{}) bool {
	// Record start time for metrics
	var startTime int64
	if c.metricsCollector != nil {
		startTime = c.timeProvider.Now()
	}

	keyHash := stringHash(key)

	// Update frequency sketch (lock-free)
	c.sketch.increment(keyHash)

	// Calculate expiration time if TTL is set
	var expireAt int64
	if c.ttlNanos > 0 {
		if c.timeProvider == nil {
			// Fallback to default if somehow nil
			c.timeProvider = &systemTimeProvider{}
		}
		expireAt = c.timeProvider.Now() + c.ttlNanos
	}

	// Find slot using linear probing
	startIdx := keyHash & uint64(c.tableMask)

	for i := uint32(0); i <= c.tableMask; i++ {
		idx := (startIdx + uint64(i)) & uint64(c.tableMask)

		// Safety check: ensure entries slice is not nil and idx is in bounds
		if c.entries == nil || idx >= uint64(len(c.entries)) {
			return false
		}

		entry := &c.entries[idx]

		// Load current state atomically
		state := atomic.LoadInt32(&entry.valid)

		// Skip entries being written/updated by other threads
		if state == entryPending {
			continue
		}

		if state == entryEmpty || state == entryDeleted {
			// Try to claim this slot with entryPending first to prevent races
			if atomic.CompareAndSwapInt32(&entry.valid, state, entryPending) {
				// Successfully claimed - populate entry atomically
				// These writes are safe because we own the slot (valid = entryPending)
				// and no other goroutine will read it until we set valid = entryValid
				atomic.StoreUint64(&entry.keyHash, keyHash)
				entry.storeKey(key)
				entry.value.Store(value)
				atomic.StoreInt64(&entry.expireAt, expireAt)

				// Mark entry as valid - this acts as a memory barrier
				// ensuring all previous writes are visible
				atomic.StoreInt32(&entry.valid, entryValid)

				// Increment size for empty or deleted slots (new or reused)
				if state == entryEmpty || state == entryDeleted {
					atomic.AddInt64(&c.size, 1)
				}
				atomic.AddInt64(&c.sets, 1)

				// Record metrics for successful Set
				if c.metricsCollector != nil {
					latency := c.timeProvider.Now() - startTime
					c.metricsCollector.RecordSet(latency)
				}

				// Critical: Check for duplicates to maintain cache consistency
				// In high concurrency, multiple threads might create the same key
				c.removeDuplicateKeys(key, keyHash, entry)

				// Check if eviction needed AFTER incrementing size
				currentSize := atomic.LoadInt64(&c.size)
				if currentSize > int64(c.maxSize) {
					c.evictOne()
				}
				return true
			}
			// CAS failed, continue
			continue
		}

		// Check if this is an update to existing key
		// We need to be careful about race conditions here
		if state == entryValid && atomic.LoadUint64(&entry.keyHash) == keyHash {
			// Try to acquire the entry for update by marking it as pending
			if atomic.CompareAndSwapInt32(&entry.valid, entryValid, entryPending) {
				// Check if this is really the same key (now safe to read)
				if storedKey := entry.loadKey(); storedKey == key {
					// Update value atomically
					entry.value.Store(value)
					atomic.StoreInt64(&entry.expireAt, expireAt)

					// Release the entry back to valid state
					atomic.StoreInt32(&entry.valid, entryValid)
					atomic.AddInt64(&c.sets, 1)

					// Record metrics for successful Set (update)
					if c.metricsCollector != nil {
						latency := c.timeProvider.Now() - startTime
						c.metricsCollector.RecordSet(latency)
					}
					return true
				}
				// Wrong key, release and continue searching
				atomic.StoreInt32(&entry.valid, entryValid)
			}
			// CAS failed or wrong key, continue
			continue
		}
	}

	// Table full - try eviction and retry insertion
	c.evictOne()

	// After eviction, try one more time to find a slot
	for i := uint32(0); i <= c.tableMask; i++ {
		idx := (startIdx + uint64(i)) & uint64(c.tableMask)
		entry := &c.entries[idx]

		state := atomic.LoadInt32(&entry.valid)

		if state == entryEmpty || state == entryDeleted {
			// Try to claim this slot
			if atomic.CompareAndSwapInt32(&entry.valid, state, entryPending) {
				// Successfully claimed - populate entry atomically
				atomic.StoreUint64(&entry.keyHash, keyHash)
				entry.storeKey(key)
				entry.value.Store(value)
				atomic.StoreInt64(&entry.expireAt, expireAt)

				// Mark entry as valid
				atomic.StoreInt32(&entry.valid, entryValid)

				// Increment size for empty or deleted slots (new or reused)
				if state == entryEmpty || state == entryDeleted {
					atomic.AddInt64(&c.size, 1)
				}
				atomic.AddInt64(&c.sets, 1)

				// Record metrics for successful Set
				if c.metricsCollector != nil {
					latency := c.timeProvider.Now() - startTime
					c.metricsCollector.RecordSet(latency)
				}
				return true
			}
		}
	}

	// Still no space available - this should be rare
	return false
}

// Get retrieves a value using lock-free operations.
func (c *wtinyLFUCache) Get(key string) (interface{}, bool) {
	// Record start time for metrics (if collector is not nil, this is fast)
	var startTime int64
	if c.metricsCollector != nil {
		startTime = c.timeProvider.Now()
	}

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

		// Skip entries being written/updated
		if state == entryPending {
			continue
		}

		if state == entryValid && atomic.LoadUint64(&entry.keyHash) == keyHash {
			// Read key atomically by checking state before and after
			// This ensures we don't read partially written data
			if atomic.LoadInt32(&entry.valid) != entryValid {
				continue
			}

			if storedKey := entry.loadKey(); storedKey == key {
				// Check if entry has expired
				if c.ttlNanos > 0 {
					expireAt := atomic.LoadInt64(&entry.expireAt)
					if expireAt > 0 && c.timeProvider.Now() > expireAt {
						// Entry expired - mark as deleted asynchronously
						// We don't wait for the CAS to succeed, just try once
						atomic.CompareAndSwapInt32(&entry.valid, entryValid, entryDeleted)
						atomic.AddInt64(&c.misses, 1)

						// Record miss metrics
						if c.metricsCollector != nil {
							latency := c.timeProvider.Now() - startTime
							c.metricsCollector.RecordGet(latency, false)
						}
						return nil, false
					}
				}

				// Read value atomically
				value := entry.value.Load()

				// Double-check state hasn't changed during read
				if atomic.LoadInt32(&entry.valid) != entryValid {
					continue
				}

				// Found key and not expired - return value
				atomic.AddInt64(&c.hits, 1)

				// Record hit metrics
				if c.metricsCollector != nil {
					latency := c.timeProvider.Now() - startTime
					c.metricsCollector.RecordGet(latency, true)
				}
				return value, true
			}
		}
	}

	atomic.AddInt64(&c.misses, 1)

	// Record miss metrics
	if c.metricsCollector != nil {
		latency := c.timeProvider.Now() - startTime
		c.metricsCollector.RecordGet(latency, false)
	}
	return nil, false
}

// Delete removes a key using lock-free operations.
func (c *wtinyLFUCache) Delete(key string) bool {
	// Record start time for metrics
	var startTime int64
	if c.metricsCollector != nil {
		startTime = c.timeProvider.Now()
	}

	keyHash := stringHash(key)
	startIdx := keyHash & uint64(c.tableMask)

	for i := uint32(0); i <= c.tableMask; i++ {
		idx := (startIdx + uint64(i)) & uint64(c.tableMask)
		entry := &c.entries[idx]

		state := atomic.LoadInt32(&entry.valid)

		if state == entryEmpty {
			return false // Key not found
		}

		// Skip entries being written/updated
		if state == entryPending {
			continue
		}

		if state == entryValid && atomic.LoadUint64(&entry.keyHash) == keyHash {
			// Check state is still valid
			if atomic.LoadInt32(&entry.valid) != entryValid {
				continue
			}

			if storedKey := entry.loadKey(); storedKey == key {
				// Mark as deleted atomically
				if atomic.CompareAndSwapInt32(&entry.valid, entryValid, entryDeleted) {
					entry.storeKey("")
					// Note: we don't clear value as atomic.Value can't store nil
					// and it will be overwritten when entry is reused
					atomic.AddInt64(&c.size, -1)
					atomic.AddInt64(&c.deletes, 1)

					// Record metrics for successful Delete
					if c.metricsCollector != nil {
						latency := c.timeProvider.Now() - startTime
						c.metricsCollector.RecordDelete(latency)
					}
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

		// Skip entries being written/updated
		if state == entryPending {
			continue
		}

		if state == entryValid && atomic.LoadUint64(&entry.keyHash) == keyHash {
			// Check state is still valid
			if atomic.LoadInt32(&entry.valid) != entryValid {
				continue
			}

			if storedKey := entry.loadKey(); storedKey == key {
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
		c.entries[i].storeKey("")
		// Note: we don't clear value as atomic.Value can't store nil
		atomic.StoreUint64(&c.entries[i].keyHash, 0)
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
	const sampleSize = 8 // Increased sample size for better victim selection
	const maxRetries = 3 // Max retries to find a victim

	tableSize := int(c.tableMask) + 1

	// Try multiple rounds of sampling before giving up
	for retry := 0; retry < maxRetries; retry++ {
		var victim *entry
		minFrequency := uint64(^uint64(0)) // Max uint64

		// Use pseudo-random sampling based on current retry
		start := (retry * 17) % tableSize // Prime number for better distribution
		step := tableSize / sampleSize
		if step < 1 {
			step = 1
		}

		// Sample entries with better distribution
		for i := 0; i < sampleSize; i++ {
			idx := (start + i*step) % tableSize
			entry := &c.entries[idx]
			state := atomic.LoadInt32(&entry.valid)

			if state == entryValid {
				// Check frequency using the sketch
				freq := c.sketch.estimate(atomic.LoadUint64(&entry.keyHash))

				if freq < minFrequency {
					minFrequency = freq
					victim = entry
				}
			}
		}

		// If we found a victim, try to evict it
		if victim != nil {
			if atomic.CompareAndSwapInt32(&victim.valid, entryValid, entryDeleted) {
				victim.storeKey("")
				// Note: we don't clear value as atomic.Value can't store nil
				atomic.AddInt64(&c.size, -1)
				atomic.AddInt64(&c.evictions, 1)

				// Record eviction metrics
				if c.metricsCollector != nil {
					c.metricsCollector.RecordEviction()
				}
				return
			}
		}
	}

	// Last resort: scan a larger portion of the table to ensure we find a victim
	// In high-load scenarios, we need to be more aggressive
	scanSize := tableSize / 4 // Scan 1/4 of the table (more aggressive)
	if scanSize < 16 {
		scanSize = 16
	}
	if scanSize > tableSize {
		scanSize = tableSize
	}

	for i := 0; i < scanSize; i++ {
		entry := &c.entries[i]
		state := atomic.LoadInt32(&entry.valid)

		if state == entryValid {
			if atomic.CompareAndSwapInt32(&entry.valid, entryValid, entryDeleted) {
				entry.storeKey("")
				// Note: we don't clear value as atomic.Value can't store nil
				atomic.AddInt64(&c.size, -1)
				atomic.AddInt64(&c.evictions, 1)

				// Record eviction metrics
				if c.metricsCollector != nil {
					c.metricsCollector.RecordEviction()
				}
				return
			}
		}
	}
}

// removeDuplicateKeys removes any duplicate entries for the same key
// This is a safety mechanism to handle race conditions in concurrent Set operations
// Uses a limited scan around the hash position for performance
func (c *wtinyLFUCache) removeDuplicateKeys(key string, keyHash uint64, keepEntry *entry) {
	// Scan a limited range around the original hash position
	startIdx := keyHash & uint64(c.tableMask)

	// Scan a reasonable window (not the entire table)
	scanRange := uint32(32) // Check up to 32 positions
	if scanRange > c.tableMask {
		scanRange = c.tableMask
	}

	for i := uint32(0); i < scanRange; i++ {
		idx := (startIdx + uint64(i)) & uint64(c.tableMask)
		entry := &c.entries[idx]

		// Skip the entry we want to keep
		if entry == keepEntry {
			continue
		}

		// Check if this entry has the same key
		state := atomic.LoadInt32(&entry.valid)
		if state == entryValid && atomic.LoadUint64(&entry.keyHash) == keyHash {
			// Double-check the actual key to avoid hash collisions
			if storedKey := entry.loadKey(); storedKey == key {
				// Found a duplicate - remove it
				if atomic.CompareAndSwapInt32(&entry.valid, entryValid, entryDeleted) {
					entry.storeKey("")
					atomic.AddInt64(&c.size, -1)
					// Note: we don't increment evictions counter as this is a cleanup operation
				}
			}
		}
	}
}
