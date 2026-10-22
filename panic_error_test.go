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
