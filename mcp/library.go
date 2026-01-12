package mcp

import (
	"context"
	"sync"

	mcplib "github.com/paularlott/mcp"
	scriptlib "github.com/paularlott/scriptling"
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
			if len(args) < 1 {
				return &object.Error{Message: "decode_response requires 1 argument: response"}
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

		// new_client(base_url, auth, prefix) - Create a new MCP client
		FunctionWithHelp("new_client", func(baseURL string, opts ...object.Object) (object.Object, error) {
			var authProvider mcplib.AuthProvider
			prefix := ""

			// Parse optional arguments
			for i, opt := range opts {
				if i == 0 {
					// First optional arg could be auth dict or prefix string
					if dict, ok := opt.(*object.Dict); ok {
						authProvider = getMCPAuth(dict)
					} else if str, err := opt.AsString(); err == nil {
						prefix = str
					}
				} else if i == 1 {
					// Second optional arg is auth dict (if first was prefix)
					if dict, ok := opt.(*object.Dict); ok {
						authProvider = getMCPAuth(dict)
					}
				}
			}

			client := mcplib.NewClient(baseURL, authProvider, prefix, "")
			return createClientInstance(client), nil
		}, `new_client(base_url, auth, prefix) - Create a new MCP client

Creates a new MCP client for connecting to a remote MCP server.

Parameters:
  base_url (str): URL of the MCP server
  auth (dict, optional): Auth configuration with "type" and "token"/"credentials"
  prefix (str, optional): Prefix for tool names (e.g., "scriptling" makes tools available as "scriptling/tool_name")

Returns:
  MCPClient: A client instance with methods for interacting with the server

Example:
  # Without prefix
  client = mcp.new_client("https://api.example.com/mcp")

  # With prefix
  client = mcp.new_client("https://api.example.com/mcp", "scriptling")

  # With auth and prefix
  client = mcp.new_client("https://api.example.com/mcp", {"type": "bearer", "token": "secret"}, "scriptling")

  tools = client.tools()
  for tool in tools:
    print(tool.name)`).
		Build()
}

// getMCPAuth creates an mcp.AuthProvider from auth configuration
func getMCPAuth(authDict *object.Dict) mcplib.AuthProvider {
	authType := ""
	var token string

	for k, v := range authDict.Pairs {
		switch k {
		case "type":
			authType, _ = v.Value.AsString()
		case "token":
			token, _ = v.Value.AsString()
		}
	}

	if authType == "bearer" && token != "" {
		return mcplib.NewBearerTokenAuth(token)
	}

	return nil
}
