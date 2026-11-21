// Package evaliface provides an interface for calling functions from libraries
// without creating circular dependencies
package evaliface

import (
	"context"

	"github.com/paularlott/scriptling/object"
)

// Evaluator interface for calling functions from libraries
type Evaluator interface {
	CallFunction(ctx context.Context, fn *object.Function, args []object.Object, kwargs map[string]object.Object) object.Object
	CallObjectFunction(ctx context.Context, fn object.Object, args []object.Object, kwargs map[string]object.Object, env *object.Environment) object.Object
	CallMethod(ctx context.Context, instance *object.Instance, method *object.Function, args []object.Object) object.Object
}

// KwargsCaller is implemented by the evaluator alongside Evaluator: calls
// with the keywords held as an object.Kwargs, passed on without a map.
// Libraries type-assert for it and fall back to CallObjectFunction.
type KwargsCaller interface {
	CallObjectFunctionKwargs(ctx context.Context, fn object.Object, args []object.Object, kwargs object.Kwargs, env *object.Environment) object.Object
}

// Operators is implemented by the evaluator alongside Evaluator: the
// language's operators with their full semantics (dunder methods, Python's
// errors), for libraries such as operator. Libraries type-assert for it, so
// an Evaluator that does not provide it keeps working.
type Operators interface {
	// BinaryOp applies an infix operator by its source spelling: "+", "//",
	// "==", "<", "in", "is", "&", "<<", ...
	BinaryOp(ctx context.Context, op string, left, right object.Object) object.Object
	// UnaryOp applies "-", "+", "~", "not" or "abs".
	UnaryOp(ctx context.Context, op string, operand object.Object) object.Object
	GetItem(ctx context.Context, obj, key object.Object) object.Object
	SetItem(ctx context.Context, obj, key, value object.Object) object.Object
	DelItem(ctx context.Context, obj, key object.Object) object.Object
	GetAttr(ctx context.Context, obj object.Object, name string) object.Object
	Truthy(ctx context.Context, obj object.Object) (bool, object.Object)
}

// OperatorsFromContext retrieves the evaluator's operators from context, nil
// when the evaluator in context does not provide them.
func OperatorsFromContext(ctx context.Context) Operators {
	if ops, ok := ctx.Value(evalKey{}).(Operators); ok {
		return ops
	}
	return nil
}

type evalKey struct{}

// WithEvaluator stores evaluator in context
func WithEvaluator(ctx context.Context, eval Evaluator) context.Context {
	return context.WithValue(ctx, evalKey{}, eval)
}

// FromContext retrieves evaluator from context
func FromContext(ctx context.Context) Evaluator {
	if eval, ok := ctx.Value(evalKey{}).(Evaluator); ok {
		return eval
	}
	return nil
}
