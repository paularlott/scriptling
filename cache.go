package scriptling

import (
	"container/list"
	"hash/maphash"
	"sync"
	"sync/atomic"

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
	key       cacheKey
	program   *ast.Program
	sizeBytes int
}

type cacheStats struct {
	evictions atomic.Uint64
	grows     atomic.Uint64
}

type programCache struct {
	mu         sync.RWMutex
	entries    map[cacheKey]*list.Element
	lru        *list.List
	maxSize    int
	maxSizeCap int
	maxBytes   int
	usedBytes  int
	stats      cacheStats
}

const (
	defaultCacheMaxEntries = 1000
	defaultCacheMaxCap     = 4000
	defaultCacheMaxBytes   = 64 << 20
)

func newProgramCache(maxSize int) *programCache {
	if maxSize < 1 {
		maxSize = 1
	}
	maxCap := maxSize * 4
	if maxCap < maxSize {
		maxCap = maxSize
	}
	if maxCap > defaultCacheMaxCap {
		maxCap = defaultCacheMaxCap
	}
	if maxCap < maxSize {
		maxCap = maxSize
	}
	return &programCache{
		entries:    make(map[cacheKey]*list.Element),
		lru:        list.New(),
		maxSize:    maxSize,
		maxSizeCap: maxCap,
		maxBytes:   defaultCacheMaxBytes,
	}
}

var globalCache = newProgramCache(defaultCacheMaxEntries)

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
		return key, nil, false
	}
	entry := elem.Value.(*cacheEntry)
	program := entry.program
	c.mu.RUnlock()

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
	c.maybeGrowLocked(entry.sizeBytes)
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
	c.maybeGrowLocked(entry.sizeBytes)
	elem := c.lru.PushFront(entry)
	c.entries[key] = elem
	c.usedBytes += entry.sizeBytes
	c.evictIfNeededLocked()
}

func (c *programCache) maybeGrowLocked(nextSize int) {
	if len(c.entries) < c.maxSize || c.maxSize >= c.maxSizeCap {
		return
	}
	if c.maxBytes > 0 && c.usedBytes+nextSize > c.maxBytes {
		return
	}
	growBy := c.maxSize / 4
	if growBy < 64 {
		growBy = 64
	}
	c.maxSize += growBy
	if c.maxSize > c.maxSizeCap {
		c.maxSize = c.maxSizeCap
	}
	c.stats.grows.Add(1)
}

func (c *programCache) evictIfNeededLocked() {
	for len(c.entries) > c.maxSize || (c.maxBytes > 0 && c.usedBytes > c.maxBytes) {
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
	h2.SetSeed(hashSeed2)
	writeCacheKeyMaterial(&h1, script)
	writeCacheKeyMaterial(&h2, script)
	return cacheKey{h1: h1.Sum64(), h2: h2.Sum64()}
}

func writeCacheKeyMaterial(h *maphash.Hash, script string) {
	const sampleSize = 64
	const fullHashThreshold = sampleSize * 3

	var lenBuf [8]byte
	n := len(script)
	for i := range lenBuf {
		lenBuf[i] = byte(n >> (i * 8))
	}
	_, _ = h.Write(lenBuf[:])

	if n <= fullHashThreshold {
		h.WriteString(script)
		return
	}

	h.WriteString(script[:sampleSize])

	midStart := n/2 - sampleSize/2
	if midStart < sampleSize {
		midStart = sampleSize
	}
	if midStart+sampleSize > n-sampleSize {
		midStart = n - (sampleSize * 2)
	}
	h.WriteString(script[midStart : midStart+sampleSize])
	h.WriteString(script[n-sampleSize:])
}

func estimateCacheEntrySize(script string, program *ast.Program) int {
	_ = script
	const entryOverhead = 128
	return ast.EstimateRetainedBytes(program, "") + entryOverhead
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
