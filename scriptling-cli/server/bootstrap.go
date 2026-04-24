package server

import (
	"fmt"
	"os"
	"strings"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs"
	"github.com/paularlott/scriptling/scriptling-cli/bootstrap"
	"github.com/paularlott/scriptling/scriptling-cli/setup"
)

// NewServer creates a new HTTP server
func NewServer(config ServerConfig) (*Server, error) {
	s := &Server{
		config:         config,
		handlers:       make(map[string]string),
		wsHandlers:     make(map[string]string),
		staticRoutes:   make(map[string]string),
		bearerExpected: "Bearer " + config.BearerToken,
	}

	packLoader, err := bootstrap.NewPackLoader(config.Packages, config.Insecure, config.CacheDir)
	if err != nil {
		return nil, err
	}
	s.packLoader = packLoader

	extlibs.ResetRuntime()

	if err := extlibs.InitKVStore(config.KVStoragePath); err != nil {
		return nil, fmt.Errorf("failed to initialize KV store: %w", err)
	}

	setup.Factories(config.LibDirs, config.AllowedPaths, config.DisabledLibs, config.SecretRegistry, Log, config.DockerSock, config.PodmanSock)

	if config.ScriptFile != "" || s.packLoader != nil {
		if err := s.runSetupScript(); err != nil {
			return nil, fmt.Errorf("setup script failed: %w", err)
		}
	}

	if config.MCPToolsDir != "" || config.MCPExecTool {
		if err := s.setupMCP(); err != nil {
			return nil, fmt.Errorf("MCP setup failed: %w", err)
		}
		if config.MCPToolsDir != "" {
			Log.Info("MCP tools enabled", "directory", config.MCPToolsDir)
		}
		if config.MCPExecTool {
			Log.Info("MCP script execution tool enabled")
		}
	}

	s.collectRoutes()
	extlibs.ReleaseBackgroundTasks()

	return s, nil
}

// runSetupScript runs the setup script once to register routes
func (s *Server) runSetupScript() error {
	p := scriptling.New()
	setup.Scriptling(p, s.config.LibDirs, false, s.config.AllowedPaths, s.config.DisabledLibs, s.config.SecretRegistry, Log, s.config.DockerSock, s.config.PodmanSock)

	if s.config.ScriptFile != "" {
		content, err := os.ReadFile(s.config.ScriptFile)
		if err != nil {
			return fmt.Errorf("failed to read setup script: %w", err)
		}
		_, err = p.Eval(string(content))
		return err
	}

	if s.packLoader != nil {
		if mod, fn, ok := s.packLoader.GetMainEntry(); ok {
			_, err := p.Eval(fmt.Sprintf("import %s\n%s.%s()", mod, mod, fn))
			return err
		}
	}
	return nil
}

// applyPackLoader sets the pack loader (if any) as the outermost loader on a scriptling instance.
func (s *Server) applyPackLoader(p *scriptling.Scriptling) {
	bootstrap.ApplyPackLoader(p, s.packLoader)
}

// collectRoutes collects registered routes from the scriptling.runtime library
func (s *Server) collectRoutes() {
	s.middleware = extlibs.RuntimeState.Middleware
	s.notFoundHandler = extlibs.RuntimeState.NotFoundHandler
	for key, route := range extlibs.RuntimeState.Routes {
		if route.Static {
			// key is "GET path" for static routes; extract the path
			_, path, _ := strings.Cut(key, " ")
			s.staticRoutes[path] = route.StaticDir
		} else {
			s.handlers[key] = route.Handler
		}
		Log.Info("Registered route", "key", key, "handler", route.Handler)
	}
	for path, wsRoute := range extlibs.RuntimeState.WebSocketRoutes {
		s.wsHandlers[path] = wsRoute.Handler
		Log.Info("Registered WebSocket route", "path", path, "handler", wsRoute.Handler)
	}
}
