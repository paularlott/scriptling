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
		if result.Inspect() != "True" {
			t.Errorf("Expected True, got %s", result.Inspect())
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
		if result.Inspect() != "True" {
			t.Errorf("Expected True, got %s", result.Inspect())
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
		if result.Inspect() != "True" {
			t.Errorf("Expected True, got %s", result.Inspect())
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

// A dangling symlink planted inside the jail (target does not exist yet) must
// not become a write primitive outside it: the jail check follows the link,
// even though it cannot be resolved by EvalSymlinks (CWE-22, vuln-0011).
func TestOSSymlinkDanglingWriteBlocked(t *testing.T) {
	base, err := os.MkdirTemp("", "scriptling-dangle")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(base)
	jail := filepath.Join(base, "jail")
	outside := filepath.Join(base, "outside")
	for _, d := range []string{jail, outside} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// A dangling link inside the jail pointing outside, planted directly.
	link := filepath.Join(jail, "planted")
	if err := os.Symlink(filepath.Join(outside, "evil.txt"), link); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}

	p := New()
	extlibs.RegisterOSLibrary(p, []string{jail})
	_, err = p.Eval(`import os
os.write_file("` + link + `", "pwned")`)
	if err == nil {
		t.Fatal("write through a dangling link out of the jail was allowed")
	}
	if _, statErr := os.Stat(filepath.Join(outside, "evil.txt")); statErr == nil {
		t.Fatal("a file was created outside the jail through the link")
	}
}

// os.symlink must refuse to create a link whose relative target escapes the
// jail, resolving the target against the link's directory (not the process
// working directory), which is where the kernel resolves it (vuln-0011 part 2).
func TestOSSymlinkRelativeTargetValidatedAgainstLinkDir(t *testing.T) {
	base, err := os.MkdirTemp("", "scriptling-symrel")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(base)
	jail := filepath.Join(base, "jail")
	if err := os.MkdirAll(filepath.Join(jail, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jail, "data.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	p := New()
	extlibs.RegisterOSLibrary(p, []string{jail})

	// jail/escape -> ../outside/x  (relative target leaves the jail).
	_, err = p.Eval(`import os
os.symlink("../outside/evil", "` + filepath.Join(jail, "escape") + `")`)
	if err == nil {
		t.Error("os.symlink created an escaping relative link")
	}
	if _, statErr := os.Lstat(filepath.Join(jail, "escape")); statErr == nil {
		t.Error("the escaping link was created on disk")
	}

	// A relative link that stays inside the jail is still allowed.
	if _, err := p.Eval(`import os
os.symlink("../data.txt", "` + filepath.Join(jail, "sub", "good") + `")`); err != nil {
		t.Errorf("a legitimate in-jail relative link was rejected: %v", err)
	}
	if _, statErr := os.Lstat(filepath.Join(jail, "sub", "good")); statErr != nil {
		t.Errorf("the legitimate link was not created: %v", statErr)
	}
}

// os.makedirs on a path with a missing component, then "..", then a link to
// the jail's parent must be denied: os.MkdirAll creates the missing directory
// first and the kernel then follows the link out of the jail. Likewise a path
// that leaves the jail, creates a directory, and comes back.
func TestOSMakedirsMissingComponentThroughLink(t *testing.T) {
	base, err := os.MkdirTemp("", "scriptling-mkdirs-jail")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(base)
	jail := filepath.Join(base, "jail")
	outside := filepath.Join(base, "outside")
	for _, d := range []string{jail, outside} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink("..", filepath.Join(jail, "l")); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}

	p := New()
	extlibs.RegisterOSLibrary(p, []string{jail})
	sep := string(os.PathSeparator)
	for _, tc := range []struct {
		path    string
		created string
	}{
		{jail + sep + "n" + sep + ".." + sep + "l" + sep + sep + "pwned", filepath.Join(base, "pwned")},
		{outside + sep + "new" + sep + ".." + sep + ".." + sep + "jail" + sep + "x", filepath.Join(outside, "new")},
	} {
		if _, err := p.Eval(`import os
os.makedirs("` + tc.path + `", exist_ok=True)`); err == nil {
			t.Errorf("os.makedirs(%q) was allowed", tc.path)
		}
		if _, statErr := os.Stat(tc.created); statErr == nil {
			t.Errorf("os.makedirs(%q) created %s outside the jail", tc.path, tc.created)
		}
	}

	// Legitimate nested and unclean paths inside the jail still work.
	for _, path := range []string{
		filepath.Join(jail, "a", "b", "c"),
		jail + sep + "d" + sep + ".." + sep + "e" + sep + sep + "f",
	} {
		if _, err := p.Eval(`import os
os.makedirs("` + path + `", exist_ok=True)`); err != nil {
			t.Errorf("os.makedirs(%q) inside the jail was rejected: %v", path, err)
		}
	}
	if _, err := os.Stat(filepath.Join(jail, "e", "f")); err != nil {
		t.Errorf("legitimate unclean makedirs did not create jail/e/f: %v", err)
	}
}

// A relative symlink target is resolved from the link's directory as the
// kernel walks it, never cleaned first: with x/s -> .. (the jail), "x/s/.."
// is the jail's parent, though filepath.Join cleans it to "x".
func TestOSSymlinkTargetResolvedUncleaned(t *testing.T) {
	base, err := os.MkdirTemp("", "scriptling-symclean")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(base)
	jail := filepath.Join(base, "jail")
	if err := os.MkdirAll(filepath.Join(jail, "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(jail, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("..", filepath.Join(jail, "x", "s")); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}

	p := New()
	extlibs.RegisterOSLibrary(p, []string{jail})
	for _, src := range []string{"x/s/..", "x/s/../secret", "missing/../x/s/.."} {
		dst := filepath.Join(jail, "l")
		if _, err := p.Eval(`import os
os.symlink("` + src + `", "` + dst + `")`); err == nil {
			t.Errorf("os.symlink(%q) escaping through x/s was allowed", src)
		}
		if _, statErr := os.Lstat(dst); statErr == nil {
			t.Fatalf("os.symlink(%q) created the escaping link", src)
		}
	}

	// Through the same link, a target that stays inside is allowed, and so
	// is an absolute in-jail target, checked as is.
	for i, src := range []string{"x/s/sub", filepath.Join(jail, "sub")} {
		dst := filepath.Join(jail, "ok"+string(rune('0'+i)))
		if _, err := p.Eval(`import os
os.symlink("` + src + `", "` + dst + `")`); err != nil {
			t.Errorf("os.symlink(%q) inside the jail was rejected: %v", src, err)
		}
	}
	if _, err := p.Eval(`import os
os.symlink("` + filepath.Join(base, "elsewhere") + `", "` + filepath.Join(jail, "absout") + `")`); err == nil {
		t.Error("an absolute target outside the jail was allowed")
	}
}
