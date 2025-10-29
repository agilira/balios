// loading_generic.go: type-safe GetOrLoad implementation with generics
//
// This file provides generic versions of GetOrLoad and GetOrLoadWithContext,
// enabling type-safe cache-aside pattern without type assertions.
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGILira fragment
// SPDX-License-Identifier: MPL-2.0
package balios

import "context"

// GetOrLoad is the generic version of Cache.GetOrLoad.
// Returns the value from cache, or loads it using the provided loader function.
//
// Type Parameters:
//   - K: Key type (must be comparable)
//   - V: Value type (any type)
//
// Parameters:
//   - key: The cache key to lookup or load
//   - loader: Function to load the value if not in cache
//
// Returns:
//   - value: The cached or loaded value (zero value on error)
//   - error: Loader error or validation error
//
// Example:
//
//	cache := NewGenericCache[int, string](Config{MaxSize: 100})
//	value, err := cache.GetOrLoad(42, func() (string, error) {
//	    return fetchFromDB(42)
//	})
func (c *GenericCache[K, V]) GetOrLoad(key K, loader func() (V, error)) (V, error) {
	var zero V

	// Convert key to string
	keyStr := keyToString(key)

	// Wrap generic loader to match interface{} signature
	wrappedLoader := func() (interface{}, error) {
		return loader()
	}

	// Call underlying cache
	result, err := c.inner.GetOrLoad(keyStr, wrappedLoader)
	if err != nil {
		return zero, err
	}

	// Type assert result
	value, ok := result.(V)
	if !ok {
		// This should never happen if used correctly
		return zero, NewErrInternal("GetOrLoad", nil)
	}

	return value, nil
}

// GetOrLoadWithContext is the generic version of Cache.GetOrLoadWithContext.
// Like GetOrLoad but respects context cancellation and timeout.
//
// Type Parameters:
//   - K: Key type (must be comparable)
//   - V: Value type (any type)
//
// Parameters:
//   - ctx: Context for cancellation and timeout control
//   - key: The cache key to lookup or load
//   - loader: Function to load the value if not in cache. Receives the context.
//
// Returns:
//   - value: The cached or loaded value (zero value on error)
//   - error: Context error, loader error, or validation error
//
// Example:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
//	defer cancel()
//	value, err := cache.GetOrLoadWithContext(ctx, 42, func(ctx context.Context) (string, error) {
//	    return fetchFromDBWithContext(ctx, 42)
//	})
func (c *GenericCache[K, V]) GetOrLoadWithContext(ctx context.Context, key K, loader func(context.Context) (V, error)) (V, error) {
	var zero V

	// Convert key to string
	keyStr := keyToString(key)

	// Wrap generic loader to match interface{} signature
	wrappedLoader := func(ctx context.Context) (interface{}, error) {
		return loader(ctx)
	}

	// Call underlying cache
	result, err := c.inner.GetOrLoadWithContext(ctx, keyStr, wrappedLoader)
	if err != nil {
		return zero, err
	}

	// Type assert result
	value, ok := result.(V)
	if !ok {
		// This should never happen if used correctly
		return zero, NewErrInternal("GetOrLoadWithContext", nil)
	}

	return value, nil
}
