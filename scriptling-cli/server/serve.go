package server

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/paularlott/logger"
	mcp_lib "github.com/paularlott/mcp"
	"github.com/paularlott/mcp/toolmetadata"
	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs"
	scriptlingmcp "github.com/paularlott/scriptling/extlibs/mcp"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/util"

	mcpcli "github.com/paularlott/scriptling/scriptling-cli/mcp"
)

var Log logger.Logger

// toOSSignals converts syscall.Signal slice to os.Signal slice
func toOSSignals(sigs []syscall.Signal) []os.Signal {
	result := make([]os.Signal, len(sigs))
	for i, sig := range sigs {
		result[i] = sig
	}
	return result
}

// ServerConfig holds the configuration for the HTTP server
type ServerConfig struct {
	Address     string
	ScriptFile  string
	LibDir      string
	BearerToken string
	ScriptMode  string // "safe" or "full"
	MCPToolsDir string // Empty means MCP disabled
	TLSCert     string
	TLSKey      string
	TLSGenerate bool
}

// scriptHandler holds the handler function reference
type scriptHandler struct {
	handlerRef string // "library.function"
	libDir     string
	safeMode   bool
}

// reloadableMCPHandler wraps an MCP server pointer to allow hot-reloading of tools
type reloadableMCPHandler struct {
	server atomic.Pointer[mcp_lib.Server]
}

func (h *reloadableMCPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	server := h.server.Load()
	if server == nil {
		http.Error(w, "Server not ready", http.StatusServiceUnavailable)
		return
	}
	(*server).HandleRequest(w, r)
}

// Server represents the HTTP server
type Server struct {
	config           ServerConfig
	httpServer       *http.Server
	scriptling       *scriptling.Scriptling
	mcpServer        *mcp_lib.Server
	mcpHandler       *reloadableMCPHandler
	handlers         map[string]*scriptHandler
	middleware       string
	staticRoutes     map[string]string
	mu               sync.RWMutex
	watcher          *fsnotify.Watcher
	reloadDebounce   *time.Timer
	debounceDuration time.Duration
}

// NewServer creates a new HTTP server
func NewServer(config ServerConfig) (*Server, error) {
	s := &Server{
		config:       config,
		handlers:     make(map[string]*scriptHandler),
		staticRoutes: make(map[string]string),
	}

	// Reset routes from previous runs
	extlibs.ResetRuntime()

	// Run setup script if provided
	if config.ScriptFile != "" {
		if err := s.runSetupScript(); err != nil {
			return nil, fmt.Errorf("setup script failed: %w", err)
		}
	}

	// Set up MCP if tools directory provided
	if config.MCPToolsDir != "" {
		if err := s.setupMCP(); err != nil {
			return nil, fmt.Errorf("MCP setup failed: %w", err)
		}
		Log.Info("MCP tools enabled", "directory", config.MCPToolsDir)
	}

	// Collect registered routes from scriptling.runtime library
	s.collectRoutes()

	// Start background tasks if any
	extlibs.ReleaseBackgroundTasks()

	return s, nil
}

// runSetupScript runs the setup script once to register routes
func (s *Server) runSetupScript() error {
	content, err := os.ReadFile(s.config.ScriptFile)
	if err != nil {
		return fmt.Errorf("failed to read setup script: %w", err)
	}

	// Create scriptling instance for setup
	p := scriptling.New()
	setupScriptling(p, s.config.LibDir, s.config.ScriptMode == "safe")

	// Execute setup script
	_, err = p.Eval(string(content))
	return err
}

// setupScriptling configures a Scriptling instance with libraries
func setupScriptling(p *scriptling.Scriptling, libDir string, safeMode bool) {
	// Register the Runtime library for route registration, KV, and background tasks
	extlibs.RegisterRuntimeLibraryAll(p)

	// Also set up the standard libraries
	mcpcli.SetupScriptling(p, libDir, false, safeMode, Log)

	// Set factory for background tasks
	extlibs.SetBackgroundFactory(func() interface{ LoadLibraryIntoEnv(string, *object.Environment) error } {
		newP := scriptling.New()
		mcpcli.SetupScriptling(newP, libDir, false, safeMode, Log)
		return newP
	})
}

// setupMCP initializes the MCP server if configured
func (s *Server) setupMCP() error {
	safeMode := s.config.ScriptMode == "safe"

	// Create reloadable handler for hot-reloading tools
	s.mcpHandler = &reloadableMCPHandler{}
	s.debounceDuration = 500 * time.Millisecond

	// Create initial MCP server
	server, err := s.createMCPServer(safeMode)
	if err != nil {
		return err
	}

	s.mcpServer = server
	s.mcpHandler.server.Store(server)

	// Set up file watcher for tools folder
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

	return nil
}

// createMCPServer creates a new MCP server with all tools registered
func (s *Server) createMCPServer(safeMode bool) (*mcp_lib.Server, error) {
	server := mcp_lib.NewServer("scriptling-server", "1.0.0")
	server.SetInstructions("Execute Scriptling tools from the tools folder.")

	// Register tools from folder
	tools, err := mcpcli.ScanToolsFolder(s.config.MCPToolsDir)
	if err != nil {
		return nil, err
	}

	for toolName, meta := range tools {
		scriptPath := filepath.Join(s.config.MCPToolsDir, toolName+".py")
		tool := toolmetadata.BuildMCPTool(toolName, meta)
		handler := createMCPToolHandler(scriptPath, s.config.LibDir, safeMode)
		server.RegisterTool(tool, handler)

		mode := "native"
		if meta.Discoverable {
			mode = "discoverable"
		}
		Log.Info("Registered MCP tool", "name", toolName, "params", len(meta.Parameters), "mode", mode)
	}

	return server, nil
}

// reloadMCPTools reloads all MCP tools
func (s *Server) reloadMCPTools() {
	Log.Info("Reloading MCP tools...")
	safeMode := s.config.ScriptMode == "safe"

	newServer, err := s.createMCPServer(safeMode)
	if err != nil {
		Log.Error("Failed to reload MCP tools", "error", err)
	} else {
		s.mcpHandler.server.Store(newServer)
		s.mu.Lock()
		s.mcpServer = newServer
		s.mu.Unlock()
		Log.Info("MCP tools reloaded successfully")
	}
}

// collectRoutes collects registered routes from the scriptling.runtime library
func (s *Server) collectRoutes() {
	routes := extlibs.RuntimeState.Routes
	s.middleware = extlibs.RuntimeState.Middleware

	for path, route := range routes {
		if route.Static {
			s.staticRoutes[path] = route.StaticDir
		} else {
			s.handlers[path] = &scriptHandler{
				handlerRef: route.Handler,
				libDir:     s.config.LibDir,
				safeMode:   s.config.ScriptMode == "safe",
			}
		}
		Log.Info("Registered route", "path", path, "methods", route.Methods, "handler", route.Handler)
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// Register MCP endpoint if configured
	if s.mcpHandler != nil {
		mux.Handle("/mcp", s.mcpHandler)
	}

	// Health check endpoint
	mux.HandleFunc("GET /health", s.handleHealth)

	// Register dynamic script handlers
	for path := range s.handlers {
		mux.HandleFunc(path, s.handleScriptRequest)
	}

	// Register static file handlers
	for path, dir := range s.staticRoutes {
		fs := http.FileServer(http.Dir(dir))
		mux.Handle(path, http.StripPrefix(path, fs))
	}

	// Apply authentication middleware
	var handler http.Handler = mux
	if s.config.BearerToken != "" && s.middleware == "" {
		// Bearer token protects all endpoints if no custom middleware
		handler = s.bearerTokenMiddleware(mux)
	} else if s.config.BearerToken != "" && s.middleware != "" {
		// Bearer token protects MCP only, custom middleware handles script routes
		handler = s.bearerTokenMCPOnlyMiddleware(mux)
	}

	// Create HTTP server
	s.httpServer = &http.Server{
		Addr:    s.config.Address,
		Handler: handler,
	}

	// Set up TLS if configured
	if s.config.TLSGenerate || (s.config.TLSCert != "" && s.config.TLSKey != "") {
		if s.config.TLSGenerate {
			cert, err := s.generateSelfSignedCert()
			if err != nil {
				return fmt.Errorf("failed to generate certificate: %w", err)
			}
			s.httpServer.TLSConfig = &tls.Config{
				Certificates: []tls.Certificate{cert},
			}
			Log.Info("Using self-signed certificate")
		} else {
			s.httpServer.TLSConfig = &tls.Config{
				MinVersion: tls.VersionTLS12,
			}
		}
	}

	// Start server in goroutine
	go func() {
		var err error

		if s.config.TLSGenerate || (s.config.TLSCert != "" && s.config.TLSKey != "") {
			if s.config.TLSCert != "" && s.config.TLSKey != "" {
				err = s.httpServer.ListenAndServeTLS(s.config.TLSCert, s.config.TLSKey)
			} else {
				err = s.httpServer.ListenAndServeTLS("", "")
			}
		} else {
			err = s.httpServer.ListenAndServe()
		}

		if err != nil && err != http.ErrServerClosed {
			Log.Error("Server error", "error", err)
		}
	}()

	return nil
}

// Stop gracefully stops the server
func (s *Server) Stop(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// handleHealth handles health check requests
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// handleScriptRequest handles requests to script handlers
func (s *Server) handleScriptRequest(w http.ResponseWriter, r *http.Request) {
	// Find matching handler
	s.mu.RLock()
	_, ok := s.handlers[r.URL.Path]
	s.mu.RUnlock()

	if !ok {
		// Try to find a matching route with trailing slash handling
		path := r.URL.Path
		if !strings.HasSuffix(path, "/") {
			path += "/"
		}
		s.mu.RLock()
		_, ok = s.handlers[path]
		s.mu.RUnlock()
	}

	if !ok {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	// Check method
	route := extlibs.RuntimeState.Routes[r.URL.Path]
	if route != nil {
		methodAllowed := false
		for _, m := range route.Methods {
			if m == r.Method {
				methodAllowed = true
				break
			}
		}
		if !methodAllowed {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
	}

	// Run middleware if configured
	if s.middleware != "" {
		// Create request object for middleware
		reqObj := s.createRequestObject(r)

		// Run middleware
		resp := s.runHandler(s.middleware, reqObj)
		if resp != nil {
			// Middleware returned a response - short-circuit
			s.writeResponse(w, resp)
			return
		}
	}

	// Create request object
	reqObj := s.createRequestObject(r)

	// Get handler reference from route
	handlerRef := ""
	if route != nil {
		handlerRef = route.Handler
	}

	// Run handler
	resp := s.runHandler(handlerRef, reqObj)
	if resp == nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Write response
	s.writeResponse(w, resp)
}

// createRequestObject creates a Request instance from an HTTP request
func (s *Server) createRequestObject(r *http.Request) *object.Instance {
	// Read body
	var body string
	if r.Body != nil {
		bodyBytes, _ := io.ReadAll(r.Body)
		body = string(bodyBytes)
	}

	// Convert headers
	headers := make(map[string]string)
	for k, v := range r.Header {
		if len(v) > 0 {
			headers[strings.ToLower(k)] = v[0]
		}
	}

	// Convert query params
	query := make(map[string]string)
	for k, v := range r.URL.Query() {
		if len(v) > 0 {
			query[k] = v[0]
		}
	}

	return extlibs.CreateRequestInstance(r.Method, r.URL.Path, body, headers, query)
}

// runHandler runs a handler function and returns the response
func (s *Server) runHandler(handlerRef string, reqObj *object.Instance) *object.Dict {
	// Parse handler reference (e.g., "mylib.testHandler")
	parts := strings.SplitN(handlerRef, ".", 2)
	if len(parts) != 2 {
		Log.Error("Invalid handler reference", "handler", handlerRef)
		return nil
	}
	libName := parts[0]

	// Create fresh scriptling environment
	p := scriptling.New()
	setupScriptling(p, s.config.LibDir, s.config.ScriptMode == "safe")

	// Import the library
	if err := p.Import(libName); err != nil {
		Log.Error("Failed to import library", "library", libName, "error", err)
		return nil
	}

	// Call the handler function using the full dotted path
	result, err := p.CallFunction(handlerRef, reqObj)
	if err != nil {
		Log.Error("Handler error", "error", err)
		// Return error response
		return object.NewStringDict(map[string]object.Object{
			"status":  object.NewInteger(500),
			"headers": &object.Dict{Pairs: map[string]object.DictPair{}},
			"body":    &object.String{Value: fmt.Sprintf(`{"error": "%s"}`, err.Error())},
		})
	}

	// Convert result to Dict
	if dict, ok := result.(*object.Dict); ok {
		return dict
	}

	// If not a dict, wrap as JSON response
	return object.NewStringDict(map[string]object.Object{
		"status":  object.NewInteger(200),
		"headers": &object.Dict{Pairs: map[string]object.DictPair{}},
		"body":    result,
	})
}

// writeResponse writes a response dict to the HTTP response writer
func (s *Server) writeResponse(w http.ResponseWriter, resp *object.Dict) {
	// Get status code
	status := int64(200)
	if statusObj, ok := resp.GetByString("status"); ok {
		if statusInt, err := statusObj.Value.AsInt(); err == nil {
			status = statusInt
		}
	}

	// Get headers
	if headersObj, ok := resp.GetByString("headers"); ok {
		if headersDict, err := headersObj.Value.AsDict(); err == nil {
			for k, v := range headersDict {
				if strVal, err := v.AsString(); err == nil {
					w.Header().Set(k, strVal)
				}
			}
		}
	}

	// Get body
	var bodyBytes []byte
	if bodyObj, ok := resp.GetByString("body"); ok {
		// Check if body is a string or needs JSON encoding
		if strVal, err := bodyObj.Value.AsString(); err == nil {
			bodyBytes = []byte(strVal)
		} else {
			// JSON encode
			jsonBytes, err := json.Marshal(objectToInterface(bodyObj.Value))
			if err != nil {
				Log.Error("Failed to encode JSON response", "error", err)
				bodyBytes = []byte(`{"error": "JSON encoding failed"}`)
			} else {
				bodyBytes = jsonBytes
				// Set JSON content type if not already set
				if w.Header().Get("Content-Type") == "" {
					w.Header().Set("Content-Type", "application/json")
				}
			}
		}
	}

	w.WriteHeader(int(status))
	w.Write(bodyBytes)
}

// objectToInterface converts a scriptling Object to a Go interface{}
func objectToInterface(obj object.Object) interface{} {
	switch v := obj.(type) {
	case *object.String:
		return v.Value
	case *object.Integer:
		return v.Value
	case *object.Float:
		return v.Value
	case *object.Boolean:
		return v.Value
	case *object.Null:
		return nil
	case *object.List:
		result := make([]interface{}, len(v.Elements))
		for i, elem := range v.Elements {
			result[i] = objectToInterface(elem)
		}
		return result
	case *object.Dict:
		result := make(map[string]interface{})
		for _, pair := range v.Pairs {
			result[pair.StringKey()] = objectToInterface(pair.Value)
		}
		return result
	default:
		return nil
	}
}

// bearerTokenMiddleware creates authentication middleware for all endpoints
func (s *Server) bearerTokenMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		expected := "Bearer " + s.config.BearerToken

		if auth != expected {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// bearerTokenMCPOnlyMiddleware creates authentication middleware for MCP only
func (s *Server) bearerTokenMCPOnlyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only protect MCP endpoint
		if r.URL.Path == "/mcp" {
			auth := r.Header.Get("Authorization")
			expected := "Bearer " + s.config.BearerToken

			if auth != expected {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

// generateSelfSignedCert generates a self-signed certificate
func (s *Server) generateSelfSignedCert() (tls.Certificate, error) {
	hosts := util.GetCertificateHosts(s.config.Address)
	return util.GenerateSelfSignedCertificate(util.CertificateConfig{
		Hosts: hosts,
	})
}

// RunServer is the main entry point for running the server
func RunServer(ctx context.Context, config ServerConfig) error {
	server, err := NewServer(config)
	if err != nil {
		return err
	}

	if err := server.Start(); err != nil {
		return err
	}

	Log.Info("Server started", "address", config.Address)
	if config.MCPToolsDir != "" {
		Log.Info(getReloadMessage())
	} else {
		Log.Info("Press Ctrl+C to exit")
	}

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signals := append([]os.Signal{os.Interrupt, syscall.SIGTERM}, toOSSignals(reloadSignals)...)
	signal.Notify(sigChan, signals...)

	// Handle file watcher events in a goroutine
	watcherDone := make(chan struct{})
	if server.watcher != nil {
		go func() {
			defer close(watcherDone)
			for {
				select {
				case event, ok := <-server.watcher.Events:
					if !ok {
						return
					}
					// Only watch for .toml file changes (create, write, remove, rename)
					if filepath.Ext(event.Name) == ".toml" {
						// Debounce: reset timer on each event
						if server.reloadDebounce != nil {
							server.reloadDebounce.Stop()
						}
						eventCopy := event
						server.reloadDebounce = time.AfterFunc(server.debounceDuration, func() {
							Log.Debug("Tool file changed", "event", eventCopy.Op.String(), "file", filepath.Base(eventCopy.Name))
							server.reloadMCPTools()
						})
					}
				case err, ok := <-server.watcher.Errors:
					if ok {
						Log.Error("File watcher error", "error", err)
					}
				}
			}
		}()
	}

	// Wait for signals
	sig := <-sigChan

	// Handle reload signals
	if sysSig, ok := sig.(syscall.Signal); ok && isReloadSignal(sysSig) {
		if config.MCPToolsDir != "" {
			server.reloadMCPTools()
		}
		// Continue waiting for shutdown signal
		sig = <-sigChan
	}

	// Clean shutdown
	Log.Info("Shutting down server...")

	// Clean up watcher
	if server.watcher != nil {
		server.watcher.Close()
		<-watcherDone
	}
	if server.reloadDebounce != nil {
		server.reloadDebounce.Stop()
	}

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Stop(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown failed: %w", err)
	}

	Log.Info("Server stopped")
	return nil
}

// createMCPToolHandler creates a handler function for an MCP tool
func createMCPToolHandler(scriptPath string, libDir string, safeMode bool) func(context.Context, *mcp_lib.ToolRequest) (*mcp_lib.ToolResponse, error) {
	return func(ctx context.Context, req *mcp_lib.ToolRequest) (*mcp_lib.ToolResponse, error) {
		script, err := os.ReadFile(scriptPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read script: %w", err)
		}

		p := scriptling.New()
		setupScriptling(p, libDir, safeMode)

		params := req.Args()
		response, exitCode, err := scriptlingmcp.RunToolScript(ctx, p, string(script), params)
		if err != nil {
			return nil, fmt.Errorf("script execution failed: %w", err)
		}

		if exitCode != 0 {
			return nil, fmt.Errorf("script exited with code %d: %s", exitCode, response)
		}

		return mcp_lib.NewToolResponseText(response), nil
	}
}
