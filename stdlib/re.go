package stdlib

import (
	"github.com/paularlott/scriptling/object"
	"regexp"
)

func ReLibrary() map[string]*object.Builtin {
	return map[string]*object.Builtin{
		"match": {
			Fn: func(args ...object.Object) object.Object {
				if len(args) != 2 {
					return newError("wrong number of arguments. got=%d, want=2", len(args))
				}
				if args[0].Type() != object.STRING_OBJ || args[1].Type() != object.STRING_OBJ {
					return newError("arguments must be STRING")
				}
				pattern := args[0].(*object.String).Value
				text := args[1].(*object.String).Value
				
				matched, err := regexp.MatchString(pattern, text)
				if err != nil {
					return newError("regex error: %s", err.Error())
				}
				return &object.Boolean{Value: matched}
			},
		},
		"find": {
			Fn: func(args ...object.Object) object.Object {
				if len(args) != 2 {
					return newError("wrong number of arguments. got=%d, want=2", len(args))
				}
				if args[0].Type() != object.STRING_OBJ || args[1].Type() != object.STRING_OBJ {
					return newError("arguments must be STRING")
				}
				pattern := args[0].(*object.String).Value
				text := args[1].(*object.String).Value
				
				re, err := regexp.Compile(pattern)
				if err != nil {
					return newError("regex compile error: %s", err.Error())
				}
				
				result := re.FindString(text)
				if result == "" {
					return &object.Null{}
				}
				return &object.String{Value: result}
			},
		},
		"findall": {
			Fn: func(args ...object.Object) object.Object {
				if len(args) != 2 {
					return newError("wrong number of arguments. got=%d, want=2", len(args))
				}
				if args[0].Type() != object.STRING_OBJ || args[1].Type() != object.STRING_OBJ {
					return newError("arguments must be STRING")
				}
				pattern := args[0].(*object.String).Value
				text := args[1].(*object.String).Value
				
				re, err := regexp.Compile(pattern)
				if err != nil {
					return newError("regex compile error: %s", err.Error())
				}
				
				matches := re.FindAllString(text, -1)
				elements := make([]object.Object, len(matches))
				for i, match := range matches {
					elements[i] = &object.String{Value: match}
				}
				return &object.List{Elements: elements}
			},
		},
		"replace": {
			Fn: func(args ...object.Object) object.Object {
				if len(args) != 3 {
					return newError("wrong number of arguments. got=%d, want=3", len(args))
				}
				if args[0].Type() != object.STRING_OBJ || args[1].Type() != object.STRING_OBJ || args[2].Type() != object.STRING_OBJ {
					return newError("arguments must be STRING")
				}
				pattern := args[0].(*object.String).Value
				text := args[1].(*object.String).Value
				replacement := args[2].(*object.String).Value
				
				re, err := regexp.Compile(pattern)
				if err != nil {
					return newError("regex compile error: %s", err.Error())
				}
				
				result := re.ReplaceAllString(text, replacement)
				return &object.String{Value: result}
			},
		},
		"split": {
			Fn: func(args ...object.Object) object.Object {
				if len(args) != 2 {
					return newError("wrong number of arguments. got=%d, want=2", len(args))
				}
				if args[0].Type() != object.STRING_OBJ || args[1].Type() != object.STRING_OBJ {
					return newError("arguments must be STRING")
				}
				pattern := args[0].(*object.String).Value
				text := args[1].(*object.String).Value
				
				re, err := regexp.Compile(pattern)
				if err != nil {
					return newError("regex compile error: %s", err.Error())
				}
				
				parts := re.Split(text, -1)
				elements := make([]object.Object, len(parts))
				for i, part := range parts {
					elements[i] = &object.String{Value: part}
				}
				return &object.List{Elements: elements}
			},
		},
	}
}
