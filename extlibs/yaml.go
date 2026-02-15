package extlibs

import (
	"context"

	"github.com/paularlott/scriptling/conversion"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
	"gopkg.in/yaml.v3"
)

func RegisterYAMLLibrary(registrar interface{ RegisterLibrary(*object.Library) }) {
	registrar.RegisterLibrary(YAMLLibrary)
}

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

	return conversion.FromGo(data)
}

func yamlDumpFunc(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	if err := errors.ExactArgs(args, 1); err != nil {
		return err
	}

	goData := conversion.ToGo(args[0])

	yamlBytes, marshalErr := yaml.Marshal(goData)
	if marshalErr != nil {
		return errors.NewError("yaml dump error: %s", marshalErr.Error())
	}

	return &object.String{Value: string(yamlBytes)}
}
