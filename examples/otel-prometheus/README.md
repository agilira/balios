# balios + OpenTelemetry + Prometheus Example

Complete example demonstrating enterprise-grade observability for balios cache with OpenTelemetry, Prometheus, and Grafana.

## Features

- ✅ **OpenTelemetry Integration**: Professional metrics collection
- ✅ **Prometheus Scraping**: Industry-standard metrics storage
- ✅ **Grafana Dashboard**: Beautiful visualization with percentiles (p50, p95, p99, p99.9)
- ✅ **Docker Compose**: One-command setup
- ✅ **Realistic Workload**: Simulated cache operations (70% reads, 20% writes, 10% deletes)
- ✅ **Hit Ratio Tracking**: Real-time cache effectiveness monitoring

## Quick Start

### Option 1: Run Locally (No Docker)

```bash
# Install dependencies
go mod tidy

# Run the example
go run main.go
```

Access metrics at: http://localhost:2112/metrics

### Option 2: Run with Docker Compose (Full Stack)

```bash
# Start all services (app + Prometheus + Grafana)
docker-compose up -d

# Check logs
docker-compose logs -f app

# Stop services
docker-compose down
```

Access:
- **Metrics**: http://localhost:2112/metrics
- **Prometheus**: http://localhost:9090
- **Grafana**: http://localhost:3000 (login: admin/admin)

## Grafana Dashboard

The dashboard is automatically provisioned when using Docker Compose. It includes:

### Panels

1. **Cache Hit Ratio** (Gauge)
   - Current hit ratio as percentage
   - Color-coded: Red (<70%), Yellow (70-85%), Green (>85%)

2. **Operations per Second** (Time Series)
   - Hits/sec, Misses/sec, Total ops/sec
   - Shows traffic patterns

3. **Get Latency Percentiles** (Time Series)
   - p50, p95, p99, p99.9 latencies
   - Identifies performance outliers

4. **Set Latency Percentiles** (Time Series)
   - Write operation latencies
   - Monitors write performance

5. **Eviction Rate** (Time Series)
   - Evictions per second
   - High rate indicates cache is too small

6. **Delete Latency Percentiles** (Time Series)
   - Delete operation latencies
   - Monitors cleanup performance

### Manual Dashboard Import (if needed)

1. Open Grafana: http://localhost:3000
2. Login: admin/admin
3. Go to **Dashboards** → **Import**
4. Upload `grafana-dashboard.json`
5. Select **Prometheus** data source
6. Click **Import**

## Metrics Exposed

### Histograms (with percentiles)

- `balios_get_latency_ns`: Get operation latency in nanoseconds
- `balios_set_latency_ns`: Set operation latency in nanoseconds
- `balios_delete_latency_ns`: Delete operation latency in nanoseconds

### Counters

- `balios_get_hits_total`: Total cache hits
- `balios_get_misses_total`: Total cache misses
- `balios_evictions_total`: Total evictions

## Example PromQL Queries

### Get P95 Latency (nanoseconds)
```promql
histogram_quantile(0.95, rate(balios_get_latency_ns_bucket[5m]))
```

### Get P99 Latency (nanoseconds)
```promql
histogram_quantile(0.99, rate(balios_get_latency_ns_bucket[5m]))
```

### Hit Ratio (last 5 minutes)
```promql
rate(balios_get_hits_total[5m]) / 
(rate(balios_get_hits_total[5m]) + rate(balios_get_misses_total[5m]))
```

### Operations per Second
```promql
rate(balios_get_hits_total[1m]) + rate(balios_get_misses_total[1m])
```

### Evictions per Minute
```promql
rate(balios_evictions_total[1m]) * 60
```

### Average Get Latency (last 5m)
```promql
rate(balios_get_latency_ns_sum[5m]) / rate(balios_get_latency_ns_count[5m])
```

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                 balios Cache Application                 │
│  • Simulated workload (70% Get, 20% Set, 10% Delete)   │
│  • OTelMetricsCollector integrated                      │
│  • Metrics exposed on :2112/metrics                     │
└────────────────────┬────────────────────────────────────┘
                     │
                     │ HTTP (Prometheus scrape format)
                     ▼
┌─────────────────────────────────────────────────────────┐
│                    Prometheus :9090                      │
│  • Scrapes metrics every 15s                            │
│  • Stores time-series data                              │
│  • Calculates percentiles from histograms               │
└────────────────────┬────────────────────────────────────┘
                     │
                     │ PromQL queries
                     ▼
┌─────────────────────────────────────────────────────────┐
│                     Grafana :3000                        │
│  • Visualizes metrics in real-time                      │
│  • Pre-configured dashboard                             │
│  • Alerts and notifications (optional)                  │
└─────────────────────────────────────────────────────────┘
```

## Workload Simulation

The example simulates realistic cache usage:

- **Pre-population**: 500 entries loaded at startup
- **Operation Mix**: 70% Gets, 20% Sets, 10% Deletes
- **Hit Ratio**: ~80% hits (80% existing keys, 20% missing keys)
- **Rate**: ~100 operations per second (configurable)
- **Key Distribution**: Random access pattern

### Expected Metrics

With the default workload:

- **Hit Ratio**: ~80% (0.80)
- **Get Latency p50**: ~100-200ns
- **Get Latency p95**: ~500-1000ns
- **Get Latency p99**: ~1-5μs
- **Operations/sec**: ~100 ops/sec
- **Evictions/sec**: ~2-5 evictions/sec (depends on cache size)

## Customization

### Change Cache Size

Edit `main.go`:
```go
cache, err := balios.NewCache[string, string](balios.Config{
    MaxSize:          10000,  // Change this value
    MetricsCollector: metricsCollector,
})
```

### Change Workload Pattern

Edit `runWorkload()` in `main.go`:
```go
// Change operation mix (currently 70% gets, 20% sets, 10% deletes)
switch {
case op < 0.70:  // Get operations
    // ...
case op < 0.90:  // Set operations
    // ...
default:         // Delete operations
    // ...
}
```

### Change Histogram Buckets

Edit `main.go` in the `metric.WithView()` configuration:
```go
metric.WithView(metric.NewView(
    metric.Instrument{Name: "balios_get_latency_ns"},
    metric.Stream{
        Aggregation: metric.AggregationExplicitBucketHistogram{
            Boundaries: []float64{100, 500, 1000, 5000, 10000, 50000, 100000},
        },
    },
)),
```

### Change Scrape Interval

Edit `prometheus.yml`:
```yaml
global:
  scrape_interval: 15s  # Change this value
```

## Alerting Examples

### Prometheus Alert Rules

Create `alerts.yml`:
```yaml
groups:
  - name: balios_alerts
    interval: 30s
    rules:
      # Alert when hit ratio drops below 80%
      - alert: LowHitRatio
        expr: |
          rate(balios_get_hits_total[5m]) / 
          (rate(balios_get_hits_total[5m]) + rate(balios_get_misses_total[5m])) < 0.8
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Cache hit ratio below 80%"
          description: "Hit ratio is {{ $value | humanizePercentage }}"

      # Alert when p99 latency exceeds 10μs
      - alert: HighLatency
        expr: |
          histogram_quantile(0.99, rate(balios_get_latency_ns_bucket[5m])) > 10000
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "P99 latency above 10μs"
          description: "P99 latency is {{ $value }}ns"

      # Alert when eviction rate is high
      - alert: HighEvictionRate
        expr: rate(balios_evictions_total[1m]) > 100
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High eviction rate"
          description: "Eviction rate is {{ $value | humanize }} evictions/sec"
```

Add to `prometheus.yml`:
```yaml
rule_files:
  - "alerts.yml"
```

## Troubleshooting

### Metrics Not Showing in Prometheus

1. **Check app is running**: `curl http://localhost:2112/metrics`
2. **Check Prometheus targets**: http://localhost:9090/targets
3. **Verify scrape config**: Check `prometheus.yml`

### Dashboard Empty in Grafana

1. **Check data source**: Grafana → Configuration → Data Sources
2. **Verify Prometheus URL**: Should be `http://prometheus:9090`
3. **Test connection**: Click "Test" in data source config
4. **Check time range**: Adjust time range in dashboard (top-right)

### Docker Compose Issues

```bash
# Check service logs
docker-compose logs app
docker-compose logs prometheus
docker-compose logs grafana

# Restart services
docker-compose restart

# Rebuild from scratch
docker-compose down -v
docker-compose up -d --build
```

### High Memory Usage

If Prometheus uses too much memory, reduce retention:

Edit `docker-compose.yml`:
```yaml
prometheus:
  command:
    - '--storage.tsdb.retention.time=1h'  # Keep only 1 hour of data
```

## Performance Considerations

- **Metrics Overhead**: ~50-100ns per operation (negligible)
- **Prometheus Scraping**: No impact on cache performance
- **Histogram Buckets**: More buckets = better percentile accuracy but more memory
- **Scrape Interval**: Lower interval = fresher data but more storage

## Production Recommendations

1. **Authentication**: Enable authentication for Prometheus and Grafana
2. **TLS**: Use HTTPS for metrics endpoint
3. **Retention**: Configure appropriate data retention
4. **Backup**: Backup Prometheus data and Grafana dashboards
5. **High Availability**: Run multiple Prometheus instances
6. **Remote Storage**: Use remote storage (Thanos, Cortex) for long-term retention

## Related

- [balios/otel package](../../otel/) - OpenTelemetry collector documentation
- [balios core](../../) - Main cache documentation
- [OpenTelemetry Go](https://opentelemetry.io/docs/instrumentation/go/) - OTEL Go docs
- [Prometheus](https://prometheus.io/docs/) - Prometheus documentation
- [Grafana](https://grafana.com/docs/) - Grafana documentation
