package scriptling

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/paularlott/scriptling/object"
)

// A panic during evaluation is reported without its Go stack trace in the
// error message, which may be shown to users or sent to clients; the trace
// stays available to the host through PanicError.
func TestPanicErrorMessageOmitsStack(t *testing.T) {
	p := New()
	p.RegisterFunc("explode", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
		var s []int
		return object.NewInteger(int64(s[3]))
	})
	result, err := p.Eval("explode()")
	if err == nil {
		t.Fatal("expected a panic error")
	}
	msg := err.Error()
	if msg != "script panic: runtime error: index out of range [3] with length 0" {
		t.Fatalf("unexpected message %q", msg)
	}
	for _, leak := range []string{"goroutine", ".go:", "\n"} {
		if strings.Contains(msg, leak) {
			t.Fatalf("message leaks stack detail %q: %q", leak, msg)
		}
	}
	var panicErr *PanicError
	if !errors.As(err, &panicErr) {
		t.Fatalf("error is %T, want *PanicError", err)
	}
	if stack := string(panicErr.Stack); !strings.Contains(stack, "goroutine") || !strings.Contains(stack, "panic_error_test.go") {
		t.Fatalf("Stack does not hold the trace: %.200q", stack)
	}
	if !strings.Contains(panicErr.Value.(error).Error(), "index out of range") {
		t.Fatalf("Value = %v", panicErr.Value)
	}
	if !object.IsError(result) || result.Inspect() == "" {
		t.Fatalf("result = %v", result)
	}
	// The interpreter stays usable after a recovered panic.
	if r, err := p.Eval("1 + 1"); err != nil || r.Inspect() != "2" {
		t.Fatalf("interpreter unusable after panic: %v %v", r, err)
	}
}

func newExplodingScriptling(t *testing.T) *Scriptling {
	t.Helper()
	p := New()
	p.RegisterFunc("explode", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
		var s []int
		return object.NewInteger(int64(s[3]))
	})
	p.RegisterLibrary(object.NewLibrary("nat", map[string]*object.Builtin{
		"explode": {Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			panic("in library")
		}},
	}, nil, ""))
	if err := p.RegisterScriptLibrary("boom", "import nat\nnat.explode()\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Eval(`
def f():
    return explode()

class C:
    def __init__(self):
        explode()

class D:
    def m(self):
        return explode()

d = D()
`); err != nil {
		t.Fatal(err)
	}
	return p
}

// Every entry point that runs script code recovers a Go panic the same way
// Eval does: a *PanicError with a one-line message, the trace in Stack, the
// panic handler called once, and the interpreter still usable. The call APIs
// used to let the panic escape to the caller.
func TestEntryPointsRecoverPanics(t *testing.T) {
	ctx := context.Background()
	for name, call := range map[string]func(p *Scriptling) error{
		"EvalWithContext": func(p *Scriptling) error {
			_, err := p.EvalWithContext(ctx, "explode()")
			return err
		},
		"CallFunctionWithContext": func(p *Scriptling) error {
			_, err := p.CallFunctionWithContext(ctx, "f")
			return err
		},
		"CallFunction": func(p *Scriptling) error {
			_, err := p.CallFunction("f")
			return err
		},
		"CreateInstanceWithContext": func(p *Scriptling) error {
			_, err := p.CreateInstanceWithContext(ctx, "C")
			return err
		},
		"CallMethodWithContext": func(p *Scriptling) error {
			d, err := p.GetVarAsObject("d")
			if err != nil {
				t.Fatal(err)
			}
			_, err = p.CallMethodWithContext(ctx, d, "m")
			return err
		},
		"ImportWithContext": func(p *Scriptling) error {
			return p.ImportWithContext(ctx, "boom")
		},
		"LoadLibraryIntoEnvWithContext": func(p *Scriptling) error {
			return p.LoadLibraryIntoEnvWithContext(ctx, "boom", object.NewEnvironment())
		},
	} {
		p := newExplodingScriptling(t)
		var handled []*PanicError
		p.SetPanicHandler(func(e *PanicError) { handled = append(handled, e) })

		var err error
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("%s: panic escaped to the caller: %v", name, r)
				}
			}()
			err = call(p)
		}()

		var panicErr *PanicError
		if !errors.As(err, &panicErr) {
			t.Fatalf("%s: error = %T %v, want *PanicError", name, err, err)
		}
		if msg := err.Error(); strings.Contains(msg, "goroutine") || strings.Contains(msg, "\n") {
			t.Errorf("%s: message carries the stack: %q", name, msg)
		}
		if !strings.Contains(string(panicErr.Stack), "panic_error_test.go") {
			t.Errorf("%s: Stack lacks the panicking frame", name)
		}
		if len(handled) != 1 || handled[0] != panicErr {
			t.Errorf("%s: panic handler called %d times", name, len(handled))
		}
		if r, err := p.Eval("1 + 1"); err != nil || r.Inspect() != "2" {
			t.Errorf("%s: interpreter unusable after the panic: %v %v", name, r, err)
		}
	}
}

// A clone keeps the panic handler, and without one nothing is called.
func TestPanicHandlerClonedAndOptional(t *testing.T) {
	p := newExplodingScriptling(t)
	if _, err := p.Eval("explode()"); err == nil {
		t.Fatal("expected a panic error without a handler")
	}
	calls := 0
	p.SetPanicHandler(func(*PanicError) { calls++ })
	child := p.Clone()
	child.RegisterFunc("explode", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
		panic("child")
	})
	if _, err := child.Eval("explode()"); err == nil || err.Error() != "script panic: child" {
		t.Fatalf("child error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("clone did not keep the panic handler: %d calls", calls)
	}
}
