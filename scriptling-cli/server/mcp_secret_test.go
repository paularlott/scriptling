package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paularlott/logger"
	"github.com/paularlott/scriptling/extlibs/runtime"
	"github.com/paularlott/scriptling/extlibs/secretprovider"
	"github.com/paularlott/scriptling/scriptling-cli/setup"
)

// --mcp-secret: instances sharing it (or one instance after a restart)
// accept each other's MCP sessions and question state; without it each
// process has its own keys.

const testMCPSecret = "a shared secret of at least thirty-two characters"

// mcpInstance serves a tool that asks a question, configured with secret.
func mcpInstance(t *testing.T, secret string) string {
	t.Helper()
	libDir := t.TempDir()
	setup.Factories([]string{libDir}, nil, nil, secretprovider.NewRegistry(), logger.NewNullLogger(), "", "", false)
	runtime.ResetRuntime()
	toolsDir := t.TempDir()
	writeFile(t, filepath.Join(toolsDir, "confirm.toml"), []byte("description = \"Confirms\"\n"))
	writeFile(t, filepath.Join(toolsDir, "confirm.py"), []byte(
		"import scriptling.mcp.tool as tool\ntool.return_object({\"ok\": tool.confirm(\"Go ahead?\")})\n"))
	s := &Server{
		config:     ServerConfig{MCPToolsDir: toolsDir, LibDirs: []string{libDir}, MCPSecret: secret},
		mcpHandler: &reloadableMCPHandler{},
	}
	server, err := s.createMCPServer()
	if err != nil {
		t.Fatalf("createMCPServer: %v", err)
	}
	s.mcpHandler.server.Store(server)
	ts := httptest.NewServer(s.buildMux())
	t.Cleanup(ts.Close)
	return ts.URL + "/mcp"
}

func secretPost(t *testing.T, url string, headers map[string]string, body any) (int, http.Header, map[string]any) {
	t.Helper()
	raw, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	var out map[string]any
	_ = json.Unmarshal(b, &out)
	return resp.StatusCode, resp.Header, out
}

// legacySession initializes a 2025-11-25 session on url and returns its id.
func legacySession(t *testing.T, url string) string {
	t.Helper()
	code, h, _ := secretPost(t, url, map[string]string{"MCP-Protocol-Version": "2025-11-25"}, map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "initialize",
		"params": map[string]any{"protocolVersion": "2025-11-25", "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "t", "version": "1"}},
	})
	id := h.Get("MCP-Session-Id")
	if code != http.StatusOK || id == "" {
		t.Fatalf("initialize: %d, session %q", code, id)
	}
	return id
}

func listWithSession(t *testing.T, url, session string) int {
	t.Helper()
	code, _, _ := secretPost(t, url, map[string]string{"MCP-Protocol-Version": "2025-11-25", "MCP-Session-Id": session},
		map[string]any{"jsonrpc": "2.0", "id": 2, "method": "tools/list", "params": map[string]any{}})
	return code
}

// subscribeWithSession reports whether url treats session as a session:
// resources/subscribe needs one, and without it is an unknown method.
func subscribeWithSession(t *testing.T, url, session string) bool {
	t.Helper()
	_, _, out := secretPost(t, url, map[string]string{"MCP-Protocol-Version": "2025-11-25", "MCP-Session-Id": session},
		map[string]any{"jsonrpc": "2.0", "id": 3, "method": "resources/subscribe", "params": map[string]any{"uri": "test://x"}})
	return out["error"] == nil
}

func TestMCPSecretSharesSessions(t *testing.T) {
	a, b := mcpInstance(t, testMCPSecret), mcpInstance(t, testMCPSecret)
	session := legacySession(t, a)
	if !subscribeWithSession(t, b, session) {
		t.Fatal("instance B does not accept instance A's session")
	}
	for name, other := range map[string]string{"another secret": mcpInstance(t, testMCPSecret[1:]+"x"), "no secret": mcpInstance(t, "")} {
		// Not its session, but still served, as before sessions existed.
		if code := listWithSession(t, other, session); code != http.StatusOK {
			t.Fatalf("%s: a session it can't verify was refused: %d", name, code)
		}
		if subscribeWithSession(t, other, session) {
			t.Fatalf("%s: accepted another instance's session as its own", name)
		}
	}
}

// A 2026-07-28 client's answer may come back to another instance.
func TestMCPSecretSharesQuestionState(t *testing.T) {
	modern := func(url string, params map[string]any) map[string]any {
		params["_meta"] = map[string]any{
			"io.modelcontextprotocol/protocolVersion":    "2026-07-28",
			"io.modelcontextprotocol/clientCapabilities": map[string]any{"elicitation": map[string]any{}},
		}
		_, _, out := secretPost(t, url, map[string]string{"MCP-Protocol-Version": "2026-07-28", "Mcp-Method": "tools/call", "Mcp-Name": "confirm"},
			map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": params})
		return out
	}
	ask := func(url string) (string, string) {
		result, _ := modern(url, map[string]any{"name": "confirm", "arguments": map[string]any{}})["result"].(map[string]any)
		requests, _ := result["inputRequests"].(map[string]any)
		if result["resultType"] != "input_required" || len(requests) != 1 {
			t.Fatalf("want a question, got %v", result)
		}
		for key := range requests {
			return key, result["requestState"].(string)
		}
		return "", ""
	}
	answer := func(url, key, state string) map[string]any {
		return modern(url, map[string]any{"name": "confirm", "arguments": map[string]any{}, "requestState": state,
			"inputResponses": map[string]any{key: map[string]any{"action": "accept", "content": map[string]any{"approve": true}}}})
	}

	key, state := ask(mcpInstance(t, testMCPSecret))
	out := answer(mcpInstance(t, testMCPSecret), key, state)
	content, _ := out["result"].(map[string]any)["content"].([]any)
	if len(content) == 0 || !strings.Contains(content[0].(map[string]any)["text"].(string), `"ok":true`) {
		t.Fatalf("answer on instance B: %v", out)
	}

	key, state = ask(mcpInstance(t, ""))
	if out := answer(mcpInstance(t, ""), key, state); out["error"] == nil {
		t.Fatalf("instances without a shared secret shared question state: %v", out)
	}
}

func TestMCPSecretTooShort(t *testing.T) {
	const secret = "too-short-secret-value"
	s := &Server{config: ServerConfig{MCPSecret: secret}, mcpHandler: &reloadableMCPHandler{}}
	_, err := s.createMCPServer()
	if err == nil || !strings.Contains(err.Error(), "at least 32") {
		t.Fatalf("err = %v", err)
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatal("the secret appears in the error")
	}
}
