package extlibs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/object"
)

// --- parseGlobKwargs unit tests (mirrors parseFindKwargs coverage) ---

func TestParseGlobKwargsDefaults(t *testing.T) {
	recursive, includeHidden, errObj := parseGlobKwargs(object.NewKwargs(nil))
	if errObj != nil {
		t.Fatalf("unexpected error: %v", errObj)
	}
	if recursive {
		t.Error("recursive should default to false")
	}
	if includeHidden {
		t.Error("include_hidden should default to false")
	}
}

func TestParseGlobKwargsAllValues(t *testing.T) {
	kwargs := object.NewKwargs(map[string]object.Object{
		"recursive":      object.NewBoolean(true),
		"include_hidden": object.NewBoolean(true),
	})
	recursive, includeHidden, errObj := parseGlobKwargs(kwargs)
	if errObj != nil {
		t.Fatalf("unexpected error: %v", errObj)
	}
	if !recursive {
		t.Error("recursive should be true")
	}
	if !includeHidden {
		t.Error("include_hidden should be true")
	}
}

// --- glob.escape unit test (pre-existing gap) ---

func TestGlobEscape(t *testing.T) {
	p := scriptling.New()
	RegisterGlobLibrary(p, nil)

	result, err := p.Eval(`import glob
glob.escape("file*.txt?")`)
	if err != nil {
		t.Fatal(err)
	}
	s, errObj := result.AsString()
	if errObj != nil {
		t.Fatalf("expected string: %v", errObj)
	}
	want := "file[*].txt[?]"
	if s != want {
		t.Errorf("escape: got %q, want %q", s, want)
	}
}

// --- Evaluator-level tests for glob kwargs ---

func newGlobInterpreter(t *testing.T, allowedPaths []string) *scriptling.Scriptling {
	t.Helper()
	p := scriptling.New()
	RegisterGlobLibrary(p, allowedPaths)
	return p
}

func TestGlobBuiltinRecursiveAndHidden(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "sub"), 0755)
	os.WriteFile(filepath.Join(dir, "a.py"), []byte("x"), 0644)
	os.WriteFile(filepath.Join(dir, ".hidden.py"), []byte("x"), 0644)
	os.WriteFile(filepath.Join(dir, "sub", "b.py"), []byte("x"), 0644)
	os.WriteFile(filepath.Join(dir, "sub", ".secret.py"), []byte("x"), 0644)

	p := newGlobInterpreter(t, []string{dir})

	// recursive=True, include_hidden=False (default) → a.py + sub/b.py = 2
	result, err := p.Eval(`import glob
len(glob.glob("**/*.py", "` + dir + `", recursive=True))`)
	if err != nil {
		t.Fatal(err)
	}
	if i, _ := result.(*object.Integer); i == nil || i.IntValue() != 2 {
		t.Errorf("recursive, no hidden: expected 2, got %v", result)
	}

	// recursive=True, include_hidden=True → all 4
	result, err = p.Eval(`import glob
len(glob.glob("**/*.py", "` + dir + `", recursive=True, include_hidden=True))`)
	if err != nil {
		t.Fatal(err)
	}
	if i, _ := result.(*object.Integer); i == nil || i.IntValue() != 4 {
		t.Errorf("recursive, hidden: expected 4, got %v", result)
	}

	// recursive=False → ** collapses to *, only sub/b.py (one level deep, no hidden)
	result, err = p.Eval(`import glob
len(glob.glob("**/*.py", "` + dir + `", recursive=False))`)
	if err != nil {
		t.Fatal(err)
	}
	if i, _ := result.(*object.Integer); i == nil || i.IntValue() != 1 {
		t.Errorf("non-recursive: expected 1, got %v", result)
	}
}

func TestGlobBuiltinIglobRecursive(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "sub"), 0755)
	os.WriteFile(filepath.Join(dir, "a.py"), []byte("x"), 0644)
	os.WriteFile(filepath.Join(dir, "sub", "b.py"), []byte("x"), 0644)

	p := newGlobInterpreter(t, []string{dir})

	result, err := p.Eval(`import glob
count = 0
for f in glob.iglob("**/*.py", "` + dir + `", recursive=True):
    count += 1
count`)
	if err != nil {
		t.Fatal(err)
	}
	if i, _ := result.(*object.Integer); i == nil || i.IntValue() != 2 {
		t.Errorf("iglob recursive: expected 2, got %v", result)
	}
}

// --- Evaluator-level tests for find.path ---

func newFindInterpreter(t *testing.T, allowedPaths []string) *scriptling.Scriptling {
	t.Helper()
	p := scriptling.New()
	RegisterFindLibrary(p, allowedPaths)
	return p
}

func TestFindBuiltinPath(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "sub"), 0755)
	os.WriteFile(filepath.Join(dir, "a.py"), []byte("x"), 0644)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("x"), 0644)
	os.WriteFile(filepath.Join(dir, "sub", "c.py"), []byte("x"), 0644)
	os.MkdirAll(filepath.Join(dir, ".git"), 0755)
	os.WriteFile(filepath.Join(dir, ".git", "config"), []byte("x"), 0644)

	p := newFindInterpreter(t, []string{dir})

	// find .py files recursively
	result, err := p.Eval(`import scriptling.find as find
len(find.path("` + dir + `", name="*.py", type="file"))`)
	if err != nil {
		t.Fatal(err)
	}
	if i, _ := result.(*object.Integer); i == nil || i.IntValue() != 2 {
		t.Errorf("find *.py: expected 2, got %v", result)
	}

	// find dirs (default: no hidden) → only sub
	result, err = p.Eval(`import scriptling.find as find
len(find.path("` + dir + `", type="dir"))`)
	if err != nil {
		t.Fatal(err)
	}
	if i, _ := result.(*object.Integer); i == nil || i.IntValue() != 1 {
		t.Errorf("find dirs: expected 1, got %v", result)
	}

	// find dirs with include_hidden → sub + .git = 2
	result, err = p.Eval(`import scriptling.find as find
len(find.path("` + dir + `", type="dir", include_hidden=True))`)
	if err != nil {
		t.Fatal(err)
	}
	if i, _ := result.(*object.Integer); i == nil || i.IntValue() != 2 {
		t.Errorf("find dirs hidden: expected 2, got %v", result)
	}

	// find with invalid type → error
	_, err = p.Eval(`import scriptling.find as find
find.path("` + dir + `", type="block")`)
	if err == nil {
		t.Error("expected error for invalid type")
	}
}

func TestFindBuiltinSecurityDenied(t *testing.T) {
	allowed := t.TempDir()
	denied := t.TempDir()
	os.WriteFile(filepath.Join(denied, "secret.txt"), []byte("x"), 0644)

	p := newFindInterpreter(t, []string{allowed})

	_, err := p.Eval(`import scriptling.find as find
find.path("` + denied + `")`)
	if err == nil {
		t.Error("expected permission error for denied path")
	}
}
