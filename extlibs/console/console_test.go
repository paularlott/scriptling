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

func TestModuleFunctionsExist(t *testing.T) {
	p := newInterpreter(t)
	_, err := p.Eval(`
import scriptling.console as console
assert hasattr(console, "panel")
assert hasattr(console, "main_panel")
assert hasattr(console, "create_panel")
assert hasattr(console, "add_left")
assert hasattr(console, "add_right")
assert hasattr(console, "clear_layout")
assert hasattr(console, "has_panels")
assert hasattr(console, "styled")
assert hasattr(console, "set_status")
assert hasattr(console, "set_status_left")
assert hasattr(console, "set_status_right")
assert hasattr(console, "set_labels")
assert hasattr(console, "register_command")
assert hasattr(console, "remove_command")
assert hasattr(console, "on_submit")
assert hasattr(console, "on_escape")
assert hasattr(console, "spinner_start")
assert hasattr(console, "spinner_stop")
assert hasattr(console, "set_progress")
assert hasattr(console, "run")
`)
	if err != nil {
		t.Fatalf("module functions check failed: %v", err)
	}
}

func TestMainPanelReturnsPanel(t *testing.T) {
	p := newInterpreter(t)
	_, err := p.Eval(`
import scriptling.console as console
main = console.main_panel()
assert main is not None
assert hasattr(main, "add_message")
assert hasattr(main, "stream_start")
assert hasattr(main, "stream_chunk")
assert hasattr(main, "stream_end")
assert hasattr(main, "clear")
assert hasattr(main, "styled")
`)
	if err != nil {
		t.Fatalf("main_panel check failed: %v", err)
	}
}

func TestPanelDefaultIsMain(t *testing.T) {
	p := newInterpreter(t)
	_, err := p.Eval(`
import scriptling.console as console
p = console.panel()
assert p is not None
`)
	if err != nil {
		t.Fatalf("panel() default check failed: %v", err)
	}
}
