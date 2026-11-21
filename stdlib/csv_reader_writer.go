package stdlib

import (
	"fmt"

	"github.com/paularlott/scriptling/object"
)

// csvSimpleError is the TypeError for a csv reader or writer used with the
// wrong arguments, receiver or row type. The reader and writer themselves
// are in csv_python.go.
func csvSimpleError(format string, a ...any) object.Object {
	return &object.Error{Message: fmt.Sprintf(format, a...), ExceptionType: object.ExceptionTypeTypeError}
}
