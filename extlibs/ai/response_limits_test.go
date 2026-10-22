package ai

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	scriptlib "github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/object"
)

// bigModelServer is an OpenAI-compatible endpoint whose answer size the test
// chooses: a JSON completion of size bytes, or a stream of chunks chunks of
// size bytes each.
func bigModelServer(t *testing.T, size, chunks int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Stream bool `json:"stream"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Stream {
			w.Header().Set("Content-Type", "text/event-stream")
			piece := strings.Repeat("A", size)
			for i := 0; i < chunks; i++ {
				fmt.Fprintf(w, `data: {"id":"c","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"%s"}}]}`+"\n\n", piece)
			}
			fmt.Fprint(w, `data: {"id":"c","object":"chat.completion.chunk","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`+"\n\n")
			fmt.Fprint(w, "data: [DONE]\n\n")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "x", "object": "chat.completion", "model": "m",
			"choices": []any{map[string]any{"index": 0, "finish_reason": "stop",
				"message": map[string]any{"role": "assistant", "content": strings.Repeat("A", size)}}},
		})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func limitedAIScript(t *testing.T, url string, opts ...scriptlib.Option) *scriptlib.Scriptling {
	t.Helper()
	p := scriptlib.New(opts...)
	Register(p)
	if err := p.SetVar("url", url); err != nil {
		t.Fatal(err)
	}
	return p
}

const mib = 1 << 20

// A model response was read whole in Go before it became a script value, so
// the interpreter's value limit never saw it: a 1 MiB limit accepted a 64 MiB
// answer (CWE-770, strix 4250 vuln-0008). The read is now held to the limit
// and fails with a MemoryError; an answer under the limit is unchanged.
func TestAIResponseHeldToValueLimit(t *testing.T) {
	big := bigModelServer(t, 4*mib, 0)
	p := limitedAIScript(t, big.URL, scriptlib.WithMaxValueBytes(mib))
	result, err := p.Eval(`
import scriptling.ai as ai
c = ai.Client(url, max_retries=0)
try:
    r = c.completion("m", "hi")
    out = "accepted " + str(len(r.choices[0].message.content))
except MemoryError as e:
    out = "MemoryError: " + str(e)
out`)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Inspect(); !strings.Contains(got, "MemoryError") || !strings.Contains(got, "AI response too large") {
		t.Fatalf("got %s", got)
	}

	small := bigModelServer(t, 100_000, 0)
	p = limitedAIScript(t, small.URL, scriptlib.WithMaxValueBytes(mib))
	result, err = p.Eval(`
import scriptling.ai as ai
len(ai.Client(url).completion("m", "hi").choices[0].message.content)`)
	if err != nil || result.Inspect() != "100000" {
		t.Fatalf("under the limit: %v %v", result, err)
	}
}

// The memory limit applies too: the body is charged to the interpreter's
// memory headroom while it is read.
func TestAIResponseHeldToMemoryLimit(t *testing.T) {
	big := bigModelServer(t, 64*mib, 0)
	p := limitedAIScript(t, big.URL, scriptlib.WithMemoryLimit(16*mib))
	_, err := p.Eval(`
import scriptling.ai as ai
ai.Client(url, max_retries=0).completion("m", "hi")`)
	if err == nil || !strings.Contains(err.Error(), "AI response too large") {
		t.Fatalf("expected the response to be refused, got %v", err)
	}
}

// A stream is held to the limits per line, not in total: a long answer
// streamed in small pieces is read chunk by chunk, so it passes. Collecting
// it into one value with collect_stream is held to the value limit.
func TestAIStreamLimits(t *testing.T) {
	srv := bigModelServer(t, 16*1024, 200) // 3.2 MB in 16 KB chunks
	p := limitedAIScript(t, srv.URL, scriptlib.WithMaxValueBytes(mib))
	result, err := p.Eval(`
import scriptling.ai as ai
c = ai.Client(url)
s = c.completion_stream("m", "hi")
n = 0
while True:
    ch = s.next()
    if ch == None:
        break
    n = n + 1
[n >= 200, s.err()]`)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Inspect(); got != "[True, None]" {
		t.Fatalf("chunked stream under the per-line limit: got %s", got)
	}

	_, err = p.Eval(`
s = c.completion_stream("m", "hi")
ai.collect_stream(s)`)
	if err == nil || !strings.Contains(err.Error(), "streamed AI response") {
		t.Fatalf("collect_stream past the value limit: expected a MemoryError, got %v", err)
	}

	// One stream line past the limit is refused.
	huge := bigModelServer(t, 4*mib, 1)
	p = limitedAIScript(t, huge.URL, scriptlib.WithMaxValueBytes(mib))
	result, err = p.Eval(`
import scriptling.ai as ai
s = ai.Client(url).completion_stream("m", "hi")
while s.next() != None:
    pass
s.err()`)
	refused := (err != nil && strings.Contains(err.Error(), "AI response too large")) ||
		(err == nil && strings.Contains(result.Inspect(), "AI response too large"))
	if !refused {
		t.Fatalf("huge stream line: expected it to be refused, got %v %v", result, err)
	}
}

// Without limits nothing is wrapped and nothing is refused, as before.
func TestAIResponseUnlimited(t *testing.T) {
	if httpPoolFor(nil, object.NewResourceGuard()) != nil {
		t.Fatal("with no limits and no policy the providers' default pool must be used")
	}
	big := bigModelServer(t, 8*mib, 0)
	p := limitedAIScript(t, big.URL)
	result, err := p.Eval(`
import scriptling.ai as ai
len(ai.Client(url).completion("m", "hi").choices[0].message.content)`)
	if err != nil || result.Inspect() != fmt.Sprint(8*mib) {
		t.Fatalf("unlimited: %v %v", result, err)
	}
}

// A provider error carries its operation prefix once: the OpenAI client
// already starts its errors with the words scriptling adds, which doubled
// them ("chat completion failed: chat completion failed: ..."); providers
// that do not prefix still get it.
func TestProviderErrorPrefixOnce(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"message":"nope"}}`, http.StatusBadRequest)
	}))
	defer srv.Close()

	for provider, calls := range map[string][]string{
		"ai.OPENAI": {`c.completion("m", "hi")`, `c.models()`, `c.embedding("m", "hi")`},
		"ai.CLAUDE": {`c.completion("m", "hi")`},
	} {
		for _, call := range calls {
			p := limitedAIScript(t, srv.URL)
			result, err := p.Eval(`
import scriptling.ai as ai
c = ai.Client(url, provider=` + provider + `, api_key="k", max_retries=0)
try:
    ` + call + `
    out = "no error"
except Exception as e:
    out = str(e)
out`)
			if err != nil {
				t.Fatalf("%s %s: %v", provider, call, err)
			}
			msg := result.Inspect()
			for _, prefix := range []string{"chat completion failed: ", "failed to get models: ", "failed to create embedding: "} {
				if strings.Count(msg, prefix) > 1 {
					t.Errorf("%s %s: prefix repeated: %s", provider, call, msg)
				}
			}
			if strings.Contains(call, "completion") && !strings.Contains(msg, "chat completion failed: ") {
				t.Errorf("%s %s: missing prefix: %s", provider, call, msg)
			}
		}
	}
}
