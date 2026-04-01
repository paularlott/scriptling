package object

import (
	"bytes"
	"sort"
	"strings"
)

// Set represents a set of unique objects
type Set struct {
	Elements map[string]Object
}

func (s *Set) Type() ObjectType { return SET_OBJ }
func (s *Set) Inspect() string {
	var out bytes.Buffer
	elements := []string{}
	for _, e := range s.Elements {
		elements = append(elements, e.Inspect())
	}
	// Sort for deterministic output
	sort.Strings(elements)

	out.WriteString("{")
	out.WriteString(strings.Join(elements, ", "))
	out.WriteString("}")
	return out.String()
}

func (s *Set) AsString() (string, Object) { return s.Inspect(), nil }
func (s *Set) AsInt() (int64, Object)     { return 0, errMustBeInteger }
func (s *Set) AsFloat() (float64, Object) { return 0, errMustBeNumber }
func (s *Set) AsBool() (bool, Object)     { return len(s.Elements) > 0, nil }
func (s *Set) AsList() ([]Object, Object) {
	elements := make([]Object, 0, len(s.Elements))
	for _, e := range s.Elements {
		elements = append(elements, e)
	}
	return elements, nil
}
func (s *Set) AsDict() (map[string]Object, Object) { return nil, errMustBeDict }

func (s *Set) CoerceString() (string, Object) { return s.Inspect(), nil }
func (s *Set) CoerceInt() (int64, Object)     { return 0, errMustBeInteger }
func (s *Set) CoerceFloat() (float64, Object) { return 0, errMustBeNumber }

// NewSet creates a new empty Set
func NewSet() *Set {
	return &Set{Elements: make(map[string]Object)}
}

// add adds an element to the set using DictKey for hashing.
// Safe for all scalar types (int, float, bool, string, None, tuple).
// Do NOT use with Instance objects that define __hash__; use
// evalSetAdd (evaluator package) instead, which calls __hash__ via the
// interpreter and then delegates to AddKeyed.
func (s *Set) add(obj Object) {
	s.Elements[DictKey(obj)] = obj
}

// remove removes an element from the set using DictKey for hashing.
// Safe for scalar types only; see add for the Instance caveat.
func (s *Set) remove(obj Object) bool {
	key := DictKey(obj)
	if _, ok := s.Elements[key]; ok {
		delete(s.Elements, key)
		return true
	}
	return false
}

// contains checks if an element is in the set using DictKey for hashing.
// Safe for scalar types only; see add for the Instance caveat.
func (s *Set) contains(obj Object) bool {
	_, ok := s.Elements[DictKey(obj)]
	return ok
}

// Union returns a new set with elements from both sets
func (s *Set) Union(other *Set) *Set {
	result := NewSet()
	for key, e := range s.Elements {
		result.AddKeyed(key, e)
	}
	for key, e := range other.Elements {
		result.AddKeyed(key, e)
	}
	return result
}

// Intersection returns a new set with elements common to both sets
func (s *Set) Intersection(other *Set) *Set {
	result := NewSet()
	for key, e := range s.Elements {
		if _, ok := other.Elements[key]; ok {
			result.AddKeyed(key, e)
		}
	}
	return result
}

// Difference returns a new set with elements in s but not in other
func (s *Set) Difference(other *Set) *Set {
	result := NewSet()
	for key, e := range s.Elements {
		if _, ok := other.Elements[key]; !ok {
			result.AddKeyed(key, e)
		}
	}
	return result
}

// SymmetricDifference returns a new set with elements in either s or other but not both
func (s *Set) SymmetricDifference(other *Set) *Set {
	result := NewSet()
	for key, e := range s.Elements {
		if _, ok := other.Elements[key]; !ok {
			result.AddKeyed(key, e)
		}
	}
	for key, e := range other.Elements {
		if _, ok := s.Elements[key]; !ok {
			result.AddKeyed(key, e)
		}
	}
	return result
}

// IsSubset checks if s is a subset of other
func (s *Set) IsSubset(other *Set) bool {
	if len(s.Elements) > len(other.Elements) {
		return false
	}
	for key := range s.Elements {
		if _, ok := other.Elements[key]; !ok {
			return false
		}
	}
	return true
}

// IsSuperset checks if s is a superset of other
func (s *Set) IsSuperset(other *Set) bool {
	return other.IsSubset(s)
}

// Copy returns a shallow copy of the set
func (s *Set) Copy() *Set {
	result := NewSet()
	for key, e := range s.Elements {
		result.AddKeyed(key, e)
	}
	return result
}

// CreateIterator returns an iterator for the set
func (s *Set) CreateIterator() *Iterator {
	elements := make([]Object, 0, len(s.Elements))
	for _, e := range s.Elements {
		elements = append(elements, e)
	}

	index := 0
	return &Iterator{
		next: func() (Object, bool) {
			if index >= len(elements) {
				return nil, false
			}
			val := elements[index]
			index++
			return val, true
		},
	}
}

// AddKeyed adds an element with a pre-computed key (used when __hash__ is involved)
func (s *Set) AddKeyed(key string, obj Object) {
	s.Elements[key] = obj
}

// ContainsKeyed checks membership using a pre-computed key
func (s *Set) ContainsKeyed(key string) bool {
	_, ok := s.Elements[key]
	return ok
}
