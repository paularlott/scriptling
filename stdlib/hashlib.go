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

var HashlibLibrary = object.NewLibrary(HashlibLibraryName, map[string]*object.Builtin{
	"sha256": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 1); err != nil { return err }
			str, err := args[0].AsString()
			if err != nil {
				return err
			}
			hash := sha256.Sum256([]byte(str))
			return &object.String{Value: hex.EncodeToString(hash[:])}
		},
		HelpText: `sha256(string) - Compute SHA-256 hash

Returns the SHA-256 hash of the input string as a hexadecimal string.`,
	},
	"sha1": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 1); err != nil { return err }
			str, err := args[0].AsString()
			if err != nil {
				return err
			}
			hash := sha1.Sum([]byte(str))
			return &object.String{Value: hex.EncodeToString(hash[:])}
		},
		HelpText: `sha1(string) - Compute SHA-1 hash

Returns the SHA-1 hash of the input string as a hexadecimal string.`,
	},
	"md5": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 1); err != nil { return err }
			str, err := args[0].AsString()
			if err != nil {
				return err
			}
			hash := md5.Sum([]byte(str))
			return &object.String{Value: hex.EncodeToString(hash[:])}
		},
		HelpText: `md5(string) - Compute MD5 hash

Returns a hexadecimal string.`,
	},
}, nil, "Cryptographic hash functions library")
