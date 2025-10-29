// loading_zombie_goroutines_test.go: tests for "zombie" goroutines" in Balios
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGILira fragment
// SPDX-License-Identifier: MPL-2.0

package balios

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestGetOrLoadWithContext_ZombieGoroutines tests for "zombie" goroutines:
// goroutines that continue running after the caller has returned due to context timeout.
//
// PROBLEM: When a waiter times out via context.Done(), it returns immediately,
// BUT the goroutine that was created to wait on flight.wg.Wait() continues running
// until the loader completes. Under high load with many timeouts, this creates
// thousands of unnecessary goroutines that consume resources.
//
// This test measures goroutine count DURING execution to detect these zombies.
func TestGetOrLoadWithContext_ZombieGoroutines(t *testing.T) {
	cache := NewCache(Config{
		MaxSize: 100,
	})
	defer cache.Clear()

	const (
		loaderDuration = 500 * time.Millisecond
		waiterTimeout  = 20 * time.Millisecond
		numWaiters     = 50
	)

	// Measure baseline
	runtime.GC()
	time.Sleep(20 * time.Millisecond)
	baseline := runtime.NumGoroutine()
	t.Logf("Baseline goroutines: %d", baseline)

	// Slow loader
	loaderStarted := make(chan struct{})
	loader := func(ctx context.Context) (interface{}, error) {
		close(loaderStarted)
		time.Sleep(loaderDuration)
		return "value", nil
	}

	// Start loader
	go func() {
		_, _ = cache.GetOrLoadWithContext(context.Background(), "key", loader)
	}()
	<-loaderStarted

	// Spawn many waiters that will timeout
	waitersDone := make(chan struct{}, numWaiters)
	for i := 0; i < numWaiters; i++ {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), waiterTimeout)
			defer cancel()
			_, _ = cache.GetOrLoadWithContext(ctx, "key", loader)
			waitersDone <- struct{}{}
		}()
	}

	// Wait for all waiters to timeout and return
	for i := 0; i < numWaiters; i++ {
		<-waitersDone
	}

	// ALL waiters have returned, but check goroutine count
	// BEFORE the loader completes
	time.Sleep(50 * time.Millisecond)
	runtime.GC()

	duringCount := runtime.NumGoroutine()
	t.Logf("Goroutines AFTER waiters returned but BEFORE loader completes: %d", duringCount)

	// Expected: baseline + 1 (the loader) + this test goroutine + a few runtime goroutines
	// Actual (BEFORE fix): baseline + numWaiters (50 zombie goroutines still waiting)

	expectedMax := baseline + 5 // loader + test + runtime variance
	if duringCount > expectedMax+numWaiters/2 {
		t.Errorf("ZOMBIE GOROUTINES DETECTED: %d goroutines still running (expected ≤ %d)",
			duringCount, expectedMax)
		t.Errorf("This likely means %d goroutines are stuck in flight.wg.Wait()",
			duringCount-expectedMax)

		// Print stack traces to confirm
		buf := make([]byte, 1<<18) // 256KB
		stackSize := runtime.Stack(buf, true)
		stacks := string(buf[:stackSize])

		// Count goroutines waiting on WaitGroup
		waitingCount := strings.Count(stacks, "flight.wg.Wait")
		t.Logf("Goroutines in flight.wg.Wait(): %d", waitingCount)

		if waitingCount > 5 {
			t.Errorf("Found %d goroutines stuck in flight.wg.Wait() after callers returned", waitingCount)
		}
	}

	// Wait for loader to complete
	time.Sleep(loaderDuration)

	// Now check again - should be back to baseline
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	finalCount := runtime.NumGoroutine()
	t.Logf("Final goroutines (after loader completes): %d", finalCount)
}

// TestGetOrLoadWithContext_MassiveTimeouts simulates production load:
// 1000s of requests timing out while waiting for slow loader.
// This is the realistic scenario that causes resource exhaustion.
func TestGetOrLoadWithContext_MassiveTimeouts(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	cache := NewCache(Config{
		MaxSize: 1000,
	})
	defer cache.Clear()

	const (
		loaderDuration = 2 * time.Second
		waiterTimeout  = 10 * time.Millisecond
		numWaiters     = 1000 // Simulate high load
	)

	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	baseline := runtime.NumGoroutine()
	t.Logf("Baseline: %d goroutines", baseline)

	loader := func(ctx context.Context) (interface{}, error) {
		time.Sleep(loaderDuration)
		return "value", nil
	}

	// Start first loader
	go func() {
		_, _ = cache.GetOrLoadWithContext(context.Background(), "heavy-key", loader)
	}()
	time.Sleep(20 * time.Millisecond)

	// Measure memory before
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	// Spawn many waiters
	start := time.Now()
	done := make(chan struct{}, numWaiters)
	for i := 0; i < numWaiters; i++ {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), waiterTimeout)
			defer cancel()
			_, _ = cache.GetOrLoadWithContext(ctx, "heavy-key", loader)
			done <- struct{}{}
		}()
	}

	// Wait for all to timeout
	for i := 0; i < numWaiters; i++ {
		<-done
	}
	elapsed := time.Since(start)

	t.Logf("All %d waiters timed out in: %v", numWaiters, elapsed)

	// Check goroutine count while loader is still running
	time.Sleep(100 * time.Millisecond)
	runtime.GC()
	duringCount := runtime.NumGoroutine()

	// Measure memory after
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)

	allocatedMB := float64(m2.Alloc-m1.Alloc) / 1024 / 1024

	t.Logf("Goroutines during load: %d (increase: %d)", duringCount, duringCount-baseline)
	t.Logf("Memory allocated: %.2f MB", allocatedMB)

	// With fix: should have minimal goroutine increase (just the loader)
	// Without fix: would have 1000+ zombie goroutines
	maxExpected := baseline + 20 // Allow some variance
	if duringCount > maxExpected+100 {
		t.Errorf("RESOURCE LEAK under load: %d goroutines (expected < %d)",
			duringCount, maxExpected+100)
		t.Errorf("Under production load, this would cause OOM!")
	}

	// Wait for loader
	time.Sleep(loaderDuration + 100*time.Millisecond)

	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	finalCount := runtime.NumGoroutine()
	t.Logf("Final: %d goroutines", finalCount)
}
