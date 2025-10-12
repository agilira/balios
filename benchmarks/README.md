# Balios Benchmarking Framework

Comprehensive benchmark suite comparing Balios against the best Go caches: **Otter** and **Ristretto**.

## 🎯 Benchmark Categories

### 1. **Single-Threaded Benchmarks**
Pure performance without contention:
- `BenchmarkX_Set_SingleThread` - Sequential write performance
- `BenchmarkX_Get_SingleThread` - Sequential read performance

### 2. **Parallel Benchmarks**
High contention scenarios:
- `BenchmarkX_Set_Parallel` - Concurrent writes
- `BenchmarkX_Get_Parallel` - Concurrent reads

### 3. **Mixed Workload Benchmarks**
Realistic scenarios with different read/write ratios:
- `BenchmarkX_WriteHeavy` - 10% reads, 90% writes
- `BenchmarkX_Balanced` - 50% reads, 50% writes
- `BenchmarkX_ReadHeavy` - 90% reads, 10% writes
- `BenchmarkX_ReadOnly` - 100% reads

### 4. **Cache Size Variants**
Different cache capacities:
- `BenchmarkX_Small_Mixed` - 1,000 items
- Default benchmarks - 10,000 items
- `BenchmarkX_Large_Mixed` - 100,000 items

### 5. **Hit Ratio Test**
Measures cache effectiveness (not a benchmark):
- `TestHitRatio` - Calculates hit percentage under Zipf distribution

## 🚀 Running Benchmarks

### Quick Test
```bash
go test -bench=. -benchtime=1s -benchmem
```

### Full Comparison (Recommended)
```bash
go test -bench=. -benchtime=5s -benchmem -cpu=1,2,4,8
```

### Specific Cache
```bash
# Only Balios
go test -bench=BenchmarkBalios -benchmem

# Only Otter
go test -bench=BenchmarkOtter -benchmem

# Only Ristretto
go test -bench=BenchmarkRistretto -benchmem
```

### Specific Scenario
```bash
# Read Heavy workload
go test -bench=ReadHeavy -benchmem

# Parallel operations
go test -bench=Parallel -benchmem
```

### Hit Ratio Test
```bash
go test -run=TestHitRatio -v
```

## 📊 Understanding Results

### Throughput
```
BenchmarkBalios_Set_SingleThread-8   5000000   250 ns/op   16 B/op   1 allocs/op
                                      ^^^^^^^   ^^^^^^^^^   ^^^^^^    ^^^^^^^^^^^
                                      ops/sec   ns per op   bytes     allocations
```

Higher ops/sec = better throughput
Lower ns/op = faster operations

### Memory
- **B/op**: Bytes allocated per operation (lower is better)
- **allocs/op**: Number of allocations (0-1 is ideal)

### CPU Cores
```
-cpu=1,2,4,8
```
Tests scalability across different core counts.

## 🎲 Zipf Distribution

All benchmarks use **Zipf distribution** (α=1.0) to simulate realistic access patterns:
- Some keys are accessed much more frequently (hot keys)
- Power law distribution mimics real-world caching scenarios
- Standard benchmark approach used by Caffeine, Otter, etc.

## 🏆 Performance Targets

### Balios Goals vs Competitors

| Metric | Otter | Ristretto | Balios Target |
|--------|-------|-----------|---------------|
| Throughput | ~100M ops/s | ~80M ops/s | **>100M ops/s** |
| Latency (p99) | <100ns | <150ns | **<100ns** |
| Memory/entry | ~24 bytes | ~32 bytes | **<24 bytes** |
| Hit Ratio | ~48% | ~46% | **>48%** |
| Allocations | 0-1/op | 1-2/op | **0/op** |

## 🔍 Analysis Tips

### Compare Throughput
```bash
go test -bench=. -benchmem | grep "Balanced"
```

### Memory Profiling
```bash
go test -bench=BenchmarkBalios_Balanced -memprofile=mem.out
go tool pprof mem.out
```

### CPU Profiling
```bash
go test -bench=BenchmarkBalios_Balanced -cpuprofile=cpu.out
go tool pprof cpu.out
```

### Benchstat Comparison
```bash
# Run benchmarks multiple times
go test -bench=. -count=10 > old.txt
# After optimizations
go test -bench=. -count=10 > new.txt
# Compare
benchstat old.txt new.txt
```

## 📈 Benchmark Strategy

1. **Establish Baseline**: Run all benchmarks to get current state
2. **Identify Bottlenecks**: Find where Balios lags behind
3. **Optimize**: Implement improvements
4. **Measure**: Re-run benchmarks
5. **Compare**: Use benchstat to validate improvements
6. **Iterate**: Repeat until goals achieved

## 🎯 What to Focus On

### High Priority
- **Parallel benchmarks**: Shows real-world concurrent performance
- **Balanced workload**: Most realistic scenario
- **Hit ratio**: Cache effectiveness indicator

### Medium Priority
- Single-threaded: Shows pure algorithm efficiency
- Read heavy: Common in production
- Large cache: Scalability test

### Low Priority
- Write heavy: Less common
- Small cache: Edge case
- Read only: Unrealistic

## 🔧 Customization

Edit constants in `benchmark_test.go`:
```go
const (
    smallCacheSize  = 1_000
    mediumCacheSize = 10_000
    largeCacheSize  = 100_000
    
    smallKeySpace  = 100
    mediumKeySpace = 1_000
    largeKeySpace  = 10_000
)
```

## 📝 Notes

- **Ristretto buffering**: May show artificially high Set() performance due to async processing
- **Warmup**: All benchmarks pre-populate cache to simulate steady-state
- **Fair comparison**: All caches configured with equivalent capacity
- **Realistic workload**: Zipf distribution matches real-world access patterns

## 🎬 Next Steps

After running benchmarks:
1. Document baseline performance in `study/ANALYSIS.md`
2. Identify performance gaps
3. Prioritize optimizations based on impact
4. Implement improvements
5. Re-benchmark to validate

## 🏁 Quick Start

```bash
# Run full benchmark suite (5 minutes)
go test -bench=. -benchtime=5s -benchmem -cpu=1,4,8 | tee results.txt

# View hit ratios
go test -run=TestHitRatio -v

# Compare with previous run
benchstat baseline.txt results.txt
```
