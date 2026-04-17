package scriptling

import (
	"container/list"
	"hash/maphash"
	"sync"

	"github.com/paularlott/scriptling/ast"
)

// cacheKey is a dual-hash key providing 128-bit collision resistance.
// Two independent maphash seeds produce two 64-bit hashes; a false match
// requires both to collide simultaneously (probability ~2^-128).
type cacheKey struct {
	h1 uint64
	h2 uint64
}

type cacheEntry struct {
	key     cacheKey
	program *ast.Program
}

type programCache struct {
	mu      sync.RWMutex
	entries map[cacheKey]*list.Element
	lru     *list.List
	maxSize int
}

var globalCache = &programCache{
	entries: make(map[cacheKey]*list.Element),
	lru:     list.New(),
	maxSize: 1000, // Max 1000 cached programs
}

// Get retrieves a cached program by script content
func Get(script string) (*ast.Program, bool) {
	return globalCache.get(script)
}

// GetKey retrieves the cache key and cached program by script content.
func GetKey(script string) (cacheKey, *ast.Program, bool) {
	return globalCache.getWithKey(script)
}

// Set stores a program in the cache by script content
func Set(script string, program *ast.Program) {
	globalCache.set(script, program)
}

// SetWithKey stores a program in the cache using a previously computed key.
func SetWithKey(key cacheKey, program *ast.Program) {
	globalCache.setWithKey(key, program)
}

func (c *programCache) get(script string) (*ast.Program, bool) {
	_, program, ok := c.getWithKey(script)
	return program, ok
}

func (c *programCache) getWithKey(script string) (cacheKey, *ast.Program, bool) {
	key := hashScript(script)

	// Fast path: read lock for lookup
	c.mu.RLock()
	elem, ok := c.entries[key]
	if !ok {
		c.mu.RUnlock()
		return key, nil, false
	}
	program := elem.Value.(*cacheEntry).program
	c.mu.RUnlock()

	// Promote under write lock (best-effort; skip if contended)
	if c.mu.TryLock() {
		if elem, ok := c.entries[key]; ok {
			c.lru.MoveToFront(elem)
		}
		c.mu.Unlock()
	}

	return key, program, true
}

func (c *programCache) set(script string, program *ast.Program) {
	c.setWithKey(hashScript(script), program)
}

func (c *programCache) setWithKey(key cacheKey, program *ast.Program) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if already exists (same dual-hash = same script)
	if elem, ok := c.entries[key]; ok {
		entry := elem.Value.(*cacheEntry)
		c.lru.MoveToFront(elem)
		entry.program = program
		return
	}

	// Evict old entries if cache is full
	for len(c.entries) >= c.maxSize {
		if !c.evictOldest() {
			break
		}
	}

	// Add new entry at front (after potential eviction)
	entry := &cacheEntry{
		key:     key,
		program: program,
	}
	// Push to front of LRU list and update map
	elem := c.lru.PushFront(entry)
	c.entries[key] = elem
}

// Two independent seeds for dual-hash collision resistance
var (
	hashSeed1 = maphash.MakeSeed()
	hashSeed2 = maphash.MakeSeed()
)

func hashScript(script string) cacheKey {
	var h1, h2 maphash.Hash
	h1.SetSeed(hashSeed1)
	h1.WriteString(script)
	h2.SetSeed(hashSeed2)
	h2.WriteString(script)
	return cacheKey{h1: h1.Sum64(), h2: h2.Sum64()}
}

func (c *programCache) evictOldest() bool {
	// Get oldest entry (at back of list)
	elem := c.lru.Back()
	if elem == nil {
		return false
	}

	entry := elem.Value.(*cacheEntry)

	// Remove oldest entry (pure LRU, no time-based protection)
	c.lru.Remove(elem)
	delete(c.entries, entry.key)
	return true
}
