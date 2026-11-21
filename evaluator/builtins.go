package evaluator

import (
	"bytes"
	"context"
	stderrors "errors"
	"fmt"
	"io"
	"math"
	"math/big"
	goruntime "runtime"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
	"unsafe"

	"github.com/paularlott/scriptling/ast"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/stdlib"
)

// Forward declarations for complex builtins
// These are defined after the builtins map for better organization
var (
	// instanceHashFn is instanceHash; set in init() to break the init cycle.
	instanceHashFn func(ctx context.Context, inst *object.Instance) (object.Object, int64, object.Object)
	mapFunction    func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object
	filterFunction func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object
	sortedFunction func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object
	minMaxFunction func(ctx context.Context, kwargs object.Kwargs, wantMax bool, args ...object.Object) object.Object
	helpFunction   func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object
	dirFunction    func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object
	iterFunction   func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object
	nextFunction   func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object
	sumGenericFn   func(ctx context.Context, start, iterable object.Object) object.Object

	// callDunderMethodFn is set in init() to break the initialization cycle
	callDunderMethodFn func(ctx context.Context, inst *object.Instance, method string, args []object.Object, env *object.Environment) object.Object

	// assignAttributeFn assigns obj.name = value with the assignment
	// statement's rules; set in init() to break the init cycle
	assignAttributeFn func(ctx context.Context, obj, name, value object.Object) error

	// getAttributeFn is getAttribute, set in init() to break the init cycle
	getAttributeFn func(ctx context.Context, obj object.Object, name string) object.Object

	// instanceDunderFn is callDunderMethod (round() and divmod() call
	// __round__ and __divmod__); set in init() to break the init cycle.
	instanceDunderFn func(ctx context.Context, inst *object.Instance, method string, args []object.Object, env *object.Environment) object.Object

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
			if err := errors.NoKwargs("help", kwargs); err != nil {
				return err
			}
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
			if err := errors.NoKwargs("yield_now", kwargs); err != nil {
				return err
			}
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
			if err := errors.NoKwargs("map", kwargs); err != nil {
				return err
			}
			return withIterTypeName(mapFunction(ctx, kwargs, args...), iterTypeNameMap)
		},
		HelpText: `map(function, iterable, ...) - Apply function to every item

Returns an iterator of results from applying function to each item.
With multiple iterables, function must take that many arguments.
Use list(map(...)) to get a list.`,
	},
	"filter": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("filter", kwargs); err != nil {
				return err
			}
			return withIterTypeName(filterFunction(ctx, kwargs, args...), iterTypeNameFilter)
		},
		HelpText: `filter(function, iterable) - Filter elements by function

Returns an iterator of elements for which function returns true.
If function is None, removes falsy elements.
Use list(filter(...)) to get a list.`,
	},
	"print": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if kwargs.Len() > 0 {
				if err := errors.CheckKwargs("print", kwargs, "sep", "end", "file", "flush"); err != nil {
					return err
				}
			}
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
				_, err := io.WriteString(writer, s)
				if stderrors.Is(err, object.ErrOutputNoRoom) {
					return object.NewMemoryError("%s: no room for more captured output", object.ErrMemoryLimitExceeded)
				}
				if stderrors.Is(err, object.ErrOutputLimitExceeded) {
					return object.NewMemoryError("captured output too large: the limit is %d bytes", object.MaxValueBytes())
				}
				return nil
			}

			// Get sep kwarg (default: " ")
			sep := " "
			if sepObj := kwargs.Get("sep"); sepObj != nil {
				if sepStr, err := sepObj.AsString(); err == nil {
					sep = sepStr
				} else if _, ok := sepObj.(*object.Null); !ok {
					return errors.NewTypeErrorTagged("sep must be None or a string, not %s", getTypeName(sepObj))
				}
			}

			// Get end kwarg (default: "\n")
			end := "\n"
			if endObj := kwargs.Get("end"); endObj != nil {
				if endStr, err := endObj.AsString(); err == nil {
					end = endStr
				} else if _, ok := endObj.(*object.Null); !ok {
					return errors.NewTypeErrorTagged("end must be None or a string, not %s", getTypeName(endObj))
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
			// The line is one output: hold it to the output limit as the
			// arguments render, since each can be within the limit while
			// together (the same value passed many times) they are not.
			limit := object.OutputLimit()
			var total int64
			for i, arg := range args {
				s, raised := stringify(arg)
				if raised != nil {
					return raised
				}
				parts[i] = s
				if limit > 0 {
					total += int64(len(s) + len(sep))
					if total > limit {
						return object.NewOutputTooLargeError("printed line", limit)
					}
				}
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
			if err := errors.NoKwargs("len", kwargs); err != nil {
				return err
			}
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
				return object.NewInteger(int64(arg.Len()))
			case *object.Tuple:
				return object.NewInteger(int64(len(arg.Elements)))
			case *object.DictKeys:
				return object.NewInteger(int64(arg.Dict.Len()))
			case *object.DictValues:
				return object.NewInteger(int64(arg.Dict.Len()))
			case *object.DictItems:
				return object.NewInteger(int64(arg.Dict.Len()))
			case *object.Set:
				return object.NewInteger(int64(arg.Len()))
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
					if propagates(result) {
						return result
					}
					n, errObj := checkLenResult(result)
					if errObj != nil {
						return errObj
					}
					return object.NewInteger(n)
				}
				return errors.NewTypeError("object with __len__", "INSTANCE")
			case *object.Class:
				// len(Color) is the number of members of an enum.
				if arg.IsEnum {
					return object.NewInteger(int64(len(arg.EnumMembers)))
				}
				return errors.NewTypeError("STRING, LIST, DICT, TUPLE, SET, BYTES, or VIEW", args[0].Type().String())
			default:
				return errors.NewTypeError("STRING, LIST, DICT, TUPLE, SET, BYTES, or VIEW", args[0].Type().String())
			}
		},
		HelpText: `len(obj) - Return the length of an object

Returns the number of items in a string, bytes, list, dict, or tuple.`,
	},
	"type": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("type", kwargs); err != nil {
				return err
			}
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			return typeOf(args[0])
		},
		HelpText: `type(obj) - Return the type of an object

Returns the type's name as a string, as Python's type(obj).__name__ spells it:
"int", "str", "list", "NoneType", "function", "list_iterator", the class name
for an instance, and the class it was raised as for an exception (e.g.
"ValueError"). type(x) == int and type(x).__name__ work as in Python.`,
	},
	"str": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if kwargs.Len() > 0 {
				if err := errors.CheckKwargs("str", kwargs, "encoding", "errors"); err != nil {
					return err
				}
				// str(b, encoding="utf-8") decodes, as b.decode() does.
				if len(args) == 1 {
					return callBytesKeywordDecode(ctx, args[0], kwargs)
				}
			}
			if len(args) == 0 {
				return object.NewString("") // str() is ''
			}
			if len(args) != 1 {
				return strDecodeArgs(ctx, args, kwargs)
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
			if kwargs.Len() > 0 {
				if err := errors.CheckKwargs("int", kwargs, "base"); err != nil {
					return err
				}
			}
			if len(args) == 0 && len(kwargs.Keys()) == 0 {
				return object.NewInteger(0) // int() is 0
			}
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
				return floatToInt(arg.FloatValue())
			case *object.Boolean:
				// bool is an int subclass: int(True) == 1.
				if hasBase {
					return &object.Error{Message: "int() can't convert non-string with explicit base", ExceptionType: object.ExceptionTypeTypeError}
				}
				return object.NewInteger(boolToInt64(arg.BoolValue()))
			case *object.String:
				return parseIntLiteral(arg.StringValue(), base)
			case *object.Bytes:
				return parseIntLiteral(string(arg.BytesValue()), base)
			case *object.Instance:
				// int(obj) calls __int__, then __index__, as in Python.
				if !hasBase {
					for _, name := range []string{"__int__", "__index__"} {
						if _, ok := arg.Class.LookupMember(name); ok {
							r := callDunderMethodFn(ctx, arg, name, nil, GetEnvFromContext(ctx))
							if r != nil {
								return r
							}
						}
					}
				}
				return errors.NewTypeErrorTagged("int() argument must be a string, a bytes-like object or a real number, not '%s'", getTypeName(arg))
			default:
				return errors.NewTypeErrorTagged("int() argument must be a string, a bytes-like object or a real number, not '%s'", getTypeName(arg))
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
			if err := errors.NoKwargs("float", kwargs); err != nil {
				return err
			}
			if len(args) == 0 {
				return object.NewFloat(0) // float() is 0.0
			}
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			switch arg := args[0].(type) {
			case *object.Float:
				return arg
			case *object.Integer:
				return object.NewFloat(float64(arg.IntValue()))
			case *object.Boolean:
				return object.NewFloat(float64(boolToInt64(arg.BoolValue())))
			case *object.String:
				return parseFloatLiteral(arg.StringValue())
			case *object.Bytes:
				return parseFloatLiteral(string(arg.BytesValue()))
			case *object.Instance:
				// float(obj) calls __float__, then __index__.
				for _, name := range []string{"__float__", "__index__"} {
					if _, ok := arg.Class.LookupMember(name); ok {
						r := callDunderMethodFn(ctx, arg, name, nil, GetEnvFromContext(ctx))
						if i, ok := r.(*object.Integer); ok {
							return object.NewFloat(float64(i.IntValue()))
						}
						if r != nil {
							return r
						}
					}
				}
				return errors.NewTypeErrorTagged("float() argument must be a string or a real number, not '%s'", getTypeName(arg))
			default:
				return errors.NewTypeErrorTagged("float() argument must be a string or a real number, not '%s'", getTypeName(arg))
			}
		},
		HelpText: `float(obj) - Convert an object to a float

Converts an integer, string, or float to a float.`,
	},

	"sum": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if kwargs.Len() > 0 {
				if err := errors.CheckKwargs("sum", kwargs, "start"); err != nil {
					return err
				}
			}
			if err := errors.RangeArgs(args, 1, 2); err != nil {
				return err
			}

			// Optional start value, as in Python: sum(prices, 0.0) seeds a
			// float sum, sum(counts, 10) offsets the total.
			startInt := int64(0)
			startFloat := 0.0
			hasFloat := false
			var start object.Object
			if len(args) == 2 {
				start = args[1]
			}
			if kwargs.Len() > 0 {
				if v, ok := kwargs.Lookup("start"); ok && kwargs.Len() == 1 && start == nil {
					start = v
				} else {
					return errors.NewTypeErrorTagged("sum() got an unexpected keyword argument")
				}
			}
			if start != nil {
				switch v := start.(type) {
				case *object.Integer:
					startInt = v.IntValue()
				case *object.Float:
					startFloat = v.FloatValue()
					hasFloat = true
				case *object.Boolean:
					startInt = boolToInt64(v.BoolValue())
				case *object.String:
					return errors.NewTypeErrorTagged("sum() can't sum strings [use ''.join(seq) instead]")
				case *object.Bytes:
					return errors.NewTypeErrorTagged("sum() can't sum bytes [use b''.join(seq) instead]")
				default:
					if v, ok := valueMember(start); ok {
						if _, isStr := v.(*object.String); isStr {
							return errors.NewTypeErrorTagged("sum() can't sum strings [use ''.join(seq) instead]")
						}
					}
					// Any other start (a list, a tuple, an instance with
					// __add__) sums with +, as Python does.
					return sumGenericFn(ctx, start, args[0])
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
			// Floats are summed with Neumaier compensation, as Python 3.12+
			// does, so sum([0.1] * 10) is 1.0.
			var comp float64
			addFloat := func(x float64) {
				t := floatSum + x
				if math.Abs(floatSum) >= math.Abs(x) {
					comp += (floatSum - t) + x
				} else {
					comp += (x - t) + floatSum
				}
				floatSum = t
			}
			compensated := func() float64 {
				if comp != 0 && !math.IsInf(comp, 0) && !math.IsNaN(comp) {
					return floatSum + comp
				}
				return floatSum
			}

			for i, elem := range elements {
				if b, isBool := elem.(*object.Boolean); isBool {
					// bool is an int subclass: sum(x > 2 for x in xs) counts matches.
					elem = object.NewInteger(boolToInt64(b.BoolValue()))
				}
				switch v := elem.(type) {
				case *object.Integer:
					if hasFloat {
						addFloat(float64(v.IntValue()))
					} else {
						intSum += v.IntValue()
					}
				case *object.Float:
					if !hasFloat {
						floatSum = float64(intSum)
						hasFloat = true
					}
					addFloat(v.FloatValue())
				default:
					// A non-number: carry on with +, which raises
					// Python's TypeError or calls __add__/__radd__.
					var acc object.Object = object.NewInteger(intSum)
					if hasFloat {
						acc = object.NewFloat(compensated())
					}
					return sumGenericFn(ctx, acc, &object.List{Elements: elements[i:]})
				}
			}

			if hasFloat {
				return object.NewFloat(compensated())
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
			if kwargs.Len() > 0 {
				if err := errors.CheckKwargs("sorted", kwargs, "key", "reverse"); err != nil {
					return err
				}
			}
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
			if err := errors.NoKwargs("range", kwargs); err != nil {
				return err
			}
			if err := errors.RangeArgs(args, 1, 3); err != nil {
				return err
			}
			var vals [3]int64
			for i, a := range args {
				if v, errObj, ok := instanceIndex(ctx, a); ok {
					if errObj != nil {
						return errObj
					}
					a = object.NewInteger(v)
				}
				v, errObj := rangeArg(a)
				if errObj != nil {
					return errObj
				}
				vals[i] = v
			}
			start, stop, step := int64(0), vals[0], int64(1)
			if len(args) > 1 {
				start, stop = vals[0], vals[1]
			}
			if len(args) == 3 {
				step = vals[2]
				if step == 0 {
					return errors.NewValueError("range() arg 3 must not be zero")
				}
			}
			return object.NewRange(start, stop, step)
		},
		HelpText: `range([start,] stop[, step]) - Generate sequence of numbers

Returns an iterator of integers from start (inclusive) to stop (exclusive).
If start is omitted, defaults to 0. If step is omitted, defaults to 1.
Use list(range(...)) to get a list.`,
	},
	"keys": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("keys", kwargs); err != nil {
				return err
			}
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
			if err := errors.NoKwargs("values", kwargs); err != nil {
				return err
			}
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
			if err := errors.NoKwargs("items", kwargs); err != nil {
				return err
			}
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
			if kwargs.Len() > 0 {
				if err := errors.CheckKwargs("enumerate", kwargs, "start"); err != nil {
					return err
				}
			}
			if len(args) < 1 || len(args) > 2 {
				return errors.NewTypeErrorTagged("enumerate() takes 1 or 2 arguments (%d given)", len(args))
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
			if kwargs.Len() > 0 {
				if err := errors.CheckKwargs("zip", kwargs, "strict"); err != nil {
					return err
				}
			}
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
			if err := errors.NoKwargs("super", kwargs); err != nil {
				return err
			}
			var classObj *object.Class
			var instanceObj *object.Instance

			if len(args) == 0 {
				// Parameterless super() - infer from context
				env := GetEnvFromContext(ctx)

				// Find __class__ (should be in closure)
				obj, ok := env.Get("__class__")
				if !ok {
					return &object.Exception{Message: "super(): __class__ cell not found - are you calling super() from within a class method?", ExceptionType: object.ExceptionTypeRuntimeError, Raised: true}
				}
				var isClass bool
				classObj, isClass = obj.(*object.Class)
				if !isClass {
					return errors.NewRuntimeError("super(): __class__ is not a class")
				}

				// The instance is the method's first argument, whatever it
				// is named (slot 0 holds the first parameter); else a
				// variable named self, for super() in a nested scope.
				obj, ok = env.GetSlotByIndex(0)
				if inst, isInst := obj.(*object.Instance); !ok || !isInst || !instanceOfClass(inst, classObj) {
					obj, ok = env.Get("self")
				}
				if !ok {
					return &object.Exception{Message: "super(): 'self' not found - parameterless super() requires 'self' argument", ExceptionType: object.ExceptionTypeRuntimeError, Raised: true}
				}
				var isInstance bool
				instanceObj, isInstance = obj.(*object.Instance)
				if !isInstance {
					return errors.NewRuntimeError("super(): 'self' is not an instance")
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
				return errors.NewTypeErrorTagged("super(): obj must be an instance or subtype of type")
			}

			return &object.Super{Class: classObj, Instance: instanceObj}
		},
		HelpText: `super([type, obj]) - Return a proxy object that delegates method calls to a parent or sibling class.

		With no arguments, returns a proxy for the parent of the class containing the method code, bound to 'self'.
		With arguments, returns a proxy for the parent of 'type', bound to 'obj'.`,
	},
	"any": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("any", kwargs); err != nil {
				return err
			}
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
			if err := errors.NoKwargs("all", kwargs); err != nil {
				return err
			}
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
			if err := errors.NoKwargs("bool", kwargs); err != nil {
				return err
			}
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
			if err := errors.NoKwargs("abs", kwargs); err != nil {
				return err
			}
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			if inst, ok := args[0].(*object.Instance); ok {
				env := GetEnvFromContext(ctx)
				if result := callDunderMethodFn(ctx, inst, "__abs__", nil, env); result != nil {
					return result
				}
			}
			switch num := args[0].(type) {
			case *object.Integer:
				if num.IntValue() < 0 {
					return object.NewInteger(-num.IntValue())
				}
				return num
			case *object.Float:
				// math.Abs rather than a < 0 branch: -0.0 < 0 is false in
				// IEEE comparisons, so the branch would keep negative zero.
				return object.NewFloat(math.Abs(num.FloatValue()))
			case *object.Boolean:
				return object.NewInteger(boolToInt64(num.BoolValue()))
			default:
				return errors.NewTypeError("INTEGER or FLOAT", args[0].Type().String())
			}
		},
		HelpText: `abs(x) - Return the absolute value of a number

Works with both integers and floats.`,
	},
	"min": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if kwargs.Len() > 0 {
				if err := errors.CheckKwargs("min", kwargs, "key", "default"); err != nil {
					return err
				}
			}
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
			if kwargs.Len() > 0 {
				if err := errors.CheckKwargs("max", kwargs, "key", "default"); err != nil {
					return err
				}
			}
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
			if kwargs.Len() > 0 {
				if err := errors.CheckKwargs("round", kwargs, "ndigits"); err != nil {
					return err
				}
			}
			if nd, ok := kwargs.Lookup("ndigits"); ok && len(args) == 1 && nd.Type() != object.NULL_OBJ {
				args = []object.Object{args[0], nd}
			}
			if len(args) < 1 || len(args) > 2 {
				return errors.NewTypeErrorTagged("round() takes 1 or 2 arguments (%d given)", len(args))
			}
			if len(args) == 2 && args[1] == NULL {
				args = args[:1] // round(x, None) is round(x)
			}
			if inst, ok := args[0].(*object.Instance); ok {
				// round(x) calls x.__round__() (Decimal, Fraction, user types).
				if r := instanceDunderFn(ctx, inst, "__round__", args[1:], GetEnvFromContext(ctx)); r != nil {
					return r
				}
			}
			if b, ok := args[0].(*object.Boolean); ok {
				args = []object.Object{object.NewInteger(boolToInt64(b.BoolValue())), args[len(args)-1]}[:len(args)]
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
				// Exact integer rounding, ties to even: round(2**62, -1).
				if ndigits >= -18 {
					return roundIntDigits(num.IntValue(), -ndigits)
				}
				wasInt = true
				value = float64(num.IntValue())
			case *object.Float:
				value = num.FloatValue()
			default:
				return errors.NewTypeError("INTEGER or FLOAT", args[0].Type().String())
			}
			if !gaveNdigits {
				return floatToInt(roundHalfEven(value))
			}
			if ndigits >= 0 {
				// FormatFloat rounds the true binary value to ndigits decimal
				// places with ties to even, which is exactly Python's
				// round(float, n). The multiply-by-10^n approach is not: it
				// can land on a binary tie the real value doesn't have
				// (round(2.675, 2) would give 2.68 instead of 2.67).
				f, err := strconv.ParseFloat(strconv.FormatFloat(value, 'f', ndigits, 64), 64)
				if err != nil {
					return errors.NewOverflowError("round() cannot represent the rounded value")
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
			if rounded == 0 {
				rounded = math.Copysign(0, value)
			}
			return object.NewFloat(rounded)
		},
		HelpText: `round(number[, ndigits]) - Round a number to given precision

Rounds to ndigits decimal places (default 0), ties going to the even digit
like Python. Returns an integer if ndigits is omitted or 0.`,
	},
	"hex": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("hex", kwargs); err != nil {
				return err
			}
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			if v, errObj, ok := instanceIndex(ctx, args[0]); ok {
				if errObj != nil {
					return errObj
				}
				args = []object.Object{object.NewInteger(v)}
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
			if err := errors.NoKwargs("bin", kwargs); err != nil {
				return err
			}
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			if v, errObj, ok := instanceIndex(ctx, args[0]); ok {
				if errObj != nil {
					return errObj
				}
				args = []object.Object{object.NewInteger(v)}
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
			if err := errors.NoKwargs("oct", kwargs); err != nil {
				return err
			}
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			if v, errObj, ok := instanceIndex(ctx, args[0]); ok {
				if errObj != nil {
					return errObj
				}
				args = []object.Object{object.NewInteger(v)}
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
			if kwargs.Len() > 0 {
				if err := errors.CheckKwargs("pow", kwargs, "base", "exp", "mod"); err != nil {
					return err
				}
				var errObj object.Object
				if args, errObj = foldKeywords("pow", args, kwargs, []string{"base", "exp", "mod"}, nil); errObj != nil {
					return errObj
				}
			}
			if len(args) < 2 || len(args) > 3 {
				return errors.NewTypeErrorTagged("pow() takes 2 or 3 arguments (%d given)", len(args))
			}
			if len(args) == 2 {
				// pow(base, exp) is base ** exp.
				return evalInfixFn(ctx, ast.OpPow, args[0], args[1], GetEnvFromContext(ctx))
			}
			return powMod(args[0], args[1], args[2])
		},
		HelpText: `pow(base, exp[, mod]) - Return base to the power exp; optionally modulo mod

Equivalent to base**exp or base**exp % mod if mod is given.`,
	},
	"divmod": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("divmod", kwargs); err != nil {
				return err
			}
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}
			a, b := boolAsInt(args[0]), boolAsInt(args[1])
			_, aInt := a.(*object.Integer)
			_, bInt := b.(*object.Integer)
			if aInt && bInt {
				// Exact integer floor division and modulo.
				env := GetEnvFromContext(ctx)
				q := evalInfixFn(ctx, ast.OpFloorDiv, a, b, env)
				if propagates(q) {
					return q
				}
				r := evalInfixFn(ctx, ast.OpMod, a, b, env)
				if propagates(r) {
					return r
				}
				return &object.Tuple{Elements: []object.Object{q, r}}
			}
			// Instances implement divmod() with __divmod__ / __rdivmod__.
			if inst, ok := a.(*object.Instance); ok {
				if r := instanceDunderFn(ctx, inst, "__divmod__", args[1:], GetEnvFromContext(ctx)); r != nil && r != notImplementedValue {
					return r
				}
			}
			if inst, ok := b.(*object.Instance); ok {
				if r := instanceDunderFn(ctx, inst, "__rdivmod__", args[:1], GetEnvFromContext(ctx)); r != nil && r != notImplementedValue {
					return r
				}
			}
			af, aerr := numberAsFloat(a)
			bf, berr := numberAsFloat(b)
			if aerr || berr {
				return errors.NewTypeErrorTagged("unsupported operand type(s) for divmod(): '%s' and '%s'", getTypeName(args[0]), getTypeName(args[1]))
			}
			if bf == 0 {
				return errors.NewZeroDivisionError()
			}
			quotient, remainder := pyFloatDivmod(af, bf)
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
			if err := errors.NoKwargs("callable", kwargs); err != nil {
				return err
			}
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			return nativeBoolToBooleanObject(isCallableObject(args[0]))
		},
		HelpText: `callable(object) - Return True if the object appears callable, False otherwise`,
	},
	"property": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if kwargs.Len() > 0 {
				if err := errors.CheckKwargs("property", kwargs, "fget", "doc"); err != nil {
					return err
				}
				// doc is accepted and ignored: properties carry no docstring.
				var errObj object.Object
				if args, errObj = foldKeywords("property", args, kwargs, []string{"fget"}, nil); errObj != nil {
					return errObj
				}
			}
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			return trackWrapper(&object.Property{Getter: args[0]})
		},
		HelpText: `property(fget) - Return a property attribute. Use as @property decorator.`,
	},
	"staticmethod": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("staticmethod", kwargs); err != nil {
				return err
			}
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			// Wrap in a StaticMethod so callInstanceMethod skips prepending self
			return trackWrapper(&object.StaticMethod{Fn: args[0]})
		},
		HelpText: `staticmethod(f) - Return a static method for function f. Use as @staticmethod decorator.`,
	},
	"classmethod": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("classmethod", kwargs); err != nil {
				return err
			}
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			return trackWrapper(&object.ClassMethod{Fn: args[0]})
		},
		HelpText: `classmethod(f) - Return a class method for function f. Use as @classmethod decorator.`,
	},
	"isinstance": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("isinstance", kwargs); err != nil {
				return err
			}
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

			for _, typeArg := range typeArgs {
				// A union (int | None) matches when any member does.
				if b, ok := typeArg.(*object.Builtin); ok && b.Attributes != nil {
					if members, isUnion := object.TypeUnionMembers(b); isUnion {
						for _, m := range unionTypeArgs(members) {
							r := isinstanceFn(ctx, kwargs, obj, m)
							if r != FALSE {
								return r
							}
						}
						continue
					}
				}
				// Check for class type (before AsString, since Class.AsString returns name)
				if class, ok := typeArg.(*object.Class); ok {
					if inst, ok := obj.(*object.Instance); ok {
						for c := inst.Class; c != nil; c = c.BaseClass {
							if c == class {
								return TRUE
							}
						}
					}
					// A user exception class matches its raised exceptions.
					if exc, ok := obj.(*object.Exception); ok && class.ExceptionBase != "" {
						if matchesNamedExceptionTypeChain(exc.ExceptionType, class.Name, exc.TypeChain) {
							return TRUE
						}
					}
					continue
				}

				if b, ok := typeArg.(*object.Builtin); ok && (b == object.DefaultDictType || b == object.OrderedDictType) {
					if d, isDict := obj.(*object.Dict); isDict && (d.IsDefaultDict() && b == object.DefaultDictType || d.IsOrderedDict() && b == object.OrderedDictType) {
						return TRUE
					}
					continue
				}

				if b, ok := typeArg.(*object.Builtin); ok {
					if b == objectBuiltin {
						return TRUE
					}
					if b == typeTypeBuiltin {
						// isinstance(C, type): C is a class or a type.
						if isClassLike(obj) {
							return TRUE
						}
						continue
					}
					if excName, isExc := exceptionClassName(b); isExc {
						if exc, ok := obj.(*object.Exception); ok {
							actual := exc.ExceptionType
							if actual == "" {
								actual = "Exception"
							}
							if matchesNamedExceptionTypeChain(actual, excName, exc.TypeChain) {
								return TRUE
							}
						}
						// An instance of a user exception class not yet raised.
						if inst, ok := obj.(*object.Instance); ok && inst.Class.ExceptionBase != "" {
							if matchesNamedExceptionTypeChain(exceptionNameOf(inst.Class), excName, nil) {
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
						return errors.NewTypeErrorTagged("isinstance() arg 2 must be a type, string, or tuple of types")
					}
				} else {
					return errors.NewTypeErrorTagged("isinstance() arg 2 must be a type, string, or tuple of types")
				}

				// A type name is what type() reports for the type ("int",
				// "NoneType", a class name), matched exactly.
				if isinstanceOfTypeName(obj, typeName) {
					return TRUE
				}
			}
			return FALSE
		},
		HelpText: `isinstance(object, type) - Return True if object is of the given type

Supports bare type names: isinstance(x, dict), isinstance(x, int)
Also supports type names as type() reports them: isinstance(x, "dict"),
isinstance(x, "int"), isinstance(x, "NoneType"), isinstance(obj, "MyClass")
Supports tuple/list of types: isinstance(x, (dict, list, tuple))
Also works with class types: isinstance(obj, MyClass)`,
	},
	"chr": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("chr", kwargs); err != nil {
				return err
			}
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			num, err := args[0].AsInt()
			if err != nil {
				return errors.ParameterError("i", err)
			}
			if num < 0 || num > 0x10FFFF {
				return errors.NewValueError("chr() arg not in range(0x110000)")
			}
			return object.NewString(string(rune(num)))
		},
		HelpText: `chr(i) - Return a string of one character from Unicode code point

The argument must be in the range 0-1114111 (0x10FFFF).`,
	},
	"ord": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("ord", kwargs); err != nil {
				return err
			}
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			if b, ok := args[0].(*object.Bytes); ok {
				if b.Len() != 1 {
					return errors.NewTypeErrorTagged("ord() expected a character, but string of length %d found", b.Len())
				}
				return object.NewInteger(int64(b.BytesValue()[0]))
			}
			str, err := args[0].AsString()
			if err != nil {
				return errors.ParameterError("c", err)
			}
			runes := []rune(str)
			if len(runes) != 1 {
				return errors.NewTypeErrorTagged("ord() expected a character, but string of length %d found", len(runes))
			}
			return object.NewInteger(int64(runes[0]))
		},
		HelpText: `ord(c) - Return Unicode code point for a one-character string

The argument must be a string of exactly one character.`,
	},
	"reversed": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("reversed", kwargs); err != nil {
				return err
			}
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			if inst, ok := args[0].(*object.Instance); ok {
				if result := callDunderMethodFn(ctx, inst, "__reversed__", nil, GetEnvFromContext(ctx)); result != nil {
					return result
				}
				// The sequence protocol: __len__ and __getitem__ give the
				// items last first.
				_, hasLen := inst.Class.LookupMember("__len__")
				_, hasItem := inst.Class.LookupMember("__getitem__")
				if hasLen && hasItem {
					env := GetEnvFromContext(ctx)
					lenResult := callDunderMethodFn(ctx, inst, "__len__", nil, env)
					if propagates(lenResult) {
						return lenResult
					}
					n, errObj := checkLenResult(lenResult)
					if errObj != nil {
						return errObj
					}
					if errObj := checkElementCount(int(n)); errObj != nil {
						return errObj
					}
					elems := make([]object.Object, 0, n)
					for i := int64(0); i < n; i++ {
						item := callDunderMethodFn(ctx, inst, "__getitem__", []object.Object{object.NewInteger(i)}, env)
						if propagates(item) {
							return item
						}
						elems = append(elems, item)
					}
					return object.NewReversedIterator(&object.List{Elements: elems}).SetTypeName(object.NewString("reversed"))
				}
				return errors.NewTypeErrorTagged("'%s' object is not reversible", inst.Class.Name)
			}
			switch args[0].(type) {
			case *object.List, *object.Tuple, *object.String, *object.Iterator, *object.FloatArray,
				*object.Dict, *object.DictKeys, *object.DictValues, *object.DictItems:
				// Dicts and their views reverse insertion order (Python 3.8+).
				// Named before reversing: reversing consumes an iterator.
				name := reversedTypeNameOf(args[0])
				return object.NewReversedIterator(args[0]).SetTypeName(name)
			case *object.Bytes:
				// The byte values, last first.
				elems, _, _ := iterableToSliceCheckedFn(ctx, args[0], GetEnvFromContext(ctx))
				return object.NewReversedIterator(&object.List{Elements: elems})
			default:
				return errors.NewTypeError("sequence (LIST, TUPLE, STRING, ITERATOR, FLOAT_ARRAY, DICT)", args[0].Type().String())
			}
		},
		HelpText: `reversed(seq) - Return a reversed iterator over the sequence

Works with lists, tuples, strings, iterators, dicts and dict views (a dict
reverses its insertion order).
Use list(reversed(...)) to get a list.`,
	},
	"list": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("list", kwargs); err != nil {
				return err
			}
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
			object.TrackSlice(result)
			return &object.List{Elements: result}
		},
		HelpText: `list([iterable]) - Create a list from an iterable

With no argument, returns an empty list.
Otherwise, returns a list containing the items of the iterable.`,
	},
	"append": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("append", kwargs); err != nil {
				return err
			}
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}
			list, ok := args[0].(*object.List)
			if !ok {
				return errors.NewTypeError("list", args[0].Type().String())
			}
			if errObj := appendElement(&list.Elements, args[1]); errObj != nil {
				return errObj
			}
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
						return errors.NewTypeErrorTagged("fromkeys() takes 1-2 arguments (%d given)", len(args))
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
					result := object.NewDict()
					for _, key := range keys {
						hk, rerr := evalHashKeyChecked(ctx, key)
						if rerr != nil {
							return rerr
						}
						result.Store(hk, key, value)
					}
					return result
				},
				HelpText: `fromkeys(iterable[, value]) - Create a dict with keys from iterable

Values default to None. Called as dict.fromkeys(...)`,
			},
		},
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			result := object.NewDict()
			// Keyword arguments are applied last, so they override keys from
			// the positional mapping, as in Python: dict({"a": 1}, a=2).
			addKwargs := func() *object.Dict {
				for _, key := range kwargs.Keys() {
					result.SetByString(key, kwargs.Get(key))
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
						result.Store(hk, k, v)
					}
					return addKwargs()
				}
			}
			if d, ok := arg.(*object.Dict); ok {
				// Copy existing dict, preserving its insertion order.
				result.StoreFrom(d)
				return addKwargs()
			}
			// Any other iterable of key/value pairs.
			env := GetEnvFromContext(ctx)
			elems, ok, rerr := iterableToSliceCheckedFn(ctx, arg, env)
			if rerr != nil {
				return rerr
			}
			if !ok {
				return errors.NewTypeErrorTagged("'%s' object is not iterable", getTypeName(arg))
			}
			if errObj := storeDictPairs(ctx, result, elems, env); errObj != nil {
				return errObj
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
			if err := errors.NoKwargs("tuple", kwargs); err != nil {
				return err
			}
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
			object.TrackSlice(result)
			return &object.Tuple{Elements: result}
		},
		HelpText: `tuple([iterable]) - Create a tuple from an iterable

With no argument, returns an empty tuple.
Otherwise, returns a tuple containing the items of the iterable.`,
	},
	"frozenset": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("frozenset", kwargs); err != nil {
				return err
			}
			if len(args) > 1 {
				return errors.NewTypeErrorTagged("frozenset() takes at most 1 argument (%d given)", len(args))
			}
			if len(args) == 0 {
				return object.NewFrozenSet()
			}
			// Elements must be hashable; evalSetAdd enforces that per
			// element, exactly as Python rejects unhashable elements.
			if s, ok := args[0].(*object.Set); ok {
				c := s.Copy()
				c.Frozen = true
				return c
			}
			elements, ok, rerr := iterableToSliceCheckedFn(ctx, args[0], GetEnvFromContext(ctx))
			if rerr != nil {
				return rerr
			}
			if !ok {
				return errors.NewTypeError("iterable", args[0].Type().String())
			}
			s := object.NewSetSized(min(len(elements), object.SetSmallMax))
			for _, e := range elements {
				if err := evalSetAdd(ctx, s, e); err != nil {
					return err
				}
			}
			return s.Freeze()
		},
		HelpText: `frozenset([iterable]) - Create an immutable set

Returns a set that cannot be modified: add/remove/update raise
AttributeError. Frozen sets are hashable (by content), so they can be
used as dict keys and set members, unlike regular sets.

Elements must be hashable, exactly as for set literals.

Example:
  fs = frozenset([1, 2, 3])
  2 in fs                     # True
  d = {frozenset("ab"): 1}    # legal: hashable key
  fs | {4}                    # frozenset({1, 2, 3, 4})`,
	},
	"set": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("set", kwargs); err != nil {
				return err
			}
			if len(args) == 0 {
				return object.NewSet()
			}
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}

			// Special case: set(s) returns a copy. Always mutable: the
			// constructor's result type is set, even when s is a frozenset
			// (use .copy() to preserve frozenness).
			if s, ok := args[0].(*object.Set); ok {
				c := s.Copy()
				c.Frozen = false
				return c
			}

			// Get elements from iterable
			elements, ok, rerr := iterableToSliceCheckedFn(ctx, args[0], GetEnvFromContext(ctx))
			if rerr != nil {
				return rerr
			}
			if !ok {
				return errors.NewTypeError("iterable", args[0].Type().String())
			}

			s := object.NewSetSized(min(len(elements), object.SetSmallMax))
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
			if err := errors.NoKwargs("repr", kwargs); err != nil {
				return err
			}
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
			if err := errors.NoKwargs("hash", kwargs); err != nil {
				return err
			}
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			// Call __hash__ on instances that define it
			if inst, ok := args[0].(*object.Instance); ok {
				fn, n, errObj := instanceHashFn(ctx, inst)
				if errObj != nil {
					return errObj
				}
				if fn != nil {
					return object.NewInteger(n)
				}
				// The identity hash, as object.__hash__ gives it: id >> 4.
				if id, ok := builtinByName("id").Fn(ctx, object.Kwargs{}, inst).(*object.Integer); ok {
					return object.NewInteger(id.IntValue() >> 4)
				}
			}
			// FNV-1a hash algorithm - fast and good distribution. It runs over
			// the value's dict key, so values that are equal as dict keys
			// (1, 1.0 and True, or equal instances in equal tuples) hash
			// alike.
			str, errObj := evalHashKeyChecked(ctx, args[0])
			if errObj != nil {
				return errObj
			}
			return object.NewInteger(hashKeyText(str))
		},
		HelpText: `hash(object) - Return the hash value of an object

Returns an integer hash value for the object using FNV-1a algorithm.`,
	},
	"vars": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("vars", kwargs); err != nil {
				return err
			}
			// vars() - the current scope's bindings as a dict (module
			// globals); vars(obj) - an instance's fields.
			if len(args) > 1 {
				return errors.NewTypeErrorTagged("vars() takes 0 or 1 arguments (%d given)", len(args))
			}
			if len(args) == 1 {
				inst, ok := args[0].(*object.Instance)
				if !ok {
					return errors.NewTypeError("instance", args[0].Type().String())
				}
				return instanceDictView(inst)
			}
			env := GetEnvFromContext(ctx)
			if env == nil {
				return object.NewDict()
			}
			d := object.NewDict()
			env.EachLocal(func(name string, v object.Object) {
				if v != nil && !strings.HasPrefix(name, "__") {
					d.SetByString(name, v)
				}
			})
			return d
		},
		HelpText: `vars() or vars(object) - Bindings of the current scope or an instance's fields`,
	},
	"id": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("id", kwargs); err != nil {
				return err
			}
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
			if err := errors.NoKwargs("format", kwargs); err != nil {
				return err
			}
			if len(args) < 1 || len(args) > 2 {
				return errors.NewTypeErrorTagged("format() takes 1 or 2 arguments (%d given)", len(args))
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
			if err := errors.NoKwargs("hasattr", kwargs); err != nil {
				return err
			}
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
			if err := errors.NoKwargs("getattr", kwargs); err != nil {
				return err
			}
			if len(args) < 2 || len(args) > 3 {
				return errors.NewTypeErrorTagged("getattr() takes 2 or 3 arguments (%d given)", len(args))
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
			if err := errors.NoKwargs("setattr", kwargs); err != nil {
				return err
			}
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
				// The same rules as obj.name = value: property setters and
				// read-only properties, frozen dataclasses.
				if err := assignAttributeFn(ctx, obj, object.NewString(name), args[2]); err != nil {
					return assignErrorToObject(err)
				}
				return NULL
			case *object.Dict:
				obj.SetByString(name, args[2])
				return NULL
			default:
				return errors.NewAttributeErrorTagged("'%s' object has no attribute '%s' and no __dict__ for setting new attributes", getTypeName(args[0]), name)
			}
		},
		HelpText: `setattr(object, name, value) - Set an attribute on an object

Sets the named attribute to the given value.
Only works on dict-like objects.`,
	},
	"delattr": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("delattr", kwargs); err != nil {
				return err
			}
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
				if _, ok := obj.Class.LookupMember("__delattr__"); ok {
					result := callDunderMethodFn(ctx, obj, "__delattr__", []object.Object{object.NewString(name)}, GetEnvFromContext(ctx))
					if propagates(result) {
						return result
					}
					return NULL
				}
				if _, ok := obj.GetField(name); ok {
					obj.DeleteField(name)
					obj.InvalidateBoundMethod(name)
					return NULL
				}
				return attributeError(obj, name)
			case *object.Dict:
				dictKey := object.DictKey(object.NewString(name))
				if _, ok := obj.Lookup(dictKey); ok {
					obj.Delete(dictKey)
					return NULL
				}
				return errors.NewAttributeErrorTagged("'dict' object has no attribute '%s'", name)
			default:
				return errors.NewAttributeErrorTagged("'%s' object has no attribute '%s'", getTypeName(args[0]), name)
			}
		},
		HelpText: `delattr(object, name) - Delete named attribute

Deletes the named attribute from the given object.`,
	},
	"slice": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("slice", kwargs); err != nil {
				return err
			}
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
					return errors.NewValueError("slice step cannot be zero")
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
			if err := errors.NoKwargs("next", kwargs); err != nil {
				return err
			}
			return nextFunction(ctx, kwargs, args...)
		},
		HelpText: `next(iterator[, default]) - Return the next item from an iterator

Calls the iterator's next method. If the iterator is exhausted and default
is provided, returns default. Otherwise raises StopIteration.`,
	},
	"iter": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("iter", kwargs); err != nil {
				return err
			}
			r := iterFunction(ctx, kwargs, args...)
			// iter() of an iterator is the iterator itself: keep its name.
			if len(args) == 1 && r != args[0] {
				if name := iterTypeNameOf(args[0]); name != nil {
					return withIterTypeName(r, name)
				}
			}
			return r
		},
		HelpText: `iter(obj) - Return an iterator for an object

Returns an iterator for lists, tuples, strings, sets, dicts, and instances
with __iter__ or __next__. Use with next() to manually advance iteration.`,
	},
	"dir": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("dir", kwargs); err != nil {
				return err
			}
			return dirFunction(ctx, kwargs, args...)
		},
		HelpText: `dir([obj]) - Return a sorted list of names

With no argument: returns all builtin names.
For instances: fields and methods (including inherited).
For classes: method names.
For dicts: key names.`,
	},
	"issubclass": {
		Fn: issubclassFn,
		HelpText: `issubclass(cls, classinfo) - Return True if cls is a subclass of classinfo

classinfo is a class, a builtin type (int, dict, ValueError, object, ...) or a
tuple of them. Checks the full inheritance chain; issubclass(C, C) is True,
issubclass(bool, int) and issubclass(MyError, Exception) are True.`,
	},
	"copy": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("copy", kwargs); err != nil {
				return err
			}
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			switch o := args[0].(type) {
			case *object.List:
				newElems := make([]object.Object, len(o.Elements))
				copy(newElems, o.Elements)
				object.TrackSlice(newElems)
				return &object.List{Elements: newElems}
			case *object.Dict:
				copied := object.NewDictSized(o.Len())
				copied.SetFactory(o.Factory())
				copied.SetOrdered(o.IsOrderedDict())
				copied.StoreFrom(o)
				return copied
			case *object.Set:
				return o.Copy()
			case *object.Tuple:
				return o // immutable, safe to return same object
			case *object.Instance:
				clone := object.NewInstance(o.Class)
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
				Args:          append([]object.Object{}, args...),
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
				Args:          append([]object.Object{}, args...),
			}
		},
		HelpText: `ValueError([message]) - Create a value error exception

Raised when an operation receives an argument with an inappropriate value.
Use with: raise ValueError("invalid value")`,
	},
	"OverflowError": {
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
				ExceptionType: object.ExceptionTypeOverflowError,
				Args:          append([]object.Object{}, args...),
			}
		},
		HelpText: `OverflowError([message]) - Create an overflow error exception

Raised when a result is too large to represent, such as a float integer
ratio beyond int64. An ArithmeticError subclass.
Use with: raise OverflowError("value too large")`,
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
				Args:          append([]object.Object{}, args...),
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
				Args:          append([]object.Object{}, args...),
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
				Args:          append([]object.Object{}, args...),
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
				Args:          append([]object.Object{}, args...),
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
			return &object.Exception{Message: message, ExceptionType: object.ExceptionTypeRuntimeError, Args: append([]object.Object{}, args...)}
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
			return &object.Exception{Message: message, ExceptionType: object.ExceptionTypeZeroDivisionError, Args: append([]object.Object{}, args...)}
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
			return &object.Exception{Message: message, ExceptionType: object.ExceptionTypeIndexError, Args: append([]object.Object{}, args...)}
		},
		HelpText: `IndexError([message]) - Create an index error exception`,
	},
	"KeyError": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			// str(KeyError(k)) is repr(k), as in Python.
			message := ""
			switch len(args) {
			case 0:
			case 1:
				message = object.ValueRepr(args[0])
			default:
				message = object.ValueRepr(&object.Tuple{Elements: args})
			}
			return &object.Exception{Message: message, ExceptionType: object.ExceptionTypeKeyError, Args: append([]object.Object{}, args...)}
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
			return &object.Exception{Message: message, ExceptionType: object.ExceptionTypeAttributeError, Args: append([]object.Object{}, args...)}
		},
		HelpText: `AttributeError([message]) - Create an attribute error exception`,
	},
	"AssertionError": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			message := ""
			if len(args) > 0 {
				if str, err := args[0].AsString(); err == nil {
					message = str
				} else {
					message = args[0].Inspect()
				}
			}
			return &object.Exception{Message: message, ExceptionType: object.ExceptionTypeAssertionError, Args: append([]object.Object{}, args...)}
		},
		HelpText: `AssertionError([message]) - Create an assertion error exception`,
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
			return &object.Exception{Message: message, ExceptionType: object.ExceptionTypeOSError, Args: append([]object.Object{}, args...)}
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
	// Instances without __lt__/__gt__ are unorderable, as in Python: a
	// silent 0 here used to leave sorted() unsorted instead of raising.
	if _, isInst := a.(*object.Instance); isInst {
		_, hasLt := a.(*object.Instance).Class.Methods["__lt__"]
		_, hasGt := a.(*object.Instance).Class.Methods["__gt__"]
		if !hasLt && !hasGt {
			return 0, &object.Exception{
				Message:       fmt.Sprintf("'<' not supported between instances of '%s' and '%s'", getTypeName(a), getTypeName(b)),
				ExceptionType: object.ExceptionTypeTypeError,
				Raised:        true,
			}
		}
	}
	if _, isInst := b.(*object.Instance); isInst {
		_, hasLt := b.(*object.Instance).Class.Methods["__lt__"]
		_, hasGt := b.(*object.Instance).Class.Methods["__gt__"]
		if !hasLt && !hasGt {
			return 0, &object.Exception{
				Message:       fmt.Sprintf("'<' not supported between instances of '%s' and '%s'", getTypeName(a), getTypeName(b)),
				ExceptionType: object.ExceptionTypeTypeError,
				Raised:        true,
			}
		}
	}
	return compareForSort(ctx, a, b, env)
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
	// compareElements starts the cancellation budget for a container walk.
	return compareForSortDepth(ctx, left, right, env, 0, nil)
}

// compareForSortDepth is compareForSort at a nesting depth; past
// maxCompareDepth nested sequences it raises RecursionError.
func compareForSortDepth(ctx context.Context, left, right object.Object, env *object.Environment, depth int, budget *cmpBudget) (int, object.Object) {
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
		// aborts the sort and propagates. Inside a container the depth
		// travels with the call (see deepEqual).
		if depth > 0 {
			ctx = object.WithCompareDepth(ctx, depth+1)
		}
		c, errObj := compareObjectsCtx(ctx, left, right, env)
		if errObj != nil && unorderedButEqual(budget, left, right, depth) {
			return 0, nil
		}
		return c, errObj
	case *object.Tuple:
		if r, ok := right.(*object.Tuple); ok {
			if l == r {
				return 0, nil
			}
			return compareElements(ctx, l.Elements, r.Elements, env, depth, budget)
		}
	case *object.List:
		if r, ok := right.(*object.List); ok {
			if l == r {
				return 0, nil
			}
			return compareElements(ctx, l.Elements, r.Elements, env, depth, budget)
		}
	}
	// Rarer orderable pairs, checked only once the common ones missed:
	// bytes, and a value against an instance (a namedtuple against a plain
	// tuple, sorted([obj, 1])) through the instance's reflected comparison.
	if l, ok := left.(*object.Bytes); ok {
		if r, ok := right.(*object.Bytes); ok {
			return bytes.Compare(l.BytesValue(), r.BytesValue()), nil
		}
	}
	if _, ok := right.(*object.Instance); ok {
		if depth > 0 {
			ctx = object.WithCompareDepth(ctx, depth+1)
		}
		c, errObj := compareObjectsCtx(ctx, left, right, env)
		if errObj != nil && unorderedButEqual(budget, left, right, depth) {
			return 0, nil
		}
		return c, errObj
	}
	if unorderedButEqual(budget, left, right, depth) {
		return 0, nil
	}
	return 0, &object.Exception{
		Message:       fmt.Sprintf("'<' not supported between instances of '%s' and '%s'", getTypeName(left), getTypeName(right)),
		ExceptionType: object.ExceptionTypeTypeError,
		Raised:        true,
	}
}

// instanceOfClass reports whether inst's class is cls or derives from it.
func instanceOfClass(inst *object.Instance, cls *object.Class) bool {
	for c := inst.Class; c != nil; c = c.BaseClass {
		if c == cls {
			return true
		}
	}
	return false
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

// compareElements compares two element slices lexicographically, mirroring
// Python's tuple/list ordering. A sequence compared with itself is equal
// without walking it (compareForSortDepth checks), so a self-containing list
// orders against itself instead of recursing forever.
func compareElements(ctx context.Context, a, b []object.Object, env *object.Environment, depth int, budget *cmpBudget) (int, object.Object) {
	if depth >= maxCompareDepth {
		return 0, errCompareTooDeep()
	}
	var local cmpBudget
	if budget == nil {
		local.ctx = ctx
		budget = &local
	}
	// The top level is not counted: a sort counts each comparison itself,
	// and a walk only grows past its values' size through nesting, which
	// is counted, so ordinary rows of scalars compare at no extra cost.
	if depth > 0 {
		if errObj := budget.step(); errObj != nil {
			return 0, errObj
		}
	}
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		c, errObj := compareForSortDepth(ctx, a[i], b[i], env, depth+1, budget)
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

// unorderedButEqual is compareForSortDepth's answer for two elements of
// sequences being compared (depth > 0) that cannot be ordered: as Python
// compares sequences by their first unequal elements, equal ones (equal
// instances, two Nones) compare as 0 and the comparison goes on. Kept out
// of line: it is the error path.
//
//go:noinline
func unorderedButEqual(budget *cmpBudget, a, b object.Object, depth int) bool {
	if depth == 0 {
		return false
	}
	if a == b {
		return true
	}
	if budget == nil {
		budget = &cmpBudget{}
	}
	eq, errObj := deepEqual(budget, a, b, depth)
	return errObj == nil && eq
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
	sumGenericFn = sumGeneric
	callDunderMethodFn = callDunderMethod
	assignAttributeFn = func(ctx context.Context, obj, name, value object.Object) error {
		return assignIndexValue(ctx, true, obj, name, value)
	}
	getAttributeFn = getAttribute
	attachTypeMethods()
	instanceDunderFn = callDunderMethod
	object.SetHashKeyHook(evalHashKeyChecked)
	instanceHashFn = instanceHash

	// Build reverse lookup for isinstance() to support bare type names
	typeTypeBuiltin = builtins["type"]
	typeBuiltins = map[*object.Builtin]string{
		builtins["int"]:       "int",
		builtins["str"]:       "str",
		builtins["float"]:     "float",
		builtins["bool"]:      "bool",
		builtins["list"]:      "list",
		builtins["dict"]:      "dict",
		builtins["tuple"]:     "tuple",
		builtins["slice"]:     "slice",
		builtins["set"]:       "set",
		builtins["frozenset"]: "frozenset",
		builtins["bytes"]:     "bytes",
	}

	// defaultdict reprs name their factory as Python does: <class 'list'>.
	object.FactoryRepr = func(factory object.Object) string {
		switch f := factory.(type) {
		case *object.Builtin:
			if name, ok := typeBuiltins[f]; ok {
				return "<class '" + name + "'>"
			}
		case *object.Class:
			return "<class '" + f.Name + "'>"
		case *object.Function:
			return fmt.Sprintf("<function %s at %p>", f.Name, f)
		case *object.LambdaFunction:
			return fmt.Sprintf("<function <lambda> at %p>", f)
		}
		return factory.Inspect()
	}

	// Exception constructors (TypeError, ValueError, ...) are types for
	// isinstance(), matched with the same hierarchy as except clauses.
	// Base classes of the hierarchy (see exceptionParents) for except
	// clauses, isinstance() and raise, plus the OSError family and other
	// named types Python code references.
	for _, name := range []string{
		"BaseException", "LookupError", "ArithmeticError",
		"FileNotFoundError", "FileExistsError", "IsADirectoryError",
		"NotADirectoryError", "TimeoutError", "ConnectionError",
		"ModuleNotFoundError", "RecursionError", "NotImplementedError",
		"UnicodeError", "UnicodeDecodeError", "UnicodeEncodeError",
		"KeyboardInterrupt", "JSONDecodeError", "AssertionError",
		"FrozenInstanceError", "MemoryError", "GeneratorExit", "UnboundLocalError",
		"SyntaxError", "IndentationError", "EOFError",
	} {
		builtins[name] = exceptionConstructor(name)
	}
	exceptionBuiltins = make(map[*object.Builtin]string)
	for name, b := range builtins {
		if name == "Exception" || name == "BaseException" || name == "StopIteration" || name == "KeyboardInterrupt" || name == "GeneratorExit" ||
			strings.HasSuffix(name, "Error") || strings.HasSuffix(name, "Exception") {
			exceptionBuiltins[b] = name
			b.Fn = trackExceptions(name, b.Fn)
		}
	}
	// Types print as classes: str(ValueError) is "<class 'ValueError'>".
	for b, name := range exceptionBuiltins {
		if b.Repr == "" {
			b.Repr = "<class '" + name + "'>"
		}
	}
	for b, name := range typeBuiltins {
		if b.Repr == "" {
			b.Repr = "<class '" + name + "'>"
		}
	}
}

// instanceIndex converts an instance to an integer through its __index__,
// where Python requires an integer (a sequence index, range(), hex()). ok
// is false when obj is not an instance with __index__.
func instanceIndex(ctx context.Context, obj object.Object) (v int64, errObj object.Object, ok bool) {
	inst, isInst := obj.(*object.Instance)
	if !isInst {
		return 0, nil, false
	}
	if _, has := inst.Class.LookupMember("__index__"); !has {
		return 0, nil, false
	}
	result := callDunderMethodFn(ctx, inst, "__index__", nil, GetEnvFromContext(ctx))
	if propagates(result) {
		return 0, result, true
	}
	switch r := result.(type) {
	case *object.Integer:
		return r.IntValue(), nil, true
	case *object.Boolean:
		return boolToInt64(r.BoolValue()), nil, true
	}
	return 0, errors.NewTypeErrorTagged("__index__ returned non-int (type %s)", getTypeName(result)), true
}

// rangeArg is one argument of range(): an int (or a bool). A float or any
// other type is a TypeError, as in Python.
func rangeArg(o object.Object) (int64, object.Object) {
	switch v := o.(type) {
	case *object.Integer:
		return v.IntValue(), nil
	case *object.Boolean:
		return boolToInt64(v.BoolValue()), nil
	}
	return 0, errors.NewTypeErrorTagged("'%s' object cannot be interpreted as an integer", getTypeName(o))
}

// boolAsInt returns a bool as the int it equals (True is 1), and any other
// value unchanged.
func boolAsInt(o object.Object) object.Object {
	if b, ok := o.(*object.Boolean); ok {
		return object.NewInteger(boolToInt64(b.BoolValue()))
	}
	return o
}

// numberAsFloat is an int or float's value as a float; bad is true for
// anything else.
func numberAsFloat(o object.Object) (f float64, bad bool) {
	switch n := o.(type) {
	case *object.Integer:
		return float64(n.IntValue()), false
	case *object.Float:
		return n.FloatValue(), false
	}
	return 0, true
}

// powMod is pow(base, exp, mod): modular exponentiation of ints (bools
// count as ints). A negative exp takes the modular inverse, as in Python
// 3.8+; the result has the sign of mod.
func powMod(baseObj, expObj, modObj object.Object) object.Object {
	base, bok := boolAsInt(baseObj).(*object.Integer)
	exp, eok := boolAsInt(expObj).(*object.Integer)
	mod, mok := boolAsInt(modObj).(*object.Integer)
	if !bok || !eok || !mok {
		return errors.NewTypeErrorTagged("pow() 3rd argument not allowed unless all arguments are integers")
	}
	if mod.IntValue() == 0 {
		return errors.NewValueError("pow() 3rd argument cannot be 0")
	}
	m := big.NewInt(mod.IntValue())
	absM := new(big.Int).Abs(m)
	r := new(big.Int).Exp(big.NewInt(base.IntValue()), big.NewInt(exp.IntValue()), absM)
	if r == nil {
		return errors.NewValueError("base is not invertible for the given modulus")
	}
	r.Mod(r, absM)
	if m.Sign() < 0 && r.Sign() != 0 {
		r.Add(r, m)
	}
	return object.NewInteger(r.Int64())
}

// floatToInt is int(f): f truncated toward zero, an OverflowError for
// infinity or a value outside the 64-bit int range, a ValueError for NaN.
func floatToInt(f float64) object.Object {
	if f >= -9223372036854775808.0 && f < 9223372036854775808.0 {
		return object.NewInteger(int64(f))
	}
	switch {
	case math.IsNaN(f):
		return errors.NewValueError("cannot convert float NaN to integer")
	case math.IsInf(f, 0):
		return errors.NewOverflowError("cannot convert float infinity to integer")
	}
	return errors.NewOverflowError("int too large to convert: %s; scriptling ints are 64-bit", object.FloatStr(f))
}

// strDecodeArgs is str(b, encoding[, errors]): a bytes value decoded, as
// b.decode(encoding, errors) does.
func strDecodeArgs(ctx context.Context, args []object.Object, kwargs object.Kwargs) object.Object {
	if len(args) > 3 {
		return errors.NewTypeErrorTagged("str() takes at most 3 arguments (%d given)", len(args))
	}
	b, ok := args[0].(*object.Bytes)
	if !ok {
		return errors.NewTypeErrorTagged("decoding to str: need a bytes-like object, %s found", getTypeName(args[0]))
	}
	return bytesDecodeFn(ctx, b, args[1:], kwargs)
}

// storeDictPairs stores each of elems, the elements of an iterable of
// key/value pairs, into d, as dict(iterable) and dict.update(iterable) do: a
// pair is any iterable of length two (a tuple, a list, a two-character
// string).
func storeDictPairs(ctx context.Context, d *object.Dict, elems []object.Object, env *object.Environment) object.Object {
	for i, elem := range elems {
		var pair []object.Object
		switch p := elem.(type) {
		case *object.Tuple:
			pair = p.Elements
		case *object.List:
			pair = p.Elements
		default:
			items, ok, rerr := iterableToSliceCheckedFn(ctx, elem, env)
			if rerr != nil {
				return rerr
			}
			if !ok {
				return errors.NewTypeErrorTagged("cannot convert dictionary update sequence element #%d to a sequence", i)
			}
			pair = items
		}
		if len(pair) != 2 {
			return errors.NewValueError("dictionary update sequence element #%d has length %d; 2 is required", i, len(pair))
		}
		hk, rerr := evalHashKeyChecked(ctx, pair[0])
		if rerr != nil {
			return rerr
		}
		if errObj := checkDictGrowth(d, hk); errObj != nil {
			return errObj
		}
		d.Store(hk, pair[0], pair[1])
	}
	return nil
}

// typeTypeBuiltin is the type builtin, for isinstance(C, type).
var typeTypeBuiltin *object.Builtin

// exceptionClassName returns the exception type a builtin exception class
// (ValueError, ...) or a library's registered one (re.error,
// json.JSONDecodeError) constructs.
func exceptionClassName(b *object.Builtin) (string, bool) {
	if name, ok := exceptionBuiltins[b]; ok {
		return name, true
	}
	return object.ExceptionClassOf(b)
}

// trackExceptions wraps an exception constructor so the exceptions a script
// creates are memory-accounted: one can hold another through its args, and
// its message is a copy of the first argument's text.
//
// It also rejects keyword arguments: exception constructors take none, except
// ImportError's name and path, which are accepted and ignored.
func trackExceptions(name string, fn object.BuiltinFunction) object.BuiltinFunction {
	importError := name == "ImportError" || name == "ModuleNotFoundError"
	return func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
		if kwargs.Len() > 0 {
			var err object.Object
			if importError {
				err = errors.CheckKwargs(name, kwargs, "name", "path")
			} else {
				err = errors.CheckKwargs(name, kwargs)
			}
			if err != nil {
				return err
			}
		}
		result := fn(ctx, kwargs, args...)
		if e, ok := result.(*object.Exception); ok {
			if len(args) > 1 && !importError {
				// str() of an exception with several arguments is the
				// repr of the argument tuple, as in Python; an OSError's
				// is "[Errno n] message".
				if matchesNamedExceptionTypeChain(name, object.ExceptionTypeOSError, nil) && len(args) == 2 {
					msg := args[1].Inspect()
					if sv, ok := args[1].(*object.String); ok {
						msg = sv.StringValue()
					}
					e.Message = "[Errno " + args[0].Inspect() + "] " + msg
				} else {
					e.Message = (&object.Tuple{Elements: args}).Inspect()
				}
			}
			object.TrackAlloc(e, exceptionObjectSize+int64(len(e.Message))+int64(cap(e.Args))*16)
		}
		return result
	}
}

// trackWrapper reports a property, staticmethod or classmethod a script
// creates: each holds another value and can be chained, out of reach of any
// counted container.
func trackWrapper[T any](w *T) *T {
	object.TrackAlloc(w, int64(unsafe.Sizeof(*w))+16)
	return w
}

// exceptionObjectSize is the memory an Exception object holds before its
// message and arguments.
const exceptionObjectSize = int64(unsafe.Sizeof(object.Exception{})) + 16

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
			return &object.Exception{Message: message, ExceptionType: name, Args: append([]object.Object{}, args...)}
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
			var callArg [1]object.Object // reused: a callee never keeps its args slice
			for i, elem := range elements {
				callArg[0] = elem
				key := applyFunctionWithContext(ctx, keyFunc, callArg[:], object.Kwargs{}, env)
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
		// A comparison that raised ends the sort: later ones are skipped. The
		// sort needs no cancellation check of its own: comparisons that call
		// script code (__lt__) are checked as function calls, nested values
		// are checked as they are walked (sortBudget), and a sort of plain
		// values is bounded by the memory limit on the list.
		sortBudget := cmpBudget{ctx: ctx}
		sortEnv := GetEnvFromContext(ctx)
		sort.SliceStable(indices, func(i, j int) bool {
			if sortErr != nil {
				return false
			}
			var left, right object.Object
			if keys != nil {
				left, right = keys[indices[i]], keys[indices[j]]
			} else {
				left, right = elements[indices[i]], elements[indices[j]]
			}
			cmp, cerr := compareForSortDepth(ctx, left, right, sortEnv, 0, &sortBudget)
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
		object.TrackSlice(newElements)
		for i, idx := range indices {
			newElements[i] = elements[idx]
		}
		elements = newElements
	}

	return &object.List{Elements: elements}
}

// hasIteratorArg reports whether any argument is a lazy iterator or a
// generator, which map() and filter() must pull from lazily (it may be
// infinite, or have side effects) rather than collect up front.
func hasIteratorArg(args []object.Object) bool {
	for _, arg := range args {
		switch a := arg.(type) {
		case *object.Iterator:
			return true
		case *object.Instance:
			if a.Class == generatorClass {
				return true
			}
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
	if _, ok := iterable.(*object.Instance); ok {
		// Pull a generator or other script iterator only up to the deciding
		// element, as Python does.
		src, rerr := acceptInstanceIterableFn(ctx, iterable)
		if rerr != nil {
			return false, rerr
		}
		if src != nil {
			iterable = src
		}
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
		return errors.NewTypeErrorTagged("map() must have at least two arguments.")
	}
	fn := args[0]
	if hasIteratorArg(args[1:]) {
		env := GetEnvFromContext(ctx)
		return lazyApply(ctx, args[1:], func(items []object.Object) (object.Object, bool) {
			// items is a fresh slice per element, so it can be the args.
			return applyFunctionWithContext(ctx, fn, items, object.Kwargs{}, env), true
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
	callArgs := make([]object.Object, len(iterables)) // reused: a callee never keeps its args slice
	for i := 0; i < minLen; i++ {
		for j := range iterables {
			callArgs[j] = iterables[j][i]
		}
		res := applyFunctionWithContext(ctx, fn, callArgs, object.Kwargs{}, env)
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
				test = applyFunctionWithContext(ctx, fn, []object.Object{elem}, object.Kwargs{}, env)
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
	var callArg [1]object.Object // reused: a callee never keeps its args slice
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
			callArg[0] = elem
			res := applyFunctionWithContext(ctx, fn, callArg[:], object.Kwargs{}, env)
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
						for _, pair := range dict.All() {
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
					if pair, ok := dict.Lookup(object.DictKey(object.NewString(funcName))); ok {
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
		for _, pair := range obj.All() {
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
	if docPair, ok := dict.Lookup(object.DictKey(object.NewString("__doc__"))); ok {
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
	for k, pair := range dict.All() {
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
		if _, _, _, isRange := o.RangeObject(); isRange {
			return &object.Exception{Message: "'range' object is not an iterator", ExceptionType: object.ExceptionTypeTypeError, Raised: true}
		}
		iter = o
	case *object.Instance:
		env := GetEnvFromContext(ctx)
		// A generator is resumed directly, so the StopIteration that ends it
		// keeps the generator's return value.
		if len(args) == 1 && o.Class == generatorClass {
			return generatorNext(ctx, o, env)
		}
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

// sumGeneric is sum(iterable, start) for values that are not all numbers:
// start + e0 + e1 + ..., with the + operator's rules.
func sumGeneric(ctx context.Context, start, iterable object.Object) object.Object {
	env := GetEnvFromContext(ctx)
	elements, ok, rerr := iterableToSliceCheckedFn(ctx, iterable, env)
	if rerr != nil {
		return rerr
	}
	if !ok {
		return errors.NewTypeError("iterable", iterable.Type().String())
	}
	acc := start
	for _, elem := range elements {
		acc = evalInfixExpression(ctx, ast.OpAdd, acc, elem, env)
		if propagates(acc) {
			return acc
		}
	}
	return acc
}

// callableSentinelIterator is iter(callable, sentinel): an iterator calling
// fn with no arguments until it returns a value equal to sentinel.
func callableSentinelIterator(ctx context.Context, fn, sentinel object.Object) object.Object {
	if !isCallableObject(fn) {
		return &object.Exception{Message: "iter(v, w): v must be callable", ExceptionType: object.ExceptionTypeTypeError, Raised: true}
	}
	env := GetEnvFromContext(ctx)
	done := false
	return object.NewIterator(func() (object.Object, bool) {
		if done {
			return nil, false
		}
		v := applyFunctionWithContext(ctx, fn, nil, object.Kwargs{}, env)
		if propagates(v) {
			done = true
			return v, true
		}
		eq, errObj := evalObjectsEqualChecked(ctx, v, sentinel, env)
		if errObj != nil {
			done = true
			return errObj, true
		}
		if eq {
			done = true
			return nil, false
		}
		return v, true
	}).SetTypeName(iterTypeNameCallable)
}

func iterFunctionImpl(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	if err := errors.RangeArgs(args, 1, 2); err != nil {
		return err
	}
	if len(args) == 2 {
		return callableSentinelIterator(ctx, args[0], args[1])
	}
	switch o := args[0].(type) {
	case *object.Iterator:
		return o.Iter()
	case *object.List, *object.Tuple, *object.String, *object.Set, *object.Dict,
		*object.DictKeys, *object.DictValues, *object.DictItems, *object.Bytes:
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
		if o.Class == generatorClass {
			return o // iter(g) is g
		}
		env := GetEnvFromContext(ctx)
		if fn, ok := findIterMethod(o); ok {
			iter, errObj := callIter(ctx, o, fn, env)
			if errObj != nil {
				return errObj
			}
			return iter
		}
		if _, ok := findDunderMethod(o, "__next__"); ok {
			return instanceToIterator(ctx, o, env)
		}
		return errors.NewTypeErrorTagged("'%s' object is not iterable", getTypeName(args[0]))
	default:
		return errors.NewTypeErrorTagged("'%s' object is not iterable", getTypeName(args[0]))
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
			for _, p := range o.All() {
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
	object.TrackSlice(elems)
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
				return errors.NewImportErrorTagged(errors.ErrImportError)
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
		return errors.NewTypeErrorTagged("%s expected at least 1 argument, got 0", name)
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
			return errors.NewTypeErrorTagged("Cannot specify a default for %s() with multiple arguments", name)
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
					return errors.NewValueError("%s() arg is an empty sequence", name)
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
				return errors.NewValueError("%s() arg is an empty sequence", name)
			}
			args = elements
		} else {
			return errors.NewTypeErrorTagged("'%s' object is not iterable", getTypeName(args[0]))
		}
	}

	// keyFor computes an element's comparison value, calling keyFunc when set.
	env := GetEnvFromContext(ctx)
	keyFor := func(elem object.Object) (object.Object, object.Object) {
		if keyFunc == nil {
			return elem, nil
		}
		key := applyFunctionWithContext(ctx, keyFunc, []object.Object{elem}, object.Kwargs{}, env)
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

// asciiDecimalDigits replaces the Unicode decimal digits in s (Arabic-Indic,
// Devanagari, fullwidth, ...) with ASCII ones, as int() and float() accept
// them; ASCII text is returned unchanged.
func asciiDecimalDigits(s string) string {
	ascii := true
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			ascii = false
			break
		}
	}
	if ascii {
		return s
	}
	var b strings.Builder
	for _, r := range s {
		if r >= utf8.RuneSelf && unicode.Is(unicode.Nd, r) {
			// Decimal digits come in runs of ten starting at zero.
			start := r
			for unicode.Is(unicode.Nd, start-1) {
				start--
			}
			r = '0' + (r-start)%10
		}
		b.WriteRune(r)
	}
	return b.String()
}

// parseIntLiteral converts a string to an int as Python's int(s, base) does:
// surrounding whitespace, a sign, a prefix matching the base (any prefix for
// base 0) and single underscores between digits are allowed. Anything else
// is a ValueError naming the original string.
func parseIntLiteral(text string, base int) object.Object {
	invalid := func() object.Object {
		return errors.NewValueError("invalid literal for int() with base %d: %s", base, object.ReprString(text))
	}
	s := strings.TrimSpace(asciiDecimalDigits(text))
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
	s := strings.TrimSpace(asciiDecimalDigits(text))
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

// boolToInt64 is Python's int(bool): True is 1, False is 0.
func boolToInt64(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

// roundIntDigits rounds n to a multiple of 10**digits (digits in 1..18),
// exact ties going to the even multiple, as Python's round(int, -digits).
func roundIntDigits(n int64, digits int) object.Object {
	p := int64(1)
	for range digits {
		p *= 10
	}
	q, r := n/p, n%p
	if r < 0 {
		q--
		r += p
	}
	if 2*uint64(r) > uint64(p) || (2*uint64(r) == uint64(p) && q%2 != 0) {
		q++
	}
	res, ok := mulOv(q, p)
	if !ok {
		return intOverflowError("round")
	}
	return object.NewInteger(res)
}
