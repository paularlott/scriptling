package scriptling

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/paularlott/scriptling/extlibs"
	"github.com/paularlott/scriptling/stdlib"
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
	// Register all standard libraries
	stdlib.RegisterAll(p)
	// Register os library with no restrictions for testing
	extlibs.RegisterOSLibrary(p, nil)
	// Register pathlib library with no restrictions for testing
	extlibs.RegisterPathlibLibrary(p, nil)
	// Register subprocess library for testing
	p.RegisterLibrary(extlibs.SubprocessLibraryName, extlibs.SubprocessLibrary)
	p.RegisterLibrary(extlibs.HTMLParserLibraryName, extlibs.HTMLParserLibrary)
	p.RegisterLibrary(extlibs.SecretsLibraryName, extlibs.SecretsLibrary)
	p.RegisterLibrary(extlibs.RequestsLibraryName, extlibs.RequestsLibrary)
	p.RegisterLibrary(extlibs.SysLibraryName, extlibs.SysLibrary)

	// Set up on-demand library loading for local .py files in test_scripts
	p.SetOnDemandLibraryCallback(func(p *Scriptling, libName string) bool {
		// Try to load from test_scripts directory
		filename := filepath.Join(testDir, libName+".py")
		content, err := os.ReadFile(filename)
		if err == nil {
			return p.RegisterScriptLibrary(libName, string(content)) == nil
		}
		return false
	})

	for _, file := range files {
		// Skip library files
		if strings.HasPrefix(filepath.Base(file), "lib_") {
			continue
		}

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

			// Accept both assert-based tests (which may return None) and legacy True-returning tests
			resultStr := result.Inspect()
			if resultStr != "true" && resultStr != "None" {
				t.Errorf("Script %s failed: expected true or None, got %s", file, resultStr)
			} else {
				t.Logf("Script %s passed", file)
			}
		})
	}
}
