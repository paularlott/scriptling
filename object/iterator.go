package object

import "fmt"

// Iterator represents a Python-style iterator
type Iterator struct {
	next     func() (Object, bool) // Returns (value, hasNext)
	consumed bool                  // Track if iterator has been exhausted
	// length is the element count when statically known (range); -1
	// otherwise. len() reports it, matching Python where only range among
	// the lazy iterators has a length.
	length int64
	// infinite marks an iterator that never ends (itertools count, cycle,
	// repeat and lazy wrappers of them), so collecting it into a list
	// fails fast instead of allocating until memory runs out.
	infinite bool
	// rangeState, for range iterators, reports the next value, the stop
	// and the step, so the values left can be indexed without iterating.
	rangeState func() (next, stop, step int64)
}

// RangeRemaining reports, for a range iterator, the values it has yet to
// produce: next, next+step, ... (n of them). ok is false for other iterators.
func (it *Iterator) RangeRemaining() (next, step, n int64, ok bool) {
	if it.rangeState == nil || it.consumed {
		return 0, 0, 0, false
	}
	next, stop, step := it.rangeState()
	if step > 0 && stop > next {
		n = (stop - next + step - 1) / step
	} else if step < 0 && stop < next {
		n = (next - stop - step - 1) / -step
	}
	return next, step, n, true
}

// InfiniteIteratorMessage is the error for collecting an infinite iterator.
const InfiniteIteratorMessage = "cannot collect an infinite iterator (itertools count, cycle or repeat); bound it with itertools.islice() or zip(), or break out of a for loop"

// Infinite reports whether the iterator never ends.
func (it *Iterator) Infinite() bool {
	return it.infinite
}

// NewInfiniteIterator creates an iterator that never ends.
func NewInfiniteIterator(nextFn func() (Object, bool)) *Iterator {
	return &Iterator{next: nextFn, length: -1, infinite: true}
}

// Len returns the iterator's length when statically known (range iterators).
func (it *Iterator) Len() (int64, bool) {
	if it.length >= 0 && !it.consumed {
		return it.length, true
	}
	return 0, false
}

// IterableToSlice converts any iterable object (List, Tuple, String, Iterator, Set) to a slice of Objects.
// Returns (elements, ok) where ok is true if the conversion succeeded.
// For strings, each character becomes a String object.
// For iterators, this consumes the iterator.
// For dicts, returns the keys (like Python's list(dict)).
func IterableToSlice(obj Object) ([]Object, bool) {
	switch iter := obj.(type) {
	case *Class:
		// Enum classes iterate over their members, in definition order.
		if iter.IsEnum {
			return iter.EnumMembers, true
		}
		return nil, false
	case *List:
		return iter.Elements, true
	case *Tuple:
		return iter.Elements, true
	case *String:
		elements := make([]Object, 0, len(iter.value))
		for _, ch := range iter.value {
			elements = append(elements, &String{value: string(ch)})
		}
		return elements, true
	case *Bytes:
		elements := make([]Object, len(iter.value))
		for i, b := range iter.value {
			elements[i] = NewInteger(int64(b))
		}
		return elements, true
	case *Iterator:
		if iter.infinite {
			return nil, false
		}
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
		// For dicts, return keys (like Python's list(dict)), in insertion order
		elements := make([]Object, 0, len(iter.Pairs))
		for _, p := range iter.OrderedPairs() {
			elements = append(elements, p.Key)
		}
		return elements, true
	case *DictKeys:
		elements := make([]Object, 0, len(iter.Dict.Pairs))
		for _, p := range iter.Dict.OrderedPairs() {
			elements = append(elements, p.Key)
		}
		return elements, true
	case *DictValues:
		elements := make([]Object, 0, len(iter.Dict.Pairs))
		for _, p := range iter.Dict.OrderedPairs() {
			elements = append(elements, p.Value)
		}
		return elements, true
	case *DictItems:
		elements := make([]Object, 0, len(iter.Dict.Pairs))
		for _, p := range iter.Dict.OrderedPairs() {
			elements = append(elements, &Tuple{Elements: []Object{p.Key, p.Value}})
		}
		return elements, true
	case *FloatArray:
		if iter.Is2D() {
			rows := iter.Rows()
			cols := iter.Cols()
			elements := make([]Object, rows)
			for i := 0; i < rows; i++ {
				off := i * cols
				rowData := make([]float64, cols)
				copy(rowData, iter.Data[off:off+cols])
				elements[i] = NewFloatArray1D(rowData)
			}
			return elements, true
		}
		elements := make([]Object, len(iter.Data))
		for i, v := range iter.Data {
			elements[i] = &Float{value: v}
		}
		return elements, true
	default:
		return nil, false
	}
}

func (it *Iterator) Type() ObjectType { return ITERATOR_OBJ }
func (it *Iterator) Inspect() string  { return "<iterator>" }

func (it *Iterator) AsString() (string, Object)          { return "", errMustBeString }
func (it *Iterator) AsInt() (int64, Object)              { return 0, errMustBeInteger }
func (it *Iterator) AsFloat() (float64, Object)          { return 0, errMustBeNumber }
func (it *Iterator) AsBool() (bool, Object)              { return !it.consumed, nil }
func (it *Iterator) AsList() ([]Object, Object)          { return nil, errMustBeList }
func (it *Iterator) AsDict() (map[string]Object, Object) { return nil, errMustBeDict }

func (it *Iterator) CoerceString() (string, Object) { return it.Inspect(), nil }
func (it *Iterator) CoerceInt() (int64, Object)     { return 0, errMustBeInteger }
func (it *Iterator) CoerceFloat() (float64, Object) { return 0, errMustBeNumber }

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
		next:   nextFn,
		length: -1,
	}
}

// RangeIterator creates an iterator for range(start, stop, step). Range is
// the one lazy iterator with a known length (Python defines len() for it and
// nothing else lazy), so the count is computed up front.
func NewRangeIterator(start, stop, step int64) *Iterator {
	current := start

	length := int64(0)
	if step > 0 && stop > start {
		length = (stop - start + step - 1) / step
	} else if step < 0 && stop < start {
		length = (start - stop - step - 1) / -step
	}

	return &Iterator{
		length:     length,
		rangeState: func() (int64, int64, int64) { return current, stop, step },
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

// IsPropagating reports whether a value yielded by an iterator is an internal
// error or a raised exception that the consumer must propagate.
func IsPropagating(obj Object) bool { return isPropagatingValue(obj) }

// isPropagatingValue reports whether a value yielded by an iterator is an
// internal error or a raised exception that the consumer must propagate.
func isPropagatingValue(obj Object) bool {
	if IsError(obj) {
		return true
	}
	ex, ok := obj.(*Exception)
	return ok && ex.Raised
}

// IterSource returns a pull function over iterable. Iterators are consumed
// lazily (so infinite iterators such as itertools.cycle work, as in Python);
// other iterables are materialized once. ok is false for non-iterables.
func IterSource(iterable Object) (func() (Object, bool), bool) {
	if it, isIter := iterable.(*Iterator); isIter {
		return it.Next, true
	}
	elements, ok := IterableToSlice(iterable)
	if !ok {
		return nil, false
	}
	index := 0
	return func() (Object, bool) {
		if index >= len(elements) {
			return nil, false
		}
		v := elements[index]
		index++
		return v, true
	}, true
}

// isInfinite reports whether obj is an iterator that never ends.
func isInfinite(obj Object) bool {
	it, ok := obj.(*Iterator)
	return ok && it.infinite
}

// emptyIterator returns an already-exhausted iterator.
func emptyIterator() *Iterator {
	return &Iterator{
		next: func() (Object, bool) {
			return nil, false
		},
		consumed: true,
		length:   -1,
	}
}

// ZipIterator creates an iterator that zips multiple iterables together.
// Iterator inputs are pulled lazily, left to right, stopping at the first
// exhausted input (Python semantics). An error or raised exception yielded
// by an input is passed through unwrapped so the consumer propagates it.
func NewZipIterator(iterables []Object) *Iterator {
	return newZipIterator(iterables, false)
}

// NewStrictZipIterator is zip(..., strict=True): inputs of different lengths
// raise ValueError when the shortest one runs out.
func NewStrictZipIterator(iterables []Object) *Iterator {
	return newZipIterator(iterables, true)
}

// zipLengthError is Python's strict zip() error for argument j (0-based)
// ending before (shorter) or after (longer) the arguments before it.
func zipLengthError(j int, shorter bool) Object {
	which := "longer"
	if shorter {
		which = "shorter"
	}
	others := "argument 1"
	if j > 1 {
		others = fmt.Sprintf("arguments 1-%d", j)
	}
	return &Exception{
		Message:       fmt.Sprintf("zip() argument %d is %s than %s", j+1, which, others),
		ExceptionType: ExceptionTypeValueError,
		Raised:        true,
	}
}

func newZipIterator(iterables []Object, strict bool) *Iterator {
	sources := make([]func() (Object, bool), len(iterables))
	for i, iterable := range iterables {
		src, ok := IterSource(iterable)
		if !ok {
			return emptyIterator()
		}
		sources[i] = src
	}
	if len(sources) == 0 {
		return emptyIterator()
	}
	done := false
	allInfinite := true
	for _, iterable := range iterables {
		allInfinite = allInfinite && isInfinite(iterable)
	}
	it := NewIterator(func() (Object, bool) {
		if done {
			return nil, false
		}
		tuple := make([]Object, len(sources))
		for j, src := range sources {
			v, ok := src()
			if !ok {
				done = true
				if !strict {
					return nil, false
				}
				if j > 0 {
					return zipLengthError(j, true), true
				}
				// The first input ended: every other one must end too.
				for k := 1; k < len(sources); k++ {
					if _, more := sources[k](); more {
						return zipLengthError(k, false), true
					}
				}
				return nil, false
			}
			if isPropagatingValue(v) {
				done = true
				return v, true
			}
			tuple[j] = v
		}
		return &Tuple{Elements: tuple}, true
	})
	it.infinite = allInfinite
	return it
}

// EnumerateIterator creates an iterator of (index, value) tuples. Iterator
// inputs are pulled lazily so infinite iterators can be enumerated.
func NewEnumerateIterator(iterable Object, start int64) *Iterator {
	src, ok := IterSource(iterable)
	if !ok {
		return emptyIterator()
	}
	index := start
	it := NewIterator(func() (Object, bool) {
		v, ok := src()
		if !ok {
			return nil, false
		}
		if isPropagatingValue(v) {
			return v, true
		}
		tuple := &Tuple{Elements: []Object{NewInteger(index), v}}
		index++
		return tuple, true
	})
	it.infinite = isInfinite(iterable)
	return it
}

// ReversedIterator creates an iterator that returns elements in reverse order
func NewReversedIterator(iterable Object) *Iterator {
	if fa, ok := iterable.(*FloatArray); ok {
		if fa.Is2D() {
			index := fa.Rows() - 1
			cols := fa.Cols()
			return &Iterator{
				next: func() (Object, bool) {
					if index < 0 {
						return nil, false
					}
					off := index * cols
					rowData := make([]float64, cols)
					copy(rowData, fa.Data[off:off+cols])
					index--
					return NewFloatArray1D(rowData), true
				},
			}
		}
		index := len(fa.Data) - 1
		return &Iterator{
			next: func() (Object, bool) {
				if index < 0 {
					return nil, false
				}
				val := &Float{value: fa.Data[index]}
				index--
				return val, true
			},
		}
	}

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
