package mcp

import (
	"context"
	"sync"

	mcplib "github.com/paularlott/mcp"
	scriptlib "github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/object"
)

// ClientInstance wraps an MCP client for use in scriptling
type ClientInstance struct {
	client *mcplib.Client
}

// GetClient returns the underlying MCP client
func (c *ClientInstance) GetClient() *mcplib.Client {
	return c.client
}

var (
	mcpClientClass     *object.Class
	mcpClientClassOnce sync.Once
)

// GetMCPClientClass returns the MCP Client class (thread-safe singleton)
func GetMCPClientClass() *object.Class {
	mcpClientClassOnce.Do(func() {
		mcpClientClass = buildMCPClientClass()
	})
	return mcpClientClass
}

// buildMCPClientClass builds the MCP Client class
func buildMCPClientClass() *object.Class {
	return object.NewClassBuilder("MCPClient").

		MethodWithHelp("tools", toolsMethod, `tools() - List available tools

Lists all tools available from this MCP server.

Returns:
  list: List of tool dicts with name, description, input_schema

Example:
  tools = client.tools()
  for tool in tools:
    print(tool.name + ": " + tool.description)`).

		MethodWithHelp("call_tool", callToolMethod, `call_tool(name, arguments) - Execute a tool

Executes a tool by name with the provided arguments.

Parameters:
  name (str): Tool name to execute
  arguments (dict): Tool arguments

Returns:
  dict: Decoded tool response

Example:
  result = client.call_tool("search", {"query": "golang"})
  print(result)`).

		MethodWithHelp("refresh_tools", refreshToolsMethod, `refresh_tools() - Refresh the tool cache

Explicitly refreshes the cached list of tools from the server.

Returns:
  null

Example:
  client.refresh_tools()`).

		MethodWithHelp("tool_search", toolSearchMethod, `tool_search(query, **kwargs) - Search for tools

Searches for tools using the tool_search MCP tool. This is useful when the
server has many tools registered via a discovery registry.

Parameters:
  query (str): Search query for tool names, descriptions, and keywords
  max_results (int, optional): Maximum number of results (default: 10)

Returns:
  list: List of matching tool dicts

Example:
  # Get up to 10 weather-related tools (default)
  results = client.tool_search("weather")

  # Get up to 5 database tools
  results = client.tool_search("database", max_results=5)`).

		MethodWithHelp("execute_discovered", executeDiscoveredMethod, `execute_discovered(name, arguments) - Execute a discovered tool

Executes a tool by name using the execute_tool MCP tool. This is the only way
to call tools that were discovered via tool_search.

Parameters:
  name (str): Tool name to execute
  arguments (dict): Tool arguments

Returns:
  dict: Tool response

Example:
  result = client.execute_discovered("custom_tool", {"param": "value"})`).

		Build()
}

// getClientInstance extracts the ClientInstance from an object.Instance
func getMCPClientInstance(instance *object.Instance) (*ClientInstance, *object.Error) {
	wrapper, ok := object.GetClientField(instance, "_client")
	if !ok {
		return nil, &object.Error{Message: "MCPClient: missing internal client reference"}
	}
	if wrapper.Client == nil {
		return nil, &object.Error{Message: "MCPClient: client is nil"}
	}
	ci, ok := wrapper.Client.(*ClientInstance)
	if !ok {
		return nil, &object.Error{Message: "MCPClient: invalid internal client reference"}
	}
	return ci, nil
}

// tools method implementation
func toolsMethod(self *object.Instance, ctx context.Context) object.Object {
	ci, cerr := getMCPClientInstance(self)
	if cerr != nil {
		return cerr
	}

	if ci.client == nil {
		return &object.Error{Message: "tools: no client configured"}
	}

	tools, err := ci.client.ListTools(ctx)
	if err != nil {
		return &object.Error{Message: "failed to get tools: " + err.Error()}
	}

	return convertToolsToList(tools)
}

// call_tool method implementation
func callToolMethod(self *object.Instance, ctx context.Context, name string, arguments map[string]any) object.Object {
	ci, cerr := getMCPClientInstance(self)
	if cerr != nil {
		return cerr
	}

	if ci.client == nil {
		return &object.Error{Message: "call_tool: no client configured"}
	}

	response, err := ci.client.CallTool(ctx, name, arguments)
	if err != nil {
		return &object.Error{Message: "tool execution failed: " + err.Error()}
	}

	return DecodeToolResponse(response)
}

// refresh_tools method implementation
func refreshToolsMethod(self *object.Instance, ctx context.Context) object.Object {
	ci, cerr := getMCPClientInstance(self)
	if cerr != nil {
		return cerr
	}

	if ci.client == nil {
		return &object.Error{Message: "refresh_tools: no client configured"}
	}

	if err := ci.client.RefreshToolCache(ctx); err != nil {
		return &object.Error{Message: "failed to refresh tools: " + err.Error()}
	}

	return &object.Null{}
}

// tool_search method implementation
func toolSearchMethod(self *object.Instance, ctx context.Context, kwargs object.Kwargs, query string) object.Object {
	ci, cerr := getMCPClientInstance(self)
	if cerr != nil {
		return cerr
	}

	if ci.client == nil {
		return &object.Error{Message: "tool_search: no client configured"}
	}

	// Get max_results from kwargs (default to 10)
	maxResults := int(kwargs.MustGetInt("max_results", 10))

	results, toolErr := ci.client.ToolSearch(ctx, query, maxResults)
	if toolErr != nil {
		return &object.Error{Message: "tool search failed: " + toolErr.Error()}
	}

	return scriptlib.FromGo(results)
}

// execute_discovered method implementation
func executeDiscoveredMethod(self *object.Instance, ctx context.Context, name string, arguments map[string]any) object.Object {
	ci, cerr := getMCPClientInstance(self)
	if cerr != nil {
		return cerr
	}

	if ci.client == nil {
		return &object.Error{Message: "execute_discovered: no client configured"}
	}

	response, err := ci.client.ExecuteDiscoveredTool(ctx, name, arguments)
	if err != nil {
		return &object.Error{Message: "tool execution failed: " + err.Error()}
	}

	return DecodeToolResponse(response)
}

// createClientInstance creates a new scriptling Instance wrapping an MCP client
func createClientInstance(client *mcplib.Client) *object.Instance {
	return &object.Instance{
		Class: GetMCPClientClass(),
		Fields: map[string]object.Object{
			"_client": &object.ClientWrapper{
				TypeName: "MCPClient",
				Client:   &ClientInstance{client: client},
			},
		},
	}
}

// convertToolsToList converts a slice of MCP tools to a scriptling List
func convertToolsToList(tools []mcplib.MCPTool) object.Object {
	toolList := make([]object.Object, 0, len(tools))
	for _, tool := range tools {
		toolDict := &object.Dict{
			Pairs: map[string]object.DictPair{
				"name": {
					Key:   &object.String{Value: "name"},
					Value: &object.String{Value: tool.Name},
				},
				"description": {
					Key:   &object.String{Value: "description"},
					Value: &object.String{Value: tool.Description},
				},
			},
		}
		if tool.InputSchema != nil {
			toolDict.Pairs["input_schema"] = object.DictPair{
				Key:   &object.String{Value: "input_schema"},
				Value: scriptlib.FromGo(tool.InputSchema),
			}
		}
		toolList = append(toolList, toolDict)
	}
	return &object.List{Elements: toolList}
}
