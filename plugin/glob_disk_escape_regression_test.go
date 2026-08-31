package plugin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGlobDiskRejectsParentTraversal pins the helper's containment: the
// literal prefix of a pattern becomes the walk start, so a ".." segment there
// used to walk (and match) the directory above root. It must be refused
// outright. The host already rejects such patterns before any RPC; this
// guards direct callers of GlobDisk.
func TestGlobDiskRejectsParentTraversal(t *testing.T) {
	outside := t.TempDir()
	root := filepath.Join(outside, "served")
	if err := os.MkdirAll(filepath.Join(root, "lib"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "lib", "a.py"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("top secret"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, pattern := range []string{"../secret.txt", "../*", "../**", "lib/../../secret.txt"} {
		entries, err := GlobDisk(root, pattern)
		if err == nil || !strings.Contains(err.Error(), "outside the served root") {
			t.Errorf("GlobDisk(%q) = %v, %v; want a refusal", pattern, entries, err)
		}
	}

	// Ordinary patterns still match inside the root.
	entries, err := GlobDisk(root, "**/*.py")
	if err != nil || len(entries) != 1 || entries[0].Name != "lib/a.py" {
		t.Fatalf("normal pattern broken: %v, %v", entries, err)
	}
}
