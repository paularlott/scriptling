package scriptling

import (
	"os"
	"strings"
	"testing"

	"github.com/paularlott/scriptling/extlibs"
	"github.com/paularlott/scriptling/object"
)

func TestTryExcept(t *testing.T) {
	p := New()
	_, err := p.Eval(`
result = 0
try:
    x = 10 / 0
except:
    result = 1
`)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}

	result, objErr := p.GetVar("result")
	if objErr != nil || result != int64(1) {
		t.Errorf("result = %v, want 1", result)
	}
}

func TestTryFinally(t *testing.T) {
	p := New()
	_, err := p.Eval(`
cleanup = 0
try:
    x = 5 + 5
finally:
    cleanup = 1
`)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}

	cleanup, objErr := p.GetVar("cleanup")
	if objErr != nil || cleanup != int64(1) {
		t.Errorf("cleanup = %v, want 1", cleanup)
	}
}

func TestTryExceptFinally(t *testing.T) {
	p := New()
	_, err := p.Eval(`
result = 0
cleanup = 0
try:
    x = 10 / 0
except:
    result = 1
finally:
    cleanup = 1
`)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}

	result, objErr := p.GetVar("result")
	cleanup, objErr := p.GetVar("cleanup")

	if objErr != nil || result != int64(1) {
		t.Errorf("result = %v, want 1", result)
	}
	if objErr != nil || cleanup != int64(1) {
		t.Errorf("cleanup = %v, want 1", cleanup)
	}
}

func TestRaiseStatement(t *testing.T) {
	p := New()
	_, err := p.Eval(`
caught = 0
try:
    raise "Test error"
except:
    caught = 1
`)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}

	caught, objErr := p.GetVar("caught")
	if objErr != nil || caught != int64(1) {
		t.Errorf("caught = %v, want 1", caught)
	}
}

func TestRaiseInFunction(t *testing.T) {
	p := New()
	_, err := p.Eval(`
def check_positive(n):
    if n < 0:
        raise "Value must be positive"
    return n * 2

result = 0
try:
    result = check_positive(-5)
except:
    result = -1
`)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}

	result, objErr := p.GetVar("result")
	if objErr != nil || result != int64(-1) {
		t.Errorf("result = %v, want -1", result)
	}
}

func TestUncaughtExceptionTerminates(t *testing.T) {
	p := New()
	_, err := p.Eval(`
x = 1
raise "uncaught error"
x = 2
`)
	// Should return an error for uncaught exception
	if err == nil {
		t.Error("Expected error for uncaught exception, got nil")
	}

	// Error message should contain the exception message
	if err != nil && !strings.Contains(err.Error(), "uncaught error") {
		t.Errorf("Error should contain 'uncaught error', got: %v", err)
	}

	// x should still be 1 (execution stopped before x = 2)
	x, objErr := p.GetVar("x")
	if objErr != nil || x != int64(1) {
		t.Errorf("x = %v, want 1 (execution should have stopped)", x)
	}
}

func TestStrException(t *testing.T) {
	p := New()
	_, err := p.Eval(`
message = ""
try:
    raise "test error message"
except Exception as e:
    message = str(e)
`)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}

	message, objErr := p.GetVar("message")
	if objErr != nil {
		t.Fatalf("Failed to get message: %v", objErr)
	}

	// str(exception) should return just the message, not "EXCEPTION: message"
	expected := "test error message"
	if message != expected {
		t.Errorf("str(exception) = %q, want %q", message, expected)
	}
}

func TestBareRaiseReRaises(t *testing.T) {
	p := New()
	_, err := p.Eval(`
caught_msg = ""
try:
    raise "original error"
except Exception as e:
    caught_msg = str(e)
    raise
`)
	// Should return an error for re-raised exception
	if err == nil {
		t.Error("Expected error for re-raised exception, got nil")
	}

	// Error should contain the original message
	if err != nil && !strings.Contains(err.Error(), "original error") {
		t.Errorf("Error should contain 'original error', got: %v", err)
	}

	// Should have captured the message before re-raising
	caught, objErr := p.GetVar("caught_msg")
	if objErr != nil || caught != "original error" {
		t.Errorf("caught_msg = %q, want 'original error'", caught)
	}
}

func TestBareRaiseOutsideExceptFails(t *testing.T) {
	p := New()
	_, err := p.Eval(`
raise
`)
	// Should return an error for bare raise outside except
	if err == nil {
		t.Error("Expected error for bare raise outside except, got nil")
	}

	// Error should indicate no active exception
	if err != nil && !strings.Contains(err.Error(), "No active exception") {
		t.Errorf("Error should contain 'No active exception', got: %v", err)
	}
}

func TestSysExitRaisesException(t *testing.T) {
	p := New()
	extlibs.RegisterSysLibrary(p, []string{})

	// Test that sys.exit() raises an exception that can be caught
	code := `
import sys

result = None
try:
    sys.exit(42)
except Exception as e:
    result = str(e)

result
`
	result, err := p.Eval(code)
	if err != nil {
		t.Fatalf("Failed to execute: %v", err)
	}

	if result.Type() != object.STRING_OBJ {
		t.Fatalf("Expected STRING, got %s", result.Type())
	}

	msg := result.(*object.String).Value
	if msg != "SystemExit: 42" {
		t.Errorf("Expected 'SystemExit: 42', got %s", msg)
	}
}

func TestSysExitWithStringMessage(t *testing.T) {
	p := New()
	extlibs.RegisterSysLibrary(p, []string{})

	// Test that sys.exit() with string message can be caught
	code := `
import sys

result = None
try:
    sys.exit("custom error")
except Exception as e:
    result = str(e)

result
`
	result, err := p.Eval(code)
	if err != nil {
		t.Fatalf("Failed to execute: %v", err)
	}

	if result.Type() != object.STRING_OBJ {
		t.Fatalf("Expected STRING, got %s", result.Type())
	}

	msg := result.(*object.String).Value
	if msg != "custom error" {
		t.Errorf("Expected 'custom error', got %s", msg)
	}
}

func TestSysExitUncaughtTerminates(t *testing.T) {
	p := New()
	extlibs.RegisterSysLibrary(p, []string{})

	// Test that uncaught sys.exit() returns SysExitCode error
	code := `
import sys

sys.exit(99)
result = "should not reach here"
`
	_, err := p.Eval(code)
	if err == nil {
		t.Fatal("Expected error for uncaught sys.exit()")
	}

	// Should be a SysExitCode error with the correct exit code
	sysExit, ok := extlibs.GetSysExitCode(err)
	if !ok {
		t.Fatalf("Expected *extlibs.SysExitCode error, got: %T", err)
	}

	if sysExit.Code != 99 {
		t.Errorf("Expected exit code 99, got: %d", sysExit.Code)
	}

	// Verify that execution stopped before the assignment
	_, objErr := p.GetVar("result")
	if objErr == nil {
		t.Error("Expected result variable to not exist (execution should have stopped)")
	}
}

func TestSysExitDefaultCode(t *testing.T) {
	p := New()
	extlibs.RegisterSysLibrary(p, []string{})

	// Test that sys.exit() with no args defaults to code 0
	code := `
import sys
sys.exit()
`
	_, err := p.Eval(code)
	if err == nil {
		t.Fatal("Expected error for uncaught sys.exit()")
	}

	sysExit, ok := err.(*extlibs.SysExitCode)
	if !ok {
		t.Fatalf("Expected *extlibs.SysExitCode error, got: %T", err)
	}

	if sysExit.Code != 0 {
		t.Errorf("Expected exit code 0 (default), got: %d", sysExit.Code)
	}
}

func TestSysExitInFunction(t *testing.T) {
	p := New()
	extlibs.RegisterSysLibrary(p, []string{})

	// Test that sys.exit() called from a function returns SysExitCode
	code := `
import sys

def my_function():
    sys.exit(123)

my_function()
`
	_, err := p.Eval(code)
	if err == nil {
		t.Fatal("Expected error for sys.exit() in function")
	}

	sysExit, ok := err.(*extlibs.SysExitCode)
	if !ok {
		t.Fatalf("Expected *extlibs.SysExitCode error, got: %T", err)
	}

	if sysExit.Code != 123 {
		t.Errorf("Expected exit code 123, got: %d", sysExit.Code)
	}
}

func TestSysExitViaCallFunction(t *testing.T) {
	p := New()
	extlibs.RegisterSysLibrary(p, []string{})

	// Register a function that calls sys.exit
	_, err := p.Eval(`
import sys

def exit_func():
    sys.exit(45)
`)
	if err != nil {
		t.Fatalf("Failed to define function: %v", err)
	}

	// Call the function via CallFunction API
	_, err = p.CallFunction("exit_func")
	if err == nil {
		t.Fatal("Expected error for sys.exit() via CallFunction")
	}

	sysExit, ok := err.(*extlibs.SysExitCode)
	if !ok {
		t.Fatalf("Expected *extlibs.SysExitCode error, got: %T", err)
	}

	if sysExit.Code != 45 {
		t.Errorf("Expected exit code 45, got: %d", sysExit.Code)
	}
}

func TestSysExitDoesNotKillCaller(t *testing.T) {
	p := New()
	extlibs.RegisterSysLibrary(p, []string{})

	// Test that sys.exit() doesn't terminate the Go process
	// If it did, this test would crash/exit
	code := `
import sys
sys.exit(1)
`
	_, err := p.Eval(code)
	if err == nil {
		t.Fatal("Expected error for uncaught sys.exit()")
	}

	// If we reach here, sys.exit() did NOT terminate the Go process
	// (which is the correct behavior now)

	sysExit, ok := err.(*extlibs.SysExitCode)
	if !ok {
		t.Fatalf("Expected *extlibs.SysExitCode error, got: %T", err)
	}

	if sysExit.Code != 1 {
		t.Errorf("Expected exit code 1, got: %d", sysExit.Code)
	}

	// Verify we can still use the interpreter
	_, err = p.Eval("x = 42")
	if err != nil {
		t.Errorf("Interpreter should still be usable after sys.exit(), got: %v", err)
	}
}

func TestErrorHandlingScript(t *testing.T) {
	script, err := os.ReadFile("examples/error_handling_test.py")
	if err != nil {
		t.Skipf("Skipping: %v", err)
		return
	}

	p := New()
	_, err = p.Eval(string(script))
	if err != nil {
		t.Fatalf("Error handling script failed: %v", err)
	}
}
