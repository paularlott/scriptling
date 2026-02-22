package stdlib

import (
	"context"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

const suppressKey = "__suppress_types__"

// resolveExcTypeName returns the exception type name string for an object passed to suppress().
// Builtins (e.g. ValueError) are called with no args to obtain their ExceptionType field.
func resolveExcTypeName(ctx context.Context, obj object.Object) string {
	switch v := obj.(type) {
	case *object.String:
		return v.Value
	case *object.Class:
		return v.Name
	case *object.Builtin:
		result := v.Fn(ctx, object.NewKwargs(nil))
		if exc, ok := object.AsException(result); ok {
			return exc.ExceptionType
		}
	}
	return ""
}

var suppressClass = &object.Class{
	Name: "suppress",
	Methods: map[string]object.Object{
		"__init__": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) < 1 {
					return errors.NewError("suppress.__init__ requires self")
				}
				inst, ok := args[0].(*object.Instance)
				if !ok {
					return errors.NewError("suppress.__init__: self must be an instance")
				}
				inst.Fields[suppressKey] = &object.List{Elements: args[1:]}
				return &object.Null{}
			},
		},
		"__enter__": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) < 1 {
					return errors.NewError("suppress.__enter__ requires self")
				}
				return args[0]
			},
		},
		"__exit__": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				// args: self, exc_type, exc_val, exc_tb
				if len(args) < 2 {
					return &object.Boolean{Value: false}
				}
				inst, ok := args[0].(*object.Instance)
				if !ok {
					return &object.Boolean{Value: false}
				}
				excType := args[1]
				if excType == nil || excType.Type() == object.NULL_OBJ {
					return &object.Boolean{Value: false}
				}
				typesObj, ok := inst.Fields[suppressKey]
				if !ok {
					return &object.Boolean{Value: false}
				}
				typesList, ok := typesObj.(*object.List)
				if !ok {
					return &object.Boolean{Value: false}
				}
				if len(typesList.Elements) == 0 {
					return &object.Boolean{Value: true}
				}
				excTypeName, _ := excType.AsString()
				for _, t := range typesList.Elements {
					name := resolveExcTypeName(ctx, t)
					if name == excTypeName || name == object.ExceptionTypeException {
						return &object.Boolean{Value: true}
					}
				}
				return &object.Boolean{Value: false}
			},
		},
	},
}

func suppressNew(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	inst := &object.Instance{
		Class:  suppressClass,
		Fields: map[string]object.Object{},
	}
	inst.Fields[suppressKey] = &object.List{Elements: args}
	return inst
}

var ContextlibLibrary = object.NewLibrary(ContextlibLibraryName, map[string]*object.Builtin{
	"suppress": {
		Fn: suppressNew,
		HelpText: `suppress(*exc_types) - Context manager that silently suppresses the given exception types.

If no exception types are given, all exceptions are suppressed.
Use Exception to suppress all standard exceptions.

Example:
    with contextlib.suppress(ValueError):
        int("not a number")  # silently ignored`,
	},
}, nil, "Utilities for common tasks involving the with statement")
