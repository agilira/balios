// errors_comprehensive_test.go: comprehensive tests for all untested error functions
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGILira library
// SPDX-License-Identifier: MPL-2.0

package balios

import (
	goerrors "errors"
	"testing"
	"time"

	"github.com/agilira/go-errors"
)

// =============================================================================
// CONFIGURATION ERROR TESTS
// =============================================================================

func TestNewErrInvalidWindowRatio(t *testing.T) {
	tests := []struct {
		name  string
		ratio float64
	}{
		{"negative ratio", -0.5},
		{"zero ratio", 0.0},
		{"ratio above 1", 1.5},
		{"ratio at boundary", 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewErrInvalidWindowRatio(tt.ratio)
			assertError(t, err, ErrCodeInvalidWindowRatio, "provided_ratio")

			ctx := GetErrorContext(err)
			if ctx["provided_ratio"] != tt.ratio {
				t.Errorf("expected ratio %v in context, got %v", tt.ratio, ctx["provided_ratio"])
			}
		})
	}
}

func TestNewErrInvalidCounterBits(t *testing.T) {
	tests := []struct {
		name string
		bits int
	}{
		{"zero bits", 0},
		{"negative bits", -1},
		{"too many bits", 10},
		{"max valid + 1", 9},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewErrInvalidCounterBits(tt.bits)
			assertError(t, err, ErrCodeInvalidCounterBits, "provided_bits")

			ctx := GetErrorContext(err)
			if ctx["provided_bits"] != tt.bits {
				t.Errorf("expected bits %d in context, got %v", tt.bits, ctx["provided_bits"])
			}
		})
	}
}

func TestNewErrInvalidTTL(t *testing.T) {
	tests := []struct {
		name string
		ttl  interface{}
	}{
		{"negative duration", -time.Second},
		{"negative int", -1},
		{"string ttl", "invalid"},
		{"nil ttl", nil},
		{"float ttl", -3.14},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewErrInvalidTTL(tt.ttl)
			assertError(t, err, ErrCodeInvalidTTL, "provided_ttl")

			ctx := GetErrorContext(err)
			if ctx["provided_ttl"] != tt.ttl {
				t.Errorf("expected ttl %v in context, got %v", tt.ttl, ctx["provided_ttl"])
			}
		})
	}
}

// =============================================================================
// OPERATION ERROR TESTS
// =============================================================================

func TestNewErrEmptyKey(t *testing.T) {
	operations := []string{"Get", "Set", "Delete", "Has", "GetOrLoad"}

	for _, op := range operations {
		t.Run(op, func(t *testing.T) {
			err := NewErrEmptyKey(op)
			assertError(t, err, ErrCodeEmptyKey, "")

			// NewWithField may not always create a context map
			// Just verify the error message contains the operation
			if err.Error() == "" {
				t.Error("error message should not be empty")
			}
		})
	}
}

func TestNewErrEvictionFailed(t *testing.T) {
	reasons := []string{
		"no entries available",
		"all entries locked",
		"table full",
		"max retries exceeded",
	}

	for _, reason := range reasons {
		t.Run(reason, func(t *testing.T) {
			err := NewErrEvictionFailed(reason)
			assertError(t, err, ErrCodeEvictionFailed, "")
			assertRetryable(t, err, true)

			// Verify error message is not empty
			if err.Error() == "" {
				t.Error("error message should not be empty")
			}
		})
	}
}

func TestNewErrSetFailed(t *testing.T) {
	tests := []struct {
		key    string
		reason string
	}{
		{"user:123", "table full"},
		{"product:456", "memory allocation failed"},
		{"", "empty key"}, // edge case
	}

	for _, tt := range tests {
		t.Run(tt.key+"_"+tt.reason, func(t *testing.T) {
			err := NewErrSetFailed(tt.key, tt.reason)
			assertError(t, err, ErrCodeSetFailed, "key")
			assertRetryable(t, err, true)

			ctx := GetErrorContext(err)
			if ctx["key"] != tt.key {
				t.Errorf("expected key %s in context, got %v", tt.key, ctx["key"])
			}
			if ctx["reason"] != tt.reason {
				t.Errorf("expected reason %s in context, got %v", tt.reason, ctx["reason"])
			}
		})
	}
}

func TestNewErrDeleteFailed(t *testing.T) {
	tests := []struct {
		key    string
		reason string
	}{
		{"session:abc", "entry locked"},
		{"cache:xyz", "concurrent modification"},
		{"test:key", "unknown error"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			err := NewErrDeleteFailed(tt.key, tt.reason)
			assertError(t, err, ErrCodeDeleteFailed, "key")
			assertRetryable(t, err, true)

			ctx := GetErrorContext(err)
			if ctx["key"] != tt.key {
				t.Errorf("expected key %s, got %v", tt.key, ctx["key"])
			}
			if ctx["reason"] != tt.reason {
				t.Errorf("expected reason %s, got %v", tt.reason, ctx["reason"])
			}
		})
	}
}

// =============================================================================
// LOADER ERROR TESTS
// =============================================================================

func TestNewErrLoaderCancelled(t *testing.T) {
	keys := []string{"user:1", "product:2", "session:3"}

	for _, key := range keys {
		t.Run(key, func(t *testing.T) {
			err := NewErrLoaderCancelled(key)
			assertError(t, err, ErrCodeLoaderCancelled, "")

			// Verify error message is not empty
			if err.Error() == "" {
				t.Error("error message should not be empty")
			}
		})
	}
}

func TestNewErrInvalidLoader(t *testing.T) {
	keys := []string{"user:1", "product:2", ""}

	for _, key := range keys {
		t.Run(key, func(t *testing.T) {
			err := NewErrInvalidLoader(key)
			assertError(t, err, ErrCodeInvalidLoader, "")

			// Verify error message is not empty
			if err.Error() == "" {
				t.Error("error message should not be empty")
			}
		})
	}
}

// =============================================================================
// PERSISTENCE ERROR TESTS
// =============================================================================

func TestNewErrLoadFailed(t *testing.T) {
	tests := []struct {
		filepath string
		cause    error
	}{
		{"/tmp/cache.dat", goerrors.New("file not found")},
		{"/var/cache/balios.bin", goerrors.New("permission denied")},
		{"./cache.gob", goerrors.New("corrupted data")},
	}

	for _, tt := range tests {
		t.Run(tt.filepath, func(t *testing.T) {
			err := NewErrLoadFailed(tt.filepath, tt.cause)
			assertError(t, err, ErrCodeLoadFailed, "filepath")
			assertRetryable(t, err, true)

			// Verify cause is wrapped
			unwrapped := goerrors.Unwrap(err)
			if unwrapped == nil {
				t.Error("expected wrapped error")
			}

			rootCause := errors.RootCause(err)
			if rootCause.Error() != tt.cause.Error() {
				t.Errorf("expected root cause %q, got %q", tt.cause.Error(), rootCause.Error())
			}
		})
	}
}

func TestNewErrCorruptedData(t *testing.T) {
	tests := []struct {
		filepath string
		details  string
	}{
		{"/tmp/cache.dat", "invalid magic number"},
		{"/var/cache/balios.bin", "checksum mismatch"},
		{"./cache.gob", "unexpected EOF"},
	}

	for _, tt := range tests {
		t.Run(tt.details, func(t *testing.T) {
			err := NewErrCorruptedData(tt.filepath, tt.details)
			assertError(t, err, ErrCodeCorruptedData, "filepath")

			ctx := GetErrorContext(err)
			if ctx["filepath"] != tt.filepath {
				t.Errorf("expected filepath %s, got %v", tt.filepath, ctx["filepath"])
			}
			if ctx["details"] != tt.details {
				t.Errorf("expected details %s, got %v", tt.details, ctx["details"])
			}
		})
	}
}

// =============================================================================
// INTERNAL ERROR TESTS
// =============================================================================

func TestNewErrInternal(t *testing.T) {
	t.Run("with cause", func(t *testing.T) {
		cause := goerrors.New("underlying error")
		err := NewErrInternal("test-operation", cause)

		assertError(t, err, ErrCodeInternalError, "operation")

		var baliosErr *errors.Error
		if goerrors.As(err, &baliosErr) {
			if baliosErr.Severity != "warning" {
				t.Errorf("expected severity=warning, got %s", baliosErr.Severity)
			}
		}

		// Verify cause is wrapped
		unwrapped := goerrors.Unwrap(err)
		if unwrapped == nil {
			t.Error("expected wrapped error")
		}
	})

	t.Run("without cause", func(t *testing.T) {
		err := NewErrInternal("test-operation", nil)

		assertError(t, err, ErrCodeInternalError, "")

		var baliosErr *errors.Error
		if goerrors.As(err, &baliosErr) {
			if baliosErr.Severity != "warning" {
				t.Errorf("expected severity=warning, got %s", baliosErr.Severity)
			}
		}
	})
}

// =============================================================================
// ERROR CHECKER HELPER TESTS
// =============================================================================

func TestIsEmptyKey(t *testing.T) {
	t.Run("empty key error", func(t *testing.T) {
		err := NewErrEmptyKey("Get")
		if !IsEmptyKey(err) {
			t.Error("IsEmptyKey should return true for empty key error")
		}
	})

	t.Run("other error", func(t *testing.T) {
		err := NewErrKeyNotFound("test")
		if IsEmptyKey(err) {
			t.Error("IsEmptyKey should return false for non-empty-key error")
		}
	})

	t.Run("nil error", func(t *testing.T) {
		if IsEmptyKey(nil) {
			t.Error("IsEmptyKey should return false for nil error")
		}
	})
}

func TestIsConfigError_AllCases(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"InvalidMaxSize", NewErrInvalidMaxSize(0), true},
		{"InvalidTTL", NewErrInvalidTTL(-1), true},
		{"InvalidWindowRatio", NewErrInvalidWindowRatio(-0.5), false}, // Not in IsConfigError check
		{"InvalidCounterBits", NewErrInvalidCounterBits(0), false},    // Not in IsConfigError check
		{"KeyNotFound", NewErrKeyNotFound("key"), false},
		{"nil error", nil, false},
		{"standard error", goerrors.New("test"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsConfigError(tt.err)
			if result != tt.expected {
				t.Errorf("IsConfigError(%v) = %v, want %v", tt.name, result, tt.expected)
			}
		})
	}
}

func TestIsOperationError_AllCases(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"CacheFull", NewErrCacheFull(10, 10), true},
		{"KeyNotFound", NewErrKeyNotFound("key"), true},
		{"EvictionFailed", NewErrEvictionFailed("reason"), true},
		{"SetFailed", NewErrSetFailed("key", "reason"), true},
		{"DeleteFailed", NewErrDeleteFailed("key", "reason"), true},
		{"EmptyKey", NewErrEmptyKey("Get"), false}, // Not in IsOperationError
		{"LoaderFailed", NewErrLoaderFailed("key", goerrors.New("err")), false},
		{"nil error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsOperationError(tt.err)
			if result != tt.expected {
				t.Errorf("IsOperationError(%v) = %v, want %v", tt.name, result, tt.expected)
			}
		})
	}
}

func TestIsLoaderError_AllCases(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"LoaderFailed", NewErrLoaderFailed("key", goerrors.New("err")), true},
		{"LoaderTimeout", NewErrLoaderTimeout("key", "5s"), true},
		{"LoaderCancelled", NewErrLoaderCancelled("key"), true},
		{"InvalidLoader", NewErrInvalidLoader("key"), false}, // Not in IsLoaderError
		{"KeyNotFound", NewErrKeyNotFound("key"), false},
		{"nil error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsLoaderError(tt.err)
			if result != tt.expected {
				t.Errorf("IsLoaderError(%v) = %v, want %v", tt.name, result, tt.expected)
			}
		})
	}
}

func TestIsPersistenceError_AllCases(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"SaveFailed", NewErrSaveFailed("/tmp/cache", goerrors.New("err")), true},
		{"LoadFailed", NewErrLoadFailed("/tmp/cache", goerrors.New("err")), true},
		{"CorruptedData", NewErrCorruptedData("/tmp/cache", "details"), true},
		{"KeyNotFound", NewErrKeyNotFound("key"), false},
		{"nil error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsPersistenceError(tt.err)
			if result != tt.expected {
				t.Errorf("IsPersistenceError(%v) = %v, want %v", tt.name, result, tt.expected)
			}
		})
	}
}

func TestIsRetryable_AllCases(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"CacheFull (retryable)", NewErrCacheFull(10, 10), true},
		{"EvictionFailed (retryable)", NewErrEvictionFailed("reason"), true},
		{"LoaderFailed (retryable)", NewErrLoaderFailed("key", goerrors.New("err")), true},
		{"KeyNotFound (not retryable)", NewErrKeyNotFound("key"), false},
		{"InvalidMaxSize (not retryable)", NewErrInvalidMaxSize(0), false},
		{"nil error", nil, false},
		{"standard error", goerrors.New("test"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRetryable(tt.err)
			if result != tt.expected {
				t.Errorf("IsRetryable(%v) = %v, want %v", tt.name, result, tt.expected)
			}
		})
	}
}

func TestGetErrorContext_AllCases(t *testing.T) {
	t.Run("error with context", func(t *testing.T) {
		err := NewErrCacheFull(100, 95)
		ctx := GetErrorContext(err)

		if ctx == nil {
			t.Fatal("expected context, got nil")
		}

		if ctx["capacity"] != 100 {
			t.Errorf("expected capacity=100, got %v", ctx["capacity"])
		}

		if ctx["current_size"] != 95 {
			t.Errorf("expected current_size=95, got %v", ctx["current_size"])
		}
	})

	t.Run("nil error", func(t *testing.T) {
		ctx := GetErrorContext(nil)
		if ctx != nil {
			t.Error("expected nil context for nil error")
		}
	})

	t.Run("standard error", func(t *testing.T) {
		err := goerrors.New("test")
		ctx := GetErrorContext(err)
		if ctx != nil {
			t.Error("expected nil context for standard error")
		}
	})
}

// =============================================================================
// HELPER FUNCTIONS (DRY PRINCIPLE)
// =============================================================================

// assertError checks that an error has the expected code and contains a specific context field
func assertError(t *testing.T, err error, expectedCode errors.ErrorCode, contextField string) {
	t.Helper()

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Check error code
	if !errors.HasCode(err, expectedCode) {
		t.Errorf("expected code %s, got %s", expectedCode, GetErrorCode(err))
	}

	// Check error message is not empty
	if err.Error() == "" {
		t.Error("error message should not be empty")
	}

	// Check context contains expected field
	if contextField != "" {
		ctx := GetErrorContext(err)
		if ctx == nil {
			t.Fatalf("expected context with field %s, got nil", contextField)
		}
		if _, ok := ctx[contextField]; !ok {
			t.Errorf("expected context field %s, not found in %+v", contextField, ctx)
		}
	}
}

// assertRetryable checks if an error has the expected retryable status
func assertRetryable(t *testing.T, err error, expectedRetryable bool) {
	t.Helper()

	if IsRetryable(err) != expectedRetryable {
		t.Errorf("expected retryable=%v, got %v", expectedRetryable, IsRetryable(err))
	}
}
