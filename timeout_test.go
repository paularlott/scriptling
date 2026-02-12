package scriptling

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/paularlott/scriptling/evaluator"
	"github.com/paularlott/scriptling/object"
)

// TestEvalWithTimeout_InfiniteLoop tests that an infinite while loop gets interrupted
func TestEvalWithTimeout_InfiniteLoop(t *testing.T) {
	p := New()

	// Script with infinite loop
	script := `
x = 0
while True:
	x = x + 1
`

	start := time.Now()
	result, err := p.EvalWithTimeout(100*time.Millisecond, script)
	elapsed := time.Since(start)

	// Should return an error (wrapped in the result)
	if err == nil {
		if result == nil || !object.IsError(result) {
			t.Errorf("expected timeout error, got nil error and result: %v", result)
		}
	}

	// Verify it's a timeout error
	if result != nil && object.IsError(result) {
		errObj := result.(*object.Error)
		if !strings.Contains(errObj.Message, "timeout") {
			t.Errorf("expected timeout error message, got: %s", errObj.Message)
		}
	}

	// Verify it actually timed out (should be close to 100ms, not run forever)
	if elapsed > 500*time.Millisecond {
		t.Errorf("script took %v to timeout, expected around 100ms", elapsed)
	}

	// Verify it didn't complete instantly (the loop did run)
	if elapsed < 50*time.Millisecond {
		t.Logf("warning: script completed very quickly (%v), loop may not have run", elapsed)
	}
}

// TestEvalWithTimeout_ForLoop tests that a long-running for loop gets interrupted
func TestEvalWithTimeout_ForLoop(t *testing.T) {
	p := New()

	// Script with a loop that would take a very long time
	script := `
x = 0
for i in range(100000000):
	x = x + 1
`

	start := time.Now()
	result, err := p.EvalWithTimeout(100*time.Millisecond, script)
	elapsed := time.Since(start)

	// Should return an error
	if err == nil && (result == nil || !object.IsError(result)) {
		t.Errorf("expected timeout error, got: %v", result)
	}

	// Verify it's a timeout error
	if result != nil && object.IsError(result) {
		errObj := result.(*object.Error)
		if !strings.Contains(errObj.Message, "timeout") {
			t.Errorf("expected timeout error message, got: %s", errObj.Message)
		}
	}

	// Verify timing
	if elapsed > 500*time.Millisecond {
		t.Errorf("script took %v to timeout, expected around 100ms", elapsed)
	}
}

// TestEvalWithTimeout_CompletesBeforeTimeout tests that scripts completing before timeout work normally
func TestEvalWithTimeout_CompletesBeforeTimeout(t *testing.T) {
	p := New()

	// Simple script that completes quickly
	script := `
result = 0
for i in range(100):
	result = result + i
`

	result, err := p.EvalWithTimeout(1*time.Second, script)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the script completed successfully
	val, objErr := p.GetVar("result")
	if objErr != nil {
		t.Fatalf("failed to get result: %v", objErr)
	}

	// Sum of 0 to 99 = 4950
	if val != int64(4950) {
		t.Errorf("expected result=4950, got %v", val)
	}

	if object.IsError(result) {
		t.Errorf("unexpected error result: %v", result)
	}
}

// TestEvalWithContext_Cancellation tests that context cancellation works
func TestEvalWithContext_Cancellation(t *testing.T) {
	p := New()

	// Script with infinite loop
	script := `
x = 0
while True:
	x = x + 1
`

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after 100ms
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	result, err := p.EvalWithContext(ctx, script)
	elapsed := time.Since(start)

	// Should return an error (either from result or error return)
	if err == nil && (result == nil || !object.IsError(result)) {
		t.Errorf("expected cancellation error, got: %v", result)
	}

	// Verify it's either a timeout or cancelled error
	if result != nil && object.IsError(result) {
		errObj := result.(*object.Error)
		if !strings.Contains(errObj.Message, "cancelled") && !strings.Contains(errObj.Message, "timeout") {
			t.Errorf("expected cancelled or timeout error message, got: %s", errObj.Message)
		}
	}

	// Verify timing
	if elapsed > 500*time.Millisecond {
		t.Errorf("script took %v to cancel, expected around 100ms", elapsed)
	}
}

// TestEvalWithTimeout_NestedLoops tests timeout with nested loops
func TestEvalWithTimeout_NestedLoops(t *testing.T) {
	p := New()

	// Script with nested infinite loops
	script := `
x = 0
while True:
	y = 0
	while True:
		y = y + 1
		x = x + 1
`

	start := time.Now()
	result, err := p.EvalWithTimeout(100*time.Millisecond, script)
	elapsed := time.Since(start)

	// Should return an error
	if err == nil && (result == nil || !object.IsError(result)) {
		t.Errorf("expected timeout error, got: %v", result)
	}

	// Verify it's a timeout error
	if result != nil && object.IsError(result) {
		errObj := result.(*object.Error)
		if !strings.Contains(errObj.Message, "timeout") {
			t.Errorf("expected timeout error message, got: %s", errObj.Message)
		}
	}

	// Verify timing
	if elapsed > 500*time.Millisecond {
		t.Errorf("script took %v to timeout, expected around 100ms", elapsed)
	}
}

// TestEvalWithTimeout_FunctionWithInfiniteLoop tests timeout in a function
func TestEvalWithTimeout_FunctionWithInfiniteLoop(t *testing.T) {
	p := New()

	// Script that defines a function with an infinite loop
	script := `
def infinite_loop():
	x = 0
	while True:
		x = x + 1
	return x

result = infinite_loop()
`

	start := time.Now()
	result, err := p.EvalWithTimeout(100*time.Millisecond, script)
	elapsed := time.Since(start)

	// Should return an error
	if err == nil && (result == nil || !object.IsError(result)) {
		t.Errorf("expected timeout error, got: %v", result)
	}

	// Verify it's a timeout error
	if result != nil && object.IsError(result) {
		errObj := result.(*object.Error)
		if !strings.Contains(errObj.Message, "timeout") {
			t.Errorf("expected timeout error message, got: %s", errObj.Message)
		}
	}

	// Verify timing
	if elapsed > 500*time.Millisecond {
		t.Errorf("script took %v to timeout, expected around 100ms", elapsed)
	}
}

// TestEvalWithTimeout_RecursiveFunction tests timeout with deep recursion
// This test verifies that deep recursion is safely caught and doesn't crash the application.
// With call depth tracking, recursion is limited and returns an error instead of crashing.
func TestEvalWithTimeout_RecursiveFunction(t *testing.T) {
	p := New()

	// Script with recursive function that would recurse forever
	script := `
def recurse(x):
	return recurse(x + 1)

result = recurse(0)
`

	start := time.Now()
	result, err := p.EvalWithTimeout(100*time.Millisecond, script)
	elapsed := time.Since(start)

	// Should return an error (call depth exceeded)
	if err == nil && (result == nil || !object.IsError(result)) {
		t.Errorf("expected error, got: %v, err: %v", result, err)
	}

	// Verify it's a call depth exceeded error
	if result != nil && object.IsError(result) {
		errObj := result.(*object.Error)
		if !strings.Contains(errObj.Message, "call depth exceeded") {
			t.Errorf("expected call depth exceeded error, got: %s", errObj.Message)
		}
	}

	// Verify timing - should be fast (not crash or hang)
	if elapsed > 1*time.Second {
		t.Errorf("script took %v, expected faster termination", elapsed)
	}
}

// TestEvalWithTimeout_AlreadyExpiredContext tests behavior with already expired context
func TestEvalWithTimeout_AlreadyExpiredContext(t *testing.T) {
	p := New()

	// Create an already-expired context
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// Wait for context to expire
	time.Sleep(10 * time.Millisecond)

	script := `x = 1 + 1`
	result, err := p.EvalWithContext(ctx, script)

	// Should return a timeout/cancellation error immediately
	if err == nil && (result == nil || !object.IsError(result)) {
		t.Errorf("expected timeout/cancel error for expired context, got: %v", result)
	}
}

// TestEvalWithTimeout_ZeroTimeout tests behavior with zero timeout
func TestEvalWithTimeout_ZeroTimeout(t *testing.T) {
	p := New()

	script := `x = 1 + 1`
	result, err := p.EvalWithTimeout(0*time.Millisecond, script)

	// Zero timeout means immediate timeout, but the context check happens at start
	// The script might still execute before the check, so we just verify no panic
	_ = result
	_ = err
}

// TestEvalWithContext_CustomCallDepth tests that custom call depth is respected
func TestEvalWithContext_CustomCallDepth(t *testing.T) {
	p := New()

	// Script with recursive function
	script := `
def recurse(x):
	return recurse(x + 1)

result = recurse(0)
`

	// Test 1: Custom lower limit (50) should trigger earlier
	t.Run("custom_lower_limit", func(t *testing.T) {
		ctx := evaluator.ContextWithCallDepth(context.Background(), 50)
		result, err := p.EvalWithContext(ctx, script)

		// Should return an error
		if err == nil && (result == nil || !object.IsError(result)) {
			t.Errorf("expected error, got: %v, err: %v", result, err)
		}

		// Verify it's a call depth exceeded error with our custom limit
		if result != nil && object.IsError(result) {
			errObj := result.(*object.Error)
			if !strings.Contains(errObj.Message, "call depth exceeded") {
				t.Errorf("expected call depth exceeded error, got: %s", errObj.Message)
			}
			if !strings.Contains(errObj.Message, "50") {
				t.Errorf("expected max depth 50 in error, got: %s", errObj.Message)
			}
		}
	})

	// Test 2: Default limit should still work when no custom context
	t.Run("default_limit", func(t *testing.T) {
		result, err := p.EvalWithContext(context.Background(), script)

		// Should return an error
		if err == nil && (result == nil || !object.IsError(result)) {
			t.Errorf("expected error, got: %v, err: %v", result, err)
		}

		// Verify it uses the default limit (1000)
		if result != nil && object.IsError(result) {
			errObj := result.(*object.Error)
			if !strings.Contains(errObj.Message, "1000") {
				t.Errorf("expected default max depth 1000 in error, got: %s", errObj.Message)
			}
		}
	})

	// Test 3: Custom higher limit (2000) should allow more recursion
	t.Run("custom_higher_limit", func(t *testing.T) {
		ctx := evaluator.ContextWithCallDepth(context.Background(), 2000)
		result, err := p.EvalWithContext(ctx, script)

		// Should return an error
		if err == nil && (result == nil || !object.IsError(result)) {
			t.Errorf("expected error, got: %v, err: %v", result, err)
		}

		// Verify it uses our custom limit (2000)
		if result != nil && object.IsError(result) {
			errObj := result.(*object.Error)
			if !strings.Contains(errObj.Message, "2000") {
				t.Errorf("expected custom max depth 2000 in error, got: %s", errObj.Message)
			}
		}
	})
}
