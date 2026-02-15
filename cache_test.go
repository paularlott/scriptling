package scriptling

import (
	"container/list"
	"fmt"
	"sync"
	"testing"

	"github.com/paularlott/scriptling/ast"
	"github.com/paularlott/scriptling/token"
)

// helper to create a small cache for testing
func newTestCache(maxSize int) *programCache {
	return &programCache{
		entries: make(map[cacheKey]*list.Element),
		lru:     list.New(),
		maxSize: maxSize,
	}
}

// helper to create a dummy program distinguishable by a label
func dummyProgram(label string) *ast.Program {
	return &ast.Program{
		Statements: []ast.Statement{
			&ast.ExpressionStatement{
				Token: token.Token{Literal: label},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Basic get / set
// ---------------------------------------------------------------------------

func TestCache_SetAndGet(t *testing.T) {
	c := newTestCache(10)
	prog := dummyProgram("hello")

	c.set("hello", prog)

	got, ok := c.get("hello")
	if !ok {
		t.Fatal("expected cache hit, got miss")
	}
	if got != prog {
		t.Fatal("returned program does not match stored program")
	}
}

func TestCache_Miss(t *testing.T) {
	c := newTestCache(10)

	_, ok := c.get("nonexistent")
	if ok {
		t.Fatal("expected cache miss, got hit")
	}
}

// ---------------------------------------------------------------------------
// Update existing entry
// ---------------------------------------------------------------------------

func TestCache_UpdateExistingEntry(t *testing.T) {
	c := newTestCache(10)
	prog1 := dummyProgram("v1")
	prog2 := dummyProgram("v2")

	c.set("script", prog1)
	c.set("script", prog2)

	got, ok := c.get("script")
	if !ok {
		t.Fatal("expected cache hit after update")
	}
	if got != prog2 {
		t.Fatal("expected updated program, got old one")
	}
	// updating should not increase entry count
	if len(c.entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(c.entries))
	}
}

// ---------------------------------------------------------------------------
// LRU eviction
// ---------------------------------------------------------------------------

func TestCache_EvictsOldestWhenFull(t *testing.T) {
	c := newTestCache(3)

	c.set("a", dummyProgram("a"))
	c.set("b", dummyProgram("b"))
	c.set("c", dummyProgram("c"))

	// Cache is now full (3/3). Adding a 4th should evict "a" (the oldest).
	c.set("d", dummyProgram("d"))

	if _, ok := c.get("a"); ok {
		t.Fatal("expected 'a' to be evicted, but it was still in cache")
	}
	for _, key := range []string{"b", "c", "d"} {
		if _, ok := c.get(key); !ok {
			t.Fatalf("expected '%s' to remain in cache", key)
		}
	}
	if len(c.entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(c.entries))
	}
}

func TestCache_EvictsMultipleToMakeRoom(t *testing.T) {
	c := newTestCache(2)

	c.set("a", dummyProgram("a"))
	c.set("b", dummyProgram("b"))

	// Adding "c" should evict "a"
	c.set("c", dummyProgram("c"))
	if _, ok := c.get("a"); ok {
		t.Fatal("expected 'a' to be evicted")
	}

	// Adding "d" should evict "b"
	c.set("d", dummyProgram("d"))
	if _, ok := c.get("b"); ok {
		t.Fatal("expected 'b' to be evicted")
	}

	if len(c.entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(c.entries))
	}
}

// ---------------------------------------------------------------------------
// LRU ordering – access moves entry to front, protecting from eviction
// ---------------------------------------------------------------------------

func TestCache_AccessPromotesEntry(t *testing.T) {
	c := newTestCache(3)

	c.set("a", dummyProgram("a"))
	c.set("b", dummyProgram("b"))
	c.set("c", dummyProgram("c"))

	// Access "a" – it moves to front and should be protected
	c.get("a")

	// Insert "d" – eviction should remove "b" (now the least-recently used)
	c.set("d", dummyProgram("d"))

	if _, ok := c.get("a"); !ok {
		t.Fatal("expected 'a' to survive after being accessed")
	}
	if _, ok := c.get("b"); ok {
		t.Fatal("expected 'b' to be evicted as the least recently used")
	}
}

func TestCache_SetPromotesExistingEntry(t *testing.T) {
	c := newTestCache(3)

	c.set("a", dummyProgram("a"))
	c.set("b", dummyProgram("b"))
	c.set("c", dummyProgram("c"))

	// Re-set "a" with updated program – should move to front
	c.set("a", dummyProgram("a-updated"))

	// Insert "d" – "b" should be evicted (oldest untouched)
	c.set("d", dummyProgram("d"))

	if _, ok := c.get("a"); !ok {
		t.Fatal("expected 'a' to survive after being re-set")
	}
	if _, ok := c.get("b"); ok {
		t.Fatal("expected 'b' to be evicted")
	}
}

// ---------------------------------------------------------------------------
// Eviction ordering across many entries
// ---------------------------------------------------------------------------

func TestCache_EvictionOrder(t *testing.T) {
	const size = 5
	c := newTestCache(size)

	// Fill cache with scripts 0..4
	for i := 0; i < size; i++ {
		c.set(fmt.Sprintf("s%d", i), dummyProgram(fmt.Sprintf("s%d", i)))
	}

	// Access s1..s4 in ascending order so s4 becomes MRU and s0 stays LRU.
	// LRU order (front→back) after accesses: s4, s3, s2, s1, s0
	for i := 1; i < size; i++ {
		c.get(fmt.Sprintf("s%d", i))
	}

	// Add 3 new entries – should evict s0, then s1, then s2 (LRU order)
	for i := 0; i < 3; i++ {
		c.set(fmt.Sprintf("new%d", i), dummyProgram(fmt.Sprintf("new%d", i)))
	}

	// s0, s1, s2 should be gone
	for i := 0; i < 3; i++ {
		if _, ok := c.get(fmt.Sprintf("s%d", i)); ok {
			t.Fatalf("expected 's%d' to be evicted", i)
		}
	}

	// s3, s4 and all new entries should survive
	for i := 3; i < size; i++ {
		if _, ok := c.get(fmt.Sprintf("s%d", i)); !ok {
			t.Fatalf("expected 's%d' to remain", i)
		}
	}
	for i := 0; i < 3; i++ {
		if _, ok := c.get(fmt.Sprintf("new%d", i)); !ok {
			t.Fatalf("expected 'new%d' to remain", i)
		}
	}
}

// ---------------------------------------------------------------------------
// Edge cases
// ---------------------------------------------------------------------------

func TestCache_MaxSizeOne(t *testing.T) {
	c := newTestCache(1)

	c.set("a", dummyProgram("a"))
	if _, ok := c.get("a"); !ok {
		t.Fatal("expected 'a' in cache")
	}

	c.set("b", dummyProgram("b"))
	if _, ok := c.get("a"); ok {
		t.Fatal("expected 'a' evicted with maxSize=1")
	}
	if _, ok := c.get("b"); !ok {
		t.Fatal("expected 'b' in cache")
	}
}

func TestCache_EmptyEvict(t *testing.T) {
	c := newTestCache(5)
	// evictOldest on empty cache should return false and not panic
	if c.evictOldest() {
		t.Fatal("expected evictOldest on empty cache to return false")
	}
}

func TestCache_LRUListAndMapStayInSync(t *testing.T) {
	c := newTestCache(3)

	c.set("a", dummyProgram("a"))
	c.set("b", dummyProgram("b"))
	c.set("c", dummyProgram("c"))
	c.set("d", dummyProgram("d")) // evicts "a"
	c.set("e", dummyProgram("e")) // evicts "b"

	if c.lru.Len() != len(c.entries) {
		t.Fatalf("lru list length (%d) != map length (%d)", c.lru.Len(), len(c.entries))
	}
	if c.lru.Len() != 3 {
		t.Fatalf("expected 3 entries, got %d", c.lru.Len())
	}
}

// ---------------------------------------------------------------------------
// Hash consistency
// ---------------------------------------------------------------------------

func TestCache_HashConsistency(t *testing.T) {
	script := "x = 42\nprint(x)"
	h1 := hashScript(script)
	h2 := hashScript(script)
	if h1 != h2 {
		t.Fatal("hashScript is not deterministic for the same input")
	}
}

func TestCache_DifferentScriptsDifferentHashes(t *testing.T) {
	h1 := hashScript("script_a")
	h2 := hashScript("script_b")
	if h1 == h2 {
		t.Fatal("different scripts produced the same dual hash (astronomically unlikely)")
	}
}

func TestCache_DualHashIndependence(t *testing.T) {
	// Verify both hash components are populated and different from each other
	key := hashScript("test_independence")
	if key.h1 == 0 && key.h2 == 0 {
		t.Fatal("both hash components are zero")
	}
	// The two hashes use different seeds, so they should differ
	// (not guaranteed but astronomically unlikely for them to match)
	if key.h1 == key.h2 {
		t.Log("warning: h1 == h2 for 'test_independence' (extremely unlikely but not impossible)")
	}
}

// ---------------------------------------------------------------------------
// Global cache API (Get / Set)
// ---------------------------------------------------------------------------

func TestCache_GlobalGetSet(t *testing.T) {
	script := "test_global_cache_api_unique_key_12345"
	prog := dummyProgram("global")

	Set(script, prog)
	got, ok := Get(script)
	if !ok {
		t.Fatal("expected global cache hit")
	}
	if got != prog {
		t.Fatal("global cache returned wrong program")
	}
}

// ---------------------------------------------------------------------------
// Concurrent safety
// ---------------------------------------------------------------------------

func TestCache_ConcurrentAccess(t *testing.T) {
	c := newTestCache(100)
	var wg sync.WaitGroup
	const goroutines = 50
	const opsPerGoroutine = 200

	// Pre-populate a few entries
	for i := 0; i < 20; i++ {
		c.set(fmt.Sprintf("init%d", i), dummyProgram(fmt.Sprintf("init%d", i)))
	}

	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			for i := 0; i < opsPerGoroutine; i++ {
				key := fmt.Sprintf("g%d-i%d", id, i)
				c.set(key, dummyProgram(key))
				c.get(key)
				// Also read an init key to exercise concurrent reads
				c.get(fmt.Sprintf("init%d", i%20))
			}
		}(g)
	}

	wg.Wait()

	// Verify structural integrity: map and list must agree
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.lru.Len() != len(c.entries) {
		t.Fatalf("after concurrent ops: lru len (%d) != map len (%d)", c.lru.Len(), len(c.entries))
	}
	if len(c.entries) > c.maxSize {
		t.Fatalf("cache exceeded maxSize: %d > %d", len(c.entries), c.maxSize)
	}
}

func TestCache_ConcurrentEviction(t *testing.T) {
	c := newTestCache(10)
	var wg sync.WaitGroup
	const goroutines = 20
	const opsPerGoroutine = 100

	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			for i := 0; i < opsPerGoroutine; i++ {
				key := fmt.Sprintf("c%d-%d", id, i)
				c.set(key, dummyProgram(key))
			}
		}(g)
	}

	wg.Wait()

	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.entries) > c.maxSize {
		t.Fatalf("cache exceeded maxSize after concurrent eviction: %d > %d", len(c.entries), c.maxSize)
	}
	if c.lru.Len() != len(c.entries) {
		t.Fatalf("lru/map mismatch: %d vs %d", c.lru.Len(), len(c.entries))
	}
}

// ---------------------------------------------------------------------------
// Stress tests — hammer the cache hard
// ---------------------------------------------------------------------------

func TestCache_StressHighVolume(t *testing.T) {
	c := newTestCache(100)
	const goroutines = 100
	const opsPerGoroutine = 1000
	var wg sync.WaitGroup

	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			for i := 0; i < opsPerGoroutine; i++ {
				script := fmt.Sprintf("script_%d_%d", id, i)
				prog := dummyProgram(script)
				c.set(script, prog)

				// Read back what we just wrote
				if got, ok := c.get(script); ok {
					if got != prog {
						t.Errorf("goroutine %d: got wrong program for %s", id, script)
					}
				}
				// Also read a shared key to exercise contention
				c.get(fmt.Sprintf("script_%d_0", id%goroutines))
			}
		}(g)
	}

	wg.Wait()

	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.entries) > c.maxSize {
		t.Fatalf("cache exceeded maxSize: %d > %d", len(c.entries), c.maxSize)
	}
	if c.lru.Len() != len(c.entries) {
		t.Fatalf("lru/map desync: list=%d map=%d", c.lru.Len(), len(c.entries))
	}

	// Walk the entire LRU list and verify every entry is also in the map
	for elem := c.lru.Front(); elem != nil; elem = elem.Next() {
		entry := elem.Value.(*cacheEntry)
		if _, ok := c.entries[entry.key]; !ok {
			t.Fatalf("LRU entry with key %v not found in map", entry.key)
		}
	}
}

func TestCache_StressConcurrentReadWrite(t *testing.T) {
	// Many concurrent readers and writers on a small cache to stress eviction
	c := newTestCache(10)
	const writers = 20
	const readers = 40
	const ops = 500
	var wg sync.WaitGroup

	wg.Add(writers + readers)

	// Writers: continuously insert new entries, causing constant eviction
	for w := 0; w < writers; w++ {
		go func(id int) {
			defer wg.Done()
			for i := 0; i < ops; i++ {
				script := fmt.Sprintf("w%d_%d", id, i)
				c.set(script, dummyProgram(script))
			}
		}(w)
	}

	// Readers: continuously read, mostly misses but shouldn't panic
	for r := 0; r < readers; r++ {
		go func(id int) {
			defer wg.Done()
			for i := 0; i < ops; i++ {
				script := fmt.Sprintf("w%d_%d", id%writers, i)
				c.get(script)
			}
		}(r)
	}

	wg.Wait()

	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.entries) > c.maxSize {
		t.Fatalf("exceeded maxSize: %d > %d", len(c.entries), c.maxSize)
	}
	if c.lru.Len() != len(c.entries) {
		t.Fatalf("desync after stress: list=%d map=%d", c.lru.Len(), len(c.entries))
	}
}

func TestCache_StressUpdateSameKeys(t *testing.T) {
	// Many goroutines updating the same set of keys
	c := newTestCache(50)
	const goroutines = 50
	const rounds = 500
	const keyCount = 20 // only 20 distinct keys
	var wg sync.WaitGroup

	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			for i := 0; i < rounds; i++ {
				key := fmt.Sprintf("shared_%d", i%keyCount)
				prog := dummyProgram(fmt.Sprintf("v%d_%d", id, i))
				c.set(key, prog)
				c.get(key)
			}
		}(g)
	}

	wg.Wait()

	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.entries) > c.maxSize {
		t.Fatalf("exceeded maxSize: %d > %d", len(c.entries), c.maxSize)
	}
	if c.lru.Len() != len(c.entries) {
		t.Fatalf("desync: list=%d map=%d", c.lru.Len(), len(c.entries))
	}
	// Should have at most keyCount entries since all goroutines share the same keys
	if len(c.entries) > keyCount {
		t.Fatalf("expected at most %d entries (shared keys), got %d", keyCount, len(c.entries))
	}
}

func TestCache_StressRapidEviction(t *testing.T) {
	// Cache of size 1: every insert evicts the previous entry
	c := newTestCache(1)
	const goroutines = 30
	const ops = 1000
	var wg sync.WaitGroup

	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			for i := 0; i < ops; i++ {
				script := fmt.Sprintf("rapid_%d_%d", id, i)
				c.set(script, dummyProgram(script))
				c.get(script) // might hit or miss depending on race
			}
		}(g)
	}

	wg.Wait()

	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.entries) != 1 {
		t.Fatalf("maxSize=1 cache should have exactly 1 entry, got %d", len(c.entries))
	}
	if c.lru.Len() != 1 {
		t.Fatalf("LRU list should have exactly 1 element, got %d", c.lru.Len())
	}
}

func TestCache_HashUniqueness(t *testing.T) {
	// Generate many hashes and verify no collisions in a reasonable set
	const count = 10000
	seen := make(map[cacheKey]string, count)

	for i := 0; i < count; i++ {
		script := fmt.Sprintf("unique_script_number_%d_with_padding", i)
		key := hashScript(script)
		if prev, exists := seen[key]; exists {
			t.Fatalf("dual-hash collision between %q and %q (key: %v)", prev, script, key)
		}
		seen[key] = script
	}
}

func TestCache_StressInterleavedOps(t *testing.T) {
	// Interleave set, get, and eviction in complex patterns
	c := newTestCache(25)
	const goroutines = 40
	const ops = 300
	var wg sync.WaitGroup

	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			for i := 0; i < ops; i++ {
				switch i % 5 {
				case 0, 1, 2:
					// Insert new unique key
					key := fmt.Sprintf("il_%d_%d", id, i)
					c.set(key, dummyProgram(key))
				case 3:
					// Read a potentially evicted key
					key := fmt.Sprintf("il_%d_%d", id, i-3)
					c.get(key)
				case 4:
					// Update a previous key
					key := fmt.Sprintf("il_%d_%d", id, i-4)
					c.set(key, dummyProgram(fmt.Sprintf("updated_%d_%d", id, i)))
				}
			}
		}(g)
	}

	wg.Wait()

	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.entries) > c.maxSize {
		t.Fatalf("exceeded maxSize: %d > %d", len(c.entries), c.maxSize)
	}
	if c.lru.Len() != len(c.entries) {
		t.Fatalf("desync: list=%d map=%d", c.lru.Len(), len(c.entries))
	}

	// Verify every map entry has a matching LRU element
	lruKeys := make(map[cacheKey]bool)
	for elem := c.lru.Front(); elem != nil; elem = elem.Next() {
		entry := elem.Value.(*cacheEntry)
		lruKeys[entry.key] = true
	}
	for key := range c.entries {
		if !lruKeys[key] {
			t.Fatalf("map entry %v not found in LRU list", key)
		}
	}
}
