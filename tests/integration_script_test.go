package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs"
	"github.com/paularlott/scriptling/extlibs/agent"
	"github.com/paularlott/scriptling/extlibs/ai"
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

	p := scriptling.New()
	stdlib.RegisterAll(p)
	extlibs.RegisterRequestsLibrary(p)
	extlibs.RegisterSysLibrary(p, []string{})
	extlibs.RegisterSecretsLibrary(p)
	extlibs.RegisterSubprocessLibrary(p)
	extlibs.RegisterHTMLParserLibrary(p)
	extlibs.RegisterRuntimeLibraryAll(p)
	extlibs.RegisterOSLibrary(p, []string{})
	extlibs.RegisterPathlibLibrary(p, []string{})
	extlibs.RegisterConsoleLibrary(p)
	extlibs.RegisterYAMLLibrary(p)
	ai.Register(p)
	agent.Register(p)
	agent.RegisterInteract(p)

	// Release background tasks so they start immediately
	extlibs.ReleaseBackgroundTasks()

	p.SetOnDemandLibraryCallback(func(p *scriptling.Scriptling, libName string) bool {
		// Try loading from file
		filename := filepath.Join(testDir, libName+".py")
		content, err := os.ReadFile(filename)
		if err == nil {
			return p.RegisterScriptLibrary(libName, string(content)) == nil
		}
		return false
	})

	for _, file := range files {
		baseName := filepath.Base(file)
		if strings.HasPrefix(baseName, "lib_") {
			continue
		}

		t.Run(baseName, func(t *testing.T) {

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
