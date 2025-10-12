# Changelog

All notable changes to Balios will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- **Automatic Loading with Singleflight**: Cache stampede prevention with GetOrLoad API
  - `GetOrLoad(key, loader)` for automatic cache population on miss
  - `GetOrLoadWithContext(ctx, key, loader)` with timeout/cancellation support
  - Generic versions: `GenericCache[K,V].GetOrLoad()` with type safety
  - **Singleflight pattern**: Multiple concurrent requests for same missing key → 1 loader call
  - Efficiency: 1000x - tested with 1000 concurrent requests = 1 loader execution
  - Performance: Cache hit **19.7 ns/op** (0 allocs), miss **124.5 ns/op** (1 alloc)
  - Overhead: Only **+3.6ns** vs manual Get+Set pattern
  - Race-free with atomic.Value wrappers (handles nil safely)
  - WaitGroup pre-initialized before LoadOrStore for race-free sync
  - Panic recovery with BALIOS_PANIC_RECOVERED error code
  - Error propagation: Loader errors NOT cached (prevents error amplification)
  - Context support for cancellation and timeout control
  - 16 comprehensive tests (cache hit, miss, concurrent, timeout, panic, generics)
  - All tests pass with `-race` detector

- **Race-Free Implementation**: Thread-safe cache operations with zero data races
  - Two-phase CAS locking pattern (valid → pending → valid)
  - Atomic operations on entry state transitions
  - `atomic.Value` for race-free value storage
  - Entry pending state (value 3) during writes
  - Double-check pattern in Get() for consistency
  - All operations skip pending entries for data integrity
  - Performance cost: only +2ns from atomic.Value
  - Validated with -race detector on all tests and benchmarks
  - Production-ready for high-concurrency workloads

- **Optimized Generics Performance**: Zero-allocation optimizations for common key types
  - Type switch dispatching for string, int, uint types (11 variants)
  - String keys: +5.4ns overhead vs non-generic - **134ns Set, 111ns Get**
  - Generic overhead: **only +3-5%** (4ns Set, 4ns Get)
  - **Zero allocations for string keys** (both Set and Get)
  - 1 allocation for int keys (unavoidable int→string conversion)
  - Uses `strconv` package for efficient integer conversion
  - Fallback to `fmt.Sprintf` for uncommon types (structs, custom types)
  - 82% reduction in overhead (from +27ns to +5.4ns for string keys)
  - Special handling for int64 minimum value edge case
  - Comprehensive optimization test suite with allocation tracking

- **Type-Safe Generics API**: Modern type-safe cache interface with Go generics
  - `NewGenericCache[K comparable, V any](config)` for compile-time type safety
  - Eliminates type assertions and runtime type errors
  - Wrapper pattern around existing Cache implementation
  - Support for any comparable key type (string, int, custom types)
  - Support for any value type (structs, pointers, slices, maps)
  - Full test coverage with 10 TDD-first tests
  - Backward compatible: legacy Cache interface still available
  - Benchmarks included for performance tracking

- **Hot Configuration Reload**: Dynamic configuration updates with Argus integration
  - Watch YAML, JSON, TOML, HCL, INI configuration files for changes
  - Automatic reload on file modification with configurable poll interval
  - Thread-safe configuration access with RWMutex
  - Support for MaxSize, TTL, WindowRatio, CounterBits hot updates
  - OnReload callback for custom handling of configuration changes
  - Non-flaky tests with proper mtime change detection
  - Integration with [Argus](https://github.com/agilira/argus) file watcher
  - Comprehensive test suite with 6 tests covering all scenarios

- **Hit Ratio Validation**: Extensive testing confirms cache quality equivalent to best competitors
  - Average hit ratio: **79.3%** (Otter: 79.7%, Ristretto: 62.8%)
  - Tested with 1M requests across 10 runs for statistical stability
  - Validated across 4 workload patterns (Zipf distributions)
  - Highly skewed (s=1.5): 89.65% (Otter: 90.73%, Ristretto: 89.80%)
  - Moderate (s=1.0): 72.12% (Otter: 70.96%, Ristretto: 63.30%)
  - Less skewed (s=0.8): 71.74% (Otter: 71.25%, Ristretto: 66.42%)
  - Large keyspace: 75.32% (Otter: 75.68%, Ristretto: 55.21%)
  - **Statistically equivalent to Otter** (< 1% difference)
  - W-TinyLFU algorithm proves both fast AND reliable

- **go-timecache Integration**: Integrated ultra-fast time caching for TTL operations
  - Replaced `time.Now().UnixNano()` with `timecache.CachedTimeNano()`  
  - ~121x faster than time.Now() with zero allocations
  - Performance boost: 3-6% improvement on all operations
  - Balanced workload now **39.49 ns/op** (was 42 ns/op)
  
- **Structured Error System**: Complete error handling with 28 error codes across 5 categories
  - Configuration errors (BALIOS_INVALID_*)
  - Operation errors (BALIOS_CACHE_FULL, etc.)
  - Loader errors (BALIOS_LOADER_*)
  - Persistence errors (BALIOS_SAVE_FAILED, etc.)
  - Internal errors (BALIOS_INTERNAL_ERROR, BALIOS_PANIC_RECOVERED)
- Error context with rich metadata for debugging
- Retry semantics (AsRetryable) for transient failures
- Severity levels (Critical, Warning) for error classification
- JSON serialization support for errors
- Error category checkers (IsConfigError, IsOperationError, etc.)
- Specific error checkers (IsNotFound, IsCacheFull, IsRetryable)
- Error information extractors (GetErrorCode, GetErrorContext)
- Integration with [go-errors](https://github.com/agilira/go-errors) library
- Comprehensive error documentation in `docs/ERRORS.md`
- 9 new error tests with benchmarks

### Performance
- **Single-threaded Operations:**
  - Set: **130.1 ns/op** (vs Otter 348.8ns - **2.67x faster**)
  - Get: **107.3 ns/op** (vs Otter 287.6ns - **2.68x faster**)
  - GenericCache Set: **134.0 ns/op** (+3% overhead)
  - GenericCache Get: **111.0 ns/op** (+3.4% overhead)

- **Parallel Workloads (GOMAXPROCS=12):**
  - Balanced (50/50 R/W): **41.2 ns/op** (vs Otter 144.1ns - **3.50x faster**)
  - ReadHeavy (90/10 R/W): **24.5 ns/op** (vs Otter 66.6ns - **2.72x faster**)
  - WriteHeavy (10/90 R/W): **54.9 ns/op** (vs Otter 271.3ns - **4.94x faster**)

- **Cache Quality:**
  - Hit ratio: **79.3%** (Otter: 79.7%, Ristretto: 62.8%)
  - Equivalent to best-in-class (< 0.5% difference from Otter)
  - **Fast AND reliable** - not just speed optimization

- **Race-Free Validation:**
  - All tests pass with `-race` detector
  - Zero data races in concurrent workloads
  - Atomic.Value cost: only +2ns per Set operation
  - Production-ready for high-concurrency scenarios

### Documentation
- Added comprehensive benchmark tables in README.md
  - Single-threaded operations comparison
  - Parallel workload performance (Balanced, ReadHeavy, WriteHeavy)
  - Hit ratio comparison with 4 workload patterns
  - Generic API overhead analysis
- Added "Why Choose Balios?" section summarizing advantages
- Updated headline to emphasize "fastest" status with proof
- Added race-free validation emphasis
- Added `docs/ERRORS.md` with complete error handling guide
- Updated README.md with current performance metrics
- Added error handling examples to README.md

### Testing
- Total tests: 52 (includes 2 new hit ratio tests)
- Code coverage: 86.1%
- **All tests pass with race detector** (-race flag validated)
- Hit ratio tests: 2 comprehensive tests with 1M+ total requests
- Hot reload tests: 6 tests, non-flaky with deterministic timing
- Generics tests: 10 tests covering all key/value type combinations
- Security scan: 0 issues (gosec validated)
- Benchmark suite: 15+ benchmarks comparing with Otter and Ristretto

## [0.1.0-dev] - 2025-01-15

### Added
- W-TinyLFU algorithm implementation
  - 4-bit frequency sketch with Count-Min Sketch
  - Lock-free operations using atomic primitives
  - Sampling-based eviction (5 candidates)
- TTL support with lazy expiration
- Cache operations: Set, Get, Delete, Has, Clear, Stats
- Configuration with sensible defaults
- TimeProvider interface for testability
- Logger interface for observability
- Comprehensive test suite (25 tests)
- Security hardening (gosec validated with #nosec annotations)
- Benchmark suite comparing with Otter and Ristretto

### Performance
- Set: 129 ns/op (2.6x faster than Otter)
- Get: 111 ns/op (1.2x faster than Otter)
- Balanced workload: 42 ns/op (3.5x faster than Otter)
- Hit ratio: 79.63% (vs Otter 78.36%, Ristretto 58.02%)
- Memory: 0-10 B/op per operation
- Zero allocations on Get, 1 allocation on Set

### Dependencies
- github.com/agilira/go-timecache v1.0.2
- Go 1.24+ required (for Otter v2 compatibility in benchmarks)

[Unreleased]: https://github.com/agilira/balios/compare/v0.1.0-dev...HEAD
[0.1.0-dev]: https://github.com/agilira/balios/releases/tag/v0.1.0-dev
