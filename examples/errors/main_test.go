// main_test.go: tests for error handling example
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGILira fragment
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"testing"
	"time"

	"github.com/agilira/balios"
)

func TestDemonstrateKeyNotFound(t *testing.T) {
	cache := balios.NewCache(balios.Config{MaxSize: 10})
	defer func() { _ = cache.Close() }()

	// Should not panic
	demonstrateKeyNotFound(cache)
}

func TestDemonstrateCacheFull(t *testing.T) {
	cache := balios.NewCache(balios.Config{MaxSize: 3})
	defer func() { _ = cache.Close() }()

	// Should not panic
	demonstrateCacheFull(cache)
}

func TestDemonstrateErrorContext(t *testing.T) {
	// Should not panic
	demonstrateErrorContext()
}

func TestDemonstrateErrorCategories(t *testing.T) {
	// Should not panic
	demonstrateErrorCategories()
}

func TestDemonstrateJSONSerialization(t *testing.T) {
	// Should not panic
	demonstrateJSONSerialization()
}

func TestDemonstrateErrorWrapping(t *testing.T) {
	// Should not panic
	demonstrateErrorWrapping()
}

func TestErrorCodeExtraction(t *testing.T) {
	err := balios.NewErrInvalidMaxSize(-1)
	code := balios.GetErrorCode(err)

	if code == "" {
		t.Error("Expected non-empty error code")
	}

	expectedCode := balios.ErrCodeInvalidMaxSize
	if code != expectedCode {
		t.Errorf("Expected code %s, got %s", expectedCode, code)
	}
}

func TestErrorCategorization(t *testing.T) {
	tests := []struct {
		name          string
		err           error
		isConfig      bool
		isOperation   bool
		isLoader      bool
		isPersistence bool
	}{
		{
			name:          "Config Error",
			err:           balios.NewErrInvalidMaxSize(-1),
			isConfig:      true,
			isOperation:   false,
			isLoader:      false,
			isPersistence: false,
		},
		{
			name:          "Operation Error",
			err:           balios.NewErrCacheFull(10, 10),
			isConfig:      false,
			isOperation:   true,
			isLoader:      false,
			isPersistence: false,
		},
		{
			name:          "Loader Error",
			err:           balios.NewErrLoaderTimeout("key", "5s"),
			isConfig:      false,
			isOperation:   false,
			isLoader:      true,
			isPersistence: false,
		},
		{
			name:          "Persistence Error",
			err:           balios.NewErrCorruptedData("/tmp/cache", "invalid"),
			isConfig:      false,
			isOperation:   false,
			isLoader:      false,
			isPersistence: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := balios.IsConfigError(tt.err); got != tt.isConfig {
				t.Errorf("IsConfigError = %v, want %v", got, tt.isConfig)
			}
			if got := balios.IsOperationError(tt.err); got != tt.isOperation {
				t.Errorf("IsOperationError = %v, want %v", got, tt.isOperation)
			}
			if got := balios.IsLoaderError(tt.err); got != tt.isLoader {
				t.Errorf("IsLoaderError = %v, want %v", got, tt.isLoader)
			}
			if got := balios.IsPersistenceError(tt.err); got != tt.isPersistence {
				t.Errorf("IsPersistenceError = %v, want %v", got, tt.isPersistence)
			}
		})
	}
}

func TestErrorContext(t *testing.T) {
	err := balios.NewErrCacheFull(100, 100)
	ctx := balios.GetErrorContext(err)

	if ctx == nil {
		t.Fatal("Expected non-nil context")
	}

	// Should have capacity and current_size in context
	if _, ok := ctx["capacity"]; !ok {
		t.Error("Expected 'capacity' in context")
	}

	if _, ok := ctx["current_size"]; !ok {
		t.Error("Expected 'current_size' in context")
	}
}

func TestRetryableErrors(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		retryable bool
	}{
		{
			name:      "Cache Full (retryable)",
			err:       balios.NewErrCacheFull(10, 10),
			retryable: true,
		},
		{
			name:      "Loader Timeout (retryable)",
			err:       balios.NewErrLoaderTimeout("key", "5s"),
			retryable: true,
		},
		{
			name:      "Config Error (not retryable)",
			err:       balios.NewErrInvalidMaxSize(-1),
			retryable: false,
		},
		{
			name:      "Key Not Found (not retryable)",
			err:       balios.NewErrKeyNotFound("key"),
			retryable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := balios.IsRetryable(tt.err); got != tt.retryable {
				t.Errorf("IsRetryable = %v, want %v", got, tt.retryable)
			}
		})
	}
}

func TestCacheWithErrorHandling(t *testing.T) {
	cache := balios.NewCache(balios.Config{
		MaxSize: 5,
		TTL:     100 * time.Millisecond,
	})
	defer func() { _ = cache.Close() }()

	// Test normal operations
	ok := cache.Set("key1", "value1")
	if !ok {
		t.Error("Set should succeed")
	}

	value, found := cache.Get("key1")
	if !found {
		err := balios.NewErrKeyNotFound("key1")
		if !balios.IsNotFound(err) {
			t.Error("Should be NotFound error")
		}
	} else if value != "value1" {
		t.Errorf("Expected 'value1', got %v", value)
	}

	// Test key not found
	_, found = cache.Get("nonexistent")
	if found {
		t.Error("Should not find nonexistent key")
	}
}
