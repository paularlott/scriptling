package object

import (
	"fmt"
)

// Kwargs is a special type for functions that accept keyword arguments.
// It wraps the raw kwargs map and provides helper methods for extracting values.
type Kwargs struct {
	Kwargs map[string]Object
}

// NewKwargs creates a new Kwargs wrapper.
func NewKwargs(kwargs map[string]Object) Kwargs {
	return Kwargs{Kwargs: kwargs}
}

// Has returns true if the key exists in kwargs.
func (k Kwargs) Has(key string) bool {
	_, ok := k.Kwargs[key]
	return ok
}

// Get returns the raw Object for the key, or nil if not found.
func (k Kwargs) Get(key string) Object {
	return k.Kwargs[key]
}

// Keys returns all kwargs keys.
func (k Kwargs) Keys() []string {
	keys := make([]string, 0, len(k.Kwargs))
	for key := range k.Kwargs {
		keys = append(keys, key)
	}
	return keys
}

// Len returns the number of kwargs.
func (k Kwargs) Len() int {
	return len(k.Kwargs)
}

// GetString extracts a string keyword argument with a default value.
func (k Kwargs) GetString(name string, defaultValue string) (string, Object) {
	if obj, ok := k.Kwargs[name]; ok {
		if s, err := obj.AsString(); err == nil {
			return s, nil
		}
		return defaultValue, &Error{Message: fmt.Sprintf("%s: must be a string", name)}
	}
	return defaultValue, nil
}

// GetInt extracts an integer keyword argument with a default value.
func (k Kwargs) GetInt(name string, defaultValue int64) (int64, Object) {
	if obj, ok := k.Kwargs[name]; ok {
		if i, err := obj.AsInt(); err == nil {
			return i, nil
		}
		return defaultValue, &Error{Message: fmt.Sprintf("%s: must be a number", name)}
	}
	return defaultValue, nil
}

// GetFloat extracts a float keyword argument with a default value.
func (k Kwargs) GetFloat(name string, defaultValue float64) (float64, Object) {
	if obj, ok := k.Kwargs[name]; ok {
		if f, err := obj.AsFloat(); err == nil {
			return f, nil
		}
		return defaultValue, &Error{Message: fmt.Sprintf("%s: must be a number", name)}
	}
	return defaultValue, nil
}

// GetBool extracts a boolean keyword argument with a default value.
func (k Kwargs) GetBool(name string, defaultValue bool) (bool, Object) {
	if obj, ok := k.Kwargs[name]; ok {
		if b, err := obj.AsBool(); err == nil {
			return b, nil
		}
		return defaultValue, &Error{Message: fmt.Sprintf("%s: must be a boolean", name)}
	}
	return defaultValue, nil
}

// GetList extracts a list keyword argument with a default value.
func (k Kwargs) GetList(name string, defaultValue []Object) ([]Object, Object) {
	if obj, ok := k.Kwargs[name]; ok {
		if l, err := obj.AsList(); err == nil {
			return l, nil
		}
		return defaultValue, &Error{Message: fmt.Sprintf("%s: must be a list", name)}
	}
	return defaultValue, nil
}

// MustGetString extracts a string keyword argument, ignoring errors.
func (k Kwargs) MustGetString(name string, defaultValue string) string {
	val, _ := k.GetString(name, defaultValue)
	return val
}

// MustGetInt extracts an integer keyword argument, ignoring errors.
func (k Kwargs) MustGetInt(name string, defaultValue int64) int64 {
	val, _ := k.GetInt(name, defaultValue)
	return val
}

// MustGetFloat extracts a float keyword argument, ignoring errors.
func (k Kwargs) MustGetFloat(name string, defaultValue float64) float64 {
	val, _ := k.GetFloat(name, defaultValue)
	return val
}

// MustGetBool extracts a boolean keyword argument, ignoring errors.
func (k Kwargs) MustGetBool(name string, defaultValue bool) bool {
	val, _ := k.GetBool(name, defaultValue)
	return val
}

// MustGetList extracts a list keyword argument, ignoring errors.
func (k Kwargs) MustGetList(name string, defaultValue []Object) []Object {
	val, _ := k.GetList(name, defaultValue)
	return val
}