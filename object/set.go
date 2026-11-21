package object

import (
	"iter"
	"math"
)

// Set represents a set of unique objects. A frozen set (Python frozenset)
// is a Set with Frozen set: it rejects mutation, hashes by content, and
// prints as frozenset(...). The algebra methods below carry the receiver's
// frozenness to their result, so frozen | set stays frozen and set | frozen
// stays mutable, matching Python's left-operand rule.
//
// A small set keeps its elements in a slice searched linearly, which needs
// no hashing and far less memory than a map; once it outgrows setSmallMax it
// moves to a map from canonical key to element and stays there.
type Set struct {
	// small holds the elements while m is nil.
	small []setEntry
	// m holds the elements once the set has outgrown small; nil until then.
	m      map[string]Object
	Frozen bool
	// charged is the element count already charged to the interpreter's
	// memory accounting; it fits in Frozen's padding, so it costs no space.
	charged uint32
}

// setEntry is one element of a small set with its canonical key.
type setEntry struct {
	key string
	val Object
}

// setSmallMax is the largest number of elements kept in the linearly
// searched slice before the set moves to a map.
const setSmallMax = 8

// SetSmallMax is setSmallMax for callers outside the package.
const SetSmallMax = setSmallMax

func (s *Set) Type() ObjectType { return SET_OBJ }
func (s *Set) Inspect() string  { return inspectContainer(s) }

func (s *Set) AsString() (string, Object)          { return s.Inspect(), nil }
func (s *Set) AsInt() (int64, Object)              { return 0, errMustBeInteger }
func (s *Set) AsFloat() (float64, Object)          { return 0, errMustBeNumber }
func (s *Set) AsBool() (bool, Object)              { return s.Len() > 0, nil }
func (s *Set) AsList() ([]Object, Object)          { return s.Values(), nil }
func (s *Set) AsDict() (map[string]Object, Object) { return nil, errMustBeDict }

func (s *Set) CoerceString() (string, Object) { return s.Inspect(), nil }
func (s *Set) CoerceInt() (int64, Object)     { return 0, errMustBeInteger }
func (s *Set) CoerceFloat() (float64, Object) { return 0, errMustBeNumber }

// NewSet creates a new empty Set
func NewSet() *Set {
	return &Set{}
}

// NewSetSized creates a new empty Set with room for n distinct elements.
// When the elements may repeat heavily, cap n at SetSmallMax so a large
// set grows only as distinct elements arrive.
func NewSetSized(n int) *Set {
	if n > setSmallMax {
		return &Set{m: make(map[string]Object, n)}
	}
	if n > 0 {
		return &Set{small: make([]setEntry, 0, n)}
	}
	return &Set{}
}

// NewFrozenSet creates a new empty frozen Set (Python frozenset).
func NewFrozenSet() *Set {
	return &Set{Frozen: true}
}

// Freeze returns s as a frozen set, sharing its elements.
func (s *Set) Freeze() *Set {
	s.Frozen = true
	return s
}

// Len returns the number of elements.
func (s *Set) Len() int {
	if s.m != nil {
		return len(s.m)
	}
	return len(s.small)
}

// find returns the position in small of a canonical key, or -1. Only for a
// set still in its small form.
func (s *Set) find(key string) int {
	n := len(key)
	if n == 0 {
		for i := range s.small {
			if s.small[i].key == "" {
				return i
			}
		}
		return -1
	}
	// Keys of the same kind share a prefix and usually differ at the end
	// (i:7 against i:8), so checking the last byte first skips most of the
	// full comparisons.
	last := key[n-1]
	for i := range s.small {
		if k := s.small[i].key; len(k) == n && k[n-1] == last && k == key {
			return i
		}
	}
	return -1
}

// Lookup returns the element stored under a canonical key.
func (s *Set) Lookup(key string) (Object, bool) {
	if s.m != nil {
		e, ok := s.m[key]
		return e, ok
	}
	if i := s.find(key); i >= 0 {
		return s.small[i].val, true
	}
	return nil, false
}

// ContainsKeyed checks membership using a pre-computed key
func (s *Set) ContainsKeyed(key string) bool {
	if s.m != nil {
		_, ok := s.m[key]
		return ok
	}
	return s.find(key) >= 0
}

// AddKeyed adds an element with a pre-computed key (used when __hash__ is
// involved). Adding a key already present replaces its element, which saves
// a lookup on every add; Python keeps the original, a difference seen only
// between equal values of different types ({2} with 2.0 added becomes {2.0}).
func (s *Set) AddKeyed(key string, obj Object) {
	if s.m != nil {
		s.m[key] = obj
	} else if i := s.find(key); i >= 0 {
		s.small[i].val = obj
		return
	} else if len(s.small) < setSmallMax {
		if s.small == nil {
			s.small = make([]setEntry, 0, 4)
		}
		s.small = append(s.small, setEntry{key, obj})
	} else {
		m := make(map[string]Object, 2*setSmallMax)
		for _, e := range s.small {
			m[e.key] = e.val
		}
		m[key] = obj
		s.m, s.small = m, nil
	}
	if accountingOn.Load() {
		s.noteGrowth()
	}
}

// DeleteKeyed removes the element with a canonical key and reports whether it
// was present.
func (s *Set) DeleteKeyed(key string) bool {
	if s.m != nil {
		if _, ok := s.m[key]; ok {
			delete(s.m, key)
			return true
		}
		return false
	}
	i := s.find(key)
	if i < 0 {
		return false
	}
	last := len(s.small) - 1
	s.small[i] = s.small[last]
	s.small[last] = setEntry{}
	s.small = s.small[:last]
	return true
}

// Clear removes every element.
func (s *Set) Clear() {
	s.small, s.m = nil, nil
}

// All iterates over the elements with their canonical keys, in no particular
// order. Deleting the element just produced is safe; any other change to the
// set during the loop is not (use Values or Keys for a snapshot).
func (s *Set) All() iter.Seq2[string, Object] {
	return func(yield func(string, Object) bool) {
		if s.m != nil {
			for k, e := range s.m {
				if !yield(k, e) {
					return
				}
			}
			return
		}
		// Backwards, so deleting the current element (which moves the last,
		// already produced one into its place) skips nothing.
		for i := len(s.small) - 1; i >= 0; i-- {
			if i >= len(s.small) {
				continue
			}
			e := s.small[i]
			if !yield(e.key, e.val) {
				return
			}
		}
	}
}

// Values returns a snapshot of the elements, in no particular order.
func (s *Set) Values() []Object {
	out := make([]Object, 0, s.Len())
	for _, e := range s.All() {
		out = append(out, e)
	}
	return out
}

// Keys returns a snapshot of the canonical keys, in no particular order.
func (s *Set) Keys() []string {
	out := make([]string, 0, s.Len())
	for k := range s.All() {
		out = append(out, k)
	}
	return out
}

// add adds an element to the set using DictKey for hashing.
// Safe for all scalar types (int, float, bool, string, None, tuple).
// Do NOT use with Instance objects that define __hash__; use
// evalSetAdd (evaluator package) instead, which calls __hash__ via the
// interpreter and then delegates to AddKeyed.
func (s *Set) add(obj Object) {
	s.AddKeyed(DictKey(obj), obj)
}

// setEntryWeight is the estimated memory per set element charged against an
// interpreter's memory limit: the map entry, its key string and bucket
// overhead.
const setEntryWeight = 64

// SetEntryWeight is setEntryWeight for callers outside the package.
const SetEntryWeight = setEntryWeight

// noteGrowth charges map growth the way a map grows: when the element count
// passes what was charged, the charge doubles.
func (s *Set) noteGrowth() {
	n := uint32(min(s.Len(), math.MaxUint32/2))
	if n <= s.charged {
		return
	}
	next := max(s.charged*2, 8)
	for next < n {
		next *= 2
	}
	trackAlloc(s, int64(next-s.charged)*setEntryWeight)
	s.charged = next
}

// remove removes an element from the set using DictKey for hashing.
// Safe for scalar types only; see add for the Instance caveat.
func (s *Set) remove(obj Object) bool {
	return s.DeleteKeyed(DictKey(obj))
}

// contains checks if an element is in the set using DictKey for hashing.
// Safe for scalar types only; see add for the Instance caveat.
func (s *Set) contains(obj Object) bool {
	return s.ContainsKeyed(DictKey(obj))
}

// Union returns a new set with elements from both sets
func (s *Set) Union(other *Set) *Set {
	result := NewSet()
	if n := s.Len() + other.Len(); n <= setSmallMax {
		result = NewSetSized(n)
	}
	for key, e := range s.All() {
		result.AddKeyed(key, e)
	}
	for key, e := range other.All() {
		result.AddKeyed(key, e)
	}
	result.Frozen = s.Frozen
	return result
}

// InPlaceUnion merges other into s (Python's s |= other). Aliases of s
// observe the update.
func (s *Set) InPlaceUnion(other *Set) {
	if other == s {
		return
	}
	for key, e := range other.All() {
		s.AddKeyed(key, e)
	}
}

// InPlaceIntersection keeps only elements of s that are also in other
// (Python's s &= other).
func (s *Set) InPlaceIntersection(other *Set) {
	if other == s {
		return
	}
	for key := range s.All() {
		if !other.ContainsKeyed(key) {
			s.DeleteKeyed(key)
		}
	}
}

// InPlaceDifference removes other's elements from s (Python's s -= other).
func (s *Set) InPlaceDifference(other *Set) {
	if other == s {
		s.Clear()
		return
	}
	for key := range other.All() {
		s.DeleteKeyed(key)
	}
}

// InPlaceSymmetricDifference keeps elements in exactly one of s and other
// (Python's s ^= other).
func (s *Set) InPlaceSymmetricDifference(other *Set) {
	if other == s {
		s.Clear()
		return
	}
	for key, e := range other.All() {
		if !s.DeleteKeyed(key) {
			s.AddKeyed(key, e)
		}
	}
}

// Intersection returns a new set with elements common to both sets
func (s *Set) Intersection(other *Set) *Set {
	result := NewSet()
	for key, e := range s.All() {
		if other.ContainsKeyed(key) {
			result.AddKeyed(key, e)
		}
	}
	result.Frozen = s.Frozen
	return result
}

// Difference returns a new set with elements in s but not in other
func (s *Set) Difference(other *Set) *Set {
	result := NewSet()
	for key, e := range s.All() {
		if !other.ContainsKeyed(key) {
			result.AddKeyed(key, e)
		}
	}
	result.Frozen = s.Frozen
	return result
}

// SymmetricDifference returns a new set with elements in either s or other but not both
func (s *Set) SymmetricDifference(other *Set) *Set {
	result := NewSet()
	for key, e := range s.All() {
		if !other.ContainsKeyed(key) {
			result.AddKeyed(key, e)
		}
	}
	for key, e := range other.All() {
		if !s.ContainsKeyed(key) {
			result.AddKeyed(key, e)
		}
	}
	result.Frozen = s.Frozen
	return result
}

// IsSubset checks if s is a subset of other
func (s *Set) IsSubset(other *Set) bool {
	if s.Len() > other.Len() {
		return false
	}
	for key := range s.All() {
		if !other.ContainsKeyed(key) {
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
	result := NewSetSized(s.Len())
	for key, e := range s.All() {
		result.AddKeyed(key, e)
	}
	result.Frozen = s.Frozen
	return result
}

// CreateIterator returns an iterator for the set. It iterates a snapshot,
// so changing the set during the loop is safe.
func (s *Set) CreateIterator() *Iterator {
	elements := s.Values()
	index := 0
	return trackIterator(&Iterator{
		next: func() (Object, bool) {
			if index >= len(elements) {
				return nil, false
			}
			val := elements[index]
			index++
			return val, true
		},
	})
}
