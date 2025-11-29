package evaluator

import (
	"context"
	"fmt"
	"io"
	"math"
	"regexp"
	"sort"
	"strings"

	"github.com/paularlott/scriptling/ast"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

var regexCache = make(map[string]*regexp.Regexp)

func getCompiledRegex(pattern string) (*regexp.Regexp, error) {
	if re, ok := regexCache[pattern]; ok {
		return re, nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	regexCache[pattern] = re
	return re, nil
}

var builtins = map[string]*object.Builtin{
	"print": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			env := getEnvFromContext(ctx)
			writer := env.GetWriter()

			// Get sep kwarg (default: " ")
			sep := " "
			if sepObj, ok := kwargs["sep"]; ok {
				if sepStr, ok := sepObj.(*object.String); ok {
					sep = sepStr.Value
				} else if _, ok := sepObj.(*object.Null); !ok {
					return errors.NewError("sep must be None or a string, not %s", sepObj.Type())
				}
			}

			// Get end kwarg (default: "\n")
			end := "\n"
			if endObj, ok := kwargs["end"]; ok {
				if endStr, ok := endObj.(*object.String); ok {
					end = endStr.Value
				} else if _, ok := endObj.(*object.Null); !ok {
					return errors.NewError("end must be None or a string, not %s", endObj.Type())
				}
			}

			// Build output string
			parts := make([]string, len(args))
			for i, arg := range args {
				parts[i] = arg.Inspect()
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
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
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
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
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
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			return &object.String{Value: args[0].Inspect()}
		},
		HelpText: `str(obj) - Convert an object to a string

Returns the string representation of any object.`,
	},
	"int": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
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
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
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
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
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
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
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
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
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
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
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
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
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
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
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
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
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
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
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
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
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
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
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
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
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
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
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
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
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
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
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
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
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
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
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

			// Check for reverse kwarg
			reverse := false
			if kwargs != nil {
				if rev, ok := kwargs["reverse"]; ok {
					if b, ok := rev.(*object.Boolean); ok {
						reverse = b.Value
					}
				}
			}

			// Use efficient O(n log n) sort
			n := len(elements)
			if n > 1 {
				// Pre-compute keys if key function is provided
				var keys []object.Object
				var sortErr object.Object
				if keyFunc != nil {
					keys = make([]object.Object, n)
					for i, elem := range elements {
						key := keyFunc.Fn(ctx, nil, elem)
						if isError(key) || isException(key) {
							return key
						}
						keys[i] = key
					}
				}

				// Create index array to track positions
				indices := make([]int, n)
				for i := range indices {
					indices[i] = i
				}

				// Sort indices by values
				sort.Slice(indices, func(i, j int) bool {
					var left, right object.Object
					if keys != nil {
						left, right = keys[indices[i]], keys[indices[j]]
					} else {
						left, right = elements[indices[i]], elements[indices[j]]
					}

					// Compare based on type
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

				// Reorder elements according to sorted indices
				newElements := make([]object.Object, n)
				for i, idx := range indices {
					newElements[i] = elements[idx]
				}
				elements = newElements
			}

			return &object.List{Elements: elements}
		},
		HelpText: `sorted(iterable[, key][, reverse=False]) - Return sorted list

Returns a new sorted list from the elements of iterable.
Optional key function can be provided for custom sorting.
Set reverse=True to sort in descending order.`,
	},
	"range": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
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
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
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
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
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
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
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
	"enumerate": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) < 1 || len(args) > 2 {
				return errors.NewError("enumerate() takes 1 or 2 arguments (%d given)", len(args))
			}
			start := int64(0)
			if len(args) == 2 {
				if startObj, ok := args[1].(*object.Integer); ok {
					start = startObj.Value
				} else {
					return errors.NewTypeError("INTEGER", args[1].Type().String())
				}
			}
			var iterable []object.Object
			switch iter := args[0].(type) {
			case *object.List:
				iterable = iter.Elements
			case *object.Tuple:
				iterable = iter.Elements
			case *object.String:
				for _, ch := range iter.Value {
					iterable = append(iterable, &object.String{Value: string(ch)})
				}
			default:
				return errors.NewTypeError("iterable (LIST, TUPLE, STRING)", args[0].Type().String())
			}
			result := make([]object.Object, len(iterable))
			for i, elem := range iterable {
				result[i] = &object.Tuple{Elements: []object.Object{
					object.NewInteger(start + int64(i)),
					elem,
				}}
			}
			return &object.List{Elements: result}
		},
		HelpText: `enumerate(iterable[, start=0]) - Return (index, value) pairs

Returns a list of tuples containing the index and value for each item.
Default start is 0.`,
	},
	"zip": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) == 0 {
				return &object.List{Elements: []object.Object{}}
			}
			// Get all iterables as slices
			iterables := make([][]object.Object, len(args))
			minLen := -1
			for i, arg := range args {
				switch iter := arg.(type) {
				case *object.List:
					iterables[i] = iter.Elements
				case *object.Tuple:
					iterables[i] = iter.Elements
				case *object.String:
					strElements := make([]object.Object, len(iter.Value))
					for j, ch := range iter.Value {
						strElements[j] = &object.String{Value: string(ch)}
					}
					iterables[i] = strElements
				default:
					return errors.NewTypeError("iterable (LIST, TUPLE, STRING)", arg.Type().String())
				}
				if minLen == -1 || len(iterables[i]) < minLen {
					minLen = len(iterables[i])
				}
			}
			result := make([]object.Object, minLen)
			for i := 0; i < minLen; i++ {
				tuple := make([]object.Object, len(iterables))
				for j := range iterables {
					tuple[j] = iterables[j][i]
				}
				result[i] = &object.Tuple{Elements: tuple}
			}
			return &object.List{Elements: result}
		},
		HelpText: `zip(*iterables) - Aggregate elements from each iterable

Returns a list of tuples where the i-th tuple contains the i-th element
from each of the argument iterables. Stops at the shortest iterable.`,
	},
	"any": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
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
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
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
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) == 0 {
				return FALSE
			}
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
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
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
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
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
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
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
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
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) < 1 || len(args) > 2 {
				return errors.NewError("round() takes 1 or 2 arguments (%d given)", len(args))
			}
			ndigits := 0
			if len(args) == 2 {
				if nd, ok := args[1].(*object.Integer); ok {
					ndigits = int(nd.Value)
				} else {
					return errors.NewTypeError("INTEGER", args[1].Type().String())
				}
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
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			if num, ok := args[0].(*object.Integer); ok {
				if num.Value >= 0 {
					return &object.String{Value: fmt.Sprintf("0x%x", num.Value)}
				}
				return &object.String{Value: fmt.Sprintf("-0x%x", -num.Value)}
			}
			return errors.NewTypeError("INTEGER", args[0].Type().String())
		},
		HelpText: `hex(x) - Convert an integer to a lowercase hexadecimal string prefixed with "0x"`,
	},
	"bin": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			if num, ok := args[0].(*object.Integer); ok {
				if num.Value >= 0 {
					return &object.String{Value: fmt.Sprintf("0b%b", num.Value)}
				}
				return &object.String{Value: fmt.Sprintf("-0b%b", -num.Value)}
			}
			return errors.NewTypeError("INTEGER", args[0].Type().String())
		},
		HelpText: `bin(x) - Convert an integer to a binary string prefixed with "0b"`,
	},
	"oct": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			if num, ok := args[0].(*object.Integer); ok {
				if num.Value >= 0 {
					return &object.String{Value: fmt.Sprintf("0o%o", num.Value)}
				}
				return &object.String{Value: fmt.Sprintf("-0o%o", -num.Value)}
			}
			return errors.NewTypeError("INTEGER", args[0].Type().String())
		},
		HelpText: `oct(x) - Convert an integer to an octal string prefixed with "0o"`,
	},
	"pow": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
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
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 2 {
				return errors.NewArgumentError(len(args), 2)
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
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
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
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 2 {
				return errors.NewArgumentError(len(args), 2)
			}
			typeName, ok := args[1].(*object.String)
			if !ok {
				return errors.NewTypeError("STRING", args[1].Type().String())
			}
			objType := args[0].Type().String()
			// Support common Python type names
			checkType := strings.ToUpper(typeName.Value)
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
		HelpText: `isinstance(object, classname) - Return True if object is of the given type

Type names: "int", "str", "float", "bool", "list", "dict", "tuple", "function", "None"`,
	},
	"chr": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			if num, ok := args[0].(*object.Integer); ok {
				if num.Value < 0 || num.Value > 0x10FFFF {
					return errors.NewError("chr() arg not in range(0x110000)")
				}
				return &object.String{Value: string(rune(num.Value))}
			}
			return errors.NewTypeError("INTEGER", args[0].Type().String())
		},
		HelpText: `chr(i) - Return a string of one character from Unicode code point

The argument must be in the range 0-1114111 (0x10FFFF).`,
	},
	"ord": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			if str, ok := args[0].(*object.String); ok {
				runes := []rune(str.Value)
				if len(runes) != 1 {
					return errors.NewError("ord() expected a character, but string of length %d found", len(runes))
				}
				return object.NewInteger(int64(runes[0]))
			}
			return errors.NewTypeError("STRING", args[0].Type().String())
		},
		HelpText: `ord(c) - Return Unicode code point for a one-character string

The argument must be a string of exactly one character.`,
	},
	"reversed": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			switch iter := args[0].(type) {
			case *object.List:
				result := make([]object.Object, len(iter.Elements))
				for i, elem := range iter.Elements {
					result[len(iter.Elements)-1-i] = elem
				}
				return &object.List{Elements: result}
			case *object.Tuple:
				result := make([]object.Object, len(iter.Elements))
				for i, elem := range iter.Elements {
					result[len(iter.Elements)-1-i] = elem
				}
				return &object.Tuple{Elements: result}
			case *object.String:
				runes := []rune(iter.Value)
				for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
					runes[i], runes[j] = runes[j], runes[i]
				}
				return &object.String{Value: string(runes)}
			default:
				return errors.NewTypeError("sequence (LIST, TUPLE, STRING)", args[0].Type().String())
			}
		},
		HelpText: `reversed(seq) - Return a reversed version of the sequence

Works with lists, tuples, and strings.`,
	},
	"list": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) == 0 {
				return &object.List{Elements: []object.Object{}}
			}
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			switch iter := args[0].(type) {
			case *object.List:
				// Return a copy
				elements := make([]object.Object, len(iter.Elements))
				copy(elements, iter.Elements)
				return &object.List{Elements: elements}
			case *object.Tuple:
				elements := make([]object.Object, len(iter.Elements))
				copy(elements, iter.Elements)
				return &object.List{Elements: elements}
			case *object.String:
				elements := make([]object.Object, 0, len(iter.Value))
				for _, ch := range iter.Value {
					elements = append(elements, &object.String{Value: string(ch)})
				}
				return &object.List{Elements: elements}
			case *object.Dict:
				elements := make([]object.Object, 0, len(iter.Pairs))
				for _, pair := range iter.Pairs {
					elements = append(elements, pair.Key)
				}
				return &object.List{Elements: elements}
			default:
				return errors.NewTypeError("iterable (LIST, TUPLE, STRING, DICT)", args[0].Type().String())
			}
		},
		HelpText: `list([iterable]) - Create a list from an iterable

With no argument, returns an empty list.
Otherwise, returns a list containing the items of the iterable.`,
	},
	"dict": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			result := &object.Dict{Pairs: make(map[string]object.DictPair)}
			// Handle kwargs
			for key, val := range kwargs {
				result.Pairs[key] = object.DictPair{
					Key:   &object.String{Value: key},
					Value: val,
				}
			}
			if len(args) == 0 {
				return result
			}
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
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
					result.Pairs[pair[0].Inspect()] = object.DictPair{Key: pair[0], Value: pair[1]}
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
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) == 0 {
				return &object.Tuple{Elements: []object.Object{}}
			}
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			switch iter := args[0].(type) {
			case *object.Tuple:
				return iter
			case *object.List:
				elements := make([]object.Object, len(iter.Elements))
				copy(elements, iter.Elements)
				return &object.Tuple{Elements: elements}
			case *object.String:
				elements := make([]object.Object, 0, len(iter.Value))
				for _, ch := range iter.Value {
					elements = append(elements, &object.String{Value: string(ch)})
				}
				return &object.Tuple{Elements: elements}
			default:
				return errors.NewTypeError("iterable (LIST, TUPLE, STRING)", args[0].Type().String())
			}
		},
		HelpText: `tuple([iterable]) - Create a tuple from an iterable

With no argument, returns an empty tuple.
Otherwise, returns a tuple containing the items of the iterable.`,
	},
	"set": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) == 0 {
				return &object.List{Elements: []object.Object{}}
			}
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}

			// Get elements from iterable
			var elements []object.Object
			switch iter := args[0].(type) {
			case *object.List:
				elements = iter.Elements
			case *object.Tuple:
				elements = iter.Elements
			case *object.String:
				for _, ch := range iter.Value {
					elements = append(elements, &object.String{Value: string(ch)})
				}
			default:
				return errors.NewTypeError("iterable (LIST, TUPLE, STRING)", args[0].Type().String())
			}

			// Remove duplicates (using string representation as key)
			seen := make(map[string]bool)
			unique := []object.Object{}
			for _, elem := range elements {
				key := elem.Inspect()
				if !seen[key] {
					seen[key] = true
					unique = append(unique, elem)
				}
			}
			return &object.List{Elements: unique}
		},
		HelpText: `set([iterable]) - Create a list of unique elements from an iterable

With no argument, returns an empty list.
Otherwise, returns a list containing unique items from the iterable.
Note: In Scriptling, set() returns a List since there is no Set type.`,
	},
	"input": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			// input() is not supported in embedded environments
			// In a scripting engine context, there's no stdin
			return errors.NewError("input() is not available in embedded scripting environments")
		},
		HelpText: `input([prompt]) - Read a line of input

Note: input() is not available in embedded scripting environments.
This function exists for compatibility but will return an error.`,
	},
	"repr": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			switch obj := args[0].(type) {
			case *object.String:
				// Add quotes around strings
				return &object.String{Value: fmt.Sprintf("'%s'", obj.Value)}
			default:
				return &object.String{Value: obj.Inspect()}
			}
		},
		HelpText: `repr(object) - Return a string representation

For strings, returns the string with quotes.
For other objects, returns the same as str().`,
	},
	"hash": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			// Simple hash based on string representation
			str := args[0].Inspect()
			var h int64 = 0
			for _, c := range str {
				h = h*31 + int64(c)
			}
			return object.NewInteger(h)
		},
		HelpText: `hash(object) - Return the hash value of an object

Returns an integer hash value for the object.`,
	},
	"id": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
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
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) < 1 || len(args) > 2 {
				return errors.NewError("format() takes 1 or 2 arguments (%d given)", len(args))
			}
			value := args[0]
			formatSpec := ""
			if len(args) == 2 {
				if spec, ok := args[1].(*object.String); ok {
					formatSpec = spec.Value
				} else {
					return errors.NewTypeError("STRING", args[1].Type().String())
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
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 2 {
				return errors.NewArgumentError(len(args), 2)
			}
			name, ok := args[1].(*object.String)
			if !ok {
				return errors.NewTypeError("STRING", args[1].Type().String())
			}
			// Check if object has the attribute/method
			switch obj := args[0].(type) {
			case *object.Dict:
				_, exists := obj.Pairs[name.Value]
				return nativeBoolToBooleanObject(exists)
			default:
				// For other objects, check if it's a known method
				return FALSE
			}
		},
		HelpText: `hasattr(object, name) - Check if object has an attribute

Returns True if the object has the named attribute.`,
	},
	"getattr": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) < 2 || len(args) > 3 {
				return errors.NewError("getattr() takes 2 or 3 arguments (%d given)", len(args))
			}
			name, ok := args[1].(*object.String)
			if !ok {
				return errors.NewTypeError("STRING", args[1].Type().String())
			}
			// Get attribute from object
			switch obj := args[0].(type) {
			case *object.Dict:
				if pair, exists := obj.Pairs[name.Value]; exists {
					return pair.Value
				}
			}
			// Return default if provided
			if len(args) == 3 {
				return args[2]
			}
			return errors.NewError("'%s' object has no attribute '%s'", args[0].Type().String(), name.Value)
		},
		HelpText: `getattr(object, name[, default]) - Get an attribute from an object

Returns the value of the named attribute.
If default is provided, returns it when attribute doesn't exist.`,
	},
	"setattr": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 3 {
				return errors.NewArgumentError(len(args), 3)
			}
			name, ok := args[1].(*object.String)
			if !ok {
				return errors.NewTypeError("STRING", args[1].Type().String())
			}
			// Set attribute on object
			switch obj := args[0].(type) {
			case *object.Dict:
				obj.Pairs[name.Value] = object.DictPair{
					Key:   name,
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
	builtins["map"] = &object.Builtin{
		Fn: mapFunction,
		HelpText: `map(function, iterable, ...) - Apply function to every item

Returns a list of results from applying function to each item.
With multiple iterables, function must take that many arguments.`,
	}
	builtins["filter"] = &object.Builtin{
		Fn: filterFunction,
		HelpText: `filter(function, iterable) - Filter elements by function

Returns a list of elements for which function returns true.
If function is None, removes falsy elements.`,
	}
}

func mapFunction(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
	if len(args) < 2 {
		return errors.NewError("map() requires at least 2 arguments")
	}
	fn := args[0]
	// Get all iterables
	iterables := make([][]object.Object, len(args)-1)
	minLen := -1
	for i, arg := range args[1:] {
		switch iter := arg.(type) {
		case *object.List:
			iterables[i] = iter.Elements
		case *object.Tuple:
			iterables[i] = iter.Elements
		default:
			return errors.NewTypeError("iterable (LIST, TUPLE)", arg.Type().String())
		}
		if minLen == -1 || len(iterables[i]) < minLen {
			minLen = len(iterables[i])
		}
	}
	result := make([]object.Object, minLen)
	env := getEnvFromContext(ctx)
	for i := 0; i < minLen; i++ {
		callArgs := make([]object.Object, len(iterables))
		for j := range iterables {
			callArgs[j] = iterables[j][i]
		}
		res := applyFunctionWithContext(ctx, fn, callArgs, nil, env)
		if isError(res) {
			return res
		}
		result[i] = res
	}
	return &object.List{Elements: result}
}

func filterFunction(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
	if len(args) != 2 {
		return errors.NewArgumentError(len(args), 2)
	}
	fn := args[0]
	var iterable []object.Object
	switch iter := args[1].(type) {
	case *object.List:
		iterable = iter.Elements
	case *object.Tuple:
		iterable = iter.Elements
	default:
		return errors.NewTypeError("iterable (LIST, TUPLE)", args[1].Type().String())
	}
	result := []object.Object{}
	env := getEnvFromContext(ctx)
	for _, elem := range iterable {
		// If function is None, use truthiness
		if fn.Type() == object.NULL_OBJ {
			if isTruthy(elem) {
				result = append(result, elem)
			}
		} else {
			res := applyFunctionWithContext(ctx, fn, []object.Object{elem}, nil, env)
			if isError(res) {
				return res
			}
			if isTruthy(res) {
				result = append(result, elem)
			}
		}
	}
	return &object.List{Elements: result}
}

func helpFunction(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
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

// getEnvFromContext retrieves environment from context
func getEnvFromContext(ctx context.Context) *object.Environment {
	if env, ok := ctx.Value(envContextKey).(*object.Environment); ok {
		return env
	}
	return object.NewEnvironment() // fallback
}

func GetImportBuiltin() *object.Builtin {
	return &object.Builtin{
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			if args[0].Type() != object.STRING_OBJ {
				return errors.NewTypeError("STRING", args[0].Type().String())
			}

			env := getEnvFromContext(ctx)
			importCallback := env.GetImportCallback()
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
