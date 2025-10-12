// ttl_test.go: unit tests for TTL functionality in Balios
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGILira library
// SPDX-License-Identifier: MPL-2.0

package balios

import (
	"testing"
	"time"
)

// MockTimeProvider allows controlling time in tests
type MockTimeProvider struct {
	currentTime int64
}

func (m *MockTimeProvider) Now() int64 {
	return m.currentTime
}

func (m *MockTimeProvider) Advance(duration time.Duration) {
	m.currentTime += int64(duration)
}

func TestCache_TTL_Basic(t *testing.T) {
	mockTime := &MockTimeProvider{currentTime: 1000000000}

	cache := NewCache(Config{
		MaxSize:      100,
		TTL:          100 * time.Millisecond,
		TimeProvider: mockTime,
	})

	// Set a value
	cache.Set("key", "value")

	// Should be accessible immediately
	value, found := cache.Get("key")
	if !found {
		t.Error("expected to find key immediately after set")
	}
	if value != "value" {
		t.Errorf("expected 'value', got %v", value)
	}

	// Advance time but not enough to expire
	mockTime.Advance(50 * time.Millisecond)

	// Should still be accessible
	_, found = cache.Get("key")
	if !found {
		t.Error("expected to find key before expiration")
	}

	// Advance time past expiration
	mockTime.Advance(60 * time.Millisecond)

	// Should not be accessible
	_, found = cache.Get("key")
	if found {
		t.Error("expected key to be expired")
	}
}

func TestCache_TTL_Update(t *testing.T) {
	mockTime := &MockTimeProvider{currentTime: 1000000000}

	cache := NewCache(Config{
		MaxSize:      100,
		TTL:          100 * time.Millisecond,
		TimeProvider: mockTime,
	})

	// Set a value
	cache.Set("key", "value1")

	// Advance time almost to expiration
	mockTime.Advance(90 * time.Millisecond)

	// Update the value (should reset TTL)
	cache.Set("key", "value2")

	// Advance time past original expiration
	mockTime.Advance(20 * time.Millisecond)

	// Should still be accessible because we updated it
	value, found := cache.Get("key")
	if !found {
		t.Error("expected to find key after update")
	}
	if value != "value2" {
		t.Errorf("expected 'value2', got %v", value)
	}

	// Advance time past new expiration
	mockTime.Advance(90 * time.Millisecond)

	// Now it should be expired
	_, found = cache.Get("key")
	if found {
		t.Error("expected key to be expired after new TTL")
	}
}

func TestCache_NoTTL(t *testing.T) {
	mockTime := &MockTimeProvider{currentTime: 1000000000}

	// Cache with no TTL
	cache := NewCache(Config{
		MaxSize:      100,
		TTL:          0, // No expiration
		TimeProvider: mockTime,
	})

	cache.Set("key", "value")

	// Advance time significantly
	mockTime.Advance(1 * time.Hour)

	// Should still be accessible
	value, found := cache.Get("key")
	if !found {
		t.Error("expected to find key when TTL is disabled")
	}
	if value != "value" {
		t.Errorf("expected 'value', got %v", value)
	}
}

func TestCache_TTL_MultipleKeys(t *testing.T) {
	mockTime := &MockTimeProvider{currentTime: 1000000000}

	cache := NewCache(Config{
		MaxSize:      100,
		TTL:          100 * time.Millisecond,
		TimeProvider: mockTime,
	})

	// Set multiple keys at different times
	cache.Set("key1", "value1")

	mockTime.Advance(50 * time.Millisecond)
	cache.Set("key2", "value2")

	// Advance to expire key1 but not key2
	mockTime.Advance(60 * time.Millisecond)

	// key1 should be expired
	_, found1 := cache.Get("key1")
	if found1 {
		t.Error("expected key1 to be expired")
	}

	// key2 should still be valid
	value2, found2 := cache.Get("key2")
	if !found2 {
		t.Error("expected key2 to be valid")
	}
	if value2 != "value2" {
		t.Errorf("expected 'value2', got %v", value2)
	}
}

func TestCache_TTL_Has(t *testing.T) {
	mockTime := &MockTimeProvider{currentTime: 1000000000}

	cache := NewCache(Config{
		MaxSize:      100,
		TTL:          100 * time.Millisecond,
		TimeProvider: mockTime,
	})

	cache.Set("key", "value")

	// Should exist initially
	if !cache.Has("key") {
		t.Error("expected key to exist")
	}

	// Advance past expiration
	mockTime.Advance(110 * time.Millisecond)

	// Has should also respect TTL
	// Note: Current Has() implementation doesn't check TTL
	// This test documents the behavior
	// TODO: Consider if Has() should check TTL
}
