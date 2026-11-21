package object

import "sync"

// exceptionClasses maps the builtins that construct a library's exception
// type (re.error, json.JSONDecodeError, ...) to the exception type name the
// exceptions they create carry.
var exceptionClasses sync.Map

// RegisterExceptionClass records that b constructs exceptions of type name,
// so b works as an exception class when it is a value: in except clauses
// (directly, through a variable or in a tuple), isinstance(), issubclass()
// and as a base class. It returns b.
func RegisterExceptionClass(b *Builtin, name string) *Builtin {
	exceptionClasses.Store(b, name)
	return b
}

// ExceptionClassOf returns the exception type name b was registered with by
// RegisterExceptionClass.
func ExceptionClassOf(b *Builtin) (string, bool) {
	v, ok := exceptionClasses.Load(b)
	if !ok {
		return "", false
	}
	return v.(string), true
}
