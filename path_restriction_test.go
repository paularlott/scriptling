package scriptling_test

// Tests that path restriction errors are correctly surfaced in all execution
// contexts: embedded (p.Eval), and through try/except in scripts.
//
// The try/except cases are regression tests for the bug where a return
// statement inside an except block was silently discarded (result=NULL),
// causing the error message to be lost and the caller to see no output.

import (
	"os"
	"strings"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs"
)

// setupRestricted creates a Scriptling instance with os and pathlib restricted
// to allowedDir only.
func setupRestricted(t *testing.T, allowedDir string) *scriptling.Scriptling {
	t.Helper()
	p := scriptling.New()
	extlibs.RegisterOSLibrary(p, []string{allowedDir})
	extlibs.RegisterPathlibLibrary(p, []string{allowedDir})
	return p
}

// TestPathRestrictionEmbedded verifies that path restriction errors propagate
// as Go errors when called via p.Eval (embedded use case).
func TestPathRestrictionEmbedded(t *testing.T) {
	allowedDir := t.TempDir()
	deniedDir := t.TempDir()

	tests := []struct {
		name   string
		script string
	}{
		{
			name:   "os.listdir denied path returns error",
			script: "import os\nos.listdir(\"" + deniedDir + "\")",
		},
		{
			name:   "os.read_file denied path returns error",
			script: "import os\nos.read_file(\"" + deniedDir + "/secret.txt\")",
		},
		{
			name:   "os.write_file denied path returns error",
			script: "import os\nos.write_file(\"" + deniedDir + "/out.txt\", \"data\")",
		},
		{
			name:   "os.path.exists denied path returns error",
			script: "import os.path\nos.path.exists(\"" + deniedDir + "/file.txt\")",
		},
		{
			name:   "pathlib.Path.read_text denied path returns error",
			script: "import pathlib\npathlib.Path(\"" + deniedDir + "/file.txt\").read_text()",
		},
		{
			name:   "pathlib.Path.exists denied path returns error",
			script: "import pathlib\npathlib.Path(\"" + deniedDir + "/file.txt\").exists()",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := setupRestricted(t, allowedDir)
			_, err := p.Eval(tt.script)
			if err == nil {
				t.Error("expected error for denied path, got nil")
			} else if !strings.Contains(err.Error(), "access denied") {
				t.Errorf("expected 'access denied' in error, got: %v", err)
			}
		})
	}
}

// TestPathRestrictionTryExcept verifies that PermissionError from path restriction
// bypasses try/except and propagates as a Go error — security violations must
// not be silently swallowed by scripts.
func TestPathRestrictionTryExcept(t *testing.T) {
	allowedDir := t.TempDir()
	deniedDir := t.TempDir()

	tests := []struct {
		name   string
		script string
	}{
		{
			name: "os.listdir denied bypasses try/except",
			script: `
import os
def f():
    try:
        return os.listdir("` + deniedDir + `")
    except Exception as e:
        return "caught: " + str(e)
f()
`,
		},
		{
			name: "os.read_file denied bypasses try/except",
			script: `
import os
def f():
    try:
        return os.read_file("` + deniedDir + `/secret.txt")
    except Exception as e:
        return "caught: " + str(e)
f()
`,
		},
		{
			name: "os.write_file denied bypasses try/except",
			script: `
import os
def f():
    try:
        os.write_file("` + deniedDir + `/out.txt", "data")
        return "wrote"
    except Exception as e:
        return "caught: " + str(e)
f()
`,
		},
		{
			name: "pathlib denied bypasses try/except",
			script: `
import pathlib
def f():
    try:
        return pathlib.Path("` + deniedDir + `/file.txt").read_text()
    except Exception as e:
        return "caught: " + str(e)
f()
`,
		},
		{
			name: "top-level try/except does not catch PermissionError",
			script: `
import os
result = "no error"
try:
    os.listdir("` + deniedDir + `")
except Exception as e:
    result = "caught: " + str(e)
result
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := setupRestricted(t, allowedDir)
			_, err := p.Eval(tt.script)
			if err == nil {
				t.Error("expected PermissionError to bypass try/except and surface as Go error, got nil")
			} else if !strings.Contains(err.Error(), "access denied") {
				t.Errorf("expected 'access denied' in error, got: %v", err)
			}
		})
	}
}

// TestPathRestrictionAllowedPathsStillWork verifies that allowed paths continue
// to work correctly after the try/except fix.
func TestPathRestrictionAllowedPathsStillWork(t *testing.T) {
	allowedDir := t.TempDir()
	if err := os.WriteFile(allowedDir+"/data.txt", []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("os.read_file allowed path works", func(t *testing.T) {
		p := setupRestricted(t, allowedDir)
		result, err := p.Eval("import os\nos.read_file(\"" + allowedDir + "/data.txt\")")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Inspect() != "hello" {
			t.Errorf("expected 'hello', got %q", result.Inspect())
		}
	})

	t.Run("os.read_file allowed path inside try/except works", func(t *testing.T) {
		p := setupRestricted(t, allowedDir)
		result, err := p.Eval(`
import os
def f():
    try:
        return os.read_file("` + allowedDir + `/data.txt")
    except Exception as e:
        return "error: " + str(e)
f()
`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Inspect() != "hello" {
			t.Errorf("expected 'hello', got %q", result.Inspect())
		}
	})

	t.Run("pathlib allowed path inside try/except works", func(t *testing.T) {
		p := setupRestricted(t, allowedDir)
		result, err := p.Eval(`
import pathlib
def f():
    try:
        return pathlib.Path("` + allowedDir + `/data.txt").read_text()
    except Exception as e:
        return "error: " + str(e)
f()
`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Inspect() != "hello" {
			t.Errorf("expected 'hello', got %q", result.Inspect())
		}
	})
}
