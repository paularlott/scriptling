package similarity

import (
	"strings"
	"testing"

	"github.com/paularlott/scriptling"
)

func TestSentencesSplitting(t *testing.T) {
	got := Sentences("Hello world. This is a test! Is it? Yes.\nA new line here\n\n  Trailing (e.g. this stays) 3.5 mm wide. Done.")
	want := []string{
		"Hello world.",
		"This is a test!",
		"Is it?",
		"Yes.",
		"A new line here",
		"Trailing (e.g. this stays) 3.5 mm wide.",
		"Done.",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("Sentences = %q, want %q", got, want)
	}
	if len(Sentences("")) != 0 || len(Sentences("  \n \n")) != 0 {
		t.Fatal("empty input should yield no sentences")
	}
	// Closing quotes and brackets stay with their sentence.
	got = Sentences(`She said "wait." Then left (quietly.) Next one.`)
	if len(got) != 3 || got[0] != `She said "wait."` || got[1] != "Then left (quietly.)" {
		t.Fatalf("quote handling: %q", got)
	}
}

func TestExtractReturnsTextUnchangedWhenItFits(t *testing.T) {
	text := "First point. Second point. Third point."
	if got := Extract(text, ExtractOptions{MaxSentences: 5}); got != text {
		t.Fatalf("within max_sentences should be unchanged: %q", got)
	}
	if got := Extract(text, ExtractOptions{MaxChars: 1000}); got != text {
		t.Fatalf("within max_chars should be unchanged: %q", got)
	}
	if got := Extract("", ExtractOptions{MaxChars: 10}); got != "" {
		t.Fatalf("empty stays empty: %q", got)
	}
	if got := Extract("One sentence only.", ExtractOptions{Ratio: 0.1}); got != "One sentence only." {
		t.Fatalf("a single sentence is never dropped: %q", got)
	}
}

// Representative sentences are kept and an off-topic one goes first; the
// output preserves original order.
func TestExtractKeepsRepresentativeSentencesInOrder(t *testing.T) {
	text := "The invoice for the server migration is due on Friday. " +
		"The migration moves the database server to the new cluster. " +
		"My cat prefers tuna to chicken. " +
		"The server migration window is Saturday night. " +
		"After the migration the old database server is retired."
	got := Extract(text, ExtractOptions{MaxSentences: 4})
	if strings.Contains(got, "cat prefers tuna") {
		t.Fatalf("the off-topic sentence should be dropped first: %q", got)
	}
	if !strings.Contains(got, "invoice for the server migration") || !strings.Contains(got, "old database server is retired") {
		t.Fatalf("representative sentences missing: %q", got)
	}
	// Original order: invoice sentence before retirement sentence.
	if strings.Index(got, "invoice") > strings.Index(got, "retired") {
		t.Fatalf("order not preserved: %q", got)
	}
	// Space-joined prose, not line-joined.
	if strings.Contains(got, "\n") {
		t.Fatalf("prose should be joined with spaces: %q", got)
	}
}

func TestExtractBounds(t *testing.T) {
	sentences := []string{}
	for i := 0; i < 20; i++ {
		sentences = append(sentences, "Point number "+strings.Repeat("x", i%3)+" about the project plan "+string(rune('a'+i))+".")
	}
	text := strings.Join(sentences, " ")

	// Ratio keeps ceil(ratio * n).
	got := Sentences(Extract(text, ExtractOptions{Ratio: 0.25}))
	if len(got) != 5 {
		t.Fatalf("ratio 0.25 of 20 should keep 5, got %d", len(got))
	}
	// max_sentences is a hard cap.
	if got := Sentences(Extract(text, ExtractOptions{MaxSentences: 3})); len(got) != 3 {
		t.Fatalf("max_sentences=3 kept %d", len(got))
	}
	// max_chars is respected including joiners.
	out := Extract(text, ExtractOptions{MaxChars: 120})
	if len([]rune(out)) > 120 || len(Sentences(out)) < 2 {
		t.Fatalf("max_chars=120 produced %d chars / %d sentences: %q", len([]rune(out)), len(Sentences(out)), out)
	}
	// Bounds combine: the tighter one wins.
	if got := Sentences(Extract(text, ExtractOptions{MaxSentences: 10, Ratio: 0.1})); len(got) != 2 {
		t.Fatalf("ratio 0.1 (2) should beat max_sentences 10, kept %d", len(got))
	}
	// No bounds at all: the default ratio applies.
	if got := Sentences(Extract(text, ExtractOptions{})); len(got) != 6 {
		t.Fatalf("default ratio 0.3 of 20 should keep 6, got %d", len(got))
	}
	// A single sentence longer than max_chars is cut rather than dropped.
	long := strings.Repeat("word ", 50) + "end."
	if got := Extract(long+" Short.", ExtractOptions{MaxChars: 30}); len([]rune(got)) == 0 || len([]rune(got)) > 30 {
		t.Fatalf("oversized top sentence should be cut to the budget: %q", got)
	}
}

// Repetitive line-oriented text (a log) is de-duplicated rather than ranked:
// the leading and trailing lines survive, repeated lines that differ only in
// numbers collapse to their first occurrence, and the rare line is kept.
func TestExtractLogModeKeepsRareLines(t *testing.T) {
	var lines []string
	lines = append(lines, "Starting import job 4412")
	for i := 0; i < 40; i++ {
		lines = append(lines, "Processed record "+string(rune('0'+i%10))+" of 40 ok")
	}
	lines = append(lines, "ERROR: foreign key violation on customer 991")
	for i := 0; i < 10; i++ {
		lines = append(lines, "Processed record "+string(rune('0'+i%10))+" of 40 ok")
	}
	lines = append(lines, "Import finished with 1 error")
	text := strings.Join(lines, "\n")

	got := Extract(text, ExtractOptions{MaxSentences: 8})
	if !strings.Contains(got, "ERROR: foreign key violation") {
		t.Fatalf("the rare error line must survive: %q", got)
	}
	if !strings.Contains(got, "Starting import job") || !strings.Contains(got, "Import finished") {
		t.Fatalf("leading and trailing lines must survive: %q", got)
	}
	if strings.Count(got, "Processed record") > 3 {
		t.Fatalf("repeated lines should collapse: %q", got)
	}
	if !strings.Contains(got, "\n") {
		t.Fatalf("line-oriented text should be re-joined with newlines: %q", got)
	}
	if ratio := distinctRatio(Sentences(text)); ratio >= lowVarietyThreshold {
		t.Fatalf("distinct ratio %.2f should flag the text as repetitive", ratio)
	}
}

// The functions are exposed to scripts with keyword bounds.
func TestSimilarityExtractFromScript(t *testing.T) {
	p := scriptling.New()
	Register(p)
	_, err := p.Eval(`
import scriptling.similarity as sim

text = "Alpha ships the parser on Monday. The parser handles quotes and brackets. " \
       "Lunch was excellent today. The parser is tested against twenty cases. " \
       "Beta reviews the parser before release."
sents = sim.sentences(text)
n = len(sents)
short = sim.extract(text, max_sentences=3)
kept = len(sim.sentences(short))
ratio = sim.extract(text, ratio=0.4)
ratio_kept = len(sim.sentences(ratio))
chars = sim.extract(text, max_chars=80)
chars_len = len(chars)
untouched = sim.extract("Just one line.", max_chars=5000) == "Just one line."
`)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	get := func(name string) any {
		v, e := p.GetVar(name)
		if e != nil {
			t.Fatalf("%s: %v", name, e)
		}
		return v
	}
	if get("n").(int64) != 5 {
		t.Fatalf("sentences() = %v, want 5", get("n"))
	}
	if get("kept").(int64) != 3 {
		t.Fatalf("extract(max_sentences=3) kept %v", get("kept"))
	}
	if get("ratio_kept").(int64) != 2 {
		t.Fatalf("extract(ratio=0.4) of 5 should keep 2, kept %v", get("ratio_kept"))
	}
	if get("chars_len").(int64) > 80 {
		t.Fatalf("extract(max_chars=80) returned %v chars", get("chars_len"))
	}
	if get("untouched") != true {
		t.Fatal("text within budget should be returned unchanged")
	}
	short, _ := p.GetVar("short")
	if strings.Contains(short.(string), "Lunch was excellent") {
		t.Fatalf("the off-topic sentence should go first: %q", short)
	}

	// Blank text gives an empty list, never None, so callers can iterate.
	if _, err := p.Eval(`
blank = sim.sentences("")
spaces = sim.sentences("  \n \n")
toks = sim.tokenize("")
blank_ok = isinstance(blank, list) and len(blank) == 0
spaces_ok = isinstance(spaces, list) and len(spaces) == 0
toks_ok = isinstance(toks, list) and len(toks) == 0
`); err != nil {
		t.Fatalf("blank input eval failed: %v", err)
	}
	for _, name := range []string{"blank_ok", "spaces_ok", "toks_ok"} {
		if get(name) != true {
			t.Fatalf("%s: blank input must give an empty list, not None", name)
		}
	}

	// Errors: wrong types are reported, not panics.
	if _, err := p.Eval(`sim.extract(42)`); err == nil {
		t.Fatal("extract(42) should be an error")
	}
	if _, err := p.Eval(`sim.sentences()`); err == nil {
		t.Fatal("sentences() without text should be an error")
	}
}

// A sentence repeated through the text (a signature) is ranked once and kept
// at most once, so repetition cannot make it look like the main point.
func TestExtractCollapsesRepeatedSentences(t *testing.T) {
	sig := "Sent from my phone, please excuse typos."
	text := strings.Join([]string{
		"The claims form rollout starts on Monday for the pilot customers.",
		sig,
		"Pilot customers get the new claims form with the auto-save feature.",
		sig,
		"Support will monitor claims form submissions during the pilot week.",
		sig,
		"After the pilot the claims form goes to every customer.",
		sig,
	}, " ")
	got := Extract(text, ExtractOptions{MaxSentences: 4})
	if strings.Count(got, sig) > 1 {
		t.Fatalf("repeated sentence kept more than once: %q", got)
	}
	kept := Sentences(got)
	content := 0
	for _, s := range kept {
		if strings.Contains(s, "claims form") {
			content++
		}
	}
	if content < 3 {
		t.Fatalf("content sentences should outrank the repeated signature: %q", got)
	}
}
