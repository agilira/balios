// Package otel provides OpenTelemetry integration for balios cache metrics.
//
// This package implements the balios.MetricsCollector interface using OpenTelemetry,
// enabling enterprise-grade observability with automatic percentile calculation (p50, p95, p99)
// and multi-backend support (Prometheus, Jaeger, DataDog, Grafana).
//
// # Features
//
//   - Automatic percentile calculation via OTEL Histograms (p50, p95, p99, p99.9)
//   - Hit/miss ratio tracking with counters
//   - Eviction monitoring
//   - Thread-safe, lock-free implementation
//   - Compatible with any OTEL backend (Prometheus, Jaeger, DataDog, etc.)
//   - Optional: separate module, no impact on core balios performance
//
// # Usage
//
//	import (
//	    "github.com/agilira/balios"
//	    baliosostel "github.com/agilira/balios/otel"
//	    "go.opentelemetry.io/otel/exporters/prometheus"
//	    "go.opentelemetry.io/otel/sdk/metric"
//	)
//
//	// Setup OTEL with Prometheus exporter
//	exporter, _ := prometheus.New()
//	provider := metric.NewMeterProvider(metric.WithReader(exporter))
//
//	// Create collector
//	metricsCollector, _ := baliosostel.NewOTelMetricsCollector(provider)
//
//	// Configure balios cache
//	cache, _ := balios.NewCache[string, string](balios.Config{
//	    MaxSize:          10000,
//	    MetricsCollector: metricsCollector,
//	})
//
// # Metrics Exposed
//
//   - balios_get_latency_ns: Histogram of Get() operation latencies in nanoseconds
//   - balios_set_latency_ns: Histogram of Set() operation latencies in nanoseconds
//   - balios_delete_latency_ns: Histogram of Delete() operation latencies in nanoseconds
//   - balios_get_hits_total: Counter of cache hits
//   - balios_get_misses_total: Counter of cache misses
//   - balios_evictions_total: Counter of evictions
//   - balios_expirations_total: Counter of TTL-based expirations
//
// All metrics are automatically aggregated by the OTEL SDK and can be exported to
// any OTEL-compatible backend. Histograms automatically calculate percentiles (p50, p95, p99).
// bounded_probing_test.go: tests for bounded linear probing improvement (v1.1.35)
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGILira fragment
// SPDX-License-Identifier: MPL-2.0
package otel

import (
	"context"
	"errors"

	"github.com/agilira/balios"
	"go.opentelemetry.io/otel/metric"
)

// OTelMetricsCollector implements balios.MetricsCollector using OpenTelemetry.
//
// This collector records cache operations to OpenTelemetry metrics, enabling
// enterprise-grade observability with automatic percentile calculation and
// multi-backend support.
//
// Thread-safety: Safe for concurrent use by multiple goroutines.
// The underlying OTEL instruments are thread-safe and lock-free.
//
// Performance: Minimal overhead (<100ns per operation), allocation-free after initialization.
type OTelMetricsCollector struct {
	// OTEL instruments for recording metrics
	getLatency    metric.Int64Histogram // Get operation latency histogram
	setLatency    metric.Int64Histogram // Set operation latency histogram
	deleteLatency metric.Int64Histogram // Delete operation latency histogram
	hits          metric.Int64Counter   // Cache hits counter
	misses        metric.Int64Counter   // Cache misses counter
	evictions     metric.Int64Counter   // Evictions counter
	expirations   metric.Int64Counter   // Expirations counter
}

// Options for configuring OTelMetricsCollector.
type Options struct {
	// MeterName is the name of the OpenTelemetry meter.
	// Default: "github.com/agilira/balios"
	MeterName string
}

// Option is a functional option for configuring OTelMetricsCollector.
type Option func(*Options)

// WithMeterName sets a custom meter name.
// This is useful for distinguishing metrics from multiple cache instances
// or integrating with existing OTEL instrumentation.
func WithMeterName(name string) Option {
	return func(o *Options) {
		o.MeterName = name
	}
}

// NewOTelMetricsCollector creates a new OpenTelemetry metrics collector.
//
// Parameters:
//   - provider: OpenTelemetry MeterProvider. Must not be nil.
//   - opts: Optional configuration options (meter name, etc.)
//
// Returns:
//   - *OTelMetricsCollector: The collector instance
//   - error: ErrNilMeterProvider if provider is nil, or OTEL instrument creation errors
//
// The collector creates the following OTEL instruments:
//   - Int64Histogram for latencies (Get, Set, Delete)
//   - Int64Counter for hits, misses, evictions
//
// All instruments are thread-safe and lock-free.
//
// Example:
//
//	exporter, _ := prometheus.New()
//	provider := metric.NewMeterProvider(metric.WithReader(exporter))
//	collector, err := NewOTelMetricsCollector(provider)
//	if err != nil {
//	    log.Fatal(err)
//	}
func NewOTelMetricsCollector(provider metric.MeterProvider, opts ...Option) (*OTelMetricsCollector, error) {
	if provider == nil {
		return nil, errors.New("meter provider cannot be nil")
	}

	// Apply options
	options := Options{
		MeterName: "github.com/agilira/balios",
	}
	for _, opt := range opts {
		opt(&options)
	}

	// Create meter
	meter := provider.Meter(options.MeterName)

	// Create collector
	collector := &OTelMetricsCollector{}

	// Create Get latency histogram
	var err error
	collector.getLatency, err = meter.Int64Histogram(
		"balios_get_latency_ns",
		metric.WithDescription("Latency of Get operations in nanoseconds"),
		metric.WithUnit("ns"),
	)
	if err != nil {
		return nil, err
	}

	// Create Set latency histogram
	collector.setLatency, err = meter.Int64Histogram(
		"balios_set_latency_ns",
		metric.WithDescription("Latency of Set operations in nanoseconds"),
		metric.WithUnit("ns"),
	)
	if err != nil {
		return nil, err
	}

	// Create Delete latency histogram
	collector.deleteLatency, err = meter.Int64Histogram(
		"balios_delete_latency_ns",
		metric.WithDescription("Latency of Delete operations in nanoseconds"),
		metric.WithUnit("ns"),
	)
	if err != nil {
		return nil, err
	}

	// Create hits counter
	collector.hits, err = meter.Int64Counter(
		"balios_get_hits_total",
		metric.WithDescription("Total number of cache hits"),
	)
	if err != nil {
		return nil, err
	}

	// Create misses counter
	collector.misses, err = meter.Int64Counter(
		"balios_get_misses_total",
		metric.WithDescription("Total number of cache misses"),
	)
	if err != nil {
		return nil, err
	}

	// Create evictions counter
	collector.evictions, err = meter.Int64Counter(
		"balios_evictions_total",
		metric.WithDescription("Total number of evictions"),
	)
	if err != nil {
		return nil, err
	}

	// Create expirations counter
	collector.expirations, err = meter.Int64Counter(
		"balios_expirations_total",
		metric.WithDescription("Total number of TTL-based expirations"),
	)
	if err != nil {
		return nil, err
	}

	return collector, nil
}

// RecordGet records a Get operation.
//
// Parameters:
//   - latencyNs: Operation latency in nanoseconds. Must be >= 0.
//   - hit: Whether the operation was a cache hit (true) or miss (false).
//
// This method:
//   - Records latency to the Get latency histogram (used for percentile calculation)
//   - Increments either hits or misses counter
//
// Thread-safety: Safe for concurrent use.
// Performance: ~50-100ns overhead, allocation-free.
func (c *OTelMetricsCollector) RecordGet(latencyNs int64, hit bool) {
	ctx := context.Background()

	// Record latency histogram
	c.getLatency.Record(ctx, latencyNs)

	// Increment hit/miss counter
	if hit {
		c.hits.Add(ctx, 1)
	} else {
		c.misses.Add(ctx, 1)
	}
}

// RecordSet records a Set operation.
//
// Parameters:
//   - latencyNs: Operation latency in nanoseconds. Must be >= 0.
//
// This method records latency to the Set latency histogram.
//
// Thread-safety: Safe for concurrent use.
// Performance: ~50-100ns overhead, allocation-free.
func (c *OTelMetricsCollector) RecordSet(latencyNs int64) {
	c.setLatency.Record(context.Background(), latencyNs)
}

// RecordDelete records a Delete operation.
//
// Parameters:
//   - latencyNs: Operation latency in nanoseconds. Must be >= 0.
//
// This method records latency to the Delete latency histogram.
//
// Thread-safety: Safe for concurrent use.
// Performance: ~50-100ns overhead, allocation-free.
func (c *OTelMetricsCollector) RecordDelete(latencyNs int64) {
	c.deleteLatency.Record(context.Background(), latencyNs)
}

// RecordEviction records an eviction event.
//
// This method increments the evictions counter.
//
// Thread-safety: Safe for concurrent use.
// Performance: ~50-100ns overhead, allocation-free.
func (c *OTelMetricsCollector) RecordEviction() {
	c.evictions.Add(context.Background(), 1)
}

// RecordExpiration records a TTL-based expiration event.
//
// This method increments the expirations counter.
//
// Thread-safety: Safe for concurrent use.
// Performance: ~50-100ns overhead, allocation-free.
func (c *OTelMetricsCollector) RecordExpiration() {
	c.expirations.Add(context.Background(), 1)
}

// Compile-time interface check
var _ balios.MetricsCollector = (*OTelMetricsCollector)(nil)
