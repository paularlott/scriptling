// Package plugintest drives database plugins in external mode: it builds
// the plugin's cmd binary, loads it through a Manager (handshake, policy,
// script-shim connect wrappers, object protocol) and evaluates a script
// against it — the full wire path a real deployment uses.
package plugintest

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/plugin"
	"github.com/paularlott/scriptling/stdlib"
)

// BuildPlugin compiles the plugin command at cmdDir (e.g. "./cmd") into a
// temp binary and returns its path. The binary is removed with the test.
func BuildPlugin(t *testing.T, cmdDir string) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "scriptling-plugin-bin")
	build := exec.Command("go", "build", "-o", bin, cmdDir)
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build %s: %v\n%s", cmdDir, err, out)
	}
	return bin
}

// External evaluates script against a fresh interpreter with the compiled
// plugin loaded as an external process carrying policy. It exercises the
// handshake (including policy delivery), the generated script library, the
// connect wrapper, and object method calls over the wire.
func External(t *testing.T, binPath string, policy *plugin.Policy, script string) (object.Object, error) {
	t.Helper()
	manager := plugin.NewManager(nil)
	manager.SetPolicy(policy)
	defer manager.Close()

	ctx := context.Background()
	if _, err := manager.LoadPlugin(ctx, binPath, nil); err != nil {
		t.Fatalf("load plugin %s: %v", binPath, err)
	}
	for _, warning := range manager.Warnings() {
		t.Logf("plugin warning: %s", warning)
	}

	p := scriptling.New()
	stdlib.RegisterAll(p)
	plugin.RegisterLibraries(p, manager, policy)
	return p.Eval(script)
}
