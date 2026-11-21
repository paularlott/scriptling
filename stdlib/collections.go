package stdlib

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// Counter class for counting elements

// defaultDictBuiltin is collections.defaultdict: an ordinary insertion-ordered
// dict that carries a default factory. Reading a missing key calls the factory
// with no arguments, stores the result and returns it; get(), `in`, pop() and
// the rest never invoke it, as in Python.
var defaultDictBuiltin = &object.Builtin{
	Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
		if len(args) > 2 {
			return errors.NewTypeErrorTagged("defaultdict expected at most 2 arguments, got %d", len(args))
		}
		var factory object.Object = &object.Null{}
		if len(args) > 0 {
			factory = args[0]
		}
		switch factory.(type) {
		case *object.Null, *object.Builtin, *object.Function, *object.LambdaFunction, *object.Class, *object.BoundMethod:
		default:
			return errors.NewTypeErrorTagged("first argument must be callable or None")
		}
		dd := object.NewDict()
		dd.SetFactory(factory)
		if len(args) == 2 {
			switch init := args[1].(type) {
			case *object.Dict:
				dd.StoreFrom(init)
			case *object.List:
				if err := defaultDictAddPairs(dd, init.Elements); err != nil {
					return err
				}
			case *object.Tuple:
				if err := defaultDictAddPairs(dd, init.Elements); err != nil {
					return err
				}
			default:
				// Any other iterable of pairs, such as a generator.
				elems, errObj := collectIterable(ctx, init)
				if errObj != nil {
					return errObj
				}
				if err := defaultDictAddPairs(dd, elems); err != nil {
					return err
				}
			}
		}
		for _, name := range kwargs.Keys() {
			dd.SetByString(name, kwargs.Get(name))
		}
		return dd
	},
	HelpText: `defaultdict([default_factory[, init]]) - dict with default values

Reading a missing key calls default_factory() (no arguments), stores the
result and returns it. default_factory may be a type (int, list, set, dict,
float, str), a function or a lambda; None makes a plain dict that raises
KeyError. init is an optional dict or list of (key, value) pairs.

Example:
  counts = defaultdict(int)
  for w in words:
      counts[w] += 1
  groups = defaultdict(list)
  groups[key].append(item)
  nested = defaultdict(lambda: defaultdict(int))`,
}

func defaultDictAddPairs(dd *object.Dict, items []object.Object) object.Object {
	for _, item := range items {
		var pair []object.Object
		switch p := item.(type) {
		case *object.Tuple:
			pair = p.Elements
		case *object.List:
			pair = p.Elements
		}
		if len(pair) != 2 {
			return errors.NewValueError("dictionary update sequence element must be a (key, value) pair")
		}
		dd.Store(object.DictKey(pair[0]), pair[0], pair[1])
	}
	return nil
}

func init() { object.DefaultDictType = defaultDictBuiltin }

// createCounterInstance creates a new Counter instance

// CollectionsLibrary provides Python-like collections functions
var CollectionsLibrary = object.NewLibrary(CollectionsLibraryName, map[string]*object.Builtin{
	"Counter": {
		Fn: counterConstructor,
		HelpText: `Counter([iterable_or_mapping], **kwargs) - Count elements

A dict of element -> count, as in Python. Missing elements count 0.

Example:
  c = collections.Counter([1, 1, 2, 3, 3, 3])
  c[3]                -> 3
  c[4]                -> 0
  c.most_common(2)    -> [(3, 3), (1, 2)]
  c.update([1]); c.total() -> 7`,
	},
	"most_common": {
		Fn:       counterMostCommonFn,
		HelpText: `most_common(counter[, n]) - Same as counter.most_common(n)`,
	},

	"defaultdict": defaultDictBuiltin,
	"DefaultDict": defaultDictBuiltin,

	"OrderedDict": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			// OrderedDict([items]) - Dict that remembers insertion order
			// Note: In modern Python (3.7+), regular dicts maintain order
			// Scriptling dicts also maintain order, so this just creates a dict
			od := object.NewDict()

			if len(args) > 1 {
				return errors.NewTypeErrorTagged("OrderedDict expected at most 1 argument, got %d", len(args))
			}
			// Initialize from a dict or a list/tuple of (key, value) pairs.
			if len(args) == 1 {
				switch arg := args[0].(type) {
				case *object.List:
					if err := defaultDictAddPairs(od, arg.Elements); err != nil {
						return err
					}
				case *object.Tuple:
					if err := defaultDictAddPairs(od, arg.Elements); err != nil {
						return err
					}
				case *object.Dict:
					od.StoreFrom(arg)
				default:
					// Any other iterable of pairs, such as a generator.
					elems, errObj := collectIterable(ctx, arg)
					if errObj != nil {
						return errObj
					}
					if err := defaultDictAddPairs(od, elems); err != nil {
						return err
					}
				}
			}
			for name, value := range kwargs.All() {
				od.SetByString(name, value)
			}
			return od
		},
		HelpText: `OrderedDict([items]) - Dict that remembers insertion order

Creates a dict that maintains insertion order.
Note: Scriptling dicts already maintain order, so this is equivalent to dict().

Example:
  od = collections.OrderedDict([("a", 1), ("b", 2)])`,
	},
	"namedtuple": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if kwargs.Len() > 0 {
				if err := errors.CheckKwargs("namedtuple", kwargs, "defaults", "rename", "module"); err != nil {
					return err
				}
			}
			// namedtuple(typename, field_names) - Create a named tuple class
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}
			typename, ok := args[0].(*object.String)
			if !ok {
				return errors.NewTypeError("STRING", args[0].Type().String())
			}

			var fieldNames []string
			switch fn := args[1].(type) {
			case *object.List:
				for _, elem := range fn.Elements {
					if s, ok := elem.(*object.String); ok {
						fieldNames = append(fieldNames, s.StringValue())
					} else {
						return errors.NewValueError("Type names and field names must be valid identifiers: %s", elem.Inspect())
					}
				}
			case *object.Tuple:
				for _, elem := range fn.Elements {
					if s, ok := elem.(*object.String); ok {
						fieldNames = append(fieldNames, s.StringValue())
					} else {
						return errors.NewValueError("Type names and field names must be valid identifiers: %s", elem.Inspect())
					}
				}
			case *object.String:
				// Space or comma separated
				fields := strings.FieldsFunc(fn.StringValue(), func(r rune) bool {
					return r == ' ' || r == ','
				})
				for _, f := range fields {
					f = strings.TrimSpace(f)
					if f != "" {
						fieldNames = append(fieldNames, f)
					}
				}
			default:
				return errors.NewTypeError("list, tuple, or string", args[1].Type().String())
			}
			rename := false
			if v := kwargs.Get("rename"); v != nil {
				rename, _ = v.AsBool()
			}
			if errObj := checkNamedTupleFields(typename.StringValue(), fieldNames, rename); errObj != nil {
				return errObj
			}

			// Create a NamedTuple class. ntClass is declared before the
			// method closures that reference it (_replace) and assigned once
			// the method map is complete.
			var ntClass *object.Class
			methods := make(map[string]object.Object)

			// defaults=(...) applies to the rightmost fields, as in Python.
			var defaults []object.Object
			if d := kwargs.Get("defaults"); d != nil {
				switch dv := d.(type) {
				case *object.List:
					defaults = dv.Elements
				case *object.Tuple:
					defaults = dv.Elements
				case *object.Null:
				default:
					return errors.NewTypeError("list or tuple", d.Type().String())
				}
				if len(defaults) > len(fieldNames) {
					return errors.NewTypeErrorTagged("Got more default values than field names")
				}
			}

			// __init__ method - stores fields as instance attributes. Values
			// come positionally, by keyword (P(x=1, y=2)) or from defaults.
			methods["__init__"] = &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					positional := args[1:]
					if len(positional) > len(fieldNames) {
						return errors.NewTypeErrorTagged("%s() takes %d positional arguments but %d were given", typename.StringValue(), len(fieldNames), len(positional))
					}
					values := make([]object.Object, len(fieldNames))
					copy(values, positional)
					for name, v := range kwargs.All() {
						idx := -1
						for i, fn := range fieldNames {
							if fn == name {
								idx = i
								break
							}
						}
						if idx < 0 {
							return errors.NewTypeErrorTagged("%s() got an unexpected keyword argument '%s'", typename.StringValue(), name)
						}
						if values[idx] != nil {
							return errors.NewTypeErrorTagged("%s() got multiple values for argument '%s'", typename.StringValue(), name)
						}
						values[idx] = v
					}
					for i := range values {
						if values[i] != nil {
							continue
						}
						if di := i - (len(fieldNames) - len(defaults)); di >= 0 {
							values[i] = defaults[di]
							continue
						}
						return errors.NewTypeErrorTagged("%s() missing required argument: '%s'", typename.StringValue(), fieldNames[i])
					}
					args = append([]object.Object{args[0]}, values...)
					nt := args[0].(*object.Instance)
					// Store field values directly as instance fields
					for i, name := range fieldNames {
						nt.SetField(name, args[i+1])
					}
					nt.SetField("__typename__", typename)
					// Store field names for reference, under both the
					// internal name and Python's public p._fields.
					fieldNameObjs := make([]object.Object, len(fieldNames))
					for i, name := range fieldNames {
						fieldNameObjs[i] = object.NewString(name)
					}
					object.TrackSlice(fieldNameObjs)
					fieldsTuple := &object.Tuple{Elements: fieldNameObjs}
					nt.SetField("__fields__", fieldsTuple)
					nt.SetField("_fields", fieldsTuple)
					// Precompute the display form (the __str_repr__ idiom
					// datetime uses) so print() shows P(x=1, y='hi').
					// A display form past the output limit is left to
					// __repr__, which reports it when asked.
					if display, errObj := fieldsRepr(typename.StringValue(), fieldNames, func(name string) (object.Object, bool) {
						return nt.GetField(name)
					}); errObj == nil {
						nt.SetField("__str_repr__", object.NewString(display))
					}
					return &object.Null{}
				},
			}

			// __getitem__ for dict-like access
			methods["__getitem__"] = &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.NoKwargs("__getitem__", kwargs); err != nil {
						return err
					}
					if err := errors.ExactArgs(args, 2); err != nil {
						return err
					}
					nt := args[0].(*object.Instance)
					if sl, ok := args[1].(*object.Slice); ok {
						vals := make([]object.Object, len(fieldNames))
						for i, name := range fieldNames {
							vals[i], _ = nt.GetField(name)
						}
						return &object.Tuple{Elements: sliceElements(vals, sl)}
					}
					if idx, ok := args[1].(*object.Integer); ok {
						// Positional access, like a tuple: p[0], p[-1],
						// IndexError out of range.
						i := int(idx.IntValue())
						if i < 0 {
							i += len(fieldNames)
						}
						if i < 0 || i >= len(fieldNames) {
							return &object.Exception{
								Message:       "tuple index out of range",
								ExceptionType: object.ExceptionTypeIndexError,
								Raised:        true,
							}
						}
						if v, exists := nt.GetField(fieldNames[i]); exists {
							return v
						}
						return &object.Null{}
					}
					key := args[1].Inspect()
					// Don't expose internal fields
					if key == "__typename__" || key == "__fields__" {
						return &object.Null{}
					}
					if value, exists := nt.GetField(key); exists {
						return value
					}
					return &object.Exception{
						Message:       "tuple index out of range",
						ExceptionType: object.ExceptionTypeIndexError,
						Raised:        true,
					}
				},
				HelpText: `__getitem__(key) - Get field value (supports nt[key] syntax)`,
			}

			// A named tuple's length is its field count, as a tuple's is.
			methods["__len__"] = &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.NoKwargs("__len__", kwargs); err != nil {
						return err
					}
					if err := errors.ExactArgs(args, 1); err != nil {
						return err
					}
					return object.NewInteger(int64(len(fieldNames)))
				},
				HelpText: `__len__() - The field count`,
			}

			// Iterating a named tuple yields its field values, so
			// tuple unpacking (x, y = p) works as in Python.
			methods["__iter__"] = &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.NoKwargs("__iter__", kwargs); err != nil {
						return err
					}
					nt := args[0].(*object.Instance)
					values := make([]object.Object, 0, len(fieldNames))
					for _, name := range fieldNames {
						if v, exists := nt.GetField(name); exists {
							values = append(values, v)
						} else {
							values = append(values, &object.Null{})
						}
					}
					i := 0
					return object.NewIterator(func() (object.Object, bool) {
						if i >= len(values) {
							return nil, false
						}
						v := values[i]
						i++
						return v, true
					})
				},
				HelpText: `__iter__() - Iterate field values`,
			}

			// __repr__ renders Python's "Typename(field=value, ...)".
			methods["__repr__"] = &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.NoKwargs("__repr__", kwargs); err != nil {
						return err
					}
					nt := args[0].(*object.Instance)
					display, errObj := fieldsRepr(typename.StringValue(), fieldNames, nt.GetField)
					if errObj != nil {
						return errObj
					}
					return object.NewString(display)
				},
			}

			// _replace(**changes) returns a new instance with the named
			// fields replaced; the original is untouched, as in Python.
			methods["_replace"] = &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					nt := args[0].(*object.Instance)
					fields := nt.FieldsSnapshot()
					for name, v := range kwargs.All() {
						known := false
						for _, fn := range fieldNames {
							if fn == name {
								known = true
								break
							}
						}
						if !known {
							return errors.NewValueError("Got unexpected field names: %s", name)
						}
						fields[name] = v
					}
					// Recompute the precomputed display form for the new values.
					delete(fields, "__str_repr__")
					if display, errObj := fieldsRepr(typename.StringValue(), fieldNames, func(name string) (object.Object, bool) {
						return fields[name], true
					}); errObj == nil {
						fields["__str_repr__"] = object.NewString(display)
					}
					return object.NewInstanceWithFields(ntClass, fields)
				},
				HelpText: `_replace(**changes) - Return a new instance with fields replaced`,
			}

			// _asdict() returns the fields as a dict.
			methods["_asdict"] = &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.NoKwargs("_asdict", kwargs); err != nil {
						return err
					}
					nt := args[0].(*object.Instance)
					d := object.NewDictSized(len(fieldNames))
					for _, name := range fieldNames {
						if v, exists := nt.GetField(name); exists {
							d.SetByString(name, v)
						}
					}
					return d
				},
				HelpText: `_asdict() - Return fields as a dict`,
			}

			// fieldValues returns a namedtuple instance's values in field order.
			fieldValues := func(nt *object.Instance) []object.Object {
				vals := make([]object.Object, len(fieldNames))
				for i, name := range fieldNames {
					v, _ := nt.GetField(name)
					if v == nil {
						v = &object.Null{}
					}
					vals[i] = v
				}
				return vals
			}
			// otherValues extracts comparable values from a namedtuple
			// instance or a plain tuple.
			otherValues := func(o object.Object) ([]object.Object, bool) {
				switch ov := o.(type) {
				case *object.Tuple:
					return ov.Elements, true
				case *object.Instance:
					if tn, has := ov.GetField("__typename__"); has && tn != nil {
						if fs, ok := ov.GetField("__fields__"); ok {
							if ft, ok := fs.(*object.Tuple); ok {
								vals := make([]object.Object, len(ft.Elements))
								for i, f := range ft.Elements {
									vals[i], _ = ov.GetField(f.(*object.String).StringValue())
								}
								return vals, true
							}
						}
					}
				}
				return nil, false
			}

			// Named tuples compare by value, as tuples do (and equal a plain
			// tuple with the same values).
			methods["__eq__"] = &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.NoKwargs("__eq__", kwargs); err != nil {
						return err
					}
					if len(args) != 2 {
						return errors.NewArgumentError(len(args), 2)
					}
					ov, ok := otherValues(args[1])
					if !ok || len(ov) != len(fieldNames) {
						return object.NewBoolean(false)
					}
					for i, av := range fieldValues(args[0].(*object.Instance)) {
						if av.Inspect() != ov[i].Inspect() {
							return object.NewBoolean(false)
						}
					}
					return object.NewBoolean(true)
				},
				HelpText: `__eq__(other) - Compare field values`,
			}

			// Ordering is tuple (lexicographic) ordering over the values.
			orderMethod := func(dunder, name string, keep func(cmp int) bool) object.Object {
				return &object.Builtin{
					Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
						if err := errors.NoKwargs(dunder, kwargs); err != nil {
							return err
						}
						if len(args) != 2 {
							return errors.NewArgumentError(len(args), 2)
						}
						ov, ok := otherValues(args[1])
						if !ok {
							return errors.NewTypeErrorTagged("'%s' not supported between instances of '%s' and '%s'", name, typename.StringValue(), args[1].Type().String())
						}
						cmp, okc := compareSequences(fieldValues(args[0].(*object.Instance)), ov, 0)
						if okc == compareTooDeep {
							return &object.Exception{
								Message:       "maximum recursion depth exceeded in comparison",
								ExceptionType: "RecursionError",
								Raised:        true,
							}
						}
						if okc != compareOK {
							return errors.NewTypeErrorTagged("'%s' not supported between values of incomparable types in '%s'", name, typename.StringValue())
						}
						return object.NewBoolean(keep(cmp))
					},
					HelpText: name + "(other) - Tuple ordering over the field values",
				}
			}
			methods["__lt__"] = orderMethod("__lt__", "<", func(c int) bool { return c < 0 })
			methods["__le__"] = orderMethod("__le__", "<=", func(c int) bool { return c <= 0 })
			methods["__gt__"] = orderMethod("__gt__", ">", func(c int) bool { return c > 0 })
			methods["__ge__"] = orderMethod("__ge__", ">=", func(c int) bool { return c >= 0 })

			// Named tuples are immutable: field writes raise AttributeError
			// (shares the marker frozen dataclasses use).
			methods["__frozen__"] = object.NewBoolean(true)

			// P._fields on the class itself, as in Python.
			classFields := make([]object.Object, len(fieldNames))
			for i, name := range fieldNames {
				classFields[i] = object.NewString(name)
			}
			methods["_fields"] = &object.Tuple{Elements: classFields}

			// ...and hash by content, as tuples do (consistent with __eq__).
			methods["__hash__"] = &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.NoKwargs("__hash__", kwargs); err != nil {
						return err
					}
					if err := errors.ExactArgs(args, 1); err != nil {
						return err
					}
					nt := args[0].(*object.Instance)
					h := uint64(14695981039346656037)
					for _, name := range fieldNames {
						if v, exists := nt.GetField(name); exists {
							for _, c := range v.Inspect() {
								h ^= uint64(c)
								h *= 1099511628211
							}
						}
					}
					return object.NewInteger(int64(h))
				},
				HelpText: `__hash__() - Hash by field values`,
			}

			ntClass = &object.Class{
				Name:    typename.StringValue(),
				Methods: methods,
			}

			return ntClass
		},
		HelpText: `namedtuple(typename, field_names) - Create a named tuple class

Creates a class for creating named tuple instances with direct attribute access.

Example:
  Point = collections.namedtuple("Point", ["x", "y"])
  p = Point(1, 2)
  p.x      # 1 (direct attribute access)
  p["y"]   # 2 (dict-style access)
  p.x()    # Also works for backward compatibility`,
	},
	"ChainMap": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("ChainMap", kwargs); err != nil {
				return err
			}
			// ChainMap(*maps) - Group multiple dicts for single lookup
			// Returns a special dict that chains lookups
			chainMap := object.NewDict()

			// Store the chain of maps
			maps := make([]object.Object, len(args))
			for i, arg := range args {
				if _, ok := arg.(*object.Dict); !ok {
					return errors.NewTypeError("DICT", arg.Type().String())
				}
				maps[i] = arg
			}

			// Merge all dicts (first has priority)
			for i := len(args) - 1; i >= 0; i-- {
				d := args[i].(*object.Dict)
				chainMap.StoreFrom(d)
			}

			return chainMap
		},
		HelpText: `ChainMap(*maps) - Group multiple dicts for single lookup

Creates a single dict view over multiple dicts. First dict has priority.

Example:
  d1 = {"a": 1}
  d2 = {"b": 2, "a": 10}
  cm = collections.ChainMap(d1, d2)
  cm["a"]  # 1 (from d1)
  cm["b"]  # 2 (from d2)`,
	},
}, map[string]object.Object{
	"Counter": CounterClass,
	"deque":   DequeClass,
}, "Python-compatible collections library for specialized container datatypes")

// copyDequeFn is wired in init() to break the DequeClass initialization
// cycle (the class's copy method creates a deque instance of the class).
var copyDequeFn func(object.Object) object.Object

var counterClassRef *object.Class

func init() {
	copyDequeFn = copyDeque
	dequeClassRef = DequeClass
	counterClassRef = CounterClass
}

// reprBuilder assembles a library type's repr, such as P(x=1, y=[2]), held
// to object.OutputLimit as it is written: each field can be a container
// holding the same value many times over, so the repr of a small value can
// be huge, and it must stop at the limit rather than be built first.
type reprBuilder struct {
	b     strings.Builder
	limit int64
	err   object.Object
}

func newReprBuilder() *reprBuilder {
	return &reprBuilder{limit: object.OutputLimit()}
}

// WriteString appends s, recording a MemoryError once over the limit.
func (r *reprBuilder) WriteString(s string) {
	if r.err != nil {
		return
	}
	r.b.WriteString(s)
	if r.limit > 0 && int64(r.b.Len()) > r.limit {
		r.err = object.NewOutputTooLargeError("repr", r.limit)
	}
}

// Value appends v as reprValue renders it, with containers rendered within
// the limit (a raise while rendering is recorded).
func (r *reprBuilder) Value(v object.Object) {
	if r.err != nil {
		return
	}
	if object.IsReprContainer(v) {
		s, errObj := object.InspectRepr(v, nil)
		if errObj != nil {
			r.err = errObj
			return
		}
		r.WriteString(s)
		return
	}
	r.WriteString(reprValue(v))
}

// Result returns the repr, or the error that stopped it.
func (r *reprBuilder) Result() (string, object.Object) {
	return r.b.String(), r.err
}

// fieldsRepr renders Typename(name=value, ...) for the fields get returns.
func fieldsRepr(typename string, names []string, get func(string) (object.Object, bool)) (string, object.Object) {
	r := newReprBuilder()
	r.WriteString(typename)
	r.WriteString("(")
	first := true
	for _, name := range names {
		v, ok := get(name)
		if !ok {
			continue
		}
		if !first {
			r.WriteString(", ")
		}
		first = false
		r.WriteString(name)
		r.WriteString("=")
		r.Value(v)
	}
	r.WriteString(")")
	return r.Result()
}

// reprValue renders a value for a namedtuple repr: strings use Python
// quoting, everything else uses its Inspect.
func reprValue(v object.Object) string {
	if s, ok := v.(*object.String); ok {
		q := "'"
		if strings.Contains(s.StringValue(), "'") && !strings.Contains(s.StringValue(), "\"") {
			q = "\""
		}
		return q + s.StringValue() + q
	}
	return v.Inspect()
}

// sliceElements applies Python slice semantics (start/end/step, negative
// indices, clamping) to a value list.
func sliceElements(vals []object.Object, sl *object.Slice) []object.Object {
	n := int64(len(vals))
	step := int64(1)
	if sl.Step != nil {
		step = sl.Step.IntValue()
	}
	if step == 0 {
		return nil
	}
	clamp := func(v *object.Integer, def, lo, hi int64) int64 {
		if v == nil {
			return def
		}
		i := v.IntValue()
		if i < 0 {
			i += n
		}
		if i < lo {
			i = lo
		}
		if i > hi {
			i = hi
		}
		return i
	}
	out := []object.Object{}
	if step > 0 {
		for i := clamp(sl.Start, 0, 0, n); i < clamp(sl.End, n, 0, n); i += step {
			out = append(out, vals[i])
		}
	} else {
		for i := clamp(sl.Start, n-1, -1, n-1); i > clamp(sl.End, -1, -1, n-1); i += step {
			out = append(out, vals[i])
		}
	}
	return out
}

// compareResult says whether compareValues could order its operands.
type compareResult int

const (
	compareOK           compareResult = iota
	compareIncomparable               // a TypeError
	compareTooDeep                    // nested past object.MaxReprDepth: a RecursionError
)

// compareValues orders two scalar or tuple values like Python (numbers
// numerically across int/float, strings lexicographically, tuples
// element-wise). Tuples recurse in Go, so nesting past object.MaxReprDepth
// is refused rather than overflowing the stack.
func compareValues(a, b object.Object, depth int) (int, compareResult) {
	if af, err := a.AsFloat(); err == nil {
		if _, isStr := a.(*object.String); !isStr {
			if bf, err := b.AsFloat(); err == nil {
				if _, isStr := b.(*object.String); !isStr {
					switch {
					case af < bf:
						return -1, compareOK
					case af > bf:
						return 1, compareOK
					}
					return 0, compareOK
				}
			}
		}
	}
	as, aok := a.(*object.String)
	bs, bok := b.(*object.String)
	if aok && bok {
		return strings.Compare(as.StringValue(), bs.StringValue()), compareOK
	}
	at, aok := a.(*object.Tuple)
	bt, bok := b.(*object.Tuple)
	if aok && bok {
		return compareSequences(at.Elements, bt.Elements, depth+1)
	}
	return 0, compareIncomparable
}

// compareSequences orders two element slices lexicographically; elements
// that are the same object are equal without being compared, as in Python.
func compareSequences(a, b []object.Object, depth int) (int, compareResult) {
	if depth >= object.MaxReprDepth {
		return 0, compareTooDeep
	}
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] == b[i] {
			continue
		}
		c, res := compareValues(a[i], b[i], depth)
		if res != compareOK {
			return 0, res
		}
		if c != 0 {
			return c, compareOK
		}
	}
	switch {
	case len(a) < len(b):
		return -1, compareOK
	case len(a) > len(b):
		return 1, compareOK
	}
	return 0, compareOK
}

// pythonKeywords are the names namedtuple refuses as field or type names.
var pythonKeywords = map[string]bool{
	"False": true, "None": true, "True": true, "and": true, "as": true, "assert": true,
	"async": true, "await": true, "break": true, "class": true, "continue": true,
	"def": true, "del": true, "elif": true, "else": true, "except": true,
	"finally": true, "for": true, "from": true, "global": true, "if": true,
	"import": true, "in": true, "is": true, "lambda": true, "nonlocal": true,
	"not": true, "or": true, "pass": true, "raise": true, "return": true,
	"try": true, "while": true, "with": true, "yield": true,
}

// isPyIdentifier reports whether s is a Python identifier.
func isPyIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if r == '_' || unicode.IsLetter(r) || (i > 0 && unicode.IsDigit(r)) {
			continue
		}
		return false
	}
	return true
}

// checkNamedTupleFields raises Python's ValueError for a namedtuple type or
// field name that is not an identifier, is a keyword, starts with an
// underscore or repeats. With rename=True, bad field names are replaced by
// _0, _1, ... in place, as in Python.
func checkNamedTupleFields(typename string, fields []string, rename bool) object.Object {
	seen := make(map[string]bool, len(fields))
	for i, f := range fields {
		if rename && (!isPyIdentifier(f) || pythonKeywords[f] || strings.HasPrefix(f, "_") || seen[f]) {
			fields[i] = fmt.Sprintf("_%d", i)
			f = fields[i]
		}
		seen[f] = true
	}
	for _, name := range append([]string{typename}, fields...) {
		if !isPyIdentifier(name) {
			return errors.NewValueError("Type names and field names must be valid identifiers: '%s'", name)
		}
		if pythonKeywords[name] {
			return errors.NewValueError("Type names and field names cannot be a keyword: '%s'", name)
		}
	}
	dup := make(map[string]bool, len(fields))
	for _, f := range fields {
		if strings.HasPrefix(f, "_") && !rename {
			return errors.NewValueError("Field names cannot start with an underscore: '%s'", f)
		}
		if dup[f] {
			return errors.NewValueError("Encountered duplicate field name: '%s'", f)
		}
		dup[f] = true
	}
	return nil
}
