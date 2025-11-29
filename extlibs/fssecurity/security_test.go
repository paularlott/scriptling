package fssecurity

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfig_IsPathAllowed_NoRestrictions(t *testing.T) {
	config := Config{}

	// With no restrictions, all paths should be allowed
	tests := []string{
		"/",
		"/home/user",
		"./relative",
		"../parent",
		"/etc/passwd",
		"C:\\Windows\\System32", // Even on Unix, should allow
	}

	for _, path := range tests {
		if !config.IsPathAllowed(path) {
			t.Errorf("Expected path %s to be allowed with no restrictions", path)
		}
	}
}

func TestConfig_IsPathAllowed_AllowedPaths(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "fssecurity_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create subdirectories
	subDir := filepath.Join(tempDir, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}

	config := Config{
		AllowedPaths: []string{tempDir},
	}

	tests := []struct {
		path     string
		expected bool
	}{
		{tempDir, true}, // Exact match
		{subDir, true},  // Subdirectory
		{filepath.Join(subDir, "file.txt"), true},     // File in subdirectory
		{filepath.Join(tempDir, "../outside"), false}, // Outside
		{"/etc/passwd", false},                        // Completely outside
		{"./relative", false},                         // Relative path outside
	}

	for _, test := range tests {
		result := config.IsPathAllowed(test.path)
		if result != test.expected {
			t.Errorf("Path %s: expected %v, got %v", test.path, test.expected, result)
		}
	}
}

func TestConfig_IsPathAllowed_PathTraversal(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "fssecurity_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := Config{
		AllowedPaths: []string{tempDir},
	}

	// Path traversal attempts
	traversalPaths := []string{
		filepath.Join(tempDir, "..", "outside"),
		filepath.Join(tempDir, "..", "..", "etc", "passwd"),
		filepath.Join(tempDir, "subdir", "..", "..", "outside"),
		filepath.Join(tempDir, ".", "file.txt"),            // Should still be allowed
		filepath.Join(tempDir, "subdir", "..", "file.txt"), // Should be allowed
	}

	for _, path := range traversalPaths {
		cleanPath := filepath.Clean(path)
		rel, err := filepath.Rel(tempDir, cleanPath)
		isInside := err == nil && !strings.HasPrefix(rel, "..")
		if !isInside {
			// Path is outside, should be denied
			if config.IsPathAllowed(path) {
				t.Errorf("Path traversal %s should be denied", path)
			}
		} else {
			// Path is inside, should be allowed
			if !config.IsPathAllowed(path) {
				t.Errorf("Valid path %s should be allowed", path)
			}
		}
	}
}

func TestConfig_IsPathAllowed_PrefixAttack(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "fssecurity_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a directory with similar prefix
	similarDir := tempDir + "_other"
	if err := os.Mkdir(similarDir, 0755); err != nil {
		t.Fatalf("Failed to create similar dir: %v", err)
	}
	defer os.RemoveAll(similarDir)

	config := Config{
		AllowedPaths: []string{tempDir},
	}

	// These should be denied
	deniedPaths := []string{
		similarDir,
		filepath.Join(similarDir, "file.txt"),
		tempDir + "_other_file",
	}

	for _, path := range deniedPaths {
		if config.IsPathAllowed(path) {
			t.Errorf("Prefix attack path %s should be denied", path)
		}
	}

	// These should be allowed
	allowedPaths := []string{
		tempDir,
		filepath.Join(tempDir, "file.txt"),
	}

	for _, path := range allowedPaths {
		if !config.IsPathAllowed(path) {
			t.Errorf("Valid path %s should be allowed", path)
		}
	}
}

func TestConfig_IsPathAllowed_Symlinks(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "fssecurity_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a file inside allowed directory
	allowedFile := filepath.Join(tempDir, "allowed.txt")
	if err := os.WriteFile(allowedFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create a symlink inside allowed directory pointing outside
	outsideDir := filepath.Join(tempDir, "..", "outside")
	os.MkdirAll(outsideDir, 0755)
	defer os.RemoveAll(outsideDir)

	symlinkPath := filepath.Join(tempDir, "symlink_to_outside")
	if err := os.Symlink(outsideDir, symlinkPath); err != nil {
		t.Skipf("Symlinks not supported or failed to create: %v", err)
	}

	config := Config{
		AllowedPaths: []string{tempDir},
	}

	// Symlink itself should be denied if it points outside (security feature)
	if config.IsPathAllowed(symlinkPath) {
		t.Errorf("Symlink pointing outside should be denied")
	}

	// But accessing through symlink should be denied
	symlinkTarget := filepath.Join(symlinkPath, "file.txt")
	if config.IsPathAllowed(symlinkTarget) {
		t.Errorf("Access through symlink to outside should be denied")
	}

	// Valid symlink within allowed directory
	validSymlink := filepath.Join(tempDir, "symlink_to_file")
	if err := os.Symlink(allowedFile, validSymlink); err == nil {
		if !config.IsPathAllowed(validSymlink) {
			t.Errorf("Symlink to file within allowed directory should be allowed")
		}
	}
}

func TestConfig_IsPathAllowed_NonExistentPaths(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "fssecurity_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := Config{
		AllowedPaths: []string{tempDir},
	}

	// Non-existent paths within allowed directory should be allowed (for write operations)
	validNonExistent := filepath.Join(tempDir, "new_file.txt")
	if !config.IsPathAllowed(validNonExistent) {
		t.Errorf("Non-existent path within allowed directory should be allowed")
	}

	// Non-existent paths outside should be denied
	invalidNonExistent := "/etc/new_file.txt"
	if config.IsPathAllowed(invalidNonExistent) {
		t.Errorf("Non-existent path outside allowed directories should be denied")
	}
}

func TestConfig_IsPathAllowed_MultipleAllowedPaths(t *testing.T) {
	tempDir1, err := os.MkdirTemp("", "fssecurity_test1")
	if err != nil {
		t.Fatalf("Failed to create temp dir1: %v", err)
	}
	defer os.RemoveAll(tempDir1)

	tempDir2, err := os.MkdirTemp("", "fssecurity_test2")
	if err != nil {
		t.Fatalf("Failed to create temp dir2: %v", err)
	}
	defer os.RemoveAll(tempDir2)

	config := Config{
		AllowedPaths: []string{tempDir1, tempDir2},
	}

	// Both should be allowed
	if !config.IsPathAllowed(filepath.Join(tempDir1, "file.txt")) {
		t.Errorf("Path in first allowed directory should be allowed")
	}
	if !config.IsPathAllowed(filepath.Join(tempDir2, "file.txt")) {
		t.Errorf("Path in second allowed directory should be allowed")
	}

	// Outside should be denied
	if config.IsPathAllowed("/etc/passwd") {
		t.Errorf("Path outside allowed directories should be denied")
	}
}
