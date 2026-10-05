package stdlib

import (
	"context"
	"encoding/base64"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

var Base64Library = object.NewLibrary(Base64LibraryName, map[string]*object.Builtin{
	"b64encode": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			// Accept Bytes (preferred) or String (encoded as UTF-8) as input.
			data, errObj := coerceToBytes(args[0])
			if errObj != nil {
				return errObj
			}
			// Python's base64.b64encode returns bytes.
			encoded := base64.StdEncoding.EncodeToString(data)
			return object.NewBytesFromString(encoded)
		},
		HelpText: `b64encode(s) - Encode bytes-like object to Base64

Accepts a Bytes value or a String (UTF-8 encoded) and returns the Base64
representation as a string.`,
	},
	"b64decode": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			var encoded string
			switch v := args[0].(type) {
			case *object.String:
				encoded = v.StringValue()
			case *object.Bytes:
				encoded = string(v.BytesValue())
			default:
				return errors.NewTypeError("STRING or BYTES", args[0].Type().String())
			}
			decoded, decodeErr := base64.StdEncoding.DecodeString(encoded)
			if decodeErr != nil {
				return errors.NewError("base64 decode error: %s", decodeErr.Error())
			}
			return object.NewBytes(decoded)
		},
		HelpText: `b64decode(s) - Decode a Base64 encoded string

Returns the decoded bytes as a Bytes value. Use .decode() on the result to
obtain a string when the underlying data is text.`,
	},
}, nil, "Base64 encoding and decoding library")
