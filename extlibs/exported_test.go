package extlibs

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// writeTempTree creates a temporary directory containing the given files
// (relative path -> content) and returns its root.
func writeTempTree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range files {
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestExportedGrepFile(t *testing.T) {
	dir := writeTempTree(t, map[string]string{
		"a.txt": "hello world\nfoo bar\nHELLO AGAIN\n",
	})
	path := filepath.Join(dir, "a.txt")

	t.Run("regex_default", func(t *testing.T) {
		m, err := Grep(context.Background(), "hello", path, GrepOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if len(m) != 1 {
			t.Fatalf("expected 1 match, got %d", len(m))
		}
		if m[0].Line != 1 || m[0].Text != "hello world" {
			t.Errorf("unexpected match: %+v", m[0])
		}
	})

	t.Run("ignore_case", func(t *testing.T) {
		m, err := Grep(context.Background(), "hello", path, GrepOptions{IgnoreCase: true})
		if err != nil {
			t.Fatal(err)
		}
		if len(m) != 2 {
			t.Fatalf("expected 2 case-insensitive matches, got %d", len(m))
		}
	})

	t.Run("literal_with_regex_chars", func(t *testing.T) {
		// Verify literal mode does not interpret regex metacharacters. The pattern
		// "ELL" has no metacharacters, but combined with ignore_case it must match
		// the "ell" inside both "hello" and "HELLO" without regex interpretation.
		m, err := Grep(context.Background(), "ELL", path, GrepOptions{Literal: true, IgnoreCase: true})
		if err != nil {
			t.Fatal(err)
		}
		if len(m) != 2 {
			t.Fatalf("expected 2 literal matches (both hello lines), got %d", len(m))
		}
		for _, match := range m {
			if !strings.Contains(strings.ToLower(match.Text), "ell") {
				t.Errorf("match line should contain 'ell': %+v", match)
			}
		}
	})

	t.Run("empty_pattern_errors", func(t *testing.T) {
		if _, err := Grep(context.Background(), "", path, GrepOptions{}); err == nil {
			t.Error("expected error for empty pattern")
		}
	})
}

func TestExportedGrepDirectory(t *testing.T) {
	dir := writeTempTree(t, map[string]string{
		"a.txt":      "hello world\nfoo\n",
		"b.py":       "def hello():\n    pass\n",
		"sub/c.txt":  "nested hello\n",
		"binary.bin": "data\x00binary\x00content",
	})

	t.Run("recursive_with_glob", func(t *testing.T) {
		m, err := Grep(context.Background(), "hello", dir, GrepOptions{Recursive: true, Glob: "*.py"})
		if err != nil {
			t.Fatal(err)
		}
		if len(m) != 1 {
			t.Fatalf("expected 1 match in *.py, got %d", len(m))
		}
		if filepath.Base(m[0].File) != "b.py" {
			t.Errorf("expected b.py, got %s", m[0].File)
		}
	})

	t.Run("non_recursive", func(t *testing.T) {
		m, err := Grep(context.Background(), "hello", dir, GrepOptions{Recursive: false})
		if err != nil {
			t.Fatal(err)
		}
		// sub/c.txt must not appear when non-recursive.
		for _, match := range m {
			if strings.Contains(match.File, "sub"+string(filepath.Separator)) {
				t.Errorf("non-recursive search descended into sub: %+v", match)
			}
		}
	})
}

func TestExportedGrepAllowedPaths(t *testing.T) {
	dir := writeTempTree(t, map[string]string{"a.txt": "hello\n"})
	outside := t.TempDir()
	outsideFile := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(outsideFile, []byte("hello\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := Grep(context.Background(), "hello", outsideFile, GrepOptions{AllowedPaths: []string{dir}}); !errors.Is(err, ErrPathNotAllowed) {
		t.Errorf("expected ErrPathNotAllowed, got %v", err)
	}

	// Inside is permitted.
	m, err := Grep(context.Background(), "hello", filepath.Join(dir, "a.txt"), GrepOptions{AllowedPaths: []string{dir}})
	if err != nil {
		t.Fatalf("expected allowed, got %v", err)
	}
	if len(m) != 1 {
		t.Errorf("expected 1 match, got %d", len(m))
	}
}

func TestExportedFind(t *testing.T) {
	dir := writeTempTree(t, map[string]string{
		"src/main.py":     "# main\n",
		"src/utils.py":    "# utils\n",
		"src/.hidden.py":  "# hidden\n",
		"tests/test_x.py": "# test\n",
		"docs/readme.md":  "# readme\n",
	})

	t.Run("default_recursive_glob", func(t *testing.T) {
		// Zero-value FindOptions preserves the scriptling default: recursive.
		got, err := Find(context.Background(), dir, FindOptions{Name: "*.py"})
		if err != nil {
			t.Fatal(err)
		}
		var bases []string
		for _, p := range got {
			bases = append(bases, filepath.Base(p))
		}
		sort.Strings(bases)
		want := []string{"main.py", "test_x.py", "utils.py"} // .hidden.py excluded by default
		if len(bases) != len(want) {
			t.Fatalf("expected %v, got %v", want, bases)
		}
		for i := range want {
			if bases[i] != want[i] {
				t.Errorf("expected %v, got %v", want, bases)
				break
			}
		}
	})

	t.Run("include_hidden", func(t *testing.T) {
		got, err := Find(context.Background(), dir, FindOptions{Name: "*.py", IncludeHidden: true})
		if err != nil {
			t.Fatal(err)
		}
		count := 0
		for _, p := range got {
			if filepath.Base(p) == ".hidden.py" {
				count++
			}
		}
		if count != 1 {
			t.Errorf("expected .hidden.py when include_hidden, got %d", count)
		}
	})

	t.Run("explicit_non_recursive", func(t *testing.T) {
		fals := false
		got, err := Find(context.Background(), dir, FindOptions{Recursive: &fals})
		if err != nil {
			t.Fatal(err)
		}
		// src/ and tests/ and docs/ are the immediate children — files inside
		// subdirectories must not appear.
		for _, p := range got {
			rel, _ := filepath.Rel(dir, p)
			if strings.Contains(rel, string(filepath.Separator)) {
				t.Errorf("non-recursive find returned nested path: %s", rel)
			}
		}
	})

	t.Run("type_filter", func(t *testing.T) {
		got, err := Find(context.Background(), dir, FindOptions{Type: "dir"})
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range got {
			if info, err := os.Stat(p); err != nil || !info.IsDir() {
				t.Errorf("type=dir returned non-directory: %s", p)
			}
		}
	})

	t.Run("invalid_type", func(t *testing.T) {
		if _, err := Find(context.Background(), dir, FindOptions{Type: "nope"}); err == nil {
			t.Error("expected error for invalid type")
		}
	})
}

func TestExportedSedReplace(t *testing.T) {
	dir := writeTempTree(t, map[string]string{
		"a.txt": "hello world\nhello again\nfoo.bar() call\n",
	})
	path := filepath.Join(dir, "a.txt")

	t.Run("literal", func(t *testing.T) {
		n, err := SedReplace(context.Background(), "hello", "hi", path, SedOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Fatalf("expected 1 file modified, got %d", n)
		}
		got, _ := os.ReadFile(path)
		if strings.Contains(string(got), "hello") {
			t.Errorf("literal replace left a 'hello': %s", got)
		}
	})

	t.Run("literal_with_regex_chars", func(t *testing.T) {
		// "foo.bar()" must match the literal text, not regex "foo<anything>bar()".
		n, err := SedReplace(context.Background(), "foo.bar()", "baz.qux()", path, SedOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Fatalf("expected 1 file modified, got %d", n)
		}
		got, _ := os.ReadFile(path)
		if !strings.Contains(string(got), "baz.qux()") {
			t.Errorf("expected baz.qux() in result, got %s", got)
		}
	})
}

func TestExportedSedReplacePattern(t *testing.T) {
	dir := writeTempTree(t, map[string]string{
		"a.py": "def get_user(id):\n    pass\ndef get_order(id):\n    pass\n",
	})
	path := filepath.Join(dir, "a.py")

	n, err := SedReplacePattern(context.Background(), `def get_(\w+)\(`, `def fetch_${1}(`, path, SedOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expected 1 file modified, got %d", n)
	}
	got, _ := os.ReadFile(path)
	if strings.Contains(string(got), "def get_") {
		t.Errorf("pattern replace left a get_ def: %s", got)
	}
	if !strings.Contains(string(got), "def fetch_user") || !strings.Contains(string(got), "def fetch_order") {
		t.Errorf("expected fetch_user/fetch_order in result: %s", got)
	}
}

func TestExportedSedReplaceDirectory(t *testing.T) {
	dir := writeTempTree(t, map[string]string{
		"a.txt": "TODO: a\n",
		"b.txt": "todo: b\n",
		"c.md":  "TODO: c\n",
	})
	n, err := SedReplace(context.Background(), "todo:", "DONE:", dir, SedOptions{Recursive: true, IgnoreCase: true, Glob: "*.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("expected 2 .txt files modified, got %d", n)
	}
	c, _ := os.ReadFile(filepath.Join(dir, "c.md"))
	if !strings.Contains(string(c), "TODO") {
		t.Error("glob should have excluded c.md from modification")
	}
}

func TestExportedSedExtract(t *testing.T) {
	dir := writeTempTree(t, map[string]string{
		"a.py": "def get_user(id):\n    pass\ndef get_order(id):\n    pass\ndef set_value(x):\n    pass\n",
	})
	path := filepath.Join(dir, "a.py")

	m, err := SedExtract(context.Background(), `def (\w+)\(`, path, SedOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 3 {
		t.Fatalf("expected 3 matches, got %d", len(m))
	}
	names := []string{m[0].Groups[0], m[1].Groups[0], m[2].Groups[0]}
	want := []string{"get_user", "get_order", "set_value"}
	for i := range want {
		if names[i] != want[i] {
			t.Errorf("match %d: want group %q, got %q", i, want[i], names[i])
		}
	}

	t.Run("two_capture_groups", func(t *testing.T) {
		cfg := writeTempTree(t, map[string]string{
			"conf.txt": "host=localhost\nport=8080\nuser=admin\n",
		})
		got, err := SedExtract(context.Background(), `(\w+)=(\S+)`, filepath.Join(cfg, "conf.txt"), SedOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 3 {
			t.Fatalf("expected 3 matches, got %d", len(got))
		}
		if got[0].Groups[0] != "host" || got[0].Groups[1] != "localhost" {
			t.Errorf("first match groups unexpected: %+v", got[0].Groups)
		}
	})
}

func TestExportedSedAllowedPaths(t *testing.T) {
	inside := writeTempTree(t, map[string]string{"a.txt": "hello\n"})
	outside := t.TempDir()
	outsideFile := filepath.Join(outside, "x.txt")
	if err := os.WriteFile(outsideFile, []byte("hello\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := SedReplace(context.Background(), "hello", "hi", outsideFile, SedOptions{AllowedPaths: []string{inside}}); !errors.Is(err, ErrPathNotAllowed) {
		t.Errorf("expected ErrPathNotAllowed, got %v", err)
	}
	if _, err := SedExtract(context.Background(), "h", outsideFile, SedOptions{AllowedPaths: []string{inside}}); !errors.Is(err, ErrPathNotAllowed) {
		t.Errorf("expected ErrPathNotAllowed, got %v", err)
	}
}

func TestExportedNilContext(t *testing.T) {
	dir := writeTempTree(t, map[string]string{"a.txt": "hello\n"})
	// Passing a nil context must not panic — RunBlocking falls back to running
	// the closure directly when no env is present on the context.
	m, err := Grep(nil, "hello", filepath.Join(dir, "a.txt"), GrepOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 1 {
		t.Errorf("expected 1 match with nil context, got %d", len(m))
	}
}

func TestExportedEditFile(t *testing.T) {
	t.Run("unique_match_replaced", func(t *testing.T) {
		dir := writeTempTree(t, map[string]string{
			"a.py": "def get_user(id):\n    pass\ndef get_order(id):\n    pass\n",
		})
		path := filepath.Join(dir, "a.py")
		search := "def get_user(id):\n    pass"
		replace := "def fetch_user(user_id):\n    return None"

		n, err := EditFile(context.Background(), path, search, replace)
		if err != nil {
			t.Fatal(err)
		}
		if n == 0 {
			t.Error("expected non-zero bytes written")
		}

		got, _ := os.ReadFile(path)
		s := string(got)
		if !strings.Contains(s, "def fetch_user(user_id):") {
			t.Errorf("expected fetch_user in result: %s", s)
		}
		if strings.Contains(s, "def get_user") {
			t.Errorf("get_user should have been replaced: %s", s)
		}
		// The other function should be untouched.
		if !strings.Contains(s, "def get_order(id):") {
			t.Errorf("get_order should be untouched: %s", s)
		}
	})

	t.Run("not_found_errors", func(t *testing.T) {
		dir := writeTempTree(t, map[string]string{"a.txt": "hello world\n"})
		_, err := EditFile(context.Background(), filepath.Join(dir, "a.txt"), "nonexistent", "x")
		if !errors.Is(err, ErrSearchNotFound) {
			t.Errorf("expected ErrSearchNotFound, got %v", err)
		}
	})

	t.Run("multiple_matches_error", func(t *testing.T) {
		dir := writeTempTree(t, map[string]string{
			"a.txt": "return None\n    return None\n",
		})
		_, err := EditFile(context.Background(), filepath.Join(dir, "a.txt"), "return None", "return 42")
		if !errors.Is(err, ErrSearchNotUnique) {
			t.Errorf("expected ErrSearchNotUnique, got %v", err)
		}
		// File must not have been modified.
		got, _ := os.ReadFile(filepath.Join(dir, "a.txt"))
		if strings.Contains(string(got), "return 42") {
			t.Error("file should not have been modified on ambiguous match")
		}
	})

	t.Run("empty_search_errors", func(t *testing.T) {
		dir := writeTempTree(t, map[string]string{"a.txt": "hello\n"})
		if _, err := EditFile(context.Background(), filepath.Join(dir, "a.txt"), "", "x"); err == nil {
			t.Error("expected error for empty search text")
		}
	})

	t.Run("preserves_file_permissions", func(t *testing.T) {
		dir := writeTempTree(t, map[string]string{"a.sh": "#!/bin/bash\necho hello\n"})
		path := filepath.Join(dir, "a.sh")
		os.Chmod(path, 0755)

		if _, err := EditFile(context.Background(), path, "echo hello", "echo world"); err != nil {
			t.Fatal(err)
		}
		info, _ := os.Stat(path)
		if info.Mode().Perm() != 0755 {
			t.Errorf("expected 0755, got %v", info.Mode().Perm())
		}
	})
}
