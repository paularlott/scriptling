package extlibs

import (
	"context"
	"os"
	"path/filepath"
	"sort"
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
