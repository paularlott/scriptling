package scriptling

import (
	"container/list"
	"fmt"
	"testing"

	"github.com/paularlott/scriptling/ast"
)

type legacyCacheEntry struct {
	key     cacheKey
	program *ast.Program
}

type legacyProgramCache struct {
	entries map[cacheKey]*list.Element
	lru     *list.List
	maxSize int
}

func newLegacyProgramCache(maxSize int) *legacyProgramCache {
	return &legacyProgramCache{
		entries: make(map[cacheKey]*list.Element),
		lru:     list.New(),
		maxSize: maxSize,
	}
}

func (c *legacyProgramCache) get(script string) (*ast.Program, bool) {
	key := hashScript(script)
	elem, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	c.lru.MoveToFront(elem)
	return elem.Value.(*legacyCacheEntry).program, true
}

func (c *legacyProgramCache) set(script string, program *ast.Program) {
	key := hashScript(script)
	if elem, ok := c.entries[key]; ok {
		entry := elem.Value.(*legacyCacheEntry)
		entry.program = program
		c.lru.MoveToFront(elem)
		return
	}
	for len(c.entries) >= c.maxSize {
		elem := c.lru.Back()
		if elem == nil {
			break
		}
		entry := elem.Value.(*legacyCacheEntry)
		c.lru.Remove(elem)
		delete(c.entries, entry.key)
	}
	elem := c.lru.PushFront(&legacyCacheEntry{key: key, program: program})
	c.entries[key] = elem
}

func benchmarkLegacyParseCachedHit(b *testing.B, script string) {
	b.Helper()
	cache := newLegacyProgramCache(1000)
	program, err := parseProgramUncached(script)
	if err != nil {
		b.Fatalf("warmup parse failed: %v", err)
	}
	cache.set(script, program)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		program, ok := cache.get(script)
		if !ok || program == nil {
			b.Fatal("expected cached program")
		}
	}
}

func benchmarkLegacyParseCachedMiss(b *testing.B) {
	cache := newLegacyProgramCache(1000)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		script := fmt.Sprintf("x = %d\n# miss_%d", i, i)
		if program, ok := cache.get(script); ok && program != nil {
			continue
		}
		program, err := parseProgramUncached(script)
		if err != nil {
			b.Fatalf("unexpected parse error: %v", err)
		}
		cache.set(script, program)
	}
}

func benchmarkLegacyParseCachedWorkingSet(b *testing.B) {
	const workingSet = 1500
	scripts := make([]string, workingSet)
	cache := newLegacyProgramCache(1000)
	for i := range scripts {
		scripts[i] = fmt.Sprintf("def f%d(x):\n    return x + %d\nresult = f%d(10)", i, i, i)
		program, err := parseProgramUncached(scripts[i])
		if err != nil {
			b.Fatalf("warmup parse failed: %v", err)
		}
		cache.set(scripts[i], program)
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		script := scripts[i%workingSet]
		if program, ok := cache.get(script); ok && program != nil {
			continue
		}
		program, err := parseProgramUncached(script)
		if err != nil {
			b.Fatalf("unexpected parse error: %v", err)
		}
		cache.set(script, program)
	}
}

func BenchmarkLegacyParseCached_Hit(b *testing.B) {
	benchmarkLegacyParseCachedHit(b, "def add(a, b):\n    return a + b\nresult = add(5, 3)")
}

func BenchmarkLegacyParseCached_Miss(b *testing.B) {
	benchmarkLegacyParseCachedMiss(b)
}

func BenchmarkLegacyParseCached_WorkingSet(b *testing.B) {
	benchmarkLegacyParseCachedWorkingSet(b)
}
