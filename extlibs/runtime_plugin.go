package extlibs

import (
	"context"
	"fmt"
	"os"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/evaluator"
	"github.com/paularlott/scriptling/object"
)

// PluginSubLibrary exposes runtime.plugin for declaring a script as a
// Scriptling plugin server. When start_server() is called the CLI replaces the
// plain JSON-RPC loop with a full plugin.Server that handles the plugin
// handshake, function.call, and object lifecycle. Available in the agent
// variant of scriptling only.
var PluginSubLibrary = object.NewLibrary(RuntimePluginLibraryName, map[string]*object.Builtin{

	"serve": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.MinArgs(args, 1); err != nil {
				return err
			}

			name, err := args[0].AsString()
			if err != nil {
				return err
			}

			version := ""
			if len(args) >= 2 {
				v, e := args[1].AsString()
				if e != nil {
					return e
				}
				version = v
			}
			description := ""
			if len(args) >= 3 {
				d, e := args[2].AsString()
				if e != nil {
					return e
				}
				description = d
			}

			RuntimeState.Lock()
			if RuntimeState.ServerStarted {
				fmt.Fprintf(os.Stderr, "warning: runtime.plugin.serve() called after start_server() — plugin identity will not be used\n")
			}
			RuntimeState.PluginName = name
			RuntimeState.PluginVersion = version
			RuntimeState.PluginDescription = description
			RuntimeState.Unlock()

			return &object.Null{}
		},
		HelpText: `serve(name, version="", description="") - Declare this script as a plugin server

When runtime.start_server() is called in stdio mode the server serves the full
Scriptling plugin protocol (scriptling.handshake, function.call, etc.) instead
of the plain JSON-RPC loop. Clients can then load the script as a plugin peer
with scriptling=True and get auto-generated proxy libraries.

Parameters:
  name (str):        Library name (e.g. "myservice"). Clients import it as plugin.<name>.
  version (str):     Optional version string (e.g. "1.0.0").
  description (str): Optional human-readable description.

Example:
  import scriptling.runtime.plugin as plugin_srv

  plugin_srv.serve("calculator", "1.0", "Basic arithmetic operations")
  plugin_srv.register_function("add", "handlers.add")
  import scriptling.runtime as runtime
  runtime.start_server()`,
	},

	"register_function": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.MinArgs(args, 1); err != nil {
				return err
			}

			// Bare decorator: @plugin.register_function
			if fn, ok := args[0].(*object.Function); ok {
				env := evaluator.GetEnvFromContext(ctx)
				ref := resolveModuleRef(env, fn.Name)
				if ref == "" {
					return errors.NewError("cannot determine module name for @plugin.register_function decorator")
				}
				RuntimeState.Lock()
				RuntimeState.PluginFunctions[fn.Name] = ref
				RuntimeState.Unlock()
				return fn
			}

			name, err := args[0].AsString()
			if err != nil {
				return err
			}

			// Decorator factory: @plugin.register_function("add")
			if len(args) == 1 {
				env := evaluator.GetEnvFromContext(ctx)
				return &object.Builtin{
					Fn: func(_ context.Context, _ object.Kwargs, dArgs ...object.Object) object.Object {
						if len(dArgs) == 0 {
							return errors.NewError("decorator requires a function")
						}
						fn, ok := dArgs[0].(*object.Function)
						if !ok {
							return errors.NewError("decorated value must be a function, got %s", dArgs[0].Type())
						}
						ref := resolveModuleRef(env, fn.Name)
						if ref == "" {
							return errors.NewError("cannot determine module name for decorator")
						}
						RuntimeState.Lock()
						RuntimeState.PluginFunctions[name] = ref
						RuntimeState.Unlock()
						return fn
					},
				}
			}

			// Imperative: register_function("add", "lib.func")
			handler, err := args[1].AsString()
			if err != nil {
				return err
			}

			RuntimeState.Lock()
			if RuntimeState.ServerStarted {
				fmt.Fprintf(os.Stderr, "warning: runtime.plugin.register_function %q registered after start_server() — function will not be served\n", name)
			}
			RuntimeState.PluginFunctions[name] = handler
			RuntimeState.Unlock()

			return &object.Null{}
		},
		HelpText: `register_function(name, handler=None) - Register a function for the plugin server, or use as decorator

Decorator form:
  import scriptling.runtime.plugin as plugin

  @plugin.register_function("add")
  def add(a, b):
      return a + b

  # Bare form uses the function name as the plugin name
  @plugin.register_function
  def greet(name):
      return "hello " + name

Imperative form:
  plugin.register_function("add", "handlers.add")`,
	},

	"register_constant": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.MinArgs(args, 2); err != nil {
				return err
			}

			name, err := args[0].AsString()
			if err != nil {
				return err
			}

			RuntimeState.Lock()
			if RuntimeState.ServerStarted {
				fmt.Fprintf(os.Stderr, "warning: runtime.plugin.register_constant %q registered after start_server() — constant will not be served\n", name)
			}
			RuntimeState.PluginConstants[name] = args[1]
			RuntimeState.Unlock()

			return &object.Null{}
		},
		HelpText: `register_constant(name, value) - Register a constant exported by the plugin server

Parameters:
  name (str):  Constant name exposed to plugin clients.
  value (any): Value — any type that the plugin transport can encode (bool, int,
               float, string, list, dict, None).

Constants are included in the handshake schema so clients can read them
directly as attributes of the plugin library.

Example:
  import scriptling.runtime.plugin as plugin_srv

  plugin_srv.register_constant("VERSION", "1.0.0")
  plugin_srv.register_constant("MAX_RETRIES", 5)`,
	},

	"register_class": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.MinArgs(args, 1); err != nil {
				return err
			}

			// Bare decorator: @plugin.register_class
			if cls, ok := args[0].(*object.Class); ok {
				env := evaluator.GetEnvFromContext(ctx)
				ref := resolveModuleRef(env, cls.Name)
				if ref == "" {
					return errors.NewError("cannot determine module name for @plugin.register_class decorator")
				}
				RuntimeState.Lock()
				RuntimeState.PluginClasses[cls.Name] = ref
				RuntimeState.Unlock()
				return cls
			}

			handler, err := args[0].AsString()
			if err != nil {
				return err
			}

			// Derive the exposed class name from the last segment of the handler ref.
			name := handler
			for i := len(handler) - 1; i >= 0; i-- {
				if handler[i] == '.' {
					name = handler[i+1:]
					break
				}
			}

			RuntimeState.Lock()
			if RuntimeState.ServerStarted {
				fmt.Fprintf(os.Stderr, "warning: runtime.plugin.register_class %q registered after start_server() — class will not be served\n", name)
			}
			RuntimeState.PluginClasses[name] = handler
			RuntimeState.Unlock()

			return &object.Null{}
		},
		HelpText: `register_class(handler) - Register a class exported by the plugin server, or use as bare decorator

Decorator form:
  import scriptling.runtime.plugin as plugin

  @plugin.register_class
  class Config:
      def __init__(self):
          self.version = "1.0"

Imperative form:
  plugin.register_class("handlers.Config")

The exposed class name is derived from the class name (decorator) or the last
segment of the handler ref (imperative).`,
	},
}, nil, "Scriptling plugin server — declare this script as a plugin peer with full handshake support")

// RegisterRuntimePluginLibrary registers the plugin sub-library and exposes it
// as runtime.plugin on the parent library so that
// `import scriptling.runtime as rt; rt.plugin.serve(...)` works.
// Call this AFTER RegisterRuntimeLibraryAll. Intentionally not included in
// RegisterRuntimeLibraryAll — available only for the agent variant.
func RegisterRuntimePluginLibrary(registrar interface{ RegisterLibrary(*object.Library) }) {
	registrar.RegisterLibrary(PluginSubLibrary)
	if v, ok := runtimeParentLibraries.LoadAndDelete(registrar); ok {
		v.(*object.Library).Constants()["plugin"] = PluginSubLibrary.GetDict()
	}
}
