package scriptling

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paularlott/scriptling/extlibs"
)

func TestOSLibraryPathSecurity(t *testing.T) {
	// Create a temp directory for testing
	tmpDir, err := os.MkdirTemp("", "scriptling-os-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a subdirectory for allowed access
	allowedDir := filepath.Join(tmpDir, "allowed")
	if err := os.Mkdir(allowedDir, 0755); err != nil {
		t.Fatalf("Failed to create allowed dir: %v", err)
	}

	// Create a file in the allowed directory
	testFile := filepath.Join(allowedDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create a directory outside allowed paths
	outsideDir := filepath.Join(tmpDir, "outside")
	if err := os.Mkdir(outsideDir, 0755); err != nil {
		t.Fatalf("Failed to create outside dir: %v", err)
	}
	outsideFile := filepath.Join(outsideDir, "secret.txt")
	if err := os.WriteFile(outsideFile, []byte("secret data"), 0644); err != nil {
		t.Fatalf("Failed to create outside file: %v", err)
	}

	t.Run("allowed path read", func(t *testing.T) {
		p := New()
		extlibs.RegisterOSLibrary(p, []string{allowedDir})

		script := `
import os
content = os.read_file("` + testFile + `")
content == "test content"
`
		result, err := p.Eval(script)
		if err != nil {
			t.Fatalf("Script error: %v", err)
		}
		if result.Inspect() != "true" {
			t.Errorf("Expected true, got %s", result.Inspect())
		}
	})

	t.Run("denied path read", func(t *testing.T) {
		p := New()
		extlibs.RegisterOSLibrary(p, []string{allowedDir})

		script := `
import os
content = os.read_file("` + outsideFile + `")
content
`
		_, err := p.Eval(script)
		if err == nil {
			t.Errorf("Expected error when reading outside allowed paths")
		}
	})

	t.Run("allowed path write", func(t *testing.T) {
		p := New()
		extlibs.RegisterOSLibrary(p, []string{allowedDir})

		newFile := filepath.Join(allowedDir, "new.txt")
		script := `
import os
os.write_file("` + newFile + `", "new content")
os.read_file("` + newFile + `") == "new content"
`
		result, err := p.Eval(script)
		if err != nil {
			t.Fatalf("Script error: %v", err)
		}
		if result.Inspect() != "true" {
			t.Errorf("Expected true, got %s", result.Inspect())
		}
	})

	t.Run("denied path write", func(t *testing.T) {
		p := New()
		extlibs.RegisterOSLibrary(p, []string{allowedDir})

		hackedFile := filepath.Join(outsideDir, "hacked.txt")
		script := `
import os
os.write_file("` + hackedFile + `", "hacked!")
`
		_, err := p.Eval(script)
		if err == nil {
			t.Errorf("Expected error when writing outside allowed paths")
		}
		// Verify file was not created
		if _, statErr := os.Stat(hackedFile); statErr == nil {
			t.Errorf("File should not have been created outside allowed paths")
		}
	})

	t.Run("path traversal attack blocked", func(t *testing.T) {
		p := New()
		extlibs.RegisterOSLibrary(p, []string{allowedDir})

		// Try to escape using ../
		traversalPath := filepath.Join(allowedDir, "..", "outside", "secret.txt")
		script := `
import os
content = os.read_file("` + traversalPath + `")
content
`
		_, err := p.Eval(script)
		if err == nil {
			t.Errorf("Expected error when using path traversal")
		}
	})

	t.Run("os.path.exists respects security", func(t *testing.T) {
		p := New()
		extlibs.RegisterOSLibrary(p, []string{allowedDir})

		script := `
import os.path
os.path.exists("` + outsideFile + `")
`
		_, err := p.Eval(script)
		if err == nil {
			t.Errorf("Expected error when checking path outside allowed directories")
		}
	})

	t.Run("listdir respects security", func(t *testing.T) {
		p := New()
		extlibs.RegisterOSLibrary(p, []string{allowedDir})

		script := `
import os
os.listdir("` + outsideDir + `")
`
		_, err := p.Eval(script)
		if err == nil {
			t.Errorf("Expected error when listing directory outside allowed paths")
		}
	})

	t.Run("no restrictions allows all", func(t *testing.T) {
		p := New()
		extlibs.RegisterOSLibrary(p, nil) // No restrictions

		script := `
import os
content = os.read_file("` + outsideFile + `")
content == "secret data"
`
		result, err := p.Eval(script)
		if err != nil {
			t.Fatalf("Script error with no restrictions: %v", err)
		}
		if result.Inspect() != "true" {
			t.Errorf("Expected true, got %s", result.Inspect())
		}
	})

	t.Run("symlink attack blocked", func(t *testing.T) {
		p := New()
		extlibs.RegisterOSLibrary(p, []string{allowedDir})

		// Create a symlink inside allowed dir that points to outside
		symlinkPath := filepath.Join(allowedDir, "evil_link")
		err := os.Symlink(outsideFile, symlinkPath)
		if err != nil {
			t.Skipf("Cannot create symlink (may need admin rights): %v", err)
		}
		defer os.Remove(symlinkPath)

		// Try to read through the symlink - should be blocked
		script := `
import os
content = os.read_file("` + symlinkPath + `")
content
`
		_, err = p.Eval(script)
		if err == nil {
			t.Errorf("Expected error when reading through symlink to outside path")
		}
	})

	t.Run("symlink dir attack blocked", func(t *testing.T) {
		p := New()
		extlibs.RegisterOSLibrary(p, []string{allowedDir})

		// Create a symlink to the outside directory
		symlinkDir := filepath.Join(allowedDir, "evil_dir_link")
		err := os.Symlink(outsideDir, symlinkDir)
		if err != nil {
			t.Skipf("Cannot create symlink (may need admin rights): %v", err)
		}
		defer os.Remove(symlinkDir)

		// Try to read file through the directory symlink
		script := `
import os
content = os.read_file("` + filepath.Join(symlinkDir, "secret.txt") + `")
content
`
		_, err = p.Eval(script)
		if err == nil {
			t.Errorf("Expected error when reading through directory symlink to outside path")
		}
	})
}
