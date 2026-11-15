// Package toml provides the toml extension library.
package toml

import (
	"bytes"
	"context"
	stderrors "errors"

	"github.com/BurntSushi/toml"
	"github.com/paularlott/scriptling/conversion"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/tomllimit"
)

// LibraryName is the script-visible name of the toml library.
const LibraryName = "toml"

// Register registers the toml library.
func Register(registrar interface{ RegisterLibrary(*object.Library) }) {
	registrar.RegisterLibrary(Library)
}

// Library provides TOML parsing and generation functionality
var Library = object.NewLibrary(LibraryName, map[string]*object.Builtin{
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

	if depthErr := tomllimit.Check(tomlStr); depthErr != nil {
		return errors.NewError("toml parse error: %s", depthErr.Error())
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

	goData, depthErr := conversion.ToGoChecked(args[0])
	if depthErr != nil {
		return depthErr
	}

	// The conversion is held to the limits, but a string shared by several
	// tables encodes in full each time: the encoder writes into a buffer
	// that refuses to grow past them.
	w := &limitedBuffer{}
	if marshalErr := toml.NewEncoder(w).Encode(goData); marshalErr != nil {
		if w.err != nil {
			return w.err
		}
		return errors.NewError("toml dump error: %s", marshalErr.Error())
	}

	return object.NewString(w.buf.String())
}

// limitedBuffer collects encoder output, failing a write that would take it
// past the value limit or the memory left.
type limitedBuffer struct {
	buf bytes.Buffer
	err object.Object
}

func (w *limitedBuffer) Write(p []byte) (int, error) {
	if errObj := object.CheckAllocFor("TOML", int64(w.buf.Len())+int64(len(p))); errObj != nil {
		w.err = errObj
		return 0, stderrors.New("output too large")
	}
	return w.buf.Write(p)
}
