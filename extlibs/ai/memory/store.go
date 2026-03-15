package memory

import (
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/paularlott/snapshotkv"
)

const (
	memPrefix = "mem:"
	keyPrefix = "key:"

	TypeFact       = "fact"
	TypePreference = "preference"
	TypeEvent      = "event"
	TypeNote       = "note"
)

// Memory is a single stored memory entry.
type Memory struct {
	ID         string    `json:"id"`
	Content    string    `json:"content"`
	Type       string    `json:"type"`
	Key        string    `json:"key,omitempty"`
	Importance float64   `json:"importance"`
	CreatedAt  time.Time `json:"created_at"`
	AccessedAt time.Time `json:"accessed_at"`
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

// Remember stores a memory. If key is non-empty a secondary index is written.
func (s *Store) Remember(content, memType, key string, importance float64) (*Memory, error) {
	if memType == "" {
		memType = TypeNote
	}
	if importance < 0 {
		importance = 0
	}
	if importance > 1 {
		importance = 1
	}

	now := time.Now().UTC()
	m := &Memory{
		ID:         uuid.New().String(),
		Content:    content,
		Type:       memType,
		Key:        key,
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
// Matches against both content and the semantic key field.
func (s *Store) Recall(query string, limit int, typeFilter string) []*Memory {
	if limit <= 0 {
		limit = 10
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	all := s.allMemories()
	now := time.Now().UTC()

	type scored struct {
		m     *Memory
		score float64
	}

	var results []scored
	queryLower := strings.ToLower(strings.TrimSpace(query))
	queryTokens := tokenise(queryLower)

	for _, m := range all {
		if typeFilter != "" && m.Type != typeFilter {
			continue
		}

		var score float64
		if queryLower == "" {
			// No query — rank by recency + importance only
			score = recencyScore(m, now)*0.6 + m.Importance*0.4
		} else {
			keyHits := keywordHits(queryTokens, m.Key)
			contentHits := keywordHits(queryTokens, m.Content)
			if keyHits == 0 && contentHits == 0 {
				continue
			}
			// Key matches weighted higher than content matches
			score = float64(keyHits)*0.5 + float64(contentHits)*0.25 + m.Importance*0.15 + recencyScore(m, now)*0.1
		}

		results = append(results, scored{m, score})
	}

	// Sort descending by score (simple insertion sort — memory stores are small)
	for i := 1; i < len(results); i++ {
		for j := i; j > 0 && results[j].score > results[j-1].score; j-- {
			results[j], results[j-1] = results[j-1], results[j]
		}
	}

	if len(results) > limit {
		results = results[:limit]
	}

	out := make([]*Memory, 0, len(results))
	for _, r := range results {
		r.m.AccessedAt = time.Now().UTC()
		_ = s.save(r.m)
		out = append(out, r.m)
	}
	return out
}

// Forget removes a memory by ID. Also removes the key index if present.
func (s *Store) Forget(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	m := s.load(id)
	if m == nil {
		return false
	}
	if m.Key != "" {
		s.db.Delete(keyPrefix + m.Key)
	}
	s.db.Delete(memPrefix + id)
	return true
}

// ForgetByKey removes the memory with the given semantic key.
func (s *Store) ForgetByKey(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	idVal, err := s.db.Get(keyPrefix + key)
	if err != nil {
		return false
	}
	id, ok := idVal.(string)
	if !ok {
		return false
	}
	s.db.Delete(keyPrefix + key)
	s.db.Delete(memPrefix + id)
	return true
}

// List returns all memories, optionally filtered by type, up to limit.
func (s *Store) List(typeFilter string, limit int) []*Memory {
	if limit <= 0 {
		limit = 50
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	all := s.allMemories()
	out := make([]*Memory, 0, len(all))
	for _, m := range all {
		if typeFilter != "" && m.Type != typeFilter {
			continue
		}
		out = append(out, m)
		if len(out) >= limit {
			break
		}
	}
	return out
}

// Count returns the total number of stored memories.
func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.db.FindKeysByPrefix(memPrefix))
}

// Compact removes memories that have not been accessed within idleTimeout,
// exempting memories with importance >= exemptThreshold.
func (s *Store) Compact(idleTimeout time.Duration, exemptThreshold float64) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().UTC().Add(-idleTimeout)
	removed := 0

	for _, m := range s.allMemories() {
		if m.Importance >= exemptThreshold {
			continue
		}
		if m.AccessedAt.Before(cutoff) {
			if m.Key != "" {
				s.db.Delete(keyPrefix + m.Key)
			}
			s.db.Delete(memPrefix + m.ID)
			removed++
		}
	}
	return removed
}

// --- internal helpers ---

func (s *Store) save(m *Memory) error {
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	if err := s.db.Set(memPrefix+m.ID, string(data)); err != nil {
		return err
	}
	if m.Key != "" {
		_ = s.db.Set(keyPrefix+m.Key, m.ID)
	}
	return nil
}

func (s *Store) load(id string) *Memory {
	val, err := s.db.Get(memPrefix + id)
	if err != nil {
		return nil
	}
	raw, ok := val.(string)
	if !ok {
		return nil
	}
	var m Memory
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil
	}
	return &m
}

func (s *Store) allMemories() []*Memory {
	keys := s.db.FindKeysByPrefix(memPrefix)
	out := make([]*Memory, 0, len(keys))
	for _, k := range keys {
		id := strings.TrimPrefix(k, memPrefix)
		if m := s.load(id); m != nil {
			out = append(out, m)
		}
	}
	return out
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
func tokenise(text string) []string {
	var tokens []string
	var buf strings.Builder
	for _, r := range strings.ToLower(text) {
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

// keywordHits counts how many query tokens appear in the content or key.
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
