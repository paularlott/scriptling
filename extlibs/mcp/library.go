package mcp

import (
	"context"
	"fmt"
	"strings"
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

		// Client(target, **kwargs) - Create a new MCP client (HTTP or stdio)
		FunctionWithHelp("Client", func(ctx context.Context, kwargs object.Kwargs, target string) (object.Object, error) {
			namespace := kwargs.MustGetString("namespace", "")

			// An http:// or https:// target is an HTTP MCP server; anything else
			// is treated as an executable path/command for a stdio MCP server.
			if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
				if kwargs.Has("args") {
					return nil, fmt.Errorf("mcp.Client: 'args' is only valid for stdio servers, not URL %q", target)
				}

				bearerToken := kwargs.MustGetString("bearer_token", "")
				var authProvider mcplib.AuthProvider
				if bearerToken != "" {
					authProvider = mcplib.NewBearerTokenAuth(bearerToken)
				}

				client := mcplib.NewClient(target, authProvider, namespace)
				return createClientInstance(client), nil
			}

			// stdio server: target is the command to launch.
			if kwargs.Has("bearer_token") {
				return nil, fmt.Errorf("mcp.Client: 'bearer_token' is only valid for HTTP servers, not command %q", target)
			}

			var args []string
			if kwargs.Has("args") {
				list, errObj := kwargs.GetList("args", nil)
				if errObj != nil {
					return nil, fmt.Errorf("mcp.Client: 'args' must be a list of strings")
				}
				for _, item := range list {
					s, sErr := item.AsString()
					if sErr != nil {
						return nil, fmt.Errorf("mcp.Client: 'args' must be a list of strings")
					}
					args = append(args, s)
				}
			}

			client, err := mcplib.NewStdioClient(target, args, namespace)
			if err != nil {
				return nil, fmt.Errorf("mcp.Client: failed to start stdio server %q: %w", target, err)
			}
			return createClientInstance(client), nil
		}, `Client(target, **kwargs) - Create a new MCP client (HTTP or stdio)

Creates a client for a remote MCP server. The transport is chosen from target:
an "http://" or "https://" URL connects over HTTP; any other value is treated
as a local executable that is launched as a stdio MCP server subprocess.

Parameters:
  target (str): HTTP(S) URL of the server, or path/command of a stdio server
  namespace (str, optional): Namespace prefixed to tool names (e.g. "t1" exposes "search" as "t1__search")
  bearer_token (str, optional): Bearer token for authentication (HTTP only)
  args (list, optional): Command-line arguments for the stdio server (stdio only)

Returns:
  MCPClient: A client instance with methods for interacting with the server

For stdio clients, call close() when done to shut the subprocess down.

Example:
  # HTTP server
  client = mcp.Client("https://api.example.com/mcp", namespace="t2", bearer_token="secret")

  # stdio server (a local executable)
  client = mcp.Client("/usr/local/bin/thebinary", args=["--server"], namespace="t1")

  tools = client.tools()
  for tool in tools:
    print(tool.name)
  client.close()`).
		Build()
}
