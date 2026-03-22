package pack

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/paularlott/scriptling/libloader"
)

func TestManifest(t *testing.T) {
	t.Run("valid manifest", func(t *testing.T) {
		manifestSrc := `
name = "testlib"
version = "1.2.3"
description = "A test library"
main = "cli.main"
`
		var m Manifest
		_, err := toml.NewDecoder(bytes.NewReader([]byte(manifestSrc))).Decode(&m)
		if err != nil {
			t.Fatalf("failed to parse manifest: %v", err)
		}
		if m.Name != "testlib" {
			t.Errorf("expected name 'testlib', got %q", m.Name)
		}
		if m.Version != "1.2.3" {
			t.Errorf("expected version '1.2.3', got %q", m.Version)
		}
		if m.Description != "A test library" {
			t.Errorf("expected description 'A test library', got %q", m.Description)
		}
		if m.Main != "cli.main" {
			t.Errorf("expected main 'cli.main', got %q", m.Main)
		}
	})

	t.Run("minimal manifest", func(t *testing.T) {
		manifestSrc := `
name = "minimal"
version = "0.0.1"
`
		var m Manifest
		_, err := toml.NewDecoder(bytes.NewReader([]byte(manifestSrc))).Decode(&m)
		if err != nil {
			t.Fatalf("failed to parse manifest: %v", err)
		}
		if m.Name != "minimal" {
			t.Errorf("expected name 'minimal', got %q", m.Name)
		}
		if m.Version != "0.0.1" {
			t.Errorf("expected version '0.0.1', got %q", m.Version)
		}
		if m.Description != "" {
			t.Errorf("expected empty description, got %q", m.Description)
		}
		if m.Main != "" {
			t.Errorf("expected empty main, got %q", m.Main)
		}
	})
}

func TestPackAndOpen(t *testing.T) {
	// Create temp source directory
	srcDir, err := os.MkdirTemp("", "pack-src-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(srcDir)

	// Create manifest
	manifestContent := `
name = "testpkg"
version = "1.0.0"
description = "Test package"
main = "app.run"
`
	if err := os.WriteFile(filepath.Join(srcDir, "manifest.toml"), []byte(manifestContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create lib directory with modules
	libDir := filepath.Join(srcDir, "lib")
	if err := os.MkdirAll(libDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create __init__.py
	if err := os.WriteFile(filepath.Join(libDir, "__init__.py"), []byte("# init"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create utils.py
	utilsCode := `
def helper():
    return "helped"
`
	if err := os.WriteFile(filepath.Join(libDir, "utils.py"), []byte(utilsCode), 0644); err != nil {
		t.Fatal(err)
	}

	// Create app.py with run function
	appCode := `
def run():
    return "running"
`
	if err := os.WriteFile(filepath.Join(libDir, "app.py"), []byte(appCode), 0644); err != nil {
		t.Fatal(err)
	}

	// Create submodule directory
	subDir := filepath.Join(libDir, "sub")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "__init__.py"), []byte("# sub init"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "mod.py"), []byte("# sub mod"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create docs directory
	docsDir := filepath.Join(srcDir, "docs")
	if err := os.MkdirAll(docsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docsDir, "guide.md"), []byte("# Guide"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a file that should be excluded
	if err := os.WriteFile(filepath.Join(srcDir, "excluded.txt"), []byte("should not be included"), 0644); err != nil {
		t.Fatal(err)
	}

	// Pack
	dstFile := filepath.Join(t.TempDir(), "testpkg.zip")
	if _, err := Pack(srcDir, dstFile, false); err != nil {
		t.Fatalf("Pack failed: %v", err)
	}

	// Open and verify
	data, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatal(err)
	}

	pkg, err := Open(bytesReaderAt(data), int64(len(data)))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	// Verify manifest
	if pkg.Manifest.Name != "testpkg" {
		t.Errorf("expected name 'testpkg', got %q", pkg.Manifest.Name)
	}
	if pkg.Manifest.Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got %q", pkg.Manifest.Version)
	}
	if pkg.Manifest.Main != "app.run" {
		t.Errorf("expected main 'app.run', got %q", pkg.Manifest.Main)
	}

	// Verify files exist
	expectedFiles := []string{
		"manifest.toml",
		"lib/__init__.py",
		"lib/utils.py",
		"lib/app.py",
		"lib/sub/__init__.py",
		"lib/sub/mod.py",
	}
	for _, f := range expectedFiles {
		if _, err := pkg.ReadFile(f); err != nil {
			t.Errorf("expected file %q in package: %v", f, err)
		}
	}

	// docs/ is not loaded into memory — verify via HasDocs and ZipDocReader
	if _, err := pkg.ReadFile("docs/guide.md"); err == nil {
		t.Error("docs/guide.md should not be accessible via ReadFile (excluded from memory)")
	}

	// Verify excluded file is not present
	if _, err := pkg.ReadFile("excluded.txt"); err == nil {
		t.Error("excluded.txt should not be in package")
	}

	// Verify HasDocs
	if !pkg.HasDocs() {
		t.Error("expected HasDocs() to return true")
	}

	// Verify List
	libFiles := pkg.List("lib")
	if len(libFiles) < 5 {
		t.Errorf("expected at least 5 files in lib/, got %d", len(libFiles))
	}
}

func TestPackRequiresManifest(t *testing.T) {
	srcDir, err := os.MkdirTemp("", "pack-nomanifest-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(srcDir)

	dstFile := filepath.Join(t.TempDir(), "test.zip")
	_, err = Pack(srcDir, dstFile, false)
	if err != ErrMissingManifest {
		t.Errorf("expected ErrMissingManifest, got %v", err)
	}
}

func TestPackOverwrite(t *testing.T) {
	srcDir, err := os.MkdirTemp("", "pack-overwrite-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(srcDir)

	// Create minimal valid source
	if err := os.WriteFile(filepath.Join(srcDir, "manifest.toml"), []byte("name = \"test\"\nversion = \"1.0.0\""), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(srcDir, "lib"), 0755); err != nil {
		t.Fatal(err)
	}

	dstFile := filepath.Join(t.TempDir(), "test.zip")

	// First pack should succeed
	if _, err := Pack(srcDir, dstFile, false); err != nil {
		t.Fatalf("first pack failed: %v", err)
	}

	// Second pack without force should fail
	if _, err := Pack(srcDir, dstFile, false); err == nil {
		t.Error("expected error when packing without force to existing file")
	}

	// Pack with force should succeed
	if _, err := Pack(srcDir, dstFile, true); err != nil {
		t.Fatalf("pack with force failed: %v", err)
	}
}

func TestUnpack(t *testing.T) {
	// Create a package first
	srcDir, err := os.MkdirTemp("", "unpack-src-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(srcDir)

	manifestContent := `
name = "unpacktest"
version = "2.0.0"
`
	if err := os.WriteFile(filepath.Join(srcDir, "manifest.toml"), []byte(manifestContent), 0644); err != nil {
		t.Fatal(err)
	}

	libDir := filepath.Join(srcDir, "lib")
	if err := os.MkdirAll(libDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(libDir, "test.py"), []byte("print('test')"), 0644); err != nil {
		t.Fatal(err)
	}

	pkgFile := filepath.Join(t.TempDir(), "test.zip")
	if _, err := Pack(srcDir, pkgFile, false); err != nil {
		t.Fatalf("Pack failed: %v", err)
	}

	// Test list - just verify no error
	t.Run("list", func(t *testing.T) {
		listDest := filepath.Join(t.TempDir(), "list-test")
		if err := Unpack(pkgFile, UnpackOptions{DestDir: listDest, List: true}); err != nil {
			t.Fatalf("Unpack with List failed: %v", err)
		}
	})

	// Test unpack
	destDir := filepath.Join(t.TempDir(), "unpacked")
	if err := Unpack(pkgFile, UnpackOptions{DestDir: destDir}); err != nil {
		t.Fatalf("Unpack failed: %v", err)
	}

	// Verify unpacked files
	if _, err := os.Stat(filepath.Join(destDir, "lib", "test.py")); err != nil {
		t.Error("lib/test.py not unpacked")
	}
	if _, err := os.Stat(filepath.Join(destDir, "manifest.toml")); err == nil {
		t.Error("manifest.toml should not be unpacked")
	}
}

func TestUnpackPathTraversal(t *testing.T) {
	// This test would require crafting a malicious zip file
	// For now, we verify the check exists in extractFile
	// The actual security test would need a crafted zip with ../ paths
}

func TestLoader(t *testing.T) {
	// Create a package
	srcDir, err := os.MkdirTemp("", "loader-src-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(srcDir)

	manifestContent := `
name = "loadertest"
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

	// Create modules
	if err := os.WriteFile(filepath.Join(libDir, "__init__.py"), []byte("init_val = 42"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(libDir, "utils.py"), []byte("def helper():\n    return 'helped'"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(libDir, "app.py"), []byte("def main():\n    return 'app main'"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create submodule package
	subDir := filepath.Join(libDir, "sub")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "__init__.py"), []byte("sub_val = 100"), 0644); err != nil {
		t.Fatal(err)
	}

	pkgFile := filepath.Join(t.TempDir(), "loader.zip")
	if _, err := Pack(srcDir, pkgFile, false); err != nil {
		t.Fatalf("Pack failed: %v", err)
	}

	// Test loader
	loader := NewLoader()
	if err := loader.AddFromPath(pkgFile, false); err != nil {
		t.Fatalf("AddFromPath failed: %v", err)
	}

	// Test loading modules
	tests := []struct {
		name     string
		module   string
		contains string
	}{
		{"root init", "__init__", "init_val = 42"},
		{"utils module", "utils", "def helper()"},
		{"app module", "app", "def main()"},
		{"submodule init", "sub.__init__", "sub_val = 100"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src, ok, err := loader.Load(tt.module)
			if err != nil {
				t.Fatalf("Load failed: %v", err)
			}
			if !ok {
				t.Fatalf("module %q not found", tt.module)
			}
			if !bytes.Contains([]byte(src), []byte(tt.contains)) {
				t.Errorf("expected source to contain %q, got %q", tt.contains, src)
			}
		})
	}

	// Test non-existent module
	t.Run("non-existent", func(t *testing.T) {
		src, ok, err := loader.Load("nonexistent")
		if err != nil {
			t.Fatalf("Load failed: %v", err)
		}
		if ok {
			t.Errorf("expected module 'nonexistent' to not be found, got: %q", src)
		}
	})

	// Test GetMainEntry
	t.Run("main entry", func(t *testing.T) {
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
	})
}

func TestLoaderPriority(t *testing.T) {
	// Create two packages with same module name
	tmpDir := t.TempDir()

	// First package
	pkg1Dir := filepath.Join(tmpDir, "pkg1")
	os.MkdirAll(filepath.Join(pkg1Dir, "lib"), 0755)
	os.WriteFile(filepath.Join(pkg1Dir, "manifest.toml"), []byte("name = \"pkg1\"\nversion = \"1.0.0\""), 0644)
	os.WriteFile(filepath.Join(pkg1Dir, "lib", "shared.py"), []byte("value = 'from pkg1'"), 0644)
	pkg1File := filepath.Join(tmpDir, "pkg1.zip")
	_, _ = Pack(pkg1Dir, pkg1File, false)

	// Second package
	pkg2Dir := filepath.Join(tmpDir, "pkg2")
	os.MkdirAll(filepath.Join(pkg2Dir, "lib"), 0755)
	os.WriteFile(filepath.Join(pkg2Dir, "manifest.toml"), []byte("name = \"pkg2\"\nversion = \"1.0.0\""), 0644)
	os.WriteFile(filepath.Join(pkg2Dir, "lib", "shared.py"), []byte("value = 'from pkg2'"), 0644)
	pkg2File := filepath.Join(tmpDir, "pkg2.zip")
	_, _ = Pack(pkg2Dir, pkg2File, false)

	// Load in order: pkg1, then pkg2
	loader := NewLoader()
	loader.AddFromPath(pkg1File, false)
	loader.AddFromPath(pkg2File, false)

	// pkg2 should win (last added = highest priority)
	src, ok, err := loader.Load("shared")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if !ok {
		t.Fatal("module 'shared' not found")
	}
	if !bytes.Contains([]byte(src), []byte("from pkg2")) {
		t.Errorf("expected 'from pkg2' (last added should win), got %q", src)
	}
}

func TestLoaderFallback(t *testing.T) {
	// Create package
	srcDir, err := os.MkdirTemp("", "fallback-src-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(srcDir)

	os.WriteFile(filepath.Join(srcDir, "manifest.toml"), []byte("name = \"fallback\"\nversion = \"1.0.0\""), 0644)
	os.MkdirAll(filepath.Join(srcDir, "lib"), 0755)
	os.WriteFile(filepath.Join(srcDir, "lib", "pkgmod.py"), []byte("# pkg module"), 0644)

	pkgFile := filepath.Join(t.TempDir(), "fallback.zip")
	_, _ = Pack(srcDir, pkgFile, false)

	// Create fallback loader that provides a module
	fallback := &mockLoader{
		modules: map[string]string{
			"fallback_mod": "# from fallback",
		},
	}

	loader := NewLoader()
	loader.AddFromPath(pkgFile, false)
	loader.SetFallback(fallback)

	// Load from package
	src, ok, err := loader.Load("pkgmod")
	if err != nil {
		t.Fatalf("Load pkgmod failed: %v", err)
	}
	if !ok {
		t.Fatal("pkgmod not found in package")
	}
	if !bytes.Contains([]byte(src), []byte("pkg module")) {
		t.Errorf("expected pkg module, got %q", src)
	}

	// Load from fallback
	src, ok, err = loader.Load("fallback_mod")
	if err != nil {
		t.Fatalf("Load fallback_mod failed: %v", err)
	}
	if !ok {
		t.Fatal("fallback_mod not found via fallback")
	}
	if !bytes.Contains([]byte(src), []byte("from fallback")) {
		t.Errorf("expected fallback module, got %q", src)
	}
}

// mockLoader implements libloader.LibraryLoader for testing
type mockLoader struct {
	modules map[string]string
}

func (m *mockLoader) Load(name string) (string, bool, error) {
	if src, ok := m.modules[name]; ok {
		return src, true, nil
	}
	return "", false, nil
}

func (m *mockLoader) Description() string {
	return "mock loader"
}

func TestFilesystemBeforePack(t *testing.T) {
	tmpDir := t.TempDir()

	// Package provides "shared" module
	pkgDir := filepath.Join(tmpDir, "pkg")
	os.MkdirAll(filepath.Join(pkgDir, "lib"), 0755)
	os.WriteFile(filepath.Join(pkgDir, "manifest.toml"), []byte("name = \"pkg\"\nversion = \"1.0.0\""), 0644)
	os.WriteFile(filepath.Join(pkgDir, "lib", "shared.py"), []byte("value = 'from pack'"), 0644)
	os.WriteFile(filepath.Join(pkgDir, "lib", "packonly.py"), []byte("value = 'pack only'"), 0644)
	pkgFile := filepath.Join(tmpDir, "pkg.zip")
	_, _ = Pack(pkgDir, pkgFile, false)

	// Filesystem loader also provides "shared" — should win
	fs := &mockLoader{
		modules: map[string]string{
			"shared": "value = 'from filesystem'",
		},
	}

	packLoader := NewLoader()
	packLoader.AddFromPath(pkgFile, false)

	chain := libloader.NewChain(fs, packLoader)

	// Filesystem wins for "shared"
	src, ok, err := chain.Load("shared")
	if err != nil {
		t.Fatalf("Load shared failed: %v", err)
	}
	if !ok {
		t.Fatal("shared not found")
	}
	if !bytes.Contains([]byte(src), []byte("from filesystem")) {
		t.Errorf("expected filesystem to win, got %q", src)
	}

	// Pack provides "packonly" when not on filesystem
	src, ok, err = chain.Load("packonly")
	if err != nil {
		t.Fatalf("Load packonly failed: %v", err)
	}
	if !ok {
		t.Fatal("packonly not found")
	}
	if !bytes.Contains([]byte(src), []byte("pack only")) {
		t.Errorf("expected pack module, got %q", src)
	}
}

func TestSplitHash(t *testing.T) {
	tests := []struct {
		input        string
		wantSource   string
		wantHash     string
	}{
		{"file.zip", "file.zip", ""},
		{"file.zip#sha256:abc123", "file.zip", "abc123"},
		{"https://example.com/pkg.zip#sha256:deadbeef", "https://example.com/pkg.zip", "deadbeef"},
		{"https://example.com/pkg.zip", "https://example.com/pkg.zip", ""},
	}
	for _, tt := range tests {
		src, hash := splitHash(tt.input)
		if src != tt.wantSource {
			t.Errorf("splitHash(%q) source = %q, want %q", tt.input, src, tt.wantSource)
		}
		if hash != tt.wantHash {
			t.Errorf("splitHash(%q) hash = %q, want %q", tt.input, hash, tt.wantHash)
		}
	}
}

func TestHashVerification(t *testing.T) {
	// Build a real package to get a real hash
	srcDir := t.TempDir()
	os.WriteFile(filepath.Join(srcDir, "manifest.toml"), []byte("name = \"hashtest\"\nversion = \"1.0.0\"\n"), 0644)
	os.MkdirAll(filepath.Join(srcDir, "lib"), 0755)
	os.WriteFile(filepath.Join(srcDir, "lib", "mod.py"), []byte("x = 1\n"), 0644)

	pkgFile := filepath.Join(t.TempDir(), "hashtest.zip")
	hash, err := Pack(srcDir, pkgFile, false)
	if err != nil {
		t.Fatalf("Pack failed: %v", err)
	}

	t.Run("no hash - local file", func(t *testing.T) {
		loader := NewLoader()
		if err := loader.AddFromPath(pkgFile, false); err != nil {
			t.Errorf("expected no error without hash, got: %v", err)
		}
	})

	t.Run("correct hash - local file", func(t *testing.T) {
		loader := NewLoader()
		if err := loader.AddFromPath(pkgFile+"#sha256:"+hash, false); err != nil {
			t.Errorf("expected no error with correct hash, got: %v", err)
		}
	})

	t.Run("wrong hash - local file", func(t *testing.T) {
		loader := NewLoader()
		err := loader.AddFromPath(pkgFile+"#sha256:0000000000000000000000000000000000000000000000000000000000000000", false)
		if err == nil {
			t.Error("expected error with wrong hash, got nil")
		}
	})

	t.Run("correct hash - FetchWithCache direct", func(t *testing.T) {
		if _, err := FetchWithCache(pkgFile+"#sha256:"+hash, false, ""); err != nil {
			t.Errorf("expected no error with correct hash, got: %v", err)
		}
	})

	t.Run("wrong hash - FetchWithCache direct", func(t *testing.T) {
		_, err := FetchWithCache(pkgFile+"#sha256:0000000000000000000000000000000000000000000000000000000000000000", false, "")
		if err == nil {
			t.Error("expected error with wrong hash, got nil")
		}
	})

	t.Run("zip suffix check with hash", func(t *testing.T) {
		// Ensure file.zip#sha256:... is still recognised as a zip, not a .py
		loader := NewLoader()
		if err := loader.AddFromPath(pkgFile+"#sha256:"+hash, false); err != nil {
			t.Fatalf("AddFromPath failed: %v", err)
		}
		if len(loader.packages) != 1 {
			t.Errorf("expected 1 package loaded, got %d", len(loader.packages))
		}
	})
}

func TestHashBytes(t *testing.T) {
	// SHA256 of empty input is known
	empty := HashBytes([]byte{})
	if empty != "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Errorf("HashBytes(empty) = %q, want known sha256", empty)
	}

	// Same input produces same hash
	data := []byte("hello world")
	h1 := HashBytes(data)
	h2 := HashBytes(data)
	if h1 != h2 {
		t.Errorf("HashBytes not deterministic: %q != %q", h1, h2)
	}

	// Different input produces different hash
	h3 := HashBytes([]byte("hello world!"))
	if h1 == h3 {
		t.Error("HashBytes: different inputs produced same hash")
	}
}

func TestPackHash(t *testing.T) {
	// Pack a directory and verify the returned hash matches HashBytes of the file
	srcDir := t.TempDir()
	os.WriteFile(filepath.Join(srcDir, "manifest.toml"), []byte("name = \"hashpack\"\nversion = \"1.0.0\"\n"), 0644)
	os.MkdirAll(filepath.Join(srcDir, "lib"), 0755)
	os.WriteFile(filepath.Join(srcDir, "lib", "mod.py"), []byte("x = 42\n"), 0644)

	pkgFile := filepath.Join(t.TempDir(), "hashpack.zip")
	hash, err := Pack(srcDir, pkgFile, false)
	if err != nil {
		t.Fatalf("Pack failed: %v", err)
	}

	// Hash returned by Pack must match HashBytes of the written file
	data, err := os.ReadFile(pkgFile)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	expected := HashBytes(data)
	if hash != expected {
		t.Errorf("Pack hash = %q, HashBytes = %q", hash, expected)
	}

	// Hash must be a 64-char hex string (sha256)
	if len(hash) != 64 {
		t.Errorf("expected 64-char hash, got %d chars: %q", len(hash), hash)
	}
}

func TestUnpackRemove(t *testing.T) {
	tmpDir := t.TempDir()

	// Build a package with lib and docs
	srcDir := filepath.Join(tmpDir, "src")
	os.MkdirAll(filepath.Join(srcDir, "lib"), 0755)
	os.MkdirAll(filepath.Join(srcDir, "docs"), 0755)
	os.WriteFile(filepath.Join(srcDir, "manifest.toml"), []byte("name = \"rm\"\nversion = \"1.0.0\"\n"), 0644)
	os.WriteFile(filepath.Join(srcDir, "lib", "mod.py"), []byte("x = 1\n"), 0644)
	os.WriteFile(filepath.Join(srcDir, "docs", "mod.md"), []byte("# mod\n"), 0644)

	pkgFile := filepath.Join(tmpDir, "rm.zip")
	if _, err := Pack(srcDir, pkgFile, false); err != nil {
		t.Fatalf("Pack failed: %v", err)
	}

	destDir := filepath.Join(tmpDir, "dest")

	// Unpack first
	if err := Unpack(pkgFile, UnpackOptions{DestDir: destDir}); err != nil {
		t.Fatalf("Unpack failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(destDir, "lib", "mod.py")); err != nil {
		t.Fatal("lib/mod.py should exist after unpack")
	}
	if _, err := os.Stat(filepath.Join(destDir, "docs", "mod.md")); err != nil {
		t.Fatal("docs/mod.md should exist after unpack")
	}

	// Remove
	if err := UnpackRemove(pkgFile, false, destDir); err != nil {
		t.Fatalf("UnpackRemove failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(destDir, "lib", "mod.py")); !os.IsNotExist(err) {
		t.Error("lib/mod.py should be removed")
	}
	if _, err := os.Stat(filepath.Join(destDir, "docs", "mod.md")); !os.IsNotExist(err) {
		t.Error("docs/mod.md should be removed")
	}

	// Remove on already-removed files should not error
	if err := UnpackRemove(pkgFile, false, destDir); err != nil {
		t.Errorf("UnpackRemove on missing files should not error: %v", err)
	}
}
