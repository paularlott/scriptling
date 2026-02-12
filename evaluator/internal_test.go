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
			a:    &object.Integer{Value: 42},
			b:    &object.Integer{Value: 42},
			want: true,
		},
		{
			name: "unequal integers",
			a:    &object.Integer{Value: 42},
			b:    &object.Integer{Value: 43},
			want: false,
		},
		{
			name: "equal floats",
			a:    &object.Float{Value: 3.14},
			b:    &object.Float{Value: 3.14},
			want: true,
		},
		{
			name: "unequal floats",
			a:    &object.Float{Value: 3.14},
			b:    &object.Float{Value: 2.71},
			want: false,
		},
		{
			name: "equal strings",
			a:    &object.String{Value: "hello"},
			b:    &object.String{Value: "hello"},
			want: true,
		},
		{
			name: "unequal strings",
			a:    &object.String{Value: "hello"},
			b:    &object.String{Value: "world"},
			want: false,
		},
		{
			name: "equal booleans",
			a:    &object.Boolean{Value: true},
			b:    &object.Boolean{Value: true},
			want: true,
		},
		{
			name: "unequal booleans",
			a:    &object.Boolean{Value: true},
			b:    &object.Boolean{Value: false},
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
			a:    &object.Integer{Value: 42},
			b:    &object.String{Value: "42"},
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
			a:    &object.Integer{Value: 42},
			b:    &object.Integer{Value: 42},
			want: true,
		},
		{
			name: "equal floats",
			a:    &object.Float{Value: 3.14},
			b:    &object.Float{Value: 3.14},
			want: true,
		},
		{
			name: "equal strings",
			a:    &object.String{Value: "hello"},
			b:    &object.String{Value: "hello"},
			want: true,
		},
		{
			name: "equal lists",
			a:    &object.List{Elements: []object.Object{&object.Integer{Value: 1}, &object.Integer{Value: 2}}},
			b:    &object.List{Elements: []object.Object{&object.Integer{Value: 1}, &object.Integer{Value: 2}}},
			want: true,
		},
		{
			name: "unequal lists different length",
			a:    &object.List{Elements: []object.Object{&object.Integer{Value: 1}}},
			b:    &object.List{Elements: []object.Object{&object.Integer{Value: 1}, &object.Integer{Value: 2}}},
			want: false,
		},
		{
			name: "unequal lists different elements",
			a:    &object.List{Elements: []object.Object{&object.Integer{Value: 1}, &object.Integer{Value: 2}}},
			b:    &object.List{Elements: []object.Object{&object.Integer{Value: 1}, &object.Integer{Value: 3}}},
			want: false,
		},
		{
			name: "nested lists equal",
			a:    &object.List{Elements: []object.Object{&object.List{Elements: []object.Object{&object.Integer{Value: 1}}}}},
			b:    &object.List{Elements: []object.Object{&object.List{Elements: []object.Object{&object.Integer{Value: 1}}}}},
			want: true,
		},
		{
			name: "equal tuples",
			a:    &object.Tuple{Elements: []object.Object{&object.Integer{Value: 1}, &object.String{Value: "a"}}},
			b:    &object.Tuple{Elements: []object.Object{&object.Integer{Value: 1}, &object.String{Value: "a"}}},
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
			a:    &object.Dict{Pairs: map[string]object.DictPair{
				"a": {Key: &object.String{Value: "a"}, Value: &object.Integer{Value: 1}},
			}},
			b:    &object.Dict{Pairs: map[string]object.DictPair{
				"a": {Key: &object.String{Value: "a"}, Value: &object.Integer{Value: 1}},
			}},
			want: true,
		},
		{
			name: "unequal dicts different keys",
			a:    &object.Dict{Pairs: map[string]object.DictPair{
				"a": {Key: &object.String{Value: "a"}, Value: &object.Integer{Value: 1}},
			}},
			b:    &object.Dict{Pairs: map[string]object.DictPair{
				"b": {Key: &object.String{Value: "b"}, Value: &object.Integer{Value: 2}},
			}},
			want: false,
		},
		{
			name: "different types",
			a:    &object.Integer{Value: 42},
			b:    &object.String{Value: "42"},
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
		&object.Integer{Value: 1},
		&object.Integer{Value: 2},
		&object.Integer{Value: 3},
	}}

	// Test positive index
	result := evalListIndexExpression(list, &object.Integer{Value: 1})
	if result.Type() != object.INTEGER_OBJ {
		t.Errorf("expected INTEGER_OBJ, got %v", result.Type())
	}
	if result.(*object.Integer).Value != 2 {
		t.Errorf("expected 2, got %d", result.(*object.Integer).Value)
	}

	// Test negative index
	result = evalListIndexExpression(list, &object.Integer{Value: -1})
	if result.Type() != object.INTEGER_OBJ {
		t.Errorf("expected INTEGER_OBJ, got %v", result.Type())
	}
	if result.(*object.Integer).Value != 3 {
		t.Errorf("expected 3, got %d", result.(*object.Integer).Value)
	}

	// Test out of bounds positive index (returns NULL for Python-like behavior)
	result = evalListIndexExpression(list, &object.Integer{Value: 10})
	if result.Type() != object.NULL_OBJ {
		t.Errorf("expected NULL_OBJ for out of bounds, got %v", result.Type())
	}

	// Test out of bounds negative index (returns NULL for Python-like behavior)
	result = evalListIndexExpression(list, &object.Integer{Value: -10})
	if result.Type() != object.NULL_OBJ {
		t.Errorf("expected NULL_OBJ for out of bounds, got %v", result.Type())
	}
}

// Test evalStringIndexExpression
func TestEvalStringIndexExpression(t *testing.T) {
	str := &object.String{Value: "hello"}

	// Test positive index
	result := evalStringIndexExpression(str, &object.Integer{Value: 1})
	if result.Type() != object.STRING_OBJ {
		t.Errorf("expected STRING_OBJ, got %v", result.Type())
	}
	if result.(*object.String).Value != "e" {
		t.Errorf("expected 'e', got %q", result.(*object.String).Value)
	}

	// Test negative index
	result = evalStringIndexExpression(str, &object.Integer{Value: -1})
	if result.Type() != object.STRING_OBJ {
		t.Errorf("expected STRING_OBJ, got %v", result.Type())
	}
	if result.(*object.String).Value != "o" {
		t.Errorf("expected 'o', got %q", result.(*object.String).Value)
	}

	// Test out of bounds (returns NULL for Python-like behavior)
	result = evalStringIndexExpression(str, &object.Integer{Value: 10})
	if result.Type() != object.NULL_OBJ {
		t.Errorf("expected NULL_OBJ for out of bounds, got %v", result.Type())
	}
}

// Test evalTupleIndexExpression
func TestEvalTupleIndexExpression(t *testing.T) {
	tuple := &object.Tuple{Elements: []object.Object{
		&object.Integer{Value: 1},
		&object.String{Value: "a"},
	}}

	// Test positive index
	result := evalTupleIndexExpression(tuple, &object.Integer{Value: 0})
	if result.Type() != object.INTEGER_OBJ {
		t.Errorf("expected INTEGER_OBJ, got %v", result.Type())
	}

	// Test negative index
	result = evalTupleIndexExpression(tuple, &object.Integer{Value: -1})
	if result.Type() != object.STRING_OBJ {
		t.Errorf("expected STRING_OBJ, got %v", result.Type())
	}

	// Test out of bounds (returns NULL for Python-like behavior)
	result = evalTupleIndexExpression(tuple, &object.Integer{Value: 10})
	if result.Type() != object.NULL_OBJ {
		t.Errorf("expected NULL_OBJ for out of bounds, got %v", result.Type())
	}
}

// Test evalListSliceExpression
func TestEvalListSliceExpression(t *testing.T) {
	list := &object.List{Elements: []object.Object{
		&object.Integer{Value: 1},
		&object.Integer{Value: 2},
		&object.Integer{Value: 3},
		&object.Integer{Value: 4},
		&object.Integer{Value: 5},
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
				Start: &object.Integer{Value: 1},
				End:   &object.Integer{Value: 3},
			},
			wantLen:   2,
			wantFirst: 2,
		},
		{
			name: ":3",
			slice: &object.Slice{
				Start: nil,
				End:   &object.Integer{Value: 3},
			},
			wantLen:   3,
			wantFirst: 1,
		},
		{
			name: "2:",
			slice: &object.Slice{
				Start: &object.Integer{Value: 2},
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
				first := gotList.Elements[0].(*object.Integer).Value
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
		&object.Integer{Value: 1},
		&object.Integer{Value: 2},
		&object.Integer{Value: 3},
	}}

	slice := &object.Slice{
		Start: &object.Integer{Value: 1},
		End:   &object.Integer{Value: 2},
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
	str := &object.String{Value: "hello"}

	tests := []struct {
		name      string
		slice     *object.Slice
		wantValue string
	}{
		{
			name: "1:4",
			slice: &object.Slice{
				Start: &object.Integer{Value: 1},
				End:   &object.Integer{Value: 4},
			},
			wantValue: "ell",
		},
		{
			name: ":3",
			slice: &object.Slice{
				Start: nil,
				End:   &object.Integer{Value: 3},
			},
			wantValue: "hel",
		},
		{
			name: "2:",
			slice: &object.Slice{
				Start: &object.Integer{Value: 2},
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
			gotStr := result.(*object.String).Value
			if gotStr != tt.wantValue {
				t.Errorf("got %q, want %q", gotStr, tt.wantValue)
			}
		})
	}
}
