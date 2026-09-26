package mcp_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/paularlott/scriptling"
	ai "github.com/paularlott/scriptling/extlibs/ai"
	"github.com/paularlott/scriptling/extlibs/mcp"
	"github.com/paularlott/scriptling/extlibs/netsecurity"
)

// slowServer sleeps before answering, so a short client timeout fires while
// a longer one succeeds.
func slowServer(sleep time.Duration) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(sleep)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2024-11-05","capabilities":{},"serverInfo":{"name":"slow","version":"1.0"}}}`))
	}))
}

// TestClientTimeoutFires: a client with timeout shorter than the server's
// response time fails with a catchable timeout error.
func TestClientTimeoutFires(t *testing.T) {
	ts := slowServer(600 * time.Millisecond)
	defer ts.Close()

	p := scriptling.New()
	mcp.Register(p)
	result, err := p.Eval(`
import scriptling.mcp as mcp
c = mcp.Client("` + ts.URL + `", namespace="slow", timeout=0.2)
msg = "no error"
try:
    c.tools()
except Exception as e:
    msg = str(e)
msg
`)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	got, _ := result.AsString()
	if !strings.Contains(strings.ToLower(got), "timeout") {
		t.Fatalf("expected a timeout error, got: %q", got)
	}
}

// TestClientTimeoutHighEnoughSucceeds: the same slow server with a generous
// timeout completes the initialize round trip.
func TestClientTimeoutHighEnoughSucceeds(t *testing.T) {
	ts := slowServer(200 * time.Millisecond)
	defer ts.Close()

	p := scriptling.New()
	mcp.Register(p)
	result, err := p.Eval(`
import scriptling.mcp as mcp
c = mcp.Client("` + ts.URL + `", namespace="slow", timeout=5)
str(len(c.tools()))
`)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if got, _ := result.AsString(); got != "0" {
		t.Fatalf("expected 0 tools from the stub server, got: %q", got)
	}
}

// TestClientTimeoutValidation: non-positive or non-numeric timeout values
// are rejected at construction, and timeout is HTTP-only.
func TestClientTimeoutValidation(t *testing.T) {
	p := scriptling.New()
	mcp.Register(p)

	for _, script := range []string{
		`mcp.Client("https://example.com/mcp", timeout=0)`,
		`mcp.Client("https://example.com/mcp", timeout=-1)`,
		`mcp.Client("https://example.com/mcp", timeout="30")`,
	} {
		if _, err := p.Eval("import scriptling.mcp as mcp\n" + script); err == nil || !strings.Contains(err.Error(), "timeout") {
			t.Fatalf("expected a timeout validation error for %q, got: %v", script, err)
		}
	}

	if _, err := p.Eval(`import scriptling.mcp as mcp
mcp.Client("/nonexistent/binary-xyz", timeout=30)
`); err == nil || !strings.Contains(err.Error(), "only valid for HTTP servers") {
		t.Fatalf("expected timeout to be rejected for stdio, got: %v", err)
	}
}

// TestClientTimeoutDoesNotAffectAIClient proves the mcp.Client timeout is
// scoped to that client alone: in the same interpreter, an mcp.Client whose
// timeout fires is created first, and an ai.Client completion against an
// endpoint slower than that timeout still succeeds. The timeout lives on a
// per-client copy of the shared pooled http.Client; the AI completions path
// uses the untouched shared client, so long (or streaming) LLM responses are
// never cut off by an MCP timeout.
func TestClientTimeoutDoesNotAffectAIClient(t *testing.T) {
	// Slow LLM endpoint: answers after 600ms, i.e. well past the 300ms MCP
	// timeout used below.
	llm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(600 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"slow llm ok"}}]}`)
	}))
	defer llm.Close()

	// Slow MCP endpoint: never answers inside the timeout.
	mcpSrv := slowServer(2 * time.Second)
	defer mcpSrv.Close()

	p := scriptling.New()
	mcp.Register(p)
	ai.Register(p)
	result, err := p.Eval(`
import scriptling.ai as ai
import scriptling.mcp as mcp

# The MCP client has a 0.3s timeout...
mc = mcp.Client("` + mcpSrv.URL + `", namespace="slow", timeout=0.3)

# ...and the LLM completion against a 0.6s endpoint still succeeds, because
# the AI client's HTTP path shares nothing with the MCP timeout.
llm = ai.Client("` + llm.URL + `")
resp = llm.completion("test-model", [{"role": "user", "content": "hi"}])
text = resp["choices"][0]["message"]["content"]
assert text == "slow llm ok", "llm content: " + str(text)

# Meanwhile the MCP client itself does time out, proving the timeout is
# active on exactly that client and nothing else.
mcpErr = "no error"
try:
    mc.tools()
except Exception as e:
    mcpErr = str(e)
assert "timeout" in mcpErr.lower() or "deadline" in mcpErr.lower(), "mcp timeout missing: " + mcpErr

"OK"
`)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if got, _ := result.AsString(); got != "OK" {
		t.Fatalf("got %q", got)
	}
}

// TestClientTimeoutUnderNetworkPolicy covers the guarded branch of the
// timeout wiring: with a network policy attached, the client still honors the
// timeout kwarg (the guard's client is shallow-copied, keeping its policy
// transport while gaining the deadline) and the policy still admits the
// loopback test server.
func TestClientTimeoutUnderNetworkPolicy(t *testing.T) {
	ts := slowServer(600 * time.Millisecond)
	defer ts.Close()

	p := scriptling.New()
	ai.Register(p)
	mcp.Register(p, &netsecurity.Config{AllowLoopback: true, AllowIPLiterals: true})
	result, err := p.Eval(`
import scriptling.ai as ai
import scriptling.mcp as mcp

fast_timeout = mcp.Client("` + ts.URL + `", namespace="slow", timeout=0.2)
msg = "no error"
try:
    fast_timeout.tools()
except Exception as e:
    msg = str(e)
assert "timeout" in msg.lower() or "deadline" in msg.lower(), "guarded timeout must fire: " + msg

slow_timeout = mcp.Client("` + ts.URL + `", namespace="slow2", timeout=5)
str(len(slow_timeout.tools()))
`)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if got, _ := result.AsString(); got != "0" {
		t.Fatalf("generous guarded timeout must complete, got %q", got)
	}
}
