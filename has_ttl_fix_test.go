// has_ttl_fix_test.go: tests for Has() TTL consistency fix
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGILira library
// SPDX-License-Identifier: MPL-2.0

package balios

import (
	"testing"
	"time"
)

// TestHasTTLConsistency verifies that Has() respects TTL and is consistent with Get()
func TestHasTTLConsistency(t *testing.T) {
	cache := NewCache(Config{
		MaxSize: 100,
		TTL:     100 * time.Millisecond, // Short TTL for testing
	})

	// Set a key
	key := "test-key"
	value := "test-value"
	cache.Set(key, value)

	// Verify key exists immediately
	if !cache.Has(key) {
		t.Error("Has() should return true immediately after Set()")
	}

	val, found := cache.Get(key)
	if !found {
		t.Error("Get() should return true immediately after Set()")
	}
	if val != value {
		t.Errorf("Get() returned wrong value: got %v, want %v", val, value)
	}

	// Wait for TTL to expire
	time.Sleep(150 * time.Millisecond)

	// BUG FIX: Has() should now return false (consistent with Get())
	// Before fix: Has() would return true even for expired entries
	hasResult := cache.Has(key)
	_, getResult := cache.Get(key)

	if hasResult != getResult {
		t.Errorf("Has() and Get() are inconsistent after TTL expiration: Has()=%v, Get()=%v",
			hasResult, getResult)
	}

	if hasResult {
		t.Error("Has() should return false for expired entries (bug fix verification)")
	}

	if getResult {
		t.Error("Get() should return false for expired entries")
	}
}

// TestHasTTLConsistencyConcurrent verifies Has()/Get() consistency under concurrent load
func TestHasTTLConsistencyConcurrent(t *testing.T) {
	cache := NewCache(Config{
		MaxSize: 1000,
		TTL:     50 * time.Millisecond,
	})

	// Pre-populate cache
	for i := 0; i < 100; i++ {
		cache.Set(formatKey(i), i)
	}

	// Wait for some to expire
	time.Sleep(75 * time.Millisecond)

	// Concurrent reads - Has() and Get() should be consistent
	done := make(chan bool)
	inconsistencies := make(chan string, 100)

	for goroutine := 0; goroutine < 10; goroutine++ {
		go func() {
			for i := 0; i < 100; i++ {
				key := formatKey(i)
				has := cache.Has(key)
				_, get := cache.Get(key)

				if has != get {
					inconsistencies <- formatKey(i)
				}
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	close(inconsistencies)

	// Check for inconsistencies
	count := 0
	for key := range inconsistencies {
		t.Errorf("Has()/Get() inconsistency for key: %s", key)
		count++
		if count >= 10 {
			t.Error("Too many inconsistencies, stopping...")
			break
		}
	}
}

// TestHasWithoutTTL verifies Has() works correctly when TTL is disabled
func TestHasWithoutTTL(t *testing.T) {
	cache := NewCache(Config{
		MaxSize: 100,
		TTL:     0, // No TTL
	})

	cache.Set("key", "value")

	// Without TTL, Has() and Get() should always be consistent
	for i := 0; i < 10; i++ {
		has := cache.Has("key")
		_, get := cache.Get("key")

		if has != get {
			t.Errorf("Has()/Get() inconsistent without TTL: Has()=%v, Get()=%v", has, get)
		}

		if !has || !get {
			t.Error("Both Has() and Get() should return true without TTL")
		}

		time.Sleep(10 * time.Millisecond)
	}
}

func formatKey(i int) string {
	return "key-" + string(rune('0'+i%10))
}
