package pack

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/libloader"
)

// TestIntegrationRunFromPackage tests running code that imports from a package
func TestIntegrationRunFromPackage(t *testing.T) {
	// Create a package
	srcDir := t.TempDir()

	manifestContent := `
name = "mathlib"
version = "1.0.0"
description = "A math library"
`
	if err := os.WriteFile(filepath.Join(srcDir, "manifest.toml"), []byte(manifestContent), 0644); err != nil {
		t.Fatal(err)
	}

	libDir := filepath.Join(srcDir, "lib")
	if err := os.MkdirAll(libDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a module with functions
	mathCode := `
def add(a, b):
    return a + b

def multiply(a, b):
    return a * b

PI = 3.14159
`
	if err := os.WriteFile(filepath.Join(libDir, "mathops.py"), []byte(mathCode), 0644); err != nil {
		t.Fatal(err)
	}

	// Create __init__.py
	if err := os.WriteFile(filepath.Join(libDir, "__init__.py"), []byte("# mathlib init"), 0644); err != nil {
		t.Fatal(err)
	}

	// Pack
	pkgFile := filepath.Join(t.TempDir(), "mathlib.zip")
	if _, err := Pack(srcDir, pkgFile, false); err != nil {
		t.Fatal(err)
	}

	// Create scriptling with package loader
	p := scriptling.New()

	loader := NewLoader()
	if err := loader.AddFromPath(pkgFile, false); err != nil {
		t.Fatal(err)
	}
	loader.SetFallback(p.GetLibraryLoader())
	p.SetLibraryLoader(loader)

	// Test importing and using the module
	result, err := p.Eval(`
import mathops
mathops.add(2, 3)
`)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if result.Inspect() != "5" {
		t.Errorf("expected 5, got %s", result.Inspect())
	}

	// Test another function
	result, err = p.Eval(`
import mathops
mathops.multiply(4, 5)
`)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if result.Inspect() != "20" {
		t.Errorf("expected 20, got %s", result.Inspect())
	}

	// Test constant
	result, err = p.Eval(`
import mathops
mathops.PI
`)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if result.Inspect() != "3.14159" {
		t.Errorf("expected 3.14159, got %s", result.Inspect())
	}
}

// TestIntegrationMainEntry tests running the main entry point from a package
func TestIntegrationMainEntry(t *testing.T) {
	// Create a package with main entry
	srcDir := t.TempDir()

	manifestContent := `
name = "cliapp"
version = "1.0.0"
main = "app.main"
`
	if err := os.WriteFile(filepath.Join(srcDir, "manifest.toml"), []byte(manifestContent), 0644); err != nil {
		t.Fatal(err)
	}

	libDir := filepath.Join(srcDir, "lib")
	if err := os.MkdirAll(libDir, 0755); err != nil {
		t.Fatal(err)
	}

	appCode := `
counter = 0

def main():
    global counter
    counter = 42
    return "main executed"

def get_counter():
    return counter
`
	if err := os.WriteFile(filepath.Join(libDir, "app.py"), []byte(appCode), 0644); err != nil {
		t.Fatal(err)
	}

	pkgFile := filepath.Join(t.TempDir(), "cliapp.zip")
	if _, err := Pack(srcDir, pkgFile, false); err != nil {
		t.Fatal(err)
	}

	// Load package
	loader := NewLoader()
	if err := loader.AddFromPath(pkgFile, false); err != nil {
		t.Fatal(err)
	}

	// Get main entry
	mod, fn, found := loader.GetMainEntry()
	if !found {
		t.Fatal("expected main entry to be found")
	}
	if mod != "app" {
		t.Errorf("expected module 'app', got %q", mod)
	}
	if fn != "main" {
		t.Errorf("expected function 'main', got %q", fn)
	}

	// Execute main entry
	p := scriptling.New()
	loader.SetFallback(p.GetLibraryLoader())
	p.SetLibraryLoader(loader)

	result, err := p.Eval("import app\napp.main()")
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if result.Inspect() != "main executed" {
		t.Errorf("expected 'main executed', got %s", result.Inspect())
	}

	// Verify state was set
	result, err = p.Eval("import app\napp.get_counter()")
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if result.Inspect() != "42" {
		t.Errorf("expected 42, got %s", result.Inspect())
	}
}

// TestIntegrationMultiplePackages tests using multiple packages together
func TestIntegrationMultiplePackages(t *testing.T) {
	tmpDir := t.TempDir()

	// Create package 1: stringslib
	pkg1Dir := filepath.Join(tmpDir, "stringslib")
	os.MkdirAll(filepath.Join(pkg1Dir, "lib"), 0755)
	os.WriteFile(filepath.Join(pkg1Dir, "manifest.toml"), []byte("name = \"stringslib\"\nversion = \"1.0.0\""), 0644)
	os.WriteFile(filepath.Join(pkg1Dir, "lib", "strings.py"), []byte(`
def uppercase(s):
    return s.upper()

def lowercase(s):
    return s.lower()
`), 0644)
	pkg1File := filepath.Join(tmpDir, "stringslib.zip")
	_, _ = Pack(pkg1Dir, pkg1File, false)

	// Create package 2: numberslib
	pkg2Dir := filepath.Join(tmpDir, "numberslib")
	os.MkdirAll(filepath.Join(pkg2Dir, "lib"), 0755)
	os.WriteFile(filepath.Join(pkg2Dir, "manifest.toml"), []byte("name = \"numberslib\"\nversion = \"1.0.0\""), 0644)
	os.WriteFile(filepath.Join(pkg2Dir, "lib", "numbers.py"), []byte(`
def double(n):
    return n * 2

def square(n):
    return n * n
`), 0644)
	pkg2File := filepath.Join(tmpDir, "numberslib.zip")
	_, _ = Pack(pkg2Dir, pkg2File, false)

	// Load both packages
	p := scriptling.New()
	loader := NewLoader()
	loader.AddFromPath(pkg1File, false)
	loader.AddFromPath(pkg2File, false)
	loader.SetFallback(p.GetLibraryLoader())
	p.SetLibraryLoader(loader)

	// Test using both
	result, err := p.Eval(`
import strings
import numbers
strings.uppercase("hello") + " " + str(numbers.double(21))
`)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if result.Inspect() != "HELLO 42" {
		t.Errorf("expected 'HELLO 42', got %s", result.Inspect())
	}
}

// TestIntegrationPackageOverrides tests that later packages override earlier ones
func TestIntegrationPackageOverrides(t *testing.T) {
	tmpDir := t.TempDir()

	// Create package 1 with utils.py
	pkg1Dir := filepath.Join(tmpDir, "pkg1")
	os.MkdirAll(filepath.Join(pkg1Dir, "lib"), 0755)
	os.WriteFile(filepath.Join(pkg1Dir, "manifest.toml"), []byte("name = \"pkg1\"\nversion = \"1.0.0\""), 0644)
	os.WriteFile(filepath.Join(pkg1Dir, "lib", "utils.py"), []byte("def get_value():\n    return 'from pkg1'"), 0644)
	pkg1File := filepath.Join(tmpDir, "pkg1.zip")
	_, _ = Pack(pkg1Dir, pkg1File, false)

	// Create package 2 with same utils.py
	pkg2Dir := filepath.Join(tmpDir, "pkg2")
	os.MkdirAll(filepath.Join(pkg2Dir, "lib"), 0755)
	os.WriteFile(filepath.Join(pkg2Dir, "manifest.toml"), []byte("name = \"pkg2\"\nversion = \"1.0.0\""), 0644)
	os.WriteFile(filepath.Join(pkg2Dir, "lib", "utils.py"), []byte("def get_value():\n    return 'from pkg2'"), 0644)
	pkg2File := filepath.Join(tmpDir, "pkg2.zip")
	_, _ = Pack(pkg2Dir, pkg2File, false)

	// Load both (pkg2 last = highest priority)
	p := scriptling.New()
	loader := NewLoader()
	loader.AddFromPath(pkg1File, false)
	loader.AddFromPath(pkg2File, false)
	loader.SetFallback(p.GetLibraryLoader())
	p.SetLibraryLoader(loader)

	// Should get pkg2's version
	result, err := p.Eval(`
import utils
utils.get_value()
`)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if result.Inspect() != "from pkg2" {
		t.Errorf("expected 'from pkg2', got %s", result.Inspect())
	}
}

// TestIntegrationWithFilesystemFallback tests that filesystem loader still works
func TestIntegrationWithFilesystemFallback(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a package
	pkgDir := filepath.Join(tmpDir, "pkg")
	os.MkdirAll(filepath.Join(pkgDir, "lib"), 0755)
	os.WriteFile(filepath.Join(pkgDir, "manifest.toml"), []byte("name = \"pkg\"\nversion = \"1.0.0\""), 0644)
	os.WriteFile(filepath.Join(pkgDir, "lib", "pkgmod.py"), []byte("def value():\n    return 'from package'"), 0644)
	pkgFile := filepath.Join(tmpDir, "pkg.zip")
	_, _ = Pack(pkgDir, pkgFile, false)

	// Create a local module on filesystem
	localModDir := filepath.Join(tmpDir, "local")
	os.MkdirAll(localModDir, 0755)
	os.WriteFile(filepath.Join(localModDir, "localmod.py"), []byte("def value():\n    return 'from filesystem'"), 0644)

	// Create scriptling with filesystem loader
	p := scriptling.New()
	fsLoader := libloader.NewMultiFilesystem(localModDir)
	p.SetLibraryLoader(fsLoader)

	// Add package loader with filesystem as fallback
	loader := NewLoader()
	loader.AddFromPath(pkgFile, false)
	loader.SetFallback(p.GetLibraryLoader())
	p.SetLibraryLoader(loader)

	// Test package module
	result, err := p.Eval(`
import pkgmod
pkgmod.value()
`)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if result.Inspect() != "from package" {
		t.Errorf("expected 'from package', got %s", result.Inspect())
	}

	// Test filesystem module (via fallback)
	result, err = p.Eval(`
import localmod
localmod.value()
`)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if result.Inspect() != "from filesystem" {
		t.Errorf("expected 'from filesystem', got %s", result.Inspect())
	}
}

// TestIntegrationSubmodules tests importing nested modules from packages
func TestIntegrationSubmodules(t *testing.T) {
	srcDir := t.TempDir()

	manifestContent := `
name = "nested"
version = "1.0.0"
`
	os.WriteFile(filepath.Join(srcDir, "manifest.toml"), []byte(manifestContent), 0644)

	libDir := filepath.Join(srcDir, "lib")
	os.MkdirAll(libDir, 0755)
	os.WriteFile(filepath.Join(libDir, "__init__.py"), []byte("# root init"), 0644)

	// Create nested package
	subDir := filepath.Join(libDir, "sub")
	os.MkdirAll(subDir, 0755)
	os.WriteFile(filepath.Join(subDir, "__init__.py"), []byte("sub_name = 'submodule'"), 0644)
	os.WriteFile(filepath.Join(subDir, "deep.py"), []byte(`
def deep_func():
    return "deep function"
`), 0644)

	pkgFile := filepath.Join(t.TempDir(), "nested.zip")
	if _, err := Pack(srcDir, pkgFile, false); err != nil {
		t.Fatal(err)
	}

	p := scriptling.New()
	loader := NewLoader()
	loader.AddFromPath(pkgFile, false)
	loader.SetFallback(p.GetLibraryLoader())
	p.SetLibraryLoader(loader)

	// Test submodule import
	result, err := p.Eval(`
import sub
sub.sub_name
`)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if result.Inspect() != "submodule" {
		t.Errorf("expected 'submodule', got %s", result.Inspect())
	}

	// Test deep module import
	result, err = p.Eval(`
import sub.deep
sub.deep.deep_func()
`)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if result.Inspect() != "deep function" {
		t.Errorf("expected 'deep function', got %s", result.Inspect())
	}
}
