package stdlib

import (
	"context"
	"regexp"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

var reLibrary = object.NewLibrary(map[string]*object.Builtin{
	"match": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 2 {
				return errors.NewArgumentError(len(args), 2)
			}
			if args[0].Type() != object.STRING_OBJ || args[1].Type() != object.STRING_OBJ {
				return errors.NewTypeError("STRING", "mixed types")
			}
			pattern := args[0].(*object.String).Value
			text := args[1].(*object.String).Value

			matched, err := regexp.MatchString(pattern, text)
			if err != nil {
				return errors.NewError("regex error: %s", err.Error())
			}
			return &object.Boolean{Value: matched}
		},
	},
	"find": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 2 {
				return errors.NewArgumentError(len(args), 2)
			}
			if args[0].Type() != object.STRING_OBJ || args[1].Type() != object.STRING_OBJ {
				return errors.NewTypeError("STRING", "mixed types")
			}
			pattern := args[0].(*object.String).Value
			text := args[1].(*object.String).Value

			re, err := regexp.Compile(pattern)
			if err != nil {
				return errors.NewError("regex compile error: %s", err.Error())
			}

			result := re.FindString(text)
			if result == "" {
				return &object.Null{}
			}
			return &object.String{Value: result}
		},
	},
	"findall": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 2 {
				return errors.NewArgumentError(len(args), 2)
			}
			if args[0].Type() != object.STRING_OBJ || args[1].Type() != object.STRING_OBJ {
				return errors.NewTypeError("STRING", "mixed types")
			}
			pattern := args[0].(*object.String).Value
			text := args[1].(*object.String).Value

			re, err := regexp.Compile(pattern)
			if err != nil {
				return errors.NewError("regex compile error: %s", err.Error())
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
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 3 {
				return errors.NewArgumentError(len(args), 3)
			}
			if args[0].Type() != object.STRING_OBJ || args[1].Type() != object.STRING_OBJ || args[2].Type() != object.STRING_OBJ {
				return errors.NewTypeError("STRING", "mixed types")
			}
			pattern := args[0].(*object.String).Value
			text := args[1].(*object.String).Value
			replacement := args[2].(*object.String).Value

			re, err := regexp.Compile(pattern)
			if err != nil {
				return errors.NewError("regex compile error: %s", err.Error())
			}

			result := re.ReplaceAllString(text, replacement)
			return &object.String{Value: result}
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
			pattern := args[0].(*object.String).Value
			text := args[1].(*object.String).Value

			re, err := regexp.Compile(pattern)
			if err != nil {
				return errors.NewError("regex compile error: %s", err.Error())
			}

			parts := re.Split(text, -1)
			elements := make([]object.Object, len(parts))
			for i, part := range parts {
				elements[i] = &object.String{Value: part}
			}
			return &object.List{Elements: elements}
		},
	},
})

func ReLibrary() *object.Library {
	return reLibrary
}
