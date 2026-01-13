package object

// IsError returns true if the object is an Error type.
func IsError(obj Object) bool {
	return obj != nil && obj.Type() == ERROR_OBJ
}
