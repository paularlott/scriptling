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
		return object.NewBoolean(v)
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
		return object.NewFloat(float64(v))
	case float64:
		return object.NewFloat(v)
	case json.Number:
		// Try to parse as integer first, then fall back to float
		if intVal, err := v.Int64(); err == nil {
			return object.NewInteger(intVal)
		}
		if floatVal, err := v.Float64(); err == nil {
			return object.NewFloat(floatVal)
		}
		return object.NewString(string(v))
	case string:
		return object.NewString(v)
	case []byte:
		return object.NewBytes(v)
	case []interface{}:
		elements := make([]object.Object, len(v))
		for i, item := range v {
			elements[i] = FromGo(item)
		}
		return &object.List{Elements: elements}
	case map[string]interface{}:
		pairs := make(map[string]object.DictPair, len(v))
		for key, val := range v {
			pairs[object.DictStringKey(key)] = object.DictPair{
				Key:   object.NewString(key),
				Value: FromGo(val),
			}
		}
		return &object.Dict{Pairs: pairs}
	case map[interface{}]interface{}:
		pairs := make(map[string]object.DictPair, len(v))
		for key, val := range v {
			keyStr := ""
			switch k := key.(type) {
			case string:
				keyStr = k
			default:
				keyStr = fmt.Sprintf("%v", k)
			}
			pairs[object.DictStringKey(keyStr)] = object.DictPair{
				Key:   object.NewString(keyStr),
				Value: FromGo(val),
			}
		}
		return &object.Dict{Pairs: pairs}
	default:
		// For unknown types, try to convert to JSON then parse
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			return object.NewString(fmt.Sprintf("%v", v))
		}
		var result interface{}
		if err := json.Unmarshal(jsonBytes, &result); err != nil {
			return object.NewString(fmt.Sprintf("%v", v))
		}
		return FromGo(result)
	}
}

// cyclicRefPlaceholder replaces a container that references itself (directly
// or through a chain) when converting to Go, mirroring how repr shows cyclic
// Python lists. Without the check a self-referential list built by a script
// recurses until the Go stack overflows, which no recover() can catch.
const cyclicRefPlaceholder = "<cyclic reference>"

// ToGo converts a scriptling Object to a Go interface{}.
// It returns the underlying Go value for the object type.
func ToGo(obj object.Object) interface{} {
	return toGo(obj, make(map[object.Object]struct{}))
}

// toGo walks obj with the set of containers on the current path. A container
// already on the path is a cycle; entries are removed again on the way out so
// a substructure shared by two parents (a DAG, not a cycle) still converts
// fully.
func toGo(obj object.Object, path map[object.Object]struct{}) interface{} {
	if obj == nil {
		return nil
	}

	switch o := obj.(type) {
	case *object.Null:
		return nil
	case *object.Boolean:
		return o.BoolValue()
	case *object.Integer:
		return o.IntValue()
	case *object.Float:
		return o.FloatValue()
	case *object.String:
		return o.StringValue()
	case *object.Bytes:
		return o.BytesValue()
	case *object.List:
		if _, cyclic := path[o]; cyclic {
			return cyclicRefPlaceholder
		}
		path[o] = struct{}{}
		result := make([]interface{}, len(o.Elements))
		for i, elem := range o.Elements {
			result[i] = toGo(elem, path)
		}
		delete(path, o)
		return result
	case *object.Tuple:
		if _, cyclic := path[o]; cyclic {
			return cyclicRefPlaceholder
		}
		path[o] = struct{}{}
		result := make([]interface{}, len(o.Elements))
		for i, elem := range o.Elements {
			result[i] = toGo(elem, path)
		}
		delete(path, o)
		return result
	case *object.Dict:
		if _, cyclic := path[o]; cyclic {
			return cyclicRefPlaceholder
		}
		path[o] = struct{}{}
		result := make(map[string]interface{})
		for _, pair := range o.Pairs {
			result[pair.StringKey()] = toGo(pair.Value, path)
		}
		delete(path, o)
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
	case *object.FloatArray:
		if o.Is2D() {
			rows := o.Rows()
			cols := o.Cols()
			result := make([]interface{}, rows)
			for i := 0; i < rows; i++ {
				row := make([]interface{}, cols)
				off := i * cols
				for j := 0; j < cols; j++ {
					row[j] = o.Data[off+j]
				}
				result[i] = row
			}
			return result
		}
		result := make([]interface{}, len(o.Data))
		for i, v := range o.Data {
			result[i] = v
		}
		return result
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

// ToBytes accepts a Bytes or String input and returns the raw byte slice.
// Used by libraries that historically took strings but now also accept Bytes
// (file I/O, HTTP bodies, socket send, etc.). Returns a Scriptling *Error
// object for any other type so callers can return it directly.
//
// Bytes are returned as-is (no copy — treat the result as read-only).
// Strings are UTF-8 encoded into a fresh byte slice.
func ToBytes(obj object.Object) ([]byte, object.Object) {
	switch v := obj.(type) {
	case *object.Bytes:
		return v.BytesValue(), nil
	case *object.String:
		return []byte(v.StringValue()), nil
	default:
		return nil, errors.NewTypeError("BYTES or STRING", obj.Type().String())
	}
}

// ToGoWithError converts a Scriptling object to a Go value, returning error for complex types
func ToGoWithError(obj object.Object) (interface{}, *object.Error) {
	switch v := obj.(type) {
	case *object.String:
		return v.StringValue(), nil
	case *object.Bytes:
		return v.BytesValue(), nil
	case *object.Integer:
		return v.IntValue(), nil
	case *object.Float:
		return v.FloatValue(), nil
	case *object.Boolean:
		return v.BoolValue(), nil
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
		for _, pair := range v.Pairs {
			converted, err := ToGoWithError(pair.Value)
			if err != nil {
				return nil, err
			}
			result[pair.StringKey()] = converted
		}
		return result, nil
	default:
		return nil, errors.NewError("cannot convert complex types to Go")
	}
}
