package extlibs

import (
	"context"
	"fmt"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
	"gopkg.in/yaml.v3"
)

// YAMLLibrary provides YAML parsing and generation functionality
var YAMLLibrary = object.NewLibrary(YAMLLibraryName, map[string]*object.Builtin{
	"load": {
		Fn: yamlLoadFunc,
		HelpText: `load(yaml_string) - Parse YAML string (deprecated, use safe_load)

Parses a YAML string and returns the corresponding Scriptling object.
Alias for safe_load(). Both functions are identical and safe in Scriptling.

Note: In PyYAML, load() is deprecated. Use safe_load() instead.

Example:
    import yaml
    data = yaml.safe_load("name: John\nage: 30")
    print(data["name"])`,
	},
	"safe_load": {
		Fn: yamlLoadFunc,
		HelpText: `safe_load(yaml_string) - Safely parse YAML string

Safely parses a YAML string and returns the corresponding Scriptling object.

Example:
    import yaml
    data = yaml.safe_load("name: John\nage: 30")
    print(data["name"])`,
	},
	"dump": {
		Fn: yamlDumpFunc,
		HelpText: `dump(obj) - Convert Scriptling object to YAML string (use safe_dump)

Converts a Scriptling object to a YAML string.
Alias for safe_dump(). Both functions are identical in Scriptling.

Example:
    import yaml
    data = {"name": "John", "age": 30}
    yaml_str = yaml.safe_dump(data)
    print(yaml_str)`,
	},
	"safe_dump": {
		Fn: yamlDumpFunc,
		HelpText: `safe_dump(obj) - Safely convert Scriptling object to YAML string

Safely converts a Scriptling object to a YAML string.

Example:
    import yaml
    data = {"name": "John", "age": 30}
    yaml_str = yaml.safe_dump(data)
    print(yaml_str)`,
	},
}, nil, "YAML parsing and generation")

func yamlLoadFunc(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	if err := errors.ExactArgs(args, 1); err != nil {
		return err
	}

	yamlStr, err := args[0].AsString()
	if err != nil {
		return err
	}

	var data interface{}
	if parseErr := yaml.Unmarshal([]byte(yamlStr), &data); parseErr != nil {
		return errors.NewError("yaml parse error: %s", parseErr.Error())
	}

	return convertYAMLToScriptling(data)
}

func yamlDumpFunc(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	if err := errors.ExactArgs(args, 1); err != nil {
		return err
	}

	goData := convertScriptlingToGo(args[0])

	yamlBytes, marshalErr := yaml.Marshal(goData)
	if marshalErr != nil {
		return errors.NewError("yaml dump error: %s", marshalErr.Error())
	}

	return &object.String{Value: string(yamlBytes)}
}

// convertYAMLToScriptling converts Go types from YAML to Scriptling objects
func convertYAMLToScriptling(data interface{}) object.Object {
	switch v := data.(type) {
	case nil:
		return &object.Null{}
	case bool:
		return &object.Boolean{Value: v}
	case int:
		return object.NewInteger(int64(v))
	case int64:
		return object.NewInteger(v)
	case float64:
		return &object.Float{Value: v}
	case string:
		return &object.String{Value: v}
	case []interface{}:
		elements := make([]object.Object, len(v))
		for i, item := range v {
			elements[i] = convertYAMLToScriptling(item)
		}
		return &object.List{Elements: elements}
	case map[string]interface{}:
		pairs := make(map[string]object.DictPair, len(v))
		for key, val := range v {
			keyObj := &object.String{Value: key}
			valObj := convertYAMLToScriptling(val)
			pairs[key] = object.DictPair{Key: keyObj, Value: valObj}
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
			keyObj := &object.String{Value: keyStr}
			valObj := convertYAMLToScriptling(val)
			pairs[keyStr] = object.DictPair{Key: keyObj, Value: valObj}
		}
		return &object.Dict{Pairs: pairs}
	default:
		return &object.String{Value: fmt.Sprintf("%v", v)}
	}
}

// convertScriptlingToGo converts Scriptling objects to Go types for YAML
func convertScriptlingToGo(obj object.Object) interface{} {
	switch v := obj.(type) {
	case *object.Null:
		return nil
	case *object.Boolean:
		return v.Value
	case *object.Integer:
		return v.Value
	case *object.Float:
		return v.Value
	case *object.String:
		return v.Value
	case *object.List:
		result := make([]interface{}, len(v.Elements))
		for i, elem := range v.Elements {
			result[i] = convertScriptlingToGo(elem)
		}
		return result
	case *object.Dict:
		result := make(map[string]interface{}, len(v.Pairs))
		for key, pair := range v.Pairs {
			result[key] = convertScriptlingToGo(pair.Value)
		}
		return result
	default:
		return v.Inspect()
	}
}
