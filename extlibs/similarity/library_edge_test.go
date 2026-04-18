package similarity

import (
	"strings"
	"testing"

	"github.com/paularlott/scriptling"
)

func TestSimilarityScoreWithKwargsAndEmptyStrings(t *testing.T) {
	p := scriptling.New()
	Register(p)

	_, err := p.Eval(`
import scriptling.similarity as sim

score_empty = sim.score(s1="", s2="hello")
score_same = sim.score(s1="alpha", s2="alpha")
`)
	if err != nil {
		t.Fatalf("failed to evaluate similarity score kwargs test: %v", err)
	}

	scoreEmpty, objErr := p.GetVar("score_empty")
	if objErr != nil || scoreEmpty != 0.0 {
		t.Fatalf("expected empty score 0.0, got %v (err=%v)", scoreEmpty, objErr)
	}

	scoreSame, objErr := p.GetVar("score_same")
	if objErr != nil || scoreSame != 1.0 {
		t.Fatalf("expected same score 1.0, got %v (err=%v)", scoreSame, objErr)
	}
}

func TestSimilarityMinHashDefaultsAndTokenFiltering(t *testing.T) {
	p := scriptling.New()
	Register(p)

	_, err := p.Eval(`
import scriptling.similarity as sim

sig = sim.minhash("an a at atom", num_hashes=0)
sig_len = len(sig)
tokens = sim.tokenize("A an at atom!")
token_len = len(tokens)
`)
	if err != nil {
		t.Fatalf("failed to evaluate minhash defaults test: %v", err)
	}

	sigLen, objErr := p.GetVar("sig_len")
	if objErr != nil || sigLen != int64(64) {
		t.Fatalf("expected sig_len=64, got %v (err=%v)", sigLen, objErr)
	}

	tokenLen, objErr := p.GetVar("token_len")
	if objErr != nil || tokenLen != int64(4) {
		t.Fatalf("expected token_len=4, got %v (err=%v)", tokenLen, objErr)
	}
}

func TestSimilaritySearchThresholdAndCustomID(t *testing.T) {
	p := scriptling.New()
	Register(p)

	_, err := p.Eval(`
import scriptling.similarity as sim

items = [
    {"id": 10, "name": "Alpha Project"},
    {"id": 20, "name": "Beta Project"},
]

results = sim.search("alpha", items, max_results=0, threshold=2.0)
first_id = results[0]["id"]
result_count = len(results)
`)
	if err != nil {
		t.Fatalf("failed to evaluate search threshold/id test: %v", err)
	}

	firstIDObj, objErr := p.GetVarAsObject("first_id")
	if objErr != nil {
		t.Fatalf("failed to get first_id object: %v", objErr)
	}
	firstID, numErr := firstIDObj.AsInt()
	if numErr != nil || firstID != 10 {
		t.Fatalf("expected first_id=10, got %v (err=%v)", firstIDObj, numErr)
	}

	resultCount, getErr := p.GetVar("result_count")
	if getErr != nil || resultCount.(int64) < 1 {
		t.Fatalf("expected at least one result, got %v (err=%v)", resultCount, getErr)
	}
}

func TestSimilarityArgumentErrors(t *testing.T) {
	tests := []struct {
		name    string
		script  string
		wantMsg string
	}{
		{
			name: "search_requires_query",
			script: `
import scriptling.similarity as sim
sim.search(items=["a", "b"])
`,
			wantMsg: "query parameter is required",
		},
		{
			name: "search_requires_items",
			script: `
import scriptling.similarity as sim
sim.search("a")
`,
			wantMsg: "items parameter is required",
		},
		{
			name: "minhash_similarity_requires_lists",
			script: `
import scriptling.similarity as sim
sim.minhash_similarity("bad", "data")
`,
			wantMsg: "a must be a list of integers",
		},
		{
			name: "tokenize_requires_string",
			script: `
import scriptling.similarity as sim
sim.tokenize(123)
`,
			wantMsg: "text must be a string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := scriptling.New()
			Register(p)

			_, err := p.Eval(tt.script)
			if err == nil {
				t.Fatal("expected eval error")
			}
			if !strings.Contains(err.Error(), tt.wantMsg) {
				t.Fatalf("expected error containing %q, got %v", tt.wantMsg, err)
			}
		})
	}
}
