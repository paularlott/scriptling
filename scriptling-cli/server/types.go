package server

import (
	"archive/zip"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
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
	Address        string
	ScriptFile     string
	LibDirs        []string
	Packages       []string // Package (.zip) paths or URLs
	Insecure       bool     // Allow self-signed HTTPS for package URLs
	CacheDir       string   // Override default OS cache dir for remote packages
	BearerToken    string
	AllowedPaths   []string // Filesystem path restrictions (empty = no restrictions)
	DisabledLibs   []string // Built-in libraries to disable (empty = all enabled)
	PluginDirs     []string
	PluginManager  *scriptlingplugin.Manager
	MCPToolsDir    string // Empty means MCP disabled
	MCPExecTool    bool   // Enable code execution tool
	JSONRPC        bool   // Mount JSON-RPC over HTTP at /json-rpc
	KVStoragePath  string // Empty means in-memory KV store
	WebRoot        string // Directory to serve static files from (empty = disabled)
	SecretRegistry *secretprovider.Registry
	DockerSock     string
	PodmanSock     string
	TLSCert        string
	TLSKey         string
	TLSGenerate    bool

	// ExtraLibs, if set, registers additional libraries on every evaluator
	// after the standard library set — the setup script and every json-rpc,
	// http, mcp and websocket request handler. Host applications use this to
	// expose their own libraries to served scripts.
	ExtraLibs func(*scriptling.Scriptling)
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
	config               ServerConfig
	httpServer           *http.Server
	mcpHandler           *reloadableMCPHandler
	handlers             map[string]string // path -> "library.function"
	wsHandlers           map[string]string // path -> "library.function" for WebSocket
	jsonrpcMethods       map[string]string // JSON-RPC method name -> "library.function"
	jsonrpcNotifications map[string]string // JSON-RPC notification name -> "library.function"
	middleware           string
	notFoundHandler      string
	staticRoutes         map[string]string
	webRootZip           *zip.ReadCloser // non-nil when WebRoot is a .zip file
	mu                   sync.RWMutex
	watcher              *fsnotify.Watcher
	reloadDebounce       *time.Timer
	debounceDuration     time.Duration
	packLoader           *pack.Loader // nil if no packages configured
	bearerExpected       string       // precomputed "Bearer <token>"
	scriptDone           chan struct{} // closed when setup script goroutine exits
}
