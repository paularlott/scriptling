package object

import (
	"context"
)

// Iterator represents a Python-style iterator
type Iterator struct {
	next     func() (Object, bool) // Returns (value, hasNext)
	consumed bool                  // Track if iterator has been exhausted
}

func (it *Iterator) Type() ObjectType { return ITERATOR_OBJ }
func (it *Iterator) Inspect() string  { return "<iterator>" }

func (it *Iterator) AsString() (string, bool)          { return "", false }
func (it *Iterator) AsInt() (int64, bool)              { return 0, false }
func (it *Iterator) AsFloat() (float64, bool)          { return 0, false }
func (it *Iterator) AsBool() (bool, bool)              { return !it.consumed, true }
func (it *Iterator) AsList() ([]Object, bool)          { return nil, false }
func (it *Iterator) AsDict() (map[string]Object, bool) { return nil, false }

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
		switch iter := iterable.(type) {
		case *List:
			slices[i] = iter.Elements
		case *Tuple:
			slices[i] = iter.Elements
		case *String:
			elements := make([]Object, 0, len(iter.Value))
			for _, ch := range iter.Value {
				elements = append(elements, &String{Value: string(ch)})
			}
			slices[i] = elements
		case *Iterator:
			// Consume iterator into a slice
			elements := make([]Object, 0)
			for {
				val, hasNext := iter.Next()
				if !hasNext {
					break
				}
				elements = append(elements, val)
			}
			slices[i] = elements
		default:
			// Return empty iterator for invalid types
			return &Iterator{
				next: func() (Object, bool) {
					return nil, false
				},
				consumed: true,
			}
		}

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
	var elements []Object

	switch iter := iterable.(type) {
	case *List:
		elements = iter.Elements
	case *Tuple:
		elements = iter.Elements
	case *String:
		elements = make([]Object, 0, len(iter.Value))
		for _, ch := range iter.Value {
			elements = append(elements, &String{Value: string(ch)})
		}
	case *Iterator:
		// Consume iterator into a slice
		elements = make([]Object, 0)
		for {
			val, hasNext := iter.Next()
			if !hasNext {
				break
			}
			elements = append(elements, val)
		}
	default:
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
				result = f.Fn(ctx, nil, elem)
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
	var elements []Object

	switch iter := iterable.(type) {
	case *List:
		elements = iter.Elements
	case *Tuple:
		elements = iter.Elements
	case *String:
		elements = make([]Object, 0, len(iter.Value))
		for _, ch := range iter.Value {
			elements = append(elements, &String{Value: string(ch)})
		}
	case *Iterator:
		// Consume iterator into a slice
		elements = make([]Object, 0)
		for {
			val, hasNext := iter.Next()
			if !hasNext {
				break
			}
			elements = append(elements, val)
		}
	default:
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
					result := f.Fn(ctx, nil, elem)
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
	var elements []Object

	switch iter := iterable.(type) {
	case *List:
		elements = iter.Elements
	case *Tuple:
		elements = iter.Elements
	case *String:
		elements = make([]Object, 0, len(iter.Value))
		for _, ch := range iter.Value {
			elements = append(elements, &String{Value: string(ch)})
		}
	case *Iterator:
		// Consume iterator into a slice
		elements = make([]Object, 0)
		for {
			val, hasNext := iter.Next()
			if !hasNext {
				break
			}
			elements = append(elements, val)
		}
	default:
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
	// Convert iterable to slice
	var elements []Object

	switch iter := iterable.(type) {
	case *List:
		elements = make([]Object, len(iter.Elements))
		copy(elements, iter.Elements)
	case *Tuple:
		elements = make([]Object, len(iter.Elements))
		copy(elements, iter.Elements)
	case *String:
		runes := []rune(iter.Value)
		elements = make([]Object, len(runes))
		for i, r := range runes {
			elements[i] = &String{Value: string(r)}
		}
	case *Iterator:
		// Consume iterator into a slice
		elements = make([]Object, 0)
		for {
			val, hasNext := iter.Next()
			if !hasNext {
				break
			}
			elements = append(elements, val)
		}
	default:
		// Return empty iterator for invalid types
		return &Iterator{
			next: func() (Object, bool) {
				return nil, false
			},
			consumed: true,
		}
	}

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
