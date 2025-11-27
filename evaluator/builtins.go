package evaluator

import (
	"context"
	"fmt"
	"strings"

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
	},
	"type": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			return &object.String{Value: args[0].Type().String()}
		},
	},
	"str": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			return &object.String{Value: args[0].Inspect()}
		},
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
	},
}

var importCallback func(string) error

func SetImportCallback(fn func(string) error) {
	importCallback = fn
}

func GetImportCallback() func(string) error {
	return importCallback
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
