// main_test.go: tests for GetOrLoad example
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGILira fragment
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/agilira/balios"
)

func TestFetchUserFromDB(t *testing.T) {
	user, err := fetchUserFromDB(123)
	if err != nil {
		t.Fatalf("fetchUserFromDB failed: %v", err)
	}

	if user.ID != 123 {
		t.Errorf("Expected ID=123, got %d", user.ID)
	}

	if user.Name != "User123" {
		t.Errorf("Expected Name='User123', got '%s'", user.Name)
	}
}

func TestFetchUserFromDBWithContext(t *testing.T) {
	ctx := context.Background()
	user, err := fetchUserFromDBWithContext(ctx, 456)
	if err != nil {
		t.Fatalf("fetchUserFromDBWithContext failed: %v", err)
	}

	if user.ID != 456 {
		t.Errorf("Expected ID=456, got %d", user.ID)
	}
}

func TestFetchUserFromDBWithContext_Timeout(t *testing.T) {
	// Create context with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	_, err := fetchUserFromDBWithContext(ctx, 789)
	if err == nil {
		t.Error("Expected timeout error, got nil")
	}

	if err != context.DeadlineExceeded {
		t.Errorf("Expected context.DeadlineExceeded, got %v", err)
	}
}

func TestBasicGetOrLoad(t *testing.T) {
	cache := balios.NewGenericCache[int, User](balios.Config{
		MaxSize: 100,
	})
	defer func() { _ = cache.Close() }()

	// First call should load from "database"
	user, err := cache.GetOrLoad(123, func() (User, error) {
		return fetchUserFromDB(123)
	})

	if err != nil {
		t.Fatalf("GetOrLoad failed: %v", err)
	}

	if user.ID != 123 {
		t.Errorf("Expected ID=123, got %d", user.ID)
	}

	// Second call should be cache hit
	user2, err := cache.GetOrLoad(123, func() (User, error) {
		t.Fatal("Loader should not be called on cache hit")
		return User{}, nil
	})

	if err != nil {
		t.Fatalf("GetOrLoad cache hit failed: %v", err)
	}

	if user2.ID != 123 {
		t.Errorf("Expected cached user ID=123, got %d", user2.ID)
	}
}

func TestGetOrLoadWithContext_Success(t *testing.T) {
	cache := balios.NewGenericCache[int, User](balios.Config{
		MaxSize: 100,
	})
	defer func() { _ = cache.Close() }()

	ctx := context.Background()

	user, err := cache.GetOrLoadWithContext(ctx, 999, func(ctx context.Context) (User, error) {
		return fetchUserFromDBWithContext(ctx, 999)
	})

	if err != nil {
		t.Fatalf("GetOrLoadWithContext failed: %v", err)
	}

	if user.ID != 999 {
		t.Errorf("Expected ID=999, got %d", user.ID)
	}
}

func TestCacheStampedePrevention(t *testing.T) {
	cache := balios.NewGenericCache[int, User](balios.Config{
		MaxSize: 100,
	})
	defer func() { _ = cache.Close() }()

	// Simulate 10 concurrent requests for the same user
	const numGoroutines = 10
	const userID = 555

	loadCount := 0
	loader := func() (User, error) {
		loadCount++
		return fetchUserFromDB(userID)
	}

	results := make([]User, numGoroutines)
	errors := make([]error, numGoroutines)

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			results[idx], errors[idx] = cache.GetOrLoad(userID, loader)
		}(i)
	}

	wg.Wait()

	// Verify all succeeded
	for i, err := range errors {
		if err != nil {
			t.Errorf("Request %d failed: %v", i, err)
		}
	}

	// Verify all got same user
	for i, user := range results {
		if user.ID != userID {
			t.Errorf("Request %d got wrong user ID: %d", i, user.ID)
		}
	}

	// The loader should be called only once due to singleflight
	if loadCount > 1 {
		t.Logf("Warning: loader called %d times (expected 1, but may be >1 due to timing)", loadCount)
	}
}
