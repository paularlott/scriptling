package evaluator

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/paularlott/scriptling/ast"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

var builtins = map[string]*object.Builtin{
	"print": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			env := getEnvFromContext(ctx)
			writer := env.GetWriter()
			for _, arg := range args {
				fmt.Fprintln(writer, arg.Inspect())
			}
			return NULL
		},
		HelpText: `print(*args) - Print values to output

Prints each argument on a separate line.`,
	},
	"len": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			switch arg := args[0].(type) {
			case *object.String:
				return object.NewInteger(int64(len(arg.Value)))
			case *object.List:
				return object.NewInteger(int64(len(arg.Elements)))
			case *object.Dict:
				return object.NewInteger(int64(len(arg.Pairs)))
			case *object.Tuple:
				return object.NewInteger(int64(len(arg.Elements)))
			default:
				return errors.NewTypeError("STRING, LIST, DICT, or TUPLE", args[0].Type().String())
			}
		},
		HelpText: `len(obj) - Return the length of an object

Returns the number of items in a string, list, dict, or tuple.`,
	},
	"type": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			return &object.String{Value: args[0].Type().String()}
		},
		HelpText: `type(obj) - Return the type of an object

Returns a string representing the type of the object.`,
	},
	"str": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			return &object.String{Value: args[0].Inspect()}
		},
		HelpText: `str(obj) - Convert an object to a string

Returns the string representation of any object.`,
	},
	"int": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
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
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
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
	"append": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 2 {
				return errors.NewArgumentError(len(args), 2)
			}
			if args[0].Type() != object.LIST_OBJ {
				return errors.NewTypeError("LIST", args[0].Type().String())
			}
			list := args[0].(*object.List)
			// Modify list in-place (Python behavior)
			list.Elements = append(list.Elements, args[1])
			return &object.Null{}
		},
		HelpText: `append(list, item) - Append item to list

Modifies the list in place by adding item to the end.
Returns null.`,
	},
	"extend": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 2 {
				return errors.NewArgumentError(len(args), 2)
			}
			if args[0].Type() != object.LIST_OBJ {
				return errors.NewTypeError("LIST", args[0].Type().String())
			}
			if args[1].Type() != object.LIST_OBJ {
				return errors.NewTypeError("LIST", args[1].Type().String())
			}
			list := args[0].(*object.List)
			other := args[1].(*object.List)
			// Modify list in-place by appending all elements from other list
			list.Elements = append(list.Elements, other.Elements...)
			return &object.Null{}
		},
		HelpText: `extend(list, other_list) - Extend list with elements from other_list

Modifies the first list in place by appending all elements from the second list.
Returns null.`,
	},
	"split": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 2 {
				return errors.NewArgumentError(len(args), 2)
			}
			if args[0].Type() != object.STRING_OBJ || args[1].Type() != object.STRING_OBJ {
				return errors.NewTypeError("STRING", "mixed types")
			}
			str := args[0].(*object.String).Value
			sep := args[1].(*object.String).Value
			parts := strings.Split(str, sep)
			elements := make([]object.Object, len(parts))
			for i, part := range parts {
				elements[i] = &object.String{Value: part}
			}
			return &object.List{Elements: elements}
		},
		HelpText: `split(str, sep) - Split string by separator

Splits the string into a list of substrings using sep as the delimiter.
Returns a list of strings.`,
	},
	"join": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 2 {
				return errors.NewArgumentError(len(args), 2)
			}
			if args[0].Type() != object.LIST_OBJ || args[1].Type() != object.STRING_OBJ {
				return errors.NewTypeError("LIST and STRING", "mixed types")
			}
			list := args[0].(*object.List)
			sep := args[1].(*object.String).Value
			if len(list.Elements) == 0 {
				return &object.String{Value: ""}
			}
			if len(list.Elements) == 1 {
				return &object.String{Value: list.Elements[0].Inspect()}
			}
			var buf strings.Builder
			buf.WriteString(list.Elements[0].Inspect())
			for i := 1; i < len(list.Elements); i++ {
				buf.WriteString(sep)
				buf.WriteString(list.Elements[i].Inspect())
			}
			return &object.String{Value: buf.String()}
		},
		HelpText: `join(list, sep) - Join list elements with separator

Joins the string representations of list elements using sep as separator.
Returns a string.`,
	},
	"upper": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			if args[0].Type() != object.STRING_OBJ {
				return errors.NewTypeError("STRING", args[0].Type().String())
			}
			str := args[0].(*object.String).Value
			return &object.String{Value: strings.ToUpper(str)}
		},
		HelpText: `upper(str) - Convert string to uppercase

Returns a new string with all characters converted to uppercase.`,
	},
	"lower": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			if args[0].Type() != object.STRING_OBJ {
				return errors.NewTypeError("STRING", args[0].Type().String())
			}
			str := args[0].(*object.String).Value
			return &object.String{Value: strings.ToLower(str)}
		},
		HelpText: `lower(str) - Convert string to lowercase

Returns a new string with all characters converted to lowercase.`,
	},
	"replace": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 3 {
				return errors.NewArgumentError(len(args), 3)
			}
			if args[0].Type() != object.STRING_OBJ || args[1].Type() != object.STRING_OBJ || args[2].Type() != object.STRING_OBJ {
				return errors.NewTypeError("STRING", "mixed types")
			}
			str := args[0].(*object.String).Value
			old := args[1].(*object.String).Value
			new := args[2].(*object.String).Value
			result := strings.Replace(str, old, new, -1)
			return &object.String{Value: result}
		},
		HelpText: `replace(str, old, new) - Replace occurrences in string

Replaces all occurrences of old substring with new substring in str.
Returns a new string.`,
	},
	"capitalize": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			if args[0].Type() != object.STRING_OBJ {
				return errors.NewTypeError("STRING", args[0].Type().String())
			}
			str := args[0].(*object.String).Value
			if len(str) == 0 {
				return &object.String{Value: str}
			}
			// Capitalize first letter, lowercase the rest
			result := strings.ToUpper(string(str[0])) + strings.ToLower(str[1:])
			return &object.String{Value: result}
		},
		HelpText: `capitalize(str) - Capitalize first character

Returns a new string with the first character capitalized and the rest lowercase.`,
	},
	"title": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			if args[0].Type() != object.STRING_OBJ {
				return errors.NewTypeError("STRING", args[0].Type().String())
			}
			str := args[0].(*object.String).Value
			result := strings.Title(strings.ToLower(str))
			return &object.String{Value: result}
		},
		HelpText: `title(str) - Convert to title case

Returns a new string with the first letter of each word capitalized.`,
	},
	"strip": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			if args[0].Type() != object.STRING_OBJ {
				return errors.NewTypeError("STRING", args[0].Type().String())
			}
			str := args[0].(*object.String).Value
			result := strings.TrimSpace(str)
			return &object.String{Value: result}
		},
		HelpText: `strip(str) - Remove leading and trailing whitespace

Returns a new string with leading and trailing whitespace removed.`,
	},
	"lstrip": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			if args[0].Type() != object.STRING_OBJ {
				return errors.NewTypeError("STRING", args[0].Type().String())
			}
			str := args[0].(*object.String).Value
			result := strings.TrimLeft(str, " \t\n\r")
			return &object.String{Value: result}
		},
		HelpText: `lstrip(str) - Remove leading whitespace

Returns a new string with leading whitespace removed.`,
	},
	"rstrip": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			if args[0].Type() != object.STRING_OBJ {
				return errors.NewTypeError("STRING", args[0].Type().String())
			}
			str := args[0].(*object.String).Value
			result := strings.TrimRight(str, " \t\n\r")
			return &object.String{Value: result}
		},
		HelpText: `rstrip(str) - Remove trailing whitespace

Returns a new string with trailing whitespace removed.`,
	},
	"startswith": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 2 {
				return errors.NewArgumentError(len(args), 2)
			}
			if args[0].Type() != object.STRING_OBJ || args[1].Type() != object.STRING_OBJ {
				return errors.NewTypeError("STRING", "mixed types")
			}
			str := args[0].(*object.String).Value
			prefix := args[1].(*object.String).Value
			if strings.HasPrefix(str, prefix) {
				return TRUE
			}
			return FALSE
		},
		HelpText: `startswith(str, prefix) - Check if string starts with prefix

Returns true if str starts with prefix, false otherwise.`,
	},
	"endswith": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 2 {
				return errors.NewArgumentError(len(args), 2)
			}
			if args[0].Type() != object.STRING_OBJ {
				return errors.NewTypeError("STRING", args[0].Type().String())
			}
			if args[1].Type() != object.STRING_OBJ {
				return errors.NewTypeError("STRING", args[1].Type().String())
			}
			s := args[0].(*object.String).Value
			suffix := args[1].(*object.String).Value
			return nativeBoolToBooleanObject(strings.HasSuffix(s, suffix))
		},
		HelpText: `endswith(str, suffix) - Check if string ends with suffix

Returns true if str ends with suffix, false otherwise.`,
	},
	"sum": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
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
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) < 1 || len(args) > 2 {
				return errors.NewArgumentError(len(args), 1)
			}

			var elements []object.Object
			switch arg := args[0].(type) {
			case *object.List:
				// Make a copy
				elements = make([]object.Object, len(arg.Elements))
				copy(elements, arg.Elements)
			case *object.Tuple:
				elements = make([]object.Object, len(arg.Elements))
				copy(elements, arg.Elements)
			default:
				return errors.NewTypeError("LIST or TUPLE", args[0].Type().String())
			}

			// Check for key function
			var keyFunc *object.Builtin
			if len(args) == 2 {
				var ok bool
				keyFunc, ok = args[1].(*object.Builtin)
				if !ok {
					return errors.NewError("sorted() key parameter must be a builtin function")
				}
			}

			// Simple bubble sort (good enough for now)
			n := len(elements)
			for i := 0; i < n-1; i++ {
				for j := 0; j < n-i-1; j++ {
					var cmp bool

					// Get comparison values
					left := elements[j]
					right := elements[j+1]

					// Apply key function if provided
					if keyFunc != nil {
						leftKey := keyFunc.Fn(ctx, left)
						if isError(leftKey) || isException(leftKey) {
							return leftKey
						}
						rightKey := keyFunc.Fn(ctx, right)
						if isError(rightKey) || isException(rightKey) {
							return rightKey
						}
						left = leftKey
						right = rightKey
					}

					// Compare based on type
					switch l := left.(type) {
					case *object.Integer:
						if r, ok := right.(*object.Integer); ok {
							cmp = l.Value > r.Value
						} else if r, ok := right.(*object.Float); ok {
							cmp = float64(l.Value) > r.Value
						} else {
							return errors.NewError("cannot compare %s with %s", left.Type(), right.Type())
						}
					case *object.Float:
						if r, ok := right.(*object.Float); ok {
							cmp = l.Value > r.Value
						} else if r, ok := right.(*object.Integer); ok {
							cmp = l.Value > float64(r.Value)
						} else {
							return errors.NewError("cannot compare %s with %s", left.Type(), right.Type())
						}
					case *object.String:
						if r, ok := right.(*object.String); ok {
							cmp = l.Value > r.Value
						} else {
							return errors.NewError("cannot compare %s with %s", left.Type(), right.Type())
						}
					default:
						return errors.NewError("unsupported type for sorting: %s", left.Type())
					}

					if cmp {
						elements[j], elements[j+1] = elements[j+1], elements[j]
					}
				}
			}

			return &object.List{Elements: elements}
		},
		HelpText: `sorted(iterable[, key]) - Return sorted list

Returns a new sorted list from the elements of iterable.
Optional key function can be provided for custom sorting.`,
	},
	"range": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) < 1 || len(args) > 3 {
				return errors.NewArgumentError(len(args), 1)
			}
			var start, stop, step int64 = 0, 0, 1
			if len(args) == 1 {
				if args[0].Type() != object.INTEGER_OBJ {
					return errors.NewTypeError("INTEGER", args[0].Type().String())
				}
				stop = args[0].(*object.Integer).Value
			} else if len(args) == 2 {
				if args[0].Type() != object.INTEGER_OBJ || args[1].Type() != object.INTEGER_OBJ {
					return errors.NewTypeError("INTEGER", "mixed types")
				}
				start = args[0].(*object.Integer).Value
				stop = args[1].(*object.Integer).Value
			} else {
				if args[0].Type() != object.INTEGER_OBJ || args[1].Type() != object.INTEGER_OBJ || args[2].Type() != object.INTEGER_OBJ {
					return errors.NewTypeError("INTEGER", "mixed types")
				}
				start = args[0].(*object.Integer).Value
				stop = args[1].(*object.Integer).Value
				step = args[2].(*object.Integer).Value
				if step == 0 {
					return errors.NewError("range step cannot be zero")
				}
			}
			elements := []object.Object{}
			if step > 0 {
				for i := start; i < stop; i += step {
					elements = append(elements, object.NewInteger(i))
				}
			} else {
				for i := start; i > stop; i += step {
					elements = append(elements, object.NewInteger(i))
				}
			}
			return &object.List{Elements: elements}
		},
		HelpText: `range([start,] stop[, step]) - Generate sequence of numbers

Returns a list of integers from start (inclusive) to stop (exclusive).
If start is omitted, defaults to 0. If step is omitted, defaults to 1.`,
	},
	"keys": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			if args[0].Type() != object.DICT_OBJ {
				return errors.NewTypeError("DICT", args[0].Type().String())
			}
			dict := args[0].(*object.Dict)
			elements := make([]object.Object, 0, len(dict.Pairs))
			for _, pair := range dict.Pairs {
				elements = append(elements, pair.Key)
			}
			return &object.List{Elements: elements}
		},
		HelpText: `keys(dict) - Return dictionary keys

Returns a list of all keys in the dictionary.`,
	},
	"values": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			if args[0].Type() != object.DICT_OBJ {
				return errors.NewTypeError("DICT", args[0].Type().String())
			}
			dict := args[0].(*object.Dict)
			elements := make([]object.Object, 0, len(dict.Pairs))
			for _, pair := range dict.Pairs {
				elements = append(elements, pair.Value)
			}
			return &object.List{Elements: elements}
		},
		HelpText: `values(dict) - Return dictionary values

Returns a list of all values in the dictionary.`,
	},
	"items": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			if args[0].Type() != object.DICT_OBJ {
				return errors.NewTypeError("DICT", args[0].Type().String())
			}
			dict := args[0].(*object.Dict)
			elements := make([]object.Object, 0, len(dict.Pairs))
			for _, pair := range dict.Pairs {
				tupleElements := []object.Object{pair.Key, pair.Value}
				elements = append(elements, &object.List{Elements: tupleElements})
			}
			return &object.List{Elements: elements}
		},
		HelpText: `items(dict) - Return dictionary key-value pairs

Returns a list of [key, value] pairs for all items in the dictionary.`,
	},
}

func init() {
	builtins["help"] = &object.Builtin{
		Fn: helpFunction,
		HelpText: `help([object]) - Display help information

  With no arguments: Show general help
  help("modules"): List all available libraries
  help("builtins"): List all builtin functions
  help("operators"): List all operators
  help(function): Show help for a function object
  help("function_name"): Show help for a builtin function
  help("library.function"): Show help for a library function
  help("library_name"): List functions in a library`,
	}
}

func helpFunction(ctx context.Context, args ...object.Object) object.Object {
	env := getEnvFromContext(ctx)
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

	if len(args) != 1 {
		return errors.NewArgumentError(len(args), 1)
	}

	// Handle string arguments
	if strObj, ok := args[0].(*object.String); ok {
		topic := strObj.Value

		// Special topic: modules
		if topic == "modules" {
			fmt.Fprintln(writer, "Available Libraries (use 'import <name>'):")
			fmt.Fprintln(writer, "")

			// Use callback if available to get all libraries
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
			parts := strings.SplitN(topic, ".", 2)
			libName := parts[0]
			funcName := parts[1]

			// Try to get the library from environment
			if libObj, ok := env.Get(libName); ok {
				if dict, ok := libObj.(*object.Dict); ok {
					if pair, ok := dict.Pairs[funcName]; ok {
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
				if docPair, ok := dict.Pairs["__doc__"]; ok {
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
				for name := range dict.Pairs {
					// Skip __doc__
					if name == "__doc__" {
						continue
					}
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
		for name := range obj.Pairs {
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

var importCallback func(string) error

func SetImportCallback(fn func(string) error) {
	importCallback = fn
}

func GetImportCallback() func(string) error {
	return importCallback
}

// LibraryInfo contains information about available libraries
type LibraryInfo struct {
	Name       string
	IsStandard bool
	IsImported bool
}

var availableLibrariesCallback func() []LibraryInfo

func SetAvailableLibrariesCallback(fn func() []LibraryInfo) {
	availableLibrariesCallback = fn
}

func GetAvailableLibrariesCallback() func() []LibraryInfo {
	return availableLibrariesCallback
}

// getEnvFromContext retrieves environment from context
func getEnvFromContext(ctx context.Context) *object.Environment {
	if env, ok := ctx.Value(envContextKey).(*object.Environment); ok {
		return env
	}
	return object.NewEnvironment() // fallback
}

func GetImportBuiltin() *object.Builtin {
	return &object.Builtin{
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			if args[0].Type() != object.STRING_OBJ {
				return errors.NewTypeError("STRING", args[0].Type().String())
			}
			if importCallback == nil {
				return errors.NewError(errors.ErrImportError)
			}
			libName := args[0].(*object.String).Value
			err := importCallback(libName)
			if err != nil {
				return errors.NewError("%s: %s", errors.ErrImportError, err.Error())
			}
			return &object.Null{}
		},
	}
}
