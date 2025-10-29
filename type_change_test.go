// type_change_test.go: tests for handling value type changes on same key
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGILira fragment
// SPDX-License-Identifier: MPL-2.0
//
// REGRESSION TEST for atomic.Value panic bug:
// Before fix: Reusing valueHolder caused panic when type changed
// After fix: Always create new valueHolder, allows any type change

package balios

import (
	"testing"
	"time"
)

// TestTypeChangeSameKey verifies that the cache handles type changes correctly
// without panicking (atomic.Value would panic if we reused the same valueHolder)
func TestTypeChangeSameKey(t *testing.T) {
	cache := NewCache(Config{
		MaxSize: 100,
	})

	key := "mykey"

	// Set with int
	ok := cache.Set(key, 123)
	if !ok {
		t.Fatal("Failed to set int value")
	}

	val, found := cache.Get(key)
	if !found || val != 123 {
		t.Fatalf("Expected 123, got %v (found=%v)", val, found)
	}

	// Change type to string (this would panic with old implementation)
	ok = cache.Set(key, "hello")
	if !ok {
		t.Fatal("Failed to set string value")
	}

	val, found = cache.Get(key)
	if !found || val != "hello" {
		t.Fatalf("Expected 'hello', got %v (found=%v)", val, found)
	}

	// Change type to struct
	type MyStruct struct {
		Field int
	}
	ok = cache.Set(key, MyStruct{Field: 42})
	if !ok {
		t.Fatal("Failed to set struct value")
	}

	val, found = cache.Get(key)
	if !found {
		t.Fatal("Key not found")
	}
	if s, ok := val.(MyStruct); !ok || s.Field != 42 {
		t.Fatalf("Expected MyStruct{42}, got %v", val)
	}

	// Change back to int
	ok = cache.Set(key, 999)
	if !ok {
		t.Fatal("Failed to set int value again")
	}

	val, found = cache.Get(key)
	if !found || val != 999 {
		t.Fatalf("Expected 999, got %v (found=%v)", val, found)
	}

	t.Logf("✅ Successfully changed value type 4 times without panic")
}

// TestTypeChangeConcurrent verifies type changes work under concurrent load
func TestTypeChangeConcurrent(t *testing.T) {
	cache := NewCache(Config{
		MaxSize: 1000,
	})

	key := "concurrent-key"

	// Pre-populate
	cache.Set(key, 0)

	// Launch concurrent goroutines that change types
	const goroutines = 10
	const iterations = 100

	done := make(chan bool, goroutines)

	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Goroutine %d panicked: %v", id, r)
				}
				done <- true
			}()

			for i := 0; i < iterations; i++ {
				// Alternate between types
				switch (id + i) % 3 {
				case 0:
					cache.Set(key, i)
				case 1:
					cache.Set(key, "string")
				case 2:
					cache.Set(key, float64(i))
				}
			}
		}(g)
	}

	// Wait for all goroutines
	for i := 0; i < goroutines; i++ {
		<-done
	}

	// Verify key still exists
	_, found := cache.Get(key)
	if !found {
		t.Error("Key disappeared during concurrent type changes")
	}

	t.Logf("✅ Survived %d concurrent type changes", goroutines*iterations)
}

// TestTypeChangeWithTTL verifies type changes work with TTL enabled
func TestTypeChangeWithTTL(t *testing.T) {
	cache := NewCache(Config{
		MaxSize: 100,
		TTL:     time.Second * 60,
	})

	key := "ttl-key"

	// Set with one type
	cache.Set(key, 123)

	// Change type
	cache.Set(key, "changed")

	// Verify
	val, found := cache.Get(key)
	if !found || val != "changed" {
		t.Fatalf("Expected 'changed', got %v (found=%v)", val, found)
	}

	t.Log("✅ Type change works with TTL enabled")
}

// TestGenericCacheTypeConsistency verifies GenericCache prevents type changes
func TestGenericCacheTypeConsistency(t *testing.T) {
	// GenericCache with string values
	cache := NewGenericCache[string, string](Config{
		MaxSize: 100,
	})

	key := "mykey"

	// Set with string
	cache.Set(key, "hello")

	// Update with another string (should work)
	cache.Set(key, "world")

	val, found := cache.Get(key)
	if !found || val != "world" {
		t.Fatalf("Expected 'world', got %v (found=%v)", val, found)
	}

	t.Log("✅ GenericCache maintains type consistency by design")
}
