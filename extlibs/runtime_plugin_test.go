package extlibs

import (
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/object"
)

func TestPluginServeRegistersState(t *testing.T) {
	ResetRuntime()
	defer ResetRuntime()

	p := scriptling.New()
	RegisterRuntimeLibraryAll(p, nil)
	RegisterRuntimePluginLibrary(p)

	_, err := p.Eval(`
import scriptling.runtime.plugin as rp
rp.serve("myservice", "1.0", "My service description")
`)
	if err != nil {
		t.Fatalf("plugin.serve: %v", err)
	}

	RuntimeState.RLock()
	name := RuntimeState.PluginName
	version := RuntimeState.PluginVersion
	desc := RuntimeState.PluginDescription
	RuntimeState.RUnlock()

	if name != "myservice" {
		t.Errorf("PluginName = %q, want %q", name, "myservice")
	}
	if version != "1.0" {
		t.Errorf("PluginVersion = %q, want %q", version, "1.0")
	}
	if desc != "My service description" {
		t.Errorf("PluginDescription = %q, want %q", desc, "My service description")
	}
}

func TestPluginServeMinimalArgs(t *testing.T) {
	ResetRuntime()
	defer ResetRuntime()

	p := scriptling.New()
	RegisterRuntimeLibraryAll(p, nil)
	RegisterRuntimePluginLibrary(p)

	_, err := p.Eval(`
import scriptling.runtime.plugin as rp
rp.serve("svc")
`)
	if err != nil {
		t.Fatalf("plugin.serve with name only: %v", err)
	}

	RuntimeState.RLock()
	name := RuntimeState.PluginName
	version := RuntimeState.PluginVersion
	desc := RuntimeState.PluginDescription
	RuntimeState.RUnlock()

	if name != "svc" {
		t.Errorf("PluginName = %q, want %q", name, "svc")
	}
	if version != "" {
		t.Errorf("PluginVersion = %q, want empty", version)
	}
	if desc != "" {
		t.Errorf("PluginDescription = %q, want empty", desc)
	}
}

func TestPluginFunctionRegistersHandler(t *testing.T) {
	ResetRuntime()
	defer ResetRuntime()

	p := scriptling.New()
	RegisterRuntimeLibraryAll(p, nil)
	RegisterRuntimePluginLibrary(p)

	_, err := p.Eval(`
import scriptling.runtime.plugin as rp
rp.serve("calc", "1.0", "Calculator")
rp.register_function("add", "handlers.add")
rp.register_function("mul", "handlers.mul")
`)
	if err != nil {
		t.Fatalf("plugin.function: %v", err)
	}

	RuntimeState.RLock()
	fns := make(map[string]string, len(RuntimeState.PluginFunctions))
	for k, v := range RuntimeState.PluginFunctions {
		fns[k] = v
	}
	RuntimeState.RUnlock()

	if h, ok := fns["add"]; !ok || h != "handlers.add" {
		t.Errorf("PluginFunctions[add] = %q, want %q", h, "handlers.add")
	}
	if h, ok := fns["mul"]; !ok || h != "handlers.mul" {
		t.Errorf("PluginFunctions[mul] = %q, want %q", h, "handlers.mul")
	}
}

func TestPluginSubLibraryAccessViaRuntimeParent(t *testing.T) {
	ResetRuntime()
	defer ResetRuntime()

	p := scriptling.New()
	RegisterRuntimeLibraryAll(p, nil)
	RegisterRuntimePluginLibrary(p)

	// Access via the parent runtime dict: runtime.plugin.serve(...)
	_, err := p.Eval(`
import scriptling.runtime as runtime
runtime.plugin.serve("via_parent", "2.0", "Accessed via parent")
`)
	if err != nil {
		t.Fatalf("runtime.plugin.serve via parent: %v", err)
	}

	RuntimeState.RLock()
	name := RuntimeState.PluginName
	RuntimeState.RUnlock()

	if name != "via_parent" {
		t.Errorf("PluginName = %q, want %q", name, "via_parent")
	}
}

func TestPluginResetClearsState(t *testing.T) {
	ResetRuntime()

	p := scriptling.New()
	RegisterRuntimeLibraryAll(p, nil)
	RegisterRuntimePluginLibrary(p)

	_, err := p.Eval(`
import scriptling.runtime.plugin as rp
rp.serve("myservice", "1.0", "desc")
rp.register_function("greet", "handlers.greet")
`)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	ResetRuntime()

	RuntimeState.RLock()
	name := RuntimeState.PluginName
	fnCount := len(RuntimeState.PluginFunctions)
	RuntimeState.RUnlock()

	if name != "" {
		t.Errorf("PluginName after reset = %q, want empty", name)
	}
	if fnCount != 0 {
		t.Errorf("PluginFunctions after reset has %d entries, want 0", fnCount)
	}
}

func TestPluginServeRequiresName(t *testing.T) {
	ResetRuntime()
	defer ResetRuntime()

	p := scriptling.New()
	RegisterRuntimeLibraryAll(p, nil)
	RegisterRuntimePluginLibrary(p)

	_, err := p.Eval(`
import scriptling.runtime.plugin as rp
rp.serve()
`)
	if err == nil {
		t.Fatal("plugin.serve() with no args should have failed")
	}
}

func TestPluginFunctionRequiresBothArgs(t *testing.T) {
	ResetRuntime()
	defer ResetRuntime()

	p := scriptling.New()
	RegisterRuntimeLibraryAll(p, nil)
	RegisterRuntimePluginLibrary(p)

	_, err := p.Eval(`
import scriptling.runtime.plugin as rp
rp.register_function("greet")
`)
	if err == nil {
		t.Fatal("plugin.function() with one arg should have failed")
	}
}

func TestPluginConstantRegistersValue(t *testing.T) {
	ResetRuntime()
	defer ResetRuntime()

	p := scriptling.New()
	RegisterRuntimeLibraryAll(p, nil)
	RegisterRuntimePluginLibrary(p)

	_, err := p.Eval(`
import scriptling.runtime.plugin as rp
rp.serve("svc", "1.0", "Service")
rp.register_constant("VERSION", "2.5.0")
rp.register_constant("MAX_RETRIES", 5)
`)
	if err != nil {
		t.Fatalf("plugin.constant: %v", err)
	}

	RuntimeState.RLock()
	consts := make(map[string]object.Object, len(RuntimeState.PluginConstants))
	for k, v := range RuntimeState.PluginConstants {
		consts[k] = v
	}
	RuntimeState.RUnlock()

	if v, ok := consts["VERSION"]; !ok {
		t.Error("VERSION constant not registered")
	} else {
		s, err := v.AsString()
		if err != nil {
			t.Fatalf("VERSION is not a string: %v", err)
		}
		if s != "2.5.0" {
			t.Errorf("VERSION = %q, want %q", s, "2.5.0")
		}
	}
	if _, ok := consts["MAX_RETRIES"]; !ok {
		t.Error("MAX_RETRIES constant not registered")
	}
}

func TestPluginClassRegistersHandler(t *testing.T) {
	ResetRuntime()
	defer ResetRuntime()

	p := scriptling.New()
	RegisterRuntimeLibraryAll(p, nil)
	RegisterRuntimePluginLibrary(p)

	_, err := p.Eval(`
import scriptling.runtime.plugin as rp
rp.serve("svc", "1.0", "Service")
rp.register_class("handlers.Config")
rp.register_class("handlers.Widget")
`)
	if err != nil {
		t.Fatalf("plugin.class: %v", err)
	}

	RuntimeState.RLock()
	classes := make(map[string]string, len(RuntimeState.PluginClasses))
	for k, v := range RuntimeState.PluginClasses {
		classes[k] = v
	}
	RuntimeState.RUnlock()

	if h, ok := classes["Config"]; !ok || h != "handlers.Config" {
		t.Errorf("PluginClasses[Config] = %q, want %q", h, "handlers.Config")
	}
	if h, ok := classes["Widget"]; !ok || h != "handlers.Widget" {
		t.Errorf("PluginClasses[Widget] = %q, want %q", h, "handlers.Widget")
	}
}

func TestPluginClassRequiresArg(t *testing.T) {
	ResetRuntime()
	defer ResetRuntime()

	p := scriptling.New()
	RegisterRuntimeLibraryAll(p, nil)
	RegisterRuntimePluginLibrary(p)

	_, err := p.Eval(`
import scriptling.runtime.plugin as rp
rp.register_class()
`)
	if err == nil {
		t.Fatal("plugin.class() with no args should have failed")
	}
}

func TestPluginResetClearsConstantsAndClasses(t *testing.T) {
	ResetRuntime()

	p := scriptling.New()
	RegisterRuntimeLibraryAll(p, nil)
	RegisterRuntimePluginLibrary(p)

	_, err := p.Eval(`
import scriptling.runtime.plugin as rp
rp.serve("svc", "1.0", "Service")
rp.register_constant("X", 42)
rp.register_class("mod.MyClass")
`)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	ResetRuntime()

	RuntimeState.RLock()
	constCount := len(RuntimeState.PluginConstants)
	classCount := len(RuntimeState.PluginClasses)
	RuntimeState.RUnlock()

	if constCount != 0 {
		t.Errorf("PluginConstants after reset has %d entries, want 0", constCount)
	}
	if classCount != 0 {
		t.Errorf("PluginClasses after reset has %d entries, want 0", classCount)
	}
}
