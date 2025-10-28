# Extensibility and Wrapper Packages

> "Perfection is achieved not when there is nothing more to add, but when there is nothing left to take away."  
> — Antoine de Saint-Exupéry

## Philosophy

Balios core is designed to be **minimal, predictable, and composable**. The core package provides the fundamental building blocks of a high-performance cache without imposing policy decisions on users. This document outlines the extensibility model for building optional wrapper packages that add specialized behaviors while maintaining the core's simplicity and performance characteristics.

## Design Principles

### 1. Core Minimalism

The `balios` core package provides:
- **Deterministic behavior**: No hidden goroutines, no background tasks
- **Transparent resource usage**: Every allocation and goroutine is explicit
- **Predictable performance**: Zero-overhead operations on the hot path
- **Clear semantics**: Every operation does exactly what it says

What the core does **not** provide:
- Background cleanup goroutines
- Automatic persistence
- Network replication
- Complex eviction policies
- Sliding window TTL

These features are intentionally excluded to keep the core lean, testable, and suitable for the widest range of use cases.

### 2. Opt-In Complexity

Users should explicitly choose the tradeoffs they want:
- **Production-critical systems**: Use core directly for maximum control
- **Web applications**: Add background cleanup wrapper for convenience
- **Session management**: Add sliding TTL wrapper for extended lifetimes
- **Distributed systems**: Add replication wrapper for high availability

### 3. Composability

Wrappers should be composable and interoperable:
```go
// Start with core
cache := balios.NewCache(config)

// Add background cleanup (optional)
bgCache := baliosbg.Wrap(cache, cleanupInterval)

// Add metrics collection (optional)
metricsCache := baliosmetrics.Wrap(bgCache, collector)

// Use the enhanced cache
metricsCache.Set("key", "value")
```

## Planned Wrapper Packages

### `balios-bg` - Background Cleanup

**Purpose**: Automatic periodic expiration of TTL-based entries.

**Use Case**: Web applications where memory management should be automatic and developers don't want to manually call `ExpireNow()`.

**Tradeoffs**:
- ✅ Convenience: Automatic cleanup without manual intervention
- ✅ Memory efficiency: Proactive removal of expired entries
- ❌ Resource overhead: Additional goroutine and periodic work
- ❌ Non-deterministic: Cleanup timing is approximate

**Example API**:
```go
import "github.com/agilira/balios-bg"

// Wrap existing cache with background cleanup
bgCache := baliosbg.New(cache, baliosbg.Config{
    CleanupInterval: 10 * time.Second,
    MaxCleanupTime:  100 * time.Millisecond,
})
defer bgCache.Stop()

// Use like normal cache
bgCache.Set("key", "value")
```

**Implementation Notes**:
- Single goroutine per cache instance
- Configurable cleanup interval
- Graceful shutdown with `Stop()`
- Respects context cancellation
- Metrics integration for cleanup statistics

---

### `balios-sliding` - Sliding Window TTL

**Purpose**: Reset TTL on every access, keeping frequently-used entries alive longer.

**Use Case**: Session management, active connection tracking, hot data caching.

**Tradeoffs**:
- ✅ Extended lifetime for active data
- ✅ Automatic LRU-like behavior for TTL entries
- ❌ Write overhead on every Get()
- ❌ Increased contention on hot keys

**Example API**:
```go
import "github.com/agilira/balios-sliding"

// Wrap cache with sliding window behavior
slidingCache := baliossliding.New(cache, baliossliding.Config{
    ResetOnGet: true,
    ResetOnHas: false,
})

// Every Get() resets the TTL
slidingCache.Set("session_123", sessionData)
time.Sleep(5 * time.Second)
slidingCache.Get("session_123") // TTL reset - entry lives longer
```

**Implementation Notes**:
- Intercepts Get/Has operations
- Updates expireAt timestamp atomically
- Optional per-operation configuration
- Minimal overhead using atomic operations

---

### `balios-adaptive` - Adaptive Expiration

**Purpose**: Automatically adjust TTL based on access patterns and cache hit ratio.

**Use Case**: Caches where optimal TTL is unknown or varies by workload.

**Tradeoffs**:
- ✅ Self-tuning: Adapts to workload automatically
- ✅ Optimized memory usage: Shorter TTL for cold data
- ❌ Complexity: Non-trivial heuristics
- ❌ Overhead: Statistics tracking and adaptation logic

**Example API**:
```go
import "github.com/agilira/balios-adaptive"

adaptiveCache := baliosadaptive.New(cache, baliosadaptive.Config{
    MinTTL:          1 * time.Minute,
    MaxTTL:          1 * time.Hour,
    TargetHitRatio:  0.85,
    AdaptInterval:   1 * time.Minute,
})

// TTL automatically adjusts based on hit ratio
adaptiveCache.Set("key", "value")
```

**Implementation Notes**:
- Monitors hit ratio per key or globally
- Adjusts TTL using feedback control
- Background goroutine for adaptation logic
- Statistical sampling to reduce overhead

---

### `balios-persist` - Persistence Layer

**Purpose**: Save and restore cache state to/from disk.

**Use Case**: Fast restarts, warm cache initialization, disaster recovery.

**Tradeoffs**:
- ✅ Fast startup: Restore hot data from disk
- ✅ Durability: Survive process restarts
- ❌ I/O overhead: Disk writes on save
- ❌ Complexity: Serialization, versioning, corruption handling

**Example API**:
```go
import "github.com/agilira/balios-persist"

persistCache := baliospersist.New(cache, baliospersist.Config{
    FilePath:       "/var/cache/balios.db",
    SaveInterval:   5 * time.Minute,
    Compression:    true,
})

// Automatically saves to disk periodically
defer persistCache.Close() // Final save on shutdown

// Manual save
if err := persistCache.Save(); err != nil {
    log.Printf("Failed to save cache: %v", err)
}

// Restore from disk
if err := persistCache.Load(); err != nil {
    log.Printf("Failed to load cache: %v", err)
}
```

**Implementation Notes**:
- Snapshot-based persistence (copy-on-write)
- Background goroutine for periodic saves
- Atomic writes with checksum validation
- Version compatibility checks
- Graceful degradation on corruption

---

### `balios-replicated` - Multi-Node Replication

**Purpose**: Synchronize cache state across multiple nodes in a distributed system.

**Use Case**: Microservices, horizontally-scaled web applications, distributed caching.

**Tradeoffs**:
- ✅ Consistency: Shared cache state across nodes
- ✅ Fault tolerance: Survive single-node failures
- ❌ Network overhead: Replication traffic
- ❌ Complexity: Consensus, conflict resolution, network partitions

**Example API**:
```go
import "github.com/agilira/balios-replicated"

replicatedCache := baliosreplicated.New(cache, baliosreplicated.Config{
    NodeID:          "node-1",
    Peers:           []string{"node-2:8080", "node-3:8080"},
    ReplicationMode: baliosreplicated.EventualConsistency,
})

// Writes are replicated to peers
replicatedCache.Set("key", "value")
```

**Implementation Notes**:
- Gossip-based or leader-based replication
- Configurable consistency levels
- Conflict resolution strategies
- Network partition handling
- Background goroutines for replication

---

## Wrapper Development Guidelines

### Interface Compatibility

All wrappers should implement the `balios.Cache` interface:

```go
type Cache interface {
    Set(key string, value interface{}) bool
    Get(key string) (interface{}, bool)
    Delete(key string) bool
    Has(key string) bool
    Clear()
    Close() error
    Capacity() int
    Len() int
    Stats() CacheStats
    ExpireNow() int
}
```

This ensures wrappers are drop-in replacements for the core cache.

### Wrapper Pattern

Use the decorator pattern for clean composition:

```go
type WrapperCache struct {
    underlying balios.Cache
    // wrapper-specific fields
}

func Wrap(cache balios.Cache, config Config) *WrapperCache {
    return &WrapperCache{
        underlying: cache,
        // initialize wrapper fields
    }
}

func (w *WrapperCache) Set(key string, value interface{}) bool {
    // wrapper-specific logic before
    result := w.underlying.Set(key, value)
    // wrapper-specific logic after
    return result
}
```

### Testing

Wrappers should:
- Include comprehensive unit tests
- Test interaction with core cache
- Use race detector (`go test -race`)
- Measure performance overhead
- Document tradeoffs clearly

### Documentation

Each wrapper package should include:
- **README.md**: Quick start and examples
- **API.md**: Complete API reference
- **BENCHMARKS.md**: Performance characteristics and overhead measurements
- **TRADEOFFS.md**: When to use (and not use) the wrapper

---

## Community Wrappers

We encourage the community to build specialized wrappers for domain-specific use cases. If you build a wrapper package:

1. Follow the interface compatibility guidelines
2. Document performance characteristics clearly
3. Include comprehensive tests and benchmarks
4. Consider submitting to [awesome-balios](https://github.com/agilira/awesome-balios)

### Naming Convention

Community wrappers should use the prefix `balios-` for discoverability:
- `balios-redis` - Redis-backed cache
- `balios-memcache` - Memcached integration
- `balios-prometheus` - Prometheus metrics exporter
- `balios-grpc` - gRPC cache service

---

## Core Stability Guarantee

The `balios` core package follows semantic versioning and maintains API stability:

- **Major versions** (1.x → 2.x): Breaking changes allowed
- **Minor versions** (1.1 → 1.2): New features, backward compatible
- **Patch versions** (1.1.0 → 1.1.1): Bug fixes only

Wrapper packages can evolve independently and may have different version numbers than core.

---

## Roadmap

- [ ] `balios-bg` - Background cleanup wrapper
- [ ] `balios-sliding` - Sliding window TTL wrapper
- [ ] `balios-adaptive` - Adaptive expiration wrapper
- [ ] `balios-persist` - Persistence layer wrapper
- [ ] `balios-replicated` - Multi-node replication wrapper
- [ ] Community wrapper showcase
- [ ] Wrapper performance comparison benchmarks

---

## Contributing

We welcome contributions to:
- Core improvements (performance, bug fixes, documentation)
- Official wrapper packages (following guidelines above)
- Community wrappers (showcase your work!)
- Documentation improvements

See [CONTRIBUTING.md](../CONTRIBUTING.md) for details.

---

## Questions?

- **GitHub Issues**: [github.com/agilira/balios/issues](https://github.com/agilira/balios/issues)
- **Discussions**: [github.com/agilira/balios/discussions](https://github.com/agilira/balios/discussions)
- **Email**: balios@agilira.com

---

**Copyright © 2025 AGILira - A. Giordano**  
**Series**: an AGILira fragment  
**License**: MPL-2.0
