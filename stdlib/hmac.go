package stdlib

import (
	"context"
	"crypto/hmac"
	"crypto/subtle"
	"encoding/hex"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// HmacLibraryName is declared in stdlib.go (alongside HashlibLibraryName).

// hmacState is the native state held by an HMAC instance.
type hmacState struct {
	key  []byte
	data []byte
	alg  string
}

// resolveHmacAlg maps a digestmod argument to an algorithm name. It accepts a
// string name ("sha256", "sha1", "md5"), one of the hashlib constructor
// builtins (e.g. hashlib.sha256), or None/omitted (defaults to sha256).
func resolveHmacAlg(digestmod object.Object) (string, object.Object) {
	if digestmod == nil {
		return "sha256", nil
	}
	if _, isNull := digestmod.(*object.Null); isNull {
		return "sha256", nil
	}
	if s, ok := digestmod.(*object.String); ok {
		alg := s.StringValue()
		if hashConstructor(alg) == nil {
			return "", errors.NewError("unsupported hash algorithm: %s", alg)
		}
		return alg, nil
	}
	// Accept hashlib constructors passed by reference. Library attribute access
	// returns the exact *object.Builtin pointer, so identity comparison works.
	switch digestmod {
	case HashlibSHA256Builtin:
		return "sha256", nil
	case HashlibSHA1Builtin:
		return "sha1", nil
	case HashlibMD5Builtin:
		return "md5", nil
	}
	return "", errors.NewError("unsupported digestmod: expected a hash algorithm name (e.g. \"sha256\") or hashlib constructor")
}

func computeHmacDigest(state *hmacState) []byte {
	h := hmac.New(hashConstructor(state.alg), state.key)
	h.Write(state.data)
	return h.Sum(nil)
}

// HMACClass is the class exposed for objects returned by hmac.new().
var HMACClass = &object.Class{
	Name: "HMAC",
	Methods: map[string]object.Object{
		"update": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 2); err != nil {
					return err
				}
				inst, ok := args[0].(*object.Instance)
				if !ok {
					return errors.NewTypeError("HMAC", args[0].Type().String())
				}
				state, ok := inst.NativeData.(*hmacState)
				if !ok {
					return errors.NewError("invalid hmac object")
				}
				b, errObj := toByteString(args[1])
				if errObj != nil {
					return errObj
				}
				state.data = append(state.data, b...)
				return &object.Null{}
			},
			HelpText: `update(data) - Feed data into the HMAC

Appends data (a string treated as bytes, or a list of byte values) to the
message being authenticated. Returns None.`,
		},
		"digest": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}
				inst, ok := args[0].(*object.Instance)
				if !ok {
					return errors.NewTypeError("HMAC", args[0].Type().String())
				}
				state, ok := inst.NativeData.(*hmacState)
				if !ok {
					return errors.NewError("invalid hmac object")
				}
				return object.NewString(string(computeHmacDigest(state)))
			},
			HelpText: `digest() - Return the raw HMAC as a byte string`,
		},
		"hexdigest": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}
				inst, ok := args[0].(*object.Instance)
				if !ok {
					return errors.NewTypeError("HMAC", args[0].Type().String())
				}
				state, ok := inst.NativeData.(*hmacState)
				if !ok {
					return errors.NewError("invalid hmac object")
				}
				return object.NewString(hex.EncodeToString(computeHmacDigest(state)))
			},
			HelpText: `hexdigest() - Return the HMAC as a hexadecimal string

Example:
  import hmac
  sig = hmac.new("secret", "payload", "sha256").hexdigest()`,
		},
		"copy": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}
				inst, ok := args[0].(*object.Instance)
				if !ok {
					return errors.NewTypeError("HMAC", args[0].Type().String())
				}
				state, ok := inst.NativeData.(*hmacState)
				if !ok {
					return errors.NewError("invalid hmac object")
				}
				newKey := make([]byte, len(state.key))
				copy(newKey, state.key)
				newData := make([]byte, len(state.data))
				copy(newData, state.data)
				return object.NewInstanceWithData(inst.Class, map[string]object.Object{
					"name":        object.NewString("hmac-" + state.alg),
					"digest_size": object.NewInteger(int64(hashDigestSize(state.alg))),
					"block_size":  object.NewInteger(int64(hashBlockSize(state.alg))),
				}, &hmacState{key: newKey, data: newData, alg: state.alg})
			},
			HelpText: `copy() - Return a copy of the HMAC object`,
		},
	},
}

// newHmacInstance builds an HMAC instance. It references the HMACClass package
// var, which is safe because HMACClass's initializer never references this
// function (so there is no initialization cycle).
func newHmacInstance(alg string, key, data []byte) *object.Instance {
	return object.NewInstanceWithData(HMACClass, map[string]object.Object{
		"name":        object.NewString("hmac-" + alg),
		"digest_size": object.NewInteger(int64(hashDigestSize(alg))),
		"block_size":  object.NewInteger(int64(hashBlockSize(alg))),
	}, &hmacState{key: key, data: data, alg: alg})
}

// CompareDigest performs a constant-time comparison of two strings. It is the
// single implementation shared by the hmac and secrets libraries (in CPython,
// secrets.compare_digest is hmac.compare_digest).
func CompareDigest(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// HmacLibrary provides HMAC (Keyed-Hashing for Message Authentication).
var HmacLibrary = object.NewLibrary(HmacLibraryName, map[string]*object.Builtin{
	"new": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.RangeArgs(args, 1, 3); err != nil {
				return err
			}
			key, errObj := toByteString(args[0])
			if errObj != nil {
				return errObj
			}
			var data []byte
			if len(args) >= 2 {
				if _, isNull := args[1].(*object.Null); !isNull {
					b, e := toByteString(args[1])
					if e != nil {
						return e
					}
					data = b
				}
			}
			var digestmod object.Object
			if len(args) >= 3 {
				digestmod = args[2]
			}
			alg, e := resolveHmacAlg(digestmod)
			if e != nil {
				return e
			}
			return newHmacInstance(alg, key, data)
		},
		HelpText: `new(key, msg=None, digestmod=None) - Create an HMAC object

key and msg are byte buffers (a string treated as bytes, or a list of byte
values such as returned by str.encode()). digestmod may be a string name
("sha256", "sha1", "md5"), a hashlib constructor (e.g. hashlib.sha256), or
omitted (defaults to sha256). Call .hexdigest() or .digest() for the result.

Example:
  import hmac, hashlib
  sig = "sha256=" + hmac.new(secret.encode(), body, hashlib.sha256).hexdigest()
  ok = hmac.compare_digest(sig, signature)`,
	},
	"digest": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 3); err != nil {
				return err
			}
			key, errObj := toByteString(args[0])
			if errObj != nil {
				return errObj
			}
			msg, errObj := toByteString(args[1])
			if errObj != nil {
				return errObj
			}
			alg, e := resolveHmacAlg(args[2])
			if e != nil {
				return e
			}
			return object.NewString(string(computeHmacDigest(&hmacState{key: key, data: msg, alg: alg})))
		},
		HelpText: `digest(key, msg, digestmod) - One-shot HMAC as a raw byte string`,
	},
	"compare_digest": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}
			a, okA := args[0].(*object.String)
			b, okB := args[1].(*object.String)
			if !okA || !okB {
				return errors.NewError("compare_digest() requires two string arguments")
			}
			return object.NewBoolean(CompareDigest(a.StringValue(), b.StringValue()))
		},
		HelpText: `compare_digest(a, b) - Constant-time string comparison

Compares two strings in constant time to help prevent timing attacks. Returns
True if equal, False otherwise. Use this to compare signature values.

Example:
  import hmac
  ok = hmac.compare_digest(expected_signature, received_signature)`,
	},
}, nil, "HMAC (Keyed-Hashing for Message Authentication) library")
