package stdlib

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"math"
	"math/big"
	"strings"
	"unsafe"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// maxTokenBytes bounds a token request far above any useful size, so the size
// arithmetic below cannot overflow.
const maxTokenBytes = 1 << 40

// tokenByteCount reads a token function's byte-count argument (default 32).
func tokenByteCount(name string, args []object.Object) (int, object.Object) {
	n := int64(32)
	if len(args) > 0 && args[0].Type() != object.NULL_OBJ {
		intVal, ok := args[0].(*object.Integer)
		if !ok {
			return 0, errors.NewTypeErrorTagged("'%s' object cannot be interpreted as an integer", object.PythonTypeName(args[0]))
		}
		n = intVal.IntValue()
	}
	if n < 0 {
		return 0, errors.NewValueError("negative argument not allowed")
	}
	if n > maxTokenBytes || n > int64(math.MaxInt) {
		return 0, object.NewMemoryError("%s: %d bytes is too large", name, n)
	}
	return int(n), nil
}

// checkToken checks a token of resultBytes against the value limit, and the
// raw random bytes held alongside it against the memory limit.
func checkToken(what string, rawBytes, resultBytes int64) object.Object {
	if errObj := object.CheckAllocFor(what, resultBytes); errObj != nil {
		return errObj
	}
	return object.CheckRoom(what, rawBytes+resultBytes)
}

// SecretsLibrary provides cryptographically strong random number generation
var SecretsLibrary = object.NewLibrary(SecretsLibraryName, map[string]*object.Builtin{
	"token_bytes": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			args, kerr := kwargsToPositional("token_bytes", args, kwargs, "nbytes")
			if kerr != nil {
				return kerr
			}
			nbytes, errObj := tokenByteCount("token_bytes", args)
			if errObj != nil {
				return errObj
			}
			// Each byte becomes an integer in the returned list.
			if errObj := checkToken("token list", int64(nbytes), int64(nbytes)*int64(unsafe.Sizeof(object.Object(nil)))); errObj != nil {
				return errObj
			}

			bytes := make([]byte, nbytes)
			_, err := rand.Read(bytes)
			if err != nil {
				return errors.NewOSError("failed to generate random bytes: %s", err.Error())
			}

			// Return as a list of integers (bytes)
			elements := make([]object.Object, nbytes)
			object.TrackSlice(elements)
			for i, b := range bytes {
				elements[i] = object.NewInteger(int64(b))
			}
			return &object.List{Elements: elements}
		},
		HelpText: `token_bytes([nbytes]) - Generate nbytes random bytes

Parameters:
  nbytes - Number of bytes to generate (default 32)

Returns: List of integers representing bytes

Example:
  import secrets
  bytes = secrets.token_bytes(16)`,
	},

	"token_hex": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			args, kerr := kwargsToPositional("token_hex", args, kwargs, "nbytes")
			if kerr != nil {
				return kerr
			}
			nbytes, errObj := tokenByteCount("token_hex", args)
			if errObj != nil {
				return errObj
			}
			// The token is two hex characters per byte.
			if errObj := checkToken("hex token", int64(nbytes), int64(nbytes)*2); errObj != nil {
				return errObj
			}

			bytes := make([]byte, nbytes)
			_, err := rand.Read(bytes)
			if err != nil {
				return errors.NewOSError("failed to generate random bytes: %s", err.Error())
			}

			return object.NewString(hex.EncodeToString(bytes))
		},
		HelpText: `token_hex([nbytes]) - Generate random text in hexadecimal

Parameters:
  nbytes - Number of random bytes (string will be 2x this length) (default 32)

Returns: Hex string

Example:
  import secrets
  token = secrets.token_hex(16)  # 32 character hex string`,
	},

	"token_urlsafe": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			args, kerr := kwargsToPositional("token_urlsafe", args, kwargs, "nbytes")
			if kerr != nil {
				return kerr
			}
			nbytes, errObj := tokenByteCount("token_urlsafe", args)
			if errObj != nil {
				return errObj
			}
			// The token is the URL-safe Base64 encoding of the bytes (built
			// padded, then trimmed).
			if errObj := checkToken("URL-safe token", int64(nbytes), int64(base64.URLEncoding.EncodedLen(nbytes))); errObj != nil {
				return errObj
			}

			bytes := make([]byte, nbytes)
			_, err := rand.Read(bytes)
			if err != nil {
				return errors.NewOSError("failed to generate random bytes: %s", err.Error())
			}

			// Use URL-safe base64 encoding without padding
			encoded := base64.URLEncoding.EncodeToString(bytes)
			// Remove padding
			encoded = strings.TrimRight(encoded, "=")
			return object.NewString(encoded)
		},
		HelpText: `token_urlsafe([nbytes]) - Generate URL-safe random text

Parameters:
  nbytes - Number of random bytes (default 32)

Returns: URL-safe base64 encoded string

Example:
  import secrets
  token = secrets.token_urlsafe(16)`,
	},

	"randbelow": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("randbelow", kwargs); err != nil {
				return err
			}
			if len(args) != 1 {
				return errors.NewTypeErrorTagged("randbelow() takes exactly 1 argument (%d given)", len(args))
			}

			n, ok := args[0].(*object.Integer)
			if !ok {
				return errors.NewTypeError("INTEGER", args[0].Type().String())
			}

			if n.IntValue() <= 0 {
				return errors.NewValueError("Upper bound must be positive.")
			}

			result, err := rand.Int(rand.Reader, big.NewInt(n.IntValue()))
			if err != nil {
				return errors.NewOSError("failed to generate random number: %s", err.Error())
			}

			return object.NewInteger(result.Int64())
		},
		HelpText: `randbelow(n) - Generate a random integer in range [0, n)

Parameters:
  n - Exclusive upper bound (must be positive)

Returns: Random integer from 0 to n-1

Example:
  import secrets
  dice = secrets.randbelow(6) + 1  # 1-6`,
	},

	"randbits": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("randbits", kwargs); err != nil {
				return err
			}
			if len(args) != 1 {
				return errors.NewTypeErrorTagged("randbits() takes exactly 1 argument (%d given)", len(args))
			}

			k, ok := args[0].(*object.Integer)
			if !ok {
				return errors.NewTypeError("INTEGER", args[0].Type().String())
			}

			if k.IntValue() < 0 {
				return errors.NewValueError("number of bits must be non-negative")
			}
			if k.IntValue() == 0 {
				return object.NewInteger(0)
			}
			// Integers are 64-bit, so more bits cannot be represented; refusing
			// them also stops a huge k sizing a huge allocation.
			if k.IntValue() > 63 {
				return &object.Error{
					Message:       "randbits() supports at most 63 bits",
					ExceptionType: object.ExceptionTypeOverflowError,
				}
			}

			// Generate a random big integer with exactly k bits
			result, err := rand.Int(rand.Reader, big.NewInt(0).Lsh(big.NewInt(1), uint(k.IntValue())))
			if err != nil {
				return errors.NewOSError("failed to generate random bits: %s", err.Error())
			}

			return object.NewInteger(result.Int64())
		},
		HelpText: `randbits(k) - Generate a random integer with k random bits

Parameters:
  k - Number of random bits (must be positive)

Returns: Random integer with k bits

Example:
  import secrets
  random_int = secrets.randbits(8)  # 0-255`,
	},

	"choice": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("choice", kwargs); err != nil {
				return err
			}
			if len(args) != 1 {
				return errors.NewTypeErrorTagged("choice() takes exactly 1 argument (%d given)", len(args))
			}

			// Handle string sequences (like Python)
			if str, ok := args[0].(*object.String); ok {
				if len(str.StringValue()) == 0 {
					return errors.NewIndexError("Cannot choose from an empty sequence")
				}
				// Choose a character, not a byte, of non-ASCII text.
				runes := []rune(str.StringValue())
				idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(runes))))
				if err != nil {
					return errors.NewOSError("failed to generate random index: %s", err.Error())
				}
				return object.NewString(string(runes[idx.Int64()]))
			}

			// Handle list and tuple sequences
			var list *object.List
			switch seq := args[0].(type) {
			case *object.List:
				list = seq
			case *object.Tuple:
				list = &object.List{Elements: seq.Elements}
			default:
				return errors.NewTypeError("LIST or STRING", args[0].Type().String())
			}

			if len(list.Elements) == 0 {
				return errors.NewIndexError("Cannot choose from an empty sequence")
			}

			idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(list.Elements))))
			if err != nil {
				return errors.NewOSError("failed to generate random index: %s", err.Error())
			}

			return list.Elements[idx.Int64()]
		},
		HelpText: `choice(sequence) - Return a random element from sequence

Parameters:
  sequence - Non-empty list or string to choose from

Returns: Random element from the sequence

Example:
  import secrets
  item = secrets.choice(["apple", "banana", "cherry"])
  char = secrets.choice("abcdef")`,
	},

	"compare_digest": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("compare_digest", kwargs); err != nil {
				return err
			}
			if len(args) != 2 {
				return errors.NewTypeErrorTagged("compare_digest() takes exactly 2 arguments (%d given)", len(args))
			}

			// The shared constant-time comparison used by hmac.
			return compareDigestArgs(args[0], args[1])
		},
		HelpText: `compare_digest(a, b) - Compare two strings using constant-time comparison

This function is designed to prevent timing attacks when comparing secret values.

Parameters:
  a - First string
  b - Second string

Returns: True if strings are equal, False otherwise

Example:
  import secrets
  secrets.compare_digest(user_token, stored_token)`,
	},
}, nil, "Cryptographically strong random number generation")
