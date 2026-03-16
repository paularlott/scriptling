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
	"github.com/paularlott/logger"
	mcpai "github.com/paularlott/mcp/ai"
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
	DefaultMinMemoriesForLLM  = 5
	DefaultMaxMemoriesForLLM  = 20
	DefaultLLMBatchSize       = 5
	DefaultDuplicateThreshold = 0.85 // Jaccard similarity above this → duplicate
)

// compactionDecision is the parsed response from the LLM for Mode 2.
type compactionDecision struct {
	Merge  []mergeAction   `json:"merge"`
	Delete json.RawMessage `json:"delete"`
}

// deleteIDs extracts IDs from the delete field, tolerating both
// []string and []object (with source_ids or id field) formats.
func (d *compactionDecision) deleteIDs() []string {
	if len(d.Delete) == 0 {
		return nil
	}
	// Try []string first
	var ids []string
	if json.Unmarshal(d.Delete, &ids) == nil {
		return cleanIDs(ids)
	}
	// Try []object with source_ids or id
	var objs []map[string]json.RawMessage
	if json.Unmarshal(d.Delete, &objs) != nil {
		return nil
	}
	for _, obj := range objs {
		if raw, ok := obj["source_ids"]; ok {
			var sub []string
			if json.Unmarshal(raw, &sub) == nil {
				ids = append(ids, sub...)
			}
		} else if raw, ok := obj["id"]; ok {
			var s string
			if json.Unmarshal(raw, &s) == nil {
				ids = append(ids, s)
			}
		}
	}
	return cleanIDs(ids)
}

// cleanIDs strips any "id: " prefix the LLM may copy from the prompt format.
func cleanIDs(ids []string) []string {
	for i, id := range ids {
		ids[i] = strings.TrimPrefix(strings.TrimSpace(id), "id: ")
	}
	return ids
}

type mergeAction struct {
	SourceIDs  []string `json:"source_ids"`
	NewContent string   `json:"new_content"`
	Type       string   `json:"type"`
	Importance float64  `json:"importance"`
}

// cleanMergeIDs strips "id: " prefixes from merge source IDs.
func cleanMergeIDs(ids []string) []string { return cleanIDs(ids) }

// compactionResult holds the outcome of a compaction run.
type compactionResult struct {
	removed int // memories deleted from the store
	added   int // new merged memories created
}

func (r compactionResult) add(other compactionResult) compactionResult {
	return compactionResult{
		removed: r.removed + other.removed,
		added:   r.added + other.added,
	}
}
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
	llmBatchSize       int
	duplicateThreshold float64
	aiClient           mcpai.Client
	aiModel            string
	logger             logger.Logger
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
		llmBatchSize:       DefaultLLMBatchSize,
		duplicateThreshold: DefaultDuplicateThreshold,
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

func WithLLMBatchSize(n int) Option {
	return func(c *storeConfig) { c.llmBatchSize = n }
}

func WithDuplicateThreshold(f float64) Option {
	return func(c *storeConfig) { c.duplicateThreshold = f }
}

// WithLogger sets the logger for the store.
func WithLogger(l logger.Logger) Option {
	return func(c *storeConfig) { c.logger = l }
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
	mu                   sync.RWMutex
	db                   *snapshotkv.DB
	cfg                  storeConfig
	memoriesSinceCompact int
	lastCompaction       time.Time
	compactionInProgress atomic.Bool
	log                  logger.Logger
}

// New creates a Store using the provided DB and optional functional options.
func New(db *snapshotkv.DB, opts ...Option) *Store {
	cfg := defaultConfig()
	for _, o := range opts {
		o(&cfg)
	}

	log := cfg.logger
	if log == nil {
		log = logger.NewNullLogger()
	} else {
		log = log.With("ai", "memory")
	}

	return &Store{
		db:             db,
		cfg:            cfg,
		lastCompaction: time.Now(),
		log:            log,
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
		added > 0 && ((added >= s.cfg.activityThreshold && since >= s.cfg.minCompactInterval) ||
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

// Compact runs compaction synchronously and resets the activity counter and timer.
// Returns the number of memories removed.
func (s *Store) Compact() int {
	if !s.compactionInProgress.CompareAndSwap(false, true) {
		s.log.Debug("compact skipped: already in progress")
		return 0
	}
	defer s.compactionInProgress.Store(false)

	s.mu.Lock()
	s.memoriesSinceCompact = 0
	s.lastCompaction = time.Now()
	s.mu.Unlock()

	s.compact()

	s.mu.RLock()
	remaining := s.db.Count(idxPrefix)
	s.mu.RUnlock()
	return remaining
}

// compact runs Mode 1 (and optionally Mode 2) compaction. Called internally.
func (s *Store) compact() {
	start := time.Now()

	s.mu.RLock()
	total := s.db.Count(idxPrefix)
	s.mu.RUnlock()

	s.log.Debug("compact started", "memories", total)
	res := compactionResult{removed: s.compactMode1()}

	if s.cfg.aiClient != nil {
		res = res.add(s.compactMode2())
	}

	s.mu.RLock()
	remaining := s.db.Count(idxPrefix)
	s.mu.RUnlock()

	s.log.Info("compact complete",
		"removed", res.removed,
		"added", res.added,
		"remaining", remaining,
		"duration", time.Since(start).Round(time.Millisecond))
}

// compactMode1 applies rule-based pruning: hard age cap, decay threshold, and
// exact/near-duplicate detection within each type group.
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

	deleted := make(map[string]bool)
	ageCap, decay, dupes := 0, 0, 0

	// Age cap and decay pass
	for _, m := range candidates {
		age := now.Sub(m.AccessedAt)
		if age > s.cfg.maxAge {
			deleted[m.ID] = true
			ageCap++
			continue
		}
		if m.Importance*s.decayFactor(m.Type, age) < s.cfg.pruneThreshold {
			deleted[m.ID] = true
			decay++
		}
	}

	// Near-duplicate pass: within each type, compare token sets.
	// Keep the newer memory (higher UUIDv7 = later), delete the older.
	byType := make(map[string][]*Memory)
	for _, m := range candidates {
		if !deleted[m.ID] {
			byType[m.Type] = append(byType[m.Type], m)
		}
	}
	for _, group := range byType {
		for i := 0; i < len(group); i++ {
			if deleted[group[i].ID] {
				continue
			}
			tokA := tokenSet(group[i].Content)
			for j := i + 1; j < len(group); j++ {
				if deleted[group[j].ID] {
					continue
				}
				if jaccardSimilarity(tokA, tokenSet(group[j].Content)) >= s.cfg.duplicateThreshold {
					// Delete the older one (lower UUIDv7 string = earlier)
					if group[i].ID < group[j].ID {
						deleted[group[i].ID] = true
					} else {
						deleted[group[j].ID] = true
					}
					dupes++
				}
			}
		}
	}

	s.log.Info("pruned", "total", len(deleted), "age_cap", ageCap, "decay", decay, "dupes", dupes)

	if len(deleted) == 0 {
		return 0
	}

	s.mu.Lock()
	_ = s.db.BeginTransaction()
	for id := range deleted {
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

	return len(deleted)
}

// compactMode2 uses an LLM to deduplicate and summarise memories.
// Processes in small batches per type to keep prompts small and fast.
// Returns the number of memories removed/replaced.
func (s *Store) compactMode2() compactionResult {
	s.mu.RLock()
	byType := make(map[string][]*Memory)
	s.scanType("", func(m *Memory) bool {
		byType[m.Type] = append(byType[m.Type], m)
		return true
	})
	s.mu.RUnlock()

	var total compactionResult
	for memType, group := range byType {
		if len(group) < s.cfg.minMemoriesForLLM {
			s.log.Debug("LLM skip", "type", memType, "count", len(group), "min", s.cfg.minMemoriesForLLM)
			continue
		}
		// Sort oldest-first, cap to maxMemoriesForLLM, then process in batches
		sort.Slice(group, func(i, j int) bool {
			return group[i].AccessedAt.Before(group[j].AccessedAt)
		})
		if len(group) > s.cfg.maxMemoriesForLLM {
			group = group[:s.cfg.maxMemoriesForLLM]
		}
		for i := 0; i < len(group); i += s.cfg.llmBatchSize {
			end := i + s.cfg.llmBatchSize
			if end > len(group) {
				end = len(group)
			}
			batch := group[i:end]
			s.log.Debug("LLM batch", "type", memType, "size", len(batch))
			total = total.add(s.compactBatch(batch))
		}
	}
	return total
}

// compactBatch sends a single small batch to the LLM and applies decisions.
func (s *Store) compactBatch(batch []*Memory) compactionResult {
	prompt := buildCompactionPrompt(batch)

	resp, err := s.cfg.aiClient.ChatCompletion(context.Background(), mcpai.ChatCompletionRequest{
		Model: s.cfg.aiModel,
		Messages: []mcpai.Message{
			{Role: "system", Content: compactionSystemPrompt},
			{Role: "user", Content: prompt},
		},
	})
	if err != nil {
		s.log.Error("LLM batch failed", "error", err)
		return compactionResult{}
	}

	var content string
	if len(resp.Choices) > 0 {
		if c, ok := resp.Choices[0].Message.Content.(string); ok {
			content = c
		}
	}

	content = stripThinkingBlocks(content)
	content = extractJSON(content)

	var decisions compactionDecision
	if err := json.Unmarshal([]byte(content), &decisions); err != nil {
		s.log.Error("failed to parse LLM response", "error", err)
		return compactionResult{}
	}

	res := s.applyDecisions(decisions)
	s.log.Debug("batch applied", "removed", res.removed, "added", res.added)
	return res
}

// rememberDirect saves a memory without touching compaction state.
func (s *Store) rememberDirect(content, memType string, importance float64) (*Memory, error) {
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
	err = s.save(m)
	s.mu.Unlock()
	return m, err
}

// applyDecisions applies the LLM compaction decisions to the store.
func (s *Store) applyDecisions(decisions compactionDecision) compactionResult {
	var res compactionResult

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
		for _, id := range cleanMergeIDs(merge.SourceIDs) {
			val, err := s.db.Get(idxPrefix + id)
			if err != nil {
				continue
			}
			if key, ok := val.(string); ok {
				s.db.Delete(key)
			}
			s.db.Delete(idxPrefix + id)
			res.removed++
		}
		_ = s.db.Commit()
		s.mu.Unlock()

		// Create merged memory directly, bypassing compaction side-effects.
		_, _ = s.rememberDirect(merge.NewContent, memType, importance)
		res.added++
	}

	// Process deletes.
	if deleteIDs := decisions.deleteIDs(); len(deleteIDs) > 0 {
		s.mu.Lock()
		_ = s.db.BeginTransaction()
		for _, id := range deleteIDs {
			val, err := s.db.Get(idxPrefix + id)
			if err != nil {
				continue
			}
			if key, ok := val.(string); ok {
				s.db.Delete(key)
			}
			s.db.Delete(idxPrefix + id)
			res.removed++
		}
		_ = s.db.Commit()
		s.mu.Unlock()
	}

	return res
}

const compactionSystemPrompt = `/nothink
You are a memory compaction assistant. Reduce redundancy in the memory store.

Rules:
- Merge memories that convey the same or very similar information into one cleaner memory.
- Delete memories that are clearly outdated, superseded, or trivially redundant after merging.
- Keep memories that are distinct and still relevant.
- Do not merge memories that cover different subjects — one memory, one fact.
- Write merged content as a single concise sentence. No padding, no filler, no conjunctions joining unrelated facts.

Respond with ONLY a valid JSON object — no explanation, no markdown, no code fences.

Schema:
{
  "merge": [
    {
      "source_ids": ["<exact uuid>", "<exact uuid>"],
      "new_content": "<combined memory text>",
      "type": "<fact|preference|event|note>",
      "importance": <0.0-1.0>
    }
  ],
  "delete": ["<exact uuid>", "<exact uuid>"]
}

IMPORTANT:
- Use the EXACT uuid values from the input, not any surrounding label.
- "merge" and "delete" must be present, even if empty arrays.
- Do NOT include the same id in both merge and delete.
- Do NOT invent new ids.`

// buildCompactionPrompt builds the user prompt for Mode 2 — memories only.
func buildCompactionPrompt(memories []*Memory) string {
	var sb strings.Builder
	sb.WriteString("Memories to compact:\n")
	for _, m := range memories {
		sb.WriteString(fmt.Sprintf("%s (%s, importance: %.1f) %q\n", m.ID, m.Type, m.Importance, m.Content))
	}
	return sb.String()
}

// tokenSet returns the unique token set for a string, used for Jaccard similarity.
func tokenSet(text string) map[string]struct{} {
	set := make(map[string]struct{})
	for _, t := range tokenise(strings.ToLower(text)) {
		if len(t) > 2 {
			set[t] = struct{}{}
		}
	}
	return set
}

// jaccardSimilarity returns the Jaccard similarity between two token sets.
func jaccardSimilarity(a, b map[string]struct{}) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1.0
	}
	intersection := 0
	for t := range a {
		if _, ok := b[t]; ok {
			intersection++
		}
	}
	union := len(a) + len(b) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
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

// stripThinkingBlocks removes <think>...</think> and similar reasoning blocks.
func stripThinkingBlocks(s string) string {
	for _, pair := range [][2]string{
		{"<think>", "</think>"},
		{"<thinking>", "</thinking>"},
		{"<Thought>", "</Thought>"},
		{"<antThinking>", "</antThinking>"},
	} {
		for {
			start := strings.Index(strings.ToLower(s), strings.ToLower(pair[0]))
			if start == -1 {
				break
			}
			end := strings.Index(strings.ToLower(s[start:]), strings.ToLower(pair[1]))
			if end == -1 {
				s = s[:start]
				break
			}
			s = s[:start] + s[start+end+len(pair[1]):]
		}
	}
	return strings.TrimSpace(s)
}

// extractJSON finds the first { ... } JSON object in s, stripping any
// surrounding markdown code fences.
func extractJSON(s string) string {
	// Strip markdown fence if present
	if idx := strings.Index(s, "```"); idx != -1 {
		s = s[idx:]
		if nl := strings.Index(s, "\n"); nl != -1 {
			s = s[nl+1:]
		}
		if end := strings.LastIndex(s, "```"); end != -1 {
			s = s[:end]
		}
	}
	// Find outermost { }
	start := strings.Index(s, "{")
	if start == -1 {
		return strings.TrimSpace(s)
	}
	end := strings.LastIndex(s, "}")
	if end == -1 || end < start {
		return strings.TrimSpace(s)
	}
	return s[start : end+1]
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
