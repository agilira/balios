# Balios

> High-performance in-memory cache library for Go

Balios is an in-memory cache implementation based on the W-TinyLFU (Window Tiny Least Frequently Used) algorithm, designed for high throughput and optimal hit ratios.

## Performance

Benchmarked on AMD Ryzen 5 7520U, race detector enabled:

**Single-Threaded Operations:**
- Set: 135.1 ns/op (1 allocation)
- Get: 113.4 ns/op (0 allocations)
- GetOrLoad (cache hit): 20.3 ns/op (0 allocations)

**Parallel Mixed Workloads:**
- Balanced (50/50 read/write): 45.6 ns/op
- Read-heavy (90/10 read/write): 39.4 ns/op
- Read-only: 34.8 ns/op

**Hit Ratio (1M requests, Zipf distribution):**
- Balios: 79.27%
- Otter: 79.68%
- Ristretto: 62.77%

**Singleflight Efficiency:**
- 1000 concurrent requests for same missing key = 1 loader execution

See [benchmarks/](benchmarks/) for detailed comparison with Otter and Ristretto.

## Features

- **Type-Safe Generics API**: `GenericCache[K comparable, V any]` with compile-time type safety
- **Automatic Loading**: `GetOrLoad()` API with singleflight pattern for cache stampede prevention
- **W-TinyLFU Algorithm**: Combines frequency and recency for optimal eviction decisions
- **Lock-Free Operations**: Uses atomic primitives for high concurrency
- **TTL Support**: Automatic expiration with lazy cleanup
- **Context Support**: Timeout and cancellation for loader functions
- **Hot Configuration Reload**: Dynamic updates via [Argus](https://github.com/agilira/argus) file watcher
- **Structured Errors**: Rich error context with [go-errors](https://github.com/agilira/go-errors)
- **Enterprise Observability**: OpenTelemetry integration for metrics (p50/p95/p99 latencies, hit ratio)
- **Race-Free**: All tests pass with `-race` detector
- **Production Ready**: 76 tests, gosec validated, zero security issues

## Installation

```bash
go get github.com/agilira/balios
```

## Quick Start

### Type-Safe Generic API (Recommended)

```go
package main

import (
    "fmt"
    "time"
    
    "github.com/agilira/balios"
)

type User struct {
    ID   int
    Name string
    Role string
}

func main() {
    // Create type-safe cache
    cache := balios.NewGenericCache[string, User](balios.Config{
        MaxSize: 10_000,
        TTL:     time.Hour,
    })
    
    // Set a value
    cache.Set("user:123", User{
        ID:   123,
        Name: "John Doe",
        Role: "admin",
    })
    
    // Get a value (no type assertion needed)
    if user, found := cache.Get("user:123"); found {
        fmt.Printf("User: %s (%s)\n", user.Name, user.Role)
    }
    
    // Check stats
    stats := cache.Stats()
    fmt.Printf("Hit ratio: %.2f%%\n", stats.HitRatio()*100)
}
```

### Enterprise Observability with OpenTelemetry

Monitor cache performance with automatic percentile calculation:

```go
import (
    "github.com/agilira/balios"
    baliosostel "github.com/agilira/balios/otel"
    "go.opentelemetry.io/otel/exporters/prometheus"
    "go.opentelemetry.io/otel/sdk/metric"
)

// Setup OTEL with Prometheus exporter
exporter, _ := prometheus.New()
provider := metric.NewMeterProvider(metric.WithReader(exporter))

// Create metrics collector
metricsCollector, _ := baliosostel.NewOTelMetricsCollector(provider)

// Configure cache with metrics
cache := balios.NewGenericCache[string, User](balios.Config{
    MaxSize:          10_000,
    MetricsCollector: metricsCollector, // Zero overhead if nil
})

// Metrics automatically collected:
// - balios_get_latency_ns (histogram with p50, p95, p99, p99.9)
// - balios_set_latency_ns (histogram)
// - balios_get_hits_total (counter)
// - balios_get_misses_total (counter)
// - balios_evictions_total (counter)
```

**Architecture:**
- **Core stays light**: No OTEL dependencies in main module
- **Optional integration**: `balios/otel` is a separate module
- **Zero overhead**: MetricsCollector defaults to no-op implementation
- **Industry standard**: Compatible with Prometheus, Jaeger, DataDog, Grafana

See [otel/README.md](otel/README.md) and [examples/otel-prometheus/](examples/otel-prometheus/) for complete setup with Grafana dashboard.

### Automatic Loading with GetOrLoad

Prevent cache stampede with singleflight pattern:

```go
// Multiple concurrent requests for same key = single loader execution
user, err := cache.GetOrLoad("user:123", func() (User, error) {
    // This expensive operation runs only once
    return fetchUserFromDB(123)
})
if err != nil {
    log.Printf("Failed to load user: %v", err)
    return
}
```

With context support for timeout/cancellation:

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

user, err := cache.GetOrLoadWithContext(ctx, "user:123", 
    func(ctx context.Context) (User, error) {
        return fetchUserFromDBWithContext(ctx, 123)
    })
```

**Key characteristics:**
- Cache hit: 20.3 ns/op (0 allocations) - same performance as `Get()`
- Concurrent requests: 1000 simultaneous requests = 1 loader call (singleflight)
- Error handling: Loader errors are NOT cached (prevents error amplification)
- Panic recovery: Returns `BALIOS_PANIC_RECOVERED` error if loader panics

See [examples/getorload/](examples/getorload/) for comprehensive examples.

### Legacy Interface API

The non-generic API is still available:

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

## Documentation

- [Architecture](docs/ARCHITECTURE.md) - W-TinyLFU internals, lock-free design, memory layout
- [Performance](docs/PERFORMANCE.md) - Comprehensive benchmarks, hit ratio analysis, scalability
- [GetOrLoad API](docs/GETORLOAD.md) - Cache stampede prevention, singleflight pattern, best practices
- [Metrics & Observability](docs/METRICS.md) - OpenTelemetry integration, Prometheus queries, monitoring best practices
- [Error Handling](docs/ERRORS.md) - Structured error codes and contexts
- [Examples](examples/) - Comprehensive usage examples
- [Benchmarks](benchmarks/) - Performance comparison with Otter and Ristretto

## Hot Configuration Reload

Enable dynamic configuration updates without restarting:

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
        PollInterval: 1 * time.Second,
        OnReload: func(oldCfg, newCfg balios.Config) {
            log.Printf("Config reloaded: MaxSize %d -> %d", 
                oldCfg.MaxSize, newCfg.MaxSize)
        },
    })
    if err != nil {
        log.Fatal(err)
    }
    defer hotConfig.Stop()
    
    // Configuration changes are applied automatically
}
```

Supported formats: YAML, JSON, TOML, HCL, INI via [Argus](https://github.com/agilira/argus).

## Architecture

**W-TinyLFU Components:**
- Window Cache: LRU for recent items
- Main Cache: S-LRU (Segmented LRU) 
- Frequency Sketch: Count-Min Sketch with decay
- Admission Policy: Based on frequency and recency

**Dependencies:**
- [`github.com/agilira/go-errors`](https://github.com/agilira/go-errors) - Structured error handling
- [`github.com/agilira/go-timecache`](https://github.com/agilira/go-timecache) - High-performance time provider (~121x faster than `time.Now()`)
- [`github.com/agilira/argus`](https://github.com/agilira/argus) - Configuration file watcher

## Testing

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

Current status: 69 tests passing, race detector clean, gosec 0 issues.

## Development Status

**Phase 1: Foundation (COMPLETE)**
- Core interfaces and data structures
- W-TinyLFU algorithm implementation
- Lock-free operations with atomic primitives
- TTL support with lazy expiration
- Structured error handling (28 error codes)
- Comprehensive test suite (86.1% coverage)

**Phase 2: Performance & Reliability (COMPLETE)**
- go-timecache integration (121x faster time)
- Hot configuration reload with Argus
- Type-safe Generics API with optimization
- Race-free implementation (validated with `-race`)
- Hit ratio validation (79.3%, equivalent to best)

**Phase 3: Advanced Features (COMPLETE)**
- Automatic Loading with Singleflight (GetOrLoad API)
- Context support for timeout/cancellation
- Cache stampede prevention
- Panic recovery with structured errors
- 76 comprehensive tests, all passing with `-race`

**Phase 4: OpenTelemetry Integration (COMPLETE)**
- MetricsCollector interface in core (zero overhead when not used)
- balios/otel package for OpenTelemetry integration
- Automatic percentile calculation (p50, p95, p99, p99.9)
- Prometheus exporter example with Grafana dashboard
- Hit ratio, eviction rate, and latency monitoring

**Phase 5: Future Enhancements (PLANNED)**
- Async refresh (stale-while-revalidate pattern)
- Persistence (save/load from disk)
- Distributed cache coordination
- Write-through/write-behind patterns

## Contributing

1. Follow TDD approach - write tests first
2. Maintain zero allocations on hot path
3. Validate race-free implementation with `-race` detector
4. Write clean and well-documented code
5. Use comprehensive error handling
6. Benchmark every performance-related change

## License

See [LICENSE](LICENSE) for details.
