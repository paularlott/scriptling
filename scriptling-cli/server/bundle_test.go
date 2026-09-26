package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paularlott/scriptling/scriptling-cli/pack"
)

// writeAppBundleDir creates a complete app bundle fixture:
// manifest + setup.py (route registration) + lib handler + one MCP tool +
// webroot asset.
func writeAppBundleDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"manifest.toml": `name = "testapp"
version = "1.0.0"
main = "setup.py"
libs = ["lib"]
serve = ["http", "mcp"]
`,
		"setup.py": `import scriptling.runtime as runtime
runtime.http.get("/api/hello", "handlers.hello")
`,
		"lib/handlers.py": `def hello(request):
    return {"status": 200, "headers": {}, "body": "hello from bundle"}
`,
		"tools/double.toml": `description = "Double a number"

[[parameters]]
name = "n"
type = "int"
description = "Number to double"
required = true
`,
		"tools/double.py": `import scriptling.mcp.tool as tool
tool.return_string(str(tool.get_int("n") * 2))
`,
		"webroot/index.html": `<h1>bundle app</h1>`,
		"webroot/app.js":     `console.log("bundle")`,
	}
	for name, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// runAppBundleAssertions runs the full set of HTTP + MCP assertions against a
// server built from the given bundle. Used for both dir and zip backends.
func runAppBundleAssertions(t *testing.T, b *pack.Bundle) {
	t.Helper()

	s, err := NewServer(ServerConfig{Bundle: b})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	ts := httptest.NewServer(s.buildMux())
	defer ts.Close()

	// HTTP route registered by the bundle's main script, handler resolved from
	// the bundle's lib dir.
	resp, err := http.Get(ts.URL + "/api/hello")
	if err != nil {
		t.Fatalf("GET /api/hello: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || string(body) != "hello from bundle" {
		t.Errorf("/api/hello = %d %q", resp.StatusCode, body)
	}

	// webroot asset served at the fallback.
	resp, err = http.Get(ts.URL + "/app.js")
	if err != nil {
		t.Fatalf("GET /app.js: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !strings.Contains(string(body), "console.log") {
		t.Errorf("/app.js = %d %q", resp.StatusCode, body)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "javascript") {
		t.Errorf("/app.js content-type = %q", ct)
	}

	// index.html served at the root.
	resp, err = http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !strings.Contains(string(body), "bundle app") {
		t.Errorf("/ = %d %q", resp.StatusCode, body)
	}

	// Traversal attempts never serve content.
	resp, err = http.Get(ts.URL + "/../manifest.toml")
	if err != nil {
		t.Fatalf("GET traversal: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode == 200 {
		t.Error("traversal attempt returned 200")
	}

	// MCP: the bundle's tool is listed and callable.
	mcpServer := s.mcpHandler.server.Load()
	if mcpServer == nil {
		t.Fatal("MCP server not initialized for bundle with serve=[http,mcp]")
	}
	client, cleanup := pipeClientServer(t, mcpServer)
	defer cleanup()

	ctx := context.Background()
	tools, err := client.ListTools(ctx)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	found := false
	for _, tool := range tools {
		if tool.Name == "double" {
			found = true
		}
	}
	if !found {
		t.Fatalf("double tool not listed: %+v", tools)
	}

	result, err := client.CallTool(ctx, "double", map[string]interface{}{"n": 21})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if len(result.Content) != 1 || result.Content[0].Text != "42" {
		t.Fatalf("double(21) = %+v, want 42", result.Content)
	}
}

func TestAppBundleFromDir(t *testing.T) {
	b, err := pack.OpenBundleDir(writeAppBundleDir(t))
	if err != nil {
		t.Fatalf("OpenBundleDir: %v", err)
	}
	runAppBundleAssertions(t, b)
}

func TestAppBundleFromZip(t *testing.T) {
	dir := writeAppBundleDir(t)
	zipPath := filepath.Join(t.TempDir(), "app.zip")
	if _, _, err := pack.Pack(dir, zipPath, false); err != nil {
		t.Fatalf("Pack: %v", err)
	}
	b, err := pack.FetchBundle(zipPath, false, "")
	if err != nil {
		t.Fatalf("FetchBundle: %v", err)
	}
	runAppBundleAssertions(t, b)
}

// TestAppBundleJSONRPC verifies a bundle declaring serve=["json-rpc"] gets its
// /json-rpc endpoint enabled and methods registered by its main script.
func TestAppBundleJSONRPC(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"manifest.toml": `name = "rpcapp"
version = "1.0.0"
main = "setup.py"
serve = ["json-rpc"]
`,
		"setup.py": `import scriptling.runtime as runtime
runtime.jsonrpc.method("echo", "handlers.echo")
`,
		"lib/handlers.py": `def echo(params):
    return params
`,
	}
	for name, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	b, err := pack.OpenBundleDir(dir)
	if err != nil {
		t.Fatalf("OpenBundleDir: %v", err)
	}
	s, err := NewServer(ServerConfig{Bundle: b})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	ts := httptest.NewServer(s.buildMux())
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/json-rpc", "application/json",
		strings.NewReader(`{"jsonrpc":"2.0","method":"echo","params":{"msg":"hi"},"id":1}`))
	if err != nil {
		t.Fatalf("POST /json-rpc: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(body), `"msg":"hi"`) {
		t.Errorf("/json-rpc echo = %d %s", resp.StatusCode, body)
	}
}

// TestAppBundleDecoratedTool verifies a bundle containing a decorated
// (.py-only, @mcp.tool) tool is discovered, listed, and callable.
func TestAppBundleDecoratedTool(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"manifest.toml": `name = "decapp"
version = "1.0.0"
main = "setup.py"
serve = ["mcp"]
`,
		"setup.py": `# no HTTP routes, just MCP
`,
		"tools/triple.py": `import scriptling.runtime.mcp as mcp

@mcp.tool("Triple a number", params={"n": {"type": "int", "description": "Number to triple"}})
def triple(n):
    return str(n * 3)
`,
	}
	for name, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	b, err := pack.OpenBundleDir(dir)
	if err != nil {
		t.Fatalf("OpenBundleDir: %v", err)
	}
	s, err := NewServer(ServerConfig{Bundle: b})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	mcpServer := s.mcpHandler.server.Load()
	if mcpServer == nil {
		t.Fatal("MCP server not initialized")
	}
	client, cleanup := pipeClientServer(t, mcpServer)
	defer cleanup()

	ctx := context.Background()

	// Tool should be listed
	tools, err := client.ListTools(ctx)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	var found bool
	for _, tool := range tools {
		if tool.Name == "triple" {
			found = true
			if tool.Description != "Triple a number" {
				t.Errorf("description: got %q", tool.Description)
			}
		}
	}
	if !found {
		names := make([]string, len(tools))
		for i, tool := range tools {
			names[i] = tool.Name
		}
		t.Fatalf("triple tool not listed, got: %v", names)
	}

	// Tool should be callable
	result, err := client.CallTool(ctx, "triple", map[string]interface{}{"n": 7})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if len(result.Content) != 1 || result.Content[0].Text != "21" {
		t.Fatalf("triple(7) = %+v, want '21'", result.Content)
	}
}

// TestAppBundleMixedTools verifies a bundle with both legacy (.toml+.py) and
// decorated (.py-only) tools discovers and serves all of them.
func TestAppBundleMixedTools(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"manifest.toml": `name = "mixedapp"
version = "1.0.0"
main = "setup.py"
serve = ["mcp"]
`,
		"setup.py": `pass
`,
		// Legacy tool
		"tools/double.toml": `description = "Double a number"
[[parameters]]
name = "n"
type = "int"
description = "Number to double"
required = true
`,
		"tools/double.py": `import scriptling.mcp.tool as tool
tool.return_string(str(tool.get_int("n") * 2))
`,
		// Decorated tool
		"tools/half.py": `import scriptling.runtime.mcp as mcp

@mcp.tool("Halve a number", params={"n": {"type": "int", "description": "Number to halve"}})
def half(n):
    return str(n // 2)
`,
	}
	for name, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	b, err := pack.OpenBundleDir(dir)
	if err != nil {
		t.Fatalf("OpenBundleDir: %v", err)
	}
	s, err := NewServer(ServerConfig{Bundle: b})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	mcpServer := s.mcpHandler.server.Load()
	if mcpServer == nil {
		t.Fatal("MCP server not initialized")
	}
	client, cleanup := pipeClientServer(t, mcpServer)
	defer cleanup()

	ctx := context.Background()

	tools, err := client.ListTools(ctx)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	toolNames := map[string]bool{}
	for _, tool := range tools {
		toolNames[tool.Name] = true
	}
	if !toolNames["double"] {
		t.Error("missing legacy tool 'double'")
	}
	if !toolNames["half"] {
		t.Error("missing decorated tool 'half'")
	}

	// Call legacy tool
	result, err := client.CallTool(ctx, "double", map[string]interface{}{"n": 5})
	if err != nil {
		t.Fatalf("CallTool double: %v", err)
	}
	if result.Content[0].Text != "10" {
		t.Errorf("double(5) = %q, want '10'", result.Content[0].Text)
	}

	// Call decorated tool
	result, err = client.CallTool(ctx, "half", map[string]interface{}{"n": 10})
	if err != nil {
		t.Fatalf("CallTool half: %v", err)
	}
	if result.Content[0].Text != "5" {
		t.Errorf("half(10) = %q, want '5'", result.Content[0].Text)
	}
}

// TestAppBundleSkillsResourcesPrompts verifies an app bundle carrying a
// skills folder alongside its tools: Pack includes the skills dir, and the
// server registers the skill (and a prompt) from the packaged zip so a
// client can list and read both.
func TestAppBundleSkillsResourcesPrompts(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"manifest.toml": `name = "skillapp"
version = "1.0.0"
main = "setup.py"
serve = ["mcp"]
`,
		"setup.py": `# MCP only
`,
		"tools/doubler.py": `import scriptling.runtime.mcp as mcp

@mcp.tool("Double a number", params={"n": "Number"})
def doubler(n):
    return str(int(n) * 2)

@mcp.resource("config://from-bundle", name="Bundle config", mime_type="application/json")
def bundle_config():
    return {"origin": "bundle"}

@mcp.prompt(description="Cheer helper")
def cheer_hint(who):
    return "Go " + who + "!"
`,
		"prompts/cheer.toml": `description = "Cheer for a team"

[[arguments]]
name = "team"
description = "Who to cheer for"
required = true
`,
		"prompts/cheer.py": `import scriptling.mcp.tool as tool
tool.return_string("Go " + tool.get_string("team") + "!")
`,
		"skills/team-spirit/SKILL.md": "---\nname: team-spirit\ndescription: How to cheer effectively\n---\n\n# Team spirit\nCheer loudly.",
		"skills/team-spirit/chants.md": "Go team!\nWin team!\n",
	}
	for name, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Pack it, then run from the zip: dev-equals-prod means the zip is the
	// thing that ships, so prove the skills dir survives packaging.
	zipPath := filepath.Join(t.TempDir(), "skillapp.zip")
	if _, _, err := pack.Pack(dir, zipPath, false); err != nil {
		t.Fatalf("Pack: %v", err)
	}
 zf, err := os.Open(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer zf.Close()
	st, err := zf.Stat()
	if err != nil {
		t.Fatal(err)
	}
	b, err := pack.OpenBundleZip(zf, st.Size(), zipPath)
	if err != nil {
		t.Fatalf("OpenBundleZip: %v", err)
	}
	s, err := NewServer(ServerConfig{Bundle: b})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	mcpServer := s.mcpHandler.server.Load()
	if mcpServer == nil {
		t.Fatal("MCP server not initialized")
	}
	client, cleanup := pipeClientServer(t, mcpServer)
	defer cleanup()
	ctx := context.Background()

	// Tool from the package
	result, err := client.CallTool(ctx, "doubler", map[string]any{"n": 4})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if len(result.Content) != 1 || result.Content[0].Text != "8" {
		t.Fatalf("doubler(4) = %+v, want '8'", result.Content)
	}

	// Decorated resource and prompt from the same bundle's tools dir
	cfgRes, err := client.ReadResource(ctx, "config://from-bundle")
	if err != nil {
		t.Fatalf("ReadResource decorated-in-bundle: %v", err)
	}
	if !strings.Contains(cfgRes.Contents[0].Text, `"origin":"bundle"`) || cfgRes.Contents[0].MimeType != "application/json" {
		t.Fatalf("decorated resource in bundle: %+v", cfgRes.Contents)
	}
	hint, err := client.GetPrompt(ctx, "cheer_hint", map[string]string{"who": "packaging"})
	if err != nil {
		t.Fatalf("GetPrompt decorated-in-bundle: %v", err)
	}
	if len(hint.Messages) != 1 || hint.Messages[0].Content.Text != "Go packaging!" {
		t.Fatalf("decorated prompt in bundle: %+v", hint.Messages)
	}

	// Prompt with a required argument from the package
	pr, err := client.GetPrompt(ctx, "cheer", map[string]string{"team": "Scriptling"})
	if err != nil {
		t.Fatalf("GetPrompt: %v", err)
	}
	if len(pr.Messages) != 1 || pr.Messages[0].Content.Text != "Go Scriptling!" {
		t.Fatalf("cheer messages: %+v", pr.Messages)
	}
	if _, err := client.GetPrompt(ctx, "cheer", map[string]string{}); err == nil || !strings.Contains(err.Error(), "missing required argument") {
		t.Fatalf("missing required argument must be -32602, got: %v", err)
	}

	// Skill from the package, with its supporting file
	skills, err := client.ListSkills(ctx)
	if err != nil {
		t.Fatalf("ListSkills: %v", err)
	}
	found := false
	for _, sk := range skills {
		if sk.URI == "skill://team-spirit/SKILL.md" {
			found = true
			if sk.Frontmatter["name"] != "team-spirit" {
				t.Errorf("frontmatter: %+v", sk.Frontmatter)
			}
		}
	}
	if !found {
		t.Fatalf("team-spirit not listed: %+v", skills)
	}
	content, err := client.ReadResource(ctx, "skill://team-spirit/chants.md")
	if err != nil {
		t.Fatalf("ReadResource supporting file: %v", err)
	}
	if !strings.Contains(content.Contents[0].Text, "Go team!") {
		t.Fatalf("chants.md content: %q", content.Contents[0].Text)
	}
	if len(s.mcpBundleEntries.skills) != 1 || s.mcpBundleEntries.skills[0] != "team-spirit" {
		t.Fatalf("bundle skills tracking: %v", s.mcpBundleEntries.skills)
	}
}
