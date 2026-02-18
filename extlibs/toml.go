package extlibs

import (
	"context"

	"github.com/BurntSushi/toml"
	"github.com/paularlott/scriptling/conversion"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

func RegisterTOMLLibrary(registrar interface{ RegisterLibrary(*object.Library) }) {
	registrar.RegisterLibrary(TOMLLibrary)
}

// TOMLLibrary provides TOML parsing and generation functionality
var TOMLLibrary = object.NewLibrary(TOMLLibraryName, map[string]*object.Builtin{
	"loads": {
		Fn: tomlLoadsFunc,
		HelpText: `loads(toml_string) - Parse TOML string

Parses a TOML string and returns the corresponding Scriptling object.

This function is compatible with Python's tomllib.loads() from Python 3.11+.

Example:
    import toml
    data = toml.loads("[database]\nhost = \"localhost\"\nport = 5432")
    print(data["database"]["host"])`,
	},
	"dumps": {
		Fn: tomlDumpsFunc,
		HelpText: `dumps(obj) - Convert Scriptling object to TOML string

Converts a Scriptling object to a TOML formatted string.

Note: Python's tomllib does not include a write function. This follows
the convention of the tomli-w library which provides dumps().

Example:
    import toml
    data = {"database": {"host": "localhost", "port": 5432}}
    toml_str = toml.dumps(data)
    print(toml_str)`,
	},
}, nil, "TOML parsing and generation")

func tomlLoadsFunc(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	if err := errors.ExactArgs(args, 1); err != nil {
		return err
	}

	tomlStr, err := args[0].AsString()
	if err != nil {
		return err
	}

	var data interface{}
	if _, parseErr := toml.Decode(tomlStr, &data); parseErr != nil {
		return errors.NewError("toml parse error: %s", parseErr.Error())
	}

	return conversion.FromGo(data)
}

func tomlDumpsFunc(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	if err := errors.ExactArgs(args, 1); err != nil {
		return err
	}

	goData := conversion.ToGo(args[0])

	var buf []byte
	var marshalErr error
	buf, marshalErr = toml.Marshal(goData)
	if marshalErr != nil {
		return errors.NewError("toml dump error: %s", marshalErr.Error())
	}

	return &object.String{Value: string(buf)}
}
