package pluginpack

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/plugin"
	"github.com/paularlott/scriptling/scriptling-cli/pack"
	"github.com/paularlott/scriptling/stdlib"
)

// buildFetcherGoExample compiles examples/plugins/fetcher-go — the documented
// full-example plugin — into a temp dir, the same way a user following the
// tutorial would.
func buildFetcherGoExample(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "fetcher-go")
	cmd := exec.Command("go", "build", "-o", bin, "./examples/plugins/fetcher-go")
	cmd.Dir = "../.."
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build fetcher-go: %v\n%s", err, out)
	}
	return bin
}

// newExampleInterpreter loads the built example plugin and returns an
// interpreter wired the way the CLI wires one: plugin libraries
// (plugin.demo), the fetcher bundle (demo://libs modules), and
// scriptling.package over the same loader.
func newExampleInterpreter(t *testing.T) (*scriptling.Scriptling, *Bridge) {
	t.Helper()
	bin := buildFetcherGoExample(t)

	manager := plugin.NewManager(nil)
	t.Cleanup(func() { _ = manager.Close() })
	if _, err := manager.LoadPlugin(context.Background(), bin, nil); err != nil {
		t.Fatalf("LoadPlugin: %v", err)
	}

	bridge := New(Options{Manager: manager})
	if err := bridge.Register(); err != nil {
		t.Fatalf("Bridge.Register: %v", err)
	}
	t.Cleanup(func() { _ = bridge.Close() })

	loader := pack.NewLoader()
	bundles, err := bridge.Bundles()
	if err != nil {
		t.Fatalf("Bundles: %v", err)
	}
	for _, b := range bundles {
		if err := loader.AddBundle(b); err != nil {
			t.Fatalf("AddBundle: %v", err)
		}
	}

	p := scriptling.New()
	stdlib.RegisterAll(p)
	plugin.RegisterLibraries(p, manager)
	loader.SetFallback(p.GetLibraryLoader())
	p.SetLibraryLoader(loader)
	pack.RegisterPackageLibrary(p, loader)
	return p, bridge
}

// TestFetcherGoExampleFullStack drives the shipped example plugin end to end:
// its registered function and class (plugin.demo), its fetcher-served library
// at any nesting depth, and its static assets read through
// scriptling.package. If the example regresses, this is the test that says so.
func TestFetcherGoExampleFullStack(t *testing.T) {
	p, _ := newExampleInterpreter(t)

	result, err := p.Eval(`
import greet
import fred
import blah.blah
import json
import plugin.demo
import scriptling.package as package

[
    greet.greeting("tour"),
    fred.value(),
    blah.blah.value(),
    json.loads(package.read_file("demo", "data/config.json"))["greeting"],
    plugin.demo.asset("docs/getting-started.md").splitlines()[0],
    plugin.demo.Doc("docs/configuration.md").title(),
    package.glob("demo", "**/*.md"),
]
`)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	list, ok := result.(*object.List)
	if !ok {
		t.Fatalf("expected a list result, got %T", result)
	}
	want := []string{
		"hello from demo://libs, tour",
		"fred, a one-level package",
		"blah.blah, a two-level package",
		"hello from data/config.json",
		"# demo://libs",
		"Configuration",
		"[docs/configuration.md, docs/getting-started.md]",
	}
	if len(list.Elements) != len(want) {
		t.Fatalf("result has %d elements (%v), want %d", len(list.Elements), result.Inspect(), len(want))
	}
	for i, expected := range want {
		if got := list.Elements[i].Inspect(); got != expected {
			t.Errorf("element %d = %s, want %s", i, got, expected)
		}
	}
}

// TestFetcherGoExampleClassErrors covers the example class's failure path: a
// document the plugin does not serve fails construction with the path named.
func TestFetcherGoExampleClassErrors(t *testing.T) {
	p, _ := newExampleInterpreter(t)

	_, err := p.Eval(`import plugin.demo
plugin.demo.Doc("docs/missing.md")`)
	if err == nil {
		t.Fatal("expected constructing Doc with an unserved path to fail")
	}
	if !strings.Contains(err.Error(), "no such document: docs/missing.md") {
		t.Fatalf("expected the error to name the missing document, got: %v", err)
	}
}

// TestFetcherGoExampleTourScript runs the tour script the example serves at
// demo://scripts/tour, the same one the README and tutorial tell users to run.
func TestFetcherGoExampleTourScript(t *testing.T) {
	p, bridge := newExampleInterpreter(t)

	source, err := bridge.FetchScript(context.Background(), "demo://scripts/tour")
	if err != nil {
		t.Fatalf("FetchScript: %v", err)
	}
	p.SetSourceFile("demo://scripts/tour")
	if _, err := p.Eval(string(source)); err != nil {
		t.Fatalf("running the served tour script: %v", err)
	}
}
