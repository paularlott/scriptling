package object

import (
	"fmt"
	"iter"
	"sort"
)

// Kwargs holds the keyword arguments of a call to a Go-implemented function.
//
// A call has only a handful of keywords, so they are kept as a short slice of
// name/value pairs searched linearly rather than as a map. The zero value is
// an empty Kwargs and costs nothing to pass.
//
// A Kwargs owns its storage and is never modified after it is built: a
// function may keep the Kwargs it was called with (or anything it returns)
// for as long as it likes. Names are unique.
type Kwargs struct {
	pairs []kwPair
}

type kwPair struct {
	name  string
	value Object
}

// NewKwargs builds a Kwargs from a map, which it does not keep. The keywords
// are held in sorted name order.
func NewKwargs(kwargs map[string]Object) Kwargs {
	if len(kwargs) == 0 {
		return Kwargs{}
	}
	pairs := make([]kwPair, 0, len(kwargs))
	for name, value := range kwargs {
		pairs = append(pairs, kwPair{name, value})
	}
	if len(pairs) > 1 {
		sort.Slice(pairs, func(i, j int) bool { return pairs[i].name < pairs[j].name })
	}
	return Kwargs{pairs: pairs}
}

// NewKwargsFrom builds a Kwargs from parallel slices of names and values,
// copying both, so the slices may be reused once it returns. The names must be
// unique; the keywords keep the order given.
func NewKwargsFrom(names []string, values []Object) Kwargs {
	if len(names) == 0 {
		return Kwargs{}
	}
	pairs := make([]kwPair, len(names))
	for i, name := range names {
		pairs[i] = kwPair{name, values[i]}
	}
	return Kwargs{pairs: pairs}
}

// KwargsBuilder builds a Kwargs one keyword at a time. The zero value is
// ready to use.
type KwargsBuilder struct {
	pairs []kwPair
}

// Grow makes room for n more keywords.
func (b *KwargsBuilder) Grow(n int) {
	if cap(b.pairs)-len(b.pairs) < n {
		pairs := make([]kwPair, len(b.pairs), len(b.pairs)+n)
		copy(pairs, b.pairs)
		b.pairs = pairs
	}
}

// Set adds a keyword, or replaces the value of one already set.
func (b *KwargsBuilder) Set(name string, value Object) {
	for i := range b.pairs {
		if b.pairs[i].name == name {
			b.pairs[i].value = value
			return
		}
	}
	b.pairs = append(b.pairs, kwPair{name, value})
}

// Kwargs returns the keywords set so far and empties the builder, which
// hands its storage to the result.
func (b *KwargsBuilder) Kwargs() Kwargs {
	k := Kwargs{pairs: b.pairs}
	b.pairs = nil
	return k
}

// Merge returns a new Kwargs holding k's keywords followed by those of other,
// a keyword in both taking other's value in k's position.
func (k Kwargs) Merge(other Kwargs) Kwargs {
	if len(other.pairs) == 0 {
		return k
	}
	if len(k.pairs) == 0 {
		return other
	}
	pairs := make([]kwPair, len(k.pairs), len(k.pairs)+len(other.pairs))
	copy(pairs, k.pairs)
outer:
	for _, p := range other.pairs {
		for i := range pairs[:len(k.pairs)] {
			if pairs[i].name == p.name {
				pairs[i].value = p.value
				continue outer
			}
		}
		pairs = append(pairs, p)
	}
	return Kwargs{pairs: pairs}
}

// Lookup returns the value of the keyword and whether it was given.
func (k Kwargs) Lookup(name string) (Object, bool) {
	for i := range k.pairs {
		if k.pairs[i].name == name {
			return k.pairs[i].value, true
		}
	}
	return nil, false
}

// Has returns true if the keyword was given.
func (k Kwargs) Has(name string) bool {
	_, ok := k.Lookup(name)
	return ok
}

// Get returns the value of the keyword, or nil if it was not given.
func (k Kwargs) Get(name string) Object {
	v, _ := k.Lookup(name)
	return v
}

// Keys returns the keyword names, in the Kwargs' order, as a new slice.
func (k Kwargs) Keys() []string {
	keys := make([]string, len(k.pairs))
	for i := range k.pairs {
		keys[i] = k.pairs[i].name
	}
	return keys
}

// Len returns the number of keywords.
func (k Kwargs) Len() int {
	return len(k.pairs)
}

// All iterates over the keywords as name/value pairs, in the Kwargs' order.
func (k Kwargs) All() iter.Seq2[string, Object] {
	return func(yield func(string, Object) bool) {
		for i := range k.pairs {
			if !yield(k.pairs[i].name, k.pairs[i].value) {
				return
			}
		}
	}
}

// Unknown returns the name of the first keyword that is not one of allowed,
// and whether there is one.
func (k Kwargs) Unknown(allowed ...string) (string, bool) {
outer:
	for i := range k.pairs {
		for _, name := range allowed {
			if k.pairs[i].name == name {
				continue outer
			}
		}
		return k.pairs[i].name, true
	}
	return "", false
}

// Map returns the keywords as a new map, which the caller owns; nil when there
// are none.
func (k Kwargs) Map() map[string]Object {
	if len(k.pairs) == 0 {
		return nil
	}
	m := make(map[string]Object, len(k.pairs))
	for i := range k.pairs {
		m[k.pairs[i].name] = k.pairs[i].value
	}
	return m
}

// GetString extracts a string keyword argument with a default value.
func (k Kwargs) GetString(name string, defaultValue string) (string, Object) {
	if obj, ok := k.Lookup(name); ok {
		if s, err := obj.AsString(); err == nil {
			return s, nil
		}
		return defaultValue, &Error{Message: fmt.Sprintf("%s: must be a string", name), ExceptionType: ExceptionTypeTypeError}
	}
	return defaultValue, nil
}

// GetInt extracts an integer keyword argument with a default value.
func (k Kwargs) GetInt(name string, defaultValue int64) (int64, Object) {
	if obj, ok := k.Lookup(name); ok {
		if i, err := obj.AsInt(); err == nil {
			return i, nil
		}
		return defaultValue, &Error{Message: fmt.Sprintf("%s: must be a number", name), ExceptionType: ExceptionTypeTypeError}
	}
	return defaultValue, nil
}

// GetFloat extracts a float keyword argument with a default value.
func (k Kwargs) GetFloat(name string, defaultValue float64) (float64, Object) {
	if obj, ok := k.Lookup(name); ok {
		if f, err := obj.AsFloat(); err == nil {
			return f, nil
		}
		return defaultValue, &Error{Message: fmt.Sprintf("%s: must be a number", name), ExceptionType: ExceptionTypeTypeError}
	}
	return defaultValue, nil
}

// GetBool extracts a boolean keyword argument with a default value.
func (k Kwargs) GetBool(name string, defaultValue bool) (bool, Object) {
	if obj, ok := k.Lookup(name); ok {
		if b, err := obj.AsBool(); err == nil {
			return b, nil
		}
		return defaultValue, &Error{Message: fmt.Sprintf("%s: must be a boolean", name), ExceptionType: ExceptionTypeTypeError}
	}
	return defaultValue, nil
}

// GetList extracts a list keyword argument with a default value.
func (k Kwargs) GetList(name string, defaultValue []Object) ([]Object, Object) {
	if obj, ok := k.Lookup(name); ok {
		if l, err := obj.AsList(); err == nil {
			return l, nil
		}
		return defaultValue, &Error{Message: fmt.Sprintf("%s: must be a list", name), ExceptionType: ExceptionTypeTypeError}
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
