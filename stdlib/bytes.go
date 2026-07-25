package stdlib

import (
	"github.com/paularlott/scriptling/conversion"
	"github.com/paularlott/scriptling/object"
)

// coerceToBytes is retained as a thin wrapper so existing call sites inside
// stdlib read naturally; new code outside stdlib should use conversion.ToBytes
// directly.
func coerceToBytes(obj object.Object) ([]byte, object.Object) {
	return conversion.ToBytes(obj)
}
