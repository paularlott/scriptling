package fuzzy

import (
	"testing"

	"github.com/paularlott/scriptling"
)

func TestFuzzyLibraryRegistration(t *testing.T) {
	p := scriptling.New()
	Register(p)

	// Test that the library is registered
	_, err := p.Eval(`
import scriptling.fuzzy as fuzzy
`)
	if err != nil {
		t.Fatalf("Failed to import fuzzy library: %v", err)
	}
}

func TestFuzzySearch(t *testing.T) {
	p := scriptling.New()
	Register(p)

	_, err := p.Eval(`
import scriptling.fuzzy as fuzzy

items = [
    {"id": 1, "name": "Website Redesign"},
    {"id": 2, "name": "Mobile App Development"},
    {"id": 3, "name": "Server Migration"},
]

results = fuzzy.search("web", items, max_results=3)
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

func TestFuzzySearchStrings(t *testing.T) {
	p := scriptling.New()
	Register(p)

	_, err := p.Eval(`
import scriptling.fuzzy as fuzzy

items = ["Apple", "Banana", "Cherry", "Apricot"]
results = fuzzy.search("app", items)
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

func TestFuzzyBest(t *testing.T) {
	p := scriptling.New()
	Register(p)

	_, err := p.Eval(`
import scriptling.fuzzy as fuzzy

items = [
    {"id": 1, "name": "Website Redesign"},
    {"id": 2, "name": "Mobile App Development"},
    {"id": 3, "name": "Server Migration"},
]

result = fuzzy.best("website redesign", items, entity_type="project")
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

func TestFuzzyBestNotFound(t *testing.T) {
	p := scriptling.New()
	Register(p)

	_, err := p.Eval(`
import scriptling.fuzzy as fuzzy

items = [
    {"id": 1, "name": "Website Redesign"},
    {"id": 2, "name": "Mobile App Development"},
]

result = fuzzy.best("xyz123", items, entity_type="project")
found = result['found']
`)
	if err != nil {
		t.Fatalf("Failed to run best: %v", err)
	}

	found, objErr := p.GetVar("found")
	if objErr != nil {
		t.Fatalf("Failed to get result: %v", objErr)
	}
	if found.(bool) != false {
		t.Errorf("Expected found=false for no match")
	}
}

func TestFuzzyScore(t *testing.T) {
	p := scriptling.New()
	Register(p)

	_, err := p.Eval(`
import scriptling.fuzzy as fuzzy

score1 = fuzzy.score("hello", "hello")
score2 = fuzzy.score("hello", "hallo")
score3 = fuzzy.score("hello", "xyz")

# Exact match should be 1.0
if score1 != 1.0:
    raise Exception("Expected 1.0 for exact match")

# Similar should be high
if score2 < 0.5:
    raise Exception("Expected > 0.5 for similar")

# Different should be low
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

func TestFuzzySearchKwargs(t *testing.T) {
	p := scriptling.New()
	Register(p)

	_, err := p.Eval(`
import scriptling.fuzzy as fuzzy

items = [{"id": 1, "name": "Test Project"}]

# Test with kwargs
results = fuzzy.search(query="test", items=items, max_results=5, threshold=0.3)
result_count = len(results)
`)
	if err != nil {
		t.Fatalf("Failed to run search with kwargs: %v", err)
	}

	count, objErr := p.GetVar("result_count")
	if objErr != nil {
		t.Fatalf("Failed to get result: %v", objErr)
	}
	if count.(int64) < 1 {
		t.Errorf("Expected at least 1 result, got %d", count)
	}
}

func TestFuzzyBestKwargs(t *testing.T) {
	p := scriptling.New()
	Register(p)

	_, err := p.Eval(`
import scriptling.fuzzy as fuzzy

items = [{"id": 1, "name": "Active"}, {"id": 2, "name": "Pending"}]

# Test with kwargs
result = fuzzy.best(query="activ", items=items, entity_type="status")
found = result['found']
`)
	if err != nil {
		t.Fatalf("Failed to run best with kwargs: %v", err)
	}

	found, objErr := p.GetVar("found")
	if objErr != nil {
		t.Fatalf("Failed to get result: %v", objErr)
	}
	if found.(bool) != true {
		t.Errorf("Expected found=true")
	}
}

func TestFuzzyHelp(t *testing.T) {
	p := scriptling.New()
	Register(p)

	_, err := p.Eval(`
import scriptling.fuzzy as fuzzy

# Get help - should not error
help(fuzzy.search)
help(fuzzy.best)
help(fuzzy.score)
help_passed = True
`)
	if err != nil {
		t.Fatalf("Failed to get help: %v", err)
	}

	passed, objErr := p.GetVar("help_passed")
	if objErr != nil {
		t.Fatalf("Failed to get result: %v", objErr)
	}
	if passed.(bool) != true {
		t.Errorf("Help tests failed")
	}
}

func TestFuzzyLibraryConstants(t *testing.T) {
	if LibraryName != "scriptling.fuzzy" {
		t.Errorf("LibraryName = %q, want %q", LibraryName, "scriptling.fuzzy")
	}

	if LibraryDesc == "" {
		t.Error("LibraryDesc should not be empty")
	}
}
