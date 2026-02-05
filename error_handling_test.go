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

func TestSysExitIsUncatchable(t *testing.T) {
	p := New()
	extlibs.RegisterSysLibrary(p, []string{})

	// Test that sys.exit() CANNOT be caught by try/except blocks
	// sys.exit() always exits the program, even inside try/except
	code := `
import sys

result = "not caught"
try:
    sys.exit(42)
    result = "should not reach here"
except Exception as e:
    # This should NOT execute because SystemExit is uncatchable
    result = "caught"

result
`
	result, err := p.Eval(code)

	// SystemExit should NOT be caught - should get error
	if err == nil {
		t.Fatal("Expected error for uncatchable SystemExit")
	}

	// Check for SystemExit exception
	ex, ok := object.AsException(result)
	if !ok || !ex.IsSystemExit() {
		t.Fatalf("Expected SystemExit exception, got: %T (result=%v, err=%v)", result, result, err)
	}

	if ex.GetExitCode() != 42 {
		t.Errorf("Expected exit code 42, got: %d", ex.GetExitCode())
	}

	// Verify that result was never changed (except block didn't execute)
	resultVal, objErr := p.GetVar("result")
	if objErr != nil {
		t.Fatalf("Failed to get result variable: %v", objErr)
	}
	if resultVal != "not caught" {
		t.Errorf("Expected result to be 'not caught' (except block didn't execute), got: %v", resultVal)
	}
}

func TestSysExitWithStringMessage(t *testing.T) {
	p := New()
	extlibs.RegisterSysLibrary(p, []string{})

	// Test that sys.exit() with string message is also uncatchable
	code := `
import sys

result = "not caught"
try:
    sys.exit("custom error")
    result = "should not reach here"
except Exception as e:
    # This should NOT execute because SystemExit is uncatchable
    result = "caught"

result
`
	result, err := p.Eval(code)

	// SystemExit should NOT be caught - should get error
	if err == nil {
		t.Fatal("Expected error for uncatchable SystemExit")
	}

	// Check for SystemExit exception with the custom message
	ex, ok := object.AsException(result)
	if !ok || !ex.IsSystemExit() {
		t.Fatalf("Expected SystemExit exception, got: %T (result=%v, err=%v)", result, result, err)
	}

	if ex.GetExitCode() != 1 {
		t.Errorf("Expected exit code 1 for string message, got: %d", ex.GetExitCode())
	}

	// Verify the message was set (stored in exception.Message)
	msg := ex.Message
	if msg != "custom error" {
		t.Errorf("Expected message 'custom error', got: %s", msg)
	}

	// Verify that result was never changed (except block didn't execute)
	resultVal, objErr := p.GetVar("result")
	if objErr != nil {
		t.Fatalf("Failed to get result variable: %v", objErr)
	}
	if resultVal != "not caught" {
		t.Errorf("Expected result to be 'not caught' (except block didn't execute), got: %v", resultVal)
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
	result, err := p.Eval(code)
	if err == nil {
		t.Fatal("Expected error for uncaught sys.exit()")
	}

	// Check for SystemExit exception using the new API
	ex, ok := object.AsException(result)
	if !ok || !ex.IsSystemExit() {
		t.Fatalf("Expected SystemExit exception, got: %T (result=%v, err=%v)", result, result, err)
	}

	if ex.GetExitCode() != 99 {
		t.Errorf("Expected exit code 99, got: %d", ex.GetExitCode())
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
	// Note: SystemExit(0) returns (Exception, nil) since it's a "clean" exit
	code := `
import sys
sys.exit()
`
	result, err := p.Eval(code)

	// Check for SystemExit exception using the new API
	ex, ok := object.AsException(result)
	if !ok || !ex.IsSystemExit() {
		t.Fatalf("Expected SystemExit exception, got: %T (result=%v, err=%v)", result, result, err)
	}

	if ex.GetExitCode() != 0 {
		t.Errorf("Expected exit code 0 (default), got: %d", ex.GetExitCode())
	}

	// For exit code 0, err should be nil (clean exit)
	if err != nil {
		t.Errorf("Expected nil error for SystemExit(0), got: %v", err)
	}
}

func TestSysExitInFunction(t *testing.T) {
	p := New()
	extlibs.RegisterSysLibrary(p, []string{})

	// Test that sys.exit() called from a function returns SystemExit exception
	code := `
import sys

def my_function():
    sys.exit(123)

my_function()
`
	result, err := p.Eval(code)
	if err == nil {
		t.Fatal("Expected error for sys.exit() in function")
	}

	// Check for SystemExit exception using the new API
	ex, ok := object.AsException(result)
	if !ok || !ex.IsSystemExit() {
		t.Fatalf("Expected SystemExit exception, got: %T (result=%v, err=%v)", result, result, err)
	}

	if ex.GetExitCode() != 123 {
		t.Errorf("Expected exit code 123, got: %d", ex.GetExitCode())
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
	result, err := p.CallFunction("exit_func")
	if err == nil {
		t.Fatal("Expected error for sys.exit() via CallFunction")
	}

	// Check for SystemExit exception using the new API
	ex, ok := object.AsException(result)
	if !ok || !ex.IsSystemExit() {
		t.Fatalf("Expected SystemExit exception, got: %T (result=%v, err=%v)", result, result, err)
	}

	if ex.GetExitCode() != 45 {
		t.Errorf("Expected exit code 45, got: %d", ex.GetExitCode())
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
	result, err := p.Eval(code)
	if err == nil {
		t.Fatal("Expected error for uncaught sys.exit()")
	}

	// If we reach here, sys.exit() did NOT terminate the Go process
	// (which is the correct behavior now)

	// Check for SystemExit exception using the new API
	ex, ok := object.AsException(result)
	if !ok || !ex.IsSystemExit() {
		t.Fatalf("Expected SystemExit exception, got: %T (result=%v, err=%v)", result, result, err)
	}

	if ex.GetExitCode() != 1 {
		t.Errorf("Expected exit code 1, got: %d", ex.GetExitCode())
	}

	// Verify we can still use the interpreter
	_, err = p.Eval("x = 42")
	if err != nil {
		t.Errorf("Interpreter should still be usable after sys.exit(), got: %v", err)
	}
}

func TestSysExitPropagatesThroughNestedTryCatch(t *testing.T) {
	p := New()
	extlibs.RegisterSysLibrary(p, []string{})

	// Test that sys.exit() propagates through nested try/catch blocks
	// SystemExit is uncatchable, so except blocks don't execute
	code := `
import sys

exit_code = "not set"
finally_executed = []

try:
    # Outer try block
    try:
        # Inner try block - call sys.exit
        sys.exit(99)
    except Exception as e:
        # Inner except does NOT execute for SystemExit
        exit_code = "inner caught"
    finally:
        finally_executed.append("inner finally")

    # Should NOT reach here because SystemExit propagates
    exit_code = "reached after inner"
except Exception as e:
    # Outer except also does NOT execute for SystemExit
    exit_code = "outer caught"
finally:
    finally_executed.append("outer finally")

# Should NOT reach here
exit_code = "should not reach"
`
	result, err := p.Eval(code)

	// SystemExit should NOT be caught - should get error
	if err == nil {
		t.Fatal("Expected error for uncatchable SystemExit")
	}

	// Check for SystemExit exception
	ex, ok := object.AsException(result)
	if !ok || !ex.IsSystemExit() {
		t.Fatalf("Expected SystemExit exception, got: %T (result=%v, err=%v)", result, result, err)
	}

	if ex.GetExitCode() != 99 {
		t.Errorf("Expected exit code 99, got: %d", ex.GetExitCode())
	}

	// Verify that exit_code was never changed (except blocks didn't execute)
	exitCodeVal, objErr := p.GetVar("exit_code")
	if objErr != nil {
		t.Fatalf("Failed to get exit_code: %v", objErr)
	}
	if exitCodeVal != "not set" {
		t.Errorf("Expected exit_code to be 'not set' (except blocks didn't execute), got: %v", exitCodeVal)
	}

	// Verify that both finally blocks executed
	finallyExecuted, objErr := p.GetVar("finally_executed")
	if objErr != nil {
		t.Fatalf("Failed to get finally_executed: %v", objErr)
	}

	// GetVar returns Go values, so we need to check for []interface{}
	list, ok := finallyExecuted.([]interface{})
	if !ok {
		t.Fatalf("Expected finally_executed to be a []interface{}, got: %T", finallyExecuted)
	}

	if len(list) != 2 {
		t.Errorf("Expected 2 finally blocks to execute, got: %d", len(list))
	}
}

func TestSysExitInFunctionPropagates(t *testing.T) {
	p := New()
	extlibs.RegisterSysLibrary(p, []string{})

	// Test that sys.exit() called from a function propagates through all try/except blocks
	code := `
import sys

def my_function():
    try:
        sys.exit(42)
        return "should not return"
    except Exception as e:
        # This does NOT execute for SystemExit
        return "caught"

# Try to call the function and catch the SystemExit
exit_code = "not set"
try:
    result = my_function()
    exit_code = "no exception"
except Exception as e:
    # This also does NOT execute for SystemExit
    exit_code = "outer caught"

# Should NOT reach here
exit_code = "should not reach"
`
	result, err := p.Eval(code)

	// SystemExit should NOT be caught - should get error
	if err == nil {
		t.Fatal("Expected error for uncatchable SystemExit")
	}

	// Check for SystemExit exception
	ex, ok := object.AsException(result)
	if !ok || !ex.IsSystemExit() {
		t.Fatalf("Expected SystemExit exception, got: %T (result=%v, err=%v)", result, result, err)
	}

	if ex.GetExitCode() != 42 {
		t.Errorf("Expected exit code 42, got: %d", ex.GetExitCode())
	}

	// Verify that exit_code was never changed (except blocks didn't execute)
	exitCodeVal, objErr := p.GetVar("exit_code")
	if objErr != nil {
		t.Fatalf("Failed to get exit_code: %v", objErr)
	}
	if exitCodeVal != "not set" {
		t.Errorf("Expected exit_code to be 'not set' (except blocks didn't execute), got: %v", exitCodeVal)
	}
}

func TestSysExitCannotBeSuppressed(t *testing.T) {
	p := New()
	extlibs.RegisterSysLibrary(p, []string{})

	// Test that sys.exit() CANNOT be suppressed by except/pass blocks
	code := `
import sys

def inner_func():
    try:
        sys.exit(123)
    except:
        # This does NOT suppress SystemExit - it's uncatchable
        pass
    return "should not return"

# Call the function - sys.exit cannot be suppressed
result = inner_func()

# Should NOT reach here
exit_code = "should not reach"
`
	result, err := p.Eval(code)

	// SystemExit should NOT be suppressed - should get error
	if err == nil {
		t.Fatal("Expected error for uncatchable SystemExit")
	}

	// Check for SystemExit exception
	ex, ok := object.AsException(result)
	if !ok || !ex.IsSystemExit() {
		t.Fatalf("Expected SystemExit exception, got: %T (result=%v, err=%v)", result, result, err)
	}

	if ex.GetExitCode() != 123 {
		t.Errorf("Expected exit code 123, got: %d", ex.GetExitCode())
	}

	// Verify that exit_code was never set
	_, objErr := p.GetVar("exit_code")
	if objErr == nil {
		t.Error("Expected exit_code variable to not exist (SystemExit should have propagated)")
	}
}

func TestSysExitFinallyBlockExecutes(t *testing.T) {
	p := New()
	extlibs.RegisterSysLibrary(p, []string{})

	// Test that finally blocks execute even when sys.exit is called
	// BUT except blocks do NOT execute for SystemExit
	code := `
import sys

finally_executed = False
exception_caught = False

try:
    sys.exit(99)
except Exception as e:
    # This does NOT execute for SystemExit
    exception_caught = True
finally:
    # This DOES execute even for SystemExit
    finally_executed = True
`
	result, err := p.Eval(code)

	// SystemExit should NOT be caught - should get error
	if err == nil {
		t.Fatal("Expected error for uncatchable SystemExit")
	}

	// Check for SystemExit exception
	ex, ok := object.AsException(result)
	if !ok || !ex.IsSystemExit() {
		t.Fatalf("Expected SystemExit exception, got: %T (result=%v, err=%v)", result, result, err)
	}

	if ex.GetExitCode() != 99 {
		t.Errorf("Expected exit code 99, got: %d", ex.GetExitCode())
	}

	// Check that finally executed but except did NOT
	finallyExecuted, objErr := p.GetVarAsBool("finally_executed")
	if objErr != nil {
		t.Fatalf("Failed to get finally_executed: %v", objErr)
	}
	if !finallyExecuted {
		t.Error("Finally block should have executed")
	}

	exceptionCaught, objErr := p.GetVarAsBool("exception_caught")
	if objErr != nil {
		t.Fatalf("Failed to get exception_caught: %v", objErr)
	}
	if exceptionCaught {
		t.Error("Except block should NOT have executed for SystemExit")
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

func TestSysExitStopsScriptExecution(t *testing.T) {
	p := New()
	extlibs.RegisterSysLibrary(p, []string{})

	// Test that sys.exit(42) propagates up and stops the script completely
	// No code after sys.exit() should execute
	code := `
import sys

# Set some variables before sys.exit()
before_exit = "set"
try:
    # This is the key line - sys.exit(42)
    sys.exit(42)
    # Nothing below this line should execute
    before_exit = "should not reach here"
    after_exit = "also should not reach"
except Exception as e:
    # This does NOT execute for SystemExit
    before_exit = "except executed"

# This code also does NOT execute because SystemExit propagated
after_exit = "after try block"
final_var = "script continued"

# This line should never execute
assert False, "script should have stopped at sys.exit(42)"
`
	result, err := p.Eval(code)

	// sys.exit(42) should return a SystemExit exception
	if err == nil {
		t.Fatal("Expected error for uncaught sys.exit(42)")
	}

	// Check for SystemExit exception with exit code 42
	ex, ok := object.AsException(result)
	if !ok || !ex.IsSystemExit() {
		t.Fatalf("Expected SystemExit exception, got: %T (result=%v, err=%v)", result, result, err)
	}

	if ex.GetExitCode() != 42 {
		t.Errorf("Expected exit code 42, got: %d", ex.GetExitCode())
	}

	// Verify that only before_exit was set, and nothing after sys.exit()
	beforeExit, objErr := p.GetVar("before_exit")
	if objErr != nil {
		t.Fatalf("Failed to get before_exit: %v", objErr)
	}
	if beforeExit != "set" {
		t.Errorf("Expected before_exit to be 'set', got: %v", beforeExit)
	}

	// Verify that after_exit was never set
	_, objErr = p.GetVar("after_exit")
	if objErr == nil {
		t.Error("Expected after_exit variable to not exist (script stopped at sys.exit)")
	}

	// Verify that final_var was never set
	_, objErr = p.GetVar("final_var")
	if objErr == nil {
		t.Error("Expected final_var variable to not exist (script stopped at sys.exit)")
	}
}
