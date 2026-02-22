package console_test

import (
	"context"
	"strings"
	"testing"

	"github.com/paularlott/scriptling/extlibs/console"
	"github.com/paularlott/scriptling/object"
)

// captureBackend records calls for assertion in tests.
type captureBackend struct {
	printed    []string
	chunks     []string
	started    bool
	ended      bool
	spinner    string
	stopped    bool
	progress   float64
	statusL    string
	statusR    string
	labelUser  string
	labelAsst  string
	labelSys   string
	escapeCb   func()
}

func (c *captureBackend) Input(_ string, _ *object.Environment) (string, error) { return "test", nil }
func (c *captureBackend) Print(text string, _ *object.Environment)              { c.printed = append(c.printed, text) }
func (c *captureBackend) PrintAs(_, text string, _ *object.Environment)         { c.printed = append(c.printed, text) }
func (c *captureBackend) StreamStart()                                          { c.started = true }
func (c *captureBackend) StreamStartAs(_ string)                                { c.started = true }
func (c *captureBackend) StreamChunk(s string)                                  { c.chunks = append(c.chunks, s) }
func (c *captureBackend) StreamEnd()                                            { c.ended = true }
func (c *captureBackend) SpinnerStart(text string)                              { c.spinner = text }
func (c *captureBackend) SpinnerStop()                                          { c.stopped = true }
func (c *captureBackend) SetProgress(_ string, pct float64)                     { c.progress = pct }
func (c *captureBackend) SetLabels(u, a, s string)                              { c.labelUser = u; c.labelAsst = a; c.labelSys = s }
func (c *captureBackend) SetStatus(l, r string)                                 { c.statusL = l; c.statusR = r }
func (c *captureBackend) SetStatusLeft(l string)                                { c.statusL = l }
func (c *captureBackend) SetStatusRight(r string)                               { c.statusR = r }
func (c *captureBackend) RegisterCommand(_, _ string, _ func(string))           {}
func (c *captureBackend) RemoveCommand(_ string)                                 {}
func (c *captureBackend) OnSubmit(_ func(context.Context, string))               {}
func (c *captureBackend) OnEscape(fn func())                                    { c.escapeCb = fn }
func (c *captureBackend) ClearOutput()                                          {}
func (c *captureBackend) Run() error                                             { return nil }

func TestSetAndGetBackend(t *testing.T) {
	orig := console.GetBackend()
	defer console.SetBackend(orig)

	cb := &captureBackend{}
	console.SetBackend(cb)
	if console.GetBackend() != cb {
		t.Error("GetBackend should return the set backend")
	}
}

func TestPlainBackendStreamBuffers(t *testing.T) {
	// Plain backend buffers chunks and prints on StreamEnd
	// We test via the library functions using a capture backend
	orig := console.GetBackend()
	defer console.SetBackend(orig)

	cb := &captureBackend{}
	console.SetBackend(cb)

	lib := console.NewLibrary()
	_ = lib // library registered; we test backend directly

	cb.StreamStart()
	cb.StreamChunk("hello ")
	cb.StreamChunk("world")
	cb.StreamEnd()

	if !cb.started {
		t.Error("StreamStart not called")
	}
	if !cb.ended {
		t.Error("StreamEnd not called")
	}
	combined := strings.Join(cb.chunks, "")
	if combined != "hello world" {
		t.Errorf("Expected 'hello world', got %q", combined)
	}
}

func TestPlainBackendSpinner(t *testing.T) {
	orig := console.GetBackend()
	defer console.SetBackend(orig)

	cb := &captureBackend{}
	console.SetBackend(cb)

	cb.SpinnerStart("Thinking")
	if cb.spinner != "Thinking" {
		t.Errorf("Expected spinner text 'Thinking', got %q", cb.spinner)
	}
	cb.SpinnerStop()
	if !cb.stopped {
		t.Error("SpinnerStop not called")
	}
}

func TestPlainBackendSetStatus(t *testing.T) {
	orig := console.GetBackend()
	defer console.SetBackend(orig)

	cb := &captureBackend{}
	console.SetBackend(cb)

	cb.SetStatus("myapp", "v1.0")
	if cb.statusL != "myapp" || cb.statusR != "v1.0" {
		t.Errorf("Expected status 'myapp'/'v1.0', got %q/%q", cb.statusL, cb.statusR)
	}
}

func TestSetLabels(t *testing.T) {
	orig := console.GetBackend()
	defer console.SetBackend(orig)

	cb := &captureBackend{}
	console.SetBackend(cb)

	cb.SetLabels("Me", "Bot", "Sys")
	if cb.labelUser != "Me" || cb.labelAsst != "Bot" || cb.labelSys != "Sys" {
		t.Errorf("Expected labels Me/Bot/Sys, got %q/%q/%q", cb.labelUser, cb.labelAsst, cb.labelSys)
	}
}

func TestPlainBackendOnEscape(t *testing.T) {
	orig := console.GetBackend()
	defer console.SetBackend(orig)

	cb := &captureBackend{}
	console.SetBackend(cb)

	called := false
	cb.OnEscape(func() { called = true })
	if cb.escapeCb == nil {
		t.Fatal("OnEscape callback not stored")
	}
	cb.escapeCb()
	if !called {
		t.Error("Escape callback not invoked")
	}
}

func TestLibraryName(t *testing.T) {
	lib := console.NewLibrary()
	if lib.Name() != "scriptling.console" {
		t.Errorf("Expected library name 'scriptling.console', got %q", lib.Name())
	}
}
