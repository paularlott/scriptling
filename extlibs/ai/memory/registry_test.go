package memory

import (
	"testing"
	"time"

	"github.com/paularlott/snapshotkv"
)

func newTestDB(t *testing.T) *snapshotkv.DB {
	t.Helper()
	db, err := snapshotkv.Open("", nil)
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

// TestRegistry_CounterAccumulatesAcrossCalls verifies that memoriesSinceCompact
// accumulates across multiple getOrCreateStore calls (simulating repeated MCP tool
// invocations), rather than resetting each time.
func TestRegistry_CounterAccumulatesAcrossCalls(t *testing.T) {
	db := newTestDB(t)

	// Simulate 3 separate "tool invocations" each calling memory.new(db)
	// Use distinct content so pre-flight dedup doesn't collapse them.
	contents := []string{"alice visited paris", "bob likes cycling", "carol prefers tea"}
	for _, c := range contents {
		s := getOrCreateStore(db, nil)
		s.Remember(c, TypeNote, 0.5)
	}

	s := getOrCreateStore(db, nil)
	s.mu.Lock()
	count := s.memoriesSinceCompact
	s.mu.Unlock()

	if count != 3 {
		t.Errorf("memoriesSinceCompact = %d, want 3 — counter not accumulating across calls", count)
	}
}

// TestRegistry_CounterResetsAfterCompaction verifies that after compaction triggers,
// the counter resets to zero on the shared Store instance.
func TestRegistry_CounterResetsAfterCompaction(t *testing.T) {
	db := newTestDB(t)
	s := getOrCreateStore(db, []Option{
		WithActivityThreshold(3),
		WithMinCompactInterval(0),
		WithMaxCompactInterval(24 * time.Hour),
		WithMaxAge(time.Millisecond),
	})
	s.lastCompaction = time.Now().Add(-time.Hour)

	// Use distinct content so pre-flight dedup doesn't collapse them.
	contents := []string{"alice visited paris", "bob likes cycling", "carol prefers tea"}
	for _, c := range contents {
		getOrCreateStore(db, nil).Remember(c, TypeNote, 0.5)
	}
	time.Sleep(50 * time.Millisecond)

	s.mu.Lock()
	count := s.memoriesSinceCompact
	s.mu.Unlock()
	if count != 0 {
		t.Errorf("memoriesSinceCompact = %d after compaction, want 0", count)
	}
}

// TestRegistry_NoKVStateForCounter verifies that the compaction counter is purely
// in-process — a new Store on the same DB path starts at zero.
func TestRegistry_NoKVStateForCounter(t *testing.T) {
	dir := t.TempDir()

	db1, err := snapshotkv.Open(dir, &snapshotkv.Config{SaveDebounce: 0})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	s1 := New(db1)
	for i := 0; i < 5; i++ {
		s1.Remember("item", TypeNote, 0.5)
	}
	db1.Close()

	// Re-open same path — new Store should start counter at zero.
	db2, err := snapshotkv.Open(dir, nil)
	if err != nil {
		t.Fatalf("Reopen: %v", err)
	}
	defer db2.Close()
	s2 := New(db2)

	s2.mu.Lock()
	count := s2.memoriesSinceCompact
	s2.mu.Unlock()
	if count != 0 {
		t.Errorf("memoriesSinceCompact = %d on fresh Store, want 0", count)
	}
}

// TestCompact_ResetsStateAndPrunes verifies that Compact() runs synchronously,
// resets the activity counter and timer, and prunes eligible memories.
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

	s.mu.Lock()
	s.memoriesSinceCompact = 5
	s.mu.Unlock()

	before := s.Count()
	remaining := s.Compact()

	if remaining != 0 {
		t.Errorf("expected 0 remaining after compact, got %d (before=%d)", remaining, before)
	}

	s.mu.Lock()
	counter := s.memoriesSinceCompact
	s.mu.Unlock()
	if counter != 0 {
		t.Errorf("memoriesSinceCompact = %d after Compact(), want 0", counter)
	}
}

// TestCompact_NoDoubleRun verifies that a second concurrent Compact() call is a no-op.
func TestCompact_NoDoubleRun(t *testing.T) {
	db := newTestDB(t)
	s := getOrCreateStore(db, nil)
	s.compactionInProgress.Store(true)

	result := s.Compact()
	if result != 0 {
		t.Errorf("expected 0 from skipped Compact(), got %d", result)
	}
	// Restore so cleanup doesn't hang
	s.compactionInProgress.Store(false)
}
