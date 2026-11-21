package evaluator

import (
	"context"

	"github.com/paularlott/scriptling/evaliface"
	"github.com/paularlott/scriptling/object"
)

type evalAdapter struct{}

func (evalAdapter) CallFunction(ctx context.Context, fn *object.Function, args []object.Object, kwargs map[string]object.Object) object.Object {
	if gilEnv := enterScript(fn.Env); gilEnv != nil {
		defer gilEnv.ExitGIL()
	}
	return applyFunctionWithContext(ctx, fn, args, object.NewKwargs(kwargs), fn.Env)
}

func (evalAdapter) CallObjectFunction(ctx context.Context, fn object.Object, args []object.Object, kwargs map[string]object.Object, env *object.Environment) object.Object {
	if gilEnv := enterScript(env); gilEnv != nil {
		defer gilEnv.ExitGIL()
	}
	// WithContext: a bound method (obj.method passed as a callback) binds self.
	return applyFunctionWithContext(ctx, fn, args, object.NewKwargs(kwargs), env)
}

// CallObjectFunctionKwargs is CallObjectFunction with the keywords as an
// object.Kwargs, which is passed on without building a map.
func (evalAdapter) CallObjectFunctionKwargs(ctx context.Context, fn object.Object, args []object.Object, kwargs object.Kwargs, env *object.Environment) object.Object {
	if gilEnv := enterScript(env); gilEnv != nil {
		defer gilEnv.ExitGIL()
	}
	return applyFunctionWithContext(ctx, fn, args, kwargs, env)
}

func (evalAdapter) CallMethod(ctx context.Context, instance *object.Instance, method *object.Function, args []object.Object) object.Object {
	if gilEnv := enterScript(method.Env); gilEnv != nil {
		defer gilEnv.ExitGIL()
	}
	allArgs := append([]object.Object{instance}, args...)
	return applyFunctionWithContext(ctx, method, allArgs, object.Kwargs{}, method.Env)
}

// WithEvaluator adds evaluator to context
func WithEvaluator(ctx context.Context) context.Context {
	return evaliface.WithEvaluator(ctx, evalAdapter{})
}
