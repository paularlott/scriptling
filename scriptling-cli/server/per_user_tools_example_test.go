package server

// Tests that drive the examples/mcp-per-user-tools app: the same setup.py,
// auth.py and usertools.py a user would run, served here against an in-process
// test server. The matrix: no token -> 401; alice -> greet + alpha_tool; bob
// -> greet + beta_tool + gamma_tool + his resource and prompt; carol -> greet
// only. A second server configuration drops the static tools dir entirely, so
// every tool comes from the middleware per user.

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// exampleDir is the mcp-per-user-tools example, relative to the repo root.
var exampleDir = filepath.Join("..", "..", "examples", "mcp-per-user-tools")

// servePerUserExample starts the example app. toolsDir overrides the static
// tools directory; an existing-but-empty directory serves MCP with no static
// tools, so every entry comes from the middleware.
func servePerUserExample(t *testing.T, toolsDir string) *httptest.Server {
	t.Helper()

	script := filepath.Join(exampleDir, "setup.py")
	if _, err := os.Stat(script); err != nil {
		t.Fatalf("example setup script: %v", err)
	}

	s, err := NewServer(ServerConfig{
		ScriptFile:  script,
		LibDirs:     []string{exampleDir},
		MCPToolsDir: toolsDir,
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	t.Cleanup(func() { signalShutdown(t, s) })

	ts := httptest.NewServer(s.buildMux())
	t.Cleanup(ts.Close)
	return ts
}

func perUserExampleToolsDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(exampleDir, "tools")
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("example tools dir: %v", err)
	}
	return dir
}

// requireToolSet asserts the caller's tools/list matches want exactly.
func requireToolSet(t *testing.T, ts *httptest.Server, auth string, want map[string]bool, label string) {
	t.Helper()
	names := listToolNames(t, ts, auth)
	if len(names) != len(want) {
		t.Fatalf("%s: tools = %v, want exactly %v", label, names, want)
	}
	for name := range want {
		if !names[name] {
			t.Fatalf("%s: tools = %v, want %q present", label, names, name)
		}
	}
}

func callExampleTool(t *testing.T, ts *httptest.Server, auth, tool string, arguments string) (string, map[string]any) {
	t.Helper()
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"` + tool + `","arguments":` + arguments + `}}`
	status, parsed := mcpPost(t, ts, body, auth)
	if status != http.StatusOK {
		t.Fatalf("tools/call %s: %d %#v", tool, status, parsed)
	}
	result, _ := parsed["result"].(map[string]any)
	content, _ := result["content"].([]any)
	if len(content) < 1 {
		return "", parsed
	}
	first, _ := content[0].(map[string]any)
	text, _ := first["text"].(string)
	return text, parsed
}

// Alice: base tool plus her own; she can call hers, not bob's.
func TestPerUserToolsExampleAlice(t *testing.T) {
	ts := servePerUserExample(t, perUserExampleToolsDir(t))

	// No token: the middleware rejects the request outright.
	if status, _ := mcpPost(t, ts, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`, ""); status != http.StatusUnauthorized {
		t.Fatalf("no token: expected 401, got %d", status)
	}

	requireToolSet(t, ts, "Bearer alice-key",
		map[string]bool{"greet": true, "alpha_tool": true}, "alice")

	text, _ := callExampleTool(t, ts, "Bearer alice-key", "alpha_tool", `{"note":"hello"}`)
	for _, want := range []string{`"user":"alice"`, `"note":"hello"`, `"transport":"http"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("alpha_tool = %s, want it to contain %s", text, want)
		}
	}

	// The static tool works for her too.
	if text, _ := callExampleTool(t, ts, "Bearer alice-key", "greet", `{"name":"alice"}`); text != "hello alice" {
		t.Fatalf("greet = %q, want hello alice", text)
	}

	// Bob's tools are unknown to her.
	status, body := mcpPost(t, ts,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"beta_tool","arguments":{"x":"hi"}}}`,
		"Bearer alice-key")
	if status != http.StatusOK {
		t.Fatalf("alice calling beta_tool: %d", status)
	}
	if obj, ok := body["error"].(map[string]any); !ok || !strings.Contains(obj["message"].(string), "unknown tool") {
		t.Fatalf("alice calling beta_tool = %#v, want unknown tool", body)
	}
}

// Bob: base tool plus his two tools, his template resource and his prompt.
func TestPerUserToolsExampleBob(t *testing.T) {
	ts := servePerUserExample(t, perUserExampleToolsDir(t))

	requireToolSet(t, ts, "Bearer bob-key",
		map[string]bool{"greet": true, "beta_tool": true, "gamma_tool": true}, "bob")

	if text, _ := callExampleTool(t, ts, "Bearer bob-key", "beta_tool", `{"x":"hi"}`); text != "beta:hi" {
		t.Fatalf("beta_tool = %q, want beta:hi", text)
	}
	// A no-parameter request tool calls cleanly.
	if text, _ := callExampleTool(t, ts, "Bearer bob-key", "gamma_tool", `{}`); text != "gamma has no parameters" {
		t.Fatalf("gamma_tool = %q", text)
	}

	// His template resource: {topic} extracted, context visible.
	status, body := mcpPost(t, ts,
		`{"jsonrpc":"2.0","id":3,"method":"resources/read","params":{"uri":"user://bob/notes/todo"}}`,
		"Bearer bob-key")
	if status != http.StatusOK {
		t.Fatalf("resources/read: %d", status)
	}
	result, _ := body["result"].(map[string]any)
	contents, _ := result["contents"].([]any)
	first, _ := contents[0].(map[string]any)
	if first["text"] != "note:todo:bob" {
		t.Fatalf("bob's note = %#v, want note:todo:bob", first["text"])
	}

	// His prompt renders with the argument.
	status, body = mcpPost(t, ts,
		`{"jsonrpc":"2.0","id":4,"method":"prompts/get","params":{"name":"bob_report","arguments":{"subject":"kv stores"}}}`,
		"Bearer bob-key")
	if status != http.StatusOK {
		t.Fatalf("prompts/get: %d", status)
	}
	result, _ = body["result"].(map[string]any)
	messages, _ := result["messages"].([]any)
	if len(messages) != 2 {
		t.Fatalf("bob_report messages = %#v, want 2", result)
	}

	// Alice can neither read his resource nor get his prompt.
	status, body = mcpPost(t, ts,
		`{"jsonrpc":"2.0","id":5,"method":"resources/read","params":{"uri":"user://bob/notes/todo"}}`,
		"Bearer alice-key")
	if status != http.StatusOK {
		t.Fatalf("alice reading bob's resource: %d", status)
	}
	if _, ok := body["error"].(map[string]any); !ok {
		t.Fatalf("alice reading bob's resource = %#v, want an error", body)
	}
	status, body = mcpPost(t, ts,
		`{"jsonrpc":"2.0","id":6,"method":"prompts/get","params":{"name":"bob_report","arguments":{"subject":"x"}}}`,
		"Bearer alice-key")
	if status != http.StatusOK {
		t.Fatalf("alice getting bob's prompt: %d", status)
	}
	if _, ok := body["error"].(map[string]any); !ok {
		t.Fatalf("alice getting bob's prompt = %#v, want an error", body)
	}
}

// Carol: authenticated, but only the static tool — including when there are
// no static tools at all, her tool set is empty.
func TestPerUserToolsExampleCarol(t *testing.T) {
	ts := servePerUserExample(t, perUserExampleToolsDir(t))
	requireToolSet(t, ts, "Bearer carol-key", map[string]bool{"greet": true}, "carol")
}

// No static tools: every tool comes from the middleware. An empty (but
// existing) tools directory enables MCP with nothing registered statically.
func TestPerUserToolsExampleNoStaticTools(t *testing.T) {
	ts := servePerUserExample(t, t.TempDir()) // empty dir: MCP on, zero static tools

	if status, _ := mcpPost(t, ts, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`, ""); status != http.StatusUnauthorized {
		t.Fatalf("no token: expected 401, got %d", status)
	}

	requireToolSet(t, ts, "Bearer alice-key", map[string]bool{"alpha_tool": true}, "alice (no static)")
	requireToolSet(t, ts, "Bearer bob-key", map[string]bool{"beta_tool": true, "gamma_tool": true}, "bob (no static)")

	// carol sees an empty tool set, and the static tool nobody registered is
	// gone for everyone.
	requireToolSet(t, ts, "Bearer carol-key", map[string]bool{}, "carol (no static)")
	status, body := mcpPost(t, ts,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"greet","arguments":{"name":"x"}}}`,
		"Bearer alice-key")
	if status != http.StatusOK {
		t.Fatalf("greet with no static tools: %d", status)
	}
	if obj, ok := body["error"].(map[string]any); !ok || !strings.Contains(obj["message"].(string), "unknown tool") {
		t.Fatalf("greet with no static tools = %#v, want unknown tool", body)
	}

	// The per-user tools still work exactly as before.
	if text, _ := callExampleTool(t, ts, "Bearer alice-key", "alpha_tool", `{"note":"n"}`); !strings.Contains(text, `"user":"alice"`) {
		t.Fatalf("alpha_tool = %q, want alice", text)
	}
}
