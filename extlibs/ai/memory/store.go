package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	mcpai "github.com/paularlott/mcp/ai"
	"github.com/paularlott/logger"
	"github.com/paularlott/snapshotkv"
)

const (
	memPrefix = "mem:"
	idxPrefix = "idx:"

	TypeFact       = "fact"
	TypePreference = "preference"
	TypeEvent      = "event"
	TypeNote       = "note"

	DefaultActivityThreshold  = 10
	DefaultMinCompactInterval = 5 * time.Minute
	DefaultMaxCompactInterval = 2 * time.Hour
	DefaultMaxAge             = 180 * 24 * time.Hour
	DefaultPruneThreshold     = 0.1
	DefaultHalfLifeFact       = 90 * 24 * time.Hour
	DefaultHalfLifeEvent      = 30 * 24 * time.Hour
	DefaultHalfLifeNote       = 7 * 24 * time.Hour
	DefaultMinMemoriesForLLM  = 20
	DefaultMaxMemoriesForLLM  = 100
)

// compactionDecision is the parsed response from the LLM for Mode 2.
type compactionDecision struct {
	Merge  []mergeAction `json:"merge"`
	Delete []string      `json:"delete"`
}

type mergeAction struct {
	SourceIDs  []string `json:"source_ids"`
	NewContent string   `json:"new_content"`
	Type       string   `json:"type"`
	Importance float64  `json:"importance"`
}

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

// storeConfig holds all tunable parameters.
type storeConfig struct {
	activityThreshold  int
	minCompactInterval time.Duration
	maxCompactInterval time.Duration
	maxAge             time.Duration
	pruneThreshold     float64
	halfLifeFact       time.Duration
	halfLifeEvent      time.Duration
	halfLifeNote       time.Duration
	minMemoriesForLLM  int
	maxMemoriesForLLM  int
	aiClient           mcpai.Client
	aiModel            string
}

func defaultConfig() storeConfig {
	return storeConfig{
		activityThreshold:  DefaultActivityThreshold,
		minCompactInterval: DefaultMinCompactInterval,
		maxCompactInterval: DefaultMaxCompactInterval,
		maxAge:             DefaultMaxAge,
		pruneThreshold:     DefaultPruneThreshold,
		halfLifeFact:       DefaultHalfLifeFact,
		halfLifeEvent:      DefaultHalfLifeEvent,
		halfLifeNote:       DefaultHalfLifeNote,
		minMemoriesForLLM:  DefaultMinMemoriesForLLM,
		maxMemoriesForLLM:  DefaultMaxMemoriesForLLM,
	}
}

// Option is a functional option for Store.
type Option func(*storeConfig)

func WithActivityThreshold(n int) Option {
	return func(c *storeConfig) { c.activityThreshold = n }
}

func WithMinCompactInterval(d time.Duration) Option {
	return func(c *storeConfig) { c.minCompactInterval = d }
}

func WithMaxCompactInterval(d time.Duration) Option {
	return func(c *storeConfig) { c.maxCompactInterval = d }
}

func WithMaxAge(d time.Duration) Option {
	return func(c *storeConfig) { c.maxAge = d }
}

func WithPruneThreshold(f float64) Option {
	return func(c *storeConfig) { c.pruneThreshold = f }
}

func WithHalfLifeFact(d time.Duration) Option {
	return func(c *storeConfig) { c.halfLifeFact = d }
}

func WithHalfLifeEvent(d time.Duration) Option {
	return func(c *storeConfig) { c.halfLifeEvent = d }
}

func WithHalfLifeNote(d time.Duration) Option {
	return func(c *storeConfig) { c.halfLifeNote = d }
}

func WithMinMemoriesForLLM(n int) Option {
	return func(c *storeConfig) { c.minMemoriesForLLM = n }
}

func WithMaxMemoriesForLLM(n int) Option {
	return func(c *storeConfig) { c.maxMemoriesForLLM = n }
}

// WithAIClient enables Mode 2 LLM-based compaction.
// If client is nil, Mode 2 is disabled.
func WithAIClient(client mcpai.Client, model string) Option {
	return func(c *storeConfig) {
		c.aiClient = client
		c.aiModel = model
	}
}

// Store is a memory store backed by a snapshotkv DB.
// It does not own the DB — the caller manages its lifecycle.
type Store struct {
	mu                 sync.RWMutex
	db                 *snapshotkv.DB
	cfg                storeConfig
	memoriesSinceCompact int
	lastCompaction     time.Time
	compactionInProgress atomic.Bool
	log                logger.Logger
}

// New creates a Store using the provided DB and optional functional options.
func New(db *snapshotkv.DB, opts ...Option) *Store {
	cfg := defaultConfig()
	for _, o := range opts {
		o(&cfg)
	}
	return &Store{
		db:             db,
		cfg:            cfg,
		lastCompaction: time.Now(),
		log:            logger.NewNullLogger(),
	}
}

// Remember stores a memory and returns it with a UUIDv7 ID.
// After saving, checks whether compaction should be triggered.
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
	if err := s.save(m); err != nil {
		s.mu.Unlock()
		return nil, err
	}
	s.memoriesSinceCompact++
	added := s.memoriesSinceCompact
	since := time.Since(s.lastCompaction)
	shouldCompact := !s.compactionInProgress.Load() &&
		added > 0 && (
		(added >= s.cfg.activityThreshold && since >= s.cfg.minCompactInterval) ||
			since >= s.cfg.maxCompactInterval)
	if shouldCompact {
		s.memoriesSinceCompact = 0
		s.lastCompaction = time.Now()
	}
	s.mu.Unlock()

	if shouldCompact {
		s.log.Debug("compact triggered",
			"reason", triggerReason(added, s.cfg.activityThreshold, since, s.cfg.maxCompactInterval),
			"added", added,
			"since", since.Round(time.Second))
		s.compactionInProgress.Store(true)
		go func() {
			defer s.compactionInProgress.Store(false)
			s.compact()
		}()
	} else {
		s.log.Debug("compact check",
			"added", added,
			"threshold", s.cfg.activityThreshold,
			"since", since.Round(time.Second))
	}

	return m, nil
}

func triggerReason(added, threshold int, since, maxInterval time.Duration) string {
	if since >= maxInterval {
		return "max_interval"
	}
	if added >= threshold {
		return "activity_threshold"
	}
	return "unknown"
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

	for i := 1; i < len(results); i++ {
		for j := i; j > 0 && results[j].score > results[j-1].score; j-- {
			results[j], results[j-1] = results[j-1], results[j]
		}
	}

	if len(results) > limit {
		results = results[:limit]
	}

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

// compact runs Mode 1 (and optionally Mode 2) compaction. Called from a goroutine.
func (s *Store) compact() {
	start := time.Now()
	totalRemoved := 0

	s.mu.RLock()
	total := s.db.Count(idxPrefix)
	s.mu.RUnlock()

	s.log.Info("mode 1 compact started", "memories", total)
	totalRemoved += s.compactMode1()

	if s.cfg.aiClient != nil {
		totalRemoved += s.compactMode2()
	}

	s.log.Info("compact complete", "total_removed", totalRemoved, "duration", time.Since(start).Round(time.Millisecond))
}

// compactMode1 applies rule-based pruning: hard age cap then decay threshold.
// Returns the number of memories deleted.
func (s *Store) compactMode1() int {
	now := time.Now().UTC()

	s.mu.RLock()
	var candidates []*Memory
	s.scanType("", func(m *Memory) bool {
		candidates = append(candidates, m)
		return true
	})
	s.mu.RUnlock()

	var toDelete []string
	ageCap, decay := 0, 0

	for _, m := range candidates {
		age := now.Sub(m.AccessedAt)

		if age > s.cfg.maxAge {
			toDelete = append(toDelete, m.ID)
			ageCap++
			continue
		}

		factor := s.decayFactor(m.Type, age)
		if m.Importance*factor < s.cfg.pruneThreshold {
			toDelete = append(toDelete, m.ID)
			decay++
		}
	}

	s.log.Info("mode 1 pruned", "total", len(toDelete), "age_cap", ageCap, "decay", decay)

	if len(toDelete) == 0 {
		return 0
	}

	s.mu.Lock()
	_ = s.db.BeginTransaction()
	for _, id := range toDelete {
		val, err := s.db.Get(idxPrefix + id)
		if err != nil {
			continue
		}
		if key, ok := val.(string); ok {
			s.db.Delete(key)
		}
		s.db.Delete(idxPrefix + id)
	}
	_ = s.db.Commit()
	s.mu.Unlock()

	return len(toDelete)
}

// compactMode2 uses an LLM to deduplicate and summarise memories.
// Runs after Mode 1. Returns the number of memories removed/replaced.
func (s *Store) compactMode2() int {
	s.mu.RLock()
	var all []*Memory
	s.scanType("", func(m *Memory) bool {
		all = append(all, m)
		return true
	})
	s.mu.RUnlock()

	if len(all) < s.cfg.minMemoriesForLLM {
		s.log.Debug("mode 2 skipped: below min threshold", "count", len(all), "min", s.cfg.minMemoriesForLLM)
		return 0
	}

	// Representative sample across time: sort by AccessedAt, then pick evenly spaced.
	sort.Slice(all, func(i, j int) bool {
		return all[i].AccessedAt.Before(all[j].AccessedAt)
	})
	batch := all
	if len(batch) > s.cfg.maxMemoriesForLLM {
		batch = sampleEvenly(all, s.cfg.maxMemoriesForLLM)
	}

	s.log.Info("mode 2 compact started", "candidates", len(batch))

	prompt := buildCompactionPrompt(batch)
	s.log.Debug("mode 2 LLM call", "memories", len(batch))

	resp, err := s.cfg.aiClient.ChatCompletion(context.Background(), mcpai.ChatCompletionRequest{
		Model: s.cfg.aiModel,
		Messages: []mcpai.Message{
			{Role: "system", Content: "You are a memory compaction assistant. Respond only with valid JSON."},
			{Role: "user", Content: prompt},
		},
	})
	if err != nil {
		s.log.Error("mode 2 LLM failed", "error", err)
		return 0
	}

	var content string
	if len(resp.Choices) > 0 {
		if c, ok := resp.Choices[0].Message.Content.(string); ok {
			content = c
		}
	}

	// Strip markdown code fences if present.
	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "```") {
		if idx := strings.Index(content, "\n"); idx != -1 {
			content = content[idx+1:]
		}
		content = strings.TrimSuffix(strings.TrimSpace(content), "```")
	}

	var decisions compactionDecision
	if err := json.Unmarshal([]byte(content), &decisions); err != nil {
		s.log.Error("mode 2 failed to parse LLM response", "error", err)
		return 0
	}

	count := s.applyDecisions(decisions)
	s.log.Info("mode 2 applied", "changes", count, "merged", len(decisions.Merge), "deleted", len(decisions.Delete))
	return count
}

// applyDecisions applies the LLM compaction decisions to the store.
func (s *Store) applyDecisions(decisions compactionDecision) int {
	count := 0

	// Process merges: delete source memories, create new merged memory.
	for _, merge := range decisions.Merge {
		if len(merge.SourceIDs) == 0 || merge.NewContent == "" {
			continue
		}
		memType := merge.Type
		if memType == "" {
			memType = TypeNote
		}
		importance := merge.Importance
		if importance <= 0 {
			importance = 0.5
		}

		// Delete sources.
		s.mu.Lock()
		_ = s.db.BeginTransaction()
		for _, id := range merge.SourceIDs {
			val, err := s.db.Get(idxPrefix + id)
			if err != nil {
				continue
			}
			if key, ok := val.(string); ok {
				s.db.Delete(key)
			}
			s.db.Delete(idxPrefix + id)
			count++
		}
		_ = s.db.Commit()
		s.mu.Unlock()

		// Create merged memory.
		_, _ = s.Remember(merge.NewContent, memType, importance)
	}

	// Process deletes.
	if len(decisions.Delete) > 0 {
		s.mu.Lock()
		_ = s.db.BeginTransaction()
		for _, id := range decisions.Delete {
			val, err := s.db.Get(idxPrefix + id)
			if err != nil {
				continue
			}
			if key, ok := val.(string); ok {
				s.db.Delete(key)
			}
			s.db.Delete(idxPrefix + id)
			count++
		}
		_ = s.db.Commit()
		s.mu.Unlock()
	}

	return count
}

// buildCompactionPrompt builds the LLM prompt for Mode 2.
func buildCompactionPrompt(memories []*Memory) string {
	var sb strings.Builder
	sb.WriteString(`You are managing a memory store. Given these memories, decide:
1. Which can be merged (combine into one)
2. Which are outdated and should be deleted
3. Which are duplicates

Memories:
`)
	for _, m := range memories {
		sb.WriteString(fmt.Sprintf("[id: %s] (%s, importance: %.1f) %q\n", m.ID, m.Type, m.Importance, m.Content))
	}
	sb.WriteString(`
Respond in JSON. Only include merge and delete — anything not listed is kept as-is:
{
  "merge": [
    {"source_ids": ["id1", "id2"], "new_content": "combined content", "type": "preference", "importance": 0.9}
  ],
  "delete": ["id3"]
}`)
	return sb.String()
}

// sampleEvenly picks n items evenly distributed across the slice.
func sampleEvenly(items []*Memory, n int) []*Memory {
	if len(items) <= n {
		return items
	}
	result := make([]*Memory, n)
	for i := 0; i < n; i++ {
		idx := i * (len(items) - 1) / (n - 1)
		result[i] = items[idx]
	}
	return result
}

// decayFactor returns the exponential decay multiplier for a memory type and age.
// preference never decays (returns 1.0 always).
func (s *Store) decayFactor(memType string, age time.Duration) float64 {
	if memType == TypePreference || age <= 0 {
		return 1.0
	}
	var halfLife time.Duration
	switch memType {
	case TypeFact:
		halfLife = s.cfg.halfLifeFact
	case TypeEvent:
		halfLife = s.cfg.halfLifeEvent
	default: // note and any unknown type
		halfLife = s.cfg.halfLifeNote
	}
	if halfLife <= 0 {
		return 1.0
	}
	exponent := float64(age) / float64(halfLife)
	return math.Pow(0.5, exponent)
}

// --- internal helpers ---

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
