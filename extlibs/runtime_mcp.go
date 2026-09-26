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
// prompts) from script code, plus the register_request_* functions middleware
// uses to expose entries for the life of a single request. Decorator
// registrations are recorded per-interpreter in __mcp_registry; request
// registrations in the per-request accumulator carried on the context.
var MCPSubLibrary = func() *object.Library {
	functions := map[string]*object.Builtin{
		"tool": {
			Fn: mcpToolDecorator,
			HelpText: `tool(description, params=None, keywords=None, discoverable=False, ui=None, icons=None) - Decorator for MCP tools

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
  ui (dict, optional): Links this tool to a companion UI resource per the MCP
    Apps extension (https://github.com/modelcontextprotocol/ext-apps). A dict
    with an optional "resourceUri" (str, the ui:// resource — omit it for an
    "app"-only action tool with no view of its own, such as a form submission
    that's only ever called by a view that's already open) and optional
    "visibility" (list of "model" and/or "app"; defaults to both). At least
    one of "resourceUri" or "visibility" is required if "ui" is given at all.
  icons (list, optional): Visual identifiers for this tool's tools/list
    descriptor. Each element is a dict with a required "src" (str, an
    https:// URL or data: URI) and optional "mimeType", "sizes" (list of
    strings like "48x48"), and "theme" ("light" or "dark").

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
      return "\n".join(f"Hello, {name}!" for _ in range(times))

  @mcp.tool(description="Get the sales report",
            ui={"resourceUri": "ui://sales-dashboard/dashboard.html"},
            icons=[{"src": "https://example.com/sales.png", "mimeType": "image/png"}])
  def sales_report():
      return {"records": [...]}

  @mcp.tool(description="Add a sale record (called by the dashboard's own form, not the model)",
            ui={"visibility": ["app"]})
  def add_sale(date, product, amount):
      return {"records": [...]}`,
		},
		"resource": {
			Fn: mcpResourceDecorator,
			HelpText: `resource(uri, name="", description="", mime_type="", template=False) - Decorator for MCP resources

Decorates a function to register it as an MCP resource (or, with template=True,
a URI template like "user://docs/{path}"). For a static resource the function
takes no parameters; for a template its parameters are the URI's {var}
variables. A string return is the content; a dict/list return is JSON encoded.
The function runs on every resources/read, so content can change between reads.

Parameters:
  uri (str): Resource URI, or the URI template when template=True
  name (str, optional): Human-readable resource name (defaults to the URI)
  description (str, optional): Resource description
  mime_type (str, optional): Content type (default "text/plain", or
    "application/json" for dict/list results). Ignored for a "ui://" uri — the
    MCP Apps extension MUSTs that exact mimeType, so it's always set for you
  template (bool, optional): Treat uri as a {var} URI template (default: False)

Returns:
  A decorator function that registers the resource and returns the original function.

Example:
  import scriptling.runtime.mcp as mcp

  @mcp.resource("config://app", name="App config", mime_type="application/json")
  def app_config():
      return {"version": "1.0", "debug": False}

  @mcp.resource("user://docs/{path}", template=True, mime_type="text/markdown")
  def user_doc(path):
      return "# doc " + path`,
		},
		"prompt": {
			Fn: mcpPromptDecorator,
			HelpText: `prompt(description="", arguments=None) - Decorator for MCP prompts

Decorates a function to register it as an MCP prompt under the function's own
name. The function's parameters become the prompt's arguments and are passed
on every prompts/get. A string return is a single user message; a dict with a
"messages" list of {"role": "user"|"assistant", "content": "..."} builds a
multi-message prompt.

Parameters:
  description (str, optional): Prompt description
  arguments (list, optional): Argument metadata dicts with "name",
    "description" and "required". Inferred from the function signature when
    omitted.

Returns:
  A decorator function that registers the prompt and returns the original function.

Example:
  import scriptling.runtime.mcp as mcp

  @mcp.prompt(description="Summarise a document")
  def summarise(text):
      return "Summarise the following:\n\n" + text

  @mcp.prompt(description="Review code")
  def review(language, code):
      return {"messages": [
          {"role": "user", "content": "Review this " + language + " code:"},
          {"role": "assistant", "content": code},
      ]}`,
		},
		"skill": {
			Fn: mcpSkillDecorator,
			HelpText: `skill(files=None) - Decorator for MCP skills

Decorates a function to register it as an MCP skill (the Agent Skills format)
under the function's own name. The function takes no parameters and returns
the SKILL.md content, including its YAML frontmatter; the frontmatter's name
must match the function name and its description seeds the skill's listing,
exactly as for a SKILL.md file in the skills folder. The function runs once
when the server starts (skills are static content; they do not reload).

Parameters:
  files (dict, optional): Supporting files mapping file name to content
    string, served alongside SKILL.md as skill://<name>/<file>

Returns:
  A decorator function that registers the skill and returns the original function.

Example:
  import scriptling.runtime.mcp as mcp

  @mcp.skill(files={"regions.md": "eu-west: Europe\n"})
  def region-guide():
      return """---
  name: region-guide
  description: How to identify a region and set it
  ---

  # Region guide

  Read regions.md for the list of regions.
  """`,
		},
	}

	for name, builtin := range requestRegistrationBuiltins() {
		functions[name] = builtin
	}
	functions["transport"] = newTransportBuiltin(`transport() - How the MCP server is being served: "http", "stdio" or None

Lets one setup script work in every mode: over stdio the middleware never
runs, so registrations that middleware would gate per user must be made
unconditionally instead.

  import scriptling.runtime.mcp as mcp

  if mcp.transport() == "stdio":
      # No middleware over stdio: expose the extra tools to everyone.
      ...

Returns "http" when serving over HTTP (also from middleware and tool handlers
mid-request), "stdio" for the MCP stdio server, and None when the script is
not being served at all.`)

	return object.NewLibrary(RuntimeMCPLibraryName, functions, nil, "MCP tool, resource, and prompt registration via decorators")
}()

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

	var uiObj object.Object
	if u := kwargs.Get("ui"); u != nil {
		if _, ok := u.(*object.Dict); !ok {
			return errors.NewError("mcp.tool: ui must be a dict, got %s", u.Type())
		}
		uiObj = u
	}

	var iconsObj object.Object
	if ic := kwargs.Get("icons"); ic != nil {
		if _, ok := ic.(*object.List); !ok {
			return errors.NewError("mcp.tool: icons must be a list, got %s", ic.Type())
		}
		iconsObj = ic
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
				"type":         object.NewString("tool"),
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
			if uiObj != nil {
				entry.SetByString("ui", uiObj)
			}
			if iconsObj != nil {
				entry.SetByString("icons", iconsObj)
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

// decoratedFunction extracts the function being decorated and its name from
// the wrapper call arguments, shared by every registration decorator.
func decoratedFunction(fn string, wrapperArgs []object.Object) (*object.Function, object.Object) {
	if len(wrapperArgs) == 0 {
		return nil, errors.NewError("%s decorator requires a function", fn)
	}
	f, ok := wrapperArgs[0].(*object.Function)
	if !ok {
		return nil, errors.NewError("%s: decorated value must be a function, got %s", fn, wrapperArgs[0].Type())
	}
	if f.Name == "" {
		return nil, errors.NewError("%s: decorated function has no name", fn)
	}
	return f, nil
}

// appendRegistryEntry appends a registration entry dict to __mcp_registry in
// the current environment, creating the list on first use.
func appendRegistryEntry(fn string, ctx context.Context, entry *object.Dict) object.Object {
	env := evaluator.GetEnvFromContext(ctx)
	if env == nil {
		return errors.NewError("%s: no environment available", fn)
	}

	registryObj, ok := env.Get(MCPRegistryVar)
	if !ok {
		registryObj = &object.List{Elements: []object.Object{}}
		env.Set(MCPRegistryVar, registryObj)
	}

	registry, ok := registryObj.(*object.List)
	if !ok {
		return errors.NewError("%s: %s is not a list", fn, MCPRegistryVar)
	}

	registry.Elements = append(registry.Elements, entry)
	return nil
}

// mcpResourceDecorator implements the runtime.mcp.resource() builtin. The
// first positional arg is the URI; the rest of the shape matches
// register_request_resource.
func mcpResourceDecorator(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	if err := errors.MinArgs(args, 1); err != nil {
		return err
	}
	uri, err := args[0].AsString()
	if err != nil || uri == "" {
		return errors.NewError("mcp.resource: uri must be a non-empty string")
	}

	name := uri
	if n := kwargs.Get("name"); n != nil {
		if s, e := n.AsString(); e == nil && s != "" {
			name = s
		}
	}
	description := ""
	if d := kwargs.Get("description"); d != nil {
		if s, e := d.AsString(); e == nil {
			description = s
		}
	}
	mimeType := ""
	if m := kwargs.Get("mime_type"); m != nil {
		if s, e := m.AsString(); e == nil {
			mimeType = s
		}
	}
	template := false
	if t := kwargs.Get("template"); t != nil {
		if b, e := t.AsBool(); e == nil {
			template = b
		}
	}

	return &object.Builtin{
		Fn: func(ctx context.Context, _ object.Kwargs, wrapperArgs ...object.Object) object.Object {
			f, errObj := decoratedFunction("mcp.resource", wrapperArgs)
			if errObj != nil {
				return errObj
			}

			entry := object.NewStringDict(map[string]object.Object{
				"type":        object.NewString("resource"),
				"uri":         object.NewString(uri),
				"name":        object.NewString(name),
				"description": object.NewString(description),
				"mime_type":   object.NewString(mimeType),
				"template":    object.NewBoolean(template),
				"func":        object.NewString(f.Name),
			})

			if errObj := appendRegistryEntry("mcp.resource", ctx, entry); errObj != nil {
				return errObj
			}
			return wrapperArgs[0]
		},
	}
}

// mcpPromptDecorator implements the runtime.mcp.prompt() builtin. The prompt
// is named after the decorated function.
func mcpPromptDecorator(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	description := ""
	if len(args) > 0 {
		if s, e := args[0].AsString(); e == nil {
			description = s
		}
	}
	if d := kwargs.Get("description"); d != nil {
		if s, e := d.AsString(); e == nil {
			description = s
		}
	}
	var argumentsObj object.Object
	if a := kwargs.Get("arguments"); a != nil {
		if _, ok := a.(*object.List); !ok {
			return errors.NewError("mcp.prompt: arguments must be a list, got %s", a.Type())
		}
		argumentsObj = a
	}

	return &object.Builtin{
		Fn: func(ctx context.Context, _ object.Kwargs, wrapperArgs ...object.Object) object.Object {
			f, errObj := decoratedFunction("mcp.prompt", wrapperArgs)
			if errObj != nil {
				return errObj
			}

			entry := object.NewStringDict(map[string]object.Object{
				"type":        object.NewString("prompt"),
				"name":        object.NewString(f.Name),
				"description": object.NewString(description),
				"func":        object.NewString(f.Name),
			})
			if argumentsObj != nil {
				entry.SetByString("arguments", argumentsObj)
			}

			if errObj := appendRegistryEntry("mcp.prompt", ctx, entry); errObj != nil {
				return errObj
			}
			return wrapperArgs[0]
		},
	}
}

// mcpSkillDecorator implements the runtime.mcp.skill() builtin. The skill is
// named after the decorated function, which returns the SKILL.md content
// (frontmatter included) when the server scans the file.
func mcpSkillDecorator(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	var filesObj object.Object
	if f := kwargs.Get("files"); f != nil {
		if _, ok := f.(*object.Dict); !ok {
			return errors.NewError("mcp.skill: files must be a dict, got %s", f.Type())
		}
		filesObj = f
	}

	return &object.Builtin{
		Fn: func(ctx context.Context, _ object.Kwargs, wrapperArgs ...object.Object) object.Object {
			f, errObj := decoratedFunction("mcp.skill", wrapperArgs)
			if errObj != nil {
				return errObj
			}

			entry := object.NewStringDict(map[string]object.Object{
				"type": object.NewString("skill"),
				"name": object.NewString(f.Name),
				"func": object.NewString(f.Name),
			})
			if filesObj != nil {
				entry.SetByString("files", filesObj)
			}

			if errObj := appendRegistryEntry("mcp.skill", ctx, entry); errObj != nil {
				return errObj
			}
			return wrapperArgs[0]
		},
	}
}
