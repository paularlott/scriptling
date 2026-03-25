package memory

import (
	"testing"
	"time"

	"github.com/paularlott/snapshotkv"
)

func newTestDB(t *testing.T) *snapshotkv.DB {
	t.Helper()
	// Use t.TempDir() which automatically cleans up when the test completes
	dir := t.TempDir()
	db, err := snapshotkv.Open(dir, nil)
	if err != nil {
		t.Fatalf("snapshotkv.Open: %v", err)
	}
	t.Cleanup(func() {
		storeRegistry.mu.Lock()
		delete(storeRegistry.stores, db)
		storeRegistry.mu.Unlock()
		db.Close()
	})
	return db
}

// TestRegistry_SameDBReturnsSameStore verifies that getOrCreateStore returns
// the identical Store pointer for the same DB, simulating multiple memory.new(db)
// calls within a process.
func TestRegistry_SameDBReturnsSameStore(t *testing.T) {
	db := newTestDB(t)
	s1 := getOrCreateStore(db, nil)
	s2 := getOrCreateStore(db, nil)
	if s1 != s2 {
		t.Error("expected same Store instance for same DB pointer")
	}
}

// TestRegistry_DifferentDBReturnsDifferentStore verifies isolation between stores.
func TestRegistry_DifferentDBReturnsDifferentStore(t *testing.T) {
	db1 := newTestDB(t)
	db2 := newTestDB(t)
	s1 := getOrCreateStore(db1, nil)
	s2 := getOrCreateStore(db2, nil)
	if s1 == s2 {
		t.Error("expected different Store instances for different DB pointers")
	}
}

// TestRegistry_MemoriesPersistAcrossCalls verifies that memories persist
// across multiple getOrCreateStore calls (simulating repeated MCP tool invocations).
func TestRegistry_MemoriesPersistAcrossCalls(t *testing.T) {
	db := newTestDB(t)

	// Simulate 3 separate "tool invocations" each calling memory.new(db)
	// Use distinct content so pre-flight dedup doesn't collapse them.
	contents := []string{"alice visited paris", "bob likes cycling", "carol prefers tea"}
	for _, c := range contents {
		s := getOrCreateStore(db, nil)
		s.Remember(c, TypeNote, 0.5)
	}

	s := getOrCreateStore(db, nil)
	if s.Count() != 3 {
		t.Errorf("Count = %d, want 3 — memories not persisting across calls", s.Count())
	}
}

// TestRegistry_CompactionCanBeTriggeredManually verifies that Compact() can be
// called to prune old memories.
func TestRegistry_CompactionCanBeTriggeredManually(t *testing.T) {
	db := newTestDB(t)
	s := getOrCreateStore(db, []Option{
		WithMaxAge(time.Millisecond),
		WithPruneThreshold(0.0),
	})

	// Use distinct content so pre-flight dedup doesn't collapse them.
	contents := []string{"alice visited paris", "bob likes cycling", "carol prefers tea"}
	for _, c := range contents {
		getOrCreateStore(db, nil).Remember(c, TypeNote, 0.5)
	}

	// Age the memories by updating their AccessedAt to the past
	// First collect IDs, then update (avoiding lock during scan)
	past := time.Now().UTC().Add(-time.Hour)
	s.mu.RLock()
	var ids []string
	s.scanType("", func(m *Memory) bool {
		ids = append(ids, m.ID)
		return true
	})
	s.mu.RUnlock()

	// Now update each memory
	for _, id := range ids {
		s.mu.Lock()
		val, err := s.db.Get(idxPrefix + id)
		if err != nil {
			s.mu.Unlock()
			continue
		}
		key, _ := val.(string)
		raw, err := s.db.Get(key)
		if err != nil {
			s.mu.Unlock()
			continue
		}
		m := toMemory(raw)
		if m != nil {
			m.AccessedAt = past
			_ = s.save(m)
		}
		s.mu.Unlock()
	}

	remaining := s.Compact()
	if remaining != 0 {
		t.Errorf("remaining = %d after compaction, want 0", remaining)
	}
}

// TestRegistry_NoKVStateForCounter verifies that the memory count is derived
// from the actual stored data, not a separate counter.
func TestRegistry_NoKVStateForCounter(t *testing.T) {
	dir := t.TempDir()

	db1, err := snapshotkv.Open(dir, &snapshotkv.Config{SaveDebounce: 0})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	s1 := New(db1)
	// Use distinct content so they don't merge
	for i := 0; i < 5; i++ {
		s1.Remember("unique item", TypeNote, 0.5)
	}
	db1.Close()

	// Re-open same path — new Store should see the persisted memories.
	db2, err := snapshotkv.Open(dir, nil)
	if err != nil {
		t.Fatalf("Reopen: %v", err)
	}
	defer db2.Close()
	s2 := New(db2)

	// Pre-flight dedup will merge identical content, so we get 1 memory
	if s2.Count() != 1 {
		t.Errorf("Count = %d on fresh Store, want 1", s2.Count())
	}
}

// TestCompact_ResetsStateAndPrunes verifies that Compact() runs synchronously
// and prunes eligible memories.
func TestCompact_ResetsStateAndPrunes(t *testing.T) {
	db := newTestDB(t)
	s := getOrCreateStore(db, []Option{
		WithMaxAge(time.Millisecond),
		WithPruneThreshold(0.0),
	})

	// Add some memories then age them past maxAge
	var ids []string
	for i := 0; i < 5; i++ {
		m, _ := s.Remember("old item", TypeNote, 0.5)
		ids = append(ids, m.ID)
	}

	// Age each memory individually without holding the store lock during save
	past := time.Now().UTC().Add(-time.Hour)
	for _, id := range ids {
		s.mu.Lock()
		val, err := s.db.Get(idxPrefix + id)
		if err != nil {
			s.mu.Unlock()
			continue
		}
		key, _ := val.(string)
		s.mu.Unlock()

		// Read the memory, update AccessedAt, re-save
		s.mu.RLock()
		raw, err := s.db.Get(key)
		s.mu.RUnlock()
		if err != nil {
			continue
		}
		m := toMemory(raw)
		if m == nil {
			continue
		}
		m.AccessedAt = past
		s.mu.Lock()
		_ = s.save(m)
		s.mu.Unlock()
	}

	before := s.Count()
	remaining := s.Compact()

	if remaining != 0 {
		t.Errorf("expected 0 remaining after compact, got %d (before=%d)", remaining, before)
	}
}

// TestCompact_CanBeCalledMultipleTimes verifies Compact() is safe to call repeatedly.
func TestCompact_CanBeCalledMultipleTimes(t *testing.T) {
	db := newTestDB(t)
	s := getOrCreateStore(db, nil)

	// Should be safe to call Compact multiple times
	s.Compact()
	s.Compact()
}
