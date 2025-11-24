package scriptling

import (
	"testing"
	"os"
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

	result, _ := p.GetVar("result")
	if result != int64(1) {
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

	cleanup, _ := p.GetVar("cleanup")
	if cleanup != int64(1) {
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

	result, _ := p.GetVar("result")
	cleanup, _ := p.GetVar("cleanup")
	
	if result != int64(1) {
		t.Errorf("result = %v, want 1", result)
	}
	if cleanup != int64(1) {
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

	caught, _ := p.GetVar("caught")
	if caught != int64(1) {
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

	result, _ := p.GetVar("result")
	if result != int64(-1) {
		t.Errorf("result = %v, want -1", result)
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
