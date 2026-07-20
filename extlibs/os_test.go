package extlibs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/object"
)

func TestOSGetenv(t *testing.T) {
	os.Setenv("TEST_OS_VAR", "test_value")
	defer os.Unsetenv("TEST_OS_VAR")
	os.Unsetenv("TEST_MISSING_VAR")

	p := scriptling.New()
	RegisterOSLibrary(p, nil)

	tests := []struct {
		name  string
		code  string
		check func(t *testing.T, result object.Object)
	}{
		{
			name: "existing var returns value",
			code: `import os
import os.path
os.getenv("TEST_OS_VAR")`,
			check: func(t *testing.T, result object.Object) {
				str, ok := result.(*object.String)
				if !ok {
					t.Fatalf("expected String, got %T", result)
				}
				if str.StringValue() != "test_value" {
					t.Errorf("expected %q, got %q", "test_value", str.StringValue())
				}
			},
		},
		{
			name: "missing var without default returns None",
			code: `import os
import os.path
os.getenv("TEST_MISSING_VAR")`,
			check: func(t *testing.T, result object.Object) {
				if _, ok := result.(*object.Null); !ok {
					t.Errorf("expected None/Null, got %T (%v)", result, result)
				}
			},
		},
		{
			name: "missing var with default returns default",
			code: `import os
import os.path
os.getenv("TEST_MISSING_VAR", "fallback")`,
			check: func(t *testing.T, result object.Object) {
				str, ok := result.(*object.String)
				if !ok {
					t.Fatalf("expected String, got %T", result)
				}
				if str.StringValue() != "fallback" {
					t.Errorf("expected %q, got %q", "fallback", str.StringValue())
				}
			},
		},
		{
			name: "existing var with default returns value not default",
			code: `import os
import os.path
os.getenv("TEST_OS_VAR", "fallback")`,
			check: func(t *testing.T, result object.Object) {
				str, ok := result.(*object.String)
				if !ok {
					t.Fatalf("expected String, got %T", result)
				}
				if str.StringValue() != "test_value" {
					t.Errorf("expected %q, got %q", "test_value", str.StringValue())
				}
			},
		},
		{
			name: "missing var returns None so 'if not' pattern works",
			code: `import os
import os.path
val = os.getenv("TEST_MISSING_VAR")
if not val:
    val = "default_applied"
val`,
			check: func(t *testing.T, result object.Object) {
				str, ok := result.(*object.String)
				if !ok {
					t.Fatalf("expected String, got %T", result)
				}
				if str.StringValue() != "default_applied" {
					t.Errorf("expected %q, got %q", "default_applied", str.StringValue())
				}
			},
		},
		{
			name: "var set to empty string returns empty string not None",
			code: `import os
import os.path
os.getenv("TEST_OS_VAR")`,
			check: func(t *testing.T, result object.Object) {
				// TEST_OS_VAR is set to "test_value", not empty — just confirm it's a String
				if _, ok := result.(*object.String); !ok {
					t.Errorf("expected String for set var, got %T", result)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := p.Eval(tt.code)
			if err != nil {
				t.Fatalf("Eval failed: %v", err)
			}
			tt.check(t, result)
		})
	}
}

func TestOSGetenvEmptyStringVar(t *testing.T) {
	// Explicitly set a var to empty string — should return "" not None
	os.Setenv("TEST_EMPTY_VAR", "")
	defer os.Unsetenv("TEST_EMPTY_VAR")

	p := scriptling.New()
	RegisterOSLibrary(p, nil)

	result, err := p.Eval(`import os
import os.path
os.getenv("TEST_EMPTY_VAR")`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	str, ok := result.(*object.String)
	if !ok {
		t.Fatalf("expected String for empty-string var, got %T", result)
	}
	if str.StringValue() != "" {
		t.Errorf("expected empty string, got %q", str.StringValue())
	}
}

func TestOSEnvironIsDict(t *testing.T) {
	p := scriptling.New()
	RegisterOSLibrary(p, nil)

	// Test that os.environ behaves like a dict by using .get() method
	result, err := p.Eval(`import os
import os.path
result = os.environ.get("PATH", "default")
len(result) > 0`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}

	boolean, ok := result.(*object.Boolean)
	if !ok {
		t.Fatalf("Expected Boolean, got %T", result)
	}

	if !boolean.BoolValue() {
		t.Error("Expected os.environ.get() to work like a dict")
	}
}

func TestOSEnvironItems(t *testing.T) {
	os.Setenv("TEST_ITER_VAR", "iter_value")
	defer os.Unsetenv("TEST_ITER_VAR")

	p := scriptling.New()
	RegisterOSLibrary(p, nil)

	// Test that we can iterate over os.environ
	result, err := p.Eval(`import os
import os.path
found = False
for key, value in os.environ.items():
    if key == "TEST_ITER_VAR" and value == "iter_value":
        found = True
        break
found`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}

	boolean, ok := result.(*object.Boolean)
	if !ok {
		t.Fatalf("Expected Boolean, got %T", result)
	}

	if !boolean.BoolValue() {
		t.Error("Expected to find TEST_ITER_VAR in os.environ.items()")
	}
}

func TestOSChmodAndMkdirMode(t *testing.T) {
	tmp := t.TempDir()
	allowedTmp, err := filepath.EvalSymlinks(tmp)
	if err != nil {
		t.Fatalf("eval temp dir: %v", err)
	}
	tmp = allowedTmp
	dir := filepath.Join(tmp, "mode-dir")
	nested := filepath.Join(tmp, "nested", "child")
	file := filepath.Join(tmp, "file.txt")

	p := scriptling.New()
	RegisterOSLibrary(p, []string{allowedTmp})

	code := `import os
import os.path
os.mkdir("` + dir + `", 0o700)
os.makedirs("` + nested + `", mode=0o755)
os.makedirs("` + nested + `", exist_ok=True)
os.write_file("` + file + `", "content")
os.chmod("` + file + `", mode=0o600)
os.chmod("` + dir + `", 0o711)
True`
	result, err := p.Eval(code)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	if boolean, ok := result.(*object.Boolean); !ok || !boolean.BoolValue() {
		t.Fatalf("expected True, got %T %v", result, result)
	}

	fileInfo, err := os.Stat(file)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	if got := fileInfo.Mode().Perm(); got != 0600 {
		t.Fatalf("file mode = %o, want 0600", got)
	}
	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if got := dirInfo.Mode().Perm(); got != 0711 {
		t.Fatalf("dir mode = %o, want 0711", got)
	}
}

func TestOSWriteFileMode(t *testing.T) {
	tmp := t.TempDir()
	allowedTmp, err := filepath.EvalSymlinks(tmp)
	if err != nil {
		t.Fatalf("eval temp dir: %v", err)
	}
	positionalFile := filepath.Join(allowedTmp, "positional.txt")
	kwargFile := filepath.Join(allowedTmp, "kwarg.txt")

	p := scriptling.New()
	RegisterOSLibrary(p, []string{allowedTmp})

	code := `import os
import os.path
os.write_file("` + positionalFile + `", "content", 0o600)
os.write_file("` + kwargFile + `", "content", mode=0o640)
True`
	result, err := p.Eval(code)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	if boolean, ok := result.(*object.Boolean); !ok || !boolean.BoolValue() {
		t.Fatalf("expected True, got %T %v", result, result)
	}

	positionalInfo, err := os.Stat(positionalFile)
	if err != nil {
		t.Fatalf("stat positional file: %v", err)
	}
	if got := positionalInfo.Mode().Perm(); got != 0600 {
		t.Fatalf("positional file mode = %o, want 0600", got)
	}
	kwargInfo, err := os.Stat(kwargFile)
	if err != nil {
		t.Fatalf("stat kwarg file: %v", err)
	}
	if got := kwargInfo.Mode().Perm(); got != 0640 {
		t.Fatalf("kwarg file mode = %o, want 0640", got)
	}
}

func TestOSRemovedirs(t *testing.T) {
	tmp := t.TempDir()
	allowedTmp, err := filepath.EvalSymlinks(tmp)
	if err != nil {
		t.Fatalf("eval temp dir: %v", err)
	}
	tmp = allowedTmp
	leaf := filepath.Join(tmp, "a", "b", "c")

	p := scriptling.New()
	RegisterOSLibrary(p, []string{tmp})

	code := `import os
import os.path
os.makedirs("` + leaf + `")
os.removedirs("` + leaf + `")
True`
	result, err := p.Eval(code)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	if boolean, ok := result.(*object.Boolean); !ok || !boolean.BoolValue() {
		t.Fatalf("expected True, got %T %v", result, result)
	}
	if _, err := os.Stat(filepath.Join(tmp, "a")); !os.IsNotExist(err) {
		t.Fatalf("expected removedirs to prune empty parents, stat err = %v", err)
	}
	if _, err := os.Stat(tmp); err != nil {
		t.Fatalf("allowed root should remain: %v", err)
	}
}

func TestOSSymlinkAndIslink(t *testing.T) {
	tmp := t.TempDir()
	allowedTmp, err := filepath.EvalSymlinks(tmp)
	if err != nil {
		t.Fatalf("eval temp dir: %v", err)
	}
	tmp = allowedTmp

	// Create a target file first.
	target := filepath.Join(tmp, "target.txt")
	if err := os.WriteFile(target, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(tmp, "link.txt")

	p := scriptling.New()
	RegisterOSLibrary(p, []string{tmp})

	// Create symlink via scriptling, then verify with islink.
	code := `import os
import os.path
import os.path
os.symlink("` + target + `", "` + link + `")
os.path.islink("` + link + `")`
	result, err := p.Eval(code)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	if boolean, ok := result.(*object.Boolean); !ok || !boolean.BoolValue() {
		t.Fatalf("expected islink=True, got %T %v", result, result)
	}

	// islink on a regular file should be False.
	code = `import os
import os.path
os.path.islink("` + target + `")`
	result, err = p.Eval(code)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	if boolean, ok := result.(*object.Boolean); !ok || boolean.BoolValue() {
		t.Fatalf("expected islink=False for regular file, got %T %v", result, result)
	}

	// islink on a nonexistent path should be False.
	code = `import os
import os.path
os.path.islink("` + filepath.Join(tmp, "nope") + `")`
	result, err = p.Eval(code)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	if boolean, ok := result.(*object.Boolean); !ok || boolean.BoolValue() {
		t.Fatalf("expected islink=False for missing path, got %T %v", result, result)
	}
}
