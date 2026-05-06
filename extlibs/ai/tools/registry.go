package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/paularlott/scriptling/conversion"

	"github.com/paularlott/scriptling/object"
)

const (
	ToolsLibraryName = "scriptling.ai.tools"
)

// Register registers the tools library
func Register(registrar interface{ RegisterLibrary(*object.Library) }) {
	registrar.RegisterLibrary(buildLibrary())
}

func buildLibrary() *object.Library {
	builder := object.NewLibraryBuilder(ToolsLibraryName, "Tool schema builder for AI agents")
	builder.Constant("Registry", GetRegistryClass())
	return builder.Build()
}

// typeAliases maps user-friendly type names to valid JSON Schema types.
// Keys are the accepted input strings (before stripping the optional "?" suffix).
// Values are the JSON Schema type names emitted in the tool schema.
var typeAliases = map[string]string{
	// Canonical JSON Schema types (pass through unchanged)
	"string":  "string",
	"integer": "integer",
	"number":  "number",
	"boolean": "boolean",
	"array":   "array",
	"object":  "object",
	// Python-style aliases
	"int":   "integer",
	"float": "number",
	"str":   "string",
	"bool":  "boolean",
	"dict":  "object",
	"list":  "array",
}

// validTypeList is the sorted list of accepted type names for error messages.
var validTypeList = "array, bool, boolean, dict, float, int, integer, list, number, object, str, string"

// GetRegistryClass returns the Registry class
func GetRegistryClass() *object.Class {
	return object.NewClassBuilder("Registry").
		Method("__init__", registryConstructor).
		MethodWithHelp("add", registryAddMethod, `add(name, description, params, handler) - Add a tool to the registry

Parameters:
  name (str): Tool name
  description (str): Tool description
  params (dict): Parameter definitions, mapping parameter name to a type string.
                 Accepted types: string, integer, number, boolean, array, object.
                 Aliases: int -> integer, float -> number, str -> string,
                 bool -> boolean, dict -> object, list -> array.
                 Append "?" to mark the parameter as optional (e.g. "string?").
  handler (callable): Function to execute when tool is called

Example:
  registry.add("read_file", "Read a file", {"path": "string", "limit": "int?"}, read_func)`).
		MethodWithHelp("build", registryBuildMethod, `build() - Build OpenAI-compatible tool schemas

Returns:
  list: List of tool schema dicts suitable for passing to AI completion requests

Example:
  # Build tool schemas for use with Agent (recommended)
  tools = ai.ToolRegistry()
  tools.add("read_file", "Read a file", {"path": "string"}, read_func)
  schemas = tools.build()
  bot = agent.Agent(client, tools=tools, model="gpt-4")

  # Build tool schemas for direct completion calls
  tools = ai.ToolRegistry()
  tools.add("get_time", "Get current time", {}, lambda args: "12:00 PM")
  schemas = tools.build()
  response = client.completion("gpt-4", [{"role": "user", "content": "What time is it?"}], tools=schemas)

  # With streaming completion
  response = client.completion_stream("gpt-4", [{"role": "user", "content": "What time is it?"}], tools=schemas)`).
		MethodWithHelp("get_handler", registryGetHandlerMethod, `get_handler(name) - Get tool handler by name

Parameters:
  name (str): Tool name

Returns:
  callable: Tool handler function`).
		Build()
}

type registryData struct {
	tools    []toolDef
	handlers map[string]object.Object
}

type toolDef struct {
	name        string
	description string
	params      map[string]string
}

func registryConstructor(self *object.Instance, ctx context.Context) object.Object {
	if self.Fields == nil {
		self.Fields = make(map[string]object.Object)
	}
	self.Fields["_data"] = &object.ClientWrapper{
		TypeName: "RegistryData",
		Client: &registryData{
			tools:    []toolDef{},
			handlers: make(map[string]object.Object),
		},
	}
	return &object.Null{}
}

func getRegistryData(self *object.Instance) (*registryData, *object.Error) {
	wrapper, ok := self.Fields["_data"].(*object.ClientWrapper)
	if !ok {
		return nil, &object.Error{Message: "Registry: missing internal data"}
	}
	data, ok := wrapper.Client.(*registryData)
	if !ok {
		return nil, &object.Error{Message: "Registry: invalid internal data"}
	}
	return data, nil
}

// BuildSchemasObject returns the built tool schema list for a registry instance.
func BuildSchemasObject(self *object.Instance, ctx context.Context) object.Object {
	return registryBuildMethod(self, ctx)
}

// GetHandlerObject returns the registered tool handler for a given name.
func GetHandlerObject(self *object.Instance, name string) (object.Object, *object.Error) {
	data, err := getRegistryData(self)
	if err != nil {
		return nil, err
	}

	handler, ok := data.handlers[name]
	if !ok {
		return nil, &object.Error{Message: fmt.Sprintf("no handler for tool: %s", name)}
	}

	return handler, nil
}

func registryAddMethod(self *object.Instance, ctx context.Context, name string, description string, params map[string]string, handler object.Object) object.Object {
	data, err := getRegistryData(self)
	if err != nil {
		return err
	}

	// Validate parameter types up front so the caller gets a clear error
	// at the point of registration rather than at build() time.
	for paramName, paramType := range params {
		baseType := strings.TrimSuffix(paramType, "?")
		if _, ok := typeAliases[baseType]; !ok {
			return &object.Error{Message: fmt.Sprintf(
				"Registry.add: unknown type %q for parameter %q in tool %q. Valid types: %s (append \"?\" for optional)",
				paramType, paramName, name, validTypeList)}
		}
	}

	data.tools = append(data.tools, toolDef{
		name:        name,
		description: description,
		params:      params,
	})
	data.handlers[name] = handler

	return &object.Null{}
}

func registryBuildMethod(self *object.Instance, ctx context.Context) object.Object {
	data, err := getRegistryData(self)
	if err != nil {
		return err
	}

	result := make([]any, 0, len(data.tools))
	for _, tool := range data.tools {
		properties := make(map[string]any)
		required := []string{}

		for paramName, paramType := range tool.params {
			isOptional := strings.HasSuffix(paramType, "?")
			baseType := strings.TrimSuffix(paramType, "?")

			// Map aliases to canonical JSON Schema types. Unknown types are
			// rejected at add() time, so anything missing here indicates a
			// programmer error rather than user input.
			jsonType, ok := typeAliases[baseType]
			if !ok {
				return &object.Error{Message: fmt.Sprintf(
					"Registry.build: unknown type %q for parameter %q in tool %q",
					paramType, paramName, tool.name)}
			}

			properties[paramName] = map[string]any{"type": jsonType}

			if !isOptional {
				required = append(required, paramName)
			}
		}

		result = append(result, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        tool.name,
				"description": tool.description,
				"parameters": map[string]any{
					"type":       "object",
					"properties": properties,
					"required":   required,
				},
			},
		})
	}

	return conversion.FromGo(result)
}

func registryGetHandlerMethod(self *object.Instance, ctx context.Context, name string) object.Object {
	data, err := getRegistryData(self)
	if err != nil {
		return err
	}

	handler, ok := data.handlers[name]
	if !ok {
		return &object.Error{Message: fmt.Sprintf("no handler for tool: %s", name)}
	}

	return handler
}
