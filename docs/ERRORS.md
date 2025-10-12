# Balios Error Handling

Balios implements a comprehensive structured error system using the [`go-errors`](https://github.com/agilira/go-errors) library. This provides rich error context, error codes, retry semantics, and JSON serialization.

## Error Categories

Balios errors are organized into logical categories with unique error codes:

### Configuration Errors (1xxx)
- `BALIOS_INVALID_CONFIG` - Invalid configuration provided
- `BALIOS_INVALID_MAX_SIZE` - MaxSize must be > 0
- `BALIOS_INVALID_WINDOW_RATIO` - WindowRatio must be between 0.0 and 1.0
- `BALIOS_INVALID_COUNTER_BITS` - CounterBits must be between 1 and 8
- `BALIOS_INVALID_TTL` - Invalid TTL duration

### Operation Errors (2xxx)
- `BALIOS_CACHE_FULL` - Cache is full and eviction failed (retryable)
- `BALIOS_KEY_NOT_FOUND` - Key does not exist in cache
- `BALIOS_EVICTION_FAILED` - Failed to evict an entry (retryable)
- `BALIOS_SET_FAILED` - Failed to set a value (retryable)
- `BALIOS_DELETE_FAILED` - Failed to delete a value (retryable)

### Loader Errors (3xxx)
- `BALIOS_LOADER_FAILED` - Auto-loader function failed (retryable)
- `BALIOS_LOADER_TIMEOUT` - Loader timed out (retryable)
- `BALIOS_LOADER_CANCELLED` - Loader was cancelled

### Persistence Errors (4xxx)
- `BALIOS_SAVE_FAILED` - Failed to save cache to disk (retryable)
- `BALIOS_LOAD_FAILED` - Failed to load cache from disk (retryable)
- `BALIOS_CORRUPTED_DATA` - Persisted data is corrupted

### Internal Errors (5xxx)
- `BALIOS_INTERNAL_ERROR` - Internal error occurred
- `BALIOS_PANIC_RECOVERED` - Recovered from a panic

## Basic Usage

### Creating Errors

```go
// Configuration error
err := balios.NewErrInvalidMaxSize(-1)

// Operation error with context
err := balios.NewErrCacheFull(capacity, currentSize)

// Wrapping underlying errors
dbErr := someDatabase.Query()
err := balios.NewErrLoaderFailed("user:123", dbErr)
```

### Checking Errors

```go
value, err := cache.Get("key")
if err != nil {
    // Check specific error type
    if balios.IsNotFound(err) {
        // Key doesn't exist
    }
    
    // Check if retryable
    if balios.IsRetryable(err) {
        // Can retry the operation
        time.Sleep(100 * time.Millisecond)
        value, err = cache.Get("key")
    }
    
    // Check error category
    if balios.IsOperationError(err) {
        log.Warn("cache operation failed")
    }
}
```

### Extracting Error Information

```go
// Get error code
code := balios.GetErrorCode(err)
fmt.Printf("Error code: %s\n", code)

// Get error context
ctx := balios.GetErrorContext(err)
if capacity, ok := ctx["capacity"]; ok {
    fmt.Printf("Cache capacity: %v\n", capacity)
}

// Unwrap to root cause
rootCause := errors.RootCause(err)
fmt.Printf("Root cause: %v\n", rootCause)
```

## Error Properties

### Error Codes
Every Balios error has a unique string code (e.g., `BALIOS_CACHE_FULL`):

```go
import "github.com/agilira/go-errors"

// Check if error has specific code
if errors.HasCode(err, balios.ErrCodeCacheFull) {
    // Handle cache full condition
}
```

### Context Information
Errors include structured context for debugging:

```go
err := balios.NewErrCacheFull(100, 100)
// Context: {"capacity": 100, "current_size": 100}

err := balios.NewErrLoaderTimeout("user:123", "5s")
// Context: {"key": "user:123", "timeout": "5s"}
```

### Retry Semantics
Some errors are marked as retryable (transient failures):

```go
// Retryable errors:
// - BALIOS_CACHE_FULL (may have space after cleanup)
// - BALIOS_LOADER_TIMEOUT (may succeed on retry)
// - BALIOS_EVICTION_FAILED (may succeed on retry)
// - BALIOS_SET_FAILED, BALIOS_DELETE_FAILED

if balios.IsRetryable(err) {
    // Implement retry logic with backoff
}
```

### Severity Levels
Critical errors are marked with severity:

```go
// Critical severity:
// - BALIOS_PANIC_RECOVERED

// Warning severity:
// - BALIOS_INTERNAL_ERROR
```

## JSON Serialization

Errors can be serialized to JSON for logging or API responses:

```go
import (
    "encoding/json"
    "github.com/agilira/go-errors"
)

err := balios.NewErrCacheFull(100, 100)

// Type assert to *errors.Error
var baliosErr *errors.Error
if errors.As(err, &baliosErr) {
    data, _ := json.Marshal(baliosErr)
    fmt.Println(string(data))
}

// Output:
// {
//   "code": "BALIOS_CACHE_FULL",
//   "message": "Cache is full and eviction failed",
//   "context": {"capacity": 100, "current_size": 100},
//   "timestamp": "2025-01-15T10:30:00Z",
//   "retryable": true
// }
```

## Helper Functions

### Category Checkers

```go
// Check error categories
balios.IsConfigError(err)      // Configuration errors
balios.IsOperationError(err)   // Operation errors
balios.IsLoaderError(err)      // Loader errors
balios.IsPersistenceError(err) // Persistence errors
```

### Specific Checkers

```go
balios.IsNotFound(err)    // Key not found
balios.IsCacheFull(err)   // Cache full
balios.IsRetryable(err)   // Can retry
```

### Information Extraction

```go
code := balios.GetErrorCode(err)       // Get error code
ctx := balios.GetErrorContext(err)     // Get context map
```

## Best Practices

1. **Always check for retryable errors** before implementing retry logic
2. **Extract context** for detailed logging and debugging
3. **Use category checkers** for high-level error handling
4. **Serialize to JSON** for structured logging in production
5. **Check error codes** for precise error handling logic
6. **Unwrap to root cause** to find underlying issues

## Performance

Error creation and checking are highly optimized:

```
BenchmarkErrorCreation/Simple-8            11677206    112.0 ns/op    208 B/op    2 allocs/op
BenchmarkErrorCreation/WithContext-8        5089401    272.5 ns/op    496 B/op    3 allocs/op
BenchmarkErrorChecking/HasCode-8          304751919      3.871 ns/op    0 B/op    0 allocs/op
BenchmarkErrorChecking/GetErrorCode-8       4402010    265.5 ns/op     16 B/op    1 allocs/op
```

Key points:
- **Simple errors**: 112 ns, 2 allocations
- **Error code checking**: 3.8 ns, zero allocations
- **Context errors**: 272 ns, 3 allocations

## Examples

### Handling Cache Operations

```go
func GetUser(id string) (*User, error) {
    value, found := cache.Get(id)
    if !found {
        // Load from database
        user, err := db.GetUser(id)
        if err != nil {
            return nil, balios.NewErrLoaderFailed(id, err)
        }
        
        // Try to cache with retry
        for retries := 0; retries < 3; retries++ {
            if ok := cache.Set(id, user, 0); ok {
                break
            }
            
            // Cache might be full, brief pause
            time.Sleep(10 * time.Millisecond)
        }
        
        return user, nil
    }
    
    return value.(*User), nil
}
```

### Structured Logging

```go
import (
    "log/slog"
    "github.com/agilira/go-errors"
)

err := balios.NewErrCacheFull(100, 100)

var baliosErr *errors.Error
if errors.As(err, &baliosErr) {
    slog.Error("cache operation failed",
        "error_code", baliosErr.Code,
        "message", baliosErr.Message,
        "context", baliosErr.Context,
        "retryable", baliosErr.IsRetryable(),
        "timestamp", baliosErr.Timestamp,
    )
}
```

### Error Recovery

```go
func SafeCacheOperation() (err error) {
    defer func() {
        if r := recover(); r != nil {
            err = balios.NewErrPanicRecovered("cache_operation", r)
            log.Error("recovered from panic", "error", err)
        }
    }()
    
    // Potentially panicking code
    cache.Set("key", value, 0)
    return nil
}
```

## Migration Guide

If you have existing error handling code, here's how to migrate:

### Before (Simple Errors)
```go
if !cache.Set(key, value, 0) {
    return fmt.Errorf("failed to set cache value")
}
```

### After (Structured Errors)
```go
if !cache.Set(key, value, 0) {
    return balios.NewErrSetFailed(key, "cache full or eviction failed")
}
```

### Before (Simple Checks)
```go
_, found := cache.Get(key)
if !found {
    return errors.New("key not found")
}
```

### After (Structured Checks)
```go
value, found := cache.Get(key)
if !found {
    return balios.NewErrKeyNotFound(key)
}

// Or check error type:
if balios.IsNotFound(err) {
    // Handle not found
}
```

## See Also

- [go-errors Documentation](https://github.com/agilira/go-errors)
- [Balios API Reference](API.md)
- [Error Handling Best Practices](https://go.dev/blog/error-handling-and-go)
