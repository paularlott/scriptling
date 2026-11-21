package evaluator

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"strings"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// bytesFromListInternal builds a Bytes value from a list of integers (each
// 0-255), mirroring Python's bytes([104, 105]) constructor.
func bytesFromListInternal(list *object.List) (*object.Bytes, object.Object) {
	out := make([]byte, 0, len(list.Elements))
	for _, el := range list.Elements {
		var v int64
		switch n := el.(type) {
		case *object.Integer:
			v = n.IntValue()
		case *object.Boolean:
			v = boolToInt64(n.BoolValue())
		default:
			return nil, errors.NewTypeErrorTagged("'%s' object cannot be interpreted as an integer", getTypeName(el))
		}
		if v < 0 || v > 255 {
			return nil, errors.NewValueError("bytes must be in range(0, 256)")
		}
		out = append(out, byte(v))
	}
	return object.NewBytes(out), nil
}

// bytesConstructorFn implements bytes(source, encoding="utf-8"). It is the
// primary Python-compatible entry point and is registered as a global builtin.
func bytesConstructorFn(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	if kwargs.Len() > 0 {
		// errors applies when a str is encoded with a codec other than
		// utf-8 (encoding a str to utf-8 cannot fail).
		if err := errors.CheckKwargs("bytes", kwargs, "encoding", "errors"); err != nil {
			return err
		}
	}
	if len(args) == 0 {
		return object.NewBytes(nil)
	}
	if len(args) > 2 {
		return errors.NewTypeErrorTagged("bytes() takes at most 2 arguments (%d given)", len(args))
	}
	switch v := boolAsInt(args[0]).(type) {
	case *object.Integer:
		// Python bytes(n) is n zero bytes.
		n := v.IntValue()
		if n < 0 {
			return errors.NewValueError("negative count")
		}
		if errObj := checkSize("bytes", n); errObj != nil {
			return errObj
		}
		return object.NewBytes(make([]byte, n))
	case *object.String:
		encoding := "utf-8"
		if len(args) == 2 {
			if s, err := args[1].AsString(); err == nil {
				encoding = s
			}
		}
		enc := kwargs.MustGetString("encoding", encoding)
		if enc == "utf-8" || enc == "utf8" {
			return object.NewBytesFromString(v.StringValue())
		}
		errorsMode := "strict"
		if em, ok := kwargs.Lookup("errors"); ok {
			if m, err := em.AsString(); err == nil {
				errorsMode = m
			}
		}
		out, errObj := encodeStr(v.StringValue(), enc, errorsMode)
		if errObj != nil {
			return errObj
		}
		return object.NewBytes(out)
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
		// Any other iterable of ints: a generator, range, iterator, ...
		elems, ok, rerr := iterableToSliceCheckedFn(ctx, args[0], GetEnvFromContext(ctx))
		if rerr != nil {
			return rerr
		}
		if ok {
			out, errObj := bytesFromListInternal(&object.List{Elements: elems})
			if errObj != nil {
				return errObj
			}
			return out
		}
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
				if err := errors.NoKwargs("fromhex", kwargs); err != nil {
					return err
				}
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}
				s, err := args[0].AsString()
				if err != nil {
					return err
				}
				// Whitespace between the byte pairs is allowed, as in
				// Python: bytes.fromhex('de ad be ef').
				s = strings.Map(func(r rune) rune {
					if r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\v' || r == '\f' {
						return -1
					}
					return r
				}, s)
				decoded, decodeErr := hex.DecodeString(s)
				if decodeErr != nil {
					return errors.NewValueError("bytes.fromhex: %s", decodeErr.Error())
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
				if err := errors.NoKwargs("frombase64", kwargs); err != nil {
					return err
				}
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}
				s, err := args[0].AsString()
				if err != nil {
					return err
				}
				decoded, decodeErr := base64.StdEncoding.DecodeString(s)
				if decodeErr != nil {
					return errors.NewValueError("bytes.frombase64: %s", decodeErr.Error())
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
