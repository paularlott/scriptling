package extlibs

import (
	"context"
	"fmt"
	"os"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/evaluator"
	"github.com/paularlott/scriptling/object"
)

// JSONRPCErrorClass is the class for error objects produced by
// runtime.jsonrpc.error(). The stdio server recognises instances of this class
// and emits a JSON-RPC error response instead of a result.
var JSONRPCErrorClass = &object.Class{
	Name: "JSONRPCError",
}

// CreateJSONRPCErrorInstance creates a JSON-RPC error object from Go code.
func CreateJSONRPCErrorInstance(code int64, message string, data object.Object) *object.Instance {
	fields := map[string]object.Object{
		"code":    object.NewInteger(code),
		"message": object.NewString(message),
	}
	if data != nil {
		fields["data"] = data
	}
	return object.NewInstanceWithFields(JSONRPCErrorClass, fields)
}

// IsJSONRPCError reports whether obj is a JSONRPCError instance.
func IsJSONRPCError(obj object.Object) bool {
	inst, ok := obj.(*object.Instance)
	return ok && inst.Class == JSONRPCErrorClass
}

// makeJSONRPCDecorator returns a builtin that, when applied to a function (by
// the @decorator mechanism), resolves the "module.function" reference and
// calls register with it.
func makeJSONRPCDecorator(ctx context.Context, register func(ref string)) object.Object {
	env := evaluator.GetEnvFromContext(ctx)
	return &object.Builtin{
		Fn: func(_ context.Context, _ object.Kwargs, args ...object.Object) object.Object {
			if len(args) == 0 {
				return errors.NewError("decorator requires a function")
			}
			fn, ok := args[0].(*object.Function)
			if !ok {
				return errors.NewError("decorated value must be a function, got %s", args[0].Type())
			}
			if fn.Name == "" {
				return errors.NewError("decorated function has no name")
			}
			ref := resolveModuleRef(env, fn.Name)
			if ref == "" {
				return errors.NewError("cannot determine module name for decorator — ensure __name__ or __file__ is set")
			}
			register(ref)
			return fn
		},
	}
}

// registerJSONRPCMethod is the shared registration logic for JSON-RPC methods.
func registerJSONRPCMethod(name, handler string) {
	RuntimeState.Lock()
	defer RuntimeState.Unlock()
	if existing, ok := RuntimeState.JSONRPCMethods[name]; ok && existing == handler {
		return
	}
	RuntimeState.JSONRPCMethods[name] = handler
}

// registerJSONRPCNotification is the shared registration logic for JSON-RPC notifications.
func registerJSONRPCNotification(name, handler string) {
	RuntimeState.Lock()
	defer RuntimeState.Unlock()
	if existing, ok := RuntimeState.JSONRPCNotifications[name]; ok && existing == handler {
		return
	}
	RuntimeState.JSONRPCNotifications[name] = handler
}

// JSONRPCSubLibrary exposes runtime.jsonrpc for registering stdio JSON-RPC 2.0
// method and notification handlers. Handlers are referenced by string
// ("library.function") and run on a fresh evaluator per request, matching the
// runtime.http / MCP / WebSocket concurrency model.
var JSONRPCSubLibrary = object.NewLibrary(RuntimeJSONRPCLibraryName, map[string]*object.Builtin{
	"method": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.MinArgs(args, 1); err != nil {
				return err
			}

			name, err := args[0].AsString()
			if err != nil {
				return err
			}

			// Decorator form: @jsonrpc.method("echo")
			if len(args) == 1 {
				return makeJSONRPCDecorator(ctx, func(ref string) {
					registerJSONRPCMethod(name, ref)
				})
			}

			// Imperative form: method("echo", "lib.func")
			handler, err := args[1].AsString()
			if err != nil {
				return err
			}

			RuntimeState.Lock()
			if RuntimeState.ServerStarted {
				fmt.Fprintf(os.Stderr, "warning: runtime.jsonrpc.method %q registered after start_server() — method will not be served\n", name)
			}
			RuntimeState.JSONRPCMethods[name] = handler
			RuntimeState.Unlock()

			return &object.Null{}
		},
		HelpText: `method(name, handler=None) - Register a JSON-RPC method handler, or use as decorator

Decorator form:
  import scriptling.jsonrpc as jsonrpc

  @jsonrpc.method("echo")
  def echo(params):
      return params

Imperative form:
  runtime.jsonrpc.method("echo", "handlers.echo")

The handler receives the decoded JSON-RPC params as its single argument and
returns a JSON-compatible result. Return runtime.jsonrpc.error(...) to produce
an error response.`,
	},

	"notification": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.MinArgs(args, 1); err != nil {
				return err
			}

			name, err := args[0].AsString()
			if err != nil {
				return err
			}

			// Decorator form: @jsonrpc.notification("updated")
			if len(args) == 1 {
				return makeJSONRPCDecorator(ctx, func(ref string) {
					registerJSONRPCNotification(name, ref)
				})
			}

			// Imperative form: notification("updated", "lib.func")
			handler, err := args[1].AsString()
			if err != nil {
				return err
			}

			RuntimeState.Lock()
			if RuntimeState.ServerStarted {
				fmt.Fprintf(os.Stderr, "warning: runtime.jsonrpc.notification %q registered after start_server() — notification will not be served\n", name)
			}
			RuntimeState.JSONRPCNotifications[name] = handler
			RuntimeState.Unlock()

			return &object.Null{}
		},
		HelpText: `notification(name, handler=None) - Register a JSON-RPC notification handler, or use as decorator

Decorator form:
  @jsonrpc.notification("updated")
  def on_updated(params):
      ...

Imperative form:
  runtime.jsonrpc.notification("updated", "handlers.on_updated")

Notifications are JSON-RPC requests without an id. The handler receives the
decoded params and no response is written.`,
	},

	"error": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.MinArgs(args, 2); err != nil {
				return err
			}

			code, err := args[0].AsInt()
			if err != nil {
				return err
			}

			message, err := args[1].AsString()
			if err != nil {
				return err
			}

			var data object.Object
			if len(args) >= 3 {
				data = args[2]
			}

			return CreateJSONRPCErrorInstance(code, message, data)
		},
		HelpText: `error(code, message, data=None) - Build a JSON-RPC error response

Parameters:
  code (int): JSON-RPC error code (e.g. -32602 for invalid params)
  message (str): Human-readable error message
  data (any, optional): Optional structured data attached to the error

Return this from a method handler to emit a JSON-RPC error response with a
custom code. If omitted the response uses the given code and message.

Example:
  def divide(params):
      if params["b"] == 0:
          return runtime.jsonrpc.error(-32602, "division by zero")
      return params["a"] / params["b"]`,
	},
}, map[string]object.Object{
	"JSONRPCError": JSONRPCErrorClass,
}, "stdio JSON-RPC 2.0 server method and notification registration")
