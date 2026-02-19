package extlibs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/stdlib"
)

// ============================================================================
// OS Library Security Tests
// ============================================================================

func TestOSLibraryPathRestriction(t *testing.T) {
	// Create allowed and denied directories
	allowedDir := t.TempDir()
	deniedDir := t.TempDir()

	// Create test files
	allowedFile := filepath.Join(allowedDir, "allowed.txt")
	deniedFile := filepath.Join(deniedDir, "denied.txt")
	os.WriteFile(allowedFile, []byte("allowed content"), 0644)
	os.WriteFile(deniedFile, []byte("denied content"), 0644)

	p := scriptling.New()
	RegisterOSLibrary(p, []string{allowedDir})

	// Test reading allowed file
	result, err := p.Eval(`import os
os.read_file("` + allowedFile + `")
`)
	if err != nil {
		t.Fatalf("Failed to read allowed file: %v", err)
	}
	if str, _ := result.(*object.String); str == nil || str.Value != "allowed content" {
		t.Errorf("Expected 'allowed content', got %v", result)
	}

	// Test reading denied file should fail
	_, err = p.Eval(`import os
os.read_file("` + deniedFile + `")
`)
	if err == nil {
		t.Error("Expected error when reading denied file")
	} else if !strings.Contains(err.Error(), "access denied") {
		t.Errorf("Expected 'access denied' error, got: %s", err.Error())
	}
}

func TestOSLibraryPathTraversal(t *testing.T) {
	allowedDir := t.TempDir()

	// Create a file inside allowed dir
	allowedFile := filepath.Join(allowedDir, "test.txt")
	os.WriteFile(allowedFile, []byte("content"), 0644)

	p := scriptling.New()
	RegisterOSLibrary(p, []string{allowedDir})

	// Try path traversal
	traversalPath := filepath.Join(allowedDir, "..", "etc", "passwd")
	_, err := p.Eval(`import os
os.read_file("` + traversalPath + `")
`)
	if err == nil {
		t.Error("Expected error for path traversal")
	} else if !strings.Contains(err.Error(), "access denied") {
		t.Errorf("Expected 'access denied' error, got: %s", err.Error())
	}
}

func TestOSLibraryWriteRestriction(t *testing.T) {
	allowedDir := t.TempDir()
	deniedDir := t.TempDir()

	p := scriptling.New()
	RegisterOSLibrary(p, []string{allowedDir})

	// Test writing to allowed directory
	allowedFile := filepath.Join(allowedDir, "new.txt")
	result, err := p.Eval(`import os
os.write_file("` + allowedFile + `", "test data")
`)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}
	if _, ok := result.(*object.Null); !ok {
		t.Errorf("Expected Null for successful write, got %v", result)
	}

	// Verify file was written
	content, _ := os.ReadFile(allowedFile)
	if string(content) != "test data" {
		t.Errorf("File content mismatch: %s", string(content))
	}

	// Test writing to denied directory should fail
	deniedFile := filepath.Join(deniedDir, "denied.txt")
	_, err = p.Eval(`import os
os.write_file("` + deniedFile + `", "should fail")
`)
	if err == nil {
		t.Error("Expected error when writing to denied path")
	}

	// Verify file was NOT created
	if _, err := os.Stat(deniedFile); !os.IsNotExist(err) {
		t.Error("File should not have been created in denied directory")
	}
}

func TestOSLibraryListdirRestriction(t *testing.T) {
	allowedDir := t.TempDir()
	deniedDir := t.TempDir()

	// Create files
	os.WriteFile(filepath.Join(allowedDir, "file1.txt"), []byte("1"), 0644)
	os.WriteFile(filepath.Join(deniedDir, "file2.txt"), []byte("2"), 0644)

	p := scriptling.New()
	RegisterOSLibrary(p, []string{allowedDir})

	// Test listdir allowed
	result, err := p.Eval(`import os
os.listdir("` + allowedDir + `")
`)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}
	if list, ok := result.(*object.List); !ok {
		t.Errorf("Expected List, got %v", result)
	} else if len(list.Elements) != 1 {
		t.Errorf("Expected 1 file, got %d", len(list.Elements))
	}

	// Test listdir denied
	_, err = p.Eval(`import os
os.listdir("` + deniedDir + `")
`)
	if err == nil {
		t.Error("Expected error for listdir denied")
	}
}

func TestOSLibraryMkdirRestriction(t *testing.T) {
	allowedDir := t.TempDir()
	deniedDir := t.TempDir()

	p := scriptling.New()
	RegisterOSLibrary(p, []string{allowedDir})

	// Test mkdir allowed
	newAllowedDir := filepath.Join(allowedDir, "newdir")
	result, err := p.Eval(`import os
os.mkdir("` + newAllowedDir + `")
`)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}
	if _, ok := result.(*object.Null); !ok {
		t.Errorf("Expected Null, got %v", result)
	}
	if _, err := os.Stat(newAllowedDir); os.IsNotExist(err) {
		t.Error("Directory should have been created")
	}

	// Test mkdir denied
	newDeniedDir := filepath.Join(deniedDir, "newdir")
	_, err = p.Eval(`import os
os.mkdir("` + newDeniedDir + `")
`)
	if err == nil {
		t.Error("Expected error for mkdir denied")
	}
	if _, err := os.Stat(newDeniedDir); !os.IsNotExist(err) {
		t.Error("Directory should NOT have been created")
	}
}

func TestOSLibraryRemoveRestriction(t *testing.T) {
	allowedDir := t.TempDir()
	deniedDir := t.TempDir()

	// Create files
	allowedFile := filepath.Join(allowedDir, "todelete.txt")
	deniedFile := filepath.Join(deniedDir, "todelete.txt")
	os.WriteFile(allowedFile, []byte("test"), 0644)
	os.WriteFile(deniedFile, []byte("test"), 0644)

	p := scriptling.New()
	RegisterOSLibrary(p, []string{allowedDir})

	// Test remove allowed
	result, err := p.Eval(`import os
os.remove("` + allowedFile + `")
`)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}
	if _, ok := result.(*object.Null); !ok {
		t.Errorf("Expected Null, got %v", result)
	}
	if _, err := os.Stat(allowedFile); !os.IsNotExist(err) {
		t.Error("File should have been deleted")
	}

	// Test remove denied
	_, err = p.Eval(`import os
os.remove("` + deniedFile + `")
`)
	if err == nil {
		t.Error("Expected error for remove denied")
	}
	if _, err := os.Stat(deniedFile); os.IsNotExist(err) {
		t.Error("File should NOT have been deleted")
	}
}

func TestOSLibraryRenameRestriction(t *testing.T) {
	allowedDir := t.TempDir()
	deniedDir := t.TempDir()

	// Create files
	oldFile := filepath.Join(allowedDir, "old.txt")
	newFile := filepath.Join(allowedDir, "new.txt")
	deniedFile := filepath.Join(deniedDir, "denied.txt")
	os.WriteFile(oldFile, []byte("test"), 0644)
	os.WriteFile(deniedFile, []byte("test"), 0644)

	p := scriptling.New()
	RegisterOSLibrary(p, []string{allowedDir})

	// Test rename within allowed
	result, err := p.Eval(`import os
os.rename("` + oldFile + `", "` + newFile + `")
`)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}
	if _, ok := result.(*object.Null); !ok {
		t.Errorf("Expected Null, got %v", result)
	}
	if _, err := os.Stat(newFile); os.IsNotExist(err) {
		t.Error("File should have been renamed")
	}

	// Test rename to denied should fail
	anotherFile := filepath.Join(allowedDir, "another.txt")
	os.WriteFile(anotherFile, []byte("test"), 0644)
	_, err = p.Eval(`import os
os.rename("` + anotherFile + `", "` + deniedFile + `")
`)
	if err == nil {
		t.Error("Expected error for rename to denied")
	}
}

func TestOSLibraryMultipleAllowedPaths(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	dir3 := t.TempDir() // Not in allowed list

	file1 := filepath.Join(dir1, "file1.txt")
	file2 := filepath.Join(dir2, "file2.txt")
	file3 := filepath.Join(dir3, "file3.txt")
	os.WriteFile(file1, []byte("1"), 0644)
	os.WriteFile(file2, []byte("2"), 0644)
	os.WriteFile(file3, []byte("3"), 0644)

	p := scriptling.New()
	RegisterOSLibrary(p, []string{dir1, dir2})

	// Test access to first allowed path
	result, err := p.Eval(`import os
os.read_file("` + file1 + `")
`)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}
	if str, _ := result.(*object.String); str == nil || str.Value != "1" {
		t.Errorf("Expected '1', got %v", result)
	}

	// Test access to second allowed path
	result, err = p.Eval(`import os
os.read_file("` + file2 + `")
`)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}
	if str, _ := result.(*object.String); str == nil || str.Value != "2" {
		t.Errorf("Expected '2', got %v", result)
	}

	// Test access to non-allowed path should fail
	_, err = p.Eval(`import os
os.read_file("` + file3 + `")
`)
	if err == nil {
		t.Error("Expected error for non-allowed path")
	}
}

// ============================================================================
// OS.Path Library Security Tests
// ============================================================================

func TestOSPathLibraryRestriction(t *testing.T) {
	allowedDir := t.TempDir()
	deniedDir := t.TempDir()

	allowedFile := filepath.Join(allowedDir, "exists.txt")
	deniedFile := filepath.Join(deniedDir, "exists.txt")
	os.WriteFile(allowedFile, []byte("test"), 0644)
	os.WriteFile(deniedFile, []byte("test"), 0644)

	p := scriptling.New()
	RegisterOSLibrary(p, []string{allowedDir})

	// Test exists on allowed file
	result, err := p.Eval(`import os.path
os.path.exists("` + allowedFile + `")
`)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}
	if b, _ := result.(*object.Boolean); b == nil || !b.Value {
		t.Errorf("Expected True for existing allowed file, got %v", result)
	}

	// Test exists on denied file should fail
	_, err = p.Eval(`import os.path
os.path.exists("` + deniedFile + `")
`)
	if err == nil {
		t.Error("Expected error for denied file")
	}
}

func TestOSPathLibraryGetsizeRestriction(t *testing.T) {
	allowedDir := t.TempDir()
	deniedDir := t.TempDir()

	allowedFile := filepath.Join(allowedDir, "file.txt")
	deniedFile := filepath.Join(deniedDir, "file.txt")
	os.WriteFile(allowedFile, []byte("hello"), 0644)
	os.WriteFile(deniedFile, []byte("hello"), 0644)

	p := scriptling.New()
	RegisterOSLibrary(p, []string{allowedDir})

	// Test getsize on allowed file
	result, err := p.Eval(`import os.path
os.path.getsize("` + allowedFile + `")
`)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}
	if i, _ := result.(*object.Integer); i == nil || i.Value != 5 {
		t.Errorf("Expected 5, got %v", result)
	}

	// Test getsize on denied file should fail
	_, err = p.Eval(`import os.path
os.path.getsize("` + deniedFile + `")
`)
	if err == nil {
		t.Error("Expected error for denied file")
	}
}

// ============================================================================
// Pathlib Library Security Tests
// ============================================================================

func TestPathlibLibraryReadRestriction(t *testing.T) {
	allowedDir := t.TempDir()
	deniedDir := t.TempDir()

	allowedFile := filepath.Join(allowedDir, "file.txt")
	deniedFile := filepath.Join(deniedDir, "file.txt")
	os.WriteFile(allowedFile, []byte("allowed content"), 0644)
	os.WriteFile(deniedFile, []byte("denied content"), 0644)

	p := scriptling.New()
	RegisterPathlibLibrary(p, []string{allowedDir})

	// Test read allowed file
	result, err := p.Eval(`import pathlib
p = pathlib.Path("` + allowedFile + `")
p.read_text()
`)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}
	if str, _ := result.(*object.String); str == nil || str.Value != "allowed content" {
		t.Errorf("Expected 'allowed content', got %v", result)
	}

	// Test read denied file should fail
	_, err = p.Eval(`import pathlib
p = pathlib.Path("` + deniedFile + `")
p.read_text()
`)
	if err == nil {
		t.Error("Expected error for denied file")
	}
}

func TestPathlibLibraryWriteRestriction(t *testing.T) {
	allowedDir := t.TempDir()
	deniedDir := t.TempDir()

	p := scriptling.New()
	RegisterPathlibLibrary(p, []string{allowedDir})

	// Test write to allowed
	allowedFile := filepath.Join(allowedDir, "new.txt")
	result, err := p.Eval(`import pathlib
p = pathlib.Path("` + allowedFile + `")
p.write_text("test data")
`)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}
	if _, ok := result.(*object.Null); !ok {
		t.Errorf("Expected Null, got %v", result)
	}

	// Test write to denied
	deniedFile := filepath.Join(deniedDir, "denied.txt")
	_, err = p.Eval(`import pathlib
p = pathlib.Path("` + deniedFile + `")
p.write_text("should fail")
`)
	if err == nil {
		t.Error("Expected error for denied write")
	}
}

func TestPathlibLibraryMkdirRestriction(t *testing.T) {
	allowedDir := t.TempDir()
	deniedDir := t.TempDir()

	p := scriptling.New()
	RegisterPathlibLibrary(p, []string{allowedDir})

	// Test mkdir allowed
	newAllowedDir := filepath.Join(allowedDir, "newdir")
	result, err := p.Eval(`import pathlib
p = pathlib.Path("` + newAllowedDir + `")
p.mkdir()
`)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}
	if _, ok := result.(*object.Null); !ok {
		t.Errorf("Expected Null, got %v", result)
	}
	if _, err := os.Stat(newAllowedDir); os.IsNotExist(err) {
		t.Error("Directory should have been created")
	}

	// Test mkdir denied
	newDeniedDir := filepath.Join(deniedDir, "newdir")
	_, err = p.Eval(`import pathlib
p = pathlib.Path("` + newDeniedDir + `")
p.mkdir()
`)
	if err == nil {
		t.Error("Expected error for denied mkdir")
	}
}

func TestPathlibLibraryUnlinkRestriction(t *testing.T) {
	allowedDir := t.TempDir()
	deniedDir := t.TempDir()

	allowedFile := filepath.Join(allowedDir, "todelete.txt")
	deniedFile := filepath.Join(deniedDir, "todelete.txt")
	os.WriteFile(allowedFile, []byte("test"), 0644)
	os.WriteFile(deniedFile, []byte("test"), 0644)

	p := scriptling.New()
	RegisterPathlibLibrary(p, []string{allowedDir})

	// Test unlink allowed
	result, err := p.Eval(`import pathlib
p = pathlib.Path("` + allowedFile + `")
p.unlink()
`)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}
	if _, ok := result.(*object.Null); !ok {
		t.Errorf("Expected Null, got %v", result)
	}
	if _, err := os.Stat(allowedFile); !os.IsNotExist(err) {
		t.Error("File should have been deleted")
	}

	// Test unlink denied
	_, err = p.Eval(`import pathlib
p = pathlib.Path("` + deniedFile + `")
p.unlink()
`)
	if err == nil {
		t.Error("Expected error for denied unlink")
	}
	if _, err := os.Stat(deniedFile); os.IsNotExist(err) {
		t.Error("File should NOT have been deleted")
	}
}

func TestPathlibLibraryExistsRestriction(t *testing.T) {
	allowedDir := t.TempDir()
	deniedDir := t.TempDir()

	allowedFile := filepath.Join(allowedDir, "exists.txt")
	deniedFile := filepath.Join(deniedDir, "exists.txt")
	os.WriteFile(allowedFile, []byte("test"), 0644)
	os.WriteFile(deniedFile, []byte("test"), 0644)

	p := scriptling.New()
	RegisterPathlibLibrary(p, []string{allowedDir})

	// Test exists on allowed
	result, err := p.Eval(`import pathlib
p = pathlib.Path("` + allowedFile + `")
p.exists()
`)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}
	if b, _ := result.(*object.Boolean); b == nil || !b.Value {
		t.Errorf("Expected True for allowed file, got %v", result)
	}

	// Test exists on denied should return error
	_, err = p.Eval(`import pathlib
p = pathlib.Path("` + deniedFile + `")
p.exists()
`)
	if err == nil {
		t.Error("Expected error for denied exists")
	}
}

// ============================================================================
// Glob Library Security Tests
// ============================================================================

func TestGlobLibraryRestriction(t *testing.T) {
	allowedDir := t.TempDir()
	deniedDir := t.TempDir()

	// Create files
	os.WriteFile(filepath.Join(allowedDir, "file1.txt"), []byte("1"), 0644)
	os.WriteFile(filepath.Join(allowedDir, "file2.txt"), []byte("2"), 0644)
	os.WriteFile(filepath.Join(deniedDir, "secret.txt"), []byte("secret"), 0644)

	p := scriptling.New()
	RegisterGlobLibrary(p, []string{allowedDir})

	// Test glob in allowed directory
	result, err := p.Eval(`import glob
glob.glob("*.txt", "` + allowedDir + `")
`)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}
	list, ok := result.(*object.List)
	if !ok {
		t.Fatalf("Expected List, got %v", result)
	}
	if len(list.Elements) != 2 {
		t.Errorf("Expected 2 files in allowed dir, got %d", len(list.Elements))
	}

	// Test glob in denied directory should fail
	_, err = p.Eval(`import glob
glob.glob("*.txt", "` + deniedDir + `")
`)
	if err == nil {
		t.Error("Expected error for glob in denied dir")
	}
}

func TestGlobLibraryRecursiveRestriction(t *testing.T) {
	allowedDir := t.TempDir()
	deniedDir := t.TempDir()

	// Create nested structure in allowed
	subDir := filepath.Join(allowedDir, "subdir")
	os.Mkdir(subDir, 0755)
	os.WriteFile(filepath.Join(allowedDir, "file1.txt"), []byte("1"), 0644)
	os.WriteFile(filepath.Join(subDir, "file2.txt"), []byte("2"), 0644)

	// Create file in denied
	os.WriteFile(filepath.Join(deniedDir, "secret.txt"), []byte("secret"), 0644)

	p := scriptling.New()
	RegisterGlobLibrary(p, []string{allowedDir})

	// Test recursive glob in allowed
	result, err := p.Eval(`import glob
glob.glob("**/*.txt", "` + allowedDir + `")
`)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}
	list, ok := result.(*object.List)
	if !ok {
		t.Fatalf("Expected List, got %v", result)
	}
	if len(list.Elements) < 1 {
		t.Errorf("Expected at least 1 file from recursive glob, got %d", len(list.Elements))
	}

	// Verify all results are within allowed path
	for _, elem := range list.Elements {
		if str, ok := elem.(*object.String); ok {
			if !strings.HasPrefix(str.Value, allowedDir) {
				t.Errorf("Glob result outside allowed path: %s", str.Value)
			}
		}
	}
}

func TestGlobLibraryIglobRestriction(t *testing.T) {
	allowedDir := t.TempDir()
	deniedDir := t.TempDir()

	os.WriteFile(filepath.Join(allowedDir, "file1.txt"), []byte("1"), 0644)
	os.WriteFile(filepath.Join(deniedDir, "secret.txt"), []byte("secret"), 0644)

	p := scriptling.New()
	RegisterGlobLibrary(p, []string{allowedDir})

	// Test iglob in allowed directory
	result, err := p.Eval(`import glob
results = []
for f in glob.iglob("*.txt", "` + allowedDir + `"):
    results.append(f)
len(results)
`)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}
	if i, _ := result.(*object.Integer); i == nil || i.Value != 1 {
		t.Errorf("Expected 1 file from iglob, got %v", result)
	}

	// Test iglob in denied directory should fail
	_, err = p.Eval(`import glob
glob.iglob("*.txt", "` + deniedDir + `")
`)
	if err == nil {
		t.Error("Expected error for iglob in denied dir")
	}
}

// ============================================================================
// Symlink Attack Tests
// ============================================================================

func TestOSLibrarySymlinkAttack(t *testing.T) {
	allowedDir := t.TempDir()
	outsideDir := t.TempDir()

	// Create a file outside allowed dir
	outsideFile := filepath.Join(outsideDir, "secret.txt")
	os.WriteFile(outsideFile, []byte("secret data"), 0644)

	// Create symlink inside allowed dir pointing outside
	symlinkPath := filepath.Join(allowedDir, "link_to_secret")
	if err := os.Symlink(outsideFile, symlinkPath); err != nil {
		t.Skipf("Symlinks not supported: %v", err)
	}

	p := scriptling.New()
	RegisterOSLibrary(p, []string{allowedDir})

	// Reading through symlink should be denied
	_, err := p.Eval(`import os
os.read_file("` + symlinkPath + `")
`)
	if err == nil {
		t.Error("Expected error for symlink attack")
	} else if !strings.Contains(err.Error(), "access denied") {
		t.Errorf("Expected 'access denied' error, got: %s", err.Error())
	}
}

func TestPathlibLibrarySymlinkAttack(t *testing.T) {
	allowedDir := t.TempDir()
	outsideDir := t.TempDir()

	// Create a file outside allowed dir
	outsideFile := filepath.Join(outsideDir, "secret.txt")
	os.WriteFile(outsideFile, []byte("secret data"), 0644)

	// Create symlink inside allowed dir pointing outside
	symlinkPath := filepath.Join(allowedDir, "link_to_secret")
	if err := os.Symlink(outsideFile, symlinkPath); err != nil {
		t.Skipf("Symlinks not supported: %v", err)
	}

	p := scriptling.New()
	RegisterPathlibLibrary(p, []string{allowedDir})

	// Reading through symlink should be denied
	_, err := p.Eval(`import pathlib
p = pathlib.Path("` + symlinkPath + `")
p.read_text()
`)
	if err == nil {
		t.Error("Expected error for symlink attack")
	}
}

// ============================================================================
// Prefix Attack Tests
// ============================================================================

func TestOSLibraryPrefixAttack(t *testing.T) {
	allowedDir := t.TempDir()
	similarDir := allowedDir + "_other"

	os.Mkdir(similarDir, 0755)
	defer os.RemoveAll(similarDir)

	similarFile := filepath.Join(similarDir, "file.txt")
	os.WriteFile(similarFile, []byte("similar"), 0644)

	p := scriptling.New()
	RegisterOSLibrary(p, []string{allowedDir})

	// Access to similar-named directory should be denied
	_, err := p.Eval(`import os
os.read_file("` + similarFile + `")
`)
	if err == nil {
		t.Error("Expected error for prefix attack")
	}
}

// ============================================================================
// No Restrictions Tests
// ============================================================================

func TestOSLibraryNoRestrictions(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("content"), 0644)

	p := scriptling.New()
	RegisterOSLibrary(p, nil) // No restrictions

	// Should be able to read any file
	result, err := p.Eval(`import os
os.read_file("` + testFile + `")
`)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}
	if str, _ := result.(*object.String); str == nil || str.Value != "content" {
		t.Errorf("Expected 'content', got %v", result)
	}
}

func TestPathlibLibraryNoRestrictions(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("content"), 0644)

	p := scriptling.New()
	RegisterPathlibLibrary(p, nil) // No restrictions

	// Should be able to read any file
	result, err := p.Eval(`import pathlib
p = pathlib.Path("` + testFile + `")
p.read_text()
`)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}
	if str, _ := result.(*object.String); str == nil || str.Value != "content" {
		t.Errorf("Expected 'content', got %v", result)
	}
}

func TestGlobLibraryNoRestrictions(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("1"), 0644)

	p := scriptling.New()
	RegisterGlobLibrary(p, nil) // No restrictions

	result, err := p.Eval(`import glob
glob.glob("*.txt", "` + tmpDir + `")
`)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}
	if list, _ := result.(*object.List); len(list.Elements) != 1 {
		t.Errorf("Expected 1 file, got %d", len(list.Elements))
	}
}

// ============================================================================
// Subdirectory Access Tests
// ============================================================================

func TestOSLibrarySubdirectoryAccess(t *testing.T) {
	allowedDir := t.TempDir()

	// Create nested structure
	subDir := filepath.Join(allowedDir, "level1", "level2")
	os.MkdirAll(subDir, 0755)
	nestedFile := filepath.Join(subDir, "nested.txt")
	os.WriteFile(nestedFile, []byte("nested content"), 0644)

	p := scriptling.New()
	RegisterOSLibrary(p, []string{allowedDir})

	// Should be able to access nested file
	result, err := p.Eval(`import os
os.read_file("` + nestedFile + `")
`)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}
	if str, _ := result.(*object.String); str == nil || str.Value != "nested content" {
		t.Errorf("Expected 'nested content', got %v", result)
	}
}

func TestPathlibLibrarySubdirectoryAccess(t *testing.T) {
	allowedDir := t.TempDir()

	// Create nested structure
	subDir := filepath.Join(allowedDir, "level1", "level2")
	os.MkdirAll(subDir, 0755)
	nestedFile := filepath.Join(subDir, "nested.txt")
	os.WriteFile(nestedFile, []byte("nested content"), 0644)

	p := scriptling.New()
	RegisterPathlibLibrary(p, []string{allowedDir})

	// Should be able to access nested file
	result, err := p.Eval(`import pathlib
p = pathlib.Path("` + nestedFile + `")
p.read_text()
`)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}
	if str, _ := result.(*object.String); str == nil || str.Value != "nested content" {
		t.Errorf("Expected 'nested content', got %v", result)
	}
}

// ============================================================================
// Relative Path Tests
// ============================================================================

func TestOSLibraryRelativeAllowedPath(t *testing.T) {
	// Save current directory and restore after test
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	// Create a temp directory and change to it
	baseDir := t.TempDir()
	os.Chdir(baseDir)

	// Create subdirectories using relative paths
	os.Mkdir("testing", 0755)
	os.Mkdir("scripts", 0755)
	os.WriteFile("testing/file.txt", []byte("test content"), 0644)
	os.WriteFile("scripts/script.py", []byte("# script"), 0644)

	// Register with relative paths
	p := scriptling.New()
	RegisterOSLibrary(p, []string{"./testing", "./scripts"})

	// Should be able to read from ./testing
	result, err := p.Eval(`import os
os.read_file("./testing/file.txt")
`)
	if err != nil {
		t.Fatalf("Failed to read from relative path: %v", err)
	}
	if str, _ := result.(*object.String); str == nil || str.Value != "test content" {
		t.Errorf("Expected 'test content', got %v", result)
	}

	// Should be able to read from ./scripts
	result, err = p.Eval(`import os
os.read_file("./scripts/script.py")
`)
	if err != nil {
		t.Fatalf("Failed to read from second relative path: %v", err)
	}
	if str, _ := result.(*object.String); str == nil || str.Value != "# script" {
		t.Errorf("Expected '# script', got %v", result)
	}

	// Should NOT be able to access outside the relative paths
	// Create a file outside both allowed dirs
	os.WriteFile("outside.txt", []byte("secret"), 0644)
	_, err = p.Eval(`import os
os.read_file("outside.txt")
`)
	if err == nil {
		t.Error("Expected error when reading file outside relative allowed paths")
	}
}

func TestOSLibraryRelativePathTraversal(t *testing.T) {
	// Test ../../ patterns
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	baseDir := t.TempDir()

	// Create structure: baseDir/allowed/file.txt
	//                    baseDir/outside/secret.txt
	allowedDir := filepath.Join(baseDir, "allowed")
	outsideDir := filepath.Join(baseDir, "outside")
	os.Mkdir(allowedDir, 0755)
	os.Mkdir(outsideDir, 0755)
	os.WriteFile(filepath.Join(allowedDir, "file.txt"), []byte("allowed"), 0644)
	os.WriteFile(filepath.Join(outsideDir, "secret.txt"), []byte("secret"), 0644)

	// Change to a nested directory so we can use ../../
	nestedDir := filepath.Join(allowedDir, "nested", "deep")
	os.MkdirAll(nestedDir, 0755)
	os.Chdir(nestedDir)

	// Register with relative path pointing to allowedDir via ../../..
	// From nested/deep we need ../../../allowed to get back to allowed
	p := scriptling.New()
	RegisterOSLibrary(p, []string{"../../../allowed"})

	// Should be able to read from the allowed directory
	result, err := p.Eval(`import os
os.read_file("../../../allowed/file.txt")
`)
	if err != nil {
		t.Fatalf("Failed to read from relative traversal path: %v", err)
	}
	if str, _ := result.(*object.String); str == nil || str.Value != "allowed" {
		t.Errorf("Expected 'allowed', got %v", result)
	}

	// Should NOT be able to access ../outside even though we can traverse
	_, err = p.Eval(`import os
os.read_file("../../../outside/secret.txt")
`)
	if err == nil {
		t.Error("Expected error when reading file outside allowed relative paths")
	}
}

func TestOSLibraryMultipleRelativePaths(t *testing.T) {
	// Test multiple relative paths like ./testing ./scripts ../../tools etc
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	baseDir := t.TempDir()

	// Create structure
	os.Mkdir(filepath.Join(baseDir, "testing"), 0755)
	os.Mkdir(filepath.Join(baseDir, "scripts"), 0755)
	os.Mkdir(filepath.Join(baseDir, "tools"), 0755)
	os.WriteFile(filepath.Join(baseDir, "testing", "a.txt"), []byte("testing"), 0644)
	os.WriteFile(filepath.Join(baseDir, "scripts", "b.txt"), []byte("scripts"), 0644)
	os.WriteFile(filepath.Join(baseDir, "tools", "c.txt"), []byte("tools"), 0644)
	os.WriteFile(filepath.Join(baseDir, "outside.txt"), []byte("outside"), 0644)

	// Change to a subdirectory
	subDir := filepath.Join(baseDir, "subdir")
	os.Mkdir(subDir, 0755)
	os.Chdir(subDir)

	// Register with mixed relative paths: ./testing would be subdir/testing (doesn't exist)
	// But ../testing, ../scripts, ../tools would work
	p := scriptling.New()
	RegisterOSLibrary(p, []string{"../testing", "../scripts", "../tools"})

	// Test access to all three allowed directories
	result, err := p.Eval(`import os
os.read_file("../testing/a.txt")
`)
	if err != nil {
		t.Fatalf("Failed to read from ../testing: %v", err)
	}
	if str, _ := result.(*object.String); str.Value != "testing" {
		t.Errorf("Expected 'testing', got %v", result)
	}

	result, err = p.Eval(`import os
os.read_file("../scripts/b.txt")
`)
	if err != nil {
		t.Fatalf("Failed to read from ../scripts: %v", err)
	}
	if str, _ := result.(*object.String); str.Value != "scripts" {
		t.Errorf("Expected 'scripts', got %v", result)
	}

	result, err = p.Eval(`import os
os.read_file("../tools/c.txt")
`)
	if err != nil {
		t.Fatalf("Failed to read from ../tools: %v", err)
	}
	if str, _ := result.(*object.String); str.Value != "tools" {
		t.Errorf("Expected 'tools', got %v", result)
	}

	// Should NOT be able to access outside.txt
	_, err = p.Eval(`import os
os.read_file("../outside.txt")
`)
	if err == nil {
		t.Error("Expected error when reading file outside all allowed paths")
	}
}

func TestPathlibLibraryRelativePath(t *testing.T) {
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	baseDir := t.TempDir()
	os.Chdir(baseDir)

	os.Mkdir("data", 0755)
	os.WriteFile("data/file.txt", []byte("relative content"), 0644)

	p := scriptling.New()
	RegisterPathlibLibrary(p, []string{"./data"})

	// Should work with relative path
	result, err := p.Eval(`import pathlib
p = pathlib.Path("./data/file.txt")
p.read_text()
`)
	if err != nil {
		t.Fatalf("Failed to read with pathlib relative: %v", err)
	}
	if str, _ := result.(*object.String); str.Value != "relative content" {
		t.Errorf("Expected 'relative content', got %v", result)
	}

	// Should NOT allow access outside
	os.WriteFile("outside.txt", []byte("secret"), 0644)
	_, err = p.Eval(`import pathlib
p = pathlib.Path("outside.txt")
p.read_text()
`)
	if err == nil {
		t.Error("Expected error for file outside relative allowed path")
	}
}

func TestGlobLibraryRelativePath(t *testing.T) {
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	baseDir := t.TempDir()
	os.Chdir(baseDir)

	os.Mkdir("files", 0755)
	os.WriteFile("files/a.txt", []byte("1"), 0644)
	os.WriteFile("files/b.txt", []byte("2"), 0644)
	os.WriteFile("secret.txt", []byte("secret"), 0644)

	p := scriptling.New()
	RegisterGlobLibrary(p, []string{"./files"})

	// Should work with relative path
	result, err := p.Eval(`import glob
glob.glob("*.txt", "./files")
`)
	if err != nil {
		t.Fatalf("Failed to glob with relative path: %v", err)
	}
	if list, _ := result.(*object.List); len(list.Elements) != 2 {
		t.Errorf("Expected 2 files, got %d", len(list.Elements))
	}

	// Should NOT allow glob in parent
	_, err = p.Eval(`import glob
glob.glob("*.txt", ".")
`)
	if err == nil {
		t.Error("Expected error for glob outside relative allowed path")
	}
}

func TestSandboxRelativePath(t *testing.T) {
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	// Create allowed and denied directories with relative paths
	baseDir := t.TempDir()
	os.Chdir(baseDir)

	os.Mkdir("allowed", 0755)
	os.Mkdir("denied", 0755)

	allowedScript := filepath.Join(baseDir, "allowed", "script.py")
	deniedScript := filepath.Join(baseDir, "denied", "script.py")
	os.WriteFile(allowedScript, []byte("result = 'from allowed'"), 0644)
	os.WriteFile(deniedScript, []byte("result = 'from denied'"), 0644)

	// Setup sandbox with relative path restriction
	ResetRuntime()
	SetSandboxFactory(nil)

	// Register runtime with sandbox restricted to ./allowed
	p := scriptling.New()
	stdlib.RegisterAll(p)
	RegisterRuntimeLibraryAll(p, []string{"./allowed"})

	SetSandboxFactory(func() SandboxInstance {
		newP := scriptling.New()
		stdlib.RegisterAll(newP)
		RegisterRuntimeLibraryAll(newP, []string{"./allowed"})
		return newP
	})

	// Should be able to exec from allowed
	script := `
import scriptling.runtime as runtime

env = runtime.sandbox.create()
env.exec_file("` + allowedScript + `")
env.get("result")
`
	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}
	if str, _ := result.(*object.String); str.Value != "from allowed" {
		t.Errorf("Expected 'from allowed', got %v", result)
	}

	// Should NOT be able to exec from denied
	script2 := `
import scriptling.runtime as runtime

env = runtime.sandbox.create()
env.exec_file("` + deniedScript + `")
env.exit_code()
`
	result, err = p.Eval(script2)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}
	if i, _ := result.(*object.Integer); i.Value != 1 {
		t.Errorf("Expected exit code 1 for denied relative path, got %d", i.Value)
	}
}
