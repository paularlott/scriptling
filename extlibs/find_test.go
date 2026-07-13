package extlibs

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/paularlott/scriptling/extlibs/fssecurity"
	"github.com/paularlott/scriptling/object"
)

// buildFindTree creates a temp tree:
//
//	root/
//	  a.txt          (small, old)
//	  b.log          (small, old)
//	  .hidden        (dot-file)
//	  big.bin        (large, old)
//	  sub/
//	    c.txt        (small, new)
//	    d.md         (small, new)
//	    .secret.md   (dot-file)
//	    deep/
//	      e.txt      (small, old)
//	  .gitdir/
//	    config      (dot-dir entry)
func buildFindTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	oldTime := time.Now().Add(-48 * time.Hour)
	newTime := time.Now().Add(-1 * time.Hour)

	writeFileAt(t, filepath.Join(root, "a.txt"), "small", oldTime)
	writeFileAt(t, filepath.Join(root, "b.log"), "small", oldTime)
	writeFileAt(t, filepath.Join(root, ".hidden"), "x", oldTime)
	writeFileAt(t, filepath.Join(root, "big.bin"), strings.Repeat("x", 5000), oldTime)

	os.MkdirAll(filepath.Join(root, "sub", "deep"), 0755)
	writeFileAt(t, filepath.Join(root, "sub", "c.txt"), "small", newTime)
	writeFileAt(t, filepath.Join(root, "sub", "d.md"), "small", newTime)
	writeFileAt(t, filepath.Join(root, "sub", ".secret.md"), "x", newTime)
	writeFileAt(t, filepath.Join(root, "sub", "deep", "e.txt"), "small", oldTime)

	os.MkdirAll(filepath.Join(root, ".gitdir"), 0755)
	writeFileAt(t, filepath.Join(root, ".gitdir", "config"), "x", oldTime)

	return root
}

func writeFileAt(t *testing.T, path, content string, mtime time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatal(err)
	}
}

func runFind(t *testing.T, root string, opts findOptions) []string {
	t.Helper()
	inst := &findLibraryInstance{config: fssecurity.Config{AllowedPaths: nil}}
	raw := inst.findPaths(context.Background(), root, opts)
	rel := make([]string, len(raw))
	for i, p := range raw {
		r, _ := filepath.Rel(root, p)
		rel[i] = r
	}
	sort.Strings(rel)
	return rel
}

func TestFindByName(t *testing.T) {
	root := buildFindTree(t)
	got := runFind(t, root, findOptions{recursive: true, entryType: "any", name: "*.txt"})
	want := []string{
		"sub/deep/e.txt",
		"sub/c.txt",
		"a.txt",
	}
	sort.Strings(want)
	if !equalStringSlices(got, want) {
		t.Errorf("name *.txt:\n got %v\nwant %v", got, want)
	}
}

func TestFindByNameIncludeHidden(t *testing.T) {
	root := buildFindTree(t)
	got := runFind(t, root, findOptions{recursive: true, entryType: "any", name: "*.md", includeHidden: true})
	want := []string{
		"sub/.secret.md",
		"sub/d.md",
	}
	sort.Strings(want)
	if !equalStringSlices(got, want) {
		t.Errorf("name *.md include_hidden:\n got %v\nwant %v", got, want)
	}
}

func TestFindNonRecursive(t *testing.T) {
	root := buildFindTree(t)
	got := runFind(t, root, findOptions{recursive: false, entryType: "any"})
	// Immediate children only: a.txt, b.log, big.bin, sub (dir). Dot-entries skipped.
	want := []string{"a.txt", "b.log", "big.bin", "sub"}
	sort.Strings(want)
	if !equalStringSlices(got, want) {
		t.Errorf("non-recursive:\n got %v\nwant %v", got, want)
	}
}

func TestFindTypeDir(t *testing.T) {
	root := buildFindTree(t)
	got := runFind(t, root, findOptions{recursive: true, entryType: "dir"})
	want := []string{"sub", "sub/deep"}
	sort.Strings(want)
	if !equalStringSlices(got, want) {
		t.Errorf("type dir:\n got %v\nwant %v", got, want)
	}
}

func TestFindTypeFile(t *testing.T) {
	root := buildFindTree(t)
	got := runFind(t, root, findOptions{recursive: true, entryType: "file"})
	for _, p := range got {
		if strings.HasPrefix(filepath.Base(p), ".") {
			t.Errorf("type file returned dot-entry: %s", p)
		}
	}
	// Should not contain "sub" or "sub/deep" (dirs).
	for _, dir := range []string{"sub", "sub/deep"} {
		for _, p := range got {
			if p == dir {
				t.Errorf("type file returned directory: %s", p)
			}
		}
	}
}

func TestFindMtimeMin(t *testing.T) {
	root := buildFindTree(t)
	cutoff := float64(time.Now().Add(-2*time.Hour).UnixNano()) / 1e9
	got := runFind(t, root, findOptions{
		recursive:   true,
		entryType:   "file",
		name:        "*.txt",
		mtimeMin:    cutoff,
		hasMtimeMin: true,
	})
	// Only c.txt was modified within the last 2h.
	want := []string{"sub/c.txt"}
	sort.Strings(want)
	if !equalStringSlices(got, want) {
		t.Errorf("mtime_min:\n got %v\nwant %v", got, want)
	}
}

func TestFindMtimeMax(t *testing.T) {
	root := buildFindTree(t)
	cutoff := float64(time.Now().Add(-24*time.Hour).UnixNano()) / 1e9
	got := runFind(t, root, findOptions{
		recursive:   true,
		entryType:   "file",
		name:        "*.txt",
		mtimeMax:    cutoff,
		hasMtimeMax: true,
	})
	// a.txt and e.txt are 48h old (older than 24h cutoff).
	want := []string{"a.txt", "sub/deep/e.txt"}
	sort.Strings(want)
	if !equalStringSlices(got, want) {
		t.Errorf("mtime_max:\n got %v\nwant %v", got, want)
	}
}

func TestFindSizeFilter(t *testing.T) {
	root := buildFindTree(t)
	got := runFind(t, root, findOptions{
		recursive:  true,
		entryType:  "file",
		sizeMin:    1000,
		hasSizeMin: true,
	})
	want := []string{"big.bin"}
	sort.Strings(want)
	if !equalStringSlices(got, want) {
		t.Errorf("size_min 1000:\n got %v\nwant %v", got, want)
	}
}

func TestFindMaxDepth(t *testing.T) {
	root := buildFindTree(t)
	got := runFind(t, root, findOptions{recursive: true, entryType: "any", maxDepth: 1})
	// maxDepth 1 = immediate children only.
	want := []string{"a.txt", "b.log", "big.bin", "sub"}
	sort.Strings(want)
	if !equalStringSlices(got, want) {
		t.Errorf("max_depth 1:\n got %v\nwant %v", got, want)
	}
}

func TestFindMaxDepth2(t *testing.T) {
	root := buildFindTree(t)
	got := runFind(t, root, findOptions{recursive: true, entryType: "any", maxDepth: 2})
	want := []string{
		"a.txt", "b.log", "big.bin",
		"sub", "sub/c.txt", "sub/d.md", "sub/deep",
	}
	sort.Strings(want)
	if !equalStringSlices(got, want) {
		t.Errorf("max_depth 2:\n got %v\nwant %v", got, want)
	}
}

func TestFindSingleFile(t *testing.T) {
	root := buildFindTree(t)
	target := filepath.Join(root, "a.txt")
	inst := &findLibraryInstance{config: fssecurity.Config{AllowedPaths: nil}}
	got := inst.findPaths(context.Background(), target, findOptions{recursive: true, entryType: "any", name: "*.txt"})
	if len(got) != 1 || got[0] != target {
		t.Errorf("single file:\n got %v\nwant [%s]", got, target)
	}
}

func TestFindSingleFileNoMatch(t *testing.T) {
	root := buildFindTree(t)
	target := filepath.Join(root, "a.txt")
	inst := &findLibraryInstance{config: fssecurity.Config{AllowedPaths: nil}}
	got := inst.findPaths(context.Background(), target, findOptions{recursive: true, entryType: "any", name: "*.md"})
	if len(got) != 0 {
		t.Errorf("single file no match: expected empty, got %v", got)
	}
}

func TestFindNonExistent(t *testing.T) {
	inst := &findLibraryInstance{config: fssecurity.Config{AllowedPaths: nil}}
	got := inst.findPaths(context.Background(), "/nonexistent/path/xyz", findOptions{recursive: true})
	if len(got) != 0 {
		t.Errorf("nonexistent: expected empty, got %v", got)
	}
}

func TestFindIncludeHiddenDescendsDotDir(t *testing.T) {
	root := buildFindTree(t)
	got := runFind(t, root, findOptions{recursive: true, entryType: "file", includeHidden: true})
	found := false
	for _, p := range got {
		if p == filepath.Join(".gitdir", "config") {
			found = true
		}
	}
	if !found {
		t.Errorf("include_hidden should descend into .gitdir: got %v", got)
	}

	// Without include_hidden, .gitdir/config must be absent.
	got = runFind(t, root, findOptions{recursive: true, entryType: "file", includeHidden: false})
	for _, p := range got {
		if p == filepath.Join(".gitdir", "config") {
			t.Errorf(".gitdir/config should be hidden: got %v", got)
		}
	}
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// --- Security (allowedPaths) tests ---

func TestFindSecurityRestrictsToAllowedDir(t *testing.T) {
	root := buildFindTree(t)
	// Restrict to the "sub" subdirectory only.
	allowed := filepath.Join(root, "sub")
	inst := &findLibraryInstance{config: fssecurity.Config{AllowedPaths: []string{allowed}}}
	got := inst.findPaths(context.Background(), root, findOptions{recursive: true, entryType: "file"})
	// Every result must be inside root/sub.
	for _, p := range got {
		rel, _ := filepath.Rel(allowed, p)
		if strings.HasPrefix(rel, "..") {
			t.Errorf("result outside allowed dir: %s (rel %s)", p, rel)
		}
	}
	// Should contain sub/c.txt but not a.txt.
	foundC := false
	foundA := false
	for _, p := range got {
		if strings.HasSuffix(p, filepath.Join("sub", "c.txt")) {
			foundC = true
		}
		if strings.HasSuffix(p, filepath.Join(root, "a.txt")) {
			foundA = true
		}
	}
	if !foundC {
		t.Errorf("expected sub/c.txt in results: %v", got)
	}
	if foundA {
		t.Errorf("a.txt should be outside allowed dir: %v", got)
	}
}

func TestFindSecurityDeniesRootOutsideAllowed(t *testing.T) {
	root := buildFindTree(t)
	allowed := filepath.Join(root, "sub")
	inst := &findLibraryInstance{config: fssecurity.Config{AllowedPaths: []string{allowed}}}
	// Root itself is outside allowed paths — the builtin would error; the
	// internal call returns matches from within the walk but nothing outside
	// the allowed prefix. Since root != allowed, os.Stat(root) is fine but
	// entries are filtered. With root outside allowed, no entries survive.
	got := inst.findPaths(context.Background(), filepath.Join(root, "deep"), findOptions{recursive: true})
	for _, p := range got {
		rel, _ := filepath.Rel(allowed, p)
		if !strings.HasPrefix(rel, "..") {
			continue // fine — inside allowed
		}
		t.Errorf("result outside allowed dir: %s", p)
	}
}

// --- Symlink / follow_links tests ---

func TestFindFollowLinks(t *testing.T) {
	root := t.TempDir()
	// Create a real file in a sibling dir.
	sibling := t.TempDir()
	target := filepath.Join(sibling, "real.txt")
	os.WriteFile(target, []byte("x"), 0644)

	// Create a symlink inside root pointing to the real file.
	link := filepath.Join(root, "link.txt")
	if err := os.Symlink(target, link); err != nil {
		t.Skip("symlink not supported:", err)
	}
	os.WriteFile(filepath.Join(root, "direct.txt"), []byte("x"), 0644)

	inst := &findLibraryInstance{config: fssecurity.Config{AllowedPaths: nil}}

	// Without follow_links: symlink should be absent.
	got := inst.findPaths(context.Background(), root, findOptions{recursive: true, entryType: "file", name: "*.txt", followLinks: false})
	for _, p := range got {
		if p == link {
			t.Errorf("symlink should not appear without follow_links: %v", got)
		}
	}

	// With follow_links (unrestricted): symlink should appear.
	got = inst.findPaths(context.Background(), root, findOptions{recursive: true, entryType: "file", name: "*.txt", followLinks: true})
	foundLink := false
	for _, p := range got {
		if p == link {
			foundLink = true
		}
	}
	if !foundLink {
		t.Errorf("symlink should appear with follow_links: %v", got)
	}
}

func TestFindFollowLinksSkipsOutsideAllowed(t *testing.T) {
	root := t.TempDir()
	sibling := t.TempDir()
	target := filepath.Join(sibling, "real.txt")
	os.WriteFile(target, []byte("x"), 0644)

	link := filepath.Join(root, "link.txt")
	if err := os.Symlink(target, link); err != nil {
		t.Skip("symlink not supported:", err)
	}

	// Restrict to root only — the symlink target is in sibling (outside).
	inst := &findLibraryInstance{config: fssecurity.Config{AllowedPaths: []string{root}}}
	got := inst.findPaths(context.Background(), root, findOptions{recursive: true, entryType: "file", followLinks: true})
	for _, p := range got {
		if p == link {
			t.Errorf("symlink to outside allowed should be skipped: %v", got)
		}
	}
}

// --- parseFindKwargs tests ---

func TestParseFindKwargsDefaults(t *testing.T) {
	opts, errObj := parseFindKwargs(object.NewKwargs(nil))
	if errObj != nil {
		t.Fatalf("unexpected error: %v", errObj)
	}
	if !opts.recursive {
		t.Errorf("recursive should default to true")
	}
	if opts.entryType != "any" {
		t.Errorf("type should default to 'any', got %q", opts.entryType)
	}
	if opts.name != "" {
		t.Errorf("name should default to empty")
	}
	if opts.includeHidden || opts.followLinks {
		t.Errorf("include_hidden/follow_links should default to false")
	}
	if opts.maxDepth != 0 {
		t.Errorf("max_depth should default to 0 (unlimited)")
	}
	if opts.hasMtimeMin || opts.hasMtimeMax || opts.hasSizeMin || opts.hasSizeMax {
		t.Errorf("no filter flags should be set by default")
	}
}

func TestParseFindKwargsAllValues(t *testing.T) {
	kwargs := object.NewKwargs(map[string]object.Object{
		"recursive":      object.NewBoolean(false),
		"type":           object.NewString("dir"),
		"name":           object.NewString("*.py"),
		"include_hidden": object.NewBoolean(true),
		"follow_links":   object.NewBoolean(true),
		"max_depth":      object.NewInteger(3),
		"mtime_min":      object.NewFloat(1000.5),
		"mtime_max":      object.NewFloat(2000.5),
		"size_min":       object.NewInteger(10),
		"size_max":       object.NewInteger(100),
	})
	opts, errObj := parseFindKwargs(kwargs)
	if errObj != nil {
		t.Fatalf("unexpected error: %v", errObj)
	}
	if opts.recursive {
		t.Errorf("recursive should be false")
	}
	if opts.entryType != "dir" {
		t.Errorf("type should be 'dir'")
	}
	if opts.name != "*.py" {
		t.Errorf("name should be '*.py'")
	}
	if !opts.includeHidden || !opts.followLinks {
		t.Errorf("include_hidden and follow_links should be true")
	}
	if opts.maxDepth != 3 {
		t.Errorf("max_depth should be 3")
	}
	if !opts.hasMtimeMin || opts.mtimeMin != 1000.5 {
		t.Errorf("mtime_min not parsed correctly")
	}
	if !opts.hasMtimeMax || opts.mtimeMax != 2000.5 {
		t.Errorf("mtime_max not parsed correctly")
	}
	if !opts.hasSizeMin || opts.sizeMin != 10 {
		t.Errorf("size_min not parsed correctly")
	}
	if !opts.hasSizeMax || opts.sizeMax != 100 {
		t.Errorf("size_max not parsed correctly")
	}
}

func TestParseFindKwargsInvalidType(t *testing.T) {
	kwargs := object.NewKwargs(map[string]object.Object{
		"type": object.NewString("block"),
	})
	_, errObj := parseFindKwargs(kwargs)
	if errObj == nil {
		t.Errorf("expected error for invalid type 'block'")
	}
}

func TestParseFindKwargsWrongArgType(t *testing.T) {
	// name should be a string, not an integer.
	kwargs := object.NewKwargs(map[string]object.Object{
		"name": object.NewInteger(42),
	})
	_, errObj := parseFindKwargs(kwargs)
	if errObj == nil {
		t.Errorf("expected error for non-string name")
	}
}
