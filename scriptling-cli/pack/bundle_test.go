package pack

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
)

// writeBundleDir creates a bundle folder fixture in a temp dir.
func writeBundleDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"manifest.toml":    "name = \"app\"\nversion = \"1.0.0\"\nmain = \"setup.py\"\nlibs = [\"lib\", \"vendor\"]\nserve = [\"http\", \"mcp\"]\n",
		"setup.py":         "print('setup')\n",
		"lib/app.py":       "def run():\n    pass\n",
		"vendor/dep.py":    "X = 1\n",
		"tools/greet.py":   "print('greet')\n",
		"tools/greet.toml": "description = \"Greet\"\n",
		"webroot/app.js":   "console.log(1)\n",
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

func TestOpenBundleDir(t *testing.T) {
	dir := writeBundleDir(t)
	b, err := OpenBundleDir(dir)
	if err != nil {
		t.Fatalf("OpenBundleDir: %v", err)
	}
	if b.Manifest.Name != "app" || b.Manifest.Version != "1.0.0" {
		t.Errorf("manifest = %+v", b.Manifest)
	}
	if b.Manifest.Main != "setup.py" {
		t.Errorf("main = %q", b.Manifest.Main)
	}
	if got := b.Manifest.LibDirs(); len(got) != 2 || got[0] != "lib" || got[1] != "vendor" {
		t.Errorf("libs = %v", got)
	}
	if len(b.Manifest.Serve) != 2 || b.Manifest.Serve[0] != "http" {
		t.Errorf("serve = %v", b.Manifest.Serve)
	}

	data, err := b.ReadFile("tools/greet.py")
	if err != nil || string(data) != "print('greet')\n" {
		t.Errorf("ReadFile = %q, %v", data, err)
	}
}

func TestOpenBundleDirMissingManifest(t *testing.T) {
	if _, err := OpenBundleDir(t.TempDir()); err != ErrMissingManifest {
		t.Fatalf("err = %v, want ErrMissingManifest", err)
	}
}

func TestOpenBundleDirNotADir(t *testing.T) {
	f := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenBundleDir(f); err == nil {
		t.Fatal("expected error for non-directory")
	}
}

func TestBundleZipEquivalence(t *testing.T) {
	dir := writeBundleDir(t)
	zipPath := filepath.Join(t.TempDir(), "app.zip")
	if _, _, err := Pack(dir, zipPath, false); err != nil {
		t.Fatalf("Pack: %v", err)
	}

	zipBundle, err := FetchBundle(zipPath, false, "")
	if err != nil {
		t.Fatalf("FetchBundle(zip): %v", err)
	}
	dirBundle, err := FetchBundle(dir, false, "")
	if err != nil {
		t.Fatalf("FetchBundle(dir): %v", err)
	}

	// Same manifest from both backends.
	if !reflect.DeepEqual(zipBundle.Manifest, dirBundle.Manifest) {
		t.Errorf("manifest mismatch: zip %+v vs dir %+v", zipBundle.Manifest, dirBundle.Manifest)
	}

	// Code files present in both (lib/, tools/, setup.py, manifest.toml).
	// All content is read on demand from the zip — nothing pre-loaded.
	codeFiles := []string{
		"manifest.toml", "setup.py",
		"lib/app.py", "vendor/dep.py",
		"tools/greet.py", "tools/greet.toml",
	}
	for _, p := range codeFiles {
		zd, err := zipBundle.ReadFile(p)
		if err != nil {
			t.Errorf("zip ReadFile %s: %v", p, err)
			continue
		}
		dd, err := dirBundle.ReadFile(p)
		if err != nil {
			t.Errorf("dir ReadFile %s: %v", p, err)
			continue
		}
		if string(zd) != string(dd) {
			t.Errorf("content mismatch for %s", p)
		}
	}

	// webroot is accessible via Sub from both backends (lazy in zip, disk in dir).
	zipWeb, zipOK := zipBundle.Sub("webroot")
	if !zipOK {
		t.Fatal("zip bundle should have webroot via Sub")
	}
	dirWeb, dirOK := dirBundle.Sub("webroot")
	if !dirOK {
		t.Fatal("dir bundle should have webroot via Sub")
	}
	// Same webroot content.
	zd, err := fs.ReadFile(zipWeb, "app.js")
	if err != nil {
		t.Fatalf("zip webroot ReadFile: %v", err)
	}
	dd, err := fs.ReadFile(dirWeb, "app.js")
	if err != nil {
		t.Fatalf("dir webroot ReadFile: %v", err)
	}
	if string(zd) != string(dd) {
		t.Errorf("webroot content mismatch: zip %q vs dir %q", zd, dd)
	}
}

func TestManifestLibDirsDefault(t *testing.T) {
	var m Manifest
	if got := m.LibDirs(); len(got) != 1 || got[0] != LibDir {
		t.Errorf("default LibDirs = %v, want [lib]", got)
	}
}

func TestBundleSub(t *testing.T) {
	b, err := OpenBundle(fstest.MapFS{
		"manifest.toml":  &fstest.MapFile{Data: []byte("name=\"a\"\nversion=\"1\"")},
		"tools/greet.py": &fstest.MapFile{Data: []byte("print()")},
	}, "test")
	if err != nil {
		t.Fatal(err)
	}

	sub, ok := b.Sub("tools")
	if !ok {
		t.Fatal("tools sub not found")
	}
	if _, err := fs.ReadFile(sub, "greet.py"); err != nil {
		t.Errorf("read via sub: %v", err)
	}

	if _, ok := b.Sub("prompts"); ok {
		t.Error("prompts sub should not exist")
	}
}

// TestZipBundleLazyAccess verifies the zip-backed bundle reads files on demand
// (nothing pre-loaded into memory) and supports the operations the server needs.
func TestZipBundleLazyAccess(t *testing.T) {
	dir := writeBundleDir(t)
	zipPath := filepath.Join(t.TempDir(), "app.zip")
	if _, _, err := Pack(dir, zipPath, false); err != nil {
		t.Fatalf("Pack: %v", err)
	}
	b, err := FetchBundle(zipPath, false, "")
	if err != nil {
		t.Fatalf("FetchBundle: %v", err)
	}

	// ReadFile — content correct.
	data, err := b.ReadFile("manifest.toml")
	if err != nil || !strings.Contains(string(data), `"app"`) {
		t.Errorf("ReadFile(manifest.toml) = %q %v", data, err)
	}

	// Stat — exists / not found.
	info, err := fs.Stat(b.FS(), "tools/greet.py")
	if err != nil || info.IsDir() {
		t.Errorf("Stat(tools/greet.py) = %+v %v", info, err)
	}
	if _, err := fs.Stat(b.FS(), "nonexistent.py"); err == nil {
		t.Error("Stat should fail for missing file")
	}

	// ReadDir — lists directory children.
	entries, err := fs.ReadDir(b.FS(), "tools")
	if err != nil {
		t.Fatalf("ReadDir(tools): %v", err)
	}
	names := []string{}
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if len(names) != 2 { // greet.py + greet.toml
		t.Errorf("ReadDir(tools) = %v, want 2 entries", names)
	}

	// Sub + ReadFile — webroot served lazily from zip.
	webFS, ok := b.Sub("webroot")
	if !ok {
		t.Fatal("Sub(webroot) not found")
	}
	data, err = fs.ReadFile(webFS, "app.js")
	if err != nil || !strings.Contains(string(data), "console.log") {
		t.Errorf("webroot app.js = %q %v", data, err)
	}

	// WalkDir works over the full bundle.
	var fileCount int
	err = fs.WalkDir(b.FS(), ".", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			fileCount++
		}
		return err
	})
	if err != nil {
		t.Fatalf("WalkDir: %v", err)
	}
	if fileCount == 0 {
		t.Error("WalkDir found no files")
	}

	// Bad paths rejected.
	for _, bad := range []string{"../x", "/abs", ""} {
		if _, err := b.FS().Open(bad); err == nil {
			t.Errorf("Open(%q) should fail", bad)
		}
	}
}
