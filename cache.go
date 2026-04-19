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

type cacheSignature struct {
	length int
	head   uint64
	tail   uint64
}

type cacheEntry struct {
	key       cacheKey
	signature cacheSignature
	script    string
	program   *ast.Program
	sizeBytes int
	hashOnly  bool
}

type cacheStats struct {
	evictions atomic.Uint64
	grows     atomic.Uint64
}

type programCache struct {
	mu              sync.RWMutex
	entries         map[cacheKey]*list.Element
	buckets         map[cacheSignature][]*list.Element
	lru             *list.List
	maxSize         int
	maxSizeCap      int
	maxBytes        int
	usedBytes       int
	hashOnlyEntries int
	stats           cacheStats
}

const (
	defaultCacheMaxEntries = 1000
	defaultCacheMaxCap     = 4000
	defaultCacheMaxBytes   = 64 << 20
	smallScriptThreshold   = 32
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
		buckets:    make(map[cacheSignature][]*list.Element),
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
func SetWithKey(key cacheKey, program *ast.Program) {
	globalCache.setWithKey(key, program)
}

func (c *programCache) get(script string) (*ast.Program, bool) {
	_, program, ok := c.getWithKey(script)
	return program, ok
}

func (c *programCache) getWithKey(script string) (cacheKey, *ast.Program, bool) {
	if len(script) <= smallScriptThreshold {
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

	sig := signatureForScript(script)
	c.mu.RLock()
	if elems, ok := c.buckets[sig]; ok {
		for _, elem := range elems {
			entry := elem.Value.(*cacheEntry)
			if entry.script == script {
				program := entry.program
				key := entry.key
				c.mu.RUnlock()
				if c.mu.TryLock() {
					if current, ok := c.entries[key]; ok {
						c.lru.MoveToFront(current)
					}
					c.mu.Unlock()
				}
				return key, program, true
			}
		}
	}

	hashOnlyEntries := c.hashOnlyEntries
	c.mu.RUnlock()

	if hashOnlyEntries == 0 {
		return cacheKey{}, nil, false
	}

	key := hashScript(script)
	c.mu.RLock()
	elem, ok := c.entries[key]
	if !ok {
		c.mu.RUnlock()
		return cacheKey{}, nil, false
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

	sig := signatureForScript(script)

	if len(script) <= smallScriptThreshold {
		key := hashScript(script)
		if elem, ok := c.entries[key]; ok {
			entry := elem.Value.(*cacheEntry)
			c.usedBytes -= entry.sizeBytes
			entry.script = script
			entry.program = program
			entry.sizeBytes = estimateCacheEntrySize(script)
			entry.hashOnly = false
			c.usedBytes += entry.sizeBytes
			c.lru.MoveToFront(elem)
			if c.hashOnlyEntries > 0 {
				c.hashOnlyEntries--
			}
			c.evictIfNeededLocked()
			return
		}

		entry := &cacheEntry{
			key:       key,
			script:    script,
			program:   program,
			sizeBytes: estimateCacheEntrySize(script),
		}
		c.maybeGrowLocked(entry.sizeBytes)
		elem := c.lru.PushFront(entry)
		c.entries[key] = elem
		c.usedBytes += entry.sizeBytes
		c.evictIfNeededLocked()
		return
	}

	if elems, ok := c.buckets[sig]; ok {
		for _, elem := range elems {
			entry := elem.Value.(*cacheEntry)
			if entry.script == script {
				entry.program = program
				c.lru.MoveToFront(elem)
				return
			}
		}
	}

	key := hashScript(script)
	if elem, ok := c.entries[key]; ok {
		entry := elem.Value.(*cacheEntry)
		c.usedBytes -= entry.sizeBytes
		entry.script = script
		entry.signature = sig
		entry.program = program
		entry.sizeBytes = estimateCacheEntrySize(script)
		entry.hashOnly = false
		c.usedBytes += entry.sizeBytes
		c.lru.MoveToFront(elem)
		c.addBucketElem(sig, elem)
		if c.hashOnlyEntries > 0 {
			c.hashOnlyEntries--
		}
		c.evictIfNeededLocked()
		return
	}

	entry := &cacheEntry{
		key:       key,
		signature: sig,
		script:    script,
		program:   program,
		sizeBytes: estimateCacheEntrySize(script),
	}
	c.maybeGrowLocked(entry.sizeBytes)
	elem := c.lru.PushFront(entry)
	c.entries[key] = elem
	c.addBucketElem(sig, elem)
	c.usedBytes += entry.sizeBytes
	c.evictIfNeededLocked()
}

func (c *programCache) setWithKey(key cacheKey, program *ast.Program) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.entries[key]; ok {
		entry := elem.Value.(*cacheEntry)
		entry.program = program
		c.lru.MoveToFront(elem)
		return
	}

	entry := &cacheEntry{
		key:       key,
		program:   program,
		sizeBytes: 128,
		hashOnly:  true,
	}
	c.maybeGrowLocked(entry.sizeBytes)
	elem := c.lru.PushFront(entry)
	c.entries[key] = elem
	c.usedBytes += entry.sizeBytes
	c.hashOnlyEntries++
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

func (c *programCache) addBucketElem(sig cacheSignature, elem *list.Element) {
	elems := c.buckets[sig]
	for _, existing := range elems {
		if existing == elem {
			return
		}
	}
	c.buckets[sig] = append(elems, elem)
}

func (c *programCache) removeBucketElem(sig cacheSignature, elem *list.Element) {
	elems := c.buckets[sig]
	for i, existing := range elems {
		if existing != elem {
			continue
		}
		last := len(elems) - 1
		elems[i] = elems[last]
		elems = elems[:last]
		if len(elems) == 0 {
			delete(c.buckets, sig)
		} else {
			c.buckets[sig] = elems
		}
		return
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
	return cacheKey{h1: h1.Sum64(), h2: h2.Sum64()}
}

func signatureForScript(script string) cacheSignature {
	return cacheSignature{
		length: len(script),
		head:   packedEdgeBytes(script, 0),
		tail:   packedEdgeBytes(script, len(script)-8),
	}
}

func packedEdgeBytes(script string, start int) uint64 {
	if start < 0 {
		start = 0
	}
	if start > len(script) {
		start = len(script)
	}
	end := start + 8
	if end > len(script) {
		end = len(script)
	}
	var out uint64
	for i := start; i < end; i++ {
		out = (out << 8) | uint64(script[i])
	}
	return out
}

func estimateCacheEntrySize(script string) int {
	return len(script) + 128
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
	if !entry.hashOnly {
		c.removeBucketElem(entry.signature, elem)
	} else if c.hashOnlyEntries > 0 {
		c.hashOnlyEntries--
	}
	c.stats.evictions.Add(1)
	return true
}
