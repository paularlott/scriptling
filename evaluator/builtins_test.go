package evaluator

import (
	"context"
	"testing"

	"github.com/paularlott/scriptling/object"
)

func TestBuiltinLen(t *testing.T) {
	tests := []struct {
		input    object.Object
		expected int64
	}{
		{&object.String{Value: "hello"}, 5},
		{&object.String{Value: ""}, 0},
		{&object.List{Elements: []object.Object{&object.Integer{Value: 1}, &object.Integer{Value: 2}}}, 2},
		{&object.List{Elements: []object.Object{}}, 0},
	}

	for _, tt := range tests {
		result := builtins["len"].Fn(context.Background(), nil, tt.input)
		integer, ok := result.(*object.Integer)
		if !ok {
			t.Errorf("object is not Integer. got=%T (%+v)", result, result)
			continue
		}
		if integer.Value != tt.expected {
			t.Errorf("wrong value. got=%d, want=%d", integer.Value, tt.expected)
		}
	}
}

func TestBuiltinLenError(t *testing.T) {
	result := builtins["len"].Fn(context.Background(), nil, &object.Integer{Value: 1})
	if result.Type() != object.ERROR_OBJ {
		t.Errorf("expected error for len(1), got %T", result)
	}
}

func TestBuiltinStr(t *testing.T) {
	tests := []struct {
		input    object.Object
		expected string
	}{
		{&object.Integer{Value: 42}, "42"},
		{&object.Float{Value: 3.14}, "3.14"},
		{&object.String{Value: "hello"}, "hello"},
		{&object.Boolean{Value: true}, "true"},
	}

	for _, tt := range tests {
		result := builtins["str"].Fn(context.Background(), nil, tt.input)
		str, ok := result.(*object.String)
		if !ok {
			t.Errorf("object is not String. got=%T (%+v)", result, result)
			continue
		}
		if str.Value != tt.expected {
			t.Errorf("wrong value. got=%q, want=%q", str.Value, tt.expected)
		}
	}
}

func TestBuiltinInt(t *testing.T) {
	tests := []struct {
		input    object.Object
		expected int64
	}{
		{&object.Integer{Value: 42}, 42},
		{&object.Float{Value: 3.14}, 3},
		{&object.String{Value: "123"}, 123},
	}

	for _, tt := range tests {
		result := builtins["int"].Fn(context.Background(), nil, tt.input)
		integer, ok := result.(*object.Integer)
		if !ok {
			t.Errorf("object is not Integer. got=%T (%+v)", result, result)
			continue
		}
		if integer.Value != tt.expected {
			t.Errorf("wrong value. got=%d, want=%d", integer.Value, tt.expected)
		}
	}
}

func TestBuiltinFloat(t *testing.T) {
	tests := []struct {
		input    object.Object
		expected float64
	}{
		{&object.Float{Value: 3.14}, 3.14},
		{&object.Integer{Value: 42}, 42.0},
		{&object.String{Value: "3.14"}, 3.14},
	}

	for _, tt := range tests {
		result := builtins["float"].Fn(context.Background(), nil, tt.input)
		float, ok := result.(*object.Float)
		if !ok {
			t.Errorf("object is not Float. got=%T (%+v)", result, result)
			continue
		}
		if float.Value != tt.expected {
			t.Errorf("wrong value. got=%f, want=%f", float.Value, tt.expected)
		}
	}
}

func TestBuiltinAppend(t *testing.T) {
	list := &object.List{Elements: []object.Object{&object.Integer{Value: 1}}}
	newElement := &object.Integer{Value: 2}

	result := builtins["append"].Fn(context.Background(), nil, list, newElement)

	if result.Type() != object.NULL_OBJ {
		t.Errorf("append should return NULL, got %T", result)
	}

	if len(list.Elements) != 2 {
		t.Errorf("list should have 2 elements, got %d", len(list.Elements))
	}

	if list.Elements[1].(*object.Integer).Value != 2 {
		t.Errorf("second element should be 2, got %d", list.Elements[1].(*object.Integer).Value)
	}
}

func TestBuiltinType(t *testing.T) {
	tests := []struct {
		input    object.Object
		expected string
	}{
		{&object.Integer{Value: 42}, "INTEGER"},
		{&object.Float{Value: 3.14}, "FLOAT"},
		{&object.String{Value: "hello"}, "STRING"},
		{&object.Boolean{Value: true}, "BOOLEAN"},
		{&object.List{Elements: []object.Object{}}, "LIST"},
		{&object.Dict{Pairs: make(map[string]object.DictPair)}, "DICT"},
	}

	for _, tt := range tests {
		result := builtins["type"].Fn(context.Background(), nil, tt.input)
		str, ok := result.(*object.String)
		if !ok {
			t.Errorf("object is not String. got=%T (%+v)", result, result)
			continue
		}
		if str.Value != tt.expected {
			t.Errorf("wrong value. got=%q, want=%q", str.Value, tt.expected)
		}
	}
}
