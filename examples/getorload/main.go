package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/agilira/balios"
	"github.com/agilira/go-errors"
)

// User represents a user entity loaded from database
type User struct {
	ID        int
	Name      string
	Email     string
	CreatedAt time.Time
}

// Simulate expensive database call
func fetchUserFromDB(id int) (User, error) {
	log.Printf("⏱️  Fetching user %d from database (slow operation)...", id)
	time.Sleep(100 * time.Millisecond) // Simulate DB latency

	return User{
		ID:        id,
		Name:      fmt.Sprintf("User%d", id),
		Email:     fmt.Sprintf("user%d@example.com", id),
		CreatedAt: time.Now(),
	}, nil
}

// Simulate expensive database call with context support
func fetchUserFromDBWithContext(ctx context.Context, id int) (User, error) {
	log.Printf("⏱️  Fetching user %d from database with context...", id)

	// Simulate DB operation that respects context cancellation
	select {
	case <-time.After(100 * time.Millisecond):
		return User{
			ID:        id,
			Name:      fmt.Sprintf("User%d", id),
			Email:     fmt.Sprintf("user%d@example.com", id),
			CreatedAt: time.Now(),
		}, nil
	case <-ctx.Done():
		return User{}, ctx.Err()
	}
}

func main() {
	// Create type-safe cache
	cache := balios.NewGenericCache[int, User](balios.Config{
		MaxSize: 1000,
		TTL:     5 * time.Minute,
	})

	fmt.Println("=== Example 1: Basic GetOrLoad ===")
	basicGetOrLoad(cache)

	fmt.Println("\n=== Example 2: Cache Stampede Prevention (Singleflight) ===")
	cacheStampedePrevention(cache)

	fmt.Println("\n=== Example 3: Context with Timeout ===")
	contextWithTimeout(cache)

	fmt.Println("\n=== Example 4: Context Cancellation ===")
	contextCancellation(cache)

	fmt.Println("\n=== Example 5: Error Handling ===")
	errorHandling(cache)
}

// Example 1: Basic GetOrLoad
func basicGetOrLoad(cache *balios.GenericCache[int, User]) {
	// First call: cache miss, loader will be called
	user1, err := cache.GetOrLoad(100, func() (User, error) {
		return fetchUserFromDB(100)
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("✅ First call: %s (%s)\n", user1.Name, user1.Email)

	// Second call: cache hit, loader NOT called
	user2, err := cache.GetOrLoad(100, func() (User, error) {
		log.Println("❌ This should NOT be printed!")
		return fetchUserFromDB(100)
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("✅ Second call (cached): %s (%s)\n", user2.Name, user2.Email)
}

// Example 2: Cache Stampede Prevention with Singleflight
func cacheStampedePrevention(cache *balios.GenericCache[int, User]) {
	userID := 200
	numGoroutines := 100

	fmt.Printf("Launching %d concurrent requests for user %d...\n", numGoroutines, userID)

	var wg sync.WaitGroup
	startTime := time.Now()

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			user, err := cache.GetOrLoad(userID, func() (User, error) {
				return fetchUserFromDB(userID)
			})

			if err != nil {
				log.Printf("Goroutine %d error: %v", goroutineID, err)
				return
			}

			if goroutineID == 0 {
				fmt.Printf("✅ Goroutine %d got user: %s\n", goroutineID, user.Name)
			}
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(startTime)

	fmt.Printf("✅ All %d requests completed in %v\n", numGoroutines, elapsed)
	fmt.Printf("🎯 Database accessed only ONCE (not %d times!) thanks to singleflight\n", numGoroutines)
}

// Example 3: Context with Timeout
func contextWithTimeout(cache *balios.GenericCache[int, User]) {
	userID := 300

	// Context with short timeout (50ms) - will fail because DB takes 100ms
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	fmt.Printf("Attempting to load user %d with 50ms timeout (DB takes 100ms)...\n", userID)

	user, err := cache.GetOrLoadWithContext(ctx, userID, func(ctx context.Context) (User, error) {
		return fetchUserFromDBWithContext(ctx, userID)
	})

	if err != nil {
		fmt.Printf("❌ Expected timeout error: %v\n", err)
	} else {
		fmt.Printf("✅ Got user: %s\n", user.Name)
	}

	// Now with sufficient timeout (200ms) - will succeed
	ctx2, cancel2 := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel2()

	fmt.Printf("Retrying with 200ms timeout...\n")

	user, err = cache.GetOrLoadWithContext(ctx2, userID, func(ctx context.Context) (User, error) {
		return fetchUserFromDBWithContext(ctx, userID)
	})

	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("✅ Success with longer timeout: %s\n", user.Name)
}

// Example 4: Context Cancellation
func contextCancellation(cache *balios.GenericCache[int, User]) {
	userID := 400

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after 30ms
	go func() {
		time.Sleep(30 * time.Millisecond)
		fmt.Println("🛑 Cancelling context...")
		cancel()
	}()

	fmt.Printf("Loading user %d with cancellable context...\n", userID)

	user, err := cache.GetOrLoadWithContext(ctx, userID, func(ctx context.Context) (User, error) {
		return fetchUserFromDBWithContext(ctx, userID)
	})

	if err != nil {
		fmt.Printf("❌ Expected cancellation error: %v\n", err)
	} else {
		fmt.Printf("✅ Got user: %s\n", user.Name)
	}
}

// Example 5: Error Handling
func errorHandling(cache *balios.GenericCache[int, User]) {
	userID := 500

	// Test 1: Loader returns error
	fmt.Println("Test 1: Loader returning error...")
	_, err := cache.GetOrLoad(userID, func() (User, error) {
		return User{}, fmt.Errorf("database connection failed")
	})
	if err != nil {
		fmt.Printf("✅ Error properly propagated: %v\n", err)
	}

	// Test 2: Nil loader (validation error)
	fmt.Println("\nTest 2: Nil loader (should fail validation)...")
	_, err = cache.GetOrLoad(userID, nil)
	if err != nil {
		fmt.Printf("✅ Validation error: %v\n", err)

		// Check for specific error code using go-errors
		if errors.HasCode(err, balios.ErrCodeInvalidLoader) {
			fmt.Println("✅ Correct error code: BALIOS_INVALID_LOADER")
		}
	}

	// Test 3: Loader panic (should be recovered)
	fmt.Println("\nTest 3: Loader panicking (should recover)...")
	_, err = cache.GetOrLoad(userID+1, func() (User, error) {
		panic("unexpected panic in loader!")
	})
	if err != nil {
		fmt.Printf("✅ Panic recovered: %v\n", err)

		// Check for panic recovered error code using go-errors
		if errors.HasCode(err, balios.ErrCodePanicRecovered) {
			fmt.Println("✅ Correct error code: BALIOS_PANIC_RECOVERED")
		}
	}
}
