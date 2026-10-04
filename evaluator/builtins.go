package evaluator

import (
	"context"
	"fmt"
	"io"
	"math"
	goruntime "runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/paularlott/scriptling/ast"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/stdlib"
)

// Forward declarations for complex builtins
// These are defined after the builtins map for better organization
var (
	mapFunction    func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object
	filterFunction func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object
	sortedFunction func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object
	minMaxFunction func(ctx context.Context, kwargs object.Kwargs, wantMax bool, args ...object.Object) object.Object
	helpFunction   func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object
	dirFunction    func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object
	iterFunction   func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object
	nextFunction   func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object

	// callDunderMethodFn is set in init() to break the initialization cycle
	callDunderMethodFn func(ctx context.Context, inst *object.Instance, method string, args []object.Object, env *object.Environment) object.Object

	// getAttributeFn is getAttribute, set in init() to break the init cycle
	getAttributeFn func(ctx context.Context, obj object.Object, name string) object.Object

	// hashInstanceFn calls __hash__ on an instance; set in init() to break init cycle
	hashInstanceFn func(ctx context.Context, inst *object.Instance) object.Object

	// typeBuiltins maps type-related builtin pointers to their names for isinstance()
	typeBuiltins map[*object.Builtin]string

	// exceptionBuiltins maps exception constructors to their type names
	exceptionBuiltins map[*object.Builtin]string
)

var builtins = map[string]*object.Builtin{
	"bytes":    BytesBuiltin,
	"sentinel": SentinelBuiltin,
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
	"yield_now": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			// Release the interpreter lock and yield the OS thread so other
			// goroutines (shared-env threads, handlers) can run. Useful inside a
			// CPU-bound loop that never hits a naturally-blocking call. The lock
			// is released by RunBlocking; Gosched yields the OS thread for fairness.
			object.RunBlocking(ctx, func() { goruntime.Gosched() })
			return &object.Null{}
		},
		HelpText: `yield_now() - Briefly release the interpreter lock so other threads can run

Call inside a long compute loop to let shared-env threads (runtime.background
with shared=True) and registered handlers make progress. Blocking calls such as
sleep, socket reads, file I/O and Queue operations release the lock on their own.`,
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

			// Get file kwarg: StringIO instances write to their buffer; any
			// other instance with a write method (sys.stdout, sys.stderr,
			// user classes) gets its write method called with the output.
			var fileInst *object.Instance
			if fileObj := kwargs.Get("file"); fileObj != nil {
				if inst, ok := fileObj.(*object.Instance); ok {
					if w, ok := stdlib.GetStringIOWriter(inst); ok {
						writer = w
					} else if _, ok := inst.Class.LookupMember("write"); ok {
						fileInst = inst
					}
				}
			}

			// emit sends the rendered output to the selected destination.
			// Returns an error object if a file instance's write fails.
			emit := func(s string) object.Object {
				if fileInst != nil {
					if result := callDunderMethodFn(ctx, fileInst, "write", []object.Object{object.NewString(s)}, env); object.IsError(result) {
						return result
					}
					return nil
				}
				fmt.Fprint(writer, s)
				return nil
			}

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

			// stringify renders one argument the way print does. Instances are
			// checked FIRST so a user __str__ is honored (Instance.AsString
			// falls back to Inspect() without calling __str__). A raise from
			// __str__ is returned so print propagates it instead of printing the
			// default repr. On success it returns (text, nil).
			stringify := func(arg object.Object) (string, object.Object) {
				if inst, ok := arg.(*object.Instance); ok {
					// str() semantics: __str__, then __repr__, then default.
					return strInstanceChecked(ctx, inst, env)
				}
				if str, err := arg.AsString(); err == nil && !object.IsReprContainer(arg) {
					return str, nil
				}
				return renderConvertedValue(ctx, arg, "s", env)
			}

			// Build output string — fast path for common single-arg case
			if len(args) == 1 && sep == " " {
				s, raised := stringify(args[0])
				if raised != nil {
					return raised
				}
				if e := emit(s + end); e != nil {
					return e
				}
				return NULL
			}
			parts := make([]string, len(args))
			for i, arg := range args {
				s, raised := stringify(arg)
				if raised != nil {
					return raised
				}
				parts[i] = s
			}
			if e := emit(strings.Join(parts, sep) + end); e != nil {
				return e
			}
			return NULL
		},
		HelpText: `print(*args, sep=" ", end="\n", file=None) - Print values to output

Prints the given arguments separated by sep and followed by end.
Default separator is a space, default ending is a newline.

file selects the destination: an io.StringIO buffer, or a stream object
with a write method such as sys.stdout or sys.stderr.

Examples:
  print("hello", "world")     # Output: hello world
  print("a", "b", sep=",")    # Output: a,b
  print("no newline", end="") # Output: no newline (no trailing newline)
  print("warning!", file=sys.stderr)  # Write to stderr instead of stdout`,
	},
	"len": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			switch arg := args[0].(type) {
			case *object.String:
				// ASCII fast-path: avoid []rune conversion for pure-ASCII strings
				if isASCII(arg.StringValue()) {
					return object.NewInteger(int64(len(arg.StringValue())))
				}
				return object.NewInteger(int64(len([]rune(arg.StringValue()))))
			case *object.Bytes:
				return object.NewInteger(int64(arg.Len()))
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
			case *object.Iterator:
				if n, ok := arg.Len(); ok {
					return object.NewInteger(n)
				}
				return errors.NewTypeError("STRING, LIST, DICT, TUPLE, SET, BYTES, or VIEW", arg.Type().String())
			case *object.FloatArray:
				if arg.Is2D() {
					return object.NewInteger(int64(arg.Rows()))
				}
				return object.NewInteger(int64(len(arg.Data)))
			case *object.Instance:
				// Call __len__ dunder method if defined
				env := GetEnvFromContext(ctx)
				if result := callDunderMethodFn(ctx, arg, "__len__", nil, env); result != nil {
					return result
				}
				return errors.NewTypeError("object with __len__", "INSTANCE")
			default:
				return errors.NewTypeError("STRING, LIST, DICT, TUPLE, SET, BYTES, or VIEW", args[0].Type().String())
			}
		},
		HelpText: `len(obj) - Return the length of an object

Returns the number of items in a string, bytes, list, dict, or tuple.`,
	},
	"type": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			obj := args[0]
			if instance, ok := obj.(*object.Instance); ok {
				return object.NewString(instance.Class.Name)
			}
			// An exception reports the class it was raised as ("ValueError"),
			// not the generic wrapper type: except blocks need to tell them
			// apart without message sniffing.
			if exc, ok := obj.(*object.Exception); ok && exc.ExceptionType != "" {
				return object.NewString(exc.ExceptionType)
			}
			return object.NewString(obj.Type().String())
		},
		HelpText: `type(obj) - Return the type of an object

Returns a string representing the type of the object. For exceptions this is
the class it was raised as (e.g. "ValueError"), not the generic "EXCEPTION".`,
	},
	"str": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			// Exceptions give their message, instances __str__ then
			// __repr__, containers repr their elements: Python's str().
			// Scalars (the hot case, e.g. str(i) in a loop) convert directly.
			switch v := args[0].(type) {
			case *object.String:
				return v
			case *object.Integer, *object.Float, *object.Boolean, *object.Null:
				return object.NewString(v.Inspect())
			}
			rendered, rerr := renderConvertedValue(ctx, args[0], "s", GetEnvFromContext(ctx))
			if rerr != nil {
				return rerr
			}
			return object.NewString(rendered)
		},
		HelpText: `str(obj) - Convert an object to a string

Returns the string representation of any object.
For exceptions, returns just the exception message.`,
	},
	"int": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.RangeArgs(args, 1, 2); err != nil {
				return err
			}
			// base may be given positionally or as base= (Python: int(x, base=10)).
			var baseObj object.Object
			if len(args) == 2 {
				baseObj = args[1]
				if kwargs.Has("base") {
					return &object.Error{Message: "int() got multiple values for argument 'base'", ExceptionType: object.ExceptionTypeTypeError}
				}
			} else if v := kwargs.Get("base"); v != nil {
				baseObj = v
			}
			for _, k := range kwargs.Keys() {
				if k != "base" {
					return &object.Error{Message: fmt.Sprintf("'%s' is an invalid keyword argument for int()", k), ExceptionType: object.ExceptionTypeTypeError}
				}
			}
			hasBase := baseObj != nil
			base := 10
			if hasBase {
				b, ok := baseObj.(*object.Integer)
				if !ok {
					return errors.NewTypeError("INTEGER", baseObj.Type().String())
				}
				base = int(b.IntValue())
				if base != 0 && (base < 2 || base > 36) {
					return errors.NewValueError("int() base must be >= 2 and <= 36, or 0")
				}
			}
			switch arg := args[0].(type) {
			case *object.Integer:
				if hasBase {
					return &object.Error{Message: "int() can't convert non-string with explicit base", ExceptionType: object.ExceptionTypeTypeError}
				}
				return arg
			case *object.Float:
				if hasBase {
					return &object.Error{Message: "int() can't convert non-string with explicit base", ExceptionType: object.ExceptionTypeTypeError}
				}
				return object.NewInteger(int64(arg.FloatValue()))
			case *object.String:
				return parseIntLiteral(arg.StringValue(), base)
			default:
				return errors.NewTypeError("INTEGER, FLOAT, or STRING", arg.Type().String())
			}
		},
		HelpText: `int(x[, base]) - Convert an object to an integer

Converts a float, string, or integer to an integer.
Floats are truncated (not rounded).
With base, converts a string in the given base (2-36) to an integer.
Examples: int("ff", 16) == 255, int("0b1010", 2) == 10, int("77", 8) == 63`,
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
				return object.NewFloat(float64(arg.IntValue()))
			case *object.String:
				return parseFloatLiteral(arg.StringValue())
			default:
				return errors.NewTypeError("INTEGER, FLOAT, or STRING", arg.Type().String())
			}
		},
		HelpText: `float(obj) - Convert an object to a float

Converts an integer, string, or float to a float.`,
	},

	"sum": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.RangeArgs(args, 1, 2); err != nil {
				return err
			}

			// Optional start value, as in Python: sum(prices, 0.0) seeds a
			// float sum, sum(counts, 10) offsets the total.
			startInt := int64(0)
			startFloat := 0.0
			hasFloat := false
			if len(args) == 2 {
				switch v := args[1].(type) {
				case *object.Integer:
					startInt = v.IntValue()
				case *object.Float:
					startFloat = v.FloatValue()
					hasFloat = true
				default:
					return errors.NewError("sum() start must be a number, got %s", args[1].Type())
				}
			}

			var elements []object.Object
			// Fast path for FloatArray (flat numeric sum, including 2D data).
			if fa, ok := args[0].(*object.FloatArray); ok {
				var faSum float64
				for _, v := range fa.Data {
					faSum += v
				}
				return object.NewFloat(faSum)
			}
			// Any other iterable: list, tuple, string, set, dict, dict views, iterator.
			var ok bool
			var rerr object.Object
			elements, ok, rerr = iterableToSliceCheckedFn(ctx, args[0], GetEnvFromContext(ctx))
			if rerr != nil {
				return rerr
			}
			if !ok {
				return errors.NewTypeError("iterable", args[0].Type().String())
			}

			intSum := startInt
			floatSum := startFloat

			for _, elem := range elements {
				switch v := elem.(type) {
				case *object.Integer:
					if hasFloat {
						floatSum += float64(v.IntValue())
					} else {
						intSum += v.IntValue()
					}
				case *object.Float:
					if !hasFloat {
						floatSum = float64(intSum)
						hasFloat = true
					}
					floatSum += v.FloatValue()
				default:
					return errors.NewError("unsupported operand type for sum(): %s", elem.Type())
				}
			}

			if hasFloat {
				return object.NewFloat(floatSum)
			}
			return object.NewInteger(intSum)
		},
		HelpText: `sum(iterable, start=0) - Sum elements of iterable

Returns the sum of all elements in an iterable, plus start (as in Python:
sum(prices, 0.0) seeds a float result).
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
			} else if kwargs.Has("start") {
				startObj, err := kwargs.Get("start").AsInt()
				if err != nil {
					return errors.ParameterError("start", err)
				}
				start = startObj
			}
			// Validate iterable type; instances run the iterator protocol.
			iterArg, rerr := acceptInstanceIterableFn(ctx, args[0])
			if rerr != nil {
				return rerr
			}
			if iterArg == nil {
				return errors.NewTypeError("iterable (LIST, TUPLE, STRING, ITERATOR, FLOAT_ARRAY)", args[0].Type().String())
			}
			return object.NewEnumerateIterator(iterArg, start)
		},
		HelpText: `enumerate(iterable[, start=0]) - Return (index, value) pairs

Returns an iterator of tuples containing the index and value for each item.
Default start is 0. Use list(enumerate(...)) to get a list.`,
	},
	"zip": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			strict := false
			for _, k := range kwargs.Keys() {
				if k != "strict" {
					return &object.Exception{Message: fmt.Sprintf("zip() got an unexpected keyword argument '%s'", k), ExceptionType: object.ExceptionTypeTypeError, Raised: true}
				}
				truthy, errObj := evalTruthyFn(ctx, kwargs.Get(k), GetEnvFromContext(ctx))
				if errObj != nil {
					return errObj
				}
				strict = truthy
			}
			if len(args) == 0 {
				// Return empty iterator for no arguments
				return object.NewZipIterator([]object.Object{})
			}
			if errObj := checkDictPairing("zip", args); errObj != nil {
				return errObj
			}
			// Validate all arguments are iterable; instances run the iterator
			// protocol (lazily — only __iter__ is called here).
			iterArgs := make([]object.Object, len(args))
			for i, arg := range args {
				iterArg, rerr := acceptInstanceIterableFn(ctx, arg)
				if rerr != nil {
					return rerr
				}
				if iterArg == nil {
					return errors.NewTypeError("iterable (LIST, TUPLE, STRING, ITERATOR, FLOAT_ARRAY)", arg.Type().String())
				}
				iterArgs[i] = iterArg
			}
			if strict {
				return object.NewStrictZipIterator(iterArgs)
			}
			return object.NewZipIterator(iterArgs)
		},
		HelpText: `zip(*iterables) - Aggregate elements from each iterable

Returns an iterator of tuples, where the i-th tuple contains the i-th element from each of the argument sequences or iterables.
The iterator stops when the shortest input iterable is exhausted; with
strict=True, inputs of different lengths raise ValueError.
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
			env := GetEnvFromContext(ctx)
			found, rerr := firstWhereTruthy(ctx, args[0], env, true)
			if rerr != nil {
				return rerr
			}
			if found {
				return TRUE
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
			env := GetEnvFromContext(ctx)
			found, rerr := firstWhereTruthy(ctx, args[0], env, false)
			if rerr != nil {
				return rerr
			}
			if found {
				return FALSE
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
			truthy, errObj := evalTruthyFn(ctx, args[0], GetEnvFromContext(ctx))
			if errObj != nil {
				return errObj
			}
			if truthy {
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
				if num.IntValue() < 0 {
					return object.NewInteger(-num.IntValue())
				}
				return num
			case *object.Float:
				if num.FloatValue() < 0 {
					return object.NewFloat(-num.FloatValue())
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
			return minMaxFunction(ctx, kwargs, false, args...)
		},
		HelpText: `min(iterable, [key], [default]) or min(a, b, c, ..., [key]) - Return the smallest item

With a single iterable argument, returns its smallest item.
With multiple arguments, returns the smallest argument.
key (a function) computes the comparison value for each item; the item with
the smallest key is returned, not the key itself. Ties keep the first item.
default is returned when a single iterable argument is empty (it cannot be
combined with multiple arguments).

Example:
  min([3, 1, 2])                          # 1
  min(words, key=lambda w: len(w))         # shortest word
  min([], default=None)                    # None`,
	},
	"max": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			return minMaxFunction(ctx, kwargs, true, args...)
		},
		HelpText: `max(iterable, [key], [default]) or max(a, b, c, ..., [key]) - Return the largest item

With a single iterable argument, returns its largest item.
With multiple arguments, returns the largest argument.
key (a function) computes the comparison value for each item; the item with
the largest key is returned, not the key itself. Ties keep the first item.
default is returned when a single iterable argument is empty (it cannot be
combined with multiple arguments).

Example:
  max([3, 1, 2])                          # 3
  max(records, key=lambda r: r["amount"]) # record with the highest amount
  max([], default=None)                    # None`,
	},
	"round": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) < 1 || len(args) > 2 {
				return errors.NewError("round() takes 1 or 2 arguments (%d given)", len(args))
			}
			ndigits := 0
			gaveNdigits := len(args) == 2
			if gaveNdigits {
				nd, err := args[1].AsInt()
				if err != nil {
					return errors.ParameterError("ndigits", err)
				}
				ndigits = int(nd)
			}
			var value float64
			wasInt := false
			switch num := args[0].(type) {
			case *object.Integer:
				if ndigits >= 0 {
					return num
				}
				wasInt = true
				value = float64(num.IntValue())
			case *object.Float:
				value = num.FloatValue()
			default:
				return errors.NewTypeError("INTEGER or FLOAT", args[0].Type().String())
			}
			if !gaveNdigits {
				return object.NewInteger(int64(roundHalfEven(value)))
			}
			if ndigits >= 0 {
				// FormatFloat rounds the true binary value to ndigits decimal
				// places with ties to even, which is exactly Python's
				// round(float, n). The multiply-by-10^n approach is not: it
				// can land on a binary tie the real value doesn't have
				// (round(2.675, 2) would give 2.68 instead of 2.67).
				f, err := strconv.ParseFloat(strconv.FormatFloat(value, 'f', ndigits, 64), 64)
				if err != nil {
					return errors.NewError("round() cannot represent the rounded value")
				}
				return object.NewFloat(f)
			}
			multiplier := math.Pow(10, float64(ndigits))
			rounded := roundHalfEven(value*multiplier) / multiplier
			// Python: negative ndigits keeps the input's type (int in, int
			// out; float in, float out).
			if wasInt {
				return object.NewInteger(int64(rounded))
			}
			return object.NewFloat(rounded)
		},
		HelpText: `round(number[, ndigits]) - Round a number to given precision

Rounds to ndigits decimal places (default 0), ties going to the even digit
like Python. Returns an integer if ndigits is omitted or 0.`,
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
				return object.NewString(fmt.Sprintf("0x%x", num))
			}
			return object.NewString(fmt.Sprintf("-0x%x", -num))
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
				return object.NewString(fmt.Sprintf("0b%b", num))
			}
			return object.NewString(fmt.Sprintf("-0b%b", -num))
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
				return object.NewString(fmt.Sprintf("0o%o", num))
			}
			return object.NewString(fmt.Sprintf("-0o%o", -num))
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
				base = float64(b.IntValue())
			case *object.Float:
				base = b.FloatValue()
			default:
				return errors.NewTypeError("INTEGER or FLOAT", args[0].Type().String())
			}
			switch e := args[1].(type) {
			case *object.Integer:
				exp = float64(e.IntValue())
			case *object.Float:
				exp = e.FloatValue()
			default:
				return errors.NewTypeError("INTEGER or FLOAT", args[1].Type().String())
			}
			result := math.Pow(base, exp)
			if len(args) == 3 {
				// pow(base, exp, mod) - modular exponentiation
				var mod float64
				switch m := args[2].(type) {
				case *object.Integer:
					mod = float64(m.IntValue())
				case *object.Float:
					mod = m.FloatValue()
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
			return object.NewFloat(result)
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
				a = float64(n.IntValue())
			case *object.Float:
				a = n.FloatValue()
				bothInts = false
			default:
				return errors.NewTypeError("INTEGER or FLOAT", args[0].Type().String())
			}
			switch n := args[1].(type) {
			case *object.Integer:
				b = float64(n.IntValue())
			case *object.Float:
				b = n.FloatValue()
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
				object.NewFloat(quotient),
				object.NewFloat(remainder),
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
			return nativeBoolToBooleanObject(isCallableObject(args[0]))
		},
		HelpText: `callable(object) - Return True if the object appears callable, False otherwise`,
	},
	"property": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			return &object.Property{Getter: args[0]}
		},
		HelpText: `property(fget) - Return a property attribute. Use as @property decorator.`,
	},
	"staticmethod": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			// Wrap in a StaticMethod so callInstanceMethod skips prepending self
			return &object.StaticMethod{Fn: args[0]}
		},
		HelpText: `staticmethod(f) - Return a static method for function f. Use as @staticmethod decorator.`,
	},
	"classmethod": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			return &object.ClassMethod{Fn: args[0]}
		},
		HelpText: `classmethod(f) - Return a class method for function f. Use as @classmethod decorator.`,
	},
	"isinstance": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}

			// Collect types to check - handle tuple/list of types
			var typeArgs []object.Object
			switch t := args[1].(type) {
			case *object.Tuple:
				typeArgs = t.Elements
			case *object.List:
				typeArgs = t.Elements
			default:
				typeArgs = []object.Object{args[1]}
			}

			obj := args[0]
			objType := obj.Type().String()

			for _, typeArg := range typeArgs {
				// Check for class type (before AsString, since Class.AsString returns name)
				if class, ok := typeArg.(*object.Class); ok {
					if inst, ok := obj.(*object.Instance); ok {
						for c := inst.Class; c != nil; c = c.BaseClass {
							if c == class {
								return TRUE
							}
						}
					}
					continue
				}

				if b, ok := typeArg.(*object.Builtin); ok {
					if excName, isExc := exceptionBuiltins[b]; isExc {
						if exc, ok := obj.(*object.Exception); ok {
							actual := exc.ExceptionType
							if actual == "" {
								actual = "Exception"
							}
							if matchesNamedExceptionType(actual, excName) {
								return TRUE
							}
						}
						continue
					}
				}

				var typeName string
				if s, err := typeArg.AsString(); err == nil {
					typeName = s
				} else if b, ok := typeArg.(*object.Builtin); ok {
					if name, found := typeBuiltins[b]; found {
						typeName = name
					} else {
						return errors.NewError("isinstance() arg 2 must be a type, string, or tuple of types")
					}
				} else {
					return errors.NewError("isinstance() arg 2 must be a type, string, or tuple of types")
				}

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
				if inst, ok := obj.(*object.Instance); ok {
					for c := inst.Class; c != nil; c = c.BaseClass {
						if strings.EqualFold(c.Name, typeName) {
							return TRUE
						}
					}
				}
			}
			return FALSE
		},
		HelpText: `isinstance(object, type) - Return True if object is of the given type

Supports bare type names: isinstance(x, dict), isinstance(x, int)
Also supports string type names: isinstance(x, "dict"), isinstance(x, "int")
Supports tuple/list of types: isinstance(x, (dict, list, tuple))
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
			return object.NewString(string(rune(num)))
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
			case *object.List, *object.Tuple, *object.String, *object.Iterator, *object.FloatArray:
				return object.NewReversedIterator(args[0])
			default:
				return errors.NewTypeError("sequence (LIST, TUPLE, STRING, ITERATOR, FLOAT_ARRAY)", args[0].Type().String())
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
			elements, ok, rerr := iterableToSliceCheckedFn(ctx, args[0], GetEnvFromContext(ctx))
			if rerr != nil {
				return rerr
			}
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
	"append": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}
			list, ok := args[0].(*object.List)
			if !ok {
				return errors.NewTypeError("list", args[0].Type().String())
			}
			list.Elements = append(list.Elements, args[1])
			return NULL
		},
		HelpText: `append(list, value) - Append a value to a list

	Adds the value to the end of the list in place.
	Returns None.`,
	},
	"dict": {
		Attributes: map[string]object.Object{
			// dict.fromkeys(iterable[, value]) — the type-level constructor
			// for default mappings and dedup patterns.
			"fromkeys": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if len(args) < 1 || len(args) > 2 {
						return errors.NewError("fromkeys() takes 1-2 arguments (%d given)", len(args))
					}
					var value object.Object = NULL
					if len(args) == 2 {
						value = args[1]
					}
					keys, ok, rerr := iterableToSliceCheckedFn(ctx, args[0], GetEnvFromContext(ctx))
					if rerr != nil {
						return rerr
					}
					if !ok {
						return errors.NewTypeError("iterable", args[0].Type().String())
					}
					result := &object.Dict{Pairs: make(map[string]object.DictPair, len(keys))}
					for _, key := range keys {
						hk, rerr := evalHashKeyChecked(ctx, key)
						if rerr != nil {
							return rerr
						}
						result.Pairs[hk] = object.DictPair{Key: key, Value: value}
					}
					return result
				},
				HelpText: `fromkeys(iterable[, value]) - Create a dict with keys from iterable

Values default to None. Called as dict.fromkeys(...)`,
			},
		},
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			result := &object.Dict{Pairs: make(map[string]object.DictPair)}
			// Keyword arguments are applied last, so they override keys from
			// the positional mapping, as in Python: dict({"a": 1}, a=2).
			addKwargs := func() *object.Dict {
				for _, key := range kwargs.Keys() {
					result.Pairs[object.DictKey(object.NewString(key))] = object.DictPair{
						Key:   object.NewString(key),
						Value: kwargs.Get(key),
					}
				}
				return result
			}
			if len(args) == 0 {
				return addKwargs()
			}
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			// Normalize any other iterable (class instances with __iter__,
			// raw iterators — Python accepts e.g. a generator of pairs) into
			// a list of elements with raise checking before the type switch.
			arg := args[0]
			// A mapping object (an instance with keys(), such as a Counter)
			// copies key -> obj[key], as Python's dict(mapping) does.
			if inst, ok := arg.(*object.Instance); ok {
				if _, hasKeys := inst.Class.LookupMember("keys"); hasKeys {
					env := GetEnvFromContext(ctx)
					keysObj := callDunderMethodFn(ctx, inst, "keys", nil, env)
					if propagates(keysObj) {
						return keysObj
					}
					keys, ok, rerr := iterableToSliceCheckedFn(ctx, keysObj, env)
					if rerr != nil {
						return rerr
					}
					if !ok {
						return errors.NewTypeError("iterable keys()", keysObj.Type().String())
					}
					for _, k := range keys {
						v := callDunderMethodFn(ctx, inst, "__getitem__", []object.Object{k}, env)
						if v == nil {
							return errors.NewTypeError("mapping with __getitem__", inst.Class.Name)
						}
						if propagates(v) {
							return v
						}
						hk, herr := evalHashKeyChecked(ctx, k)
						if herr != nil {
							return herr
						}
						result.Pairs[hk] = object.DictPair{Key: k, Value: v}
					}
					return addKwargs()
				}
			}
			switch arg.(type) {
			case *object.Instance, *object.Iterator:
				elems, ok, rerr := iterableToSliceCheckedFn(ctx, arg, GetEnvFromContext(ctx))
				if rerr != nil {
					return rerr
				}
				if !ok {
					return errors.NewTypeError("DICT or LIST of pairs", arg.Type().String())
				}
				arg = &object.List{Elements: elems}
			}
			switch iter := arg.(type) {
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
				return errors.NewTypeError("DICT or LIST of pairs", arg.Type().String())
			}
			return addKwargs()
		},
		HelpText: `dict([mapping], **kwargs) - Create a dictionary

With no argument, returns an empty dict.
Can initialize from another dict or list of [key, value] pairs.
Keyword arguments are added last and override keys from the mapping.`,
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
			elements, ok, rerr := iterableToSliceCheckedFn(ctx, args[0], GetEnvFromContext(ctx))
			if rerr != nil {
				return rerr
			}
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
			elements, ok, rerr := iterableToSliceCheckedFn(ctx, args[0], GetEnvFromContext(ctx))
			if rerr != nil {
				return rerr
			}
			if !ok {
				return errors.NewTypeError("iterable", args[0].Type().String())
			}

			s := object.NewSet()
			for _, e := range elements {
				if err := evalSetAdd(ctx, s, e); err != nil {
					return err
				}
			}
			return s
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
			rendered, rerr := renderConvertedValue(ctx, args[0], "r", GetEnvFromContext(ctx))
			if rerr != nil {
				return rerr
			}
			return object.NewString(rendered)
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
			// Call __hash__ on instances that define it
			if inst, ok := args[0].(*object.Instance); ok {
				if _, hasHash := inst.Class.Methods["__hash__"]; hasHash && hashInstanceFn != nil {
					return hashInstanceFn(ctx, inst)
				}
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
			// Same rendering as f-strings and str.format, so format(42, "05d"),
			// f"{42:05d}" and "{:05d}".format(42) always agree.
			rendered, rerr := renderFormatArg(ctx, value, formatSpec, "", GetEnvFromContext(ctx))
			if rerr != nil {
				return rerr
			}
			return object.NewString(rendered)
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
			// As in Python: getattr, with only AttributeError meaning "no".
			result := getAttributeFn(ctx, args[0], name)
			if isAttributeError(result) {
				return FALSE
			}
			if propagates(result) {
				return result
			}
			return TRUE
		},
		HelpText: `hasattr(object, name) - Check if object has an attribute

Returns True if getattr(object, name) succeeds, False if it raises
AttributeError.`,
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
			result := getAttributeFn(ctx, args[0], name)
			if len(args) == 3 && isAttributeError(result) {
				return args[2]
			}
			return result
		},
		HelpText: `getattr(object, name[, default]) - Get an attribute from an object

Returns the value of the named attribute, exactly as object.name would.
If default is provided, returns it when the attribute doesn't exist.`,
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
				obj.SetField(name, args[2])
				obj.InvalidateBoundMethod(name)
				return NULL
			case *object.Dict:
				obj.Pairs[object.DictKey(object.NewString(name))] = object.DictPair{
					Key:   object.NewString(name),
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
				if _, ok := obj.GetField(name); ok {
					obj.DeleteField(name)
					obj.InvalidateBoundMethod(name)
					return NULL
				}
				return attributeError(obj, name)
			case *object.Dict:
				dictKey := object.DictKey(object.NewString(name))
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
				if step != nil && step.IntValue() == 0 {
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
	"next": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			return nextFunction(ctx, kwargs, args...)
		},
		HelpText: `next(iterator[, default]) - Return the next item from an iterator

Calls the iterator's next method. If the iterator is exhausted and default
is provided, returns default. Otherwise raises StopIteration.`,
	},
	"iter": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			return iterFunction(ctx, kwargs, args...)
		},
		HelpText: `iter(obj) - Return an iterator for an object

Returns an iterator for lists, tuples, strings, sets, dicts, and instances
with __iter__ or __next__. Use with next() to manually advance iteration.`,
	},
	"dir": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			return dirFunction(ctx, kwargs, args...)
		},
		HelpText: `dir([obj]) - Return a sorted list of names

With no argument: returns all builtin names.
For instances: fields and methods (including inherited).
For classes: method names.
For dicts: key names.`,
	},
	"issubclass": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}
			cls, ok := args[0].(*object.Class)
			if !ok {
				return errors.NewTypeError("CLASS", args[0].Type().String())
			}
			parent, ok := args[1].(*object.Class)
			if !ok {
				return errors.NewTypeError("CLASS", args[1].Type().String())
			}
			for c := cls; c != nil; c = c.BaseClass {
				if c == parent {
					return TRUE
				}
			}
			return FALSE
		},
		HelpText: `issubclass(cls, parent) - Return True if cls is a subclass of parent

Checks the full inheritance chain. issubclass(C, C) is True.`,
	},
	"copy": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			switch o := args[0].(type) {
			case *object.List:
				newElems := make([]object.Object, len(o.Elements))
				copy(newElems, o.Elements)
				return &object.List{Elements: newElems}
			case *object.Dict:
				newPairs := make(map[string]object.DictPair, len(o.Pairs))
				for k, v := range o.Pairs {
					newPairs[k] = v
				}
				return &object.Dict{Pairs: newPairs}
			case *object.Set:
				return o.Copy()
			case *object.Tuple:
				return o // immutable, safe to return same object
			case *object.Instance:
				clone := &object.Instance{Class: o.Class}
				o.RangeFields(func(k string, v object.Object) bool {
					clone.SetField(k, v)
					return true
				})
				return clone
			default:
				return args[0] // scalars are immutable
			}
		},
		HelpText: `copy(obj) - Return a shallow copy of an object

For lists, dicts, sets, and instances: returns a new object with the same
top-level contents. Nested objects are not copied (use copy.deepcopy for that).
Tuples and scalars are returned as-is (they are immutable).`,
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
	"ImportError": {
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
				ExceptionType: object.ExceptionTypeImportError,
			}
		},
		HelpText: `ImportError([message]) - Create an import error exception

Raised when a library or imported name cannot be imported.
Use with: raise ImportError("module not found")`,
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
	"RuntimeError": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			message := ""
			if len(args) > 0 {
				if str, err := args[0].AsString(); err == nil {
					message = str
				} else {
					message = args[0].Inspect()
				}
			}
			return &object.Exception{Message: message, ExceptionType: object.ExceptionTypeRuntimeError}
		},
		HelpText: `RuntimeError([message]) - Create a runtime error exception`,
	},
	"ZeroDivisionError": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			message := ""
			if len(args) > 0 {
				if str, err := args[0].AsString(); err == nil {
					message = str
				} else {
					message = args[0].Inspect()
				}
			}
			return &object.Exception{Message: message, ExceptionType: object.ExceptionTypeZeroDivisionError}
		},
		HelpText: `ZeroDivisionError([message]) - Create a zero division error exception`,
	},
	"IndexError": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			message := ""
			if len(args) > 0 {
				if str, err := args[0].AsString(); err == nil {
					message = str
				} else {
					message = args[0].Inspect()
				}
			}
			return &object.Exception{Message: message, ExceptionType: object.ExceptionTypeIndexError}
		},
		HelpText: `IndexError([message]) - Create an index error exception`,
	},
	"KeyError": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			message := ""
			if len(args) > 0 {
				if str, err := args[0].AsString(); err == nil {
					message = str
				} else {
					message = args[0].Inspect()
				}
			}
			return &object.Exception{Message: message, ExceptionType: object.ExceptionTypeKeyError}
		},
		HelpText: `KeyError([message]) - Create a key error exception`,
	},
	"AttributeError": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			message := ""
			if len(args) > 0 {
				if str, err := args[0].AsString(); err == nil {
					message = str
				} else {
					message = args[0].Inspect()
				}
			}
			return &object.Exception{Message: message, ExceptionType: object.ExceptionTypeAttributeError}
		},
		HelpText: `AttributeError([message]) - Create an attribute error exception`,
	},
	"OSError": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			message := ""
			if len(args) > 0 {
				if str, err := args[0].AsString(); err == nil {
					message = str
				} else {
					message = args[0].Inspect()
				}
			}
			return &object.Exception{Message: message, ExceptionType: object.ExceptionTypeOSError}
		},
		HelpText: `OSError([message]) - Create an OS error exception`,
	},
}

// compareObjectsCtx compares two objects, using __lt__/__eq__ for user
// instances (honoring custom ordering) and propagating any raise from them.
// When the left operand defines no __lt__, the reflected right.__gt__(left) is
// tried (Python's rich-comparison reflection). The second return is non-nil
// exactly when a comparison dunder raised/errored. Non-instance operands fall
// back to the pure compareObjects.
func compareObjectsCtx(ctx context.Context, a, b object.Object, env *object.Environment) (int, object.Object) {
	if inst, ok := a.(*object.Instance); ok {
		if result := callDunderMethodFn(ctx, inst, "__lt__", []object.Object{b}, env); result != nil {
			if object.IsError(result) || result.Type() == object.EXCEPTION_OBJ {
				return 0, result
			}
			if bl, ok := result.(*object.Boolean); ok && bl.BoolValue() {
				return -1, nil
			}
			if eqResult := callDunderMethodFn(ctx, inst, "__eq__", []object.Object{b}, env); eqResult != nil {
				if object.IsError(eqResult) || eqResult.Type() == object.EXCEPTION_OBJ {
					return 0, eqResult
				}
				if b2, ok := eqResult.(*object.Boolean); ok && b2.BoolValue() {
					return 0, nil
				}
			}
			return 1, nil
		}
	}
	// Reflected comparison: b.__gt__(a) is true exactly when a < b.
	if inst, ok := b.(*object.Instance); ok {
		if result := callDunderMethodFn(ctx, inst, "__gt__", []object.Object{a}, env); result != nil {
			if object.IsError(result) || result.Type() == object.EXCEPTION_OBJ {
				return 0, result
			}
			if bl, ok := result.(*object.Boolean); ok && bl.BoolValue() {
				return -1, nil
			}
			if eqResult := callDunderMethodFn(ctx, inst, "__eq__", []object.Object{a}, env); eqResult != nil {
				if object.IsError(eqResult) || eqResult.Type() == object.EXCEPTION_OBJ {
					return 0, eqResult
				}
				if b2, ok := eqResult.(*object.Boolean); ok && b2.BoolValue() {
					return 0, nil
				}
			}
			return 1, nil
		}
	}
	return compareObjects(a, b), nil
}

// boolNumber returns 0 or 1 for a boolean, giving booleans Python's ordering:
// False < True, and booleans order against numbers as 0 and 1.
func boolNumber(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

// roundHalfEven rounds to the nearest integer with exact .5 ties going to
// the even neighbor, matching Python's round(). math.Round would send ties
// away from zero instead.
func roundHalfEven(v float64) float64 {
	floor := math.Floor(v)
	switch diff := v - floor; {
	case diff > 0.5:
		return floor + 1
	case diff < 0.5:
		return floor
	}
	if math.Mod(floor, 2) == 0 {
		return floor
	}
	return floor + 1
}

// compareForSort orders two values for sorted() and list.sort(): numbers
// order together (booleans as 0 and 1), strings lexicographically, tuples
// and lists element-wise, and instances through their dunder comparisons
// (__lt__ with reflected __gt__, then __eq__) via compareObjectsCtx.
// Incomparable values are an error, never a silent no-op.
func compareForSort(ctx context.Context, left, right object.Object, env *object.Environment) (int, object.Object) {
	switch l := left.(type) {
	case *object.Integer:
		if r, ok := right.(*object.Integer); ok {
			if l.IntValue() < r.IntValue() {
				return -1, nil
			} else if l.IntValue() > r.IntValue() {
				return 1, nil
			}
			return 0, nil
		} else if r, ok := right.(*object.Float); ok {
			return cmpFloats(float64(l.IntValue()), r.FloatValue()), nil
		} else if r, ok := right.(*object.Boolean); ok {
			return cmpFloats(float64(l.IntValue()), boolNumber(r.BoolValue())), nil
		}
	case *object.Float:
		if r, ok := right.(*object.Float); ok {
			return cmpFloats(l.FloatValue(), r.FloatValue()), nil
		} else if r, ok := right.(*object.Integer); ok {
			return cmpFloats(l.FloatValue(), float64(r.IntValue())), nil
		} else if r, ok := right.(*object.Boolean); ok {
			return cmpFloats(l.FloatValue(), boolNumber(r.BoolValue())), nil
		}
	case *object.Boolean:
		if r, ok := right.(*object.Boolean); ok {
			return cmpFloats(boolNumber(l.BoolValue()), boolNumber(r.BoolValue())), nil
		} else if r, ok := right.(*object.Integer); ok {
			return cmpFloats(boolNumber(l.BoolValue()), float64(r.IntValue())), nil
		} else if r, ok := right.(*object.Float); ok {
			return cmpFloats(boolNumber(l.BoolValue()), r.FloatValue()), nil
		}
	case *object.String:
		if r, ok := right.(*object.String); ok {
			if l.StringValue() < r.StringValue() {
				return -1, nil
			} else if l.StringValue() > r.StringValue() {
				return 1, nil
			}
			return 0, nil
		}
	case *object.Instance:
		// __lt__ (with reflected __gt__), then __eq__. A raise from a dunder
		// aborts the sort and propagates.
		return compareObjectsCtx(ctx, left, right, env)
	case *object.Tuple:
		if r, ok := right.(*object.Tuple); ok {
			return compareElements(ctx, l.Elements, r.Elements, env)
		}
	case *object.List:
		if r, ok := right.(*object.List); ok {
			return compareElements(ctx, l.Elements, r.Elements, env)
		}
	}
	return 0, &object.Exception{
		Message:       fmt.Sprintf("'<' not supported between instances of '%s' and '%s'", getTypeName(left), getTypeName(right)),
		ExceptionType: object.ExceptionTypeTypeError,
		Raised:        true,
	}
}

// cmpFloats compares two numbers for the boolean/number ordering arms.
func cmpFloats(a, b float64) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func compareObjects(a, b object.Object) int {
	switch av := a.(type) {
	case *object.Integer:
		switch bv := b.(type) {
		case *object.Integer:
			if av.IntValue() < bv.IntValue() {
				return -1
			} else if av.IntValue() > bv.IntValue() {
				return 1
			}
			return 0
		case *object.Float:
			af := float64(av.IntValue())
			if af < bv.FloatValue() {
				return -1
			} else if af > bv.FloatValue() {
				return 1
			}
			return 0
		case *object.Boolean:
			return cmpFloats(float64(av.IntValue()), boolNumber(bv.BoolValue()))
		}
	case *object.Float:
		switch bv := b.(type) {
		case *object.Float:
			if av.FloatValue() < bv.FloatValue() {
				return -1
			} else if av.FloatValue() > bv.FloatValue() {
				return 1
			}
			return 0
		case *object.Integer:
			bf := float64(bv.IntValue())
			if av.FloatValue() < bf {
				return -1
			} else if av.FloatValue() > bf {
				return 1
			}
			return 0
		case *object.Boolean:
			return cmpFloats(av.FloatValue(), boolNumber(bv.BoolValue()))
		}
	case *object.Boolean:
		af := boolNumber(av.BoolValue())
		switch bv := b.(type) {
		case *object.Boolean:
			return cmpFloats(af, boolNumber(bv.BoolValue()))
		case *object.Integer:
			return cmpFloats(af, float64(bv.IntValue()))
		case *object.Float:
			return cmpFloats(af, bv.FloatValue())
		}
	case *object.String:
		if bv, ok := b.(*object.String); ok {
			if av.StringValue() < bv.StringValue() {
				return -1
			} else if av.StringValue() > bv.StringValue() {
				return 1
			}
			return 0
		}
	case *object.Tuple:
		if bv, ok := b.(*object.Tuple); ok {
			return compareElementsLenient(av.Elements, bv.Elements)
		}
	case *object.List:
		if bv, ok := b.(*object.List); ok {
			return compareElementsLenient(av.Elements, bv.Elements)
		}
	}
	// For incomparable types, return 0 (no swap)
	return 0
}

// compareElements compares two element slices lexicographically, mirroring
// Python's tuple/list ordering. Used by compareObjects for Tuple and List.

// compareElementsLenient is the error-blind element comparison used inside
// legacy compareObjects; sorting and ordering operators use the strict
// compareElements, which raises on incomparable elements like Python.
func compareElementsLenient(a, b []object.Object) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if c := compareObjects(a[i], b[i]); c != 0 {
			return c
		}
	}
	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}
	return 0
}

func compareElements(ctx context.Context, a, b []object.Object, env *object.Environment) (int, object.Object) {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		c, errObj := compareForSort(ctx, a[i], b[i], env)
		if errObj != nil {
			return 0, errObj
		}
		if c != 0 {
			return c, nil
		}
	}
	if len(a) < len(b) {
		return -1, nil
	}
	if len(a) > len(b) {
		return 1, nil
	}
	return 0, nil
}

// Initialize the complex builtin functions
// These are defined as variable assignments to allow forward declaration in the builtins map
// attributeError is the catchable AttributeError for a missing attribute,
// worded as Python words it: "'P' object has no attribute 'x'" for an
// instance, "type object 'P' ..." for a class, Python type names otherwise.
func attributeError(obj object.Object, name string) object.Object {
	var message string
	switch o := obj.(type) {
	case *object.Instance:
		message = fmt.Sprintf("'%s' object has no attribute '%s'", o.Class.Name, name)
	case *object.Dict:
		if o.Module != "" {
			message = fmt.Sprintf("module '%s' has no attribute '%s'", o.Module, name)
		} else {
			message = fmt.Sprintf("'dict' object has no attribute '%s'", name)
		}
	case *object.Class:
		message = fmt.Sprintf("type object '%s' has no attribute '%s'", o.Name, name)
	case *object.Super:
		message = fmt.Sprintf("'super' object has no attribute '%s'", name)
	case *object.Function, *object.LambdaFunction:
		message = fmt.Sprintf("'function' object has no attribute '%s'", name)
	case *object.Builtin:
		message = fmt.Sprintf("'builtin_function_or_method' object has no attribute '%s'", name)
	case *object.Property:
		message = fmt.Sprintf("'property' object has no attribute '%s'", name)
	default:
		message = fmt.Sprintf("'%s' object has no attribute '%s'", getTypeName(obj), name)
	}
	return &object.Exception{
		Message:       message,
		ExceptionType: object.ExceptionTypeAttributeError,
		Raised:        true,
	}
}

// getAttribute reads obj.name with dot-access semantics, so getattr() and
// hasattr() see bound methods, properties, __getattr__ and __class__ exactly
// as obj.name does. Dicts (including library modules) treat keys as
// attributes, then fall back to dict methods.
func getAttribute(ctx context.Context, obj object.Object, name string) object.Object {
	return evalIndexExpression(ctx, obj, object.NewString(name), true)
}

// isAttributeError reports whether obj is a raised AttributeError.
func isAttributeError(obj object.Object) bool {
	exc, ok := obj.(*object.Exception)
	return ok && exc.ExceptionType == object.ExceptionTypeAttributeError
}

// notCallableError is the catchable TypeError for calling a non-callable.
func notCallableError(obj object.Object) object.Object {
	return &object.Exception{
		Message:       fmt.Sprintf("'%s' object is not callable", getTypeName(obj)),
		ExceptionType: object.ExceptionTypeTypeError,
		Raised:        true,
	}
}

func init() {
	mapFunction = mapFunctionImpl
	filterFunction = filterFunctionImpl
	sortedFunction = sortedFunctionImpl
	minMaxFunction = minMaxFunctionImpl
	helpFunction = helpFunctionImpl
	dirFunction = dirFunctionImpl
	iterFunction = iterFunctionImpl
	nextFunction = nextFunctionImpl
	callDunderMethodFn = callDunderMethod
	getAttributeFn = getAttribute
	attachTypeMethods()
	hashInstanceFn = func(ctx context.Context, inst *object.Instance) object.Object {
		hashFn := inst.Class.Methods["__hash__"]
		return applyFunctionWithContext(ctx, hashFn, []object.Object{inst}, nil, nil)
	}

	// Build reverse lookup for isinstance() to support bare type names
	typeBuiltins = map[*object.Builtin]string{
		builtins["int"]:   "int",
		builtins["str"]:   "str",
		builtins["float"]: "float",
		builtins["bool"]:  "bool",
		builtins["list"]:  "list",
		builtins["dict"]:  "dict",
		builtins["tuple"]: "tuple",
		builtins["set"]:   "set",
		builtins["bytes"]: "bytes",
	}

	// Exception constructors (TypeError, ValueError, ...) are types for
	// isinstance(), matched with the same hierarchy as except clauses.
	// Base classes of the hierarchy (see exceptionParents) for except
	// clauses, isinstance() and raise.
	for _, name := range []string{"BaseException", "LookupError", "ArithmeticError"} {
		builtins[name] = exceptionConstructor(name)
	}
	exceptionBuiltins = make(map[*object.Builtin]string)
	for name, b := range builtins {
		if name == "Exception" || name == "BaseException" || name == "StopIteration" || strings.HasSuffix(name, "Error") {
			exceptionBuiltins[b] = name
		}
	}
}

// exceptionConstructor builds the constructor for a built-in exception type.
func exceptionConstructor(name string) *object.Builtin {
	return &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			message := ""
			if len(args) > 0 {
				if str, err := args[0].AsString(); err == nil {
					message = str
				} else {
					message = args[0].Inspect()
				}
			}
			return &object.Exception{Message: message, ExceptionType: name}
		},
		HelpText: name + "([message]) - Create a " + name + " exception",
	}
}

func sortedFunctionImpl(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	if err := errors.RangeArgs(args, 1, 2); err != nil {
		return err
	}

	var elements []object.Object
	// Accept any iterable (list, tuple, string, set, dict, dict views, iterator,
	// float array). Sorting uses an index permutation and builds a fresh result
	// slice, so the input is never mutated even when IterableToSlice aliases it.
	elements, ok, rerr := iterableToSliceCheckedFn(ctx, args[0], GetEnvFromContext(ctx))
	if rerr != nil {
		return rerr
	}
	if !ok {
		return errors.NewTypeError("iterable", args[0].Type().String())
	}

	// Check for key function - support builtin, function, and lambda
	var keyFunc object.Object
	keyArg := kwargs.Get("key")
	if len(args) == 2 {
		keyArg = args[1]
	}
	if keyArg != nil {
		if _, isNone := keyArg.(*object.Null); !isNone {
			if !isCallableObject(keyArg) {
				return notCallableError(keyArg)
			}
			keyFunc = keyArg
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
				key := applyFunctionWithContext(ctx, keyFunc, []object.Object{elem}, nil, env)
				if propagates(key) {
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
		sortEnv := GetEnvFromContext(ctx)
		sort.SliceStable(indices, func(i, j int) bool {
			var left, right object.Object
			if keys != nil {
				left, right = keys[indices[i]], keys[indices[j]]
			} else {
				left, right = elements[indices[i]], elements[indices[j]]
			}
			cmp, cerr := compareForSort(ctx, left, right, sortEnv)
			if cerr != nil {
				sortErr = cerr
				return false
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

// checkDictPairing rejects two or more dicts or dict views walked in step.
// Scriptling dict order is unspecified and can differ between two walks of
// the same dict, so zip(d.keys(), d.values()) could silently pair the wrong
// items.
func checkDictPairing(name string, args []object.Object) object.Object {
	dicts := 0
	for _, arg := range args {
		switch arg.(type) {
		case *object.Dict, *object.DictKeys, *object.DictValues, *object.DictItems:
			dicts++
		}
	}
	if dicts < 2 {
		return nil
	}
	return &object.Exception{
		Message:       fmt.Sprintf("%s() cannot walk two dicts or dict views together because dict order is unspecified; use d.items() or sorted(d)", name),
		ExceptionType: object.ExceptionTypeTypeError,
		Raised:        true,
	}
}

// hasIteratorArg reports whether any argument is a lazy iterator, which map()
// and filter() must pull from lazily (it may be infinite) rather than
// collect up front.
func hasIteratorArg(args []object.Object) bool {
	for _, arg := range args {
		if _, ok := arg.(*object.Iterator); ok {
			return true
		}
	}
	return false
}

// lazyApply returns an iterator over the iterables zipped together, passing
// each tuple to step. step returns the value to yield and whether to yield
// it. An error or raised exception from a source ends the iteration; one from
// step is yielded and the iteration can continue.
func lazyApply(ctx context.Context, iterables []object.Object, step func(items []object.Object) (object.Object, bool)) object.Object {
	sources := make([]object.Object, len(iterables))
	for i, arg := range iterables {
		src, rerr := acceptInstanceIterableFn(ctx, arg)
		if rerr != nil {
			return rerr
		}
		if src == nil {
			return errors.NewTypeError("iterable (LIST, TUPLE, STRING, ITERATOR)", arg.Type().String())
		}
		sources[i] = src
	}
	zipped := object.NewZipIterator(sources)
	done := false
	cc := newContextChecker(ctx)
	next := func() (object.Object, bool) {
		for !done {
			// filter() may skip any number of elements of an endless
			// input, so a timeout must be able to stop it.
			if err := cc.check(); err != nil {
				done = true
				return err, true
			}
			t, ok := zipped.Next()
			if !ok {
				done = true
				break
			}
			if propagates(t) {
				done = true
				return t, true
			}
			// A raise from the function is yielded; as in Python, later
			// elements can still be pulled.
			if res, keep := step(t.(*object.Tuple).Elements); keep || propagates(res) {
				return res, true
			}
		}
		return nil, false
	}
	if zipped.Infinite() {
		return object.NewInfiniteIterator(next)
	}
	return object.NewIterator(next)
}

// firstWhereTruthy reports whether any element of iterable has truthiness
// want, stopping at the first one; iterators are pulled one element at a
// time, so any() and all() finish on infinite iterators as in Python.
func firstWhereTruthy(ctx context.Context, iterable object.Object, env *object.Environment, want bool) (bool, object.Object) {
	test := func(elem object.Object) (bool, object.Object) {
		if propagates(elem) {
			return false, elem
		}
		truthy, errObj := evalTruthyFn(ctx, elem, env)
		if errObj != nil {
			return false, errObj
		}
		return truthy == want, nil
	}
	if it, ok := iterable.(*object.Iterator); ok {
		cc := newContextChecker(ctx)
		for {
			if err := cc.check(); err != nil {
				return false, err
			}
			elem, more := it.Next()
			if !more {
				return false, nil
			}
			if hit, errObj := test(elem); errObj != nil || hit {
				return hit, errObj
			}
		}
	}
	elements, ok, rerr := iterableToSliceCheckedFn(ctx, iterable, env)
	if rerr != nil {
		return false, rerr
	}
	if !ok {
		return false, errors.NewTypeError("iterable", iterable.Type().String())
	}
	for _, elem := range elements {
		if hit, errObj := test(elem); errObj != nil || hit {
			return hit, errObj
		}
	}
	return false, nil
}

func mapFunctionImpl(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	if len(args) < 2 {
		return errors.NewError("map() requires at least 2 arguments")
	}
	fn := args[0]
	if len(args) > 2 {
		if errObj := checkDictPairing("map", args[1:]); errObj != nil {
			return errObj
		}
	}
	if hasIteratorArg(args[1:]) {
		env := GetEnvFromContext(ctx)
		return lazyApply(ctx, args[1:], func(items []object.Object) (object.Object, bool) {
			// items is a fresh slice per element, so it can be the args.
			return applyFunctionWithContext(ctx, fn, items, nil, env), true
		})
	}
	// Get all iterables as slices
	iterables := make([][]object.Object, len(args)-1)
	minLen := -1
	for i, arg := range args[1:] {
		elements, ok, rerr := iterableToSliceCheckedFn(ctx, arg, GetEnvFromContext(ctx))
		if rerr != nil {
			return rerr
		}
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
		// A raised exception from the mapped function must propagate at the
		// first raising element, not be stored as a result element.
		if propagates(res) {
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
	if hasIteratorArg(args[1:]) {
		env := GetEnvFromContext(ctx)
		return lazyApply(ctx, args[1:], func(items []object.Object) (object.Object, bool) {
			elem := items[0]
			test := elem
			if fn.Type() != object.NULL_OBJ {
				test = applyFunctionWithContext(ctx, fn, []object.Object{elem}, nil, env)
				if propagates(test) {
					return test, true
				}
			}
			truthy, errObj := evalTruthyFn(ctx, test, env)
			if errObj != nil {
				return errObj, true
			}
			return elem, truthy
		})
	}
	iterable, ok, rerr := iterableToSliceCheckedFn(ctx, args[1], GetEnvFromContext(ctx))
	if rerr != nil {
		return rerr
	}
	if !ok {
		return errors.NewTypeError("iterable (LIST, TUPLE, STRING, ITERATOR)", args[1].Type().String())
	}

	// Eagerly evaluate and filter
	results := []object.Object{}
	env := GetEnvFromContext(ctx)
	for _, elem := range iterable {
		// If function is None, use truthiness of the element itself.
		if fn.Type() == object.NULL_OBJ {
			truthy, errObj := evalTruthyFn(ctx, elem, env)
			if errObj != nil {
				return errObj
			}
			if truthy {
				results = append(results, elem)
			}
		} else {
			res := applyFunctionWithContext(ctx, fn, []object.Object{elem}, nil, env)
			// A raised exception from the predicate must propagate, not be
			// coerced to "keep" via truthiness.
			if propagates(res) {
				return res
			}
			truthy, errObj := evalTruthyFn(ctx, res, env)
			if errObj != nil {
				return errObj
			}
			if truthy {
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
		topic := strObj.StringValue()

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
					if pair, ok := dict.Pairs[object.DictKey(object.NewString(funcName))]; ok {
						// Sub-library (nested dict) — treat topic as a library name
						if subDict, ok := pair.Value.(*object.Dict); ok {
							printLibraryHelp(writer, fmt.Sprintf("%s.%s", libName, funcName), subDict)
							return NULL
						}
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
				printLibraryHelp(writer, topic, dict)
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
		obj.RangeFields(func(name string, _ object.Object) bool {
			fmt.Fprintf(writer, "  - %s\n", name)
			return true
		})
		return NULL
	default:
		fmt.Fprintf(writer, "Help for %s:\n", obj.Type())
		fmt.Fprintf(writer, "  Type: %s\n", obj.Type())
		fmt.Fprintf(writer, "  Value: %s\n", obj.Inspect())
		return NULL
	}
}

// Helper to extract and print docstrings from functions
func printLibraryHelp(writer io.Writer, name string, dict *object.Dict) {
	fmt.Fprintf(writer, "%s functions:\n", name)
	if docPair, ok := dict.Pairs[object.DictKey(object.NewString("__doc__"))]; ok {
		if docStr, ok := docPair.Value.(*object.String); ok {
			fmt.Fprintln(writer, "")
			fmt.Fprintln(writer, "Description:")
			for _, line := range strings.Split(docStr.StringValue(), "\n") {
				fmt.Fprintf(writer, "  %s\n", line)
			}
			fmt.Fprintln(writer, "")
		}
	}
	fmt.Fprintln(writer, "Available functions:")
	var names []string
	docKey := object.DictKey(object.NewString("__doc__"))
	for k, pair := range dict.Pairs {
		if k != docKey {
			keyStr, _ := pair.Key.AsString()
			names = append(names, keyStr)
		}
	}
	sort.Strings(names)
	for _, n := range names {
		fmt.Fprintf(writer, "  - %s\n", n)
	}
	fmt.Fprintln(writer, "")
	fmt.Fprintf(writer, "Use help(\"%s.function_name\") for details on a specific function\n", name)
}

func printFunctionHelp(writer io.Writer, name string, fn *object.Function) {
	// Build function signature
	fmt.Fprintf(writer, "%s(", name)
	for i, param := range fn.Parameters {
		if i > 0 {
			fmt.Fprint(writer, ", ")
		}
		fmt.Fprint(writer, param.Value())
		if fn.DefaultValues != nil {
			if _, hasDefault := fn.DefaultValues[param.Value()]; hasDefault {
				fmt.Fprint(writer, "=...")
			}
		}
	}
	if fn.Variadic != nil {
		if len(fn.Parameters) > 0 {
			fmt.Fprint(writer, ", ")
		}
		fmt.Fprintf(writer, "*%s", fn.Variadic.Value())
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

func nextFunctionImpl(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	if err := errors.RangeArgs(args, 1, 2); err != nil {
		return err
	}
	var iter *object.Iterator
	switch o := args[0].(type) {
	case *object.Iterator:
		iter = o
	case *object.Instance:
		env := GetEnvFromContext(ctx)
		iter = instanceToIterator(ctx, o, env)
	default:
		return errors.NewTypeError("ITERATOR or iterable instance", args[0].Type().String())
	}
	val, ok := iter.Next()
	if !ok {
		if len(args) == 2 {
			return args[1]
		}
		return &object.Exception{Message: "StopIteration", ExceptionType: object.ExceptionTypeStopIteration, Raised: true}
	}
	return val
}

func iterFunctionImpl(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	if err := errors.ExactArgs(args, 1); err != nil {
		return err
	}
	switch o := args[0].(type) {
	case *object.Iterator:
		return o
	case *object.List, *object.Tuple, *object.String, *object.Set, *object.Dict,
		*object.DictKeys, *object.DictValues, *object.DictItems:
		elems, _ := object.IterableToSlice(args[0])
		i := 0
		return object.NewIterator(func() (object.Object, bool) {
			if i >= len(elems) {
				return nil, false
			}
			v := elems[i]
			i++
			return v, true
		})
	case *object.Instance:
		env := GetEnvFromContext(ctx)
		if fn, ok := findDunderMethod(o, "__iter__"); ok {
			iter, errObj := callIter(ctx, o, fn, env)
			if errObj != nil {
				return errObj
			}
			return iter
		}
		if _, ok := findDunderMethod(o, "__next__"); ok {
			return instanceToIterator(ctx, o, env)
		}
		return errors.NewTypeError("iterable", "INSTANCE (no __iter__ or __next__)")
	default:
		return errors.NewTypeError("iterable", args[0].Type().String())
	}
}

func dirFunctionImpl(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	var names []string
	if len(args) == 0 {
		for name := range builtins {
			names = append(names, name)
		}
	} else {
		switch o := args[0].(type) {
		case *object.Instance:
			seen := map[string]bool{}
			o.RangeFields(func(name string, _ object.Object) bool {
				if !seen[name] {
					names = append(names, name)
					seen[name] = true
				}
				return true
			})
			for c := o.Class; c != nil; c = c.BaseClass {
				for name := range c.Methods {
					if !seen[name] {
						names = append(names, name)
						seen[name] = true
					}
				}
			}
		case *object.Class:
			for name := range o.Methods {
				names = append(names, name)
			}
		case *object.Dict:
			// A module lists its members; an ordinary dict its methods.
			if o.Module == "" {
				names = builtinMethodDir(object.DICT_OBJ)
				break
			}
			for _, p := range o.Pairs {
				if s, err := p.Key.AsString(); err == nil {
					names = append(names, s)
				}
			}
		default:
			// Built-in values list their methods: dir([]) shows append, ...
			names = builtinMethodDir(o.Type())
		}
	}
	sort.Strings(names)
	elems := make([]object.Object, len(names))
	for i, n := range names {
		elems[i] = object.NewString(n)
	}
	return &object.List{Elements: elems}
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
			importCallback := env.GetImportCallbackWithContext()
			if importCallback == nil {
				return errors.NewError(errors.ErrImportError)
			}
			importErr := importCallback(ctx, libName)
			if importErr != nil {
				return importErrorToObject(importErr, errors.ErrImportError)
			}
			return &object.Null{}
		},
	}
}

// minMaxFunction implements both min() and max() (wantMax picks which).
// Both accept a single iterable or multiple arguments, an optional key
// function (the item whose key wins is returned, never the key itself, ties
// keep the first item) and, for the single-iterable form only, a default for
// an empty input, mirroring Python's semantics.
func minMaxFunctionImpl(ctx context.Context, kwargs object.Kwargs, wantMax bool, args ...object.Object) object.Object {
	name := "min"
	if wantMax {
		name = "max"
	}
	if len(args) == 0 {
		return errors.NewError("%s() requires at least 1 argument", name)
	}

	// Optional key function: builtin, function, or lambda, like sorted().
	var keyFunc object.Object
	if k := kwargs.Get("key"); k != nil {
		if _, isNone := k.(*object.Null); !isNone { // key=None means no key
			if !isCallableObject(k) {
				return notCallableError(k)
			}
			keyFunc = k
		}
	}

	// Optional default, only for the single-iterable form (as in Python).
	var defaultVal object.Object
	hasDefault := false
	if d := kwargs.Get("default"); d != nil {
		defaultVal = d
		hasDefault = true
		if len(args) > 1 {
			return errors.NewError("Cannot specify a default for %s() with multiple arguments", name)
		}
	}

	// Single argument is treated as an iterable (the FloatArray fast path
	// only applies without a key function, which needs per-element calls).
	if len(args) == 1 {
		if keyFunc == nil {
			if fa, ok := args[0].(*object.FloatArray); ok {
				if len(fa.Data) == 0 {
					if hasDefault {
						return defaultVal
					}
					return errors.NewError("%s() arg is an empty sequence", name)
				}
				best := fa.Data[0]
				for _, v := range fa.Data[1:] {
					if (wantMax && v > best) || (!wantMax && v < best) {
						best = v
					}
				}
				return object.NewFloat(best)
			}
		}
		// Any other iterable: list, tuple, string, set, dict, dict views, iterator.
		if elements, ok, rerr := iterableToSliceCheckedFn(ctx, args[0], GetEnvFromContext(ctx)); rerr != nil {
			return rerr
		} else if ok {
			if len(elements) == 0 {
				if hasDefault {
					return defaultVal
				}
				return errors.NewError("%s() arg is an empty sequence", name)
			}
			args = elements
		}
	}

	// keyFor computes an element's comparison value, calling keyFunc when set.
	env := GetEnvFromContext(ctx)
	keyFor := func(elem object.Object) (object.Object, object.Object) {
		if keyFunc == nil {
			return elem, nil
		}
		key := applyFunctionWithContext(ctx, keyFunc, []object.Object{elem}, nil, env)
		if propagates(key) {
			return nil, key
		}
		return key, nil
	}

	best := args[0]
	bestKey, rerr := keyFor(best)
	if rerr != nil {
		return rerr
	}
	for _, arg := range args[1:] {
		key, rerr := keyFor(arg)
		if rerr != nil {
			return rerr
		}
		cmp, raised := compareObjectsCtx(ctx, bestKey, key, env)
		if raised != nil {
			return raised
		}
		// Strict comparison: ties keep the first item, as in Python.
		if (wantMax && cmp < 0) || (!wantMax && cmp > 0) {
			best = arg
			bestKey = key
		}
	}
	return best
}

// parseIntLiteral converts a string to an int as Python's int(s, base) does:
// surrounding whitespace, a sign, a prefix matching the base (any prefix for
// base 0) and single underscores between digits are allowed. Anything else
// is a ValueError naming the original string.
func parseIntLiteral(text string, base int) object.Object {
	invalid := func() object.Object {
		return errors.NewValueError("invalid literal for int() with base %d: %s", base, object.ReprString(text))
	}
	s := strings.TrimSpace(text)
	sign := ""
	if strings.HasPrefix(s, "-") || strings.HasPrefix(s, "+") {
		sign, s = s[:1], s[1:]
	}
	digitBase := base
	prefixed := false
	if len(s) >= 2 && s[0] == '0' {
		switch p := s[1] | 0x20; {
		case p == 'x' && (base == 16 || base == 0):
			digitBase, prefixed = 16, true
		case p == 'b' && (base == 2 || base == 0):
			digitBase, prefixed = 2, true
		case p == 'o' && (base == 8 || base == 0):
			digitBase, prefixed = 8, true
		}
	}
	if prefixed {
		s = s[2:]
		// An underscore may follow the prefix: 0x_ff.
		s = strings.TrimPrefix(s, "_")
	} else if base == 0 {
		// Base 0 reads a decimal literal, where leading zeros are invalid.
		digitBase = 10
		if len(s) > 1 && s[0] == '0' && strings.Trim(s, "0_") != "" {
			return invalid()
		}
	}
	digits, ok := stripDigitUnderscores(s, isAlnumDigit)
	// The only sign is the one before any prefix: 0x-5 is invalid.
	if !ok || digits == "" || digits[0] == '-' || digits[0] == '+' {
		return invalid()
	}
	val, err := strconv.ParseInt(sign+digits, digitBase, 64)
	if err != nil {
		if isRangeError(err) {
			return errors.NewValueError("int() value out of range: %s", object.ReprString(text))
		}
		return invalid()
	}
	return object.NewInteger(val)
}

// parseFloatLiteral converts a string to a float as Python's float(s) does:
// decimal and exponent forms, inf/infinity/nan in any case, and single
// underscores between digits.
func parseFloatLiteral(text string) object.Object {
	invalid := func() object.Object {
		return errors.NewValueError("could not convert string to float: %s", object.ReprString(text))
	}
	s := strings.TrimSpace(text)
	body := strings.TrimLeft(s, "+-")
	if len(s)-len(body) > 1 {
		return invalid()
	}
	switch strings.ToLower(body) {
	case "inf", "infinity":
		if s[0] == '-' {
			return object.NewFloat(math.Inf(-1))
		}
		return object.NewFloat(math.Inf(1))
	case "nan":
		return object.NewFloat(math.NaN())
	}
	// Go also accepts hex floats and other forms Python's float() rejects.
	for _, c := range body {
		if !(c >= '0' && c <= '9') && c != '.' && c != 'e' && c != 'E' && c != '+' && c != '-' && c != '_' {
			return invalid()
		}
	}
	digits, ok := stripDigitUnderscores(s, isDecimalDigit)
	if !ok {
		return invalid()
	}
	v, err := strconv.ParseFloat(digits, 64)
	if err != nil && !isRangeError(err) {
		return invalid()
	}
	return object.NewFloat(v)
}

func isRangeError(err error) bool {
	numErr, ok := err.(*strconv.NumError)
	return ok && numErr.Err == strconv.ErrRange
}

func isDecimalDigit(c byte) bool { return c >= '0' && c <= '9' }

func isAlnumDigit(c byte) bool {
	return c >= '0' && c <= '9' || c|0x20 >= 'a' && c|0x20 <= 'z'
}

// stripDigitUnderscores removes underscores that sit between two digits (in
// any base) and reports false for any other underscore.
func stripDigitUnderscores(s string, isDigit func(c byte) bool) (string, bool) {
	if !strings.Contains(s, "_") {
		return s, true
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '_' {
			if i == 0 || i == len(s)-1 || !isDigit(s[i-1]) || !isDigit(s[i+1]) {
				return "", false
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String(), true
}

// isCallableObject reports whether obj can be called: functions, lambdas,
// builtins, bound methods, classes, instances defining __call__ and
// callable Go clients.
func isCallableObject(obj object.Object) bool {
	switch o := obj.(type) {
	case *object.Function, *object.LambdaFunction, *object.Builtin, *object.BoundMethod, *object.Class:
		return true
	case *object.Instance:
		_, ok := o.Class.Methods["__call__"]
		return ok
	case *object.ClientWrapper:
		_, ok := o.Client.(object.ScriptCallable)
		return ok
	}
	return false
}
