package plugin

import (
	"context"
	"fmt"

	"github.com/paularlott/scriptling/object"
)

// Callback is a host callback passed into a plugin call. It is valid only
// until the outer plugin function, constructor, or method returns.
type Callback interface {
	Call(ctx context.Context, args ...any) (Value, error)
}

type callbackHandle struct {
	id string
}

type callbackRuntimeKey struct{}

func (c *callbackHandle) Call(ctx context.Context, args ...any) (Value, error) {
	runtime, ok := ctx.Value(callbackRuntimeKey{}).(*serverRuntime)
	if !ok || runtime == nil {
		return Value{}, fmt.Errorf("callback %s is not attached to an active plugin call", c.id)
	}
	values := make([]Value, 0, len(args))
	for _, arg := range args {
		values = append(values, goValueToTransport(arg))
	}
	return runtime.callCallback(ctx, callbackCallParams{
		ID:   c.id,
		Args: values,
	})
}

// ScriptCall implements object.ScriptCallable so that a callback received as a
// plugin argument can be invoked directly from scriptling code (e.g. cb(1, 2)).
// It converts object.Object args/kwargs to the plugin transport format, sends
// callback.call back to the client over the active serverRuntime connection, and
// converts the result back to an object.Object.
//
// Only valid during an active stdio plugin call; HTTP transport does not support
// callbacks (the connection is not bidirectional).
func (c *callbackHandle) ScriptCall(ctx context.Context, args []object.Object, kwargs map[string]object.Object) object.Object {
	runtime, ok := ctx.Value(callbackRuntimeKey{}).(*serverRuntime)
	if !ok || runtime == nil {
		return &object.Error{Message: fmt.Sprintf("callback %s is not attached to an active plugin call (stdio transport required)", c.id)}
	}

	values := make([]Value, 0, len(args))
	for _, arg := range args {
		values = append(values, goValueToTransport(arg))
	}

	kwValues := make(map[string]Value, len(kwargs))
	for k, v := range kwargs {
		kwValues[k] = goValueToTransport(v)
	}

	result, err := runtime.callCallback(ctx, callbackCallParams{
		ID:     c.id,
		Args:   values,
		Kwargs: kwValues,
	})
	if err != nil {
		return &object.Error{Message: err.Error()}
	}

	obj, convErr := valueToObject(result)
	if convErr != nil {
		return &object.Error{Message: convErr.Error()}
	}
	return obj
}
