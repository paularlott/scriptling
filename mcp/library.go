package mcp

import (
	"context"
	"sync"

	mcplib "github.com/paularlott/mcp"
	scriptlib "github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

const (
	MCPLibraryName = "mcp"
	MCPLibraryDesc = "MCP (Model Context Protocol) tool interaction library"
)

var (
	library     *object.Library
	libraryOnce sync.Once
)

// WrapClient wraps an MCP client as a scriptling Object that can be
// passed into a script via SetObjectVar. This allows multiple clients
// to be used simultaneously.
func WrapClient(c *mcplib.Client) object.Object {
	return createClientInstance(c)
}

// Register registers the mcp library with the given registrar
// First call builds the library, subsequent calls just register it
func Register(registrar interface{ RegisterLibrary(string, *object.Library) }) {
	libraryOnce.Do(func() {
		library = buildLibrary()
	})
	registrar.RegisterLibrary(MCPLibraryName, library)
}

// buildLibrary builds the MCP library
func buildLibrary() *object.Library {
	return object.NewLibraryBuilder(MCPLibraryName, MCPLibraryDesc).

		// decode_response(response) - Decode a raw MCP tool response
		RawFunctionWithHelp("decode_response", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}

			// Convert args[0] to mcplib.ToolResponse
			response := &mcplib.ToolResponse{}

			// Try to convert from dict representation
			if dict, ok := args[0].(*object.Dict); ok {
				// Build response from dict
				for k, v := range dict.Pairs {
					switch k {
					case "structured_content":
						response.StructuredContent = scriptlib.ToGo(v.Value)
					case "content":
						if list, ok := v.Value.(*object.List); ok {
							for _, item := range list.Elements {
								if contentDict, ok := item.(*object.Dict); ok {
									content := mcplib.ToolContent{}
									for ck, cv := range contentDict.Pairs {
										switch ck {
										case "type":
											if t, err := cv.Value.AsString(); err == nil {
												content.Type = t
											}
										case "text":
											if t, err := cv.Value.AsString(); err == nil {
												content.Text = t
											}
										case "data":
											if t, err := cv.Value.AsString(); err == nil {
												content.Data = t
											}
										case "mimeType":
											if t, err := cv.Value.AsString(); err == nil {
												content.MimeType = t
											}
										}
									}
									response.Content = append(response.Content, content)
								}
							}
						}
					}
				}
			}

			return DecodeToolResponse(response)
		}, `decode_response(response) - Decode an MCP tool response

Decodes a raw MCP tool response into scriptling objects.

Parameters:
  response (dict): Raw tool response dict

Returns:
  object: Decoded response (parsed JSON or string)

Example:
  decoded = mcp.decode_response(raw_response)`).

		// new_client(base_url, **kwargs) - Create a new MCP client
		FunctionWithHelp("new_client", func(ctx context.Context, kwargs object.Kwargs, baseURL string) (object.Object, error) {
			// Get optional parameters from kwargs
			namespace := kwargs.MustGetString("namespace", "")
			bearerToken := kwargs.MustGetString("bearer_token", "")

			// Create auth provider if bearer token is provided
			var authProvider mcplib.AuthProvider
			if bearerToken != "" {
				authProvider = mcplib.NewBearerTokenAuth(bearerToken)
			}

			client := mcplib.NewClient(baseURL, authProvider, namespace)
			return createClientInstance(client), nil
		}, `new_client(base_url, **kwargs) - Create a new MCP client

Creates a new MCP client for connecting to a remote MCP server.

Parameters:
  base_url (str): URL of the MCP server
  namespace (str, optional): Namespace for tool names (e.g., "scriptling" makes tools available as "scriptling/tool_name")
  bearer_token (str, optional): Bearer token for authentication

Returns:
  MCPClient: A client instance with methods for interacting with the server

Example:
  # Without namespace or auth
  client = mcp.new_client("https://api.example.com/mcp")

  # With namespace only
  client = mcp.new_client("https://api.example.com/mcp", namespace="scriptling")

  # With bearer token only
  client = mcp.new_client("https://api.example.com/mcp", bearer_token="secret")

  # With both namespace and bearer token
  client = mcp.new_client("https://api.example.com/mcp", namespace="scriptling", bearer_token="secret")

  tools = client.tools()
  for tool in tools:
    print(tool.name)`).
		Build()
}
