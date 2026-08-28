package extlibs

import (
	"context"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// requestRegistrationBuiltins returns the register_request_* functions of the
// scriptling.runtime.mcp sub-library. Middleware calls them to expose MCP
// tools, resources and prompts for the life of the request being served —
// per-user tool sets, scoped resources and so on. The entries are recorded in
// the per-request accumulator the server attaches alongside the stashed
// request; the server turns them into per-request providers once the
// middleware passes. Over the stdio transports middleware never runs, so
// scripts usually gate these calls on mcp.transport() != "stdio".
func requestRegistrationBuiltins() map[string]*object.Builtin {
	return map[string]*object.Builtin{
		"register_request_tool": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}
				name, err := args[0].AsString()
				if err != nil || name == "" {
					return errors.NewError("register_request_tool: name must be a non-empty string")
				}

				regs, regErr := requestRegistrations(ctx, "register_request_tool")
				if regErr != nil {
					return regErr
				}
				handler, herr := requiredKwargString(kwargs, "handler", "register_request_tool")
				if herr != nil {
					return herr
				}

				entry := object.NewStringDict(map[string]object.Object{
					"name":    object.NewString(name),
					"handler": object.NewString(handler),
				})
				copyOptionalKwargs(entry, kwargs, "description", "params", "keywords", "discoverable")
				regs.AddTool(entry)
				return &object.Null{}
			},
			HelpText: `register_request_tool(name, handler, description="", params=None, keywords=None, discoverable=False) - Register an MCP tool for this request

Call from middleware to expose a tool for the life of the request being
served: tools/list shows it and tools/call runs it, but only for requests
whose middleware registered it — which makes per-user tool sets possible.
Authorization is re-evaluated on every MCP message, since the middleware runs
per request.

Parameters:
  name (str): Tool name (static tools win on a name collision)
  handler (str): Handler function as "module.function", called with the tool
    arguments as keyword parameters on a fresh interpreter
  description (str): Tool description shown to the AI
  params (dict, optional): Parameter metadata keyed by name; each value is a
    string (description) or a dict with "type", "description" and "required"
  keywords (list, optional): Keywords for tool search/discovery
  discoverable (bool, optional): Hide from tools/list, expose via search only

Only meaningful while serving MCP over HTTP (in middleware); returns an error
otherwise. Inside the handler, mcp.tool.get_string() reads arguments and
mcp.tool.request_context() reads the middleware's context.

Example:
  import scriptling.runtime.mcp as mcp

  def auth(request):
      user = identify(request)
      if user == "admin":
          mcp.register_request_tool("restart_service",
              handler="admintools.restart",
              description="Restart a service",
              params={"service": {"type": "string", "description": "Service to restart", "required": True}},
          )
      return None`,
		},

		"register_request_resource": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}
				uri, err := args[0].AsString()
				if err != nil || uri == "" {
					return errors.NewError("register_request_resource: uri must be a non-empty string")
				}

				regs, regErr := requestRegistrations(ctx, "register_request_resource")
				if regErr != nil {
					return regErr
				}
				handler, herr := requiredKwargString(kwargs, "handler", "register_request_resource")
				if herr != nil {
					return herr
				}

				entry := object.NewStringDict(map[string]object.Object{
					"uri":     object.NewString(uri),
					"handler": object.NewString(handler),
				})
				copyOptionalKwargs(entry, kwargs, "name", "description", "mime_type", "template")
				regs.AddResource(entry)
				return &object.Null{}
			},
			HelpText: `register_request_resource(uri, handler, name, description="", mime_type="", template=False) - Register an MCP resource for this request

Call from middleware to expose a resource (or, with template=True, a URI
template like "user://docs/{path}") for the life of the request being served.
resources/list and resources/templates/list show it; resources/read runs the
handler. Static resources win on a URI collision.

Parameters:
  uri (str): Resource URI, or the URI template when template=True
  handler (str): Handler function as "module.function", called with the
    template variables as keyword parameters (and "__uri" holding the full
    URI); a string return is the content, a dict/list is JSON encoded
  name (str): Human-readable resource name
  description (str): Resource description
  mime_type (str): Content type (default "text/plain", or "application/json"
    for dict/list results)
  template (bool, optional): Treat uri as a {var} URI template (default: False)

Only meaningful while serving MCP over HTTP (in middleware); returns an error
otherwise.

Example:
  import scriptling.runtime.mcp as mcp

  def auth(request):
      user = identify(request)
      if user:
          mcp.register_request_resource("user://" + user + "/profile",
              handler="restools.profile",
              name="My profile",
              mime_type="application/json",
          )
      return None`,
		},

		"register_request_prompt": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}
				name, err := args[0].AsString()
				if err != nil || name == "" {
					return errors.NewError("register_request_prompt: name must be a non-empty string")
				}

				regs, regErr := requestRegistrations(ctx, "register_request_prompt")
				if regErr != nil {
					return regErr
				}
				handler, herr := requiredKwargString(kwargs, "handler", "register_request_prompt")
				if herr != nil {
					return herr
				}

				entry := object.NewStringDict(map[string]object.Object{
					"name":    object.NewString(name),
					"handler": object.NewString(handler),
				})
				copyOptionalKwargs(entry, kwargs, "description", "arguments")
				regs.AddPrompt(entry)
				return &object.Null{}
			},
			HelpText: `register_request_prompt(name, handler, description="", arguments=None) - Register an MCP prompt for this request

Call from middleware to expose a prompt for the life of the request being
served. prompts/list shows it; prompts/get renders it by running the handler
with the prompt arguments as keyword parameters. Static prompts win on a name
collision.

Parameters:
  name (str): Prompt name
  handler (str): Handler function as "module.function". A string return is a
    single user message; a dict with a "messages" list of
    {"role": "user"|"assistant", "content": "..."} builds a multi-message
    prompt
  description (str): Prompt description
  arguments (list, optional): Argument metadata dicts with "name",
    "description" and "required"

Only meaningful while serving MCP over HTTP (in middleware); returns an error
otherwise.

Example:
  import scriptling.runtime.mcp as mcp

  def auth(request):
      if identify(request):
          mcp.register_request_prompt("summarise_notes",
              handler="prompts.summarise",
              description="Summarise the caller's notes",
              arguments=[{"name": "topic", "description": "Notes to focus on", "required": True}],
          )
      return None`,
		},
	}
}

// requestRegistrations fetches the per-request accumulator from the context,
// returning a script error when there is none (the call is outside a served
// HTTP request — e.g. stdio transports, where middleware never runs).
func requestRegistrations(ctx context.Context, fn string) (*RequestRegistrations, object.Object) {
	regs := RegistrationsFrom(ctx)
	if regs == nil {
		return nil, errors.NewError("%s: only callable while serving a request over HTTP (from middleware); over stdio register statically instead, e.g. if mcp.transport() != \"stdio\": ...", fn)
	}
	return regs, nil
}

// requiredKwargString extracts a required string kwarg.
func requiredKwargString(kwargs object.Kwargs, key, fn string) (string, object.Object) {
	v := kwargs.Get(key)
	if v == nil {
		return "", errors.NewError("%s: missing required argument %q", fn, key)
	}
	s, err := v.AsString()
	if err != nil || s == "" {
		return "", errors.NewError("%s: %s must be a non-empty string", fn, key)
	}
	return s, nil
}

// copyOptionalKwargs copies the named kwargs, when present, into entry.
func copyOptionalKwargs(entry *object.Dict, kwargs object.Kwargs, names ...string) {
	for _, name := range names {
		if v := kwargs.Get(name); v != nil {
			entry.SetByString(name, v)
		}
	}
}
