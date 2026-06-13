package scriptling

import (
	"strings"
	"testing"

	"github.com/paularlott/scriptling/libloader"
)

func TestImportErrorCanBeCaughtAndIgnored(t *testing.T) {
	p := New()

	result, err := p.Eval(`
status = "before"
try:
    import definitely_missing_library
    status = "imported"
except ImportError:
    pass
status = status + ":after"
status
`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	if got := result.Inspect(); got != "before:after" {
		t.Fatalf("result = %q, want %q", got, "before:after")
	}
}

func TestUnhandledImportErrorStillFailsEval(t *testing.T) {
	p := New()

	_, err := p.Eval(`import definitely_missing_unhandled_library`)
	if err == nil {
		t.Fatal("expected unhandled import failure to return an Eval error")
	}
	if !strings.Contains(err.Error(), "import error") {
		t.Fatalf("Eval error = %q, want import error", err.Error())
	}
	if !strings.Contains(err.Error(), "unknown library") {
		t.Fatalf("Eval error = %q, want unknown library detail", err.Error())
	}
}

func TestImportErrorCanBeCaughtAndHandled(t *testing.T) {
	p := New()

	result, err := p.Eval(`
try:
    import missing_for_handler
    result = "not caught"
except ImportError as err:
    result = "caught:" + str(err)
result
`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	got := result.Inspect()
	if !strings.HasPrefix(got, "caught:import error") {
		t.Fatalf("result = %q, want ImportError handler output", got)
	}
}

func TestFromImportErrorCanBeCaught(t *testing.T) {
	p := New()
	if err := p.RegisterScriptLibrary("has_value", `VALUE = 42`); err != nil {
		t.Fatalf("RegisterScriptLibrary failed: %v", err)
	}

	result, err := p.Eval(`
try:
    from has_value import MISSING
    result = "not caught"
except ImportError:
    result = "caught missing name"
result
`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	if got := result.Inspect(); got != "caught missing name" {
		t.Fatalf("result = %q, want %q", got, "caught missing name")
	}
}

func TestImportErrorMatchesTupleExcept(t *testing.T) {
	p := New()

	result, err := p.Eval(`
try:
    import missing_tuple_library
    result = "not caught"
except (ValueError, ImportError):
    result = "caught tuple"
result
`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	if got := result.Inspect(); got != "caught tuple" {
		t.Fatalf("result = %q, want %q", got, "caught tuple")
	}
}

func TestSuccessfulImportInTryIsCachedForNextImport(t *testing.T) {
	p := New()
	loads := 0
	p.SetLibraryLoader(libloader.NewFuncLoader(func(name string) (string, bool, error) {
		loads++
		if name == "dynamic_try_lib" {
			return `VALUE = 7`, true, nil
		}
		return "", false, nil
	}, "test-loader"))

	result, err := p.Eval(`
try:
    import dynamic_try_lib as first
    first_value = first.VALUE
except ImportError:
    first_value = -1

import dynamic_try_lib as second
first_value + second.VALUE
`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	if got := result.Inspect(); got != "14" {
		t.Fatalf("result = %q, want %q", got, "14")
	}
	if loads != 1 {
		t.Fatalf("loader called %d times, want 1", loads)
	}
}

func TestScriptLibraryImportFailureCanBeCaught(t *testing.T) {
	p := New()
	if err := p.RegisterScriptLibrary("broken_import_lib", `
import missing_nested_dependency
VALUE = 1
`); err != nil {
		t.Fatalf("RegisterScriptLibrary failed: %v", err)
	}

	result, err := p.Eval(`
try:
    import broken_import_lib
    result = "not caught"
except ImportError:
    result = "caught nested import failure"
result
`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	if got := result.Inspect(); got != "caught nested import failure" {
		t.Fatalf("result = %q, want %q", got, "caught nested import failure")
	}
}
