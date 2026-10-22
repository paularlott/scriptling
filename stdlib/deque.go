package stdlib

import (
	"context"
	"math"
	"strings"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// dequeBuf is a deque's storage: a ring buffer, so elements are added and
// removed at either end in amortised O(1), as with Python's deque. Every
// growth is checked against the interpreter's value and memory limits and
// reported to the memory accounting, as list growth is.
type dequeBuf struct {
	buf    []object.Object
	head   int   // index in buf of the first element
	n      int   // number of elements
	maxlen int64 // -1 = unbounded
	// state counts changes to the deque's length or order, so an iterator
	// can report the deque was mutated under it, as Python's does.
	state uint64
}

// dequeMinCap is the smallest ring allocated, and the size below which a
// ring is never shrunk.
const dequeMinCap = 8

func (d *dequeBuf) slot(i int) int {
	return (d.head + i) % len(d.buf)
}

func (d *dequeBuf) at(i int) object.Object {
	return d.buf[d.slot(i)]
}

// elements returns a copy of the elements in order.
func (d *dequeBuf) elements() []object.Object {
	out := make([]object.Object, d.n)
	d.copyTo(out)
	return out
}

func (d *dequeBuf) copyTo(dst []object.Object) {
	if d.n == 0 {
		return
	}
	first := min(d.n, len(d.buf)-d.head)
	copy(dst, d.buf[d.head:d.head+first])
	copy(dst[first:], d.buf[:d.n-first])
}

// resize moves the elements into a new ring of capacity c.
func (d *dequeBuf) resize(c int) {
	nb := make([]object.Object, c)
	object.TrackSlice(nb)
	d.copyTo(nb)
	d.buf, d.head = nb, 0
}

// reserve makes room for extra more elements, checking the grown deque
// against the interpreter's limits first.
func (d *dequeBuf) reserve(extra int) object.Object {
	need := d.n + extra
	if errObj := object.CheckElements(int64(need)); errObj != nil {
		return errObj
	}
	if need > len(d.buf) {
		d.resize(max(need, 2*len(d.buf), dequeMinCap))
	}
	return nil
}

// shrink returns memory once the deque is a quarter full.
func (d *dequeBuf) shrink() {
	if len(d.buf) > dequeMinCap && d.n < len(d.buf)/4 {
		d.resize(max(len(d.buf)/2, dequeMinCap))
	}
}

func (d *dequeBuf) full() bool {
	return d.maxlen >= 0 && int64(d.n) >= d.maxlen
}

// pushRight appends v; at maxlen the leftmost element is dropped first, as
// in Python.
func (d *dequeBuf) pushRight(v object.Object) object.Object {
	if d.maxlen == 0 {
		return nil
	}
	if d.full() {
		d.popLeft()
	}
	if errObj := d.reserve(1); errObj != nil {
		return errObj
	}
	d.buf[d.slot(d.n)] = v
	d.n++
	d.state++
	return nil
}

// pushLeft prepends v; at maxlen the rightmost element is dropped first.
func (d *dequeBuf) pushLeft(v object.Object) object.Object {
	if d.maxlen == 0 {
		return nil
	}
	if d.full() {
		d.popRight()
	}
	if errObj := d.reserve(1); errObj != nil {
		return errObj
	}
	d.head = (d.head - 1 + len(d.buf)) % len(d.buf)
	d.buf[d.head] = v
	d.n++
	d.state++
	return nil
}

// popRight and popLeft remove an element, clearing its slot so the deque no
// longer keeps it alive. The deque must not be empty.
func (d *dequeBuf) popRight() object.Object {
	i := d.slot(d.n - 1)
	v := d.buf[i]
	d.buf[i] = nil
	d.n--
	d.state++
	d.shrink()
	return v
}

func (d *dequeBuf) popLeft() object.Object {
	v := d.buf[d.head]
	d.buf[d.head] = nil
	d.head = (d.head + 1) % len(d.buf)
	d.n--
	d.state++
	d.shrink()
	return v
}

// extendRight appends vals in order. Without maxlen the room for all of them
// is reserved (and checked) once and they are written straight in.
func (d *dequeBuf) extendRight(vals []object.Object) object.Object {
	if d.maxlen < 0 {
		if errObj := d.reserve(len(vals)); errObj != nil {
			return errObj
		}
		for _, v := range vals {
			d.buf[d.slot(d.n)] = v
			d.n++
		}
		d.state++
		return nil
	}
	if int64(len(vals)) > d.maxlen {
		vals = vals[int64(len(vals))-d.maxlen:]
	}
	for _, v := range vals {
		if errObj := d.pushRight(v); errObj != nil {
			return errObj
		}
	}
	return nil
}

// extendLeft prepends vals one at a time, so their order reverses, as in
// Python.
func (d *dequeBuf) extendLeft(vals []object.Object) object.Object {
	if d.maxlen < 0 {
		if errObj := d.reserve(len(vals)); errObj != nil {
			return errObj
		}
		for _, v := range vals {
			d.head = (d.head - 1 + len(d.buf)) % len(d.buf)
			d.buf[d.head] = v
			d.n++
		}
		d.state++
		return nil
	}
	if int64(len(vals)) > d.maxlen {
		vals = vals[int64(len(vals))-d.maxlen:]
	}
	for _, v := range vals {
		if errObj := d.pushLeft(v); errObj != nil {
			return errObj
		}
	}
	return nil
}

// rotate moves the last k elements to the front (k > 0) or the first -k to
// the back, taking the shorter way round. It never grows the ring.
func (d *dequeBuf) rotate(k int64) {
	d.state++
	if d.n <= 1 {
		return
	}
	n := int64(d.n)
	k = ((k % n) + n) % n
	if d.n == len(d.buf) {
		// A full ring rotates by moving its head alone.
		d.head = int((int64(d.head) - k + n) % n)
		return
	}
	if k > n/2 {
		k -= n
	}
	for ; k > 0; k-- {
		d.head = (d.head - 1 + len(d.buf)) % len(d.buf)
		last := d.slot(d.n)
		d.buf[d.head], d.buf[last] = d.buf[last], nil
	}
	for ; k < 0; k++ {
		end := d.slot(d.n)
		d.buf[end], d.buf[d.head] = d.buf[d.head], nil
		d.head = (d.head + 1) % len(d.buf)
	}
}

// removeAt deletes the element at index i, moving the shorter side.
func (d *dequeBuf) removeAt(i int) {
	if i < d.n/2 {
		for j := i; j > 0; j-- {
			d.buf[d.slot(j)] = d.buf[d.slot(j-1)]
		}
		d.popLeft()
	} else {
		for j := i; j < d.n-1; j++ {
			d.buf[d.slot(j)] = d.buf[d.slot(j+1)]
		}
		d.popRight()
	}
}

// insertAt puts v before index i (0 <= i <= n). The deque must have room
// under maxlen.
func (d *dequeBuf) insertAt(i int, v object.Object) object.Object {
	if i == d.n {
		return d.pushRight(v)
	}
	if errObj := d.pushLeft(v); errObj != nil {
		return errObj
	}
	for j := 0; j < i; j++ {
		d.buf[d.slot(j)] = d.buf[d.slot(j+1)]
	}
	d.buf[d.slot(i)] = v
	return nil
}

func (d *dequeBuf) reverse() {
	for i, j := 0, d.n-1; i < j; i, j = i+1, j-1 {
		a, b := d.slot(i), d.slot(j)
		d.buf[a], d.buf[b] = d.buf[b], d.buf[a]
	}
}

func (d *dequeBuf) clear() {
	d.buf, d.head, d.n = nil, 0, 0
	d.state++
}

// clone returns a deque buffer holding the same elements, checked and
// tracked like any new container.
func (d *dequeBuf) clone() (*dequeBuf, object.Object) {
	c := &dequeBuf{maxlen: d.maxlen}
	if errObj := c.reserve(d.n); errObj != nil {
		return nil, errObj
	}
	d.copyTo(c.buf)
	c.n = d.n
	return c, nil
}

// repeat makes the deque hold its elements k times over (k <= 0 empties
// it), keeping only the last maxlen when bounded, as Python's *= does.
func (d *dequeBuf) repeat(k int64) object.Object {
	if k <= 0 || d.n == 0 {
		d.clear()
		return nil
	}
	if k == 1 {
		return nil
	}
	vals := d.elements()
	if d.maxlen >= 0 {
		// Only the last maxlen elements survive; add just enough copies.
		k = min(k-1, d.maxlen/int64(len(vals))+1)
		for ; k > 0; k-- {
			if errObj := d.extendRight(vals); errObj != nil {
				return errObj
			}
		}
		return nil
	}
	if k > math.MaxInt64/int64(len(vals)) {
		return object.NewMemoryError("deque too large: %d repetitions", k)
	}
	if errObj := d.reserve(int(int64(len(vals)) * (k - 1))); errObj != nil {
		return errObj
	}
	for ; k > 1; k-- {
		d.extendRight(vals)
	}
	return nil
}

const dequeField = "_deque"

// dequeClassRef is DequeClass, set in init() so the class's own methods can
// refer to it without an initialization cycle.
var dequeClassRef *object.Class

// newDequeOf returns a new instance of cls (deque or a subclass) holding buf.
func newDequeOf(cls *object.Class, buf *dequeBuf) *object.Instance {
	inst := object.NewInstance(cls)
	setDeque(inst, buf)
	return inst
}

func setDeque(inst *object.Instance, buf *dequeBuf) {
	inst.SetField(dequeField, &object.ClientWrapper{TypeName: "deque", Client: buf})
}

func dequeOf(obj object.Object) *dequeBuf {
	inst, ok := obj.(*object.Instance)
	if !ok {
		return nil
	}
	w, ok := inst.Field(dequeField).(*object.ClientWrapper)
	if !ok {
		return nil
	}
	d, _ := w.Client.(*dequeBuf)
	return d
}

// isDeque reports whether obj is a collections.deque (or subclass) instance.
func isDeque(obj object.Object) bool {
	return dequeOf(obj) != nil
}

// copyDeque is copy.copy for a deque: a new deque of the same class sharing
// the elements.
func copyDeque(obj object.Object) object.Object {
	c, errObj := dequeOf(obj).clone()
	if errObj != nil {
		return errObj
	}
	return newDequeOf(obj.(*object.Instance).Class, c)
}

// dequeTypeName is the name Python's messages give the deque type.
func dequeTypeName(obj object.Object) string {
	if inst, ok := obj.(*object.Instance); ok && inst.Class != dequeClassRef {
		return inst.Class.Name
	}
	return "collections.deque"
}

// dequeInit implements deque([iterable[, maxlen]]): it (re)initialises self.
func dequeInit(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	if len(args) < 1 || len(args) > 3 {
		return errors.NewTypeErrorTagged("deque() takes at most 2 arguments (%d given)", len(args)-1)
	}
	self, ok := args[0].(*object.Instance)
	if !ok {
		return errors.NewTypeErrorTagged("deque.__init__ needs a deque")
	}
	args = args[1:]
	d := &dequeBuf{maxlen: -1}
	mlObj := object.Object(nil)
	if len(args) == 2 {
		mlObj = args[1]
	} else if v := kwargs.Get("maxlen"); v != nil {
		mlObj = v
	}
	if mlObj != nil && mlObj.Type() != object.NULL_OBJ {
		ml, ok := mlObj.(*object.Integer)
		if !ok {
			return errors.NewTypeErrorTagged("an integer is required")
		}
		if ml.IntValue() < 0 {
			return errors.NewValueError("maxlen must be non-negative")
		}
		d.maxlen = ml.IntValue()
	}
	var initial object.Object
	if len(args) >= 1 {
		initial = args[0]
	} else if v := kwargs.Get("iterable"); v != nil {
		initial = v
	}
	if initial != nil {
		if initial.Type() == object.NULL_OBJ {
			return errors.NewTypeErrorTagged("'NoneType' object is not iterable")
		}
		vals, errObj := dequeValues(ctx, initial)
		if errObj != nil {
			return errObj
		}
		if errObj := d.extendRight(vals); errObj != nil {
			return errObj
		}
	}
	setDeque(self, d)
	return &object.Null{}
}

// dequeValues collects an iterable into a slice. A string is split into its
// characters with the value checked first; a deque yields its elements.
func dequeValues(ctx context.Context, obj object.Object) ([]object.Object, object.Object) {
	if d := dequeOf(obj); d != nil {
		return d.elements(), nil
	}
	return collectIterable(ctx, obj)
}

func emptyDequeError(op string) object.Object {
	return &object.Exception{Message: op + " from an empty deque", ExceptionType: object.ExceptionTypeIndexError, Raised: true}
}

func dequeIndexError() object.Object {
	return &object.Exception{Message: "deque index out of range", ExceptionType: object.ExceptionTypeIndexError, Raised: true}
}

// dequeIndex resolves a script index (negatives count from the end).
func dequeIndex(d *dequeBuf, idxObj object.Object) (int, object.Object) {
	idx, err := idxObj.AsInt()
	if err != nil {
		return 0, errors.NewTypeErrorTagged("sequence index must be integer, not '%s'", pyTypeName(idxObj))
	}
	if idx < 0 {
		idx += int64(d.n)
	}
	if idx < 0 || idx >= int64(d.n) {
		return 0, dequeIndexError()
	}
	return int(idx), nil
}

func pyTypeName(obj object.Object) string {
	switch obj.(type) {
	case *object.Slice:
		return "slice"
	case *object.String:
		return "str"
	case *object.Float:
		return "float"
	case *object.Null:
		return "NoneType"
	}
	return strings.ToLower(obj.Type().String())
}

// dequeMatch is the equality Python's deque searches use: the object itself,
// or one equal to it.
func dequeMatch(ctx context.Context, elem, x object.Object) (bool, object.Object) {
	if elem == x {
		return true, nil
	}
	return object.Equal(ctx, elem, x)
}

// dequeFind returns the index of the first element in [start, stop) matching
// x, or -1. A comparison that mutates the deque is an error, as in Python.
func dequeFind(ctx context.Context, d *dequeBuf, x object.Object, start, stop int, op string) (int, object.Object) {
	state := d.state
	for i := start; i < stop && i < d.n; i++ {
		eq, errObj := dequeMatch(ctx, d.at(i), x)
		if errObj != nil {
			return -1, errObj
		}
		if d.state != state {
			return -1, &object.Exception{Message: "deque mutated during " + op, ExceptionType: object.ExceptionTypeRuntimeError, Raised: true}
		}
		if eq {
			return i, nil
		}
	}
	return -1, nil
}

// dequeIterator walks the deque from front to back (or back to front),
// raising RuntimeError if it is mutated meanwhile, as in Python.
func dequeIterator(d *dequeBuf, reverse bool) object.Object {
	state, n, i, done := d.state, d.n, 0, false
	return object.NewIterator(func() (object.Object, bool) {
		if done {
			return nil, false
		}
		// The state is checked before the end, as in CPython: a mutation
		// while handling the last element is still reported.
		if d.state != state {
			done = true
			return &object.Exception{Message: "deque mutated during iteration", ExceptionType: object.ExceptionTypeRuntimeError, Raised: true}, true
		}
		if i >= n {
			done = true
			return nil, false
		}
		idx := i
		if reverse {
			idx = n - 1 - i
		}
		i++
		return d.at(idx), true
	})
}

// dequeOrder implements the ordering operators between two deques.
func dequeOrder(op string, keep func(int) bool) *object.Builtin {
	return dequeMethod(1, func(ctx context.Context, d *dequeBuf, self *object.Instance, args []object.Object) object.Object {
		o := dequeOf(args[0])
		if o == nil {
			return errors.NewTypeErrorTagged("'%s' not supported between instances of '%s' and '%s'", op, dequeTypeName(self), pyTypeName(args[0]))
		}
		c, errObj := object.Compare(ctx, &object.List{Elements: d.elements()}, &object.List{Elements: o.elements()})
		if errObj != nil {
			return errObj
		}
		return object.NewBoolean(keep(c))
	})
}

func dequeRepr(self *object.Instance, d *dequeBuf) string {
	name := "deque"
	if self.Class != dequeClassRef {
		name = self.Class.Name
	}
	// A list renders its elements as Python's repr does.
	out := name + "(" + (&object.List{Elements: d.elements()}).Inspect()
	if d.maxlen >= 0 {
		out += ", maxlen=" + object.NewInteger(d.maxlen).Inspect()
	}
	return out + ")"
}

// dequeMethod adapts a method body taking the deque's buffer; nargs < 0
// skips the argument count check.
func dequeMethod(nargs int, fn func(ctx context.Context, d *dequeBuf, self *object.Instance, args []object.Object) object.Object) *object.Builtin {
	return &object.Builtin{Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
		if kwargs.Len() > 0 {
			return errors.NewTypeErrorTagged("deque methods take no keyword arguments")
		}
		if nargs >= 0 {
			if err := errors.ExactArgs(args, nargs+1); err != nil {
				return err
			}
		}
		d := dequeOf(args[0])
		if d == nil {
			return errors.NewTypeErrorTagged("descriptor requires a 'collections.deque' object")
		}
		return fn(ctx, d, args[0].(*object.Instance), args[1:])
	}}
}

// DequeClass is collections.deque: a double-ended queue.
var DequeClass = &object.Class{
	Name: "deque",
	Methods: map[string]object.Object{
		"__init__": &object.Builtin{Fn: dequeInit, HelpText: `deque([iterable[, maxlen]]) - Double-ended queue`},
		"maxlen": &object.Property{Getter: dequeMethod(0, func(ctx context.Context, d *dequeBuf, _ *object.Instance, _ []object.Object) object.Object {
			if d.maxlen < 0 {
				return &object.Null{}
			}
			return object.NewInteger(d.maxlen)
		})},
		"append": dequeMethod(1, func(ctx context.Context, d *dequeBuf, _ *object.Instance, args []object.Object) object.Object {
			if errObj := d.pushRight(args[0]); errObj != nil {
				return errObj
			}
			return &object.Null{}
		}),
		"appendleft": dequeMethod(1, func(ctx context.Context, d *dequeBuf, _ *object.Instance, args []object.Object) object.Object {
			if errObj := d.pushLeft(args[0]); errObj != nil {
				return errObj
			}
			return &object.Null{}
		}),
		"pop": dequeMethod(0, func(ctx context.Context, d *dequeBuf, _ *object.Instance, _ []object.Object) object.Object {
			if d.n == 0 {
				return emptyDequeError("pop")
			}
			return d.popRight()
		}),
		"popleft": dequeMethod(0, func(ctx context.Context, d *dequeBuf, _ *object.Instance, _ []object.Object) object.Object {
			if d.n == 0 {
				return emptyDequeError("pop")
			}
			return d.popLeft()
		}),
		"extend": dequeMethod(1, func(ctx context.Context, d *dequeBuf, _ *object.Instance, args []object.Object) object.Object {
			add, errObj := dequeValues(ctx, args[0])
			if errObj != nil {
				return errObj
			}
			if errObj := d.extendRight(add); errObj != nil {
				return errObj
			}
			return &object.Null{}
		}),
		"extendleft": dequeMethod(1, func(ctx context.Context, d *dequeBuf, _ *object.Instance, args []object.Object) object.Object {
			add, errObj := dequeValues(ctx, args[0])
			if errObj != nil {
				return errObj
			}
			if errObj := d.extendLeft(add); errObj != nil {
				return errObj
			}
			return &object.Null{}
		}),
		"insert": dequeMethod(2, func(ctx context.Context, d *dequeBuf, _ *object.Instance, args []object.Object) object.Object {
			i, err := args[0].AsInt()
			if err != nil {
				return errors.ParameterError("index", err)
			}
			if d.full() {
				return &object.Exception{Message: "deque already at its maximum size", ExceptionType: object.ExceptionTypeIndexError, Raised: true}
			}
			n := int64(d.n)
			if i < 0 {
				i = max(i+n, 0)
			}
			i = min(i, n)
			if errObj := d.insertAt(int(i), args[1]); errObj != nil {
				return errObj
			}
			return &object.Null{}
		}),
		"remove": dequeMethod(1, func(ctx context.Context, d *dequeBuf, _ *object.Instance, args []object.Object) object.Object {
			i, errObj := dequeFind(ctx, d, args[0], 0, d.n, "remove")
			if errObj != nil {
				return errObj
			}
			if i < 0 {
				return errors.NewValueError("deque.remove(x): x not in deque")
			}
			d.removeAt(i)
			return &object.Null{}
		}),
		"index": dequeMethod(-1, func(ctx context.Context, d *dequeBuf, _ *object.Instance, args []object.Object) object.Object {
			if len(args) < 1 || len(args) > 3 {
				return errors.NewTypeErrorTagged("index expected at least 1 argument, got %d", len(args))
			}
			n := int64(d.n)
			bound := func(o object.Object, def int64) (int64, object.Object) {
				v, err := o.AsInt()
				if err != nil {
					return 0, errors.ParameterError("index", err)
				}
				if v < 0 {
					v = max(v+n, 0)
				}
				return min(v, n), nil
			}
			start, stop := int64(0), n
			var errObj object.Object
			if len(args) >= 2 {
				if start, errObj = bound(args[1], 0); errObj != nil {
					return errObj
				}
			}
			if len(args) == 3 {
				if stop, errObj = bound(args[2], n); errObj != nil {
					return errObj
				}
			}
			i, errObj := dequeFind(ctx, d, args[0], int(start), int(stop), "iteration")
			if errObj != nil {
				return errObj
			}
			if i < 0 {
				return errors.NewValueError("deque.index(x): x not in deque")
			}
			return object.NewInteger(int64(i))
		}),
		"count": dequeMethod(1, func(ctx context.Context, d *dequeBuf, _ *object.Instance, args []object.Object) object.Object {
			state := d.state
			count := int64(0)
			for i := 0; i < d.n; i++ {
				eq, errObj := dequeMatch(ctx, d.at(i), args[0])
				if errObj != nil {
					return errObj
				}
				if d.state != state {
					return &object.Exception{Message: "deque mutated during iteration", ExceptionType: object.ExceptionTypeRuntimeError, Raised: true}
				}
				if eq {
					count++
				}
			}
			return object.NewInteger(count)
		}),
		"reverse": dequeMethod(0, func(ctx context.Context, d *dequeBuf, _ *object.Instance, _ []object.Object) object.Object {
			d.reverse()
			return &object.Null{}
		}),
		"rotate": dequeMethod(-1, func(ctx context.Context, d *dequeBuf, _ *object.Instance, args []object.Object) object.Object {
			if len(args) > 1 {
				return errors.NewTypeErrorTagged("rotate expected at most 1 argument, got %d", len(args))
			}
			k := int64(1)
			if len(args) == 1 {
				v, err := args[0].AsInt()
				if err != nil {
					return errors.ParameterError("n", err)
				}
				k = v
			}
			d.rotate(k)
			return &object.Null{}
		}),
		"clear": dequeMethod(0, func(ctx context.Context, d *dequeBuf, _ *object.Instance, _ []object.Object) object.Object {
			d.clear()
			return &object.Null{}
		}),
		"copy": dequeMethod(0, func(ctx context.Context, _ *dequeBuf, self *object.Instance, _ []object.Object) object.Object {
			return copyDequeFn(self)
		}),
		"__len__": dequeMethod(0, func(ctx context.Context, d *dequeBuf, _ *object.Instance, _ []object.Object) object.Object {
			return object.NewInteger(int64(d.n))
		}),
		"__bool__": dequeMethod(0, func(ctx context.Context, d *dequeBuf, _ *object.Instance, _ []object.Object) object.Object {
			return object.NewBoolean(d.n > 0)
		}),
		"__iter__": dequeMethod(0, func(ctx context.Context, d *dequeBuf, _ *object.Instance, _ []object.Object) object.Object {
			return dequeIterator(d, false)
		}),
		"__reversed__": dequeMethod(0, func(ctx context.Context, d *dequeBuf, _ *object.Instance, _ []object.Object) object.Object {
			return dequeIterator(d, true)
		}),
		"__contains__": dequeMethod(1, func(ctx context.Context, d *dequeBuf, _ *object.Instance, args []object.Object) object.Object {
			i, errObj := dequeFind(ctx, d, args[0], 0, d.n, "iteration")
			if errObj != nil {
				return errObj
			}
			return object.NewBoolean(i >= 0)
		}),
		"__getitem__": dequeMethod(1, func(ctx context.Context, d *dequeBuf, _ *object.Instance, args []object.Object) object.Object {
			i, errObj := dequeIndex(d, args[0])
			if errObj != nil {
				return errObj
			}
			return d.at(i)
		}),
		"__setitem__": dequeMethod(2, func(ctx context.Context, d *dequeBuf, _ *object.Instance, args []object.Object) object.Object {
			i, errObj := dequeIndex(d, args[0])
			if errObj != nil {
				return errObj
			}
			d.buf[d.slot(i)] = args[1]
			return &object.Null{}
		}),
		"__delitem__": dequeMethod(1, func(ctx context.Context, d *dequeBuf, _ *object.Instance, args []object.Object) object.Object {
			i, errObj := dequeIndex(d, args[0])
			if errObj != nil {
				return errObj
			}
			d.removeAt(i)
			return &object.Null{}
		}),
		"__eq__": dequeMethod(1, func(ctx context.Context, d *dequeBuf, _ *object.Instance, args []object.Object) object.Object {
			o := dequeOf(args[0])
			if o == nil || o.n != d.n {
				return object.NewBoolean(false)
			}
			if o == d {
				return object.NewBoolean(true)
			}
			eq, errObj := object.Equal(ctx, &object.List{Elements: d.elements()}, &object.List{Elements: o.elements()})
			if errObj != nil {
				return errObj
			}
			return object.NewBoolean(eq)
		}),
		"__lt__": dequeOrder("<", func(c int) bool { return c < 0 }),
		"__le__": dequeOrder("<=", func(c int) bool { return c <= 0 }),
		"__gt__": dequeOrder(">", func(c int) bool { return c > 0 }),
		"__ge__": dequeOrder(">=", func(c int) bool { return c >= 0 }),
		"__add__": dequeMethod(1, func(ctx context.Context, d *dequeBuf, self *object.Instance, args []object.Object) object.Object {
			o := dequeOf(args[0])
			if o == nil {
				return errors.NewTypeErrorTagged("can only concatenate deque (not \"%s\") to deque", pyTypeName(args[0]))
			}
			c, errObj := d.clone()
			if errObj != nil {
				return errObj
			}
			if errObj := c.extendRight(o.elements()); errObj != nil {
				return errObj
			}
			return newDequeOf(self.Class, c)
		}),
		"__iadd__": dequeMethod(1, func(ctx context.Context, d *dequeBuf, self *object.Instance, args []object.Object) object.Object {
			add, errObj := dequeValues(ctx, args[0])
			if errObj != nil {
				return errObj
			}
			if errObj := d.extendRight(add); errObj != nil {
				return errObj
			}
			return self
		}),
		"__mul__":  dequeMultiply(false),
		"__rmul__": dequeMultiply(false),
		"__imul__": dequeMultiply(true),
		"__str__": dequeMethod(0, func(ctx context.Context, d *dequeBuf, self *object.Instance, _ []object.Object) object.Object {
			return object.NewString(dequeRepr(self, d))
		}),
		"__repr__": dequeMethod(0, func(ctx context.Context, d *dequeBuf, self *object.Instance, _ []object.Object) object.Object {
			return object.NewString(dequeRepr(self, d))
		}),
	},
}

// dequeMultiply implements d * k, k * d and d *= k.
func dequeMultiply(inPlace bool) *object.Builtin {
	return dequeMethod(1, func(ctx context.Context, d *dequeBuf, self *object.Instance, args []object.Object) object.Object {
		k, err := args[0].AsInt()
		if err != nil {
			return errors.NewTypeErrorTagged("can't multiply sequence by non-int of type '%s'", pyTypeName(args[0]))
		}
		target, target2 := d, self
		if !inPlace {
			c, errObj := d.clone()
			if errObj != nil {
				return errObj
			}
			target, target2 = c, newDequeOf(self.Class, c)
		}
		if errObj := target.repeat(k); errObj != nil {
			return errObj
		}
		return target2
	})
}
