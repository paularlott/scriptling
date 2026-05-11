package evaluator

import (
	"context"
	"testing"

	"github.com/paularlott/scriptling/object"
)

// Test objectsEqual function
func TestObjectsEqual(t *testing.T) {
	tests := []struct {
		name string
		a    object.Object
		b    object.Object
		want bool
	}{
		{
			name: "equal integers",
			a:    object.NewInteger(42),
			b:    object.NewInteger(42),
			want: true,
		},
		{
			name: "unequal integers",
			a:    object.NewInteger(42),
			b:    object.NewInteger(43),
			want: false,
		},
		{
			name: "equal floats",
			a:    object.NewFloat(3.14),
			b:    object.NewFloat(3.14),
			want: true,
		},
		{
			name: "unequal floats",
			a:    object.NewFloat(3.14),
			b:    object.NewFloat(2.71),
			want: false,
		},
		{
			name: "equal strings",
			a:    object.NewString("hello"),
			b:    object.NewString("hello"),
			want: true,
		},
		{
			name: "unequal strings",
			a:    object.NewString("hello"),
			b:    object.NewString("world"),
			want: false,
		},
		{
			name: "equal booleans",
			a:    object.NewBoolean(true),
			b:    object.NewBoolean(true),
			want: true,
		},
		{
			name: "unequal booleans",
			a:    object.NewBoolean(true),
			b:    object.NewBoolean(false),
			want: false,
		},
		{
			name: "equal nulls",
			a:    &object.Null{},
			b:    &object.Null{},
			want: true,
		},
		{
			name: "different types",
			a:    object.NewInteger(42),
			b:    object.NewString("42"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := objectsEqual(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("objectsEqual() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Test objectsDeepEqual function
func TestObjectsDeepEqual(t *testing.T) {
	tests := []struct {
		name string
		a    object.Object
		b    object.Object
		want bool
	}{
		{
			name: "equal integers",
			a:    object.NewInteger(42),
			b:    object.NewInteger(42),
			want: true,
		},
		{
			name: "equal floats",
			a:    object.NewFloat(3.14),
			b:    object.NewFloat(3.14),
			want: true,
		},
		{
			name: "equal strings",
			a:    object.NewString("hello"),
			b:    object.NewString("hello"),
			want: true,
		},
		{
			name: "equal lists",
			a:    &object.List{Elements: []object.Object{object.NewInteger(1), object.NewInteger(2)}},
			b:    &object.List{Elements: []object.Object{object.NewInteger(1), object.NewInteger(2)}},
			want: true,
		},
		{
			name: "unequal lists different length",
			a:    &object.List{Elements: []object.Object{object.NewInteger(1)}},
			b:    &object.List{Elements: []object.Object{object.NewInteger(1), object.NewInteger(2)}},
			want: false,
		},
		{
			name: "unequal lists different elements",
			a:    &object.List{Elements: []object.Object{object.NewInteger(1), object.NewInteger(2)}},
			b:    &object.List{Elements: []object.Object{object.NewInteger(1), object.NewInteger(3)}},
			want: false,
		},
		{
			name: "nested lists equal",
			a:    &object.List{Elements: []object.Object{&object.List{Elements: []object.Object{object.NewInteger(1)}}}},
			b:    &object.List{Elements: []object.Object{&object.List{Elements: []object.Object{object.NewInteger(1)}}}},
			want: true,
		},
		{
			name: "equal tuples",
			a:    &object.Tuple{Elements: []object.Object{object.NewInteger(1), object.NewString("a")}},
			b:    &object.Tuple{Elements: []object.Object{object.NewInteger(1), object.NewString("a")}},
			want: true,
		},
		{
			name: "equal empty dicts",
			a:    &object.Dict{Pairs: map[string]object.DictPair{}},
			b:    &object.Dict{Pairs: map[string]object.DictPair{}},
			want: true,
		},
		{
			name: "equal dicts",
			a:    object.NewStringDict(map[string]object.Object{
				"a": object.NewInteger(1),
			}),
			b:    object.NewStringDict(map[string]object.Object{
				"a": object.NewInteger(1),
			}),
			want: true,
		},
		{
			name: "unequal dicts different keys",
			a:    object.NewStringDict(map[string]object.Object{
				"a": object.NewInteger(1),
			}),
			b:    object.NewStringDict(map[string]object.Object{
				"b": object.NewInteger(2),
			}),
			want: false,
		},
		{
			name: "different types",
			a:    object.NewInteger(42),
			b:    object.NewString("42"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := objectsDeepEqual(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("objectsDeepEqual() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Test SetEnvInContext and GetEnvFromContext
func TestEnvContext(t *testing.T) {
	ctx := context.Background()
	env := object.NewEnvironment()

	// Test SetEnvInContext
	ctxWithEnv := SetEnvInContext(ctx, env)
	if ctxWithEnv == ctx {
		t.Error("SetEnvInContext should return a new context")
	}

	// Test GetEnvFromContext with env set
	retrievedEnv := GetEnvFromContext(ctxWithEnv)
	if retrievedEnv != env {
		t.Error("GetEnvFromContext should return the same environment that was set")
	}

	// Test GetEnvFromContext without env set should return a new environment
	newEnv := GetEnvFromContext(ctx)
	if newEnv == nil {
		t.Error("GetEnvFromContext should return a new environment when none is set")
	}
}

// Test contextChecker
func TestContextChecker(t *testing.T) {
	ctx := context.Background()
	checker := newContextChecker(ctx)

	if checker.ctx != ctx {
		t.Error("contextChecker.ctx is not the same as the passed context")
	}

	if checker.batchSize != 10 {
		t.Errorf("contextChecker.batchSize = %d, want 10", checker.batchSize)
	}

	// Test check method - should return nil when context is not done
	result := checker.check()
	if result != nil {
		t.Errorf("contextChecker.check() = %v, want nil", result)
	}

	// Test checkAlways method - should return nil when context is not done
	result = checker.checkAlways()
	if result != nil {
		t.Errorf("contextChecker.checkAlways() = %v, want nil", result)
	}
}

// Test checkContext
func TestCheckContext(t *testing.T) {
	ctx := context.Background()

	// Test with background context (not done)
	result := checkContext(ctx)
	if result != nil {
		t.Errorf("checkContext() = %v, want nil", result)
	}
}

// Test nativeBoolToBooleanObject
func TestNativeBoolToBooleanObject(t *testing.T) {
	tests := []struct {
		name  string
		input bool
		want  object.Object
	}{
		{
			name:  "true",
			input: true,
			want:  TRUE,
		},
		{
			name:  "false",
			input: false,
			want:  FALSE,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nativeBoolToBooleanObject(tt.input)
			if got != tt.want {
				t.Errorf("nativeBoolToBooleanObject() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Test evalIndexExpression helper functions
func TestEvalListIndexExpression(t *testing.T) {
	list := &object.List{Elements: []object.Object{
		object.NewInteger(1),
		object.NewInteger(2),
		object.NewInteger(3),
	}}

	result := evalListIndexExpression(list, object.NewInteger(1))
	if result.Type() != object.INTEGER_OBJ {
		t.Errorf("expected INTEGER_OBJ, got %v", result.Type())
	}
	if result.(*object.Integer).IntValue() != 2 {
		t.Errorf("expected 2, got %d", result.(*object.Integer).IntValue())
	}

	result = evalListIndexExpression(list, object.NewInteger(-1))
	if result.Type() != object.INTEGER_OBJ {
		t.Errorf("expected INTEGER_OBJ, got %v", result.Type())
	}
	if result.(*object.Integer).IntValue() != 3 {
		t.Errorf("expected 3, got %d", result.(*object.Integer).IntValue())
	}

	result = evalListIndexExpression(list, object.NewInteger(10))
	if result.Type() != object.NULL_OBJ {
		t.Errorf("expected NULL_OBJ for out of bounds, got %v", result.Type())
	}

	result = evalListIndexExpression(list, object.NewInteger(-10))
	if result.Type() != object.NULL_OBJ {
		t.Errorf("expected NULL_OBJ for out of bounds, got %v", result.Type())
	}
}

// Test evalStringIndexExpression
func TestEvalStringIndexExpression(t *testing.T) {
	str := object.NewString("hello")

	result := evalStringIndexExpression(str, object.NewInteger(1))
	if result.Type() != object.STRING_OBJ {
		t.Errorf("expected STRING_OBJ, got %v", result.Type())
	}
	if result.(*object.String).StringValue() != "e" {
		t.Errorf("expected 'e', got %q", result.(*object.String).StringValue())
	}

	result = evalStringIndexExpression(str, object.NewInteger(-1))
	if result.Type() != object.STRING_OBJ {
		t.Errorf("expected STRING_OBJ, got %v", result.Type())
	}
	if result.(*object.String).StringValue() != "o" {
		t.Errorf("expected 'o', got %q", result.(*object.String).StringValue())
	}

	result = evalStringIndexExpression(str, object.NewInteger(10))
	if result.Type() != object.NULL_OBJ {
		t.Errorf("expected NULL_OBJ for out of bounds, got %v", result.Type())
	}
}

// Test evalTupleIndexExpression
func TestEvalTupleIndexExpression(t *testing.T) {
	tuple := &object.Tuple{Elements: []object.Object{
		object.NewInteger(1),
		object.NewString("a"),
	}}

	result := evalTupleIndexExpression(tuple, object.NewInteger(0))
	if result.Type() != object.INTEGER_OBJ {
		t.Errorf("expected INTEGER_OBJ, got %v", result.Type())
	}

	result = evalTupleIndexExpression(tuple, object.NewInteger(-1))
	if result.Type() != object.STRING_OBJ {
		t.Errorf("expected STRING_OBJ, got %v", result.Type())
	}

	result = evalTupleIndexExpression(tuple, object.NewInteger(10))
	if result.Type() != object.NULL_OBJ {
		t.Errorf("expected NULL_OBJ for out of bounds, got %v", result.Type())
	}
}

// Test evalListSliceExpression
func TestEvalListSliceExpression(t *testing.T) {
	list := &object.List{Elements: []object.Object{
		object.NewInteger(1),
		object.NewInteger(2),
		object.NewInteger(3),
		object.NewInteger(4),
		object.NewInteger(5),
	}}

	tests := []struct {
		name     string
		slice    *object.Slice
		wantLen  int
		wantFirst int64
	}{
		{
			name: "1:3",
			slice: &object.Slice{
				Start: object.NewInteger(1),
				End:   object.NewInteger(3),
			},
			wantLen:   2,
			wantFirst: 2,
		},
		{
			name: ":3",
			slice: &object.Slice{
				Start: nil,
				End:   object.NewInteger(3),
			},
			wantLen:   3,
			wantFirst: 1,
		},
		{
			name: "2:",
			slice: &object.Slice{
				Start: object.NewInteger(2),
				End:   nil,
			},
			wantLen:   3,
			wantFirst: 3,
		},
		{
			name: ":",
			slice: &object.Slice{
				Start: nil,
				End:   nil,
			},
			wantLen:   5,
			wantFirst: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := evalListSliceExpression(list, tt.slice)
			if result.Type() != object.LIST_OBJ {
				t.Errorf("expected LIST_OBJ, got %v", result.Type())
				return
			}
			gotList := result.(*object.List)
			if len(gotList.Elements) != tt.wantLen {
				t.Errorf("length = %d, want %d", len(gotList.Elements), tt.wantLen)
			}
			if len(gotList.Elements) > 0 {
				first := gotList.Elements[0].(*object.Integer).IntValue()
				if first != tt.wantFirst {
					t.Errorf("first element = %d, want %d", first, tt.wantFirst)
				}
			}
		})
	}
}

// Test evalTupleSliceExpression
func TestEvalTupleSliceExpression(t *testing.T) {
	tuple := &object.Tuple{Elements: []object.Object{
		object.NewInteger(1),
		object.NewInteger(2),
		object.NewInteger(3),
	}}

	slice := &object.Slice{
		Start: object.NewInteger(1),
		End:   object.NewInteger(2),
	}

	result := evalTupleSliceExpression(tuple, slice)
	if result.Type() != object.TUPLE_OBJ {
		t.Errorf("expected TUPLE_OBJ, got %v", result.Type())
	}
	gotTuple := result.(*object.Tuple)
	if len(gotTuple.Elements) != 1 {
		t.Errorf("length = %d, want 1", len(gotTuple.Elements))
	}
}

// Test evalStringSliceExpression
func TestEvalStringSliceExpression(t *testing.T) {
	str := object.NewString("hello")

	tests := []struct {
		name      string
		slice     *object.Slice
		wantValue string
	}{
		{
			name: "1:4",
			slice: &object.Slice{
				Start: object.NewInteger(1),
				End:   object.NewInteger(4),
			},
			wantValue: "ell",
		},
		{
			name: ":3",
			slice: &object.Slice{
				Start: nil,
				End:   object.NewInteger(3),
			},
			wantValue: "hel",
		},
		{
			name: "2:",
			slice: &object.Slice{
				Start: object.NewInteger(2),
				End:   nil,
			},
			wantValue: "llo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := evalStringSliceExpression(str, tt.slice)
			if result.Type() != object.STRING_OBJ {
				t.Errorf("expected STRING_OBJ, got %v", result.Type())
				return
			}
			gotStr := result.(*object.String).StringValue()
			if gotStr != tt.wantValue {
				t.Errorf("got %q, want %q", gotStr, tt.wantValue)
			}
		})
	}
}
