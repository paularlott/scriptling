// Package scripttest holds helpers shared by the library registration tests:
// evaluating a script with clear failure messages, and asserting that an
// import succeeds, or fails the way a script can catch. It is imported only
// from _test.go files.
package scripttest

import (
	"errors"
	"strings"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/object"
)

// Eval runs src on p and fails the test if it errors or panics.
func Eval(t testing.TB, p *scriptling.Scriptling, src string) object.Object {
	t.Helper()
	res, err := p.Eval(src)
	var pe *scriptling.PanicError
	if errors.As(err, &pe) {
		t.Fatalf("script panicked (a library must return errors, never panic): %v\nscript:\n%s", pe, src)
	}
	if err != nil {
		t.Fatalf("script failed: %v\nscript:\n%s", err, src)
	}
	return res
}

// EvalString runs src and returns its result as a string.
func EvalString(t testing.TB, p *scriptling.Scriptling, src string) string {
	t.Helper()
	res := Eval(t, p, src)
	s, oerr := res.AsString()
	if oerr != nil {
		t.Fatalf("script result %s is not a string\nscript:\n%s", res.Inspect(), src)
	}
	return s
}

// EvalInt runs src and returns its result as an int.
func EvalInt(t testing.TB, p *scriptling.Scriptling, src string) int64 {
	t.Helper()
	res := Eval(t, p, src)
	n, oerr := res.AsInt()
	if oerr != nil {
		t.Fatalf("script result %s is not an int\nscript:\n%s", res.Inspect(), src)
	}
	return n
}

// Imports asserts that `import name` succeeds on p.
func Imports(t testing.TB, p *scriptling.Scriptling, name string) {
	t.Helper()
	_, err := p.Eval("import " + name)
	var pe *scriptling.PanicError
	if errors.As(err, &pe) {
		t.Fatalf("import %s panicked: %v", name, pe)
	}
	if err != nil {
		t.Fatalf("import %s failed after registration: %v", name, err)
	}
}

// ImportFails asserts that `import name` on p fails with an import error,
// without panicking, and that a script can catch it as ImportError. hint
// tells the reader what registering the library would take, for the failure
// message.
func ImportFails(t testing.TB, p *scriptling.Scriptling, name, hint string) {
	t.Helper()
	_, err := p.Eval("import " + name)
	var pe *scriptling.PanicError
	switch {
	case errors.As(err, &pe):
		t.Fatalf("import %s panicked instead of failing cleanly: %v", name, pe)
	case err == nil:
		t.Fatalf("import %s succeeded on an interpreter that never registered it; %s", name, hint)
	case !strings.Contains(err.Error(), "unknown library"):
		t.Fatalf("import %s failed, but not with an unknown-library import error: %v", name, err)
	}

	src := "try:\n    import " + name + "\n    __r = 'imported'\nexcept ImportError as e:\n    __r = 'ImportError'\n__r"
	res, err := p.Eval(src)
	if err != nil {
		t.Fatalf("import %s inside try/except ImportError escaped the handler: %v", name, err)
	}
	if got, _ := res.AsString(); got != "ImportError" {
		t.Fatalf("import %s inside try/except: got %q, want the ImportError to be caught", name, got)
	}
}

// Refused asserts that src is stopped by a security policy: the script fails
// with a PermissionError even though src runs inside
// `try: ... except Exception:`. Policy refusals (path outside allowedPaths,
// and the like) are protected exceptions that a handler cannot swallow (see
// isProtected in the evaluator), so the error must reach the embedder. It
// returns the error message.
func Refused(t testing.TB, p *scriptling.Scriptling, src string) string {
	t.Helper()
	var b strings.Builder
	b.WriteString("try:\n")
	for _, l := range strings.Split(strings.TrimRight(src, "\n"), "\n") {
		b.WriteString("    " + l + "\n")
	}
	b.WriteString("    __r = 'no exception'\nexcept Exception as e:\n    __r = 'swallowed: ' + type(e).__name__ + ': ' + str(e)\n__r")
	res, err := p.Eval(b.String())
	var pe *scriptling.PanicError
	switch {
	case errors.As(err, &pe):
		t.Fatalf("policy check panicked: %v\nscript:\n%s", pe, src)
	case err == nil:
		got := ""
		if res != nil {
			got, _ = res.AsString()
		}
		t.Fatalf("expected a PermissionError the script cannot swallow, got %q\nscript:\n%s", got, src)
	}
	if ex, ok := res.(*object.Exception); !ok || ex.ExceptionType != object.ExceptionTypePermissionError {
		t.Fatalf("policy refusal must be a PermissionError, got %T %v\nscript:\n%s", res, err, src)
	}
	return err.Error()
}

// Raises runs src inside `try: ... except Exception as e:` on p and returns
// the caught exception's type name and message. It fails the test when src
// raises nothing, or the error escapes the handler (so a script could not
// catch it), or the interpreter panics.
func Raises(t testing.TB, p *scriptling.Scriptling, src string) (excType, msg string) {
	t.Helper()
	var b strings.Builder
	b.WriteString("try:\n")
	for _, l := range strings.Split(strings.TrimRight(src, "\n"), "\n") {
		b.WriteString("    " + l + "\n")
	}
	b.WriteString("    __r = ['', 'no exception']\nexcept Exception as e:\n    __r = [type(e).__name__, str(e)]\n__r")
	res := Eval(t, p, b.String())
	list, ok := res.(*object.List)
	if !ok || len(list.Elements) != 2 {
		t.Fatalf("unexpected handler result %s", res.Inspect())
	}
	excType, _ = list.Elements[0].AsString()
	msg, _ = list.Elements[1].AsString()
	if excType == "" {
		t.Fatalf("expected the script to raise a catchable exception, but it ran to completion:\n%s", src)
	}
	return excType, msg
}
