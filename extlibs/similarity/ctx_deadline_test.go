package similarity

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/object"
)

const (
	deadlineTestTimeout = 300 * time.Millisecond
	deadlineTestSlack   = 1500 * time.Millisecond
)

// expectDeadline runs script under deadlineTestTimeout and fails unless it
// stops with a timeout error soon after the deadline.
func expectDeadline(t *testing.T, script string) {
	t.Helper()
	p := scriptling.New()
	Register(p)
	ctx, cancel := context.WithTimeout(context.Background(), deadlineTestTimeout)
	defer cancel()
	start := time.Now()
	result, err := p.EvalWithContext(ctx, script)
	elapsed := time.Since(start)
	msg := ""
	if err != nil {
		msg = err.Error()
	} else if e, ok := result.(*object.Error); ok {
		msg = e.Message
	} else {
		t.Fatalf("expected a timeout, script finished in %v with a %s", elapsed, result.Type())
	}
	if !strings.Contains(strings.ToLower(msg), "timeout") {
		t.Fatalf("expected a timeout error, got %q after %v", msg, elapsed)
	}
	if elapsed > deadlineTestTimeout+deadlineTestSlack {
		t.Fatalf("call ran %v past a %v deadline", elapsed-deadlineTestTimeout, deadlineTestTimeout)
	}
}

func TestScoreHonoursDeadline(t *testing.T) {
	expectDeadline(t, "import scriptling.similarity as sim\nsim.score('ab'*100000, 'ba'*100000)\n")
}

func TestSearchHonoursDeadline(t *testing.T) {
	expectDeadline(t, "import scriptling.similarity as sim\nsim.search('ab'*100000, ['ba'*100000]*10, threshold=0.0)\n")
}

func TestBestHonoursDeadline(t *testing.T) {
	expectDeadline(t, "import scriptling.similarity as sim\nsim.best('ab'*100000, ['ba'*100000]*10)\n")
}

func TestExtractHonoursDeadline(t *testing.T) {
	expectDeadline(t, `import scriptling.similarity as sim
words = ["alpha", "bravo", "charlie", "delta", "echo", "foxtrot", "golf", "hotel", "india", "juliet", "kilo", "lima", "mike", "november", "oscar", "papa", "quebec", "romeo", "sierra", "tango", "uniform", "victor", "whiskey", "xray", "yankee", "zulu"]
def sentence(i):
    return "The " + words[i % 26] + " " + words[(i // 26) % 26] + " " + words[(i // 676) % 26] + " " + words[(i * 7) % 26] + " report"
text = ". ".join([sentence(i) for i in range(15000)])
sim.extract(text, ratio=0.1)
`)
}

func TestMinhashHonoursDeadline(t *testing.T) {
	expectDeadline(t, "import scriptling.similarity as sim\nsim.minhash(' '.join(['word' + str(i) for i in range(50000)]), num_hashes=1000000)\n")
}

// The fuzzy functions give the same answers as github.com/paularlott/cli/fuzzy.
func TestFuzzyMatchesClifuzzy(t *testing.T) {
	cases := [][2]string{{"hello world", "hallo world"}, {"Kitten", "sitting"}, {"abc", "ABC"}, {"a", ""}, {"flaw", "lawn"}}
	for _, c := range cases {
		got, errObj := fuzzyScore(c[0], c[1], nil)
		if errObj != nil {
			t.Fatal(errObj)
		}
		if want := clifuzzyScore(c[0], c[1]); got != want {
			t.Errorf("score(%q, %q) = %v, want %v", c[0], c[1], got, want)
		}
	}
}

// TextRank's similarity matrix is refused for the memory it would take
// rather than built: 15000 distinct sentences need 1.8 GB.
func TestExtractMatrixRespectsMemoryLimit(t *testing.T) {
	p := scriptling.New(scriptling.WithMemoryLimit(64 << 20))
	Register(p)
	result, err := p.Eval(`import scriptling.similarity as sim
words = ["alpha", "bravo", "charlie", "delta", "echo", "foxtrot", "golf", "hotel", "india", "juliet", "kilo", "lima", "mike", "november", "oscar", "papa", "quebec", "romeo", "sierra", "tango", "uniform", "victor", "whiskey", "xray", "yankee", "zulu"]
def sentence(i):
    return "The " + words[i % 26] + " " + words[(i // 26) % 26] + " " + words[(i // 676) % 26] + " " + words[(i * 7) % 26] + " report"
text = ". ".join([sentence(i) for i in range(15000)])
try:
    sim.extract(text, ratio=0.1)
    r = "built"
except MemoryError:
    r = "MemoryError"
r
`)
	if err != nil {
		t.Fatal(err)
	}
	if s, _ := result.AsString(); s != "MemoryError" {
		t.Fatalf("got %v, want MemoryError", result.Inspect())
	}
}

// Sizes a script picks for the vectors it asks for are checked before they
// are allocated.
func TestVectorSizesRespectValueLimit(t *testing.T) {
	p := scriptling.New(scriptling.WithMaxValueBytes(1 << 20))
	Register(p)
	for _, call := range []string{"sim.vectorize('a b c', dims=10**9)", "sim.minhash('alpha bravo', num_hashes=10**9)"} {
		result, err := p.Eval("import scriptling.similarity as sim\ntry:\n    " + call + "\n    r = 'built'\nexcept MemoryError:\n    r = 'MemoryError'\nr\n")
		if err != nil {
			t.Fatal(err)
		}
		if s, _ := result.AsString(); s != "MemoryError" {
			t.Fatalf("%s: got %v, want MemoryError", call, result.Inspect())
		}
	}
}
