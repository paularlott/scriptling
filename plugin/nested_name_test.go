package plugin

import (
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/object"
)

// TestDottedNameUsedVerbatim proves a plugin declaring a dotted name keeps
// it: paul.hello registers (and imports) as paul.hello — the author owns
// the namespace, and only bare names get the plugin. prefix.
func TestDottedNameUsedVerbatim(t *testing.T) {
	server := NewServer("paul.hello", "1.0.0", "namespaced test")
	fb := object.NewFunctionBuilder()
	fb.Function(func() string { return "hi" })
	server.RegisterFunc("greet", fb)
	client := policyPipeServer(t, server, nil)
	defer client.Close()

	if got := client.Metadata().Name; got != "paul.hello" {
		t.Fatalf("metadata name: %s", got)
	}

	p := scriptling.New()
	RegisterLibraries(p, NewManager(nil))
	RegisterClientLibrary(p, client)
	result, err := p.Eval("import paul.hello as ph\nreturn ph.greet()")
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if result.Inspect() != "hi" {
		t.Fatalf("result: %s", result.Inspect())
	}
}

// TestDottedNameCollisionSkipped proves a dotted name colliding with a
// library the host already registered is refused with a warning rather
// than shadowing it.
func TestDottedNameCollisionSkipped(t *testing.T) {
	server := NewServer("scriptling.json", "1.0.0", "imposter")
	fb := object.NewFunctionBuilder()
	fb.Function(func() string { return "imposter" })
	server.RegisterFunc("dumps", fb)
	client := policyPipeServer(t, server, nil)
	defer client.Close()

	p := scriptling.New()
	// A stand-in for an already-registered library under the same name.
	existing := object.NewLibraryBuilder("scriptling.json", "the real one")
	existing.Function("dumps", func() string { return "real" })
	p.RegisterLibrary(existing.Build())

	manager := NewManager(nil)
	manager.mu.Lock()
	manager.clients["scriptling.json"] = client
	manager.mu.Unlock()

	RegisterLibraries(p, manager)

	warned := false
	for _, warning := range manager.Warnings() {
		if warning == "plugin scriptling.json ignored: library name already registered" {
			warned = true
		}
	}
	if !warned {
		t.Fatalf("expected collision warning, got %v", manager.Warnings())
	}
	result, err := p.Eval("import scriptling.json as j\nreturn j.dumps()")
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if result.Inspect() != "real" {
		t.Fatalf("imposter shadowed the registered library: %s", result.Inspect())
	}
}
