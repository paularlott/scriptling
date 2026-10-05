package stdlib

import (
	"context"
	"strings"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// Counter class for counting elements

// DefaultDict class for dicts with default factory behavior
var DefaultDictClass = &object.Class{
	Name: "DefaultDict",
	Methods: map[string]object.Object{
		"__init__": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				// __init__(self, default_factory) - Initialize defaultdict
				if err := errors.ExactArgs(args, 2); err != nil {
					return err
				}
				dd := args[0].(*object.Instance)
				factory := args[1]

				// Store factory
				dd.SetField("__default_factory__", factory)
				return &object.Null{}
			},
		},
		"__getitem__": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				// __getitem__(self, key) - Get value with default creation
				if err := errors.ExactArgs(args, 2); err != nil {
					return err
				}
				dd := args[0].(*object.Instance)
				key := args[1].Inspect()

				// Check if key exists
				if value, exists := dd.GetField(key); exists {
					return value
				}

				// Get factory
				factory, hasFactory := dd.GetField("__default_factory__")
				if !hasFactory {
					return &object.Null{}
				}

				// Create default value based on factory
				var defaultValue object.Object
				switch f := factory.(type) {
				case *object.Builtin:
					// Call builtin with appropriate default arg
					// For int(), float(), str(), list(), dict() we call with no args or default values
					// Try calling with no args first (for list, dict constructors)
					defaultValue = f.Fn(ctx, object.NewKwargs(nil))
					if object.IsError(defaultValue) {
						// If that fails, try with a default value (for int, float, str)
						defaultValue = f.Fn(ctx, object.NewKwargs(nil), object.NewInteger(0))
						if object.IsError(defaultValue) {
							return defaultValue
						}
					}
				case *object.String:
					// Type name as string (for backward compatibility)
					switch f.StringValue() {
					case "int":
						defaultValue = object.NewInteger(0)
					case "float":
						defaultValue = object.NewFloat(0)
					case "str":
						defaultValue = object.NewString("")
					case "list":
						defaultValue = &object.List{Elements: []object.Object{}}
					case "dict":
						defaultValue = &object.Dict{Pairs: make(map[string]object.DictPair)}
					default:
						return errors.NewError("unknown default factory type: %s", f.StringValue())
					}
				default:
					return errors.NewError("default_factory must be a builtin function or type name")
				}

				// Store and return
				dd.SetField(key, defaultValue)
				return defaultValue
			},
			HelpText: `__getitem__(key) - Get value with default creation (supports d[key] syntax)`,
		},
		"__setitem__": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				// __setitem__(self, key, value) - Set value
				if err := errors.ExactArgs(args, 3); err != nil {
					return err
				}
				dd := args[0].(*object.Instance)
				key := args[1].Inspect()
				value := args[2]

				dd.SetField(key, value)
				return &object.Null{}
			},
			HelpText: `__setitem__(key, value) - Set value (supports d[key] = value syntax)`,
		},
		"__contains__": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				// __contains__(self, key) - key in d; entries are fields
				if err := errors.ExactArgs(args, 2); err != nil {
					return err
				}
				dd := args[0].(*object.Instance)
				_, exists := dd.GetField(args[1].Inspect())
				return object.NewBoolean(exists)
			},
			HelpText: `__contains__(key) - Support the ` + "`in`" + ` operator`,
		},
		"keys": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				// keys() - entry names, used by dict(defaultdict)
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}
				dd := args[0].(*object.Instance)
				names := []object.Object{}
				dd.RangeFields(func(name string, _ object.Object) bool {
					if !strings.HasPrefix(name, "__") {
						names = append(names, object.NewString(name))
					}
					return true
				})
				return &object.List{Elements: names}
			},
			HelpText: `keys() - The mapping's keys`,
		},
		"__len__": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				// __len__(self) - Number of entries (internal dunder fields excluded)
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}
				dd := args[0].(*object.Instance)
				count := 0
				dd.RangeFields(func(name string, _ object.Object) bool {
					if !strings.HasPrefix(name, "__") {
						count++
					}
					return true
				})
				return object.NewInteger(int64(count))
			},
			HelpText: `__len__() - Number of entries`,
		},
	},
}

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

	"OrderedDict": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			// OrderedDict([items]) - Dict that remembers insertion order
			// Note: In modern Python (3.7+), regular dicts maintain order
			// Scriptling dicts also maintain order, so this just creates a dict
			od := &object.Dict{Pairs: make(map[string]object.DictPair)}

			if len(args) == 0 {
				return od
			}
			if err := errors.MaxArgs(args, 1); err != nil {
				return err
			}

			// Initialize from list of tuples or dict
			switch arg := args[0].(type) {
			case *object.List:
				for _, elem := range arg.Elements {
					tuple, ok := elem.(*object.Tuple)
					if !ok || len(tuple.Elements) != 2 {
						return errors.NewError("OrderedDict() items must be (key, value) tuples")
					}
					key := object.DictKey(tuple.Elements[0])
					od.Pairs[key] = object.DictPair{
						Key:   tuple.Elements[0],
						Value: tuple.Elements[1],
					}
				}
			case *object.Dict:
				for k, v := range arg.Pairs {
					od.Pairs[k] = v
				}
			default:
				return errors.NewTypeError("list of tuples or dict", args[0].Type().String())
			}
			return od
		},
		HelpText: `OrderedDict([items]) - Dict that remembers insertion order

Creates a dict that maintains insertion order.
Note: Scriptling dicts already maintain order, so this is equivalent to dict().

Example:
  od = collections.OrderedDict([("a", 1), ("b", 2)])`,
	},
	"deque": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			// deque([iterable[, maxlen]]) - Double-ended queue
			// Implemented as a list with special methods accessed via collections.* functions
			elements := []object.Object{}

			if len(args) >= 1 {
				switch arg := args[0].(type) {
				case *object.List:
					elements = make([]object.Object, len(arg.Elements))
					copy(elements, arg.Elements)
				case *object.Tuple:
					elements = make([]object.Object, len(arg.Elements))
					copy(elements, arg.Elements)
				case *object.String:
					for _, ch := range arg.StringValue() {
						elements = append(elements, object.NewString(string(ch)))
					}
				default:
					elems, errObj := collectIterable(ctx, args[0])
					if errObj != nil {
						return errObj
					}
					elements = elems
				}
			}

			// Handle maxlen (positional or kwarg; None means unbounded)
			maxlen := int64(-1)
			mlObj := object.Object(nil)
			if len(args) >= 2 {
				mlObj = args[1]
			} else if v := kwargs.Get("maxlen"); v != nil {
				mlObj = v
			}
			if mlObj != nil {
				if ml, ok := mlObj.(*object.Integer); ok {
					maxlen = ml.IntValue()
					if maxlen >= 0 && int64(len(elements)) > maxlen {
						// Trim from left
						elements = elements[len(elements)-int(maxlen):]
					}
				} else if mlObj.Type() != object.NULL_OBJ {
					return errors.NewTypeError("INTEGER or None", mlObj.Type().String())
				}
			}

			return createDequeInstance(elements, maxlen)
		},
		HelpText: `deque([iterable[, maxlen]]) - Double-ended queue

Creates a double-ended queue with appendleft/popleft/rotate and friends,
like Python's collections.deque. maxlen (or None) bounds the length,
dropping from the opposite end on overflow.

Example:
  d = collections.deque([1, 2, 3])
  d.appendleft(0)  # deque([0, 1, 2, 3])`,
	},
	"namedtuple": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
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
						return errors.NewError("field names must be strings")
					}
				}
			case *object.Tuple:
				for _, elem := range fn.Elements {
					if s, ok := elem.(*object.String); ok {
						fieldNames = append(fieldNames, s.StringValue())
					} else {
						return errors.NewError("field names must be strings")
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
					for name, v := range kwargs.Kwargs {
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
					fieldsTuple := &object.Tuple{Elements: fieldNameObjs}
					nt.SetField("__fields__", fieldsTuple)
					nt.SetField("_fields", fieldsTuple)
					// Precompute the display form (the __str_repr__ idiom
					// datetime uses) so print() shows P(x=1, y='hi').
					parts := make([]string, 0, len(fieldNames))
					for i, name := range fieldNames {
						parts = append(parts, name+"="+reprValue(args[i+1]))
					}
					nt.SetField("__str_repr__", object.NewString(typename.StringValue()+"("+strings.Join(parts, ", ")+")"))
					return &object.Null{}
				},
			}

			// __getitem__ for dict-like access
			methods["__getitem__"] = &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
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
					nt := args[0].(*object.Instance)
					parts := make([]string, 0, len(fieldNames))
					for _, name := range fieldNames {
						if v, exists := nt.GetField(name); exists {
							parts = append(parts, name+"="+reprValue(v))
						}
					}
					return object.NewString(typename.StringValue() + "(" + strings.Join(parts, ", ") + ")")
				},
			}

			// _replace(**changes) returns a new instance with the named
			// fields replaced; the original is untouched, as in Python.
			methods["_replace"] = &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					nt := args[0].(*object.Instance)
					fields := nt.FieldsSnapshot()
					for name, v := range kwargs.Kwargs {
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
					parts := make([]string, 0, len(fieldNames))
					for _, name := range fieldNames {
						parts = append(parts, name+"="+reprValue(fields[name]))
					}
					fields["__str_repr__"] = object.NewString(typename.StringValue() + "(" + strings.Join(parts, ", ") + ")")
					return object.NewInstanceWithFields(ntClass, fields)
				},
				HelpText: `_replace(**changes) - Return a new instance with fields replaced`,
			}

			// _asdict() returns the fields as a dict.
			methods["_asdict"] = &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					nt := args[0].(*object.Instance)
					d := &object.Dict{Pairs: make(map[string]object.DictPair, len(fieldNames))}
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
			orderMethod := func(name string, keep func(cmp int) bool) object.Object {
				return &object.Builtin{
					Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
						if len(args) != 2 {
							return errors.NewArgumentError(len(args), 2)
						}
						ov, ok := otherValues(args[1])
						if !ok {
							return errors.NewTypeErrorTagged("'%s' not supported between instances of '%s' and '%s'", name, typename.StringValue(), args[1].Type().String())
						}
						cmp, okc := compareSequences(fieldValues(args[0].(*object.Instance)), ov)
						if !okc {
							return errors.NewTypeErrorTagged("'%s' not supported between values of incomparable types in '%s'", name, typename.StringValue())
						}
						return object.NewBoolean(keep(cmp))
					},
					HelpText: name + "(other) - Tuple ordering over the field values",
				}
			}
			methods["__lt__"] = orderMethod("<", func(c int) bool { return c < 0 })
			methods["__le__"] = orderMethod("<=", func(c int) bool { return c <= 0 })
			methods["__gt__"] = orderMethod(">", func(c int) bool { return c > 0 })
			methods["__ge__"] = orderMethod(">=", func(c int) bool { return c >= 0 })

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
			// ChainMap(*maps) - Group multiple dicts for single lookup
			// Returns a special dict that chains lookups
			chainMap := &object.Dict{Pairs: make(map[string]object.DictPair)}

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
				for k, v := range d.Pairs {
					chainMap.Pairs[k] = v
				}
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
	"Counter":     CounterClass,
	"DefaultDict": DefaultDictClass,
	"defaultdict": DefaultDictClass,
}, "Python-compatible collections library for specialized container datatypes")

// createDequeInstance wraps elements in a Deque object with Python's deque
// methods; maxlen (nil/-1 for none) drops from the opposite end on overflow.
func createDequeInstance(elements []object.Object, maxlen int64) *object.Instance {
	if elements == nil {
		elements = []object.Object{}
	}
	var maxObj object.Object = &object.Null{}
	if maxlen >= 0 {
		maxObj = object.NewInteger(maxlen)
	}
	return object.NewInstanceWithFields(DequeClass, map[string]object.Object{
		"_elements": &object.List{Elements: elements},
		"maxlen":    maxObj,
	})
}

func dequeElems(inst *object.Instance) []object.Object {
	return inst.Field("_elements").(*object.List).Elements
}

func setDequeElems(inst *object.Instance, elems []object.Object) {
	inst.SetField("_elements", &object.List{Elements: elems})
}

// dequeClamp enforces maxlen after items were added: as in Python, overflow
// drops from the opposite end (addedRight: from the left, else the right).
func dequeClamp(inst *object.Instance, elems []object.Object, addedRight bool) []object.Object {
	ml, ok := inst.Field("maxlen").(*object.Integer)
	if !ok || ml.IntValue() < 0 {
		return elems
	}
	for int64(len(elems)) > ml.IntValue() {
		if addedRight {
			elems = elems[1:]
		} else {
			elems = elems[:len(elems)-1]
		}
	}
	return elems
}

// DequeClass is collections.deque: a double-ended queue.
var DequeClass = &object.Class{
	Name: "deque",
	Methods: map[string]object.Object{
		"append": &object.Builtin{Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}
			inst := args[0].(*object.Instance)
			elems := append(dequeElems(inst), args[1])
			setDequeElems(inst, dequeClamp(inst, elems, true))
			return &object.Null{}
		}},
		"appendleft": &object.Builtin{Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}
			inst := args[0].(*object.Instance)
			elems := append([]object.Object{args[1]}, dequeElems(inst)...)
			setDequeElems(inst, dequeClamp(inst, elems, false))
			return &object.Null{}
		}},
		"pop": &object.Builtin{Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			inst := args[0].(*object.Instance)
			elems := dequeElems(inst)
			if len(elems) == 0 {
				return &object.Exception{Message: "pop from an empty deque", ExceptionType: object.ExceptionTypeIndexError, Raised: true}
			}
			v := elems[len(elems)-1]
			setDequeElems(inst, elems[:len(elems)-1])
			return v
		}},
		"popleft": &object.Builtin{Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			inst := args[0].(*object.Instance)
			elems := dequeElems(inst)
			if len(elems) == 0 {
				return &object.Exception{Message: "pop from an empty deque", ExceptionType: object.ExceptionTypeIndexError, Raised: true}
			}
			v := elems[0]
			setDequeElems(inst, elems[1:])
			return v
		}},
		"extend": &object.Builtin{Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}
			inst := args[0].(*object.Instance)
			add, errObj := collectIterable(ctx, args[1])
			if errObj != nil {
				return errObj
			}
			elems := append(dequeElems(inst), add...)
			setDequeElems(inst, dequeClamp(inst, elems, true))
			return &object.Null{}
		}},
		"extendleft": &object.Builtin{Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}
			inst := args[0].(*object.Instance)
			add, errObj := collectIterable(ctx, args[1])
			if errObj != nil {
				return errObj
			}
			// Python's extendleft appends each item to the left, so the
			// iterable's order reverses.
			elems := dequeElems(inst)
			for _, v := range add {
				elems = append([]object.Object{v}, elems...)
			}
			setDequeElems(inst, dequeClamp(inst, elems, false))
			return &object.Null{}
		}},
		"rotate": &object.Builtin{Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) < 1 || len(args) > 2 {
				return errors.NewError("rotate() takes 0-1 arguments (%d given)", len(args)-1)
			}
			inst := args[0].(*object.Instance)
			n := int64(1)
			if len(args) == 2 {
				v, err := args[1].AsInt()
				if err != nil {
					return errors.ParameterError("n", err)
				}
				n = v
			}
			elems := dequeElems(inst)
			length := int64(len(elems))
			if length == 0 {
				return &object.Null{}
			}
			n = ((n % length) + length) % length
			rotated := append(append([]object.Object{}, elems[length-n:]...), elems[:length-n]...)
			setDequeElems(inst, rotated)
			return &object.Null{}
		}},
		"clear": &object.Builtin{Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			setDequeElems(args[0].(*object.Instance), []object.Object{})
			return &object.Null{}
		}},
		"copy": &object.Builtin{Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			inst := args[0].(*object.Instance)
			elems := append([]object.Object{}, dequeElems(inst)...)
			ml := int64(-1)
			if m, ok := inst.Field("maxlen").(*object.Integer); ok {
				ml = m.IntValue()
			}
			return newDeque(elems, ml)
		}},
		"count": &object.Builtin{Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}
			n := int64(0)
			for _, v := range dequeElems(args[0].(*object.Instance)) {
				if objectsEqual(v, args[1]) {
					n++
				}
			}
			return object.NewInteger(n)
		}},
		"__len__": &object.Builtin{Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			return object.NewInteger(int64(len(dequeElems(args[0].(*object.Instance)))))
		}},
		"__iter__": &object.Builtin{Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			elems := append([]object.Object{}, dequeElems(args[0].(*object.Instance))...)
			i := 0
			return object.NewIterator(func() (object.Object, bool) {
				if i >= len(elems) {
					return nil, false
				}
				v := elems[i]
				i++
				return v, true
			})
		}},
		"__getitem__": &object.Builtin{Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}
			elems := dequeElems(args[0].(*object.Instance))
			idx, err := args[1].AsInt()
			if err != nil {
				return errors.ParameterError("index", err)
			}
			if idx < 0 {
				idx += int64(len(elems))
			}
			if idx < 0 || idx >= int64(len(elems)) {
				return &object.Exception{Message: "deque index out of range", ExceptionType: object.ExceptionTypeIndexError, Raised: true}
			}
			return elems[idx]
		}},
		"__bool__": &object.Builtin{Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			return object.NewBoolean(len(dequeElems(args[0].(*object.Instance))) > 0)
		}},
		"__str__": &object.Builtin{Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			inst := args[0].(*object.Instance)
			parts := make([]string, 0, 8)
			for _, v := range dequeElems(inst) {
				parts = append(parts, v.Inspect())
			}
			out := "deque([" + strings.Join(parts, ", ") + "]"
			if m, ok := inst.Field("maxlen").(*object.Integer); ok {
				out += ", maxlen=" + m.Inspect()
			}
			return object.NewString(out + ")")
		}},
	},
}

// newDeque is wired in init() to break the DequeClass initialization cycle.
var newDeque func(elements []object.Object, maxlen int64) *object.Instance

var counterClassRef *object.Class

func init() {
	newDeque = createDequeInstance
	counterClassRef = CounterClass
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

// compareValues orders two scalar or tuple values like Python (numbers
// numerically across int/float, strings lexicographically, tuples
// element-wise); ok is false for incomparable types.
func compareValues(a, b object.Object) (int, bool) {
	if af, err := a.AsFloat(); err == nil {
		if _, isStr := a.(*object.String); !isStr {
			if bf, err := b.AsFloat(); err == nil {
				if _, isStr := b.(*object.String); !isStr {
					switch {
					case af < bf:
						return -1, true
					case af > bf:
						return 1, true
					}
					return 0, true
				}
			}
		}
	}
	as, aok := a.(*object.String)
	bs, bok := b.(*object.String)
	if aok && bok {
		return strings.Compare(as.StringValue(), bs.StringValue()), true
	}
	at, aok := a.(*object.Tuple)
	bt, bok := b.(*object.Tuple)
	if aok && bok {
		return compareSequences(at.Elements, bt.Elements)
	}
	return 0, false
}

func compareSequences(a, b []object.Object) (int, bool) {
	for i := 0; i < len(a) && i < len(b); i++ {
		c, ok := compareValues(a[i], b[i])
		if !ok {
			return 0, false
		}
		if c != 0 {
			return c, true
		}
	}
	switch {
	case len(a) < len(b):
		return -1, true
	case len(a) > len(b):
		return 1, true
	}
	return 0, true
}
