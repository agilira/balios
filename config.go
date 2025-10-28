// config.go: configuration for Balios
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGILira library
// SPDX-License-Identifier: MPL-2.0

package balios

import (
	"time"

	"github.com/agilira/go-timecache"
)

// Config holds configuration parameters for the cache.
type Config struct {
	// MaxSize is the maximum number of entries the cache can hold.
	// Must be > 0. Default: DefaultMaxSize.
	MaxSize int

	// WindowRatio is the ratio of window cache to total cache size.
	// Must be between 0.0 and 1.0. Default: DefaultWindowRatio.
	WindowRatio float64

	// CounterBits is the number of bits per counter in the frequency sketch.
	// Must be between 1 and 8. Default: DefaultCounterBits.
	CounterBits int

	// TTL is the time-to-live for cache entries.
	// If 0, entries never expire. Default: 0 (no expiration).
	TTL time.Duration

	// NegativeCacheTTL is the time-to-live for caching loader errors.
	// When GetOrLoad fails, the error can be cached to prevent repeated
	// expensive operations that consistently fail.
	// If 0, errors are not cached (default behavior).
	// Recommended: 1-10 seconds for most use cases.
	// Example: Database unreachable errors don't need to be retried every millisecond.
	NegativeCacheTTL time.Duration

	// CleanupInterval is how often to run cleanup of expired entries.
	// Only used if TTL > 0. Default: TTL / 10.
	CleanupInterval time.Duration

	// Logger is used for debugging and monitoring.
	// If nil, NoOpLogger is used. Default: NoOpLogger.
	Logger Logger

	// TimeProvider provides current time for TTL calculations.
	// If nil, a default implementation is used. Default: system time.
	TimeProvider TimeProvider

	// MetricsCollector is used for collecting operation metrics (latencies, hit/miss rates).
	// If nil, NoOpMetricsCollector is used (zero overhead). Default: NoOpMetricsCollector.
	// Use this to integrate with Prometheus, DataDog, StatsD, or other monitoring systems.
	MetricsCollector MetricsCollector

	// OnEvict is called when an entry is evicted from the cache.
	// This callback must be fast and non-blocking.
	OnEvict func(key string, value interface{})

	// OnExpire is called when an entry expires (TTL-based removal).
	// This callback must be fast and non-blocking.
	OnExpire func(key string, value interface{})
}

// Validate checks configuration parameters and applies sensible defaults.
// Returns nil (no actual validation errors, only normalization).
//
// This method is automatically called by NewCache and NewGenericCache,
// so you typically don't need to call it manually. However, it's provided
// as a public API if you want to inspect the normalized configuration
// before creating a cache.
//
// Default values applied:
//   - MaxSize: DefaultMaxSize (10,000) if <= 0
//   - WindowRatio: DefaultWindowRatio (0.01) if <= 0 or >= 1
//   - CounterBits: DefaultCounterBits (4) if < 1 or > 8
//   - CleanupInterval: TTL/10 if TTL > 0 and CleanupInterval <= 0
//   - Logger: NoOpLogger{} if nil
//   - TimeProvider: systemTimeProvider{} if nil
//   - MetricsCollector: NoOpMetricsCollector{} if nil
func (c *Config) Validate() error {
	if c.MaxSize <= 0 {
		c.MaxSize = DefaultMaxSize
	}

	if c.WindowRatio <= 0 || c.WindowRatio >= 1 {
		c.WindowRatio = DefaultWindowRatio
	}

	if c.CounterBits < 1 || c.CounterBits > 8 {
		c.CounterBits = DefaultCounterBits
	}

	if c.TTL > 0 && c.CleanupInterval <= 0 {
		c.CleanupInterval = c.TTL / 10
		if c.CleanupInterval < time.Second {
			c.CleanupInterval = time.Second
		}
	}

	if c.Logger == nil {
		c.Logger = NoOpLogger{}
	}

	if c.TimeProvider == nil {
		c.TimeProvider = &systemTimeProvider{}
	}

	if c.MetricsCollector == nil {
		c.MetricsCollector = NoOpMetricsCollector{}
	}

	return nil
}

// DefaultConfig returns a configuration with sensible defaults.
func DefaultConfig() Config {
	return Config{
		MaxSize:          DefaultMaxSize,
		WindowRatio:      DefaultWindowRatio,
		CounterBits:      DefaultCounterBits,
		Logger:           NoOpLogger{},
		TimeProvider:     &systemTimeProvider{},
		MetricsCollector: NoOpMetricsCollector{},
	}
}

// systemTimeProvider is the default time provider using go-timecache.
// This provides ~121x faster time access compared to time.Now() with zero allocations.
type systemTimeProvider struct{}

func (t *systemTimeProvider) Now() int64 {
	return timecache.CachedTimeNano()
}
