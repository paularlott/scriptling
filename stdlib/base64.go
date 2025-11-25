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
				return errors.NewTypeError("STRING", string(args[0].Type()))
			}
			encoded := base64.StdEncoding.EncodeToString([]byte(str.Value))
			return &object.String{Value: encoded}
		},
	},
	"decode": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			str, ok := args[0].(*object.String)
			if !ok {
				return errors.NewTypeError("STRING", string(args[0].Type()))
			}
			decoded, err := base64.StdEncoding.DecodeString(str.Value)
			if err != nil {
				return errors.NewError("decode() invalid base64 string")
			}
			return &object.String{Value: string(decoded)}
		},
	},
})

func GetBase64Library() *object.Library {
	return base64Library
}
