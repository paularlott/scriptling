package evaluator

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/paularlott/scriptling/lexer"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/parser"
)

func TestExceptionTypeMatching(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		isError  bool
	}{
		{
			name: "bare except catches all",
			input: `
try:
    x = 1 / 0
except:
    result = "caught"
result
`,
			expected: "caught",
		},
		{
			name: "except Exception catches all",
			input: `
try:
    x = 1 / 0
except Exception as e:
    result = "caught"
result
`,
			expected: "caught",
		},
		{
			name: "specific exception type doesn't match",
			input: `
result = "not caught"
try:
    try:
        x = 1 / 0
    except ValueError as e:
        result = "caught ValueError"
except:
    result = "outer caught"
result
`,
			expected: "outer caught",
		},
		{
			name: "raise Exception with message",
			input: `
try:
    raise Exception("test error")
except Exception as e:
    result = str(e)
result
`,
			expected: "test error",
		},
		{
			name: "raise ValueError",
			input: `
try:
    raise ValueError("bad value")
except ValueError as e:
    result = str(e)
result
`,
			expected: "bad value",
		},
		{
			name: "ValueError doesn't match TypeError",
			input: `
try:
    raise ValueError("bad value")
except TypeError as e:
    result = "caught TypeError"
except:
    result = "caught by bare except"
result
`,
			expected: "caught by bare except",
		},
		{
			name: "Exception catches ValueError",
			input: `
try:
    raise ValueError("bad value")
except Exception as e:
    result = "caught"
result
`,
			expected: "caught",
		},
		{
			name: "tuple except catches matching exception",
			input: `
try:
    raise ValueError("bad value")
except (TypeError, ValueError) as e:
    result = str(e)
result
`,
			expected: "bad value",
		},
		{
			name: "tuple except matches first type",
			input: `
try:
    raise TypeError("wrong type")
except (TypeError, ValueError) as e:
    result = str(e)
result
`,
			expected: "wrong type",
		},
		{
			name:    "tuple except does not catch non-matching exception",
			isError: true,
			input: `
try:
    raise RuntimeError("boom")
except (TypeError, ValueError):
    result = "should not get here"
result
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.New(tt.input)
			p := parser.New(l)
			program := p.ParseProgram()

			if len(p.Errors()) != 0 {
				t.Fatalf("parser errors: %v", p.Errors())
			}

			env := object.NewEnvironment()
			result := EvalWithContext(context.Background(), program, env)

			if tt.isError {
				if !object.IsError(result) {
					t.Fatalf("expected error, got %T (%+v)", result, result)
				}
				return
			}

			if object.IsError(result) {
				t.Fatalf("unexpected error: %s", result.Inspect())
			}

			if result.Inspect() != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result.Inspect())
			}
		})
	}
}

// TestExceptBlockReturnPropagation ensures that return/break/continue inside
// an except block are not swallowed (regression for the result=NULL bug).
func TestExceptBlockReturnPropagation(t *testing.T) {
	t.Run("return from except propagates out of function", func(t *testing.T) {
		input := `
def f():
    try:
        raise Exception("boom")
    except Exception as e:
        return "caught: " + str(e)

f()
`
		l := lexer.New(input)
		p := parser.New(l)
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatalf("parser errors: %v", p.Errors())
		}
		env := object.NewEnvironment()
		result := EvalWithContext(context.Background(), program, env)
		if object.IsError(result) {
			t.Fatalf("unexpected error: %s", result.Inspect())
		}
		if result.Inspect() != "caught: boom" {
			t.Errorf("expected %q, got %q", "caught: boom", result.Inspect())
		}
	})

	t.Run("break from except exits loop", func(t *testing.T) {
		input := `
result = 0
for i in range(5):
    try:
        if i == 2:
            raise Exception("stop")
    except Exception:
        break
    result = i
result
`
		l := lexer.New(input)
		p := parser.New(l)
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatalf("parser errors: %v", p.Errors())
		}
		env := object.NewEnvironment()
		result := EvalWithContext(context.Background(), program, env)
		if object.IsError(result) {
			t.Fatalf("unexpected error: %s", result.Inspect())
		}
		if result.Inspect() != "1" {
			t.Errorf("expected %q, got %q", "1", result.Inspect())
		}
	})

	t.Run("continue from except skips to next iteration", func(t *testing.T) {
		input := `
result = []
for i in range(4):
    try:
        if i == 2:
            raise Exception("skip")
    except Exception:
        continue
    result.append(i)
result
`
		l := lexer.New(input)
		p := parser.New(l)
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatalf("parser errors: %v", p.Errors())
		}
		env := object.NewEnvironment()
		result := EvalWithContext(context.Background(), program, env)
		if object.IsError(result) {
			t.Fatalf("unexpected error: %s", result.Inspect())
		}
		// Should be [0, 1, 3] — 2 was skipped
		list, ok := result.(*object.List)
		if !ok {
			t.Fatalf("expected List, got %T", result)
		}
		if len(list.Elements) != 3 {
			t.Errorf("expected 3 elements, got %d: %s", len(list.Elements), result.Inspect())
		}
	})
}

func TestExceptionInspect(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains string
	}{
		{
			name: "exception variable contains message",
			input: `
try:
    raise Exception("test message")
except Exception as e:
    result = str(e)
result
`,
			contains: "test message",
		},
		{
			name: "error converted to exception",
			input: `
try:
    x = 1 / 0
except Exception as e:
    result = str(e)
result
`,
			contains: "division by zero",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.New(tt.input)
			p := parser.New(l)
			program := p.ParseProgram()

			if len(p.Errors()) != 0 {
				t.Fatalf("parser errors: %v", p.Errors())
			}

			env := object.NewEnvironment()
			result := EvalWithContext(context.Background(), program, env)

			if object.IsError(result) {
				t.Fatalf("unexpected error: %s", result.Inspect())
			}

			resultStr := result.Inspect()
			if len(resultStr) < len(tt.contains) || resultStr[:len(tt.contains)] != tt.contains &&
				!contains(resultStr, tt.contains) {
				t.Errorf("expected result to contain %q, got %q", tt.contains, resultStr)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// evalInEnv runs source through the package's own lexer/parser/evaluator.
func evalInEnv(t *testing.T, src string) (object.Object, error) {
	t.Helper()
	l := lexer.New(src)
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}
	result := EvalWithContext(context.Background(), program, object.NewEnvironment())
	if result == nil {
		t.Fatal("nil result")
	}
	if isException(result) || object.IsError(result) {
		return nil, fmt.Errorf("%s", result.Inspect())
	}
	return result, nil
}

// TestFinallyControlFlow pins what a finally block may do to the try
// statement's outcome: a raise replaces the in-flight result (an exception
// raised in finally must not be swallowed), break and continue propagate
// instead of being dropped, and return still overrides. Before this was
// fixed, only return was honored: `finally: raise` was discarded and the
// function returned normally.
func TestFinallyControlFlow(t *testing.T) {
	// raise in finally replaces the pending return
	_, err := evalInEnv(t, `
def f():
    try:
        return 1
    finally:
        raise ValueError("cleanup failed")
f()
`)
	if err == nil {
		t.Fatal("expected the exception raised in finally to propagate")
	}
	if !strings.Contains(err.Error(), "cleanup failed") {
		t.Fatalf("expected the finally exception to surface, got: %v", err)
	}

	// raise in finally replaces an in-flight exception too
	_, err = evalInEnv(t, `
try:
    raise ValueError("original")
finally:
    raise ValueError("replacement")
`)
	if err == nil || !strings.Contains(err.Error(), "replacement") {
		t.Fatalf("expected the finally exception to replace the original, got: %v", err)
	}

	// break in finally leaves the loop instead of being ignored
	result, err := evalInEnv(t, `
count = 0
for i in range(3):
    try:
        count = count + 1
    finally:
        break
count
`)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if result.Inspect() != "1" {
		t.Fatalf("break in finally should leave the loop after one pass, count = %s", result.Inspect())
	}

	// a finally that ends normally still changes nothing
	result, err = evalInEnv(t, `
def g():
    try:
        return 42
    finally:
        cleanup = 1
g()
`)
	if err != nil || result.Inspect() != "42" {
		t.Fatalf("plain finally must not disturb the result: %v %s", err, result.Inspect())
	}
}

// TestFinallyReturnNotOverridden pins that a return inside finally stays a
// control-flow marker: unwrapping it too early let a later statement (here,
// the second return) replace the finally block's answer.
func TestFinallyReturnNotOverridden(t *testing.T) {
	result, err := evalInEnv(t, `
def f():
    try:
        return 1
    finally:
        return 2
    return 3
f()
`)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if result.Inspect() != "2" {
		t.Fatalf("finally return must win over a later return, got %s", result.Inspect())
	}
}

// TestDelFinalizerCannotCrashProcess pins the finalizer's panic boundary: a
// __del__ that raises runs on the runtime's finalizer goroutine, where a
// panic is fatal to the whole process. The host must survive collection.
func TestDelFinalizerCannotCrashProcess(t *testing.T) {
	_, err := evalInEnv(t, `
class Boom:
    def __del__(self):
        raise ValueError("boom in destructor")
Boom()
"made"
`)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	for i := 0; i < 5; i++ {
		runtime.GC()
	}
	time.Sleep(100 * time.Millisecond)
	// Reaching here means the finalizer did not take the process down.
}
