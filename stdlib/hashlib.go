package stdlib

import (
	"context"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

var hashlibLibrary = object.NewLibrary(map[string]*object.Builtin{
	"sha256": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			str, ok := args[0].(*object.String)
			if !ok {
				return errors.NewTypeError("STRING", string(args[0].Type()))
			}
			hash := sha256.Sum256([]byte(str.Value))
			return &object.String{Value: hex.EncodeToString(hash[:])}
		},
	},
	"sha1": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			str, ok := args[0].(*object.String)
			if !ok {
				return errors.NewTypeError("STRING", string(args[0].Type()))
			}
			hash := sha1.Sum([]byte(str.Value))
			return &object.String{Value: hex.EncodeToString(hash[:])}
		},
	},
	"md5": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			str, ok := args[0].(*object.String)
			if !ok {
				return errors.NewTypeError("STRING", string(args[0].Type()))
			}
			hash := md5.Sum([]byte(str.Value))
			return &object.String{Value: hex.EncodeToString(hash[:])}
		},
	},
})

func GetHashlibLibrary() *object.Library {
	return hashlibLibrary
}
