package server

import (
	"archive/zip"
	"io/fs"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/paularlott/jsonrpc"
	"github.com/paularlott/logger"
	mcp_lib "github.com/paularlott/mcp"
	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/secretprovider"
	scriptlingplugin "github.com/paularlott/scriptling/plugin"
	"github.com/paularlott/scriptling/scriptling-cli/pack"
)

var Log logger.Logger = logger.NewNullLogger()

// ServerConfig holds the configuration for the HTTP server
type ServerConfig struct {
	Address         string
	ScriptFile      string
	LibDirs         []string
	Packages        []string // Package (.zip) paths or URLs
	Bundle          *pack.Bundle   // The single app bundle to serve; when set the server runs in app-bundle mode
	LibBundles      []*pack.Bundle // Pre-opened library bundles (module providers only, no app behavior)
	Insecure        bool     // Allow self-signed HTTPS for package URLs
	CacheDir        string   // Override default OS cache dir for remote packages
	BearerToken     string
	AllowedPaths    []string // Filesystem path restrictions (empty = no restrictions)
	DisabledLibs    []string // Built-in libraries to disable (empty = all enabled)
	PluginDirs      []string
	PluginManager   *scriptlingplugin.Manager
	MCPToolsDir     string // Empty means MCP disabled
	MCPResourcesDir string // Folder of resource .toml/.py pairs (empty = none)
	MCPPromptsDir   string // Folder of prompt .toml/.py pairs (empty = none)
	MCPExecTool     bool   // Enable code execution tool
	JSONRPC         bool   // Mount JSON-RPC over HTTP at /json-rpc
	KVStoragePath   string // Empty means in-memory KV store
	WebRoot         string // Directory to serve static files from (empty = disabled)
	SecretRegistry  *secretprovider.Registry
	DockerSock      string
	PodmanSock      string
	TLSCert         string
	TLSKey          string
	TLSGenerate     bool

	// ExtraLibs, if set, registers additional libraries on every evaluator
	// after the standard library set — the setup script and every json-rpc,
	// http, mcp and websocket request handler. Host applications use this to
	// expose their own libraries to served scripts.
	ExtraLibs func(*scriptling.Scriptling)
}

// serveSet returns the set of protocols the app bundle declares in its
// manifest serve list (empty when there is no app bundle).
func (c ServerConfig) serveSet() map[string]bool {
	set := map[string]bool{}
	if c.Bundle != nil {
		for _, s := range c.Bundle.Manifest.Serve {
			set[s] = true
		}
	}
	return set
}

// appMode reports whether the server is running an app bundle rather than
// legacy flag-based configuration.
func (c ServerConfig) appMode() bool {
	return c.Bundle != nil
}

// reloadableMCPHandler wraps an MCP server pointer to allow hot-reloading of tools
type reloadableMCPHandler struct {
	server atomic.Pointer[mcp_lib.Server]
}

func (h *reloadableMCPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	Log.Trace("MCP request", "method", r.Method, "path", r.URL.Path, "remote", r.RemoteAddr)
	server := h.server.Load()
	if server == nil {
		http.Error(w, "Server not ready", http.StatusServiceUnavailable)
		return
	}
	(*server).HandleRequest(w, r)
}

// Server represents the HTTP server
type Server struct {
	config                ServerConfig
	httpServer            *http.Server
	mcpHandler            *reloadableMCPHandler
	pluginServer          *scriptlingplugin.Server // non-nil when plugin mode is active
	handlers              map[string]string        // path -> "library.function"
	wsHandlers            map[string]string        // path -> "library.function" for WebSocket
	jsonrpcMethods        map[string]string        // JSON-RPC method name -> "library.function"
	jsonrpcNotifications  map[string]string        // JSON-RPC notification name -> "library.function"
	jsonrpcServer         *jsonrpc.Server          // built lazily from the maps above
	jsonrpcServerOnce     sync.Once
	middleware            string
	notFoundHandler       string
	staticRoutes          map[string]string
	webRootZip            *zip.ReadCloser // non-nil when WebRoot is a .zip file
	mu                    sync.RWMutex
	watcher               *fsnotify.Watcher
	reloadDebounce        *time.Timer
	debounceDuration      time.Duration
	mcpFolderEntries mcpEntries    // folder-sourced MCP registrations (reloadable)
	mcpBundleEntries mcpEntries    // bundle-sourced MCP registrations (reloadable)
	packLoader       *pack.Loader  // nil if no packages configured
	webRootFS        fs.FS         // non-nil when serving webroot/ from the app bundle (cached sub-FS)
	bearerExpected   string        // precomputed "Bearer <token>"
	scriptDone       chan struct{} // closed when setup script goroutine exits
}

// mcpEntries tracks what was registered on the MCP server from one source so
// reload can unregister it all before re-registering.
type mcpEntries struct {
	tools             []string // tool names
	staticResources   []string // static resource URIs
	templateResources []string // resource template URI templates
	prompts           []string // prompt names
}

// unregisterAll removes every tracked entry from the MCP server.
func (e mcpEntries) unregisterAll(server *mcp_lib.Server) {
	for _, name := range e.tools {
		server.UnregisterTool(name)
	}
	for _, uri := range e.staticResources {
		server.UnregisterResource(uri)
	}
	for _, uriTmpl := range e.templateResources {
		server.UnregisterResourceTemplate(uriTmpl)
	}
	for _, name := range e.prompts {
		server.UnregisterPrompt(name)
	}
}
