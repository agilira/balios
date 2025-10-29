# Balios Architecture

This document describes Balios' internal architecture and implementation details.

## Overview

Balios implements the **W-TinyLFU** (Window Tiny Least Frequently Used) algorithm with a focus on lock-free operations and zero allocations on the hot path.

## Core Components

### 1. Entry Storage

Each cache entry is stored in a fixed-size array with atomic operations:

```go
type entry struct {
    key      string
    value    atomic.Value  // Thread-safe value storage
    keyHash  uint64
    expireAt int64         // Expiration timestamp (0 = no expiration)
    valid    int32         // Atomic flag: 0=empty, 1=valid, 2=deleted, 3=pending
}
```

**States:**
- `0` (entryEmpty): Slot is available
- `1` (entryValid): Contains valid data
- `2` (entryDeleted): Marked for deletion
- `3` (entryPending): Being written/updated

### 2. Hash Table

- **Size**: Next power of 2, at least 2x `MaxSize` for good load factor
- **Minimum**: 16 slots
- **Collision Resolution**: Bounded linear probing (max 128 slots)
- **Hash Function**: Custom `stringHash()` implementation

**Example:** For `MaxSize=10000`, table size = 32768 (2^15)

**Bounded Probing (v1.1.35+):**

Linear probing is bounded to a maximum of 128 slot checks to prevent O(capacity) worst-case scenarios:

```go
const maxProbeLength = 128  // Covers P99.99 cases
```

**Rationale:**
- At 50% load factor: P99 < 10 probes, P99.9 < 20 probes
- At 75% load factor: P99 < 30 probes, P99.9 < 60 probes  
- At 90% load factor: P99 < 128 probes
- Limit of 128 covers P99.99 of realistic scenarios

**Fallback mechanism:**
When max probes is reached without finding a slot, the cache triggers eviction and retries the operation. This maintains correctness while preventing pathological O(capacity) searches.

**Impact:**
- Zero performance regression (benchmarked)
- Maintains O(1) worst-case complexity
- Graceful degradation under extreme conditions

### 3. Frequency Sketch (Count-Min Sketch)

Lock-free frequency tracking for W-TinyLFU admission policy:

```go
type frequencySketch struct {
    table []uint64        // 4-bit counters packed in uint64
    tableMask uint64      // For fast modulo (power of 2)
    seed1, seed2, seed3, seed4 uint64  // 4 hash function seeds
    sampleSize int64      // Operation counter
    resetThreshold int64  // Aging trigger (maxSize * 10)
}
```

**Key Features:**
- **4-bit counters**: Each uint64 holds 16 counters (64 bits / 4 bits)
- **4 hash functions**: Golden ratio hash seeds for distribution
- **Saturation at 15**: Counters max out at 15 (4 bits)
- **Aging mechanism**: Reset after `maxSize * 10` operations

**Table Size Calculation:**
```go
tableSize = nextPowerOf2(maxSize / 4)  // Each uint64 holds 16 counters
if tableSize < 64 {
    tableSize = 64  // Minimum size
}
```

### 4. W-TinyLFU Algorithm

**Admission Policy:**
1. New item frequency is estimated using Count-Min Sketch
2. If cache is full, compare with victim's frequency
3. Admit only if new item has higher frequency
4. Prevents cache pollution from infrequent items

**Why W-TinyLFU?**
- Superior hit ratio vs pure LRU or LFU
- Handles recency and frequency simultaneously
- Proven performance in academic research

## Lock-Free Operations

### Set Operation

1. **Hash the key**: `keyHash := stringHash(key)`
2. **Update sketch**: `sketch.increment(keyHash)` (lock-free)
3. **Find slot**: Linear probing with atomic CAS
4. **Update entry**: Atomic operations on `valid` flag
5. **Store value**: `atomic.Value.Store()`

**No locks used** - only atomic operations.

### Get Operation

1. **Hash the key**: `keyHash := stringHash(key)`
2. **Find slot**: Linear probing
3. **Check validity**: Atomic load of `valid` flag
4. **Check TTL**: Compare `expireAt` with current time
5. **Load value**: `atomic.Value.Load()`
6. **Update sketch**: `sketch.increment(keyHash)` (lock-free)

**Zero allocations** on cache hit.

## TTL Implementation

**Time Provider:**
```go
type TimeProvider interface {
    Now() int64  // Returns nanoseconds
}
```

**Default Implementation:** [go-timecache](https://github.com/agilira/go-timecache)
- Uses `go-timecache.CachedTimeNano()`
- Updates every 1ms with dedicated goroutine
- Shared across all caches

**Expiration Strategy:**

Balios uses a **hybrid expiration approach** combining passive and active cleanup:

### 1. Inline Opportunistic Expiration (Zero Overhead)

During normal cache operations (Get, Has, Set), when accessing an entry:
- Check `expireAt` atomically
- If expired, mark as `entryDeleted` via CAS
- Increment `expirations` counter
- Continue operation (treat as miss/empty slot)

**Benefit:** No background goroutines, zero overhead when TTL=0

**Implementation:**
```go
func (c *wtinyLFUCache) isExpired(entry *entry, now int64) bool {
    if c.ttlNanos == 0 {
        return false // Fast path: TTL disabled
    }
    expireAt := atomic.LoadInt64(&entry.expireAt)
    return expireAt > 0 && now > expireAt
}
```

**During Set() - Opportunistic Cleanup:**
- While performing linear probing to find a slot
- If we encounter an expired entry, clean it immediately
- Reuse the slot or continue probing
- No extra scans, cleanup happens "for free"

### 2. Manual Expiration API (On-Demand)

`ExpireNow()` allows explicit, full-table expiration:
- Scans all entries in the hash table
- Removes expired entries via lock-free CAS
- Returns count of expired entries
- O(n) complexity where n = cache capacity
- Safe for concurrent calls

**Use Cases:**
- Scheduled cleanup (cron, ticker)
- Pre-emptive cleanup before traffic spikes
- Memory pressure mitigation
- Testing and debugging

**Example:**
```go
// Periodic cleanup
ticker := time.NewTicker(5 * time.Minute)
go func() {
    for range ticker.C {
        expired := cache.ExpireNow()
        metrics.RecordExpirations(expired)
    }
}()
```

### 3. Storage Format

- **Storage**: `expireAt` field (int64 nanoseconds)
- **No TTL**: `expireAt = 0` (never expires)
- **Atomic Access**: All reads/writes use `atomic.LoadInt64` / `atomic.StoreInt64`

**Performance Characteristics:**
- Inline cleanup: < 1ns overhead per check (branch prediction friendly)
- ExpireNow(): ~1-5µs per 1000 entries (lock-free CAS)
- Zero allocations for both approaches

## Memory Layout

### Cache Structure

```go
type wtinyLFUCache struct {
    // Immutable configuration
    maxSize      int32         // Maximum number of entries
    tableMask    uint32        // Hash table mask (tableSize - 1)
    ttlNanos     int64         // TTL in nanoseconds
    timeProvider TimeProvider  // Time source
    
    // Data structures
    entries []entry            // Fixed-size entry array
    sketch  *frequencySketch   // Frequency tracking
    
    // Atomic statistics
    hits        int64
    misses      int64
    sets        int64
    deletes     int64
    evictions   int64
    expirations int64  // TTL-based removals
    size        int64
}
```

**Memory Overhead:**
- Entry: ~48 bytes (key string, value interface{}, metadata)
- Frequency Sketch: ~8 bytes per uint64 (holds 16 counters)
- Total: Approximately `maxSize * 48 + (maxSize / 4) * 8` bytes

## Generics Implementation

### Type-Safe Wrapper

```go
type GenericCache[K comparable, V any] struct {
    inner Cache  // Wraps wtinyLFUCache
}
```

**Key Conversion:**
```go
func keyToString[K comparable](key K) string {
    switch v := any(key).(type) {
    case string:
        return v
    case int:
        return strconv.Itoa(v)
    case int64:
        return strconv.FormatInt(v, 10)
    // ... more optimized cases
    default:
        return fmt.Sprintf("%v", key)
    }
}
```

**Performance Overhead:**
- String keys: **0%** (direct passthrough)
- Integer keys: **+3-5%** (optimized conversion)
- Complex keys: **+5-10%** (fmt.Sprintf fallback)

## Singleflight Pattern

Prevents cache stampede for `GetOrLoad()` operations:

```go
type inflightCall struct {
    wg  sync.WaitGroup  // Synchronization primitive
    val atomic.Value    // Result storage (resultWrapper)
    err atomic.Value    // Error storage (errorWrapper)
}
```

**Two-Phase Pattern:**
1. **Pre-initialize**: `newFlight.wg.Add(1)` BEFORE LoadOrStore
2. **LoadOrStore**: Atomic insertion into sync.Map
3. **Execute**: First goroutine runs loader, others wait
4. **Broadcast**: `wg.Done()` wakes all waiters

**Why Wrappers?**
- `atomic.Value` cannot store `nil`
- Use `resultWrapper{value: val}` and `errorWrapper{err: err}`
- Allows storing nil values and nil errors safely

## Configuration

### Default Values

From `config.go`:
```go
const (
    DefaultMaxSize      = 10000
    DefaultWindowRatio  = 0.01
    DefaultCounterBits  = 4
    DefaultTTL          = 0  // No expiration
)
```

### Validation

- `MaxSize <= 0`: Use DefaultMaxSize
- `WindowRatio <= 0`: Use DefaultWindowRatio
- `TimeProvider == nil`: Use systemTimeProvider

## Performance Characteristics

### Time Complexity

- **Set**: O(1) average, O(1) worst case (bounded probing to 128 slots)
- **Get**: O(1) average, O(1) worst case (bounded probing to 128 slots)
- **Delete**: O(1) average, O(1) worst case (bounded probing to 128 slots)
- **Has**: O(1) average, O(1) worst case (bounded probing to 128 slots)

### Space Complexity

- O(n) where n = maxSize
- Hash table: 2x maxSize for good load factor
- Frequency sketch: ~maxSize/4 uint64 values

### Allocation Profile

- **Set**: 1 allocation (storing value in atomic.Value)
- **Get** (hit): 0 allocations
- **Get** (miss): 0 allocations
- **GetOrLoad** (hit): 0 allocations
- **GetOrLoad** (miss): 1 allocation

### Concurrency

- **Read operations**: Lock-free with atomic operations
- **Write operations**: Lock-free with CAS (Compare-And-Swap)
- **Frequency sketch**: Lock-free with atomic counters
- **Singleflight**: Uses sync.Map and sync.WaitGroup

## Comparison with Other Algorithms

### LRU (Least Recently Used)
- **Pros**: Simple, good for recency
- **Cons**: Poor hit ratio with scan-like access patterns
- **Balios advantage**: W-TinyLFU handles both frequency and recency

### LFU (Least Frequently Used)
- **Pros**: Good for frequency-based workloads
- **Cons**: Doesn't adapt to changing patterns (frequency never decays)
- **Balios advantage**: Aging mechanism prevents stale frequencies

### ARC (Adaptive Replacement Cache)
- **Pros**: Self-tuning, adapts to workload
- **Cons**: Complex, uses locks
- **Balios advantage**: Lock-free, simpler implementation

## Implementation Notes

### Atomic Operations

All state changes use atomic operations from `sync/atomic`:
- `atomic.LoadInt32()` / `atomic.StoreInt32()` for flags
- `atomic.CompareAndSwapInt32()` for CAS operations
- `atomic.AddInt64()` for statistics counters
- `atomic.Value` for value storage

### Race Detection

All code passes `-race` detector:
```bash
go test -race ./...
# ok  github.com/agilira/balios  4.171s
```

### Security Scanning

All code passes gosec security scanner:
```bash
gosec ./...
# Issues: 0
```

## Future Enhancements

1. **Async Refresh**: Stale-while-revalidate pattern
2. **Persistence**: Save/load cache from disk
3. **Prometheus Metrics**: Integration with monitoring
4. **Distributed Cache**: Coordination across instances

## References

- **W-TinyLFU Paper**: ["TinyLFU: A Highly Efficient Cache Admission Policy"](https://arxiv.org/abs/1512.00727) by Gil Einziger, Roy Friedman, Ben Manes (2017)
- **Count-Min Sketch**: ["An improved data stream summary: the count-min sketch and its applications"](https://doi.org/10.1016/j.jalgor.2003.12.001) by Graham Cormode, S. Muthukrishnan (2005)
- **Frequency Sketch Implementation**: Based on [Caffeine](https://github.com/ben-manes/caffeine) (Java) and [Otter](https://github.com/maypok86/otter) (Go)

---

Balios • an AGILira fragment
