# balios/otel - OpenTelemetry Integration

Professional OpenTelemetry integration for balios cache, enabling enterprise-grade observability with automatic percentile calculation and multi-backend support.

## Features

- **Automatic Percentiles**: OTEL Histograms automatically calculate p50, p95, p99, p99.9 latencies
- **Multi-Backend Support**: Works with Prometheus, Jaeger, DataDog, Grafana, and any OTEL-compatible backend
- **Hit Ratio Tracking**: Real-time cache hit/miss ratio monitoring
- **Eviction Monitoring**: Track cache evictions over time
- **Thread-Safe**: Lock-free, safe for concurrent use
- **Zero Core Impact**: Separate module, no impact on balios core performance
- **Industry Standard**: Uses OpenTelemetry, the CNCF standard for observability

## Installation

```bash
go get github.com/agilira/balios/otel
```

## Quick Start

```go
package main

import (
    "log"
    "net/http"

    "github.com/agilira/balios"
    baliosostel "github.com/agilira/balios/otel"
    "go.opentelemetry.io/otel/exporters/prometheus"
    "go.opentelemetry.io/otel/sdk/metric"
)

func main() {
    // Setup Prometheus exporter
    exporter, err := prometheus.New()
    if err != nil {
        log.Fatal(err)
    }

    // Create OTEL MeterProvider
    provider := metric.NewMeterProvider(metric.WithReader(exporter))

    // Create metrics collector
    metricsCollector, err := baliosostel.NewOTelMetricsCollector(provider)
    if err != nil {
        log.Fatal(err)
    }

    // Create cache with metrics
    cache, err := balios.NewCache[string, string](balios.Config{
        MaxSize:          10000,
        MetricsCollector: metricsCollector,
    })
    if err != nil {
        log.Fatal(err)
    }

    // Use cache
    cache.Set("key", "value")
    value, ok := cache.Get("key")

    // Expose metrics endpoint
    http.Handle("/metrics", exporter)
    log.Fatal(http.ListenAndServe(":2112", nil))
}
```

## Metrics Exposed

### Histograms (with automatic percentiles)

- `balios_get_latency_ns`: Get() operation latency in nanoseconds
- `balios_set_latency_ns`: Set() operation latency in nanoseconds  
- `balios_delete_latency_ns`: Delete() operation latency in nanoseconds

**Note**: OTEL automatically calculates percentiles (p50, p95, p99, p99.9) from histogram data.

### Counters

- `balios_get_hits_total`: Total number of cache hits
- `balios_get_misses_total`: Total number of cache misses
- `balios_evictions_total`: Total number of evictions

### Derived Metrics

From the above metrics, you can calculate:

- **Hit Ratio**: `balios_get_hits_total / (balios_get_hits_total + balios_get_misses_total)`
- **Miss Ratio**: `balios_get_misses_total / (balios_get_hits_total + balios_get_misses_total)`
- **Operations Rate**: `rate(balios_get_hits_total[1m]) + rate(balios_get_misses_total[1m])`

## Configuration Options

### Custom Meter Name

Use `WithMeterName()` to customize the OTEL meter name:

```go
collector, err := baliosostel.NewOTelMetricsCollector(
    provider,
    baliosostel.WithMeterName("my_service_balios"),
)
```

This is useful for:
- Distinguishing metrics from multiple cache instances
- Integrating with existing OTEL instrumentation
- Custom namespacing in multi-tenant environments

## Prometheus Integration

### PromQL Queries

**Get P95 Latency**:
```promql
histogram_quantile(0.95, rate(balios_get_latency_ns_bucket[5m]))
```

**Get P99 Latency**:
```promql
histogram_quantile(0.99, rate(balios_get_latency_ns_bucket[5m]))
```

**Hit Ratio (last 5 minutes)**:
```promql
rate(balios_get_hits_total[5m]) / 
(rate(balios_get_hits_total[5m]) + rate(balios_get_misses_total[5m]))
```

**Operations per Second**:
```promql
rate(balios_get_hits_total[1m]) + rate(balios_get_misses_total[1m])
```

**Evictions per Minute**:
```promql
rate(balios_evictions_total[1m]) * 60
```

### Grafana Dashboard

Example Grafana panels:

**Latency Panel** (Graph):
```json
{
  "targets": [
    {
      "expr": "histogram_quantile(0.50, rate(balios_get_latency_ns_bucket[5m]))",
      "legendFormat": "p50"
    },
    {
      "expr": "histogram_quantile(0.95, rate(balios_get_latency_ns_bucket[5m]))",
      "legendFormat": "p95"
    },
    {
      "expr": "histogram_quantile(0.99, rate(balios_get_latency_ns_bucket[5m]))",
      "legendFormat": "p99"
    }
  ]
}
```

**Hit Ratio Panel** (Gauge):
```json
{
  "targets": [
    {
      "expr": "rate(balios_get_hits_total[5m]) / (rate(balios_get_hits_total[5m]) + rate(balios_get_misses_total[5m]))",
      "legendFormat": "Hit Ratio"
    }
  ]
}
```

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    balios Cache                          │
│  (core module - no OTEL dependencies)                   │
└──────────────────┬──────────────────────────────────────┘
                   │ MetricsCollector interface
                   │
┌──────────────────▼──────────────────────────────────────┐
│              OTelMetricsCollector                        │
│         (balios/otel package)                            │
│  • Int64Histogram (latencies)                           │
│  • Int64Counter (hits/misses/evictions)                 │
└──────────────────┬──────────────────────────────────────┘
                   │ OTEL SDK
                   │
┌──────────────────▼──────────────────────────────────────┐
│              OTEL MeterProvider                          │
│  • Aggregates metrics                                    │
│  • Calculates percentiles                                │
└──────────────────┬──────────────────────────────────────┘
                   │
         ┌─────────┴─────────┬─────────────┐
         │                   │             │
    Prometheus          Jaeger        DataDog
    (HTTP)             (gRPC)         (HTTP)
```

## Performance

- **Overhead**: ~50-100ns per operation
- **Memory**: Allocation-free after initialization
- **Thread-Safety**: Lock-free OTEL instruments
- **Scalability**: Tested with 10+ goroutines, 1000+ ops/sec

Benchmark comparison:
```
BenchmarkCache_Get_NoMetrics    10000000    108.8 ns/op
BenchmarkCache_Get_WithOTel     9500000     115.2 ns/op  (+5.9%)
```

The overhead is minimal and acceptable for production use.

## Best Practices

### 1. Reuse MeterProvider

Create one MeterProvider per application and reuse it:

```go
// At application startup
provider := metric.NewMeterProvider(metric.WithReader(exporter))
defer provider.Shutdown(context.Background())

// Use for all caches
collector1, _ := baliosostel.NewOTelMetricsCollector(provider)
collector2, _ := baliosostel.NewOTelMetricsCollector(provider, 
    baliosostel.WithMeterName("cache2"))
```

### 2. Graceful Shutdown

Always shutdown the MeterProvider on application exit:

```go
defer func() {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    if err := provider.Shutdown(ctx); err != nil {
        log.Printf("Failed to shutdown meter provider: %v", err)
    }
}()
```

### 3. Configure Histogram Buckets

For optimal percentile accuracy, configure custom histogram buckets:

```go
provider := metric.NewMeterProvider(
    metric.WithReader(exporter),
    metric.WithView(metric.NewView(
        metric.Instrument{Name: "balios_get_latency_ns"},
        metric.Stream{
            Aggregation: metric.AggregationExplicitBucketHistogram{
                Boundaries: []float64{100, 500, 1000, 5000, 10000, 50000},
            },
        },
    )),
)
```

### 4. Monitor Eviction Rate

High eviction rates indicate the cache is too small:

```promql
rate(balios_evictions_total[5m]) > 100  # Alert if > 100 evictions/sec
```

### 5. Track Hit Ratio

Monitor hit ratio to ensure cache effectiveness:

```promql
rate(balios_get_hits_total[5m]) / 
(rate(balios_get_hits_total[5m]) + rate(balios_get_misses_total[5m])) < 0.8
# Alert if hit ratio drops below 80%
```

## Examples

See [examples/otel-prometheus/](../examples/otel-prometheus/) for a complete working example with:
- Prometheus exporter setup
- Docker Compose for Prometheus + Grafana
- Pre-configured Grafana dashboard
- Load testing script

## Troubleshooting

### Metrics Not Appearing

**Check MeterProvider is not nil**:
```go
if provider == nil {
    log.Fatal("MeterProvider is nil")
}
```

**Verify exporter is registered**:
```go
exporter, err := prometheus.New()
if err != nil {
    log.Fatal(err)
}
provider := metric.NewMeterProvider(metric.WithReader(exporter))
```

**Check metrics endpoint**:
```bash
curl http://localhost:2112/metrics | grep balios
```

### High Latency Reported

OTEL histograms measure end-to-end operation time including:
- Lock contention (if any)
- Memory allocation
- Eviction processing

This is correct behavior. If you see high p99 latencies, investigate:
- Cache size (too small = more evictions)
- Concurrent access patterns
- Memory pressure

### Race Detector Warnings

The OTEL SDK is race-free. If you see warnings, they're likely from:
- User code accessing shared state
- Improper cache configuration

Run tests with `-race` to identify issues:
```bash
go test -race ./...
```

## Compatibility

- **Go**: 1.23+
- **OpenTelemetry**: v1.31.0+
- **Prometheus**: v2.30.0+
- **Grafana**: v8.0.0+

## License

Same as balios core (see main repository LICENSE).

## Related

- [balios core](../) - Main cache implementation
- [Architecture docs](../docs/ARCHITECTURE.md) - W-TinyLFU internals
- [Performance docs](../docs/PERFORMANCE.md) - Benchmark results
- [Examples](../examples/) - Complete working examples
