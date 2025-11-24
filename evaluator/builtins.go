package evaluator

import (
	"fmt"
	"github.com/paularlott/scriptling/object"
	"strings"
)

var builtins = map[string]*object.Builtin{
	"print": {
		Fn: func(args ...object.Object) object.Object {
			for _, arg := range args {
				fmt.Println(arg.Inspect())
			}
			return NULL
		},
	},
	"len": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("wrong number of arguments. got=%d, want=1", len(args))
			}
			switch arg := args[0].(type) {
			case *object.String:
				return &object.Integer{Value: int64(len(arg.Value))}
			case *object.List:
				return &object.Integer{Value: int64(len(arg.Elements))}
			case *object.Dict:
				return &object.Integer{Value: int64(len(arg.Pairs))}
			default:
				return newError("argument to len not supported, got %s", args[0].Type())
			}
		},
	},
	"str": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("wrong number of arguments. got=%d, want=1", len(args))
			}
			return &object.String{Value: args[0].Inspect()}
		},
	},
	"int": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("wrong number of arguments. got=%d, want=1", len(args))
			}
			switch arg := args[0].(type) {
			case *object.Integer:
				return arg
			case *object.Float:
				return &object.Integer{Value: int64(arg.Value)}
			case *object.String:
				var val int64
				_, err := fmt.Sscanf(arg.Value, "%d", &val)
				if err != nil {
					return newError("cannot convert %s to int", arg.Value)
				}
				return &object.Integer{Value: val}
			default:
				return newError("cannot convert %s to int", arg.Type())
			}
		},
	},
	"float": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("wrong number of arguments. got=%d, want=1", len(args))
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
					return newError("cannot convert %s to float", arg.Value)
				}
				return &object.Float{Value: val}
			default:
				return newError("cannot convert %s to float", arg.Type())
			}
		},
	},
	"append": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 2 {
				return newError("wrong number of arguments. got=%d, want=2", len(args))
			}
			if args[0].Type() != object.LIST_OBJ {
				return newError("argument to append must be LIST, got %s", args[0].Type())
			}
			list := args[0].(*object.List)
			// Modify list in-place (Python behavior)
			list.Elements = append(list.Elements, args[1])
			return &object.Null{}
		},
	},
	"split": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 2 {
				return newError("wrong number of arguments. got=%d, want=2", len(args))
			}
			if args[0].Type() != object.STRING_OBJ || args[1].Type() != object.STRING_OBJ {
				return newError("arguments to split must be STRING")
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
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 2 {
				return newError("wrong number of arguments. got=%d, want=2", len(args))
			}
			if args[0].Type() != object.LIST_OBJ || args[1].Type() != object.STRING_OBJ {
				return newError("join requires LIST and STRING")
			}
			list := args[0].(*object.List)
			sep := args[1].(*object.String).Value
			parts := make([]string, len(list.Elements))
			for i, el := range list.Elements {
				parts[i] = el.Inspect()
			}
			return &object.String{Value: strings.Join(parts, sep)}
		},
	},
	"upper": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("wrong number of arguments. got=%d, want=1", len(args))
			}
			if args[0].Type() != object.STRING_OBJ {
				return newError("argument to upper must be STRING")
			}
			str := args[0].(*object.String).Value
			return &object.String{Value: strings.ToUpper(str)}
		},
	},
	"lower": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("wrong number of arguments. got=%d, want=1", len(args))
			}
			if args[0].Type() != object.STRING_OBJ {
				return newError("argument to lower must be STRING")
			}
			str := args[0].(*object.String).Value
			return &object.String{Value: strings.ToLower(str)}
		},
	},
	"replace": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 3 {
				return newError("wrong number of arguments. got=%d, want=3", len(args))
			}
			if args[0].Type() != object.STRING_OBJ || args[1].Type() != object.STRING_OBJ || args[2].Type() != object.STRING_OBJ {
				return newError("arguments to replace must be STRING")
			}
			str := args[0].(*object.String).Value
			old := args[1].(*object.String).Value
			new := args[2].(*object.String).Value
			return &object.String{Value: strings.ReplaceAll(str, old, new)}
		},
	},
	"range": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) < 1 || len(args) > 3 {
				return newError("wrong number of arguments. got=%d, want=1-3", len(args))
			}
			var start, stop, step int64 = 0, 0, 1
			if len(args) == 1 {
				if args[0].Type() != object.INTEGER_OBJ {
					return newError("range arguments must be INTEGER")
				}
				stop = args[0].(*object.Integer).Value
			} else if len(args) == 2 {
				if args[0].Type() != object.INTEGER_OBJ || args[1].Type() != object.INTEGER_OBJ {
					return newError("range arguments must be INTEGER")
				}
				start = args[0].(*object.Integer).Value
				stop = args[1].(*object.Integer).Value
			} else {
				if args[0].Type() != object.INTEGER_OBJ || args[1].Type() != object.INTEGER_OBJ || args[2].Type() != object.INTEGER_OBJ {
					return newError("range arguments must be INTEGER")
				}
				start = args[0].(*object.Integer).Value
				stop = args[1].(*object.Integer).Value
				step = args[2].(*object.Integer).Value
				if step == 0 {
					return newError("range step cannot be zero")
				}
			}
			elements := []object.Object{}
			if step > 0 {
				for i := start; i < stop; i += step {
					elements = append(elements, &object.Integer{Value: i})
				}
			} else {
				for i := start; i > stop; i += step {
					elements = append(elements, &object.Integer{Value: i})
				}
			}
			return &object.List{Elements: elements}
		},
	},
	"keys": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("wrong number of arguments. got=%d, want=1", len(args))
			}
			if args[0].Type() != object.DICT_OBJ {
				return newError("argument to keys must be DICT")
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
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("wrong number of arguments. got=%d, want=1", len(args))
			}
			if args[0].Type() != object.DICT_OBJ {
				return newError("argument to values must be DICT")
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
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("wrong number of arguments. got=%d, want=1", len(args))
			}
			if args[0].Type() != object.DICT_OBJ {
				return newError("argument to items must be DICT")
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

func GetImportBuiltin() *object.Builtin {
	return &object.Builtin{
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("wrong number of arguments. got=%d, want=1", len(args))
			}
			if args[0].Type() != object.STRING_OBJ {
				return newError("argument to import must be STRING")
			}
			if importCallback == nil {
				return newError("import not available")
			}
			libName := args[0].(*object.String).Value
			err := importCallback(libName)
			if err != nil {
				return newError("import error: %s", err.Error())
			}
			return &object.Null{}
		},
	}
}
