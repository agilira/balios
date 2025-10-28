# Balios API Reference
### High-performance, thread-safe, in-memory cache for Go

## Overview

Balios is a production-ready cache implementation based on the **W-TinyLFU** (Window Tiny Least Frequently Used) algorithm, designed to deliver exceptional performance through zero-allocation operations and lock-free data structures.

## Installation

```bash
go get github.com/agilira/balios
```

## Quick Start

### Basic Usage (Generic API)

```go
import "github.com/agilira/balios"

type User struct {
    ID   int
    Name string
}

func main() {
    // Create type-safe cache
    cache := balios.NewGenericCache[string, User](balios.Config{
        MaxSize: 10_000,
        TTL:     time.Hour,
    })

    // Set and get (no type assertions needed)
    cache.Set("user:123", User{ID: 123, Name: "Alice"})
    
    if user, found := cache.Get("user:123"); found {
        fmt.Printf("Found: %s\n", user.Name)
    }

    // Check performance
    stats := cache.Stats()
    fmt.Printf("Hit ratio: %.2f%%\n", stats.HitRatio())
}
```

### Cache Stampede Prevention

```go
// Multiple concurrent requests execute loader only once
user, err := cache.GetOrLoad("user:123", func() (User, error) {
    return fetchUserFromDB(123) // Runs once even with 1000 concurrent calls
})
if err != nil {
    log.Printf("Load failed: %v", err)
}

// With timeout and cancellation
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

user, err := cache.GetOrLoadWithContext(ctx, "user:123",
    func(ctx context.Context) (User, error) {
        return fetchUserFromDBWithContext(ctx, 123)
    })
```

---

## Core API

### Cache Creation

#### `NewGenericCache[K, V](config Config) *GenericCache[K, V]`

Creates a type-safe generic cache with compile-time type checking.

**Type Parameters:**
- `K comparable` - Key type (must be comparable)
- `V any` - Value type (any type)

**Parameters:**
- `config` - Cache configuration

**Returns:** New GenericCache instance

**Example:**
```go
cache := balios.NewGenericCache[string, *User](balios.Config{
    MaxSize: 10_000,
    TTL:     30 * time.Minute,
})
```

#### `NewCache(config Config) Cache`

Creates a cache using interface{} (legacy API for compatibility).

**Prefer `NewGenericCache` for type safety.**

---

### Cache Operations

#### `Get(key K) (value V, found bool)`

Retrieves a value from the cache.

**Performance:** 113.1 ns/op, zero allocations

**Returns:**
- `value` - The stored value (zero value if not found)
- `found` - true if key exists and not expired

**Example:**
```go
user, found := cache.Get("user:123")
if !found {
    // Handle cache miss
}
```

#### `Set(key K, value V)`

Stores a key-value pair in the cache.

**Performance:** 143.5 ns/op, zero allocations

**Behavior:**
- Value stored until evicted or expired (if TTL set)
- Triggers eviction if cache is full
- Updates frequency tracking for W-TinyLFU

**Note:** GenericCache.Set() does not return a value (unlike the base Cache interface which returns bool).

**Example:**
```go
cache.Set("user:123", User{ID: 123, Name: "Alice"})
```

#### `Delete(key K)`

Removes a key from the cache.

**Note:** GenericCache.Delete() does not return a value (unlike the base Cache interface which returns bool).

**Example:**
```go
cache.Delete("user:123")
```

#### `Has(key K) bool`

Checks if a key exists without retrieving the value.

**More efficient than Get when only existence matters.**

**Example:**
```go
if cache.Has("user:123") {
    // Key exists
}
```

#### `Clear()`

Removes all entries and resets statistics.

**Example:**
```go
cache.Clear()
```

#### `ExpireNow() int`

Manually triggers expiration of all TTL-expired entries in the cache.

**Returns:** Number of entries that were expired and removed.

**Behavior:**
- Scans the entire cache and removes all entries whose TTL has expired
- Uses lock-free CAS operations for thread-safety
- Returns immediately if TTL is disabled (TTL=0)
- Safe to call concurrently with other operations
- Updates the `Expirations` metric

**Use Cases:**
- Proactive cleanup before expected traffic spikes
- Manual cache maintenance in low-traffic periods
- Integration with cron jobs or scheduled tasks
- Memory pressure mitigation

**Performance:** O(n) where n is cache capacity. Typical: ~1-5µs per 1000 entries.

**Example:**
```go
// Periodic cleanup via ticker
ticker := time.NewTicker(5 * time.Minute)
defer ticker.Stop()

for range ticker.C {
    expired := cache.ExpireNow()
    if expired > 0 {
        log.Printf("Expired %d entries", expired)
    }
}

// On-demand cleanup
if memoryPressure() {
    cache.ExpireNow()
}
```

**Note:** Balios also performs **opportunistic inline expiration** during normal operations (Get/Set/Has), so calling `ExpireNow()` manually is optional. It's most useful when you want guaranteed cleanup at specific intervals.

#### `Stats() CacheStats`

Returns current cache statistics.

**Returns:** `CacheStats` with hits, misses, sets, deletes, evictions, expirations, size, capacity

**Example:**
```go
stats := cache.Stats()
fmt.Printf("Hit Ratio: %.2f%%, Size: %d/%d\n", 
    stats.HitRatio(), stats.Size, stats.Capacity)
fmt.Printf("Evictions: %d, Expirations: %d\n",
    stats.Evictions, stats.Expirations)
```

#### `Len() int`

Returns the current number of entries in the cache.

**Example:**
```go
currentSize := cache.Len()
fmt.Printf("Cache contains %d entries\n", currentSize)
```

#### `Capacity() int`

Returns the maximum number of entries the cache can hold.

**Example:**
```go
maxSize := cache.Capacity()
fmt.Printf("Cache capacity: %d entries\n", maxSize)
```

#### `Close() error`

Gracefully shuts down the cache and releases resources.

**Example:**
```go
defer cache.Close()
```

---

### GetOrLoad API (Cache-Aside Pattern)

#### `GetOrLoad(key K, loader func() (V, error)) (V, error)`

Returns value from cache or loads it using the provided function.

**Features:**
- **Singleflight:** Multiple concurrent calls execute loader only once
- **Performance:** 20.3 ns/op on cache hit
- **Error handling:** Errors are NOT cached
- **Panic recovery:** Returns `BALIOS_PANIC_RECOVERED` error

**Parameters:**
- `key` - Cache key
- `loader` - Function to load value if not in cache

**Returns:**
- `value` - Cached or loaded value (zero value on error)
- `error` - Loader error or validation error

**Example:**
```go
value, err := cache.GetOrLoad(42, func() (string, error) {
    return fetchFromDB(42)
})
if err != nil {
    return fmt.Errorf("failed to load: %w", err)
}
```

#### `GetOrLoadWithContext(ctx context.Context, key K, loader func(context.Context) (V, error)) (V, error)`

Like GetOrLoad but respects context cancellation and timeout.

**Parameters:**
- `ctx` - Context for cancellation/timeout
- `key` - Cache key
- `loader` - Function receiving context

**Example:**
```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

value, err := cache.GetOrLoadWithContext(ctx, 42, 
    func(ctx context.Context) (string, error) {
        return fetchFromDBWithContext(ctx, 42)
    })
```

---

## Configuration

### `Config` Struct

```go
type Config struct {
    MaxSize          int                            // Required: Maximum entries
    TTL              time.Duration                  // Optional: Time-to-live (0 = no expiration)
    WindowRatio      float64                        // Optional: Window cache ratio (default: 0.01)
    CounterBits      int                            // Optional: Frequency counter bits (default: 4)
    CleanupInterval  time.Duration                  // Optional: Cleanup interval (default: TTL/10)
    Logger           Logger                         // Optional: Logger implementation
    MetricsCollector MetricsCollector               // Optional: Metrics collector
    TimeProvider     TimeProvider                   // Optional: Time provider (for testing)
    OnEvict          func(key string, value interface{}) // Optional: Eviction callback
    OnExpire         func(key string, value interface{}) // Optional: Expiration callback
}
```

### `DefaultConfig() Config`

Returns sensible defaults:
- `MaxSize: 10_000`
- `WindowRatio: 0.01` (1% window cache)
- `CounterBits: 4`
- `TTL: 0` (no expiration)

### `Validate() error`

Validates configuration and sets defaults.

**Validation Rules:**
- `MaxSize > 0` (sets `DefaultMaxSize` if invalid)
- `0.0 < WindowRatio < 1.0` (sets `DefaultWindowRatio` if invalid)
- `1 <= CounterBits <= 8` (sets `DefaultCounterBits` if invalid)
- `TTL >= 0` (no default, 0 means no expiration)
- Sets `CleanupInterval = TTL/10` if `TTL > 0` and `CleanupInterval` not set

---

## Data Types

### `CacheStats`

```go
type CacheStats struct {
    Hits        uint64  // Cache hits
    Misses      uint64  // Cache misses
    Sets        uint64  // Set operations
    Deletes     uint64  // Delete operations
    Evictions   uint64  // Evictions (capacity-based removal)
    Expirations uint64  // TTL-based expirations
    Size        int     // Current entries
    Capacity    int     // Maximum entries
}
```

**Field Descriptions:**

- **Hits**: Number of successful Get() operations
- **Misses**: Number of Get() operations that didn't find the key
- **Sets**: Total number of Set() operations
- **Deletes**: Total number of Delete() operations
- **Evictions**: Entries removed due to capacity constraints (W-TinyLFU algorithm)
- **Expirations**: Entries removed due to TTL expiration (inline or via ExpireNow())
- **Size**: Current number of entries in cache
- **Capacity**: Maximum number of entries (from Config.MaxSize)

#### `HitRatio() float64`

Returns hit ratio as percentage (0.0-100.0).

**Example:**
```go
stats := cache.Stats()
if stats.HitRatio() < 70.0 {
    log.Warn("Low hit ratio, consider increasing cache size")
}
```

---

## Error Handling

Balios uses structured errors with error codes from [go-errors](https://github.com/agilira/go-errors).

### Error Codes

#### Configuration Errors
- `BALIOS_INVALID_CONFIG` - Invalid configuration
- `BALIOS_INVALID_MAX_SIZE` - Invalid MaxSize
- `BALIOS_INVALID_WINDOW_RATIO` - Invalid WindowRatio
- `BALIOS_INVALID_COUNTER_BITS` - Invalid CounterBits
- `BALIOS_INVALID_TTL` - Invalid TTL

#### Operation Errors
- `BALIOS_CACHE_FULL` - Cache full, eviction failed
- `BALIOS_KEY_NOT_FOUND` - Key not found
- `BALIOS_EVICTION_FAILED` - Eviction failed
- `BALIOS_SET_FAILED` - Set operation failed
- `BALIOS_DELETE_FAILED` - Delete operation failed

#### Loader Errors
- `BALIOS_LOADER_FAILED` - Loader function failed
- `BALIOS_LOADER_TIMEOUT` - Loader timeout
- `BALIOS_LOADER_CANCELLED` - Loader cancelled
- `BALIOS_INVALID_LOADER` - Nil loader function
- `BALIOS_PANIC_RECOVERED` - Loader panicked

#### Persistence Errors
- `BALIOS_SAVE_FAILED` - Save to file failed
- `BALIOS_LOAD_FAILED` - Load from file failed
- `BALIOS_CORRUPTED_DATA` - Corrupted cache data

#### Internal Errors
- `BALIOS_INTERNAL_ERROR` - Internal cache error

### Error Helper Functions

```go
// Error classification
func IsNotFound(err error) bool
func IsCacheFull(err error) bool
func IsConfigError(err error) bool
func IsOperationError(err error) bool
func IsLoaderError(err error) bool
func IsRetryable(err error) bool

// Error inspection
func GetErrorCode(err error) errors.ErrorCode
func GetErrorContext(err error) map[string]interface{}
```

### Error Handling Example

```go
user, err := cache.GetOrLoad("user:123", loader)
if err != nil {
    if balios.IsLoaderError(err) {
        log.Printf("Loader failed: %v", err)
        // Implement retry logic
    } else if balios.IsRetryable(err) {
        // Retry operation
    } else {
        return fmt.Errorf("unrecoverable error: %w", err)
    }
}
```

---

## Interfaces

### `Logger`

Minimal logging interface with zero overhead.

```go
type Logger interface {
    Debug(msg string, keyvals ...interface{})
    Info(msg string, keyvals ...interface{})
    Warn(msg string, keyvals ...interface{})
    Error(msg string, keyvals ...interface{})
}
```

**Default:** `NoOpLogger` (no-op implementation)

### `MetricsCollector`

Interface for collecting operation metrics.

```go
type MetricsCollector interface {
    RecordGet(latencyNs int64, hit bool)
    RecordSet(latencyNs int64)
    RecordDelete(latencyNs int64)
    RecordEviction()
}
```

**Default:** `NoOpMetricsCollector` (zero overhead)

**See:** [balios/otel](https://github.com/agilira/balios/tree/main/otel) for OpenTelemetry integration

### `TimeProvider`

Interface for time operations (useful for testing).

```go
type TimeProvider interface {
    Now() int64 // Nanoseconds since epoch
}
```

**Default:** System time

---

## Advanced Features

### Hot Configuration Reload

Dynamic configuration updates without restarting, powered by [Argus](https://github.com/agilira/argus).

#### Types

```go
type HotConfig struct {
    OnReload func(oldConfig, newConfig Config)
    // contains filtered or unexported fields
}

type HotConfigOptions struct {
    ConfigPath   string        // Path to config file (JSON, YAML, TOML, HCL, INI, Properties)
    PollInterval time.Duration // Check interval (default: 1s, min: 100ms)
    OnReload     func(oldConfig, newConfig Config)
    Logger       Logger        // Optional logger
}
```

#### Functions

**`NewHotConfig(cache Cache, opts HotConfigOptions) (*HotConfig, error)`**

Creates a hot-reloadable configuration watcher.

**`Start() error`**  
Begins watching the configuration file.

**`Stop() error`**  
Stops watching the configuration file.

**`GetConfig() Config`**  
Returns current configuration (thread-safe).

#### Example

```go
hotConfig, err := balios.NewHotConfig(cache, balios.HotConfigOptions{
    ConfigPath:   "/etc/myapp/cache.yaml",
    PollInterval: 30 * time.Second,
    OnReload: func(old, new balios.Config) {
        log.Printf("Config reloaded: MaxSize %d -> %d", old.MaxSize, new.MaxSize)
    },
})
if err != nil {
    log.Fatal(err)
}

if err := hotConfig.Start(); err != nil {
    log.Fatal(err)
}
defer hotConfig.Stop()

// Configuration updates are applied automatically
```

#### Configuration File Format (YAML example)

```yaml
cache:
  max_size: 20000
  ttl: 1h
  window_ratio: 0.01
  counter_bits: 4
```

**Supported Keys:**
- `cache.max_size` (int) - Maximum cache entries
- `cache.ttl` (duration string) - Time-to-live (e.g., "1h", "30m")
- `cache.window_ratio` (float) - Window cache ratio (0.0-1.0)
- `cache.counter_bits` (int) - Frequency counter bits (1-8)

**Note:** MaxSize changes require cache reconstruction and are not applied dynamically in the current version. Only TTL and other runtime parameters support hot reload.

---

### Memory Layout (10,000 entry cache)

```
Component       Size
Cache Entry     48 bytes
Hash Table      160 KB
TinyLFU         20 KB
Window LRU      96 KB
Main Cache      384 KB
Total Overhead  ~660 KB + entry values
```

**Per-entry overhead:** ~66 bytes (excluding value size)

---

## Thread Safety

All cache operations are thread-safe and designed for high concurrency:

- **Reads:** Atomic loads, no locks (except during eviction)
- **Writes:** CAS (Compare-And-Swap) operations
- **Eviction:** Fine-grained locking (only contested entries)
- **Testing:** Zero race conditions detected with `-race`

**Example:**
```go
cache := balios.NewGenericCache[string, int](balios.Config{MaxSize: 1000})

// Safe concurrent access
go func() { cache.Set("key1", 1) }()
go func() { cache.Get("key1") }()
go func() { cache.Delete("key1") }()
go func() { stats := cache.Stats() }()
```

---

## Best Practices

### 1. Size the Cache Appropriately

- **Too small:** High eviction rate, poor hit ratio
- **Too large:** Wasted memory, slower lookups
- **Rule of thumb:** ~2x your working set

### 2. Monitor Hit Ratio

- **Target:** >70% for most workloads
- **Low hit ratio:** Cache too small or poor key distribution

### 3. Use GetOrLoad for Expensive Operations

- Prevents cache stampede
- Automatic deduplication of concurrent requests
- Error handling built-in

### 4. Set Appropriate TTL

- **Too short:** More cache misses
- **Too long:** Stale data
- Consider data freshness requirements

### 5. Handle Loader Errors

- Errors are NOT cached (prevents error amplification)
- Implement retry logic in loader if needed

### 6. Use Context with Timeout

- Prevents hanging on slow loaders
- Enables graceful cancellation

### 7. Enable Metrics in Production

- Use `balios/otel` package for observability
- Monitor p95/p99 latencies
- Alert on low hit ratio or high eviction rate

---

## Related Documentation

- **[ARCHITECTURE.md](ARCHITECTURE.md)** - W-TinyLFU internals, lock-free design
- **[PERFORMANCE.md](PERFORMANCE.md)** - Benchmark results, hit ratio analysis
- **[GETORLOAD.md](GETORLOAD.md)** - Cache stampede prevention, singleflight pattern
- **[METRICS.md](METRICS.md)** - Observability, PromQL queries, Grafana dashboards
- **[ERRORS.md](ERRORS.md)** - Error codes, structured errors
- **[FUZZING.md](FUZZING.md)** - Fuzz testing guide, security hardening

---

## Examples

Complete working examples in the repository:

- **[examples/getorload/](../examples/getorload/)** - GetOrLoad API usage
- **[examples/otel-prometheus/](../examples/otel-prometheus/)** - OpenTelemetry + Prometheus + Grafana
- **[examples/errors/](../examples/errors/)** - Error handling patterns

---

## Packages

- **`github.com/agilira/balios`** - Core cache (zero external dependencies)
- **`github.com/agilira/balios/otel`** - OpenTelemetry integration (separate module)

---

Balios • an AGILira fragment
