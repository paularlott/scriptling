// Package kwarg converts object-side argument errors into Go errors, for
// typed class constructors whose signatures require an error return.
package kwarg

import (
	"errors"

	"github.com/paularlott/scriptling/object"
)

// Err returns nil when obj is nil (success), and an error carrying the
// object error's message otherwise.
func Err(obj object.Object) error {
	if obj == nil {
		return nil
	}
	if err, ok := obj.(*object.Error); ok {
		return errors.New(err.Message)
	}
	return errors.New(obj.Inspect())
}
