package mcp

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	mcplib "github.com/paularlott/mcp"
	"github.com/paularlott/mcp/pool"
	"github.com/paularlott/scriptling/extlibs/netsecurity"
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

// guardedPool adapts a netsecurity-guarded *http.Client to the mcp/pool.HTTPPool
// interface expected by paularlott/mcp's client constructors.
type guardedPool struct {
	client *http.Client
}

func (p *guardedPool) GetHTTPClient() *http.Client { return p.client }

// httpPoolFor returns an HTTPPool that routes through guard's checked
// transport, or nil (the default, unrestricted pool) when guard is nil.
func httpPoolFor(guard *netsecurity.Guard) pool.HTTPPool {
	if guard == nil {
		return nil
	}
	return &guardedPool{client: guard.HTTPClient()}
}

// defaultClientTimeout is the per-request HTTP timeout for clients created
// via mcp.Client(). The library pool's own default (5 minutes) is sized for
// Go hosts federating slow LLM-backed tools; scripts get a tighter default
// and can opt up per server with the timeout kwarg (seconds) when they know
// a server's tools are slow.
const defaultClientTimeout = 30 * time.Second

// clientPool adapts an *http.Client to the library's HTTPPool interface.
type clientPool struct {
	client *http.Client
}

func (p *clientPool) GetHTTPClient() *http.Client { return p.client }

// poolWithTimeout returns an HTTPPool whose client applies the given
// per-request HTTP timeout. The underlying pooled (or policy-guarded) client
// is shallow-copied so the shared transport and its connection pool survive
// while the copy gains the timeout; the library pool's own Timeout config
// field is deliberately inert (its client never sets one, to keep streaming
// responses alive), so this is the only path that actually applies one.
//
// The timeout covers the whole request including reading the response body,
// so a server that streams a response for longer than the timeout is cut
// off: raise the timeout for servers whose tools are known to be slow.
func poolWithTimeout(guard *netsecurity.Guard, d time.Duration) pool.HTTPPool {
	base := pool.GetPool().GetHTTPClient()
	if guard != nil {
		base = guard.HTTPClient()
	}
	hc := *base
	hc.Timeout = d
	return &clientPool{client: &hc}
}

// declareUIAppsSupport advertises this client's own support for the MCP
// Apps extension (SEP-1865) to the remote server a script connects to via
// mcp.Client(...). Without this, a spec-conformant remote server that only
// attaches _meta.ui for clients that declared
// capabilities.extensions[io.modelcontextprotocol/ui] has no way to know
// this client can handle one, and silently serves a plain-text-only tool
// instead — MCP Apps then quietly never works for that server, with no
// error anywhere to explain why. Unconditional: the cost to a
// non-supporting host is nil, it just ignores unknown _meta.
func declareUIAppsSupport(client *mcplib.Client) {
	client.DeclareExtension(mcplib.UIAppsExtensionID, map[string]any{
		"mimeTypes": []string{mcplib.UIAppMimeType},
	})
}

// Register registers the mcp library with the given registrar.
// cfg is an optional outbound network policy: when provided (non-nil), every
// HTTP-transport client created via mcp.Client() is restricted to it (an
// invalid policy fails closed rather than falling back to unrestricted
// access). Stdio clients launch a local subprocess and are unaffected — the
// policy only governs network access. Omitting cfg preserves the previous,
// unrestricted behaviour.
//
// First call with no cfg builds and caches the unrestricted library;
// subsequent calls just register it. A call with cfg always builds a fresh,
// guard-bound library.
func Register(registrar interface{ RegisterLibrary(*object.Library) }, cfg ...*netsecurity.Config) {
	lib := defaultLibrary()
	if len(cfg) > 0 && cfg[0] != nil {
		guard, gerr := netsecurity.NewGuard(cfg[0])
		if gerr != nil {
			guard = netsecurity.FailClosed(gerr)
		}
		lib = buildLibrary(guard)
	}
	registrar.RegisterLibrary(lib)
}

// defaultLibrary returns the cached, unrestricted library (thread-safe singleton).
func defaultLibrary() *object.Library {
	libraryOnce.Do(func() {
		library = buildLibrary(nil)
	})
	return library
}

// buildLibrary builds the MCP library. A nil guard leaves HTTP clients
// created by mcp.Client() unrestricted (the previous behaviour).
func buildLibrary(guard *netsecurity.Guard) *object.Library {
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
				if kwargs.Has("args") || kwargs.Has("env") {
					return nil, fmt.Errorf("mcp.Client: 'args' and 'env' are only valid for stdio servers, not URL %q", target)
				}

				bearerToken := kwargs.MustGetString("bearer_token", "")
				var authProvider mcplib.AuthProvider
				if bearerToken != "" {
					authProvider = mcplib.NewBearerTokenAuth(bearerToken)
				}

				timeout := defaultClientTimeout
				if kwargs.Has("timeout") {
					tv := kwargs.Get("timeout")
					switch tv.(type) {
					case *object.Integer, *object.Float:
					default:
						return nil, fmt.Errorf("mcp.Client: 'timeout' must be a positive number of seconds")
					}
					secs, terr := tv.CoerceFloat()
					if terr != nil || secs <= 0 {
						return nil, fmt.Errorf("mcp.Client: 'timeout' must be a positive number of seconds")
					}
					timeout = time.Duration(secs * float64(time.Second))
				}

				client := mcplib.NewClientWithPool(target, authProvider, namespace, poolWithTimeout(guard, timeout))
				declareUIAppsSupport(client)
				return createClientInstance(client), nil
			}

			// stdio server: target is the command to launch.
			if kwargs.Has("bearer_token") {
				return nil, fmt.Errorf("mcp.Client: 'bearer_token' is only valid for HTTP servers, not command %q", target)
			}
			if kwargs.Has("timeout") {
				return nil, fmt.Errorf("mcp.Client: 'timeout' is only valid for HTTP servers, not command %q", target)
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

			var env []string
			if kwargs.Has("env") {
				list, errObj := kwargs.GetList("env", nil)
				if errObj != nil {
					return nil, fmt.Errorf("mcp.Client: 'env' must be a list of strings")
				}
				for _, item := range list {
					s, sErr := item.AsString()
					if sErr != nil {
						return nil, fmt.Errorf("mcp.Client: 'env' must be a list of strings")
					}
					env = append(env, s)
				}
			}

			client, err := mcplib.NewStdioClient(target, args, namespace, mcplib.WithClientExtraEnv(env...))
			if err != nil {
				return nil, fmt.Errorf("mcp.Client: failed to start stdio server %q: %w", target, err)
			}
			declareUIAppsSupport(client)
			return createClientInstance(client), nil
		}, `Client(target, **kwargs) - Create a new MCP client (HTTP or stdio)

Creates a client for a remote MCP server. The transport is chosen from target:
an "http://" or "https://" URL connects over HTTP; any other value is treated
as a local executable that is launched as a stdio MCP server subprocess.

Parameters:
  target (str): HTTP(S) URL of the server, or path/command of a stdio server
  namespace (str, optional): Namespace prefixed to tool names (e.g. "t1" exposes "search" as "t1__search")
  bearer_token (str, optional): Bearer token for authentication (HTTP only)
  timeout (number, optional): Per-request HTTP timeout in seconds, default 30.
                 Raise it for servers whose tools are known to be slow
                 (e.g. LLM-backed tools taking minutes). HTTP only.
  args (list, optional): Command-line arguments for the stdio server (stdio only)
  env (list, optional): Extra KEY=value environment variables for the stdio subprocess (stdio only); merged on top of the inherited environment

Returns:
  MCPClient: A client instance with methods for interacting with the server

For stdio clients, call close() when done to shut the subprocess down.

Example:
  # HTTP server
  client = mcp.Client("https://api.example.com/mcp", namespace="t2", bearer_token="secret")

  # stdio server (a local executable)
  client = mcp.Client("/usr/local/bin/thebinary", args=["--server"], namespace="t1")

  # stdio server with extra environment variables
  client = mcp.Client("npx", args=["-y", "@modelcontextprotocol/server-filesystem", "/data"], env=["FS_ROOT=/data", "LOG_LEVEL=debug"])

  tools = client.tools()
  for tool in tools:
    print(tool.name)
  client.close()`).
		Build()
}
