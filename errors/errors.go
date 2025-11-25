package errors

import (
	"fmt"
	"github.com/paularlott/scriptling/object"
)

// Common error types
const (
	ErrTimeout           = "execution timeout"
	ErrCancelled         = "execution cancelled"
	ErrDivisionByZero    = "division by zero"
	ErrIndexOutOfRange   = "index out of range"
	ErrKeyNotFound       = "key not found"
	ErrTypeError         = "type error"
	ErrArgumentError     = "argument error"
	ErrIdentifierNotFound = "identifier not found"
	ErrUnknownOperator   = "unknown operator"
	ErrImportError       = "import error"
)

// NewError creates a new error object
func NewError(format string, args ...interface{}) *object.Error {
	return &object.Error{Message: fmt.Sprintf(format, args...)}
}

// NewTimeoutError creates a timeout error
func NewTimeoutError() *object.Error {
	return &object.Error{Message: ErrTimeout}
}

// NewCancelledError creates a cancellation error
func NewCancelledError() *object.Error {
	return &object.Error{Message: ErrCancelled}
}

// NewTypeError creates a type error
func NewTypeError(expected, got string) *object.Error {
	return &object.Error{Message: fmt.Sprintf("%s: expected %s, got %s", ErrTypeError, expected, got)}
}

// NewArgumentError creates an argument error
func NewArgumentError(got, want int) *object.Error {
	return &object.Error{Message: fmt.Sprintf("%s: got %d arguments, want %d", ErrArgumentError, got, want)}
}

// NewIdentifierError creates an identifier not found error
func NewIdentifierError(name string) *object.Error {
	return &object.Error{Message: fmt.Sprintf("%s: %s", ErrIdentifierNotFound, name)}
}