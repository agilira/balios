// main_test.go: tests for otel-prometheus example
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGILira library
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"testing"
	"time"

	"github.com/agilira/balios"
	baliosostel "github.com/agilira/balios/otel"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/metric"
)

func TestPrometheusSetup(t *testing.T) {
	// Setup Prometheus exporter
	exporter, err := prometheus.New()
	if err != nil {
		t.Fatalf("Failed to create Prometheus exporter: %v", err)
	}

	// Create meter provider
	provider := metric.NewMeterProvider(metric.WithReader(exporter))

	if provider == nil {
		t.Fatal("MeterProvider should not be nil")
	}
}

func TestOTelMetricsCollector(t *testing.T) {
	// Setup Prometheus exporter
	exporter, err := prometheus.New()
	if err != nil {
		t.Fatalf("Failed to create Prometheus exporter: %v", err)
	}

	// Create meter provider
	provider := metric.NewMeterProvider(metric.WithReader(exporter))

	// Create OTEL metrics collector
	metricsCollector, err := baliosostel.NewOTelMetricsCollector(provider)
	if err != nil {
		t.Fatalf("Failed to create OTEL metrics collector: %v", err)
	}

	if metricsCollector == nil {
		t.Fatal("MetricsCollector should not be nil")
	}
}

func TestCacheWithOTelMetrics(t *testing.T) {
	// Setup Prometheus exporter
	exporter, err := prometheus.New()
	if err != nil {
		t.Fatalf("Failed to create Prometheus exporter: %v", err)
	}

	// Create meter provider
	provider := metric.NewMeterProvider(metric.WithReader(exporter))

	// Create OTEL metrics collector
	metricsCollector, err := baliosostel.NewOTelMetricsCollector(provider)
	if err != nil {
		t.Fatalf("Failed to create OTEL metrics collector: %v", err)
	}

	// Create cache with OTEL metrics
	cache := balios.NewGenericCache[string, string](balios.Config{
		MaxSize:          100,
		MetricsCollector: metricsCollector,
	})
	defer func() { _ = cache.Close() }()

	// Test basic operations (metrics should be collected)
	cache.Set("key1", "value1")
	cache.Set("key2", "value2")
	cache.Set("key3", "value3")

	value, found := cache.Get("key1")
	if !found || value != "value1" {
		t.Errorf("Expected to find key1=value1")
	}

	// Cache miss
	_, found = cache.Get("nonexistent")
	if found {
		t.Error("Should not find nonexistent key")
	}

	// Delete
	cache.Delete("key2")

	// Verify stats
	stats := cache.Stats()
	if stats.Sets != 3 {
		t.Errorf("Expected 3 sets, got %d", stats.Sets)
	}

	if stats.Hits == 0 {
		t.Error("Expected at least one hit")
	}

	if stats.Misses == 0 {
		t.Error("Expected at least one miss")
	}
}

func TestRunWorkloadSimulation(t *testing.T) {
	// This is a quick smoke test of the workload simulation
	// We just verify it doesn't panic or error

	// Setup
	exporter, err := prometheus.New()
	if err != nil {
		t.Fatalf("Failed to create Prometheus exporter: %v", err)
	}

	provider := metric.NewMeterProvider(metric.WithReader(exporter))

	metricsCollector, err := baliosostel.NewOTelMetricsCollector(provider)
	if err != nil {
		t.Fatalf("Failed to create OTEL metrics collector: %v", err)
	}

	cache := balios.NewGenericCache[string, string](balios.Config{
		MaxSize:          100,
		TTL:              5 * time.Minute,
		MetricsCollector: metricsCollector,
	})
	defer func() { _ = cache.Close() }()

	// Run a very short workload (just a few operations)
	const numOps = 10

	for i := 0; i < numOps; i++ {
		key := "user:" + string(rune(i%10))
		cache.Set(key, "data")
		cache.Get(key)
	}

	// Verify some operations happened
	stats := cache.Stats()
	if stats.Sets == 0 {
		t.Error("Expected some Set operations")
	}

	if stats.Hits == 0 {
		t.Error("Expected some Get hits")
	}
}

func TestCacheOperationLatencies(t *testing.T) {
	exporter, err := prometheus.New()
	if err != nil {
		t.Fatalf("Failed to create Prometheus exporter: %v", err)
	}

	provider := metric.NewMeterProvider(metric.WithReader(exporter))

	metricsCollector, err := baliosostel.NewOTelMetricsCollector(provider)
	if err != nil {
		t.Fatalf("Failed to create OTEL metrics collector: %v", err)
	}

	cache := balios.NewGenericCache[string, int](balios.Config{
		MaxSize:          1000,
		MetricsCollector: metricsCollector,
	})
	defer func() { _ = cache.Close() }()

	// Perform operations and measure
	start := time.Now()

	for i := 0; i < 1000; i++ {
		cache.Set("key", i)
		cache.Get("key")
	}

	elapsed := time.Since(start)

	// Should complete 2000 operations (1000 Sets + 1000 Gets) in reasonable time
	if elapsed > 5*time.Second {
		t.Errorf("Operations took too long: %v", elapsed)
	}

	// Verify metrics were collected
	stats := cache.Stats()
	if stats.Sets != 1000 {
		t.Errorf("Expected 1000 sets, got %d", stats.Sets)
	}

	if stats.Hits != 1000 {
		t.Errorf("Expected 1000 hits, got %d", stats.Hits)
	}
}
