package memory

import (
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/paularlott/snapshotkv"
)

const (
	memPrefix = "mem:"
	idxPrefix = "idx:"

	TypeFact       = "fact"
	TypePreference = "preference"
	TypeEvent      = "event"
	TypeNote       = "note"
)

// typePrefix returns the full KV key prefix for a memory type.
func typePrefix(memType string) string {
	return memPrefix + memType + ":"
}

// Memory is a single stored memory entry.
type Memory struct {
	ID         string    `msgpack:"id"`
	Content    string    `msgpack:"content"`
	Type       string    `msgpack:"type"`
	Importance float64   `msgpack:"importance"`
	CreatedAt  time.Time `msgpack:"created_at"`
	AccessedAt time.Time `msgpack:"accessed_at"`
}

// Store is a memory store backed by a snapshotkv DB.
// It does not own the DB — the caller manages its lifecycle.
type Store struct {
	mu          sync.RWMutex
	db          *snapshotkv.DB
	idleTimeout time.Duration
	stopCompact chan struct{}
}

// New creates a Store using the provided DB.
// idleTimeout is how long a memory can go unaccessed before compaction removes it.
// Pass 0 to disable automatic compaction.
func New(db *snapshotkv.DB, idleTimeout time.Duration) *Store {
	s := &Store{
		db:          db,
		idleTimeout: idleTimeout,
		stopCompact: make(chan struct{}),
	}
	if idleTimeout > 0 {
		go s.compactLoop()
	}
	return s
}

// Close stops the background compaction goroutine.
// It does NOT close the underlying DB.
func (s *Store) Close() {
	select {
	case <-s.stopCompact:
	default:
		close(s.stopCompact)
	}
}

// Remember stores a memory and returns it with a UUIDv7 ID.
func (s *Store) Remember(content, memType string, importance float64) (*Memory, error) {
	if memType == "" {
		memType = TypeNote
	}
	if importance < 0 {
		importance = 0
	}
	if importance > 1 {
		importance = 1
	}

	id, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	m := &Memory{
		ID:         id.String(),
		Content:    content,
		Type:       memType,
		Importance: importance,
		CreatedAt:  now,
		AccessedAt: now,
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.save(m); err != nil {
		return nil, err
	}
	return m, nil
}

// Recall searches memories by keyword and returns up to limit results ranked by score.
func (s *Store) Recall(query string, limit int, typeFilter string) []*Memory {
	if limit <= 0 {
		limit = 10
	}

	now := time.Now().UTC()
	queryLower := strings.ToLower(strings.TrimSpace(query))
	queryTokens := tokenise(queryLower)

	type scored struct {
		m     *Memory
		score float64
	}
	var results []scored

	// Scan phase is read-only.
	s.mu.RLock()
	s.scanType(typeFilter, func(m *Memory) bool {
		var score float64
		if queryLower == "" {
			score = recencyScore(m, now)*0.6 + m.Importance*0.4
		} else {
			contentHits := keywordHits(queryTokens, m.Content)
			if contentHits == 0 {
				return true
			}
			score = float64(contentHits)*0.5 + m.Importance*0.3 + recencyScore(m, now)*0.2
		}
		results = append(results, scored{m, score})
		return true
	})
	s.mu.RUnlock()

	// Sort descending by score (insertion sort — memory stores are small)
	for i := 1; i < len(results); i++ {
		for j := i; j > 0 && results[j].score > results[j-1].score; j-- {
			results[j], results[j-1] = results[j-1], results[j]
		}
	}

	if len(results) > limit {
		results = results[:limit]
	}

	// Batch-update AccessedAt — write lock only for the mutation phase.
	// Re-check existence: a concurrent Forget may have removed a result between
	// releasing RLock and acquiring Lock here.
	out := make([]*Memory, 0, len(results))
	accessed := time.Now().UTC()
	s.mu.Lock()
	_ = s.db.BeginTransaction()
	for _, r := range results {
		if !s.db.Exists(idxPrefix + r.m.ID) {
			continue
		}
		r.m.AccessedAt = accessed
		_ = s.save(r.m)
		out = append(out, r.m)
	}
	_ = s.db.Commit()
	s.mu.Unlock()

	return out
}

// Forget removes a memory by ID.
func (s *Store) Forget(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	val, err := s.db.Get(idxPrefix + id)
	if err != nil {
		return false
	}
	key, ok := val.(string)
	if !ok {
		return false
	}
	s.db.Delete(key)
	s.db.Delete(idxPrefix + id)
	return true
}

// List returns all memories, optionally filtered by type, up to limit.
func (s *Store) List(typeFilter string, limit int) []*Memory {
	if limit <= 0 {
		limit = 50
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]*Memory, 0, limit)
	s.scanType(typeFilter, func(m *Memory) bool {
		out = append(out, m)
		return len(out) < limit
	})
	return out
}

// Count returns the total number of stored memories.
func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.db.Count(idxPrefix)
}

// Compact removes memories that have not been accessed within idleTimeout,
// exempting memories with importance >= exemptThreshold.
// A zero idleTimeout is a no-op.
func (s *Store) Compact(idleTimeout time.Duration, exemptThreshold float64) int {
	if idleTimeout == 0 {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().UTC().Add(-idleTimeout)

	// Collect keys to delete — no mutations inside Scan callback.
	type toDelete struct{ memKey, idxKey string }
	var victims []toDelete

	s.scanType("", func(m *Memory) bool {
		if m.Importance < exemptThreshold && m.AccessedAt.Before(cutoff) {
			victims = append(victims, toDelete{typePrefix(m.Type) + m.ID, idxPrefix + m.ID})
		}
		return true
	})

	_ = s.db.BeginTransaction()
	for _, v := range victims {
		s.db.Delete(v.memKey)
		s.db.Delete(v.idxKey)
	}
	_ = s.db.Commit()

	return len(victims)
}

// --- internal helpers ---

// scanType iterates memories, optionally filtered to a single type, calling fn for each.
// Stops early if fn returns false. Must be called with s.mu held.
// fn must not call any DB methods (Scan holds db.mu.RLock).
func (s *Store) scanType(typeFilter string, fn func(*Memory) bool) {
	prefix := memPrefix
	if typeFilter != "" {
		prefix = typePrefix(typeFilter)
	}
	s.db.Scan(prefix, func(_ string, value any) bool {
		m := toMemory(value)
		if m == nil {
			return true
		}
		return fn(m)
	})
}

// toMemory converts a decoded map[string]any value to a *Memory.
// msgpack always decodes into map[string]any with concrete types: string, float64, time.Time.
func toMemory(value any) *Memory {
	m, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	mem := &Memory{}
	mem.ID, _ = m["id"].(string)
	mem.Content, _ = m["content"].(string)
	mem.Type, _ = m["type"].(string)
	mem.Importance, _ = m["importance"].(float64)
	mem.CreatedAt, _ = m["created_at"].(time.Time)
	mem.AccessedAt, _ = m["accessed_at"].(time.Time)
	if mem.ID == "" {
		return nil
	}
	return mem
}

func (s *Store) save(m *Memory) error {
	key := typePrefix(m.Type) + m.ID
	if err := s.db.Set(key, m); err != nil {
		return err
	}
	return s.db.Set(idxPrefix+m.ID, key)
}

func (s *Store) compactLoop() {
	ticker := time.NewTicker(s.idleTimeout / 4)
	defer ticker.Stop()
	for {
		select {
		case <-s.stopCompact:
			return
		case <-ticker.C:
			s.Compact(s.idleTimeout, 0.8)
		}
	}
}

// tokenise splits text into lowercase words, stripping punctuation.
// text must already be lowercased.
func tokenise(text string) []string {
	var tokens []string
	var buf strings.Builder
	for _, r := range text {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			buf.WriteRune(r)
		} else if buf.Len() > 0 {
			tokens = append(tokens, buf.String())
			buf.Reset()
		}
	}
	if buf.Len() > 0 {
		tokens = append(tokens, buf.String())
	}
	return tokens
}

// keywordHits counts how many query tokens appear in content.
// Tries exact match first, then strips a trailing 's' for basic plural tolerance.
func keywordHits(queryTokens []string, content string) int {
	contentLower := strings.ToLower(content)
	hits := 0
	for _, t := range queryTokens {
		if strings.Contains(contentLower, t) {
			hits++
		} else if len(t) > 3 && t[len(t)-1] == 's' && strings.Contains(contentLower, t[:len(t)-1]) {
			hits++
		}
	}
	return hits
}

// recencyScore returns a 0–1 score based on how recently the memory was accessed.
// Memories accessed within the last hour score 1.0; score decays over 30 days.
func recencyScore(m *Memory, now time.Time) float64 {
	age := now.Sub(m.AccessedAt)
	if age <= time.Hour {
		return 1.0
	}
	days := age.Hours() / 24
	if days >= 30 {
		return 0.0
	}
	return 1.0 - (days / 30)
}
