package runtime

import (
	"testing"

	"github.com/paularlott/scriptling"
)

func TestJSONRPCMethodDecorator(t *testing.T) {
	ResetRuntime()
	p := scriptling.New()
	registerAll(p, nil)
	p.SetVar("__name__", "rpc")

	script := `
import scriptling.runtime.jsonrpc as jsonrpc

@jsonrpc.method("echo")
def echo(params):
    return params

@jsonrpc.method("add")
def add(params):
    return params["a"] + params["b"]
`
	_, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Failed to execute script: %v", err)
	}

	RuntimeState.RLock()
	defer RuntimeState.RUnlock()

	if handler, ok := RuntimeState.JSONRPCMethods["echo"]; !ok || handler != "rpc.echo" {
		t.Errorf("method 'echo': expected 'rpc.echo', got %q", handler)
	}
	if handler, ok := RuntimeState.JSONRPCMethods["add"]; !ok || handler != "rpc.add" {
		t.Errorf("method 'add': expected 'rpc.add', got %q", handler)
	}
}

func TestJSONRPCNotificationDecorator(t *testing.T) {
	ResetRuntime()
	p := scriptling.New()
	registerAll(p, nil)
	p.SetVar("__name__", "rpc")

	script := `
import scriptling.runtime.jsonrpc as jsonrpc

@jsonrpc.notification("updated")
def on_updated(params):
    pass
`
	_, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Failed to execute script: %v", err)
	}

	RuntimeState.RLock()
	defer RuntimeState.RUnlock()

	if handler, ok := RuntimeState.JSONRPCNotifications["updated"]; !ok || handler != "rpc.on_updated" {
		t.Errorf("notification 'updated': expected 'rpc.on_updated', got %q", handler)
	}
}

func TestJSONRPCImperativeBackwardCompat(t *testing.T) {
	ResetRuntime()
	p := scriptling.New()
	registerAll(p, nil)

	script := `
import scriptling.runtime as runtime

runtime.jsonrpc.method("echo", "handlers.echo")
runtime.jsonrpc.notification("updated", "handlers.on_updated")
`
	_, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Failed to execute script: %v", err)
	}

	RuntimeState.RLock()
	defer RuntimeState.RUnlock()

	if handler, ok := RuntimeState.JSONRPCMethods["echo"]; !ok || handler != "handlers.echo" {
		t.Errorf("method 'echo': expected 'handlers.echo', got %q", handler)
	}
	if handler, ok := RuntimeState.JSONRPCNotifications["updated"]; !ok || handler != "handlers.on_updated" {
		t.Errorf("notification 'updated': expected 'handlers.on_updated', got %q", handler)
	}
}

func TestPluginFunctionDecoratorNamed(t *testing.T) {
	ResetRuntime()
	p := scriptling.New()
	registerAll(p, nil)
	RegisterPlugin(p)
	p.SetVar("__name__", "calc")

	script := `
import scriptling.runtime.plugin as plugin

@plugin.register_function("add")
def add(a, b):
    return a + b
`
	_, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Failed to execute script: %v", err)
	}

	RuntimeState.RLock()
	defer RuntimeState.RUnlock()

	if ref, ok := RuntimeState.PluginFunctions["add"]; !ok || ref != "calc.add" {
		t.Errorf("function 'add': expected 'calc.add', got %q", ref)
	}
}

func TestPluginFunctionDecoratorBare(t *testing.T) {
	ResetRuntime()
	p := scriptling.New()
	registerAll(p, nil)
	RegisterPlugin(p)
	p.SetVar("__name__", "svc")

	script := `
import scriptling.runtime.plugin as plugin

@plugin.register_function
def greet(name):
    return "hello " + name
`
	_, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Failed to execute script: %v", err)
	}

	RuntimeState.RLock()
	defer RuntimeState.RUnlock()

	if ref, ok := RuntimeState.PluginFunctions["greet"]; !ok || ref != "svc.greet" {
		t.Errorf("function 'greet': expected 'svc.greet', got %q", ref)
	}
}

func TestPluginClassDecoratorBare(t *testing.T) {
	ResetRuntime()
	p := scriptling.New()
	registerAll(p, nil)
	RegisterPlugin(p)
	p.SetVar("__name__", "config")

	script := `
import scriptling.runtime.plugin as plugin

@plugin.register_class
class Config:
    def __init__(self):
        self.version = "1.0"
`
	_, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Failed to execute script: %v", err)
	}

	RuntimeState.RLock()
	defer RuntimeState.RUnlock()

	if ref, ok := RuntimeState.PluginClasses["Config"]; !ok || ref != "config.Config" {
		t.Errorf("class 'Config': expected 'config.Config', got %q", ref)
	}
}

func TestPluginImperativeBackwardCompat(t *testing.T) {
	ResetRuntime()
	p := scriptling.New()
	registerAll(p, nil)
	RegisterPlugin(p)

	script := `
import scriptling.runtime.plugin as plugin

plugin.register_function("add", "handlers.add")
plugin.register_class("handlers.Config")
plugin.register_constant("VERSION", "1.0.0")
`
	_, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Failed to execute script: %v", err)
	}

	RuntimeState.RLock()
	defer RuntimeState.RUnlock()

	if ref, ok := RuntimeState.PluginFunctions["add"]; !ok || ref != "handlers.add" {
		t.Errorf("function 'add': expected 'handlers.add', got %q", ref)
	}
	if ref, ok := RuntimeState.PluginClasses["Config"]; !ok || ref != "handlers.Config" {
		t.Errorf("class 'Config': expected 'handlers.Config', got %q", ref)
	}
}
