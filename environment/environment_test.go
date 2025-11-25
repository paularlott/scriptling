package environment

import (
	"testing"
	"github.com/paularlott/scriptling/object"
)

func TestNewEnvironment(t *testing.T) {
	env := object.NewEnvironment()
	if env == nil {
		t.Fatal("NewEnvironment() returned nil")
	}
}

func TestSetAndGet(t *testing.T) {
	env := object.NewEnvironment()
	val := &object.Integer{Value: 42}
	
	env.Set("test", val)
	
	result, ok := env.Get("test")
	if !ok {
		t.Fatal("Get() returned false for existing key")
	}
	
	if result != val {
		t.Errorf("Get() returned %v, want %v", result, val)
	}
}

func TestGetNonExistent(t *testing.T) {
	env := object.NewEnvironment()
	
	_, ok := env.Get("nonexistent")
	if ok {
		t.Error("Get() returned true for non-existent key")
	}
}

func TestEnclosedEnvironment(t *testing.T) {
	outer := object.NewEnvironment()
	outer.Set("outer", &object.String{Value: "outer_value"})
	
	inner := object.NewEnclosedEnvironment(outer)
	inner.Set("inner", &object.String{Value: "inner_value"})
	
	// Inner should see outer variables
	result, ok := inner.Get("outer")
	if !ok {
		t.Fatal("Inner environment should see outer variables")
	}
	if result.(*object.String).Value != "outer_value" {
		t.Errorf("Got %q, want %q", result.(*object.String).Value, "outer_value")
	}
	
	// Inner should see its own variables
	result, ok = inner.Get("inner")
	if !ok {
		t.Fatal("Inner environment should see its own variables")
	}
	if result.(*object.String).Value != "inner_value" {
		t.Errorf("Got %q, want %q", result.(*object.String).Value, "inner_value")
	}
	
	// Outer should not see inner variables
	_, ok = outer.Get("inner")
	if ok {
		t.Error("Outer environment should not see inner variables")
	}
}