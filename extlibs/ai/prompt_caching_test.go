package ai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	scriptlib "github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/stdlib"
)

// claudeBodyCapture records the raw request bodies a stub Claude endpoint
// receives, answering each with a minimal valid Messages response.
type claudeBodyCapture struct {
	mu     sync.Mutex
	bodies []string
}

func (c *claudeBodyCapture) handler(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	c.mu.Lock()
	c.bodies = append(c.bodies, string(body))
	c.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"id":"msg_t","type":"message","role":"assistant","model":"claude-test","content":[{"type":"text","text":"ok"}],"stop_reason":"end_turn","usage":{"input_tokens":5,"output_tokens":1}}`))
}

func (c *claudeBodyCapture) last(t *testing.T) string {
	t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.bodies) == 0 {
		t.Fatal("no request captured")
	}
	return c.bodies[len(c.bodies)-1]
}

// runClaudeCompletion evaluates a scriptling ai.Client constructor + one
// completion against the stub endpoint, returning the client instance.
func runClaudeCompletion(t *testing.T, url, kwargsSrc string) object.Object {
	t.Helper()
	script := `
import scriptling.ai as ai
client = ai.Client("` + url + `", provider="claude", api_key="test-key"` + kwargsSrc + `)
client.completion("claude-test", [{"role": "user", "content": "hi"}])
`
	p := scriptlib.New()
	stdlib.RegisterAll(p)
	Register(p)
	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	return result
}

// prompt_caching=False reaches the Claude client: no cache_control markers
// on the outbound request. Default (omitted) keeps the breakpoints.
func TestClientPromptCachingKwargReachesClaude(t *testing.T) {
	capture := &claudeBodyCapture{}
	ts := httptest.NewServer(http.HandlerFunc(capture.handler))
	defer ts.Close()

	// Default: breakpoints present (system prompt block form).
	runClaudeCompletion(t, ts.URL, "")
	if body := capture.last(t); !strings.Contains(body, "cache_control") {
		t.Fatalf("default should send cache_control breakpoints: %s", body)
	}

	// Disabled: the marker must be absent everywhere.
	runClaudeCompletion(t, ts.URL, ", prompt_caching=False")
	if body := capture.last(t); strings.Contains(body, "cache_control") {
		t.Fatalf("prompt_caching=False must strip cache_control: %s", body)
	}

	// Explicit True behaves like the default.
	runClaudeCompletion(t, ts.URL, ", prompt_caching=True")
	if body := capture.last(t); !strings.Contains(body, "cache_control") {
		t.Fatalf("prompt_caching=True should send breakpoints: %s", body)
	}
}

// A non-boolean prompt_caching is an argument error, not a silent default.
func TestClientPromptCachingKwargTypeChecked(t *testing.T) {
	capture := &claudeBodyCapture{}
	ts := httptest.NewServer(http.HandlerFunc(capture.handler))
	defer ts.Close()

	script := `
import scriptling.ai as ai
client = ai.Client("` + ts.URL + `", provider="claude", api_key="test-key", prompt_caching="yes")
`
	p := scriptlib.New()
	stdlib.RegisterAll(p)
	Register(p)
	_, err := p.Eval(script)
	if err == nil {
		t.Fatal("non-boolean prompt_caching should error")
	}
	if !strings.Contains(err.Error(), "prompt_caching") {
		t.Fatalf("error should name the kwarg: %v", err)
	}
}
