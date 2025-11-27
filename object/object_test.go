package object

import (
	"context"
	"testing"

	"github.com/paularlott/scriptling/ast"
)

func TestObjectTypes(t *testing.T) {
	tests := []struct {
		obj      Object
		expected ObjectType
	}{
		{&Integer{Value: 42}, INTEGER_OBJ},
		{&Float{Value: 3.14}, FLOAT_OBJ},
		{&Boolean{Value: true}, BOOLEAN_OBJ},
		{&String{Value: "hello"}, STRING_OBJ},
		{&Null{}, NULL_OBJ},
		{&ReturnValue{Value: &Integer{Value: 1}}, RETURN_OBJ},
		{&Break{}, BREAK_OBJ},
		{&Continue{}, CONTINUE_OBJ},
		{&Function{}, FUNCTION_OBJ},
		{&Builtin{}, BUILTIN_OBJ},
		{&List{}, LIST_OBJ},
		{&Dict{}, DICT_OBJ},
		{&HttpResponse{}, HTTP_RESP_OBJ},
		{&Error{Message: "test"}, ERROR_OBJ},
		{&Exception{Message: "test"}, EXCEPTION_OBJ},
	}

	for _, tt := range tests {
		if tt.obj.Type() != tt.expected {
			t.Errorf("obj.Type() = %q, want %q", tt.obj.Type(), tt.expected)
		}
	}
}

func TestObjectInspect(t *testing.T) {
	tests := []struct {
		obj      Object
		expected string
	}{
		{&Integer{Value: 42}, "42"},
		{&Float{Value: 3.14}, "3.14"},
		{&Boolean{Value: true}, "true"},
		{&Boolean{Value: false}, "false"},
		{&String{Value: "hello"}, "hello"},
		{&Null{}, "None"},
		{&Break{}, "break"},
		{&Continue{}, "continue"},
		{&Function{}, "<function>"},
		{&Builtin{}, "<builtin function>"},
		{&Error{Message: "test error"}, "ERROR: test error"},
		{&Exception{Message: "test exception"}, "EXCEPTION: test exception"},
	}

	for _, tt := range tests {
		if tt.obj.Inspect() != tt.expected {
			t.Errorf("obj.Inspect() = %q, want %q", tt.obj.Inspect(), tt.expected)
		}
	}
}

func TestListInspect(t *testing.T) {
	list := &List{
		Elements: []Object{
			&Integer{Value: 1},
			&String{Value: "hello"},
			&Boolean{Value: true},
		},
	}
	expected := "[1, hello, true]"
	if list.Inspect() != expected {
		t.Errorf("list.Inspect() = %q, want %q", list.Inspect(), expected)
	}
}

func TestDictInspect(t *testing.T) {
	dict := &Dict{
		Pairs: map[string]DictPair{
			"name": {Key: &String{Value: "name"}, Value: &String{Value: "Alice"}},
			"age":  {Key: &String{Value: "age"}, Value: &Integer{Value: 30}},
		},
	}
	result := dict.Inspect()
	// Dict order is not guaranteed, so check both possibilities
	if result != "{name: Alice, age: 30}" && result != "{age: 30, name: Alice}" {
		t.Errorf("dict.Inspect() = %q, want either order", result)
	}
}

func TestEnvironment(t *testing.T) {
	env := NewEnvironment()

	// Test Set and Get
	val := &Integer{Value: 42}
	env.Set("x", val)

	result, ok := env.Get("x")
	if !ok {
		t.Fatal("expected to find variable x")
	}
	if result != val {
		t.Errorf("got %v, want %v", result, val)
	}
}

func TestEnclosedEnvironment(t *testing.T) {
	outer := NewEnvironment()
	outer.Set("x", &Integer{Value: 10})

	inner := NewEnclosedEnvironment(outer)
	inner.Set("y", &Integer{Value: 20})

	// Inner should see outer variables
	x, ok := inner.Get("x")
	if !ok {
		t.Fatal("expected to find variable x from outer scope")
	}
	if x.(*Integer).Value != 10 {
		t.Errorf("x = %d, want 10", x.(*Integer).Value)
	}

	// Inner should see its own variables
	y, ok := inner.Get("y")
	if !ok {
		t.Fatal("expected to find variable y")
	}
	if y.(*Integer).Value != 20 {
		t.Errorf("y = %d, want 20", y.(*Integer).Value)
	}

	// Outer should not see inner variables
	_, ok = outer.Get("y")
	if ok {
		t.Error("outer scope should not see inner variable y")
	}
}

func TestGlobalVariables(t *testing.T) {
	outer := NewEnvironment()
	inner := NewEnclosedEnvironment(outer)

	// Mark variable as global in inner scope
	inner.MarkGlobal("global_var")

	// Set global variable from inner scope
	inner.Set("global_var", &Integer{Value: 42})

	// Should be set in outer (global) scope
	result, ok := outer.Get("global_var")
	if !ok {
		t.Fatal("expected global variable to be set in outer scope")
	}
	if result.(*Integer).Value != 42 {
		t.Errorf("global_var = %d, want 42", result.(*Integer).Value)
	}

	// Check IsGlobal
	if !inner.IsGlobal("global_var") {
		t.Error("expected global_var to be marked as global")
	}
}

func TestNonlocalVariables(t *testing.T) {
	outer := NewEnvironment()
	outer.Set("nonlocal_var", &Integer{Value: 10})

	inner := NewEnclosedEnvironment(outer)
	inner.MarkNonlocal("nonlocal_var")

	// Modify nonlocal variable from inner scope
	inner.Set("nonlocal_var", &Integer{Value: 20})

	// Should be modified in outer scope
	result, ok := outer.Get("nonlocal_var")
	if !ok {
		t.Fatal("expected nonlocal variable to exist in outer scope")
	}
	if result.(*Integer).Value != 20 {
		t.Errorf("nonlocal_var = %d, want 20", result.(*Integer).Value)
	}

	// Check IsNonlocal
	if !inner.IsNonlocal("nonlocal_var") {
		t.Error("expected nonlocal_var to be marked as nonlocal")
	}
}

func TestReturnValue(t *testing.T) {
	val := &Integer{Value: 42}
	ret := &ReturnValue{Value: val}

	if ret.Type() != RETURN_OBJ {
		t.Errorf("ret.Type() = %q, want %q", ret.Type(), RETURN_OBJ)
	}
	if ret.Inspect() != "42" {
		t.Errorf("ret.Inspect() = %q, want %q", ret.Inspect(), "42")
	}
}

func TestHttpResponse(t *testing.T) {
	resp := &HttpResponse{
		StatusCode: 200,
		Body:       "OK",
		Headers:    map[string]string{"Content-Type": "text/plain"},
	}

	if resp.Type() != HTTP_RESP_OBJ {
		t.Errorf("resp.Type() = %q, want %q", resp.Type(), HTTP_RESP_OBJ)
	}
	if resp.Inspect() != "OK" {
		t.Errorf("resp.Inspect() = %q, want %q", resp.Inspect(), "OK")
	}
}

func TestFunction(t *testing.T) {
	// Create a simple function object
	params := []*ast.Identifier{
		{Value: "x"},
		{Value: "y"},
	}
	body := &ast.BlockStatement{}
	env := NewEnvironment()

	fn := &Function{
		Name:       "test_function",
		Parameters: params,
		Body:       body,
		Env:        env,
	}

	if fn.Type() != FUNCTION_OBJ {
		t.Errorf("fn.Type() = %q, want %q", fn.Type(), FUNCTION_OBJ)
	}
	if fn.Inspect() != "<function>" {
		t.Errorf("fn.Inspect() = %q, want %q", fn.Inspect(), "<function>")
	}
}

func TestBuiltinFunction(t *testing.T) {
	builtin := &Builtin{
		Fn: func(ctx context.Context, args ...Object) Object {
			return &Integer{Value: 42}
		},
	}

	if builtin.Type() != BUILTIN_OBJ {
		t.Errorf("builtin.Type() = %q, want %q", builtin.Type(), BUILTIN_OBJ)
	}
	if builtin.Inspect() != "<builtin function>" {
		t.Errorf("builtin.Inspect() = %q, want %q", builtin.Inspect(), "<builtin function>")
	}

	// Test function call
	result := builtin.Fn(context.Background())
	if result.(*Integer).Value != 42 {
		t.Errorf("builtin function result = %d, want 42", result.(*Integer).Value)
	}
}
