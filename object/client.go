package object

// ClientWrapper is a generic wrapper for storing Go client pointers in object.Instance fields.
// The underlying client is stored as an opaque pointer and accessed via type assertion.
//
// Example usage:
//
//	type MyClientWrapper struct {
//	    instance *MyClientInstance
//	}
//
//	func (w *MyClientWrapper) Type() ObjectType { return INSTANCE_OBJ }
//	func (w *MyClientWrapper) Inspect() string { return "<MyClient>" }
//	// ... implement other Object methods ...
//
//	// Store in instance:
//	instance.Fields["_client"] = &MyClientWrapper{instance: &MyClientInstance{...}}
//
//	// Extract from instance:
//	wrapper, _ := instance.Fields["_client"].(*MyClientWrapper)
//	client := wrapper.instance
//
// For convenience, use NewClientWrapper to create a wrapper with a custom type name.
type ClientWrapper struct {
	// TypeName is the display name used in Inspect() (e.g., "OpenAIClient", "MCPClient")
	TypeName string
	// Client is the underlying Go client pointer (opaque to scriptling)
	Client any
}

// Type returns INSTANCE_OBJ as this wrapper represents an instance
func (w *ClientWrapper) Type() ObjectType { return INSTANCE_OBJ }

// Inspect returns a string representation of the client
func (w *ClientWrapper) Inspect() string {
	if w.TypeName != "" {
		return "<" + w.TypeName + ">"
	}
	return "<client>"
}

// AsString returns the inspect representation
func (w *ClientWrapper) AsString() (string, Object) { return w.Inspect(), nil }

// AsInt returns an error - clients cannot be converted to int
func (w *ClientWrapper) AsInt() (int64, Object) { return 0, &Error{Message: ErrMustBeInteger} }

// AsFloat returns an error - clients cannot be converted to float
func (w *ClientWrapper) AsFloat() (float64, Object) { return 0, &Error{Message: ErrMustBeNumber} }

// AsBool returns true - clients are truthy
func (w *ClientWrapper) AsBool() (bool, Object) { return true, nil }

// AsList returns an error - clients cannot be converted to list
func (w *ClientWrapper) AsList() ([]Object, Object) { return nil, &Error{Message: ErrMustBeList} }

// AsDict returns an error - clients cannot be converted to dict
func (w *ClientWrapper) AsDict() (map[string]Object, Object) { return nil, &Error{Message: ErrMustBeDict} }

// AsError returns the error message from an Object, or empty string if not an error.
// This is a shared helper for extracting error messages from Objects.
func AsError(obj Object) string {
	if errObj, ok := obj.(*Error); ok {
		return errObj.Message
	}
	return ""
}

// GetClientField extracts a ClientWrapper from an object.Instance field.
// Returns the wrapper and true if found, nil and false otherwise.
//
// This is a convenience function for the common pattern of extracting
// a client wrapper from the "_client" field of an instance.
func GetClientField(instance *Instance, fieldName string) (*ClientWrapper, bool) {
	if instance == nil {
		return nil, false
	}
	obj, ok := instance.Fields[fieldName]
	if !ok {
		return nil, false
	}
	wrapper, ok := obj.(*ClientWrapper)
	return wrapper, ok
}
