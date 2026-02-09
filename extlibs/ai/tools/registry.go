package tools

import (
	"context"
	"fmt"
	"strings"

	scriptlib "github.com/paularlott/scriptling"
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

// GetRegistryClass returns the Registry class
func GetRegistryClass() *object.Class {
	return object.NewClassBuilder("Registry").
		Method("__init__", registryConstructor).
		MethodWithHelp("add", registryAddMethod, `add(name, description, params, handler) - Add a tool to the registry

Parameters:
  name (str): Tool name
  description (str): Tool description
  params (dict): Parameter definitions (e.g., {"path": "string", "limit": "integer?"})
  handler (callable): Function to execute when tool is called

Example:
  registry.add("read_file", "Read a file", {"path": "string"}, read_func)`).
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

func registryAddMethod(self *object.Instance, ctx context.Context, name string, description string, params map[string]string, handler object.Object) object.Object {
	data, err := getRegistryData(self)
	if err != nil {
		return err
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

			// Map types
			jsonType := baseType
			switch baseType {
			case "number":
				jsonType = "integer"
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

	return scriptlib.FromGo(result)
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
