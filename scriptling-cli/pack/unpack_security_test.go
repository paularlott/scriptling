package pack

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// buildZip packs files (slash path -> content) into a real zip archive.
func buildZip(t *testing.T, path string, files map[string]string, sizes map[string]uint64) {
	t.Helper()
	zw := zip.NewWriter(mustCreate(t, path))
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	_ = sizes // declared sizes are set below when needed
}

func mustCreate(t *testing.T, path string) *os.File {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.Close() })
	return f
}

// TestUnpackRefusesSymlinkRedirect pins the extraction containment: a
// pre-existing symlink in the destination tree must not redirect extraction
// outside it, whether the link is a directory component or the final path.
func TestUnpackRefusesSymlinkRedirect(t *testing.T) {
	base := t.TempDir()
	outside := filepath.Join(base, "outside")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(base, "dest")
	if err := os.MkdirAll(filepath.Join(dest, "lib"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A directory-component symlink pointing outside the destination.
	if err := os.Symlink(outside, filepath.Join(dest, "lib", "sub")); err != nil {
		t.Fatal(err)
	}

	archive := filepath.Join(base, "p.zip")
	zw := zip.NewWriter(mustCreate(t, archive))
	w, err := zw.Create("lib/sub/leak.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("secret")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	err = Unpack(archive, UnpackOptions{DestDir: dest, Force: true})
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("expected extraction through the symlink to be refused, got: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(outside, "leak.txt")); !os.IsNotExist(statErr) {
		t.Fatal("extraction escaped the destination through a symlink")
	}
}

// TestUnpackBudgets pins the expansion budgets: an archive whose entries
// declare more uncompressed data than the total budget is refused before
// extraction begins.
func TestUnpackBudgets(t *testing.T) {
	base := t.TempDir()
	dest := filepath.Join(base, "dest")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}

	many := filepath.Join(base, "many.zip")
	zw2 := zip.NewWriter(mustCreate(t, many))
	for i := 0; i < maxUnpackEntries+5; i++ {
		w, err := zw2.Create("lib/f" + string(rune('a'+i%26)) + "/" + itoa(i) + ".txt")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte("x")); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw2.Close(); err != nil {
		t.Fatal(err)
	}
	if err := Unpack(many, UnpackOptions{DestDir: dest, Force: true}); err == nil || !strings.Contains(err.Error(), "entries") {
		t.Fatalf("expected an entry-count refusal, got: %v", err)
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}
