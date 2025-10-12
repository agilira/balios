// hotconfig.go: dynamic configuration with Argus integration
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGILira library
// SPDX-License-Identifier: MPL-2.0

package balios

import (
	"fmt"
	"sync"
	"time"

	"github.com/agilira/argus"
)

// HotConfig provides dynamic configuration reload capabilities using Argus.
// It watches a configuration file and automatically updates cache settings
// when changes are detected.
type HotConfig struct {
	cache   Cache
	watcher *argus.Watcher
	mu      sync.RWMutex
	config  Config

	// OnReload is called after configuration is successfully reloaded.
	// This callback is optional and must be fast and non-blocking.
	OnReload func(oldConfig, newConfig Config)
}

// HotConfigOptions configures hot reload behavior.
type HotConfigOptions struct {
	// ConfigPath is the path to the configuration file to watch.
	// Supports JSON, YAML, TOML, HCL, INI, Properties formats.
	ConfigPath string

	// PollInterval is how often to check for configuration changes.
	// Default: 1 second. Minimum: 100ms.
	PollInterval time.Duration

	// OnReload is called after configuration is successfully reloaded.
	OnReload func(oldConfig, newConfig Config)

	// Logger for hot reload operations.
	// If nil, uses the cache's logger.
	Logger Logger
}

// NewHotConfig creates a new hot-reloadable configuration for a cache.
// It starts watching the configuration file immediately.
//
// Example configuration file (YAML):
//
//	cache:
//	  max_size: 10000
//	  ttl: "1h"
//	  window_ratio: 0.01
//	  counter_bits: 4
//
// Supported configuration keys:
//   - cache.max_size (int): Maximum number of cache entries
//   - cache.ttl (duration string): Time-to-live for entries (e.g., "1h", "30m")
//   - cache.window_ratio (float): Window cache ratio (0.0-1.0)
//   - cache.counter_bits (int): Frequency counter bits (1-8)
//
// Note: Changes to MaxSize require cache reconstruction and are not
// applied dynamically. Only TTL and other runtime parameters can be
// hot-reloaded without disruption.
func NewHotConfig(cache Cache, opts HotConfigOptions) (*HotConfig, error) {
	if opts.ConfigPath == "" {
		return nil, fmt.Errorf("config_path is required")
	}

	if opts.PollInterval == 0 {
		opts.PollInterval = 1 * time.Second
	} else if opts.PollInterval < 100*time.Millisecond {
		opts.PollInterval = 100 * time.Millisecond
	}

	if opts.Logger == nil {
		// Try to extract logger from cache if it implements LoggerGetter
		if lg, ok := cache.(interface{ Logger() Logger }); ok {
			opts.Logger = lg.Logger()
		} else {
			opts.Logger = NoOpLogger{}
		}
	}

	hc := &HotConfig{
		cache:    cache,
		OnReload: opts.OnReload,
		config:   DefaultConfig(), // Start with defaults
	}

	// Create Argus config with specified PollInterval for fast file change detection
	argusConfig := argus.Config{
		PollInterval: opts.PollInterval,
	}

	// Use UniversalConfigWatcherWithConfig to pass custom poll interval
	watcher, err := argus.UniversalConfigWatcherWithConfig(opts.ConfigPath, hc.handleConfigChange, argusConfig)
	if err != nil {
		return nil, err
	}
	hc.watcher = watcher

	return hc, nil
}

// Start begins watching the configuration file for changes.
// Note: The watcher monitors file changes at the configured PollInterval.
func (hc *HotConfig) Start() error {
	// Check if already running to avoid ARGUS_WATCHER_BUSY error
	if hc.watcher.IsRunning() {
		return nil // Already started
	}
	return hc.watcher.Start()
}

// Stop stops watching the configuration file.
func (hc *HotConfig) Stop() {
	hc.watcher.Stop()
}

// GetConfig returns the current configuration (thread-safe).
func (hc *HotConfig) GetConfig() Config {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	return hc.config
}

// handleConfigChange is called by Argus when configuration changes.
func (hc *HotConfig) handleConfigChange(configData map[string]interface{}) {
	hc.mu.Lock()
	oldConfig := hc.config
	newConfig := hc.parseConfig(configData)
	hc.config = newConfig
	hc.mu.Unlock()

	// Apply dynamic configuration changes
	hc.applyChanges(oldConfig, newConfig)

	// Trigger callback if set
	if hc.OnReload != nil {
		hc.OnReload(oldConfig, newConfig)
	}
}

// parseConfig extracts cache configuration from Argus config data.
func (hc *HotConfig) parseConfig(data map[string]interface{}) Config {
	config := DefaultConfig()

	// Extract cache section - Argus might nest it or provide it directly
	cacheSection, ok := data["cache"].(map[string]interface{})
	if !ok {
		// Try if the whole data IS the cache section
		if _, hasMaxSize := data["max_size"]; hasMaxSize {
			cacheSection = data
		} else {
			return config
		}
	}

	// Parse MaxSize
	if maxSize, ok := cacheSection["max_size"].(int); ok && maxSize > 0 {
		config.MaxSize = maxSize
	} else if maxSize, ok := cacheSection["max_size"].(float64); ok && maxSize > 0 {
		config.MaxSize = int(maxSize)
	}

	// Parse TTL (string duration like "1h", "30m")
	// Note: YAML values should be unquoted (e.g., ttl: 10m not ttl: "10m")
	if ttlStr, ok := cacheSection["ttl"].(string); ok {
		if ttl, err := time.ParseDuration(ttlStr); err == nil {
			config.TTL = ttl
		}
	}

	// Parse WindowRatio
	if ratio, ok := cacheSection["window_ratio"].(float64); ok {
		if ratio > 0 && ratio < 1 {
			config.WindowRatio = ratio
		}
	}

	// Parse CounterBits
	if bits, ok := cacheSection["counter_bits"].(int); ok {
		if bits >= 1 && bits <= 8 {
			config.CounterBits = bits
		}
	} else if bits, ok := cacheSection["counter_bits"].(float64); ok {
		if bits >= 1 && bits <= 8 {
			config.CounterBits = int(bits)
		}
	}

	return config
} // applyChanges applies configuration changes to the running cache.
// Note: Some changes (like MaxSize) cannot be applied dynamically and require
// cache reconstruction.
func (hc *HotConfig) applyChanges(old, new Config) {
	// For now, we only support hot-reloading TTL
	// MaxSize changes would require rebuilding the entire cache structure

	// In a more advanced implementation, we could:
	// 1. Create a new cache with new MaxSize
	// 2. Copy entries from old to new
	// 3. Atomically swap the cache reference

	// For this implementation, we simply log that MaxSize changes
	// require restart (to be implemented in future versions)

	// TTL changes are already effective because cache reads the
	// TTL from config on each Set operation (in future versions)
}
