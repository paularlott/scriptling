package stdlib

import (
	"context"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"hash"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// hashState is the native state held by a Hash instance. It accumulates the
// input data and remembers the algorithm so that digest/hexdigest can be
// computed on demand (and copy() yields a true independent copy).
type hashState struct {
	data []byte
	alg  string
}

// hashConstructor returns the Go constructor for the given algorithm name, or
// nil if the algorithm is unsupported.
func hashConstructor(alg string) func() hash.Hash {
	switch alg {
	case "sha256":
		return sha256.New
	case "sha1":
		return sha1.New
	case "md5":
		return md5.New
	}
	return nil
}

func hashBlockSize(alg string) int {
	switch alg {
	case "sha256", "sha1", "md5":
		return 64
	}
	return 0
}

func hashDigestSize(alg string) int {
	switch alg {
	case "sha256":
		return 32
	case "sha1":
		return 20
	case "md5":
		return 16
	}
	return 0
}

// toByteString converts a Scriptling object representing a byte buffer into a
// raw Go byte slice. Strings are treated as byte buffers directly; lists of
// integers (as produced by str.encode()) are also accepted.
func toByteString(obj object.Object) ([]byte, object.Object) {
	switch v := obj.(type) {
	case *object.String:
		return []byte(v.StringValue()), nil
	case *object.List:
		b := make([]byte, 0, len(v.Elements))
		for i, el := range v.Elements {
			iv, ok := el.(*object.Integer)
			if !ok {
				return nil, errors.NewError("byte buffer element %d is not an integer", i)
			}
			val := iv.IntValue()
			if val < 0 || val > 255 {
				return nil, errors.NewError("byte buffer element %d out of range (0-255): %d", i, val)
			}
			b = append(b, byte(val))
		}
		return b, nil
	default:
		return nil, errors.NewTypeError("STRING or LIST", v.Type().String())
	}
}

// HashClass is the class exposed for objects returned by hashlib constructors.
var HashClass = &object.Class{
	Name: "Hash",
	Methods: map[string]object.Object{
		"update": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 2); err != nil {
					return err
				}
				inst, ok := args[0].(*object.Instance)
				if !ok {
					return errors.NewTypeError("Hash", args[0].Type().String())
				}
				state, ok := inst.NativeData.(*hashState)
				if !ok {
					return errors.NewError("invalid hash object")
				}
				b, errObj := toByteString(args[1])
				if errObj != nil {
					return errObj
				}
				state.data = append(state.data, b...)
				return &object.Null{}
			},
			HelpText: `update(data) - Feed data into the hash

Appends the given data (a string treated as bytes, or a list of byte values)
to the data already fed into the hash. Returns None.

Example:
  import hashlib
  h = hashlib.sha256()
  h.update("foo")
  h.update("bar")
  print(h.hexdigest())  # hash of "foobar"`,
		},
		"digest": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}
				inst, ok := args[0].(*object.Instance)
				if !ok {
					return errors.NewTypeError("Hash", args[0].Type().String())
				}
				state, ok := inst.NativeData.(*hashState)
				if !ok {
					return errors.NewError("invalid hash object")
				}
				h := hashConstructor(state.alg)()
				h.Write(state.data)
				return object.NewString(string(h.Sum(nil)))
			},
			HelpText: `digest() - Return the raw hash as a byte string

Returns the digest of the data passed to the hash so far, as a string of raw
bytes (Scriptling has no dedicated bytes type).`,
		},
		"hexdigest": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}
				inst, ok := args[0].(*object.Instance)
				if !ok {
					return errors.NewTypeError("Hash", args[0].Type().String())
				}
				state, ok := inst.NativeData.(*hashState)
				if !ok {
					return errors.NewError("invalid hash object")
				}
				h := hashConstructor(state.alg)()
				h.Write(state.data)
				return object.NewString(hex.EncodeToString(h.Sum(nil)))
			},
			HelpText: `hexdigest() - Return the hash as a hexadecimal string

Returns the digest of the data passed to the hash so far, encoded as lowercase
hexadecimal.

Example:
  import hashlib
  print(hashlib.sha256("hello").hexdigest())
  # "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"`,
		},
		"copy": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}
				inst, ok := args[0].(*object.Instance)
				if !ok {
					return errors.NewTypeError("Hash", args[0].Type().String())
				}
				state, ok := inst.NativeData.(*hashState)
				if !ok {
					return errors.NewError("invalid hash object")
				}
				newData := make([]byte, len(state.data))
				copy(newData, state.data)
				// Build from inst.Class (a parameter) rather than the HashClass
				// package var to avoid a package initialization cycle.
				return object.NewInstanceWithData(inst.Class, map[string]object.Object{
					"name":        object.NewString(state.alg),
					"digest_size": object.NewInteger(int64(hashDigestSize(state.alg))),
					"block_size":  object.NewInteger(int64(hashBlockSize(state.alg))),
				}, &hashState{data: newData, alg: state.alg})
			},
			HelpText: `copy() - Return a copy of the hash object

Returns a new hash object with the same algorithm and accumulated data.
Updates to one object do not affect the other.`,
		},
	},
}

// newHashInstance builds a Hash instance wrapping the given algorithm and data.
func newHashInstance(alg string, data []byte) *object.Instance {
	return object.NewInstanceWithData(HashClass, map[string]object.Object{
		"name":        object.NewString(alg),
		"digest_size": object.NewInteger(int64(hashDigestSize(alg))),
		"block_size":  object.NewInteger(int64(hashBlockSize(alg))),
	}, &hashState{data: data, alg: alg})
}

// makeHashBuiltin builds a hashlib constructor builtin for the given algorithm.
func makeHashBuiltin(alg, summary string) *object.Builtin {
	return &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.MaxArgs(args, 1); err != nil {
				return err
			}
			data := []byte{}
			if len(args) == 1 {
				if _, isNull := args[0].(*object.Null); !isNull {
					b, errObj := toByteString(args[0])
					if errObj != nil {
						return errObj
					}
					data = b
				}
			}
			return newHashInstance(alg, data)
		},
		HelpText: summary,
	}
}

// Package-level references to the constructor builtins so that hmac can
// identify them when passed as the digestmod argument (e.g. hashlib.sha256).
var (
	HashlibSHA256Builtin = makeHashBuiltin("sha256", `sha256([data]) - Create a SHA-256 hash object

Returns a hash object. Call .hexdigest() or .digest() to get the result, or
.update() to feed more data.

Example:
  import hashlib
  print(hashlib.sha256("hello").hexdigest())`)
	HashlibSHA1Builtin = makeHashBuiltin("sha1", `sha1([data]) - Create a SHA-1 hash object

Returns a hash object. Call .hexdigest() or .digest() to get the result.`)
	HashlibMD5Builtin = makeHashBuiltin("md5", `md5([data]) - Create an MD5 hash object

Returns a hash object. Call .hexdigest() or .digest() to get the result.`)
)

var HashlibLibrary = object.NewLibrary(HashlibLibraryName, map[string]*object.Builtin{
	"sha256": HashlibSHA256Builtin,
	"sha1":   HashlibSHA1Builtin,
	"md5":    HashlibMD5Builtin,
}, nil, "Cryptographic hash functions library")
