package evaluator

import (
	"context"

	"github.com/paularlott/scriptling/evaliface"
	"github.com/paularlott/scriptling/object"
)

type evalAdapter struct{}

func (evalAdapter) CallFunction(ctx context.Context, fn *object.Function, args []object.Object, kwargs map[string]object.Object) object.Object {
	ctx, release := enterScript(ctx, fn.Env)
	defer release()
	return applyFunctionWithContext(ctx, fn, args, kwargs, fn.Env)
}

func (evalAdapter) CallObjectFunction(ctx context.Context, fn object.Object, args []object.Object, kwargs map[string]object.Object, env *object.Environment) object.Object {
	ctx, release := enterScript(ctx, env)
	defer release()
	return applyFunction(ctx, fn, args, kwargs, env)
}

func (evalAdapter) CallMethod(ctx context.Context, instance *object.Instance, method *object.Function, args []object.Object) object.Object {
	ctx, release := enterScript(ctx, method.Env)
	defer release()
	allArgs := append([]object.Object{instance}, args...)
	return applyFunctionWithContext(ctx, method, allArgs, nil, method.Env)
}

// WithEvaluator adds evaluator to context
func WithEvaluator(ctx context.Context) context.Context {
	return evaliface.WithEvaluator(ctx, evalAdapter{})
}
