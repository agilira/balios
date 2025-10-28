// Package balios provides a high-performance, thread-safe, in-memory cache
// implementation using the W-TinyLFU (Window-TinyLFU) eviction algorithm.
//
// # Overview
//
// Balios is designed for production use with focus on:
//   - Performance: 108.7 ns/op Get, 135.5 ns/op Set (AMD Ryzen 5 7520U)
//   - Concurrency: Lock-free operations using atomic primitives
//   - Type Safety: Generic API with compile-time type checking
//   - Observability: OpenTelemetry integration (optional separate package)
//
// # Features
//
//   - W-TinyLFU Algorithm: Optimal cache hit ratio (combines frequency and recency)
//   - Lock-Free Design: Atomic operations for high concurrency
//   - Type-Safe Generics: GenericCache[K comparable, V any]
//   - TTL Support: Automatic expiration with lazy cleanup + manual ExpireNow() API
//   - GetOrLoad API: Cache stampede prevention with singleflight pattern
//   - Negative Caching: Cache loader errors to prevent repeated failures (v1.1.2+)
//   - Structured Errors: Rich error context with error codes
//   - Metrics Collection: MetricsCollector interface for observability
//
// # Quick Start
//
// Basic usage with generic API:
//
//	import "github.com/agilira/balios"
//
//	type User struct {
//	    ID   int
//	    Name string
//	}
//
//	func main() {
//	    // Create cache with generics (type-safe)
//	    cache := balios.NewGenericCache[string, User](balios.Config{
//	        MaxSize: 10_000,
//	        TTL:     time.Hour,
//	    })
//
//	    // Set value (type-safe, no interface{})
//	    cache.Set("user:123", User{ID: 123, Name: "Alice"})
//
//	    // Get value (no type assertion needed)
//	    if user, found := cache.Get("user:123"); found {
//	        fmt.Printf("User: %s\n", user.Name)
//	    }
//
//	    // Check stats
//	    stats := cache.Stats()
//	    fmt.Printf("Hit ratio: %.2f%%\n", stats.HitRatio()*100)
//	}
//
// # Cache Stampede Prevention
//
// The GetOrLoad API prevents cache stampede using singleflight pattern.
// Multiple concurrent requests for the same key execute the loader function only once:
//
//	user, err := cache.GetOrLoad("user:123", func() (User, error) {
//	    // This expensive operation runs only once
//	    // even if 1000 goroutines call GetOrLoad concurrently
//	    return fetchUserFromDB(123)
//	})
//	if err != nil {
//	    log.Printf("Failed to load user: %v", err)
//	}
//
// With context support for timeout and cancellation:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
//	defer cancel()
//
//	user, err := cache.GetOrLoadWithContext(ctx, "user:123",
//	    func(ctx context.Context) (User, error) {
//	        return fetchUserFromDBWithContext(ctx, 123)
//	    })
//
// Key characteristics of GetOrLoad:
//   - Cache hit: Same performance as Get() (27.90 ns/op parallel)
//   - Concurrent requests: N requests = 1 loader call (singleflight)
//   - Error handling: Errors can be cached with NegativeCacheTTL option (v1.1.2+)
//   - Panic recovery: Returns BALIOS_PANIC_RECOVERED error if loader panics
//
// # W-TinyLFU Algorithm
//
// W-TinyLFU (Window-TinyLFU) provides near-optimal cache hit ratios by combining:
//
//   - Window Cache: Recent items (20% of capacity) using LRU
//   - Main Cache: Frequent items (80% of capacity) using LFU with Count-Min Sketch
//   - Admission Policy: TinyLFU filter prevents one-hit-wonders from evicting valuable entries
//
// The algorithm achieves 90-95% of OPT (optimal) hit ratio in real-world workloads
// while maintaining O(1) time complexity for all operations.
//
// Memory overhead: ~4 bytes per cache entry (for frequency tracking).
//
// # Concurrency Model
//
// Balios uses a lock-free design with atomic operations:
//
//   - Reads: Atomic loads, no locks (except during eviction)
//   - Writes: CAS (Compare-And-Swap) operations
//   - Eviction: Fine-grained locking (only contested entries)
//   - Thread-Safe: All operations safe for concurrent use
//
// Benchmark with 8 goroutines:
//   - Get: 27.90 ns/op (8 parallel)
//   - Set: 39.47 ns/op (8 parallel)
//   - No deadlocks or race conditions
//
// # TTL (Time To Live)
//
// Automatic expiration with configurable TTL:
//
//	cache := balios.NewGenericCache[string, User](balios.Config{
//	    MaxSize: 10_000,
//	    TTL:     5 * time.Minute, // Entries expire after 5 minutes
//	})
//
// TTL features:
//   - Lazy Expiration: Checked on access, not proactive scanning
//   - Per-Entry Timestamps: Nanosecond precision
//   - Zero Overhead: No background goroutines
//   - Configurable: Set via Config.TTL
//
// # Observability
//
// Built-in stats tracking:
//
//	stats := cache.Stats()
//	fmt.Printf("Hits: %d, Misses: %d, Hit Ratio: %.2f%%\n",
//	    stats.Hits, stats.Misses, stats.HitRatio()*100)
//	fmt.Printf("Size: %d, Evictions: %d\n",
//	    stats.Size, stats.Evictions)
//
// Enterprise observability with OpenTelemetry (optional):
//
//	import baliosostel "github.com/agilira/balios/otel"
//
//	// Setup OTEL with Prometheus exporter
//	exporter, _ := prometheus.New()
//	provider := metric.NewMeterProvider(metric.WithReader(exporter))
//
//	// Create metrics collector
//	metricsCollector, _ := baliosostel.NewOTelMetricsCollector(provider)
//
//	// Configure cache with metrics
//	cache := balios.NewGenericCache[string, User](balios.Config{
//	    MaxSize:          10_000,
//	    MetricsCollector: metricsCollector, // Optional, zero overhead if nil
//	})
//
// Metrics exposed (via OpenTelemetry):
//   - balios_get_latency_ns: Histogram with automatic percentiles (p50, p95, p99, p99.9)
//   - balios_set_latency_ns: Set operation latencies
//   - balios_delete_latency_ns: Delete operation latencies
//   - balios_get_hits_total: Counter of cache hits
//   - balios_get_misses_total: Counter of cache misses
//   - balios_evictions_total: Counter of evictions
//
// The core balios package has zero OTEL dependencies. The balios/otel package
// is a separate module (~5% overhead when used).
//
// # Configuration
//
// Complete configuration options:
//
//	config := balios.Config{
//	    // Required: Maximum number of entries
//	    MaxSize: 10_000,
//
//	    // Optional: Time-to-live for entries (default: no expiration)
//	    TTL: time.Hour,
//
//	    // Optional: Negative cache TTL for loader errors (default: 0, disabled)
//	    // When enabled, failed loads are cached to prevent repeated expensive failures
//	    NegativeCacheTTL: 5 * time.Second,
//
//	    // Optional: Logger for errors and events (default: nil)
//	    Logger: myLogger,
//
//	    // Optional: Metrics collector (default: NoOp, zero overhead)
//	    MetricsCollector: metricsCollector,
//
//	    // Optional: Custom time provider for testing (default: real time)
//	    TimeProvider: myTimeProvider,
//	}
//
//	cache := balios.NewGenericCache[string, User](config)
//
// # Error Handling
//
// Balios uses structured errors with error codes:
//
//	user, err := cache.GetOrLoad("user:123", loader)
//	if err != nil {
//	    if errors.Is(err, balios.ErrLoaderPanic) {
//	        // Loader panicked, check error for details
//	        log.Printf("Loader panic: %v", err)
//	    } else if errors.Is(err, balios.ErrContextCanceled) {
//	        // Context was canceled
//	        log.Printf("Operation canceled: %v", err)
//	    } else {
//	        // Other loader error
//	        log.Printf("Loader failed: %v", err)
//	    }
//	    return
//	}
//
// Available error codes:
//   - BALIOS_EMPTY_KEY: Empty key provided (keys cannot be empty)
//   - BALIOS_INVALID_LOADER: Loader function is nil
//   - BALIOS_PANIC_RECOVERED: Loader function panicked (panic value included)
//   - BALIOS_LOADER_FAILED: Loader function returned error
//   - BALIOS_INVALID_CONFIG: Invalid configuration
//
// All errors implement error interface and can be unwrapped.
//
// # Performance
//
// Benchmark results (AMD Ryzen 5 7520U, Go 1.25+):
//
//	BenchmarkBalios_Set_SingleThread-8       7565774   159.8 ns/op    10 B/op    1 allocs/op
//	BenchmarkBalios_Get_SingleThread-8       9937665   118.9 ns/op     0 B/op    0 allocs/op
//	BenchmarkBalios_Set_Parallel-8          25257999    42.25 ns/op   10 B/op    1 allocs/op
//	BenchmarkBalios_Get_Parallel-8          42264384    24.99 ns/op    0 B/op    0 allocs/op
//
// Key characteristics:
//   - Zero allocations on hot path (Get/Set)
//   - Lock-free reads (except during eviction)
//   - Excellent parallel scalability
//   - Sub-microsecond latencies
//
// Hit ratio comparison (100K requests, Zipf distribution):
//   - Balios: 79.86%
//   - Balios-Generic: 79.71%
//   - Otter: 79.53%
//   - Ristretto: 71.19%
//
// # Memory Layout
//
// Internal structure (for 10,000 entry cache):
//
//	Cache Entry: 48 bytes (key hash + value pointer + metadata)
//	Hash Table:  160 KB (20,000 buckets * 8 bytes)
//	TinyLFU:     20 KB (Count-Min Sketch: 4 rows * 5,000 counters * 1 byte)
//	Window LRU:  96 KB (2,000 entries * 48 bytes)
//	Main Cache:  384 KB (8,000 entries * 48 bytes)
//	Total:       ~660 KB overhead + entry values
//
// Memory per entry: ~66 bytes overhead (excluding value size)
//
// # Thread Safety
//
// All cache operations are thread-safe:
//
//	cache := balios.NewGenericCache[string, int](balios.Config{MaxSize: 1000})
//
//	// Safe to use from multiple goroutines
//	go func() { cache.Set("key1", 1) }()
//	go func() { cache.Get("key1") }()
//	go func() { cache.Delete("key1") }()
//	go func() { stats := cache.Stats() }()
//
// Internal synchronization:
//   - Atomic operations for reads
//   - CAS for writes
//   - Fine-grained locks during eviction
//   - No global locks
//
// Tested with -race detector: zero race conditions detected.
//
// # Legacy Interface API
//
// Non-generic API for compatibility (uses interface{}):
//
//	cache := balios.NewCache(balios.Config{MaxSize: 10_000})
//
//	cache.Set("key", User{ID: 123, Name: "Alice"})
//	if value, found := cache.Get("key"); found {
//	    user := value.(User) // Type assertion required
//	    fmt.Printf("User: %s\n", user.Name)
//	}
//
// Prefer the generic API (NewGenericCache) for type safety.
//
// # Best Practices
//
// 1. Size the cache appropriately:
//   - Too small: High eviction rate, poor hit ratio
//   - Too large: Wasted memory, slower lookups
//   - Rule of thumb: ~2x your working set
//
// 2. Monitor hit ratio:
//   - Target: >70% for most workloads
//   - Low hit ratio indicates cache too small or poor key distribution
//
// 3. Use GetOrLoad for expensive operations:
//   - Prevents cache stampede
//   - Automatic deduplication of concurrent requests
//
// 4. Set appropriate TTL:
//   - Too short: More cache misses
//   - Too long: Stale data
//   - Consider data freshness requirements
//
// 5. Handle loader errors:
//   - Errors can be cached with NegativeCacheTTL to prevent repeated failures
//   - Implement retry logic in loader if needed
//   - Use context timeouts to prevent hanging
//
// 6. Use context with timeout:
//   - Prevents hanging on slow loaders
//   - Enables graceful cancellation
//
// 7. Enable metrics in production:
//   - Use balios/otel package for observability
//   - Monitor p95/p99 latencies
//   - Alert on low hit ratio or high eviction rate
//
// # Examples
//
// See the examples directory for complete working examples:
//   - examples/getorload/: GetOrLoad API usage
//   - examples/otel-prometheus/: OpenTelemetry + Prometheus + Grafana integration
//   - examples/errors/: Error handling patterns
//
// # Documentation
//
// Detailed documentation:
//   - docs/ARCHITECTURE.md: W-TinyLFU internals, lock-free design
//   - docs/PERFORMANCE.md: Benchmark results, hit ratio analysis
//   - docs/GETORLOAD.md: Cache stampede prevention, singleflight pattern
//   - docs/METRICS.md: Observability, PromQL queries, Grafana dashboards
//   - docs/ERRORS.md: Error codes, structured errors
//
// # Packages
//
//   - github.com/agilira/balios: Core cache implementation
//   - github.com/agilira/balios/otel: OpenTelemetry integration (separate module)
//
// # License
//
// See LICENSE file in the repository.
//
// Contributions welcome at https://github.com/agilira/balios
package balios
