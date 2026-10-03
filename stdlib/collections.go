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
					return errors.NewTypeError("iterable", args[0].Type().String())
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

			// Create a NamedTuple class
			methods := make(map[string]object.Object)

			// __init__ method - stores fields as instance attributes
			methods["__init__"] = &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if len(args) != len(fieldNames)+1 {
						return errors.NewArgumentError(len(args), len(fieldNames)+1)
					}
					nt := args[0].(*object.Instance)
					// Store field values directly as instance fields
					for i, name := range fieldNames {
						nt.SetField(name, args[i+1])
					}
					nt.SetField("__typename__", typename)
					// Store field names for reference
					fieldNameObjs := make([]object.Object, len(fieldNames))
					for i, name := range fieldNames {
						fieldNameObjs[i] = object.NewString(name)
					}
					nt.SetField("__fields__", &object.Tuple{Elements: fieldNameObjs})
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
					key := args[1].Inspect()
					// Don't expose internal fields
					if key == "__typename__" || key == "__fields__" {
						return &object.Null{}
					}
					if value, exists := nt.GetField(key); exists {
						return value
					}
					return &object.Null{}
				},
				HelpText: `__getitem__(key) - Get field value (supports nt[key] syntax)`,
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

			ntClass := &object.Class{
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
			add, ok := object.IterableToSlice(args[1])
			if !ok {
				return errors.NewTypeError("iterable", args[1].Type().String())
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
			add, ok := object.IterableToSlice(args[1])
			if !ok {
				return errors.NewTypeError("iterable", args[1].Type().String())
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
