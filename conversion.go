package scriptling

import (
	"encoding/json"
	"fmt"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// FromGo converts a Go interface{} value to a scriptling Object.
// It handles primitive types (nil, bool, int, float, string), nested structures
// (maps, slices), and falls back to JSON marshaling for unknown types.
func FromGo(v interface{}) object.Object {
	switch v := v.(type) {
	case nil:
		return &object.Null{}
	case bool:
		return &object.Boolean{Value: v}
	case int:
		return object.NewInteger(int64(v))
	case int8:
		return object.NewInteger(int64(v))
	case int16:
		return object.NewInteger(int64(v))
	case int32:
		return object.NewInteger(int64(v))
	case int64:
		return object.NewInteger(v)
	case uint:
		return object.NewInteger(int64(v))
	case uint8:
		return object.NewInteger(int64(v))
	case uint16:
		return object.NewInteger(int64(v))
	case uint32:
		return object.NewInteger(int64(v))
	case uint64:
		// Note: May overflow for very large uint64 values
		return object.NewInteger(int64(v))
	case float32:
		return &object.Float{Value: float64(v)}
	case float64:
		return &object.Float{Value: v}
	case string:
		return &object.String{Value: v}
	case []interface{}:
		elements := make([]object.Object, len(v))
		for i, item := range v {
			elements[i] = FromGo(item)
		}
		return &object.List{Elements: elements}
	case map[string]interface{}:
		pairs := make(map[string]object.DictPair)
		for key, val := range v {
			pairs[key] = object.DictPair{
				Key:   &object.String{Value: key},
				Value: FromGo(val),
			}
		}
		return &object.Dict{Pairs: pairs}
	default:
		// For unknown types, try to convert to JSON then parse
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			return &object.String{Value: fmt.Sprintf("%v", v)}
		}
		var result interface{}
		if err := json.Unmarshal(jsonBytes, &result); err != nil {
			return &object.String{Value: fmt.Sprintf("%v", v)}
		}
		return FromGo(result)
	}
}

// ToGo converts a scriptling Object to a Go interface{}.
// It returns the underlying Go value for the object type.
func ToGo(obj object.Object) interface{} {
	if obj == nil {
		return nil
	}

	switch o := obj.(type) {
	case *object.Null:
		return nil
	case *object.Boolean:
		return o.Value
	case *object.Integer:
		return o.Value
	case *object.Float:
		return o.Value
	case *object.String:
		return o.Value
	case *object.List:
		result := make([]interface{}, len(o.Elements))
		for i, elem := range o.Elements {
			result[i] = ToGo(elem)
		}
		return result
	case *object.Tuple:
		result := make([]interface{}, len(o.Elements))
		for i, elem := range o.Elements {
			result[i] = ToGo(elem)
		}
		return result
	case *object.Dict:
		result := make(map[string]interface{})
		for _, pair := range o.Pairs {
			// Keys can be any Object, but typically are strings
			if keyStr, ok := pair.Key.AsString(); ok {
				result[keyStr] = ToGo(pair.Value)
			} else {
				// For non-string keys, use Inspect() representation
				result[pair.Key.Inspect()] = ToGo(pair.Value)
			}
		}
		return result
	case *object.Error:
		return o.Message
	case *object.Builtin:
		// Return help text if available, otherwise generic string
		if o.HelpText != "" {
			return o.HelpText
		}
		return "<builtin function>"
	case *object.Function:
		return o.Name
	default:
		// For other types (like ReturnValue, Break, Continue), return string representation
		return o.Inspect()
	}
}

// Helper functions for extracting arguments with automatic error generation.
// These make stdlib/extlib function implementations more compact and consistent.

// GetString extracts a string argument at the given index.
// Returns the value and nil on success, or empty string and an error on failure.
func GetString(args []object.Object, index int, name string) (string, object.Object) {
	if index >= len(args) {
		return "", errors.NewError("%s: missing argument", name)
	}
	if s, ok := args[index].AsString(); ok {
		return s, nil
	}
	return "", errors.NewError("%s: must be a string", name)
}

// GetInt extracts an integer argument at the given index.
// Returns the value and nil on success, or 0 and an error on failure.
func GetInt(args []object.Object, index int, name string) (int64, object.Object) {
	if index >= len(args) {
		return 0, errors.NewError("%s: missing argument", name)
	}
	if i, ok := args[index].AsInt(); ok {
		return i, nil
	}
	return 0, errors.NewError("%s: must be an integer", name)
}

// GetFloat extracts a float argument at the given index.
// Returns the value and nil on success, or 0 and an error on failure.
func GetFloat(args []object.Object, index int, name string) (float64, object.Object) {
	if index >= len(args) {
		return 0, errors.NewError("%s: missing argument", name)
	}
	if f, ok := args[index].AsFloat(); ok {
		return f, nil
	}
	// Also accept integers and convert to float
	if i, ok := args[index].AsInt(); ok {
		return float64(i), nil
	}
	return 0, errors.NewError("%s: must be a number", name)
}

// GetBool extracts a boolean argument at the given index.
// Returns the value and nil on success, or false and an error on failure.
func GetBool(args []object.Object, index int, name string) (bool, object.Object) {
	if index >= len(args) {
		return false, errors.NewError("%s: missing argument", name)
	}
	if b, ok := args[index].AsBool(); ok {
		return b, nil
	}
	return false, errors.NewError("%s: must be a boolean", name)
}

// GetList extracts a list argument at the given index.
// Returns the value and nil on success, or nil and an error on failure.
func GetList(args []object.Object, index int, name string) ([]object.Object, object.Object) {
	if index >= len(args) {
		return nil, errors.NewError("%s: missing argument", name)
	}
	if l, ok := args[index].AsList(); ok {
		return l, nil
	}
	return nil, errors.NewError("%s: must be a list", name)
}

// GetDict extracts a dict argument at the given index.
// Returns the value and nil on success, or nil and an error on failure.
func GetDict(args []object.Object, index int, name string) (map[string]object.Object, object.Object) {
	if index >= len(args) {
		return nil, errors.NewError("%s: missing argument", name)
	}
	if d, ok := args[index].AsDict(); ok {
		return d, nil
	}
	return nil, errors.NewError("%s: must be a dict", name)
}

// GetStringOptional extracts an optional string argument at the given index.
// Returns the value, true if present, or default value, false if not present or error.
func GetStringOptional(args []object.Object, index int, name string, defaultValue string) (string, bool, object.Object) {
	if index >= len(args) {
		return defaultValue, false, nil
	}
	if s, ok := args[index].AsString(); ok {
		return s, true, nil
	}
	return defaultValue, false, errors.NewError("%s: must be a string", name)
}

// GetIntOptional extracts an optional integer argument at the given index.
// Returns the value, true if present, or default value, false if not present or error.
func GetIntOptional(args []object.Object, index int, name string, defaultValue int64) (int64, bool, object.Object) {
	if index >= len(args) {
		return defaultValue, false, nil
	}
	if i, ok := args[index].AsInt(); ok {
		return i, true, nil
	}
	return defaultValue, false, errors.NewError("%s: must be an integer", name)
}
