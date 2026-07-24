package stdlib

import (
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// coerceToBytes accepts a String or Bytes input and returns the raw byte slice.
// Used by libraries that historically took strings but now also accept Bytes
// (e.g. base64.b64encode).
func coerceToBytes(obj object.Object) ([]byte, object.Object) {
	switch v := obj.(type) {
	case *object.Bytes:
		return v.BytesValue(), nil
	case *object.String:
		return []byte(v.StringValue()), nil
	default:
		return nil, errors.NewTypeError("BYTES or STRING", obj.Type().String())
	}
}
