package extlibs

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"math/big"
	"strings"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

func RegisterSecretsLibrary(registrar interface{ RegisterLibrary(string, *object.Library) }) {
	registrar.RegisterLibrary(SecretsLibraryName, SecretsLibrary)
}

// SecretsLibrary provides cryptographically strong random number generation
// NOTE: This is an extended library and not enabled by default
var SecretsLibrary = object.NewLibrary(map[string]*object.Builtin{
	"token_bytes": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			nbytes := 32 // Default
			if len(args) > 0 {
				if intVal, ok := args[0].(*object.Integer); ok {
					nbytes = int(intVal.Value)
				}
			}

			if nbytes < 1 {
				return errors.NewError("token_bytes requires a positive number of bytes")
			}

			bytes := make([]byte, nbytes)
			_, err := rand.Read(bytes)
			if err != nil {
				return errors.NewError("failed to generate random bytes: %s", err.Error())
			}

			// Return as a list of integers (bytes)
			elements := make([]object.Object, nbytes)
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
			nbytes := 32 // Default
			if len(args) > 0 {
				if intVal, ok := args[0].(*object.Integer); ok {
					nbytes = int(intVal.Value)
				}
			}

			if nbytes < 1 {
				return errors.NewError("token_hex requires a positive number of bytes")
			}

			bytes := make([]byte, nbytes)
			_, err := rand.Read(bytes)
			if err != nil {
				return errors.NewError("failed to generate random bytes: %s", err.Error())
			}

			return &object.String{Value: hex.EncodeToString(bytes)}
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
			nbytes := 32 // Default
			if len(args) > 0 {
				if intVal, ok := args[0].(*object.Integer); ok {
					nbytes = int(intVal.Value)
				}
			}

			if nbytes < 1 {
				return errors.NewError("token_urlsafe requires a positive number of bytes")
			}

			bytes := make([]byte, nbytes)
			_, err := rand.Read(bytes)
			if err != nil {
				return errors.NewError("failed to generate random bytes: %s", err.Error())
			}

			// Use URL-safe base64 encoding without padding
			encoded := base64.URLEncoding.EncodeToString(bytes)
			// Remove padding
			encoded = strings.TrimRight(encoded, "=")
			return &object.String{Value: encoded}
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
			if len(args) != 1 {
				return errors.NewError("randbelow() requires exactly 1 argument")
			}

			n, ok := args[0].(*object.Integer)
			if !ok {
				return errors.NewTypeError("INTEGER", args[0].Type().String())
			}

			if n.Value <= 0 {
				return errors.NewError("randbelow requires a positive upper bound")
			}

			result, err := rand.Int(rand.Reader, big.NewInt(n.Value))
			if err != nil {
				return errors.NewError("failed to generate random number: %s", err.Error())
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
			if len(args) != 1 {
				return errors.NewError("randbits() requires exactly 1 argument")
			}

			k, ok := args[0].(*object.Integer)
			if !ok {
				return errors.NewTypeError("INTEGER", args[0].Type().String())
			}

			if k.Value < 1 {
				return errors.NewError("randbits requires a positive number of bits")
			}

			// Generate a random big integer with exactly k bits
			result, err := rand.Int(rand.Reader, big.NewInt(0).Lsh(big.NewInt(1), uint(k.Value)))
			if err != nil {
				return errors.NewError("failed to generate random bits: %s", err.Error())
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
			if len(args) != 1 {
				return errors.NewError("choice() requires exactly 1 argument")
			}

			// Handle string sequences (like Python)
			if str, ok := args[0].(*object.String); ok {
				if len(str.Value) == 0 {
					return errors.NewError("cannot choose from empty sequence")
				}
				idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(str.Value))))
				if err != nil {
					return errors.NewError("failed to generate random index: %s", err.Error())
				}
				return &object.String{Value: string(str.Value[idx.Int64()])}
			}

			// Handle list sequences
			list, ok := args[0].(*object.List)
			if !ok {
				return errors.NewTypeError("LIST or STRING", args[0].Type().String())
			}

			if len(list.Elements) == 0 {
				return errors.NewError("cannot choose from empty sequence")
			}

			idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(list.Elements))))
			if err != nil {
				return errors.NewError("failed to generate random index: %s", err.Error())
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
			if len(args) != 2 {
				return errors.NewError("compare_digest() requires exactly 2 arguments")
			}

			a, okA := args[0].(*object.String)
			b, okB := args[1].(*object.String)
			if !okA || !okB {
				return errors.NewError("compare_digest() requires two string arguments")
			}

			// Use crypto/subtle for constant-time comparison to prevent timing attacks
			if subtle.ConstantTimeCompare([]byte(a.Value), []byte(b.Value)) == 1 {
				return &object.Boolean{Value: true}
			}
			return &object.Boolean{Value: false}
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
}, nil, "Cryptographically strong random number generation (extended library)")
