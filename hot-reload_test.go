// hot-reload_test.go: tests for dynamic configuration
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGILira library
// SPDX-License-Identifier: MPL-2.0

package balios

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// TestNewHotConfig tests HotConfig creation
func TestNewHotConfig(t *testing.T) {
	cache := NewCache(DefaultConfig())
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test-config.yaml")

	// Create initial config file
	initialConfig := `cache:
  max_size: 1000
  ttl: 10m
  window_ratio: 0.01
  counter_bits: 4
`
	if err := os.WriteFile(configPath, []byte(initialConfig), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Create hot config
	hc, err := NewHotConfig(cache, HotConfigOptions{
		ConfigPath:   configPath,
		PollInterval: 100 * time.Millisecond,
	})

	if err != nil {
		t.Fatalf("NewHotConfig failed: %v", err)
	}
	defer func() { _ = hc.Stop() }()

	if hc == nil {
		t.Fatal("Expected non-nil HotConfig")
	}

	if hc.cache != cache {
		t.Error("HotConfig cache reference mismatch")
	}

	if hc.watcher == nil {
		t.Error("Expected non-nil watcher")
	}
}

// TestNewHotConfig_EmptyPath tests error handling for empty path
func TestNewHotConfig_EmptyPath(t *testing.T) {
	cache := NewCache(DefaultConfig())

	_, err := NewHotConfig(cache, HotConfigOptions{
		ConfigPath: "",
	})

	if err == nil {
		t.Error("Expected error for empty config path")
	}
}

// TestHotConfig_StartStop tests starting and stopping the watcher
func TestHotConfig_StartStop(t *testing.T) {
	cache := NewCache(DefaultConfig())
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test-config.yaml")

	// Create config file
	config := `cache:
  max_size: 500
  ttl: 5m
`
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	hc, err := NewHotConfig(cache, HotConfigOptions{
		ConfigPath:   configPath,
		PollInterval: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewHotConfig failed: %v", err)
	}

	// Start watching
	if err := hc.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Give it a moment to start
	time.Sleep(50 * time.Millisecond)

	// Stop watching
	if err := hc.Stop(); err != nil {
		t.Errorf("Failed to stop: %v", err)
	}
}

// TestHotConfig_ConfigReload tests configuration hot reload
func TestHotConfig_ConfigReload(t *testing.T) {
	cache := NewCache(DefaultConfig())
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test-config.yaml")

	// Create initial config
	initialConfig := `cache:
  max_size: 1000
  ttl: 10m
  window_ratio: 0.01
`
	if err := os.WriteFile(configPath, []byte(initialConfig), 0644); err != nil {
		t.Fatalf("Failed to write initial config: %v", err)
	}

	// Track reload events with synchronization
	var mu sync.Mutex
	reloadCount := 0
	reloadCh := make(chan Config, 2) // Buffered for initial + updated config

	hc, err := NewHotConfig(cache, HotConfigOptions{
		ConfigPath:   configPath,
		PollInterval: 50 * time.Millisecond, // Faster polling for test reliability
		OnReload: func(oldConfig, newConfig Config) {
			mu.Lock()
			reloadCount++
			mu.Unlock()
			// Non-blocking send to avoid deadlock
			select {
			case reloadCh <- newConfig:
			default:
			}
		},
	})
	if err != nil {
		t.Fatalf("NewHotConfig failed: %v", err)
	}
	defer func() { _ = hc.Stop() }()

	// Note: UniversalConfigWatcherWithConfig auto-starts the watcher
	if err := hc.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Verify watcher is running
	if !hc.watcher.IsRunning() {
		t.Fatal("Watcher is not running after Start()")
	}

	// Wait for and consume initial config load
	select {
	case initialCfg := <-reloadCh:
		if initialCfg.MaxSize != 1000 {
			t.Fatalf("Initial config wrong: MaxSize=%d, expected 1000", initialCfg.MaxSize)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Timeout waiting for initial config load")
	}

	// CRITICAL: Ensure enough time passes for mtime to change
	// Many filesystems have 1-second mtime granularity (FAT32, some ext4 configs)
	// We need the mtime to be VISIBLY different from the initial file
	time.Sleep(1500 * time.Millisecond)

	// Update config file with atomic write
	updatedConfig := `cache:
  max_size: 2000
  ttl: 20m
  window_ratio: 0.02
`
	tempPath := configPath + ".tmp"
	if err := os.WriteFile(tempPath, []byte(updatedConfig), 0644); err != nil {
		t.Fatalf("Failed to write temp config: %v", err)
	}

	// Atomic rename
	if err := os.Rename(tempPath, configPath); err != nil {
		t.Fatalf("Failed to rename config: %v", err)
	}

	// Force filesystem sync to ensure mtime is updated
	if file, err := os.Open(configPath); err == nil {
		_ = file.Sync()
		_ = file.Close()
	}

	// Wait for reload with generous timeout
	// Argus polls every 50ms, but we need to account for:
	// 1. Poll cycle timing (0-50ms)
	// 2. File system latency
	// 3. Callback processing time
	select {
	case newConfig := <-reloadCh:
		if newConfig.MaxSize != 2000 {
			t.Errorf("Expected MaxSize=2000, got %d", newConfig.MaxSize)
		}
		if newConfig.TTL != 20*time.Minute {
			t.Errorf("Expected TTL=20m, got %v", newConfig.TTL)
		}
		if newConfig.WindowRatio != 0.02 {
			t.Errorf("Expected WindowRatio=0.02, got %f", newConfig.WindowRatio)
		}
	case <-time.After(3 * time.Second):
		mu.Lock()
		count := reloadCount
		mu.Unlock()
		t.Fatalf("Timeout waiting for config reload. reloadCount=%d (expected at least 2)", count)
	}

	// Verify we got both callbacks (initial + update)
	mu.Lock()
	finalCount := reloadCount
	mu.Unlock()
	if finalCount < 2 {
		t.Errorf("Expected at least 2 reload events (initial + update), got %d", finalCount)
	}
}

// TestHotConfig_GetConfig tests thread-safe config access
func TestHotConfig_GetConfig(t *testing.T) {
	cache := NewCache(DefaultConfig())
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test-config.yaml")

	config := `cache:
  max_size: 750
  ttl: 15m
`
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	hc, err := NewHotConfig(cache, HotConfigOptions{
		ConfigPath:   configPath,
		PollInterval: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewHotConfig failed: %v", err)
	}
	defer func() { _ = hc.Stop() }()

	// GetConfig should work before Start
	cfg := hc.GetConfig()
	if cfg.MaxSize == 0 {
		t.Error("Expected default config before start")
	}

	// Start and wait for load
	if err := hc.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	time.Sleep(200 * time.Millisecond)

	// GetConfig should return loaded config
	cfg = hc.GetConfig()
	if cfg.MaxSize != 750 {
		t.Errorf("Expected MaxSize=750, got %d", cfg.MaxSize)
	}
}

// TestHotConfig_ParseConfig tests configuration parsing
func TestHotConfig_ParseConfig(t *testing.T) {
	cache := NewCache(DefaultConfig())
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "dummy.yaml")

	if err := os.WriteFile(configPath, []byte("cache: {}"), 0644); err != nil {
		t.Fatalf("Failed to write dummy config: %v", err)
	}

	hc, err := NewHotConfig(cache, HotConfigOptions{
		ConfigPath: configPath,
	})
	if err != nil {
		t.Fatalf("NewHotConfig failed: %v", err)
	}
	defer func() { _ = hc.Stop() }()

	tests := []struct {
		name   string
		data   map[string]interface{}
		expect func(*testing.T, Config)
	}{
		{
			name: "valid config with all fields",
			data: map[string]interface{}{
				"cache": map[string]interface{}{
					"max_size":     float64(5000),
					"ttl":          "30m",
					"window_ratio": 0.05,
					"counter_bits": float64(6),
				},
			},
			expect: func(t *testing.T, cfg Config) {
				if cfg.MaxSize != 5000 {
					t.Errorf("MaxSize: expected 5000, got %d", cfg.MaxSize)
				}
				if cfg.TTL != 30*time.Minute {
					t.Errorf("TTL: expected 30m, got %v", cfg.TTL)
				}
				if cfg.WindowRatio != 0.05 {
					t.Errorf("WindowRatio: expected 0.05, got %f", cfg.WindowRatio)
				}
				if cfg.CounterBits != 6 {
					t.Errorf("CounterBits: expected 6, got %d", cfg.CounterBits)
				}
			},
		},
		{
			name: "missing cache section returns defaults",
			data: map[string]interface{}{
				"other": "value",
			},
			expect: func(t *testing.T, cfg Config) {
				if cfg.MaxSize != DefaultMaxSize {
					t.Errorf("Expected default MaxSize=%d, got %d", DefaultMaxSize, cfg.MaxSize)
				}
			},
		},
		{
			name: "invalid ttl string ignored",
			data: map[string]interface{}{
				"cache": map[string]interface{}{
					"ttl": "invalid-duration",
				},
			},
			expect: func(t *testing.T, cfg Config) {
				if cfg.TTL != 0 {
					t.Errorf("Expected TTL=0 for invalid duration, got %v", cfg.TTL)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := hc.parseConfig(tt.data)
			tt.expect(t, cfg)
		})
	}
}

// TestHotConfig_JSONFormat tests JSON configuration format
func TestHotConfig_JSONFormat(t *testing.T) {
	cache := NewCache(DefaultConfig())
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test-config.json")

	// JSON config
	jsonConfig := `{
  "cache": {
    "max_size": 3000,
    "ttl": "25m",
    "window_ratio": 0.03,
    "counter_bits": 5
  }
}`
	if err := os.WriteFile(configPath, []byte(jsonConfig), 0644); err != nil {
		t.Fatalf("Failed to write JSON config: %v", err)
	}

	reloadCh := make(chan Config, 1)
	hc, err := NewHotConfig(cache, HotConfigOptions{
		ConfigPath:   configPath,
		PollInterval: 100 * time.Millisecond,
		OnReload: func(oldConfig, newConfig Config) {
			select {
			case reloadCh <- newConfig:
			default:
			}
		},
	})
	if err != nil {
		t.Fatalf("NewHotConfig failed: %v", err)
	}
	defer func() { _ = hc.Stop() }()

	if err := hc.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Wait for initial load
	select {
	case cfg := <-reloadCh:
		if cfg.MaxSize != 3000 {
			t.Errorf("Expected MaxSize=3000, got %d", cfg.MaxSize)
		}
		if cfg.TTL != 25*time.Minute {
			t.Errorf("Expected TTL=25m, got %v", cfg.TTL)
		}
	case <-time.After(2 * time.Second):
		t.Error("Timeout waiting for JSON config load")
	}
}

// BenchmarkHotConfig_GetConfig benchmarks thread-safe config access
func BenchmarkHotConfig_GetConfig(b *testing.B) {
	cache := NewCache(DefaultConfig())
	tempDir := b.TempDir()
	configPath := filepath.Join(tempDir, "bench-config.yaml")

	if err := os.WriteFile(configPath, []byte("cache: {max_size: 1000}"), 0644); err != nil {
		b.Fatalf("Failed to write config: %v", err)
	}

	hc, err := NewHotConfig(cache, HotConfigOptions{
		ConfigPath: configPath,
	})
	if err != nil {
		b.Fatalf("NewHotConfig failed: %v", err)
	}
	defer func() { _ = hc.Stop() }()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = hc.GetConfig()
	}
}
