package stdlib

import (
	"context"
	"encoding/json"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

var JSONLibrary = object.NewLibrary(map[string]*object.Builtin{
	"loads": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewError("wrong number of arguments. got=%d, want=1", len(args))
			}
			if args[0].Type() != object.STRING_OBJ {
				return errors.NewError("argument to loads must be STRING")
			}
			str, _ := args[0].AsString()
			var data interface{}
			err := json.Unmarshal([]byte(str), &data)
			if err != nil {
				return errors.NewError("json loads error: %s", err.Error())
			}
			return jsonToObject(data)
		},
		HelpText: `loads(json_string) - Parse JSON string

Parses a JSON string and returns the corresponding Scriptling object.`,
	},
	"dumps": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewError("wrong number of arguments. got=%d, want=1", len(args))
			}
			data := objectToJSON(args[0])
			bytes, err := json.Marshal(data)
			if err != nil {
				return errors.NewError("json dumps error: %s", err.Error())
			}
			return &object.String{Value: string(bytes)}
		},
		HelpText: `dumps(obj) - Serialize object to JSON string

Converts a Scriptling object to its JSON string representation.`,
	},
}, nil, "JSON encoding and decoding library")

func jsonToObject(data interface{}) object.Object {
	switch v := data.(type) {
	case float64:
		if v == float64(int64(v)) {
			return &object.Integer{Value: int64(v)}
		}
		return &object.Float{Value: v}
	case string:
		return &object.String{Value: v}
	case bool:
		return &object.Boolean{Value: v}
	case []interface{}:
		elements := make([]object.Object, len(v))
		for i, el := range v {
			elements[i] = jsonToObject(el)
		}
		return &object.List{Elements: elements}
	case map[string]interface{}:
		pairs := make(map[string]object.DictPair)
		for key, val := range v {
			keyObj := &object.String{Value: key}
			valObj := jsonToObject(val)
			pairs[key] = object.DictPair{Key: keyObj, Value: valObj}
		}
		return &object.Dict{Pairs: pairs}
	case nil:
		return &object.Null{}
	default:
		return &object.Null{}
	}
}

func objectToJSON(obj object.Object) interface{} {
	switch obj := obj.(type) {
	case *object.Integer:
		return obj.Value
	case *object.Float:
		return obj.Value
	case *object.String:
		return obj.Value
	case *object.Boolean:
		return obj.Value
	case *object.List:
		arr := make([]interface{}, len(obj.Elements))
		for i, el := range obj.Elements {
			arr[i] = objectToJSON(el)
		}
		return arr
	case *object.Dict:
		m := make(map[string]interface{})
		for key, pair := range obj.Pairs {
			m[key] = objectToJSON(pair.Value)
		}
		return m
	case *object.Null:
		return nil
	default:
		return obj.Inspect()
	}
}
