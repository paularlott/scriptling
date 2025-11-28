package stdlib

import (
	"context"
	"encoding/base64"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

var Base64Library = object.NewLibrary(map[string]*object.Builtin{
	"b64encode": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			str, ok := args[0].AsString()
			if !ok {
				return errors.NewTypeError("STRING", args[0].Type().String())
			}
			encoded := base64.StdEncoding.EncodeToString([]byte(str))
			return &object.String{Value: encoded}
		},
		HelpText: `b64encode(s) - Encode bytes-like object to Base64

Returns a Base64-encoded version of the input string.`,
	},
	"b64decode": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			str, ok := args[0].AsString()
			if !ok {
				return errors.NewTypeError("STRING", args[0].Type().String())
			}
			decoded, err := base64.StdEncoding.DecodeString(str)
			if err != nil {
				return errors.NewError("base64 decode error: %s", err.Error())
			}
			return &object.String{Value: string(decoded)}
		},
		HelpText: `b64decode(s) - Decode a Base64 encoded string

Returns the decoded string from a Base64-encoded input.`,
	},
}, nil, "Base64 encoding and decoding library")
