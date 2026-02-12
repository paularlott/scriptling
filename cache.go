package scriptling

import (
	"container/list"
	"hash/maphash"
	"sync"
	"time"

	"github.com/paularlott/scriptling/ast"
)

type cacheEntry struct {
	key      uint64
	program  *ast.Program
	lastUsed time.Time
}

type programCache struct {
	mu      sync.RWMutex
	entries map[uint64]*list.Element
	lru     *list.List
	maxSize int
}

var globalCache = &programCache{
	entries: make(map[uint64]*list.Element),
	lru:     list.New(),
	maxSize: 1000, // Max 1000 cached programs
}

// Get retrieves a cached program by script content
func Get(script string) (*ast.Program, bool) {
	return globalCache.get(script)
}

// Set stores a program in the cache by script content
func Set(script string, program *ast.Program) {
	globalCache.set(script, program)
}

func (c *programCache) get(script string) (*ast.Program, bool) {
	key := hashScript(script)

	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.entries[key]
	if !ok {
		return nil, false
	}

	// Move to front (most recently used)
	c.lru.MoveToFront(elem)
	entry := elem.Value.(*cacheEntry)
	entry.lastUsed = time.Now()

	return entry.program, true
}

func (c *programCache) set(script string, program *ast.Program) {
	key := hashScript(script)

	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if already exists
	if elem, ok := c.entries[key]; ok {
		c.lru.MoveToFront(elem)
		entry := elem.Value.(*cacheEntry)
		entry.program = program
		entry.lastUsed = time.Now()
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
		key:      key,
		program:  program,
		lastUsed: time.Now(),
	}
	// Push to front of LRU list and update map
	elem := c.lru.PushFront(entry)
	c.entries[key] = elem
}

var hashSeed = maphash.MakeSeed()

func hashScript(script string) uint64 {
	var h maphash.Hash
	h.SetSeed(hashSeed)
	h.WriteString(script)
	return h.Sum64()
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
