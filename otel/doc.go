// Package otel provides OpenTelemetry integration for balios cache metrics.
//
// # Overview
//
// This package implements the balios.MetricsCollector interface using OpenTelemetry,
// enabling enterprise-grade observability with automatic percentile calculation and
// multi-backend support (Prometheus, Jaeger, DataDog, Grafana).
//
// The package is a separate module to keep the balios core lightweight.
// Applications that don't need metrics collection don't pay for the OTEL dependencies.
//
// # Features
//
//   - Automatic Percentiles: OTEL Histograms calculate p50, p95, p99, p99.9 latencies
//   - Multi-Backend Support: Works with Prometheus, Jaeger, DataDog, any OTEL-compatible backend
//   - Hit Ratio Tracking: Real-time cache hit/miss monitoring
//   - Eviction Monitoring: Track cache pressure and evictions
//   - Thread-Safe: Lock-free, safe for concurrent use
//   - Low Overhead: ~50-100ns per operation (~5% overhead)
//   - Industry Standard: Uses OpenTelemetry (CNCF standard)
//
// # Installation
//
//	go get github.com/agilira/balios/otel
//
// # Quick Start
//
// Basic setup with Prometheus exporter:
//
//	import (
//	    "github.com/agilira/balios"
//	    baliosostel "github.com/agilira/balios/otel"
//	    "go.opentelemetry.io/otel/exporters/prometheus"
//	    "go.opentelemetry.io/otel/sdk/metric"
//	)
//
//	// Setup Prometheus exporter
//	exporter, err := prometheus.New()
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Create OTEL MeterProvider
//	provider := metric.NewMeterProvider(metric.WithReader(exporter))
//	defer provider.Shutdown(context.Background())
//
//	// Create metrics collector
//	metricsCollector, err := baliosostel.NewOTelMetricsCollector(provider)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Configure cache with metrics
//	cache := balios.NewGenericCache[string, User](balios.Config{
//	    MaxSize:          10_000,
//	    MetricsCollector: metricsCollector,
//	})
//
//	// Use cache normally - metrics are automatically collected
//	cache.Set("key", value)
//	cache.Get("key")
//
//	// Expose metrics endpoint
//	http.Handle("/metrics", promhttp.Handler())
//	log.Fatal(http.ListenAndServe(":2112", nil))
//
// # Metrics Exposed
//
// Histograms (with automatic percentiles):
//   - balios_get_latency_ns: Get() operation latency in nanoseconds
//   - balios_set_latency_ns: Set() operation latency in nanoseconds
//   - balios_delete_latency_ns: Delete() operation latency in nanoseconds
//
// Counters:
//   - balios_get_hits_total: Total number of cache hits
//   - balios_get_misses_total: Total number of cache misses
//   - balios_evictions_total: Total number of evictions
//
// All metrics are thread-safe and use lock-free OTEL instruments.
//
// # Configuration
//
// Custom meter name (useful for multiple cache instances):
//
//	collector, err := baliosostel.NewOTelMetricsCollector(
//	    provider,
//	    baliosostel.WithMeterName("myapp_user_cache"),
//	)
//
// Custom histogram buckets for better percentile accuracy:
//
//	provider := metric.NewMeterProvider(
//	    metric.WithReader(exporter),
//	    metric.WithView(metric.NewView(
//	        metric.Instrument{Name: "balios_get_latency_ns"},
//	        metric.Stream{
//	            Aggregation: metric.AggregationExplicitBucketHistogram{
//	                // Buckets in nanoseconds: 100ns, 500ns, 1μs, 5μs, 10μs, 50μs, 100μs
//	                Boundaries: []float64{100, 500, 1000, 5000, 10000, 50000, 100000},
//	            },
//	        },
//	    )),
//	)
//
// # Prometheus Queries
//
// Calculate P95 latency (last 5 minutes):
//
//	histogram_quantile(0.95, rate(balios_get_latency_ns_bucket[5m]))
//
// Calculate P99 latency:
//
//	histogram_quantile(0.99, rate(balios_get_latency_ns_bucket[5m]))
//
// Calculate hit ratio:
//
//	rate(balios_get_hits_total[5m]) /
//	(rate(balios_get_hits_total[5m]) + rate(balios_get_misses_total[5m]))
//
// Calculate operations per second:
//
//	rate(balios_get_hits_total[1m]) + rate(balios_get_misses_total[1m])
//
// Calculate evictions per minute:
//
//	rate(balios_evictions_total[1m]) * 60
//
// # Grafana Integration
//
// Example Grafana panel for latency percentiles:
//
//	{
//	  "targets": [
//	    {
//	      "expr": "histogram_quantile(0.50, rate(balios_get_latency_ns_bucket[5m]))",
//	      "legendFormat": "p50"
//	    },
//	    {
//	      "expr": "histogram_quantile(0.95, rate(balios_get_latency_ns_bucket[5m]))",
//	      "legendFormat": "p95"
//	    },
//	    {
//	      "expr": "histogram_quantile(0.99, rate(balios_get_latency_ns_bucket[5m]))",
//	      "legendFormat": "p99"
//	    }
//	  ]
//	}
//
// See examples/otel-prometheus/ for a complete Grafana dashboard with 6 pre-configured panels.
//
// # Performance
//
// Overhead measurements (compared to cache without metrics):
//
//	BenchmarkCache_Get_NoMetrics    11033690    108.8 ns/op
//	BenchmarkCache_Get_WithOTel     10481242    114.5 ns/op  (+5.2%)
//
// The overhead is minimal (~5%) and acceptable for production use.
// The core balios package uses a nil check before calling metrics methods,
// so there's zero overhead when MetricsCollector is nil (default).
//
// # Architecture
//
// Separation of concerns:
//
//	┌─────────────────────────────────────┐
//	│     balios Cache (Core Module)      │
//	│  • No OTEL dependencies             │
//	│  • MetricsCollector interface       │
//	│  • NoOpMetricsCollector (default)   │
//	└──────────────┬──────────────────────┘
//	               │
//	               │ implements
//	               ▼
//	┌─────────────────────────────────────┐
//	│    balios/otel (This Package)       │
//	│  • OTelMetricsCollector             │
//	│  • OTEL SDK dependencies            │
//	│  • Histograms + Counters            │
//	└──────────────┬──────────────────────┘
//	               │
//	               │ exports to
//	               ▼
//	┌─────────────────────────────────────┐
//	│      OTEL MeterProvider             │
//	│  • Aggregates metrics               │
//	│  • Calculates percentiles           │
//	│  • Exports to backends              │
//	└──────────────┬──────────────────────┘
//	               │
//	     ┌─────────┴──────┬────────┐
//	     ▼                ▼        ▼
//	Prometheus        Jaeger   DataDog
//
// This architecture keeps the core lightweight while enabling enterprise observability
// as an optional add-on.
//
// # Thread Safety
//
// All methods are thread-safe and use lock-free OTEL instruments:
//
//	collector, _ := baliosostel.NewOTelMetricsCollector(provider)
//
//	// Safe to call from multiple goroutines
//	go func() { collector.RecordGet(1000, true) }()
//	go func() { collector.RecordSet(2000) }()
//	go func() { collector.RecordDelete(500) }()
//	go func() { collector.RecordEviction() }()
//
// Tested with -race detector: zero race conditions detected (9 concurrent tests passing).
//
// # Best Practices
//
// 1. Reuse MeterProvider across cache instances:
//
//	provider := metric.NewMeterProvider(metric.WithReader(exporter))
//	defer provider.Shutdown(context.Background())
//
//	collector1, _ := baliosostel.NewOTelMetricsCollector(provider)
//	collector2, _ := baliosostel.NewOTelMetricsCollector(provider,
//	    baliosostel.WithMeterName("cache2"))
//
// 2. Always shutdown MeterProvider on exit:
//
//	defer func() {
//	    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
//	    defer cancel()
//	    if err := provider.Shutdown(ctx); err != nil {
//	        log.Printf("Failed to shutdown meter provider: %v", err)
//	    }
//	}()
//
// 3. Configure histogram buckets based on your latency profile:
//
//	// For sub-microsecond caches (very fast)
//	Boundaries: []float64{50, 100, 200, 500, 1000, 2000, 5000}
//
//	// For microsecond-range caches (typical)
//	Boundaries: []float64{100, 500, 1000, 5000, 10000, 50000, 100000}
//
// 4. Monitor key metrics:
//   - Hit ratio: Target >80%
//   - P95 latency: Target <1μs
//   - P99 latency: Target <5μs
//   - Eviction rate: Target <10% of cache size per minute
//
// 5. Set up alerts:
//   - Low hit ratio (<70%)
//   - High P99 latency (>10μs)
//   - High eviction rate (>100 evictions/sec)
//
// # Troubleshooting
//
// Metrics not appearing:
//   - Verify MeterProvider is not nil
//   - Check exporter is registered with provider
//   - Ensure metrics endpoint is accessible
//   - Verify Prometheus is scraping the endpoint
//
// High latency reported:
//   - OTEL measures end-to-end time including lock contention
//   - Check p99 for tail latencies (may indicate GC or eviction spikes)
//   - Consider increasing cache size if eviction rate is high
//
// Memory usage:
//   - OTEL histograms use memory for buckets (~10-50 bytes per metric)
//   - Cardinality matters: avoid high-cardinality labels
//   - Configure retention in Prometheus to limit storage
//
// # Examples
//
// Complete working example with Docker Compose:
//
//	examples/otel-prometheus/
//	├── main.go              # Application with simulated workload
//	├── docker-compose.yml   # Prometheus + Grafana stack
//	├── prometheus.yml       # Scrape configuration
//	├── grafana-dashboard.json  # Pre-configured dashboard
//	└── README.md           # Setup instructions
//
// Run the example:
//
//	cd examples/otel-prometheus
//	docker-compose up -d
//	go run main.go
//
// Access:
//   - Metrics: http://localhost:2112/metrics
//   - Prometheus: http://localhost:9090
//   - Grafana: http://localhost:3000 (admin/admin)
//
// # Documentation
//
// Detailed guides:
//   - otel/README.md: This package documentation
//   - docs/METRICS.md: Complete metrics and observability guide
//   - examples/otel-prometheus/README.md: Example setup instructions
//
// # Compatibility
//
//   - Go: 1.23+
//   - OpenTelemetry: v1.31.0+
//   - Prometheus: v2.30.0+
//   - Grafana: v8.0.0+
//
// # Testing
//
// The package includes 9 comprehensive tests covering:
//   - Interface implementation verification
//   - Constructor validation (nil provider handling)
//   - Metric recording for all operations (Get/Set/Delete/Eviction)
//   - Metric collection and verification
//   - Thread safety (concurrent test with 10 goroutines)
//   - Custom options (meter name)
//
// Run tests:
//
//	cd otel
//	go test -v           # Run all tests
//	go test -race        # Run with race detector
//
// All tests pass with -race detector, confirming zero race conditions.
//
// # Version
//
// Current version: v0.1.0 (matches balios core)
//
// # License
//
// Same as balios core (see LICENSE in main repository).
package otel
