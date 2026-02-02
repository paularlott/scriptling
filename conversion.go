package scriptling

import (
	"encoding/json"
	"fmt"

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

// convertArgsAndKwargs converts Go arguments to Object arguments and separates kwargs.
// If prependSelf is not nil, it will be prepended to the argument list.
// Returns (objArgs, objKwargs).
func convertArgsAndKwargs(args []interface{}, prependSelf object.Object) ([]object.Object, map[string]object.Object) {
	var objArgs []object.Object
	var objKwargs map[string]object.Object

	if len(args) > 0 {
		// Check if last argument is Kwargs
		lastIdx := len(args) - 1
		if kwargsMap, ok := args[lastIdx].(Kwargs); ok {
			// Convert kwargs
			objKwargs = make(map[string]object.Object, len(kwargsMap))
			for key, val := range kwargsMap {
				objKwargs[key] = FromGo(val)
			}
			// Convert positional args (excluding kwargs)
			if prependSelf != nil {
				objArgs = make([]object.Object, lastIdx+1)
				objArgs[0] = prependSelf
				for i, arg := range args[:lastIdx] {
					objArgs[i+1] = FromGo(arg)
				}
			} else {
				objArgs = make([]object.Object, lastIdx)
				for i, arg := range args[:lastIdx] {
					objArgs[i] = FromGo(arg)
				}
			}
		} else {
			// No kwargs, convert all args
			if prependSelf != nil {
				objArgs = make([]object.Object, len(args)+1)
				objArgs[0] = prependSelf
				for i, arg := range args {
					objArgs[i+1] = FromGo(arg)
				}
			} else {
				objArgs = make([]object.Object, len(args))
				for i, arg := range args {
					objArgs[i] = FromGo(arg)
				}
			}
		}
	} else if prependSelf != nil {
		// No args, just self
		objArgs = []object.Object{prependSelf}
	}

	return objArgs, objKwargs
}
