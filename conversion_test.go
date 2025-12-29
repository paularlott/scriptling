package scriptling

import (
	"testing"

	"github.com/paularlott/scriptling/object"
)

func TestFromGo_Primitives(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected object.Object
	}{
		{"nil", nil, &object.Null{}},
		{"bool true", true, &object.Boolean{Value: true}},
		{"bool false", false, &object.Boolean{Value: false}},
		{"int", 42, object.NewInteger(42)},
		{"int8", int8(8), object.NewInteger(8)},
		{"int16", int16(16), object.NewInteger(16)},
		{"int32", int32(32), object.NewInteger(32)},
		{"int64", int64(64), object.NewInteger(64)},
		{"uint", uint(42), object.NewInteger(42)},
		{"uint8", uint8(8), object.NewInteger(8)},
		{"uint16", uint16(16), object.NewInteger(16)},
		{"uint32", uint32(32), object.NewInteger(32)},
		{"float32", float32(3.14), &object.Float{Value: float64(float32(3.14))}},
		{"float64", 3.14159, &object.Float{Value: 3.14159}},
		{"string", "hello", &object.String{Value: "hello"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FromGo(tt.input)
			if result.Inspect() != tt.expected.Inspect() {
				t.Errorf("FromGo(%v) = %v, want %v", tt.input, result.Inspect(), tt.expected.Inspect())
			}
		})
	}
}

func TestFromGo_NestedStructures(t *testing.T) {
	// Test list
	list := []interface{}{1, "two", true}
	result := FromGo(list)
	resultList, ok := result.(*object.List)
	if !ok {
		t.Fatalf("FromGo([]interface{}) returned %T, want *object.List", result)
	}
	if len(resultList.Elements) != 3 {
		t.Errorf("FromGo([]interface{}) returned %d elements, want 3", len(resultList.Elements))
	}

	// Test dict
	dict := map[string]interface{}{
		"key1": "value1",
		"key2": 42,
		"nested": map[string]interface{}{
			"inner": "value",
		},
	}
	result = FromGo(dict)
	resultDict, ok := result.(*object.Dict)
	if !ok {
		t.Fatalf("FromGo(map[string]interface{}) returned %T, want *object.Dict", result)
	}
	if len(resultDict.Pairs) != 3 {
		t.Errorf("FromGo(map[string]interface{}) returned %d pairs, want 3", len(resultDict.Pairs))
	}
}

func TestFromGo_UnknownTypes(t *testing.T) {
	// Test a custom struct - should be converted via JSON
	type customStruct struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	input := customStruct{Name: "test", Value: 123}
	result := FromGo(input)

	// Should be converted to a Dict with the JSON fields
	resultDict, ok := result.(*object.Dict)
	if !ok {
		t.Fatalf("FromGo(customStruct) returned %T, want *object.Dict", result)
	}

	if len(resultDict.Pairs) != 2 {
		t.Errorf("FromGo(customStruct) returned %d pairs, want 2", len(resultDict.Pairs))
	}
}

func TestToGo_AllTypes(t *testing.T) {
	tests := []struct {
		name     string
		input    object.Object
		expected interface{}
	}{
		{"null", &object.Null{}, nil},
		{"bool true", &object.Boolean{Value: true}, true},
		{"bool false", &object.Boolean{Value: false}, false},
		{"integer", object.NewInteger(42), int64(42)},
		{"float", &object.Float{Value: 3.14}, 3.14},
		{"string", &object.String{Value: "hello"}, "hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToGo(tt.input)
			if result != tt.expected {
				t.Errorf("ToGo(%v) = %v (%T), want %v (%T)", tt.input, result, result, tt.expected, tt.expected)
			}
		})
	}
}

func TestToGo_List(t *testing.T) {
	input := &object.List{
		Elements: []object.Object{
			&object.String{Value: "one"},
			object.NewInteger(2),
			&object.Boolean{Value: true},
		},
	}

	result := ToGo(input)
	resultList, ok := result.([]interface{})
	if !ok {
		t.Fatalf("ToGo(List) returned %T, want []interface{}", result)
	}

	if len(resultList) != 3 {
		t.Errorf("ToGo(List) returned %d elements, want 3", len(resultList))
	}
}

func TestToGo_Dict(t *testing.T) {
	input := &object.Dict{
		Pairs: map[string]object.DictPair{
			"key1": {
				Key:   &object.String{Value: "key1"},
				Value: &object.String{Value: "value1"},
			},
			"key2": {
				Key:   &object.String{Value: "key2"},
				Value: object.NewInteger(42),
			},
		},
	}

	result := ToGo(input)
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("ToGo(Dict) returned %T, want map[string]interface{}", result)
	}

	if len(resultMap) != 2 {
		t.Errorf("ToGo(Dict) returned %d pairs, want 2", len(resultMap))
	}

	if resultMap["key1"] != "value1" {
		t.Errorf("ToGo(Dict)[\"key1\"] = %v, want \"value1\"", resultMap["key1"])
	}

	if resultMap["key2"] != int64(42) {
		t.Errorf("ToGo(Dict)[\"key2\"] = %v, want 42", resultMap["key2"])
	}
}

func TestRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
	}{
		{"null", nil},
		{"bool", true},
		{"int", 42},
		{"float", 3.14159},
		{"string", "hello world"},
		{"list", []interface{}{1, "two", true}},
		{"dict", map[string]interface{}{
			"key1": "value1",
			"key2": 42,
			"nested": []interface{}{"a", "b"},
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Go -> Scriptling -> Go
			scriptlingObj := FromGo(tt.value)
			result := ToGo(scriptlingObj)

			// For complex types, compare recursively
			compareResults(t, tt.value, result)
		})
	}
}

func compareResults(t *testing.T, expected, actual interface{}) {
	switch exp := expected.(type) {
	case nil:
		if actual != nil {
			t.Errorf("expected nil, got %v", actual)
		}
	case bool:
		if act, ok := actual.(bool); !ok || act != exp {
			t.Errorf("expected %v, got %v", exp, actual)
		}
	case int:
		if act, ok := actual.(int64); !ok || act != int64(exp) {
			t.Errorf("expected %v, got %v", exp, actual)
		}
	case int64:
		if act, ok := actual.(int64); !ok || act != exp {
			t.Errorf("expected %v, got %v", exp, actual)
		}
	case float64:
		if act, ok := actual.(float64); !ok || act != exp {
			t.Errorf("expected %v, got %v", exp, actual)
		}
	case string:
		if act, ok := actual.(string); !ok || act != exp {
			t.Errorf("expected %q, got %q", exp, actual)
		}
	case []interface{}:
		act, ok := actual.([]interface{})
		if !ok || len(act) != len(exp) {
			t.Errorf("expected list of length %d, got %v", len(exp), actual)
			return
		}
		for i := range exp {
			compareResults(t, exp[i], act[i])
		}
	case map[string]interface{}:
		act, ok := actual.(map[string]interface{})
		if !ok || len(act) != len(exp) {
			t.Errorf("expected dict of length %d, got %v", len(exp), actual)
			return
		}
		for key, expVal := range exp {
			actVal, ok := act[key]
			if !ok {
				t.Errorf("missing key %q in result", key)
				continue
			}
			compareResults(t, expVal, actVal)
		}
	default:
		t.Errorf("unsupported type %T in comparison", expected)
	}
}
