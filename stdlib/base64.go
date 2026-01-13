package stdlib

import (
	"context"
	"encoding/base64"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

var Base64Library = object.NewLibrary(map[string]*object.Builtin{
	"b64encode": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			str, err := args[0].AsString()
			if err != nil {
				return err
			}
			encoded := base64.StdEncoding.EncodeToString([]byte(str))
			return &object.String{Value: encoded}
		},
		HelpText: `b64encode(s) - Encode bytes-like object to Base64

Returns a Base64-encoded version of the input string.`,
	},
	"b64decode": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			str, err := args[0].AsString()
			if err != nil {
				return err
			}
			decoded, decodeErr := base64.StdEncoding.DecodeString(str)
			if decodeErr != nil {
				return errors.NewError("base64 decode error: %s", decodeErr.Error())
			}
			return &object.String{Value: string(decoded)}
		},
		HelpText: `b64decode(s) - Decode a Base64 encoded string

Returns the decoded string from a Base64-encoded input.`,
	},
}, nil, "Base64 encoding and decoding library")
