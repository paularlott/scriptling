package mcp

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestScanToolsFS(t *testing.T) {
	fsys := fstest.MapFS{
		"greet.toml": &fstest.MapFile{Data: []byte(`description = "Greet"
[[parameters]]
name = "name"
type = "string"
required = true
`)},
		"greet.py":      &fstest.MapFile{Data: []byte(`print("hi")`)},
		"noop.toml":     &fstest.MapFile{Data: []byte(`description = "No params"`)},
		"readme.md":     &fstest.MapFile{Data: []byte(`# ignored`)},
		"sub/nest.toml": &fstest.MapFile{Data: []byte(`description = "nested, ignored"`)},
	}

	tools, err := ScanToolsFS(fsys)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("tools = %d, want 2: %v", len(tools), tools)
	}
	greet := tools["greet"]
	if greet == nil || greet.Description != "Greet" {
		t.Fatalf("greet = %+v", greet)
	}
	if len(greet.Parameters) != 1 || greet.Parameters[0].Name != "name" || !greet.Parameters[0].Required {
		t.Errorf("greet params = %+v", greet.Parameters)
	}
	if tools["noop"] == nil {
		t.Error("noop missing")
	}
	if tools["nest"] != nil {
		t.Error("nested toml should not be scanned (flat root only)")
	}
}

func TestScanToolsFSErrors(t *testing.T) {
	if _, err := ScanToolsFS(fstest.MapFS{
		"bad.toml": &fstest.MapFile{Data: []byte(`description = "unclosed`)},
	}); err == nil {
		t.Error("expected parse error")
	}
}

func TestScanResourcesFS(t *testing.T) {
	fsys := fstest.MapFS{
		"config/app.json":      &fstest.MapFile{Data: []byte(`{"a":1}`)},
		"config/_app.toml":     &fstest.MapFile{Data: []byte(`name = "App Config"`)},
		"docs/readme.json":     &fstest.MapFile{Data: []byte(`{"doc":true}`)},
		"kv/{key}.py":          &fstest.MapFile{Data: []byte(`print("x")`)},
		"kv/_{key}.toml":       &fstest.MapFile{Data: []byte(`description = "Key value"`)},
		"orphan/{id}.txt":      &fstest.MapFile{Data: []byte(`no handler`)},
		"_hidden.md":           &fstest.MapFile{Data: []byte(`hidden`)},
		"rootlevel.md":         &fstest.MapFile{Data: []byte(`no scheme dir`)},
		"kv/deep/{a}/{b}.py":   &fstest.MapFile{Data: []byte(`print("y")`)},
		"config/settings.toml": &fstest.MapFile{Data: []byte(`x = 1`)},
	}

	res, err := ScanResourcesFS(fsys)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byURI := map[string]scannedResource{}
	for _, r := range res {
		byURI[r.URI] = r
	}

	// Static with metadata sibling.
	r, ok := byURI["config://app.json"]
	if !ok {
		t.Fatalf("missing config://app.json: %v", byURI)
	}
	if r.Template || r.Name != "App Config" || r.FilePath != "config/app.json" {
		t.Errorf("app.json = %+v", r)
	}

	// Static without metadata: name falls back to URI, mime from extension.
	r, ok = byURI["docs://readme.json"]
	if !ok || r.Template || r.Name != "docs://readme.json" || r.MimeType != "application/json" {
		t.Errorf("readme = %+v", r)
	}

	// Template with vars and metadata.
	r, ok = byURI["kv://{key}"]
	if !ok || !r.Template || r.Description != "Key value" || len(r.Vars) != 1 || r.Vars[0] != "key" {
		t.Errorf("kv template = %+v", r)
	}

	// Multi-var nested template.
	r, ok = byURI["kv://deep/{a}/{b}"]
	if !ok || !r.Template || len(r.Vars) != 2 {
		t.Errorf("deep template = %+v", r)
	}

	// {var} without .py handler is skipped.
	if _, ok := byURI["orphan://{id}.txt"]; ok {
		t.Error("orphan template without .py should be skipped")
	}

	// _-prefixed and root-level files are skipped.
	if _, ok := byURI["kv://_hidden.md"]; ok {
		t.Error("_-prefixed file should be skipped")
	}
	for _, r := range res {
		if r.FilePath == "rootlevel.md" || r.FilePath == "_hidden.md" {
			t.Errorf("unexpected resource: %+v", r)
		}
	}

	// A .toml that is NOT a _-prefixed metadata sibling is served as a static resource.
	if _, ok := byURI["config://settings.toml"]; !ok {
		t.Error("settings.toml should be a static resource")
	}
}

func TestScanResourcesFSBadMetadata(t *testing.T) {
	fsys := fstest.MapFS{
		"cfg/a.json":   &fstest.MapFile{Data: []byte(`{}`)},
		"cfg/_a.toml":  &fstest.MapFile{Data: []byte(`name = "unclosed`)},
		"cfg/ok.json":  &fstest.MapFile{Data: []byte(`{}`)},
		"cfg/_ok.toml": &fstest.MapFile{Data: []byte(`name = "OK"`)},
	}
	if _, err := ScanResourcesFS(fsys); err == nil {
		t.Error("expected error for malformed metadata sibling")
	}
}

func TestScanPromptsFS(t *testing.T) {
	fsys := fstest.MapFS{
		"summarize.toml": &fstest.MapFile{Data: []byte(`description = "Summarize"
[[arguments]]
name = "text"
required = true
`)},
		"summarize.py":  &fstest.MapFile{Data: []byte(`print("s")`)},
		"summarize.md":  &fstest.MapFile{Data: []byte(`dynamic wins`)},
		"note.md":       &fstest.MapFile{Data: []byte("\nFirst real line\nsecond\n")},
		"quote.txt":     &fstest.MapFile{Data: []byte(`TXT prompt`)},
		"declared.toml": &fstest.MapFile{Data: []byte(`description = "no script, skipped"`)},
		"sub/nest.toml": &fstest.MapFile{Data: []byte("description = \"nested\"\n")},
	}

	prompts, err := ScanPromptsFS(fsys)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := map[string]scannedPrompt{}
	for _, p := range prompts {
		byName[p.Name] = p
	}

	// Dynamic wins over static with same stem.
	p, ok := byName["summarize"]
	if !ok || p.Static || p.Description != "Summarize" || len(p.Arguments) != 1 || p.FilePath != "summarize.py" {
		t.Errorf("summarize = %+v", p)
	}

	// Static md: description is first non-empty line.
	p, ok = byName["note"]
	if !ok || !p.Static || p.Description != "First real line" || p.FilePath != "note.md" {
		t.Errorf("note = %+v", p)
	}

	// .txt static prompt.
	if _, ok := byName["quote"]; !ok {
		t.Error("quote.txt missing")
	}

	// toml without .py is skipped.
	if _, ok := byName["declared"]; ok {
		t.Error("declared without .py should be skipped")
	}

	// Nested files are not scanned (flat root only).
	if _, ok := byName["nest"]; ok {
		t.Error("nested prompt should not be scanned")
	}

	if len(prompts) != 3 {
		t.Errorf("prompts = %d, want 3: %v", len(prompts), byName)
	}
}

// TestFolderWrappers ensures the disk wrappers produce disk paths and find the
// same entries as the FS scanners.
func TestFolderWrappers(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("a.toml", `description = "A"`)
	write("a.py", `print("a")`)

	tools, err := ScanToolsFolder(dir)
	if err != nil || len(tools) != 1 || tools["a"].Description != "A" {
		t.Fatalf("tools = %v, %v", tools, err)
	}

	prompts, err := ScanPromptsFolder(dir)
	if err != nil || len(prompts) != 1 {
		t.Fatalf("prompts = %v, %v", prompts, err)
	}
	if prompts[0].FilePath != dir+"/a.py" {
		t.Errorf("prompt FilePath = %q, want disk path under %q", prompts[0].FilePath, dir)
	}
}
