package scriptling

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIntegrationScripts(t *testing.T) {
	// Path to test scripts
	testDir := "test_scripts"

	// Get all .py files in the directory
	files, err := filepath.Glob(filepath.Join(testDir, "*.py"))
	if err != nil {
		t.Fatalf("Failed to list test scripts: %v", err)
	}

	if len(files) == 0 {
		t.Fatalf("No test scripts found in %s", testDir)
	}

	p := New()

	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			content, err := os.ReadFile(file)
			if err != nil {
				t.Fatalf("Failed to read script %s: %v", file, err)
			}

			result, err := p.EvalWithTimeout(30*time.Second, string(content))
			if err != nil {
				t.Errorf("Script %s failed to execute: %v", file, err)
				return
			}

			if result.Inspect() != "true" {
				t.Errorf("Script %s failed: expected true, got %s", file, result.Inspect())
			} else {
				t.Logf("Script %s passed", file)
			}
		})
	}
}
