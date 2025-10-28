// example_test.go: godoc examples for Balios cache
//
// These examples appear in the generated documentation on pkg.go.dev
// and are executed as part of the test suite to ensure they remain valid.
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGILira library
// SPDX-License-Identifier: MPL-2.0

package balios_test

import (
	"context"
	"fmt"
	"time"

	"github.com/agilira/balios"
)

// ExampleNewCache demonstrates basic cache creation and usage.
func ExampleNewCache() {
	// Create a cache with default configuration
	cache := balios.NewCache(balios.Config{
		MaxSize: 1000,
		TTL:     time.Hour,
	})
	defer cache.Close()

	// Store a value
	cache.Set("user:123", map[string]string{
		"name":  "John Doe",
		"email": "john@example.com",
	})

	// Retrieve the value
	if _, found := cache.Get("user:123"); found {
		fmt.Println("Found user in cache")
	}

	// Output: Found user in cache
}

// ExampleNewGenericCache demonstrates type-safe generic cache usage.
func ExampleNewGenericCache() {
	// Create a type-safe cache for User structs
	type User struct {
		ID    int
		Name  string
		Email string
	}

	cache := balios.NewGenericCache[string, User](balios.Config{
		MaxSize: 1000,
		TTL:     time.Hour,
	})
	defer cache.Close()

	// Store a user (type-safe!)
	cache.Set("user:123", User{
		ID:    123,
		Name:  "John Doe",
		Email: "john@example.com",
	})

	// Retrieve the user (returns User, not interface{})
	if user, found := cache.Get("user:123"); found {
		fmt.Printf("User: %s (%s)\n", user.Name, user.Email)
	}

	// Output: User: John Doe (john@example.com)
}

// ExampleGenericCache_Set demonstrates storing values in a generic cache.
func ExampleGenericCache_Set() {
	cache := balios.NewGenericCache[string, int](balios.Config{
		MaxSize: 100,
	})
	defer cache.Close()

	// Store multiple values
	cache.Set("answer", 42)
	cache.Set("count", 1337)
	cache.Set("total", 9001)

	// Check if values exist
	if cache.Has("answer") {
		fmt.Println("Answer exists in cache")
	}

	// Output: Answer exists in cache
}

// ExampleGenericCache_Get demonstrates retrieving values from a generic cache.
func ExampleGenericCache_Get() {
	cache := balios.NewGenericCache[int, string](balios.Config{
		MaxSize: 100,
	})
	defer cache.Close()

	// Store a value with integer key
	cache.Set(404, "Not Found")
	cache.Set(200, "OK")

	// Retrieve values (type-safe)
	if message, found := cache.Get(404); found {
		fmt.Printf("Status 404: %s\n", message)
	}

	// Output: Status 404: Not Found
}

// ExampleCache_GetOrLoad demonstrates lazy loading with singleflight pattern.
func ExampleCache_GetOrLoad() {
	cache := balios.NewCache(balios.Config{
		MaxSize: 100,
		TTL:     time.Minute,
	})
	defer cache.Close()

	// Define an expensive loader function
	expensiveLoader := func() (interface{}, error) {
		// Simulate expensive database query or API call
		time.Sleep(10 * time.Millisecond)
		return "expensive result", nil
	}

	// First call: executes loader and caches result
	value, err := cache.GetOrLoad("expensive:key", expensiveLoader)
	if err == nil {
		fmt.Printf("Loaded: %s\n", value)
	}

	// Second call: returns cached value instantly (no loader execution)
	value, err = cache.GetOrLoad("expensive:key", expensiveLoader)
	if err == nil {
		fmt.Printf("Cached: %s\n", value)
	}

	// Output: Loaded: expensive result
	// Cached: expensive result
}

// ExampleCache_GetOrLoadWithContext demonstrates context-aware loading.
func ExampleCache_GetOrLoadWithContext() {
	cache := balios.NewCache(balios.Config{
		MaxSize: 100,
		TTL:     time.Minute,
	})
	defer cache.Close()

	// Define a loader that respects context cancellation
	loaderWithContext := func(ctx context.Context) (interface{}, error) {
		// Simulate work that can be cancelled
		select {
		case <-time.After(100 * time.Millisecond):
			return "result", nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Load with context (completes before timeout)
	value, err := cache.GetOrLoadWithContext(ctx, "key", loaderWithContext)
	if err == nil {
		fmt.Printf("Loaded: %s\n", value)
	}

	// Output: Loaded: result
}

// ExampleCache_Stats demonstrates monitoring cache performance.
func ExampleCache_Stats() {
	cache := balios.NewCache(balios.Config{
		MaxSize: 100,
	})
	defer cache.Close()

	// Perform some operations
	cache.Set("key1", "value1")
	cache.Set("key2", "value2")
	cache.Get("key1") // Hit
	cache.Get("key3") // Miss

	// Get statistics
	stats := cache.Stats()
	fmt.Printf("Size: %d/%d\n", stats.Size, stats.Capacity)
	fmt.Printf("Hits: %d, Misses: %d\n", stats.Hits, stats.Misses)
	fmt.Printf("Hit Ratio: %.1f%%\n", stats.HitRatio())

	// Output: Size: 2/100
	// Hits: 1, Misses: 1
	// Hit Ratio: 50.0%
}

// ExampleConfig demonstrates advanced cache configuration.
func ExampleConfig() {
	cache := balios.NewCache(balios.Config{
		MaxSize:          10_000,           // Maximum 10k entries
		TTL:              30 * time.Minute, // Entries expire after 30 minutes
		NegativeCacheTTL: 5 * time.Second,  // Cache errors for 5 seconds
		WindowRatio:      0.01,             // 1% window cache (W-TinyLFU)
		OnEvict: func(key string, value interface{}) {
			// Called when an entry is evicted
			fmt.Printf("Evicted: %s\n", key)
		},
		OnExpire: func(key string, value interface{}) {
			// Called when an entry expires
			fmt.Printf("Expired: %s\n", key)
		},
	})
	defer cache.Close()

	cache.Set("key", "value")
	// Cache is now configured and ready to use
}

// ExampleCache_ExpireNow demonstrates manual expiration of TTL entries.
func ExampleCache_ExpireNow() {
	cache := balios.NewCache(balios.Config{
		MaxSize: 100,
		TTL:     100 * time.Millisecond, // Short TTL for demonstration
	})
	defer cache.Close()

	// Add some entries
	cache.Set("key1", "value1")
	cache.Set("key2", "value2")
	cache.Set("key3", "value3")

	fmt.Printf("Initial size: %d\n", cache.Len())

	// Wait for entries to expire
	time.Sleep(150 * time.Millisecond)

	// Manually expire all TTL entries
	expired := cache.ExpireNow()
	fmt.Printf("Expired entries: %d\n", expired)
	fmt.Printf("Final size: %d\n", cache.Len())

	// Output: Initial size: 3
	// Expired entries: 3
	// Final size: 0
}

// ExampleGenericCache_integer_keys demonstrates using integer keys.
func ExampleGenericCache_integer_keys() {
	// Create cache with integer keys
	cache := balios.NewGenericCache[int, string](balios.Config{
		MaxSize: 100,
	})
	defer cache.Close()

	// Store HTTP status messages
	cache.Set(200, "OK")
	cache.Set(404, "Not Found")
	cache.Set(500, "Internal Server Error")

	// Retrieve by integer key
	if msg, found := cache.Get(404); found {
		fmt.Printf("HTTP 404: %s\n", msg)
	}

	// Output: HTTP 404: Not Found
}

// ExampleCache_negative_caching demonstrates error caching.
func ExampleCache_negative_caching() {
	cache := balios.NewCache(balios.Config{
		MaxSize:          100,
		TTL:              time.Hour,
		NegativeCacheTTL: 5 * time.Second, // Cache errors for 5 seconds
	})
	defer cache.Close()

	callCount := 0
	failingLoader := func() (interface{}, error) {
		callCount++
		return nil, fmt.Errorf("database unavailable")
	}

	// First call: loader fails, error is cached
	_, err := cache.GetOrLoad("key", failingLoader)
	fmt.Printf("First call - Count: %d, Error: %v\n", callCount, err != nil)

	// Second call within 5 seconds: returns cached error without calling loader
	_, err = cache.GetOrLoad("key", failingLoader)
	fmt.Printf("Second call - Count: %d, Error: %v\n", callCount, err != nil)

	// Output: First call - Count: 1, Error: true
	// Second call - Count: 1, Error: true
}
