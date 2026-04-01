package shared

import (
	b64 "encoding/base64"
	"fmt"

	"github.com/paularlott/scriptling/object"
)

// ObjectToNative recursively converts a Scriptling object to a plain Go value for JSON marshalling.
func ObjectToNative(v interface{}) interface{} {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case *object.Dict:
		m := make(map[string]interface{}, len(val.Pairs))
		for _, pair := range val.Pairs {
			m[pair.StringKey()] = ObjectToNative(pair.Value)
		}
		return m
	case *object.List:
		s := make([]interface{}, len(val.Elements))
		for i, el := range val.Elements {
			s[i] = ObjectToNative(el)
		}
		return s
	case *object.Tuple:
		s := make([]interface{}, len(val.Elements))
		for i, el := range val.Elements {
			s[i] = ObjectToNative(el)
		}
		return s
	case *object.String:
		return val.Value
	case *object.Integer:
		return val.Value
	case *object.Float:
		return val.Value
	case *object.Boolean:
		return val.Value
	case *object.Null:
		return nil
	default:
		return nil
	}
}

// ConvertToObject recursively converts a plain Go value to a Scriptling object.
func ConvertToObject(v interface{}) object.Object {
	if v == nil {
		return &object.Null{}
	}
	switch val := v.(type) {
	case map[string]interface{}:
		d := &object.Dict{Pairs: make(map[string]object.DictPair)}
		for k, item := range val {
			d.SetByString(k, ConvertToObject(item))
		}
		return d
	case []interface{}:
		elems := make([]object.Object, len(val))
		for i, item := range val {
			elems[i] = ConvertToObject(item)
		}
		return &object.List{Elements: elems}
	case string:
		return &object.String{Value: val}
	case float64:
		if val == float64(int64(val)) {
			return object.NewInteger(int64(val))
		}
		return &object.Float{Value: val}
	case bool:
		return object.NewBoolean(val)
	default:
		return &object.String{Value: fmt.Sprintf("%v", val)}
	}
}

// EncodeBase64 encodes bytes to a base64 string.
func EncodeBase64(data []byte) string {
	return b64.StdEncoding.EncodeToString(data)
}
