package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
	mcp_lib "github.com/paularlott/mcp"
	"github.com/paularlott/mcp/toolmetadata"
	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/mcp"
	"github.com/paularlott/scriptling/scriptling-cli/bootstrap"
	mcpcli "github.com/paularlott/scriptling/scriptling-cli/mcp"
	"github.com/paularlott/scriptling/scriptling-cli/pack"
	"github.com/paularlott/scriptling/scriptling-cli/setup"
)

// setupMCP initializes the MCP server if configured
func (s *Server) setupMCP() error {
	s.mcpHandler = &reloadableMCPHandler{}
	s.debounceDuration = 500 * time.Millisecond

	server, err := s.createMCPServer()
	if err != nil {
		return err
	}

	s.mcpHandler.server.Store(server)

	if s.config.MCPToolsDir != "" {
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			Log.Warn("Failed to create file watcher, auto-reload disabled", "error", err)
		} else {
			if err := watcher.Add(s.config.MCPToolsDir); err != nil {
				Log.Warn("Failed to watch tools folder, auto-reload disabled", "error", err)
				watcher.Close()
			} else {
				s.watcher = watcher
				Log.Info("Watching tools folder for changes", "path", s.config.MCPToolsDir)
			}
		}
	}

	return nil
}

// createMCPServer creates a new MCP server with all tools registered
func (s *Server) createMCPServer() (*mcp_lib.Server, error) {
	server := mcp_lib.NewServer("scriptling-server", "1.0.0")
	server.SetInstructions("Execute Scriptling tools from the tools folder.")

	if s.config.MCPExecTool {
		s.registerExecTool(server)
	}

	if s.config.MCPToolsDir != "" {
		tools, err := mcpcli.ScanToolsFolder(s.config.MCPToolsDir)
		if err != nil {
			return nil, err
		}

		for toolName, meta := range tools {
			scriptPath := filepath.Join(s.config.MCPToolsDir, toolName+".py")
			tool := toolmetadata.BuildMCPTool(toolName, meta)
			handler, err := createMCPToolHandler(scriptPath, s.config.LibDirs, s.config.AllowedPaths, s.packLoader)
			if err != nil {
				return nil, fmt.Errorf("failed to load tool %s: %w", toolName, err)
			}
			server.RegisterTool(tool, handler)

			mode := "native"
			if meta.Discoverable {
				mode = "discoverable"
			}
			Log.Info("Registered MCP tool", "name", toolName, "params", len(meta.Parameters), "mode", mode)
		}
	}

	return server, nil
}

// registerExecTool registers the built-in code execution tool
func (s *Server) registerExecTool(server *mcp_lib.Server) {
	server.RegisterTool(
		mcp_lib.NewTool("execute_script",
			`Execute Scriptling code and return the result. Scriptling is a Python 3-like scripting language.

KEY SYNTAX RULES:
- Use True/False (capitalized), None for null
- Use elif (not else if)
- 4-space indentation for blocks
- No nested classes, no multiple inheritance, no generators/yield

HTTP & JSON:
- HTTP response is an object: response.status_code, response.body, response.headers
- Use json.loads(str) and json.dumps(obj) for JSON
- Use requests.get(url, options), requests.post(url, body, options) for HTTP
- Default HTTP timeout is 5 seconds
- HTTP options dict: {"timeout": 10, "headers": {"Authorization": "Bearer token"}}

COMMON PATTERNS:
- Dict iteration: for item in items(dict): key=item[0], value=item[1]
- List append: append(list, item) modifies in-place
- Use join() for string building in loops: result = "".join(parts)
- Error handling: try/except/finally, raise "message" or raise ValueError("msg")

RETURNING RESULTS:
- print() output is captured and returned automatically
- For structured data: import scriptling.mcp.tool; tool.return_object(data)
- For text: tool.return_string(text)
- Use help(topic) for built-in help: help("builtins"), help("json"), help("requests")`,
			mcp_lib.String("code", "Scriptling code to execute (Python 3-like syntax)", mcp_lib.Required()),
		),
		func(ctx context.Context, req *mcp_lib.ToolRequest) (*mcp_lib.ToolResponse, error) {
			code, _ := req.String("code")
			p := scriptling.New()
			setup.Scriptling(p, s.config.LibDirs, false, s.config.AllowedPaths, Log)

			response, exitCode, err := mcp.RunToolScript(ctx, p, code, map[string]interface{}{})
			if err != nil && exitCode != 0 {
				return nil, fmt.Errorf("execution error: %w", err)
			}
			return mcp_lib.NewToolResponseText(response), nil
		},
	)
	Log.Info("Registered MCP tool", "name", "execute_script", "params", 1, "mode", "native")
}

// reloadMCPTools reloads all MCP tools
func (s *Server) reloadMCPTools() {
	Log.Info("Reloading MCP tools...")
	newServer, err := s.createMCPServer()
	if err != nil {
		Log.Error("Failed to reload MCP tools", "error", err)
		return
	}
	s.mcpHandler.server.Store(newServer)
	Log.Info("MCP tools reloaded successfully")
}

// createMCPToolHandler creates a handler function for an MCP tool.
// The script is read once at registration time; packLoader is already loaded
// into memory at startup - no fetching happens per call.
func createMCPToolHandler(scriptPath string, libDirs []string, allowedPaths []string, packLoader *pack.Loader) (func(context.Context, *mcp_lib.ToolRequest) (*mcp_lib.ToolResponse, error), error) {
	script, err := os.ReadFile(scriptPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read script %s: %w", scriptPath, err)
	}

	scriptDir := filepath.Dir(scriptPath)
	toolLibDirs := append([]string{scriptDir}, libDirs...)

	handler := func(ctx context.Context, req *mcp_lib.ToolRequest) (*mcp_lib.ToolResponse, error) {
		p := scriptling.New()
		setup.Scriptling(p, toolLibDirs, false, allowedPaths, Log)
		bootstrap.ApplyPackLoader(p, packLoader)

		params := req.Args()
		response, exitCode, err := mcp.RunToolScript(ctx, p, string(script), params)
		if err != nil {
			return nil, fmt.Errorf("script execution failed: %w", err)
		}

		if exitCode != 0 {
			return nil, fmt.Errorf("script exited with code %d: %s", exitCode, response)
		}

		return mcp_lib.NewToolResponseText(response), nil
	}
	return handler, nil
}
