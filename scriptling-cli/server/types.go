package server

import (
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/paularlott/logger"
	mcp_lib "github.com/paularlott/mcp"
	"github.com/paularlott/scriptling/extlibs/secretprovider"
	"github.com/paularlott/scriptling/scriptling-cli/pack"
)

var Log logger.Logger

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
	MCPToolsDir    string   // Empty means MCP disabled
	MCPExecTool    bool     // Enable code execution tool
	KVStoragePath  string   // Empty means in-memory KV store
	SecretRegistry *secretprovider.Registry
	TLSCert        string
	TLSKey         string
	TLSGenerate    bool
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
	mcpHandler       *reloadableMCPHandler
	handlers         map[string]string // path -> "library.function"
	wsHandlers       map[string]string // path -> "library.function" for WebSocket
	middleware       string
	staticRoutes     map[string]string
	mu               sync.RWMutex
	watcher          *fsnotify.Watcher
	reloadDebounce   *time.Timer
	debounceDuration time.Duration
	packLoader       *pack.Loader // nil if no packages configured
	bearerExpected   string       // precomputed "Bearer <token>"
}
