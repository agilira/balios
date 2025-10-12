# Fuzz Test Corpus

This directory contains the **fuzz test corpus** for Balios cache library.

## What is a Fuzz Corpus?

The fuzz corpus is a collection of test inputs that have been discovered by the fuzzer to be "interesting" - meaning they:
- Achieve new code coverage
- Trigger edge cases or unusual code paths
- Potentially reveal bugs or security issues

## Structure

Each subdirectory corresponds to a specific fuzz test:

```
testdata/fuzz/
├── FuzzStringHash/           # Hash function fuzzing corpus
├── FuzzCacheSetGet/          # Cache Set/Get fuzzing corpus
├── FuzzCacheConcurrentOps/   # Concurrent operations fuzzing corpus
├── FuzzGetOrLoad/            # GetOrLoad fuzzing corpus
├── FuzzGetOrLoadWithContext/ # GetOrLoadWithContext fuzzing corpus
├── FuzzCacheConfig/          # Configuration fuzzing corpus
└── FuzzCacheMemorySafety/    # Memory safety fuzzing corpus
```

## Files in Corpus

Each file in the corpus contains a test case that was found to be interesting:
- Filename format: `<hex_hash>`
- Content: Raw binary data representing the fuzz input
- Purpose: Serves as a regression test - the fuzzer will always re-test these inputs

## Why Commit the Corpus?

The corpus should be **committed to git** for several reasons:

1. **Regression Testing**: Ensures bugs once found stay fixed
2. **Incremental Improvement**: Each fuzzing run builds on previous discoveries
3. **CI/CD Integration**: GitHub Actions and other CI systems use the corpus
4. **Knowledge Sharing**: Team members benefit from discovered edge cases

## Growing the Corpus

The corpus grows automatically as fuzzing discovers new cases:
- Local development fuzzing adds cases
- CI/CD fuzzing adds cases
- Manual fuzzing campaigns add cases

Over time, the corpus becomes more comprehensive and valuable.

## Corpus Size Management

If the corpus becomes too large:
- **Minimize**: Use `go test -fuzz=. -fuzzminimizetime=30s` to reduce inputs
- **Prune**: Manually remove redundant cases (be careful!)
- **Archive**: Move old corpus to a separate archive repo

Current recommendation: Keep corpus < 100MB total.

## Using the Corpus

### Run All Corpus Cases (Fast)
```bash
# Run all saved corpus cases without fuzzing new inputs
go test -run=FuzzStringHash
```

### Fuzz with Existing Corpus
```bash
# Fuzz for 1 minute using existing corpus as starting point
go test -fuzz=FuzzStringHash -fuzztime=1m
```

### Clear and Rebuild Corpus
```bash
# Remove all corpus (careful!)
rm -rf testdata/fuzz/*

# Rebuild corpus from scratch
make fuzz-extended
```

## Interpreting Corpus Files

Corpus files are binary and not human-readable. To inspect:

```bash
# View as hex dump
xxd testdata/fuzz/FuzzStringHash/abc123def456

# View as escaped string (if text-like)
cat testdata/fuzz/FuzzStringHash/abc123def456 | od -c
```

## Security Note

⚠️ **Some corpus entries may contain attack patterns!**

The corpus intentionally includes:
- Malicious keys designed to cause hash collisions
- Very long strings to test memory limits
- Invalid UTF-8 sequences
- Control characters and null bytes
- Patterns that exploit known vulnerabilities

This is **by design** - these are the inputs we want to continuously test against.

## Contributing

When contributing:
- ✅ **DO** commit your corpus additions
- ✅ **DO** run fuzzing before submitting PRs
- ✅ **DO** investigate any new failures
- ❌ **DON'T** delete corpus entries without good reason
- ❌ **DON'T** commit excessively large corpus files

## References

- [Go Fuzzing Documentation](https://go.dev/security/fuzz/)
- [Balios Fuzzing Guide](../../docs/FUZZING.md)
- [Corpus Management Best Practices](https://google.github.io/oss-fuzz/getting-started/new-project-guide/#seed-corpus)

---

**Last Updated**: 2025-10-12  
**Maintainer**: AGILira Security Team
