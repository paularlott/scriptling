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

func TestRememberAndRecallByKey(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	m, err := store.Remember("User's name is Alice", TypeFact, "user_name", 0.9)
	if err != nil {
		t.Fatalf("Remember: %v", err)
	}
	if m.ID == "" {
		t.Fatal("expected non-empty ID")
	}

	results := store.Recall("user_name", 1, "")
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
	if got.Key != "user_name" {
		t.Errorf("key = %q, want %q", got.Key, "user_name")
	}
}

func TestRecallByKey_Missing(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	results := store.Recall("no_such_key", 1, "")
	if len(results) != 0 {
		t.Errorf("expected no results, got %+v", results)
	}
}

func TestRecallByKey_UpdatesAccessedAt(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	before := time.Now().UTC().Add(-time.Second)
	store.Remember("test", TypeNote, "k", 0.5)
	results := store.Recall("k", 1, "")
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

	store.Remember("User prefers dark mode", TypePreference, "ui_theme", 0.7)
	store.Remember("API rate limit is 1000 per day", TypeFact, "api_limit", 0.9)
	store.Remember("Deployed version 2.1 on Friday", TypeEvent, "", 0.5)

	results := store.Recall("dark mode", 10, "")
	if len(results) == 0 {
		t.Fatal("expected at least one result for 'dark mode'")
	}
	if results[0].Key != "ui_theme" {
		t.Errorf("top result key = %q, want %q", results[0].Key, "ui_theme")
	}
}

func TestRecall_TypeFilter(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	store.Remember("Alice likes dark mode", TypePreference, "", 0.5)
	store.Remember("Alice's name is Alice", TypeFact, "user_name", 0.9)
	store.Remember("Alice deployed on Friday", TypeEvent, "", 0.5)

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

	// Store old memory then manually backdate its accessed_at
	old, _ := store.Remember("old memory", TypeNote, "", 0.3)
	old.AccessedAt = now.Add(-10 * 24 * time.Hour)
	store.mu.Lock()
	_ = store.save(old)
	store.mu.Unlock()

	store.Remember("recent memory", TypeNote, "", 0.3)

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
		store.Remember("memory about cats", TypeNote, "", 0.5)
	}

	results := store.Recall("cats", 3, "")
	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}
}

func TestForget_ByID(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	m, _ := store.Remember("to be forgotten", TypeNote, "", 0.5)
	if !store.Forget(m.ID) {
		t.Fatal("Forget returned false")
	}
	if store.Count() != 0 {
		t.Errorf("expected 0 memories, got %d", store.Count())
	}
}

func TestForget_ByKey(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	store.Remember("secret value", TypeFact, "secret_key", 0.9)
	if !store.ForgetByKey("secret_key") {
		t.Fatal("ForgetByKey returned false")
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
	if store.ForgetByKey("nonexistent-key") {
		t.Error("ForgetByKey should return false for missing key")
	}
}

func TestList(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	store.Remember("fact one", TypeFact, "", 0.5)
	store.Remember("fact two", TypeFact, "", 0.5)
	store.Remember("a preference", TypePreference, "", 0.5)

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
		store.Remember("item", TypeNote, "", 0.5)
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
	store.Remember("one", TypeNote, "", 0.5)
	store.Remember("two", TypeNote, "", 0.5)
	if store.Count() != 2 {
		t.Errorf("expected 2, got %d", store.Count())
	}
}

func TestCompact_RemovesIdle(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	store.Remember("low importance idle", TypeNote, "", 0.3)
	store.Remember("high importance idle", TypeFact, "important", 0.9)

	// Compact with a very short idle timeout — both are "old enough"
	removed := store.Compact(-1*time.Second, 0.8) // negative = everything is past cutoff
	if removed != 1 {
		t.Errorf("expected 1 removed (low importance), got %d", removed)
	}

	// High importance memory should survive
	if store.Count() != 1 {
		t.Errorf("expected 1 high importance memory to survive, got %d", store.Count())
	}
}

func TestCompact_ExemptsHighImportance(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	store.Remember("critical fact", TypeFact, "critical", 1.0)
	store.Remember("disposable note", TypeNote, "", 0.1)

	removed := store.Compact(-1*time.Second, 0.8)
	if removed != 1 {
		t.Errorf("expected 1 removed, got %d", removed)
	}
	if store.Count() != 1 {
		t.Errorf("expected 1 remaining, got %d", store.Count())
	}
}

func TestImportanceClamping(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	m1, _ := store.Remember("too high", TypeNote, "", 2.0)
	if m1.Importance != 1.0 {
		t.Errorf("importance should be clamped to 1.0, got %f", m1.Importance)
	}

	m2, _ := store.Remember("too low", TypeNote, "", -1.0)
	if m2.Importance != 0.0 {
		t.Errorf("importance should be clamped to 0.0, got %f", m2.Importance)
	}
}

func TestDefaultType(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	m, _ := store.Remember("no type given", "", "", 0.5)
	if m.Type != TypeNote {
		t.Errorf("default type = %q, want %q", m.Type, TypeNote)
	}
}

func TestNew_WithIdleTimeout(t *testing.T) {
	db, err := snapshotkv.Open("", nil)
	if err != nil {
		t.Fatalf("snapshotkv.Open: %v", err)
	}
	store := New(db, 100*time.Millisecond) // starts compactLoop
	store.Remember("disposable", TypeNote, "", 0.1)
	time.Sleep(500 * time.Millisecond) // let compactLoop run at least once
	store.Close()
	db.Close()
}

func TestRecall_KeyMatchScoresHigherThanContent(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	// "wife" appears in content of both, but only one has it in the key
	store.Remember("User's wife is named Cindy", TypeFact, "user_wife_name", 0.9)
	store.Remember("User mentioned his wife likes cats", TypeNote, "user_note", 0.5)

	results := store.Recall("wife", 10, "")
	if len(results) < 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Key != "user_wife_name" {
		t.Errorf("key match should rank first, got %q", results[0].Key)
	}
}

func TestForget_CleansKeyIndex(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	m, _ := store.Remember("keyed memory", TypeFact, "my_key", 0.9)
	store.Forget(m.ID)

	// Key index should be gone — recall should return nothing
	results := store.Recall("my_key", 1, "")
	if len(results) != 0 {
		t.Errorf("expected no results after forget, got %d", len(results))
	}
}

func TestRecall_NoMatchReturnsEmpty(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	store.Remember("User's name is Alice", TypeFact, "user_name", 0.9)

	results := store.Recall("xyzzy", 10, "")
	if len(results) != 0 {
		t.Errorf("expected no results for unmatched query, got %d", len(results))
	}
}

func TestRecall_PluralTolerance(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	store.Remember("User prefers dark mode", TypePreference, "ui_theme_preference", 0.7)
	store.Remember("User's name is Alice", TypeFact, "user_name", 0.9)

	// "preferences" should match "preference" in both content and key
	results := store.Recall("preferences", 10, "")
	if len(results) == 0 {
		t.Fatal("expected results for plural query 'preferences'")
	}
	if results[0].Type != TypePreference {
		t.Errorf("expected preference type, got %q", results[0].Type)
	}
}

func TestRecall_FuzzyKeyMatch(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	store.Remember("User's wife is named Cindy", TypeFact, "user_wife_name", 0.9)
	store.Remember("User's name is Paul", TypeFact, "user_name", 0.9)

	// LLM queries with partial key "wife_name" — should still find the right memory
	results := store.Recall("wife_name", 1, "")
	if len(results) == 0 {
		t.Fatal("expected a result for fuzzy key 'wife_name'")
	}
	if results[0].Key != "user_wife_name" {
		t.Errorf("top result key = %q, want %q", results[0].Key, "user_wife_name")
	}
}

func TestTokenise(t *testing.T) {
	tokens := tokenise("Hello, World! 123 foo-bar")
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
