package mcp

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paularlott/logger"
	mcplib "github.com/paularlott/mcp"
	"github.com/paularlott/scriptling/extlibs/secretprovider"
)

// writeScript writes data to a path under the test's temp area, creating
// intermediate dirs as needed. Returns the full path.
func writeScript(t *testing.T, path string, data []byte) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestBuildToolHandlerReturnsString(t *testing.T) {
	dir := t.TempDir()
	script := writeScript(t, filepath.Join(dir, "echo.py"),
		[]byte("import scriptling.mcp.tool as tool\ntool.return_string('hello ' + tool.get_string('name'))\n"))

	cfg := NewHandlerConfig(nil, WithSecrets(secretprovider.NewRegistry()), WithLogger(logger.NewNullLogger()))
	handler, err := BuildToolHandler(script, cfg)
	if err != nil {
		t.Fatalf("BuildToolHandler: %v", err)
	}

	resp, err := handler(context.Background(), mcplib.NewToolRequest(map[string]any{"name": "world"}))
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if len(resp.Content) != 1 || resp.Content[0].Text != "hello world" {
		t.Fatalf("expected hello world, got %+v", resp.Content)
	}
}

func TestBuildToolHandlerPropagatesScriptError(t *testing.T) {
	dir := t.TempDir()
	script := writeScript(t, filepath.Join(dir, "boom.py"),
		[]byte("import scriptling.mcp.tool as tool\ntool.return_error('bang')\n"))

	cfg := NewHandlerConfig(nil, WithLogger(logger.NewNullLogger()))
	handler, err := BuildToolHandler(script, cfg)
	if err != nil {
		t.Fatalf("BuildToolHandler: %v", err)
	}

	if _, err := handler(context.Background(), mcplib.NewToolRequest(nil)); err == nil {
		t.Fatalf("expected error from return_error, got nil")
	}
}

func TestBuildToolHandlerMissingFile(t *testing.T) {
	cfg := NewHandlerConfig(nil)
	if _, err := BuildToolHandler(filepath.Join(t.TempDir(), "missing.py"), cfg); err == nil {
		t.Fatal("expected error for missing script file")
	}
}

func TestBuildToolHandlerRespectsLibDirs(t *testing.T) {
	libDir := t.TempDir()
	writeScript(t, filepath.Join(libDir, "helper.py"), []byte("def value():\n    return 'lib'\n"))

	toolDir := t.TempDir()
	script := writeScript(t, filepath.Join(toolDir, "tool.py"),
		[]byte("import helper\nimport scriptling.mcp.tool as tool\ntool.return_string(helper.value())\n"))

	cfg := NewHandlerConfig([]string{libDir}, WithLogger(logger.NewNullLogger()))
	handler, err := BuildToolHandler(script, cfg)
	if err != nil {
		t.Fatalf("BuildToolHandler: %v", err)
	}

	resp, err := handler(context.Background(), mcplib.NewToolRequest(nil))
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if resp.Content[0].Text != "lib" {
		t.Fatalf("expected lib, got %q", resp.Content[0].Text)
	}
}

func TestBuildStaticResourceHandlerServesText(t *testing.T) {
	path := writeScript(t, filepath.Join(t.TempDir(), "file.txt"), []byte("plain text"))
	h := BuildStaticResourceHandler(path, "memo://file", "text/plain")
	resp, err := h(context.Background(), mcplib.NewResourceRequest("memo://file", nil))
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if len(resp.Contents) != 1 || resp.Contents[0].Text != "plain text" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestBuildStaticResourceHandlerServesBinaryAsBlob(t *testing.T) {
	// 0xFF is invalid UTF-8 so the handler takes the blob path.
	raw := []byte{0xFF, 0xFE, 0x00, 0x01}
	path := writeScript(t, filepath.Join(t.TempDir(), "blob.bin"), raw)
	h := BuildStaticResourceHandler(path, "bin://blob", "")
	resp, err := h(context.Background(), mcplib.NewResourceRequest("bin://blob", nil))
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if len(resp.Contents) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(resp.Contents))
	}
	decoded, err := base64.StdEncoding.DecodeString(resp.Contents[0].Blob)
	if err != nil {
		t.Fatalf("decoding base64: %v", err)
	}
	if string(decoded) != string(raw) {
		t.Fatalf("roundtrip mismatch")
	}
}

func TestBuildResourceScriptHandlerRunsScriptWithVars(t *testing.T) {
	dir := t.TempDir()
	script := writeScript(t, filepath.Join(dir, "tpl.py"),
		[]byte("import scriptling.mcp.tool as tool\ntool.return_string('Hi ' + tool.get_string('name') + ' @ ' + tool.get_string('__uri'))\n"))

	cfg := NewHandlerConfig(nil, WithLogger(logger.NewNullLogger()))
	h, err := BuildResourceScriptHandler(script, "text/plain", cfg)
	if err != nil {
		t.Fatalf("BuildResourceScriptHandler: %v", err)
	}

	resp, err := h(context.Background(), mcplib.NewResourceRequest("greeting://Ada", map[string]string{"name": "Ada"}))
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	want := "Hi Ada @ greeting://Ada"
	if len(resp.Contents) != 1 || resp.Contents[0].Text != want {
		t.Fatalf("expected %q, got %+v", want, resp.Contents)
	}
}

func TestBuildStaticPromptHandlerReturnsFileContent(t *testing.T) {
	path := writeScript(t, filepath.Join(t.TempDir(), "p.md"), []byte("Summarise this."))
	h := BuildStaticPromptHandler(path)
	resp, err := h(context.Background(), mcplib.NewPromptRequest(nil))
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if len(resp.Messages) != 1 || !strings.Contains(resp.Messages[0].Content.Text, "Summarise") {
		t.Fatalf("unexpected: %+v", resp)
	}
}

func TestBuildPromptScriptHandlerAcceptsString(t *testing.T) {
	dir := t.TempDir()
	script := writeScript(t, filepath.Join(dir, "p.py"),
		[]byte("import scriptling.mcp.tool as tool\ntool.return_string('review ' + tool.get_string('lang'))\n"))

	cfg := NewHandlerConfig(nil, WithLogger(logger.NewNullLogger()))
	h, err := BuildPromptScriptHandler(script, cfg)
	if err != nil {
		t.Fatalf("BuildPromptScriptHandler: %v", err)
	}
	resp, err := h(context.Background(), mcplib.NewPromptRequest(map[string]string{"lang": "go"}))
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if len(resp.Messages) != 1 || resp.Messages[0].Content.Text != "review go" {
		t.Fatalf("unexpected: %+v", resp)
	}
}

func TestBuildPromptScriptHandlerAcceptsStructuredMessages(t *testing.T) {
	dir := t.TempDir()
	script := writeScript(t, filepath.Join(dir, "p.py"),
		[]byte(`import scriptling.mcp.tool as tool
tool.return_object({"description":"d","messages":[{"role":"user","content":"hi"},{"role":"assistant","content":"yo"}]})
`))

	cfg := NewHandlerConfig(nil, WithLogger(logger.NewNullLogger()))
	h, err := BuildPromptScriptHandler(script, cfg)
	if err != nil {
		t.Fatalf("BuildPromptScriptHandler: %v", err)
	}
	resp, err := h(context.Background(), mcplib.NewPromptRequest(nil))
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if resp.Description != "d" || len(resp.Messages) != 2 || resp.Messages[0].Content.Text != "hi" || resp.Messages[1].Content.Text != "yo" {
		t.Fatalf("unexpected: %+v", resp)
	}
}

func TestDecodePromptScriptResponse(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string // expected message contents
		desc string
	}{
		{"empty", "", []string{}, ""},
		{"plain string", "hello world", []string{"hello world"}, ""},
		{"json object with messages", `{"description":"d","messages":[{"role":"user","content":"a"}]}`, []string{"a"}, "d"},
		{"json array", `[{"role":"user","content":"x"},{"role":"user","content":"y"}]`, []string{"x", "y"}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := DecodePromptScriptResponse(tc.in)
			if r.Description != tc.desc {
				t.Fatalf("description: want %q got %q", tc.desc, r.Description)
			}
			if len(r.Messages) != len(tc.want) {
				t.Fatalf("messages: want %d got %d (%+v)", len(tc.want), len(r.Messages), r.Messages)
			}
			for i, m := range r.Messages {
				if m.Content.Text != tc.want[i] {
					t.Fatalf("msg %d: want %q got %q", i, tc.want[i], m.Content.Text)
				}
			}
		})
	}
}

func TestNewHandlerConfigDefaultsAndOverrides(t *testing.T) {
	c := NewHandlerConfig([]string{"a"})
	if c.LibDirs == nil || len(c.LibDirs) != 1 || c.LibDirs[0] != "a" {
		t.Fatalf("LibDirs default: %+v", c.LibDirs)
	}
	if c.PluginManager != nil || c.PackLoader != nil || c.Logger != nil || c.SecretRegistry != nil {
		t.Fatalf("expected zero values, got %+v", c)
	}

	r := secretprovider.NewRegistry()
	pm := struct{}{} // sentinel — replaced below
	_ = pm
	c = NewHandlerConfig([]string{"x"},
		WithAllowedPaths([]string{"/p"}),
		WithDisabledLibs([]string{"os"}),
		WithSecrets(r),
		WithLogger(logger.NewNullLogger()),
	)
	if len(c.AllowedPaths) != 1 || c.AllowedPaths[0] != "/p" {
		t.Fatalf("AllowedPaths: %+v", c.AllowedPaths)
	}
	if len(c.DisabledLibs) != 1 || c.DisabledLibs[0] != "os" {
		t.Fatalf("DisabledLibs: %+v", c.DisabledLibs)
	}
	if c.SecretRegistry != r {
		t.Fatalf("SecretRegistry not set")
	}
	if c.Logger == nil {
		t.Fatalf("Logger not set")
	}
}

func TestPrepareScriptlingHandlesNilLoggerAndRegistry(t *testing.T) {
	// Should not panic when neither Logger nor SecretRegistry is provided.
	cfg := NewHandlerConfig(nil)
	p := prepareScriptling(cfg, nil)
	if p == nil {
		t.Fatal("prepareScriptling returned nil")
	}
}
