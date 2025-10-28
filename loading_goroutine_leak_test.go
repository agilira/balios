// loading_goroutine_leak_test.go: tests for goroutine leak in GetOrLoadWithContext
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGILira library
// SPDX-License-Identifier: MPL-2.0

package balios

import (
	"context"
	"fmt"
	"runtime"
	"testing"
	"time"
)

// TestGetOrLoadWithContext_NoGoroutineLeak verifies that context cancellation
// doesn't leave goroutines running in the background.
// This is a CRITICAL test for production stability under high load.
//
// Scenario: Multiple requests for SAME key with short context timeout
// Expected: Waiters return quickly with context error, no goroutine leak
// Actual (before fix): Goroutines blocked on wg.Wait() until loader completes
func TestGetOrLoadWithContext_NoGoroutineLeak(t *testing.T) {
	cache := NewCache(Config{
		MaxSize: 100,
	})
	defer cache.Clear()

	// Measure baseline goroutines
	runtime.GC()
	time.Sleep(50 * time.Millisecond) // Let GC settle
	baseline := runtime.NumGoroutine()

	t.Logf("Baseline goroutines: %d", baseline)

	// Test parameters
	const (
		numWaiters     = 50 // Number of waiters on SAME key
		contextTimeout = 10 * time.Millisecond
		loaderDuration = 300 * time.Millisecond // Loader takes much longer
	)

	// Slow loader that IGNORES context (simulates blocking I/O that can't be interrupted)
	loaderStarted := make(chan struct{})
	slowLoader := func(ctx context.Context) (interface{}, error) {
		// Only first call signals
		select {
		case <-loaderStarted:
		default:
			close(loaderStarted)
		}
		// Simulate uninterruptible blocking operation (e.g., legacy database call)
		time.Sleep(loaderDuration)
		return "value", nil
	}

	// Start first loader (will actually execute)
	go func() {
		ctx := context.Background()
		_, err := cache.GetOrLoadWithContext(ctx, "shared-key", slowLoader)
		if err != nil {
			t.Errorf("Unexpected error in loader: %v", err)
		}
	}()

	// Wait for loader to start
	<-loaderStarted
	time.Sleep(5 * time.Millisecond)

	// Now spawn many waiters with SHORT timeout
	done := make(chan struct{})
	for i := 0; i < numWaiters; i++ {
		go func(iteration int) {
			defer func() { done <- struct{}{} }()

			// Each waiter has a very short context
			ctx, cancel := context.WithTimeout(context.Background(), contextTimeout)
			defer cancel()

			// This should timeout quickly
			_, err := cache.GetOrLoadWithContext(ctx, "shared-key", slowLoader)

			// We expect context.DeadlineExceeded
			if err != context.DeadlineExceeded {
				t.Errorf("Waiter %d: expected context.DeadlineExceeded, got: %v", iteration, err)
			}
		}(i)
	}

	// Wait for all waiters to timeout
	for i := 0; i < numWaiters; i++ {
		<-done
	}

	// Check goroutine count BEFORE loader completes
	time.Sleep(50 * time.Millisecond)
	runtime.GC()
	duringCount := runtime.NumGoroutine()
	t.Logf("Goroutines while loader still running: %d", duringCount)

	// Expected: baseline + 1 (the loader) + test goroutine + minimal variance
	// Without fix: baseline + 50+ (zombie goroutines from each waiter)
	maxExpected := baseline + 5
	if duringCount > maxExpected+10 {
		t.Errorf("GOROUTINE LEAK DETECTED: %d goroutines (expected ≤ %d)",
			duringCount, maxExpected+10)
	}

	// Wait for loader to complete
	time.Sleep(loaderDuration)

	// Final cleanup check
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	final := runtime.NumGoroutine()
	t.Logf("Final goroutines: %d", final)

	if final > baseline+5 {
		t.Errorf("Goroutines not cleaned up: baseline=%d, final=%d", baseline, final)
	}
}

// TestGetOrLoadWithContext_MultipleWaiters_NoLeakOnTimeout verifies that
// when multiple goroutines are waiting for the same key and context times out,
// no goroutines are leaked from the waiting goroutines.
func TestGetOrLoadWithContext_MultipleWaiters_NoLeakOnTimeout(t *testing.T) {
	cache := NewCache(Config{
		MaxSize: 100,
	})
	defer cache.Clear()

	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	baseline := runtime.NumGoroutine()

	t.Logf("Baseline goroutines: %d", baseline)

	const (
		numWaiters     = 50 // Many goroutines waiting on same key
		contextTimeout = 5 * time.Millisecond
		loaderDuration = 200 * time.Millisecond
	)

	// One slow loader that IGNORES context (the problematic case)
	loaderStarted := make(chan struct{})
	slowLoader := func(ctx context.Context) (interface{}, error) {
		close(loaderStarted)
		// Simulate uninterruptible operation
		time.Sleep(loaderDuration)
		return "value", nil
	}

	// First goroutine starts the loader
	go func() {
		ctx := context.Background()
		_, err := cache.GetOrLoadWithContext(ctx, "shared-key", slowLoader)
		if err != nil {
			t.Errorf("Unexpected error in loader: %v", err)
		}
	}()

	// Wait for loader to start
	<-loaderStarted

	// Now spawn many waiters that will timeout
	done := make(chan struct{})
	for i := 0; i < numWaiters; i++ {
		go func() {
			defer func() { done <- struct{}{} }()

			ctx, cancel := context.WithTimeout(context.Background(), contextTimeout)
			defer cancel()

			_, err := cache.GetOrLoadWithContext(ctx, "shared-key", slowLoader)
			if err != context.DeadlineExceeded {
				t.Errorf("Expected context.DeadlineExceeded, got: %v", err)
			}
		}()
	}

	// Wait for all waiters
	for i := 0; i < numWaiters; i++ {
		<-done
	}

	// Wait for the first loader to finish
	time.Sleep(loaderDuration + 50*time.Millisecond)

	// Cleanup
	runtime.GC()
	time.Sleep(50 * time.Millisecond)

	final := runtime.NumGoroutine()
	t.Logf("Final goroutines: %d", final)

	maxAllowedIncrease := 10
	leaked := final - baseline

	if leaked > maxAllowedIncrease {
		t.Errorf("GOROUTINE LEAK DETECTED (multi-waiter): baseline=%d, final=%d, leaked=%d",
			baseline, final, leaked)
	}
}

// TestGetOrLoadWithContext_ContextCanceledBeforeCall ensures that if context
// is already canceled when GetOrLoadWithContext is called, no goroutines are created.
func TestGetOrLoadWithContext_ContextCanceledBeforeCall(t *testing.T) {
	cache := NewCache(Config{
		MaxSize: 100,
	})
	defer cache.Clear()

	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	baseline := runtime.NumGoroutine()

	// Create an already-canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	loaderCalled := false
	loader := func(ctx context.Context) (interface{}, error) {
		loaderCalled = true
		return "value", nil
	}

	// This should return immediately without calling loader or creating goroutines
	_, err := cache.GetOrLoadWithContext(ctx, "test-key", loader)
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got: %v", err)
	}

	if loaderCalled {
		t.Error("Loader should not be called when context is already canceled")
	}

	time.Sleep(50 * time.Millisecond)
	runtime.GC()
	time.Sleep(50 * time.Millisecond)

	final := runtime.NumGoroutine()
	if final > baseline+2 { // Allow minimal variance
		t.Errorf("Goroutines created despite canceled context: baseline=%d, final=%d",
			baseline, final)
	}
}

// BenchmarkGetOrLoadWithContext_ContextTimeout benchmarks the performance
// of context timeout path to ensure no performance regression.
func BenchmarkGetOrLoadWithContext_ContextTimeout(b *testing.B) {
	cache := NewCache(Config{
		MaxSize: 10000,
	})
	defer cache.Clear()

	slowLoader := func(ctx context.Context) (interface{}, error) {
		<-time.After(100 * time.Millisecond)
		return "value", nil
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		_, _ = cache.GetOrLoadWithContext(ctx, fmt.Sprintf("key-%d", i), slowLoader)
		cancel()
	}
}
