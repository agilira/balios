// Example: OpenTelemetry + Prometheus Integration with balios cache
//
// This example demonstrates enterprise-grade observability for balios cache:
//   - OpenTelemetry metrics collection
//   - Prometheus exporter for metrics scraping
//   - Automatic percentile calculation (p50, p95, p99)
//   - Grafana-ready metrics endpoint
//
// Run this example:
//
//	go run main.go
//
// Access metrics:
//
//	curl http://localhost:2112/metrics | grep balios
//
// Setup with Prometheus + Grafana:
//
//	docker-compose up -d
//	Open http://localhost:3000 (Grafana)
//	Login: admin/admin
//	Import dashboard from grafana-dashboard.json

package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/agilira/balios"
	baliosostel "github.com/agilira/balios/otel"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/metric"
)

func main() {
	fmt.Println("🚀 balios + OpenTelemetry + Prometheus Example")
	fmt.Println("================================================")

	// Setup Prometheus exporter
	fmt.Println("⚙️  Setting up Prometheus exporter...")
	exporter, err := prometheus.New()
	if err != nil {
		log.Fatalf("Failed to create Prometheus exporter: %v", err)
	}

	// Create OTEL MeterProvider
	provider := metric.NewMeterProvider(
		metric.WithReader(exporter),
		// Configure histogram buckets for latency percentiles
		// Buckets in nanoseconds: 100ns, 500ns, 1μs, 5μs, 10μs, 50μs, 100μs
		metric.WithView(metric.NewView(
			metric.Instrument{Name: "balios_get_latency_ns"},
			metric.Stream{
				Aggregation: metric.AggregationExplicitBucketHistogram{
					Boundaries: []float64{100, 500, 1000, 5000, 10000, 50000, 100000},
				},
			},
		)),
		metric.WithView(metric.NewView(
			metric.Instrument{Name: "balios_set_latency_ns"},
			metric.Stream{
				Aggregation: metric.AggregationExplicitBucketHistogram{
					Boundaries: []float64{100, 500, 1000, 5000, 10000, 50000, 100000},
				},
			},
		)),
		metric.WithView(metric.NewView(
			metric.Instrument{Name: "balios_delete_latency_ns"},
			metric.Stream{
				Aggregation: metric.AggregationExplicitBucketHistogram{
					Boundaries: []float64{100, 500, 1000, 5000, 10000, 50000, 100000},
				},
			},
		)),
	)

	// Graceful shutdown
	defer func() {
		fmt.Println("\n🛑 Shutting down OTEL provider...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := provider.Shutdown(ctx); err != nil {
			log.Printf("⚠️  Failed to shutdown provider: %v", err)
		}
	}()

	// Create metrics collector
	fmt.Println("📊 Creating OTel metrics collector...")
	metricsCollector, err := baliosostel.NewOTelMetricsCollector(provider)
	if err != nil {
		log.Fatalf("Failed to create metrics collector: %v", err)
	}

	// Create cache with metrics
	fmt.Println("🗄️  Creating balios cache with metrics...")
	cache := balios.NewCache(balios.Config{
		MaxSize:          1000,
		MetricsCollector: metricsCollector,
	})

	// Start metrics server
	fmt.Println("🌐 Starting metrics server on :2112...")
	http.Handle("/metrics", promhttp.Handler())

	server := &http.Server{Addr: ":2112"}
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start metrics server: %v", err)
		}
	}()

	fmt.Println("✅ Metrics available at http://localhost:2112/metrics")
	fmt.Println("")
	fmt.Println("📈 Example PromQL queries:")
	fmt.Println("   P95 Get Latency: histogram_quantile(0.95, rate(balios_get_latency_ns_bucket[5m]))")
	fmt.Println("   Hit Ratio:       rate(balios_get_hits_total[5m]) / (rate(balios_get_hits_total[5m]) + rate(balios_get_misses_total[5m]))")
	fmt.Println("   Operations/sec:  rate(balios_get_hits_total[1m]) + rate(balios_get_misses_total[1m])")
	fmt.Println("")

	// Simulate cache workload
	fmt.Println("🔄 Starting simulated workload...")
	fmt.Println("   Press Ctrl+C to stop")
	fmt.Println("")

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\n🛑 Received shutdown signal")
		cancel()

		// Shutdown HTTP server
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("⚠️  HTTP server shutdown error: %v", err)
		}
	}()

	// Run workload
	runWorkload(ctx, cache)

	fmt.Println("👋 Goodbye!")
}

// runWorkload simulates realistic cache usage patterns
func runWorkload(ctx context.Context, cache balios.Cache) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	// Pre-populate cache with some data
	fmt.Println("📝 Pre-populating cache with 500 entries...")
	for i := 0; i < 500; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := fmt.Sprintf("value-%d", i)
		cache.Set(key, value)
	}

	operations := 0
	startTime := time.Now()
	lastReport := startTime

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			// Simulate mixed workload
			for i := 0; i < 10; i++ {
				operations++

				// 70% gets, 20% sets, 10% deletes
				op := rand.Float64()

				switch {
				case op < 0.70: // Get operations
					// 80% hits (existing keys), 20% misses
					var key string
					if rand.Float64() < 0.80 {
						key = fmt.Sprintf("key-%d", rand.Intn(500))
					} else {
						key = fmt.Sprintf("missing-key-%d", rand.Intn(1000))
					}
					cache.Get(key)

				case op < 0.90: // Set operations
					key := fmt.Sprintf("key-%d", rand.Intn(1000))
					value := fmt.Sprintf("value-%d-%d", rand.Intn(1000), time.Now().Unix())
					cache.Set(key, value)

				default: // Delete operations
					key := fmt.Sprintf("key-%d", rand.Intn(500))
					cache.Delete(key)
				}
			}

			// Report stats every 5 seconds
			now := time.Now()
			if now.Sub(lastReport) >= 5*time.Second {
				elapsed := now.Sub(startTime)
				opsPerSec := float64(operations) / elapsed.Seconds()

				stats := cache.Stats()
				hitRatio := float64(0)
				totalGets := stats.Hits + stats.Misses
				if totalGets > 0 {
					hitRatio = float64(stats.Hits) / float64(totalGets) * 100
				}

				fmt.Printf("📊 Stats: %d ops (%.1f ops/sec) | Hit Ratio: %.1f%% | Size: %d | Evictions: %d\n",
					operations, opsPerSec, hitRatio, stats.Size, stats.Evictions)

				lastReport = now
			}
		}
	}
}
