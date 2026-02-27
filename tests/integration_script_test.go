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
	"github.com/paularlott/scriptling/extlibs/console"
	"github.com/paularlott/scriptling/stdlib"
)

func newTestInterpreter(t *testing.T) *scriptling.Scriptling {
	t.Helper()
	extlibs.ResetRuntime()
	p := scriptling.New()
	stdlib.RegisterAll(p)
	extlibs.RegisterRequestsLibrary(p)
	extlibs.RegisterSysLibrary(p, nil, nil)
	extlibs.RegisterSecretsLibrary(p)
	extlibs.RegisterSubprocessLibrary(p)
	extlibs.RegisterHTMLParserLibrary(p)
	extlibs.RegisterRuntimeLibraryAll(p, nil)
	extlibs.RegisterOSLibrary(p, nil)
	extlibs.RegisterPathlibLibrary(p, nil)
	console.Register(p)
	extlibs.RegisterYAMLLibrary(p)
	ai.Register(p)
	agent.Register(p)
	agent.RegisterInteract(p)
	extlibs.ReleaseBackgroundTasks()
	return p
}

func TestIntegrationScripts(t *testing.T) {
	testDir := "test_scripts"

	files, err := filepath.Glob(filepath.Join(testDir, "*.py"))
	if err != nil {
		t.Fatalf("Failed to list test scripts: %v", err)
	}
	if len(files) == 0 {
		t.Fatalf("No test scripts found in %s", testDir)
	}

	for _, file := range files {
		baseName := filepath.Base(file)
		if strings.HasPrefix(baseName, "lib_") {
			continue
		}

		t.Run(baseName, func(t *testing.T) {
			p := newTestInterpreter(t)
			p.SetOnDemandLibraryCallback(func(p *scriptling.Scriptling, libName string) bool {
				filename := filepath.Join(testDir, libName+".py")
				content, err := os.ReadFile(filename)
				if err == nil {
					return p.RegisterScriptLibrary(libName, string(content)) == nil
				}
				return false
			})

			content, err := os.ReadFile(file)
			if err != nil {
				t.Fatalf("Failed to read script %s: %v", file, err)
			}

			result, err := p.EvalWithTimeout(30*time.Second, string(content))
			if err != nil {
				t.Errorf("Script %s failed to execute: %v", file, err)
				return
			}

			resultStr := result.Inspect()
			if resultStr != "true" && resultStr != "None" {
				t.Errorf("Script %s failed: expected true or None, got %s", file, resultStr)
			} else {
				t.Logf("Script %s passed", file)
			}
		})
	}
}
