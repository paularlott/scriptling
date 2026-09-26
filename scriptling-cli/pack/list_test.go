package pack

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestListPackage packs a small app and verifies the summary names the
// manifest, lists convention directories with counts, and carries the sha256.
func TestListPackage(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"manifest.toml": "name = \"listapp\"\nversion = \"2.1.0\"\nmain = \"setup.py\"\nserve = [\"mcp\", \"http\"]\n",
		"setup.py":      "# entry\n",
		"tools/one.py":  "import scriptling.runtime.mcp as mcp\n\n@mcp.tool(\"One\")\ndef one():\n    return 1\n",
		"tools/two.py":  "import scriptling.runtime.mcp as mcp\n\n@mcp.tool(\"Two\")\ndef two():\n    return 2\n",
		"skills/s/SKILL.md": "---\nname: s\ndescription: d\n---\n\nbody",
		"webroot/index.html": "<html></html>",
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

	out := filepath.Join(t.TempDir(), "listapp.zip")
	hash, _, err := Pack(dir, out, false)
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}

	summary, err := ListPackage(out)
	if err != nil {
		t.Fatalf("ListPackage: %v", err)
	}
	for _, want := range []string{
		"package: listapp 2.1.0",
		"serves: mcp,http",
		"tools/       2 file(s)",
		"skills/      1 file(s)",
		"webroot/     1 file(s)",
		"(root)/      2 file(s)",
		"total        6",
		"sha256=" + hash,
	} {
		if !strings.Contains(summary, want) {
			t.Errorf("summary missing %q:\n%s", want, summary)
		}
	}
}

// TestListPackageMissingFile: a nonexistent package is an error.
func TestListPackageMissingFile(t *testing.T) {
	if _, err := ListPackage(filepath.Join(t.TempDir(), "nope.zip")); err == nil {
		t.Fatal("expected an error for a missing package")
	}
}
