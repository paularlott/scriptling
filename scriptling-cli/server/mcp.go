package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	mcp_lib "github.com/paularlott/mcp"
	"github.com/paularlott/mcp/toolmetadata"
	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/mcp"
	mcpcli "github.com/paularlott/scriptling/scriptling-cli/mcp"
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

	// Watch every configured source folder so any change triggers a reload.
	watchDirs := make([]string, 0, 3)
	if s.config.MCPToolsDir != "" {
		watchDirs = append(watchDirs, s.config.MCPToolsDir)
	}
	if s.config.MCPResourcesDir != "" {
		watchDirs = append(watchDirs, s.config.MCPResourcesDir)
	}
	if s.config.MCPPromptsDir != "" {
		watchDirs = append(watchDirs, s.config.MCPPromptsDir)
	}
	if len(watchDirs) > 0 {
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			Log.Warn("Failed to create file watcher, auto-reload disabled", "error", err)
		} else {
			failed := false
			for _, dir := range watchDirs {
				if err := watcher.Add(dir); err != nil {
					Log.Warn("Failed to watch folder, auto-reload disabled for it", "path", dir, "error", err)
					failed = true
				} else {
					Log.Info("Watching folder for changes", "path", dir)
				}
			}
			if failed && len(watchDirs) == 0 {
				watcher.Close()
			} else {
				s.watcher = watcher
			}
		}
	}

	return nil
}

// createMCPServer creates a new MCP server with all tools, resources and prompts
// registered. The returned server is complete and ready to serve; setupMCP
// stores it and reloadMCPTools mutates it in place thereafter (rather than
// swapping a new server) so notification subscribers stay valid across reloads.
func (s *Server) createMCPServer() (*mcp_lib.Server, error) {
	server := mcp_lib.NewServer("scriptling-server", "1.0.0")
	server.SetInstructions("Execute Scriptling tools from the tools folder.")

	if s.config.MCPExecTool {
		s.registerExecTool(server)
	}

	// Resources and prompts are registered once and persist across reloads.
	s.registerMCPResources(server)
	s.registerMCPPrompts(server)

	if s.config.MCPToolsDir != "" {
		names, err := s.registerFolderTools(server)
		if err != nil {
			return nil, err
		}
		s.mcpFolderToolNames = names
	}

	if s.config.MCPResourcesDir != "" {
		staticKeys, templateKeys, err := s.registerFolderResources(server)
		if err != nil {
			return nil, err
		}
		s.mcpFolderResourceKeys = staticKeys
		s.mcpFolderTemplateKeys = templateKeys
	}

	if s.config.MCPPromptsDir != "" {
		names, err := s.registerFolderPrompts(server)
		if err != nil {
			return nil, err
		}
		s.mcpFolderPromptNames = names
	}

	return server, nil
}

// handlerConfig builds the shared HandlerConfig used by every folder-sourced
// tool/resource/prompt handler. It does NOT include ExtraLibs — those are only
// applied to in-process handlers (execute_script, setup script, json-rpc, http,
// websocket) via s.setupScriptling. Folder handlers run scripts in isolated
// per-call interpreters that mirror the pre-refactor wiring.
func (s *Server) handlerConfig() mcpcli.HandlerConfig {
	return mcpcli.NewHandlerConfig(s.config.LibDirs,
		mcpcli.WithAllowedPaths(s.config.AllowedPaths),
		mcpcli.WithDisabledLibs(s.config.DisabledLibs),
		mcpcli.WithSecrets(s.config.SecretRegistry),
		mcpcli.WithLogger(Log),
		mcpcli.WithPackLoader(s.packLoader),
		mcpcli.WithPlugins(s.config.PluginManager),
	)
}

// registerFolderTools scans the tools folder and registers every tool on
// server, returning the registered tool names so reload can later unregister
// them. Tools whose script is missing are skipped with a warning.
func (s *Server) registerFolderTools(server *mcp_lib.Server) ([]string, error) {
	if s.config.MCPToolsDir == "" {
		return nil, nil
	}
	tools, err := mcpcli.ScanToolsFolder(s.config.MCPToolsDir)
	if err != nil {
		return nil, err
	}

	cfg := s.handlerConfig()
	var names []string
	for toolName, meta := range tools {
		scriptPath := filepath.Join(s.config.MCPToolsDir, toolName+".py")

		if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
			Log.Warn("Skipping tool with missing script", "tool", toolName, "expected", scriptPath)
			continue
		}

		tool, err := toolmetadata.BuildMCPTool(toolName, meta)
		if err != nil {
			return nil, fmt.Errorf("failed to build tool %s: %w", toolName, err)
		}
		handler, err := mcpcli.BuildToolHandler(scriptPath, cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to load tool %s: %w", toolName, err)
		}
		server.RegisterTool(tool, handler)

		mode := "native"
		if meta.Discoverable {
			mode = "discoverable"
		}
		Log.Info("Registered MCP tool", "name", toolName, "params", len(meta.Parameters), "mode", mode)
		names = append(names, toolName)
	}
	return names, nil
}

// registerFolderResources scans the resources tree and registers every static
// resource and template. The first path segment is the URI scheme; the rest
// mirrors the URI. A {var} segment + .py is a template (run); everything else
// is a static resource served verbatim. Returns the static URIs and template
// URI templates so reload can unregister them.
func (s *Server) registerFolderResources(server *mcp_lib.Server) (staticKeys, templateKeys []string, err error) {
	if s.config.MCPResourcesDir == "" {
		return nil, nil, nil
	}
	entries, err := mcpcli.ScanResourcesTree(s.config.MCPResourcesDir)
	if err != nil {
		return nil, nil, err
	}
	cfg := s.handlerConfig()
	for _, e := range entries {
		if e.Template {
			handler, err := mcpcli.BuildResourceScriptHandler(e.FilePath, e.MimeType, cfg)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to load resource template %s: %w", e.URI, err)
			}
			server.RegisterResourceTemplate(
				mcp_lib.NewResourceTemplate(e.URI, e.Name, e.Description, e.MimeType),
				handler,
			)
			templateKeys = append(templateKeys, e.URI)
			Log.Info("Registered MCP resource template", "uri", e.URI)
		} else {
			handler := mcpcli.BuildStaticResourceHandler(e.FilePath, e.URI, e.MimeType)
			server.RegisterResource(
				mcp_lib.NewResource(e.URI, e.Name, e.Description, e.MimeType),
				handler,
			)
			staticKeys = append(staticKeys, e.URI)
			Log.Info("Registered MCP resource", "uri", e.URI)
		}
	}
	return staticKeys, templateKeys, nil
}

// registerFolderPrompts scans the prompts folder and registers every prompt.
// A name.toml + name.py pair is a dynamic prompt (args declared in the toml);
// a lone name.md/name.txt is a static prompt (single user message = file
// content). If both exist for a name, the dynamic one wins.
func (s *Server) registerFolderPrompts(server *mcp_lib.Server) ([]string, error) {
	if s.config.MCPPromptsDir == "" {
		return nil, nil
	}
	entries, err := mcpcli.ScanPromptsFolder(s.config.MCPPromptsDir)
	if err != nil {
		return nil, err
	}
	cfg := s.handlerConfig()
	var names []string
	for _, e := range entries {
		var handler mcp_lib.PromptHandler
		if e.Static {
			handler = mcpcli.BuildStaticPromptHandler(e.FilePath)
		} else {
			h, err := mcpcli.BuildPromptScriptHandler(e.FilePath, cfg)
			if err != nil {
				return nil, fmt.Errorf("failed to load prompt %s: %w", e.Name, err)
			}
			handler = h
		}
		builder := mcp_lib.NewPrompt(e.Name, e.Description)
		for _, arg := range e.Arguments {
			builder.Argument(arg.Name, arg.Description, arg.Required)
		}
		server.RegisterPrompt(builder, handler)
		names = append(names, e.Name)
		mode := "static"
		if !e.Static {
			mode = "dynamic"
		}
		Log.Info("Registered MCP prompt", "prompt", e.Name, "mode", mode, "args", len(e.Arguments))
	}
	return names, nil
}

// registerMCPResources exposes the source of each tool script as a resource
// template, so clients can read tool source code by name.
func (s *Server) registerMCPResources(server *mcp_lib.Server) {
	// Tool source template: scriptling://script/{name} -> the tool's .py source.
	if s.config.MCPToolsDir != "" {
		server.RegisterResourceTemplate(
			mcp_lib.NewResourceTemplate("scriptling://script/{name}", "Tool Source", "Source code of a Scriptling tool by name", "text/plain"),
			func(ctx context.Context, req *mcp_lib.ResourceRequest) (*mcp_lib.ResourceResponse, error) {
				name := req.StringOr("name", "")
				if name == "" || strings.ContainsAny(name, "/\\..") {
					return nil, mcp_lib.NewToolErrorInvalidParams("invalid tool name")
				}
				scriptPath := filepath.Join(s.config.MCPToolsDir, name+".py")
				src, err := os.ReadFile(scriptPath)
				if err != nil {
					return nil, mcp_lib.NewToolErrorInvalidParams("tool script not found: " + name)
				}
				return mcp_lib.NewResourceResponseText(req.URI(), string(src), "text/plain"), nil
			},
		)
	}
}

// registerMCPPrompts exposes Scriptling prompts. A prompt renders a message the
// model can use, e.g. to ask it to write a Scriptling script.
func (s *Server) registerMCPPrompts(server *mcp_lib.Server) {
	server.RegisterPrompt(
		mcp_lib.NewPrompt("write_script", "Generate a Scriptling script for a task").
			Argument("task", "What the script should do", true).
			Argument("context", "Extra context or requirements", false),
		func(ctx context.Context, req *mcp_lib.PromptRequest) (*mcp_lib.PromptResponse, error) {
			task := req.StringOr("task", "")
			extra := req.StringOr("context", "")
			msg := "Write a Scriptling script (Python 3-like) that: " + task + "."
			if extra != "" {
				msg += "\n\nAdditional context: " + extra
			}
			msg += "\n\nUse tool.return_string(...) or tool.return_object(...) to return the result."
			return mcp_lib.NewPromptResponseText(msg), nil
		},
	)
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
			Log.Trace("MCP execute_script invoked", "code_len", len(code))
			p := scriptling.New()
			s.setupScriptling(p)

			response, exitCode, err := mcp.RunToolScript(ctx, p, code, map[string]interface{}{})

			// If the script produced an explicit response (via return_error, return_string, etc.),
			// return it to the client. return_error sets a response AND exits non-zero, so check
			// for a response before treating non-zero exit as a failure.
			if response != "" {
				if exitCode != 0 {
					Log.Debug("MCP execute_script returned error response", "exit_code", exitCode)
					return nil, mcp_lib.NewToolErrorInternal(response)
				}
				Log.Trace("MCP execute_script completed", "exit_code", exitCode, "response_len", len(response))
				return mcp_lib.NewToolResponseText(response), nil
			}

			if err != nil {
				Log.Debug("MCP execute_script failed", "exit_code", exitCode, "error", err)
				return nil, fmt.Errorf("execution error: %w", err)
			}

			return mcp_lib.NewToolResponseText(""), nil
		},
	)
	Log.Info("Registered MCP tool", "name", "execute_script", "params", 1, "mode", "native")
}

// reloadMCP refreshes the folder-sourced tools, resources and prompts on the
// live MCP server in place (unregister removed, register added) and emits a
// listChanged notification for each so connected clients re-fetch. Mutating in
// place — rather than swapping a new server — keeps SSE/stdio notification
// subscribers valid across reloads.
func (s *Server) reloadMCP() {
	Log.Info("Reloading MCP tools, resources and prompts...")
	server := s.mcpHandler.server.Load()
	if server == nil {
		Log.Error("Failed to reload MCP: server not ready")
		return
	}

	// Tools.
	for _, name := range s.mcpFolderToolNames {
		server.UnregisterTool(name)
	}
	toolNames, err := s.registerFolderTools(server)
	if err != nil {
		Log.Error("Failed to reload MCP tools", "error", err)
	} else {
		s.mcpFolderToolNames = toolNames
	}
	server.NotifyToolsChanged()

	// Resources (static + templates).
	for _, uri := range s.mcpFolderResourceKeys {
		server.UnregisterResource(uri)
	}
	for _, uriTmpl := range s.mcpFolderTemplateKeys {
		server.UnregisterResourceTemplate(uriTmpl)
	}
	staticKeys, templateKeys, err := s.registerFolderResources(server)
	if err != nil {
		Log.Error("Failed to reload MCP resources", "error", err)
	} else {
		s.mcpFolderResourceKeys = staticKeys
		s.mcpFolderTemplateKeys = templateKeys
	}
	server.NotifyResourcesChanged()

	// Prompts.
	for _, name := range s.mcpFolderPromptNames {
		server.UnregisterPrompt(name)
	}
	promptNames, err := s.registerFolderPrompts(server)
	if err != nil {
		Log.Error("Failed to reload MCP prompts", "error", err)
	} else {
		s.mcpFolderPromptNames = promptNames
	}
	server.NotifyPromptsChanged()

	Log.Info("MCP reloaded successfully")
}
