package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/agent"
	"github.com/paularlott/scriptling/extlibs/ai"
	"github.com/paularlott/scriptling/extlibs/console"
	"github.com/paularlott/scriptling/extlibs/filesystem"
	"github.com/paularlott/scriptling/extlibs/platform"
	"github.com/paularlott/scriptling/extlibs/requests"
	"github.com/paularlott/scriptling/extlibs/runtime"
	rthttp "github.com/paularlott/scriptling/extlibs/runtime/http"
	"github.com/paularlott/scriptling/extlibs/runtime/kv"
	scriptlingsimilarity "github.com/paularlott/scriptling/extlibs/similarity"
	"github.com/paularlott/scriptling/extlibs/subprocess"
	"github.com/paularlott/scriptling/extlibs/sys"
	"github.com/paularlott/scriptling/extlibs/toml"
	"github.com/paularlott/scriptling/extlibs/yaml"
	"github.com/paularlott/scriptling/libloader"
	"github.com/paularlott/scriptling/stdlib"
)

// registerRuntime registers scriptling.runtime with all its sub-libraries,
// as the CLI does.
func registerRuntime(p *scriptling.Scriptling) {
	rthttp.Register(p, nil)
	kv.Register(p, nil)
	runtime.RegisterSync(p)
	runtime.RegisterSandbox(p, nil)
	runtime.RegisterJSONRPC(p)
	runtime.RegisterMCP(p)
	runtime.Register(p)
}

func newTestInterpreter(t *testing.T) *scriptling.Scriptling {
	t.Helper()
	runtime.ResetRuntime()
	p := scriptling.New()
	stdlib.RegisterAll(p)
	platform.Register(p)
	requests.Register(p)
	sys.Register(p, nil, nil)
	subprocess.Register(p)
	registerRuntime(p)
	filesystem.RegisterOS(p, nil)
	filesystem.RegisterPathlib(p, nil)
	filesystem.RegisterFS(p, nil)
	console.Register(p)
	yaml.Register(p)
	toml.Register(p)
	filesystem.RegisterGlob(p, nil)
	filesystem.RegisterGrep(p, nil)
	filesystem.RegisterFind(p, nil)
	filesystem.RegisterSed(p, nil)
	filesystem.RegisterTempfile(p, nil)
	filesystem.RegisterShutil(p, nil)
	filesystem.RegisterZipfile(p, nil)
	filesystem.RegisterTarfile(p, nil)
	scriptlingsimilarity.Register(p)
	ai.Register(p)
	agent.Register(p)
	agent.RegisterInteract(p)
	runtime.SetBackgroundFactory(func() runtime.SandboxInstance {
		p2 := scriptling.New()
		stdlib.RegisterAll(p2)
		registerRuntime(p2)
		return p2
	})
	runtime.ReleaseBackgroundTasks()
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

			// Set up library loader for test libraries
			loader := libloader.NewFilesystem(testDir)
			p.SetLibraryLoader(loader)

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
			if resultStr != "True" && resultStr != "true" && resultStr != "None" {
				t.Errorf("Script %s failed: expected True or None, got %s", file, resultStr)
			} else {
				t.Logf("Script %s passed", file)
			}
		})
	}
}
