package scriptling

import (
	"fmt"

	"github.com/paularlott/scriptling/object"
)

// Kwargs is a special type for functions that accept keyword arguments.
// It wraps the raw kwargs map and provides helper methods for extracting values.
// When used as the last parameter in a Fluent API function, the builder will
// automatically pass the kwargs map to it.
//
// Example:
//
//	import "github.com/paularlott/scriptling/object"
//
//	builder.Function("connect", func(host string, port int, kwargs scriptling.Kwargs) error {
//	    timeout, err := kwargs.GetInt("timeout", 30)
//	    if err != nil {
//	        return err
//	    }
//	    debug, err := kwargs.GetBool("debug", false)
//	    if err != nil {
//	        return err
//	    }
//	    return connectTo(host, port, timeout, debug)
//	})
type Kwargs struct {
	Kwargs map[string]object.Object
}

// NewKwargs creates a new Kwargs wrapper.
func NewKwargs(kwargs map[string]object.Object) Kwargs {
	return Kwargs{Kwargs: kwargs}
}

// Has returns true if the key exists in kwargs.
func (k Kwargs) Has(key string) bool {
	_, ok := k.Kwargs[key]
	return ok
}

// Get returns the raw Object for the key, or nil if not found.
func (k Kwargs) Get(key string) object.Object {
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
// Returns the string value from kwargs if present, otherwise returns defaultValue.
// Returns an error if the kwarg is provided but is not a string.
func (k Kwargs) GetString(name string, defaultValue string) (string, error) {
	val, err := GetStringFromKwargs(k.Kwargs, name, defaultValue)
	if err != nil {
		if errObj, ok := err.(*object.Error); ok {
			return defaultValue, fmt.Errorf("%s", errObj.Message)
		}
		return defaultValue, fmt.Errorf("unknown error")
	}
	return val, nil
}

// GetInt extracts an integer keyword argument with a default value.
// Returns the int64 value from kwargs if present (accepts both Integer and Float), otherwise returns defaultValue.
// Returns an error if the kwarg is provided but is not a number.
func (k Kwargs) GetInt(name string, defaultValue int64) (int64, error) {
	val, err := GetIntFromKwargs(k.Kwargs, name, defaultValue)
	if err != nil {
		if errObj, ok := err.(*object.Error); ok {
			return defaultValue, fmt.Errorf("%s", errObj.Message)
		}
		return defaultValue, fmt.Errorf("unknown error")
	}
	return val, nil
}

// GetFloat extracts a float keyword argument with a default value.
// Returns the float64 value from kwargs if present (accepts both Integer and Float), otherwise returns defaultValue.
// Returns an error if the kwarg is provided but is not a number.
func (k Kwargs) GetFloat(name string, defaultValue float64) (float64, error) {
	val, err := GetFloatFromKwargs(k.Kwargs, name, defaultValue)
	if err != nil {
		if errObj, ok := err.(*object.Error); ok {
			return defaultValue, fmt.Errorf("%s", errObj.Message)
		}
		return defaultValue, fmt.Errorf("unknown error")
	}
	return val, nil
}

// GetBool extracts a boolean keyword argument with a default value.
// Returns the bool value from kwargs if present, otherwise returns defaultValue.
// Returns an error if the kwarg is provided but is not a boolean.
func (k Kwargs) GetBool(name string, defaultValue bool) (bool, error) {
	val, err := GetBoolFromKwargs(k.Kwargs, name, defaultValue)
	if err != nil {
		if errObj, ok := err.(*object.Error); ok {
			return defaultValue, fmt.Errorf("%s", errObj.Message)
		}
		return defaultValue, fmt.Errorf("unknown error")
	}
	return val, nil
}

// GetList extracts a list keyword argument with a default value.
// Returns the list value from kwargs if present (accepts both List and Tuple), otherwise returns defaultValue.
// Returns an error if the kwarg is provided but is not a list.
func (k Kwargs) GetList(name string, defaultValue []object.Object) ([]object.Object, error) {
	val, err := GetListFromKwargs(k.Kwargs, name, defaultValue)
	if err != nil {
		if errObj, ok := err.(*object.Error); ok {
			return defaultValue, fmt.Errorf("%s", errObj.Message)
		}
		return defaultValue, fmt.Errorf("unknown error")
	}
	return val, nil
}

// MustGetString extracts a string keyword argument, panicking on error.
// Useful for simple cases where you want to fail fast on type errors.
func (k Kwargs) MustGetString(name string, defaultValue string) string {
	val, _ := k.GetString(name, defaultValue)
	return val
}

// MustGetInt extracts an integer keyword argument, panicking on error.
func (k Kwargs) MustGetInt(name string, defaultValue int64) int64 {
	val, _ := k.GetInt(name, defaultValue)
	return val
}

// MustGetFloat extracts a float keyword argument, panicking on error.
func (k Kwargs) MustGetFloat(name string, defaultValue float64) float64 {
	val, _ := k.GetFloat(name, defaultValue)
	return val
}

// MustGetBool extracts a boolean keyword argument, panicking on error.
func (k Kwargs) MustGetBool(name string, defaultValue bool) bool {
	val, _ := k.GetBool(name, defaultValue)
	return val
}

// MustGetList extracts a list keyword argument, panicking on error.
func (k Kwargs) MustGetList(name string, defaultValue []object.Object) []object.Object {
	val, _ := k.GetList(name, defaultValue)
	return val
}
