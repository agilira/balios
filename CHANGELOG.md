# Changelog

All notable changes to Balios will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- **Optimized Generics Performance**: Zero-allocation optimizations for common key types
  - Type switch dispatching for string, int, uint types (11 variants)
  - String keys: +5.4ns overhead (16%) vs non-generic - **Set 39.5ns, Get 19.1ns**
  - Int keys: +24ns overhead (includes required int→string conversion) - **Set 58.3ns, Get 20.2ns**
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
- Set: **124.8 ns/op** (3.3% faster, was 129 ns/op)
- Get: **106.8 ns/op** (3.8% faster, was 111 ns/op)  
- Balanced: **39.49 ns/op** (6.0% faster, was 42 ns/op)
- **4.5x faster than Otter** on balanced workloads
- Error creation: 112 ns/op (simple), 272 ns/op (with context)
- Error checking: 3.8 ns/op (HasCode), zero allocations

### Documentation
- Added `docs/ERRORS.md` with complete error handling guide
- Updated README.md with current performance metrics
- Updated README.md with Phase 1 completion status
- Added error handling examples to README.md

### Testing
- Total tests: 50 (from 25)
- Code coverage: 86.1%
- All tests pass with race detector
- Hot reload tests: 6 tests, non-flaky with deterministic timing
- Generics tests: 10 tests covering all key/value type combinations
- Security scan: 0 issues (gosec validated)

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
