package extlibs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/object"
)

// setupGrepTestDir creates a temporary directory with test files.
func setupGrepTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	files := map[string]string{
		"a.txt":      "hello world\nfoo bar\nHELLO AGAIN\n",
		"b.py":       "def hello():\n    pass\n# TODO: fix this\n",
		"sub/c.txt":  "nested hello\nnested foo\n",
		"sub/d.go":   "package main\n// hello from go\n",
		"binary.bin": "data\x00binary\x00content",
		"empty.txt":  "",
	}

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

func newGrepInterpreter(t *testing.T, allowedPaths []string) *scriptling.Scriptling {
	t.Helper()
	p := scriptling.New()
	RegisterGrepLibrary(p, allowedPaths)
	return p
}

func TestGrepPatternFileBasic(t *testing.T) {
	dir := setupGrepTestDir(t)
	p := newGrepInterpreter(t, nil)

	result, err := p.Eval(`import scriptling.grep as grep
grep.pattern("hello", "` + filepath.Join(dir, "a.txt") + `")`)
	if err != nil {
		t.Fatal(err)
	}

	list, ok := result.(*object.List)
	if !ok {
		t.Fatalf("expected List, got %T", result)
	}
	if len(list.Elements) != 1 {
		t.Fatalf("expected 1 match, got %d", len(list.Elements))
	}
	d := list.Elements[0].(*object.Dict)
	lineVal, _ := d.GetByString("line")
	if lineVal.Value.(*object.Integer).Value != 1 {
		t.Errorf("expected line 1, got %v", lineVal.Value)
	}
}

func TestGrepPatternIgnoreCase(t *testing.T) {
	dir := setupGrepTestDir(t)
	p := newGrepInterpreter(t, nil)

	result, err := p.Eval(`import scriptling.grep as grep
grep.pattern("hello", "` + filepath.Join(dir, "a.txt") + `", ignore_case=True)`)
	if err != nil {
		t.Fatal(err)
	}

	list := result.(*object.List)
	if len(list.Elements) != 2 {
		t.Errorf("expected 2 case-insensitive matches, got %d", len(list.Elements))
	}
}

func TestGrepPatternRegexDotMatchesAny(t *testing.T) {
	dir := setupGrepTestDir(t)
	p := newGrepInterpreter(t, nil)

	// "foo.bar" as regex: . matches any char, so matches "foo bar"
	result, err := p.Eval(`import scriptling.grep as grep
grep.pattern("foo.bar", "` + filepath.Join(dir, "a.txt") + `")`)
	if err != nil {
		t.Fatal(err)
	}
	list := result.(*object.List)
	if len(list.Elements) != 1 {
		t.Errorf("expected 1 regex match for foo.bar, got %d", len(list.Elements))
	}
}

func TestGrepStringLiteralDotNotRegex(t *testing.T) {
	dir := setupGrepTestDir(t)
	p := newGrepInterpreter(t, nil)

	// "foo.bar" as literal: should NOT match "foo bar"
	result, err := p.Eval(`import scriptling.grep as grep
grep.string("foo.bar", "` + filepath.Join(dir, "a.txt") + `")`)
	if err != nil {
		t.Fatal(err)
	}
	list := result.(*object.List)
	if len(list.Elements) != 0 {
		t.Errorf("expected 0 literal matches for foo.bar, got %d", len(list.Elements))
	}
}

func TestGrepStringLiteralMatch(t *testing.T) {
	dir := setupGrepTestDir(t)
	p := newGrepInterpreter(t, nil)

	// "foo bar" as literal: should match exactly
	result, err := p.Eval(`import scriptling.grep as grep
grep.string("foo bar", "` + filepath.Join(dir, "a.txt") + `")`)
	if err != nil {
		t.Fatal(err)
	}
	list := result.(*object.List)
	if len(list.Elements) != 1 {
		t.Errorf("expected 1 literal match for 'foo bar', got %d", len(list.Elements))
	}
}

func TestGrepStringIgnoreCase(t *testing.T) {
	dir := setupGrepTestDir(t)
	p := newGrepInterpreter(t, nil)

	result, err := p.Eval(`import scriptling.grep as grep
grep.string("hello", "` + filepath.Join(dir, "a.txt") + `", ignore_case=True)`)
	if err != nil {
		t.Fatal(err)
	}
	list := result.(*object.List)
	if len(list.Elements) != 2 {
		t.Errorf("expected 2 case-insensitive literal matches, got %d", len(list.Elements))
	}
}

func TestGrepPatternDirNonRecursive(t *testing.T) {
	dir := setupGrepTestDir(t)
	p := newGrepInterpreter(t, nil)

	result, err := p.Eval(`import scriptling.grep as grep
grep.pattern("hello", "` + dir + `")`)
	if err != nil {
		t.Fatal(err)
	}

	list := result.(*object.List)
	// Only top-level files: a.txt (1 match), b.py (1 match) — sub/ skipped
	if len(list.Elements) != 2 {
		t.Errorf("expected 2 non-recursive matches, got %d", len(list.Elements))
	}
}

func TestGrepPatternDirRecursive(t *testing.T) {
	dir := setupGrepTestDir(t)
	p := newGrepInterpreter(t, nil)

	result, err := p.Eval(`import scriptling.grep as grep
grep.pattern("hello", "` + dir + `", recursive=True)`)
	if err != nil {
		t.Fatal(err)
	}

	list := result.(*object.List)
	// a.txt:1, b.py:1, sub/c.txt:1, sub/d.go:1 = 4 matches
	if len(list.Elements) != 4 {
		t.Errorf("expected 4 recursive matches, got %d", len(list.Elements))
	}
}

func TestGrepStringDirRecursive(t *testing.T) {
	dir := setupGrepTestDir(t)
	p := newGrepInterpreter(t, nil)

	result, err := p.Eval(`import scriptling.grep as grep
grep.string("hello", "` + dir + `", recursive=True)`)
	if err != nil {
		t.Fatal(err)
	}

	list := result.(*object.List)
	if len(list.Elements) != 4 {
		t.Errorf("expected 4 recursive literal matches, got %d", len(list.Elements))
	}
}

func TestGrepPatternGlob(t *testing.T) {
	dir := setupGrepTestDir(t)
	p := newGrepInterpreter(t, nil)

	result, err := p.Eval(`import scriptling.grep as grep
grep.pattern("hello", "` + dir + `", recursive=True, glob="*.py")`)
	if err != nil {
		t.Fatal(err)
	}

	list := result.(*object.List)
	if len(list.Elements) != 1 {
		t.Errorf("expected 1 .py match, got %d", len(list.Elements))
	}
}

func TestGrepSkipsBinaryFiles(t *testing.T) {
	dir := setupGrepTestDir(t)
	p := newGrepInterpreter(t, nil)

	result, err := p.Eval(`import scriptling.grep as grep
grep.string("binary", "` + filepath.Join(dir, "binary.bin") + `")`)
	if err != nil {
		t.Fatal(err)
	}

	list := result.(*object.List)
	if len(list.Elements) != 0 {
		t.Errorf("expected binary file to be skipped, got %d matches", len(list.Elements))
	}
}

func TestGrepMaxSize(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "big.txt")
	if err := os.WriteFile(path, []byte("hello world this is a long line\n"), 0644); err != nil {
		t.Fatal(err)
	}

	p := newGrepInterpreter(t, nil)
	result, err := p.Eval(`import scriptling.grep as grep
grep.string("hello", "` + path + `", max_size=10)`)
	if err != nil {
		t.Fatal(err)
	}

	list := result.(*object.List)
	if len(list.Elements) != 0 {
		t.Errorf("expected file to be skipped due to max_size, got %d matches", len(list.Elements))
	}
}

func TestGrepMaxSizeNone(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "big.txt")
	if err := os.WriteFile(path, []byte("hello world\n"), 0644); err != nil {
		t.Fatal(err)
	}

	p := newGrepInterpreter(t, nil)
	result, err := p.Eval(`import scriptling.grep as grep
grep.string("hello", "` + path + `", max_size=None)`)
	if err != nil {
		t.Fatal(err)
	}

	list := result.(*object.List)
	if len(list.Elements) != 1 {
		t.Errorf("expected 1 match with no size limit, got %d", len(list.Elements))
	}
}

func TestGrepPathRestriction(t *testing.T) {
	dir := setupGrepTestDir(t)
	otherDir := t.TempDir()

	p := newGrepInterpreter(t, []string{otherDir})

	_, err := p.Eval(`import scriptling.grep as grep
grep.pattern("hello", "` + dir + `")`)
	if err == nil {
		t.Error("expected permission error for restricted path, got nil")
	}
}

func TestGrepPathRestrictionAllowed(t *testing.T) {
	dir := setupGrepTestDir(t)
	p := newGrepInterpreter(t, []string{dir})

	result, err := p.Eval(`import scriptling.grep as grep
grep.pattern("hello", "` + dir + `")`)
	if err != nil {
		t.Fatalf("expected success within allowed path, got: %v", err)
	}

	list := result.(*object.List)
	if len(list.Elements) == 0 {
		t.Error("expected matches within allowed path")
	}
}

func TestGrepMatchDictShape(t *testing.T) {
	dir := setupGrepTestDir(t)
	p := newGrepInterpreter(t, nil)

	result, err := p.Eval(`import scriptling.grep as grep
matches = grep.string("hello", "` + filepath.Join(dir, "a.txt") + `")
m = matches[0]
[m["file"], m["line"], m["text"]]`)
	if err != nil {
		t.Fatal(err)
	}

	list := result.(*object.List)
	if len(list.Elements) != 3 {
		t.Fatalf("expected 3 elements in result list, got %d", len(list.Elements))
	}
	if _, ok := list.Elements[0].(*object.String); !ok {
		t.Error("file should be a string")
	}
	if _, ok := list.Elements[1].(*object.Integer); !ok {
		t.Error("line should be an integer")
	}
	if _, ok := list.Elements[2].(*object.String); !ok {
		t.Error("text should be a string")
	}
}
