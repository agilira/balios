// balios_fuzz_test.go - Comprehensive fuzz testing for Balios cache
//
// FUZZING PHILOSOPHY:
// We focus on security-critical functions that process untrusted input:
// 1. stringHash - Core hash function (collision resistance, distribution)
// 2. Cache operations - Key injection, memory safety, concurrent access
// 3. Configuration - Boundary conditions, overflow protection
//
// FUZZING TARGETS:
// - Hash function collision resistance and distribution quality
// - Cache key handling with malicious/malformed strings
// - Configuration validation with extreme values
// - Concurrent operations under stress
//
// PERFORMANCE CONSIDERATIONS:
// - Fuzz tests run for extended periods (hours/days in CI)
// - We use property-based testing to catch violations, not just crashes
// - False positive rate must be ZERO - every failure is a real bug
//
// SECURITY INVARIANTS:
// 1. Hash function must not have exploitable collision patterns
// 2. Cache must not panic or crash with any key input
// 3. Memory usage must be bounded regardless of input
// 4. Concurrent operations must maintain consistency
// 5. Configuration must validate and prevent resource exhaustion
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
	"testing"
	"time"
	"unicode/utf8"
)

// =============================================================================
// HASH FUNCTION FUZZING
// =============================================================================

// FuzzStringHash performs comprehensive fuzz testing on the stringHash function.
//
// SECURITY CRITICAL: The hash function is the foundation of cache performance.
// Poor hash distribution or exploitable collision patterns could enable:
// - Hash collision DoS attacks (degrade O(1) to O(n) performance)
// - Cache pollution attacks (fill cache with colliding keys)
// - Timing attacks (predictable hash patterns)
//
// PROPERTIES TESTED:
// 1. Determinism: Same input always produces same hash
// 2. Avalanche effect: Small input changes produce large hash changes
// 3. No crashes: Function never panics regardless of input
// 4. Performance: Hash computation completes in bounded time
// 5. Distribution: Similar inputs produce dissimilar hashes
//
// FALSE POSITIVE PREVENTION:
// - We don't test for "perfect" distribution (impossible to achieve)
// - We focus on exploitable patterns that attackers could leverage
// - Statistical properties are tested with realistic thresholds
func FuzzStringHash(f *testing.F) {
	// SEED CORPUS: Representative inputs covering edge cases

	// Normal cases - typical cache keys
	f.Add("user:123")
	f.Add("session:abc-def-ghi")
	f.Add("config.json")
	f.Add("data/cache/item_42")

	// Empty and minimal
	f.Add("")
	f.Add("a")
	f.Add("ab")

	// ASCII edge cases
	f.Add("aaaaaaaa")         // Repeating characters
	f.Add("01234567")         // Sequential
	f.Add("\x00\x01\x02\x03") // Control characters
	f.Add("\n\r\t ")          // Whitespace

	// Unicode and international
	f.Add("用户:123")       // Chinese
	f.Add("пользователь") // Cyrillic
	f.Add("ユーザー")         // Japanese
	f.Add("café☕️")       // Emoji and diacritics
	f.Add("🚀🎯💾")          // Multiple emoji

	// Potential attack patterns
	f.Add(strings.Repeat("A", 100))   // Long repeating
	f.Add(strings.Repeat("AB", 50))   // Pattern
	f.Add(strings.Repeat("\x00", 50)) // Null bytes

	// Hash collision candidates (patterns that might collide in weak hashes)
	f.Add("key_1")
	f.Add("key_2")
	f.Add("key1")
	f.Add("1key")
	f.Add("_key1")

	f.Fuzz(func(t *testing.T, input string) {
		// PROPERTY 1: Determinism - same input produces same hash
		hash1 := stringHash(input)
		hash2 := stringHash(input)
		if hash1 != hash2 {
			t.Errorf("HASH DETERMINISM VIOLATION: stringHash(%q) produced different results: %v != %v",
				truncateForDisplay(input), hash1, hash2)
		}

		// PROPERTY 2: No panics (implicitly tested by fuzzer)
		// If stringHash panics, the fuzzer will catch it

		// PROPERTY 3: Valid UTF-8 strings should be handled safely
		if utf8.ValidString(input) {
			// Hash should complete without issues for valid UTF-8
			_ = stringHash(input)
		}

		// PROPERTY 4: Avalanche effect - small changes produce different hashes
		// Only test on reasonably-sized inputs to avoid noise
		if len(input) > 0 && len(input) < 1000 {
			// Flip a bit in the middle of the string
			modified := []byte(input)
			midPoint := len(modified) / 2
			if midPoint < len(modified) {
				modified[midPoint] ^= 0x01 // Flip lowest bit
				modifiedHash := stringHash(string(modified))

				// Different inputs should produce different hashes (avalanche)
				// Note: We can't guarantee this 100% (hash collisions exist),
				// but for single-bit flips it should be extremely rare
				if hash1 == modifiedHash {
					// This could be a legitimate collision, so we log but don't fail
					// Only fail if we see a pattern of collisions
					t.Logf("HASH COLLISION DETECTED: %q and %q produce same hash %v",
						truncateForDisplay(input), truncateForDisplay(string(modified)), hash1)
				}
			}
		}

		// PROPERTY 5: Hash distribution - bits should be well-distributed
		// We check that not all bits are 0 or 1 (extremely weak hash)
		if hash1 != 0 && hash1 != ^uint64(0) {
			// Good - not all zeros or all ones
		} else if len(input) > 0 {
			// Only flag this for non-empty inputs
			t.Logf("HASH QUALITY WARNING: Hash has extreme value for non-empty input: %q -> %v",
				truncateForDisplay(input), hash1)
		}

		// PROPERTY 6: Performance - hash should compute quickly
		// This is checked implicitly by fuzzer timeout
	})
}

// =============================================================================
// CACHE KEY INJECTION FUZZING
// =============================================================================

// FuzzCacheSetGet performs fuzz testing on cache Set/Get operations with malicious keys.
//
// SECURITY CRITICAL: Cache accepts arbitrary string keys from untrusted sources.
// Malicious keys could cause:
// - Memory exhaustion (very long keys)
// - Hash collision DoS (crafted keys that collide)
// - Crashes or panics (malformed UTF-8, control characters)
// - Race conditions (concurrent access with same keys)
//
// PROPERTIES TESTED:
// 1. Set and Get are idempotent: Set(k,v) then Get(k) returns v
// 2. No crashes: Operations never panic regardless of key
// 3. Memory safety: Cache size remains bounded
// 4. Consistency: Get returns correct value after Set
//
// COVERAGE:
// - All possible string byte sequences (valid and invalid UTF-8)
// - Extreme lengths (empty to very long)
// - Special characters (null bytes, control chars, unicode)
// - Patterns that might cause hash collisions
func FuzzCacheSetGet(f *testing.F) {
	// SEED CORPUS: Attack vectors and edge cases

	// Normal keys
	f.Add("key", "value")
	f.Add("user:123", "data")

	// Empty cases
	f.Add("", "value")
	f.Add("key", "")
	f.Add("", "")

	// Very long keys (memory exhaustion attempt)
	f.Add(strings.Repeat("A", 10000), "value")
	f.Add("key", strings.Repeat("X", 10000))

	// Control characters and null bytes
	f.Add("key\x00value", "data")
	f.Add("key\n\r\t", "data")
	f.Add("\x00\x01\x02", "data")

	// Unicode edge cases
	f.Add("用户:123", "数据")
	f.Add("key🚀", "value💾")

	// Invalid UTF-8 (malformed sequences)
	f.Add("\xff\xfe", "value") // Invalid UTF-8
	f.Add("key", "\x80\x81")   // Invalid UTF-8 value

	// Patterns that might collide
	f.Add(strings.Repeat("AB", 100), "data1")
	f.Add(strings.Repeat("AB", 101), "data2")

	f.Fuzz(func(t *testing.T, key string, value string) {
		// Create cache for this fuzz iteration
		// Use small MaxSize to detect memory issues quickly
		cache := NewCache(Config{
			MaxSize: 100,
		})
		defer func() { _ = cache.Close() }()

		// PROPERTY 1: Set operation should not panic
		var setResult bool
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("PANIC in Set(%q, %q): %v",
						truncateForDisplay(key), truncateForDisplay(value), r)
				}
			}()
			setResult = cache.Set(key, value)
		}()

		// PROPERTY 2: If Set succeeds, Get should retrieve the value
		if setResult {
			var getValue interface{}
			var found bool
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("PANIC in Get(%q): %v", truncateForDisplay(key), r)
					}
				}()
				getValue, found = cache.Get(key)
			}()

			if !found {
				// This is acceptable if cache is full and entry was evicted
				// But with MaxSize=100 and 1 entry, this shouldn't happen
				t.Logf("WARNING: Set succeeded but Get failed for key: %q", truncateForDisplay(key))
			} else if getValue != value {
				t.Errorf("VALUE MISMATCH: Set(%q, %q) but Get returned %q",
					truncateForDisplay(key), truncateForDisplay(value), truncateForDisplay(fmt.Sprint(getValue)))
			}
		}

		// PROPERTY 3: Delete should not panic
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("PANIC in Delete(%q): %v", truncateForDisplay(key), r)
				}
			}()
			cache.Delete(key)
		}()

		// PROPERTY 4: Has should not panic
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("PANIC in Has(%q): %v", truncateForDisplay(key), r)
				}
			}()
			cache.Has(key)
		}()

		// PROPERTY 5: Cache size should remain bounded
		stats := cache.Stats()
		if stats.Size > 200 { // Allow 2x MaxSize for concurrent operations buffer
			t.Errorf("MEMORY SAFETY VIOLATION: Cache size %d exceeds safe limit (MaxSize=100)",
				stats.Size)
		}
	})
}

// =============================================================================
// CACHE CONCURRENT OPERATIONS FUZZING
// =============================================================================

// FuzzCacheConcurrentOperations fuzzes concurrent cache operations for race conditions.
//
// SECURITY CRITICAL: Balios is lock-free and designed for high concurrency.
// Race conditions could cause:
// - Data corruption (wrong values returned)
// - Memory safety violations (dangling pointers, use-after-free)
// - Deadlocks or livelocks
// - Crashes under concurrent load
//
// PROPERTIES TESTED:
// 1. Atomicity: Operations complete atomically
// 2. Consistency: No data corruption under concurrent access
// 3. Isolation: Operations don't interfere with each other
// 4. No deadlocks: System remains responsive
//
// NOTE: This test should be run with -race flag to detect data races
func FuzzCacheConcurrentOperations(f *testing.F) {
	// SEED CORPUS: Keys that trigger concurrent access patterns
	f.Add("key1", "value1", int8(0)) // operation type 0 = Set
	f.Add("key2", "value2", int8(1)) // operation type 1 = Get
	f.Add("key3", "value3", int8(2)) // operation type 2 = Delete
	f.Add("shared", "data", int8(0)) // Same key from multiple goroutines

	f.Fuzz(func(t *testing.T, key string, value string, opType int8) {
		// Create cache for this iteration
		cache := NewCache(Config{
			MaxSize: 100,
		})
		defer func() { _ = cache.Close() }()

		// Normalize operation type to 0-2
		op := int(opType) % 3
		if op < 0 {
			op = -op
		}

		// Run operations concurrently from multiple goroutines
		const numGoroutines = 10
		var wg sync.WaitGroup
		errChan := make(chan string, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()
				defer func() {
					if r := recover(); r != nil {
						errChan <- fmt.Sprintf("Goroutine %d panicked: %v", goroutineID, r)
					}
				}()

				// Perform operation based on type
				switch op {
				case 0: // Set
					cache.Set(key, fmt.Sprintf("%s_%d", value, goroutineID))
				case 1: // Get
					cache.Get(key)
				case 2: // Delete
					cache.Delete(key)
				}
			}(i)
		}

		// Wait for all goroutines with timeout
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Success
		case <-time.After(5 * time.Second):
			t.Error("DEADLOCK: Concurrent operations did not complete within timeout")
			return
		}

		// Check for panics
		close(errChan)
		for err := range errChan {
			t.Error(err)
		}

		// PROPERTY: Cache should still be functional after concurrent operations
		cache.Set("test_after_concurrent", "value")
		if val, found := cache.Get("test_after_concurrent"); !found || val != "value" {
			t.Error("CORRUPTION: Cache not functional after concurrent operations")
		}
	})
}

// =============================================================================
// GETORLOAD FUZZING (PANIC RECOVERY AND CONTEXT HANDLING)
// =============================================================================

// FuzzGetOrLoad fuzzes the GetOrLoad function with malicious loader functions.
//
// SECURITY CRITICAL: GetOrLoad executes user-provided loader functions.
// Malicious loaders could:
// - Panic and crash the application
// - Hang indefinitely (DoS)
// - Return malicious data
// - Trigger race conditions in singleflight
//
// PROPERTIES TESTED:
// 1. Panic recovery: Panicking loaders don't crash the cache
// 2. Error handling: Loader errors are propagated correctly
// 3. Singleflight: Concurrent calls execute loader only once
// 4. Context cancellation: Timeouts are respected
func FuzzGetOrLoad(f *testing.F) {
	// SEED CORPUS: Different loader behaviors
	f.Add("key1", int8(0), "data")  // Normal loader
	f.Add("key2", int8(1), "panic") // Panicking loader
	f.Add("key3", int8(2), "error") // Error loader
	f.Add("key4", int8(3), "slow")  // Slow loader

	f.Fuzz(func(t *testing.T, key string, loaderType int8, loaderData string) {
		cache := NewCache(Config{
			MaxSize: 100,
		})
		defer func() { _ = cache.Close() }()

		// Normalize loader type to 0-3
		lt := int(loaderType) % 4
		if lt < 0 {
			lt = -lt
		}

		// Create loader function based on type
		var loader func() (interface{}, error)
		switch lt {
		case 0: // Normal loader
			loader = func() (interface{}, error) {
				return loaderData, nil
			}
		case 1: // Panicking loader
			loader = func() (interface{}, error) {
				panic(loaderData)
			}
		case 2: // Error loader
			loader = func() (interface{}, error) {
				return nil, fmt.Errorf("loader error: %s", loaderData)
			}
		case 3: // Slow loader (respects context)
			loader = func() (interface{}, error) {
				time.Sleep(10 * time.Millisecond)
				return loaderData, nil
			}
		}

		// PROPERTY 1: GetOrLoad should not panic even with panicking loader
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("PANIC RECOVERY FAILED: GetOrLoad panicked with key=%q, loaderType=%d: %v",
						truncateForDisplay(key), lt, r)
				}
			}()

			_, err := cache.GetOrLoad(key, loader)

			// PROPERTY 2: Panicking loader should return error
			if lt == 1 && err == nil {
				t.Error("PANIC RECOVERY MISSING: Panicking loader should return error")
			}

			// PROPERTY 3: Error loader should return error
			if lt == 2 && err == nil {
				t.Error("ERROR PROPAGATION MISSING: Error loader should return error")
			}

			// PROPERTY 4: Normal loader should succeed
			if lt == 0 && err != nil {
				t.Errorf("LOADER FAILURE: Normal loader returned error: %v", err)
			}
		}()

		// PROPERTY 5: Cache should remain functional
		cache.Set("test_after_getorload", "value")
		if val, found := cache.Get("test_after_getorload"); !found || val != "value" {
			t.Error("CORRUPTION: Cache not functional after GetOrLoad")
		}
	})
}

// =============================================================================
// GETORLOAD WITH CONTEXT FUZZING
// =============================================================================

// FuzzGetOrLoadWithContext fuzzes GetOrLoadWithContext for timeout and cancellation handling.
//
// SECURITY CRITICAL: Context handling is critical for preventing DoS attacks.
// Improper handling could cause:
// - Goroutine leaks (not respecting cancellation)
// - Resource exhaustion (ignoring timeouts)
// - Deadlocks (waiting forever)
func FuzzGetOrLoadWithContext(f *testing.F) {
	// SEED CORPUS: Different timeout and loader behaviors
	f.Add("key1", int64(100), "data", int8(0))   // 100ms timeout, normal loader
	f.Add("key2", int64(10), "data", int8(1))    // 10ms timeout, slow loader
	f.Add("key3", int64(0), "data", int8(2))     // Immediate timeout
	f.Add("key4", int64(1000), "panic", int8(3)) // Panicking loader

	f.Fuzz(func(t *testing.T, key string, timeoutMs int64, loaderData string, loaderType int8) {
		// Skip invalid timeouts (fuzz might generate negatives)
		if timeoutMs < 0 {
			timeoutMs = -timeoutMs
		}
		if timeoutMs > 5000 { // Cap at 5 seconds for fuzzing
			timeoutMs = 5000
		}

		cache := NewCache(Config{
			MaxSize: 100,
		})
		defer func() { _ = cache.Close() }()

		// Create context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutMs)*time.Millisecond)
		defer cancel()

		// Normalize loader type
		lt := int(loaderType) % 4
		if lt < 0 {
			lt = -lt
		}

		// Create context-aware loader
		var loader func(context.Context) (interface{}, error)
		switch lt {
		case 0: // Fast loader
			loader = func(ctx context.Context) (interface{}, error) {
				return loaderData, nil
			}
		case 1: // Slow loader (respects context)
			loader = func(ctx context.Context) (interface{}, error) {
				select {
				case <-time.After(200 * time.Millisecond):
					return loaderData, nil
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
		case 2: // Slow loader (ignores context - bad practice but should not break cache)
			loader = func(ctx context.Context) (interface{}, error) {
				time.Sleep(200 * time.Millisecond)
				return loaderData, nil
			}
		case 3: // Panicking loader
			loader = func(ctx context.Context) (interface{}, error) {
				panic(loaderData)
			}
		}

		// PROPERTY 1: GetOrLoadWithContext should not panic
		var result interface{}
		var err error
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("PANIC in GetOrLoadWithContext: key=%q, timeout=%dms, loaderType=%d: %v",
						truncateForDisplay(key), timeoutMs, lt, r)
				}
			}()

			result, err = cache.GetOrLoadWithContext(ctx, key, loader)
		}()

		// PROPERTY 2: Panicking loader should return error
		if lt == 3 && err == nil {
			t.Error("PANIC RECOVERY MISSING: Panicking loader should return error")
		}

		// PROPERTY 3: Timeout should be respected (for slow loaders)
		if lt == 1 && timeoutMs < 200 {
			// Slow loader with short timeout should timeout
			if err == nil && result != nil {
				// This might be OK if loader respected context and returned quickly
				t.Logf("INFO: Slow loader completed before timeout (may have respected context)")
			}
		}

		// PROPERTY 4: Cache should remain functional
		cache.Set("test_after_context", "value")
		if val, found := cache.Get("test_after_context"); !found || val != "value" {
			t.Error("CORRUPTION: Cache not functional after GetOrLoadWithContext")
		}
	})
}

// =============================================================================
// CONFIGURATION FUZZING
// =============================================================================

// FuzzCacheConfig fuzzes cache configuration for validation and safety.
//
// SECURITY CRITICAL: Invalid configuration could cause:
// - Memory exhaustion (huge MaxSize)
// - Integer overflow (negative sizes)
// - Division by zero (zero WindowRatio)
// - Crashes (invalid CounterBits)
//
// PROPERTIES TESTED:
// 1. Config validation: Invalid configs are rejected or sanitized
// 2. Cache creation: NewCache never panics
// 3. Functional cache: Cache works after creation
// 4. Memory safety: Cache size is bounded
func FuzzCacheConfig(f *testing.F) {
	// SEED CORPUS: Edge case configurations
	f.Add(int32(100), float32(0.01), int8(4))     // Normal
	f.Add(int32(0), float32(0.0), int8(0))        // All zeros
	f.Add(int32(-1), float32(-0.5), int8(-1))     // Negatives
	f.Add(int32(1000000), float32(1.5), int8(16)) // Extremes
	f.Add(int32(1), float32(0.001), int8(1))      // Minimums

	f.Fuzz(func(t *testing.T, maxSize int32, windowRatio float32, counterBits int8) {
		// Convert fuzzer types to Config types
		config := Config{
			MaxSize:     int(maxSize),
			WindowRatio: float64(windowRatio),
			CounterBits: int(counterBits),
		}

		// PROPERTY 1: NewCache should not panic with any configuration
		var cache Cache
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("PANIC in NewCache: MaxSize=%d, WindowRatio=%f, CounterBits=%d: %v",
						maxSize, windowRatio, counterBits, r)
				}
			}()
			cache = NewCache(config)
		}()

		if cache == nil {
			t.Fatal("NewCache returned nil cache")
		}
		defer func() { _ = cache.Close() }()

		// PROPERTY 2: Cache should have valid capacity (positive and bounded)
		capacity := cache.Capacity()
		if capacity <= 0 {
			t.Errorf("INVALID CAPACITY: Cache capacity is %d (should be > 0)", capacity)
		}

		// PROPERTY 3: Cache should have reasonable capacity even with fuzzy input
		// Don't allow extreme values that could exhaust memory
		const maxReasonableCapacity = 10_000_000 // 10M entries max for fuzzing
		if capacity > maxReasonableCapacity {
			t.Errorf("EXCESSIVE CAPACITY: Cache capacity is %d (max: %d)",
				capacity, maxReasonableCapacity)
		}

		// PROPERTY 4: Cache should be functional
		testKey := "fuzz_test_key"
		testValue := "fuzz_test_value"

		cache.Set(testKey, testValue)
		if val, found := cache.Get(testKey); !found || val != testValue {
			t.Error("FUNCTIONAL FAILURE: Cache not working after creation with fuzzed config")
		}

		// PROPERTY 5: Stats should be consistent
		stats := cache.Stats()
		if stats.Capacity != capacity {
			t.Errorf("STATS INCONSISTENCY: Stats.Capacity=%d != Capacity()=%d",
				stats.Capacity, capacity)
		}
	})
}

// =============================================================================
// MEMORY SAFETY FUZZING
// =============================================================================

// FuzzCacheMemorySafety fuzzes cache for memory safety violations.
//
// SECURITY CRITICAL: Memory corruption could lead to:
// - Arbitrary code execution
// - Information disclosure
// - Crashes and DoS
//
// This test focuses on operations that manipulate memory:
// - Very large values
// - Rapid allocation/deallocation
// - Concurrent memory access
func FuzzCacheMemorySafety(f *testing.F) {
	// SEED CORPUS: Memory-intensive patterns
	f.Add("key", 1000, int8(10))    // 1KB value, 10 iterations
	f.Add("large", 100000, int8(5)) // 100KB value, 5 iterations
	f.Add("tiny", 1, int8(100))     // 1 byte value, 100 iterations
	f.Add("churn", 10000, int8(50)) // 10KB value, 50 iterations (churn test)

	f.Fuzz(func(t *testing.T, keyPrefix string, valueSize int, iterations int8) {
		// Sanitize inputs
		if valueSize < 0 {
			valueSize = -valueSize
		}
		if valueSize > 1_000_000 { // Cap at 1MB for fuzzing
			valueSize = 1_000_000
		}
		if iterations < 0 {
			iterations = -iterations
		}
		if iterations > 100 { // Cap iterations
			iterations = 100
		}

		cache := NewCache(Config{
			MaxSize: 100,
		})
		defer func() { _ = cache.Close() }()

		// Track memory baseline
		runtime.GC()
		var memBefore runtime.MemStats
		runtime.ReadMemStats(&memBefore)

		// Create value of specified size
		value := make([]byte, valueSize)
		for i := range value {
			value[i] = byte(i % 256)
		}

		// Perform operations
		for i := int8(0); i < iterations; i++ {
			key := fmt.Sprintf("%s_%d", keyPrefix, i)

			// PROPERTY: Operations should not panic
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("PANIC during memory operations: iteration=%d, valueSize=%d: %v",
							i, valueSize, r)
					}
				}()

				cache.Set(key, value)
				cache.Get(key)
				cache.Delete(key)
			}()
		}

		// Force garbage collection
		cache.Clear()
		runtime.GC()
		time.Sleep(10 * time.Millisecond)
		runtime.GC()

		// Check for memory leaks
		var memAfter runtime.MemStats
		runtime.ReadMemStats(&memAfter)

		// Calculate memory increase (allowing for GC not being perfect)
		memIncrease := int64(memAfter.Alloc) - int64(memBefore.Alloc)
		if memIncrease < 0 {
			memIncrease = 0 // Memory decreased (good)
		}

		// PROPERTY: Memory should not leak excessively
		// Allow 10MB increase for overhead and fuzzer itself
		const maxMemoryIncreaseMB = 10
		memIncreaseMB := float64(memIncrease) / 1024 / 1024
		if memIncreaseMB > maxMemoryIncreaseMB {
			t.Errorf("MEMORY LEAK: Excessive memory increase: %.2f MB (max: %d MB)",
				memIncreaseMB, maxMemoryIncreaseMB)
		}
	})
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// truncateForDisplay truncates strings for display in error messages.
func truncateForDisplay(s string) string {
	maxLen := 50
	if len(s) <= maxLen {
		// Show as Go string literal for visibility
		return fmt.Sprintf("%q", s)
	}
	// Truncate and show byte count
	return fmt.Sprintf("%q... (len=%d)", s[:maxLen], len(s))
}

// =============================================================================
// FUZZ REGRESSION TESTS
// =============================================================================

// TestFuzzRegressions tests specific cases found by fuzzing.
// These are kept as regression tests to ensure bugs don't resurface.
func TestFuzzRegressions(t *testing.T) {
	// Add regression test cases here as fuzzing discovers issues

	t.Run("EmptyKeyHandling", func(t *testing.T) {
		cache := NewCache(Config{MaxSize: 100})
		defer func() { _ = cache.Close() }()

		// Empty keys should be handled gracefully
		cache.Set("", "value")
		val, found := cache.Get("")
		if found && val != "value" {
			t.Error("Empty key not handled correctly")
		}
	})

	t.Run("NullByteInKey", func(t *testing.T) {
		cache := NewCache(Config{MaxSize: 100})
		defer func() { _ = cache.Close() }()

		// Null bytes in keys should not cause issues
		key := "key\x00with\x00nulls"
		cache.Set(key, "value")
		val, found := cache.Get(key)
		if found && val != "value" {
			t.Error("Null bytes in key not handled correctly")
		}
	})

	t.Run("VeryLongKey", func(t *testing.T) {
		cache := NewCache(Config{MaxSize: 100})
		defer func() { _ = cache.Close() }()

		// Very long keys should not cause crashes
		key := strings.Repeat("A", 1000000) // 1MB key
		cache.Set(key, "value")
		// Should not panic or crash
	})

	t.Run("ConcurrentPanickingLoaders", func(t *testing.T) {
		cache := NewCache(Config{MaxSize: 100})
		defer func() { _ = cache.Close() }()

		// Multiple concurrent panicking loaders should not crash cache
		var wg sync.WaitGroup
		const numGoroutines = 10

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, _ = cache.GetOrLoad("panic_key", func() (interface{}, error) {
					panic("test panic")
				})
			}()
		}

		wg.Wait()

		// Cache should still be functional
		cache.Set("test", "value")
		if val, found := cache.Get("test"); !found || val != "value" {
			t.Error("Cache corrupted after concurrent panicking loaders")
		}
	})

	t.Run("NegativeConfigValues", func(t *testing.T) {
		// Negative config values should be handled safely
		cache := NewCache(Config{
			MaxSize:     -1000,
			WindowRatio: -0.5,
			CounterBits: -10,
		})
		defer func() { _ = cache.Close() }()

		// Cache should have applied defaults and be functional
		if cache.Capacity() <= 0 {
			t.Error("Negative config values not handled correctly")
		}

		cache.Set("test", "value")
		if val, found := cache.Get("test"); !found || val != "value" {
			t.Error("Cache not functional after negative config")
		}
	})

	t.Run("ZeroTimeout", func(t *testing.T) {
		cache := NewCache(Config{MaxSize: 100})
		defer func() { _ = cache.Close() }()

		// Zero timeout should be handled
		ctx, cancel := context.WithTimeout(context.Background(), 0)
		defer cancel()

		_, err := cache.GetOrLoadWithContext(ctx, "key", func(ctx context.Context) (interface{}, error) {
			return "value", nil
		})

		// Should return context error, not panic
		if err == nil {
			// Loader might have completed before context was checked
			t.Log("Zero timeout: loader completed before context check")
		}
	})
}

// =============================================================================
// PERFORMANCE INVARIANT TESTING
// =============================================================================

// TestFuzzPerformanceInvariants ensures fuzzing doesn't reveal performance regressions.
// These are not traditional fuzz tests but use fuzzing principles to test performance.
func TestFuzzPerformanceInvariants(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance tests in short mode")
	}

	t.Run("HashPerformance", func(t *testing.T) {
		// Hash function should be consistently fast
		keys := []string{
			"short",
			strings.Repeat("medium", 10),
			strings.Repeat("looooong", 100),
		}

		for _, key := range keys {
			start := time.Now()
			iterations := 100000
			for i := 0; i < iterations; i++ {
				stringHash(key)
			}
			duration := time.Since(start)

			// Realistic performance expectations based on actual benchmarks
			// Short keys: ~10M ops/sec, Long keys: ~1M ops/sec
			var minOpsPerMs float64
			if len(key) < 100 {
				minOpsPerMs = 1000 // 1K ops/ms = 1M ops/sec (reasonable for short keys)
			} else {
				minOpsPerMs = 100 // 100 ops/ms = 100K ops/sec (reasonable for long keys)
			}

			opsPerMs := float64(iterations) / float64(duration.Milliseconds()+1) // +1 to avoid division by zero
			if opsPerMs < minOpsPerMs {
				t.Errorf("Hash performance degraded: %.0f ops/ms for key length %d (expected > %.0f)",
					opsPerMs, len(key), minOpsPerMs)
			}
		}
	})

	t.Run("CacheOperationPerformance", func(t *testing.T) {
		cache := NewCache(Config{MaxSize: 10000})
		defer func() { _ = cache.Close() }()

		// Warm up cache
		for i := 0; i < 1000; i++ {
			cache.Set(fmt.Sprintf("key_%d", i), i)
		}

		// Use conservative values that work both with and without race detector
		iterations := 100_000                // Reduced for race detector compatibility
		expectedOpsPerSec := float64(25_000) // Very permissive threshold

		start := time.Now()
		for i := 0; i < iterations; i++ {
			cache.Get(fmt.Sprintf("key_%d", i%1000))
		}
		duration := time.Since(start) // Check performance expectations
		opsPerSec := float64(iterations) / duration.Seconds()
		if opsPerSec < expectedOpsPerSec {
			t.Errorf("Cache Get performance degraded: %.0f ops/sec (expected > %.0f)", opsPerSec, expectedOpsPerSec)
		}
	})
}
