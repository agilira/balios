// cache_test.go: unit tests and benchmarks for Balios
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGILira fragment
// SPDX-License-Identifier: MPL-2.0

package balios

import (
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewCache(t *testing.T) {
	cache := NewCache(Config{MaxSize: 100})
	if cache == nil {
		t.Fatal("NewCache returned nil")
	}

	if cache.Capacity() != 100 {
		t.Errorf("expected capacity 100, got %d", cache.Capacity())
	}

	if cache.Len() != 0 {
		t.Errorf("expected empty cache, got size %d", cache.Len())
	}
}

func TestCache_SetGet_Basic(t *testing.T) {
	cache := NewCache(Config{MaxSize: 100})

	// Test setting and getting a value
	ok := cache.Set("key1", "value1")
	if !ok {
		t.Error("Set should return true")
	}

	value, found := cache.Get("key1")
	if !found {
		t.Error("expected to find key1")
	}
	if value != "value1" {
		t.Errorf("expected 'value1', got %v", value)
	}

	// Test non-existent key
	_, found = cache.Get("nonexistent")
	if found {
		t.Error("expected not to find nonexistent key")
	}
}

func TestCache_SetGet_Update(t *testing.T) {
	cache := NewCache(Config{MaxSize: 100})

	// Set initial value
	cache.Set("key", "value1")

	// Update with new value
	cache.Set("key", "value2")

	value, found := cache.Get("key")
	if !found {
		t.Error("expected to find key")
	}
	if value != "value2" {
		t.Errorf("expected 'value2', got %v", value)
	}

	// Size should still be 1
	if cache.Len() != 1 {
		t.Errorf("expected size 1, got %d", cache.Len())
	}
}

func TestCache_MultipleKeys(t *testing.T) {
	cache := NewCache(Config{MaxSize: 100})

	// Set multiple key-value pairs
	testData := map[string]interface{}{
		"string": "hello",
		"number": 42,
		"bool":   true,
	}

	for key, expectedValue := range testData {
		cache.Set(key, expectedValue)
	}

	// Verify all values can be retrieved
	for key, expectedValue := range testData {
		value, found := cache.Get(key)
		if !found {
			t.Errorf("expected to find key %s", key)
		}
		if value != expectedValue {
			t.Errorf("key %s: expected %v, got %v", key, expectedValue, value)
		}
	}

	if cache.Len() != len(testData) {
		t.Errorf("expected size %d, got %d", len(testData), cache.Len())
	}
}

func TestCache_Delete(t *testing.T) {
	cache := NewCache(Config{MaxSize: 100})

	// Add entry
	cache.Set("key", "value")

	// Verify it exists
	if !cache.Has("key") {
		t.Error("key should exist before delete")
	}

	// Delete entry
	deleted := cache.Delete("key")
	if !deleted {
		t.Error("delete should return true for existing key")
	}

	// Verify it's gone
	if cache.Has("key") {
		t.Error("key should not exist after delete")
	}

	// Delete non-existent key
	deleted = cache.Delete("nonexistent")
	if deleted {
		t.Error("delete should return false for non-existent key")
	}
}

func TestCache_Has(t *testing.T) {
	cache := NewCache(Config{MaxSize: 100})

	// Key should not exist initially
	if cache.Has("key") {
		t.Error("key should not exist initially")
	}

	// Add key
	cache.Set("key", "value")

	// Key should exist now
	if !cache.Has("key") {
		t.Error("key should exist after set")
	}
}

func TestCache_Clear(t *testing.T) {
	cache := NewCache(Config{MaxSize: 100})

	// Add entries
	for i := 0; i < 10; i++ {
		key := strconv.Itoa(i)
		cache.Set(key, i)
	}

	// Verify entries exist
	if cache.Len() != 10 {
		t.Errorf("expected 10 entries before clear, got %d", cache.Len())
	}

	// Clear cache
	cache.Clear()

	// Verify cache is empty
	if cache.Len() != 0 {
		t.Errorf("expected 0 entries after clear, got %d", cache.Len())
	}

	// Verify no entries are accessible
	for i := 0; i < 10; i++ {
		key := strconv.Itoa(i)
		if cache.Has(key) {
			t.Errorf("key %s should not exist after clear", key)
		}
	}
}

func TestCache_Stats(t *testing.T) {
	cache := NewCache(Config{MaxSize: 100})

	// Initial stats should be zero
	stats := cache.Stats()
	if stats.Hits != 0 {
		t.Errorf("expected 0 hits, got %d", stats.Hits)
	}
	if stats.Misses != 0 {
		t.Errorf("expected 0 misses, got %d", stats.Misses)
	}
	if stats.Size != 0 {
		t.Errorf("expected 0 size, got %d", stats.Size)
	}

	// Add some entries and access them
	cache.Set("key1", "value1")
	cache.Set("key2", "value2")

	cache.Get("key1") // hit
	cache.Get("key1") // hit
	cache.Get("key3") // miss

	stats = cache.Stats()
	if stats.Hits != 2 {
		t.Errorf("expected 2 hits, got %d", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("expected 1 miss, got %d", stats.Misses)
	}
	if stats.Size != 2 {
		t.Errorf("expected 2 entries, got %d", stats.Size)
	}
}

func TestCache_Eviction(t *testing.T) {
	// Small cache to test eviction
	cache := NewCache(Config{MaxSize: 3})

	// Fill cache to capacity
	cache.Set("key1", "value1")
	cache.Set("key2", "value2")
	cache.Set("key3", "value3")

	if cache.Len() != 3 {
		t.Errorf("expected size 3, got %d", cache.Len())
	}

	// Add one more to trigger eviction
	cache.Set("key4", "value4")

	// Size should not exceed capacity
	if cache.Len() > 3 {
		t.Errorf("cache size %d exceeds capacity 3", cache.Len())
	}

	// At least one key should still be accessible
	found := 0
	keys := []string{"key1", "key2", "key3", "key4"}
	for _, key := range keys {
		if cache.Has(key) {
			found++
		}
	}

	if found == 0 {
		t.Error("no keys found after eviction - cache appears broken")
	}
}

func TestCache_Close(t *testing.T) {
	cache := NewCache(Config{MaxSize: 100})

	// Populate cache
	for i := 0; i < 10; i++ {
		cache.Set("key"+strconv.Itoa(i), i)
	}

	if cache.Len() != 10 {
		t.Errorf("Expected 10 items, got %d", cache.Len())
	}

	// Close the cache
	err := cache.Close()
	if err != nil {
		t.Errorf("Close returned error: %v", err)
	}

	// Verify cache is empty after Close
	if cache.Len() != 0 {
		t.Errorf("Expected empty cache after Close, got %d items", cache.Len())
	}

	stats := cache.Stats()
	if stats.Size != 0 {
		t.Errorf("Expected Size=0 after Close, got %d", stats.Size)
	}

	// Verify cache can still be used after Close (graceful degradation)
	success := cache.Set("new-key", "new-value")
	if !success {
		t.Error("Expected Set to succeed after Close")
	}

	value, found := cache.Get("new-key")
	if !found || value != "new-value" {
		t.Error("Expected cache to work after Close (graceful degradation)")
	}
}

// Single-threaded benchmarks to measure raw performance
func BenchmarkCache_Set_SingleThread(b *testing.B) {
	cache := NewCache(Config{MaxSize: 10000})

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		key := strconv.Itoa(i)
		cache.Set(key, i)
	}
}

func BenchmarkCache_Get_SingleThread(b *testing.B) {
	cache := NewCache(Config{MaxSize: 10000})

	// Pre-populate cache
	for i := 0; i < 1000; i++ {
		key := strconv.Itoa(i)
		cache.Set(key, i)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		key := strconv.Itoa(i % 1000)
		cache.Get(key)
	}
}

// =============================================================================
// HELPER FUNCTIONS TESTS
// =============================================================================

func TestCalculateExpiration(t *testing.T) {
	// Test with TTL enabled
	cache := NewCache(Config{MaxSize: 100, TTL: 100 * time.Millisecond}).(*wtinyLFUCache)
	now := int64(1000000000)
	expireAt := cache.calculateExpiration(now)
	expected := now + int64(100*time.Millisecond)
	if expireAt != expected {
		t.Errorf("Expected %d, got %d", expected, expireAt)
	}

	// Test with TTL disabled
	cacheNoTTL := NewCache(Config{MaxSize: 100}).(*wtinyLFUCache)
	expireAt = cacheNoTTL.calculateExpiration(now)
	if expireAt != 0 {
		t.Errorf("Expected 0, got %d", expireAt)
	}

	// Test with zero time
	expireAt = cache.calculateExpiration(0)
	if expireAt != 0 {
		t.Errorf("Expected 0 for zero time, got %d", expireAt)
	}
}

func TestCalculateEffectiveMaxProbes(t *testing.T) {
	// Test with small cache
	cache := NewCache(Config{MaxSize: 10}).(*wtinyLFUCache)
	probes := cache.calculateEffectiveMaxProbes()
	// For MaxSize=10, tableSize=32 (nextPowerOf2(10*2)), tableMask=31, so min(128, 31) = 31
	if probes != 31 {
		t.Errorf("Expected 31, got %d", probes)
	}

	// Test with large cache
	cacheLarge := NewCache(Config{MaxSize: 10000}).(*wtinyLFUCache)
	probes = cacheLarge.calculateEffectiveMaxProbes()
	if probes != 128 { // maxProbeLength
		t.Errorf("Expected 128, got %d", probes)
	}
}

func TestCalculateStartIndex(t *testing.T) {
	cache := NewCache(Config{MaxSize: 100}).(*wtinyLFUCache)
	keyHash := uint64(0x123456789ABCDEF0)
	startIdx := cache.calculateStartIndex(keyHash)
	expected := keyHash & uint64(cache.tableMask)
	if startIdx != expected {
		t.Errorf("Expected %d, got %d", expected, startIdx)
	}
}

func TestValidateKey(t *testing.T) {
	cache := NewCache(Config{MaxSize: 100}).(*wtinyLFUCache)

	// Test valid key
	if !cache.validateKey("valid_key") {
		t.Error("Expected valid key to return true")
	}

	// Test empty key
	if cache.validateKey("") {
		t.Error("Expected empty key to return false")
	}

	// Test non-empty key
	if !cache.validateKey("a") {
		t.Error("Expected single char key to return true")
	}
}

func TestShouldSkipEntry(t *testing.T) {
	cache := NewCache(Config{MaxSize: 100}).(*wtinyLFUCache)

	// Test pending entry
	if !cache.shouldSkipEntry(entryPending) {
		t.Error("Expected pending entry to be skipped")
	}

	// Test non-pending entries
	if cache.shouldSkipEntry(entryEmpty) {
		t.Error("Expected empty entry to not be skipped")
	}
	if cache.shouldSkipEntry(entryValid) {
		t.Error("Expected valid entry to not be skipped")
	}
	if cache.shouldSkipEntry(entryDeleted) {
		t.Error("Expected deleted entry to not be skipped")
	}
}

func TestIsSlotAvailable(t *testing.T) {
	cache := NewCache(Config{MaxSize: 100}).(*wtinyLFUCache)

	// Test available slots
	if !cache.isSlotAvailable(entryEmpty) {
		t.Error("Expected empty slot to be available")
	}
	if !cache.isSlotAvailable(entryDeleted) {
		t.Error("Expected deleted slot to be available")
	}

	// Test unavailable slots
	if cache.isSlotAvailable(entryValid) {
		t.Error("Expected valid slot to not be available")
	}
	if cache.isSlotAvailable(entryPending) {
		t.Error("Expected pending slot to not be available")
	}
}

func TestAttemptExpiredCleanup(t *testing.T) {
	// Create cache with short TTL
	cache := NewCache(Config{
		MaxSize: 100,
		TTL:     50 * time.Millisecond,
	}).(*wtinyLFUCache)

	// Set a value
	cache.Set("key1", "value1")

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Find the entry (this is a bit hacky but necessary for testing)
	var targetEntry *entry
	for i := range cache.entries {
		entry := &cache.entries[i]
		if atomic.LoadInt32(&entry.valid) == entryValid {
			if key := entry.loadKey(); key == "key1" {
				targetEntry = entry
				break
			}
		}
	}

	if targetEntry == nil {
		t.Fatal("Could not find target entry")
	}

	// Test cleanup
	now := cache.timeProvider.Now()
	cleaned := cache.attemptExpiredCleanup(targetEntry, now)

	if !cleaned {
		t.Error("Expected expired entry to be cleaned up")
	}

	// Verify entry is now deleted
	state := atomic.LoadInt32(&targetEntry.valid)
	if state != entryDeleted {
		t.Errorf("Expected entry to be deleted, got state %d", state)
	}
}
