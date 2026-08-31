package pack

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeBuildFixture creates a source dir exercising the full inclusion rules.
func writeBuildFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"manifest.toml":      "name=\"app\"\nversion=\"1\"\nmain=\"setup.py\"\nlibs=[\"lib\",\"vendor\"]\nserve=[\"http\",\"mcp\"]\n",
		"setup.py":           "print('setup')",
		"lib/app.py":         "app",
		"vendor/dep.py":      "dep",
		"tools/t.py":         "tool",
		"tools/t.toml":       "description=\"t\"",
		"resources/r/a.json": "{}",
		"prompts/p.md":       "prompt",
		"webroot/index.html": "<h1>x</h1>",
		"docs/guide.md":      "guide",
		"unknown/x.py":       "not included",
		"stray.txt":          "not included",
		".git/config":        "not included",
		".DS_Store":          "not included",
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

func zipEntries(t *testing.T, zipPath string) map[string]bool {
	t.Helper()
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	out := map[string]bool{}
	for _, f := range zr.File {
		out[f.Name] = true
	}
	return out
}

func TestPackBuildInclusion(t *testing.T) {
	dir := writeBuildFixture(t)
	zipPath := filepath.Join(t.TempDir(), "app.zip")
	_, warnings, err := Pack(dir, zipPath, false)
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}

	entries := zipEntries(t, zipPath)
	want := []string{
		"manifest.toml", "setup.py",
		"lib/app.py", "vendor/dep.py",
		"tools/t.py", "tools/t.toml",
		"resources/r/a.json", "prompts/p.md",
		"webroot/index.html", "docs/guide.md",
	}
	for _, name := range want {
		if !entries[name] {
			t.Errorf("missing entry %s in zip: %v", name, entries)
		}
	}

	notWant := []string{"unknown/x.py", "stray.txt", ".git/config", ".DS_Store"}
	for _, name := range notWant {
		if entries[name] {
			t.Errorf("unexpected entry %s in zip", name)
		}
	}

	// Warnings mention the unknown dir and stray file, never dotfiles.
	joined := strings.Join(warnings, "\n")
	if !strings.Contains(joined, "unknown/") {
		t.Errorf("warnings should mention unknown/: %v", warnings)
	}
	if !strings.Contains(joined, "stray.txt") {
		t.Errorf("warnings should mention stray.txt: %v", warnings)
	}
	if strings.Contains(joined, ".git") || strings.Contains(joined, ".DS_Store") {
		t.Errorf("warnings should not mention dotfiles: %v", warnings)
	}
}

func TestPackBuildMissingLibsDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "manifest.toml"),
		[]byte("name=\"a\"\nversion=\"1\"\nlibs=[\"lib\",\"missing\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "lib"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, _, err := Pack(dir, filepath.Join(t.TempDir(), "a.zip"), false)
	if err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("expected error naming the missing libs dir, got %v", err)
	}
}

func TestPackBuildMissingMainScript(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "manifest.toml"),
		[]byte("name=\"a\"\nversion=\"1\"\nmain=\"nope.py\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "lib"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, _, err := Pack(dir, filepath.Join(t.TempDir(), "a.zip"), false)
	if err == nil || !strings.Contains(err.Error(), "nope.py") {
		t.Fatalf("expected error naming the missing main script, got %v", err)
	}
}

// TestPackBuildLibraryPack verifies a classic library pack (no serve, no main
// script) still builds with no warnings.
func TestPackBuildLibraryPack(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"manifest.toml":     "name=\"lib\"\nversion=\"1\"\nmain=\"demo.run\"\n",
		"lib/demo.py":       "def run():\n    pass\n",
		"docs/lib/index.md": "docs",
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
	_, warnings, err := Pack(dir, filepath.Join(t.TempDir(), "lib.zip"), false)
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
}

// TestPackBuildMinimalApp verifies an app with just manifest + startup.py and
// no lib/ dir builds successfully (libs defaults to ["lib"] but isn't required
// to exist when not explicitly declared).
func TestPackBuildMinimalApp(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"manifest.toml": "name=\"mini\"\nversion=\"1\"\nmain=\"startup.py\"\nserve=[\"http\"]\n",
		"startup.py":    "import scriptling.runtime as runtime\nruntime.http.get(\"/\", \"startup.hello\")\n",
	}
	for name, content := range files {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	zipPath := filepath.Join(t.TempDir(), "mini.zip")
	_, _, err := Pack(dir, zipPath, false)
	if err != nil {
		t.Fatalf("Pack should succeed without lib/ dir: %v", err)
	}
	entries := zipEntries(t, zipPath)
	if !entries["manifest.toml"] || !entries["startup.py"] {
		t.Errorf("expected manifest.toml + startup.py, got: %v", entries)
	}
}

// TestPackBuildMainScriptInSubfolder verifies main = "app/startup.py" is
// included even though app/ isn't a declared libs dir or convention dir.
func TestPackBuildMainScriptInSubfolder(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "app"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"manifest.toml":  "name=\"sub\"\nversion=\"1\"\nmain=\"app/startup.py\"\nserve=[\"http\"]\n",
		"app/startup.py": "print('hello')\n",
		"app/other.py":   "# not included (only main script under app/)\n",
	}
	for name, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	zipPath := filepath.Join(t.TempDir(), "sub.zip")
	_, warnings, err := Pack(dir, zipPath, false)
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}

	entries := zipEntries(t, zipPath)
	if !entries["app/startup.py"] {
		t.Errorf("main script app/startup.py missing from zip: %v", entries)
	}
	// other.py under app/ is NOT included (only the main script is, not siblings)
	if entries["app/other.py"] {
		t.Error("app/other.py should not be included (not the main script, app/ not in libs)")
	}
	// The app/ dir should produce a warning since it's not declared
	joined := strings.Join(warnings, "\n")
	if !strings.Contains(joined, "app/") {
		t.Errorf("expected warning about app/ dir, got: %v", warnings)
	}
}

// TestPackOutputAtomicAndOutsideSource pins the pack output contract: a
// failure mid-pack leaves any previous artifact untouched (no partial zip),
// and an output path inside the source tree is refused rather than letting
// the archive include itself.
func TestPackOutputAtomicAndOutsideSource(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "app")
	if err := os.MkdirAll(filepath.Join(src, "lib"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "manifest.toml"), []byte("name=\"a\"\nversion=\"1\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "lib", "x.py"), []byte("x = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Output inside the source tree: refused outright.
	if _, _, err := Pack(src, filepath.Join(src, "app.zip"), true); err == nil {
		t.Fatal("expected an in-tree output to be refused")
	}

	// A valid pack succeeds and leaves no temp file behind.
	out := filepath.Join(dir, "app.zip")
	if _, _, err := Pack(src, out, true); err != nil {
		t.Fatalf("pack: %v", err)
	}
	if _, err := os.Stat(out + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("temp file left behind: %v", err)
	}
	before, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}

	// A failing pack (main script vanished) leaves the good artifact intact.
	broken := filepath.Join(dir, "broken")
	if err := os.MkdirAll(filepath.Join(broken, "lib"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(broken, "manifest.toml"), []byte("name=\"b\"\nversion=\"1\"\nmain=\"gone.py\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Pack(broken, out, true); err == nil {
		t.Fatal("expected the broken pack to fail")
	}
	after, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("a failed pack modified the previous artifact")
	}
}

// TestPackSkipsSourceSymlinks pins that a symlink inside the source tree is
// never followed into the archive: os.Open would have copied whatever the
// link pointed at, including files outside the tree.
func TestPackSkipsSourceSymlinks(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "app")
	if err := os.MkdirAll(filepath.Join(src, "lib"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "manifest.toml"), []byte("name=\"a\"\nversion=\"1\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "lib", "real.py"), []byte("x = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(dir, "outside")
	if err := os.WriteFile(outside, []byte("TOP SECRET"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(src, "lib", "leak.py")); err != nil {
		t.Fatal(err)
	}

	archive := filepath.Join(dir, "app.zip")
	_, warnings, err := Pack(src, archive, true)
	if err != nil {
		t.Fatalf("pack: %v", err)
	}
	foundLeak, foundWarning := false, false
	for _, w := range warnings {
		if strings.Contains(w, "leak.py") {
			foundWarning = true
		}
	}
	zr, err := zip.OpenReader(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	for _, f := range zr.File {
		if f.Name == "lib/leak.py" {
			foundLeak = true
		}
	}
	if foundLeak {
		t.Fatal("symlinked file was followed into the archive")
	}
	if !foundWarning {
		t.Fatalf("expected a warning about the skipped symlink, got %v", warnings)
	}
}

// TestPackUniqueTempFile pins that concurrent packs targeting one destination
// stage in separate files instead of overwriting each other's dst+".tmp".
func TestPackUniqueTempFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "app")
	if err := os.MkdirAll(filepath.Join(src, "lib"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "manifest.toml"), []byte("name=\"a\"\nversion=\"1\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "lib", "x.py"), []byte("x = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(dir, "app.zip")
	// Two packs in flight; both must succeed and the archive must be valid.
	done := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			_, _, err := Pack(src, out, true)
			done <- err
		}()
	}
	for i := 0; i < 2; i++ {
		if err := <-done; err != nil {
			t.Fatalf("concurrent pack: %v", err)
		}
	}
	if _, err := zip.OpenReader(out); err != nil {
		t.Fatalf("archive invalid after concurrent packs: %v", err)
	}
}
