# Fuzzing Guide for Balios

## Overview

Balios includes comprehensive fuzz tests to ensure security, robustness, and performance under adversarial conditions. Fuzzing is a critical part of our security hardening process.

## What is Fuzzing?

Fuzzing is an automated testing technique that provides random, malformed, or unexpected input to find bugs, crashes, and security vulnerabilities. Go's native fuzzing support (Go 1.18+) makes it easy to integrate into our testing pipeline.

## Why Fuzzing for Balios?

- **Security**: Resist attacks like hash collision DoS, memory exhaustion
- **Reliability**: Handle any input without crashes or panics
- **Performance**: Maintain speed even with adversarial inputs
- **Correctness**: Ensure data consistency under all conditions

## Fuzz Test Coverage

### 1. `FuzzStringHash` - Hash Function Security
**Target**: Core `stringHash()` function  
**Attack Vectors**:
- Hash collision DoS (crafted keys that produce same hash)
- Poor distribution (predictable patterns)
- Crashes with malformed UTF-8

**Properties Tested**:
- Determinism (same input = same hash)
- Avalanche effect (small changes = large hash differences)
- No panics with any input
- Good bit distribution

**Why Critical**: Hash function quality directly impacts cache performance and security.

### 2. `FuzzCacheSetGet` - Key Injection Attacks
**Target**: Cache `Set()` and `Get()` operations  
**Attack Vectors**:
- Very long keys (memory exhaustion)
- Null bytes and control characters
- Invalid UTF-8 sequences
- Keys designed to collide

**Properties Tested**:
- Set/Get idempotence
- No crashes or panics
- Memory safety (bounded cache size)
- Value consistency

**Why Critical**: Cache accepts untrusted keys from users.

### 3. `FuzzCacheConcurrentOperations` - Race Conditions
**Target**: Lock-free concurrent operations  
**Attack Vectors**:
- Concurrent Set/Get/Delete on same keys
- Race condition exploitation
- Data corruption under load

**Properties Tested**:
- Atomicity of operations
- No data corruption
- No deadlocks
- Cache remains functional

**Why Critical**: Balios is lock-free; race conditions could cause data corruption.

### 4. `FuzzGetOrLoad` - Loader Exploitation
**Target**: `GetOrLoad()` panic recovery  
**Attack Vectors**:
- Panicking loader functions
- Slow or hanging loaders
- Malformed return values

**Properties Tested**:
- Panic recovery works correctly
- Errors propagated properly
- Singleflight prevents multiple loads
- Cache remains functional

**Why Critical**: User-provided loaders could be malicious or buggy.

### 5. `FuzzGetOrLoadWithContext` - Timeout Handling
**Target**: Context cancellation and timeouts  
**Attack Vectors**:
- Loaders that ignore context
- Zero or negative timeouts
- Concurrent cancellations

**Properties Tested**:
- Timeouts are respected
- Context cancellation works
- No goroutine leaks
- Graceful degradation

**Why Critical**: Improper timeout handling can cause resource exhaustion.

### 6. `FuzzCacheConfig` - Configuration Validation
**Target**: Config validation and sanitization  
**Attack Vectors**:
- Negative values (integer overflow)
- Zero values (division by zero)
- Extreme values (memory exhaustion)

**Properties Tested**:
- Config validation catches invalid values
- Defaults are applied correctly
- Cache creation never panics
- Capacity is bounded

**Why Critical**: Invalid config could crash application or exhaust resources.

### 7. `FuzzCacheMemorySafety` - Memory Attacks
**Target**: Memory allocation and deallocation  
**Attack Vectors**:
- Very large values (OOM)
- Rapid allocation/deallocation (fragmentation)
- Concurrent memory access

**Properties Tested**:
- No memory leaks
- Memory usage is bounded
- No crashes or corruption
- GC can reclaim memory

**Why Critical**: Memory safety violations are critical security issues.

## Running Fuzz Tests

### Quick Test (30 seconds per fuzz function, ~3.5 min total)
```powershell
# Run all fuzz tests for 30 seconds each - perfect for development
go test -fuzz=FuzzStringHash -fuzztime=30s
go test -fuzz=FuzzCacheSetGet -fuzztime=30s
go test -fuzz=FuzzCacheConcurrentOperations -fuzztime=30s
go test -fuzz=FuzzGetOrLoad -fuzztime=30s
go test -fuzz=FuzzGetOrLoadWithContext -fuzztime=30s
go test -fuzz=FuzzCacheConfig -fuzztime=30s
go test -fuzz=FuzzCacheMemorySafety -fuzztime=30s
```

### Extended Test (5 minutes per function, ~35 min total)
```powershell
# Recommended for pre-commit hooks or PR checks
go test -fuzz=Fuzz -fuzztime=5m
```

### Using Makefile Targets

```powershell
# Quick fuzz test (30 seconds each, ~3.5 min total)
./Makefile.ps1 fuzz

# Extended fuzz test (5 minutes each, ~35 min total)
./Makefile.ps1 fuzz-extended
```

**Note**: For continuous fuzzing (overnight/weekend), run individual tests manually:
```powershell
# Run specific test for extended period (e.g., overnight)
go test -fuzz=FuzzStringHash -fuzztime=8h
```

## Interpreting Results

### Success (No Issues Found)
```
fuzz: elapsed: 1m0s, execs: 52341 (873/sec), new interesting: 12 (total: 56)
fuzz: elapsed: 2m0s, execs: 104682 (873/sec), new interesting: 15 (total: 59)
...
PASS
```
- `execs`: Number of test cases executed
- `new interesting`: New code coverage or edge cases discovered
- All tests pass

### Failure (Bug Found!)
```
--- FAIL: FuzzStringHash (0.23s)
    --- FAIL: FuzzStringHash (0.00s)
        balios_fuzz_test.go:123: HASH DETERMINISM VIOLATION: stringHash("...") produced different results
    
    Failing input written to testdata/fuzz/FuzzStringHash/...
```

When a failure is found:
1. **Input is saved** in `testdata/fuzz/FuzzStringHash/` for regression testing
2. **Failure is reproducible**: Run `go test -run=FuzzStringHash/` to replay
3. **Fix the bug** in source code
4. **Verify fix**: The saved input becomes a regression test

## Best Practices

### 1. Run Fuzz Tests Regularly
- **During development**: Quick fuzz (30 sec) - fast feedback loop
- **Pre-commit**: Extended fuzz (5 min) - catches most issues
- **In CI/CD**: Quick fuzz (30 sec) on every PR - keeps CI fast
- **Pre-release**: Extended fuzz (5 min) or manual overnight run

### 2. Monitor Coverage
```powershell
# Generate coverage report during fuzzing
go test -fuzz=Fuzz -fuzztime=1m -coverprofile=fuzz_coverage.out
go tool cover -html=fuzz_coverage.out
```

### 3. Corpus Management
The fuzzer builds a corpus of interesting inputs in `testdata/fuzz/`:
- **Keep this directory**: Commit to git for regression testing
- **Corpus grows over time**: More edge cases = better testing
- **Share corpus**: CI builds contribute to corpus

### 4. Combine with Other Testing
Fuzzing complements but doesn't replace:
- Unit tests (specific scenarios)
- Security tests (known attack vectors)
- Benchmark tests (performance)
- Integration tests (real-world usage)

## Performance Considerations

### Fuzzing is CPU-Intensive
- Uses all available CPU cores
- Generates millions of test cases per minute
- May heat up laptops during long runs

### Speed vs Coverage Trade-off
- **Faster iterations**: More test cases, broader coverage
- **Slower iterations**: Deeper exploration of complex inputs

Balios fuzz tests are optimized for speed while maintaining quality.

## Troubleshooting

### Fuzzing is Slow
- **Expected**: Fuzzing is computationally intensive
- **Solution**: Use shorter `-fuzztime` or run on more powerful hardware
- **Tip**: Run overnight or in CI instead of locally

### Out of Memory
- **Cause**: Fuzz tests generate large inputs
- **Solution**: Reduce `-fuzztime` or cap input sizes in fuzz functions
- **Note**: Balios fuzz tests cap inputs to reasonable sizes (1MB keys, etc.)

### False Positives
- **Should be ZERO**: Every failure should be a real bug
- **If you see one**: Check if it's a test bug or a real cache bug
- **Report it**: Open an issue with the failing input from `testdata/fuzz/`

### Corpus Too Large
- **Normal**: Corpus grows as more interesting cases are found
- **Management**: Periodically review and prune redundant entries
- **Tip**: Keep unique edge cases, remove duplicates

## Security Disclosure

If fuzzing reveals a **security vulnerability**:
1. **DO NOT** open a public GitHub issue
2. **DO** report privately to security contact in SECURITY.md
3. **WAIT** for coordinated disclosure before discussing publicly

## References

- **Balios Fuzz Tests**: [`balios_fuzz_test.go`](../balios_fuzz_test.go) - All 7 fuzz test implementations
- **Balios Security Tests**: [`balios_security_test.go`](../balios_security_test.go) - Additional, security hardening tests
- [Go Fuzzing Documentation](https://go.dev/security/fuzz/)
- [Fuzzing Best Practices](https://google.github.io/oss-fuzz/getting-started/best-practices/)
- [Hash DoS Attacks](https://www.youtube.com/watch?v=R2Cq3CLI6H8)
- [Balios Security Policy](../SECURITY.md)

---

Balios • an AGILira fragment
