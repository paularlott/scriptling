package extlibs

import (
	"context"
	"testing"

	"github.com/paularlott/scriptling/object"
)

func TestYAMLFunctions(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		yamlStr  string
		expected interface{}
	}{
		{
			name:     "load basic dict",
			yamlStr:  "name: John\nage: 30",
			expected: map[string]interface{}{"name": "John", "age": int64(30)},
		},
		{
			name:     "load list",
			yamlStr:  "- item1\n- item2\n- item3",
			expected: []interface{}{"item1", "item2", "item3"},
		},
		{
			name:     "load null",
			yamlStr:  "value: null",
			expected: map[string]interface{}{"value": nil},
		},
		{
			name:     "load boolean",
			yamlStr:  "flag: true",
			expected: map[string]interface{}{"flag": true},
		},
		{
			name:     "load integer",
			yamlStr:  "count: 42",
			expected: map[string]interface{}{"count": int64(42)},
		},
		{
			name:     "load float",
			yamlStr:  "pi: 3.14",
			expected: map[string]interface{}{"pi": 3.14},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := yamlLoadFunc(ctx, object.NewKwargs(nil), &object.String{Value: tt.yamlStr})
			if err, ok := result.(*object.Error); ok {
				t.Fatalf("unexpected error: %s", err.Message)
			}

			// Check the type and value
			checkResult(t, result, tt.expected)
		})
	}
}

func checkResult(t *testing.T, obj object.Object, expected interface{}) {
	switch exp := expected.(type) {
	case map[string]interface{}:
		dict, ok := obj.(*object.Dict)
		if !ok {
			t.Errorf("expected dict, got %T", obj)
			return
		}
		for key, val := range exp {
			pair, exists := dict.GetByString(key)
			if !exists {
				t.Errorf("missing key %s", key)
				continue
			}
			checkResult(t, pair.Value, val)
		}
	case []interface{}:
		list, ok := obj.(*object.List)
		if !ok {
			t.Errorf("expected list, got %T", obj)
			return
		}
		if len(list.Elements) != len(exp) {
			t.Errorf("expected %d elements, got %d", len(exp), len(list.Elements))
			return
		}
		for i, val := range exp {
			checkResult(t, list.Elements[i], val)
		}
	case string:
		str, ok := obj.(*object.String)
		if !ok || str.Value != exp {
			t.Errorf("expected string %q, got %v", exp, obj)
		}
	case int64:
		i, ok := obj.(*object.Integer)
		if !ok || i.Value != exp {
			t.Errorf("expected int64 %d, got %v", exp, obj)
		}
	case float64:
		f, ok := obj.(*object.Float)
		if !ok || f.Value != exp {
			t.Errorf("expected float64 %f, got %v", exp, obj)
		}
	case bool:
		b, ok := obj.(*object.Boolean)
		if !ok || b.Value != exp {
			t.Errorf("expected bool %t, got %v", exp, obj)
		}
	case nil:
		if _, ok := obj.(*object.Null); !ok {
			t.Errorf("expected null, got %v", obj)
		}
	}
}

func TestYAMLConversionRoundTrip(t *testing.T) {
	ctx := context.Background()

	// Create a dict to test round-trip
	original := &object.Dict{
		Pairs: map[string]object.DictPair{
			object.DictKey(&object.String{Value: "name"}): {
				Key:   &object.String{Value: "name"},
				Value: &object.String{Value: "Test"},
			},
			object.DictKey(&object.String{Value: "count"}): {
				Key:   &object.String{Value: "count"},
				Value: &object.Integer{Value: 123},
			},
			object.DictKey(&object.String{Value: "active"}): {
				Key:   &object.String{Value: "active"},
				Value: &object.Boolean{Value: true},
			},
			object.DictKey(&object.String{Value: "items"}): {
				Key: &object.String{Value: "items"},
				Value: &object.List{
					Elements: []object.Object{
						&object.String{Value: "a"},
						&object.String{Value: "b"},
						&object.String{Value: "c"},
					},
				},
			},
		},
	}

	// Dump to YAML
	yamlResult := yamlDumpFunc(ctx, object.NewKwargs(nil), original)
	yamlStr, ok := yamlResult.(*object.String)
	if !ok {
		t.Fatalf("expected string, got %T", yamlResult)
	}

	// Load back
	loaded := yamlLoadFunc(ctx, object.NewKwargs(nil), yamlStr)
	if err, ok := loaded.(*object.Error); ok {
		t.Fatalf("unexpected error: %s", err.Message)
	}

	// Check if it's a dict
	loadedDict, ok := loaded.(*object.Dict)
	if !ok {
		t.Fatalf("expected dict, got %T", loaded)
	}

	// Check some values
	namePair, exists := loadedDict.GetByString("name")
	if !exists {
		t.Error("missing name key")
	} else if str, ok := namePair.Value.(*object.String); !ok || str.Value != "Test" {
		t.Errorf("expected name 'Test', got %v", namePair.Value)
	}

	countPair, exists := loadedDict.GetByString("count")
	if !exists {
		t.Error("missing count key")
	} else if i, ok := countPair.Value.(*object.Integer); !ok || i.Value != 123 {
		t.Errorf("expected count 123, got %v", countPair.Value)
	}
}

func TestYAMLErrors(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		input       object.Object
		expectError bool
	}{
		{
			name:        "load invalid yaml",
			input:       &object.String{Value: "invalid: yaml: content"},
			expectError: true,
		},
		{
			name:        "load wrong type",
			input:       &object.Integer{Value: 42},
			expectError: true,
		},
		{
			name:        "load no args",
			input:       nil,
			expectError: true,
		},
		{
			name:        "dump no args",
			input:       nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result object.Object
			if tt.input == nil {
				result = yamlLoadFunc(ctx, object.NewKwargs(nil))
			} else {
				result = yamlLoadFunc(ctx, object.NewKwargs(nil), tt.input)
			}

			if tt.expectError {
				if _, ok := result.(*object.Error); !ok {
					t.Error("expected error but got none")
				}
			} else {
				if _, ok := result.(*object.Error); ok {
					t.Errorf("unexpected error: %v", result)
				}
			}
		})
	}
}