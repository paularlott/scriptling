package mcp

import (
	"context"
	"sync"

	mcplib "github.com/paularlott/mcp"
	"github.com/paularlott/scriptling/object"
)

const (
	MCPLibraryName = "scriptling.mcp"
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
func Register(registrar interface{ RegisterLibrary(*object.Library) }) {
	libraryOnce.Do(func() {
		library = buildLibrary()
	})
	registrar.RegisterLibrary(library)
}

// buildLibrary builds the MCP library
func buildLibrary() *object.Library {
	return object.NewLibraryBuilder(MCPLibraryName, MCPLibraryDesc).

		// decode_response(response) - Decode a raw MCP tool response
		FunctionWithHelp("decode_response", func(ctx context.Context, responseMap map[string]any) object.Object {
			// Convert map to mcplib.ToolResponse
			response := &mcplib.ToolResponse{}

			if structuredContent, ok := responseMap["structured_content"]; ok {
				response.StructuredContent = structuredContent
			}

			if contentList, ok := responseMap["content"].([]any); ok {
				for _, item := range contentList {
					if contentMap, ok := item.(map[string]any); ok {
						content := mcplib.ToolContent{}
						if t, ok := contentMap["type"].(string); ok {
							content.Type = t
						}
						if t, ok := contentMap["text"].(string); ok {
							content.Text = t
						}
						if t, ok := contentMap["data"].(string); ok {
							content.Data = t
						}
						if t, ok := contentMap["mimeType"].(string); ok {
							content.MimeType = t
						}
						response.Content = append(response.Content, content)
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

		// Client(base_url, **kwargs) - Create a new MCP client
		FunctionWithHelp("Client", func(ctx context.Context, kwargs object.Kwargs, baseURL string) (object.Object, error) {
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
		}, `Client(base_url, **kwargs) - Create a new MCP client

Creates a new MCP client for connecting to a remote MCP server.

Parameters:
  base_url (str): URL of the MCP server
  namespace (str, optional): Namespace for tool names (e.g., "scriptling" makes tools available as "scriptling/tool_name")
  bearer_token (str, optional): Bearer token for authentication

Returns:
  MCPClient: A client instance with methods for interacting with the server

Example:
  # Without namespace or auth
  client = mcp.Client("https://api.example.com/mcp")

  # With namespace only
  client = mcp.Client("https://api.example.com/mcp", namespace="scriptling")

  # With bearer token only
  client = mcp.Client("https://api.example.com/mcp", bearer_token="secret")

  # With both namespace and bearer token
  client = mcp.Client("https://api.example.com/mcp", namespace="scriptling", bearer_token="secret")

  tools = client.tools()
  for tool in tools:
    print(tool.name)`).
		Build()
}
