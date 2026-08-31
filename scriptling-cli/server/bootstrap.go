package server

import (
	"archive/zip"
	"fmt"
	"os"
	"strings"
	"time"

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
		serverRunningCh:      make(chan struct{}),
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

	if err := extlibs.InitKVStore(config.KVStoragePath, Log); err != nil {
		return nil, fmt.Errorf("failed to initialize KV store: %w", err)
	}

	setup.Factories(config.LibDirs, config.AllowedPaths, config.DisabledLibs, config.SecretRegistry, Log, config.DockerSock, config.PodmanSock, config.NetworkPolicy)

	// Initialize server lifecycle channels and the collection callback after
	// ResetRuntime. ServerCollect is called inside start_server() (and the
	// backward-compat goroutine exit path) while the RuntimeState lock is held,
	// so the route snapshot is atomic with the ServerStarted flag — anything
	// registered after start_server() returns is definitively excluded.
	extlibs.RuntimeState.Lock()
	extlibs.RuntimeState.ServerStartCh = make(chan struct{})
	extlibs.RuntimeState.ServerRunningCh = s.serverRunningCh
	extlibs.RuntimeState.ServerCollect = func() {
		s.collectRoutes()
		s.collectJSONRPCMethods()
	}
	extlibs.RuntimeState.Unlock()

	// Background task instances come from the process-wide factory, which
	// knows nothing about this server's packages: a task handler named as a
	// bundle module ("mod.fn") would not resolve. Layer the pack loader onto
	// the factory so server-mode tasks see the same modules request handlers
	// do. Like the rest of extlibs.RuntimeState this is process-global: the
	// last server to configure a loader wins, which matches the one-server
	// CLI process and the existing global release of background tasks.
	if s.packLoader != nil {
		if base := extlibs.BackgroundFactory(); base != nil {
			loader := s.packLoader
			extlibs.SetBackgroundFactory(func() extlibs.SandboxInstance {
				p := base()
				if instance, ok := p.(*scriptling.Scriptling); ok {
					bootstrap.ApplyPackLoader(instance, loader)
				}
				return p
			})
		}
	}

	hasScript := config.ScriptFile != "" || len(config.ScriptSource) > 0 || s.packLoader != nil

	// From this point onward, construction owns the setup lifecycle. Any error
	// before the Server is returned must release background work and stop a
	// setup script blocked in server_running(); callers cannot clean up a nil
	// Server. Watchers and an opened web-root archive are construction-owned too.
	setupTransferred := false
	defer func() {
		if setupTransferred {
			return
		}
		s.shutdownSetup()
		if s.watcher != nil {
			_ = s.watcher.Close()
		}
		if s.webRootZip != nil {
			_ = s.webRootZip.Close()
		}
	}()

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
		if !alreadyStarted && extlibs.RuntimeState.ServerStartCh == nil {
			// Stale goroutine: a newer server's ResetRuntime cleared the start
			// channel (and ServerStarted) before this one — leaked from an
			// earlier server, e.g. a test that never signalled shutdown — could
			// signal. Abandon; the new server drives its own lifecycle, and
			// closing the nil channel or flipping ServerStarted would panic or
			// hang it.
			extlibs.RuntimeState.Unlock()
			if runErr != nil {
				Log.Error("Setup script error after server start", "error", runErr)
			}
			return
		}
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

	// Conflicting routes are a configuration error, not something to serve
	// around: decide deterministically before anything binds. Signal setup
	// shutdown and wait boundedly so a server_running loop cannot leak.
	if err := s.checkRouteConflicts(); err != nil {
		s.shutdownSetup()
		return nil, err
	}

	// Build the plugin server if the setup script called runtime.plugin.serve().
	// Must happen before buildMux so the HTTP /json-rpc handler can use it.
	s.buildPluginServer()

	if s.mcpEnabled() {
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
	s.releaseBackgroundTasks()

	// Open zip web root if configured
	if strings.HasSuffix(strings.ToLower(config.WebRoot), ".zip") {
		zr, err := zip.OpenReader(config.WebRoot)
		if err != nil {
			return nil, fmt.Errorf("failed to open web root zip %s: %w", config.WebRoot, err)
		}
		s.webRootZip = zr
	}

	setupTransferred = true
	return s, nil
}

const setupShutdownTimeout = 5 * time.Second

// mcpEnabled is the single predicate used both before MCP setup and when
// probing built-in route conflicts.
func (s *Server) mcpEnabled() bool {
	return s.config.MCPToolsDir != "" || s.config.MCPExecTool || s.config.serveSet()["mcp"]
}

func (s *Server) releaseBackgroundTasks() {
	s.backgroundReleaseOnce.Do(extlibs.ReleaseBackgroundTasks)
}

// shutdownSetup releases setup-created work, signals server_running() to stop,
// and waits boundedly for the setup goroutine. Signalling and release are
// idempotent so normal lifecycle cleanup can safely share this path.
func (s *Server) shutdownSetup() {
	s.setupShutdownOnce.Do(func() {
		extlibs.RuntimeState.Lock()
		if extlibs.RuntimeState.ServerRunningCh == s.serverRunningCh {
			close(s.serverRunningCh)
			extlibs.RuntimeState.ServerRunningCh = nil
		}
		extlibs.RuntimeState.Unlock()

		s.releaseBackgroundTasks()
	})

	if s.scriptDone == nil {
		return
	}
	timer := time.NewTimer(setupShutdownTimeout)
	defer timer.Stop()
	select {
	case <-s.scriptDone:
	case <-timer.C:
		Log.Warn("Setup script did not exit within shutdown timeout")
	}
}

// runSetupScript runs the setup script once to register routes
func (s *Server) runSetupScript() error {
	p := scriptling.New()
	s.setupScriptling(p)
	s.applyPackLoader(p)

	// In-memory source wins: a script fetched from a plugin scheme source has
	// no file on disk, and staging one would leave a temp file behind.
	if len(s.config.ScriptSource) > 0 {
		name := s.config.ScriptName
		if name == "" {
			name = "<script>"
		}
		Log.Debug("Running setup script", "source", name)
		p.SetSourceFile(name)
		_, err := p.Eval(string(s.config.ScriptSource))
		return err
	}

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
	setup.Scriptling(p, s.config.LibDirs, false, s.config.AllowedPaths, s.config.DisabledLibs, s.config.SecretRegistry, Log, s.config.DockerSock, s.config.PodmanSock, s.config.NetworkPolicy)
	setup.RegisterSys(p, s.config.Argv)
	// Compiled-in plugins register even with no external plugin manager;
	// RegisterLibraries handles a nil manager.
	scriptlingplugin.RegisterLibraries(p, s.config.PluginManager, scriptlingplugin.PolicyFromSecurity(s.config.NetworkPolicy, s.config.AllowedPaths))
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
