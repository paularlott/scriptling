package fssecurity

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfig_IsPathAllowed_NoRestrictions(t *testing.T) {
	config := Config{}

	// With nil AllowedPaths, all paths should be allowed
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

func TestConfig_IsPathAllowed_DenyAll(t *testing.T) {
	// Empty slice (not nil) means deny all
	config := Config{
		AllowedPaths: []string{}, // Empty slice, not nil
	}

	// All paths should be denied
	tests := []string{
		"/",
		"/home/user",
		"./relative",
		"/etc/passwd",
		"/tmp",
	}

	for _, path := range tests {
		if config.IsPathAllowed(path) {
			t.Errorf("Expected path %s to be denied with empty AllowedPaths", path)
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

// A single call that creates several directory levels at once (a fresh
// extraction directory, a multi-segment config path) means neither the
// final path nor its immediate parent exist yet at check time. Stopping the
// symlink resolution at one level up (instead of walking to the nearest
// existing ancestor) falls back to an unresolved path and can wrongly deny
// a legitimate nested path whenever an allowed directory itself sits under
// a symlink (e.g. macOS's /tmp -> /private/tmp).
func TestConfig_IsPathAllowed_MultiLevelNonExistentPath(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "fssecurity_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := Config{AllowedPaths: []string{tempDir}}

	// Neither "a", "a/b", nor "a/b/c.txt" exist yet.
	deepNonExistent := filepath.Join(tempDir, "a", "b", "c.txt")
	if !config.IsPathAllowed(deepNonExistent) {
		t.Errorf("multi-level non-existent path within an allowed directory should be allowed, got denied for %s", deepNonExistent)
	}
}

// The symlink case from TestConfig_IsPathAllowed_Symlinks, but with the
// escaping symlink itself several levels deep and additional non-existent
// path components trailing after it — proving the walk-to-nearest-ancestor
// resolution still finds and resolves the symlink rather than giving up
// after one level (which would fall back to an unresolved, and therefore
// wrongly-allowed, path).
func TestConfig_IsPathAllowed_MultiLevelSymlinkEscape(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "fssecurity_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	outsideDir := filepath.Join(tempDir, "..", "outside_multilevel")
	if err := os.MkdirAll(outsideDir, 0755); err != nil {
		t.Fatalf("Failed to create outside dir: %v", err)
	}
	defer os.RemoveAll(outsideDir)

	nested := filepath.Join(tempDir, "sub")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatalf("Failed to create nested dir: %v", err)
	}
	symlinkPath := filepath.Join(nested, "evil")
	if err := os.Symlink(outsideDir, symlinkPath); err != nil {
		t.Skipf("Symlinks not supported or failed to create: %v", err)
	}

	config := Config{AllowedPaths: []string{tempDir}}

	// "evil" exists (it's the symlink); "new" and "file.txt" do not.
	escaping := filepath.Join(symlinkPath, "new", "file.txt")
	if config.IsPathAllowed(escaping) {
		t.Errorf("path through a multi-level symlink escape should be denied, got allowed for %s", escaping)
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

// A DANGLING symlink inside the allowed directory (its target does not exist
// yet) must be followed to where it points, not treated as a plain file.
// EvalSymlinks fails on a dangling link, so the previous resolver fell back
// to the link's own path and wrongly allowed a write that the kernel would
// land outside the jail. Covers relative, absolute, chained and
// link-then-more-path forms, for a write (CWE-22, vuln-0011).
func TestConfig_IsPathAllowed_DanglingSymlinkEscape(t *testing.T) {
	base, err := os.MkdirTemp("", "fssecurity_dangle")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(base)
	jail := filepath.Join(base, "jail")
	outside := filepath.Join(base, "outside")
	for _, d := range []string{jail, filepath.Join(jail, "sub"), outside} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	mklink := func(name, target string) string {
		p := filepath.Join(jail, name)
		if err := os.Symlink(target, p); err != nil {
			t.Skipf("symlinks unsupported: %v", err)
		}
		return p
	}
	config := Config{AllowedPaths: []string{jail}}

	// Dangling links (targets do not exist) pointing outside the jail.
	relLink := mklink("rel", filepath.Join("..", "outside", "planted.txt"))
	absLink := mklink("abs", filepath.Join(outside, "planted2.txt"))
	dirLink := mklink("dir", outside) // existing dir, path continues through it
	chain := mklink("chain", "rel")   // link -> link -> outside

	for _, tc := range []struct {
		name string
		path string
	}{
		{"relative dangling link", relLink},
		{"absolute dangling link", absLink},
		{"write through link to existing outside dir", filepath.Join(dirLink, "new.txt")},
		{"chained links", chain},
		{"link then non-existent components", filepath.Join(dirLink, "a", "b.txt")},
	} {
		if config.IsPathAllowed(tc.path) {
			t.Errorf("%s: %s should be denied (escapes the jail)", tc.name, tc.path)
		}
	}

	// A dangling link pointing to a NEW file inside the jail is still allowed.
	insideLink := mklink("inside", filepath.Join("sub", "willcreate.txt"))
	if !config.IsPathAllowed(insideLink) {
		t.Errorf("dangling link to a new path inside the jail should be allowed: %s", insideLink)
	}
}

// "link/.." means the parent of the link's TARGET to the kernel, so a path
// that is lexically inside the jail can resolve outside it. filepath.Clean
// would collapse "link/.." to the jail before any symlink is followed, which
// wrongly allowed it; ResolvePath must apply ".." only after resolving the
// link (vuln-0011, lexical-clean variant).
func TestConfig_IsPathAllowed_SymlinkParentEscape(t *testing.T) {
	base, err := os.MkdirTemp("", "fssecurity_parent")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(base)
	jail := filepath.Join(base, "jail")
	outsideSub := filepath.Join(base, "outside", "a")
	if err := os.MkdirAll(outsideSub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(jail, 0o755); err != nil {
		t.Fatal(err)
	}
	// jail/l -> <base>/outside/a ; so jail/l/../secret resolves to
	// <base>/outside/secret, outside the jail.
	if err := os.Symlink(outsideSub, filepath.Join(jail, "l")); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}
	config := Config{AllowedPaths: []string{jail}}
	// Build the path literally: filepath.Join would clean "l/.." away
	// before any symlink is considered, which is the bug under test.
	escaping := jail + string(os.PathSeparator) + "l" + string(os.PathSeparator) + ".." + string(os.PathSeparator) + "secret.txt"
	if config.IsPathAllowed(escaping) {
		t.Errorf("%s resolves outside the jail and must be denied", escaping)
	}
}

// A symlink loop must fail closed (denied), not hang.
func TestConfig_IsPathAllowed_SymlinkLoop(t *testing.T) {
	base, err := os.MkdirTemp("", "fssecurity_loop")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(base)
	a := filepath.Join(base, "a")
	b := filepath.Join(base, "b")
	if err := os.Symlink(b, a); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}
	if err := os.Symlink(a, b); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}
	config := Config{AllowedPaths: []string{base}}
	if config.IsPathAllowed(filepath.Join(a, "x")) {
		t.Errorf("a symlink loop must be denied, not allowed")
	}
}

// A filename whose rune has low byte 0x2F (e.g. U+012F 'į') must not be split
// as if it contained '/': the resolver compares runes, not truncated bytes,
// so a legitimate name resolves to itself rather than through a spurious
// intermediate component.
func TestConfig_IsPathAllowed_MultibyteNotSplit(t *testing.T) {
	base, err := os.MkdirTemp("", "fssecurity_rune")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(base)
	jail := filepath.Join(base, "jail")
	if err := os.MkdirAll(jail, 0o755); err != nil {
		t.Fatal(err)
	}
	config := Config{AllowedPaths: []string{jail}}

	name := "fįle.txt" // 'į' truncates to 0x2F under a byte cast
	parts := splitPath(name)
	if len(parts) != 1 || parts[0] != name {
		t.Fatalf("splitPath(%q) = %#v, want one component", name, parts)
	}
	// The resolved path must keep the real filename, not a split version.
	resolved, ok := ResolvePath(filepath.Join(jail, name))
	if !ok || filepath.Base(resolved) != name {
		t.Fatalf("ResolvePath mangled %q -> %q (ok=%v)", name, resolved, ok)
	}
	if !config.IsPathAllowed(filepath.Join(jail, name)) {
		t.Errorf("a legitimate in-jail file with a multibyte name was denied")
	}
}
