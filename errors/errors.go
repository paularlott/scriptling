package errors

import (
	"fmt"

	"github.com/paularlott/scriptling/object"
)

// Common error types
const (
	ErrTimeout            = "execution timeout"
	ErrCancelled          = "execution cancelled"
	ErrDivisionByZero     = "division by zero"
	ErrIndexOutOfRange    = "index out of range"
	ErrKeyNotFound        = "key not found"
	ErrTypeError          = "type error"
	ErrArgumentError      = "argument error"
	ErrIdentifierNotFound = "identifier not found"
	ErrUnknownOperator    = "unknown operator"
	ErrImportError        = "import error"
	ErrCallDepthExceeded  = "call depth exceeded"
	ErrPanic              = "script panic"
)

// NewPermissionError creates a PermissionError exception that bypasses try/except
func NewPermissionError(format string, args ...interface{}) *object.Exception {
	return &object.Exception{
		Message:       fmt.Sprintf(format, args...),
		ExceptionType: object.ExceptionTypePermissionError,
	}
}

// NewError creates a new error object
func NewError(format string, args ...interface{}) *object.Error {
	return &object.Error{Message: fmt.Sprintf(format, args...)}
}

// NewZeroDivisionError creates a division-by-zero error tagged so that
// `except ZeroDivisionError:` matches it. It stays an *object.Error (not an
// Exception) so it propagates through the evaluator's existing object.IsError
// checks and keeps its file/line/function annotation.
func NewZeroDivisionError() *object.Error {
	return &object.Error{
		Message:       ErrDivisionByZero,
		ExceptionType: object.ExceptionTypeZeroDivisionError,
	}
}

// NewValueError creates an error tagged so that `except ValueError:` matches it.
// Use it for a value that is the right type but outside the accepted domain —
// the maths-domain errors CPython reports as ValueError, for example. Like
// NewZeroDivisionError it stays an *object.Error so it propagates through the
// evaluator's object.IsError checks and keeps its file/line/function annotation.
func NewValueError(format string, args ...interface{}) *object.Error {
	return &object.Error{
		Message:       fmt.Sprintf(format, args...),
		ExceptionType: object.ExceptionTypeValueError,
	}
}

// NewTimeoutError creates a timeout error
func NewTimeoutError() *object.Error {
	return &object.Error{Message: ErrTimeout}
}

// NewCancelledError creates a cancellation error
func NewCancelledError() *object.Error {
	return &object.Error{Message: ErrCancelled}
}

// NewCallDepthExceededError creates a call depth exceeded error
func NewCallDepthExceededError(max int) *object.Error {
	return &object.Error{Message: fmt.Sprintf("%s: maximum depth is %d", ErrCallDepthExceeded, max)}
}

// NewPanicError creates a panic error with the panic value
func NewPanicError(panicValue interface{}) *object.Error {
	return &object.Error{Message: fmt.Sprintf("%s: %v", ErrPanic, panicValue)}
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

// ParameterError wraps an error with a parameter name for context
func ParameterError(name string, err object.Object) object.Object {
	return &object.Error{Message: fmt.Sprintf("%s: %s", name, err.(*object.Error).Message)}
}

// Argument validation helpers return nil if validation passes, otherwise return an error

// NoArgs checks that args has no elements
func NoArgs(args []object.Object) object.Object {
	if len(args) != 0 {
		return NewArgumentError(len(args), 0)
	}
	return nil
}

// ExactArgs checks that args has exactly n elements
func ExactArgs(args []object.Object, n int) object.Object {
	if len(args) != n {
		return NewArgumentError(len(args), n)
	}
	return nil
}

// MinArgs checks that args has at least n elements
func MinArgs(args []object.Object, n int) object.Object {
	if len(args) < n {
		return NewError("expected at least %d arguments, got %d", n, len(args))
	}
	return nil
}

// MaxArgs checks that args has at most n elements
func MaxArgs(args []object.Object, n int) object.Object {
	if len(args) > n {
		return NewError("expected at most %d arguments, got %d", n, len(args))
	}
	return nil
}

// RangeArgs checks that args has between min and max elements (inclusive)
func RangeArgs(args []object.Object, min, max int) object.Object {
	if len(args) < min {
		return NewError("expected at least %d arguments, got %d", min, len(args))
	}
	if len(args) > max {
		return NewError("expected at most %d arguments, got %d", max, len(args))
	}
	return nil
}
