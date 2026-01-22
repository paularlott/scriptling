package scriptling

import (
"context"
"testing"

"github.com/paularlott/scriptling/object"
)

// TestCallFunctionWithMapAsArgument tests that maps passed as arguments
// are correctly passed as dicts to the function, not treated as kwargs
func TestCallFunctionWithMapAsArgument(t *testing.T) {
	t.Run("map_as_single_argument", func(t *testing.T) {
p := New()

		// Register a function that expects a dict as first positional argument
		p.RegisterFunc("process_dict", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
if len(args) != 1 {
return &object.Error{Message: "expected 1 argument"}
			}
			dict, err := args[0].AsDict()
			if err != nil {
				return &object.Error{Message: "argument is not a dict"}
			}
			// Return the value of "key" from the dict
			if val, ok := dict["key"]; ok {
				return val
			}
			return &object.Error{Message: "key not found"}
		})

		// Call with a map as the ONLY argument - should be treated as a dict, not kwargs
		dataMap := map[string]interface{}{
			"key": "value123",
		}
		result, err := p.CallFunction("process_dict", dataMap)
		if err != nil {
			t.Fatalf("CallFunction failed: %v", err)
		}

		str, objErr := result.AsString()
		if objErr != nil {
			t.Fatalf("result is not a string: %v", objErr)
		}
		if str != "value123" {
			t.Errorf("expected 'value123', got '%s'", str)
		}
	})

	t.Run("map_as_last_argument", func(t *testing.T) {
p := New()

		// Register a function that expects a string and a dict
		p.RegisterFunc("process_with_dict", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
if len(args) != 2 {
return &object.Error{Message: "expected 2 arguments"}
			}
			prefix, err := args[0].AsString()
			if err != nil {
				return &object.Error{Message: "first argument is not a string"}
			}
			dict, err := args[1].AsDict()
			if err != nil {
				return &object.Error{Message: "second argument is not a dict"}
			}
			// Return prefix + value from dict
			if val, ok := dict["key"]; ok {
				valStr, _ := val.AsString()
				return &object.String{Value: prefix + valStr}
			}
			return &object.Error{Message: "key not found"}
		})

		// Call with a map as the LAST argument - should be treated as a dict, not kwargs
		dataMap := map[string]interface{}{
			"key": "data",
		}
		result, err := p.CallFunction("process_with_dict", "prefix:", dataMap)
		if err != nil {
			t.Fatalf("CallFunction failed: %v", err)
		}

		str, objErr := result.AsString()
		if objErr != nil {
			t.Fatalf("result is not a string: %v", objErr)
		}
		if str != "prefix:data" {
			t.Errorf("expected 'prefix:data', got '%s'", str)
		}
	})

	t.Run("map_as_middle_argument", func(t *testing.T) {
p := New()

		// Register a function that expects string, dict, int
		p.RegisterFunc("process_multi", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
if len(args) != 3 {
return &object.Error{Message: "expected 3 arguments"}
			}
			prefix, err := args[0].AsString()
			if err != nil {
				return &object.Error{Message: "first argument is not a string"}
			}
			dict, err := args[1].AsDict()
			if err != nil {
				return &object.Error{Message: "second argument is not a dict"}
			}
			num, err := args[2].AsInt()
			if err != nil {
				return &object.Error{Message: "third argument is not an int"}
			}
			// Return combined result
			if val, ok := dict["key"]; ok {
				valStr, _ := val.AsString()
				return &object.String{Value: prefix + valStr + string(rune('0'+num))}
			}
			return &object.Error{Message: "key not found"}
		})

		// Call with a map in the MIDDLE - should work fine
		dataMap := map[string]interface{}{
			"key": "middle",
		}
		result, err := p.CallFunction("process_multi", "start:", dataMap, 7)
		if err != nil {
			t.Fatalf("CallFunction failed: %v", err)
		}

		str, objErr := result.AsString()
		if objErr != nil {
			t.Fatalf("result is not a string: %v", objErr)
		}
		if str != "start:middle7" {
			t.Errorf("expected 'start:middle7', got '%s'", str)
		}
	})
}
