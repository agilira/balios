// config_test.go: unit tests for Balios configuration
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGILira fragment
// SPDX-License-Identifier: MPL-2.0

package balios

import (
	"testing"
	"time"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name   string
		config Config
		want   Config
	}{
		{
			name:   "empty config uses defaults",
			config: Config{},
			want: Config{
				MaxSize:      DefaultMaxSize,
				WindowRatio:  DefaultWindowRatio,
				CounterBits:  DefaultCounterBits,
				Logger:       NoOpLogger{},
				TimeProvider: &systemTimeProvider{},
			},
		},
		{
			name: "invalid window ratio uses default",
			config: Config{
				MaxSize:     1000,
				WindowRatio: -0.1,
			},
			want: Config{
				MaxSize:      1000,
				WindowRatio:  DefaultWindowRatio,
				CounterBits:  DefaultCounterBits,
				Logger:       NoOpLogger{},
				TimeProvider: &systemTimeProvider{},
			},
		},
		{
			name: "TTL sets cleanup interval",
			config: Config{
				TTL: 10 * time.Second,
			},
			want: Config{
				MaxSize:         DefaultMaxSize,
				WindowRatio:     DefaultWindowRatio,
				CounterBits:     DefaultCounterBits,
				TTL:             10 * time.Second,
				CleanupInterval: time.Second,
				Logger:          NoOpLogger{},
				TimeProvider:    &systemTimeProvider{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if err != nil {
				t.Errorf("Config.Validate() error = %v", err)
				return
			}

			// Check individual fields since we can't compare structs with function fields
			if tt.config.MaxSize != tt.want.MaxSize {
				t.Errorf("MaxSize = %v, want %v", tt.config.MaxSize, tt.want.MaxSize)
			}
			if tt.config.WindowRatio != tt.want.WindowRatio {
				t.Errorf("WindowRatio = %v, want %v", tt.config.WindowRatio, tt.want.WindowRatio)
			}
			if tt.config.CounterBits != tt.want.CounterBits {
				t.Errorf("CounterBits = %v, want %v", tt.config.CounterBits, tt.want.CounterBits)
			}
			if tt.config.TTL != tt.want.TTL {
				t.Errorf("TTL = %v, want %v", tt.config.TTL, tt.want.TTL)
			}
			if tt.config.CleanupInterval != tt.want.CleanupInterval {
				t.Errorf("CleanupInterval = %v, want %v", tt.config.CleanupInterval, tt.want.CleanupInterval)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.MaxSize != DefaultMaxSize {
		t.Errorf("MaxSize = %v, want %v", config.MaxSize, DefaultMaxSize)
	}
	if config.WindowRatio != DefaultWindowRatio {
		t.Errorf("WindowRatio = %v, want %v", config.WindowRatio, DefaultWindowRatio)
	}
	if config.CounterBits != DefaultCounterBits {
		t.Errorf("CounterBits = %v, want %v", config.CounterBits, DefaultCounterBits)
	}
	if config.TTL != 0 {
		t.Errorf("TTL = %v, want 0", config.TTL)
	}
}

func TestCacheStats_HitRatio(t *testing.T) {
	tests := []struct {
		name  string
		stats CacheStats
		want  float64
	}{
		{
			name:  "no hits or misses",
			stats: CacheStats{Hits: 0, Misses: 0},
			want:  0,
		},
		{
			name:  "all hits",
			stats: CacheStats{Hits: 100, Misses: 0},
			want:  100,
		},
		{
			name:  "all misses",
			stats: CacheStats{Hits: 0, Misses: 100},
			want:  0,
		},
		{
			name:  "50% hit ratio",
			stats: CacheStats{Hits: 50, Misses: 50},
			want:  50,
		},
		{
			name:  "75% hit ratio",
			stats: CacheStats{Hits: 75, Misses: 25},
			want:  75,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.stats.HitRatio()
			if got != tt.want {
				t.Errorf("CacheStats.HitRatio() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSystemTimeProvider(t *testing.T) {
	provider := &systemTimeProvider{}

	now1 := provider.Now()

	// Verify it returns a reasonable timestamp (not zero)
	if now1 <= 0 {
		t.Errorf("Expected positive timestamp, got: %v", now1)
	}

	// Verify it's in a reasonable range (within last year and next day)
	oneYearAgo := time.Now().Add(-365 * 24 * time.Hour).UnixNano()
	tomorrow := time.Now().Add(24 * time.Hour).UnixNano()
	if now1 < oneYearAgo || now1 > tomorrow {
		t.Errorf("Timestamp out of reasonable range: %v", now1)
	}

	// Verify it returns consistent values (caching is working)
	now2 := provider.Now()

	// Note: go-timecache caches time for performance (~121x faster than time.Now())
	// Multiple rapid calls may return the same cached value - this is expected behavior
	// and a feature, not a bug. We just verify it's not moving backwards.
	if now2 < now1 {
		t.Errorf("Time should not go backwards: now1=%v, now2=%v", now1, now2)
	}
}

func TestNoOpLogger(t *testing.T) {
	// Just test that NoOpLogger doesn't panic
	logger := NoOpLogger{}

	logger.Debug("test")
	logger.Info("test")
	logger.Warn("test")
	logger.Error("test")

	logger.Debug("test", "key", "value")
	logger.Info("test", "key", "value")
	logger.Warn("test", "key", "value")
	logger.Error("test", "key", "value")
}

// TestNewCache_CallsValidate verifies that NewCache calls Config.Validate()
// to apply defaults, eliminating code duplication (issue #15 from code review)
func TestNewCache_CallsValidate(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		wantMaxSize int
		wantRatio   float64
	}{
		{
			name:        "empty config gets defaults",
			config:      Config{},
			wantMaxSize: DefaultMaxSize,
			wantRatio:   DefaultWindowRatio,
		},
		{
			name: "zero MaxSize gets default",
			config: Config{
				MaxSize: 0,
			},
			wantMaxSize: DefaultMaxSize,
			wantRatio:   DefaultWindowRatio,
		},
		{
			name: "negative MaxSize gets default",
			config: Config{
				MaxSize: -100,
			},
			wantMaxSize: DefaultMaxSize,
			wantRatio:   DefaultWindowRatio,
		},
		{
			name: "invalid WindowRatio gets default",
			config: Config{
				MaxSize:     1000,
				WindowRatio: -0.5, // Invalid
			},
			wantMaxSize: 1000,
			wantRatio:   DefaultWindowRatio,
		},
		{
			name: "WindowRatio >= 1.0 gets default",
			config: Config{
				MaxSize:     1000,
				WindowRatio: 1.5, // Invalid
			},
			wantMaxSize: 1000,
			wantRatio:   DefaultWindowRatio,
		},
		{
			name: "valid config preserved",
			config: Config{
				MaxSize:     5000,
				WindowRatio: 0.05,
			},
			wantMaxSize: 5000,
			wantRatio:   0.05,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := NewCache(tt.config)
			defer func() { _ = cache.Close() }()

			// Verify capacity reflects validated MaxSize
			capacity := cache.Capacity()
			if capacity != tt.wantMaxSize {
				t.Errorf("Cache capacity = %v, want %v (Config.Validate should have set defaults)", capacity, tt.wantMaxSize)
			}

			// Verify cache is functional with validated config
			cache.Set("test", "value")
			if val, found := cache.Get("test"); !found {
				t.Error("Cache should be functional after NewCache with validated config")
			} else if val != "value" {
				t.Errorf("Got value %v, want 'value'", val)
			}
		})
	}
}

// TestNewCache_ValidateAppliesAllDefaults ensures all defaults are applied
// when Config fields are nil or zero
func TestNewCache_ValidateAppliesAllDefaults(t *testing.T) {
	cache := NewCache(Config{})
	defer func() { _ = cache.Close() }()

	// Cache should be created successfully with all defaults
	if cache == nil {
		t.Fatal("NewCache with empty config should return valid cache")
	}

	// Verify it's functional
	cache.Set("key", "value")
	if _, found := cache.Get("key"); !found {
		t.Error("Cache with default config should be functional")
	}

	// Verify capacity was set to default
	if cache.Capacity() != DefaultMaxSize {
		t.Errorf("Capacity = %v, want %v", cache.Capacity(), DefaultMaxSize)
	}
}
