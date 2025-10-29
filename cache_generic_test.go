// cache_generic_test.go: tests for generic cache API
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGILira fragment
// SPDX-License-Identifier: MPL-2.0

package balios

import (
	"strconv"
	"testing"
	"time"
)

// TestGenericCache_StringInt tests basic string->int cache
func TestGenericCache_StringInt(t *testing.T) {
	cache := NewGenericCache[string, int](DefaultConfig())

	// Test Set/Get
	cache.Set("one", 1)
	cache.Set("two", 2)
	cache.Set("three", 3)

	if value, found := cache.Get("one"); !found || value != 1 {
		t.Errorf("Expected 1, got %v (found=%v)", value, found)
	}

	if value, found := cache.Get("two"); !found || value != 2 {
		t.Errorf("Expected 2, got %v (found=%v)", value, found)
	}

	// Test missing key returns zero value
	if value, found := cache.Get("missing"); found {
		t.Errorf("Expected not found, got value=%v", value)
	}
}

// TestGenericCache_IntString tests int->string cache
func TestGenericCache_IntString(t *testing.T) {
	cache := NewGenericCache[int, string](DefaultConfig())

	cache.Set(1, "one")
	cache.Set(2, "two")

	if value, found := cache.Get(1); !found || value != "one" {
		t.Errorf("Expected 'one', got %v", value)
	}

	if value, found := cache.Get(2); !found || value != "two" {
		t.Errorf("Expected 'two', got %v", value)
	}
}

// User represents a complex value type
type User struct {
	ID   int
	Name string
	Role string
}

// TestGenericCache_StructValue tests cache with struct values
func TestGenericCache_StructValue(t *testing.T) {
	cache := NewGenericCache[string, User](DefaultConfig())

	user1 := User{ID: 123, Name: "Alice", Role: "admin"}
	user2 := User{ID: 456, Name: "Bob", Role: "user"}

	cache.Set("user:123", user1)
	cache.Set("user:456", user2)

	if value, found := cache.Get("user:123"); !found {
		t.Error("Expected user:123 to be found")
	} else {
		if value.ID != 123 || value.Name != "Alice" {
			t.Errorf("Expected user1, got %+v", value)
		}
	}

	if value, found := cache.Get("user:456"); !found {
		t.Error("Expected user:456 to be found")
	} else {
		if value.ID != 456 || value.Name != "Bob" {
			t.Errorf("Expected user2, got %+v", value)
		}
	}
}

// TestGenericCache_PointerValue tests cache with pointer values
func TestGenericCache_PointerValue(t *testing.T) {
	cache := NewGenericCache[string, *User](DefaultConfig())

	user1 := &User{ID: 123, Name: "Alice", Role: "admin"}
	user2 := &User{ID: 456, Name: "Bob", Role: "user"}

	cache.Set("user:123", user1)
	cache.Set("user:456", user2)

	if value, found := cache.Get("user:123"); !found {
		t.Error("Expected user:123 to be found")
	} else {
		if value.ID != 123 {
			t.Errorf("Expected user1, got %+v", value)
		}
	}

	// Test nil pointer
	var nilUser *User
	cache.Set("nil", nilUser)
	if value, found := cache.Get("nil"); !found || value != nil {
		t.Errorf("Expected nil pointer, got %v", value)
	}
}

// TestGenericCache_Delete tests deletion with generics
func TestGenericCache_Delete(t *testing.T) {
	cache := NewGenericCache[string, int](DefaultConfig())

	cache.Set("key", 42)
	if _, found := cache.Get("key"); !found {
		t.Error("Expected key to exist")
	}

	cache.Delete("key")
	if _, found := cache.Get("key"); found {
		t.Error("Expected key to be deleted")
	}
}

// TestGenericCache_Has tests Has method
func TestGenericCache_Has(t *testing.T) {
	cache := NewGenericCache[string, int](DefaultConfig())

	if cache.Has("key") {
		t.Error("Expected key not to exist")
	}

	cache.Set("key", 42)
	if !cache.Has("key") {
		t.Error("Expected key to exist")
	}

	cache.Delete("key")
	if cache.Has("key") {
		t.Error("Expected key not to exist after delete")
	}
}

// TestGenericCache_Clear tests clearing cache
func TestGenericCache_Clear(t *testing.T) {
	cache := NewGenericCache[string, int](DefaultConfig())

	cache.Set("one", 1)
	cache.Set("two", 2)
	cache.Set("three", 3)

	cache.Clear()

	if cache.Has("one") || cache.Has("two") || cache.Has("three") {
		t.Error("Expected cache to be empty after Clear")
	}

	stats := cache.Stats()
	if stats.Size != 0 {
		t.Errorf("Expected Size=0, got %d", stats.Size)
	}
}

// TestGenericCache_TTL tests TTL with generics
func TestGenericCache_TTL(t *testing.T) {
	cache := NewGenericCache[string, int](Config{
		MaxSize: 100,
		TTL:     50 * time.Millisecond,
	})

	cache.Set("key", 42)
	if value, found := cache.Get("key"); !found || value != 42 {
		t.Error("Expected key to exist")
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	if _, found := cache.Get("key"); found {
		t.Error("Expected key to be expired")
	}
}

// TestGenericCache_Stats tests stats collection
func TestGenericCache_Stats(t *testing.T) {
	cache := NewGenericCache[string, int](DefaultConfig())

	cache.Set("one", 1)
	cache.Set("two", 2)

	// Generate some hits
	cache.Get("one")
	cache.Get("one")
	cache.Get("two")

	// Generate some misses
	cache.Get("missing1")
	cache.Get("missing2")

	stats := cache.Stats()
	if stats.Hits != 3 {
		t.Errorf("Expected 3 hits, got %d", stats.Hits)
	}
	if stats.Misses != 2 {
		t.Errorf("Expected 2 misses, got %d", stats.Misses)
	}
	if stats.Size != 2 {
		t.Errorf("Expected 2 items, got %d", stats.Size)
	}
}

// TestGenericCache_Eviction tests eviction with generics
func TestGenericCache_Eviction(t *testing.T) {
	cache := NewGenericCache[string, int](Config{
		MaxSize:     3,
		WindowRatio: 0.01,
	})

	// Fill cache beyond capacity
	cache.Set("one", 1)
	cache.Set("two", 2)
	cache.Set("three", 3)
	cache.Set("four", 4) // Should trigger eviction

	stats := cache.Stats()
	if stats.Size > 3 {
		t.Errorf("Expected max 3 items, got %d", stats.Size)
	}
	if stats.Evictions == 0 {
		t.Error("Expected at least one eviction")
	}
}

func TestGenericCache_Close(t *testing.T) {
	cache := NewGenericCache[string, int](Config{MaxSize: 100})

	// Populate cache
	for i := 0; i < 10; i++ {
		cache.Set("key-"+strconv.Itoa(i), i)
	}

	stats := cache.Stats()
	if stats.Size != 10 {
		t.Errorf("Expected 10 items, got %d", stats.Size)
	}

	// Close the cache
	err := cache.Close()
	if err != nil {
		t.Errorf("Close returned error: %v", err)
	}

	// Verify cache is empty after Close
	stats = cache.Stats()
	if stats.Size != 0 {
		t.Errorf("Expected Size=0 after Close, got %d", stats.Size)
	}

	// Verify cache can still be used after Close (graceful degradation)
	cache.Set("new-key", 999)

	value, found := cache.Get("new-key")
	if !found || value != 999 {
		t.Error("Expected cache to work after Close (graceful degradation)")
	}
}

// BenchmarkGenericCache_SetGet benchmarks generic cache operations
func BenchmarkGenericCache_SetGet(b *testing.B) {
	cache := NewGenericCache[string, int](Config{MaxSize: 10000})

	b.Run("Set", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			cache.Set("key", i)
		}
	})

	b.Run("Get", func(b *testing.B) {
		cache.Set("key", 42)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			cache.Get("key")
		}
	})
}

// BenchmarkGenericCache_StructValue benchmarks with struct values
func BenchmarkGenericCache_StructValue(b *testing.B) {
	cache := NewGenericCache[string, User](Config{MaxSize: 10000})
	user := User{ID: 123, Name: "Alice", Role: "admin"}

	b.Run("Set", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			cache.Set("user:123", user)
		}
	})

	b.Run("Get", func(b *testing.B) {
		cache.Set("user:123", user)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			cache.Get("user:123")
		}
	})
}
