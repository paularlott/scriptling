package memory

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math"
	"strings"
	"sync"
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

	DefaultMaxAge                  = 180 * 24 * time.Hour
	DefaultPruneThreshold          = 0.1
	DefaultHalfLifeFact            = 90 * 24 * time.Hour
	DefaultHalfLifeEvent           = 30 * 24 * time.Hour
	DefaultHalfLifeNote            = 7 * 24 * time.Hour
	DefaultSimilarityHighThreshold = 0.85 // MinHash >= this -> update existing in place
	DefaultSimilarityMidThreshold  = 0.50 // MinHash >= this -> ask LLM if available

	// MinHash signature size - 64 hashes gives good accuracy with low overhead (256 bytes)
	minHashSize = 64
)

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
	MinHash    []uint32  `msgpack:"min_hash"` // pre-computed MinHash signature for similarity
}

// storeConfig holds all tunable parameters.
type storeConfig struct {
	maxAge                  time.Duration
	pruneThreshold          float64
	halfLifeFact            time.Duration
	halfLifeEvent           time.Duration
	halfLifeNote            time.Duration
	similarityHighThreshold float64
	similarityMidThreshold  float64
	aiClient                mcpai.Client
	aiModel                 string
	logger                  logger.Logger
}

func defaultConfig() storeConfig {
	return storeConfig{
		maxAge:                  DefaultMaxAge,
		pruneThreshold:          DefaultPruneThreshold,
		halfLifeFact:            DefaultHalfLifeFact,
		halfLifeEvent:           DefaultHalfLifeEvent,
		halfLifeNote:            DefaultHalfLifeNote,
		similarityHighThreshold: DefaultSimilarityHighThreshold,
		similarityMidThreshold:  DefaultSimilarityMidThreshold,
	}
}

// Option is a functional option for Store.
type Option func(*storeConfig)

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

// WithSimilarityMergeRange sets the Jaccard similarity thresholds for pre-flight
// deduplication at write time.
// high: score >= high -> update existing memory in place (no LLM needed).
// mid:  score >= mid  -> ask LLM to merge or keep separate (if LLM configured).
func WithSimilarityMergeRange(mid, high float64) Option {
	return func(c *storeConfig) {
		c.similarityMidThreshold = mid
		c.similarityHighThreshold = high
	}
}

// WithLogger sets the logger for the store.
func WithLogger(l logger.Logger) Option {
	return func(c *storeConfig) { c.logger = l }
}

// WithAIClient enables LLM-based similarity resolution for compaction.
// If client is nil, only rule-based pruning and auto-merging are performed.
func WithAIClient(client mcpai.Client, model string) Option {
	return func(c *storeConfig) {
		c.aiClient = client
		c.aiModel = model
	}
}

// Store is a memory store backed by a snapshotkv DB.
// It does not own the DB - the caller manages its lifecycle.
type Store struct {
	mu  sync.RWMutex
	db  *snapshotkv.DB
	cfg storeConfig
	log logger.Logger
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
		db:  db,
		cfg: cfg,
		log: log,
	}
}

// Remember stores a memory and returns it with a UUIDv7 ID.
// Before saving, performs a pre-flight similarity check against existing memories
// of the same type:
//   - score >= similarityHighThreshold -> update the existing memory in place
//   - score >= similarityMidThreshold  -> ask LLM to merge or keep (if configured)
//
// After saving, checks whether background compaction should be triggered.
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

	// Pre-flight: find the closest existing memory of the same type.
	best, bestScore := s.findMostSimilar(content, memType)

	if best != nil && bestScore >= s.cfg.similarityHighThreshold {
		// Near-exact duplicate - update in place, no new memory.
		s.log.Debug("pre-flight merge", "score", bestScore, "existing", best.ID)
		updated := s.updateExisting(best, content, importance)
		return updated, nil
	}

	if best != nil && bestScore >= s.cfg.similarityMidThreshold && s.cfg.aiClient != nil {
		// Ambiguous - ask the LLM whether to merge or keep separate.
		s.log.Debug("pre-flight LLM resolve", "score", bestScore, "existing", best.ID)
		if merged := s.resolveSimilarWithLLM(content, importance, best); merged != nil {
			return merged, nil
		}
		// LLM said keep separate (or failed) - fall through to normal save.
	}

	return s.saveNew(content, memType, importance)
}

// saveNew creates a new memory entry.
func (s *Store) saveNew(content, memType string, importance float64) (*Memory, error) {
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
		MinHash:    computeMinHash(content),
	}

	s.mu.Lock()
	if err := s.save(m); err != nil {
		s.mu.Unlock()
		return nil, err
	}
	s.mu.Unlock()

	return m, nil
}

// findMostSimilar returns the existing memory of the given type with the highest
// MinHash similarity to content, and its score. Returns nil if the store is empty.
func (s *Store) findMostSimilar(content, memType string) (*Memory, float64) {
	newHash := computeMinHash(content)
	var best *Memory
	var bestScore float64
	s.mu.RLock()
	s.scanType(memType, func(m *Memory) bool {
		if score := minHashSimilarity(newHash, m.MinHash); score > bestScore {
			bestScore = score
			best = m
		}
		return true
	})
	s.mu.RUnlock()
	return best, bestScore
}

// updateExisting updates an existing memory's content and importance (taking the
// higher value) and refreshes AccessedAt. Returns the updated memory.
func (s *Store) updateExisting(existing *Memory, newContent string, newImportance float64) *Memory {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing.Content = newContent
	existing.MinHash = computeMinHash(newContent) // recompute for new content
	if newImportance > existing.Importance {
		existing.Importance = newImportance
	}
	existing.AccessedAt = time.Now().UTC()
	_ = s.save(existing)
	return existing
}

// resolveSimilarWithLLM sends the new content and the closest existing memory to
// the LLM and asks whether to merge them. Returns the merged/updated memory if the
// LLM decides to merge, or nil if it decides to keep them separate.
func (s *Store) resolveSimilarWithLLM(newContent string, importance float64, existing *Memory) *Memory {
	prompt := fmt.Sprintf(
		"Existing memory:\n%s (%s, importance: %.1f) %q\n\nNew memory:\n%q\n",
		existing.ID, existing.Type, existing.Importance, existing.Content, newContent,
	)
	resp, err := s.cfg.aiClient.ChatCompletion(context.Background(), mcpai.ChatCompletionRequest{
		Model: s.cfg.aiModel,
		Messages: []mcpai.Message{
			{Role: "system", Content: resolveSystemPrompt},
			{Role: "user", Content: prompt},
		},
	})
	if err != nil {
		s.log.Error("LLM resolve failed", "error", err)
		return nil
	}

	var content string
	if len(resp.Choices) > 0 {
		if c, ok := resp.Choices[0].Message.Content.(string); ok {
			content = c
		}
	}
	content = stripThinkingBlocks(content)
	content = extractJSON(content)

	var decision struct {
		Action     string  `json:"action"` // "merge" or "keep"
		NewContent string  `json:"new_content"`
		Importance float64 `json:"importance"`
	}
	if err := json.Unmarshal([]byte(content), &decision); err != nil {
		s.log.Error("failed to parse LLM resolve response", "error", err)
		return nil
	}
	if decision.Action != "merge" || decision.NewContent == "" {
		return nil
	}
	mergedImportance := decision.Importance
	if mergedImportance <= 0 {
		mergedImportance = importance
		if existing.Importance > mergedImportance {
			mergedImportance = existing.Importance
		}
	}
	return s.updateExisting(existing, decision.NewContent, mergedImportance)
}

const resolveSystemPrompt = `You are a memory deduplication assistant. You are given an existing memory and a new memory that are similar but not identical.

Decide whether to merge them into one updated memory, or keep them as separate memories.

Merge when: they describe the same fact/preference/event with updated or complementary details.
Keep separate when: they cover genuinely different subjects or both pieces of information are independently useful.

Respond with ONLY a valid JSON object - no explanation, no markdown, no code fences.

Schema:
{ "action": "merge", "new_content": "<single concise sentence>", "importance": <0.0-1.0> }
or
{ "action": "keep" }

IMPORTANT: new_content must be a single concise sentence. Do not pad or combine unrelated facts.`

// Recall searches memories by keyword and semantic similarity, returning up to limit results ranked by score.
func (s *Store) Recall(query string, limit int, typeFilter string) []*Memory {
	if limit <= 0 {
		limit = 10
	}

	now := time.Now().UTC()
	queryLower := strings.ToLower(strings.TrimSpace(query))
	queryTokens := tokenise(queryLower)
	queryMinHash := computeMinHash(queryLower)

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
			// Hybrid scoring: keyword hits + MinHash similarity
			contentHits := keywordHits(queryTokens, m.Content)
			semScore := minHashSimilarity(queryMinHash, m.MinHash)
			// Require at least some relevance (keyword match OR semantic similarity)
			if contentHits == 0 && semScore < 0.1 {
				return true
			}
			score = float64(contentHits)*0.3 + semScore*0.3 + m.Importance*0.2 + recencyScore(m, now)*0.2
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

// Compact runs compaction synchronously (prune + deduplicate).
// Returns the number of memories remaining after compaction.
func (s *Store) Compact() int {
	s.compact()

	s.mu.RLock()
	remaining := s.db.Count(idxPrefix)
	s.mu.RUnlock()
	return remaining
}

// compact runs pruning followed by pairwise similarity-based deduplication.
func (s *Store) compact() {
	start := time.Now()

	s.mu.RLock()
	total := s.db.Count(idxPrefix)
	s.mu.RUnlock()

	s.log.Debug("compact started", "memories", total)

	// Phase 1: Prune by age and decay
	pruned := s.prune()

	// Phase 2: Pairwise similarity deduplication (if AI client available)
	merged := 0
	if s.cfg.aiClient != nil {
		merged = s.deduplicateSimilar()
	}

	s.mu.RLock()
	remaining := s.db.Count(idxPrefix)
	s.mu.RUnlock()

	s.log.Info("compact complete",
		"pruned", pruned,
		"merged", merged,
		"remaining", remaining,
		"duration", time.Since(start).Round(time.Millisecond))
}

// prune applies rule-based pruning: hard age cap and decay threshold.
// Returns the number of memories deleted.
func (s *Store) prune() int {
	now := time.Now().UTC()
	var toDelete []string
	ageCap, decay := 0, 0

	s.mu.RLock()
	s.scanType("", func(m *Memory) bool {
		age := now.Sub(m.AccessedAt)
		if age > s.cfg.maxAge {
			toDelete = append(toDelete, m.ID)
			ageCap++
			return true
		}
		if m.Importance*s.decayFactor(m.Type, age) < s.cfg.pruneThreshold {
			toDelete = append(toDelete, m.ID)
			decay++
		}
		return true
	})
	s.mu.RUnlock()

	if len(toDelete) == 0 {
		return 0
	}

	s.log.Info("pruned", "total", len(toDelete), "age_cap", ageCap, "decay", decay)

	s.mu.Lock()
	_ = s.db.BeginTransaction()
	for _, id := range toDelete {
		s.deleteByID(id)
	}
	_ = s.db.Commit()
	s.mu.Unlock()

	return len(toDelete)
}

// memData is a compact struct for deduplication (avoids holding full Memory objects).
type memData struct {
	id, content, memType string
	importance            float64
	minHash               []uint32
}

// deduplicateSimilar scans all memories for similar pairs and merges them.
// Uses the same logic as Remember's pre-flight check: high threshold auto-merges,
// mid threshold asks LLM. Returns count of memories removed via merging.
func (s *Store) deduplicateSimilar() int {
	s.mu.RLock()
	byType := make(map[string][]memData)
	s.scanType("", func(m *Memory) bool {
		byType[m.Type] = append(byType[m.Type], memData{
			id:         m.ID,
			content:    m.Content,
			memType:    m.Type,
			importance: m.Importance,
			minHash:    m.MinHash,
		})
		return true
	})
	s.mu.RUnlock()

	totalMerged := 0
	for memType, memories := range byType {
		if len(memories) < 2 {
			continue
		}
		merged := s.deduplicateType(memories)
		if merged > 0 {
			s.log.Debug("deduplicated", "type", memType, "merged", merged)
			totalMerged += merged
		}
	}
	return totalMerged
}

// deduplicateType finds and merges similar pairs within a single type.
// Does not hold locks during LLM calls.
func (s *Store) deduplicateType(memories []memData) int {
	merged := make(map[string]bool)
	mergeCount := 0

	for i := 0; i < len(memories); i++ {
		if merged[memories[i].id] {
			continue
		}
		for j := i + 1; j < len(memories); j++ {
			if merged[memories[j].id] {
				continue
			}

			score := minHashSimilarity(memories[i].minHash, memories[j].minHash)
			if score < s.cfg.similarityMidThreshold {
				continue
			}

			existingID := memories[i].id
			candidateID := memories[j].id

			if score >= s.cfg.similarityHighThreshold {
				// Auto-merge: update existing, delete candidate
				s.log.Debug("auto-merge", "score", score, "into", existingID, "from", candidateID)
				s.mu.Lock()
				s.mergeInto(existingID, memories[j].content, memories[j].importance)
				s.deleteByID(candidateID)
				s.mu.Unlock()
				merged[candidateID] = true
				mergeCount++
			} else if s.cfg.aiClient != nil {
				// Ask LLM to resolve (no lock held during call)
				if s.resolveAndMergePair(memories[i], memories[j]) {
					merged[candidateID] = true
					mergeCount++
				}
			}
		}
	}
	return mergeCount
}

// mergeInto updates an existing memory with new content and importance.
// Caller must hold lock.
func (s *Store) mergeInto(existingID, newContent string, newImportance float64) {
	m := s.getMemoryByID(existingID)
	if m == nil {
		return
	}
	m.Content = newContent
	m.MinHash = computeMinHash(newContent) // recompute for new content
	if newImportance > m.Importance {
		m.Importance = newImportance
	}
	m.AccessedAt = time.Now().UTC()
	_ = s.save(m)
}

// getMemoryByID retrieves a memory by ID. Caller must hold lock.
func (s *Store) getMemoryByID(id string) *Memory {
	keyVal, err := s.db.Get(idxPrefix + id)
	if err != nil {
		return nil
	}
	key, ok := keyVal.(string)
	if !ok {
		return nil
	}
	val, err := s.db.Get(key)
	if err != nil {
		return nil
	}
	return toMemory(val)
}

// resolveAndMergePair asks the LLM whether to merge two memories.
// Returns true if they were merged (candidate deleted into existing).
// Does not hold locks during LLM call.
func (s *Store) resolveAndMergePair(existing, candidate memData) bool {
	prompt := fmt.Sprintf(
		"Memory A:\n%s (%s, importance: %.1f) %q\n\nMemory B:\n%s (%s, importance: %.1f) %q\n",
		existing.id, existing.memType, existing.importance, existing.content,
		candidate.id, candidate.memType, candidate.importance, candidate.content,
	)

	resp, err := s.cfg.aiClient.ChatCompletion(context.Background(), mcpai.ChatCompletionRequest{
		Model: s.cfg.aiModel,
		Messages: []mcpai.Message{
			{Role: "system", Content: resolveSystemPrompt},
			{Role: "user", Content: prompt},
		},
	})
	if err != nil {
		s.log.Error("LLM resolve pair failed", "error", err)
		return false
	}

	var content string
	if len(resp.Choices) > 0 {
		if c, ok := resp.Choices[0].Message.Content.(string); ok {
			content = c
		}
	}
	content = stripThinkingBlocks(content)
	content = extractJSON(content)

	var decision struct {
		Action     string  `json:"action"` // "merge" or "keep"
		NewContent string  `json:"new_content"`
		Importance float64 `json:"importance"`
	}
	if err := json.Unmarshal([]byte(content), &decision); err != nil {
		s.log.Error("failed to parse LLM resolve response", "error", err)
		return false
	}

	if decision.Action != "merge" || decision.NewContent == "" {
		return false
	}

	// Merge: update existing with merged content, delete candidate
	s.log.Debug("LLM merge", "into", existing.id, "from", candidate.id)
	mergedImportance := decision.Importance
	if mergedImportance <= 0 {
		mergedImportance = existing.importance
		if candidate.importance > mergedImportance {
			mergedImportance = candidate.importance
		}
	}

	s.mu.Lock()
	s.mergeInto(existing.id, decision.NewContent, mergedImportance)
	s.deleteByID(candidate.id)
	s.mu.Unlock()

	return true
}

// deleteByID removes a memory by ID (caller must hold lock).
func (s *Store) deleteByID(id string) {
	val, err := s.db.Get(idxPrefix + id)
	if err != nil {
		return
	}
	if key, ok := val.(string); ok {
		s.db.Delete(key)
	}
	s.db.Delete(idxPrefix + id)
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

// stripThinkingBlocks removes <think...</think}> and similar reasoning blocks.
func stripThinkingBlocks(s string) string {
	for _, pair := range [][2]string{
		{"<think", "</think"},
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

// --- MinHash functions ---

// computeMinHash generates a MinHash signature for the given text.
// Uses FNV-1a hash with different seeds for each hash function.
func computeMinHash(text string) []uint32 {
	tokens := tokenise(strings.ToLower(text))
	if len(tokens) == 0 {
		return make([]uint32, minHashSize)
	}

	signature := make([]uint32, minHashSize)
	for i := range signature {
		signature[i] = ^uint32(0) // max uint32
	}

	for _, token := range tokens {
		if len(token) <= 2 {
			continue
		}
		// Compute base hash of token
		h := fnv.New128a()
		h.Write([]byte(token))
		tokenHash := h.Sum(nil)

		// Derive all hash values from the base hash
		for i := 0; i < minHashSize; i++ {
			// Mix in the index to get different hash per position
			seed := uint32(i)
			hashVal := binary.BigEndian.Uint32(tokenHash[:4]) ^ seed
			hashVal ^= hashVal >> 13
			hashVal *= 0x5bd1e995
			hashVal ^= hashVal >> 15

			if hashVal < signature[i] {
				signature[i] = hashVal
			}
		}
	}
	return signature
}

// minHashSimilarity returns the estimated Jaccard similarity from two MinHash signatures.
// It's simply the fraction of matching hash values.
func minHashSimilarity(a, b []uint32) float64 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0
	}
	matches := 0
	for i := range a {
		if a[i] == b[i] {
			matches++
		}
	}
	return float64(matches) / float64(len(a))
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
	// Extract MinHash from storage (msgpack stores as []any with various numeric types)
	if mh, ok := m["min_hash"].([]any); ok {
		mem.MinHash = make([]uint32, len(mh))
		for i, v := range mh {
			switch val := v.(type) {
			case uint32:
				mem.MinHash[i] = val
			case int:
				mem.MinHash[i] = uint32(val)
			case float64:
				mem.MinHash[i] = uint32(val)
			}
		}
	}
	// Recompute MinHash if missing (legacy data without min_hash field)
	if len(mem.MinHash) == 0 && mem.Content != "" {
		mem.MinHash = computeMinHash(mem.Content)
	}
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
