package scriptling

import (
	"context"
	"testing"

	"github.com/paularlott/scriptling/object"
)

func TestBasicArithmetic(t *testing.T) {
	p := New()
	result, err := p.Eval("5 + 3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Inspect() != "8" {
		t.Errorf("expected 8, got %s", result.Inspect())
	}
}

func TestVariables(t *testing.T) {
	p := New()
	p.SetVar("x", 10)
	_, err := p.Eval("y = x * 2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	val, ok := p.GetVar("y")
	if !ok {
		t.Fatal("variable y not found")
	}
	if val != int64(20) {
		t.Errorf("expected 20, got %v", val)
	}
}

func TestFunctions(t *testing.T) {
	p := New()
	_, err := p.Eval(`
def add(a, b):
    return a + b

result = add(5, 3)
`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	val, ok := p.GetVar("result")
	if !ok {
		t.Fatal("variable result not found")
	}
	if val != int64(8) {
		t.Errorf("expected 8, got %v", val)
	}
}

func TestGoFunctionRegistration(t *testing.T) {
	p := New()

	// Register a custom Go function
	p.RegisterFunc("multiply", func(ctx context.Context, args ...object.Object) object.Object {
		if len(args) != 2 {
			return &object.Error{Message: "multiply requires 2 arguments"}
		}

		var a, b int64
		if intA, ok := args[0].(*object.Integer); ok {
			a = intA.Value
		} else {
			return &object.Error{Message: "first argument must be an integer"}
		}

		if intB, ok := args[1].(*object.Integer); ok {
			b = intB.Value
		} else {
			return &object.Error{Message: "second argument must be an integer"}
		}

		return &object.Integer{Value: a * b}
	})

	_, err := p.Eval("result = multiply(6, 7)")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, ok := p.GetVar("result")
	if !ok || result != int64(42) {
		t.Errorf("expected 42, got %v", result)
	}
}

func TestConditionals(t *testing.T) {
	p := New()
	_, err := p.Eval(`
x = 10
if x > 5:
    result = "large"
else:
    result = "small"
`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	val, ok := p.GetVar("result")
	if !ok {
		t.Fatal("variable result not found")
	}
	if val != "large" {
		t.Errorf("expected 'large', got %v", val)
	}
}

func TestWhileLoop(t *testing.T) {
	p := New()
	_, err := p.Eval(`
counter = 0
sum = 0
while counter < 5:
    sum = sum + counter
    counter = counter + 1
`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	val, ok := p.GetVar("sum")
	if !ok {
		t.Fatal("variable sum not found")
	}
	if val != int64(10) {
		t.Errorf("expected 10, got %v", val)
	}
}
