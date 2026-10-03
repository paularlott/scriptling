package similarity

import (
	"math"
	"runtime"
	"sort"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

// Sentences splits text into sentences. A sentence ends at ". ", "! " or
// "? " (the terminator may be followed by closing quotes or brackets) when
// the next character starts a new sentence, and at every line break, so
// transcripts, chat logs and pasted output split one line per sentence. The
// returned sentences are trimmed and never empty.
func Sentences(text string) []string {
	var out []string
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		for _, s := range splitLine(line) {
			s = strings.TrimSpace(s)
			if s != "" {
				out = append(out, s)
			}
		}
	}
	return out
}

// splitLine splits one line on sentence terminators. A terminator counts when
// it is followed by whitespace and then an upper-case letter, a digit, or an
// opening quote or bracket, so "e.g. this" and "3.5 mm" stay whole.
func splitLine(line string) []string {
	var out []string
	runes := []rune(line)
	start := 0
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r != '.' && r != '!' && r != '?' {
			continue
		}
		// Swallow runs of terminators and closing punctuation: "?!", ".)", '."'.
		j := i + 1
		for j < len(runes) && (runes[j] == '.' || runes[j] == '!' || runes[j] == '?' || runes[j] == '"' || runes[j] == '\'' || runes[j] == ')' || runes[j] == ']' || runes[j] == '”' || runes[j] == '’') {
			j++
		}
		if j >= len(runes) {
			break
		}
		if !unicode.IsSpace(runes[j]) {
			continue
		}
		k := j
		for k < len(runes) && unicode.IsSpace(runes[k]) {
			k++
		}
		if k >= len(runes) {
			break
		}
		next := runes[k]
		if !(unicode.IsUpper(next) || unicode.IsDigit(next) || next == '"' || next == '\'' || next == '(' || next == '[' || next == '“' || next == '‘') {
			continue
		}
		out = append(out, string(runes[start:j]))
		start = k
		i = k - 1
	}
	if start < len(runes) {
		out = append(out, string(runes[start:]))
	}
	return out
}

// ExtractOptions bounds the output of Extract. Zero means no bound on that
// axis. When every bound is zero, Ratio defaults to DefaultExtractRatio.
type ExtractOptions struct {
	MaxChars     int     // keep sentences until adding the next would exceed this many characters
	MaxSentences int     // keep at most this many sentences
	Ratio        float64 // keep this fraction of the sentences (0 < Ratio <= 1)
}

// DefaultExtractRatio is the fraction of sentences kept when Extract is given
// no bounds.
const DefaultExtractRatio = 0.3

// Tunables for the TextRank pass.
const (
	textRankDamping    = 0.85
	textRankIterations = 40
	textRankTolerance  = 1e-5
	extractVectorDims  = 256
	// lowVarietyThreshold is the distinct-sentence ratio below which a text is
	// treated as repetitive (a log, a stack trace, a status feed) and
	// ranking gives way to de-duplication, since in such text the rare lines
	// carry the information and ranking would keep the repeated ones.
	lowVarietyThreshold = 0.5
	logModeEdgeLines    = 3
)

// Extract returns the most informative sentences of text, in their original
// order, within the given bounds. Text that already fits is returned
// unchanged.
//
// Sentences are scored with TextRank: each sentence is a node, edges are
// weighted by the cosine similarity of the sentences' hashed word vectors,
// and the stationary distribution ranks them, so the sentences most
// representative of the whole text score highest. The similarity matrix is
// built in parallel across CPUs.
//
// Repetitive text (many near-identical lines once digits are normalised, as
// in logs) is handled differently: the first and last few lines are kept
// along with the first occurrence of each distinct line, in order, because
// there the unusual lines are the informative ones.
func Extract(text string, opts ExtractOptions) string {
	sentences := Sentences(text)
	n := len(sentences)
	if n == 0 {
		return strings.TrimSpace(text)
	}

	if opts.MaxChars <= 0 && opts.MaxSentences <= 0 && opts.Ratio <= 0 {
		opts.Ratio = DefaultExtractRatio
	}
	if opts.Ratio > 1 {
		opts.Ratio = 1
	}

	// Within budget already: hand the text back untouched.
	if n == 1 || fitsBudget(sentences, opts) {
		if opts.Ratio > 0 && opts.Ratio < 1 && n > 1 && opts.MaxChars <= 0 && opts.MaxSentences <= 0 {
			// A pure ratio request always reduces.
		} else if opts.MaxChars <= 0 || utf8.RuneCountInString(text) <= opts.MaxChars {
			if opts.MaxSentences <= 0 || n <= opts.MaxSentences {
				if opts.Ratio <= 0 || opts.Ratio >= 1 || n == 1 {
					return strings.TrimSpace(text)
				}
			}
		}
	}

	lineMode := isLineOriented(text, n)
	joiner := " "
	if lineMode {
		joiner = "\n"
	}

	maxSentences := n
	if opts.MaxSentences > 0 && opts.MaxSentences < maxSentences {
		maxSentences = opts.MaxSentences
	}
	if opts.Ratio > 0 && opts.Ratio < 1 {
		byRatio := int(math.Ceil(opts.Ratio * float64(n)))
		if byRatio < 1 {
			byRatio = 1
		}
		if byRatio < maxSentences {
			maxSentences = byRatio
		}
	}

	// Exact repeats (a signature or footer carried through a thread) are
	// ranked once: repetition would otherwise make a sentence look central.
	// Only the first occurrence is a candidate; later copies are never kept.
	unique := firstOccurrences(sentences)

	var order []int
	if distinctRatio(sentences) < lowVarietyThreshold {
		order = logModeOrder(sentences)
	} else {
		order = textRankOrderOf(sentences, unique)
	}

	// Greedy selection by rank within the bounds, then restore original order.
	selected := make([]bool, n)
	count := 0
	chars := 0
	for _, idx := range order {
		if count >= maxSentences {
			break
		}
		length := utf8.RuneCountInString(sentences[idx])
		if opts.MaxChars > 0 {
			add := length
			if count > 0 {
				add += utf8.RuneCountInString(joiner)
			}
			if chars+add > opts.MaxChars {
				if count == 0 {
					// Nothing fits whole: keep the top sentence, cut to the
					// budget, which is then spent.
					sentences[idx] = truncateRunes(sentences[idx], opts.MaxChars)
					selected[idx] = true
					count++
					chars = opts.MaxChars
				}
				continue
			}
			chars += add
		}
		selected[idx] = true
		count++
	}

	var out []string
	for i := 0; i < n; i++ {
		if selected[i] {
			out = append(out, sentences[i])
		}
	}
	return strings.Join(out, joiner)
}

// fitsBudget reports whether every bound that is set is already satisfied.
func fitsBudget(sentences []string, opts ExtractOptions) bool {
	if opts.MaxSentences > 0 && len(sentences) > opts.MaxSentences {
		return false
	}
	if opts.MaxChars > 0 {
		total := 0
		for i, s := range sentences {
			total += utf8.RuneCountInString(s)
			if i > 0 {
				total++
			}
		}
		if total > opts.MaxChars {
			return false
		}
	}
	if opts.Ratio > 0 && opts.Ratio < 1 && len(sentences) > 1 {
		return false
	}
	return true
}

// isLineOriented reports whether most sentence boundaries were line breaks,
// in which case the output is re-joined with newlines.
func isLineOriented(text string, sentenceCount int) bool {
	lines := 0
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		if strings.TrimSpace(line) != "" {
			lines++
		}
	}
	return lines*2 >= sentenceCount && lines > 1
}

// normaliseForVariety lower-cases a sentence and collapses digit runs so
// lines that differ only by timestamps, counters or ids compare equal.
func normaliseForVariety(s string) string {
	var b strings.Builder
	lastDigit := false
	for _, r := range strings.ToLower(s) {
		if unicode.IsDigit(r) {
			if !lastDigit {
				b.WriteByte('0')
			}
			lastDigit = true
			continue
		}
		lastDigit = false
		if unicode.IsLetter(r) || unicode.IsSpace(r) {
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// distinctRatio is the share of sentences that are distinct once normalised.
func distinctRatio(sentences []string) float64 {
	if len(sentences) == 0 {
		return 1
	}
	seen := make(map[string]struct{}, len(sentences))
	for _, s := range sentences {
		seen[normaliseForVariety(s)] = struct{}{}
	}
	return float64(len(seen)) / float64(len(sentences))
}

// logModeOrder ranks repetitive text: the leading and trailing lines first
// (context and outcome), then the first occurrence of each distinct line in
// order. Repeated lines are left out altogether.
func logModeOrder(sentences []string) []int {
	n := len(sentences)
	order := make([]int, 0, n)
	taken := make([]bool, n)
	seen := make(map[string]struct{}, n)
	// takeDistinct records a line unless an equivalent one (digits
	// normalised) has already been taken, so the leading and trailing
	// context lines do not repeat each other either.
	takeDistinct := func(i int) {
		if i < 0 || i >= n || taken[i] {
			return
		}
		key := normaliseForVariety(sentences[i])
		if _, dup := seen[key]; dup {
			return
		}
		seen[key] = struct{}{}
		taken[i] = true
		order = append(order, i)
	}
	for i := 0; i < logModeEdgeLines; i++ {
		takeDistinct(i)
	}
	for i := n - logModeEdgeLines; i < n; i++ {
		takeDistinct(i)
	}
	for i := 0; i < n; i++ {
		takeDistinct(i)
	}
	// Repeats are never added: once every distinct line is in, the rest
	// carries no information, so the result may be shorter than the bound.
	return order
}

// firstOccurrences returns the indexes of the first occurrence of each
// distinct sentence (case- and whitespace-insensitive), in order.
func firstOccurrences(sentences []string) []int {
	seen := make(map[string]struct{}, len(sentences))
	out := make([]int, 0, len(sentences))
	for i, s := range sentences {
		key := strings.Join(strings.Fields(strings.ToLower(s)), " ")
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, i)
	}
	return out
}

// textRankOrderOf ranks the given candidate sentence indexes from highest
// to lowest TextRank score, ties broken by position.
func textRankOrderOf(sentences []string, candidates []int) []int {
	texts := make([]string, len(candidates))
	for i, idx := range candidates {
		texts[i] = sentences[idx]
	}
	ranked := textRankOrder(texts)
	order := make([]int, len(ranked))
	for i, r := range ranked {
		order[i] = candidates[r]
	}
	return order
}

// textRankOrder returns sentence indexes from highest to lowest TextRank
// score, ties broken by position.
func textRankOrder(sentences []string) []int {
	n := len(sentences)
	vectors := make([][]float64, n)
	parallelFor(n, func(i int) {
		vectors[i] = VectorFromText(sentences[i], extractVectorDims)
	})

	// Symmetric similarity matrix; rows computed in parallel.
	sim := make([][]float64, n)
	parallelFor(n, func(i int) {
		row := make([]float64, n)
		for j := 0; j < n; j++ {
			if i == j {
				continue
			}
			score, err := CosineSimilarity(vectors[i], vectors[j])
			if err == nil && score > 0 {
				row[j] = score
			}
		}
		sim[i] = row
	})

	rowSums := make([]float64, n)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			rowSums[i] += sim[i][j]
		}
	}

	scores := make([]float64, n)
	next := make([]float64, n)
	for i := range scores {
		scores[i] = 1
	}
	for iter := 0; iter < textRankIterations; iter++ {
		delta := 0.0
		for i := 0; i < n; i++ {
			sum := 0.0
			for j := 0; j < n; j++ {
				if sim[j][i] > 0 && rowSums[j] > 0 {
					sum += sim[j][i] / rowSums[j] * scores[j]
				}
			}
			next[i] = (1 - textRankDamping) + textRankDamping*sum
			delta += math.Abs(next[i] - scores[i])
		}
		scores, next = next, scores
		if delta < textRankTolerance {
			break
		}
	}

	order := make([]int, n)
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool {
		if scores[order[a]] == scores[order[b]] {
			return order[a] < order[b]
		}
		return scores[order[a]] > scores[order[b]]
	})
	return order
}

// parallelFor runs fn(i) for i in [0, n) across up to GOMAXPROCS goroutines.
func parallelFor(n int, fn func(i int)) {
	workers := runtime.GOMAXPROCS(0)
	if workers > n {
		workers = n
	}
	if workers <= 1 {
		for i := 0; i < n; i++ {
			fn(i)
		}
		return
	}
	var wg sync.WaitGroup
	next := make(chan int, n)
	for i := 0; i < n; i++ {
		next <- i
	}
	close(next)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				fn(i)
			}
		}()
	}
	wg.Wait()
}

func truncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}
