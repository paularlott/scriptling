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

func (s *Set) AsString() (string, bool) { return s.Inspect(), true }
func (s *Set) AsInt() (int64, bool)     { return 0, false }
func (s *Set) AsFloat() (float64, bool) { return 0, false }
func (s *Set) AsBool() (bool, bool)     { return len(s.Elements) > 0, true }
func (s *Set) AsList() ([]Object, bool) {
	elements := make([]Object, 0, len(s.Elements))
	for _, e := range s.Elements {
		elements = append(elements, e)
	}
	return elements, true
}
func (s *Set) AsDict() (map[string]Object, bool) { return nil, false }

// NewSet creates a new empty Set
func NewSet() *Set {
	return &Set{Elements: make(map[string]Object)}
}

// NewSetFromElements creates a new Set from a slice of objects
func NewSetFromElements(elements []Object) *Set {
	s := &Set{Elements: make(map[string]Object)}
	for _, e := range elements {
		s.Add(e)
	}
	return s
}

// Add adds an element to the set
func (s *Set) Add(obj Object) {
	s.Elements[obj.Inspect()] = obj
}

// Remove removes an element from the set
func (s *Set) Remove(obj Object) bool {
	key := obj.Inspect()
	if _, ok := s.Elements[key]; ok {
		delete(s.Elements, key)
		return true
	}
	return false
}

// Contains checks if an element is in the set
func (s *Set) Contains(obj Object) bool {
	_, ok := s.Elements[obj.Inspect()]
	return ok
}

// Union returns a new set with elements from both sets
func (s *Set) Union(other *Set) *Set {
	result := NewSet()
	for _, e := range s.Elements {
		result.Add(e)
	}
	for _, e := range other.Elements {
		result.Add(e)
	}
	return result
}

// Intersection returns a new set with elements common to both sets
func (s *Set) Intersection(other *Set) *Set {
	result := NewSet()
	for key, e := range s.Elements {
		if _, ok := other.Elements[key]; ok {
			result.Add(e)
		}
	}
	return result
}

// Difference returns a new set with elements in s but not in other
func (s *Set) Difference(other *Set) *Set {
	result := NewSet()
	for key, e := range s.Elements {
		if _, ok := other.Elements[key]; !ok {
			result.Add(e)
		}
	}
	return result
}

// SymmetricDifference returns a new set with elements in either s or other but not both
func (s *Set) SymmetricDifference(other *Set) *Set {
	result := NewSet()
	for key, e := range s.Elements {
		if _, ok := other.Elements[key]; !ok {
			result.Add(e)
		}
	}
	for key, e := range other.Elements {
		if _, ok := s.Elements[key]; !ok {
			result.Add(e)
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
	for _, e := range s.Elements {
		result.Add(e)
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
