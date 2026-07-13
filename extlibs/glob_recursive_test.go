package extlibs

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/paularlott/scriptling/extlibs/fssecurity"
)

func TestGlobRecursiveAndHidden(t *testing.T) {
	dir := t.TempDir()

	// visible.txt
	// .hidden
	// sub/inner.py
	// sub/.hidden.py
	// .git/config
	// deep/a/b/c/deep.py
	mustWrite(t, filepath.Join(dir, "visible.txt"))
	mustWrite(t, filepath.Join(dir, ".hidden"))
	os.MkdirAll(filepath.Join(dir, "sub"), 0755)
	mustWrite(t, filepath.Join(dir, "sub", "inner.py"))
	mustWrite(t, filepath.Join(dir, "sub", ".hidden.py"))
	os.MkdirAll(filepath.Join(dir, ".git"), 0755)
	mustWrite(t, filepath.Join(dir, ".git", "config"))
	os.MkdirAll(filepath.Join(dir, "deep", "a", "b", "c"), 0755)
	mustWrite(t, filepath.Join(dir, "deep", "a", "b", "c", "deep.py"))

	config := fssecurity.Config{AllowedPaths: nil}

	cases := []struct {
		name          string
		pattern       string
		recursive     bool
		includeHidden bool
		want          []string
	}{
		{"nonrec collapse ** to * (matches one level)", "**/*.py", false, false, []string{
			filepath.Join("sub", "inner.py"),
		}},
		{"nonrec simple star default (dirs+files, no hidden)", "*", false, false, []string{
			"deep", "sub", "visible.txt",
		}},
		{"nonrec simple star hidden", "*", false, true, []string{
			".git", ".hidden", "deep", "sub", "visible.txt",
		}},
		{"rec py no hidden", "**/*.py", true, false, []string{
			filepath.Join("deep", "a", "b", "c", "deep.py"),
			filepath.Join("sub", "inner.py"),
		}},
		{"rec py hidden", "**/*.py", true, true, []string{
			filepath.Join("deep", "a", "b", "c", "deep.py"),
			filepath.Join("sub", ".hidden.py"),
			filepath.Join("sub", "inner.py"),
		}},
		{"rec star default", "**/*", true, false, []string{
			"visible.txt",
			filepath.Join("sub", "inner.py"),
			filepath.Join("deep", "a", "b", "c", "deep.py"),
			"sub",
			"deep",
			filepath.Join("deep", "a"),
			filepath.Join("deep", "a", "b"),
			filepath.Join("deep", "a", "b", "c"),
		}},
		{"rec star hidden (descends into .git)", "**/*", true, true, []string{
			".git",
			".hidden",
			"visible.txt",
			filepath.Join(".git", "config"),
			filepath.Join("sub", ".hidden.py"),
			filepath.Join("sub", "inner.py"),
			filepath.Join("deep", "a", "b", "c", "deep.py"),
			"sub",
			"deep",
			filepath.Join("deep", "a"),
			filepath.Join("deep", "a", "b"),
			filepath.Join("deep", "a", "b", "c"),
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			raw := globMatches(context.Background(), config, c.pattern, dir, c.recursive, c.includeHidden)
			rel := make([]string, 0, len(raw))
			for _, p := range raw {
				if p == dir {
					rel = append(rel, ".")
					continue
				}
				r, err := filepath.Rel(dir, p)
				if err != nil {
					t.Fatal(err)
				}
				rel = append(rel, r)
			}
			sort.Strings(rel)
			sort.Strings(c.want)
			if !equalSlices(rel, c.want) {
				t.Errorf("\n got %v\nwant %v", rel, c.want)
			}
		})
	}
}

func mustWrite(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	// treat nil and empty as equal
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestGlobRecursiveSecurityFiltersDisallowed(t *testing.T) {
	dir := t.TempDir()
	// A real file outside the allowed tree.
	outside := t.TempDir()
	mustWrite(t, filepath.Join(outside, "secret.txt"))

	// Inside dir: a visible file and a symlink that escapes outside.
	mustWrite(t, filepath.Join(dir, "visible.txt"))
	linkPath := filepath.Join(dir, "escape.txt")
	if err := os.Symlink(filepath.Join(outside, "secret.txt"), linkPath); err != nil {
		t.Skip("symlink not supported:", err)
	}

	// Restrict to dir — the symlink target resolves outside.
	config := fssecurity.Config{AllowedPaths: []string{dir}}

	// Non-recursive: IsPathAllowed evaluates the symlink and filters it.
	matches := globMatches(context.Background(), config, "*", dir, false, false)
	for _, m := range matches {
		if m == linkPath {
			t.Errorf("escaping symlink should be filtered (non-recursive): %v", matches)
		}
	}
	foundVisible := false
	for _, m := range matches {
		if strings.HasSuffix(m, "visible.txt") {
			foundVisible = true
		}
	}
	if !foundVisible {
		t.Errorf("visible.txt should be present: %v", matches)
	}

	// Recursive: same filtering applies during the parallel walk.
	matches = globMatches(context.Background(), config, "**/*", dir, true, false)
	for _, m := range matches {
		if m == linkPath {
			t.Errorf("escaping symlink should be filtered (recursive): %v", matches)
		}
	}
}

func TestGlobRecursiveMultiComponentSuffix(t *testing.T) {
	dir := t.TempDir()
	// dir/sub/deep/*.py  and  dir/sub/*.py
	os.MkdirAll(filepath.Join(dir, "sub", "deep"), 0755)
	mustWrite(t, filepath.Join(dir, "sub", "inner.py"))
	mustWrite(t, filepath.Join(dir, "sub", "deep", "nested.py"))

	config := fssecurity.Config{AllowedPaths: nil}
	// **/deep/*.py — suffix "deep/*.py" spans a path separator.
	matches := globMatches(context.Background(), config, "**/deep/*.py", dir, true, false)
	if len(matches) != 1 {
		t.Fatalf("expected 1 match for **/deep/*.py, got %d: %v", len(matches), matches)
	}
	if !strings.HasSuffix(matches[0], filepath.Join("sub", "deep", "nested.py")) {
		t.Errorf("expected nested.py, got %s", matches[0])
	}
}

func TestGlobRecursiveMultiStarStar(t *testing.T) {
	dir := t.TempDir()
	// Build a tree to exercise **/sub/**/*.py:
	//   sub/a.py             ✓ (zero dirs on each side)
	//   sub/deep/b.py        ✓ (one dir between sub and *.py)
	//   x/sub/c.py           ✓ (one dir before sub)
	//   x/y/sub/deep/e/d.py  ✓ (dirs on both sides)
	//   x/other/d.py         ✗ (no "sub" segment)
	//   sub/a.txt            ✗ (wrong extension)
	mustWrite(t, filepath.Join(dir, "sub", "a.py"))
	mustWrite(t, filepath.Join(dir, "sub", "deep", "b.py"))
	mustWrite(t, filepath.Join(dir, "x", "sub", "c.py"))
	mustWrite(t, filepath.Join(dir, "x", "y", "sub", "deep", "e", "d.py"))
	mustWrite(t, filepath.Join(dir, "x", "other", "d.py"))
	mustWrite(t, filepath.Join(dir, "sub", "a.txt"))

	config := fssecurity.Config{AllowedPaths: nil}
	matches := globMatches(context.Background(), config, "**/sub/**/*.py", dir, true, false)

	rel := make([]string, len(matches))
	for i, m := range matches {
		r, _ := filepath.Rel(dir, m)
		rel[i] = r
	}
	sort.Strings(rel)

	want := []string{
		filepath.Join("sub", "a.py"),
		filepath.Join("sub", "deep", "b.py"),
		filepath.Join("x", "sub", "c.py"),
		filepath.Join("x", "y", "sub", "deep", "e", "d.py"),
	}
	sort.Strings(want)

	if !equalSlices(rel, want) {
		t.Errorf("multi-** pattern **/sub/**/*.py:\n got %v\nwant %v", rel, want)
	}
}

func TestMatchGlobSegments(t *testing.T) {
	cases := []struct {
		pattern string
		path    string
		want    bool
	}{
		{"**/*.py", "a.py", true},
		{"**/*.py", "sub/a.py", true},
		{"**/*.py", "sub/deep/a.py", true},
		{"**/*.py", "a.txt", false},
		{"src/**/*.py", "src/a.py", true},
		{"src/**/*.py", "src/sub/a.py", true},
		{"src/**/*.py", "other/a.py", false},
		{"**/sub/**/*.py", "sub/a.py", true},
		{"**/sub/**/*.py", "a/sub/b.py", true},
		{"**/sub/**/*.py", "a/b/sub/c/d/e.py", true},
		{"**/sub/**/*.py", "a/other/b.py", false},
		{"**/sub/**/*.py", "sub/a.txt", false},
		{"*", "a", true},
		{"*", "a/b", false},
		{"**", "a", true},
		{"**", "a/b/c", true},
	}
	sep := string(filepath.Separator)
	for _, c := range cases {
		patSegs := strings.Split(c.pattern, "/")
		pathSegs := strings.Split(c.path, "/")
		_ = sep
		got := matchGlobSegments(patSegs, pathSegs)
		if got != c.want {
			t.Errorf("matchGlobSegments(%q, %q) = %v, want %v", c.pattern, c.path, got, c.want)
		}
	}
}
