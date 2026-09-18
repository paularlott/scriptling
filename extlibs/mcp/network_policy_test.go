package mcp_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/paularlott/scriptling"
	scriptmcp "github.com/paularlott/scriptling/extlibs/mcp"
	"github.com/paularlott/scriptling/extlibs/netsecurity"
)

// mcpHandler answers the minimal JSON-RPC 2.0 methods a client needs to
// complete Initialize() and ListTools(): "initialize" and "tools/list".
func mcpHandler(t *testing.T) http.HandlerFunc {
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
// hostname text from the URL, not the resolved address, so tests can
// distinguish loopback endpoints by host without a second bindable loopback
// IP (e.g. 127.0.0.2, which many sandboxes/CI hosts won't let a process bind).
func urlWithHost(t *testing.T, rawURL, host string) string {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse %q: %v", rawURL, err)
	}
	u.Host = host + ":" + u.Port()
	return u.String()
}

func TestMCPClientNetworkPolicyBlocksIPLiteralByDefault(t *testing.T) {
	p := scriptling.New()
	scriptmcp.Register(p, &netsecurity.Config{AllowLoopback: true})

	_, err := p.Eval(`
import scriptling.mcp as mcp

client = mcp.Client("http://127.0.0.1:1/")
client.tools()
`)
	if err == nil || !strings.Contains(err.Error(), "IP literals") {
		t.Errorf("expected IP-literal block, got: %v", err)
	}
}

func TestMCPClientNetworkPolicyAllowsLoopback(t *testing.T) {
	server := httptest.NewServer(mcpHandler(t))
	defer server.Close()

	p := scriptling.New()
	scriptmcp.Register(p, &netsecurity.Config{AllowIPLiterals: true, AllowLoopback: true})
	if err := p.SetVar("server_url", server.URL); err != nil {
		t.Fatalf("SetVar(server_url): %v", err)
	}

	result, err := p.Eval(`
import scriptling.mcp as mcp

client = mcp.Client(server_url)
tools = client.tools()
len(tools)
`)
	if err != nil {
		t.Fatalf("allowed mcp request failed: %v", err)
	}
	n, ierr := result.AsInt()
	if ierr != nil || n != 1 {
		t.Errorf("len(tools) = %v, want 1", result.Inspect())
	}
}

func TestMCPClientNetworkPolicyBlocksDeniedHost(t *testing.T) {
	server := httptest.NewServer(mcpHandler(t))
	defer server.Close()
	deniedURL := urlWithHost(t, server.URL, "localhost")

	p := scriptling.New()
	scriptmcp.Register(p, &netsecurity.Config{
		AllowIPLiterals: true,
		AllowLoopback:   true,
		DenyHosts:       []string{"localhost"},
	})
	if err := p.SetVar("server_url", deniedURL); err != nil {
		t.Fatalf("SetVar(server_url): %v", err)
	}

	_, err := p.Eval(`
import scriptling.mcp as mcp

client = mcp.Client(server_url)
client.tools()
`)
	if err == nil || !strings.Contains(err.Error(), "denied") {
		t.Errorf("expected denied-host block, got: %v", err)
	}
}

func TestMCPClientNoPolicyIsUnrestricted(t *testing.T) {
	server := httptest.NewServer(mcpHandler(t))
	defer server.Close()

	p := scriptling.New()
	scriptmcp.Register(p) // no cfg at all — previous, unrestricted behaviour
	if err := p.SetVar("server_url", server.URL); err != nil {
		t.Fatalf("SetVar(server_url): %v", err)
	}

	result, err := p.Eval(`
import scriptling.mcp as mcp

client = mcp.Client(server_url)
tools = client.tools()
len(tools)
`)
	if err != nil {
		t.Fatalf("unrestricted mcp request failed: %v", err)
	}
	n, ierr := result.AsInt()
	if ierr != nil || n != 1 {
		t.Errorf("len(tools) = %v, want 1", result.Inspect())
	}
}
