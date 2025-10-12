package benchmarks

import (
	"testing"
)

// TestHitRatioExtended performs multiple runs to get stable averages
func TestHitRatioExtended(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping extended hit ratio test in short mode")
	}

	const runs = 10
	const requestsPerRun = 100_000

	caches := []struct {
		name    string
		factory func(int) CacheInterface
	}{
		{"Balios", func(size int) CacheInterface { return NewBaliosCache(size) }},
		{"Balios-Generic", func(size int) CacheInterface { return NewBaliosGenericCache(size) }},
		{"Otter", func(size int) CacheInterface { return NewOtterCache(size) }},
		{"Ristretto", func(size int) CacheInterface { return NewRistrettoCache(size) }},
	}

	for _, cache := range caches {
		totalHits := 0
		totalRequests := 0

		for run := 0; run < runs; run++ {
			c := cache.factory(mediumCacheSize)

			// Warmup with Zipf distribution
			zipf := NewZipfGenerator(1.0, 1.0, uint64(mediumKeySpace-1))
			for i := 0; i < mediumKeySpace; i++ {
				key := zipf.NextString()
				c.Set(key, i)
			}

			// Test phase - create new zipf for consistency
			zipf = NewZipfGenerator(1.0, 1.0, uint64(mediumKeySpace-1))
			hits := 0
			for i := 0; i < requestsPerRun; i++ {
				key := zipf.NextString()
				if _, ok := c.Get(key); ok {
					hits++
				}
			}

			totalHits += hits
			totalRequests += requestsPerRun
			c.Close()
		}

		avgHitRatio := float64(totalHits) / float64(totalRequests) * 100
		t.Logf("%s Average Hit Ratio (10 runs): %.2f%% (total hits: %d/%d)",
			cache.name, avgHitRatio, totalHits, totalRequests)
	}
}

// TestHitRatioDifferentWorkloads tests hit ratio under different access patterns
func TestHitRatioDifferentWorkloads(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping workload hit ratio test in short mode")
	}

	workloads := []struct {
		name     string
		s        float64 // Zipf exponent (higher = more skewed)
		keySpace int
	}{
		{"Highly Skewed (s=1.5)", 1.5, mediumKeySpace},
		{"Moderate (s=1.0)", 1.0, mediumKeySpace},
		{"Less Skewed (s=0.8)", 1.01, mediumKeySpace}, // 1.01 minimum for Zipf
		{"Large KeySpace", 1.0, largeKeySpace},
	}

	caches := []struct {
		name    string
		factory func(int) CacheInterface
	}{
		{"Balios", func(size int) CacheInterface { return NewBaliosCache(size) }},
		{"Otter", func(size int) CacheInterface { return NewOtterCache(size) }},
		{"Ristretto", func(size int) CacheInterface { return NewRistrettoCache(size) }},
	}

	for _, wl := range workloads {
		t.Logf("\n=== Workload: %s ===", wl.name)

		for _, cache := range caches {
			c := cache.factory(mediumCacheSize)

			// Warmup
			zipf := NewZipfGenerator(wl.s, 1.0, uint64(wl.keySpace-1))
			for i := 0; i < wl.keySpace/2; i++ {
				key := zipf.NextString()
				c.Set(key, i)
			}

			// Test
			zipf = NewZipfGenerator(wl.s, 1.0, uint64(wl.keySpace-1))
			hits := 0
			requests := 100_000
			for i := 0; i < requests; i++ {
				key := zipf.NextString()
				if _, ok := c.Get(key); ok {
					hits++
				}
			}

			hitRatio := float64(hits) / float64(requests) * 100
			t.Logf("  %s: %.2f%% (hits: %d/%d)", cache.name, hitRatio, hits, requests)
			c.Close()
		}
	}
}
