package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"

	"github.com/gorilla/websocket"
	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs"
	"github.com/paularlott/scriptling/object"
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

	// The middleware guards WebSocket upgrades like every other endpoint: a
	// returned response dict rejects the upgrade (e.g. a 401), None lets it
	// proceed. The request is stashed on the context so the handler can read
	// what the middleware put in request.context.
	reqObj := s.createRequestObject(r, nil)
	ctx := extlibs.WithRequestContext(r.Context(), reqObj)
	if s.middleware != "" {
		Log.Trace("Running middleware", "handler", s.middleware, "path", path)
		if resp := s.runHandler(ctx, s.middleware, reqObj); resp != nil {
			s.writeResponse(w, resp)
			return
		}
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

	// Register connection for lifecycle tracking
	extlibs.RuntimeState.Lock()
	extlibs.RuntimeState.WebSocketConnections[connID] = wsConn
	extlibs.RuntimeState.Unlock()

	// Create client object for scriptling
	clientObj := extlibs.CreateWebSocketClientInstance(wsConn)

	// Run the handler. The upgrade request's context values (the stashed
	// request) carry over, but its cancellation does not: the handler outlives
	// the HTTP request once the connection is hijacked.
	handlerCtx := context.WithoutCancel(ctx)
	s.runWebSocketHandler(handlerCtx, route.Handler, clientObj, wsConn, path, connID)

	// Cleanup after handler returns
	extlibs.RuntimeState.Lock()
	delete(extlibs.RuntimeState.WebSocketConnections, connID)
	extlibs.RuntimeState.Unlock()

	wsConn.Close()
	Log.Info("WebSocket disconnected", "path", path, "id", connID)
}

// runWebSocketHandler runs the scriptling WebSocket handler function
func (s *Server) runWebSocketHandler(ctx context.Context, handlerRef string, clientObj *object.Instance, conn *extlibs.WebSocketServerConn, path, connID string) {
	libName, _, ok := splitHandlerRef(handlerRef)
	if !ok {
		Log.Error("Invalid WebSocket handler reference", "handler", handlerRef)
		return
	}

	Log.Trace("WebSocket handler starting", "id", connID, "path", path, "handler", handlerRef)

	// Create fresh scriptling environment
	p := scriptling.New()
	s.setupScriptling(p)
	s.applyPackLoader(p)

	// Import the library
	if err := p.Import(libName); err != nil {
		Log.Error("Failed to import library", "library", libName, "error", err)
		return
	}

	// Call the handler function
	_, err := p.CallFunctionWithContext(ctx, handlerRef, clientObj)
	if err != nil {
		Log.Error("WebSocket handler error", "path", path, "id", connID, "error", err)
	} else {
		Log.Trace("WebSocket handler completed", "id", connID, "path", path)
	}
}
