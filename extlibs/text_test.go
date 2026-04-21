package extlibs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/object"
)

func newTextInterpreter(t *testing.T, allowedPaths []string) *scriptling.Scriptling {
	t.Helper()
	p := scriptling.New()
	RegisterTextLibrary(p, allowedPaths)
	return p
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestTextReplaceFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	writeFile(t, path, "hello world\nfoo bar\n")

	p := newTextInterpreter(t, nil)
	result, err := p.Eval(`import scriptling.text as text
text.replace("world", "earth", "` + path + `")`)
	if err != nil {
		t.Fatal(err)
	}

	if result.(*object.Integer).Value != 1 {
		t.Errorf("expected 1 file modified, got %v", result.(*object.Integer).Value)
	}
	got := readFile(t, path)
	if got != "hello earth\nfoo bar\n" {
		t.Errorf("unexpected content: %q", got)
	}
}

func TestTextReplaceFileNoMatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	writeFile(t, path, "hello world\n")

	p := newTextInterpreter(t, nil)
	result, err := p.Eval(`import scriptling.text as text
text.replace("notfound", "x", "` + path + `")`)
	if err != nil {
		t.Fatal(err)
	}
	if result.(*object.Integer).Value != 0 {
		t.Errorf("expected 0 files modified, got %v", result.(*object.Integer).Value)
	}
	if got := readFile(t, path); got != "hello world\n" {
		t.Errorf("file should be unchanged, got %q", got)
	}
}

func TestTextReplaceLiteralNotRegex(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	writeFile(t, path, "foo.bar\nfoo bar\n")

	p := newTextInterpreter(t, nil)
	_, err := p.Eval(`import scriptling.text as text
text.replace("foo.bar", "replaced", "` + path + `")`)
	if err != nil {
		t.Fatal(err)
	}
	got := readFile(t, path)
	if got != "replaced\nfoo bar\n" {
		t.Errorf("unexpected content: %q", got)
	}
}

func TestTextReplaceIgnoreCase(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	writeFile(t, path, "Hello World\nhello world\n")

	p := newTextInterpreter(t, nil)
	result, err := p.Eval(`import scriptling.text as text
text.replace("hello", "hi", "` + path + `", ignore_case=True)`)
	if err != nil {
		t.Fatal(err)
	}
	if result.(*object.Integer).Value != 1 {
		t.Errorf("expected 1 file modified")
	}
	got := readFile(t, path)
	if got != "hi World\nhi world\n" {
		t.Errorf("unexpected content: %q", got)
	}
}

func TestTextReplacePatternFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.py")
	writeFile(t, path, "def old_func(x):\n    pass\n")

	p := newTextInterpreter(t, nil)
	result, err := p.Eval(`import scriptling.text as text
text.replace_pattern("old_(\\w+)", "new_${1}", "` + path + `")`)
	if err != nil {
		t.Fatal(err)
	}
	if result.(*object.Integer).Value != 1 {
		t.Errorf("expected 1 file modified")
	}
	got := readFile(t, path)
	if got != "def new_func(x):\n    pass\n" {
		t.Errorf("unexpected content: %q", got)
	}
}

func TestTextReplaceDirNonRecursive(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"), "hello world\n")
	writeFile(t, filepath.Join(dir, "b.txt"), "hello world\n")
	writeFile(t, filepath.Join(dir, "sub/c.txt"), "hello world\n")

	p := newTextInterpreter(t, nil)
	result, err := p.Eval(`import scriptling.text as text
text.replace("hello", "hi", "` + dir + `")`)
	if err != nil {
		t.Fatal(err)
	}
	if result.(*object.Integer).Value != 2 {
		t.Errorf("expected 2 files modified, got %v", result.(*object.Integer).Value)
	}
	if got := readFile(t, filepath.Join(dir, "sub/c.txt")); got != "hello world\n" {
		t.Errorf("sub file should be unchanged, got %q", got)
	}
}

func TestTextReplaceDirRecursive(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"), "hello world\n")
	writeFile(t, filepath.Join(dir, "sub/b.txt"), "hello world\n")

	p := newTextInterpreter(t, nil)
	result, err := p.Eval(`import scriptling.text as text
text.replace("hello", "hi", "` + dir + `", recursive=True)`)
	if err != nil {
		t.Fatal(err)
	}
	if result.(*object.Integer).Value != 2 {
		t.Errorf("expected 2 files modified, got %v", result.(*object.Integer).Value)
	}
	if got := readFile(t, filepath.Join(dir, "sub/b.txt")); got != "hi world\n" {
		t.Errorf("unexpected sub file content: %q", got)
	}
}

func TestTextReplaceGlob(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.py"), "hello world\n")
	writeFile(t, filepath.Join(dir, "b.txt"), "hello world\n")

	p := newTextInterpreter(t, nil)
	result, err := p.Eval(`import scriptling.text as text
text.replace("hello", "hi", "` + dir + `", glob="*.py")`)
	if err != nil {
		t.Fatal(err)
	}
	if result.(*object.Integer).Value != 1 {
		t.Errorf("expected 1 file modified, got %v", result.(*object.Integer).Value)
	}
	if got := readFile(t, filepath.Join(dir, "b.txt")); got != "hello world\n" {
		t.Errorf("txt file should be unchanged, got %q", got)
	}
}

func TestTextReplaceSkipsBinary(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bin.dat")
	writeFile(t, path, "hello\x00world")

	p := newTextInterpreter(t, nil)
	result, err := p.Eval(`import scriptling.text as text
text.replace("hello", "hi", "` + path + `")`)
	if err != nil {
		t.Fatal(err)
	}
	if result.(*object.Integer).Value != 0 {
		t.Errorf("expected binary file to be skipped")
	}
}

func TestTextReplaceMaxSize(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "big.txt")
	writeFile(t, path, "hello world this is a long line\n")

	p := newTextInterpreter(t, nil)
	result, err := p.Eval(`import scriptling.text as text
text.replace("hello", "hi", "` + path + `", max_size=10)`)
	if err != nil {
		t.Fatal(err)
	}
	if result.(*object.Integer).Value != 0 {
		t.Errorf("expected file to be skipped due to max_size")
	}
}

func TestTextReplacePathRestriction(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"), "hello\n")
	otherDir := t.TempDir()

	p := newTextInterpreter(t, []string{otherDir})
	_, err := p.Eval(`import scriptling.text as text
text.replace("hello", "hi", "` + dir + `")`)
	if err == nil {
		t.Error("expected permission error for restricted path")
	}
}

func TestTextReplacePathRestrictionAllowed(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"), "hello world\n")

	p := newTextInterpreter(t, []string{dir})
	result, err := p.Eval(`import scriptling.text as text
text.replace("hello", "hi", "` + dir + `")`)
	if err != nil {
		t.Fatalf("expected success within allowed path: %v", err)
	}
	if result.(*object.Integer).Value != 1 {
		t.Errorf("expected 1 file modified")
	}
}

func TestTextReplaceAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	writeFile(t, path, "line one\nline two\nline three\n")

	p := newTextInterpreter(t, nil)
	_, err := p.Eval(`import scriptling.text as text
text.replace("two", "TWO", "` + path + `")`)
	if err != nil {
		t.Fatal(err)
	}
	got := readFile(t, path)
	if got != "line one\nline TWO\nline three\n" {
		t.Errorf("unexpected content after atomic write: %q", got)
	}
}

// ── extract tests ─────────────────────────────────────────────────────────────

func TestTextExtractFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.py")
	writeFile(t, path, "def get_user(id):\ndef get_order(id):\ndef set_value(x):\n")

	p := newTextInterpreter(t, nil)
	result, err := p.Eval(`import scriptling.text as text
text.extract(r"def (\w+)\(", "` + path + `")`)
	if err != nil {
		t.Fatal(err)
	}

	list := result.(*object.List)
	if len(list.Elements) != 3 {
		t.Fatalf("expected 3 matches, got %d", len(list.Elements))
	}
	d := list.Elements[0].(*object.Dict)
	groupsPair, _ := d.GetByString("groups")
	groups := groupsPair.Value.(*object.List)
	if len(groups.Elements) != 1 {
		t.Fatalf("expected 1 capture group, got %d", len(groups.Elements))
	}
	if groups.Elements[0].(*object.String).Value != "get_user" {
		t.Errorf("expected 'get_user', got %q", groups.Elements[0].(*object.String).Value)
	}
}

func TestTextExtractMultipleGroupsPerLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	writeFile(t, path, "key=value\nfoo=bar\n")

	p := newTextInterpreter(t, nil)
	result, err := p.Eval(`import scriptling.text as text
text.extract(r"(\w+)=(\w+)", "` + path + `")`)
	if err != nil {
		t.Fatal(err)
	}

	list := result.(*object.List)
	if len(list.Elements) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(list.Elements))
	}
	d := list.Elements[0].(*object.Dict)
	groupsPair, _ := d.GetByString("groups")
	groups := groupsPair.Value.(*object.List)
	if len(groups.Elements) != 2 {
		t.Fatalf("expected 2 capture groups, got %d", len(groups.Elements))
	}
	if groups.Elements[0].(*object.String).Value != "key" {
		t.Errorf("expected group[0]='key', got %q", groups.Elements[0].(*object.String).Value)
	}
	if groups.Elements[1].(*object.String).Value != "value" {
		t.Errorf("expected group[1]='value', got %q", groups.Elements[1].(*object.String).Value)
	}
}

func TestTextExtractNoGroups(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	writeFile(t, path, "hello world\n")

	p := newTextInterpreter(t, nil)
	result, err := p.Eval(`import scriptling.text as text
text.extract(r"hello", "` + path + `")`)
	if err != nil {
		t.Fatal(err)
	}

	list := result.(*object.List)
	if len(list.Elements) != 1 {
		t.Fatalf("expected 1 match, got %d", len(list.Elements))
	}
	d := list.Elements[0].(*object.Dict)
	groupsPair, _ := d.GetByString("groups")
	groups := groupsPair.Value.(*object.List)
	if len(groups.Elements) != 0 {
		t.Errorf("expected 0 capture groups, got %d", len(groups.Elements))
	}
}

func TestTextExtractDictShape(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	writeFile(t, path, "version=1.2.3\n")

	p := newTextInterpreter(t, nil)
	result, err := p.Eval(`import scriptling.text as text
matches = text.extract(r"version=(\S+)", "` + path + `")
m = matches[0]
[m["file"], m["line"], m["text"], m["groups"]]`)
	if err != nil {
		t.Fatal(err)
	}

	list := result.(*object.List)
	if len(list.Elements) != 4 {
		t.Fatalf("expected 4 elements, got %d", len(list.Elements))
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
	if _, ok := list.Elements[3].(*object.List); !ok {
		t.Error("groups should be a list")
	}
}

func TestTextExtractDirRecursive(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.py"), "def foo():\n")
	writeFile(t, filepath.Join(dir, "sub/b.py"), "def bar():\n")

	p := newTextInterpreter(t, nil)
	result, err := p.Eval(`import scriptling.text as text
text.extract(r"def (\w+)\(", "` + dir + `", recursive=True, glob="*.py")`)
	if err != nil {
		t.Fatal(err)
	}

	list := result.(*object.List)
	if len(list.Elements) != 2 {
		t.Errorf("expected 2 matches, got %d", len(list.Elements))
	}
}

func TestTextExtractIgnoreCase(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	writeFile(t, path, "TODO: fix\ntodo: also\n")

	p := newTextInterpreter(t, nil)
	result, err := p.Eval(`import scriptling.text as text
text.extract(r"(todo): (\w+)", "` + path + `", ignore_case=True)`)
	if err != nil {
		t.Fatal(err)
	}

	list := result.(*object.List)
	if len(list.Elements) != 2 {
		t.Errorf("expected 2 case-insensitive matches, got %d", len(list.Elements))
	}
}

func TestTextExtractPathRestriction(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"), "hello\n")
	otherDir := t.TempDir()

	p := newTextInterpreter(t, []string{otherDir})
	_, err := p.Eval(`import scriptling.text as text
text.extract(r"hello", "` + dir + `")`)
	if err == nil {
		t.Error("expected permission error for restricted path")
	}
}
