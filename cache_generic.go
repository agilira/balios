// cache_generic.go: type-safe generic cache API
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGILira library
// SPDX-License-Identifier: MPL-2.0

package balios

import (
	"fmt"
	"strconv"
)

// GenericCache provides a type-safe cache interface using Go generics.
// K must be comparable (can be used as map key).
// V can be any type.
//
// Example:
//
//	cache := balios.NewGenericCache[string, User](balios.Config{
//	    MaxSize: 10_000,
//	    TTL:     time.Hour,
//	})
//	cache.Set("user:123", user)
//	if value, found := cache.Get("user:123"); found {
//	    fmt.Printf("User: %+v\n", value)
//	}
type GenericCache[K comparable, V any] struct {
	inner Cache // Wraps existing cache implementation
}

// NewGenericCache creates a new type-safe generic cache.
//
// Parameters:
//   - cfg: Cache configuration (MaxSize, TTL, WindowRatio, etc.)
//
// Returns a new GenericCache instance.
func NewGenericCache[K comparable, V any](cfg Config) *GenericCache[K, V] {
	return &GenericCache[K, V]{
		inner: NewCache(cfg),
	}
}

// Set stores a key-value pair in the cache.
// The value will be stored until evicted or expired (if TTL is set).
//
// Parameters:
//   - key: The key to store (must be comparable)
//   - value: The value to store (can be any type)
func (c *GenericCache[K, V]) Set(key K, value V) {
	// Fast path: convert key to string with zero allocations for common types
	keyStr := keyToString(key)
	c.inner.Set(keyStr, value)
}

// Get retrieves a value from the cache.
//
// Parameters:
//   - key: The key to retrieve
//
// Returns:
//   - value: The stored value (zero value if not found)
//   - found: true if key exists and is not expired
func (c *GenericCache[K, V]) Get(key K) (value V, found bool) {
	keyStr := keyToString(key)
	val, found := c.inner.Get(keyStr)
	if !found {
		var zero V
		return zero, false
	}

	// Type assertion - safe because we control what goes in
	typedValue, ok := val.(V)
	if !ok {
		// This should never happen if cache is used correctly
		var zero V
		return zero, false
	}

	return typedValue, true
}

// Delete removes a key from the cache.
//
// Parameters:
//   - key: The key to remove
func (c *GenericCache[K, V]) Delete(key K) {
	keyStr := keyToString(key)
	c.inner.Delete(keyStr)
}

// Has checks if a key exists in the cache without retrieving it.
// This is more efficient than Get when you only need to check existence.
//
// Parameters:
//   - key: The key to check
//
// Returns true if key exists and is not expired.
func (c *GenericCache[K, V]) Has(key K) bool {
	keyStr := keyToString(key)
	return c.inner.Has(keyStr)
}

// keyToString converts a key of any comparable type to string efficiently.
// Uses type switch to avoid allocations for common types (string, int, uint).
// Falls back to fmt.Sprintf for other types.
func keyToString[K comparable](key K) string {
	// Type assertion to interface{} to enable type switch
	switch v := any(key).(type) {
	case string:
		// Zero allocation for string keys
		return v
	case int:
		return strconv.Itoa(v)
	case int8:
		return strconv.FormatInt(int64(v), 10)
	case int16:
		return strconv.FormatInt(int64(v), 10)
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case int64:
		return strconv.FormatInt(v, 10)
	case uint:
		return strconv.FormatUint(uint64(v), 10)
	case uint8:
		return strconv.FormatUint(uint64(v), 10)
	case uint16:
		return strconv.FormatUint(uint64(v), 10)
	case uint32:
		return strconv.FormatUint(uint64(v), 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	default:
		// Fallback to fmt.Sprintf for other types (structs, arrays, etc.)
		// This allocates but is only used for uncommon key types
		return fmt.Sprintf("%v", key)
	}
}

// Clear removes all entries from the cache and resets statistics.
func (c *GenericCache[K, V]) Clear() {
	c.inner.Clear()
}

// Stats returns current cache statistics.
//
// Returns CacheStats containing:
//   - Hits: Number of successful Get operations
//   - Misses: Number of failed Get operations
//   - ItemCount: Current number of items in cache
//   - Evictions: Number of items evicted
func (c *GenericCache[K, V]) Stats() CacheStats {
	return c.inner.Stats()
}

// Close cleans up cache resources and stops background goroutines.
// After calling Close, the cache should not be used.
func (c *GenericCache[K, V]) Close() {
	c.inner.Close()
}
