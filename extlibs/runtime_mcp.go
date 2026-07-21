package extlibs

import (
	"context"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/evaluator"
	"github.com/paularlott/scriptling/object"
)

const (
	RuntimeMCPLibraryName = "scriptling.runtime.mcp"

	// MCPRegistryVar is the environment variable name where the mcp.tool()
	// decorator records tool registrations. The folder scanner reads this after
	// evaluating a .py file to discover decorated tools.
	MCPRegistryVar = "__mcp_registry"
)

// MCPSubLibrary is the scriptling.runtime.mcp sub-library. It provides
// decorator functions for defining MCP tools (and in future, resources and
// prompts) from script code. Registrations are recorded per-interpreter in
// __mcp_registry rather than the global RuntimeState.
var MCPSubLibrary = object.NewLibrary(RuntimeMCPLibraryName, map[string]*object.Builtin{
	"tool": {
		Fn: mcpToolDecorator,
		HelpText: `tool(description, params=None, keywords=None, discoverable=False) - Decorator for MCP tools

Decorates a function to register it as an MCP tool. The function's parameters
become the tool's input schema; the return value becomes the tool response.

Parameters:
  description (str): Tool description shown to the AI
  params (dict, optional): Parameter metadata keyed by name. Each value is either
    a string (the description; type inferred from default or defaults to "string")
    or a dict with keys "type", "description", and optional "required".
  keywords (list, optional): Keywords for tool search/discovery
  discoverable (bool, optional): If True, tool is hidden from tools/list and
    only available via search (default: False)

Returns:
  A decorator function that registers the tool and returns the original function.

Example:
  import scriptling.runtime.mcp as mcp

  @mcp.tool(
      description="Calculate a mathematical expression",
      params={"expr": "Expression to evaluate (e.g. 2+3*4)"},
  )
  def calc(expr):
      return f"{expr} = {eval(expr)}"

  @mcp.tool(description="Greet someone", params={
      "name": "Name of the person",
      "times": {"type": "int", "description": "Number of greetings"},
  })
  def greet(name, times=1):
      return "\n".join(f"Hello, {name}!" for _ in range(times))`,
	},
}, nil, "MCP tool, resource, and prompt registration via decorators")

// mcpToolDecorator implements the runtime.mcp.tool() builtin. It accepts the
// decorator kwargs (description, params, keywords, discoverable) and returns a
// wrapper function that, when called with the decorated function, records the
// registration in __mcp_registry and returns the function unchanged.
func mcpToolDecorator(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	// mcp.tool(description, params=None, keywords=None, discoverable=False)
	// First positional arg is the description (required).
	if err := errors.MinArgs(args, 1); err != nil {
		return err
	}

	description, err := args[0].AsString()
	if err != nil {
		return errors.NewError("mcp.tool: description must be a string")
	}

	// Optional kwargs
	var paramsObj object.Object
	if p := kwargs.Get("params"); p != nil {
		paramsObj = p
	}

	var keywordsObj object.Object
	if k := kwargs.Get("keywords"); k != nil {
		keywordsObj = k
	}

	discoverable := false
	if d := kwargs.Get("discoverable"); d != nil {
		if b, e := d.AsBool(); e == nil {
			discoverable = b
		}
	}

	// Return a wrapper builtin that accepts the function being decorated.
	return &object.Builtin{
		Fn: func(ctx context.Context, _ object.Kwargs, wrapperArgs ...object.Object) object.Object {
			if len(wrapperArgs) == 0 {
				return errors.NewError("mcp.tool decorator requires a function")
			}

			fn := wrapperArgs[0]

			// Get the function name via __name__.
			var funcName string
			switch f := fn.(type) {
			case *object.Function:
				funcName = f.Name
			default:
				return errors.NewError("mcp.tool: decorated value must be a function, got %s", fn.Type())
			}

			if funcName == "" {
				return errors.NewError("mcp.tool: decorated function has no name")
			}

			// Build the registration entry dict.
			entry := object.NewStringDict(map[string]object.Object{
				"name":         object.NewString(funcName),
				"description":  object.NewString(description),
				"discoverable": object.NewBoolean(discoverable),
			})

			if paramsObj != nil {
				entry.SetByString("params", paramsObj)
			}
			if keywordsObj != nil {
				entry.SetByString("keywords", keywordsObj)
			}

			// Append to __mcp_registry in the current environment.
			env := evaluator.GetEnvFromContext(ctx)
			if env == nil {
				return errors.NewError("mcp.tool: no environment available")
			}

			registryObj, ok := env.Get(MCPRegistryVar)
			if !ok {
				// First registration in this interpreter — create the list.
				registryObj = &object.List{Elements: []object.Object{}}
				env.Set(MCPRegistryVar, registryObj)
			}

			registry, ok := registryObj.(*object.List)
			if !ok {
				return errors.NewError("mcp.tool: %s is not a list", MCPRegistryVar)
			}

			registry.Elements = append(registry.Elements, entry)

			// Return the function unchanged so it remains callable.
			return fn
		},
	}
}
