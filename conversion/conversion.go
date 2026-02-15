package conversion

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// ParseJSON parses a JSON string and returns a Scriptling object.
// It uses UseNumber() to preserve large integers.
func ParseJSON(jsonStr string) (object.Object, error) {
	var result interface{}
	decoder := json.NewDecoder(strings.NewReader(jsonStr))
	decoder.UseNumber()
	if err := decoder.Decode(&result); err != nil {
		return nil, err
	}
	return FromGo(result), nil
}

// MustParseJSON parses a JSON string and returns a Scriptling object,
// returning an Error object if parsing fails.
func MustParseJSON(jsonStr string) object.Object {
	result, err := ParseJSON(jsonStr)
	if err != nil {
		return errors.NewError("JSONDecodeError: %s", err.Error())
	}
	return result
}

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
	case json.Number:
		// Try to parse as integer first, then fall back to float
		if intVal, err := v.Int64(); err == nil {
			return object.NewInteger(intVal)
		}
		if floatVal, err := v.Float64(); err == nil {
			return &object.Float{Value: floatVal}
		}
		return &object.String{Value: string(v)}
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
	case map[interface{}]interface{}:
		// Handle YAML-specific map[interface{}]interface{} type
		pairs := make(map[string]object.DictPair)
		for key, val := range v {
			keyStr := ""
			switch k := key.(type) {
			case string:
				keyStr = k
			default:
				keyStr = fmt.Sprintf("%v", k)
			}
			pairs[keyStr] = object.DictPair{
				Key:   &object.String{Value: keyStr},
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
			if keyStr, err := pair.Key.AsString(); err == nil {
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

// ToGoError converts a scriptling Object to a Go error.
// If the object is an Error type, it returns a Go error with the error message.
// Otherwise, it returns nil.
func ToGoError(obj object.Object) error {
	if obj == nil {
		return nil
	}
	if err, ok := obj.(*object.Error); ok {
		return fmt.Errorf("%s", err.Inspect())
	}
	return nil
}

// ToGoWithError converts a Scriptling object to a Go value, returning error for complex types
func ToGoWithError(obj object.Object) (interface{}, *object.Error) {
	switch v := obj.(type) {
	case *object.String:
		return v.Value, nil
	case *object.Integer:
		return v.Value, nil
	case *object.Float:
		return v.Value, nil
	case *object.Boolean:
		return v.Value, nil
	case *object.Null:
		return nil, nil
	case *object.List:
		result := make([]interface{}, len(v.Elements))
		for i, elem := range v.Elements {
			converted, err := ToGoWithError(elem)
			if err != nil {
				return nil, err
			}
			result[i] = converted
		}
		return result, nil
	case *object.Dict:
		result := make(map[string]interface{})
		for key, pair := range v.Pairs {
			converted, err := ToGoWithError(pair.Value)
			if err != nil {
				return nil, err
			}
			result[key] = converted
		}
		return result, nil
	default:
		return nil, errors.NewError("cannot convert complex types to Go")
	}
}
