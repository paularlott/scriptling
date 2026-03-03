package libloader

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMemoryLoader(t *testing.T) {
	libs := map[string]string{
		"json":     "def loads(s): pass",
		"knot.api": "def call(): pass",
	}

	loader := NewMemoryLoader(libs)

	t.Run("found", func(t *testing.T) {
		source, found, err := loader.Load("json")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !found {
			t.Fatal("expected to find json")
		}
		if source != "def loads(s): pass" {
			t.Errorf("unexpected source: %s", source)
		}
	})

	t.Run("found nested", func(t *testing.T) {
		source, found, err := loader.Load("knot.api")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !found {
			t.Fatal("expected to find knot.api")
		}
		if source != "def call(): pass" {
			t.Errorf("unexpected source: %s", source)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, found, err := loader.Load("nonexistent")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if found {
			t.Fatal("expected not to find nonexistent")
		}
	})

	t.Run("description", func(t *testing.T) {
		if loader.Description() != "memory" {
			t.Errorf("unexpected description: %s", loader.Description())
		}
	})

	t.Run("set and remove", func(t *testing.T) {
		loader.Set("newlib", "new source")
		source, found, err := loader.Load("newlib")
		if err != nil || !found || source != "new source" {
			t.Errorf("Set failed: err=%v, found=%v, source=%s", err, found, source)
		}

		loader.Remove("newlib")
		_, found, _ = loader.Load("newlib")
		if found {
			t.Error("Remove failed")
		}
	})
}

func TestFuncLoader(t *testing.T) {
	loader := NewFuncLoader(func(name string) (string, bool, error) {
		if name == "test" {
			return "test source", true, nil
		}
		return "", false, nil
	}, "test loader")

	source, found, err := loader.Load("test")
	if err != nil || !found || source != "test source" {
		t.Errorf("Load failed: err=%v, found=%v, source=%s", err, found, source)
	}

	if loader.Description() != "test loader" {
		t.Errorf("unexpected description: %s", loader.Description())
	}
}

func TestChain(t *testing.T) {
	loader1 := NewMemoryLoader(map[string]string{
		"lib1": "source1",
	})
	loader2 := NewMemoryLoader(map[string]string{
		"lib2": "source2",
	})

	chain := NewChain(loader1, loader2)

	t.Run("found in first loader", func(t *testing.T) {
		source, found, err := chain.Load("lib1")
		if err != nil || !found || source != "source1" {
			t.Errorf("Load failed: err=%v, found=%v, source=%s", err, found, source)
		}
	})

	t.Run("found in second loader", func(t *testing.T) {
		source, found, err := chain.Load("lib2")
		if err != nil || !found || source != "source2" {
			t.Errorf("Load failed: err=%v, found=%v, source=%s", err, found, source)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, found, err := chain.Load("nonexistent")
		if err != nil || found {
			t.Errorf("expected not found: err=%v, found=%v", err, found)
		}
	})

	t.Run("add loader", func(t *testing.T) {
		loader3 := NewMemoryLoader(map[string]string{
			"lib3": "source3",
		})
		chain.Add(loader3)

		source, found, err := chain.Load("lib3")
		if err != nil || !found || source != "source3" {
			t.Errorf("Add failed: err=%v, found=%v, source=%s", err, found, source)
		}
	})

	t.Run("description", func(t *testing.T) {
		desc := chain.Description()
		if desc == "" {
			t.Error("description should not be empty")
		}
	})
}

func TestFilesystemLoader(t *testing.T) {
	// Create temp directory structure
	tmpDir, err := os.MkdirTemp("", "libloader-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test files
	// - json.py (flat)
	// - knot/groups.py (folder structure)
	// - knot.roles.py (flat nested)
	jsonContent := `def loads(s): pass`
	if err := os.WriteFile(filepath.Join(tmpDir, "json.py"), []byte(jsonContent), 0644); err != nil {
		t.Fatalf("failed to create json.py: %v", err)
	}

	knotDir := filepath.Join(tmpDir, "knot")
	if err := os.Mkdir(knotDir, 0755); err != nil {
		t.Fatalf("failed to create knot dir: %v", err)
	}

	groupsContent := `def get_groups(): pass`
	if err := os.WriteFile(filepath.Join(knotDir, "groups.py"), []byte(groupsContent), 0644); err != nil {
		t.Fatalf("failed to create groups.py: %v", err)
	}

	rolesContent := `def get_roles(): pass`
	if err := os.WriteFile(filepath.Join(tmpDir, "knot.roles.py"), []byte(rolesContent), 0644); err != nil {
		t.Fatalf("failed to create knot.roles.py: %v", err)
	}

	loader := NewFilesystem(tmpDir)

	t.Run("flat file", func(t *testing.T) {
		source, found, err := loader.Load("json")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !found {
			t.Fatal("expected to find json")
		}
		if source != jsonContent {
			t.Errorf("unexpected source: %s", source)
		}
	})

	t.Run("folder structure preferred over flat", func(t *testing.T) {
		// knot.groups should load from knot/groups.py (folder structure)
		// NOT from knot.groups.py (flat)
		source, found, err := loader.Load("knot.groups")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !found {
			t.Fatal("expected to find knot.groups")
		}
		if source != groupsContent {
			t.Errorf("expected folder structure content, got: %s", source)
		}
	})

	t.Run("flat fallback when no folder", func(t *testing.T) {
		// knot.roles only exists as knot.roles.py (no folder structure)
		source, found, err := loader.Load("knot.roles")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !found {
			t.Fatal("expected to find knot.roles")
		}
		if source != rolesContent {
			t.Errorf("unexpected source: %s", source)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, found, err := loader.Load("nonexistent")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if found {
			t.Fatal("expected not to find nonexistent")
		}
	})

	t.Run("description", func(t *testing.T) {
		desc := loader.Description()
		if desc == "" {
			t.Error("description should not be empty")
		}
	})

	t.Run("base dir", func(t *testing.T) {
		if loader.BaseDir() != tmpDir {
			t.Errorf("unexpected base dir: %s", loader.BaseDir())
		}
	})

	t.Run("extension", func(t *testing.T) {
		if loader.Extension() != ".py" {
			t.Errorf("unexpected extension: %s", loader.Extension())
		}
	})
}

func TestFilesystemLoaderWithCustomExtension(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "libloader-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create .scriptling file
	content := `def test(): pass`
	if err := os.WriteFile(filepath.Join(tmpDir, "test.scriptling"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test.scriptling: %v", err)
	}

	loader := NewFilesystem(tmpDir, WithExtension(".scriptling"))

	source, found, err := loader.Load("test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Fatal("expected to find test")
	}
	if source != content {
		t.Errorf("unexpected source: %s", source)
	}

	if loader.Extension() != ".scriptling" {
		t.Errorf("unexpected extension: %s", loader.Extension())
	}
}

func TestMultiFilesystemLoader(t *testing.T) {
	// Create two temp directories
	dir1, err := os.MkdirTemp("", "libloader-test1-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir1)

	dir2, err := os.MkdirTemp("", "libloader-test2-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir2)

	// Create files in both directories
	content1 := `def lib1(): pass`
	if err := os.WriteFile(filepath.Join(dir1, "lib1.py"), []byte(content1), 0644); err != nil {
		t.Fatalf("failed to create lib1.py: %v", err)
	}

	content2 := `def lib2(): pass`
	if err := os.WriteFile(filepath.Join(dir2, "lib2.py"), []byte(content2), 0644); err != nil {
		t.Fatalf("failed to create lib2.py: %v", err)
	}

	// Create override test - same lib in both dirs
	override1 := `def override(): version1`
	if err := os.WriteFile(filepath.Join(dir1, "override.py"), []byte(override1), 0644); err != nil {
		t.Fatalf("failed to create override.py in dir1: %v", err)
	}

	override2 := `def override(): version2`
	if err := os.WriteFile(filepath.Join(dir2, "override.py"), []byte(override2), 0644); err != nil {
		t.Fatalf("failed to create override.py in dir2: %v", err)
	}

	loader := NewMultiFilesystem(dir1, dir2)

	t.Run("found in first dir", func(t *testing.T) {
		source, found, err := loader.Load("lib1")
		if err != nil || !found || source != content1 {
			t.Errorf("Load failed: err=%v, found=%v, source=%s", err, found, source)
		}
	})

	t.Run("found in second dir", func(t *testing.T) {
		source, found, err := loader.Load("lib2")
		if err != nil || !found || source != content2 {
			t.Errorf("Load failed: err=%v, found=%v, source=%s", err, found, source)
		}
	})

	t.Run("first dir takes priority", func(t *testing.T) {
		source, found, err := loader.Load("override")
		if err != nil || !found || source != override1 {
			t.Errorf("expected override1, got: err=%v, found=%v, source=%s", err, found, source)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, found, err := loader.Load("nonexistent")
		if err != nil || found {
			t.Errorf("expected not found: err=%v, found=%v", err, found)
		}
	})

	t.Run("description", func(t *testing.T) {
		desc := loader.Description()
		if desc == "" {
			t.Error("description should not be empty")
		}
	})
}
