package evaluator

import (
	"context"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"

	"github.com/paularlott/scriptling/ast"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// Forward declarations for complex builtins
// These are defined after the builtins map for better organization
var (
	mapFunction    func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object
	filterFunction func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object
	sortedFunction func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object
	helpFunction   func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object

	// callDunderMethodFn is set in init() to break the initialization cycle
	callDunderMethodFn func(ctx context.Context, inst *object.Instance, method string, args []object.Object, env *object.Environment) object.Object

	// typeBuiltins maps type-related builtin pointers to their names for isinstance()
	typeBuiltins map[*object.Builtin]string
)

var builtins = map[string]*object.Builtin{
	"help": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			return helpFunction(ctx, kwargs, args...)
		},
		HelpText: `help([object]) - Display help information

  With no arguments: Show general help
  help("modules"): List all available libraries
  help("builtins"): List all builtin functions
  help("operators"): List all operators
  help(function): Show help for a function object
  help("function_name"): Show help for a builtin function
  help("library.function"): Show help for a library function
  help("library_name"): List functions in a library`,
	},
	"map": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			return mapFunction(ctx, kwargs, args...)
		},
		HelpText: `map(function, iterable, ...) - Apply function to every item

Returns an iterator of results from applying function to each item.
With multiple iterables, function must take that many arguments.
Use list(map(...)) to get a list.`,
	},
	"filter": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			return filterFunction(ctx, kwargs, args...)
		},
		HelpText: `filter(function, iterable) - Filter elements by function

Returns an iterator of elements for which function returns true.
If function is None, removes falsy elements.
Use list(filter(...)) to get a list.`,
	},
	"print": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			env := GetEnvFromContext(ctx)
			writer := env.GetWriter()

			// Get sep kwarg (default: " ")
			sep := " "
			if sepObj := kwargs.Get("sep"); sepObj != nil {
				if sepStr, err := sepObj.AsString(); err == nil {
					sep = sepStr
				} else if _, ok := sepObj.(*object.Null); !ok {
					return errors.NewError("sep must be None or a string, not %s", sepObj.Type())
				}
			}

			// Get end kwarg (default: "\n")
			end := "\n"
			if endObj := kwargs.Get("end"); endObj != nil {
				if endStr, err := endObj.AsString(); err == nil {
					end = endStr
				} else if _, ok := endObj.(*object.Null); !ok {
					return errors.NewError("end must be None or a string, not %s", endObj.Type())
				}
			}

			// Build output string — fast path for common single-arg case
			if len(args) == 1 && sep == " " {
				if str, err := args[0].AsString(); err == nil {
					fmt.Fprint(writer, str+end)
				} else if inst, ok := args[0].(*object.Instance); ok {
					if result := callDunderMethodFn(ctx, inst, "__str__", nil, env); result != nil {
						if s, err2 := result.AsString(); err2 == nil {
							fmt.Fprint(writer, s+end)
						} else {
							fmt.Fprint(writer, result.Inspect()+end)
						}
					} else {
						fmt.Fprint(writer, args[0].Inspect()+end)
					}
				} else {
					fmt.Fprint(writer, args[0].Inspect()+end)
				}
				return NULL
			}
			parts := make([]string, len(args))
			for i, arg := range args {
				// Use AsString() for strings to get actual value, Inspect() for others
				if str, err := arg.AsString(); err == nil {
					parts[i] = str
				} else if inst, ok := arg.(*object.Instance); ok {
					if result := callDunderMethodFn(ctx, inst, "__str__", nil, env); result != nil {
						if s, err2 := result.AsString(); err2 == nil {
							parts[i] = s
						} else {
							parts[i] = result.Inspect()
						}
					} else {
						parts[i] = arg.Inspect()
					}
				} else {
					parts[i] = arg.Inspect()
				}
			}
			fmt.Fprint(writer, strings.Join(parts, sep)+end)
			return NULL
		},
		HelpText: `print(*args, sep=" ", end="\n") - Print values to output

Prints the given arguments separated by sep and followed by end.
Default separator is a space, default ending is a newline.

Examples:
  print("hello", "world")     # Output: hello world
  print("a", "b", sep=",")    # Output: a,b
  print("no newline", end="") # Output: no newline (no trailing newline)`,
	},
	"len": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			switch arg := args[0].(type) {
			case *object.String:
				// ASCII fast-path: avoid []rune conversion for pure-ASCII strings
				if isASCII(arg.Value) {
					return object.NewInteger(int64(len(arg.Value)))
				}
				return object.NewInteger(int64(len([]rune(arg.Value))))
			case *object.List:
				return object.NewInteger(int64(len(arg.Elements)))
			case *object.Dict:
				return object.NewInteger(int64(len(arg.Pairs)))
			case *object.Tuple:
				return object.NewInteger(int64(len(arg.Elements)))
			case *object.DictKeys:
				return object.NewInteger(int64(len(arg.Dict.Pairs)))
			case *object.DictValues:
				return object.NewInteger(int64(len(arg.Dict.Pairs)))
			case *object.DictItems:
				return object.NewInteger(int64(len(arg.Dict.Pairs)))
			case *object.Set:
				return object.NewInteger(int64(len(arg.Elements)))
			case *object.Instance:
				// Call __len__ dunder method if defined
				env := GetEnvFromContext(ctx)
				if result := callDunderMethodFn(ctx, arg, "__len__", nil, env); result != nil {
					return result
				}
				return errors.NewTypeError("object with __len__", "INSTANCE")
			default:
				return errors.NewTypeError("STRING, LIST, DICT, TUPLE, SET, or VIEW", args[0].Type().String())
			}
		},
		HelpText: `len(obj) - Return the length of an object

Returns the number of items in a string, list, dict, or tuple.`,
	},
	"type": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			obj := args[0]
			if instance, ok := obj.(*object.Instance); ok {
				return &object.String{Value: instance.Class.Name}
			}
			return &object.String{Value: obj.Type().String()}
		},
		HelpText: `type(obj) - Return the type of an object

Returns a string representing the type of the object.`,
	},
	"str": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			// For exceptions, return just the message (like Python)
			if exc, ok := args[0].(*object.Exception); ok {
				return &object.String{Value: exc.Message}
			}
			// Call __str__ dunder method on instances
			if inst, ok := args[0].(*object.Instance); ok {
				env := GetEnvFromContext(ctx)
				if result := callDunderMethodFn(ctx, inst, "__str__", nil, env); result != nil {
					return result
				}
			}
			return &object.String{Value: args[0].Inspect()}
		},
		HelpText: `str(obj) - Convert an object to a string

Returns the string representation of any object.
For exceptions, returns just the exception message.`,
	},
	"int": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			switch arg := args[0].(type) {
			case *object.Integer:
				return arg
			case *object.Float:
				return object.NewInteger(int64(arg.Value))
			case *object.String:
				var val int64
				_, err := fmt.Sscanf(arg.Value, "%d", &val)
				if err != nil {
					return errors.NewError("cannot convert %s to int", arg.Value)
				}
				return object.NewInteger(val)
			default:
				return errors.NewTypeError("INTEGER, FLOAT, or STRING", arg.Type().String())
			}
		},
		HelpText: `int(obj) - Convert an object to an integer

Converts a float, string, or integer to an integer.
Floats are truncated (not rounded).`,
	},
	"float": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			switch arg := args[0].(type) {
			case *object.Float:
				return arg
			case *object.Integer:
				return &object.Float{Value: float64(arg.Value)}
			case *object.String:
				var val float64
				_, err := fmt.Sscanf(arg.Value, "%f", &val)
				if err != nil {
					return errors.NewError("cannot convert %s to float", arg.Value)
				}
				return &object.Float{Value: val}
			default:
				return errors.NewTypeError("INTEGER, FLOAT, or STRING", arg.Type().String())
			}
		},
		HelpText: `float(obj) - Convert an object to a float

Converts an integer, string, or float to a float.`,
	},

	"sum": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}

			var elements []object.Object
			switch arg := args[0].(type) {
			case *object.List:
				elements = arg.Elements
			case *object.Tuple:
				elements = arg.Elements
			default:
				return errors.NewTypeError("LIST or TUPLE", args[0].Type().String())
			}

			// Start with integer 0
			var intSum int64 = 0
			var floatSum float64 = 0
			hasFloat := false

			for _, elem := range elements {
				switch v := elem.(type) {
				case *object.Integer:
					if hasFloat {
						floatSum += float64(v.Value)
					} else {
						intSum += v.Value
					}
				case *object.Float:
					if !hasFloat {
						// Convert accumulated int sum to float
						floatSum = float64(intSum)
						hasFloat = true
					}
					floatSum += v.Value
				default:
					return errors.NewError("unsupported operand type for sum(): %s", elem.Type())
				}
			}

			if hasFloat {
				return &object.Float{Value: floatSum}
			}
			return object.NewInteger(intSum)
		},
		HelpText: `sum(iterable) - Sum elements of iterable

Returns the sum of all elements in a list or tuple.
Supports integers and floats, returns appropriate type.`,
	},
	"sorted": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			return sortedFunction(ctx, kwargs, args...)
		},
		HelpText: `sorted(iterable[, key][, reverse=False]) - Return sorted list

Returns a new sorted list from the elements of iterable.
Optional key function (builtin, function, or lambda) can be provided for custom sorting.
Set reverse=True to sort in descending order.

Example:
  sorted([3, 1, 2])                    # [1, 2, 3]
  sorted([3, 1, 2], reverse=True)      # [3, 2, 1]
  sorted(["a", "bb", "ccc"], key=len)  # ["a", "bb", "ccc"]
  sorted(files, key=lambda f: os.path.getmtime(f))  # Sort by mtime`,
	},
	"range": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.RangeArgs(args, 1, 3); err != nil {
				return err
			}
			var start, stop, step int64
			var errObj object.Object
			if len(args) == 1 {
				stop, errObj = args[0].AsInt()
				if errObj != nil {
					return errors.ParameterError("stop", errObj)
				}
				step = 1
			} else if len(args) == 2 {
				start, errObj = args[0].AsInt()
				if errObj != nil {
					return errors.ParameterError("start", errObj)
				}
				stop, errObj = args[1].AsInt()
				if errObj != nil {
					return errors.ParameterError("stop", errObj)
				}
				step = 1
			} else {
				start, errObj = args[0].AsInt()
				if errObj != nil {
					return errors.ParameterError("start", errObj)
				}
				stop, errObj = args[1].AsInt()
				if errObj != nil {
					return errors.ParameterError("stop", errObj)
				}
				step, errObj = args[2].AsInt()
				if errObj != nil {
					return errors.ParameterError("step", errObj)
				}
				if step == 0 {
					return errors.NewError("range step cannot be zero")
				}
			}
			return object.NewRangeIterator(start, stop, step)
		},
		HelpText: `range([start,] stop[, step]) - Generate sequence of numbers

Returns an iterator of integers from start (inclusive) to stop (exclusive).
If start is omitted, defaults to 0. If step is omitted, defaults to 1.
Use list(range(...)) to get a list.`,
	},
	"keys": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			if args[0].Type() != object.DICT_OBJ {
				return errors.NewTypeError("DICT", args[0].Type().String())
			}
			dict := args[0].(*object.Dict)
			return &object.DictKeys{Dict: dict}
		},
		HelpText: `keys(dict) - Return dictionary keys

Returns a view object of all keys in the dictionary.`,
	},
	"values": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			if args[0].Type() != object.DICT_OBJ {
				return errors.NewTypeError("DICT", args[0].Type().String())
			}
			dict := args[0].(*object.Dict)
			return &object.DictValues{Dict: dict}
		},
		HelpText: `values(dict) - Return dictionary values

Returns a view object of all values in the dictionary.`,
	},
	"items": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			if args[0].Type() != object.DICT_OBJ {
				return errors.NewTypeError("DICT", args[0].Type().String())
			}
			dict := args[0].(*object.Dict)
			return &object.DictItems{Dict: dict}
		},
		HelpText: `items(dict) - Return dictionary key-value pairs

Returns a view object of (key, value) pairs for all items in the dictionary.`,
	},
	"enumerate": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) < 1 || len(args) > 2 {
				return errors.NewError("enumerate() takes 1 or 2 arguments (%d given)", len(args))
			}
			start := int64(0)
			if len(args) == 2 {
				startObj, err := args[1].AsInt()
				if err != nil {
					return errors.ParameterError("start", err)
				}
				start = startObj
			}
			// Validate iterable type
			switch args[0].(type) {
			case *object.List, *object.Tuple, *object.String, *object.Iterator:
				// Valid iterable types
			default:
				return errors.NewTypeError("iterable (LIST, TUPLE, STRING, ITERATOR)", args[0].Type().String())
			}
			return object.NewEnumerateIterator(args[0], start)
		},
		HelpText: `enumerate(iterable[, start=0]) - Return (index, value) pairs

Returns an iterator of tuples containing the index and value for each item.
Default start is 0. Use list(enumerate(...)) to get a list.`,
	},
	"zip": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) == 0 {
				// Return empty iterator for no arguments
				return object.NewZipIterator([]object.Object{})
			}
			// Validate all arguments are iterable
			for _, arg := range args {
				switch arg.(type) {
				case *object.List, *object.Tuple, *object.String, *object.Iterator:
					// Valid iterable types
				default:
					return errors.NewTypeError("iterable (LIST, TUPLE, STRING, ITERATOR)", arg.Type().String())
				}
			}
			return object.NewZipIterator(args)
		},
		HelpText: `zip(*iterables) - Aggregate elements from each iterable

Returns an iterator of tuples, where the i-th tuple contains the i-th element from each of the argument sequences or iterables.
The iterator stops when the shortest input iterable is exhausted.
Use list(zip(...)) to get a list.`,
	},
	"super": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			var classObj *object.Class
			var instanceObj *object.Instance

			if len(args) == 0 {
				// Parameterless super() - infer from context
				env := GetEnvFromContext(ctx)

				// Find __class__ (should be in closure)
				obj, ok := env.Get("__class__")
				if !ok {
					return errors.NewError("super(): __class__ cell not found - are you calling super() from within a class method?")
				}
				var isClass bool
				classObj, isClass = obj.(*object.Class)
				if !isClass {
					return errors.NewError("super(): __class__ is not a class")
				}

				// Find instance (conventionally 'self')
				// Note: This relies on the first argument being named 'self'
				obj, ok = env.Get("self")
				if !ok {
					return errors.NewError("super(): 'self' not found - parameterless super() requires 'self' argument")
				}
				var isInstance bool
				instanceObj, isInstance = obj.(*object.Instance)
				if !isInstance {
					return errors.NewError("super(): 'self' is not an instance")
				}
			} else if len(args) == 2 {
				var ok bool
				classObj, ok = args[0].(*object.Class)
				if !ok {
					return errors.NewTypeError("CLASS", args[0].Type().String())
				}

				instanceObj, ok = args[1].(*object.Instance)
				if !ok {
					return errors.NewTypeError("INSTANCE", args[1].Type().String())
				}
			} else {
				if err := errors.MaxArgs(args, 2); err != nil {
					return err
				}
			}

			// Verify instance is an instance of class (or subclass)
			isInstance := false
			current := instanceObj.Class
			for current != nil {
				if current == classObj {
					isInstance = true
					break
				}
				current = current.BaseClass
			}

			if !isInstance {
				return errors.NewError("super(): obj must be an instance or subtype of type")
			}

			return &object.Super{Class: classObj, Instance: instanceObj}
		},
		HelpText: `super([type, obj]) - Return a proxy object that delegates method calls to a parent or sibling class.

		With no arguments, returns a proxy for the parent of the class containing the method code, bound to 'self'.
		With arguments, returns a proxy for the parent of 'type', bound to 'obj'.`,
	},
	"any": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			var iterable []object.Object
			switch iter := args[0].(type) {
			case *object.List:
				iterable = iter.Elements
			case *object.Tuple:
				iterable = iter.Elements
			default:
				return errors.NewTypeError("iterable (LIST, TUPLE)", args[0].Type().String())
			}
			for _, elem := range iterable {
				if isTruthy(elem) {
					return TRUE
				}
			}
			return FALSE
		},
		HelpText: `any(iterable) - Return True if any element is truthy

Returns True if at least one element in the iterable is truthy.
Returns False for an empty iterable.`,
	},
	"all": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			var iterable []object.Object
			switch iter := args[0].(type) {
			case *object.List:
				iterable = iter.Elements
			case *object.Tuple:
				iterable = iter.Elements
			default:
				return errors.NewTypeError("iterable (LIST, TUPLE)", args[0].Type().String())
			}
			for _, elem := range iterable {
				if !isTruthy(elem) {
					return FALSE
				}
			}
			return TRUE
		},
		HelpText: `all(iterable) - Return True if all elements are truthy

Returns True if all elements in the iterable are truthy (or if empty).`,
	},
	"bool": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) == 0 {
				return FALSE
			}
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			if isTruthy(args[0]) {
				return TRUE
			}
			return FALSE
		},
		HelpText: `bool([x]) - Convert value to boolean

Returns True if x is truthy, False otherwise.
With no argument, returns False.`,
	},
	"abs": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			switch num := args[0].(type) {
			case *object.Integer:
				if num.Value < 0 {
					return object.NewInteger(-num.Value)
				}
				return num
			case *object.Float:
				if num.Value < 0 {
					return &object.Float{Value: -num.Value}
				}
				return num
			default:
				return errors.NewTypeError("INTEGER or FLOAT", args[0].Type().String())
			}
		},
		HelpText: `abs(x) - Return the absolute value of a number

Works with both integers and floats.`,
	},
	"min": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) == 0 {
				return errors.NewError("min() requires at least 1 argument")
			}
			// If single argument, treat as iterable
			if len(args) == 1 {
				switch iter := args[0].(type) {
				case *object.List:
					if len(iter.Elements) == 0 {
						return errors.NewError("min() arg is an empty sequence")
					}
					args = iter.Elements
				case *object.Tuple:
					if len(iter.Elements) == 0 {
						return errors.NewError("min() arg is an empty sequence")
					}
					args = iter.Elements
				}
			}
			minVal := args[0]
			for _, arg := range args[1:] {
				cmp := compareObjects(minVal, arg)
				if cmp > 0 {
					minVal = arg
				}
			}
			return minVal
		},
		HelpText: `min(iterable) or min(a, b, c, ...) - Return the smallest item

With a single iterable argument, returns its smallest item.
With multiple arguments, returns the smallest argument.`,
	},
	"max": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) == 0 {
				return errors.NewError("max() requires at least 1 argument")
			}
			// If single argument, treat as iterable
			if len(args) == 1 {
				switch iter := args[0].(type) {
				case *object.List:
					if len(iter.Elements) == 0 {
						return errors.NewError("max() arg is an empty sequence")
					}
					args = iter.Elements
				case *object.Tuple:
					if len(iter.Elements) == 0 {
						return errors.NewError("max() arg is an empty sequence")
					}
					args = iter.Elements
				}
			}
			maxVal := args[0]
			for _, arg := range args[1:] {
				cmp := compareObjects(maxVal, arg)
				if cmp < 0 {
					maxVal = arg
				}
			}
			return maxVal
		},
		HelpText: `max(iterable) or max(a, b, c, ...) - Return the largest item

With a single iterable argument, returns its largest item.
With multiple arguments, returns the largest argument.`,
	},
	"round": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) < 1 || len(args) > 2 {
				return errors.NewError("round() takes 1 or 2 arguments (%d given)", len(args))
			}
			ndigits := 0
			if len(args) == 2 {
				nd, err := args[1].AsInt()
				if err != nil {
					return errors.ParameterError("ndigits", err)
				}
				ndigits = int(nd)
			}
			var value float64
			switch num := args[0].(type) {
			case *object.Integer:
				if ndigits >= 0 {
					return num
				}
				value = float64(num.Value)
			case *object.Float:
				value = num.Value
			default:
				return errors.NewTypeError("INTEGER or FLOAT", args[0].Type().String())
			}
			if ndigits == 0 {
				return object.NewInteger(int64(math.Round(value)))
			}
			multiplier := math.Pow(10, float64(ndigits))
			rounded := math.Round(value*multiplier) / multiplier
			if ndigits < 0 {
				return object.NewInteger(int64(rounded))
			}
			return &object.Float{Value: rounded}
		},
		HelpText: `round(number[, ndigits]) - Round a number to given precision

Rounds to ndigits decimal places (default 0).
Returns an integer if ndigits is omitted or 0.`,
	},
	"hex": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			num, err := args[0].AsInt()
			if err != nil {
				return errors.ParameterError("x", err)
			}
			if num >= 0 {
				return &object.String{Value: fmt.Sprintf("0x%x", num)}
			}
			return &object.String{Value: fmt.Sprintf("-0x%x", -num)}
		},
		HelpText: `hex(x) - Convert an integer to a lowercase hexadecimal string prefixed with "0x"`,
	},
	"bin": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			num, err := args[0].AsInt()
			if err != nil {
				return errors.ParameterError("x", err)
			}
			if num >= 0 {
				return &object.String{Value: fmt.Sprintf("0b%b", num)}
			}
			return &object.String{Value: fmt.Sprintf("-0b%b", -num)}
		},
		HelpText: `bin(x) - Convert an integer to a binary string prefixed with "0b"`,
	},
	"oct": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			num, err := args[0].AsInt()
			if err != nil {
				return errors.ParameterError("x", err)
			}
			if num >= 0 {
				return &object.String{Value: fmt.Sprintf("0o%o", num)}
			}
			return &object.String{Value: fmt.Sprintf("-0o%o", -num)}
		},
		HelpText: `oct(x) - Convert an integer to an octal string prefixed with "0o"`,
	},
	"pow": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) < 2 || len(args) > 3 {
				return errors.NewError("pow() takes 2 or 3 arguments (%d given)", len(args))
			}
			var base, exp float64
			switch b := args[0].(type) {
			case *object.Integer:
				base = float64(b.Value)
			case *object.Float:
				base = b.Value
			default:
				return errors.NewTypeError("INTEGER or FLOAT", args[0].Type().String())
			}
			switch e := args[1].(type) {
			case *object.Integer:
				exp = float64(e.Value)
			case *object.Float:
				exp = e.Value
			default:
				return errors.NewTypeError("INTEGER or FLOAT", args[1].Type().String())
			}
			result := math.Pow(base, exp)
			if len(args) == 3 {
				// pow(base, exp, mod) - modular exponentiation
				var mod float64
				switch m := args[2].(type) {
				case *object.Integer:
					mod = float64(m.Value)
				case *object.Float:
					mod = m.Value
				default:
					return errors.NewTypeError("INTEGER or FLOAT", args[2].Type().String())
				}
				if mod == 0 {
					return errors.NewError("pow() 3rd argument cannot be 0")
				}
				result = math.Mod(result, mod)
			}
			// Return integer if result is whole number
			if result == math.Trunc(result) && result >= math.MinInt64 && result <= math.MaxInt64 {
				return object.NewInteger(int64(result))
			}
			return &object.Float{Value: result}
		},
		HelpText: `pow(base, exp[, mod]) - Return base to the power exp; optionally modulo mod

Equivalent to base**exp or base**exp % mod if mod is given.`,
	},
	"divmod": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}
			var a, b float64
			var bothInts bool = true
			switch n := args[0].(type) {
			case *object.Integer:
				a = float64(n.Value)
			case *object.Float:
				a = n.Value
				bothInts = false
			default:
				return errors.NewTypeError("INTEGER or FLOAT", args[0].Type().String())
			}
			switch n := args[1].(type) {
			case *object.Integer:
				b = float64(n.Value)
			case *object.Float:
				b = n.Value
				bothInts = false
			default:
				return errors.NewTypeError("INTEGER or FLOAT", args[1].Type().String())
			}
			if b == 0 {
				return errors.NewError("integer division or modulo by zero")
			}
			quotient := math.Floor(a / b)
			remainder := a - quotient*b
			if bothInts {
				return &object.Tuple{Elements: []object.Object{
					object.NewInteger(int64(quotient)),
					object.NewInteger(int64(remainder)),
				}}
			}
			return &object.Tuple{Elements: []object.Object{
				&object.Float{Value: quotient},
				&object.Float{Value: remainder},
			}}
		},
		HelpText: `divmod(a, b) - Return the tuple (a // b, a % b)

Equivalent to (a // b, a % b) for integers.`,
	},
	"callable": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			switch args[0].(type) {
			case *object.Function, *object.Builtin, *object.LambdaFunction:
				return TRUE
			default:
				return FALSE
			}
		},
		HelpText: `callable(object) - Return True if the object appears callable, False otherwise`,
	},
	"isinstance": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}

			// Determine the type name from the second argument
			var typeName string

			// Check for class type first (before AsString, since Class.AsString returns name)
			if class, ok := args[1].(*object.Class); ok {
				// Class type: isinstance(x, MyClass)
				if inst, ok := args[0].(*object.Instance); ok {
					// Check direct class match and inheritance chain
					for c := inst.Class; c != nil; c = c.BaseClass {
						if c == class {
							return TRUE
						}
					}
					return FALSE
				}
				return FALSE
			} else if s, err := args[1].AsString(); err == nil {
				// String type name: isinstance(x, "dict")
				typeName = s
			} else if b, ok := args[1].(*object.Builtin); ok {
				// Bare type builtin: isinstance(x, dict) where dict is a builtin function
				if name, found := typeBuiltins[b]; found {
					typeName = name
				} else {
					return errors.NewError("isinstance() arg 2 must be a type or string")
				}
			} else {
				return errors.NewError("isinstance() arg 2 must be a type or string")
			}

			objType := args[0].Type().String()
			// Support common Python type names
			checkType := strings.ToUpper(typeName)
			switch checkType {
			case "INT", "INTEGER":
				checkType = "INTEGER"
			case "STR", "STRING":
				checkType = "STRING"
			case "FLOAT":
				checkType = "FLOAT"
			case "BOOL", "BOOLEAN":
				checkType = "BOOLEAN"
			case "LIST":
				checkType = "LIST"
			case "DICT":
				checkType = "DICT"
			case "TUPLE":
				checkType = "TUPLE"
			case "FUNCTION":
				checkType = "FUNCTION"
			case "NONE", "NULL", "NONETYPE":
				checkType = "NULL"
			}
			if objType == checkType {
				return TRUE
			}
			return FALSE
		},
		HelpText: `isinstance(object, type) - Return True if object is of the given type

Supports bare type names: isinstance(x, dict), isinstance(x, int)
Also supports string type names: isinstance(x, "dict"), isinstance(x, "int")
Type names: int, str, float, bool, list, dict, tuple, function, None
Also works with class types: isinstance(obj, MyClass)`,
	},
	"chr": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			num, err := args[0].AsInt()
			if err != nil {
				return errors.ParameterError("i", err)
			}
			if num < 0 || num > 0x10FFFF {
				return errors.NewError("chr() arg not in range(0x110000)")
			}
			return &object.String{Value: string(rune(num))}
		},
		HelpText: `chr(i) - Return a string of one character from Unicode code point

The argument must be in the range 0-1114111 (0x10FFFF).`,
	},
	"ord": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			str, err := args[0].AsString()
			if err != nil {
				return errors.ParameterError("c", err)
			}
			runes := []rune(str)
			if len(runes) != 1 {
				return errors.NewError("ord() expected a character, but string of length %d found", len(runes))
			}
			return object.NewInteger(int64(runes[0]))
		},
		HelpText: `ord(c) - Return Unicode code point for a one-character string

The argument must be a string of exactly one character.`,
	},
	"reversed": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			switch args[0].(type) {
			case *object.List, *object.Tuple, *object.String, *object.Iterator:
				return object.NewReversedIterator(args[0])
			default:
				return errors.NewTypeError("sequence (LIST, TUPLE, STRING, ITERATOR)", args[0].Type().String())
			}
		},
		HelpText: `reversed(seq) - Return a reversed iterator over the sequence

Works with lists, tuples, strings, and iterators.
Use list(reversed(...)) to get a list.`,
	},
	"list": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) == 0 {
				return &object.List{Elements: []object.Object{}}
			}
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			elements, ok := object.IterableToSlice(args[0])
			if !ok {
				return errors.NewTypeError("iterable", args[0].Type().String())
			}
			// Make a copy to avoid modifying the original
			result := make([]object.Object, len(elements))
			copy(result, elements)
			return &object.List{Elements: result}
		},
		HelpText: `list([iterable]) - Create a list from an iterable

With no argument, returns an empty list.
Otherwise, returns a list containing the items of the iterable.`,
	},
	"dict": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			result := &object.Dict{Pairs: make(map[string]object.DictPair)}
			// Handle kwargs
			for _, key := range kwargs.Keys() {
				val := kwargs.Get(key)
				result.Pairs[object.DictKey(&object.String{Value: key})] = object.DictPair{
					Key:   &object.String{Value: key},
					Value: val,
				}
			}
			if len(args) == 0 {
				return result
			}
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			switch iter := args[0].(type) {
			case *object.Dict:
				// Copy existing dict
				for k, v := range iter.Pairs {
					result.Pairs[k] = v
				}
			case *object.List:
				// List of [key, value] pairs
				for _, elem := range iter.Elements {
					var pair []object.Object
					switch p := elem.(type) {
					case *object.List:
						pair = p.Elements
					case *object.Tuple:
						pair = p.Elements
					default:
						return errors.NewError("dictionary update sequence element must be [key, value] pair")
					}
					if len(pair) != 2 {
						return errors.NewError("dictionary update sequence element must be [key, value] pair")
					}
					result.Pairs[object.DictKey(pair[0])] = object.DictPair{Key: pair[0], Value: pair[1]}
				}
			default:
				return errors.NewTypeError("DICT or LIST of pairs", args[0].Type().String())
			}
			return result
		},
		HelpText: `dict([mapping], **kwargs) - Create a dictionary

With no argument, returns an empty dict.
Can initialize from another dict or list of [key, value] pairs.
Keyword arguments are added to the dict.`,
	},
	"tuple": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) == 0 {
				return &object.Tuple{Elements: []object.Object{}}
			}
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			// Special case: tuple returns itself (no copy needed for immutable)
			if t, ok := args[0].(*object.Tuple); ok {
				return t
			}
			elements, ok := object.IterableToSlice(args[0])
			if !ok {
				return errors.NewTypeError("iterable", args[0].Type().String())
			}
			// Make a copy for the tuple
			result := make([]object.Object, len(elements))
			copy(result, elements)
			return &object.Tuple{Elements: result}
		},
		HelpText: `tuple([iterable]) - Create a tuple from an iterable

With no argument, returns an empty tuple.
Otherwise, returns a tuple containing the items of the iterable.`,
	},
	"set": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) == 0 {
				return object.NewSet()
			}
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}

			// Special case: set returns a copy
			if s, ok := args[0].(*object.Set); ok {
				return s.Copy()
			}

			// Get elements from iterable
			elements, ok := object.IterableToSlice(args[0])
			if !ok {
				return errors.NewTypeError("iterable", args[0].Type().String())
			}

			return object.NewSetFromElements(elements)
		},
		HelpText: `set([iterable]) - Create a set from an iterable

With no argument, returns an empty set.
Otherwise, returns a set containing unique items from the iterable.`,
	},
	"repr": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			switch obj := args[0].(type) {
			case *object.String:
				// Add quotes around strings
				return &object.String{Value: fmt.Sprintf("'%s'", obj.Value)}
			case *object.Instance:
				// Call __repr__ first, then __str__, then fallback
				env := GetEnvFromContext(ctx)
				if result := callDunderMethodFn(ctx, obj, "__repr__", nil, env); result != nil {
					return result
				}
				if result := callDunderMethodFn(ctx, obj, "__str__", nil, env); result != nil {
					return result
				}
				return &object.String{Value: obj.Inspect()}
			default:
				return &object.String{Value: obj.Inspect()}
			}
		},
		HelpText: `repr(object) - Return a string representation

For strings, returns the string with quotes.
For other objects, returns the same as str().`,
	},
	"hash": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			// FNV-1a hash algorithm - fast and good distribution
			str := args[0].Inspect()
			const (
				offset64 = 14695981039346656037
				prime64  = 1099511628211
			)
			h := uint64(offset64)
			for i := 0; i < len(str); i++ {
				h ^= uint64(str[i])
				h *= prime64
			}
			return object.NewInteger(int64(h))
		},
		HelpText: `hash(object) - Return the hash value of an object

Returns an integer hash value for the object using FNV-1a algorithm.`,
	},
	"id": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			// Use hash of inspect value as id (stable for same object)
			str := fmt.Sprintf("%p", args[0])
			var h int64 = 0
			for _, c := range str {
				h = h*31 + int64(c)
			}
			return object.NewInteger(h)
		},
		HelpText: `id(object) - Return the identity of an object

Returns a unique integer identifier for the object.`,
	},
	"format": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) < 1 || len(args) > 2 {
				return errors.NewError("format() takes 1 or 2 arguments (%d given)", len(args))
			}
			value := args[0]
			formatSpec := ""
			if len(args) == 2 {
				if spec, err := args[1].AsString(); err == nil {
					formatSpec = spec
				} else {
					return err
				}
			}
			// Handle format specifiers
			if formatSpec == "" {
				return &object.String{Value: value.Inspect()}
			}
			// Parse format spec and apply formatting inline to avoid initialization cycle
			switch v := value.(type) {
			case *object.Integer:
				// Format integer
				if len(formatSpec) > 0 {
					switch formatSpec[len(formatSpec)-1] {
					case 'd':
						return &object.String{Value: fmt.Sprintf("%d", v.Value)}
					case 'x':
						return &object.String{Value: fmt.Sprintf("%x", v.Value)}
					case 'X':
						return &object.String{Value: fmt.Sprintf("%X", v.Value)}
					case 'o':
						return &object.String{Value: fmt.Sprintf("%o", v.Value)}
					case 'b':
						return &object.String{Value: fmt.Sprintf("%b", v.Value)}
					}
				}
				var width int
				fmt.Sscanf(formatSpec, "%d", &width)
				if width > 0 {
					return &object.String{Value: fmt.Sprintf("%*d", width, v.Value)}
				}
				return &object.String{Value: fmt.Sprintf("%d", v.Value)}
			case *object.Float:
				// Format float
				if len(formatSpec) > 0 {
					switch formatSpec[len(formatSpec)-1] {
					case 'f', 'F':
						if idx := strings.Index(formatSpec, "."); idx >= 0 {
							var prec int
							fmt.Sscanf(formatSpec[idx+1:len(formatSpec)-1], "%d", &prec)
							return &object.String{Value: fmt.Sprintf("%.*f", prec, v.Value)}
						}
						return &object.String{Value: fmt.Sprintf("%f", v.Value)}
					case 'e':
						return &object.String{Value: fmt.Sprintf("%e", v.Value)}
					case 'E':
						return &object.String{Value: fmt.Sprintf("%E", v.Value)}
					case '%':
						return &object.String{Value: fmt.Sprintf("%.2f%%", v.Value*100)}
					}
				}
				return &object.String{Value: fmt.Sprintf("%g", v.Value)}
			case *object.String:
				// Format string
				if formatSpec == "" {
					return &object.String{Value: v.Value}
				}
				var width int
				align := '<' // default left align for strings
				spec := formatSpec
				if len(spec) > 0 && (spec[0] == '<' || spec[0] == '>' || spec[0] == '^') {
					align = rune(spec[0])
					spec = spec[1:]
				}
				fmt.Sscanf(spec, "%d", &width)
				if width <= len(v.Value) {
					return &object.String{Value: v.Value}
				}
				padding := width - len(v.Value)
				switch align {
				case '>':
					return &object.String{Value: strings.Repeat(" ", padding) + v.Value}
				case '^':
					left := padding / 2
					right := padding - left
					return &object.String{Value: strings.Repeat(" ", left) + v.Value + strings.Repeat(" ", right)}
				default: // '<'
					return &object.String{Value: v.Value + strings.Repeat(" ", padding)}
				}
			default:
				return &object.String{Value: value.Inspect()}
			}
		},
		HelpText: `format(value[, format_spec]) - Format a value

Format a value according to the format specifier.
Supports width, alignment, and type specifiers.`,
	},
	"hasattr": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}
			name, err := args[1].AsString()
			if err != nil {
				return err
			}
			// Check if object has the attribute/method
			switch obj := args[0].(type) {
			case *object.Instance:
				if _, ok := obj.Fields[name]; ok {
					return TRUE
				}
				if _, ok := obj.Class.Methods[name]; ok {
					return TRUE
				}
				return FALSE
			case *object.Dict:
				_, exists := obj.Pairs[object.DictKey(&object.String{Value: name})]
				return nativeBoolToBooleanObject(exists)
			default:
				return FALSE
			}
		},
		HelpText: `hasattr(object, name) - Check if object has an attribute

Returns True if the object has the named attribute.`,
	},
	"getattr": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) < 2 || len(args) > 3 {
				return errors.NewError("getattr() takes 2 or 3 arguments (%d given)", len(args))
			}
			name, err := args[1].AsString()
			if err != nil {
				return err
			}
			// Get attribute from object
			switch obj := args[0].(type) {
			case *object.Instance:
				if val, ok := obj.Fields[name]; ok {
					return val
				}
				if method, ok := obj.Class.Methods[name]; ok {
					return method
				}
			case *object.Dict:
				if pair, exists := obj.Pairs[object.DictKey(&object.String{Value: name})]; exists {
					return pair.Value
				}
			}
			// Return default if provided
			if len(args) == 3 {
				return args[2]
			}
			return errors.NewError("'%s' object has no attribute '%s'", args[0].Type().String(), name)
		},
		HelpText: `getattr(object, name[, default]) - Get an attribute from an object

Returns the value of the named attribute.
If default is provided, returns it when attribute doesn't exist.`,
	},
	"setattr": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 3); err != nil {
				return err
			}
			name, err := args[1].AsString()
			if err != nil {
				return err
			}
			// Set attribute on object
			switch obj := args[0].(type) {
			case *object.Instance:
				obj.Fields[name] = args[2]
				return NULL
			case *object.Dict:
				obj.Pairs[object.DictKey(&object.String{Value: name})] = object.DictPair{
					Key:   &object.String{Value: name},
					Value: args[2],
				}
				return NULL
			default:
				return errors.NewError("'%s' object does not support attribute assignment", args[0].Type().String())
			}
		},
		HelpText: `setattr(object, name, value) - Set an attribute on an object

Sets the named attribute to the given value.
Only works on dict-like objects.`,
	},
	"delattr": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}
			name, err := args[1].AsString()
			if err != nil {
				return err
			}
			// Delete attribute from object
			switch obj := args[0].(type) {
			case *object.Instance:
				if _, ok := obj.Fields[name]; ok {
					delete(obj.Fields, name)
					return NULL
				}
				return errors.NewError("'%s' object has no attribute '%s'", obj.Class.Name, name)
			case *object.Dict:
				dictKey := object.DictKey(&object.String{Value: name})
				if _, ok := obj.Pairs[dictKey]; ok {
					delete(obj.Pairs, dictKey)
					return NULL
				}
				return errors.NewError("dictionary has no key '%s'", name)
			default:
				return errors.NewError("'%s' object does not support attribute deletion", args[0].Type().String())
			}
		},
		HelpText: `delattr(object, name) - Delete named attribute

Deletes the named attribute from the given object.`,
	},
	"slice": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.MaxArgs(args, 3); err != nil {
				return err
			}

			// Default values: slice(stop) where start=0, step=1
			// slice(start, stop) where step=1
			// slice(start, stop, step)

			var start, end, step *object.Integer

			// Handle arguments
			if len(args) == 1 {
				// slice(stop) - treat as slice(0, stop, 1)
				if args[0].Type() == object.NULL_OBJ {
					end = nil
				} else if i, ok := args[0].(*object.Integer); ok {
					end = i
				} else {
					return errors.NewTypeError("INTEGER or None", args[0].Type().String())
				}
				step = object.NewInteger(1)
			} else if len(args) == 2 {
				// slice(start, stop)
				if args[0].Type() == object.NULL_OBJ {
					start = nil
				} else if i, ok := args[0].(*object.Integer); ok {
					start = i
				} else {
					return errors.NewTypeError("INTEGER or None", args[0].Type().String())
				}

				if args[1].Type() == object.NULL_OBJ {
					end = nil
				} else if i, ok := args[1].(*object.Integer); ok {
					end = i
				} else {
					return errors.NewTypeError("INTEGER or None", args[1].Type().String())
				}
				step = object.NewInteger(1)
			} else if len(args) == 3 {
				// slice(start, stop, step)
				if args[0].Type() == object.NULL_OBJ {
					start = nil
				} else if i, ok := args[0].(*object.Integer); ok {
					start = i
				} else {
					return errors.NewTypeError("INTEGER or None", args[0].Type().String())
				}

				if args[1].Type() == object.NULL_OBJ {
					end = nil
				} else if i, ok := args[1].(*object.Integer); ok {
					end = i
				} else {
					return errors.NewTypeError("INTEGER or None", args[1].Type().String())
				}

				if args[2].Type() == object.NULL_OBJ {
					step = nil
				} else if i, ok := args[2].(*object.Integer); ok {
					step = i
				} else {
					return errors.NewTypeError("INTEGER or None", args[2].Type().String())
				}

				// Check for zero step
				if step != nil && step.Value == 0 {
					return errors.NewError("slice step cannot be zero")
				}
			}

			return &object.Slice{
				Start: start,
				End:   end,
				Step:  step,
			}
		},
		HelpText: `slice([start,] stop[, step]) - Create a slice object

Used for extended slicing. Returns a slice object that can be used
with sequence objects to select a range of elements.

Examples:
  seq[1:3]      # equivalent to seq[slice(1, 3)]
  seq[::2]      # equivalent to seq[slice(None, None, 2)]
  seq[::-1]     # equivalent to seq[slice(None, None, -1)]

Parameters:
  start - start index (default: 0)
  stop - end index (default: end of sequence)
  step - step value (default: 1)

Use None for any parameter to use its default value.`,
	},
	"Exception": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			message := ""
			if len(args) > 0 {
				if str, err := args[0].AsString(); err == nil {
					message = str
				} else {
					message = args[0].Inspect()
				}
			}
			return &object.Exception{
				Message:       message,
				ExceptionType: object.ExceptionTypeException,
			}
		},
		HelpText: `Exception([message]) - Create a generic exception

Creates an exception object that can be raised with the raise statement.
Use with: raise Exception("error message")`,
	},
	"ValueError": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			message := ""
			if len(args) > 0 {
				if str, err := args[0].AsString(); err == nil {
					message = str
				} else {
					message = args[0].Inspect()
				}
			}
			return &object.Exception{
				Message:       message,
				ExceptionType: object.ExceptionTypeValueError,
			}
		},
		HelpText: `ValueError([message]) - Create a value error exception

Raised when an operation receives an argument with an inappropriate value.
Use with: raise ValueError("invalid value")`,
	},
	"TypeError": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			message := ""
			if len(args) > 0 {
				if str, err := args[0].AsString(); err == nil {
					message = str
				} else {
					message = args[0].Inspect()
				}
			}
			return &object.Exception{
				Message:       message,
				ExceptionType: object.ExceptionTypeTypeError,
			}
		},
		HelpText: `TypeError([message]) - Create a type error exception

Raised when an operation is applied to an object of inappropriate type.
Use with: raise TypeError("wrong type")`,
	},
	"NameError": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			message := ""
			if len(args) > 0 {
				if str, err := args[0].AsString(); err == nil {
					message = str
				} else {
					message = args[0].Inspect()
				}
			}
			return &object.Exception{
				Message:       message,
				ExceptionType: object.ExceptionTypeNameError,
			}
		},
		HelpText: `NameError([message]) - Create a name error exception

Raised when a local or global name is not found.
Use with: raise NameError("name not defined")`,
	},
	"StopIteration": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			message := ""
			if len(args) > 0 {
				if str, err := args[0].AsString(); err == nil {
					message = str
				} else {
					message = args[0].Inspect()
				}
			}
			return &object.Exception{
				Message:       message,
				ExceptionType: object.ExceptionTypeStopIteration,
			}
		},
		HelpText: `StopIteration([message]) - Signal end of iteration

Raised by __next__ to signal that there are no more items.
Use with: raise StopIteration()`,
	},
}

func compareObjects(a, b object.Object) int {
	switch av := a.(type) {
	case *object.Integer:
		switch bv := b.(type) {
		case *object.Integer:
			if av.Value < bv.Value {
				return -1
			} else if av.Value > bv.Value {
				return 1
			}
			return 0
		case *object.Float:
			af := float64(av.Value)
			if af < bv.Value {
				return -1
			} else if af > bv.Value {
				return 1
			}
			return 0
		}
	case *object.Float:
		switch bv := b.(type) {
		case *object.Float:
			if av.Value < bv.Value {
				return -1
			} else if av.Value > bv.Value {
				return 1
			}
			return 0
		case *object.Integer:
			bf := float64(bv.Value)
			if av.Value < bf {
				return -1
			} else if av.Value > bf {
				return 1
			}
			return 0
		}
	case *object.String:
		if bv, ok := b.(*object.String); ok {
			if av.Value < bv.Value {
				return -1
			} else if av.Value > bv.Value {
				return 1
			}
			return 0
		}
	}
	// For incomparable types, return 0 (no swap)
	return 0
}

// Initialize the complex builtin functions
// These are defined as variable assignments to allow forward declaration in the builtins map
func init() {
	mapFunction = mapFunctionImpl
	filterFunction = filterFunctionImpl
	sortedFunction = sortedFunctionImpl
	helpFunction = helpFunctionImpl
	callDunderMethodFn = callDunderMethod

	// Build reverse lookup for isinstance() to support bare type names
	typeBuiltins = map[*object.Builtin]string{
		builtins["int"]:   "int",
		builtins["str"]:   "str",
		builtins["float"]: "float",
		builtins["bool"]:  "bool",
		builtins["list"]:  "list",
		builtins["dict"]:  "dict",
		builtins["tuple"]: "tuple",
	}
}

func sortedFunctionImpl(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	if err := errors.RangeArgs(args, 1, 2); err != nil {
		return err
	}

	var elements []object.Object
	switch arg := args[0].(type) {
	case *object.List:
		elements = make([]object.Object, len(arg.Elements))
		copy(elements, arg.Elements)
	case *object.Tuple:
		elements = make([]object.Object, len(arg.Elements))
		copy(elements, arg.Elements)
	default:
		return errors.NewTypeError("LIST or TUPLE", args[0].Type().String())
	}

	// Check for key function - support builtin, function, and lambda
	var keyFunc object.Object
	if len(args) == 2 {
		switch args[1].(type) {
		case *object.Builtin, *object.Function, *object.LambdaFunction:
			keyFunc = args[1]
		default:
			return errors.NewError("sorted() key parameter must be a function")
		}
	} else if kwargs.Len() > 0 {
		if keyArg := kwargs.Get("key"); keyArg != nil {
			switch keyArg.(type) {
			case *object.Builtin, *object.Function, *object.LambdaFunction:
				keyFunc = keyArg
			default:
				return errors.NewError("sorted() key parameter must be a function")
			}
		}
	}

	// Check for reverse kwarg
	reverse := false
	if kwargs.Len() > 0 {
		if rev := kwargs.Get("reverse"); rev != nil {
			if b, err := rev.AsBool(); err == nil {
				reverse = b
			}
		}
	}

	n := len(elements)
	if n > 1 {
		// Pre-compute keys if key function is provided
		var keys []object.Object
		var sortErr object.Object
		if keyFunc != nil {
			keys = make([]object.Object, n)
			env := GetEnvFromContext(ctx)
			for i, elem := range elements {
				var key object.Object
				switch fn := keyFunc.(type) {
				case *object.Builtin:
					key = fn.Fn(ctx, object.NewKwargs(nil), elem)
				case *object.Function, *object.LambdaFunction:
					key = applyFunctionWithContext(ctx, fn, []object.Object{elem}, nil, env)
				}
				if object.IsError(key) || isException(key) {
					return key
				}
				keys[i] = key
			}
		}

		// Create index array
		indices := make([]int, n)
		for i := range indices {
			indices[i] = i
		}

		// Sort indices
		sort.Slice(indices, func(i, j int) bool {
			var left, right object.Object
			if keys != nil {
				left, right = keys[indices[i]], keys[indices[j]]
			} else {
				left, right = elements[indices[i]], elements[indices[j]]
			}

			var cmp int
			switch l := left.(type) {
			case *object.Integer:
				if r, ok := right.(*object.Integer); ok {
					if l.Value < r.Value {
						cmp = -1
					} else if l.Value > r.Value {
						cmp = 1
					}
				} else if r, ok := right.(*object.Float); ok {
					lf := float64(l.Value)
					if lf < r.Value {
						cmp = -1
					} else if lf > r.Value {
						cmp = 1
					}
				} else {
					sortErr = errors.NewError("cannot compare %s with %s", left.Type(), right.Type())
				}
			case *object.Float:
				if r, ok := right.(*object.Float); ok {
					if l.Value < r.Value {
						cmp = -1
					} else if l.Value > r.Value {
						cmp = 1
					}
				} else if r, ok := right.(*object.Integer); ok {
					rf := float64(r.Value)
					if l.Value < rf {
						cmp = -1
					} else if l.Value > rf {
						cmp = 1
					}
				} else {
					sortErr = errors.NewError("cannot compare %s with %s", left.Type(), right.Type())
				}
			case *object.String:
				if r, ok := right.(*object.String); ok {
					if l.Value < r.Value {
						cmp = -1
					} else if l.Value > r.Value {
						cmp = 1
					}
				} else {
					sortErr = errors.NewError("cannot compare %s with %s", left.Type(), right.Type())
				}
			case *object.Instance:
				// Use __lt__ dunder method for instance comparison
				if result := callDunderMethodFn(ctx, l, "__lt__", []object.Object{right}, GetEnvFromContext(ctx)); result != nil {
					if object.IsError(result) {
						sortErr = result
					} else if b, ok := result.(*object.Boolean); ok && b.Value {
						cmp = -1
					} else {
						// Check __eq__ for cmp == 0
						if eqResult := callDunderMethodFn(ctx, l, "__eq__", []object.Object{right}, GetEnvFromContext(ctx)); eqResult != nil {
							if b2, ok := eqResult.(*object.Boolean); ok && b2.Value {
								cmp = 0
							} else {
								cmp = 1
							}
						} else {
							cmp = 1
						}
					}
				} else {
					sortErr = errors.NewError("unsupported type for sorting: %s (no __lt__)", left.Type())
				}
			default:
				sortErr = errors.NewError("unsupported type for sorting: %s", left.Type())
			}

			if reverse {
				return cmp > 0
			}
			return cmp < 0
		})

		if sortErr != nil {
			return sortErr
		}

		// Reorder elements
		newElements := make([]object.Object, n)
		for i, idx := range indices {
			newElements[i] = elements[idx]
		}
		elements = newElements
	}

	return &object.List{Elements: elements}
}

func mapFunctionImpl(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	if len(args) < 2 {
		return errors.NewError("map() requires at least 2 arguments")
	}
	fn := args[0]
	// Get all iterables as slices
	iterables := make([][]object.Object, len(args)-1)
	minLen := -1
	for i, arg := range args[1:] {
		elements, ok := object.IterableToSlice(arg)
		if !ok {
			return errors.NewTypeError("iterable (LIST, TUPLE, STRING, ITERATOR)", arg.Type().String())
		}
		iterables[i] = elements
		if minLen == -1 || len(iterables[i]) < minLen {
			minLen = len(iterables[i])
		}
	}

	// Eagerly evaluate all results
	results := make([]object.Object, minLen)
	env := GetEnvFromContext(ctx)
	for i := 0; i < minLen; i++ {
		callArgs := make([]object.Object, len(iterables))
		for j := range iterables {
			callArgs[j] = iterables[j][i]
		}
		res := applyFunctionWithContext(ctx, fn, callArgs, nil, env)
		if object.IsError(res) {
			return res
		}
		results[i] = res
	}

	// Return as iterator
	index := 0
	return object.NewIterator(func() (object.Object, bool) {
		if index >= len(results) {
			return nil, false
		}
		val := results[index]
		index++
		return val, true
	})
}

func filterFunctionImpl(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	if err := errors.ExactArgs(args, 2); err != nil {
		return err
	}
	fn := args[0]
	iterable, ok := object.IterableToSlice(args[1])
	if !ok {
		return errors.NewTypeError("iterable (LIST, TUPLE, STRING, ITERATOR)", args[1].Type().String())
	}

	// Eagerly evaluate and filter
	results := []object.Object{}
	env := GetEnvFromContext(ctx)
	for _, elem := range iterable {
		// If function is None, use truthiness
		if fn.Type() == object.NULL_OBJ {
			if isTruthy(elem) {
				results = append(results, elem)
			}
		} else {
			res := applyFunctionWithContext(ctx, fn, []object.Object{elem}, nil, env)
			if object.IsError(res) {
				return res
			}
			if isTruthy(res) {
				results = append(results, elem)
			}
		}
	}

	// Return as iterator
	index := 0
	return object.NewIterator(func() (object.Object, bool) {
		if index >= len(results) {
			return nil, false
		}
		val := results[index]
		index++
		return val, true
	})
}

func helpFunctionImpl(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	env := GetEnvFromContext(ctx)
	writer := env.GetWriter()

	// No arguments - show general help
	if len(args) == 0 {
		fmt.Fprintln(writer, "Scriptling Help System")
		fmt.Fprintln(writer, "")
		fmt.Fprintln(writer, "Usage:")
		fmt.Fprintln(writer, "  help()                    - Show this help message")
		fmt.Fprintln(writer, "  help(\"modules\")           - List all available libraries")
		fmt.Fprintln(writer, "  help(\"builtins\")          - List all builtin functions")
		fmt.Fprintln(writer, "  help(\"operators\")         - List all operators")
		fmt.Fprintln(writer, "  help(function)            - Show help for a function")
		fmt.Fprintln(writer, "  help(\"function_name\")     - Show help for a builtin function")
		fmt.Fprintln(writer, "  help(\"library.function\")  - Show help for a library function")
		fmt.Fprintln(writer, "")
		fmt.Fprintln(writer, "For a list of builtin functions, use: help(\"builtins\")")
		return NULL
	}

	if err := errors.ExactArgs(args, 1); err != nil {
		return err
	}

	// Handle string arguments
	if strObj, ok := args[0].(*object.String); ok {
		topic := strObj.Value

		// Special topic: modules
		if topic == "modules" {
			fmt.Fprintln(writer, "Available Libraries (use 'import <name>'):")
			fmt.Fprintln(writer, "")

			// Use callback if available to get all libraries
			availableLibrariesCallback := env.GetAvailableLibrariesCallback()
			if availableLibrariesCallback != nil {
				libs := availableLibrariesCallback()

				var allLibs []string
				for _, lib := range libs {
					allLibs = append(allLibs, lib.Name)
				}
				sort.Strings(allLibs)

				for _, name := range allLibs {
					fmt.Fprintf(writer, "  - %s\n", name)
				}
				fmt.Fprintln(writer, "")
			} else {
				// Fallback to checking environment if callback not set
				// Get all library names from environment
				globalEnv := env.GetGlobal()
				store := globalEnv.GetStore()

				var allLibs []string

				for name, obj := range store {
					if dict, ok := obj.(*object.Dict); ok {
						// Check if it's a library (has functions)
						hasBuiltins := false
						for _, pair := range dict.Pairs {
							if _, ok := pair.Value.(*object.Builtin); ok {
								hasBuiltins = true
								break
							}
						}

						if hasBuiltins {
							allLibs = append(allLibs, name)
						}
					}
				}

				sort.Strings(allLibs)

				if len(allLibs) > 0 {
					fmt.Fprintln(writer, "Libraries (use 'import <name>'):")
					for _, name := range allLibs {
						fmt.Fprintf(writer, "  - %s\n", name)
					}
					fmt.Fprintln(writer, "")
				}
			}

			fmt.Fprintln(writer, "To see functions in a library, first import it, then use: help(\"library_name\")")
			return NULL
		}

		// Special topic: builtins
		if topic == "builtins" {
			fmt.Fprintln(writer, "Builtin Functions:")
			fmt.Fprintln(writer, "")

			// Collect and sort builtin names
			var names []string
			for name := range builtins {
				names = append(names, name)
			}
			sort.Strings(names)

			for _, name := range names {
				fmt.Fprintf(writer, "  - %s\n", name)
			}
			fmt.Fprintln(writer, "")
			fmt.Fprintln(writer, "Use help(\"function_name\") for details on a specific function")
			return NULL
		}

		// Special topic: operators
		if topic == "operators" {
			fmt.Fprintln(writer, "Operators:")
			fmt.Fprintln(writer, "")
			fmt.Fprintln(writer, "Arithmetic Operators:")
			fmt.Fprintln(writer, "  +   - Addition")
			fmt.Fprintln(writer, "  -   - Subtraction")
			fmt.Fprintln(writer, "  *   - Multiplication (also string repetition)")
			fmt.Fprintln(writer, "  /   - Division (true division, always float)")
			fmt.Fprintln(writer, "  **  - Exponentiation")
			fmt.Fprintln(writer, "")
			fmt.Fprintln(writer, "Comparison Operators:")
			fmt.Fprintln(writer, "  ==  - Equal")
			fmt.Fprintln(writer, "  !=  - Not equal")
			fmt.Fprintln(writer, "  <   - Less than")
			fmt.Fprintln(writer, "  <=  - Less than or equal")
			fmt.Fprintln(writer, "  >   - Greater than")
			fmt.Fprintln(writer, "  >=  - Greater than or equal")
			fmt.Fprintln(writer, "")
			fmt.Fprintln(writer, "Logical Operators:")
			fmt.Fprintln(writer, "  and - Logical and (short-circuit)")
			fmt.Fprintln(writer, "  or  - Logical or (short-circuit)")
			fmt.Fprintln(writer, "")
			fmt.Fprintln(writer, "Membership Operators:")
			fmt.Fprintln(writer, "  in      - Check membership")
			fmt.Fprintln(writer, "  not in  - Check non-membership")
			fmt.Fprintln(writer, "")
			fmt.Fprintln(writer, "String Repetition:")
			fmt.Fprintln(writer, "  string * int - Repeat string int times")
			fmt.Fprintln(writer, "  int * string - Repeat string int times")
			fmt.Fprintln(writer, "  Example: \"hello\" * 3 = \"hellohellohello\"")
			return NULL
		}

		// Check if it's a library.function format
		if strings.Contains(topic, ".") {
			// Split on last dot to handle dotted library names like "knot.ai.completion"
			lastDot := strings.LastIndex(topic, ".")
			libName := topic[:lastDot]
			funcName := topic[lastDot+1:]

			// Try to get the library from environment
			if libObj, ok := env.Get(libName); ok {
				if dict, ok := libObj.(*object.Dict); ok {
					if pair, ok := dict.Pairs[object.DictKey(&object.String{Value: funcName})]; ok {
						if builtin, ok := pair.Value.(*object.Builtin); ok {
							fmt.Fprintf(writer, "Help for %s.%s:\n", libName, funcName)
							if builtin.HelpText != "" {
								fmt.Fprintln(writer, builtin.HelpText)
							} else {
								fmt.Fprintln(writer, "  No documentation available")
							}
							return NULL
						}
						// Handle Scriptling functions in libraries (with docstrings)
						if fn, ok := pair.Value.(*object.Function); ok {
							fmt.Fprintf(writer, "Help for %s.%s:\n", libName, funcName)
							printFunctionHelp(writer, fmt.Sprintf("%s.%s", libName, funcName), fn)
							return NULL
						}
					}
					fmt.Fprintf(writer, "Function '%s' not found in library '%s'\n", funcName, libName)
					return NULL
				}
			}
			fmt.Fprintf(writer, "Library '%s' not found. Did you import it?\n", libName)
			return NULL
		}

		// Check if it's a library name
		if libObj, ok := env.Get(topic); ok {
			if dict, ok := libObj.(*object.Dict); ok {
				fmt.Fprintf(writer, "%s functions:\n", topic)

				// Check for module docstring
				if docPair, ok := dict.Pairs[object.DictKey(&object.String{Value: "__doc__"})]; ok {
					if docStr, ok := docPair.Value.(*object.String); ok {
						fmt.Fprintln(writer, "")
						fmt.Fprintln(writer, "Description:")
						// Indent the docstring
						lines := strings.Split(docStr.Value, "\n")
						for _, line := range lines {
							fmt.Fprintf(writer, "  %s\n", line)
						}
						fmt.Fprintln(writer, "")
					}
				}

				fmt.Fprintln(writer, "Available functions:")
				// Collect and sort function names
				var names []string
				docKey := object.DictKey(&object.String{Value: "__doc__"})
				for k, pair := range dict.Pairs {
					if k != docKey {
						keyStr, _ := pair.Key.AsString()
						names = append(names, keyStr)
					}
				}
				sort.Strings(names)
				for _, name := range names {
					fmt.Fprintf(writer, "  - %s\n", name)
				}
				fmt.Fprintln(writer, "")
				fmt.Fprintf(writer, "Use help(\"%s.function_name\") for details on a specific function\n", topic)
				return NULL
			}
		} // Check if it's a builtin function name (from builtins map)
		if builtin, ok := builtins[topic]; ok {
			fmt.Fprintf(writer, "Help for builtin function '%s':\n", topic)
			if builtin.HelpText != "" {
				fmt.Fprintln(writer, builtin.HelpText)
			} else {
				fmt.Fprintln(writer, "  No documentation available")
			}
			return NULL
		}

		// Check if it's a variable/function in environment
		if obj, ok := env.Get(topic); ok {
			switch fn := obj.(type) {
			case *object.Function:
				fmt.Fprintf(writer, "Help for function '%s':\n", topic)
				printFunctionHelp(writer, topic, fn)
				return NULL
			case *object.LambdaFunction:
				fmt.Fprintf(writer, "Help for lambda function '%s':\n", topic)
				fmt.Fprintf(writer, "  Lambda function with %d parameter(s)\n", len(fn.Parameters))
				return NULL
			case *object.Builtin:
				fmt.Fprintf(writer, "Help for builtin '%s':\n", topic)
				if fn.HelpText != "" {
					fmt.Fprintln(writer, fn.HelpText)
				} else {
					fmt.Fprintln(writer, "  No documentation available")
				}
				return NULL
			}
		}

		fmt.Fprintf(writer, "No help available for '%s'\n", topic)
		return NULL
	}

	// Handle object arguments (e.g., help(print))
	switch obj := args[0].(type) {
	case *object.Function:
		name := obj.Name
		if name == "" {
			name = "<anonymous>"
		}
		fmt.Fprintf(writer, "Help for function '%s':\n", name)
		printFunctionHelp(writer, name, obj)
		return NULL
	case *object.LambdaFunction:
		fmt.Fprintln(writer, "Help for lambda function:")
		fmt.Fprintf(writer, "  Lambda function with %d parameter(s)\n", len(obj.Parameters))
		return NULL
	case *object.Builtin:
		fmt.Fprintln(writer, "Help for builtin function:")
		if obj.HelpText != "" {
			fmt.Fprintln(writer, obj.HelpText)
		} else {
			fmt.Fprintln(writer, "  No documentation available")
		}
		return NULL
	case *object.Dict:
		// Could be a library
		fmt.Fprintln(writer, "Help for dictionary/library:")
		fmt.Fprintln(writer, "")
		fmt.Fprintln(writer, "Available keys:")
		// Collect and sort keys
		var names []string
		for _, pair := range obj.Pairs {
			keyStr, _ := pair.Key.AsString()
			names = append(names, keyStr)
		}
		sort.Strings(names)
		for _, name := range names {
			fmt.Fprintf(writer, "  - %s\n", name)
		}
		return NULL
	case *object.Class:
		fmt.Fprintf(writer, "Help for class '%s':\n", obj.Name)
		fmt.Fprintln(writer, "")
		fmt.Fprintln(writer, "Available methods:")
		for name := range obj.Methods {
			fmt.Fprintf(writer, "  - %s\n", name)
		}

		// Show typical fields for known classes
		if obj.Name == "Response" {
			fmt.Fprintln(writer, "")
			fmt.Fprintln(writer, "Instance fields:")
			fmt.Fprintln(writer, "  - status_code (integer) - HTTP status code")
			fmt.Fprintln(writer, "  - text (string) - Response body as string")
			fmt.Fprintln(writer, "  - headers (dict) - HTTP headers")
			fmt.Fprintln(writer, "  - body (string) - Raw response body")
			fmt.Fprintln(writer, "  - url (string) - Request URL")
		}
		return NULL
	case *object.Instance:
		fmt.Fprintf(writer, "Help for %s instance:\n", obj.Class.Name)
		fmt.Fprintln(writer, "")
		fmt.Fprintln(writer, "Available methods:")
		for name := range obj.Class.Methods {
			fmt.Fprintf(writer, "  - %s\n", name)
		}
		fmt.Fprintln(writer, "")
		fmt.Fprintln(writer, "Available fields:")
		for name := range obj.Fields {
			fmt.Fprintf(writer, "  - %s\n", name)
		}
		return NULL
	default:
		fmt.Fprintf(writer, "Help for %s:\n", obj.Type())
		fmt.Fprintf(writer, "  Type: %s\n", obj.Type())
		fmt.Fprintf(writer, "  Value: %s\n", obj.Inspect())
		return NULL
	}
}

// Helper to extract and print docstrings from functions
func printFunctionHelp(writer io.Writer, name string, fn *object.Function) {
	// Build function signature
	fmt.Fprintf(writer, "%s(", name)
	for i, param := range fn.Parameters {
		if i > 0 {
			fmt.Fprint(writer, ", ")
		}
		fmt.Fprint(writer, param.Value)
		if fn.DefaultValues != nil {
			if _, hasDefault := fn.DefaultValues[param.Value]; hasDefault {
				fmt.Fprint(writer, "=...")
			}
		}
	}
	if fn.Variadic != nil {
		if len(fn.Parameters) > 0 {
			fmt.Fprint(writer, ", ")
		}
		fmt.Fprintf(writer, "*%s", fn.Variadic.Value)
	}
	fmt.Fprint(writer, ")")

	// Check for docstring (first statement is a string literal)
	if fn.Body != nil && len(fn.Body.Statements) > 0 {
		if exprStmt, ok := fn.Body.Statements[0].(*ast.ExpressionStatement); ok {
			if strLit, ok := exprStmt.Expression.(*ast.StringLiteral); ok {
				// Format like builtin help: signature - first line of docstring
				lines := strings.Split(strLit.Value, "\n")
				if len(lines) > 0 && strings.TrimSpace(lines[0]) != "" {
					fmt.Fprintf(writer, " - %s\n", strings.TrimSpace(lines[0]))
				} else {
					fmt.Fprintln(writer, "")
				}

				// Print remaining docstring lines
				for _, line := range lines[1:] {
					if strings.TrimSpace(line) != "" {
						fmt.Fprintf(writer, "%s\n", line)
					}
				}
				return
			}
		}
	}

	// No docstring
	fmt.Fprintf(writer, " - User-defined function\n")
}

func GetImportBuiltin() *object.Builtin {
	return &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			libName, err := args[0].AsString()
			if err != nil {
				return errors.ParameterError("module_name", err)
			}

			env := GetEnvFromContext(ctx)
			importCallback := env.GetImportCallback()
			if importCallback == nil {
				return errors.NewError(errors.ErrImportError)
			}
			importErr := importCallback(libName)
			if importErr != nil {
				return errors.NewError("%s: %s", errors.ErrImportError, importErr.Error())
			}
			return &object.Null{}
		},
	}
}

