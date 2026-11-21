package stdlib

import (
	"context"
	"encoding/base32"
	"encoding/base64"
	"encoding/hex"
	"strings"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// The rest of Python's commonly used base64 functions: standard_b64encode
// and standard_b64decode, b64encode's altchars, encodebytes and
// decodebytes (MIME lines of 76 characters), and the Base32 and Base16
// codecs.

func base64Data(name string, args []object.Object) ([]byte, object.Object) {
	if err := errors.ExactArgs(args, 1); err != nil {
		return nil, err
	}
	switch v := args[0].(type) {
	case *object.Bytes:
		return v.BytesValue(), nil
	case *object.String:
		return []byte(v.StringValue()), nil
	}
	return nil, errors.NewTypeErrorTagged("%s() argument must be bytes-like or str, not '%s'", name, object.PythonTypeName(args[0]))
}

func init() {
	fns := Base64Library.Functions()

	plainB64 := fns["b64encode"]
	fns["b64encode"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if kwargs.Len() == 0 && len(args) == 1 {
				return plainB64.Fn(ctx, kwargs, args...)
			}
			if kwargs.Len() > 0 {
				if err := errors.CheckKwargs("b64encode", kwargs, "altchars"); err != nil {
					return err
				}
			}
			alt := kwargs.Get("altchars")
			if len(args) == 2 {
				alt = args[1]
				args = args[:1]
			}
			r := plainB64.Fn(ctx, object.NewKwargs(nil), args...)
			out, ok := r.(*object.Bytes)
			if !ok || alt == nil || alt.Type() == object.NULL_OBJ {
				return r
			}
			a, ok := alt.(*object.Bytes)
			if !ok || a.Len() != 2 {
				return errors.NewValueError("altchars must be a bytes-like object of length 2")
			}
			b := append([]byte(nil), out.BytesValue()...)
			for i, c := range b {
				switch c {
				case '+':
					b[i] = a.BytesValue()[0]
				case '/':
					b[i] = a.BytesValue()[1]
				}
			}
			return object.NewBytes(b)
		},
		HelpText: plainB64.HelpText + "\n\naltchars (2 bytes) replaces + and /.",
	}
	fns["standard_b64encode"] = &object.Builtin{Fn: plainB64.Fn, HelpText: "standard_b64encode(s) - Encode to standard Base64"}
	fns["standard_b64decode"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("standard_b64decode", kwargs); err != nil {
				return err
			}
			return b64DecodeCall("standard_b64decode", false, kwargs, args)
		},
		HelpText: "standard_b64decode(s) - Decode standard Base64",
	}
	fns["encodebytes"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("encodebytes", kwargs); err != nil {
				return err
			}
			data, errObj := base64Data("encodebytes", args)
			if errObj != nil {
				return errObj
			}
			if errObj := object.CheckAllocFor("Base64 result", int64(base64.StdEncoding.EncodedLen(len(data)))*77/76+1); errObj != nil {
				return errObj
			}
			enc := base64.StdEncoding.EncodeToString(data)
			var b strings.Builder
			for len(enc) > 76 {
				b.WriteString(enc[:76])
				b.WriteByte('\n')
				enc = enc[76:]
			}
			if enc != "" {
				b.WriteString(enc)
				b.WriteByte('\n')
			}
			return object.NewBytesFromString(b.String())
		},
		HelpText: "encodebytes(s) - Base64 in lines of 76 characters, each ending in a newline",
	}
	fns["decodebytes"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("decodebytes", kwargs); err != nil {
				return err
			}
			return b64DecodeCall("decodebytes", false, kwargs, args)
		},
		HelpText: "decodebytes(s) - Decode Base64 that may span lines",
	}
	fns["b32encode"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("b32encode", kwargs); err != nil {
				return err
			}
			data, errObj := base64Data("b32encode", args)
			if errObj != nil {
				return errObj
			}
			if errObj := object.CheckAllocFor("Base32 result", int64(base32.StdEncoding.EncodedLen(len(data)))); errObj != nil {
				return errObj
			}
			return object.NewBytesFromString(base32.StdEncoding.EncodeToString(data))
		},
		HelpText: "b32encode(s) - Encode to Base32",
	}
	fns["b32decode"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			casefold := false
			if kwargs.Len() > 0 {
				if err := errors.CheckKwargs("b32decode", kwargs, "casefold"); err != nil {
					return err
				}
				casefold = b64Truthy(kwargs.Get("casefold"))
			}
			if len(args) == 2 {
				casefold = b64Truthy(args[1])
				args = args[:1]
			}
			data, errObj := base64Data("b32decode", args)
			if errObj != nil {
				return errObj
			}
			s := string(data)
			if casefold {
				s = strings.ToUpper(s)
			}
			out, err := base32.StdEncoding.DecodeString(s)
			if err != nil {
				return errors.NewValueError("Non-base32 digit found or incorrect padding")
			}
			return object.NewBytes(out)
		},
		HelpText: "b32decode(s, casefold=False) - Decode Base32",
	}
	fns["b16encode"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("b16encode", kwargs); err != nil {
				return err
			}
			data, errObj := base64Data("b16encode", args)
			if errObj != nil {
				return errObj
			}
			if errObj := object.CheckAllocFor("Base16 result", int64(len(data))*2); errObj != nil {
				return errObj
			}
			return object.NewBytesFromString(strings.ToUpper(hex.EncodeToString(data)))
		},
		HelpText: "b16encode(s) - Encode to Base16 (upper-case hex)",
	}
	fns["b16decode"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			casefold := false
			if kwargs.Len() > 0 {
				if err := errors.CheckKwargs("b16decode", kwargs, "casefold"); err != nil {
					return err
				}
				casefold = b64Truthy(kwargs.Get("casefold"))
			}
			if len(args) == 2 {
				casefold = b64Truthy(args[1])
				args = args[:1]
			}
			data, errObj := base64Data("b16decode", args)
			if errObj != nil {
				return errObj
			}
			s := string(data)
			if !casefold && strings.ToUpper(s) != s {
				return errors.NewValueError("Non-base16 digit found")
			}
			out, err := hex.DecodeString(s)
			if err != nil {
				return errors.NewValueError("Non-base16 digit found")
			}
			return object.NewBytes(out)
		},
		HelpText: "b16decode(s, casefold=False) - Decode Base16 (upper-case hex unless casefold)",
	}
}

func b64Truthy(o object.Object) bool {
	if o == nil {
		return false
	}
	b, err := o.AsBool()
	return err == nil && b
}
