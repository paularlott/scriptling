package server

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path"
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

	if s.config.MCPSkillsDir != "" {
		watchDirs = append(watchDirs, s.config.MCPSkillsDir)
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
	server.DeclareExtension(mcp_lib.UIAppsExtensionID, map[string]any{
		"mimeTypes": []string{mcp_lib.UIAppMimeType},
	})
	// The library's own Origin check (localhost-or-no-header by default)
	// is disabled here — mcpCorsMiddleware (http.go) is the single,
	// authoritative decision for this route instead, and runs before
	// HandleRequest is ever reached. It has to be: it's request-aware
	// (its same-origin default needs r.TLS/r.Host, which a plain
	// func(origin string) bool validator can't see) and honors
	// MCPCorsOrigins, neither of which the library's own default knows
	// about. Leaving the library's check enabled too would just make it
	// reject requests mcpCorsMiddleware already approved (e.g. an
	// operator's explicit MCPCorsOrigins wildcard, or same-origin on a
	// non-localhost deployment).
	server.SetOriginValidator(func(string) bool { return true })

	if s.config.MCPExecTool {
		s.registerExecTool(server)
	}

	// Folder-sourced entries.
	if s.config.MCPToolsDir != "" {
		regs, err := s.registerToolsFromFS(server, os.DirFS(s.config.MCPToolsDir), s.config.MCPToolsDir)
		if err != nil {
			return nil, err
		}
		s.mcpFolderEntries.tools = regs.tools
		s.mcpFolderEntries.staticResources = regs.staticResources
		s.mcpFolderEntries.templateResources = regs.templateResources
		s.mcpFolderEntries.prompts = regs.prompts
		s.mcpFolderEntries.skills = regs.skills
	}
	if s.config.MCPResourcesDir != "" {
		static, template, err := s.registerResourcesFromFS(server, os.DirFS(s.config.MCPResourcesDir), s.config.MCPResourcesDir)
		if err != nil {
			return nil, err
		}
		s.mcpFolderEntries.staticResources = append(s.mcpFolderEntries.staticResources, static...)
		s.mcpFolderEntries.templateResources = append(s.mcpFolderEntries.templateResources, template...)
	}
	if s.config.MCPPromptsDir != "" {
		names, err := s.registerPromptsFromFS(server, os.DirFS(s.config.MCPPromptsDir), s.config.MCPPromptsDir)
		if err != nil {
			return nil, err
		}
		s.mcpFolderEntries.prompts = append(s.mcpFolderEntries.prompts, names...)
	}

	if s.config.MCPSkillsDir != "" {
		names, err := s.registerSkillsFromFS(server, os.DirFS(s.config.MCPSkillsDir), s.config.MCPSkillsDir)
		if err != nil {
			return nil, err
		}
		s.mcpFolderEntries.skills = append(s.mcpFolderEntries.skills, names...)
	}

	// Bundle-sourced entries.
	if s.config.appMode() {
		b := s.config.Bundle
		if toolsFS, ok := b.Sub("tools"); ok {
			regs, err := s.registerToolsFromFS(server, toolsFS, b.Source())
			if err != nil {
				return nil, err
			}
			s.mcpBundleEntries.tools = regs.tools
			s.mcpBundleEntries.staticResources = regs.staticResources
			s.mcpBundleEntries.templateResources = regs.templateResources
			s.mcpBundleEntries.prompts = regs.prompts
			s.mcpBundleEntries.skills = regs.skills
		}
		if resFS, ok := b.Sub("resources"); ok {
			static, template, err := s.registerResourcesFromFS(server, resFS, b.Source())
			if err != nil {
				return nil, err
			}
			s.mcpBundleEntries.staticResources = append(s.mcpBundleEntries.staticResources, static...)
			s.mcpBundleEntries.templateResources = append(s.mcpBundleEntries.templateResources, template...)
		}
		if promptFS, ok := b.Sub("prompts"); ok {
			names, err := s.registerPromptsFromFS(server, promptFS, b.Source())
			if err != nil {
				return nil, err
			}
			s.mcpBundleEntries.prompts = append(s.mcpBundleEntries.prompts, names...)
		}
		if skillsFS, ok := b.Sub("skills"); ok {
			names, err := s.registerSkillsFromFS(server, skillsFS, b.Source())
			if err != nil {
				return nil, err
			}
			s.mcpBundleEntries.skills = append(s.mcpBundleEntries.skills, names...)
		}
	}

	return server, nil
}

// fsRegistrations is what one registration source (the tools folder, or a
// bundle's tools subtree) contributed to the MCP server, for reload tracking.
type fsRegistrations struct {
	tools             []string // tool names
	staticResources   []string // static resource URIs
	templateResources []string // resource template URI templates
	prompts           []string // prompt names
	skills            []string // skill names
}

// registerToolsFromFS scans fsys for tools (both legacy and decorated formats)
// and registers them on the MCP server, along with every @mcp.resource(),
// @mcp.prompt() and @mcp.skill() registration found in the same decorated .py
// files. source is a label for logging.
func (s *Server) registerToolsFromFS(server *mcp_lib.Server, fsys fs.FS, source string) (*fsRegistrations, error) {
	cfg := s.handlerConfig()
	entries, err := mcpcli.ScanRegistrationsFSDual(fsys, cfg)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", source, err)
	}
	out := &fsRegistrations{}
	for _, entry := range entries.Tools {
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
		out.tools = append(out.tools, entry.Name)
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

	for _, e := range entries.Resources {
		handler := mcpcli.BuildResourceFuncHandler(e.Source, e.FuncName, e.URI, e.MimeType, cfg)
		if e.Template {
			server.RegisterResourceTemplate(
				mcp_lib.NewResourceTemplate(e.URI, e.Name, e.Description, resourceEffectiveMIME(e.URI, e.MimeType)),
				handler,
			)
			out.templateResources = append(out.templateResources, e.URI)
			Log.Info("Registered MCP resource template", "uri", e.URI, "source", source)
		} else {
			server.RegisterResource(
				mcp_lib.NewResource(e.URI, e.Name, e.Description, resourceEffectiveMIME(e.URI, e.MimeType)),
				handler,
			)
			out.staticResources = append(out.staticResources, e.URI)
			Log.Info("Registered MCP resource", "uri", e.URI, "source", source)
		}
	}

	for _, e := range entries.Prompts {
		builder := mcp_lib.NewPrompt(e.Name, e.Description)
		for _, arg := range e.Arguments {
			builder.Argument(arg.Name, arg.Description, arg.Required)
		}
		server.RegisterPrompt(builder, mcpcli.BuildPromptFuncHandler(e.Source, e.FuncName, cfg))
		out.prompts = append(out.prompts, e.Name)
		Log.Info("Registered MCP prompt", "prompt", e.Name, "mode", "decorated", "args", len(e.Arguments), "source", source)
	}

	for _, e := range entries.Skills {
		builder := mcp_lib.NewSkill(e.Name)
		for fileName, content := range e.Files {
			// File returns the builder for chaining; content problems
			// surface at RegisterSkill below.
			builder.File(fileName, content)
		}
		if err := server.RegisterSkill(builder); err != nil {
			// One bad skill must not fail the whole server, matching the
			// skills folder's rule.
			Log.Warn("Skipping MCP skill", "name", e.Name, "error", err, "source", source)
			continue
		}
		out.skills = append(out.skills, e.Name)
		Log.Info("registered MCP skill", "name", e.Name, "source", source)
	}

	return out, nil
}

// resourceEffectiveMIME resolves a decorated resource's content type: the
// MCP Apps extension MUSTs the mimeType of ui:// resources, everything else
// keeps the decorator's value (or text/plain when unset).
func resourceEffectiveMIME(uri, mimeType string) string {
	if strings.HasPrefix(uri, "ui://") {
		return mcp_lib.UIAppMimeType
	}
	if mimeType == "" {
		return "text/plain"
	}
	return mimeType
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
			resourceBuilder := mcp_lib.NewResource(e.URI, e.Name, e.Description, e.MimeType)
			readFn := func() ([]byte, error) { return fs.ReadFile(fsys, filePath) }
			var handler mcp_lib.ResourceHandler
			if e.UIMeta != nil {
				resourceBuilder.UIMeta(*e.UIMeta)
				handler = mcpcli.BuildStaticResourceHandlerWithMeta(readFn, e.URI, e.MimeType, e.UIMeta)
			} else {
				handler = mcpcli.BuildStaticResourceHandler(readFn, e.URI, e.MimeType)
			}
			server.RegisterResource(resourceBuilder, handler)
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
		mcpcli.WithNetworkPolicy(s.config.NetworkPolicy),
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
- HTTP response is an object: response.status_code, response.text, response.headers; response.json() parses the body as JSON
- Use json.loads(str) and json.dumps(obj) for JSON
- Use requests.get(url, options), requests.post(url, json=data) for HTTP
- Request options (dict or kwargs): timeout, headers, params, auth
- Default HTTP timeout is 5 seconds
- HTTP options dict: {"timeout": 10, "headers": {"Authorization": "Bearer token"}}

COMMON PATTERNS:
- Dict iteration: for k, v in dict.items()
- List append: my_list.append(item) modifies in-place
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
		regs, err := s.registerToolsFromFS(server, os.DirFS(s.config.MCPToolsDir), s.config.MCPToolsDir)
		if err != nil {
			Log.Error("Failed to reload MCP tools", "error", err)
		} else {
			s.mcpFolderEntries.tools = regs.tools
			s.mcpFolderEntries.staticResources = regs.staticResources
			s.mcpFolderEntries.templateResources = regs.templateResources
			s.mcpFolderEntries.prompts = regs.prompts
			s.mcpFolderEntries.skills = regs.skills
		}
	}
	server.NotifyToolsChanged()

	if s.config.MCPResourcesDir != "" {
		staticKeys, templateKeys, err := s.registerResourcesFromFS(server, os.DirFS(s.config.MCPResourcesDir), s.config.MCPResourcesDir)
		if err != nil {
			Log.Error("Failed to reload MCP resources", "error", err)
		} else {
			s.mcpFolderEntries.staticResources = append(s.mcpFolderEntries.staticResources, staticKeys...)
			s.mcpFolderEntries.templateResources = append(s.mcpFolderEntries.templateResources, templateKeys...)
		}
	}
	server.NotifyResourcesChanged()

	if s.config.MCPPromptsDir != "" {
		promptNames, err := s.registerPromptsFromFS(server, os.DirFS(s.config.MCPPromptsDir), s.config.MCPPromptsDir)
		if err != nil {
			Log.Error("Failed to reload MCP prompts", "error", err)
		} else {
			s.mcpFolderEntries.prompts = append(s.mcpFolderEntries.prompts, promptNames...)
		}
	}
	server.NotifyPromptsChanged()

	// Skills have no listChanged notification in the extension (SEP-2640), so
	// a reload refreshes them silently; clients pick changes up on their next
	// skills/list.
	if s.config.MCPSkillsDir != "" {
		names, err := s.registerSkillsFromFS(server, os.DirFS(s.config.MCPSkillsDir), s.config.MCPSkillsDir)
		if err != nil {
			Log.Error("Failed to reload MCP skills", "error", err)
		} else {
			s.mcpFolderEntries.skills = append(s.mcpFolderEntries.skills, names...)
		}
	}

	// Bundle-sourced entries: re-scan (dir-backed bundles pick up changes;
	// zip-backed bundles re-register identical content).
	if s.config.appMode() {
		s.mcpBundleEntries.unregisterAll(server)
		b := s.config.Bundle
		if toolsFS, ok := b.Sub("tools"); ok {
			regs, err := s.registerToolsFromFS(server, toolsFS, b.Source())
			if err != nil {
				Log.Error("Failed to reload bundle MCP tools", "error", err)
			} else {
				s.mcpBundleEntries.tools = regs.tools
				s.mcpBundleEntries.staticResources = regs.staticResources
				s.mcpBundleEntries.templateResources = regs.templateResources
				s.mcpBundleEntries.prompts = regs.prompts
				s.mcpBundleEntries.skills = regs.skills
			}
		}
		if resFS, ok := b.Sub("resources"); ok {
			static, template, err := s.registerResourcesFromFS(server, resFS, b.Source())
			if err != nil {
				Log.Error("Failed to reload bundle MCP resources", "error", err)
			} else {
				s.mcpBundleEntries.staticResources = append(s.mcpBundleEntries.staticResources, static...)
				s.mcpBundleEntries.templateResources = append(s.mcpBundleEntries.templateResources, template...)
			}
		}
		if promptFS, ok := b.Sub("prompts"); ok {
			names, err := s.registerPromptsFromFS(server, promptFS, b.Source())
			if err != nil {
				Log.Error("Failed to reload bundle MCP prompts", "error", err)
			} else {
				s.mcpBundleEntries.prompts = append(s.mcpBundleEntries.prompts, names...)
			}
		}
		if skillsFS, ok := b.Sub("skills"); ok {
			names, err := s.registerSkillsFromFS(server, skillsFS, b.Source())
			if err != nil {
				Log.Error("Failed to reload bundle MCP skills", "error", err)
			} else {
				s.mcpBundleEntries.skills = append(s.mcpBundleEntries.skills, names...)
			}
		}
		server.NotifyToolsChanged()
		server.NotifyResourcesChanged()
		server.NotifyPromptsChanged()
	}

	Log.Info("MCP reloaded successfully")
}

// registerSkillsFromFS registers one MCP skill (SEP-2640) per
// subdirectory containing a SKILL.md (the Agent Skills format). Every file
// in the skill directory becomes a skill resource; the SKILL.md
// frontmatter's name must match the directory name (enforced by the
// library's URI rules) and its description seeds the skill's frontmatter.
// Directories without a SKILL.md are skipped. Skills are registered once
// at startup; the watcher does not live-reload skill content.
func (s *Server) registerSkillsFromFS(server *mcp_lib.Server, fsys fs.FS, source string) ([]string, error) {
	dirs, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("%s: %w", source, err)
	}
	var names []string
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		skillMD, err := fs.ReadFile(fsys, path.Join(d.Name(), "SKILL.md"))
		if err != nil {
			continue // not a skill directory
		}
		if name := frontmatterValue(string(skillMD), "name"); name != "" && name != d.Name() {
			// One bad skill directory must not fail the whole server.
			Log.Warn("Skipping skill directory: SKILL.md frontmatter name must match the directory name",
				"source", source, "dir", d.Name(), "name", name)
			continue
		}
		// The listing frontmatter is parsed verbatim from the SKILL.md by
		// the library; Description is only the fallback for a file without
		// a frontmatter block.
		builder := mcp_lib.NewSkill(d.Name()).Description(frontmatterValue(string(skillMD), "description"))
		err = fs.WalkDir(fsys, d.Name(), func(p string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return err
			}
			content, err := fs.ReadFile(fsys, p)
			if err != nil {
				return err
			}
			builder.File(strings.TrimPrefix(p, d.Name()+"/"), content)
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("%s: %w", source, err)
		}
		if err := server.RegisterSkill(builder); err != nil {
			Log.Warn("Skipping MCP skill", "source", source, "name", d.Name(), "error", err)
			continue
		}
		names = append(names, d.Name())
		Log.Info("registered MCP skill", "name", d.Name())
	}
	return names, nil
}

// frontmatterValue pulls one top-level value out of a SKILL.md's YAML
// frontmatter block ("---\nkey: value\n---"), without a YAML dependency.
func frontmatterValue(doc, key string) string {
	lines := strings.Split(doc, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return ""
	}
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "---" {
			break
		}
		if strings.HasPrefix(line, key+":") {
			return strings.TrimSpace(strings.TrimPrefix(line, key+":"))
		}
	}
	return ""
}
