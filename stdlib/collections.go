package stdlib

import (
	"context"
	"sort"
	"strings"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// CollectionsLibrary provides Python-like collections functions
var CollectionsLibrary = object.NewLibrary(map[string]*object.Builtin{
	"Counter": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			// Counter([iterable]) - Count elements
			counter := &object.Dict{Pairs: make(map[string]object.DictPair)}

			if len(args) == 0 {
				return counter
			}
			if len(args) > 1 {
				return errors.NewArgumentError(len(args), 1)
			}

			switch arg := args[0].(type) {
			case *object.List:
				for _, elem := range arg.Elements {
					key := elem.Inspect()
					if pair, exists := counter.Pairs[key]; exists {
						if count, ok := pair.Value.(*object.Integer); ok {
							counter.Pairs[key] = object.DictPair{
								Key:   elem,
								Value: object.NewInteger(count.Value + 1),
							}
						}
					} else {
						counter.Pairs[key] = object.DictPair{
							Key:   elem,
							Value: object.NewInteger(1),
						}
					}
				}
			case *object.Tuple:
				for _, elem := range arg.Elements {
					key := elem.Inspect()
					if pair, exists := counter.Pairs[key]; exists {
						if count, ok := pair.Value.(*object.Integer); ok {
							counter.Pairs[key] = object.DictPair{
								Key:   elem,
								Value: object.NewInteger(count.Value + 1),
							}
						}
					} else {
						counter.Pairs[key] = object.DictPair{
							Key:   elem,
							Value: object.NewInteger(1),
						}
					}
				}
			case *object.String:
				for _, ch := range arg.Value {
					key := string(ch)
					strKey := &object.String{Value: key}
					if pair, exists := counter.Pairs[key]; exists {
						if count, ok := pair.Value.(*object.Integer); ok {
							counter.Pairs[key] = object.DictPair{
								Key:   strKey,
								Value: object.NewInteger(count.Value + 1),
							}
						}
					} else {
						counter.Pairs[key] = object.DictPair{
							Key:   strKey,
							Value: object.NewInteger(1),
						}
					}
				}
			case *object.Dict:
				// Copy existing dict
				for k, v := range arg.Pairs {
					counter.Pairs[k] = v
				}
			default:
				return errors.NewTypeError("iterable or dict", args[0].Type().String())
			}

			return counter
		},
		HelpText: `Counter([iterable]) - Count elements

Creates a dict-like object that counts occurrences of elements.

Example:
  collections.Counter([1, 1, 2, 3, 3, 3]) -> {1: 2, 2: 1, 3: 3}
  collections.Counter("hello") -> {"h": 1, "e": 1, "l": 2, "o": 1}`,
	},
	"most_common": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			// most_common(counter[, n]) - Return n most common elements
			if len(args) < 1 || len(args) > 2 {
				return errors.NewArgumentError(len(args), 1)
			}
			counter, ok := args[0].(*object.Dict)
			if !ok {
				return errors.NewTypeError("DICT (Counter)", args[0].Type().String())
			}

			n := len(counter.Pairs)
			if len(args) == 2 {
				if nArg, ok := args[1].(*object.Integer); ok {
					n = int(nArg.Value)
				} else {
					return errors.NewTypeError("INTEGER", args[1].Type().String())
				}
			}

			// Convert to sortable slice
			type pair struct {
				key   object.Object
				count int64
			}
			pairs := make([]pair, 0, len(counter.Pairs))
			for _, p := range counter.Pairs {
				if count, ok := p.Value.(*object.Integer); ok {
					pairs = append(pairs, pair{key: p.Key, count: count.Value})
				}
			}

			// Sort by count descending
			sort.Slice(pairs, func(i, j int) bool {
				return pairs[i].count > pairs[j].count
			})

			// Take top n
			if n > len(pairs) {
				n = len(pairs)
			}
			result := make([]object.Object, n)
			for i := 0; i < n; i++ {
				result[i] = &object.Tuple{Elements: []object.Object{
					pairs[i].key,
					object.NewInteger(pairs[i].count),
				}}
			}
			return &object.List{Elements: result}
		},
		HelpText: `most_common(counter[, n]) - Return n most common elements

Returns a list of (element, count) tuples sorted by count.

Example:
  c = collections.Counter([1, 1, 2, 3, 3, 3])
  collections.most_common(c, 2) -> [(3, 3), (1, 2)]`,
	},
	"defaultdict": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			// defaultdict(default_factory) - Dict with default values
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}

			// Store the default factory in a special wrapper
			factory := args[0]
			defaultDict := &object.Dict{Pairs: make(map[string]object.DictPair)}

			// Store factory as metadata (using a special key)
			defaultDict.Pairs["__default_factory__"] = object.DictPair{
				Key:   &object.String{Value: "__default_factory__"},
				Value: factory,
			}

			return defaultDict
		},
		HelpText: `defaultdict(default_factory) - Dict with default values

Creates a dict that returns a default value for missing keys.
The default_factory should be a type like int, list, str, or a function.

Note: In Scriptling, defaultdict returns a regular dict with a stored factory.
Use collections.get_default() to get values with auto-creation.

Example:
  d = collections.defaultdict(list)
  collections.get_default(d, "key")  # Returns []`,
	},
	"get_default": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			// get_default(defaultdict, key) - Get value with default creation
			if len(args) != 2 {
				return errors.NewArgumentError(len(args), 2)
			}
			dd, ok := args[0].(*object.Dict)
			if !ok {
				return errors.NewTypeError("DICT", args[0].Type().String())
			}
			key := args[1].Inspect()

			// Check if key exists
			if pair, exists := dd.Pairs[key]; exists && key != "__default_factory__" {
				return pair.Value
			}

			// Get factory
			factoryPair, hasFactory := dd.Pairs["__default_factory__"]
			if !hasFactory {
				return &object.Null{}
			}

			// Create default value based on factory
			var defaultValue object.Object
			switch f := factoryPair.Value.(type) {
			case *object.Builtin:
				defaultValue = f.Fn(ctx, nil)
				if isError(defaultValue) {
					return defaultValue
				}
			case *object.String:
				// Type name as string
				switch f.Value {
				case "int":
					defaultValue = object.NewInteger(0)
				case "float":
					defaultValue = &object.Float{Value: 0}
				case "str":
					defaultValue = &object.String{Value: ""}
				case "list":
					defaultValue = &object.List{Elements: []object.Object{}}
				case "dict":
					defaultValue = &object.Dict{Pairs: make(map[string]object.DictPair)}
				default:
					return errors.NewError("unknown default factory type: %s", f.Value)
				}
			default:
				return errors.NewError("default_factory must be a builtin function or type name")
			}

			// Store and return
			dd.Pairs[key] = object.DictPair{
				Key:   args[1],
				Value: defaultValue,
			}
			return defaultValue
		},
		HelpText: `get_default(defaultdict, key) - Get value with default creation

Gets a value from a defaultdict, creating it if it doesn't exist.

Example:
  d = collections.defaultdict("list")
  collections.get_default(d, "items")  # Returns [] and stores it`,
	},
	"OrderedDict": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			// OrderedDict([items]) - Dict that remembers insertion order
			// Note: In modern Python (3.7+), regular dicts maintain order
			// Scriptling dicts also maintain order, so this just creates a dict
			od := &object.Dict{Pairs: make(map[string]object.DictPair)}

			if len(args) == 0 {
				return od
			}
			if len(args) > 1 {
				return errors.NewArgumentError(len(args), 1)
			}

			// Initialize from list of tuples or dict
			switch arg := args[0].(type) {
			case *object.List:
				for _, elem := range arg.Elements {
					tuple, ok := elem.(*object.Tuple)
					if !ok || len(tuple.Elements) != 2 {
						return errors.NewError("OrderedDict() items must be (key, value) tuples")
					}
					key := tuple.Elements[0].Inspect()
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
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
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
					for _, ch := range arg.Value {
						elements = append(elements, &object.String{Value: string(ch)})
					}
				default:
					return errors.NewTypeError("iterable", args[0].Type().String())
				}
			}

			// Handle maxlen
			maxlen := int64(-1)
			if len(args) >= 2 {
				if ml, ok := args[1].(*object.Integer); ok {
					maxlen = ml.Value
					if maxlen >= 0 && int64(len(elements)) > maxlen {
						// Trim from left
						elements = elements[len(elements)-int(maxlen):]
					}
				} else if args[1].Type() != object.NULL_OBJ {
					return errors.NewTypeError("INTEGER or None", args[1].Type().String())
				}
			}

			// Store maxlen as metadata
			dequeList := &object.List{Elements: elements}
			// We'll store maxlen in a wrapper. For simplicity, return the list.
			// Users should use deque_* functions for operations
			return dequeList
		},
		HelpText: `deque([iterable[, maxlen]]) - Double-ended queue

Creates a double-ended queue (deque) from an iterable.
Use collections.deque_* functions for deque-specific operations.

Example:
  d = collections.deque([1, 2, 3])
  collections.deque_appendleft(d, 0)  # [0, 1, 2, 3]`,
	},
	"deque_appendleft": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			// deque_appendleft(deque, elem) - Add element to left
			if len(args) != 2 {
				return errors.NewArgumentError(len(args), 2)
			}
			deque, ok := args[0].(*object.List)
			if !ok {
				return errors.NewTypeError("LIST (deque)", args[0].Type().String())
			}
			newElements := make([]object.Object, len(deque.Elements)+1)
			newElements[0] = args[1]
			copy(newElements[1:], deque.Elements)
			deque.Elements = newElements
			return &object.Null{}
		},
		HelpText: `deque_appendleft(deque, elem) - Add element to left side

Adds an element to the left side of the deque.

Example:
  d = collections.deque([1, 2, 3])
  collections.deque_appendleft(d, 0)  # d is now [0, 1, 2, 3]`,
	},
	"deque_popleft": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			// deque_popleft(deque) - Remove and return element from left
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			deque, ok := args[0].(*object.List)
			if !ok {
				return errors.NewTypeError("LIST (deque)", args[0].Type().String())
			}
			if len(deque.Elements) == 0 {
				return errors.NewError("popleft from empty deque")
			}
			elem := deque.Elements[0]
			deque.Elements = deque.Elements[1:]
			return elem
		},
		HelpText: `deque_popleft(deque) - Remove and return element from left

Removes and returns the leftmost element.

Example:
  d = collections.deque([1, 2, 3])
  x = collections.deque_popleft(d)  # x=1, d=[2, 3]`,
	},
	"deque_extendleft": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			// deque_extendleft(deque, iterable) - Extend left with iterable (reversed)
			if len(args) != 2 {
				return errors.NewArgumentError(len(args), 2)
			}
			deque, ok := args[0].(*object.List)
			if !ok {
				return errors.NewTypeError("LIST (deque)", args[0].Type().String())
			}
			var elements []object.Object
			switch arg := args[1].(type) {
			case *object.List:
				elements = arg.Elements
			case *object.Tuple:
				elements = arg.Elements
			default:
				return errors.NewTypeError("iterable", args[1].Type().String())
			}
			// Extend left (reversed order)
			newElements := make([]object.Object, len(elements)+len(deque.Elements))
			for i, elem := range elements {
				newElements[len(elements)-1-i] = elem
			}
			copy(newElements[len(elements):], deque.Elements)
			deque.Elements = newElements
			return &object.Null{}
		},
		HelpText: `deque_extendleft(deque, iterable) - Extend left with iterable

Extends the left side with elements from iterable (in reverse order).

Example:
  d = collections.deque([1, 2, 3])
  collections.deque_extendleft(d, [4, 5])  # d is now [5, 4, 1, 2, 3]`,
	},
	"deque_rotate": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			// deque_rotate(deque, n) - Rotate deque n steps
			if len(args) != 2 {
				return errors.NewArgumentError(len(args), 2)
			}
			deque, ok := args[0].(*object.List)
			if !ok {
				return errors.NewTypeError("LIST (deque)", args[0].Type().String())
			}
			n, ok := args[1].(*object.Integer)
			if !ok {
				return errors.NewTypeError("INTEGER", args[1].Type().String())
			}
			if len(deque.Elements) == 0 {
				return &object.Null{}
			}

			// Normalize rotation
			steps := int(n.Value) % len(deque.Elements)
			if steps < 0 {
				steps += len(deque.Elements)
			}
			if steps == 0 {
				return &object.Null{}
			}

			// Rotate right by steps
			newElements := make([]object.Object, len(deque.Elements))
			splitPoint := len(deque.Elements) - steps
			copy(newElements, deque.Elements[splitPoint:])
			copy(newElements[steps:], deque.Elements[:splitPoint])
			deque.Elements = newElements
			return &object.Null{}
		},
		HelpText: `deque_rotate(deque, n) - Rotate deque n steps

Rotates the deque n steps to the right. If n is negative, rotates left.

Example:
  d = collections.deque([1, 2, 3, 4])
  collections.deque_rotate(d, 1)  # d is now [4, 1, 2, 3]
  collections.deque_rotate(d, -1) # d is now [1, 2, 3, 4]`,
	},
	"namedtuple": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			// namedtuple(typename, field_names) - Create a named tuple class
			if len(args) != 2 {
				return errors.NewArgumentError(len(args), 2)
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
						fieldNames = append(fieldNames, s.Value)
					} else {
						return errors.NewError("field names must be strings")
					}
				}
			case *object.Tuple:
				for _, elem := range fn.Elements {
					if s, ok := elem.(*object.String); ok {
						fieldNames = append(fieldNames, s.Value)
					} else {
						return errors.NewError("field names must be strings")
					}
				}
			case *object.String:
				// Space or comma separated
				fields := strings.FieldsFunc(fn.Value, func(r rune) bool {
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

			// Return a factory function
			return &object.Builtin{
				Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
					if len(args) != len(fieldNames) {
						return errors.NewArgumentError(len(args), len(fieldNames))
					}
					// Create a dict with field names as keys
					nt := &object.Dict{Pairs: make(map[string]object.DictPair)}
					nt.Pairs["__typename__"] = object.DictPair{
						Key:   &object.String{Value: "__typename__"},
						Value: typename,
					}
					for i, name := range fieldNames {
						nt.Pairs[name] = object.DictPair{
							Key:   &object.String{Value: name},
							Value: args[i],
						}
					}
					return nt
				},
				HelpText: typename.Value + "(" + strings.Join(fieldNames, ", ") + ") - Create named tuple instance",
			}
		},
		HelpText: `namedtuple(typename, field_names) - Create a named tuple factory

Creates a factory function for creating named tuple-like dicts.

Example:
  Point = collections.namedtuple("Point", ["x", "y"])
  p = Point(1, 2)
  p["x"]  # 1`,
	},
	"ChainMap": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
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
}, nil, "Python-compatible collections library for specialized container datatypes")
