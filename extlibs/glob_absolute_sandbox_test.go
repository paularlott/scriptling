package extlibs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paularlott/scriptling/object"
)

// TestGlobAbsolutePatternsWithSandbox: absolute patterns must match inside
// allowed directories and return nothing (not error, not CWD-relative
// results) outside them — the per-match IsPathAllowed filter is the
// enforcement, so no root check applies to the implicit root.
func TestGlobAbsolutePatternsWithSandbox(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	os.WriteFile(filepath.Join(dir, "keep.txt"), []byte("x"), 0644)
	os.WriteFile(filepath.Join(dir, "drop.md"), []byte("x"), 0644)
	os.WriteFile(filepath.Join(outside, "other.txt"), []byte("x"), 0644)

	p := newGlobInterpreter(t, []string{dir})

	// Inside the allowed dir: matches, and only the matching suffix.
	result, err := p.Eval(`import glob
len(glob.glob("` + filepath.Join(dir, "*.txt") + `"))`)
	if err != nil {
		t.Fatalf("absolute pattern inside allowed dir errored: %v", err)
	}
	if i, _ := result.(*object.Integer); i == nil || i.IntValue() != 1 {
		t.Errorf("absolute *.txt in allowed dir: want 1, got %v", result)
	}

	// No match in the allowed dir: empty list, no error.
	result, err = p.Eval(`import glob
len(glob.glob("` + filepath.Join(dir, "*.nomatch") + `"))`)
	if err != nil {
		t.Fatalf("no-match absolute pattern errored: %v", err)
	}
	if i, _ := result.(*object.Integer); i == nil || i.IntValue() != 0 {
		t.Errorf("no-match absolute pattern: want 0, got %v", result)
	}

	// Outside the allowed dir: silently filtered to empty, not an error,
	// and never a CWD-relative result.
	result, err = p.Eval(`import glob
len(glob.glob("` + filepath.Join(outside, "*.txt") + `"))`)
	if err != nil {
		t.Fatalf("absolute pattern outside allowed dir errored: %v", err)
	}
	if i, _ := result.(*object.Integer); i == nil || i.IntValue() != 0 {
		t.Errorf("absolute pattern outside allowed dir: want 0 (filtered), got %v", result)
	}

	// Recursive absolute pattern inside the allowed dir still walks.
	os.MkdirAll(filepath.Join(dir, "nested"), 0755)
	os.WriteFile(filepath.Join(dir, "nested", "deep.txt"), []byte("x"), 0644)
	result, err = p.Eval(`import glob
len(glob.glob("` + filepath.Join(dir, "**", "*.txt") + `", recursive=True))`)
	if err != nil {
		t.Fatalf("recursive absolute pattern errored: %v", err)
	}
	// keep.txt + nested/deep.txt (root itself is not a *.txt)
	if i, _ := result.(*object.Integer); i == nil || i.IntValue() != 2 {
		t.Errorf("recursive absolute pattern: want 2, got %v", result)
	}
}

// TestGlobExplicitRootStillChecked: an explicitly passed root directory is
// still security-checked, absolute pattern or not.
func TestGlobExplicitRootStillChecked(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()

	p := newGlobInterpreter(t, []string{dir})

	_, err := p.Eval(`import glob
glob.glob("*.txt", "` + outside + `")`)
	if err == nil || !strings.Contains(err.Error(), "outside allowed") {
		t.Fatalf("explicit disallowed root should be a permission error, got %v", err)
	}
}
