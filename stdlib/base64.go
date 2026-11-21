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
			if err := errors.NoKwargs("b64encode", kwargs); err != nil {
				return err
			}
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			// Accept Bytes (preferred) or String (encoded as UTF-8) as input.
			data, errObj := coerceToBytes(args[0])
			if errObj != nil {
				return errObj
			}
			// Python's base64.b64encode returns bytes. Check the result
			// before encoding rather than after.
			if errObj := object.CheckAllocFor("Base64 result", int64(base64.StdEncoding.EncodedLen(len(data)))); errObj != nil {
				return errObj
			}
			encoded := base64.StdEncoding.EncodeToString(data)
			return object.NewBytesFromString(encoded)
		},
		HelpText: `b64encode(s) - Encode bytes-like object to Base64

Accepts a Bytes value or a String (UTF-8 encoded) and returns the Base64
representation as a string.`,
	},
	"b64decode": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			return b64DecodeCall("b64decode", false, kwargs, args)
		},
		HelpText: `b64decode(s, validate=False) - Decode a Base64 encoded string

Returns the decoded bytes as a Bytes value. Use .decode() on the result to
obtain a string when the underlying data is text. As in Python, characters
outside the Base64 alphabet are discarded unless validate=True, which
rejects them; bad padding raises ValueError (binascii.Error in Python).`,
	},
	"urlsafe_b64encode": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("urlsafe_b64encode", kwargs); err != nil {
				return err
			}
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			data, errObj := coerceToBytes(args[0])
			if errObj != nil {
				return errObj
			}
			if errObj := object.CheckAllocFor("Base64 result", int64(base64.URLEncoding.EncodedLen(len(data)))); errObj != nil {
				return errObj
			}
			return object.NewBytesFromString(base64.URLEncoding.EncodeToString(data))
		},
		HelpText: `urlsafe_b64encode(s) - Encode to Base64 with - and _ in place of + and /`,
	},
	"urlsafe_b64decode": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			return b64DecodeCall("urlsafe_b64decode", true, kwargs, args)
		},
		HelpText: `urlsafe_b64decode(s) - Decode Base64 that uses - and _ in place of + and /`,
	},
}, nil, "Base64 encoding and decoding library")

// b64DecodeCall implements b64decode and urlsafe_b64decode.
func b64DecodeCall(name string, urlsafe bool, kwargs object.Kwargs, args []object.Object) object.Object {
	validate := false
	if kwargs.Len() > 0 {
		allowed := []string{"validate"}
		if urlsafe {
			allowed = nil
		}
		if err := errors.CheckKwargs(name, kwargs, allowed...); err != nil {
			return err
		}
		if v := kwargs.Get("validate"); v != nil {
			b, err := v.AsBool()
			if err != nil {
				return err
			}
			validate = b
		}
	}
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
	decoded, errObj := b64Decode(encoded, urlsafe, validate)
	if errObj != nil {
		return errObj
	}
	return object.NewBytes(decoded)
}

// b64Decode decodes Base64 as Python's binascii.a2b_base64 does: without
// validate, characters outside the alphabet are discarded and padding may be
// excess; with it, they are an error. Either way missing padding is an
// error. Errors are ValueError, the parent of Python's binascii.Error.
func b64Decode(s string, urlsafe, validate bool) ([]byte, object.Object) {
	c62, c63 := byte('+'), byte('/')
	enc := base64.RawStdEncoding
	if urlsafe {
		c62, c63 = '-', '_'
		enc = base64.RawURLEncoding
	}
	data := make([]byte, 0, len(s))
	pads := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case 'A' <= c && c <= 'Z', 'a' <= c && c <= 'z', '0' <= c && c <= '9', c == c62, c == c63,
			urlsafe && (c == '+' || c == '/'):
			// Python's urlsafe decoder accepts the standard alphabet too.
			if c == '+' {
				c = c62
			} else if c == '/' {
				c = c63
			}
			if validate && pads > 0 {
				return nil, errors.NewValueError("Excess data after padding")
			}
			data = append(data, c)
		case c == '=':
			if validate && len(data) == 0 {
				return nil, errors.NewValueError("Leading padding not allowed")
			}
			pads++
		default:
			if validate {
				return nil, errors.NewValueError("Only base64 data is allowed")
			}
		}
	}
	if len(data)%4 == 1 {
		return nil, errors.NewValueError("Invalid base64-encoded string: number of data characters (%d) cannot be 1 more than a multiple of 4", len(data))
	}
	if rem := len(data) % 4; rem != 0 && pads < 4-rem {
		return nil, errors.NewValueError("Incorrect padding")
	}
	if validate {
		if rem := len(data) % 4; (rem == 0 && pads > 0) || (rem != 0 && pads > 4-rem) {
			return nil, errors.NewValueError("Excess padding not allowed")
		}
	}
	out, err := enc.DecodeString(string(data))
	if err != nil {
		return nil, errors.NewValueError("Invalid base64-encoded string: %s", err.Error())
	}
	return out, nil
}
