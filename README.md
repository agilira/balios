# Balios: High-performance in-memory cache library for Go
### an AGILira fragment

Balios is an in-memory cache implementation based on the W-TinyLFU (Window Tiny Least Frequently Used) algorithm, designed for high throughput and optimal hit ratios with advanced security and observability.

## Features

- **Type-Safe Generics API**: `GenericCache[K comparable, V any]` with compile-time type safety
- **Automatic Loading**: `GetOrLoad()` API with singleflight pattern for cache stampede prevention
- **W-TinyLFU Algorithm**: Combines frequency and recency for optimal eviction decisions
- **Lock-Free**: Uses atomic primitives for high concurrency
- **TTL Support**: Automatic expiration with lazy cleanup
- **Context Support**: Timeout and cancellation for loader functions
- **Hot Reload**: Dynamic configuration updates via [Argus](https://github.com/agilira/argus)
- **Structured Errors**: Rich error context with [go-errors](https://github.com/agilira/go-errors) - see [examples/errors/](examples/errors/)
- **Observability**: OpenTelemetry integration for metrics (p50/p95/p99 latencies, hit ratio) & logger interface. Zero overhead when disabled (compiler eliminates no-op implementations) - see [examples/otel-prometheus/](examples/otel-prometheus/)
- **Secure by Design**: [Red-team tested](balios_security_test.go) and [fuzz tested](balios_fuzz_test.go)

## Performance

**Single-Threaded Performance:**

| Package | Set (ns/op) | Set % vs Balios | Get (ns/op) | Get % vs Balios | Allocations |
| :------ | ----------: | --------------: | ----------: | --------------: | ----------: |
| **Balios** | **131.3 ns/op** | **+0%** | **105.5 ns/op** | **+0%** | **1/0 allocs/op** |
| Balios-Generic | 139.0 ns/op | +6% | 108.8 ns/op | +3% | 1/0 allocs/op |
| Otter | 338.7 ns/op | +158% | 120.1 ns/op | +14% | 1/0 allocs/op |
| Ristretto | 282.7 ns/op | +115% | 156.6 ns/op | +48% | 2/0 allocs/op |

**Parallel Performance (8 cores):**

| Package | Set (ns/op) | Set % vs Balios | Get (ns/op) | Get % vs Balios | Allocations |
| :------ | ----------: | --------------: | ----------: | --------------: | ----------: |
| **Balios** | **39.90 ns/op** | **+0%** | **30.06 ns/op** | **+0%** | **1/0 allocs/op** |
| Balios-Generic | 42.21 ns/op | +6% | 32.34 ns/op | +8% | 1/0 allocs/op |
| Otter | 230.8 ns/op | +478% | 27.62 ns/op | -8% | 1/0 allocs/op |
| Ristretto | 114.7 ns/op | +187% | 35.42 ns/op | +18% | 1/0 allocs/op |

**Mixed Workloads (Realistic Scenarios):**

| Workload | Balios | Balios-Generic | Otter | Ristretto | Best |
| :------- | -----: | -------------: | ----: | --------: | :--- |
| Write-Heavy (10% R / 90% W) | **42.47 ns/op** | 44.86 ns/op | 211.6 ns/op | 125.4 ns/op | **Balios** |
| Balanced (50% R / 50% W) | **41.76 ns/op** | 42.55 ns/op | 149.9 ns/op | 110.2 ns/op | **Balios** |
| Read-Heavy (90% R / 10% W) | **46.97 ns/op** | 58.68 ns/op | 51.35 ns/op | 81.68 ns/op | **Balios** |
| Read-Only (100% R) | 34.41 ns/op | 33.94 ns/op | **28.10 ns/op** | 33.02 ns/op | **Otter** |

**Hit Ratio (100K requests, Zipf distribution):**

| Cache | Hit Ratio | Notes |
| :---- | --------: | :---- |
| **Balios** | **80.20%** | Statistically equivalent to Otter |
| Otter | 79.64% | -0.7% (within noise margin) |
| Ristretto | 71.39% | -11% |

**Test Environment:** AMD Ryzen 5 7520U Go 1.25+

See [benchmarks/](benchmarks/) for comprehensive results and [docs/PERFORMANCE.md](docs/PERFORMANCE.md) for detailed analysis.

## Installation

```bash
go get github.com/agilira/balios
```

## Quick Start

### Type-Safe Generic API

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

## The Philosophy Behind Balios

Balios and his brother Xanthos were the immortal horses of Achilles, born from Zephyros, the swiftest of the Anemoi. They were not merely fast—they were the children of the wind itself, incomparable to mortal steeds. Balios possessed intelligence beyond any horse, an instinct that guided Achilles through every battle with perfect judgment.

When Patroclus fell, it was Xanthos who spoke—granted voice by Hera herself—to warn Achilles of his fate. But Balios remained silent, his wisdom expressed not in words but in action, in knowing when to charge and when to wheel away, in the perfect synchrony between horse and hero that transcends command.

## Documentation

- [Architecture](docs/ARCHITECTURE.md) - W-TinyLFU internals, lock-free design, memory layout
- [Performance](docs/PERFORMANCE.md) - Comprehensive benchmarks, hit ratio analysis, scalability
- [GetOrLoad API](docs/GETORLOAD.md) - Cache stampede prevention, singleflight pattern, best practices
- [Metrics & Observability](docs/METRICS.md) - OpenTelemetry integration, Prometheus queries, monitoring best practices
- [Error Handling](docs/ERRORS.md) - Structured error codes and contexts
- [Examples](examples/) - Comprehensive usage examples
- [Benchmarks](benchmarks/) - Performance comparison with popular libraries
- [otel/README.md](otel/README.md) and [examples/otel-prometheus/](examples/otel-prometheus/) for complete setup with Grafana dashboard.

## Future Enhancements (PLANNED)
- Async refresh (stale-while-revalidate pattern)
- Persistence (save/load from disk)
- Distributed cache coordination
- Write-through/write-behind patterns

## License

Balios is licensed under the [Mozilla Public License 2.0](./LICENSE.md).

---

Balios • an AGILira fragment
