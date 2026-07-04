package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/fsnotify/fsnotify"
	mcp_lib "github.com/paularlott/mcp"
	"github.com/paularlott/mcp/toolmetadata"
	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/mcp"
	"github.com/paularlott/scriptling/extlibs/secretprovider"
	scriptlingplugin "github.com/paularlott/scriptling/plugin"
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
		handler, err := createMCPToolHandler(scriptPath, s.config.LibDirs, s.config.AllowedPaths, s.config.DisabledLibs, s.config.SecretRegistry, s.packLoader, s.config.PluginManager)
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
	entries, err := mcpcli.ScanResourcesTree(s.config.MCPResourcesDir)
	if err != nil {
		return nil, nil, err
	}
	for _, e := range entries {
		if e.Template {
			handler, err := createMCPResourceScriptHandler(e.FilePath, s.config.LibDirs, s.config.AllowedPaths, s.config.DisabledLibs, s.config.SecretRegistry, s.packLoader, s.config.PluginManager, e.MimeType)
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
			handler := createStaticResourceHandler(e.FilePath, e.URI, e.MimeType)
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

// createStaticResourceHandler serves a file verbatim: text content for UTF-8
// files, base64 blob otherwise. The file is read on every read so content is
// always current.
func createStaticResourceHandler(filePath, uri, mimeType string) mcp_lib.ResourceHandler {
	return func(ctx context.Context, req *mcp_lib.ResourceRequest) (*mcp_lib.ResourceResponse, error) {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, mcp_lib.NewToolErrorInternal(fmt.Sprintf("failed to read resource: %v", err))
		}
		if utf8.Valid(data) {
			mime := mimeType
			if mime == "" {
				mime = "text/plain"
			}
			return mcp_lib.NewResourceResponseText(uri, string(data), mime), nil
		}
		mime := mimeType
		if mime == "" {
			mime = "application/octet-stream"
		}
		return mcp_lib.NewResourceResponseBlob(uri, data, mime), nil
	}
}

// registerFolderPrompts scans the prompts folder and registers every prompt.
// A name.toml + name.py pair is a dynamic prompt (args declared in the toml);
// a lone name.md/name.txt is a static prompt (single user message = file
// content). If both exist for a name, the dynamic one wins.
func (s *Server) registerFolderPrompts(server *mcp_lib.Server) ([]string, error) {
	entries, err := mcpcli.ScanPromptsFolder(s.config.MCPPromptsDir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		var handler mcp_lib.PromptHandler
		if e.Static {
			handler = createStaticPromptHandler(e.FilePath)
		} else {
			h, err := createMCPPromptScriptHandler(e.FilePath, s.config.LibDirs, s.config.AllowedPaths, s.config.DisabledLibs, s.config.SecretRegistry, s.packLoader, s.config.PluginManager)
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

// createStaticPromptHandler returns a prompt whose single user message is the
// file's content (read fresh each call).
func createStaticPromptHandler(filePath string) mcp_lib.PromptHandler {
	return func(ctx context.Context, req *mcp_lib.PromptRequest) (*mcp_lib.PromptResponse, error) {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, mcp_lib.NewToolErrorInternal(fmt.Sprintf("failed to read prompt: %v", err))
		}
		return mcp_lib.NewPromptResponseText(string(data)), nil
	}
}

// createMCPResourceScriptHandler builds a ResourceHandler that runs a Scriptling
// script. The script receives the request URI as "__uri" and any template
// variables as parameters (read via mcp.tool.get_string), and returns the
// resource content via mcp.tool.return_string / return_object.
func createMCPResourceScriptHandler(scriptPath string, libDirs []string, allowedPaths []string, disabledLibs []string, secretRegistry *secretprovider.Registry, packLoader *pack.Loader, pluginManager *scriptlingplugin.Manager, mimeType string) (mcp_lib.ResourceHandler, error) {
	script, err := os.ReadFile(scriptPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read script %s: %w", scriptPath, err)
	}
	scriptDir := filepath.Dir(scriptPath)
	toolLibDirs := append([]string{scriptDir}, libDirs...)
	return func(ctx context.Context, req *mcp_lib.ResourceRequest) (*mcp_lib.ResourceResponse, error) {
		p := scriptling.New()
		setup.Scriptling(p, toolLibDirs, false, allowedPaths, disabledLibs, secretRegistry, Log, "", "")
		if pluginManager != nil {
			scriptlingplugin.RegisterLibraries(p, pluginManager)
		}
		bootstrap.ApplyPackLoader(p, packLoader)

		params := map[string]any{"__uri": req.URI()}
		for k, v := range req.Vars() {
			params[k] = v
		}
		response, exitCode, err := mcp.RunToolScript(ctx, p, string(script), params)
		if response != "" {
			if exitCode != 0 {
				return nil, mcp_lib.NewToolErrorInternal(response)
			}
			return mcp_lib.NewResourceResponseText(req.URI(), response, mimeType), nil
		}
		if err != nil {
			return nil, fmt.Errorf("resource script failed: %w", err)
		}
		return mcp_lib.NewResourceResponseText(req.URI(), "", mimeType), nil
	}, nil
}

// createMCPPromptScriptHandler builds a PromptHandler that runs a Scriptling
// script. The script receives the prompt arguments as parameters and returns
// messages via mcp.tool.return_object({"messages": [{"role": ..., "content": ...}]})
// (a bare string is treated as a single user message).
func createMCPPromptScriptHandler(scriptPath string, libDirs []string, allowedPaths []string, disabledLibs []string, secretRegistry *secretprovider.Registry, packLoader *pack.Loader, pluginManager *scriptlingplugin.Manager) (mcp_lib.PromptHandler, error) {
	script, err := os.ReadFile(scriptPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read script %s: %w", scriptPath, err)
	}
	scriptDir := filepath.Dir(scriptPath)
	toolLibDirs := append([]string{scriptDir}, libDirs...)
	return func(ctx context.Context, req *mcp_lib.PromptRequest) (*mcp_lib.PromptResponse, error) {
		p := scriptling.New()
		setup.Scriptling(p, toolLibDirs, false, allowedPaths, disabledLibs, secretRegistry, Log, "", "")
		if pluginManager != nil {
			scriptlingplugin.RegisterLibraries(p, pluginManager)
		}
		bootstrap.ApplyPackLoader(p, packLoader)

		params := map[string]any{}
		for k, v := range req.Args() {
			params[k] = v
		}
		response, exitCode, err := mcp.RunToolScript(ctx, p, string(script), params)
		if exitCode != 0 && err != nil {
			return nil, fmt.Errorf("prompt script failed: %w", err)
		}
		return decodePromptScriptResponse(response), nil
	}, nil
}

// decodePromptScriptResponse interprets a prompt script's return value into a
// PromptResponse. Accepted shapes: a JSON object {"description":..., "messages":
// [{"role","content"}]}, a JSON array of messages, or a plain string (single
// user message).
func decodePromptScriptResponse(response string) *mcp_lib.PromptResponse {
	trimmed := strings.TrimSpace(response)
	if trimmed == "" {
		return mcp_lib.NewPromptResponseMessages()
	}
	var raw any
	if err := json.Unmarshal([]byte(trimmed), &raw); err == nil {
		switch v := raw.(type) {
		case map[string]any:
			desc, _ := v["description"].(string)
			msgs := promptMessagesFromAny(v["messages"])
			if msgs == nil {
				msgs = []mcp_lib.PromptMessage{}
			}
			return &mcp_lib.PromptResponse{Description: desc, Messages: msgs}
		case []any:
			msgs := promptMessagesFromAny(v)
			if msgs == nil {
				msgs = []mcp_lib.PromptMessage{}
			}
			return &mcp_lib.PromptResponse{Messages: msgs}
		}
	}
	return mcp_lib.NewPromptResponseText(response)
}

// promptMessagesFromAny converts a parsed JSON value (list of {role, content})
// into PromptMessages. Returns nil if the value is not a message list.
func promptMessagesFromAny(v any) []mcp_lib.PromptMessage {
	list, ok := v.([]any)
	if !ok {
		return nil
	}
	var msgs []mcp_lib.PromptMessage
	for _, elem := range list {
		m, ok := elem.(map[string]any)
		if !ok {
			continue
		}
		role, _ := m["role"].(string)
		if role == "" {
			role = "user"
		}
		content := promptContentFromAny(m["content"])
		msgs = append(msgs, mcp_lib.PromptMessage{Role: mcp_lib.PromptMessageRole(role), Content: content})
	}
	return msgs
}

// promptContentFromAny converts a message's "content" value into a
// PromptMessageContent. Strings become text blocks; dicts are used as-is.
func promptContentFromAny(v any) mcp_lib.PromptMessageContent {
	switch c := v.(type) {
	case string:
		return mcp_lib.PromptMessageContent{Type: "text", Text: c}
	case map[string]any:
		typ, _ := c["type"].(string)
		if typ == "" {
			typ = "text"
		}
		text, _ := c["text"].(string)
		data, _ := c["data"].(string)
		mime, _ := c["mimeType"].(string)
		return mcp_lib.PromptMessageContent{Type: typ, Text: text, Data: data, MimeType: mime}
	default:
		return mcp_lib.PromptMessageContent{Type: "text", Text: fmt.Sprintf("%v", v)}
	}
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

// createMCPToolHandler creates a handler function for an MCP tool.
// The script is read once at registration time; packLoader is already loaded
// into memory at startup - no fetching happens per call.
func createMCPToolHandler(scriptPath string, libDirs []string, allowedPaths []string, disabledLibs []string, secretRegistry *secretprovider.Registry, packLoader *pack.Loader, pluginManager *scriptlingplugin.Manager) (func(context.Context, *mcp_lib.ToolRequest) (*mcp_lib.ToolResponse, error), error) {
	script, err := os.ReadFile(scriptPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read script %s: %w", scriptPath, err)
	}

	scriptDir := filepath.Dir(scriptPath)
	toolLibDirs := append([]string{scriptDir}, libDirs...)

	handler := func(ctx context.Context, req *mcp_lib.ToolRequest) (*mcp_lib.ToolResponse, error) {
		params := req.Args()
		Log.Trace("MCP tool invoked", "script", filepath.Base(scriptPath), "params", params)
		p := scriptling.New()
		setup.Scriptling(p, toolLibDirs, false, allowedPaths, disabledLibs, secretRegistry, Log, "", "")
		if pluginManager != nil {
			scriptlingplugin.RegisterLibraries(p, pluginManager)
		}
		bootstrap.ApplyPackLoader(p, packLoader)

		response, exitCode, err := mcp.RunToolScript(ctx, p, string(script), params)

		// If the script produced an explicit response (via return_error, return_string, etc.),
		// return it to the client. return_error sets a response AND exits non-zero, so check
		// for a response before treating non-zero exit as a failure.
		if response != "" {
			if exitCode != 0 {
				Log.Debug("MCP tool returned error response", "script", filepath.Base(scriptPath), "exit_code", exitCode)
				return nil, mcp_lib.NewToolErrorInternal(response)
			}
			Log.Trace("MCP tool completed", "script", filepath.Base(scriptPath), "response_len", len(response))
			return mcp_lib.NewToolResponseText(response), nil
		}

		if err != nil {
			Log.Debug("MCP tool failed", "script", filepath.Base(scriptPath), "error", err)
			return nil, fmt.Errorf("script execution failed: %w", err)
		}

		if exitCode != 0 {
			Log.Debug("MCP tool exited non-zero", "script", filepath.Base(scriptPath), "exit_code", exitCode)
			return nil, fmt.Errorf("script exited with code %d", exitCode)
		}

		return mcp_lib.NewToolResponseText(""), nil
	}
	return handler, nil
}
