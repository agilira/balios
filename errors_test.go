// errors_test.go: tests and benchmarks for error handling in Balios
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGILira fragment
// SPDX-License-Identifier: MPL-2.0

package balios

import (
	"encoding/json"
	goerrors "errors"
	"testing"

	"github.com/agilira/go-errors"
)

// Test error code creation and basic properties
func TestErrorCodes(t *testing.T) {
	tests := []struct {
		name         string
		errFunc      func() error
		expectedCode errors.ErrorCode
		shouldRetry  bool
	}{
		{
			name:         "InvalidMaxSize",
			errFunc:      func() error { return NewErrInvalidMaxSize(-1) },
			expectedCode: ErrCodeInvalidMaxSize,
			shouldRetry:  false,
		},
		{
			name:         "CacheFull",
			errFunc:      func() error { return NewErrCacheFull(100, 100) },
			expectedCode: ErrCodeCacheFull,
			shouldRetry:  true,
		},
		{
			name:         "KeyNotFound",
			errFunc:      func() error { return NewErrKeyNotFound("test-key") },
			expectedCode: ErrCodeKeyNotFound,
			shouldRetry:  false,
		},
		{
			name:         "LoaderTimeout",
			errFunc:      func() error { return NewErrLoaderTimeout("test-key", "5s") },
			expectedCode: ErrCodeLoaderTimeout,
			shouldRetry:  true,
		},
		{
			name:         "PanicRecovered",
			errFunc:      func() error { return NewErrPanicRecovered("test-op", "panic message") },
			expectedCode: ErrCodePanicRecovered,
			shouldRetry:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.errFunc()
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			// Check error code
			if !errors.HasCode(err, tt.expectedCode) {
				t.Errorf("expected code %s, got %s", tt.expectedCode, GetErrorCode(err))
			}

			// Check retryable
			if IsRetryable(err) != tt.shouldRetry {
				t.Errorf("expected retryable=%v, got %v", tt.shouldRetry, IsRetryable(err))
			}

			// Ensure error message is not empty
			if err.Error() == "" {
				t.Error("error message should not be empty")
			}
		})
	}
}

// Test error wrapping with cause
func TestErrorWrapping(t *testing.T) {
	cause := goerrors.New("underlying database error")

	err := NewErrLoaderFailed("test-key", cause)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Check that we can unwrap to get the cause
	unwrapped := goerrors.Unwrap(err)
	if unwrapped == nil {
		t.Fatal("expected unwrapped error, got nil")
	}

	// Check root cause
	rootCause := errors.RootCause(err)
	if rootCause.Error() != cause.Error() {
		t.Errorf("expected root cause %q, got %q", cause.Error(), rootCause.Error())
	}
}

// Test error context extraction
func TestErrorContext(t *testing.T) {
	err := NewErrCacheFull(100, 100)

	ctx := GetErrorContext(err)
	if ctx == nil {
		t.Fatal("expected context, got nil")
	}

	capacity, ok := ctx["capacity"]
	if !ok {
		t.Error("expected 'capacity' in context")
	}
	if capacity != 100 {
		t.Errorf("expected capacity=100, got %v", capacity)
	}

	size, ok := ctx["current_size"]
	if !ok {
		t.Error("expected 'current_size' in context")
	}
	if size != 100 {
		t.Errorf("expected current_size=100, got %v", size)
	}
}

// Test error category helpers
func TestErrorCategoryHelpers(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		isConfig  bool
		isOp      bool
		isLoader  bool
		isPersist bool
	}{
		{
			name:     "ConfigError",
			err:      NewErrInvalidMaxSize(0),
			isConfig: true,
		},
		{
			name: "OperationError",
			err:  NewErrCacheFull(10, 10),
			isOp: true,
		},
		{
			name:     "LoaderError",
			err:      NewErrLoaderTimeout("key", "5s"),
			isLoader: true,
		},
		{
			name:      "PersistenceError",
			err:       NewErrSaveFailed("/tmp/cache.dat", goerrors.New("disk full")),
			isPersist: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if IsConfigError(tt.err) != tt.isConfig {
				t.Errorf("IsConfigError: expected %v, got %v", tt.isConfig, IsConfigError(tt.err))
			}
			if IsOperationError(tt.err) != tt.isOp {
				t.Errorf("IsOperationError: expected %v, got %v", tt.isOp, IsOperationError(tt.err))
			}
			if IsLoaderError(tt.err) != tt.isLoader {
				t.Errorf("IsLoaderError: expected %v, got %v", tt.isLoader, IsLoaderError(tt.err))
			}
			if IsPersistenceError(tt.err) != tt.isPersist {
				t.Errorf("IsPersistenceError: expected %v, got %v", tt.isPersist, IsPersistenceError(tt.err))
			}
		})
	}
}

// Test specific error checkers
func TestSpecificErrorCheckers(t *testing.T) {
	notFoundErr := NewErrKeyNotFound("missing-key")
	if !IsNotFound(notFoundErr) {
		t.Error("IsNotFound should return true for KeyNotFound error")
	}

	fullErr := NewErrCacheFull(100, 100)
	if !IsCacheFull(fullErr) {
		t.Error("IsCacheFull should return true for CacheFull error")
	}

	// Test with nil error
	if IsNotFound(nil) {
		t.Error("IsNotFound should return false for nil error")
	}
	if IsCacheFull(nil) {
		t.Error("IsCacheFull should return false for nil error")
	}
}

// Test JSON serialization
func TestErrorJSONSerialization(t *testing.T) {
	err := NewErrCacheFull(100, 100)

	// Type assert to *errors.Error to access MarshalJSON
	var baliosErr *errors.Error
	if !goerrors.As(err, &baliosErr) {
		t.Fatal("expected *errors.Error type")
	}

	data, jsonErr := json.Marshal(baliosErr)
	if jsonErr != nil {
		t.Fatalf("JSON marshal failed: %v", jsonErr)
	}

	// Verify JSON contains expected fields
	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	if decoded["code"] != string(ErrCodeCacheFull) {
		t.Errorf("expected code %q in JSON, got %v", ErrCodeCacheFull, decoded["code"])
	}

	if decoded["message"] == "" {
		t.Error("expected non-empty message in JSON")
	}

	// Check context is present
	ctx, ok := decoded["context"].(map[string]interface{})
	if !ok {
		t.Error("expected context in JSON")
	}
	if ctx["capacity"] != float64(100) { // JSON numbers decode as float64
		t.Errorf("expected capacity=100 in context, got %v", ctx["capacity"])
	}
}

// Test error severity levels
func TestErrorSeverity(t *testing.T) {
	// Panic errors should be critical
	panicErr := NewErrPanicRecovered("test-op", "panic!")
	var baliosErr *errors.Error
	if goerrors.As(panicErr, &baliosErr) {
		if baliosErr.Severity != "critical" {
			t.Errorf("expected severity=critical, got %s", baliosErr.Severity)
		}
	}

	// Internal errors should be warning
	internalErr := NewErrInternal("test-op", nil)
	if goerrors.As(internalErr, &baliosErr) {
		if baliosErr.Severity != "warning" {
			t.Errorf("expected severity=warning, got %s", baliosErr.Severity)
		}
	}
}

// Test GetErrorCode with nil and non-balios errors
func TestGetErrorCode(t *testing.T) {
	// Nil error
	if GetErrorCode(nil) != "" {
		t.Error("expected empty string for nil error")
	}

	// Standard error
	stdErr := goerrors.New("standard error")
	if GetErrorCode(stdErr) != "" {
		t.Error("expected empty string for standard error")
	}

	// Balios error
	baliosErr := NewErrKeyNotFound("test")
	if GetErrorCode(baliosErr) != ErrCodeKeyNotFound {
		t.Errorf("expected code %s, got %s", ErrCodeKeyNotFound, GetErrorCode(baliosErr))
	}
}

// Benchmark error creation
func BenchmarkErrorCreation(b *testing.B) {
	b.Run("Simple", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = NewErrKeyNotFound("test-key")
		}
	})

	b.Run("WithContext", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = NewErrCacheFull(100, 100)
		}
	})

	b.Run("Wrapped", func(b *testing.B) {
		cause := goerrors.New("underlying error")
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = NewErrLoaderFailed("test-key", cause)
		}
	})
}

// Benchmark error checking
func BenchmarkErrorChecking(b *testing.B) {
	err := NewErrCacheFull(100, 100)

	b.Run("HasCode", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = errors.HasCode(err, ErrCodeCacheFull)
		}
	})

	b.Run("IsRetryable", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = IsRetryable(err)
		}
	})

	b.Run("GetErrorCode", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = GetErrorCode(err)
		}
	})

	b.Run("GetErrorContext", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = GetErrorContext(err)
		}
	})
}
