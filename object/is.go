package object

// IsError returns true if the object is an Error type.
//
// This uses a concrete type assertion rather than obj.Type() == ERROR_OBJ:
// the assertion compiles to a single itab pointer comparison, avoiding the
// non-inlinable interface method call on this very hot path.
func IsError(obj Object) bool {
	_, ok := obj.(*Error)
	return ok
}
