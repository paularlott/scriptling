package evaluator

import (
	"context"
	"encoding/base64"
	"encoding/hex"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// bytesFromListInternal builds a Bytes value from a list of integers (each
// 0-255), mirroring Python's bytes([104, 105]) constructor.
func bytesFromListInternal(list *object.List) (*object.Bytes, object.Object) {
	out := make([]byte, 0, len(list.Elements))
	for i, el := range list.Elements {
		iv, ok := el.(*object.Integer)
		if !ok {
			return nil, errors.NewError("bytes: element %d is not an integer", i)
		}
		v := iv.IntValue()
		if v < 0 || v > 255 {
			return nil, errors.NewError("bytes: element %d out of range (0-255): %d", i, v)
		}
		out = append(out, byte(v))
	}
	return object.NewBytes(out), nil
}

// bytesConstructorFn implements bytes(source, encoding="utf-8"). It is the
// primary Python-compatible entry point and is registered as a global builtin.
func bytesConstructorFn(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	if len(args) == 0 {
		return object.NewBytes(nil)
	}
	if len(args) > 2 {
		return errors.NewError("bytes() takes at most 2 arguments (%d given)", len(args))
	}
	switch v := args[0].(type) {
	case *object.String:
		encoding := "utf-8"
		if len(args) == 2 {
			if s, err := args[1].AsString(); err == nil {
				encoding = s
			}
		}
		if enc := kwargs.MustGetString("encoding", encoding); enc != "utf-8" && enc != "utf8" {
			return errors.NewError("bytes: unsupported encoding %q (only utf-8 is supported)", enc)
		}
		return object.NewBytesFromString(v.StringValue())
	case *object.Bytes:
		return object.NewBytes(v.BytesValue())
	case *object.List:
		out, errObj := bytesFromListInternal(v)
		if errObj != nil {
			return errObj
		}
		return out
	case *object.Tuple:
		out, errObj := bytesFromListInternal(&object.List{Elements: v.Elements})
		if errObj != nil {
			return errObj
		}
		return out
	case *object.Null:
		return object.NewBytes(nil)
	default:
		return errors.NewTypeError("STRING, LIST, TUPLE, or BYTES", args[0].Type().String())
	}
}

// BytesBuiltin is the global bytes builtin. Calling bytes(...) invokes the
// constructor; bytes.fromhex(...) and bytes.frombase64(...) go through the
// Attributes map.
var BytesBuiltin = &object.Builtin{
	Fn: bytesConstructorFn,
	HelpText: `bytes(source="", encoding="utf-8") - Construct a Bytes value

Returns an immutable Bytes object holding the given binary data. Bytes is the
canonical binary type in Scriptling and is produced by hashlib.digest(),
hmac.digest(), base64.b64decode(), and msgpack.packb().

Parameters:
  source:  A string (encoded as UTF-8), a list/tuple of ints (0-255),
           an existing Bytes value, or nothing (empty bytes).
  encoding (str, optional): Encoding to use when source is a string.
           Only "utf-8" is supported. Default: "utf-8".

Returns:
  bytes: a Bytes object.

Example:
  b = bytes("hi")          # b'hi'
  b = bytes([104, 105])    # b'hi'
  b = bytes()              # b''
  s = b.decode()           # "hi"
  b = bytes.fromhex("6869") # b'hi'`,
	Attributes: map[string]object.Object{
		"fromhex": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}
				s, err := args[0].AsString()
				if err != nil {
					return err
				}
				decoded, decodeErr := hex.DecodeString(s)
				if decodeErr != nil {
					return errors.NewError("bytes.fromhex: %s", decodeErr.Error())
				}
				return object.NewBytes(decoded)
			},
			HelpText: `bytes.fromhex(hex_string) - Build Bytes from a hex string

Returns a Bytes value whose contents are the decoded hex.

Example:
  b = bytes.fromhex("6869")   # b'hi'`,
		},
		"frombase64": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}
				s, err := args[0].AsString()
				if err != nil {
					return err
				}
				decoded, decodeErr := base64.StdEncoding.DecodeString(s)
				if decodeErr != nil {
					return errors.NewError("bytes.frombase64: %s", decodeErr.Error())
				}
				return object.NewBytes(decoded)
			},
			HelpText: `bytes.frombase64(b64_string) - Build Bytes from a Base64 string

Returns a Bytes value whose contents are the decoded Base64 input.

Example:
  b = bytes.frombase64("aGk=")   # b'hi'`,
		},
	},
}
