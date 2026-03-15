package memory

import (
	"testing"
	"time"

	"github.com/paularlott/snapshotkv"
)

func newTestStore(t *testing.T) (*Store, func()) {
	t.Helper()
	db, err := snapshotkv.Open("", nil)
	if err != nil {
		t.Fatalf("snapshotkv.Open: %v", err)
	}
	store := New(db, 0) // no background compaction in tests
	return store, func() {
		store.Close()
		db.Close()
	}
}

func TestRememberAndRecall(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	m, err := store.Remember("User's name is Alice", TypeFact, 0.9)
	if err != nil {
		t.Fatalf("Remember: %v", err)
	}
	if m.ID == "" {
		t.Fatal("expected non-empty ID")
	}

	results := store.Recall("Alice", 1, "")
	if len(results) == 0 {
		t.Fatal("Recall returned no results")
	}
	got := results[0]
	if got.Content != "User's name is Alice" {
		t.Errorf("content = %q, want %q", got.Content, "User's name is Alice")
	}
	if got.Type != TypeFact {
		t.Errorf("type = %q, want %q", got.Type, TypeFact)
	}
}

func TestRecall_Missing(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	results := store.Recall("no_such_content", 1, "")
	if len(results) != 0 {
		t.Errorf("expected no results, got %+v", results)
	}
}

func TestRecall_UpdatesAccessedAt(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	before := time.Now().UTC().Add(-time.Second)
	store.Remember("test content", TypeNote, 0.5)
	results := store.Recall("test", 1, "")
	if len(results) == 0 {
		t.Fatal("Recall returned no results")
	}
	if !results[0].AccessedAt.After(before) {
		t.Error("AccessedAt should be updated on recall")
	}
}

func TestRecall_KeywordMatch(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	store.Remember("User prefers dark mode", TypePreference, 0.7)
	store.Remember("API rate limit is 1000 per day", TypeFact, 0.9)
	store.Remember("Deployed version 2.1 on Friday", TypeEvent, 0.5)

	results := store.Recall("dark mode", 10, "")
	if len(results) == 0 {
		t.Fatal("expected at least one result for 'dark mode'")
	}
	if results[0].Content != "User prefers dark mode" {
		t.Errorf("top result = %q, want dark mode preference", results[0].Content)
	}
}

func TestRecall_TypeFilter(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	store.Remember("Alice likes dark mode", TypePreference, 0.5)
	store.Remember("Alice's name is Alice", TypeFact, 0.9)
	store.Remember("Alice deployed on Friday", TypeEvent, 0.5)

	results := store.Recall("Alice", 10, TypeFact)
	if len(results) != 1 {
		t.Fatalf("expected 1 fact result, got %d", len(results))
	}
	if results[0].Type != TypeFact {
		t.Errorf("type = %q, want %q", results[0].Type, TypeFact)
	}
}

func TestRecall_EmptyQuery_ReturnsByRecency(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	now := time.Now().UTC()

	old, _ := store.Remember("old memory", TypeNote, 0.3)
	old.AccessedAt = now.Add(-10 * 24 * time.Hour)
	store.mu.Lock()
	_ = store.save(old)
	store.mu.Unlock()

	store.Remember("recent memory", TypeNote, 0.3)

	results := store.Recall("", 10, "")
	if len(results) < 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Content != "recent memory" {
		t.Errorf("expected recent memory first, got %q", results[0].Content)
	}
}

func TestRecall_Limit(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	for i := 0; i < 10; i++ {
		store.Remember("memory about cats", TypeNote, 0.5)
	}

	results := store.Recall("cats", 3, "")
	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}
}

func TestRecall_ForgetRace(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	m, _ := store.Remember("to be forgotten during recall", TypeNote, 0.5)

	// Simulate: scan found the memory, then it gets forgotten before the write phase.
	// We do this sequentially to test the existence check in the write phase.
	store.mu.RLock()
	var found []*Memory
	store.scanType("", func(mem *Memory) bool {
		found = append(found, mem)
		return true
	})
	store.mu.RUnlock()

	store.Forget(m.ID)

	// Now manually run the write phase — the existence check should skip the deleted memory.
	accessed := time.Now().UTC()
	store.mu.Lock()
	_ = store.db.BeginTransaction()
	for _, mem := range found {
		if !store.db.Exists(idxPrefix + mem.ID) {
			continue
		}
		mem.AccessedAt = accessed
		_ = store.save(mem)
	}
	_ = store.db.Commit()
	store.mu.Unlock()

	// Memory should still be gone, not re-created.
	if store.Count() != 0 {
		t.Errorf("forgotten memory was re-created by recall write phase, count = %d", store.Count())
	}
}

func TestForget_ByID(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	m, _ := store.Remember("to be forgotten", TypeNote, 0.5)
	if !store.Forget(m.ID) {
		t.Fatal("Forget returned false")
	}
	if store.Count() != 0 {
		t.Errorf("expected 0 memories, got %d", store.Count())
	}
}

func TestForget_Missing(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	if store.Forget("nonexistent-id") {
		t.Error("Forget should return false for missing ID")
	}
}

func TestList(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	store.Remember("fact one", TypeFact, 0.5)
	store.Remember("fact two", TypeFact, 0.5)
	store.Remember("a preference", TypePreference, 0.5)

	all := store.List("", 50)
	if len(all) != 3 {
		t.Errorf("expected 3, got %d", len(all))
	}

	facts := store.List(TypeFact, 50)
	if len(facts) != 2 {
		t.Errorf("expected 2 facts, got %d", len(facts))
	}
}

func TestList_Limit(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	for i := 0; i < 10; i++ {
		store.Remember("item", TypeNote, 0.5)
	}

	results := store.List("", 4)
	if len(results) != 4 {
		t.Errorf("expected 4, got %d", len(results))
	}
}

func TestCount(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	if store.Count() != 0 {
		t.Errorf("expected 0, got %d", store.Count())
	}
	store.Remember("one", TypeNote, 0.5)
	store.Remember("two", TypeNote, 0.5)
	if store.Count() != 2 {
		t.Errorf("expected 2, got %d", store.Count())
	}
}

func TestCompact_RemovesIdle(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	store.Remember("low importance idle", TypeNote, 0.3)
	store.Remember("high importance idle", TypeFact, 0.9)

	removed := store.Compact(-1*time.Second, 0.8)
	if removed != 1 {
		t.Errorf("expected 1 removed (low importance), got %d", removed)
	}
	if store.Count() != 1 {
		t.Errorf("expected 1 high importance memory to survive, got %d", store.Count())
	}
}

func TestCompact_ExemptsHighImportance(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	store.Remember("critical fact", TypeFact, 1.0)
	store.Remember("disposable note", TypeNote, 0.1)

	removed := store.Compact(-1*time.Second, 0.8)
	if removed != 1 {
		t.Errorf("expected 1 removed, got %d", removed)
	}
	if store.Count() != 1 {
		t.Errorf("expected 1 remaining, got %d", store.Count())
	}
}

func TestCompact_ZeroTimeout(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	store.Remember("should survive", TypeNote, 0.1)
	removed := store.Compact(0, 0)
	if removed != 0 {
		t.Errorf("Compact(0) should be a no-op, removed %d", removed)
	}
	if store.Count() != 1 {
		t.Errorf("expected 1 memory to survive, got %d", store.Count())
	}
}

func TestImportanceClamping(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	m1, _ := store.Remember("too high", TypeNote, 2.0)
	if m1.Importance != 1.0 {
		t.Errorf("importance should be clamped to 1.0, got %f", m1.Importance)
	}

	m2, _ := store.Remember("too low", TypeNote, -1.0)
	if m2.Importance != 0.0 {
		t.Errorf("importance should be clamped to 0.0, got %f", m2.Importance)
	}
}

func TestDefaultType(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	m, _ := store.Remember("no type given", "", 0.5)
	if m.Type != TypeNote {
		t.Errorf("default type = %q, want %q", m.Type, TypeNote)
	}
}

func TestNew_WithIdleTimeout(t *testing.T) {
	db, err := snapshotkv.Open("", nil)
	if err != nil {
		t.Fatalf("snapshotkv.Open: %v", err)
	}
	store := New(db, 100*time.Millisecond)
	store.Remember("disposable", TypeNote, 0.1)
	time.Sleep(500 * time.Millisecond)
	store.Close()
	if store.Count() != 0 {
		t.Errorf("expected compaction to remove idle memory, got %d", store.Count())
	}
	db.Close()
}

func TestTokenise(t *testing.T) {
	tokens := tokenise("hello, world! 123 foo-bar")
	expected := []string{"hello", "world", "123", "foo", "bar"}
	if len(tokens) != len(expected) {
		t.Fatalf("tokenise: got %v, want %v", tokens, expected)
	}
	for i, tok := range tokens {
		if tok != expected[i] {
			t.Errorf("token[%d] = %q, want %q", i, tok, expected[i])
		}
	}
}

func TestRecencyScore(t *testing.T) {
	now := time.Now().UTC()

	recent := &Memory{AccessedAt: now.Add(-30 * time.Minute)}
	if recencyScore(recent, now) != 1.0 {
		t.Error("memory accessed 30min ago should score 1.0")
	}

	old := &Memory{AccessedAt: now.Add(-31 * 24 * time.Hour)}
	if recencyScore(old, now) != 0.0 {
		t.Error("memory accessed 31 days ago should score 0.0")
	}

	mid := &Memory{AccessedAt: now.Add(-15 * 24 * time.Hour)}
	score := recencyScore(mid, now)
	if score <= 0 || score >= 1 {
		t.Errorf("15-day-old memory score = %f, want between 0 and 1", score)
	}
}

func TestPersistence_SnapshotRoundTrip(t *testing.T) {
	dir := t.TempDir()

	db1, err := snapshotkv.Open(dir, &snapshotkv.Config{SaveDebounce: 0})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	store1 := New(db1, 0)

	m, err := store1.Remember("persisted fact", TypeFact, 0.8)
	if err != nil {
		t.Fatalf("Remember: %v", err)
	}
	id := m.ID

	store1.Close()
	db1.Close()

	db2, err := snapshotkv.Open(dir, nil)
	if err != nil {
		t.Fatalf("Reopen: %v", err)
	}
	defer db2.Close()
	store2 := New(db2, 0)
	defer store2.Close()

	results := store2.Recall("persisted", 1, "")
	if len(results) == 0 {
		t.Fatal("no results after snapshot reload")
	}
	if results[0].ID != id {
		t.Errorf("ID = %q, want %q", results[0].ID, id)
	}
	if results[0].Content != "persisted fact" {
		t.Errorf("Content = %q, want %q", results[0].Content, "persisted fact")
	}
	if results[0].Type != TypeFact {
		t.Errorf("Type = %q, want %q", results[0].Type, TypeFact)
	}
	if results[0].Importance != 0.8 {
		t.Errorf("Importance = %f, want 0.8", results[0].Importance)
	}

	if !store2.Forget(id) {
		t.Error("Forget returned false after reload")
	}
	if store2.Count() != 0 {
		t.Errorf("Count = %d after forget, want 0", store2.Count())
	}
}

func TestRecall_Concurrent(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	for i := 0; i < 20; i++ {
		store.Remember("concurrent memory", TypeNote, 0.5)
	}

	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func() {
			store.Recall("concurrent", 5, "")
			done <- struct{}{}
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}
}
