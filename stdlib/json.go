package stdlib

import (
	"encoding/json"
	"fmt"
	"github.com/paularlott/scriptling/object"
)

func JSONLibrary() map[string]*object.Builtin {
	return map[string]*object.Builtin{
		"parse": {
			Fn: func(args ...object.Object) object.Object {
				if len(args) != 1 {
					return newError("wrong number of arguments. got=%d, want=1", len(args))
				}
				if args[0].Type() != object.STRING_OBJ {
					return newError("argument to parse must be STRING")
				}
				str := args[0].(*object.String).Value
				var data interface{}
				err := json.Unmarshal([]byte(str), &data)
				if err != nil {
					return newError("json parse error: %s", err.Error())
				}
				return jsonToObject(data)
			},
		},
		"stringify": {
			Fn: func(args ...object.Object) object.Object {
				if len(args) != 1 {
					return newError("wrong number of arguments. got=%d, want=1", len(args))
				}
				data := objectToJSON(args[0])
				bytes, err := json.Marshal(data)
				if err != nil {
					return newError("json stringify error: %s", err.Error())
				}
				return &object.String{Value: string(bytes)}
			},
		},
	}
}

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

func newError(format string, a ...interface{}) *object.Error {
	return &object.Error{Message: fmt.Sprintf(format, a...)}
}
