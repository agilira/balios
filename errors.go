// errors.go: comprehensive error handling for balios cache operations
//
// This file provides structured error types using the go-errors library,
// enabling rich error context, categorization, and standardized error codes
// for all cache operations.
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGILira library
// SPDX-License-Identifier: MPL-2.0
package balios

import (
	goerrors "errors"
	"fmt"

	"github.com/agilira/go-errors"
)

// Error codes for Balios cache operations
const (
	// Configuration errors (1xxx)
	ErrCodeInvalidConfig      errors.ErrorCode = "BALIOS_INVALID_CONFIG"
	ErrCodeInvalidMaxSize     errors.ErrorCode = "BALIOS_INVALID_MAX_SIZE"
	ErrCodeInvalidWindowRatio errors.ErrorCode = "BALIOS_INVALID_WINDOW_RATIO"
	ErrCodeInvalidCounterBits errors.ErrorCode = "BALIOS_INVALID_COUNTER_BITS"
	ErrCodeInvalidTTL         errors.ErrorCode = "BALIOS_INVALID_TTL"

	// Operation errors (2xxx)
	ErrCodeCacheFull      errors.ErrorCode = "BALIOS_CACHE_FULL"
	ErrCodeKeyNotFound    errors.ErrorCode = "BALIOS_KEY_NOT_FOUND"
	ErrCodeEmptyKey       errors.ErrorCode = "BALIOS_EMPTY_KEY"
	ErrCodeEvictionFailed errors.ErrorCode = "BALIOS_EVICTION_FAILED"
	ErrCodeSetFailed      errors.ErrorCode = "BALIOS_SET_FAILED"
	ErrCodeDeleteFailed   errors.ErrorCode = "BALIOS_DELETE_FAILED"

	// Loader errors (3xxx)
	ErrCodeLoaderFailed    errors.ErrorCode = "BALIOS_LOADER_FAILED"
	ErrCodeLoaderTimeout   errors.ErrorCode = "BALIOS_LOADER_TIMEOUT"
	ErrCodeLoaderCancelled errors.ErrorCode = "BALIOS_LOADER_CANCELLED"
	ErrCodeInvalidLoader   errors.ErrorCode = "BALIOS_INVALID_LOADER"

	// Persistence errors (4xxx)
	ErrCodeSaveFailed    errors.ErrorCode = "BALIOS_SAVE_FAILED"
	ErrCodeLoadFailed    errors.ErrorCode = "BALIOS_LOAD_FAILED"
	ErrCodeCorruptedData errors.ErrorCode = "BALIOS_CORRUPTED_DATA"

	// Internal errors (5xxx)
	ErrCodeInternalError  errors.ErrorCode = "BALIOS_INTERNAL_ERROR"
	ErrCodePanicRecovered errors.ErrorCode = "BALIOS_PANIC_RECOVERED"
)

// Common error messages
const (
	msgInvalidMaxSize     = "invalid max size: must be greater than 0"
	msgInvalidWindowRatio = "invalid window ratio: must be between 0.0 and 1.0"
	msgInvalidCounterBits = "invalid counter bits: must be between 1 and 8"
	msgInvalidTTL         = "invalid TTL: must be non-negative"
	msgCacheFull          = "cache is full and eviction failed"
	msgKeyNotFound        = "key not found in cache"
	msgEmptyKey           = "key cannot be empty"
	msgEvictionFailed     = "failed to evict entry from cache"
	msgSetFailed          = "failed to set key-value pair"
	msgDeleteFailed       = "failed to delete key"
	msgLoaderFailed       = "loader function failed"
	msgLoaderTimeout      = "loader function timed out"
	msgLoaderCancelled    = "loader function was cancelled"
	msgInvalidLoader      = "loader function cannot be nil"
	msgSaveFailed         = "failed to save cache to file"
	msgLoadFailed         = "failed to load cache from file"
	msgCorruptedData      = "corrupted cache data"
	msgInternalError      = "internal cache error"
	msgPanicRecovered     = "panic recovered in cache operation"
)

// =============================================================================
// CONFIGURATION ERRORS
// =============================================================================

// NewErrInvalidMaxSize creates an error for invalid max size
func NewErrInvalidMaxSize(size int) error {
	return errors.NewWithContext(ErrCodeInvalidMaxSize, msgInvalidMaxSize, map[string]interface{}{
		"provided_size":    size,
		"minimum_required": 1,
	})
}

// NewErrInvalidWindowRatio creates an error for invalid window ratio
func NewErrInvalidWindowRatio(ratio float64) error {
	return errors.NewWithContext(ErrCodeInvalidWindowRatio, msgInvalidWindowRatio, map[string]interface{}{
		"provided_ratio": ratio,
		"valid_range":    "0.0 < ratio < 1.0",
	})
}

// NewErrInvalidCounterBits creates an error for invalid counter bits
func NewErrInvalidCounterBits(bits int) error {
	return errors.NewWithContext(ErrCodeInvalidCounterBits, msgInvalidCounterBits, map[string]interface{}{
		"provided_bits": bits,
		"valid_range":   "1-8",
	})
}

// NewErrInvalidTTL creates an error for invalid TTL
func NewErrInvalidTTL(ttl interface{}) error {
	return errors.NewWithContext(ErrCodeInvalidTTL, msgInvalidTTL, map[string]interface{}{
		"provided_ttl": ttl,
	})
}

// =============================================================================
// OPERATION ERRORS
// =============================================================================

// NewErrCacheFull creates an error when cache is full and eviction fails
func NewErrCacheFull(capacity int, size int) error {
	return errors.NewWithContext(ErrCodeCacheFull, msgCacheFull, map[string]interface{}{
		"capacity":     capacity,
		"current_size": size,
	}).AsRetryable() // Can be retried after some items expire
}

// NewErrKeyNotFound creates an error when key is not found
func NewErrKeyNotFound(key string) error {
	return errors.NewWithField(ErrCodeKeyNotFound, msgKeyNotFound, "key", key)
}

// NewErrEmptyKey creates an error when key is empty
func NewErrEmptyKey(operation string) error {
	return errors.NewWithField(ErrCodeEmptyKey, msgEmptyKey, "operation", operation)
}

// NewErrEvictionFailed creates an error when eviction fails
func NewErrEvictionFailed(reason string) error {
	return errors.NewWithField(ErrCodeEvictionFailed, msgEvictionFailed, "reason", reason).
		AsRetryable()
}

// NewErrSetFailed creates an error when Set operation fails
func NewErrSetFailed(key string, reason string) error {
	return errors.NewWithContext(ErrCodeSetFailed, msgSetFailed, map[string]interface{}{
		"key":    key,
		"reason": reason,
	}).AsRetryable()
}

// NewErrDeleteFailed creates an error when Delete operation fails
func NewErrDeleteFailed(key string, reason string) error {
	return errors.NewWithContext(ErrCodeDeleteFailed, msgDeleteFailed, map[string]interface{}{
		"key":    key,
		"reason": reason,
	}).AsRetryable()
}

// =============================================================================
// LOADER ERRORS
// =============================================================================

// NewErrLoaderFailed creates an error when loader function fails
func NewErrLoaderFailed(key string, cause error) error {
	return errors.Wrap(cause, ErrCodeLoaderFailed, msgLoaderFailed).
		WithContext("key", key).
		AsRetryable()
}

// NewErrLoaderTimeout creates an error when loader times out
func NewErrLoaderTimeout(key string, timeout interface{}) error {
	return errors.NewWithContext(ErrCodeLoaderTimeout, msgLoaderTimeout, map[string]interface{}{
		"key":     key,
		"timeout": timeout,
	}).AsRetryable()
}

// NewErrLoaderCancelled creates an error when loader is cancelled
func NewErrLoaderCancelled(key string) error {
	return errors.NewWithField(ErrCodeLoaderCancelled, msgLoaderCancelled, "key", key)
}

// NewErrInvalidLoader creates an error when loader function is nil
func NewErrInvalidLoader(key string) error {
	return errors.NewWithField(ErrCodeInvalidLoader, msgInvalidLoader, "key", key)
}

// =============================================================================
// PERSISTENCE ERRORS
// =============================================================================

// NewErrSaveFailed creates an error when save operation fails
func NewErrSaveFailed(filepath string, cause error) error {
	return errors.Wrap(cause, ErrCodeSaveFailed, msgSaveFailed).
		WithContext("filepath", filepath).
		AsRetryable()
}

// NewErrLoadFailed creates an error when load operation fails
func NewErrLoadFailed(filepath string, cause error) error {
	return errors.Wrap(cause, ErrCodeLoadFailed, msgLoadFailed).
		WithContext("filepath", filepath).
		AsRetryable()
}

// NewErrCorruptedData creates an error when data is corrupted
func NewErrCorruptedData(filepath string, details string) error {
	return errors.NewWithContext(ErrCodeCorruptedData, msgCorruptedData, map[string]interface{}{
		"filepath": filepath,
		"details":  details,
	})
}

// =============================================================================
// INTERNAL ERRORS
// =============================================================================

// NewErrInternal creates a generic internal error
func NewErrInternal(operation string, cause error) error {
	if cause != nil {
		return errors.Wrap(cause, ErrCodeInternalError, msgInternalError).
			WithContext("operation", operation).
			WithSeverity("warning")
	}
	return errors.NewWithField(ErrCodeInternalError, msgInternalError, "operation", operation).
		WithSeverity("warning")
}

// NewErrPanicRecovered creates an error when a panic is recovered
func NewErrPanicRecovered(operation string, panicValue interface{}) error {
	return errors.NewWithContext(ErrCodePanicRecovered, msgPanicRecovered, map[string]interface{}{
		"operation":   operation,
		"panic_value": fmt.Sprintf("%v", panicValue),
	}).WithSeverity("critical")
}

// =============================================================================
// ERROR CHECKING HELPERS
// =============================================================================

// IsNotFound checks if error is a key not found error
func IsNotFound(err error) bool {
	return errors.HasCode(err, ErrCodeKeyNotFound)
}

// IsEmptyKey checks if error is an empty key error
func IsEmptyKey(err error) bool {
	return errors.HasCode(err, ErrCodeEmptyKey)
}

// IsCacheFull checks if error is a cache full error
func IsCacheFull(err error) bool {
	return errors.HasCode(err, ErrCodeCacheFull)
}

// IsConfigError checks if error is a configuration error
func IsConfigError(err error) bool {
	if err == nil {
		return false
	}
	// Check if error implements ErrorCoder interface
	var coder errors.ErrorCoder
	if goerrors.As(err, &coder) {
		code := coder.ErrorCode()
		// Config error codes start with "BALIOS_INVALID_"
		return len(code) > 7 && code[:7] == "BALIOS_" && (code == ErrCodeInvalidMaxSize ||
			code == ErrCodeInvalidTTL || code == ErrCodeInvalidConfig)
	}
	return false
}

// IsOperationError checks if error is an operation error
func IsOperationError(err error) bool {
	if err == nil {
		return false
	}
	var coder errors.ErrorCoder
	if goerrors.As(err, &coder) {
		code := coder.ErrorCode()
		// Operation errors: BALIOS_CACHE_FULL, BALIOS_KEY_NOT_FOUND, etc.
		return code == ErrCodeCacheFull || code == ErrCodeKeyNotFound ||
			code == ErrCodeEvictionFailed || code == ErrCodeSetFailed || code == ErrCodeDeleteFailed
	}
	return false
}

// IsLoaderError checks if error is a loader error
func IsLoaderError(err error) bool {
	if err == nil {
		return false
	}
	var coder errors.ErrorCoder
	if goerrors.As(err, &coder) {
		code := coder.ErrorCode()
		// Loader errors: BALIOS_LOADER_*
		return code == ErrCodeLoaderFailed || code == ErrCodeLoaderTimeout || code == ErrCodeLoaderCancelled
	}
	return false
}

// IsPersistenceError checks if error is a persistence error
func IsPersistenceError(err error) bool {
	if err == nil {
		return false
	}
	var coder errors.ErrorCoder
	if goerrors.As(err, &coder) {
		code := coder.ErrorCode()
		// Persistence errors: BALIOS_SAVE_FAILED, BALIOS_LOAD_FAILED, BALIOS_CORRUPTED_DATA
		return code == ErrCodeSaveFailed || code == ErrCodeLoadFailed || code == ErrCodeCorruptedData
	}
	return false
}

// IsRetryable checks if the error can be retried
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	// Check if error implements Retryable interface
	var retryable errors.Retryable
	if goerrors.As(err, &retryable) {
		return retryable.IsRetryable()
	}
	return false
}

// GetErrorCode extracts the error code from an error
func GetErrorCode(err error) errors.ErrorCode {
	if err == nil {
		return ""
	}
	var coder errors.ErrorCoder
	if goerrors.As(err, &coder) {
		return coder.ErrorCode()
	}
	return ""
}

// GetErrorContext extracts context from an error
func GetErrorContext(err error) map[string]interface{} {
	if err == nil {
		return nil
	}
	// Type assert to *errors.Error to access Context field
	var baliosErr *errors.Error
	if goerrors.As(err, &baliosErr) {
		return baliosErr.Context
	}
	return nil
}
