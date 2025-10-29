// loading_context_timeout_test.go: tests for GetOrLoadWithContext timeouts
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGILira fragment
// SPDX-License-Identifier: MPL-2.0

package balios

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestGetOrLoadWithContext_WaitersReturnFast tests the CRITICAL issue:
// When multiple goroutines wait for the same key loading and their contexts timeout,
// they should return IMMEDIATELY with context error, NOT wait for loader completion.
//
// PROBLEM (before fix): Waiting goroutines call `flight.wg.Wait()` which blocks
// until loader completes, IGNORING the context timeout.
//
// EXPECTED BEHAVIOR: Waiters should respect their context deadline and return quickly.
func TestGetOrLoadWithContext_WaitersReturnFast(t *testing.T) {
	cache := NewCache(Config{
		MaxSize: 100,
	})
	defer cache.Clear()

	const (
		loaderDuration  = 500 * time.Millisecond // Slow loader
		waiterTimeout   = 20 * time.Millisecond  // Short timeout for waiters
		numWaiters      = 5
		maxWaitDuration = 100 * time.Millisecond // Waiters should return within this time
	)

	// Slow loader that ignores context (e.g., legacy blocking I/O)
	loaderStarted := make(chan struct{})
	loader := func(ctx context.Context) (interface{}, error) {
		close(loaderStarted)
		time.Sleep(loaderDuration) // Blocks regardless of context
		return "value", nil
	}

	// Start the first goroutine that actually loads
	go func() {
		ctx := context.Background() // No timeout for the actual loader
		_, err := cache.GetOrLoadWithContext(ctx, "shared-key", loader)
		if err != nil {
			t.Errorf("Unexpected error in loader: %v", err)
		}
	}()

	// Wait for loader to start
	<-loaderStarted

	// Now spawn waiters with SHORT timeout
	// CRITICAL: They should return FAST, not wait for loader
	waitersCompleted := make(chan time.Duration, numWaiters)

	for i := 0; i < numWaiters; i++ {
		go func(id int) {
			ctx, cancel := context.WithTimeout(context.Background(), waiterTimeout)
			defer cancel()

			start := time.Now()
			_, err := cache.GetOrLoadWithContext(ctx, "shared-key", loader)
			duration := time.Since(start)

			if err != context.DeadlineExceeded {
				t.Errorf("Waiter %d: expected context.DeadlineExceeded, got: %v", id, err)
			}

			waitersCompleted <- duration
		}(i)
	}

	// Collect results
	var maxWaiterDuration time.Duration
	for i := 0; i < numWaiters; i++ {
		duration := <-waitersCompleted
		if duration > maxWaiterDuration {
			maxWaiterDuration = duration
		}
		t.Logf("Waiter %d completed in: %v", i+1, duration)
	}

	// CRITICAL ASSERTION: Waiters should return FAST with context error
	// NOT wait for the full loader duration (500ms)
	if maxWaiterDuration > maxWaitDuration {
		t.Errorf("FAIL: Waiters took too long: %v (max allowed: %v, loader duration: %v)",
			maxWaiterDuration, maxWaitDuration, loaderDuration)
		t.Errorf("This indicates waiters are blocking on wg.Wait() instead of respecting context timeout")
	} else {
		t.Logf("SUCCESS: All waiters returned in < %v (max was %v)", maxWaitDuration, maxWaiterDuration)
	}
}

// TestGetOrLoadWithContext_FirstCallerIgnoresWaiterContexts tests that
// the first caller (loader) should complete even if waiters timeout.
func TestGetOrLoadWithContext_FirstCallerIgnoresWaiterContexts(t *testing.T) {
	cache := NewCache(Config{
		MaxSize: 100,
	})
	defer cache.Clear()

	const (
		loaderDuration = 100 * time.Millisecond
		waiterTimeout  = 10 * time.Millisecond
	)

	loaderCompleted := false
	loader := func(ctx context.Context) (interface{}, error) {
		time.Sleep(loaderDuration)
		loaderCompleted = true
		return "value", nil
	}

	// First caller with long timeout
	firstCaller := make(chan struct{})
	go func() {
		ctx := context.Background()
		val, err := cache.GetOrLoadWithContext(ctx, "key", loader)
		if err != nil {
			t.Errorf("First caller failed: %v", err)
		}
		if val != "value" {
			t.Errorf("First caller got wrong value: %v", val)
		}
		close(firstCaller)
	}()

	// Give first caller time to start
	time.Sleep(5 * time.Millisecond)

	// Waiters with short timeout
	waitersDone := make(chan struct{}, 3)
	for i := 0; i < 3; i++ {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), waiterTimeout)
			defer cancel()
			_, err := cache.GetOrLoadWithContext(ctx, "key", loader)
			if err != nil && !errors.Is(err, context.DeadlineExceeded) {
				t.Errorf("Unexpected error in waiter: %v", err)
			}
			waitersDone <- struct{}{}
		}()
	}

	// Wait for waiters to timeout
	for i := 0; i < 3; i++ {
		<-waitersDone
	}

	// Wait for first caller to complete
	<-firstCaller

	if !loaderCompleted {
		t.Error("Loader should have completed despite waiter timeouts")
	}
}

// TestGetOrLoadWithContext_PreCanceledContext ensures no work is done
// if context is already canceled before the call.
func TestGetOrLoadWithContext_PreCanceledContext(t *testing.T) {
	cache := NewCache(Config{
		MaxSize: 100,
	})
	defer cache.Clear()

	loaderCalled := false
	loader := func(ctx context.Context) (interface{}, error) {
		loaderCalled = true
		return "value", nil
	}

	// Create already-canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	_, err := cache.GetOrLoadWithContext(ctx, "key", loader)
	duration := time.Since(start)

	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got: %v", err)
	}

	if loaderCalled {
		t.Error("Loader should not be called when context is pre-canceled")
	}

	// Should return immediately
	if duration > 10*time.Millisecond {
		t.Errorf("Pre-canceled context took too long: %v", duration)
	}
}
