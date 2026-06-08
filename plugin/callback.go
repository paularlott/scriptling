package plugin

import (
	"context"
	"fmt"
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
