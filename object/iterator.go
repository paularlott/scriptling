package object

import (
	"context"
)

// Iterator represents a Python-style iterator
type Iterator struct {
	next     func() (Object, bool) // Returns (value, hasNext)
	consumed bool                  // Track if iterator has been exhausted
}

// IterableToSlice converts any iterable object (List, Tuple, String, Iterator, Set) to a slice of Objects.
// Returns (elements, ok) where ok is true if the conversion succeeded.
// For strings, each character becomes a String object.
// For iterators, this consumes the iterator.
// For dicts, returns the keys (like Python's list(dict)).
func IterableToSlice(obj Object) ([]Object, bool) {
	switch iter := obj.(type) {
	case *List:
		return iter.Elements, true
	case *Tuple:
		return iter.Elements, true
	case *String:
		elements := make([]Object, 0, len(iter.Value))
		for _, ch := range iter.Value {
			elements = append(elements, &String{Value: string(ch)})
		}
		return elements, true
	case *Iterator:
		elements := make([]Object, 0)
		for {
			val, hasNext := iter.Next()
			if !hasNext {
				break
			}
			elements = append(elements, val)
		}
		return elements, true
	case *Set:
		elements := make([]Object, 0, len(iter.Elements))
		for _, v := range iter.Elements {
			elements = append(elements, v)
		}
		return elements, true
	case *Dict:
		// For dicts, return keys (like Python's list(dict))
		elements := make([]Object, 0, len(iter.Pairs))
		for _, p := range iter.Pairs {
			elements = append(elements, p.Key)
		}
		return elements, true
	case *DictKeys:
		elements := make([]Object, 0, len(iter.Dict.Pairs))
		for _, p := range iter.Dict.Pairs {
			elements = append(elements, p.Key)
		}
		return elements, true
	case *DictValues:
		elements := make([]Object, 0, len(iter.Dict.Pairs))
		for _, p := range iter.Dict.Pairs {
			elements = append(elements, p.Value)
		}
		return elements, true
	case *DictItems:
		elements := make([]Object, 0, len(iter.Dict.Pairs))
		for _, p := range iter.Dict.Pairs {
			elements = append(elements, &Tuple{Elements: []Object{p.Key, p.Value}})
		}
		return elements, true
	default:
		return nil, false
	}
}

func (it *Iterator) Type() ObjectType { return ITERATOR_OBJ }
func (it *Iterator) Inspect() string  { return "<iterator>" }

func (it *Iterator) AsString() (string, Object)          { return "", &Error{Message: ErrMustBeString} }
func (it *Iterator) AsInt() (int64, Object)              { return 0, &Error{Message: ErrMustBeInteger} }
func (it *Iterator) AsFloat() (float64, Object)          { return 0, &Error{Message: ErrMustBeNumber} }
func (it *Iterator) AsBool() (bool, Object)              { return !it.consumed, nil }
func (it *Iterator) AsList() ([]Object, Object)          { return nil, &Error{Message: ErrMustBeList} }
func (it *Iterator) AsDict() (map[string]Object, Object) { return nil, &Error{Message: ErrMustBeDict} }

func (it *Iterator) CoerceString() (string, Object) { return it.Inspect(), nil }
func (it *Iterator) CoerceInt() (int64, Object)     { return 0, &Error{Message: ErrMustBeInteger} }
func (it *Iterator) CoerceFloat() (float64, Object) { return 0, &Error{Message: ErrMustBeNumber} }

// Next returns the next value from the iterator
func (it *Iterator) Next() (Object, bool) {
	if it.consumed {
		return nil, false
	}
	val, hasNext := it.next()
	if !hasNext {
		it.consumed = true
	}
	return val, hasNext
}

// NewIterator creates an iterator with a custom next function
// This allows creating iterators that can call functions with proper context
func NewIterator(nextFn func() (Object, bool)) *Iterator {
	return &Iterator{
		next: nextFn,
	}
}

// RangeIterator creates an iterator for range(start, stop, step)
func NewRangeIterator(start, stop, step int64) *Iterator {
	current := start

	return &Iterator{
		next: func() (Object, bool) {
			if step > 0 {
				if current >= stop {
					return nil, false
				}
			} else {
				if current <= stop {
					return nil, false
				}
			}

			val := NewInteger(current)
			current += step
			return val, true
		},
	}
}

// ZipIterator creates an iterator that zips multiple iterables together
func NewZipIterator(iterables []Object) *Iterator {
	// Convert all iterables to slices
	slices := make([][]Object, len(iterables))
	minLen := -1

	for i, iterable := range iterables {
		elements, ok := IterableToSlice(iterable)
		if !ok {
			// Return empty iterator for invalid types
			return &Iterator{
				next: func() (Object, bool) {
					return nil, false
				},
				consumed: true,
			}
		}
		slices[i] = elements

		if minLen == -1 || len(slices[i]) < minLen {
			minLen = len(slices[i])
		}
	}

	index := 0

	return &Iterator{
		next: func() (Object, bool) {
			if index >= minLen {
				return nil, false
			}

			tuple := make([]Object, len(slices))
			for j := range slices {
				tuple[j] = slices[j][index]
			}
			index++

			return &Tuple{Elements: tuple}, true
		},
	}
}

// MapIterator creates an iterator that applies a function to each element
func NewMapIterator(ctx context.Context, fn Object, iterable Object) *Iterator {
	// Convert iterable to slice
	elements, ok := IterableToSlice(iterable)
	if !ok {
		// Return empty iterator for invalid types
		return &Iterator{
			next: func() (Object, bool) {
				return nil, false
			},
			consumed: true,
		}
	}

	index := 0

	return &Iterator{
		next: func() (Object, bool) {
			if index >= len(elements) {
				return nil, false
			}

			elem := elements[index]
			index++

			// Apply function
			var result Object
			switch f := fn.(type) {
			case *Builtin:
				result = f.Fn(ctx, NewKwargs(nil), elem)
			case *Function:
				// Would need evaluator to call function properly
				// For now, return the element unchanged
				result = elem
			case *LambdaFunction:
				// Would need evaluator to call lambda properly
				// For now, return the element unchanged
				result = elem
			default:
				result = elem
			}

			return result, true
		},
	}
}

// FilterIterator creates an iterator that filters elements based on a predicate
func NewFilterIterator(ctx context.Context, fn Object, iterable Object) *Iterator {
	// Convert iterable to slice
	elements, ok := IterableToSlice(iterable)
	if !ok {
		// Return empty iterator for invalid types
		return &Iterator{
			next: func() (Object, bool) {
				return nil, false
			},
			consumed: true,
		}
	}

	index := 0

	return &Iterator{
		next: func() (Object, bool) {
			for index < len(elements) {
				elem := elements[index]
				index++

				// Apply filter function
				var passes bool
				switch f := fn.(type) {
				case *Builtin:
					result := f.Fn(ctx, NewKwargs(nil), elem)
					if b, ok := result.(*Boolean); ok {
						passes = b.Value
					} else {
						// Truthy check
						passes = isTruthy(result)
					}
				case *Function, *LambdaFunction:
					// Would need evaluator to call function properly
					// For now, include all elements
					passes = true
				default:
					passes = true
				}

				if passes {
					return elem, true
				}
			}

			return nil, false
		},
	}
}

// isTruthy checks if an object is truthy
func isTruthy(obj Object) bool {
	switch o := obj.(type) {
	case *Boolean:
		return o.Value
	case *Null:
		return false
	case *Integer:
		return o.Value != 0
	case *Float:
		return o.Value != 0
	case *String:
		return o.Value != ""
	case *List:
		return len(o.Elements) > 0
	case *Tuple:
		return len(o.Elements) > 0
	case *Dict:
		return len(o.Pairs) > 0
	default:
		return true
	}
}

// EnumerateIterator creates an iterator that returns (index, value) tuples
func NewEnumerateIterator(iterable Object, start int64) *Iterator {
	// Convert iterable to slice
	elements, ok := IterableToSlice(iterable)
	if !ok {
		// Return empty iterator for invalid types
		return &Iterator{
			next: func() (Object, bool) {
				return nil, false
			},
			consumed: true,
		}
	}

	index := 0

	return &Iterator{
		next: func() (Object, bool) {
			if index >= len(elements) {
				return nil, false
			}

			tuple := &Tuple{Elements: []Object{
				NewInteger(start + int64(index)),
				elements[index],
			}}
			index++

			return tuple, true
		},
	}
}

// ReversedIterator creates an iterator that returns elements in reverse order
func NewReversedIterator(iterable Object) *Iterator {
	// Convert iterable to slice - need to copy to avoid modifying original
	srcElements, ok := IterableToSlice(iterable)
	if !ok {
		// Return empty iterator for invalid types
		return &Iterator{
			next: func() (Object, bool) {
				return nil, false
			},
			consumed: true,
		}
	}

	// Make a copy so we don't affect the original
	elements := make([]Object, len(srcElements))
	copy(elements, srcElements)

	index := len(elements) - 1

	return &Iterator{
		next: func() (Object, bool) {
			if index < 0 {
				return nil, false
			}

			val := elements[index]
			index--

			return val, true
		},
	}
}
