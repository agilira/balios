// loading.go: GetOrLoad implementation with singleflight pattern
//
// This file implements the GetOrLoad and GetOrLoadWithContext methods,
// providing cache-aside pattern with automatic deduplication of concurrent
// loads using a singleflight mechanism.
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGILira library
// SPDX-License-Identifier: MPL-2.0
package balios

import (
	"context"
	"sync"
	"sync/atomic"
)

// inflightCall represents an in-flight loader call with its waitgroup and result.
// Uses atomic.Value for race-free access to val and err fields.
// Note: atomic.Value cannot store nil, so we use wrapper types.
type inflightCall struct {
	wg  sync.WaitGroup
	val atomic.Value // stores *resultWrapper
	err atomic.Value // stores *errorWrapper
}

// resultWrapper wraps a value to allow storing nil in atomic.Value
type resultWrapper struct {
	value interface{}
}

// errorWrapper wraps an error to allow storing nil in atomic.Value
type errorWrapper struct {
	err error
}

// inflight tracks in-flight loader calls for singleflight pattern
// Uses sync.Map for race-free concurrent access without global lock
var inflight sync.Map

// GetOrLoad returns the value from cache, or loads it using the provided loader function.
// If multiple goroutines call GetOrLoad for the same missing key concurrently,
// only one loader will be executed (singleflight pattern to prevent cache stampede).
//
// The loaded value is cached with the cache's default TTL.
// If the loader returns an error, the error is NOT cached.
//
// Parameters:
//   - key: The cache key to lookup or load
//   - loader: Function to load the value if not in cache. Must not be nil.
//
// Returns:
//   - value: The cached or loaded value
//   - error: BALIOS_INVALID_LOADER if loader is nil,
//     BALIOS_PANIC_RECOVERED if loader panics,
//     or the error returned by the loader
//
// Performance:
//   - Cache hit: ~110ns (same as Get)
//   - Cache miss: loader execution time + ~50ns overhead
//   - Concurrent misses: Only one loader execution (singleflight win!)
//
// Example:
//
//	value, err := cache.GetOrLoad("user:123", func() (interface{}, error) {
//	    return fetchUserFromDB(123)
//	})
func (c *wtinyLFUCache) GetOrLoad(key string, loader func() (interface{}, error)) (interface{}, error) {
	// Fast path: check cache first
	if value, found := c.Get(key); found {
		return value, nil
	}

	// Validate loader
	if loader == nil {
		return nil, NewErrInvalidLoader(key)
	}

	// Singleflight: check if another goroutine is already loading this key
	callKey := "load:" + key

	// Create and initialize flight BEFORE putting it in map
	newFlight := &inflightCall{}
	newFlight.wg.Add(1) // Initialize WaitGroup before any other goroutine can see it

	actual, loaded := inflight.LoadOrStore(callKey, newFlight)
	flight := actual.(*inflightCall)

	if loaded {
		// Another goroutine is loading, wait for result
		// The WaitGroup was already initialized by the first goroutine
		flight.wg.Wait()
		valWrapper, _ := flight.val.Load().(*resultWrapper)
		errWrapper, _ := flight.err.Load().(*errorWrapper)
		if valWrapper != nil && errWrapper != nil {
			return valWrapper.value, errWrapper.err
		}
		return nil, nil // Should never happen
	}

	// We are the first (we inserted newFlight), execute the loader
	defer func() {
		flight.wg.Done()
		inflight.Delete(callKey)
	}()

	// Execute loader with panic recovery
	var loaderVal interface{}
	var loaderErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				loaderErr = NewErrPanicRecovered("GetOrLoad:"+key, r)
			}
		}()
		loaderVal, loaderErr = loader()
	}()

	// Store results atomically using wrappers
	flight.val.Store(&resultWrapper{value: loaderVal})
	flight.err.Store(&errorWrapper{err: loaderErr})

	// If successful, cache the value
	if loaderErr == nil && loaderVal != nil {
		c.Set(key, loaderVal)
	}

	return loaderVal, loaderErr
}

// GetOrLoadWithContext is like GetOrLoad but respects context cancellation and timeout.
// The context is passed to the loader function, allowing it to cancel long-running operations.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control
//   - key: The cache key to lookup or load
//   - loader: Function to load the value if not in cache. Receives the context.
//
// Returns:
//   - value: The cached or loaded value
//   - error: Context error (Canceled, DeadlineExceeded), loader error, or validation error
//
// Example:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
//	defer cancel()
//	value, err := cache.GetOrLoadWithContext(ctx, "user:123", func(ctx context.Context) (interface{}, error) {
//	    return fetchUserFromDBWithContext(ctx, 123)
//	})
func (c *wtinyLFUCache) GetOrLoadWithContext(ctx context.Context, key string, loader func(context.Context) (interface{}, error)) (interface{}, error) {
	// Fast path: check cache first (no context needed for cache hit)
	if value, found := c.Get(key); found {
		return value, nil
	}

	// Validate loader
	if loader == nil {
		return nil, NewErrInvalidLoader(key)
	}

	// Check context before starting
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Singleflight with context awareness
	callKey := "load:" + key

	// Create and initialize flight BEFORE putting it in map
	newFlight := &inflightCall{}
	newFlight.wg.Add(1) // Initialize WaitGroup before any other goroutine can see it

	actual, loaded := inflight.LoadOrStore(callKey, newFlight)
	flight := actual.(*inflightCall)

	if loaded {
		// Another goroutine is loading, wait with context
		// The WaitGroup was already initialized by the first goroutine
		done := make(chan struct{})
		go func() {
			flight.wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			valWrapper, _ := flight.val.Load().(*resultWrapper)
			errWrapper, _ := flight.err.Load().(*errorWrapper)
			if valWrapper != nil && errWrapper != nil {
				return valWrapper.value, errWrapper.err
			}
			return nil, nil // Should never happen
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	// We are the first (we inserted newFlight), execute the loader
	defer func() {
		flight.wg.Done()
		inflight.Delete(callKey)
	}()

	// Execute loader with panic recovery and context
	var loaderVal interface{}
	var loaderErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				loaderErr = NewErrPanicRecovered("GetOrLoadWithContext:"+key, r)
			}
		}()
		loaderVal, loaderErr = loader(ctx)
	}()

	// Store results atomically using wrappers
	flight.val.Store(&resultWrapper{value: loaderVal})
	flight.err.Store(&errorWrapper{err: loaderErr})

	// If successful, cache the value
	if loaderErr == nil && loaderVal != nil {
		c.Set(key, loaderVal)
	}

	return loaderVal, loaderErr
}
