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

// --- FindEntries (rich) tests ------------------------------------------------

func runFindEntries(t *testing.T, root string, opts FindOptions) []FindEntry {
	t.Helper()
	got, err := FindEntries(context.Background(), root, opts)
	if err != nil {
		t.Fatalf("FindEntries error: %v", err)
	}
	return got
}

func TestFindEntriesReturnsMetadata(t *testing.T) {
	root := buildFindTree(t)
	got := runFindEntries(t, root, FindOptions{Recursive: ptrBool(true), Type: "file", Name: "big.bin"})
	if len(got) != 1 {
		t.Fatalf("expected 1 entry, got %d: %+v", len(got), got)
	}
	e := got[0]
	if filepath.Base(e.Path) != "big.bin" {
		t.Errorf("path: got %q, want big.bin", e.Path)
	}
	if e.Size != 5000 {
		t.Errorf("size: got %d, want 5000", e.Size)
	}
	if e.IsDir {
		t.Errorf("is_dir: got true, want false")
	}
	// mtime should be ~48h ago, within a generous tolerance.
	age := time.Since(e.Mtime).Hours()
	if age < 47 || age > 49 {
		t.Errorf("mtime: got age %.1fh, want ~48h", age)
	}
}

func TestFindEntriesMarksDirectories(t *testing.T) {
	root := buildFindTree(t)
	got := runFindEntries(t, root, FindOptions{Recursive: ptrBool(true), Type: "dir"})
	paths := make(map[string]bool)
	for _, e := range got {
		if !e.IsDir {
			t.Errorf("type=dir returned non-dir entry: %+v", e)
		}
		paths[filepath.Base(e.Path)] = true
	}
	if !paths["sub"] || !paths["deep"] {
		t.Errorf("expected sub and deep dirs, got %v", paths)
	}
}

func TestFindEntriesRootIsSingleFile(t *testing.T) {
	root := buildFindTree(t)
	target := filepath.Join(root, "a.txt")
	got := runFindEntries(t, target, FindOptions{Recursive: ptrBool(true), Type: "any", Name: "*.txt"})
	if len(got) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(got))
	}
	if got[0].Path != target {
		t.Errorf("path: got %q, want %q", got[0].Path, target)
	}
	if got[0].Size != 5 { // "small"
		t.Errorf("size: got %d, want 5", got[0].Size)
	}
}

func TestFindEntriesMatchesFindPathSet(t *testing.T) {
	// Find and FindEntries should agree on which paths match.
	root := buildFindTree(t)
	opts := FindOptions{Recursive: ptrBool(true), Type: "any"}

	rich, err := FindEntries(context.Background(), root, opts)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := Find(context.Background(), root, opts)
	if err != nil {
		t.Fatal(err)
	}

	richPaths := make([]string, len(rich))
	for i, e := range rich {
		richPaths[i] = e.Path
	}
	sort.Strings(richPaths)
	sort.Strings(plain)

	if !equalStringSlices(richPaths, plain) {
		t.Errorf("Find/FindEntries disagree:\n rich %v\n plain %v", richPaths, plain)
	}
}

func TestFindEntriesRejectsBadType(t *testing.T) {
	root := buildFindTree(t)
	_, err := FindEntries(context.Background(), root, FindOptions{Type: "block"})
	if err == nil {
		t.Fatalf("expected error for bad type")
	}
}

func TestFindEntriesAppliesSizeFilter(t *testing.T) {
	root := buildFindTree(t)
	min := int64(1000)
	got := runFindEntries(t, root, FindOptions{Recursive: ptrBool(true), Type: "file", SizeMin: &min})
	for _, e := range got {
		if e.Size < 1000 {
			t.Errorf("size_min leak: %+v", e)
		}
	}
	// Only big.bin should survive.
	if len(got) != 1 || filepath.Base(got[0].Path) != "big.bin" {
		t.Errorf("expected just big.bin, got %d entries", len(got))
	}
}

func ptrBool(b bool) *bool { return &b }

// --- find.entries builtin tests ----------------------------------------------

// callEntriesBuiltin invokes the registered "entries" builtin with one
// positional arg (the path) and the given kwargs, returning the result list.
func callEntriesBuiltin(t *testing.T, searchPath string, kwargs map[string]object.Object) *object.List {
	t.Helper()
	inst := &findLibraryInstance{config: fssecurity.Config{AllowedPaths: nil}}
	lib := inst.createLibrary()
	entriesBuiltin := lib.Functions()["entries"]
	if entriesBuiltin == nil {
		t.Fatal("entries builtin not registered in library")
	}
	res := entriesBuiltin.Fn(context.Background(), object.NewKwargs(kwargs), object.NewString(searchPath))
	if err, ok := res.(*object.Error); ok {
		t.Fatalf("entries builtin returned error: %s", err.Inspect())
	}
	list, ok := res.(*object.List)
	if !ok {
		t.Fatalf("entries builtin returned %T, want *List", res)
	}
	return list
}

func TestFindEntriesBuiltinReturnsListOfDicts(t *testing.T) {
	root := buildFindTree(t)
	got := callEntriesBuiltin(t, root, map[string]object.Object{
		"type": object.NewString("file"),
		"name": object.NewString("big.bin"),
	})
	if len(got.Elements) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(got.Elements))
	}
	d, ok := got.Elements[0].(*object.Dict)
	if !ok {
		t.Fatalf("entry is %T, want *Dict", got.Elements[0])
	}
	for _, key := range []string{"path", "size", "mtime", "is_dir"} {
		if !d.HasByString(key) {
			t.Errorf("entry dict missing key %q: %s", key, d.Inspect())
		}
	}
	sizePair, _ := d.GetByString("size")
	if sizeInt, _ := sizePair.Value.AsInt(); sizeInt != 5000 {
		t.Errorf("size: got %d, want 5000", sizeInt)
	}
	isDirPair, _ := d.GetByString("is_dir")
	if dir, _ := isDirPair.Value.AsBool(); dir {
		t.Errorf("is_dir: got true, want false for big.bin")
	}
}

func TestFindEntriesBuiltinMarksDirectories(t *testing.T) {
	root := buildFindTree(t)
	got := callEntriesBuiltin(t, root, map[string]object.Object{
		"type": object.NewString("dir"),
	})
	if len(got.Elements) == 0 {
		t.Fatal("expected at least one directory entry")
	}
	for _, el := range got.Elements {
		d, ok := el.(*object.Dict)
		if !ok {
			t.Fatalf("entry is %T, want *Dict", el)
		}
		isDirPair, _ := d.GetByString("is_dir")
		dir, _ := isDirPair.Value.AsBool()
		if !dir {
			t.Errorf("type=dir returned non-dir entry: %s", d.Inspect())
		}
	}
}

func TestFindEntriesBuiltinMtimeIsFloatSeconds(t *testing.T) {
	root := buildFindTree(t)
	got := callEntriesBuiltin(t, root, map[string]object.Object{
		"type": object.NewString("file"),
		"name": object.NewString("big.bin"),
	})
	d := got.Elements[0].(*object.Dict)
	mtimePair, _ := d.GetByString("mtime")
	mtime, err := mtimePair.Value.AsFloat()
	if err != nil {
		t.Fatalf("mtime not a float: %v", err)
	}
	// big.bin was written ~48h ago. The float should be a recent epoch seconds
	// value (~1.7e9) and correspond to ~48h back, within a generous tolerance.
	if mtime < 1e9 {
		t.Errorf("mtime %v does not look like epoch seconds", mtime)
	}
	ageHours := time.Since(time.Unix(int64(mtime), 0)).Hours()
	if ageHours < 47 || ageHours > 49 {
		t.Errorf("mtime age: got %.1fh, want ~48h", ageHours)
	}
}

func TestFindEntriesBuiltinMatchesPathBuiltin(t *testing.T) {
	root := buildFindTree(t)
	inst := &findLibraryInstance{config: fssecurity.Config{AllowedPaths: nil}}
	lib := inst.createLibrary()
	pathBuiltin := lib.Functions()["path"]
	entriesBuiltin := lib.Functions()["entries"]

	kwargsAny := map[string]object.Object{"type": object.NewString("any")}
	pathRes := pathBuiltin.Fn(context.Background(), object.NewKwargs(kwargsAny), object.NewString(root))
	entriesRes := entriesBuiltin.Fn(context.Background(), object.NewKwargs(kwargsAny), object.NewString(root))

	pathList := pathRes.(*object.List)
	entriesList := entriesRes.(*object.List)

	if len(pathList.Elements) != len(entriesList.Elements) {
		t.Fatalf("path/entries disagree on count: path=%d entries=%d",
			len(pathList.Elements), len(entriesList.Elements))
	}

	pathSet := make(map[string]bool, len(pathList.Elements))
	for _, el := range pathList.Elements {
		s, _ := el.AsString()
		pathSet[s] = true
	}
	for _, el := range entriesList.Elements {
		d := el.(*object.Dict)
		pathPair, _ := d.GetByString("path")
		s, _ := pathPair.Value.AsString()
		if !pathSet[s] {
			t.Errorf("entries returned path not in path() result: %s", s)
		}
	}
}

func TestFindEntriesBuiltinRejectsBadType(t *testing.T) {
	root := buildFindTree(t)
	inst := &findLibraryInstance{config: fssecurity.Config{AllowedPaths: nil}}
	entriesBuiltin := inst.createLibrary().Functions()["entries"]

	res := entriesBuiltin.Fn(context.Background(),
		object.NewKwargs(map[string]object.Object{"type": object.NewString("block")}),
		object.NewString(root))
	if _, ok := res.(*object.Error); !ok {
		t.Errorf("expected Error for bad type, got %T: %s", res, res.Inspect())
	}
}

func TestFindEntriesBuiltinPermissionDenied(t *testing.T) {
	root := buildFindTree(t)
	allowed := filepath.Join(root, "sub")
	inst := &findLibraryInstance{config: fssecurity.Config{AllowedPaths: []string{allowed}}}
	entriesBuiltin := inst.createLibrary().Functions()["entries"]

	// Search from root which is outside the allowed dir.
	res := entriesBuiltin.Fn(context.Background(), object.NewKwargs(nil), object.NewString(root))
	// NewPermissionError returns *Exception; either Error or Exception is fine.
	inspect := res.Inspect()
	if !strings.Contains(inspect, "access denied") {
		t.Errorf("expected 'access denied' in error, got %T: %s", res, inspect)
	}
}

func TestFindEntriesBuiltinRootIsSingleFile(t *testing.T) {
	root := buildFindTree(t)
	target := filepath.Join(root, "a.txt")
	got := callEntriesBuiltin(t, target, map[string]object.Object{
		"type": object.NewString("any"),
		"name": object.NewString("*.txt"),
	})
	if len(got.Elements) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(got.Elements))
	}
	d := got.Elements[0].(*object.Dict)
	pathPair, _ := d.GetByString("path")
	if s, _ := pathPair.Value.AsString(); s != target {
		t.Errorf("path: got %q, want %q", s, target)
	}
	sizePair, _ := d.GetByString("size")
	if size, _ := sizePair.Value.AsInt(); size != 5 { // "small"
		t.Errorf("size: got %d, want 5", size)
	}
}

func TestFindEntryToDictShape(t *testing.T) {
	mtime := time.Unix(1700000000, 0)
	e := FindEntry{Path: "/x/y.txt", Size: 42, Mtime: mtime, IsDir: false}
	d := findEntryToDict(e)

	pathPair, _ := d.GetByString("path")
	if s, _ := pathPair.Value.AsString(); s != "/x/y.txt" {
		t.Errorf("path: got %q", s)
	}
	sizePair, _ := d.GetByString("size")
	if size, _ := sizePair.Value.AsInt(); size != 42 {
		t.Errorf("size: got %d", size)
	}
	mtimePair, _ := d.GetByString("mtime")
	if m, _ := mtimePair.Value.AsFloat(); m != 1700000000.0 {
		t.Errorf("mtime: got %v, want 1700000000.0", m)
	}
	isDirPair, _ := d.GetByString("is_dir")
	if b, _ := isDirPair.Value.AsBool(); b {
		t.Errorf("is_dir: got true, want false")
	}
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
