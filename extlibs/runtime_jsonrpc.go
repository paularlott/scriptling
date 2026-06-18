package extlibs

import (
	"context"

	"github.com/paularlott/scriptling/errors"
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
	return &object.Instance{
		Class:  JSONRPCErrorClass,
		Fields: fields,
	}
}

// IsJSONRPCError reports whether obj is a JSONRPCError instance.
func IsJSONRPCError(obj object.Object) bool {
	inst, ok := obj.(*object.Instance)
	return ok && inst.Class == JSONRPCErrorClass
}

// JSONRPCSubLibrary exposes runtime.jsonrpc for registering stdio JSON-RPC 2.0
// method and notification handlers. Handlers are referenced by string
// ("library.function") and run on a fresh evaluator per request, matching the
// runtime.http / MCP / WebSocket concurrency model.
var JSONRPCSubLibrary = object.NewLibrary(RuntimeJSONRPCLibraryName, map[string]*object.Builtin{
	"method": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.MinArgs(args, 2); err != nil {
				return err
			}

			name, err := args[0].AsString()
			if err != nil {
				return err
			}

			handler, err := args[1].AsString()
			if err != nil {
				return err
			}

			RuntimeState.Lock()
			RuntimeState.JSONRPCMethods[name] = handler
			RuntimeState.Unlock()

			return &object.Null{}
		},
		HelpText: `method(name, handler) - Register a JSON-RPC method handler

Parameters:
  name (str): JSON-RPC method name
  handler (str): Handler function as "library.function" string

The handler receives the decoded JSON-RPC params as its single argument and
returns a JSON-compatible result. Raise an exception or return
runtime.jsonrpc.error(...) to produce an error response.

Example:
  import scriptling.runtime as runtime

  runtime.jsonrpc.method("echo", "handlers.echo")`,
	},

	"notification": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.MinArgs(args, 2); err != nil {
				return err
			}

			name, err := args[0].AsString()
			if err != nil {
				return err
			}

			handler, err := args[1].AsString()
			if err != nil {
				return err
			}

			RuntimeState.Lock()
			RuntimeState.JSONRPCNotifications[name] = handler
			RuntimeState.Unlock()

			return &object.Null{}
		},
		HelpText: `notification(name, handler) - Register a JSON-RPC notification handler

Parameters:
  name (str): JSON-RPC notification name
  handler (str): Handler function as "library.function" string

Notifications are JSON-RPC requests without an id. The handler receives the
decoded params and no response is written. Return values are ignored.

Example:
  import scriptling.runtime as runtime

  runtime.jsonrpc.notification("updated", "handlers.on_updated")`,
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
