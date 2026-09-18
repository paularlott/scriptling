package ai

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	scriptlib "github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/netsecurity"
	"github.com/paularlott/scriptling/object"
)

// chatCompletionHandler answers any /v1/chat/completions POST with a minimal
// valid completion, so tests can assert a request reached the server rather
// than exercising real provider behaviour. When gotTools is non-nil it
// records whether the request carried any "tools" (i.e. whether a remote MCP
// server's tool list was successfully gathered).
func chatCompletionHandler(t *testing.T, gotTools *atomic.Bool) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		if gotTools != nil {
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if tools, ok := body["tools"].([]any); ok && len(tools) > 0 {
				gotTools.Store(true)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      "chatcmpl-test",
			"object":  "chat.completion",
			"created": 1,
			"model":   "test-model",
			"choices": []map[string]any{{
				"index":         0,
				"finish_reason": "stop",
				"message": map[string]any{
					"role":    "assistant",
					"content": "ok",
				},
			}},
		})
	}
}

// mcpToolListHandler answers the "initialize" and "tools/list" JSON-RPC
// methods a remote MCP server needs to hand a single tool to the AI client.
func mcpToolListHandler(t *testing.T) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			JSONRPC string `json:"jsonrpc"`
			ID      any    `json:"id"`
			Method  string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)

		resp := map[string]any{"jsonrpc": "2.0", "id": req.ID}
		switch req.Method {
		case "initialize":
			resp["result"] = map[string]any{}
		case "tools/list":
			resp["result"] = map[string]any{"tools": []map[string]any{{
				"name":        "echo",
				"description": "Echoes input",
				"inputSchema": map[string]any{"type": "object"},
			}}}
		default:
			resp["result"] = map[string]any{}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}

// urlWithHost rewrites a server's URL to address it by a different hostname
// that still resolves to the loopback interface (e.g. "localhost" in place
// of "127.0.0.1"). netsecurity's AllowHosts/DenyHosts match the literal
// hostname text from the URL, not the resolved address, so this lets a test
// tell two loopback endpoints apart by host without needing a second
// bindable loopback IP (e.g. 127.0.0.2, which many sandboxes/CI hosts won't
// let a process bind).
func urlWithHost(t *testing.T, rawURL, host string) string {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse %q: %v", rawURL, err)
	}
	u.Host = host + ":" + u.Port()
	return u.String()
}

func TestAIClientNetworkPolicyBlocksIPLiteralByDefault(t *testing.T) {
	p := scriptlib.New()
	Register(p, &netsecurity.Config{AllowLoopback: true})

	_, err := p.Eval(`
import scriptling.ai as ai

client = ai.Client("http://127.0.0.1:1/v1")
client.completion("test-model", "hello")
`)
	if err == nil || !strings.Contains(err.Error(), "IP literals") {
		t.Errorf("expected IP-literal block, got: %v", err)
	}
}

func TestAIClientNetworkPolicyAllowsLoopback(t *testing.T) {
	server := httptest.NewServer(chatCompletionHandler(t, nil))
	defer server.Close()

	p := scriptlib.New()
	Register(p, &netsecurity.Config{AllowIPLiterals: true, AllowLoopback: true})
	if err := p.SetVar("server_url", server.URL); err != nil {
		t.Fatalf("SetVar(server_url): %v", err)
	}

	result, err := p.Eval(`
import scriptling.ai as ai

client = ai.Client(server_url + "/v1")
response = client.completion("test-model", "hello")
response.choices[0].message.content
`)
	if err != nil {
		t.Fatalf("allowed AI request failed: %v", err)
	}
	str, ok := result.(*object.String)
	if !ok || str.StringValue() != "ok" {
		t.Errorf("response content = %v, want ok", result.Inspect())
	}
}

func TestAIClientNetworkPolicyBlocksDeniedHost(t *testing.T) {
	server := httptest.NewServer(chatCompletionHandler(t, nil))
	defer server.Close()

	p := scriptlib.New()
	Register(p, &netsecurity.Config{
		AllowIPLiterals: true,
		AllowLoopback:   true,
		DenyHosts:       []string{"127.0.0.1"},
	})
	if err := p.SetVar("server_url", server.URL); err != nil {
		t.Fatalf("SetVar(server_url): %v", err)
	}

	_, err := p.Eval(`
import scriptling.ai as ai

client = ai.Client(server_url + "/v1")
client.completion("test-model", "hello")
`)
	if err == nil || !strings.Contains(err.Error(), "denied") {
		t.Errorf("expected denied-host block, got: %v", err)
	}
}

// TestAIClientNetworkPolicyGuardsRemoteMCPServers verifies that
// remote_servers — a second, independent path to an outbound HTTP client
// (openai.RemoteServerConfig.HTTPPool) — is guarded too, not just the main
// completion request. The AI client swallows remote tool-listing errors
// internally (it degrades to "no tools" rather than failing the whole
// completion), so the guard's effect is observed indirectly: the main
// provider only receives a non-empty "tools" array when the remote MCP
// server was actually reachable under the policy.
func TestAIClientNetworkPolicyGuardsRemoteMCPServers(t *testing.T) {
	remoteSrv := httptest.NewServer(mcpToolListHandler(t))
	defer remoteSrv.Close()
	// Address the remote server as "localhost" rather than "127.0.0.1" so
	// DenyHosts can single it out by hostname, even though both endpoints
	// dial the same loopback interface — no second bindable loopback IP
	// (e.g. 127.0.0.2) required.
	remoteURL := urlWithHost(t, remoteSrv.URL, "localhost")

	run := func(policy *netsecurity.Config) bool {
		var gotTools atomic.Bool
		mainSrv := httptest.NewServer(chatCompletionHandler(t, &gotTools))
		defer mainSrv.Close()

		p := scriptlib.New()
		Register(p, policy)
		if err := p.SetVar("server_url", mainSrv.URL); err != nil {
			t.Fatalf("SetVar(server_url): %v", err)
		}
		if err := p.SetVar("remote_url", remoteURL); err != nil {
			t.Fatalf("SetVar(remote_url): %v", err)
		}

		_, err := p.Eval(`
import scriptling.ai as ai

client = ai.Client(server_url + "/v1", remote_servers=[
    {"base_url": remote_url, "namespace": "t"},
])
client.completion("test-model", "hello")
`)
		if err != nil {
			t.Fatalf("completion failed: %v", err)
		}
		return gotTools.Load()
	}

	if !run(&netsecurity.Config{AllowIPLiterals: true, AllowLoopback: true}) {
		t.Error("expected tools from the remote MCP server when it is allowed by policy")
	}
	if run(&netsecurity.Config{AllowIPLiterals: true, AllowLoopback: true, DenyHosts: []string{"localhost"}}) {
		t.Error("expected the remote MCP server to be blocked (denied host), but its tools were used")
	}
}

func TestAIClientNoPolicyIsUnrestricted(t *testing.T) {
	server := httptest.NewServer(chatCompletionHandler(t, nil))
	defer server.Close()

	p := scriptlib.New()
	Register(p) // no cfg at all — previous, unrestricted behaviour
	if err := p.SetVar("server_url", server.URL); err != nil {
		t.Fatalf("SetVar(server_url): %v", err)
	}

	result, err := p.Eval(`
import scriptling.ai as ai

client = ai.Client(server_url + "/v1")
response = client.completion("test-model", "hello")
response.choices[0].message.content
`)
	if err != nil {
		t.Fatalf("unrestricted AI request failed: %v", err)
	}
	str, ok := result.(*object.String)
	if !ok || str.StringValue() != "ok" {
		t.Errorf("response content = %v, want ok", result.Inspect())
	}
}
