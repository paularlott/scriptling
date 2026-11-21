package stdlib

import (
	"context"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// The bisect and heapq modules, as in Python: binary search and insertion
// in sorted lists, and a binary min-heap kept in a list. Elements compare
// with the language's < (so tuples, strings and instances with __lt__
// work), and key= functions are called through the evaluator.

// pyLess is a < b with Python's semantics; a raise is returned.
func pyLess(ctx context.Context, a, b object.Object) (bool, object.Object) {
	ops, errObj := operators(ctx)
	if errObj != nil {
		return false, errObj
	}
	r := ops.BinaryOp(ctx, "<", a, b)
	if object.IsPropagating(r) || object.IsError(r) {
		return false, r
	}
	return ops.Truthy(ctx, r)
}

func listArg(name string, o object.Object) (*object.List, object.Object) {
	l, ok := o.(*object.List)
	if !ok {
		return nil, errors.NewTypeErrorTagged("%s() argument must be list, not %s", name, object.PythonTypeName(o))
	}
	return l, nil
}

// bisectArgs reads (a, x, lo=0, hi=len(a), *, key=None).
func bisectArgs(name string, args []object.Object, kwargs object.Kwargs) (*object.List, object.Object, int, int, object.Object, object.Object) {
	if kwargs.Len() > 0 {
		if err := errors.CheckKwargs(name, kwargs, "lo", "hi", "key"); err != nil {
			return nil, nil, 0, 0, nil, err
		}
	}
	if len(args) < 2 || len(args) > 4 {
		return nil, nil, 0, 0, nil, errors.NewTypeErrorTagged("%s() takes from 2 to 4 positional arguments (%d given)", name, len(args))
	}
	l, errObj := listArg(name, args[0])
	if errObj != nil {
		return nil, nil, 0, 0, nil, errObj
	}
	lo, hi := 0, len(l.Elements)
	intArg := func(o object.Object) (int, object.Object) {
		if o == nil || o.Type() == object.NULL_OBJ {
			return -1, nil
		}
		n, ok := o.(*object.Integer)
		if !ok {
			return 0, errors.NewTypeErrorTagged("'%s' object cannot be interpreted as an integer", object.PythonTypeName(o))
		}
		return int(n.IntValue()), nil
	}
	loObj, hiObj := kwargs.Get("lo"), kwargs.Get("hi")
	if len(args) > 2 {
		loObj = args[2]
	}
	if len(args) > 3 {
		hiObj = args[3]
	}
	if loObj != nil {
		v, errObj := intArg(loObj)
		if errObj != nil {
			return nil, nil, 0, 0, nil, errObj
		}
		if v < 0 {
			return nil, nil, 0, 0, nil, errors.NewValueError("lo must be non-negative")
		}
		lo = v
	}
	if hiObj != nil && hiObj.Type() != object.NULL_OBJ {
		v, errObj := intArg(hiObj)
		if errObj != nil {
			return nil, nil, 0, 0, nil, errObj
		}
		hi = v
	}
	hi = min(hi, len(l.Elements))
	key := kwargs.Get("key")
	if key != nil && key.Type() == object.NULL_OBJ {
		key = nil
	}
	return l, args[1], lo, hi, key, nil
}

// bisectIndex is bisect_left (right false) or bisect_right over
// a[lo:hi], comparing x with key(a[i]).
func bisectIndex(ctx context.Context, l *object.List, x object.Object, lo, hi int, key object.Object, right bool) (int, object.Object) {
	for lo < hi {
		mid := (lo + hi) / 2
		v := l.Elements[mid]
		if key != nil {
			v = callCallable(ctx, key, v)
			if object.IsPropagating(v) || object.IsError(v) {
				return 0, v
			}
		}
		if right {
			// x < a[mid]: x goes left of mid.
			less, errObj := pyLess(ctx, x, v)
			if errObj != nil {
				return 0, errObj
			}
			if less {
				hi = mid
			} else {
				lo = mid + 1
			}
			continue
		}
		// a[mid] < x: x goes right of mid.
		less, errObj := pyLess(ctx, v, x)
		if errObj != nil {
			return 0, errObj
		}
		if less {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return lo, nil
}

func bisectFn(name string, right bool) *object.Builtin {
	return &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			l, x, lo, hi, key, errObj := bisectArgs(name, args, kwargs)
			if errObj != nil {
				return errObj
			}
			i, errObj := bisectIndex(ctx, l, x, lo, hi, key, right)
			if errObj != nil {
				return errObj
			}
			return object.NewInteger(int64(i))
		},
		HelpText: name + "(a, x, lo=0, hi=len(a), *, key=None) - Where x goes in the sorted list a (" + map[bool]string{false: "before", true: "after"}[right] + " equal items)",
	}
}

func insortFn(name string, right bool) *object.Builtin {
	return &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			l, x, lo, hi, key, errObj := bisectArgs(name, args, kwargs)
			if errObj != nil {
				return errObj
			}
			kx := x
			if key != nil {
				kx = callCallable(ctx, key, x)
				if object.IsPropagating(kx) || object.IsError(kx) {
					return kx
				}
			}
			i, errObj := bisectIndex(ctx, l, kx, lo, hi, key, right)
			if errObj != nil {
				return errObj
			}
			if errObj := object.CheckElements(int64(len(l.Elements) + 1)); errObj != nil {
				return errObj
			}
			l.Elements = append(l.Elements, nil)
			copy(l.Elements[i+1:], l.Elements[i:])
			l.Elements[i] = x
			object.TrackSlice(l.Elements)
			return &object.Null{}
		},
		HelpText: name + "(a, x, lo=0, hi=len(a), *, key=None) - Insert x into the sorted list a, keeping it sorted",
	}
}

// BisectLibrary is Python's bisect module.
var BisectLibrary = object.NewLibrary(BisectLibraryName, map[string]*object.Builtin{
	"bisect_left":  bisectFn("bisect_left", false),
	"bisect_right": bisectFn("bisect_right", true),
	"bisect":       bisectFn("bisect", true),
	"insort_left":  insortFn("insort_left", false),
	"insort_right": insortFn("insort_right", true),
	"insort":       insortFn("insort", true),
}, nil, "Binary search and insertion in sorted lists")

// --- heapq ---

func siftDown(ctx context.Context, h []object.Object, start, pos int) object.Object {
	item := h[pos]
	for pos > start {
		parent := (pos - 1) >> 1
		less, errObj := pyLess(ctx, item, h[parent])
		if errObj != nil {
			return errObj
		}
		if !less {
			break
		}
		h[pos] = h[parent]
		pos = parent
	}
	h[pos] = item
	return nil
}

func siftUp(ctx context.Context, h []object.Object, pos int) object.Object {
	end, start := len(h), pos
	item := h[pos]
	child := 2*pos + 1
	for child < end {
		if right := child + 1; right < end {
			less, errObj := pyLess(ctx, h[child], h[right])
			if errObj != nil {
				return errObj
			}
			if !less {
				child = right
			}
		}
		h[pos] = h[child]
		pos = child
		child = 2*pos + 1
	}
	h[pos] = item
	return siftDown(ctx, h, start, pos)
}

func heapList(name string, args []object.Object, kwargs object.Kwargs, n int) (*object.List, object.Object) {
	if err := errors.NoKwargs(name, kwargs); err != nil {
		return nil, err
	}
	if len(args) != n {
		return nil, errors.NewTypeErrorTagged("%s expected %d arguments, got %d", name, n, len(args))
	}
	return listArg(name, args[0])
}

// heapTopN is nsmallest (largest false) or nlargest over an iterable.
func heapTopN(name string, largest bool) *object.Builtin {
	return &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if kwargs.Len() > 0 {
				if err := errors.CheckKwargs(name, kwargs, "key"); err != nil {
					return err
				}
			}
			if len(args) != 2 {
				return errors.NewTypeErrorTagged("%s() takes 2 positional arguments (%d given)", name, len(args))
			}
			n, ok := args[0].(*object.Integer)
			if !ok {
				return errors.NewTypeErrorTagged("'%s' object cannot be interpreted as an integer", object.PythonTypeName(args[0]))
			}
			elems, errObj := iterableElements(ctx, args[1])
			if errObj != nil {
				return errObj
			}
			key := kwargs.Get("key")
			if key != nil && key.Type() == object.NULL_OBJ {
				key = nil
			}
			keys := make([]object.Object, len(elems))
			for i, e := range elems {
				keys[i] = e
				if key != nil {
					keys[i] = callCallable(ctx, key, e)
					if object.IsPropagating(keys[i]) || object.IsError(keys[i]) {
						return keys[i]
					}
				}
			}
			// A stable insertion sort of indices into the first n places:
			// equal keys keep their order, as Python's nsmallest/nlargest.
			k := int(min(max(n.IntValue(), 0), int64(len(elems))))
			idx := make([]int, 0, k+1)
			for i := range elems {
				pos := len(idx)
				for pos > 0 {
					var before bool
					var errObj object.Object
					if largest {
						before, errObj = pyLess(ctx, keys[idx[pos-1]], keys[i])
					} else {
						before, errObj = pyLess(ctx, keys[i], keys[idx[pos-1]])
					}
					if errObj != nil {
						return errObj
					}
					if !before {
						break
					}
					pos--
				}
				if pos >= k {
					continue
				}
				idx = append(idx, 0)
				copy(idx[pos+1:], idx[pos:])
				idx[pos] = i
				if len(idx) > k {
					idx = idx[:k]
				}
			}
			out := make([]object.Object, len(idx))
			for i, j := range idx {
				out[i] = elems[j]
			}
			object.TrackSlice(out)
			return &object.List{Elements: out}
		},
		HelpText: name + "(n, iterable, key=None) - The n " + map[bool]string{false: "smallest", true: "largest"}[largest] + " items, in order",
	}
}

// HeapqLibrary is Python's heapq module.
var HeapqLibrary = object.NewLibrary(HeapqLibraryName, map[string]*object.Builtin{
	"heappush": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			l, errObj := heapList("heappush", args, kwargs, 2)
			if errObj != nil {
				return errObj
			}
			if errObj := object.CheckElements(int64(len(l.Elements) + 1)); errObj != nil {
				return errObj
			}
			l.Elements = append(l.Elements, args[1])
			object.TrackSlice(l.Elements)
			if errObj := siftDown(ctx, l.Elements, 0, len(l.Elements)-1); errObj != nil {
				return errObj
			}
			return &object.Null{}
		},
		HelpText: "heappush(heap, item) - Push item onto the heap",
	},
	"heappop": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			l, errObj := heapList("heappop", args, kwargs, 1)
			if errObj != nil {
				return errObj
			}
			n := len(l.Elements)
			if n == 0 {
				return errors.NewIndexError("index out of range")
			}
			last := l.Elements[n-1]
			l.Elements[n-1] = nil
			l.Elements = l.Elements[:n-1]
			if len(l.Elements) == 0 {
				return last
			}
			top := l.Elements[0]
			l.Elements[0] = last
			if errObj := siftUp(ctx, l.Elements, 0); errObj != nil {
				return errObj
			}
			return top
		},
		HelpText: "heappop(heap) - Pop the smallest item off the heap",
	},
	"heapify": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			l, errObj := heapList("heapify", args, kwargs, 1)
			if errObj != nil {
				return errObj
			}
			for i := len(l.Elements)/2 - 1; i >= 0; i-- {
				if errObj := siftUp(ctx, l.Elements, i); errObj != nil {
					return errObj
				}
			}
			return &object.Null{}
		},
		HelpText: "heapify(x) - Make the list x a heap, in place",
	},
	"heapreplace": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			l, errObj := heapList("heapreplace", args, kwargs, 2)
			if errObj != nil {
				return errObj
			}
			if len(l.Elements) == 0 {
				return errors.NewIndexError("index out of range")
			}
			top := l.Elements[0]
			l.Elements[0] = args[1]
			if errObj := siftUp(ctx, l.Elements, 0); errObj != nil {
				return errObj
			}
			return top
		},
		HelpText: "heapreplace(heap, item) - Pop the smallest item, then push item",
	},
	"heappushpop": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			l, errObj := heapList("heappushpop", args, kwargs, 2)
			if errObj != nil {
				return errObj
			}
			item := args[1]
			if len(l.Elements) > 0 {
				less, errObj := pyLess(ctx, l.Elements[0], item)
				if errObj != nil {
					return errObj
				}
				if less {
					item, l.Elements[0] = l.Elements[0], item
					if errObj := siftUp(ctx, l.Elements, 0); errObj != nil {
						return errObj
					}
				}
			}
			return item
		},
		HelpText: "heappushpop(heap, item) - Push item, then pop the smallest item (faster than the two calls)",
	},
	"nsmallest": heapTopN("nsmallest", false),
	"nlargest":  heapTopN("nlargest", true),
	"merge": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if kwargs.Len() > 0 {
				if err := errors.CheckKwargs("merge", kwargs, "key", "reverse"); err != nil {
					return err
				}
			}
			key := kwargs.Get("key")
			if key != nil && key.Type() == object.NULL_OBJ {
				key = nil
			}
			reverse := false
			if r := kwargs.Get("reverse"); r != nil {
				reverse = isTruthy(r)
			}
			var lists [][]object.Object
			for _, a := range args {
				elems, errObj := iterableElements(ctx, a)
				if errObj != nil {
					return errObj
				}
				lists = append(lists, elems)
			}
			keyOf := func(v object.Object) object.Object {
				if key == nil {
					return v
				}
				return callCallable(ctx, key, v)
			}
			pos := make([]int, len(lists))
			var out []object.Object
			for {
				best := -1
				var bestKey object.Object
				for i, l := range lists {
					if pos[i] >= len(l) {
						continue
					}
					k := keyOf(l[pos[i]])
					if object.IsPropagating(k) || object.IsError(k) {
						return k
					}
					if best < 0 {
						best, bestKey = i, k
						continue
					}
					a, b := k, bestKey
					if reverse {
						a, b = b, a
					}
					less, errObj := pyLess(ctx, a, b)
					if errObj != nil {
						return errObj
					}
					if less {
						best, bestKey = i, k
					}
				}
				if best < 0 {
					break
				}
				out = append(out, lists[best][pos[best]])
				pos[best]++
			}
			if out == nil {
				out = []object.Object{}
			}
			object.TrackSlice(out)
			next, _ := object.IterSource(&object.List{Elements: out})
			return object.NewIterator(next)
		},
		HelpText: "merge(*iterables, key=None, reverse=False) - Merge sorted inputs into one sorted iterator",
	},
}, nil, "Heap queue (priority queue) algorithm on lists")
