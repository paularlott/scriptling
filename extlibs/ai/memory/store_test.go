package memory

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/paularlott/snapshotkv"
)

func newTestStore(t *testing.T, opts ...Option) *Store {
	t.Helper()
	db, err := snapshotkv.Open("", nil)
	if err != nil {
		t.Fatalf("snapshotkv.Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return New(db, opts...)
}

// --- Remember ---

func TestRemember_ReturnsMemoryWithID(t *testing.T) {
	s := newTestStore(t)
	m, err := s.Remember("User's name is Alice", TypeFact, 0.9)
	if err != nil {
		t.Fatalf("Remember: %v", err)
	}
	if m.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if m.Type != TypeFact {
		t.Errorf("type = %q, want %q", m.Type, TypeFact)
	}
	if m.Importance != 0.9 {
		t.Errorf("importance = %f, want 0.9", m.Importance)
	}
}

func TestRemember_DefaultType(t *testing.T) {
	s := newTestStore(t)
	m, _ := s.Remember("no type given", "", 0.5)
	if m.Type != TypeNote {
		t.Errorf("default type = %q, want %q", m.Type, TypeNote)
	}
}

func TestRemember_ImportanceClamping(t *testing.T) {
	s := newTestStore(t)
	m1, _ := s.Remember("too high", TypeNote, 2.0)
	if m1.Importance != 1.0 {
		t.Errorf("importance should be clamped to 1.0, got %f", m1.Importance)
	}
	m2, _ := s.Remember("too low", TypeNote, -1.0)
	if m2.Importance != 0.0 {
		t.Errorf("importance should be clamped to 0.0, got %f", m2.Importance)
	}
}

// --- Recall ---

func TestRecall_KeywordMatch(t *testing.T) {
	s := newTestStore(t)
	s.Remember("User prefers dark mode", TypePreference, 0.7)
	s.Remember("API rate limit is 1000 per day", TypeFact, 0.9)

	results := s.Recall("dark mode", 10, "")
	if len(results) == 0 {
		t.Fatal("expected at least one result for 'dark mode'")
	}
	if results[0].Content != "User prefers dark mode" {
		t.Errorf("top result = %q, want dark mode preference", results[0].Content)
	}
}

func TestRecall_NoMatch(t *testing.T) {
	s := newTestStore(t)
	results := s.Recall("no_such_content", 1, "")
	if len(results) != 0 {
		t.Errorf("expected no results, got %+v", results)
	}
}

func TestRecall_TypeFilter(t *testing.T) {
	s := newTestStore(t)
	s.Remember("Alice likes dark mode", TypePreference, 0.5)
	s.Remember("Alice's name is Alice", TypeFact, 0.9)
	s.Remember("Alice deployed on Friday", TypeEvent, 0.5)

	results := s.Recall("Alice", 10, TypeFact)
	if len(results) != 1 {
		t.Fatalf("expected 1 fact result, got %d", len(results))
	}
	if results[0].Type != TypeFact {
		t.Errorf("type = %q, want %q", results[0].Type, TypeFact)
	}
}

func TestRecall_SemanticSimilarity(t *testing.T) {
	s := newTestStore(t)
	s.Remember("user prefers dark theme for coding", TypePreference, 0.7)
	s.Remember("API rate limit is 1000 per day", TypeFact, 0.9)

	// Query with similar but not identical words - should find the preference
	results := s.Recall("programming with dark mode settings", 10, "")
	if len(results) == 0 {
		t.Fatal("expected at least one result from semantic similarity")
	}
	// The preference should rank highly due to MinHash similarity
	found := false
	for _, r := range results {
		if r.Type == TypePreference {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find preference memory via semantic similarity")
	}
}

func TestRecall_UpdatesAccessedAt(t *testing.T) {
	s := newTestStore(t)
	before := time.Now().UTC().Add(-time.Second)
	s.Remember("test content", TypeNote, 0.5)
	results := s.Recall("test", 1, "")
	if len(results) == 0 {
		t.Fatal("Recall returned no results")
	}
	if !results[0].AccessedAt.After(before) {
		t.Error("AccessedAt should be updated on recall")
	}
}

func TestRecall_EmptyQuery_ReturnsByRecency(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC()

	old, _ := s.Remember("old memory", TypeNote, 0.3)
	old.AccessedAt = now.Add(-10 * 24 * time.Hour)
	s.mu.Lock()
	_ = s.save(old)
	s.mu.Unlock()

	s.Remember("recent memory", TypeNote, 0.3)

	results := s.Recall("", 10, "")
	if len(results) < 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Content != "recent memory" {
		t.Errorf("expected recent memory first, got %q", results[0].Content)
	}
}

func TestRecall_ExcludeType(t *testing.T) {
	s := newTestStore(t)

	// Create distinct preferences
	s.Remember("user prefers dark mode theme", TypePreference, 0.5)
	s.Remember("user likes concise responses", TypePreference, 0.5)
	s.Remember("user speaks english language", TypePreference, 0.5)

	// Create distinct notes
	s.Remember("alice is working on the api integration", TypeNote, 0.5)
	s.Remember("the database migration completed successfully", TypeNote, 0.5)
	s.Remember("meeting scheduled for friday afternoon", TypeNote, 0.5)

	// Get all non-preferences with limit=2
	results := s.Recall("", 2, "!preference")

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for _, r := range results {
		if r.Type == TypePreference {
			t.Errorf("should not return preferences, got %q", r.Content)
		}
	}
}

func TestRecall_Limit(t *testing.T) {
	s := newTestStore(t)
	words := []string{"alpha", "beta", "gamma", "delta", "epsilon", "zeta", "eta", "theta", "iota", "kappa"}
	for _, w := range words {
		s.Remember("cats like "+w, TypeNote, 0.5)
	}
	results := s.Recall("cats", 3, "")
	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}
}

func TestRecall_ForgetRace(t *testing.T) {
	s := newTestStore(t)
	m, _ := s.Remember("to be forgotten during recall", TypeNote, 0.5)

	s.mu.RLock()
	var found []*Memory
	s.scanType("", func(mem *Memory) bool {
		found = append(found, mem)
		return true
	})
	s.mu.RUnlock()

	s.Forget(m.ID)

	accessed := time.Now().UTC()
	s.mu.Lock()
	_ = s.db.BeginTransaction()
	for _, mem := range found {
		if !s.db.Exists(idxPrefix + mem.ID) {
			continue
		}
		mem.AccessedAt = accessed
		_ = s.save(mem)
	}
	_ = s.db.Commit()
	s.mu.Unlock()

	if s.Count() != 0 {
		t.Errorf("forgotten memory was re-created by recall write phase, count = %d", s.Count())
	}
}

// --- Forget ---

func TestForget_ByID(t *testing.T) {
	s := newTestStore(t)
	m, _ := s.Remember("to be forgotten", TypeNote, 0.5)
	if !s.Forget(m.ID) {
		t.Fatal("Forget returned false")
	}
	if s.Count() != 0 {
		t.Errorf("expected 0 memories, got %d", s.Count())
	}
}

func TestForget_Missing(t *testing.T) {
	s := newTestStore(t)
	if s.Forget("nonexistent-id") {
		t.Error("Forget should return false for missing ID")
	}
}

// --- Count ---

func TestCount(t *testing.T) {
	s := newTestStore(t)
	if s.Count() != 0 {
		t.Errorf("expected 0, got %d", s.Count())
	}
	s.Remember("one", TypeNote, 0.5)
	s.Remember("two unique", TypeNote, 0.5)
	if s.Count() != 2 {
		t.Errorf("expected 2, got %d", s.Count())
	}
}

// --- Decay ---

func TestDecayFactor_Preference_NeverDecays(t *testing.T) {
	s := newTestStore(t)
	for _, age := range []time.Duration{0, 7 * 24 * time.Hour, 90 * 24 * time.Hour, 365 * 24 * time.Hour} {
		if f := s.decayFactor(TypePreference, age); f != 1.0 {
			t.Errorf("preference decay at age %v = %f, want 1.0", age, f)
		}
	}
}

func TestDecayFactor_ZeroAge(t *testing.T) {
	s := newTestStore(t)
	for _, typ := range []string{TypeFact, TypeEvent, TypeNote} {
		if f := s.decayFactor(typ, 0); f != 1.0 {
			t.Errorf("decay at age 0 for %q = %f, want 1.0", typ, f)
		}
	}
}

func TestDecayFactor_HalfLife(t *testing.T) {
	s := newTestStore(t)
	cases := []struct {
		typ      string
		halfLife time.Duration
	}{
		{TypeFact, DefaultHalfLifeFact},
		{TypeEvent, DefaultHalfLifeEvent},
		{TypeNote, DefaultHalfLifeNote},
	}
	for _, c := range cases {
		f := s.decayFactor(c.typ, c.halfLife)
		if f < 0.49 || f > 0.51 {
			t.Errorf("%s at half-life: decay = %f, want ~0.5", c.typ, f)
		}
	}
}

func TestDecayFactor_TwoHalfLives(t *testing.T) {
	s := newTestStore(t)
	f := s.decayFactor(TypeNote, 2*DefaultHalfLifeNote)
	if f < 0.24 || f > 0.26 {
		t.Errorf("note at 2 half-lives: decay = %f, want ~0.25", f)
	}
}

func TestDecayFactor_CustomHalfLife(t *testing.T) {
	s := newTestStore(t, WithHalfLifeNote(14*24*time.Hour))
	f := s.decayFactor(TypeNote, 14*24*time.Hour)
	if f < 0.49 || f > 0.51 {
		t.Errorf("custom half-life: decay = %f, want ~0.5", f)
	}
}

// --- Prune compaction ---

func TestPrune_HardAgeCap(t *testing.T) {
	s := newTestStore(t, WithMaxAge(30*24*time.Hour))

	// Old memory (beyond max age)
	old, _ := s.Remember("old note", TypeNote, 0.9)
	old.AccessedAt = time.Now().UTC().Add(-31 * 24 * time.Hour)
	s.mu.Lock()
	_ = s.save(old)
	s.mu.Unlock()

	// Recent memory
	s.Remember("recent note", TypeNote, 0.9)

	removed := s.prune()
	if removed != 1 {
		t.Errorf("expected 1 removed by age cap, got %d", removed)
	}
	if s.Count() != 1 {
		t.Errorf("expected 1 remaining, got %d", s.Count())
	}
}

func TestPrune_DecayThreshold(t *testing.T) {
	s := newTestStore(t,
		WithHalfLifeNote(7*24*time.Hour),
		WithPruneThreshold(0.1),
	)

	// importance=0.8, age=3 half-lives → 0.8 * 0.125 = 0.1 → at threshold, should prune
	m, _ := s.Remember("decayed note", TypeNote, 0.8)
	m.AccessedAt = time.Now().UTC().Add(-21 * 24 * time.Hour)
	s.mu.Lock()
	_ = s.save(m)
	s.mu.Unlock()

	// High importance note — should survive
	s.Remember("important note", TypeNote, 0.9)

	removed := s.prune()
	if removed != 1 {
		t.Errorf("expected 1 removed by decay, got %d", removed)
	}
}

func TestPrune_PreferenceNotDecayed(t *testing.T) {
	s := newTestStore(t,
		WithPruneThreshold(0.1),
		WithMaxAge(365*24*time.Hour),
	)

	// Old preference with low importance — should NOT be pruned by decay (preference never decays)
	pref, _ := s.Remember("user prefers dark mode", TypePreference, 0.2)
	pref.AccessedAt = time.Now().UTC().Add(-60 * 24 * time.Hour)
	s.mu.Lock()
	_ = s.save(pref)
	s.mu.Unlock()

	removed := s.prune()
	if removed != 0 {
		t.Errorf("preference should not be pruned by decay, removed %d", removed)
	}
}

func TestPrune_PreferenceHardAgeCap(t *testing.T) {
	s := newTestStore(t, WithMaxAge(30*24*time.Hour))

	// Preference beyond hard age cap — should be deleted
	pref, _ := s.Remember("old preference", TypePreference, 1.0)
	pref.AccessedAt = time.Now().UTC().Add(-31 * 24 * time.Hour)
	s.mu.Lock()
	_ = s.save(pref)
	s.mu.Unlock()

	removed := s.prune()
	if removed != 1 {
		t.Errorf("preference beyond hard age cap should be removed, got %d removed", removed)
	}
}

func TestPrune_NothingToPrune(t *testing.T) {
	s := newTestStore(t)
	s.Remember("fresh high importance", TypeFact, 0.9)
	removed := s.prune()
	if removed != 0 {
		t.Errorf("expected 0 removed, got %d", removed)
	}
}

func TestPrune_Empty(t *testing.T) {
	s := newTestStore(t)
	removed := s.prune()
	if removed != 0 {
		t.Errorf("expected 0 on empty store, got %d", removed)
	}
}

// --- Persistence ---

func TestPersistence_SnapshotRoundTrip(t *testing.T) {
	dir := t.TempDir()

	db1, err := snapshotkv.Open(dir, &snapshotkv.Config{SaveDebounce: 0})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	s1 := New(db1)
	m, err := s1.Remember("persisted fact", TypeFact, 0.8)
	if err != nil {
		t.Fatalf("Remember: %v", err)
	}
	id := m.ID
	db1.Close()

	db2, err := snapshotkv.Open(dir, nil)
	if err != nil {
		t.Fatalf("Reopen: %v", err)
	}
	defer db2.Close()
	s2 := New(db2)

	results := s2.Recall("persisted", 1, "")
	if len(results) == 0 {
		t.Fatal("no results after snapshot reload")
	}
	if results[0].ID != id {
		t.Errorf("ID = %q, want %q", results[0].ID, id)
	}
	if results[0].Content != "persisted fact" {
		t.Errorf("Content = %q, want %q", results[0].Content, "persisted fact")
	}
	if results[0].Importance != 0.8 {
		t.Errorf("Importance = %f, want 0.8", results[0].Importance)
	}
	if !s2.Forget(id) {
		t.Error("Forget returned false after reload")
	}
	if s2.Count() != 0 {
		t.Errorf("Count = %d after forget, want 0", s2.Count())
	}
}

// --- Concurrency ---

func TestRecall_Concurrent(t *testing.T) {
	s := newTestStore(t)
	for i := 0; i < 20; i++ {
		s.Remember("concurrent memory", TypeNote, 0.5)
	}
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.Recall("concurrent", 5, "")
		}()
	}
	wg.Wait()
}

func TestRememberForget_Concurrent(t *testing.T) {
	s := newTestStore(t)
	var wg sync.WaitGroup
	ids := make(chan string, 50)

	for i := 0; i < 25; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			m, _ := s.Remember(fmt.Sprintf("concurrent write unique %d", n), TypeNote, 0.5)
			if m != nil {
				ids <- m.ID
			}
		}(i)
	}
	wg.Wait()
	close(ids)

	seen := make(map[string]bool)
	for id := range ids {
		if !seen[id] {
			seen[id] = true
			s.Forget(id)
		}
	}
	if s.Count() != 0 {
		t.Errorf("expected 0 after concurrent forget, got %d", s.Count())
	}
}

// --- Helpers ---

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

// --- MinHash ---

func TestMinHash_IdenticalContent(t *testing.T) {
	hash1 := computeMinHash("the quick brown fox jumps over the lazy dog")
	hash2 := computeMinHash("the quick brown fox jumps over the lazy dog")
	score := minHashSimilarity(hash1, hash2)
	if score != 1.0 {
		t.Errorf("identical content should have similarity 1.0, got %f", score)
	}
}

func TestMinHash_SimilarContent(t *testing.T) {
	hash1 := computeMinHash("the quick brown fox jumps over the lazy dog")
	hash2 := computeMinHash("the quick brown fox jumped over the lazy dogs")
	score := minHashSimilarity(hash1, hash2)
	// MinHash estimates Jaccard similarity with some variance
	if score < 0.4 {
		t.Errorf("similar content should have moderate-high similarity, got %f", score)
	}
}

func TestMinHash_DifferentContent(t *testing.T) {
	hash1 := computeMinHash("the quick brown fox jumps over the lazy dog")
	hash2 := computeMinHash("completely different text about programming go")
	score := minHashSimilarity(hash1, hash2)
	if score > 0.3 {
		t.Errorf("different content should have low similarity, got %f", score)
	}
}

func TestMinHash_EmptyContent(t *testing.T) {
	hash := computeMinHash("")
	if len(hash) != minHashSize {
		t.Errorf("empty content should still produce %d hash values", minHashSize)
	}
}

func TestMinHash_PreComputedVsComputed(t *testing.T) {
	s := newTestStore(t)
	m, _ := s.Remember("user prefers dark mode for coding", TypePreference, 0.8)

	// Verify MinHash was computed and stored
	if len(m.MinHash) != minHashSize {
		t.Errorf("stored memory should have %d hash values, got %d", minHashSize, len(m.MinHash))
	}

	// Verify similarity check uses stored MinHash
	newHash := computeMinHash("user prefers dark mode for coding")
	score := minHashSimilarity(m.MinHash, newHash)
	if score != 1.0 {
		t.Errorf("stored MinHash should match recomputed hash, got %f", score)
	}
}

// --- Benchmarks ---

func BenchmarkMinHashSimilarity(b *testing.B) {
	hash1 := computeMinHash("the quick brown fox jumps over the lazy dog and some more text")
	hash2 := computeMinHash("the quick brown fox jumped over the lazy dogs with extra words")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = minHashSimilarity(hash1, hash2)
	}
}

func BenchmarkComputeMinHash(b *testing.B) {
	text := "the quick brown fox jumps over the lazy dog and some additional content here"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = computeMinHash(text)
	}
}

func BenchmarkFindMostSimilar(b *testing.B) {
	db, err := snapshotkv.Open("", nil)
	if err != nil {
		b.Fatalf("snapshotkv.Open: %v", err)
	}
	defer db.Close()
	s := New(db)
	// Add 100 memories
	for i := 0; i < 100; i++ {
		s.Remember(fmt.Sprintf("memory content number %d about various topics", i), TypeNote, 0.5)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = s.findMostSimilar("memory content about topics", TypeNote)
	}
}
