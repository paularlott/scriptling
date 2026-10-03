package libloader

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeAged writes content to path and sets its mtime to at.
func writeAged(t *testing.T, path, content string, at time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, at, at); err != nil {
		t.Fatal(err)
	}
}

func isCached(path string) bool {
	sourceCache.RLock()
	defer sourceCache.RUnlock()
	_, ok := sourceCache.files[path]
	return ok
}

func mustLoad(t *testing.T, l *FilesystemLoader, name, want string) {
	t.Helper()
	got, found, err := l.Load(name)
	if err != nil {
		t.Fatalf("Load(%q): %v", name, err)
	}
	if !found {
		t.Fatalf("Load(%q): not found", name)
	}
	if got != want {
		t.Fatalf("Load(%q) = %q, want %q", name, got, want)
	}
}

func TestSourceCacheHitAndInvalidate(t *testing.T) {
	ClearSourceCache()
	dir := t.TempDir()
	path := filepath.Join(dir, "lib.py")
	old := time.Now().Add(-time.Hour)
	writeAged(t, path, "x = 1", old)

	mustLoad(t, NewFilesystem(dir), "lib", "x = 1")
	if !isCached(path) {
		t.Fatal("old file was not cached")
	}
	// A different loader instance shares the cache.
	mustLoad(t, NewFilesystem(dir), "lib", "x = 1")

	// Same size, different (still old) mtime: must re-read.
	writeAged(t, path, "x = 2", old.Add(time.Minute))
	mustLoad(t, NewFilesystem(dir), "lib", "x = 2")
}

func TestSourceCacheRecentFileNotTrusted(t *testing.T) {
	ClearSourceCache()
	dir := t.TempDir()
	path := filepath.Join(dir, "lib.py")
	// Simulate a coarse-timestamp filesystem: both writes land on the same
	// recent mtime with the same size.
	recent := time.Now().Truncate(time.Second)
	writeAged(t, path, "x = 1", recent)
	mustLoad(t, NewFilesystem(dir), "lib", "x = 1")
	if isCached(path) {
		t.Fatal("recently modified file was cached")
	}
	writeAged(t, path, "x = 2", recent)
	mustLoad(t, NewFilesystem(dir), "lib", "x = 2")
}

func TestSourceCacheReplacedFile(t *testing.T) {
	ClearSourceCache()
	dir := t.TempDir()
	path := filepath.Join(dir, "lib.py")
	old := time.Now().Add(-time.Hour)
	writeAged(t, path, "x = 1", old)
	mustLoad(t, NewFilesystem(dir), "lib", "x = 1")

	// Atomic save: new inode, same size and mtime.
	tmp := filepath.Join(dir, "lib.tmp")
	writeAged(t, tmp, "x = 2", old)
	if err := os.Rename(tmp, path); err != nil {
		t.Fatal(err)
	}
	mustLoad(t, NewFilesystem(dir), "lib", "x = 2")
}

func TestSourceCacheDeletedFile(t *testing.T) {
	ClearSourceCache()
	dir := t.TempDir()
	path := filepath.Join(dir, "lib.py")
	writeAged(t, path, "x = 1", time.Now().Add(-time.Hour))
	mustLoad(t, NewFilesystem(dir), "lib", "x = 1")

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	_, found, err := NewFilesystem(dir).Load("lib")
	if err != nil || found {
		t.Fatalf("deleted file: found=%v err=%v", found, err)
	}
}

func TestSourceCacheHigherPriorityAppears(t *testing.T) {
	ClearSourceCache()
	dir := t.TempDir()
	old := time.Now().Add(-time.Hour)
	// Only the legacy flat file exists at first.
	writeAged(t, filepath.Join(dir, "pkg.mod.py"), "flat", old)
	mustLoad(t, NewFilesystem(dir), "pkg.mod", "flat")

	// The preferred folder layout now wins.
	writeAged(t, filepath.Join(dir, "pkg", "mod.py"), "folder", old)
	mustLoad(t, NewFilesystem(dir), "pkg.mod", "folder")
}

func TestSourceCacheMultiDirOverride(t *testing.T) {
	ClearSourceCache()
	user, system := t.TempDir(), t.TempDir()
	old := time.Now().Add(-time.Hour)
	writeAged(t, filepath.Join(system, "lib.py"), "system", old)

	m := NewMultiFilesystem(user, system)
	got, _, _ := m.Load("lib")
	if got != "system" {
		t.Fatalf("got %q, want system", got)
	}
	writeAged(t, filepath.Join(user, "lib.py"), "user", old)
	got, _, _ = NewMultiFilesystem(user, system).Load("lib")
	if got != "user" {
		t.Fatalf("got %q, want user override", got)
	}
}

func TestSourceCacheDirectoryCandidateSkipped(t *testing.T) {
	ClearSourceCache()
	dir := t.TempDir()
	// A directory named like the module file must not be read.
	if err := os.MkdirAll(filepath.Join(dir, "pkg.py"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeAged(t, filepath.Join(dir, "pkg", "__init__.py"), "init", time.Now().Add(-time.Hour))
	mustLoad(t, NewFilesystem(dir), "pkg", "init")
}

func TestSourceCacheByteLimit(t *testing.T) {
	ClearSourceCache()
	SetSourceCacheMaxBytes(10)
	defer SetSourceCacheMaxBytes(DefaultSourceCacheMaxBytes)
	dir := t.TempDir()
	old := time.Now().Add(-time.Hour)
	small, big := filepath.Join(dir, "small.py"), filepath.Join(dir, "big.py")
	writeAged(t, small, "x = 1", old)
	writeAged(t, big, "y = 22222222", old)

	mustLoad(t, NewFilesystem(dir), "small", "x = 1")
	mustLoad(t, NewFilesystem(dir), "big", "y = 22222222")
	if !isCached(small) {
		t.Fatal("file within the limit was not cached")
	}
	if isCached(big) {
		t.Fatal("file over the limit was cached")
	}
	// Still loads correctly, just from disk.
	mustLoad(t, NewFilesystem(dir), "big", "y = 22222222")

	sourceCache.RLock()
	used := sourceCache.bytes
	sourceCache.RUnlock()
	if used != len("x = 1") {
		t.Fatalf("cache accounts %d bytes, want %d", used, len("x = 1"))
	}
}
