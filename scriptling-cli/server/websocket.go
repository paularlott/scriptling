package server

import (
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"

	"github.com/gorilla/websocket"
	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/scriptling-cli/setup"
)

// websocketUpgrader upgrades HTTP connections to WebSocket
var websocketUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Allow all origins for now - can be configured later
		return true
	},
}

// websocketConnCounter generates unique IDs for WebSocket connections
var websocketConnCounter int64

// isWebSocketRequest checks if an HTTP request is a WebSocket upgrade request
func isWebSocketRequest(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket") &&
		strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade")
}

// handleWebSocketUpgrade handles a WebSocket upgrade request
func (s *Server) handleWebSocketUpgrade(w http.ResponseWriter, r *http.Request, path string) {
	route := extlibs.RuntimeState.WebSocketRoutes[path]
	if route == nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	// Upgrade the connection
	conn, err := websocketUpgrader.Upgrade(w, r, nil)
	if err != nil {
		Log.Error("WebSocket upgrade failed", "path", path, "error", err)
		return
	}

	// Generate unique connection ID
	connID := fmt.Sprintf("ws_%d", atomic.AddInt64(&websocketConnCounter, 1))

	// Create wrapped connection
	wsConn := extlibs.NewWebSocketServerConn(conn, connID)

	Log.Info("WebSocket connected", "path", path, "id", connID, "remote", conn.RemoteAddr())

	// Create client object for scriptling
	clientObj := extlibs.CreateWebSocketClientInstance(wsConn)

	// Run the handler
	s.runWebSocketHandler(route.Handler, clientObj, wsConn, path, connID)

	// Cleanup after handler returns
	extlibs.RuntimeState.Lock()
	delete(extlibs.RuntimeState.WebSocketConnections, connID)
	extlibs.RuntimeState.Unlock()

	wsConn.Close()
	Log.Info("WebSocket disconnected", "path", path, "id", connID)
}

// runWebSocketHandler runs the scriptling WebSocket handler function
func (s *Server) runWebSocketHandler(handlerRef string, clientObj *object.Instance, conn *extlibs.WebSocketServerConn, path, connID string) {
	libName, _, ok := strings.Cut(handlerRef, ".")
	if !ok {
		Log.Error("Invalid WebSocket handler reference", "handler", handlerRef)
		return
	}

	// Create fresh scriptling environment
	p := scriptling.New()
	setup.Scriptling(p, s.config.LibDirs, false, s.config.AllowedPaths, s.config.SecretRegistry, Log)
	s.applyPackLoader(p)

	// Import the library
	if err := p.Import(libName); err != nil {
		Log.Error("Failed to import library", "library", libName, "error", err)
		return
	}

	// Call the handler function
	_, err := p.CallFunction(handlerRef, clientObj)
	if err != nil {
		Log.Error("WebSocket handler error", "path", path, "id", connID, "error", err)
	}
}
