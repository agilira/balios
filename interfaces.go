// interface.go: public interfaces for Balios
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGILira library
// SPDX-License-Identifier: MPL-2.0

package balios

// Cache represents a high-performance in-memory cache interface.
// All methods must be safe for concurrent use.
type Cache interface {
	// Get retrieves a value from the cache.
	// Returns the value and true if found, nil and false otherwise.
	// This method must be zero-allocation on the hot path.
	Get(key string) (value interface{}, found bool)

	// Set stores a key-value pair in the cache.
	// Returns true if the item was successfully stored.
	// This method must be zero-allocation on the hot path.
	Set(key string, value interface{}) bool

	// Delete removes an item from the cache.
	// Returns true if the item was present and removed.
	Delete(key string) bool

	// Has checks if a key exists in the cache without retrieving the value.
	// This method should be faster than Get when only existence matters.
	Has(key string) bool

	// Len returns the current number of items in the cache.
	Len() int

	// Capacity returns the maximum number of items the cache can hold.
	Capacity() int

	// Clear removes all items from the cache.
	Clear()

	// Stats returns cache statistics.
	Stats() CacheStats

	// Close gracefully shuts down the cache and releases resources.
	Close() error
}

// CacheStats provides statistics about cache performance.
type CacheStats struct {
	// Hits is the number of cache hits
	Hits uint64

	// Misses is the number of cache misses
	Misses uint64

	// Sets is the number of successful set operations
	Sets uint64

	// Deletes is the number of successful delete operations
	Deletes uint64

	// Evictions is the number of items evicted from the cache
	Evictions uint64

	// Size is the current number of items in the cache
	Size int

	// Capacity is the maximum number of items the cache can hold
	Capacity int
}

// HitRatio returns the cache hit ratio as a percentage (0-100).
func (s CacheStats) HitRatio() float64 {
	total := s.Hits + s.Misses
	if total == 0 {
		return 0
	}
	return float64(s.Hits) / float64(total) * 100
}

// Logger defines a minimal logging interface with zero overhead.
// Implementations should use structured logging and be allocation-free.
type Logger interface {
	// Debug logs a debug message with optional key-value pairs.
	Debug(msg string, keyvals ...interface{})

	// Info logs an info message with optional key-value pairs.
	Info(msg string, keyvals ...interface{})

	// Warn logs a warning message with optional key-value pairs.
	Warn(msg string, keyvals ...interface{})

	// Error logs an error message with optional key-value pairs.
	Error(msg string, keyvals ...interface{})
}

// NoOpLogger is a logger that does nothing. Used as default to avoid nil checks.
type NoOpLogger struct{}

func (NoOpLogger) Debug(msg string, keyvals ...interface{}) {}
func (NoOpLogger) Info(msg string, keyvals ...interface{})  {}
func (NoOpLogger) Warn(msg string, keyvals ...interface{})  {}
func (NoOpLogger) Error(msg string, keyvals ...interface{}) {}

// TimeProvider provides current time with caching for performance.
// This interface allows injecting optimized time implementations.
type TimeProvider interface {
	// Now returns the current time in nanoseconds since epoch.
	// This method must be very fast and allocation-free.
	Now() int64
}
