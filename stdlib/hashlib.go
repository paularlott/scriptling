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

var HashlibLibrary = object.NewLibrary(map[string]*object.Builtin{
	"sha256": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			str, ok := args[0].AsString()
			if !ok {
				return errors.NewTypeError("STRING", args[0].Type().String())
			}
			hash := sha256.Sum256([]byte(str))
			return &object.String{Value: hex.EncodeToString(hash[:])}
		},
		HelpText: `sha256(string) - Compute SHA-256 hash

Returns the SHA-256 hash of the input string as a hexadecimal string.`,
	},
	"sha1": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			str, ok := args[0].AsString()
			if !ok {
				return errors.NewTypeError("STRING", args[0].Type().String())
			}
			hash := sha1.Sum([]byte(str))
			return &object.String{Value: hex.EncodeToString(hash[:])}
		},
		HelpText: `sha1(string) - Compute SHA-1 hash

Returns the SHA-1 hash of the input string as a hexadecimal string.`,
	},
	"md5": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			str, ok := args[0].AsString()
			if !ok {
				return errors.NewTypeError("STRING", args[0].Type().String())
			}
			hash := md5.Sum([]byte(str))
			return &object.String{Value: hex.EncodeToString(hash[:])}
		},
		HelpText: `md5(string) - Compute MD5 hash

Returns a hexadecimal string.`,
	},
}, nil, "Cryptographic hash functions library")
