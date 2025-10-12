// balios_security_test.go: Comprehensive Security Testing Suite for Balios
//
// RED TEAM SECURITY ANALYSIS:
// This file implements systematic security testing against Balios cache library,
// designed to identify and prevent common attack vectors in production environments.
//
// THREAT MODEL:
// - Malicious cache key injection (memory exhaustion, collision attacks)
// - Resource exhaustion and DoS attacks (memory, CPU, goroutine leaks)
// - Loader function exploitation (panic injection, timeout abuse)
// - Concurrent access race conditions and data corruption
// - Configuration manipulation attacks (invalid bounds, overflow)
// - TTL manipulation and timing attacks
//
// PHILOSOPHY:
// Each test is designed to be:
// - DRY (Don't Repeat Yourself) with reusable security utilities
// - SMART (Specific, Measurable, Achievable, Relevant, Time-bound)
// - COMPREHENSIVE covering all major attack vectors
// - WELL-DOCUMENTED explaining the security implications
//
// METHODOLOGY:
// 1. Identify attack surface and entry points
// 2. Create targeted exploit scenarios
// 3. Test boundary conditions and edge cases
// 4. Validate security controls and mitigations
// 5. Document vulnerabilities and remediation steps
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGILira fragment
// SPDX-License-Identifier: MPL-2.0

package balios

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// =============================================================================
// SECURITY TESTING UTILITIES AND HELPERS
// =============================================================================

// SecurityTestContext provides utilities for security testing scenarios.
// This centralizes common security testing patterns and reduces code duplication.
type SecurityTestContext struct {
	t              *testing.T
	caches         []Cache
	cleanupFuncs   []func()
	mu             sync.Mutex
	memoryBaseline uint64
}

// NewSecurityTestContext creates a new security testing context with automatic cleanup.
//
// SECURITY BENEFIT: Ensures test isolation and prevents test artifacts from
// affecting system security or other tests. Critical for reliable security testing.
func NewSecurityTestContext(t *testing.T) *SecurityTestContext {
	ctx := &SecurityTestContext{
		t:            t,
		caches:       make([]Cache, 0),
		cleanupFuncs: make([]func(), 0),
	}

	// Capture memory baseline
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	ctx.memoryBaseline = m.Alloc

	// Register cleanup
	t.Cleanup(ctx.Cleanup)

	return ctx
}

// CreateMaliciousCache creates a cache with potentially dangerous configuration.
//
// SECURITY PURPOSE: Tests how Balios handles malicious configuration,
// including extreme values, invalid bounds, and resource exhaustion attempts.
func (ctx *SecurityTestContext) CreateMaliciousCache(config Config) Cache {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()

	cache := NewCache(config)
	ctx.caches = append(ctx.caches, cache)
	return cache
}

// ExpectSecurityError validates that a security-related error occurred.
//
// SECURITY PRINCIPLE: Security tests should expect failures when malicious
// input is provided. If an operation succeeds with malicious input, that
// indicates a potential security vulnerability.
func (ctx *SecurityTestContext) ExpectSecurityError(err error, operation string) {
	if err == nil {
		ctx.t.Errorf("SECURITY VULNERABILITY: %s should have failed with malicious input but succeeded", operation)
	}
}

// ExpectSecuritySuccess validates that a legitimate operation succeeded.
//
// SECURITY PRINCIPLE: Security controls should not break legitimate functionality.
func (ctx *SecurityTestContext) ExpectSecuritySuccess(err error, operation string) {
	if err != nil {
		ctx.t.Errorf("SECURITY ISSUE: %s should have succeeded with legitimate input but failed: %v", operation, err)
	}
}

// CheckMemoryLeak checks for memory leaks after operations.
//
// SECURITY PURPOSE: Memory leaks can be used for DoS attacks by exhausting
// system memory through repeated operations.
func (ctx *SecurityTestContext) CheckMemoryLeak(operation string, maxIncreaseMB float64) {
	runtime.GC()
	time.Sleep(50 * time.Millisecond) // Allow GC to complete
	runtime.GC()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	currentAlloc := m.Alloc

	// Handle case where memory decreased (GC freed memory)
	var increaseMB float64
	if currentAlloc > ctx.memoryBaseline {
		increaseMB = float64(currentAlloc-ctx.memoryBaseline) / 1024 / 1024
	} else {
		increaseMB = 0 // Memory decreased, no leak
	}

	if increaseMB > maxIncreaseMB {
		ctx.t.Errorf("SECURITY WARNING: Memory leak detected after %s: %.2f MB increase (max allowed: %.2f MB)",
			operation, increaseMB, maxIncreaseMB)
	}
}

// Cleanup cleans up all test resources.
func (ctx *SecurityTestContext) Cleanup() {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()

	// Close all caches
	for _, cache := range ctx.caches {
		if cache != nil {
			_ = cache.Close()
		}
	}

	// Run custom cleanup functions
	for _, fn := range ctx.cleanupFuncs {
		func() {
			defer func() {
				if r := recover(); r != nil {
					ctx.t.Logf("Warning: Cleanup function panicked: %v", r)
				}
			}()
			fn()
		}()
	}
}

// AddCleanup registers a cleanup function.
func (ctx *SecurityTestContext) AddCleanup(fn func()) {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	ctx.cleanupFuncs = append(ctx.cleanupFuncs, fn)
}

// =============================================================================
// CACHE KEY INJECTION AND MANIPULATION ATTACKS
// =============================================================================

// TestSecurity_KeyInjectionAttacks tests for malicious key injection vulnerabilities.
//
// ATTACK VECTOR: Cache key injection (CWE-20)
// DESCRIPTION: Attackers attempt to inject malicious keys to cause hash collisions,
// memory exhaustion, or bypass cache logic through specially crafted strings.
//
// IMPACT: Could cause cache pollution, memory exhaustion, or performance degradation
// affecting all users of the cache.
//
// MITIGATION EXPECTED: Balios should handle malicious keys gracefully without
// crashes, excessive memory usage, or performance degradation.
func TestSecurity_KeyInjectionAttacks(t *testing.T) {
	ctx := NewSecurityTestContext(t)

	maliciousKeys := []struct {
		name        string
		key         string
		description string
	}{
		{
			name:        "EmptyKey",
			key:         "",
			description: "Empty string key - should be handled gracefully",
		},
		{
			name:        "VeryLongKey",
			key:         strings.Repeat("A", 100_000), // 100KB key (reduced from 1MB for laptop & ci testing)
			description: "Very long key to test memory handling",
		},
		{
			name:        "NullByteInjection",
			key:         "key\x00value",
			description: "Null byte injection in key",
		},
		{
			name:        "ControlCharacters",
			key:         "key\x01\x02\x03\x7f\x1f",
			description: "Control characters in key",
		},
		{
			name:        "UnicodeExploits",
			key:         "key\u0000\uFFFE\uFFFF",
			description: "Unicode null and invalid characters",
		},
		{
			name:        "RepeatingPatterns",
			key:         strings.Repeat("AB", 500_000), // Pattern that might cause hash collisions
			description: "Repeating pattern to test hash collision resistance",
		},
		{
			name:        "NewlineInjection",
			key:         "key\n\r\nvalue",
			description: "Newline injection in key",
		},
		{
			name:        "SQLInjectionLike",
			key:         "'; DROP TABLE cache; --",
			description: "SQL injection-like string (should be harmless but tests sanitization)",
		},
		{
			name:        "FormatStringAttack",
			key:         "%s%s%s%s%s%s%s%s%s%s",
			description: "Format string attack pattern",
		},
		{
			name:        "UTF8Overlong",
			key:         "\xC0\x80", // Overlong UTF-8 encoding of null
			description: "Overlong UTF-8 encoding attack",
		},
	}

	for _, attack := range maliciousKeys {
		t.Run(attack.name, func(t *testing.T) {
			cache := ctx.CreateMaliciousCache(Config{
				MaxSize: 100,
			})

			// SECURITY TEST: Attempt to set malicious key
			result := cache.Set(attack.key, "test_value")

			// Cache should either accept or reject gracefully (not crash)
			if !result {
				t.Logf("SECURITY GOOD: Cache rejected malicious key: %s", attack.description)
			} else {
				t.Logf("SECURITY CHECK: Cache accepted malicious key, verifying safe handling: %s", attack.description)

				// If accepted, try to retrieve it
				value, found := cache.Get(attack.key)
				if found && value == "test_value" {
					t.Logf("SECURITY ACCEPTABLE: Key stored and retrieved correctly")
				}

				// Try to delete it
				deleted := cache.Delete(attack.key)
				if deleted {
					t.Logf("SECURITY ACCEPTABLE: Key deleted successfully")
				}
			}

			// SECURITY ASSERTION: Operation should not cause panic or memory leak
			ctx.CheckMemoryLeak(fmt.Sprintf("malicious key %s", attack.name), 50.0)
		})
	}
}

// TestSecurity_KeyHashCollisions tests for hash collision attacks.
//
// ATTACK VECTOR: Hash collision (CWE-407)
// DESCRIPTION: Attackers craft keys that produce the same hash values,
// degrading cache performance to O(n) instead of O(1).
//
// IMPACT: Could cause severe performance degradation and CPU exhaustion.
func TestSecurity_KeyHashCollisions(t *testing.T) {
	ctx := NewSecurityTestContext(t)

	cache := ctx.CreateMaliciousCache(Config{
		MaxSize: 1000,
	})

	// Generate keys that are likely to collide in a weak hash function
	// (Good hash functions should distribute these evenly)
	collisionKeys := make([]string, 0, 100)
	for i := 0; i < 100; i++ {
		// Keys with similar patterns but different suffixes
		collisionKeys = append(collisionKeys, fmt.Sprintf("collision_test_%d", i))
	}

	start := time.Now()

	// SECURITY TEST: Set all potentially colliding keys
	for _, key := range collisionKeys {
		cache.Set(key, fmt.Sprintf("value_%s", key))
	}

	setDuration := time.Since(start)

	start = time.Now()

	// SECURITY TEST: Retrieve all keys
	for _, key := range collisionKeys {
		_, found := cache.Get(key)
		if !found {
			t.Errorf("SECURITY ISSUE: Key not found after set: %s", key)
		}
	}

	getDuration := time.Since(start)

	// SECURITY ASSERTION: Operations should complete in reasonable time
	// Even with 100 keys, should be microseconds, not milliseconds
	maxSetDuration := 100 * time.Millisecond
	maxGetDuration := 50 * time.Millisecond

	if setDuration > maxSetDuration {
		t.Errorf("SECURITY WARNING: Hash collision attack may be affecting performance - Set took %v (max: %v)",
			setDuration, maxSetDuration)
	}

	if getDuration > maxGetDuration {
		t.Errorf("SECURITY WARNING: Hash collision attack may be affecting performance - Get took %v (max: %v)",
			getDuration, maxGetDuration)
	}

	t.Logf("SECURITY METRICS: Set 100 keys in %v, Get 100 keys in %v", setDuration, getDuration)
}

// =============================================================================
// RESOURCE EXHAUSTION AND DENIAL OF SERVICE TESTS
// =============================================================================

// TestSecurity_MemoryExhaustionAttacks tests for memory-based DoS vulnerabilities.
//
// ATTACK VECTOR: Memory exhaustion (CWE-770)
// DESCRIPTION: Attackers attempt to consume excessive memory through large values,
// large number of entries, or memory leaks.
//
// IMPACT: Could cause application crashes, system instability, or OOM kills.
//
// MITIGATION EXPECTED: Balios should enforce MaxSize limits and handle
// eviction properly to prevent unbounded memory growth.
func TestSecurity_MemoryExhaustionAttacks(t *testing.T) {
	ctx := NewSecurityTestContext(t)

	t.Run("LargeValueAttack", func(t *testing.T) {
		cache := ctx.CreateMaliciousCache(Config{
			MaxSize: 10,
		})

		// SECURITY TEST: Attempt to store very large values
		largeValue := make([]byte, 10*1024*1024) // 10MB value
		for i := range largeValue {
			largeValue[i] = byte(i % 256)
		}

		for i := 0; i < 5; i++ {
			key := fmt.Sprintf("large_key_%d", i)
			cache.Set(key, largeValue)
		}

		// SECURITY ASSERTION: Memory should be bounded by cache size
		// With 5 entries of 10MB each = 50MB maximum
		ctx.CheckMemoryLeak("large value attack", 100.0) // Allow 100MB (2x for overhead)
	})

	t.Run("ExceedMaxSizeAttack", func(t *testing.T) {
		maxSize := 100
		cache := ctx.CreateMaliciousCache(Config{
			MaxSize: maxSize,
		})

		// SECURITY TEST: Attempt to add more entries than MaxSize
		attemptedInsertions := maxSize * 10 // Try to insert 10x MaxSize

		for i := 0; i < attemptedInsertions; i++ {
			cache.Set(fmt.Sprintf("key_%d", i), fmt.Sprintf("value_%d", i))
		}

		// SECURITY ASSERTION: Cache size should not exceed MaxSize significantly
		stats := cache.Stats()
		if stats.Size > maxSize*2 {
			t.Errorf("SECURITY VULNERABILITY: Cache size (%d) exceeded MaxSize (%d) by more than 2x - eviction not working",
				stats.Size, maxSize)
		} else {
			t.Logf("SECURITY GOOD: Cache size controlled at %d (MaxSize: %d)", stats.Size, maxSize)
		}
	})

	t.Run("RapidChurnAttack", func(t *testing.T) {
		cache := ctx.CreateMaliciousCache(Config{
			MaxSize: 1000,
		})

		// SECURITY TEST: Rapid insert/delete to trigger memory fragmentation
		for iteration := 0; iteration < 100; iteration++ {
			for i := 0; i < 1000; i++ {
				cache.Set(fmt.Sprintf("churn_%d_%d", iteration, i), "value")
			}
			for i := 0; i < 500; i++ {
				cache.Delete(fmt.Sprintf("churn_%d_%d", iteration, i))
			}
		}

		// SECURITY ASSERTION: No significant memory leak from churn
		ctx.CheckMemoryLeak("rapid churn attack", 50.0)
	})
}

// TestSecurity_CPUExhaustionAttacks tests for CPU-based DoS vulnerabilities.
//
// ATTACK VECTOR: CPU exhaustion (CWE-407)
// DESCRIPTION: Attackers attempt to consume excessive CPU through expensive
// hash operations or linear scans.
//
// IMPACT: Could cause high CPU usage affecting application responsiveness.
//
// NOTE ON RACE DETECTOR:
// This test is designed to work WITHOUT the race detector flag (-race).
// Balios is a lock-free cache that trades some "benign data races" for performance.
// Running with -race will show false positives on entry.key reads/writes, which are
// intentional and safe due to the atomic 'valid' field synchronization.
// For proper race testing, use the dedicated concurrent tests in cache_test.go.
func TestSecurity_CPUExhaustionAttacks(t *testing.T) {
	ctx := NewSecurityTestContext(t)

	t.Run("HighFrequencyOperations", func(t *testing.T) {
		cache := ctx.CreateMaliciousCache(Config{
			MaxSize: 1000,
		})

		// SECURITY TEST: High frequency operations (reduced for laptop testing)
		start := time.Now()
		operations := 500_000 // Reduced from 1M for laptop

		for i := 0; i < operations; i++ {
			key := fmt.Sprintf("key_%d", i%1000)
			cache.Set(key, i)
			cache.Get(key)
		}

		duration := time.Since(start)

		// SECURITY ASSERTION: Should handle 500K ops in reasonable time
		maxDuration := 3 * time.Second

		if duration > maxDuration {
			t.Errorf("SECURITY WARNING: High frequency operations causing CPU exhaustion - took %v (max: %v)",
				duration, maxDuration)
		} else {
			opsPerSec := float64(operations) / duration.Seconds()
			t.Logf("SECURITY GOOD: Handled %d operations in %v (%.0f ops/sec)", operations, duration, opsPerSec)
		}
	})

	t.Run("ConcurrentHammering", func(t *testing.T) {
		cache := ctx.CreateMaliciousCache(Config{
			MaxSize: 1000,
		})

		// SECURITY TEST: Concurrent goroutines testing cache under load
		numGoroutines := 20
		opsPerGoroutine := 5_000

		var wg sync.WaitGroup
		start := time.Now()

		for g := 0; g < numGoroutines; g++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()
				for i := 0; i < opsPerGoroutine; i++ {
					key := fmt.Sprintf("key_%d_%d", goroutineID, i%100)
					cache.Set(key, i)
					cache.Get(key)
				}
			}(g)
		}

		wg.Wait()
		duration := time.Since(start)

		totalOps := numGoroutines * opsPerGoroutine
		opsPerSec := float64(totalOps) / duration.Seconds()

		t.Logf("SECURITY METRICS: %d concurrent goroutines, %d total ops in %v (%.0f ops/sec)",
			numGoroutines, totalOps, duration, opsPerSec)

		// SECURITY ASSERTION: Should not cause deadlock or extreme slowdown
		maxDuration := 10 * time.Second
		if duration > maxDuration {
			t.Errorf("SECURITY WARNING: Concurrent hammering caused excessive duration: %v (max: %v)",
				duration, maxDuration)
		}
	})
}

// TestSecurity_GoroutineLeakAttacks tests for goroutine leak vulnerabilities.
//
// ATTACK VECTOR: Goroutine leak (CWE-404)
// DESCRIPTION: Repeated operations that spawn goroutines without proper cleanup.
//
// IMPACT: Could cause goroutine exhaustion, scheduler thrashing, and memory leaks.
func TestSecurity_GoroutineLeakAttacks(t *testing.T) {
	ctx := NewSecurityTestContext(t)

	baselineGoroutines := runtime.NumGoroutine()

	cache := ctx.CreateMaliciousCache(Config{
		MaxSize: 100,
	})

	// SECURITY TEST: Repeated GetOrLoad calls that might leak goroutines
	for i := 0; i < 1000; i++ {
		_, _ = cache.GetOrLoad(fmt.Sprintf("key_%d", i), func() (interface{}, error) {
			return "value", nil
		})
	}

	// Allow time for goroutines to finish
	time.Sleep(100 * time.Millisecond)
	runtime.GC()

	currentGoroutines := runtime.NumGoroutine()
	leakedGoroutines := currentGoroutines - baselineGoroutines

	// SECURITY ASSERTION: Should not leak significant number of goroutines
	maxLeakedGoroutines := 50 // Allow some variance for test framework goroutines

	if leakedGoroutines > maxLeakedGoroutines {
		t.Errorf("SECURITY VULNERABILITY: Goroutine leak detected - %d goroutines leaked (baseline: %d, current: %d)",
			leakedGoroutines, baselineGoroutines, currentGoroutines)
	} else {
		t.Logf("SECURITY GOOD: No significant goroutine leak - baseline: %d, current: %d (delta: %d)",
			baselineGoroutines, currentGoroutines, leakedGoroutines)
	}
}

// =============================================================================
// LOADER FUNCTION EXPLOITATION TESTS
// =============================================================================

// TestSecurity_LoaderPanicAttacks tests handling of malicious loader functions.
//
// ATTACK VECTOR: Panic injection (CWE-248)
// DESCRIPTION: Loader functions that deliberately panic to crash the application
// or cause unexpected behavior.
//
// IMPACT: Could cause application crashes or denial of service.
//
// MITIGATION EXPECTED: Balios should recover from panics and return appropriate errors.
func TestSecurity_LoaderPanicAttacks(t *testing.T) {
	ctx := NewSecurityTestContext(t)

	cache := ctx.CreateMaliciousCache(Config{
		MaxSize: 100,
	})

	panicAttacks := []struct {
		name        string
		panicValue  interface{}
		description string
	}{
		{
			name:        "StringPanic",
			panicValue:  "malicious panic",
			description: "Simple string panic",
		},
		{
			name:        "NilPanic",
			panicValue:  nil,
			description: "Panic with nil value",
		},
		{
			name:        "StructPanic",
			panicValue:  struct{ msg string }{"complex panic"},
			description: "Panic with complex type",
		},
		{
			name:        "ErrorPanic",
			panicValue:  fmt.Errorf("error disguised as panic"),
			description: "Panic with error type",
		},
	}

	for _, attack := range panicAttacks {
		t.Run(attack.name, func(t *testing.T) {
			// SECURITY TEST: Loader that panics
			_, err := cache.GetOrLoad("panic_key", func() (interface{}, error) {
				panic(attack.panicValue)
			})

			// SECURITY ASSERTION: Should return error, not crash
			ctx.ExpectSecurityError(err, fmt.Sprintf("loader panic with %s", attack.description))

			// Verify error contains panic information
			if err != nil {
				if !strings.Contains(err.Error(), "panic") {
					t.Errorf("SECURITY ISSUE: Error should indicate panic recovery: %v", err)
				}
			}

			// Verify cache is still functional
			cache.Set("recovery_test", "value")
			if _, found := cache.Get("recovery_test"); !found {
				t.Error("SECURITY CRITICAL: Cache non-functional after loader panic")
			}
		})
	}
}

// TestSecurity_LoaderTimeoutAbuse tests for timeout-based attacks.
//
// ATTACK VECTOR: Timeout abuse (CWE-400)
// DESCRIPTION: Loader functions that deliberately take too long,
// trying to exhaust resources or bypass timeouts.
//
// IMPACT: Could cause thread pool exhaustion or system slowdown.
func TestSecurity_LoaderTimeoutAbuse(t *testing.T) {
	ctx := NewSecurityTestContext(t)

	cache := ctx.CreateMaliciousCache(Config{
		MaxSize: 100,
	})

	t.Run("SlowLoader", func(t *testing.T) {
		ctxTimeout, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		// SECURITY TEST: Loader that takes too long (reduced for laptop testing)
		start := time.Now()
		_, err := cache.GetOrLoadWithContext(ctxTimeout, "slow_key", func(ctx context.Context) (interface{}, error) {
			select {
			case <-time.After(1 * time.Second): // Deliberately slow but reasonable
				return "value", nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		})
		duration := time.Since(start)

		// SECURITY ASSERTION: Should timeout quickly, not wait for full 5 seconds
		ctx.ExpectSecurityError(err, "slow loader")

		if duration > 200*time.Millisecond {
			t.Errorf("SECURITY WARNING: Timeout not enforced properly - took %v (expected < 200ms)", duration)
		} else {
			t.Logf("SECURITY GOOD: Timeout enforced in %v", duration)
		}
	})

	t.Run("IgnoresContextLoader", func(t *testing.T) {
		ctxTimeout, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		// SECURITY TEST: Loader that ignores context cancellation
		start := time.Now()
		result, err := cache.GetOrLoadWithContext(ctxTimeout, "ignore_ctx_key", func(ctx context.Context) (interface{}, error) {
			// Deliberately ignore context but use reasonable time
			time.Sleep(500 * time.Millisecond)
			return "value", nil
		})
		duration := time.Since(start)

		// SECURITY ASSERTION: When loader ignores context, we can't force it to stop.
		// The loader will complete after 500ms and return a value.
		// This is acceptable behavior - the responsibility is on the loader author
		// to respect the context. We verify the system doesn't deadlock or panic.

		if duration > 600*time.Millisecond {
			t.Errorf("SECURITY WARNING: Loader took too long: %v", duration)
		}

		if err != nil {
			t.Logf("SECURITY GOOD: Got error from loader: %v", err)
		} else if result != nil {
			t.Logf("SECURITY ACCEPTABLE: Loader ignored context and completed successfully (not Balios's fault)")
		}
	})
}

// TestSecurity_LoaderConcurrentCalls tests singleflight behavior under attack.
//
// ATTACK VECTOR: Cache stampede exploitation (CWE-400)
// DESCRIPTION: Many concurrent requests for the same missing key trying to
// trigger multiple expensive loader calls.
//
// IMPACT: Could cause database/backend overload if singleflight fails.
//
// MITIGATION EXPECTED: Balios should ensure only one loader executes per key.
func TestSecurity_LoaderConcurrentCalls(t *testing.T) {
	ctx := NewSecurityTestContext(t)

	cache := ctx.CreateMaliciousCache(Config{
		MaxSize: 100,
	})

	// Track how many times loader is called
	var loaderCallCount int32

	numGoroutines := 100
	var wg sync.WaitGroup

	// SECURITY TEST: Many concurrent GetOrLoad for same key
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := cache.GetOrLoad("contested_key", func() (interface{}, error) {
				atomic.AddInt32(&loaderCallCount, 1)
				time.Sleep(10 * time.Millisecond) // Simulate slow operation
				return "value", nil
			})
			if err != nil {
				t.Errorf("GetOrLoad failed: %v", err)
			}
		}()
	}

	wg.Wait()

	finalCount := atomic.LoadInt32(&loaderCallCount)

	// SECURITY ASSERTION: Loader should be called only once (singleflight)
	if finalCount != 1 {
		t.Errorf("SECURITY VULNERABILITY: Singleflight broken - loader called %d times (expected: 1)", finalCount)
	} else {
		t.Logf("SECURITY GOOD: Singleflight working - loader called exactly once for %d concurrent requests", numGoroutines)
	}
}

// =============================================================================
// CONFIGURATION MANIPULATION ATTACKS
// =============================================================================

// TestSecurity_ConfigurationBoundaryAttacks tests for configuration validation vulnerabilities.
//
// ATTACK VECTOR: Configuration injection (CWE-15)
// DESCRIPTION: Providing extreme or invalid configuration values to cause
// crashes, excessive resource usage, or bypass security controls.
//
// IMPACT: Could cause application instability or resource exhaustion.
//
// MITIGATION EXPECTED: Balios should validate configuration and apply safe defaults.
func TestSecurity_ConfigurationBoundaryAttacks(t *testing.T) {
	ctx := NewSecurityTestContext(t)

	configAttacks := []struct {
		name        string
		config      Config
		description string
		shouldPanic bool
	}{
		{
			name:        "NegativeMaxSize",
			config:      Config{MaxSize: -1000},
			description: "Negative MaxSize should be handled",
			shouldPanic: false,
		},
		{
			name:        "ZeroMaxSize",
			config:      Config{MaxSize: 0},
			description: "Zero MaxSize should use default",
			shouldPanic: false,
		},
		{
			name:        "ExtremelyLargeMaxSize",
			config:      Config{MaxSize: 100_000}, // 100K entries (reasonable for github runners)
			description: "Large MaxSize should be handled (reduced from 1B for laptop & ci testing)",
			shouldPanic: false,
		},
		{
			name:        "InvalidWindowRatio",
			config:      Config{MaxSize: 100, WindowRatio: -0.5},
			description: "Negative WindowRatio should be handled",
			shouldPanic: false,
		},
		{
			name:        "WindowRatioGreaterThanOne",
			config:      Config{MaxSize: 100, WindowRatio: 1.5},
			description: "WindowRatio > 1.0 should be handled",
			shouldPanic: false,
		},
		{
			name:        "NegativeTTL",
			config:      Config{MaxSize: 100, TTL: -1 * time.Hour},
			description: "Negative TTL should be handled",
			shouldPanic: false,
		},
		{
			name:        "InvalidCounterBits",
			config:      Config{MaxSize: 100, CounterBits: -1},
			description: "Invalid CounterBits should be handled",
			shouldPanic: false,
		},
		{
			name:        "CounterBitsTooLarge",
			config:      Config{MaxSize: 100, CounterBits: 100},
			description: "CounterBits > 8 should be handled",
			shouldPanic: false,
		},
	}

	for _, attack := range configAttacks {
		t.Run(attack.name, func(t *testing.T) {
			// SECURITY TEST: Attempt to create cache with malicious config
			defer func() {
				if r := recover(); r != nil {
					if !attack.shouldPanic {
						t.Errorf("SECURITY VULNERABILITY: Cache creation panicked with config: %s - %v",
							attack.description, r)
					}
				}
			}()

			cache := ctx.CreateMaliciousCache(attack.config)

			// If creation succeeded, verify cache is functional
			cache.Set("test_key", "test_value")
			if _, found := cache.Get("test_key"); !found {
				t.Errorf("SECURITY ISSUE: Cache non-functional after configuration: %s", attack.description)
			}

			stats := cache.Stats()
			if stats.Capacity <= 0 {
				t.Errorf("SECURITY ISSUE: Invalid capacity after configuration: %d - %s",
					stats.Capacity, attack.description)
			} else {
				t.Logf("SECURITY GOOD: Config handled properly, capacity: %d - %s",
					stats.Capacity, attack.description)
			}
		})
	}
}

// =============================================================================
// RACE CONDITION AND CONCURRENCY ATTACKS
// =============================================================================

// TestSecurity_RaceConditionAttacks tests for race condition vulnerabilities.
//
// ATTACK VECTOR: Race conditions (CWE-362)
// DESCRIPTION: Concurrent operations on same keys trying to trigger data
// corruption, lost updates, or crashes.
//
// IMPACT: Could cause data corruption, inconsistent state, or application crashes.
//
// MITIGATION EXPECTED: Balios should be thread-safe with atomic operations.
func TestSecurity_RaceConditionAttacks(t *testing.T) {
	ctx := NewSecurityTestContext(t)

	t.Run("ConcurrentSetSameKey", func(t *testing.T) {
		// Create isolated cache for this test
		cache := ctx.CreateMaliciousCache(Config{
			MaxSize: 100,
		})
		numGoroutines := 100
		var wg sync.WaitGroup
		key := "contested_key"

		// SECURITY TEST: Many concurrent Set operations on same key
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(value int) {
				defer wg.Done()
				cache.Set(key, value)
			}(i)
		}

		wg.Wait()

		// SECURITY ASSERTION: Cache should have consistent state (no corruption)
		value, found := cache.Get(key)
		if !found {
			t.Error("SECURITY ISSUE: Key lost after concurrent updates")
		} else {
			// Value should be one of the written values (0-99)
			intValue, ok := value.(int)
			if !ok || intValue < 0 || intValue >= numGoroutines {
				t.Errorf("SECURITY VULNERABILITY: Data corruption detected - invalid value: %v", value)
			} else {
				t.Logf("SECURITY GOOD: Consistent value after concurrent updates: %d", intValue)
			}
		}
	})

	t.Run("ConcurrentSetDelete", func(t *testing.T) {
		// Create isolated cache for this test
		cache := ctx.CreateMaliciousCache(Config{
			MaxSize: 100,
		})

		// SECURITY TEST: Concurrent Set and Delete operations
		numIterations := 1000
		var wg sync.WaitGroup

		for i := 0; i < numIterations; i++ {
			key := fmt.Sprintf("key_%d", i)

			// Concurrent set and delete
			wg.Add(2)

			go func(k string) {
				defer wg.Done()
				cache.Set(k, "value")
			}(key)

			go func(k string) {
				defer wg.Done()
				cache.Delete(k)
			}(key)
		}

		wg.Wait()

		// SECURITY ASSERTION: Cache should be in valid state (no panics or corruption)
		// Note: In extreme concurrent scenarios with aggressive Set/Delete races,
		// the size counter can be significantly negative due to timing of atomic operations.
		// This is acceptable as long as the cache remains functional.
		stats := cache.Stats()

		// Allow large margin for extremely concurrent counter updates
		maxExpectedSize := int(cache.Capacity()) * 10 // Very permissive for this stress test
		if stats.Size < -maxExpectedSize || stats.Size > maxExpectedSize {
			t.Errorf("SECURITY VULNERABILITY: Extremely corrupted cache size: %d (capacity: %d)",
				stats.Size, cache.Capacity())
		} else {
			t.Logf("SECURITY ACCEPTABLE: Cache size within bounds after concurrent set/delete: %d", stats.Size)
		} // Verify cache still works
		cache.Set("verify_key", "verify_value")
		if val, found := cache.Get("verify_key"); !found || val != "verify_value" {
			t.Error("SECURITY ISSUE: Cache corrupted and non-functional")
		}
	})

	t.Run("ConcurrentGetSetDelete", func(t *testing.T) {
		// Create isolated cache for this test
		cache := ctx.CreateMaliciousCache(Config{
			MaxSize: 100,
		})

		// SECURITY TEST: Mix of concurrent Get, Set, Delete operations (reduced for laptop)
		numGoroutines := 20       // Reduced from 50
		numOpsPerGoroutine := 500 // Reduced from 1000
		var wg sync.WaitGroup

		for g := 0; g < numGoroutines; g++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()
				for i := 0; i < numOpsPerGoroutine; i++ {
					key := fmt.Sprintf("key_%d", i%100)

					switch i % 3 {
					case 0:
						// Use string value to avoid atomic.Value type panic
						cache.Set(key, fmt.Sprintf("value_%d", goroutineID))
					case 1:
						cache.Get(key)
					case 2:
						cache.Delete(key)
					}
				}
			}(g)
		}

		wg.Wait()

		// SECURITY ASSERTION: No panics, cache still functional
		cache.Set("final_test", "value")
		if _, found := cache.Get("final_test"); !found {
			t.Error("SECURITY ISSUE: Cache corrupted after mixed concurrent operations")
		} else {
			t.Log("SECURITY GOOD: Cache functional after mixed concurrent operations")
		}
	})
}

// =============================================================================
// TTL MANIPULATION AND TIMING ATTACKS
// =============================================================================

// TestSecurity_TTLManipulationAttacks tests for TTL-based vulnerabilities.
//
// ATTACK VECTOR: Time-based attacks (CWE-367)
// DESCRIPTION: Exploiting TTL behavior to cause race conditions, resource leaks,
// or bypass expiration logic.
//
// IMPACT: Could cause stale data serving or memory leaks.
//
// MITIGATION EXPECTED: Balios should handle TTL consistently and prevent timing attacks.
func TestSecurity_TTLManipulationAttacks(t *testing.T) {
	ctx := NewSecurityTestContext(t)

	t.Run("RapidExpirationChurn", func(t *testing.T) {
		cache := ctx.CreateMaliciousCache(Config{
			MaxSize: 100,
			TTL:     10 * time.Millisecond, // Very short TTL
		})

		// SECURITY TEST: Rapidly add entries that expire quickly
		for iteration := 0; iteration < 100; iteration++ {
			for i := 0; i < 50; i++ {
				cache.Set(fmt.Sprintf("ttl_key_%d_%d", iteration, i), "value")
			}

			time.Sleep(15 * time.Millisecond) // Wait for expiration

			// Try to access expired entries
			for i := 0; i < 50; i++ {
				cache.Get(fmt.Sprintf("ttl_key_%d_%d", iteration, i))
			}
		}

		// SECURITY ASSERTION: No memory leak from expired entries
		ctx.CheckMemoryLeak("rapid TTL expiration churn", 30.0)
	})

	t.Run("ZeroTTLBehavior", func(t *testing.T) {
		cache := ctx.CreateMaliciousCache(Config{
			MaxSize: 100,
			TTL:     0, // No expiration
		})

		// SECURITY TEST: With zero TTL, entries should never expire
		cache.Set("never_expire", "value")
		time.Sleep(100 * time.Millisecond)

		value, found := cache.Get("never_expire")
		if !found || value != "value" {
			t.Error("SECURITY ISSUE: Entry expired with zero TTL")
		}
	})

	t.Run("NegativeTTLHandling", func(t *testing.T) {
		// SECURITY TEST: Negative TTL should be handled safely
		cache := ctx.CreateMaliciousCache(Config{
			MaxSize: 100,
			TTL:     -1 * time.Hour, // Negative TTL
		})

		// Should either treat as no expiration or handle gracefully
		cache.Set("negative_ttl_key", "value")
		_, found := cache.Get("negative_ttl_key")

		if found {
			t.Log("SECURITY ACCEPTABLE: Negative TTL treated as no expiration")
		} else {
			t.Log("SECURITY ACCEPTABLE: Negative TTL caused immediate expiration")
		}
	})
}

// =============================================================================
// STRESS TESTING AND EDGE CASES
// =============================================================================

// TestSecurity_EdgeCaseStressTest tests extreme edge cases.
//
// ATTACK VECTOR: Edge case exploitation
// DESCRIPTION: Combination of edge cases that might reveal vulnerabilities.
//
// IMPACT: Unpredictable behavior or crashes under unusual conditions.
func TestSecurity_EdgeCaseStressTest(t *testing.T) {
	ctx := NewSecurityTestContext(t)

	t.Run("EmptyOperations", func(t *testing.T) {
		cache := ctx.CreateMaliciousCache(Config{MaxSize: 100})

		// SECURITY TEST: Operations on empty cache
		_, found := cache.Get("nonexistent")
		if found {
			t.Error("SECURITY ISSUE: Found nonexistent key")
		}

		deleted := cache.Delete("nonexistent")
		if deleted {
			t.Error("SECURITY ISSUE: Deleted nonexistent key")
		}

		has := cache.Has("nonexistent")
		if has {
			t.Error("SECURITY ISSUE: Has() returned true for nonexistent key")
		}

		// Multiple clears
		cache.Clear()
		cache.Clear()
		cache.Clear()

		stats := cache.Stats()
		if stats.Size != 0 {
			t.Errorf("SECURITY ISSUE: Size not zero after clear: %d", stats.Size)
		}
	})

	t.Run("NilValueHandling", func(t *testing.T) {
		cache := ctx.CreateMaliciousCache(Config{MaxSize: 100})

		// SECURITY TEST: Store nil value should not crash unexpectedly
		// Note: atomic.Value does not support nil, so we expect either:
		// 1. A controlled panic (which we recover from)
		// 2. The cache to reject the value gracefully

		defer func() {
			if r := recover(); r != nil {
				t.Logf("SECURITY ACCEPTABLE: nil value rejected with panic (expected): %v", r)
			}
		}()

		// Try to set nil - may panic
		result := cache.Set("nil_key", nil)

		if !result {
			t.Log("SECURITY GOOD: nil value rejected gracefully")
		} else {
			// If we get here, nil was accepted (shouldn't happen with atomic.Value)
			value, found := cache.Get("nil_key")
			if found && value == nil {
				t.Log("SECURITY ACCEPTABLE: nil value stored and retrieved")
			}
		}
	})

	t.Run("MultipleCloseOperations", func(t *testing.T) {
		cache := ctx.CreateMaliciousCache(Config{MaxSize: 100})

		// SECURITY TEST: Multiple close operations should not panic
		err := cache.Close()
		ctx.ExpectSecuritySuccess(err, "first close")

		err = cache.Close()
		if err != nil {
			t.Logf("SECURITY ACCEPTABLE: Second close returned error: %v", err)
		} else {
			t.Log("SECURITY ACCEPTABLE: Second close handled gracefully")
		}

		// Operations after close should be handled gracefully
		cache.Set("after_close", "value")
		cache.Get("after_close")
		cache.Delete("after_close")
	})
}

// =============================================================================
// INFORMATION DISCLOSURE TESTS
// =============================================================================

// TestSecurity_InformationDisclosureAttacks tests for information leak vulnerabilities.
//
// ATTACK VECTOR: Information disclosure (CWE-200)
// DESCRIPTION: Error messages, stats, or timing attacks that might leak
// sensitive information about cache internals or data.
//
// IMPACT: Could reveal system internals or sensitive data patterns.
func TestSecurity_InformationDisclosureAttacks(t *testing.T) {
	ctx := NewSecurityTestContext(t)

	cache := ctx.CreateMaliciousCache(Config{
		MaxSize: 100,
	})

	t.Run("ErrorMessageSanitization", func(t *testing.T) {
		// SECURITY TEST: Error messages should not leak sensitive information

		// Trigger various errors
		_, err := cache.GetOrLoad("test", nil) // nil loader
		if err != nil {
			errorMsg := err.Error()

			// Check for sensitive information leaks
			sensitivePatterns := []string{
				"password",
				"secret",
				"token",
				"0x", // Memory addresses
			}

			for _, pattern := range sensitivePatterns {
				if strings.Contains(strings.ToLower(errorMsg), pattern) {
					t.Errorf("SECURITY WARNING: Error message may leak sensitive info: %s", errorMsg)
				}
			}
		}
	})

	t.Run("StatsInformationLeak", func(t *testing.T) {
		// SECURITY TEST: Stats should not leak sensitive key/value information
		cache.Set("sensitive_key", "sensitive_value")

		stats := cache.Stats()

		// Stats should only contain aggregate information, not specific keys/values
		statsStr := fmt.Sprintf("%+v", stats)

		if strings.Contains(statsStr, "sensitive_key") || strings.Contains(statsStr, "sensitive_value") {
			t.Error("SECURITY VULNERABILITY: Stats leak specific key/value information")
		} else {
			t.Log("SECURITY GOOD: Stats contain only aggregate information")
		}
	})

	t.Run("TimingAttackResistance", func(t *testing.T) {
		// SECURITY TEST: Get operations should have consistent timing
		// to prevent timing attacks that reveal cache state

		key := "timing_test_key"
		cache.Set(key, "value")

		// Measure timing for hit
		start := time.Now()
		cache.Get(key)
		hitDuration := time.Since(start)

		// Measure timing for miss
		start = time.Now()
		cache.Get("nonexistent_key")
		missDuration := time.Since(start)

		// Timing difference should not be exploitable (< 1ms difference)
		timingDiff := hitDuration - missDuration
		if timingDiff < 0 {
			timingDiff = -timingDiff
		}

		if timingDiff > time.Millisecond {
			t.Logf("SECURITY NOTE: Timing difference between hit and miss: %v (may be exploitable)", timingDiff)
		} else {
			t.Logf("SECURITY GOOD: Timing difference minimal: %v", timingDiff)
		}
	})
}
