package benchmarks

import (
	"fmt"
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/agilira/balios"
	ristretto "github.com/dgraph-io/ristretto/v2"
	"github.com/maypok86/otter/v2"
)

// Benchmark configuration
const (
	// Cache sizes to test
	smallCacheSize  = 1_000
	mediumCacheSize = 10_000
	largeCacheSize  = 100_000

	// Key spaces for different scenarios
	smallKeySpace  = 100
	mediumKeySpace = 1_000
	largeKeySpace  = 10_000

	// Workload ratios (read percentage)
	writeHeavy = 0.1  // 10% reads, 90% writes
	balanced   = 0.5  // 50% reads, 50% writes
	readHeavy  = 0.9  // 90% reads, 10% writes
	readOnly   = 1.0  // 100% reads
)

// =============================================================================
// ZIPF DISTRIBUTION GENERATOR
// =============================================================================

// ZipfGenerator generates keys following Zipf distribution
// This simulates realistic access patterns where some items are much more
// popular than others (power law distribution)
type ZipfGenerator struct {
	zipf *rand.Zipf
	max  uint64
}

// NewZipfGenerator creates a new Zipf distribution generator
// s: exponent (must be > 1.0 for Zipf to work)
// v: second parameter for Zipf (must be >= 1.0)
// imax: maximum value (key space)
func NewZipfGenerator(s, v float64, imax uint64) *ZipfGenerator {
	// Ensure imax is at least 1
	if imax < 1 {
		imax = 1
	}
	// Ensure s > 1 and v >= 1 for valid Zipf
	if s <= 1.0 {
		s = 1.01
	}
	if v < 1.0 {
		v = 1.0
	}
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	zipf := rand.NewZipf(r, s, v, imax)
	if zipf == nil {
		panic(fmt.Sprintf("failed to create Zipf generator: s=%f, v=%f, imax=%d", s, v, imax))
	}
	return &ZipfGenerator{
		zipf: zipf,
		max:  imax,
	}
}

// Next returns the next key in the Zipf distribution
func (z *ZipfGenerator) Next() uint64 {
	return z.zipf.Uint64()
}

// NextString returns the next key as a string
func (z *ZipfGenerator) NextString() string {
	return strconv.FormatUint(z.Next(), 10)
}

// =============================================================================
// CACHE WRAPPERS FOR UNIFORM INTERFACE
// =============================================================================

// CacheInterface provides a uniform interface for all caches
type CacheInterface interface {
	Set(key string, value int) bool
	Get(key string) (int, bool)
	Name() string
	Close()
}

// =============================================================================
// BALIOS WRAPPER (Non-Generic Legacy API)
// =============================================================================

type BaliosCache struct {
	cache balios.Cache
}

func NewBaliosCache(size int) *BaliosCache {
	return &BaliosCache{
		cache: balios.NewCache(balios.Config{
			MaxSize: size,
		}),
	}
}

func (c *BaliosCache) Set(key string, value int) bool {
	return c.cache.Set(key, value)
}

func (c *BaliosCache) Get(key string) (int, bool) {
	v, ok := c.cache.Get(key)
	if !ok {
		return 0, false
	}
	return v.(int), true
}

func (c *BaliosCache) Name() string {
	return "Balios"
}

func (c *BaliosCache) Close() {
	c.cache.Close()
}

// =============================================================================
// BALIOS GENERIC WRAPPER (Optimized Generic API)
// =============================================================================

type BaliosGenericCache struct {
	cache *balios.GenericCache[string, int]
}

func NewBaliosGenericCache(size int) *BaliosGenericCache {
	return &BaliosGenericCache{
		cache: balios.NewGenericCache[string, int](balios.Config{
			MaxSize: size,
		}),
	}
}

func (c *BaliosGenericCache) Set(key string, value int) bool {
	c.cache.Set(key, value)
	return true
}

func (c *BaliosGenericCache) Get(key string) (int, bool) {
	return c.cache.Get(key)
}

func (c *BaliosGenericCache) Name() string {
	return "Balios-Generic"
}

func (c *BaliosGenericCache) Close() {
	c.cache.Close()
}

// =============================================================================
// OTTER WRAPPER
// =============================================================================

type OtterCache struct {
	cache *otter.Cache[string, int]
}

func NewOtterCache(size int) *OtterCache {
	cache := otter.Must(&otter.Options[string, int]{
		MaximumSize: size,
	})
	return &OtterCache{cache: cache}
}

func (c *OtterCache) Set(key string, value int) bool {
	c.cache.Set(key, value)
	return true
}

func (c *OtterCache) Get(key string) (int, bool) {
	return c.cache.GetIfPresent(key)
}

func (c *OtterCache) Name() string {
	return "Otter"
}

func (c *OtterCache) Close() {
	// Otter v2 Close is handled automatically
}

// =============================================================================
// RISTRETTO WRAPPER
// =============================================================================

type RistrettoCache struct {
	cache *ristretto.Cache[string, int]
}

func NewRistrettoCache(size int) *RistrettoCache {
	cache, err := ristretto.NewCache(&ristretto.Config[string, int]{
		NumCounters: int64(size * 10),
		MaxCost:     int64(size),
		BufferItems: 64,
	})
	if err != nil {
		panic(err)
	}
	return &RistrettoCache{cache: cache}
}

func (c *RistrettoCache) Set(key string, value int) bool {
	return c.cache.Set(key, value, 1)
}

func (c *RistrettoCache) Get(key string) (int, bool) {
	return c.cache.Get(key)
}

func (c *RistrettoCache) Name() string {
	return "Ristretto"
}

func (c *RistrettoCache) Close() {
	c.cache.Close()
}

// =============================================================================
// BENCHMARK HELPERS
// =============================================================================

// warmupCache pre-populates cache with data following Zipf distribution
func warmupCache(c CacheInterface, keySpace int) {
	zipf := NewZipfGenerator(1.0, 1.0, uint64(keySpace-1))
	for i := 0; i < keySpace/2; i++ {
		key := zipf.NextString()
		c.Set(key, i)
	}
}

// runMixedWorkload executes a mixed read/write workload
func runMixedWorkload(b *testing.B, c CacheInterface, keySpace int, readRatio float64, parallel bool) {
	// Warmup
	warmupCache(c, keySpace)

	b.ResetTimer()
	b.ReportAllocs()

	if parallel {
		b.RunParallel(func(pb *testing.PB) {
			zipf := NewZipfGenerator(1.0, 1.0, uint64(keySpace-1))
			i := 0
			for pb.Next() {
				key := zipf.NextString()
				
				// Determine if this is a read or write
				if rand.Float64() < readRatio {
					c.Get(key)
				} else {
					c.Set(key, i)
					i++
				}
			}
		})
	} else {
		zipf := NewZipfGenerator(1.0, 1.0, uint64(keySpace-1))
		for i := 0; i < b.N; i++ {
			key := zipf.NextString()
			
			if rand.Float64() < readRatio {
				c.Get(key)
			} else {
				c.Set(key, i)
			}
		}
	}
}

// =============================================================================
// SINGLE-THREADED BENCHMARKS - Pure Performance
// =============================================================================

func BenchmarkBalios_Set_SingleThread(b *testing.B) {
	benchmarkSet(b, NewBaliosCache(mediumCacheSize), mediumKeySpace, false)
}

func BenchmarkBaliosGeneric_Set_SingleThread(b *testing.B) {
	benchmarkSet(b, NewBaliosGenericCache(mediumCacheSize), mediumKeySpace, false)
}

func BenchmarkOtter_Set_SingleThread(b *testing.B) {
	benchmarkSet(b, NewOtterCache(mediumCacheSize), mediumKeySpace, false)
}

func BenchmarkRistretto_Set_SingleThread(b *testing.B) {
	benchmarkSet(b, NewRistrettoCache(mediumCacheSize), mediumKeySpace, false)
}

func benchmarkSet(b *testing.B, c CacheInterface, keySpace int, parallel bool) {
	defer c.Close()
	
	b.ResetTimer()
	b.ReportAllocs()

	if parallel {
		b.RunParallel(func(pb *testing.PB) {
			zipf := NewZipfGenerator(1.0, 1.0, uint64(keySpace-1))
			i := 0
			for pb.Next() {
				key := zipf.NextString()
				c.Set(key, i)
				i++
			}
		})
	} else {
		zipf := NewZipfGenerator(1.0, 1.0, uint64(keySpace-1))
		for i := 0; i < b.N; i++ {
			key := zipf.NextString()
			c.Set(key, i)
		}
	}
}

// =============================================================================
// GET BENCHMARKS
// =============================================================================

func BenchmarkBalios_Get_SingleThread(b *testing.B) {
	benchmarkGet(b, NewBaliosCache(mediumCacheSize), mediumKeySpace, false)
}

func BenchmarkBaliosGeneric_Get_SingleThread(b *testing.B) {
	benchmarkGet(b, NewBaliosGenericCache(mediumCacheSize), mediumKeySpace, false)
}

func BenchmarkOtter_Get_SingleThread(b *testing.B) {
	benchmarkGet(b, NewOtterCache(mediumCacheSize), mediumKeySpace, false)
}

func BenchmarkRistretto_Get_SingleThread(b *testing.B) {
	benchmarkGet(b, NewRistrettoCache(mediumCacheSize), mediumKeySpace, false)
}

func benchmarkGet(b *testing.B, c CacheInterface, keySpace int, parallel bool) {
	defer c.Close()
	
	// Warmup
	warmupCache(c, keySpace)
	
	b.ResetTimer()
	b.ReportAllocs()

	if parallel {
		b.RunParallel(func(pb *testing.PB) {
			zipf := NewZipfGenerator(1.0, 1.0, uint64(keySpace-1))
			for pb.Next() {
				key := zipf.NextString()
				c.Get(key)
			}
		})
	} else {
		zipf := NewZipfGenerator(1.0, 1.0, uint64(keySpace-1))
		for i := 0; i < b.N; i++ {
			key := zipf.NextString()
			c.Get(key)
		}
	}
}

// =============================================================================
// PARALLEL BENCHMARKS - High Contention
// =============================================================================

func BenchmarkBalios_Set_Parallel(b *testing.B) {
	benchmarkSet(b, NewBaliosCache(mediumCacheSize), mediumKeySpace, true)
}

func BenchmarkBaliosGeneric_Set_Parallel(b *testing.B) {
	benchmarkSet(b, NewBaliosGenericCache(mediumCacheSize), mediumKeySpace, true)
}

func BenchmarkOtter_Set_Parallel(b *testing.B) {
	benchmarkSet(b, NewOtterCache(mediumCacheSize), mediumKeySpace, true)
}

func BenchmarkRistretto_Set_Parallel(b *testing.B) {
	benchmarkSet(b, NewRistrettoCache(mediumCacheSize), mediumKeySpace, true)
}

func BenchmarkBalios_Get_Parallel(b *testing.B) {
	benchmarkGet(b, NewBaliosCache(mediumCacheSize), mediumKeySpace, true)
}

func BenchmarkBaliosGeneric_Get_Parallel(b *testing.B) {
	benchmarkGet(b, NewBaliosGenericCache(mediumCacheSize), mediumKeySpace, true)
}

func BenchmarkOtter_Get_Parallel(b *testing.B) {
	benchmarkGet(b, NewOtterCache(mediumCacheSize), mediumKeySpace, true)
}

func BenchmarkRistretto_Get_Parallel(b *testing.B) {
	benchmarkGet(b, NewRistrettoCache(mediumCacheSize), mediumKeySpace, true)
}

// =============================================================================
// MIXED WORKLOAD BENCHMARKS - Realistic Scenarios
// =============================================================================

// Write Heavy (10% reads, 90% writes)
func BenchmarkBalios_WriteHeavy(b *testing.B) {
	c := NewBaliosCache(mediumCacheSize)
	defer c.Close()
	runMixedWorkload(b, c, mediumKeySpace, writeHeavy, true)
}

func BenchmarkBaliosGeneric_WriteHeavy(b *testing.B) {
	c := NewBaliosGenericCache(mediumCacheSize)
	defer c.Close()
	runMixedWorkload(b, c, mediumKeySpace, writeHeavy, true)
}

func BenchmarkOtter_WriteHeavy(b *testing.B) {
	c := NewOtterCache(mediumCacheSize)
	defer c.Close()
	runMixedWorkload(b, c, mediumKeySpace, writeHeavy, true)
}

func BenchmarkRistretto_WriteHeavy(b *testing.B) {
	c := NewRistrettoCache(mediumCacheSize)
	defer c.Close()
	runMixedWorkload(b, c, mediumKeySpace, writeHeavy, true)
}

// Balanced (50% reads, 50% writes)
func BenchmarkBalios_Balanced(b *testing.B) {
	c := NewBaliosCache(mediumCacheSize)
	defer c.Close()
	runMixedWorkload(b, c, mediumKeySpace, balanced, true)
}

func BenchmarkBaliosGeneric_Balanced(b *testing.B) {
	c := NewBaliosGenericCache(mediumCacheSize)
	defer c.Close()
	runMixedWorkload(b, c, mediumKeySpace, balanced, true)
}

func BenchmarkOtter_Balanced(b *testing.B) {
	c := NewOtterCache(mediumCacheSize)
	defer c.Close()
	runMixedWorkload(b, c, mediumKeySpace, balanced, true)
}

func BenchmarkRistretto_Balanced(b *testing.B) {
	c := NewRistrettoCache(mediumCacheSize)
	defer c.Close()
	runMixedWorkload(b, c, mediumKeySpace, balanced, true)
}

// Read Heavy (90% reads, 10% writes)
func BenchmarkBalios_ReadHeavy(b *testing.B) {
	c := NewBaliosCache(mediumCacheSize)
	defer c.Close()
	runMixedWorkload(b, c, mediumKeySpace, readHeavy, true)
}

func BenchmarkBaliosGeneric_ReadHeavy(b *testing.B) {
	c := NewBaliosGenericCache(mediumCacheSize)
	defer c.Close()
	runMixedWorkload(b, c, mediumKeySpace, readHeavy, true)
}

func BenchmarkOtter_ReadHeavy(b *testing.B) {
	c := NewOtterCache(mediumCacheSize)
	defer c.Close()
	runMixedWorkload(b, c, mediumKeySpace, readHeavy, true)
}

func BenchmarkRistretto_ReadHeavy(b *testing.B) {
	c := NewRistrettoCache(mediumCacheSize)
	defer c.Close()
	runMixedWorkload(b, c, mediumKeySpace, readHeavy, true)
}

// Read Only (100% reads)
func BenchmarkBalios_ReadOnly(b *testing.B) {
	c := NewBaliosCache(mediumCacheSize)
	defer c.Close()
	runMixedWorkload(b, c, mediumKeySpace, readOnly, true)
}

func BenchmarkBaliosGeneric_ReadOnly(b *testing.B) {
	c := NewBaliosGenericCache(mediumCacheSize)
	defer c.Close()
	runMixedWorkload(b, c, mediumKeySpace, readOnly, true)
}

func BenchmarkOtter_ReadOnly(b *testing.B) {
	c := NewOtterCache(mediumCacheSize)
	defer c.Close()
	runMixedWorkload(b, c, mediumKeySpace, readOnly, true)
}

func BenchmarkRistretto_ReadOnly(b *testing.B) {
	c := NewRistrettoCache(mediumCacheSize)
	defer c.Close()
	runMixedWorkload(b, c, mediumKeySpace, readOnly, true)
}

// =============================================================================
// CACHE SIZE VARIANTS
// =============================================================================

func BenchmarkBalios_Small_Mixed(b *testing.B) {
	c := NewBaliosCache(smallCacheSize)
	defer c.Close()
	runMixedWorkload(b, c, smallKeySpace, balanced, true)
}

func BenchmarkOtter_Small_Mixed(b *testing.B) {
	c := NewOtterCache(smallCacheSize)
	defer c.Close()
	runMixedWorkload(b, c, smallKeySpace, balanced, true)
}

func BenchmarkRistretto_Small_Mixed(b *testing.B) {
	c := NewRistrettoCache(smallCacheSize)
	defer c.Close()
	runMixedWorkload(b, c, smallKeySpace, balanced, true)
}

func BenchmarkBalios_Large_Mixed(b *testing.B) {
	c := NewBaliosCache(largeCacheSize)
	defer c.Close()
	runMixedWorkload(b, c, largeKeySpace, balanced, true)
}

func BenchmarkOtter_Large_Mixed(b *testing.B) {
	c := NewOtterCache(largeCacheSize)
	defer c.Close()
	runMixedWorkload(b, c, largeKeySpace, balanced, true)
}

func BenchmarkRistretto_Large_Mixed(b *testing.B) {
	c := NewRistrettoCache(largeCacheSize)
	defer c.Close()
	runMixedWorkload(b, c, largeKeySpace, balanced, true)
}

// =============================================================================
// HIT RATIO TEST (Not a benchmark, but useful for comparison)
// =============================================================================

func TestHitRatio(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping hit ratio test in short mode")
	}

	caches := []CacheInterface{
		NewBaliosCache(mediumCacheSize),
		NewOtterCache(mediumCacheSize),
		NewRistrettoCache(mediumCacheSize),
	}

	for _, c := range caches {
		testHitRatio(t, c, mediumKeySpace)
		c.Close()
	}
}

func testHitRatio(t *testing.T, c CacheInterface, keySpace int) {
	zipf := NewZipfGenerator(1.0, 1.0, uint64(keySpace-1))
	
	// Warmup phase
	for i := 0; i < keySpace; i++ {
		key := zipf.NextString()
		c.Set(key, i)
	}

	// Test phase
	hits := 0
	misses := 0
	requests := 100_000

	for i := 0; i < requests; i++ {
		key := zipf.NextString()
		if _, ok := c.Get(key); ok {
			hits++
		} else {
			misses++
		}
	}

	hitRatio := float64(hits) / float64(requests) * 100
	t.Logf("%s Hit Ratio: %.2f%% (hits: %d, misses: %d)", 
		c.Name(), hitRatio, hits, misses)
}
