package scriptling_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paularlott/scriptling"
)

func TestMainNameEvalFile(t *testing.T) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "script.py")
	if err := os.WriteFile(scriptPath, []byte("name = __name__"), 0644); err != nil {
		t.Fatal(err)
	}

	p := scriptling.New()
	if _, err := p.EvalFile(scriptPath); err != nil {
		t.Fatalf("EvalFile failed: %v", err)
	}

	got, objErr := p.GetVarAsString("name")
	if objErr != nil {
		t.Fatalf("GetVarAsString failed: %v", objErr)
	}
	if got != "__main__" {
		t.Errorf("__name__ = %q, want %q", got, "__main__")
	}
}

func TestMainNameGuardExecuted(t *testing.T) {
	dir := t.TempDir()
	script := `
executed = False
if __name__ == "__main__":
    executed = True
`
	scriptPath := filepath.Join(dir, "script.py")
	if err := os.WriteFile(scriptPath, []byte(script), 0644); err != nil {
		t.Fatal(err)
	}

	p := scriptling.New()
	if _, err := p.EvalFile(scriptPath); err != nil {
		t.Fatalf("EvalFile failed: %v", err)
	}

	got, objErr := p.GetVarAsBool("executed")
	if objErr != nil {
		t.Fatalf("GetVarAsBool failed: %v", objErr)
	}
	if !got {
		t.Error("expected if __name__ == '__main__' block to execute, but it did not")
	}
}

func TestMainNameGuardNotExecutedInLibrary(t *testing.T) {
	libScript := `
executed = False
if __name__ == "__main__":
    executed = True
`
	mainScript := `
import mylib
lib_executed = mylib.executed
`
	dir := t.TempDir()
	mainPath := filepath.Join(dir, "main.py")
	if err := os.WriteFile(mainPath, []byte(mainScript), 0644); err != nil {
		t.Fatal(err)
	}

	p := scriptling.New()
	if err := p.RegisterScriptLibrary("mylib", libScript); err != nil {
		t.Fatal(err)
	}
	if _, err := p.EvalFile(mainPath); err != nil {
		t.Fatalf("EvalFile failed: %v", err)
	}

	got, objErr := p.GetVarAsBool("lib_executed")
	if objErr != nil {
		t.Fatalf("GetVarAsBool failed: %v", objErr)
	}
	if got {
		t.Error("expected if __name__ == '__main__' block NOT to execute in library, but it did")
	}
}

func TestMainNameSetWithPlainEval(t *testing.T) {
	// Plain Eval should have __name__ set to "__main__" (Python REPL behavior)
	p := scriptling.New()
	val, err := p.GetVarAsString("__name__")
	if err != nil {
		t.Fatalf("expected __name__ to be accessible, got error: %v", err)
	}
	if val != "__main__" {
		t.Errorf("__name__ = %q, want %q", val, "__main__")
	}
}

func TestMainNamePersistsAfterEvalFile(t *testing.T) {
	// __name__ should remain "__main__" after EvalFile (consistent with being set in New())
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "script.py")
	if err := os.WriteFile(scriptPath, []byte("x = 1"), 0644); err != nil {
		t.Fatal(err)
	}

	p := scriptling.New()
	if _, err := p.EvalFile(scriptPath); err != nil {
		t.Fatalf("EvalFile failed: %v", err)
	}

	// After EvalFile returns, __name__ should still be "__main__"
	val, err := p.GetVarAsString("__name__")
	if err != nil {
		t.Fatalf("expected __name__ to be accessible after EvalFile, got error: %v", err)
	}
	if val != "__main__" {
		t.Errorf("__name__ = %q, want %q", val, "__main__")
	}
}

func TestMainNameGuardFunctionNotCalledInLibrary(t *testing.T) {
	// The canonical Python pattern: define functions, then call them only when run as main
	libScript := `
def helper():
    return 42

result = None
if __name__ == "__main__":
    result = helper()
`
	mainScript := `
import mylib
lib_result = mylib.result
`
	dir := t.TempDir()
	mainPath := filepath.Join(dir, "main.py")
	if err := os.WriteFile(mainPath, []byte(mainScript), 0644); err != nil {
		t.Fatal(err)
	}

	p := scriptling.New()
	if err := p.RegisterScriptLibrary("mylib", libScript); err != nil {
		t.Fatal(err)
	}
	if _, err := p.EvalFile(mainPath); err != nil {
		t.Fatalf("EvalFile failed: %v", err)
	}

	result, objErr := p.GetVar("lib_result")
	if objErr != nil {
		t.Fatalf("GetVar failed: %v", objErr)
	}
	if result != nil {
		t.Errorf("expected lib_result to be None (guard not triggered), got %v", result)
	}
}
