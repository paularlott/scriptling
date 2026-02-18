package extlibs

import (
	"context"
	"testing"

	"github.com/paularlott/scriptling/object"
)

func TestTOMLFunctions(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		tomlStr  string
		expected interface{}
	}{
		{
			name:     "load basic table",
			tomlStr:  "[section]\nkey = \"value\"",
			expected: map[string]interface{}{"section": map[string]interface{}{"key": "value"}},
		},
		{
			name:     "load string value",
			tomlStr:  "name = \"John\"",
			expected: map[string]interface{}{"name": "John"},
		},
		{
			name:     "load integer value",
			tomlStr:  "count = 42",
			expected: map[string]interface{}{"count": int64(42)},
		},
		{
			name:     "load float value",
			tomlStr:  "pi = 3.14",
			expected: map[string]interface{}{"pi": 3.14},
		},
		{
			name:     "load boolean true",
			tomlStr:  "enabled = true",
			expected: map[string]interface{}{"enabled": true},
		},
		{
			name:     "load boolean false",
			tomlStr:  "enabled = false",
			expected: map[string]interface{}{"enabled": false},
		},
		{
			name:     "load array",
			tomlStr:  "items = [\"a\", \"b\", \"c\"]",
			expected: map[string]interface{}{"items": []interface{}{"a", "b", "c"}},
		},
		{
			name:     "load inline table",
			tomlStr:  "point = {x = 1, y = 2}",
			expected: map[string]interface{}{"point": map[string]interface{}{"x": int64(1), "y": int64(2)}},
		},
		{
			name:     "load nested tables",
			tomlStr:  "[database.connection]\nhost = \"localhost\"\nport = 5432",
			expected: map[string]interface{}{"database": map[string]interface{}{"connection": map[string]interface{}{"host": "localhost", "port": int64(5432)}}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tomlLoadsFunc(ctx, object.NewKwargs(nil), &object.String{Value: tt.tomlStr})
			if err, ok := result.(*object.Error); ok {
				t.Fatalf("unexpected error: %s", err.Message)
			}

			// Check the type and value
			checkTOMLResult(t, result, tt.expected)
		})
	}
}

func checkTOMLResult(t *testing.T, obj object.Object, expected interface{}) {
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
			checkTOMLResult(t, pair.Value, val)
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
			checkTOMLResult(t, list.Elements[i], val)
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

func TestTOMLConversionRoundTrip(t *testing.T) {
	ctx := context.Background()

	// Create a dict to test round-trip
	original := &object.Dict{
		Pairs: map[string]object.DictPair{
			object.DictKey(&object.String{Value: "title"}): {
				Key:   &object.String{Value: "title"},
				Value: &object.String{Value: "My App"},
			},
			object.DictKey(&object.String{Value: "count"}): {
				Key:   &object.String{Value: "count"},
				Value: &object.Integer{Value: 123},
			},
			object.DictKey(&object.String{Value: "active"}): {
				Key:   &object.String{Value: "active"},
				Value: &object.Boolean{Value: true},
			},
			object.DictKey(&object.String{Value: "database"}): {
				Key: &object.String{Value: "database"},
				Value: &object.Dict{
					Pairs: map[string]object.DictPair{
						object.DictKey(&object.String{Value: "host"}): {
							Key:   &object.String{Value: "host"},
							Value: &object.String{Value: "localhost"},
						},
						object.DictKey(&object.String{Value: "port"}): {
							Key:   &object.String{Value: "port"},
							Value: &object.Integer{Value: 5432},
						},
					},
				},
			},
		},
	}

	// Dump to TOML
	tomlResult := tomlDumpsFunc(ctx, object.NewKwargs(nil), original)
	tomlStr, ok := tomlResult.(*object.String)
	if !ok {
		t.Fatalf("expected string, got %T", tomlResult)
	}

	// Load back
	loaded := tomlLoadsFunc(ctx, object.NewKwargs(nil), tomlStr)
	if err, ok := loaded.(*object.Error); ok {
		t.Fatalf("unexpected error: %s", err.Message)
	}

	// Check if it's a dict
	loadedDict, ok := loaded.(*object.Dict)
	if !ok {
		t.Fatalf("expected dict, got %T", loaded)
	}

	// Check some values
	titlePair, exists := loadedDict.GetByString("title")
	if !exists {
		t.Error("missing title key")
	} else if str, ok := titlePair.Value.(*object.String); !ok || str.Value != "My App" {
		t.Errorf("expected title 'My App', got %v", titlePair.Value)
	}

	countPair, exists := loadedDict.GetByString("count")
	if !exists {
		t.Error("missing count key")
	} else if i, ok := countPair.Value.(*object.Integer); !ok || i.Value != 123 {
		t.Errorf("expected count 123, got %v", countPair.Value)
	}
}

func TestTOMLErrors(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		input       object.Object
		expectError bool
	}{
		{
			name:        "load invalid toml",
			input:       &object.String{Value: "invalid = [toml"},
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
				result = tomlLoadsFunc(ctx, object.NewKwargs(nil))
			} else {
				result = tomlLoadsFunc(ctx, object.NewKwargs(nil), tt.input)
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
