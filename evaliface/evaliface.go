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
