package otel

import (
	"context"
	"testing"
	"time"

	"github.com/agilira/balios"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// TestOTelMetricsCollector_Interface verifies OTelMetricsCollector implements balios.MetricsCollector
func TestOTelMetricsCollector_Interface(t *testing.T) {
	var _ balios.MetricsCollector = (*OTelMetricsCollector)(nil)
}

// TestNewOTelMetricsCollector tests constructor with valid meter provider
func TestNewOTelMetricsCollector(t *testing.T) {
	reader := metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(reader))
	defer func() {
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Errorf("Failed to shutdown provider: %v", err)
		}
	}()

	collector, err := NewOTelMetricsCollector(provider)
	if err != nil {
		t.Fatalf("NewOTelMetricsCollector() error = %v", err)
	}
	if collector == nil {
		t.Fatal("NewOTelMetricsCollector() returned nil")
	}
}

// TestNewOTelMetricsCollector_NilProvider tests error handling with nil provider
func TestNewOTelMetricsCollector_NilProvider(t *testing.T) {
	collector, err := NewOTelMetricsCollector(nil)
	if err == nil {
		t.Fatal("NewOTelMetricsCollector(nil) should return error")
	}
	if collector != nil {
		t.Fatal("NewOTelMetricsCollector(nil) should return nil collector")
	}
}

// TestOTelMetricsCollector_RecordGet tests Get operation metrics
func TestOTelMetricsCollector_RecordGet(t *testing.T) {
	reader := metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(reader))
	defer provider.Shutdown(context.Background())

	collector, err := NewOTelMetricsCollector(provider)
	if err != nil {
		t.Fatalf("NewOTelMetricsCollector() error = %v", err)
	}

	// Record operations
	collector.RecordGet(1000, true)  // 1μs hit
	collector.RecordGet(2000, false) // 2μs miss
	collector.RecordGet(1500, true)  // 1.5μs hit

	// Collect metrics
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("Failed to collect metrics: %v", err)
	}

	// Verify metrics were recorded
	if len(rm.ScopeMetrics) == 0 {
		t.Fatal("No scope metrics recorded")
	}

	// Find and verify get_latency histogram
	var foundLatency bool
	var foundHits bool
	var foundMisses bool

	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			switch m.Name {
			case "balios_get_latency_ns":
				foundLatency = true
				hist, ok := m.Data.(metricdata.Histogram[int64])
				if !ok {
					t.Errorf("Expected Histogram[int64], got %T", m.Data)
					continue
				}
				if len(hist.DataPoints) == 0 {
					t.Error("No histogram data points")
					continue
				}
				// Verify we have 3 data points
				totalCount := uint64(0)
				for _, dp := range hist.DataPoints {
					totalCount += dp.Count
				}
				if totalCount != 3 {
					t.Errorf("Expected 3 operations, got %d", totalCount)
				}

			case "balios_get_hits_total":
				foundHits = true
				sum, ok := m.Data.(metricdata.Sum[int64])
				if !ok {
					t.Errorf("Expected Sum[int64], got %T", m.Data)
					continue
				}
				if len(sum.DataPoints) == 0 {
					t.Error("No sum data points")
					continue
				}
				// Should have 2 hits
				if sum.DataPoints[0].Value != 2 {
					t.Errorf("Expected 2 hits, got %d", sum.DataPoints[0].Value)
				}

			case "balios_get_misses_total":
				foundMisses = true
				sum, ok := m.Data.(metricdata.Sum[int64])
				if !ok {
					t.Errorf("Expected Sum[int64], got %T", m.Data)
					continue
				}
				if len(sum.DataPoints) == 0 {
					t.Error("No sum data points")
					continue
				}
				// Should have 1 miss
				if sum.DataPoints[0].Value != 1 {
					t.Errorf("Expected 1 miss, got %d", sum.DataPoints[0].Value)
				}
			}
		}
	}

	if !foundLatency {
		t.Error("balios_get_latency_ns metric not found")
	}
	if !foundHits {
		t.Error("balios_get_hits_total metric not found")
	}
	if !foundMisses {
		t.Error("balios_get_misses_total metric not found")
	}
}

// TestOTelMetricsCollector_RecordSet tests Set operation metrics
func TestOTelMetricsCollector_RecordSet(t *testing.T) {
	reader := metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(reader))
	defer provider.Shutdown(context.Background())

	collector, err := NewOTelMetricsCollector(provider)
	if err != nil {
		t.Fatalf("NewOTelMetricsCollector() error = %v", err)
	}

	// Record operations
	collector.RecordSet(500)
	collector.RecordSet(1000)
	collector.RecordSet(750)

	// Collect metrics
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("Failed to collect metrics: %v", err)
	}

	// Find set_latency histogram
	var foundLatency bool
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "balios_set_latency_ns" {
				foundLatency = true
				hist, ok := m.Data.(metricdata.Histogram[int64])
				if !ok {
					t.Errorf("Expected Histogram[int64], got %T", m.Data)
					continue
				}
				if len(hist.DataPoints) == 0 {
					t.Error("No histogram data points")
					continue
				}
				totalCount := uint64(0)
				for _, dp := range hist.DataPoints {
					totalCount += dp.Count
				}
				if totalCount != 3 {
					t.Errorf("Expected 3 operations, got %d", totalCount)
				}
			}
		}
	}

	if !foundLatency {
		t.Error("balios_set_latency_ns metric not found")
	}
}

// TestOTelMetricsCollector_RecordDelete tests Delete operation metrics
func TestOTelMetricsCollector_RecordDelete(t *testing.T) {
	reader := metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(reader))
	defer provider.Shutdown(context.Background())

	collector, err := NewOTelMetricsCollector(provider)
	if err != nil {
		t.Fatalf("NewOTelMetricsCollector() error = %v", err)
	}

	// Record operations
	collector.RecordDelete(300)
	collector.RecordDelete(600)

	// Collect metrics
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("Failed to collect metrics: %v", err)
	}

	// Find delete_latency histogram
	var foundLatency bool
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "balios_delete_latency_ns" {
				foundLatency = true
				hist, ok := m.Data.(metricdata.Histogram[int64])
				if !ok {
					t.Errorf("Expected Histogram[int64], got %T", m.Data)
					continue
				}
				if len(hist.DataPoints) == 0 {
					t.Error("No histogram data points")
					continue
				}
				totalCount := uint64(0)
				for _, dp := range hist.DataPoints {
					totalCount += dp.Count
				}
				if totalCount != 2 {
					t.Errorf("Expected 2 operations, got %d", totalCount)
				}
			}
		}
	}

	if !foundLatency {
		t.Error("balios_delete_latency_ns metric not found")
	}
}

// TestOTelMetricsCollector_RecordEviction tests eviction counter
func TestOTelMetricsCollector_RecordEviction(t *testing.T) {
	reader := metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(reader))
	defer provider.Shutdown(context.Background())

	collector, err := NewOTelMetricsCollector(provider)
	if err != nil {
		t.Fatalf("NewOTelMetricsCollector() error = %v", err)
	}

	// Record evictions
	collector.RecordEviction()
	collector.RecordEviction()
	collector.RecordEviction()

	// Collect metrics
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("Failed to collect metrics: %v", err)
	}

	// Find evictions counter
	var foundEvictions bool
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "balios_evictions_total" {
				foundEvictions = true
				sum, ok := m.Data.(metricdata.Sum[int64])
				if !ok {
					t.Errorf("Expected Sum[int64], got %T", m.Data)
					continue
				}
				if len(sum.DataPoints) == 0 {
					t.Error("No sum data points")
					continue
				}
				if sum.DataPoints[0].Value != 3 {
					t.Errorf("Expected 3 evictions, got %d", sum.DataPoints[0].Value)
				}
			}
		}
	}

	if !foundEvictions {
		t.Error("balios_evictions_total metric not found")
	}
}

// TestOTelMetricsCollector_Concurrent tests thread safety
func TestOTelMetricsCollector_Concurrent(t *testing.T) {
	reader := metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(reader))
	defer provider.Shutdown(context.Background())

	collector, err := NewOTelMetricsCollector(provider)
	if err != nil {
		t.Fatalf("NewOTelMetricsCollector() error = %v", err)
	}

	const numGoroutines = 10
	const opsPerGoroutine = 100
	done := make(chan bool, numGoroutines)

	// Launch concurrent operations
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			for j := 0; j < opsPerGoroutine; j++ {
				collector.RecordGet(int64(100+id), j%2 == 0)
				collector.RecordSet(int64(200 + id))
				collector.RecordDelete(int64(50 + id))
				collector.RecordEviction()
			}
			done <- true
		}(i)
	}

	// Wait for completion
	for i := 0; i < numGoroutines; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("Test timeout - deadlock?")
		}
	}

	// Collect metrics
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("Failed to collect metrics: %v", err)
	}

	// Verify we got metrics (exact counts may vary due to OTEL aggregation)
	if len(rm.ScopeMetrics) == 0 {
		t.Fatal("No metrics collected after concurrent operations")
	}
}

// TestOTelMetricsCollector_WithOptions tests constructor with options
func TestOTelMetricsCollector_WithOptions(t *testing.T) {
	reader := metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(reader))
	defer provider.Shutdown(context.Background())

	collector, err := NewOTelMetricsCollector(
		provider,
		WithMeterName("custom_balios"),
	)
	if err != nil {
		t.Fatalf("NewOTelMetricsCollector() error = %v", err)
	}
	if collector == nil {
		t.Fatal("NewOTelMetricsCollector() returned nil")
	}

	// Record operation
	collector.RecordGet(1000, true)

	// Collect and verify meter name
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("Failed to collect metrics: %v", err)
	}

	if len(rm.ScopeMetrics) == 0 {
		t.Fatal("No scope metrics")
	}

	// Verify scope name
	if rm.ScopeMetrics[0].Scope.Name != "custom_balios" {
		t.Errorf("Expected scope name 'custom_balios', got '%s'", rm.ScopeMetrics[0].Scope.Name)
	}
}
