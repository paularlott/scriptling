package server

import (
	"context"
	"fmt"
	"io/fs"
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

	// Folder-sourced entries.
	if s.config.MCPToolsDir != "" {
		names, err := s.registerToolsFromFS(server, os.DirFS(s.config.MCPToolsDir), s.config.MCPToolsDir)
		if err != nil {
			return nil, err
		}
		s.mcpFolderEntries.tools = names
	}
	if s.config.MCPResourcesDir != "" {
		static, template, err := s.registerResourcesFromFS(server, os.DirFS(s.config.MCPResourcesDir), s.config.MCPResourcesDir)
		if err != nil {
			return nil, err
		}
		s.mcpFolderEntries.staticResources = static
		s.mcpFolderEntries.templateResources = template
	}
	if s.config.MCPPromptsDir != "" {
		names, err := s.registerPromptsFromFS(server, os.DirFS(s.config.MCPPromptsDir), s.config.MCPPromptsDir)
		if err != nil {
			return nil, err
		}
		s.mcpFolderEntries.prompts = names
	}

	// Bundle-sourced entries.
	if s.config.appMode() {
		b := s.config.Bundle
		if toolsFS, ok := b.Sub("tools"); ok {
			names, err := s.registerToolsFromFS(server, toolsFS, b.Source())
			if err != nil {
				return nil, err
			}
			s.mcpBundleEntries.tools = names
		}
		if resFS, ok := b.Sub("resources"); ok {
			static, template, err := s.registerResourcesFromFS(server, resFS, b.Source())
			if err != nil {
				return nil, err
			}
			s.mcpBundleEntries.staticResources = static
			s.mcpBundleEntries.templateResources = template
		}
		if promptFS, ok := b.Sub("prompts"); ok {
			names, err := s.registerPromptsFromFS(server, promptFS, b.Source())
			if err != nil {
				return nil, err
			}
			s.mcpBundleEntries.prompts = names
		}
	}

	return server, nil
}

// registerToolsFromFS scans fsys for tools (both legacy and decorated formats)
// and registers them on the MCP server. source is a label for logging.
func (s *Server) registerToolsFromFS(server *mcp_lib.Server, fsys fs.FS, source string) ([]string, error) {
	cfg := s.handlerConfig()
	entries, err := mcpcli.ScanToolsFSDual(fsys, cfg)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", source, err)
	}
	var names []string
	for _, entry := range entries {
		tool, buildErr := toolmetadata.BuildMCPTool(entry.Name, entry.Meta)
		if buildErr != nil {
			return nil, fmt.Errorf("failed to build tool %s: %w", entry.Name, buildErr)
		}
		var handler mcp_lib.ToolHandler
		if entry.Legacy {
			if entry.Source == nil {
				Log.Warn("Skipping tool with missing script", "tool", entry.Name, "source", source)
				continue
			}
			handler = mcpcli.BuildToolHandlerSource(entry.Source, cfg)
		} else {
			handler = mcpcli.BuildToolHandlerFunc(entry.Source, entry.FuncName, cfg)
		}
		server.RegisterTool(tool, handler)
		names = append(names, entry.Name)
		mode := "native"
		if entry.Meta.Discoverable {
			mode = "discoverable"
		}
		format := "legacy"
		if !entry.Legacy {
			format = "decorated"
		}
		Log.Info("Registered MCP tool", "name", entry.Name, "params", len(entry.Meta.Parameters), "mode", mode, "format", format, "source", source)
	}
	return names, nil
}

// registerResourcesFromFS scans fsys for MCP resources (static and templates)
// and registers them on the MCP server. source is a label for logging.
func (s *Server) registerResourcesFromFS(server *mcp_lib.Server, fsys fs.FS, source string) (staticKeys, templateKeys []string, err error) {
	entries, err := mcpcli.ScanResourcesFS(fsys)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", source, err)
	}
	cfg := s.handlerConfig()
	for _, e := range entries {
		if e.Template {
			src, readErr := fs.ReadFile(fsys, e.FilePath)
			if readErr != nil {
				return nil, nil, fmt.Errorf("failed to read resource template %s: %w", e.URI, readErr)
			}
			server.RegisterResourceTemplate(
				mcp_lib.NewResourceTemplate(e.URI, e.Name, e.Description, e.MimeType),
				mcpcli.BuildResourceScriptHandlerSource(src, e.MimeType, cfg),
			)
			templateKeys = append(templateKeys, e.URI)
			Log.Info("Registered MCP resource template", "uri", e.URI, "source", source)
		} else {
			filePath := e.FilePath
			server.RegisterResource(
				mcp_lib.NewResource(e.URI, e.Name, e.Description, e.MimeType),
				mcpcli.BuildStaticResourceHandler(func() ([]byte, error) { return fs.ReadFile(fsys, filePath) }, e.URI, e.MimeType),
			)
			staticKeys = append(staticKeys, e.URI)
			Log.Info("Registered MCP resource", "uri", e.URI, "source", source)
		}
	}
	return staticKeys, templateKeys, nil
}

// registerPromptsFromFS scans fsys for MCP prompts (dynamic and static) and
// registers them on the MCP server. source is a label for logging.
func (s *Server) registerPromptsFromFS(server *mcp_lib.Server, fsys fs.FS, source string) ([]string, error) {
	entries, err := mcpcli.ScanPromptsFS(fsys)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", source, err)
	}
	cfg := s.handlerConfig()
	var names []string
	for _, e := range entries {
		var handler mcp_lib.PromptHandler
		if e.Static {
			filePath := e.FilePath
			handler = mcpcli.BuildStaticPromptHandler(func() ([]byte, error) { return fs.ReadFile(fsys, filePath) })
		} else {
			src, readErr := fs.ReadFile(fsys, e.FilePath)
			if readErr != nil {
				return nil, fmt.Errorf("failed to read prompt %s: %w", e.Name, readErr)
			}
			handler = mcpcli.BuildPromptScriptHandlerSource(src, cfg)
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
		Log.Info("Registered MCP prompt", "prompt", e.Name, "mode", mode, "args", len(e.Arguments), "source", source)
	}
	return names, nil
}

// handlerConfig builds the shared HandlerConfig used by every folder-sourced
// tool/resource/prompt handler. It does NOT include ExtraLibs — those are only
// applied to in-process handlers (execute_script, setup script, json-rpc, http,
// websocket) via s.setupScriptling. Folder handlers run scripts in isolated
// per-call interpreters that mirror the pre-refactor wiring.
func (s *Server) handlerConfig() mcpcli.HandlerConfig {
	opts := []mcpcli.HandlerOption{
		mcpcli.WithAllowedPaths(s.config.AllowedPaths),
		mcpcli.WithDisabledLibs(s.config.DisabledLibs),
		mcpcli.WithSecrets(s.config.SecretRegistry),
		mcpcli.WithLogger(Log),
		mcpcli.WithPackLoader(s.packLoader),
		mcpcli.WithPlugins(s.config.PluginManager),
		mcpcli.WithDockerSock(s.config.DockerSock),
		mcpcli.WithPodmanSock(s.config.PodmanSock),
	}
	if s.config.Argv != nil {
		opts = append(opts, mcpcli.WithArgv(s.config.Argv))
	}
	if s.config.ExtraLibs != nil {
		opts = append(opts, mcpcli.WithSetupHook(s.config.ExtraLibs))
	}
	return mcpcli.NewHandlerConfig(s.config.LibDirs, opts...)
}

// registerMCPResources exposes the source of each tool script as a resource
// template, so clients can read tool source code by name.
func (s *Server) registerMCPResources(server *mcp_lib.Server) {
	// Bundle tool source: scriptling://script/{name} reads from the app
	// bundle's tools/ dir.
	if s.config.appMode() {
		server.RegisterResourceTemplate(
			mcp_lib.NewResourceTemplate("scriptling://script/{name}", "Tool Source", "Source code of a Scriptling tool by name", "text/plain"),
			func(ctx context.Context, req *mcp_lib.ResourceRequest) (*mcp_lib.ResourceResponse, error) {
				name := req.StringOr("name", "")
				if name == "" || strings.ContainsAny(name, "/\\..") {
					return nil, mcp_lib.NewToolErrorInvalidParams("invalid tool name")
				}
				if toolsFS, ok := s.config.Bundle.Sub("tools"); ok {
					if src, err := fs.ReadFile(toolsFS, name+".py"); err == nil {
						return mcp_lib.NewResourceResponseText(req.URI(), string(src), "text/plain"), nil
					}
				}
				return nil, mcp_lib.NewToolErrorInvalidParams("tool script not found: " + name)
			},
		)
		return
	}

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
	s.reloadMu.Lock()
	defer s.reloadMu.Unlock()

	Log.Info("Reloading MCP tools, resources and prompts...")
	server := s.mcpHandler.server.Load()
	if server == nil {
		Log.Error("Failed to reload MCP: server not ready")
		return
	}

	// Folder-sourced entries.
	s.mcpFolderEntries.unregisterAll(server)

	if s.config.MCPToolsDir != "" {
		toolNames, err := s.registerToolsFromFS(server, os.DirFS(s.config.MCPToolsDir), s.config.MCPToolsDir)
		if err != nil {
			Log.Error("Failed to reload MCP tools", "error", err)
		} else {
			s.mcpFolderEntries.tools = toolNames
		}
	}
	server.NotifyToolsChanged()

	if s.config.MCPResourcesDir != "" {
		staticKeys, templateKeys, err := s.registerResourcesFromFS(server, os.DirFS(s.config.MCPResourcesDir), s.config.MCPResourcesDir)
		if err != nil {
			Log.Error("Failed to reload MCP resources", "error", err)
		} else {
			s.mcpFolderEntries.staticResources = staticKeys
			s.mcpFolderEntries.templateResources = templateKeys
		}
	}
	server.NotifyResourcesChanged()

	if s.config.MCPPromptsDir != "" {
		promptNames, err := s.registerPromptsFromFS(server, os.DirFS(s.config.MCPPromptsDir), s.config.MCPPromptsDir)
		if err != nil {
			Log.Error("Failed to reload MCP prompts", "error", err)
		} else {
			s.mcpFolderEntries.prompts = promptNames
		}
	}
	server.NotifyPromptsChanged()

	// Bundle-sourced entries: re-scan (dir-backed bundles pick up changes;
	// zip-backed bundles re-register identical content).
	if s.config.appMode() {
		s.mcpBundleEntries.unregisterAll(server)
		b := s.config.Bundle
		if toolsFS, ok := b.Sub("tools"); ok {
			names, err := s.registerToolsFromFS(server, toolsFS, b.Source())
			if err != nil {
				Log.Error("Failed to reload bundle MCP tools", "error", err)
			} else {
				s.mcpBundleEntries.tools = names
			}
		}
		if resFS, ok := b.Sub("resources"); ok {
			static, template, err := s.registerResourcesFromFS(server, resFS, b.Source())
			if err != nil {
				Log.Error("Failed to reload bundle MCP resources", "error", err)
			} else {
				s.mcpBundleEntries.staticResources = static
				s.mcpBundleEntries.templateResources = template
			}
		}
		if promptFS, ok := b.Sub("prompts"); ok {
			names, err := s.registerPromptsFromFS(server, promptFS, b.Source())
			if err != nil {
				Log.Error("Failed to reload bundle MCP prompts", "error", err)
			} else {
				s.mcpBundleEntries.prompts = names
			}
		}
		server.NotifyToolsChanged()
		server.NotifyResourcesChanged()
		server.NotifyPromptsChanged()
	}

	Log.Info("MCP reloaded successfully")
}
