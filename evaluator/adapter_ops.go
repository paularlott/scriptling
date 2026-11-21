package evaluator

import (
	"context"

	"github.com/paularlott/scriptling/ast"
	"github.com/paularlott/scriptling/evaliface"
	"github.com/paularlott/scriptling/object"
)

// evalAdapter's evaliface.Operators: the language's operators for libraries
// (the operator module).

// binaryOps maps an infix operator's spelling to its ast.Op.
var binaryOps = func() map[string]ast.Op {
	m := map[string]ast.Op{}
	for op := ast.OpAdd; op <= ast.OpIsNot; op++ {
		if op != ast.OpAnd && op != ast.OpOr {
			m[op.String()] = op
		}
	}
	// The augmented assignments ("+=", ...) give the operator module's
	// in-place functions: lists, dicts and sets update in place.
	for op := ast.OpAddEq; op <= ast.OpDivEq; op++ {
		m[op.String()] = op
	}
	return m
}()

func (evalAdapter) BinaryOp(ctx context.Context, op string, left, right object.Object) object.Object {
	o, ok := binaryOps[op]
	if ok && o >= ast.OpAddEq && o <= ast.OpDivEq {
		result, done, errObj := applyAugmentedOp(ctx, o, left, right, GetEnvFromContext(ctx))
		switch {
		case errObj != nil:
			return errObj
		case done:
			return left
		}
		return result
	}
	if !ok {
		return &object.Exception{Message: "unknown operator " + op, ExceptionType: object.ExceptionTypeValueError, Raised: true}
	}
	return evalInfixExpression(ctx, o, left, right, GetEnvFromContext(ctx))
}

func (evalAdapter) UnaryOp(ctx context.Context, op string, operand object.Object) object.Object {
	env := GetEnvFromContext(ctx)
	switch op {
	case "-":
		return evalPrefixExpression(ctx, ast.OpSub, operand, env)
	case "~":
		return evalPrefixExpression(ctx, ast.OpBitNot, operand, env)
	case "not":
		return evalNotOperatorExpression(ctx, operand, env)
	case "abs":
		if b, ok := operand.(*object.Boolean); ok {
			return object.NewInteger(boolToInt64(b.BoolValue()))
		}
		return builtins["abs"].Fn(ctx, object.NewKwargs(nil), operand)
	case "+":
		switch v := operand.(type) {
		case *object.Integer, *object.Float:
			return v
		case *object.Boolean:
			return object.NewInteger(boolToInt64(v.BoolValue()))
		case *object.Instance:
			if r := callDunderMethod(ctx, v, "__pos__", nil, env); r != nil {
				return r
			}
		}
		return &object.Exception{Message: "bad operand type for unary +: '" + getTypeName(operand) + "'", ExceptionType: object.ExceptionTypeTypeError, Raised: true}
	}
	return &object.Exception{Message: "unknown operator " + op, ExceptionType: object.ExceptionTypeValueError, Raised: true}
}

func (evalAdapter) GetItem(ctx context.Context, obj, key object.Object) object.Object {
	return evalIndexExpression(ctx, obj, key, false)
}

func (evalAdapter) SetItem(ctx context.Context, obj, key, value object.Object) object.Object {
	if err := assignIndexValue(ctx, false, obj, key, value); err != nil {
		return assignErrorToObject(err)
	}
	return NULL
}

func (evalAdapter) DelItem(ctx context.Context, obj, key object.Object) object.Object {
	env := GetEnvFromContext(ctx)
	switch o := obj.(type) {
	case *object.List:
		idx, ok := key.(*object.Integer)
		if !ok {
			return &object.Exception{Message: "list indices must be integers or slices, not " + getTypeName(key), ExceptionType: object.ExceptionTypeTypeError, Raised: true}
		}
		i, n := idx.IntValue(), int64(len(o.Elements))
		if i < 0 {
			i += n
		}
		if i < 0 || i >= n {
			return &object.Exception{Message: "list assignment index out of range", ExceptionType: object.ExceptionTypeIndexError, Raised: true}
		}
		o.Elements = append(o.Elements[:i], o.Elements[i+1:]...)
		return NULL
	case *object.Dict:
		k, rerr := evalHashKeyChecked(ctx, key)
		if rerr != nil {
			return rerr
		}
		if _, ok := o.Lookup(k); !ok {
			return &object.Exception{Message: key.Inspect(), ExceptionType: object.ExceptionTypeKeyError, Raised: true}
		}
		o.Delete(k)
		return NULL
	case *object.Instance:
		if r := callDunderMethod(ctx, o, "__delitem__", []object.Object{key}, env); r != nil {
			return r
		}
	}
	return &object.Exception{Message: "'" + getTypeName(obj) + "' object doesn't support item deletion", ExceptionType: object.ExceptionTypeTypeError, Raised: true}
}

func (evalAdapter) GetAttr(ctx context.Context, obj object.Object, name string) object.Object {
	return getAttribute(ctx, obj, name)
}

func (evalAdapter) Truthy(ctx context.Context, obj object.Object) (bool, object.Object) {
	return evalTruthy(ctx, obj, GetEnvFromContext(ctx))
}

var _ evaliface.Operators = evalAdapter{}
