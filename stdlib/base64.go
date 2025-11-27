package stdlib

import (
	"context"
	"encoding/base64"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

var base64Library = object.NewLibrary(map[string]*object.Builtin{
	"encode": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			str, ok := args[0].(*object.String)
			if !ok {
				return errors.NewTypeError("STRING", args[0].Type().String())
			}
			encoded := base64.StdEncoding.EncodeToString([]byte(str.Value))
			return &object.String{Value: encoded}
		},
		HelpText: `encode(string) - Encode string to base64

Returns the base64 encoded version of the input string.`,
	},
	"decode": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			str, ok := args[0].(*object.String)
			if !ok {
				return errors.NewTypeError("STRING", args[0].Type().String())
			}
			decoded, err := base64.StdEncoding.DecodeString(str.Value)
			if err != nil {
				return errors.NewError("decode() invalid base64 string")
			}
			return &object.String{Value: string(decoded)}
		},
		HelpText: `decode(base64_string) - Decode base64 string

Returns the decoded string from a base64 encoded input.`,
	},
})

func GetBase64Library() *object.Library {
	return object.NewLibraryWithDescription(base64Library.Functions(), "Base64 encoding and decoding library")
}
