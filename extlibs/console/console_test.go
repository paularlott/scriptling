package console_test

import (
	"testing"

	scriptlib "github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/console"
	"github.com/paularlott/scriptling/stdlib"
)

func newInterpreter(t *testing.T) *scriptlib.Scriptling {
	t.Helper()
	p := scriptlib.New()
	stdlib.RegisterAll(p)
	console.Register(p)
	return p
}

func TestLibraryName(t *testing.T) {
	lib := console.NewLibrary()
	if lib.Name() != "scriptling.console" {
		t.Errorf("expected library name 'scriptling.console', got %q", lib.Name())
	}
}

func TestConsoleClassExists(t *testing.T) {
	p := newInterpreter(t)
	_, err := p.Eval(`
import scriptling.console as console
c = console.Console()
`)
	if err != nil {
		t.Fatalf("Console() constructor failed: %v", err)
	}
}

func TestConsoleColorConstants(t *testing.T) {
	p := newInterpreter(t)
	_, err := p.Eval(`
import scriptling.console as console
assert console.PRIMARY == "primary"
assert console.SECONDARY == "secondary"
assert console.ERROR == "error"
assert console.DIM == "dim"
assert console.USER == "user"
assert console.TEXT == "text"
`)
	if err != nil {
		t.Fatalf("color constants check failed: %v", err)
	}
}

func TestConsoleMethodsExist(t *testing.T) {
	p := newInterpreter(t)
	_, err := p.Eval(`
import scriptling.console as console
c = console.Console()
# Verify all expected methods are callable attributes
assert hasattr(c, "add_message")
assert hasattr(c, "stream_start")
assert hasattr(c, "stream_chunk")
assert hasattr(c, "stream_end")
assert hasattr(c, "spinner_start")
assert hasattr(c, "spinner_stop")
assert hasattr(c, "set_progress")
assert hasattr(c, "set_labels")
assert hasattr(c, "set_status")
assert hasattr(c, "set_status_left")
assert hasattr(c, "set_status_right")
assert hasattr(c, "register_command")
assert hasattr(c, "remove_command")
assert hasattr(c, "clear_output")
assert hasattr(c, "styled")
assert hasattr(c, "on_escape")
assert hasattr(c, "on_submit")
assert hasattr(c, "run")
`)
	if err != nil {
		t.Fatalf("method existence check failed: %v", err)
	}
}

func TestMultipleIndependentInstances(t *testing.T) {
	p := newInterpreter(t)
	// Two Console instances should be independent objects
	_, err := p.Eval(`
import scriptling.console as console
c1 = console.Console()
c2 = console.Console()
assert c1 is not c2
`)
	if err != nil {
		t.Fatalf("independent instances check failed: %v", err)
	}
}
