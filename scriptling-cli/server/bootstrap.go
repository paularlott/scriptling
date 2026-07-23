package server

import (
	"archive/zip"
	"fmt"
	"os"
	"strings"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs"
	scriptlingplugin "github.com/paularlott/scriptling/plugin"
	"github.com/paularlott/scriptling/scriptling-cli/bootstrap"
	"github.com/paularlott/scriptling/scriptling-cli/pack"
	"github.com/paularlott/scriptling/scriptling-cli/setup"
)

// NewServer creates a new HTTP server
func NewServer(config ServerConfig) (*Server, error) {
	s := &Server{
		config:               config,
		handlers:             make(map[string]string),
		wsHandlers:           make(map[string]string),
		jsonrpcMethods:       make(map[string]string),
		jsonrpcNotifications: make(map[string]string),
		staticRoutes:         make(map[string]string),
		bearerExpected:       "Bearer " + config.BearerToken,
		scriptDone:           make(chan struct{}),
	}

	// Pre-opened bundles take precedence over legacy package sources. The app
	// bundle is added last so its modules win over library bundles.
	if config.Bundle != nil || len(config.LibBundles) > 0 {
		loader := pack.NewLoader()
		loader.SetCacheDir(config.CacheDir)
		for _, b := range config.LibBundles {
			if err := loader.AddBundle(b); err != nil {
				return nil, err
			}
		}
		if config.Bundle != nil {
			if err := loader.AddBundle(config.Bundle); err != nil {
				return nil, err
			}
		}
		s.packLoader = loader
	} else {
		packLoader, err := bootstrap.NewPackLoader(config.Packages, config.Insecure, config.CacheDir)
		if err != nil {
			return nil, err
		}
		s.packLoader = packLoader
	}

	// An app bundle declares its protocols via serve; JSON-RPC bundles enable
	// the /json-rpc endpoint without requiring the CLI flag, and HTTP bundles
	// serve their webroot/ dir.
	serve := config.serveSet()
	if serve["json-rpc"] {
		s.config.JSONRPC = true
	}
	if serve["http"] && config.Bundle != nil {
		if webFS, ok := config.Bundle.Sub("webroot"); ok {
			s.webRootFS = webFS
		}
	}

	extlibs.ResetRuntime()

	if err := extlibs.InitKVStore(config.KVStoragePath); err != nil {
		return nil, fmt.Errorf("failed to initialize KV store: %w", err)
	}

	setup.Factories(config.LibDirs, config.AllowedPaths, config.DisabledLibs, config.SecretRegistry, Log, config.DockerSock, config.PodmanSock)

	// Initialize server lifecycle channels and the collection callback after
	// ResetRuntime. ServerCollect is called inside start_server() (and the
	// backward-compat goroutine exit path) while the RuntimeState lock is held,
	// so the route snapshot is atomic with the ServerStarted flag — anything
	// registered after start_server() returns is definitively excluded.
	extlibs.RuntimeState.Lock()
	extlibs.RuntimeState.ServerStartCh = make(chan struct{})
	extlibs.RuntimeState.ServerRunningCh = make(chan struct{})
	extlibs.RuntimeState.ServerCollect = func() {
		s.collectRoutes()
		s.collectJSONRPCMethods()
	}
	extlibs.RuntimeState.Unlock()

	hasScript := config.ScriptFile != "" || s.packLoader != nil

	// startErrCh carries a pre-start script error (buffered so goroutine never blocks).
	startErrCh := make(chan error, 1)

	go func() {
		defer close(s.scriptDone)

		var runErr error
		if hasScript {
			func() {
				defer func() {
					if r := recover(); r != nil {
						runErr = fmt.Errorf("setup script panicked: %v", r)
						Log.Error("Setup script panicked", "panic", r)
					}
				}()
				runErr = s.runSetupScript()
			}()
		}

		// If start_server() was not called, collect routes and signal start now
		// (backward compat). Mirrors the collection done inside start_server().
		extlibs.RuntimeState.Lock()
		alreadyStarted := extlibs.RuntimeState.ServerStarted
		if !alreadyStarted {
			extlibs.RuntimeState.ServerStarted = true
			if extlibs.RuntimeState.ServerCollect != nil {
				extlibs.RuntimeState.ServerCollect()
			}
			close(extlibs.RuntimeState.ServerStartCh)
			if runErr != nil {
				startErrCh <- runErr
			}
		} else if runErr != nil {
			Log.Error("Setup script error after server start", "error", runErr)
		}
		extlibs.RuntimeState.Unlock()
	}()

	// Wait until routes are collected and the start signal is sent.
	<-extlibs.RuntimeState.ServerStartCh

	// Check for a pre-start error (non-blocking — buffered channel).
	select {
	case err := <-startErrCh:
		if err != nil {
			<-s.scriptDone
			return nil, fmt.Errorf("setup script failed: %w", err)
		}
	default:
	}

	// Build the plugin server if the setup script called runtime.plugin.serve().
	// Must happen before buildMux so the HTTP /json-rpc handler can use it.
	s.buildPluginServer()

	if config.MCPToolsDir != "" || config.MCPExecTool || serve["mcp"] {
		if err := s.setupMCP(); err != nil {
			return nil, fmt.Errorf("MCP setup failed: %w", err)
		}
		if config.MCPToolsDir != "" {
			Log.Info("MCP tools enabled", "directory", config.MCPToolsDir)
		}
		if serve["mcp"] && config.Bundle != nil {
			Log.Info("MCP tools enabled from bundle", "source", config.Bundle.Source())
		}
		if config.MCPExecTool {
			Log.Info("MCP script execution tool enabled")
		}
	}

	// Routes and JSON-RPC methods were already collected inside start_server()
	// (or the backward-compat goroutine exit). Only background tasks remain.
	extlibs.ReleaseBackgroundTasks()

	// Open zip web root if configured
	if strings.HasSuffix(strings.ToLower(config.WebRoot), ".zip") {
		zr, err := zip.OpenReader(config.WebRoot)
		if err != nil {
			return nil, fmt.Errorf("failed to open web root zip %s: %w", config.WebRoot, err)
		}
		s.webRootZip = zr
	}

	return s, nil
}

// runSetupScript runs the setup script once to register routes
func (s *Server) runSetupScript() error {
	p := scriptling.New()
	s.setupScriptling(p)
	s.applyPackLoader(p)

	if s.config.ScriptFile != "" {
		Log.Debug("Running setup script", "file", s.config.ScriptFile)
		content, err := os.ReadFile(s.config.ScriptFile)
		if err != nil {
			return fmt.Errorf("failed to read setup script: %w", err)
		}
		p.SetSourceFile(s.config.ScriptFile)
		_, err = p.Eval(string(content))
		return err
	}

	if s.packLoader != nil {
		entry, found, err := s.packLoader.ResolveMain()
		if err != nil {
			return err
		}
		if found {
			if entry.Script != nil {
				Log.Debug("Running setup script from bundle", "file", entry.ScriptName)
				p.SetSourceFile(entry.ScriptName)
				_, err := p.Eval(string(entry.Script))
				return err
			}
			Log.Debug("Running setup from package", "module", entry.Module, "entry", entry.Function)
			_, err = p.Eval(fmt.Sprintf("import %s\n%s.%s()", entry.Module, entry.Module, entry.Function))
			return err
		}
	}
	return nil
}

// applyPackLoader sets the pack loader (if any) as the outermost loader on a scriptling instance.
func (s *Server) applyPackLoader(p *scriptling.Scriptling) {
	bootstrap.ApplyPackLoader(p, s.packLoader)
}

func (s *Server) setupScriptling(p *scriptling.Scriptling) {
	setup.Scriptling(p, s.config.LibDirs, false, s.config.AllowedPaths, s.config.DisabledLibs, s.config.SecretRegistry, Log, s.config.DockerSock, s.config.PodmanSock)
	setup.RegisterSys(p, s.config.Argv)
	if s.config.PluginManager != nil {
		scriptlingplugin.RegisterLibraries(p, s.config.PluginManager)
	}
	s.applyPackLoader(p)
	if s.packLoader != nil {
		pack.RegisterPackageLibrary(p, s.packLoader)
	}
	if s.config.ExtraLibs != nil {
		s.config.ExtraLibs(p)
	}
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
