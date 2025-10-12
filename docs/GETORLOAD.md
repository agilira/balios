# GetOrLoad API Documentation

Complete guide to Balios' automatic loading feature with cache stampede prevention.

## Overview

The `GetOrLoad()` API provides automatic value loading with **singleflight pattern**, preventing the cache stampede problem where multiple concurrent requests trigger redundant expensive operations.

## The Problem: Cache Stampede

```go
// WRONG: Without GetOrLoad
func getUser(id int) (User, error) {
    // 1000 concurrent requests for same missing key
    if user, found := cache.Get(id); found {
        return user, nil
    }
    
    // ALL 1000 goroutines hit the database!
    user, err := fetchFromDB(id)
    if err != nil {
        return User{}, err
    }
    
    cache.Set(id, user)
    return user, nil
}
```

**Problem**: If key is not in cache, all concurrent requests execute the expensive loader.

## The Solution: Singleflight

```go
// CORRECT: With GetOrLoad
func getUser(id int) (User, error) {
    // Only ONE goroutine executes loader
    // Others wait and receive the same result
    return cache.GetOrLoad(id, func() (User, error) {
        return fetchFromDB(id)
    })
}
```

**Benefit**: 1000 concurrent requests = 1 database call (1000x efficiency!)

## API Reference

### GenericCache

```go
// GetOrLoad returns cached value or loads it using the provided loader.
// Multiple concurrent requests for same key execute loader only ONCE.
func (c *GenericCache[K, V]) GetOrLoad(
    key K,
    loader func() (V, error),
) (V, error)

// GetOrLoadWithContext is context-aware version with timeout/cancellation.
func (c *GenericCache[K, V]) GetOrLoadWithContext(
    ctx context.Context,
    key K,
    loader func(context.Context) (V, error),
) (V, error)
```

### Non-Generic Cache

```go
// GetOrLoad returns cached value or loads it using the provided loader.
func (c Cache) GetOrLoad(
    key string,
    loader func() (interface{}, error),
) (interface{}, error)

// GetOrLoadWithContext is context-aware version.
func (c Cache) GetOrLoadWithContext(
    ctx context.Context,
    key string,
    loader func(context.Context) (interface{}, error),
) (interface{}, error)
```

## Performance Characteristics

From real benchmarks (`loading_bench_test.go`):

| Scenario | Performance | Allocations | Notes |
|----------|-------------|-------------|-------|
| **Cache Hit** | 20.3 ns/op | 0 allocs | Same as Get() |
| **Cache Miss** | Loader time + overhead | 1 alloc | Minimal overhead |
| **Singleflight** | 1 loader call | Amortized | 1000x efficiency |

**Key Insights:**
- Cache hit performance: Identical to `Get()` (no overhead)
- Cache miss overhead: Only +3.6ns vs manual Get+Set pattern
- Concurrent efficiency: Proven 1000x with atomic counter validation

## Usage Examples

### Example 1: Basic Usage

```go
package main

import (
    "fmt"
    "time"
    "github.com/agilira/balios"
)

type User struct {
    ID    int
    Name  string
    Email string
}

func fetchUserFromDB(id int) (User, error) {
    // Simulate expensive database call
    time.Sleep(100 * time.Millisecond)
    
    return User{
        ID:    id,
        Name:  fmt.Sprintf("User%d", id),
        Email: fmt.Sprintf("user%d@example.com", id),
    }, nil
}

func main() {
    cache := balios.NewGenericCache[int, User](balios.Config{
        MaxSize: 1000,
        TTL:     5 * time.Minute,
    })
    
    // First call: cache miss, loader executes
    user, err := cache.GetOrLoad(123, func() (User, error) {
        return fetchUserFromDB(123)
    })
    if err != nil {
        panic(err)
    }
    fmt.Printf("Loaded: %s\n", user.Name)
    
    // Second call: cache hit, loader NOT called
    user, _ = cache.GetOrLoad(123, func() (User, error) {
        panic("This should never be called!")
    })
    fmt.Printf("Cached: %s\n", user.Name)
}
```

### Example 2: Cache Stampede Prevention

Demonstrates singleflight effectiveness:

```go
func demonstrateSingleflight() {
    cache := balios.NewGenericCache[int, User](balios.Config{
        MaxSize: 1000,
    })
    
    userID := 200
    numGoroutines := 100
    
    var wg sync.WaitGroup
    start := time.Now()
    
    // Launch 100 concurrent requests for same key
    for i := 0; i < numGoroutines; i++ {
        wg.Add(1)
        go func(goroutineID int) {
            defer wg.Done()
            
            // All goroutines call GetOrLoad
            user, err := cache.GetOrLoad(userID, func() (User, error) {
                return fetchUserFromDB(userID)
            })
            
            if err != nil {
                fmt.Printf("Goroutine %d failed: %v\n", goroutineID, err)
                return
            }
            
            fmt.Printf("Goroutine %d got user: %s\n", goroutineID, user.Name)
        }(i)
    }
    
    wg.Wait()
    elapsed := time.Since(start)
    
    fmt.Printf("All %d requests completed in %v\n", numGoroutines, elapsed)
    fmt.Println("Database accessed only ONCE (not 100 times!) thanks to singleflight")
}
```

**Expected Output:**
```
Goroutine 0 got user: User200
Goroutine 1 got user: User200
... (98 more)
All 100 requests completed in 101ms
Database accessed only ONCE (not 100 times!) thanks to singleflight
```

### Example 3: Context with Timeout

Respect context deadlines:

```go
func contextWithTimeout() {
    cache := balios.NewGenericCache[int, User](balios.Config{
        MaxSize: 1000,
    })
    
    // Attempt 1: Timeout too short (50ms < 100ms DB latency)
    ctx1, cancel1 := context.WithTimeout(context.Background(), 50*time.Millisecond)
    defer cancel1()
    
    _, err := cache.GetOrLoadWithContext(ctx1, 300, 
        func(ctx context.Context) (User, error) {
            return fetchUserFromDBWithContext(ctx, 300)
        })
    
    if err != nil {
        fmt.Printf("Expected timeout error: %v\n", err)
    }
    
    // Attempt 2: Timeout sufficient (200ms > 100ms DB latency)
    ctx2, cancel2 := context.WithTimeout(context.Background(), 200*time.Millisecond)
    defer cancel2()
    
    user, err := cache.GetOrLoadWithContext(ctx2, 300,
        func(ctx context.Context) (User, error) {
            return fetchUserFromDBWithContext(ctx, 300)
        })
    
    if err != nil {
        panic(err)
    }
    
    fmt.Printf("Success with longer timeout: %s\n", user.Name)
}
```

### Example 4: Context Cancellation

Handle explicit cancellation:

```go
func contextCancellation() {
    cache := balios.NewGenericCache[int, User](balios.Config{
        MaxSize: 1000,
    })
    
    ctx, cancel := context.WithCancel(context.Background())
    
    // Start loading in background
    go func() {
        time.Sleep(50 * time.Millisecond)
        fmt.Println("Cancelling context...")
        cancel()
    }()
    
    // This will be cancelled mid-execution
    _, err := cache.GetOrLoadWithContext(ctx, 400,
        func(ctx context.Context) (User, error) {
            return fetchUserFromDBWithContext(ctx, 400)
        })
    
    if err != nil {
        fmt.Printf("❌ Expected cancellation error: %v\n", err)
    }
}
```

### Example 5: Error Handling

Handle loader errors properly:

```go
func errorHandling() {
    cache := balios.NewGenericCache[int, User](balios.Config{
        MaxSize: 1000,
    })
    
    // Test 1: Loader returns error
    _, err := cache.GetOrLoad(500, func() (User, error) {
        return User{}, fmt.Errorf("database connection failed")
    })
    
    if err != nil {
        fmt.Printf("Error properly propagated: %v\n", err)
    }
    
    // Test 2: Nil loader (validation error)
    cache2 := balios.NewCache(balios.Config{MaxSize: 1000})
    _, err = cache2.GetOrLoad("key", nil)
    
    if err != nil {
        fmt.Printf("Validation error: %v\n", err)
    }
    
    // Test 3: Loader panics (should recover)
    _, err = cache.GetOrLoad(600, func() (User, error) {
        panic("oops!")
    })
    
    if err != nil {
        fmt.Printf("Panic recovered: %v\n", err)
        
        // Check for panic recovered error code
        if errors.HasCode(err, balios.ErrCodePanicRecovered) {
            fmt.Println("Correct error code: BALIOS_PANIC_RECOVERED")
        }
    }
}
```

## Error Handling

### Error Types

1. **Loader Errors**: Returned by your loader function
   ```go
   _, err := cache.GetOrLoad(key, func() (V, error) {
       return V{}, fmt.Errorf("DB error")
   })
   // err = "DB error"
   ```

2. **Validation Errors**: Invalid parameters
   ```go
   _, err := cache.GetOrLoad("key", nil)  // nil loader
   // err = BALIOS_INVALID_LOADER
   ```

3. **Panic Recovery**: Loader panics
   ```go
   _, err := cache.GetOrLoad(key, func() (V, error) {
       panic("oops")
   })
   // err = BALIOS_PANIC_RECOVERED
   ```

4. **Context Errors**: Timeout or cancellation
   ```go
   ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
   defer cancel()
   
   _, err := cache.GetOrLoadWithContext(ctx, key, slowLoader)
   // err = context.DeadlineExceeded
   ```

### Error Checking

```go
import "github.com/agilira/go-errors"

_, err := cache.GetOrLoad(key, loader)
if err != nil {
    // Check specific error codes
    if errors.HasCode(err, balios.ErrCodeInvalidLoader) {
        // Handle validation error
    }
    
    if errors.HasCode(err, balios.ErrCodePanicRecovered) {
        // Handle panic recovery
    }
    
    // Check context errors
    if errors.Is(err, context.DeadlineExceeded) {
        // Handle timeout
    }
}
```

### Error Caching Behavior

**Important**: Loader errors are NOT cached!

```go
// First call: loader returns error
_, err := cache.GetOrLoad(key, func() (V, error) {
    return V{}, fmt.Errorf("DB error")
})
// err = "DB error"

// Second call: loader called AGAIN (error wasn't cached)
_, err = cache.GetOrLoad(key, func() (V, error) {
    return fetchFromDB(key)  // This will execute
})
```

**Rationale**: Prevents error amplification. Temporary errors (network glitches, DB timeouts) shouldn't be cached.

## Implementation Details

### Singleflight Pattern

From `loading.go`:

```go
type inflightCall struct {
    wg  sync.WaitGroup  // Synchronization
    val atomic.Value    // Result storage (resultWrapper)
    err atomic.Value    // Error storage (errorWrapper)
}
```

**Two-Phase Pattern:**
1. **Pre-initialize**: `newFlight.wg.Add(1)` BEFORE LoadOrStore
2. **LoadOrStore**: Atomic insertion into sync.Map
3. **Execute**: First goroutine runs loader, others wait
4. **Broadcast**: `wg.Done()` wakes all waiters

**Race-Free Guarantee:**
- WaitGroup initialized before any goroutine can wait
- atomic.Value with wrappers for nil-safe storage
- All tests pass with `-race` detector

### Nil Handling

**Problem**: `atomic.Value` cannot store `nil`

**Solution**: Wrapper types
```go
type resultWrapper struct {
    value interface{}
}

type errorWrapper struct {
    err error
}

// Usage
flight.val.Store(&resultWrapper{value: val})
flight.err.Store(&errorWrapper{err: err})
```

### Panic Recovery

All loader executions wrapped in `defer recover()`:

```go
defer func() {
    if r := recover(); r != nil {
        flight.err.Store(&errorWrapper{
            err: NewErrPanicRecovered(fmt.Sprintf("%v", r)),
        })
    }
    flight.wg.Done()
}()

val, err := loader()
```

## Best Practices

### Do

1. **Use GetOrLoad for expensive operations**
   ```go
   user, _ := cache.GetOrLoad(id, func() (User, error) {
       return fetchFromDB(id)  // Expensive!
   })
   ```

2. **Use context for production code**
   ```go
   ctx, cancel := context.WithTimeout(req.Context(), 5*time.Second)
   defer cancel()
   
   user, err := cache.GetOrLoadWithContext(ctx, id, loaderWithContext)
   ```

3. **Handle errors appropriately**
   ```go
   user, err := cache.GetOrLoad(id, loader)
   if err != nil {
       if errors.HasCode(err, balios.ErrCodePanicRecovered) {
           // Log panic for investigation
           log.Error("Loader panicked", "error", err)
           return defaultUser, nil
       }
       return User{}, err
   }
   ```

### Don't

1. **Don't use GetOrLoad for cheap operations**
   ```go
   // WRONG: Loader is trivial
   val, _ := cache.GetOrLoad(key, func() (int, error) {
       return 42, nil  // Just use Set() instead!
   })
   ```

2. **Don't cache errors intentionally**
   ```go
   // WRONG: Errors aren't cached anyway
   _, err := cache.GetOrLoad(key, loader)
   if err != nil {
       // Don't try to cache errors manually
       cache.Set(key, ErrorValue{err})
   }
   ```

3. **Don't ignore context in loader**
   ```go
   // WRONG: Loader ignores context
   cache.GetOrLoadWithContext(ctx, key, func(ctx context.Context) (V, error) {
       // Ignoring ctx parameter!
       return slowOperation()
   })
   
   // CORRECT: Respect context
   cache.GetOrLoadWithContext(ctx, key, func(ctx context.Context) (V, error) {
       return slowOperationWithContext(ctx)
   })
   ```

## Testing GetOrLoad

### Test Singleflight Effectiveness

```go
func TestSingleflight(t *testing.T) {
    cache := balios.NewGenericCache[int, string](balios.Config{MaxSize: 100})
    
    var loaderCalls int32
    
    var wg sync.WaitGroup
    for i := 0; i < 1000; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            
            _, err := cache.GetOrLoad(123, func() (string, error) {
                atomic.AddInt32(&loaderCalls, 1)
                time.Sleep(10 * time.Millisecond)
                return "value", nil
            })
            
            if err != nil {
                t.Error(err)
            }
        }()
    }
    
    wg.Wait()
    
    // Verify only ONE loader call
    if atomic.LoadInt32(&loaderCalls) != 1 {
        t.Errorf("Expected 1 loader call, got %d", loaderCalls)
    }
}
```

### Test Context Cancellation

```go
func TestCancellation(t *testing.T) {
    cache := balios.NewGenericCache[int, string](balios.Config{MaxSize: 100})
    
    ctx, cancel := context.WithCancel(context.Background())
    
    go func() {
        time.Sleep(50 * time.Millisecond)
        cancel()
    }()
    
    _, err := cache.GetOrLoadWithContext(ctx, 123,
        func(ctx context.Context) (string, error) {
            select {
            case <-time.After(200 * time.Millisecond):
                return "value", nil
            case <-ctx.Done():
                return "", ctx.Err()
            }
        })
    
    if !errors.Is(err, context.Canceled) {
        t.Errorf("Expected context.Canceled, got %v", err)
    }
}
```

## Complete Example

See [examples/getorload/main.go](../examples/getorload/main.go) for a complete runnable example demonstrating all features.

Run with:
```bash
cd examples/getorload
go run main.go
```

## Performance Tuning

### Optimize Loader Performance

```go
// WRONG: Loader does unnecessary work
cache.GetOrLoad(key, func() (User, error) {
    user := fetchFromDB(key)  // Slow
    enrichUser(&user)         // Even slower!
    return user, nil
})

// BETTER: Lazy enrichment
cache.GetOrLoad(key, func() (User, error) {
    return fetchFromDB(key), nil  // Fast as possible
})
// Enrich after caching
enrichUser(&user)
```

### Batch Loading

```go
// For multiple keys, use goroutines
var wg sync.WaitGroup
users := make(map[int]User)
var mu sync.Mutex

for _, id := range userIDs {
    wg.Add(1)
    go func(id int) {
        defer wg.Done()
        
        user, err := cache.GetOrLoad(id, func() (User, error) {
            return fetchFromDB(id)
        })
        
        if err == nil {
            mu.Lock()
            users[id] = user
            mu.Unlock()
        }
    }(id)
}

wg.Wait()
```
## References

- Implementation: `loading.go`, `loading_generic.go`
- Tests: `loading_test.go`, `loading_generic_test.go`
- Benchmarks: `loading_bench_test.go`
- Example: `examples/getorload/main.go`

---

Balios • an AGILira fragment