package errors

import (
	"context"
	"fmt"

	"github.com/paularlott/scriptling/object"
)

// Common error types
const (
	ErrTimeout   = "execution timeout"
	ErrCancelled = "execution cancelled"
	// ErrMemoryLimitStop stops a run that went over its memory limit. Unlike
	// other errors a script cannot catch or suppress it.
	ErrMemoryLimitStop    = "memory limit exceeded"
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
		// Security violations always unwind the stack; they are a raise.
		Raised: true,
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

// NewTypeErrorTagged creates a type error tagged so that `except TypeError:`
// matches it — for the TypeError classes CPython raises where the generic
// NewTypeError(expected, got) message shape does not fit ("not enough
// arguments for format string", wrong value type for a % conversion, ...).
func NewTypeErrorTagged(format string, args ...interface{}) *object.Error {
	return &object.Error{
		Message:       fmt.Sprintf(format, args...),
		ExceptionType: object.ExceptionTypeTypeError,
	}
}

// NoKwargs returns the TypeError "fn() takes no keyword arguments" when the
// call passed any keywords, and nil otherwise. It is inlined, so a call with
// no keywords pays a single length test.
func NoKwargs(fn string, kwargs object.Kwargs) object.Object {
	if kwargs.Len() == 0 {
		return nil
	}
	return noKwargsError(fn)
}

// noKwargsError is kept out of line so NoKwargs stays cheap enough to inline.
//
//go:noinline
func noKwargsError(fn string) *object.Error {
	return NewTypeErrorTagged("%s() takes no keyword arguments", fn)
}

// CheckKwargs returns the TypeError "fn() got an unexpected keyword argument
// 'x'" for the first keyword that is not one of allowed, and nil when every
// keyword is allowed. With no allowed names it reports that fn takes no
// keyword arguments.
//
// Building the allowed list costs a few stores even when the call has no
// keywords, so a hot function guards the check:
//
//	if kwargs.Len() > 0 {
//		if err := errors.CheckKwargs("sorted", kwargs, "key", "reverse"); err != nil {
//			return err
//		}
//	}
func CheckKwargs(fn string, kwargs object.Kwargs, allowed ...string) object.Object {
	name, bad := kwargs.Unknown(allowed...)
	if !bad {
		return nil
	}
	if len(allowed) == 0 {
		return noKwargsError(fn)
	}
	return NewTypeErrorTagged("%s() got an unexpected keyword argument '%s'", fn, name)
}

// NewTimeoutError creates a timeout error
func NewTimeoutError() *object.Error {
	return &object.Error{Message: ErrTimeout}
}

// IsRunStop reports whether obj is an error that ends the run it occurs in: a
// timeout or cancellation of the run's context, or the memory-limit stop.
// try/except and with blocks let these through, so a script cannot keep
// running after its host has stopped it.
func IsRunStop(obj object.Object) bool {
	e, ok := obj.(*object.Error)
	if !ok {
		return false
	}
	switch e.Message {
	case ErrTimeout, ErrCancelled, ErrMemoryLimitStop:
		return true
	}
	return false
}

// ForCaller returns a result produced by another run (a background task) for
// a caller to use. That run being stopped does not stop a caller whose own run
// is live, so its stop becomes an ordinary error the caller can catch.
func ForCaller(ctx context.Context, result object.Object) object.Object {
	if IsRunStop(result) && ctx.Err() == nil {
		return NewError("async error: background task stopped: %s", result.(*object.Error).Message)
	}
	return result
}

// NewMemoryLimitStopError creates the error that stops a run over its memory
// limit; try/except and with blocks let it through.
func NewMemoryLimitStopError() *object.Error {
	return &object.Error{Message: ErrMemoryLimitStop}
}

// NewCancelledError creates a cancellation error
func NewCancelledError() *object.Error {
	return &object.Error{Message: ErrCancelled}
}

// NewCallDepthExceededError creates the error for recursion past the call
// depth limit: a RecursionError, as Python raises, which `except
// RecursionError` (and RuntimeError or Exception) catches.
func NewCallDepthExceededError(max int) *object.Error {
	return &object.Error{
		Message:       fmt.Sprintf("%s: maximum depth is %d", ErrCallDepthExceeded, max),
		ExceptionType: object.ExceptionTypeRecursionError,
	}
}

// NewMemoryError creates an error that `except MemoryError` catches, for an
// operation that would create a value larger than the interpreter allows.
func NewMemoryError(format string, args ...interface{}) *object.Error {
	return object.NewMemoryError(format, args...)
}

// NewPanicError creates a panic error with the panic value
func NewPanicError(panicValue interface{}) *object.Error {
	return &object.Error{Message: fmt.Sprintf("%s: %v", ErrPanic, panicValue)}
}

// NewTypeError creates a type error
func NewTypeError(expected, got string) *object.Error {
	return &object.Error{Message: fmt.Sprintf("%s: expected %s, got %s", ErrTypeError, expected, got)}
}

// NewArgumentError creates an argument error, tagged TypeError so
// `except TypeError:` catches it (Python raises TypeError for arity).
func NewArgumentError(got, want int) *object.Error {
	return &object.Error{
		Message:       fmt.Sprintf("%s: got %d arguments, want %d", ErrArgumentError, got, want),
		ExceptionType: object.ExceptionTypeTypeError,
	}
}

// NewIdentifierError creates a name-not-defined error, Python-shaped so
// models recognize it ("name 'x' is not defined").
func NewIdentifierError(name string) *object.Error {
	err := &object.Error{Message: fmt.Sprintf("name '%s' is not defined", name)}
	err.ExceptionType = object.ExceptionTypeNameError
	return err
}

// ParameterError wraps an error with a parameter name for context
func ParameterError(name string, err object.Object) object.Object {
	e := err.(*object.Error)
	return &object.Error{Message: fmt.Sprintf("%s: %s", name, e.Message), ExceptionType: e.ExceptionType}
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
		return NewTypeErrorTagged("expected at least %d arguments, got %d", n, len(args))
	}
	return nil
}

// MaxArgs checks that args has at most n elements
func MaxArgs(args []object.Object, n int) object.Object {
	if len(args) > n {
		return NewTypeErrorTagged("expected at most %d arguments, got %d", n, len(args))
	}
	return nil
}

// RangeArgs checks that args has between min and max elements (inclusive)
func RangeArgs(args []object.Object, min, max int) object.Object {
	if len(args) < min {
		return NewTypeErrorTagged("expected at least %d arguments, got %d", min, len(args))
	}
	if len(args) > max {
		return NewTypeErrorTagged("expected at most %d arguments, got %d", max, len(args))
	}
	return nil
}

// NewOverflowError creates an error tagged so that `except OverflowError:`
// (and ArithmeticError) matches it — for a result too large to represent,
// such as math.exp(1000) or converting float infinity to an integer.
func NewOverflowError(format string, args ...interface{}) *object.Error {
	return &object.Error{
		Message:       fmt.Sprintf(format, args...),
		ExceptionType: object.ExceptionTypeOverflowError,
	}
}

// NewIndexError creates an error tagged so that `except IndexError:` (and
// LookupError) matches it.
func NewIndexError(format string, args ...interface{}) *object.Error {
	return &object.Error{
		Message:       fmt.Sprintf(format, args...),
		ExceptionType: object.ExceptionTypeIndexError,
	}
}

// NewOSError creates an error tagged so that `except OSError:` matches it.
func NewOSError(format string, args ...interface{}) *object.Error {
	return &object.Error{
		Message:       fmt.Sprintf(format, args...),
		ExceptionType: object.ExceptionTypeOSError,
	}
}

// newTyped creates an error tagged with exceptionType so the matching except
// clause catches it.
func newTyped(exceptionType, format string, args ...interface{}) *object.Error {
	return &object.Error{
		Message:       fmt.Sprintf(format, args...),
		ExceptionType: exceptionType,
	}
}

// NewKeyErrorTagged creates an error tagged so that `except KeyError:` (and
// LookupError) matches it.
func NewKeyErrorTagged(format string, args ...interface{}) *object.Error {
	return newTyped(object.ExceptionTypeKeyError, format, args...)
}

// NewAttributeErrorTagged creates an error tagged so that
// `except AttributeError:` matches it.
func NewAttributeErrorTagged(format string, args ...interface{}) *object.Error {
	return newTyped(object.ExceptionTypeAttributeError, format, args...)
}

// NewRuntimeError creates an error tagged so that `except RuntimeError:`
// matches it, for an operation the interpreter's current state does not
// allow (re-raising with no active exception, a generator already running).
func NewRuntimeError(format string, args ...interface{}) *object.Error {
	return newTyped(object.ExceptionTypeRuntimeError, format, args...)
}

// NewSyntaxError creates an error tagged so that `except SyntaxError:`
// matches it, for a construct the parser accepts but the evaluator rejects.
func NewSyntaxError(format string, args ...interface{}) *object.Error {
	return newTyped(object.ExceptionTypeSyntaxError, format, args...)
}

// NewImportErrorTagged creates an error tagged so that `except ImportError:`
// matches it. The message keeps the "import error" prefix.
func NewImportErrorTagged(format string, args ...interface{}) *object.Error {
	return newTyped(object.ExceptionTypeImportError, format, args...)
}

// NewLookupError creates an error tagged so that `except LookupError:`
// matches it, as Python raises for an unknown codec name.
func NewLookupError(format string, args ...interface{}) *object.Error {
	return newTyped(object.ExceptionTypeLookupError, format, args...)
}

// NewUnicodeEncodeError and NewUnicodeDecodeError create the errors for text
// that a codec cannot encode or bytes it cannot decode: subclasses of
// UnicodeError and so of ValueError, which `except ValueError:` still catches.
func NewUnicodeEncodeError(format string, args ...interface{}) *object.Error {
	return newTyped(object.ExceptionTypeUnicodeEncodeError, format, args...)
}

func NewUnicodeDecodeError(format string, args ...interface{}) *object.Error {
	return newTyped(object.ExceptionTypeUnicodeDecodeError, format, args...)
}

// NewInfiniteIteratorError is the RuntimeError for collecting an infinite
// iterator (itertools.count, cycle or repeat) into a container.
func NewInfiniteIteratorError() *object.Error {
	return NewRuntimeError("%s", object.InfiniteIteratorMessage)
}
