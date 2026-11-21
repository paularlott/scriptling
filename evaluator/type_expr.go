package evaluator

import (
	"context"

	"github.com/paularlott/scriptling/object"
)

// isTypeOperand reports whether o can be a member of a X | Y type union:
// a builtin type, a class, None, or another union or generic alias.
func isTypeOperand(o object.Object) bool {
	switch v := o.(type) {
	case *object.Null, *object.Class:
		return true
	case *object.Builtin:
		if _, ok := typeBuiltins[v]; ok {
			return true
		}
		if _, ok := exceptionClassName(v); ok {
			return true
		}
		return object.IsTypeExpr(v)
	}
	return false
}

// typeExprName renders a type in a type expression: int, None, MyClass,
// list[int].
func typeExprName(o object.Object) string {
	switch v := o.(type) {
	case *object.Null:
		return "None"
	case *object.Class:
		return v.Name
	case *object.String:
		return object.ReprString(v.StringValue())
	case *object.Builtin:
		if name, ok := typeBuiltins[v]; ok {
			return name
		}
		if name, ok := exceptionClassName(v); ok {
			return name
		}
	case *object.Sentinel:
		if v == ellipsisValue {
			return "..."
		}
	}
	return o.Inspect()
}

// isinstanceFn is the isinstance builtin, for its own union members (set in
// init to break the builtins initialisation cycle).
var isinstanceFn object.BuiltinFunction

func init() {
	object.TypeExprName = typeExprName
	isinstanceFn = builtins["isinstance"].Fn
	object.IsInstance = func(ctx context.Context, obj, classinfo object.Object) object.Object {
		return isinstanceFn(ctx, object.Kwargs{}, obj, classinfo)
	}
}

// unionTypeArgs maps a union's members to isinstance arguments: None is
// the type name NoneType.
func unionTypeArgs(members []object.Object) []object.Object {
	out := make([]object.Object, len(members))
	for i, m := range members {
		if _, none := m.(*object.Null); none {
			m = object.NewString("NoneType")
		}
		out[i] = m
	}
	return out
}
