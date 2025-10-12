# Balios

> High-performance in-memory cache library for Go

Balios is an in-memory cache implementation based on W-TinyLFU (Window Tiny Least Frequently Used) algorithm, designed to outperform existing solutions like Otter and Ristretto through:

- Zero allocations in critical operations
- Lock-free operations with atomic primitives
- W-TinyLFU for optimal hit ratio
- TDD approach for maximum reliability
- Clean code without over-engineering

## Performance Goals

**Current Performance** (2025-10-11, with go-timecache):
- ✅ Set operations: **124.8 ns/op** (2.75x faster than Otter's 343.6ns)
- ✅ Get operations: **106.8 ns/op** (1.19x faster than Otter's 126.6ns)
- ✅ Balanced workload: **39.49 ns/op** (4.5x faster than Otter's 177.9ns)
- ✅ Hit ratio: **79.63%** (vs Otter 78.36%, Ristretto 58.02%)
- ✅ Memory: **0-10 B/op** (vs Otter 0-51 B/op)

Target performance metrics:
- Throughput: > 100M ops/sec on Get/Set operations
- Latency: < 10ns for cache hits
- Memory efficiency: Zero allocations on hot path
- Hit ratio: Superior performance through W-TinyLFU

## Features

- **Type-Safe Generics API**: `Cache[K comparable, V any]` for compile-time type safety
- **W-TinyLFU Algorithm**: Optimal hit ratio through frequency + recency
- **Lock-Free Operations**: Atomic primitives for high concurrency
- **TTL Support**: Automatic expiration with lazy cleanup
- **Hot Configuration Reload**: Dynamic updates via [Argus](https://github.com/agilira/argus) file watcher
- **Ultra-Fast Time**: go-timecache integration (~121x faster than time.Now())
- **Structured Errors**: Rich error context with [go-errors](https://github.com/agilira/go-errors)
- **Zero Allocations**: 0-1 allocations per operation
- **Production Ready**: 50 tests passing, race detector clean, gosec validated

## Architecture

### W-TinyLFU Components
- Window Cache: LRU for recent items
- Main Cache: S-LRU (Segmented LRU) 
- Frequency Sketch: Count-Min Sketch with decay
- Admission Policy: Based on frequency and recency

### Dependencies
- [`github.com/agilira/go-errors`](https://github.com/agilira/go-errors) - Structured error handling
- [`github.com/agilira/go-timecache`](https://github.com/agilira/go-timecache) - High-performance time provider
- [`github.com/agilira/argus`](https://github.com/agilira/argus) - Configuration file watcher for hot reload

## Quick Start

### Type-Safe Generic API (Recommended)

```go
package main

import (
    "fmt"
    "time"
    
    "github.com/agilira/balios"
)

// Define your value type
type User struct {
    ID   int
    Name string
    Role string
}

func main() {
    // Create type-safe cache with generics
    cache := balios.NewGenericCache[string, User](balios.Config{
        MaxSize: 10_000,
        TTL:     time.Hour,
    })
    
    // Set a value (type-safe!)
    cache.Set("user:123", User{
        ID:   123,
        Name: "John Doe",
        Role: "admin",
    })
    
    // Get a value (no type assertion needed!)
    if user, found := cache.Get("user:123"); found {
        fmt.Printf("User: %s (%s)\n", user.Name, user.Role)
    }
    
    // Check stats
    stats := cache.Stats()
    fmt.Printf("Hit ratio: %.2f%%\n", stats.HitRatio()*100)
}
```

### Legacy Interface{} API

The original non-generic API is still available:

```go
cache := balios.NewCache(balios.Config{
    MaxSize: 10_000,
    TTL:     time.Hour,
})

cache.Set("key", value)
if value, found := cache.Get("key"); found {
    user := value.(User)  // Type assertion required
    fmt.Printf("User: %+v\n", user)
}
```

## Error Handling

Balios uses structured errors with rich context:

```go
import "github.com/agilira/balios"

// Check for specific errors
value, found := cache.Get("key")
if !found {
    err := balios.NewErrKeyNotFound("key")
    if balios.IsRetryable(err) {
        // Retry logic
    }
}

// Get error details
code := balios.GetErrorCode(err)
ctx := balios.GetErrorContext(err)
```

See [Error Handling Documentation](docs/ERRORS.md) for details.

## Hot Configuration Reload

Balios supports dynamic configuration updates without restarting your application:

```go
import (
    "time"
    "github.com/agilira/balios"
)

func main() {
    cache := balios.NewCache(balios.DefaultConfig())
    
    // Enable hot reload from YAML config file
    hotConfig, err := balios.NewHotConfig(cache, balios.HotConfigOptions{
        ConfigPath:   "config.yaml",
        PollInterval: 1 * time.Second, // Check for changes every second
        OnReload: func(oldCfg, newCfg balios.Config) {
            log.Printf("Config reloaded: MaxSize %d -> %d", oldCfg.MaxSize, newCfg.MaxSize)
        },
    })
    if err != nil {
        log.Fatal(err)
    }
    defer hotConfig.Stop()
    
    // Your application continues running...
    // Configuration changes are applied automatically
}
```

**Example config.yaml:**
```yaml
cache:
  max_size: 10000
  ttl: 1h       # Duration format: 1h, 30m, 5s
  window_ratio: 0.01
  counter_bits: 4
```

Supported formats: YAML, JSON, TOML, HCL, INI via [Argus](https://github.com/agilira/argus).

## Benchmarks

See [benchmarks/](benchmarks/) directory for comprehensive comparisons with Otter and Ristretto.

```bash
cd benchmarks
go test -bench=. -benchmem
```

Key results (as of 2025-01-15):

| Operation | Balios | Otter | Ristretto | Improvement |
|-----------|--------|-------|-----------|-------------|
| Set | 124.8 ns/op | 343.6 ns/op | 282.4 ns/op | **2.75x faster** |
| Get | 106.8 ns/op | 126.6 ns/op | 157.9 ns/op | **1.19x faster** |
| Balanced | 39.49 ns/op | 177.9 ns/op | 112.3 ns/op | **4.5x faster** |
| Hit Ratio | 79.63% | 78.36% | 58.02% | **+1.6%** |

## Testing

All tests pass with race detector:

```bash
# Run all tests
go test -v ./...

# Run with race detector
go test -race ./...

# Run benchmarks
go test -bench=. -benchmem ./...

# Security scan
gosec ./...
```

Current status: **50 tests passing**, race detector clean, 0 security issues.

## Development Status

**Phase 1: Foundation (COMPLETE)** ✅
- [x] Core interfaces and data structures
- [x] W-TinyLFU implementation (4-bit frequency sketch)
- [x] Lock-free operations (atomic primitives)
- [x] Zero allocation optimizations (Get: 0 allocs, Set: 1 alloc)
- [x] TTL support with lazy expiration
- [x] Structured error system with go-errors
- [x] Hot configuration reload with Argus
- [x] Type-safe Generics API (`GenericCache[K, V]`)
- [x] **Optimized generics: String keys +5.4ns overhead (16%), zero allocations**
- [x] Comprehensive test suite (50 tests, non-flaky)
- [x] Security hardening (gosec validated)
- [x] Benchmarks vs Otter/Ristretto

**Phase 2: Advanced Features (IN PROGRESS)** 🚧
- [ ] Automatic loading with singleflight
- [ ] Context support for cancellation
- [ ] Advanced stats (percentiles, eviction reasons)
- [ ] Async refresh (stale-while-revalidate)

**Phase 3: Production Features (PLANNED)** 📋
- [ ] Persistence (save/load from disk)
- [ ] Prometheus metrics integration
- [ ] Distributed cache coordination
- [ ] Write-through/write-behind patterns

See [study/ANALYSIS.md](study/ANALYSIS.md) for detailed roadmap.

## Contributing

1. TDD approach - test first
2. Zero allocations on hot path
3. Clean and simple code
4. Benchmark every change