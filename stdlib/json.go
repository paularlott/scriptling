package stdlib

import (
	"context"
	"encoding/json"

	"github.com/paularlott/scriptling/conversion"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

func jsonLoads(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	if len(args) != 1 {
		return errors.NewError("wrong number of arguments. got=%d, want=1", len(args))
	}
	if args[0].Type() != object.STRING_OBJ {
		return errors.NewError("argument to loads/parse must be STRING")
	}
	str, _ := args[0].AsString()
	return conversion.MustParseJSON(str)
}

func jsonDumps(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	if len(args) != 1 {
		return errors.NewError("wrong number of arguments. got=%d, want=1", len(args))
	}
	
	// Use kwargs helpers for optional parameters
	indent, _ := kwargs.GetString("indent", "")
	
	data := objectToJSON(args[0])
	
	var bytes []byte
	var err error
	
	if indent != "" {
		bytes, err = json.MarshalIndent(data, "", indent)
	} else {
		bytes, err = json.Marshal(data)
	}
	
	if err != nil {
		return errors.NewError("json serialize error: %s", err.Error())
	}
	return &object.String{Value: string(bytes)}
}

var JSONLibrary = object.NewLibrary(JSONLibraryName, map[string]*object.Builtin{
	"loads": {
		Fn: jsonLoads,
		HelpText: `loads(json_string) - Parse JSON string

Parses a JSON string and returns the corresponding Scriptling object.`,
	},
	"dumps": {
		Fn: jsonDumps,
		HelpText: `dumps(obj, indent="") - Serialize object to JSON string

Converts a Scriptling object to its JSON string representation.
Optional indent parameter for pretty-printing.`,
	},
	"parse": {
		Fn: jsonLoads,
		HelpText: `parse(json_string) - Parse JSON string (alias for loads)

Parses a JSON string and returns the corresponding Scriptling object.`,
	},
	"stringify": {
		Fn: jsonDumps,
		HelpText: `stringify(obj, indent="") - Serialize object to JSON string (alias for dumps)

Converts a Scriptling object to its JSON string representation.
Optional indent parameter for pretty-printing.`,
	},
}, nil, "JSON encoding and decoding library")

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
