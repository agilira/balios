// interfaces.go: public interfaces for Balios
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGILira library
// SPDX-License-Identifier: MPL-2.0

package balios

import "context"

// Cache represents a high-performance in-memory cache interface.
// All methods must be safe for concurrent use.
type Cache interface {
	// Get retrieves a value from the cache.
	// Returns the value and true if found, nil and false otherwise.
	// This method must be zero-allocation on the hot path.
	Get(key string) (value interface{}, found bool)

	// Set stores a key-value pair in the cache.
	// Returns true if the item was successfully stored.
	//
	// Note: Returns false only in extreme cases when the cache is full and
	// eviction fails repeatedly, which is virtually impossible in normal operation
	// (< 0.001% probability with proper cache sizing). In practice, Set() always succeeds.
	//
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
	// Note: This operation is not atomic. During Clear(), other goroutines
	// may still read/write, potentially observing a partially cleared cache.
	// This is acceptable for most use cases (cache flush, shutdown, testing).
	Clear()

	// Stats returns cache statistics.
	Stats() CacheStats

	// GetOrLoad returns the value from cache, or loads it using the provided loader.
	// If multiple goroutines call GetOrLoad for the same missing key concurrently,
	// only one loader will be executed (singleflight pattern).
	// The loaded value is cached with the cache's default TTL.
	// If the loader returns an error, the error is NOT cached.
	GetOrLoad(key string, loader func() (interface{}, error)) (interface{}, error)

	// GetOrLoadWithContext is like GetOrLoad but respects context cancellation and timeout.
	// The context is passed to the loader function for cancellation control.
	GetOrLoadWithContext(ctx context.Context, key string, loader func(context.Context) (interface{}, error)) (interface{}, error)

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
// Returns 0.0 if no Get operations have been performed yet.
// Formula: (Hits / (Hits + Misses)) * 100
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

// Debug does nothing (no-op implementation).
func (NoOpLogger) Debug(msg string, keyvals ...interface{}) {}

// Info does nothing (no-op implementation).
func (NoOpLogger) Info(msg string, keyvals ...interface{}) {}

// Warn does nothing (no-op implementation).
func (NoOpLogger) Warn(msg string, keyvals ...interface{}) {}

// Error does nothing (no-op implementation).
func (NoOpLogger) Error(msg string, keyvals ...interface{}) {}

// TimeProvider provides current time with caching for performance.
// This interface allows injecting optimized time implementations.
type TimeProvider interface {
	// Now returns the current time in nanoseconds since epoch.
	// This method must be very fast and allocation-free.
	Now() int64
}

// MetricsCollector defines an interface for collecting cache operation metrics.
// Implementations can send metrics to Prometheus, DataDog, StatsD, or other monitoring systems.
// This interface is designed for zero overhead when nil - no metrics are collected.
//
// Performance requirements:
//   - All methods must be lock-free or use minimal locking
//   - All methods must be allocation-free
//   - All methods must complete in < 100ns for production use
//
// Thread-safety:
//   - All methods must be safe for concurrent use
//   - Multiple goroutines will call these methods simultaneously
type MetricsCollector interface {
	// RecordGet records a Get operation with its latency and hit/miss result.
	// latencyNs is the duration of the Get operation in nanoseconds.
	// hit indicates whether the key was found (true) or not (false).
	RecordGet(latencyNs int64, hit bool)

	// RecordSet records a Set operation with its latency.
	// latencyNs is the duration of the Set operation in nanoseconds.
	RecordSet(latencyNs int64)

	// RecordDelete records a Delete operation with its latency.
	// latencyNs is the duration of the Delete operation in nanoseconds.
	RecordDelete(latencyNs int64)

	// RecordEviction records a cache eviction event.
	// Called when an entry is evicted due to cache being full.
	RecordEviction()
}

// NoOpMetricsCollector is a metrics collector that does nothing.
// Used as default to avoid nil checks and ensure zero overhead.
// All methods are inlined by the compiler for maximum performance.
type NoOpMetricsCollector struct{}

// RecordGet does nothing. Inlined by compiler.
func (NoOpMetricsCollector) RecordGet(latencyNs int64, hit bool) {}

// RecordSet does nothing. Inlined by compiler.
func (NoOpMetricsCollector) RecordSet(latencyNs int64) {}

// RecordDelete does nothing. Inlined by compiler.
func (NoOpMetricsCollector) RecordDelete(latencyNs int64) {}

// RecordEviction does nothing. Inlined by compiler.
func (NoOpMetricsCollector) RecordEviction() {}
