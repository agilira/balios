// main.go: package main - demonstrates error handling in Balios
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGILira fragment
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/agilira/balios"
)

func main() {
	fmt.Println("=== Balios Error Handling Demo ===")
	fmt.Println()

	// Create a small cache for demonstration
	cache := balios.NewCache(balios.Config{
		MaxSize: 3,
		TTL:     5 * time.Second,
	})

	// Example 1: Key not found
	fmt.Println("1. Key Not Found:")
	demonstrateKeyNotFound(cache)

	// Example 2: Cache full with retryable error
	fmt.Println("\n2. Cache Full (Retryable):")
	demonstrateCacheFull(cache)

	// Example 3: Error context extraction
	fmt.Println("\n3. Error Context Extraction:")
	demonstrateErrorContext()

	// Example 4: Error categorization
	fmt.Println("\n4. Error Categorization:")
	demonstrateErrorCategories()

	// Example 5: JSON serialization
	fmt.Println("\n5. JSON Serialization:")
	demonstrateJSONSerialization()

	// Example 6: Error wrapping
	fmt.Println("\n6. Error Wrapping:")
	demonstrateErrorWrapping()
}

func demonstrateKeyNotFound(cache balios.Cache) {
	key := "nonexistent"
	value, found := cache.Get(key)
	if !found {
		err := balios.NewErrKeyNotFound(key)
		fmt.Printf("  Error: %v\n", err)
		fmt.Printf("  Code: %s\n", balios.GetErrorCode(err))
		fmt.Printf("  Is Not Found: %v\n", balios.IsNotFound(err))
		fmt.Printf("  Is Retryable: %v\n", balios.IsRetryable(err))
	} else {
		fmt.Printf("  Value: %v\n", value)
	}
}

func demonstrateCacheFull(cache balios.Cache) {
	// Fill cache to capacity
	cache.Set("key1", "value1")
	cache.Set("key2", "value2")
	cache.Set("key3", "value3")

	// Try to add one more - will succeed due to eviction
	ok := cache.Set("key4", "value4")
	if !ok {
		// This won't happen in this demo, but shows how to handle
		err := balios.NewErrCacheFull(3, 3)
		fmt.Printf("  Error: %v\n", err)
		fmt.Printf("  Is Cache Full: %v\n", balios.IsCacheFull(err))
		fmt.Printf("  Is Retryable: %v\n", balios.IsRetryable(err))

		if balios.IsRetryable(err) {
			fmt.Println("  → Can retry after cleanup or eviction")
		}
	} else {
		fmt.Println("  ✓ Set succeeded (eviction worked)")
	}
}

func demonstrateErrorContext() {
	err := balios.NewErrCacheFull(100, 100)
	ctx := balios.GetErrorContext(err)

	fmt.Printf("  Error: %v\n", err)
	fmt.Println("  Context:")
	for key, value := range ctx {
		fmt.Printf("    %s: %v\n", key, value)
	}
}

func demonstrateErrorCategories() {
	errors := []struct {
		name string
		err  error
	}{
		{"Config Error", balios.NewErrInvalidMaxSize(-1)},
		{"Operation Error", balios.NewErrCacheFull(10, 10)},
		{"Loader Error", balios.NewErrLoaderTimeout("key", "5s")},
		{"Persistence Error", balios.NewErrCorruptedData("/tmp/cache", "invalid format")},
		{"Internal Error", balios.NewErrPanicRecovered("operation", "panic message")},
	}

	for _, item := range errors {
		fmt.Printf("  %s:\n", item.name)
		fmt.Printf("    Code: %s\n", balios.GetErrorCode(item.err))
		fmt.Printf("    IsConfig: %v, IsOperation: %v, IsLoader: %v, IsPersistence: %v\n",
			balios.IsConfigError(item.err),
			balios.IsOperationError(item.err),
			balios.IsLoaderError(item.err),
			balios.IsPersistenceError(item.err))
	}
}

func demonstrateJSONSerialization() {
	err := balios.NewErrCacheFull(100, 100)

	// Type assert to *errors.Error for JSON marshaling
	var baliosErr interface{} = err
	data, jsonErr := json.MarshalIndent(baliosErr, "  ", "  ")
	if jsonErr != nil {
		log.Printf("JSON marshal failed: %v", jsonErr)
		return
	}

	fmt.Printf("  JSON:\n%s\n", string(data))
}

func demonstrateErrorWrapping() {
	// Simulate a database error
	dbErr := fmt.Errorf("connection timeout")

	// Wrap with Balios error
	err := balios.NewErrLoaderFailed("user:123", dbErr)

	fmt.Printf("  Wrapped Error: %v\n", err)
	fmt.Printf("  Code: %s\n", balios.GetErrorCode(err))
	fmt.Printf("  Is Retryable: %v\n", balios.IsRetryable(err))

	// Extract context
	ctx := balios.GetErrorContext(err)
	if key, ok := ctx["key"]; ok {
		fmt.Printf("  Failed Key: %v\n", key)
	}
}
