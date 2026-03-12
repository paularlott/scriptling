package scriptling_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs"
	"github.com/paularlott/scriptling/libloader"
)

func TestFileVarEvalFile(t *testing.T) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "myscript.py")
	if err := os.WriteFile(scriptPath, []byte("__file__"), 0644); err != nil {
		t.Fatal(err)
	}

	p := scriptling.New()
	result, err := p.EvalFile(scriptPath)
	if err != nil {
		t.Fatalf("EvalFile failed: %v", err)
	}

	got, objErr := result.AsString()
	if objErr != nil {
		t.Fatalf("AsString failed: %v", objErr)
	}
	if got != scriptPath {
		t.Errorf("__file__ = %q, want %q", got, scriptPath)
	}
}

func TestFileVarSetSourceFile(t *testing.T) {
	p := scriptling.New()
	p.SetSourceFile("/some/path/tool.py")

	result, err := p.Eval("__file__")
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}

	got, objErr := result.AsString()
	if objErr != nil {
		t.Fatalf("AsString failed: %v", objErr)
	}
	if got != "/some/path/tool.py" {
		t.Errorf("__file__ = %q, want %q", got, "/some/path/tool.py")
	}
}

func TestFileVarOsPathDirname(t *testing.T) {
	// The canonical use-case: os.path.dirname(__file__)
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "tool.py")
	script := `import os.path
os.path.dirname(__file__)
`
	if err := os.WriteFile(scriptPath, []byte(script), 0644); err != nil {
		t.Fatal(err)
	}

	p := scriptling.New()
	extlibs.RegisterOSLibrary(p, nil)

	result, err := p.EvalFile(scriptPath)
	if err != nil {
		t.Fatalf("EvalFile failed: %v", err)
	}

	got, objErr := result.AsString()
	if objErr != nil {
		t.Fatalf("AsString failed: %v", objErr)
	}
	if got != dir {
		t.Errorf("os.path.dirname(__file__) = %q, want %q", got, dir)
	}
}

func TestFileVarNotSetWithoutFile(t *testing.T) {
	// When using plain Eval (no file), __file__ should not be set
	p := scriptling.New()
	_, err := p.Eval("__file__")
	if err == nil {
		t.Error("expected error accessing __file__ when no source file set, got nil")
	}
}

func TestFileVarEvalFileRestored(t *testing.T) {
	// After EvalFile returns, __file__ should not leak into subsequent Eval calls
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "script.py")
	if err := os.WriteFile(scriptPath, []byte("x = 1"), 0644); err != nil {
		t.Fatal(err)
	}

	p := scriptling.New()
	if _, err := p.EvalFile(scriptPath); err != nil {
		t.Fatalf("EvalFile failed: %v", err)
	}

	_, err := p.Eval("__file__")
	if err == nil {
		t.Error("expected __file__ to be unset after EvalFile, but it was accessible")
	}
}

func TestFileVarSequentialEvalFiles(t *testing.T) {
	// Go calls EvalFile(a.py) then EvalFile(b.py) — each should see its own __file__
	dir := t.TempDir()
	aPath := filepath.Join(dir, "a.py")
	bPath := filepath.Join(dir, "b.py")
	if err := os.WriteFile(aPath, []byte("file_a = __file__"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bPath, []byte("file_b = __file__"), 0644); err != nil {
		t.Fatal(err)
	}

	p := scriptling.New()
	if _, err := p.EvalFile(aPath); err != nil {
		t.Fatalf("EvalFile(a) failed: %v", err)
	}
	if _, err := p.EvalFile(bPath); err != nil {
		t.Fatalf("EvalFile(b) failed: %v", err)
	}

	gotA, _ := p.GetVarAsString("file_a")
	gotB, _ := p.GetVarAsString("file_b")

	if gotA != aPath {
		t.Errorf("file_a = %q, want %q", gotA, aPath)
	}
	if gotB != bPath {
		t.Errorf("file_b = %q, want %q", gotB, bPath)
	}
	// After both complete, __file__ should be gone
	_, err := p.Eval("__file__")
	if err == nil {
		t.Error("expected __file__ to be unset after both EvalFiles, but it was accessible")
	}
}

func TestFileVarNestedEvalFiles(t *testing.T) {
	// Go calls EvalFile(a.py); while a is running, Go calls EvalFile(b.py);
	// b should see its own __file__, and after b returns __file__ should be a's path again.
	dir := t.TempDir()
	aPath := filepath.Join(dir, "a.py")
	bPath := filepath.Join(dir, "b.py")

	// a.py just sets a variable so we can check __file__ was correct during its run
	if err := os.WriteFile(aPath, []byte("file_in_a = __file__"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bPath, []byte("file_in_b = __file__"), 0644); err != nil {
		t.Fatal(err)
	}

	p := scriptling.New()

	// Simulate nested: eval a, then mid-stream eval b (as Go would do e.g. in a handler)
	if _, err := p.EvalFile(aPath); err != nil {
		t.Fatalf("EvalFile(a) failed: %v", err)
	}
	// At this point __file__ should be restored (gone)
	_, errAfterA := p.Eval("__file__")
	if errAfterA == nil {
		t.Error("__file__ should be unset after EvalFile(a) returns")
	}

	if _, err := p.EvalFile(bPath); err != nil {
		t.Fatalf("EvalFile(b) failed: %v", err)
	}

	gotA, _ := p.GetVarAsString("file_in_a")
	gotB, _ := p.GetVarAsString("file_in_b")

	if gotA != aPath {
		t.Errorf("file_in_a = %q, want %q", gotA, aPath)
	}
	if gotB != bPath {
		t.Errorf("file_in_b = %q, want %q", gotB, bPath)
	}
}

func TestFileVarLibraryImportDoesNotAffectFile(t *testing.T) {
	// When a script imports a library, __file__ in the main script should not change.
	// Libraries run in their own environment so this should be fine, but verify it.
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "main.py")
	libPath := filepath.Join(dir, "mylib.py")

	if err := os.WriteFile(libPath, []byte("def helper(): return 42"), 0644); err != nil {
		t.Fatal(err)
	}
	script := `import mylib
file_after_import = __file__
`
	if err := os.WriteFile(scriptPath, []byte(script), 0644); err != nil {
		t.Fatal(err)
	}

	p := scriptling.New()
	p.SetLibraryLoader(libloader.NewFilesystem(dir))

	if _, err := p.EvalFile(scriptPath); err != nil {
		t.Fatalf("EvalFile failed: %v", err)
	}

	got, _ := p.GetVarAsString("file_after_import")
	if got != scriptPath {
		t.Errorf("__file__ after import = %q, want %q", got, scriptPath)
	}
}
