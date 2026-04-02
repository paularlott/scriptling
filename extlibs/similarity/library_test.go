package similarity

import (
	"testing"

	"github.com/paularlott/scriptling"
)

func TestSimilarityLibraryRegistration(t *testing.T) {
	p := scriptling.New()
	Register(p)

	_, err := p.Eval(`
import scriptling.similarity as sim
`)
	if err != nil {
		t.Fatalf("Failed to import similarity library: %v", err)
	}
}

func TestSimilaritySearch(t *testing.T) {
	p := scriptling.New()
	Register(p)

	_, err := p.Eval(`
import scriptling.similarity as sim

items = [
    {"id": 1, "name": "Website Redesign"},
    {"id": 2, "name": "Mobile App Development"},
    {"id": 3, "name": "Server Migration"},
]

results = sim.search("web", items, max_results=3)
result_count = len(results)
`)
	if err != nil {
		t.Fatalf("Failed to run search: %v", err)
	}

	count, objErr := p.GetVar("result_count")
	if objErr != nil {
		t.Fatalf("Failed to get result: %v", objErr)
	}
	if count.(int64) < 1 {
		t.Errorf("Expected at least 1 result, got %d", count)
	}
}

func TestSimilarityBest(t *testing.T) {
	p := scriptling.New()
	Register(p)

	_, err := p.Eval(`
import scriptling.similarity as sim

items = [
    {"id": 1, "name": "Website Redesign"},
    {"id": 2, "name": "Mobile App Development"},
]

result = sim.best("website redesign", items, entity_type="project")
found = result['found']
`)
	if err != nil {
		t.Fatalf("Failed to run best: %v", err)
	}

	found, objErr := p.GetVar("found")
	if objErr != nil {
		t.Fatalf("Failed to get result: %v", objErr)
	}
	if found.(bool) != true {
		t.Errorf("Expected found=true for exact match")
	}
}

func TestSimilarityScore(t *testing.T) {
	p := scriptling.New()
	Register(p)

	_, err := p.Eval(`
import scriptling.similarity as sim

score1 = sim.score("hello", "hello")
score2 = sim.score("hello", "hallo")
score3 = sim.score("hello", "xyz")

if score1 != 1.0:
    raise Exception("Expected 1.0 for exact match")
if score2 < 0.5:
    raise Exception("Expected > 0.5 for similar")
if score3 > 0.5:
    raise Exception("Expected < 0.5 for different")

test_passed = True
`)
	if err != nil {
		t.Fatalf("Failed to run score: %v", err)
	}

	passed, objErr := p.GetVar("test_passed")
	if objErr != nil {
		t.Fatalf("Failed to get result: %v", objErr)
	}
	if passed.(bool) != true {
		t.Errorf("Score tests failed")
	}
}

func TestSimilarityMinHash(t *testing.T) {
	p := scriptling.New()
	Register(p)

	_, err := p.Eval(`
import scriptling.similarity as sim

sig1 = sim.minhash("The quick brown fox")
sig2 = sim.minhash("the quick brown fox")
sig3 = sim.minhash("A totally different sentence")

same = sim.minhash_similarity(sig1, sig2)
diff = sim.minhash_similarity(sig1, sig3)
token_count = len(sim.tokenize("Hello, world! 123"))
`)
	if err != nil {
		t.Fatalf("Failed to run minhash helpers: %v", err)
	}

	same, objErr := p.GetVar("same")
	if objErr != nil {
		t.Fatalf("Failed to get same: %v", objErr)
	}
	diff, objErr := p.GetVar("diff")
	if objErr != nil {
		t.Fatalf("Failed to get diff: %v", objErr)
	}
	tokenCount, objErr := p.GetVar("token_count")
	if objErr != nil {
		t.Fatalf("Failed to get token_count: %v", objErr)
	}

	if same.(float64) != 1.0 {
		t.Fatalf("expected same similarity 1.0, got %v", same)
	}
	if diff.(float64) >= 0.8 {
		t.Fatalf("expected different similarity to be lower, got %v", diff)
	}
	if tokenCount.(int64) != 3 {
		t.Fatalf("expected 3 tokens, got %v", tokenCount)
	}
}

func TestSimilarityLibraryConstants(t *testing.T) {
	if LibraryName != "scriptling.similarity" {
		t.Errorf("LibraryName = %q, want %q", LibraryName, "scriptling.similarity")
	}

	if LibraryDesc == "" {
		t.Error("LibraryDesc should not be empty")
	}
}

func TestSimilaritySearchWithStringItems(t *testing.T) {
	p := scriptling.New()
	Register(p)

	_, err := p.Eval(`
import scriptling.similarity as sim

items = ["apple", "banana", "apricot"]
results = sim.search("app", items, max_results=3, threshold=0.3)
result_count = len(results)
`)
	if err != nil {
		t.Fatalf("Failed to run search with string items: %v", err)
	}

	count, objErr := p.GetVar("result_count")
	if objErr != nil {
		t.Fatalf("Failed to get result_count: %v", objErr)
	}
	if count.(int64) < 1 {
		t.Errorf("Expected at least 1 result, got %d", count)
	}
}

func TestSimilarityBestNotFound(t *testing.T) {
	p := scriptling.New()
	Register(p)

	_, err := p.Eval(`
import scriptling.similarity as sim

items = [
    {"id": 1, "name": "apple"},
    {"id": 2, "name": "banana"},
]

result = sim.best("zzzzzz", items, entity_type="fruit", threshold=0.9)
found = result['found']
has_error = result['error'] != None
`)
	if err != nil {
		t.Fatalf("Failed to run best: %v", err)
	}

	found, objErr := p.GetVar("found")
	if objErr != nil {
		t.Fatalf("Failed to get found: %v", objErr)
	}
	if found.(bool) != false {
		t.Errorf("Expected found=false for no-match query")
	}

	hasError, objErr := p.GetVar("has_error")
	if objErr != nil {
		t.Fatalf("Failed to get has_error: %v", objErr)
	}
	if hasError.(bool) != true {
		t.Errorf("Expected error message to be set when not found")
	}
}

func TestSimilaritySearchCustomKey(t *testing.T) {
	p := scriptling.New()
	Register(p)

	_, err := p.Eval(`
import scriptling.similarity as sim

items = [
    {"id": 1, "label": "Red Widget"},
    {"id": 2, "label": "Blue Gadget"},
]

results = sim.search("red", items, key="label", max_results=3)
result_count = len(results)
`)
	if err != nil {
		t.Fatalf("Failed to run search with custom key: %v", err)
	}

	count, objErr := p.GetVar("result_count")
	if objErr != nil {
		t.Fatalf("Failed to get result_count: %v", objErr)
	}
	if count.(int64) < 1 {
		t.Errorf("Expected at least 1 result with custom key, got %d", count)
	}
}
