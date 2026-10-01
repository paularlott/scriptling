package scriptling

import (
	"container/list"
	"hash/maphash"
	"sync"
	"sync/atomic"

	"github.com/paularlott/scriptling/ast"
)

// cacheKey provides collision resistance via script length + dual 64-bit hash.
// Different-length scripts never share a key; same-length scripts need both
// hashes to collide simultaneously (probability ~2^-128).
type cacheKey struct {
	length int
	h1     uint64
	h2     uint64
}

type cacheEntry struct {
	key       cacheKey
	program   *ast.Program
	sizeBytes int
}

type cacheStats struct {
	evictions atomic.Uint64
	hits      atomic.Uint64
	misses    atomic.Uint64
}

// DefaultProgramCacheMaxBytes is the default memory budget of the program
// cache: 64 MiB of parsed and compiled scripts.
const DefaultProgramCacheMaxBytes = defaultCacheMaxBytes

// ProgramCacheStats describes the process-wide program cache at one moment.
type ProgramCacheStats struct {
	Entries   int    // programs currently cached
	UsedBytes int    // estimated memory retained by those programs
	MaxBytes  int    // configured budget; 0 means no byte limit
	Hits      uint64 // lookups that found a cached program
	Misses    uint64 // lookups that had to parse and compile
	Evictions uint64 // programs dropped to stay within the limits
}

// SetProgramCacheMaxBytes sets the memory budget, in bytes, of the
// process-wide cache that holds every parsed and compiled script so repeated
// evaluations skip parsing. The default is DefaultProgramCacheMaxBytes
// (64 MiB). A budget of 0 makes the cache unbounded; negative values are
// treated as 0. Lowering the budget evicts the least recently used programs
// immediately. A program that no longer fits is simply parsed again on its
// next use, so an undersized budget costs time, never correctness.
func SetProgramCacheMaxBytes(maxBytes int) {
	globalCache.setMaxBytes(maxBytes)
}

// ProgramCacheMaxBytes returns the program cache's current byte budget, 0 when
// unlimited.
func ProgramCacheMaxBytes() int {
	return globalCache.maxBytesLimit()
}

// GetProgramCacheStats returns a snapshot of the program cache. Hit and miss
// counts accumulate for the life of the process and are the quickest way to
// tell whether the budget suits the set of scripts a host runs.
func GetProgramCacheStats() ProgramCacheStats {
	return globalCache.snapshot()
}

// programCache is an LRU of parsed and compiled programs bounded by one
// number: the estimated bytes they retain (see estimateCacheEntrySize).
type programCache struct {
	mu        sync.RWMutex
	entries   map[cacheKey]*list.Element
	lru       *list.List
	maxBytes  int // 0 means unbounded
	usedBytes int
	stats     cacheStats
}

const defaultCacheMaxBytes = 64 << 20

func newProgramCache(maxBytes int) *programCache {
	if maxBytes < 0 {
		maxBytes = 0
	}
	return &programCache{
		entries:  make(map[cacheKey]*list.Element),
		lru:      list.New(),
		maxBytes: maxBytes,
	}
}

var globalCache = newProgramCache(defaultCacheMaxBytes)

// Get retrieves a cached program by script content.
func Get(script string) (*ast.Program, bool) {
	return globalCache.get(script)
}

// GetKey retrieves the cache key and cached program by script content.
// For normal cache entries this avoids hashing the full script on a miss.
func GetKey(script string) (cacheKey, *ast.Program, bool) {
	return globalCache.getWithKey(script)
}

// Set stores a program in the cache by script content.
func Set(script string, program *ast.Program) {
	globalCache.set(script, program)
}

// SetWithKey stores a program in the cache using a previously computed key.
// This path is kept for compatibility; the normal parser path now uses Set().
func SetWithKey(key cacheKey, script string, program *ast.Program) {
	globalCache.setWithKey(key, script, program)
}

func (c *programCache) get(script string) (*ast.Program, bool) {
	_, program, ok := c.getWithKey(script)
	return program, ok
}

func (c *programCache) getWithKey(script string) (cacheKey, *ast.Program, bool) {
	key := hashScript(script)
	c.mu.RLock()
	elem, ok := c.entries[key]
	if !ok {
		c.mu.RUnlock()
		c.stats.misses.Add(1)
		return key, nil, false
	}
	entry := elem.Value.(*cacheEntry)
	program := entry.program
	c.mu.RUnlock()
	c.stats.hits.Add(1)

	if c.mu.TryLock() {
		if current, ok := c.entries[key]; ok {
			c.lru.MoveToFront(current)
		}
		c.mu.Unlock()
	}
	return key, program, true
}

func (c *programCache) set(script string, program *ast.Program) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := hashScript(script)
	if elem, ok := c.entries[key]; ok {
		entry := elem.Value.(*cacheEntry)
		c.usedBytes -= entry.sizeBytes
		entry.program = program
		entry.sizeBytes = estimateCacheEntrySize(script, program)
		c.usedBytes += entry.sizeBytes
		c.lru.MoveToFront(elem)
		c.evictIfNeededLocked()
		return
	}

	entry := &cacheEntry{
		key:       key,
		program:   program,
		sizeBytes: estimateCacheEntrySize(script, program),
	}
	elem := c.lru.PushFront(entry)
	c.entries[key] = elem
	c.usedBytes += entry.sizeBytes
	c.evictIfNeededLocked()
}

func (c *programCache) setWithKey(key cacheKey, script string, program *ast.Program) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.entries[key]; ok {
		entry := elem.Value.(*cacheEntry)
		c.usedBytes -= entry.sizeBytes
		entry.program = program
		entry.sizeBytes = estimateCacheEntrySize(script, program)
		c.usedBytes += entry.sizeBytes
		c.lru.MoveToFront(elem)
		c.evictIfNeededLocked()
		return
	}

	entry := &cacheEntry{
		key:       key,
		program:   program,
		sizeBytes: estimateCacheEntrySize(script, program),
	}
	elem := c.lru.PushFront(entry)
	c.entries[key] = elem
	c.usedBytes += entry.sizeBytes
	c.evictIfNeededLocked()
}

func (c *programCache) evictIfNeededLocked() {
	for c.maxBytes > 0 && c.usedBytes > c.maxBytes {
		if !c.evictOldestLocked() {
			return
		}
	}
}

// Two independent seeds for dual-hash collision resistance.
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
	return cacheKey{length: len(script), h1: h1.Sum64(), h2: h2.Sum64()}
}

// estimateCacheEntrySize is the retained cost of one cache entry: the AST
// plus the evaluator's compiled closure tree, which EstimateRetainedBytes
// includes, plus the entry bookkeeping.
func estimateCacheEntrySize(script string, program *ast.Program) int {
	_ = script
	const entryOverhead = 128
	return ast.EstimateRetainedBytes(program, "") + entryOverhead
}

// setMaxBytes applies a new byte budget and evicts down to it at once.
func (c *programCache) setMaxBytes(maxBytes int) {
	if maxBytes < 0 {
		maxBytes = 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.maxBytes = maxBytes
	c.evictIfNeededLocked()
}

func (c *programCache) maxBytesLimit() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.maxBytes
}

func (c *programCache) snapshot() ProgramCacheStats {
	c.mu.RLock()
	s := ProgramCacheStats{
		Entries:   len(c.entries),
		UsedBytes: c.usedBytes,
		MaxBytes:  c.maxBytes,
	}
	c.mu.RUnlock()
	s.Hits = c.stats.hits.Load()
	s.Misses = c.stats.misses.Load()
	s.Evictions = c.stats.evictions.Load()
	return s
}

func (c *programCache) evictOldest() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.evictOldestLocked()
}

func (c *programCache) evictOldestLocked() bool {
	elem := c.lru.Back()
	if elem == nil {
		return false
	}

	entry := elem.Value.(*cacheEntry)
	c.lru.Remove(elem)
	delete(c.entries, entry.key)
	c.usedBytes -= entry.sizeBytes
	c.stats.evictions.Add(1)
	return true
}
