package http

import (
	nethttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/runtime"
)

func TestWebSocketRouteRegistration(t *testing.T) {
	runtime.ResetRuntime()
	p := scriptling.New()
	registerAll(p, nil)

	script := `
import scriptling.runtime as runtime

runtime.http.websocket("/ws", "handlers.ws_handler")
`

	_, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}

	runtime.RuntimeState.RLock()
	defer runtime.RuntimeState.RUnlock()

	if len(State.WebSocketRoutes) != 1 {
		t.Errorf("Expected 1 WebSocket route, got %d", len(State.WebSocketRoutes))
	}

	route, ok := State.WebSocketRoutes["/ws"]
	if !ok {
		t.Error("WebSocket route /ws not found")
	} else if route.Handler != "handlers.ws_handler" {
		t.Errorf("Expected handler 'handlers.ws_handler', got '%s'", route.Handler)
	}
}

func TestWebSocketServerConn(t *testing.T) {
	// Test the WebSocketServerConn wrapper
	server := httptest.NewServer(nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
		upgrader := websocket.Upgrader{
			CheckOrigin: func(r *nethttp.Request) bool { return true },
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Wrap the connection
		wsConn := NewWebSocketServerConn(conn, "test_conn_1")

		// Test ID
		if wsConn.ID() != "test_conn_1" {
			t.Errorf("Expected ID 'test_conn_1', got '%s'", wsConn.ID())
		}

		// Test IsConnected
		if !wsConn.IsConnected() {
			t.Error("Expected to be connected")
		}

		// Read and echo
		msgType, data, err := wsConn.ReadWithTimeout(5 * time.Second)
		if err != nil {
			t.Logf("Read error: %v", err)
			return
		}
		if data == nil {
			t.Log("Read timeout")
			return
		}

		wsConn.WriteMessage(msgType, data)
		wsConn.Close()

		if wsConn.IsConnected() {
			t.Error("Expected to be disconnected after close")
		}
	}))
	defer server.Close()

	// Connect as client
	wsURL := "ws://" + strings.TrimPrefix(server.URL, "http://")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Dial error: %v", err)
	}
	defer conn.Close()

	// Send a message
	err = conn.WriteMessage(websocket.TextMessage, []byte("test"))
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}

	// Read the echo
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("Read error: %v", err)
	}

	if string(msg) != "test" {
		t.Errorf("Expected 'test', got '%s'", string(msg))
	}
}

func TestWebSocketResetRuntime(t *testing.T) {
	runtime.ResetRuntime()

	p := scriptling.New()
	registerAll(p, nil)

	script := `
import scriptling.runtime as runtime

runtime.http.websocket("/ws1", "handlers.ws1")
runtime.http.websocket("/ws2", "handlers.ws2")
`

	_, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}

	runtime.RuntimeState.RLock()
	if len(State.WebSocketRoutes) != 2 {
		t.Errorf("Expected 2 WebSocket routes, got %d", len(State.WebSocketRoutes))
	}
	runtime.RuntimeState.RUnlock()

	// Reset and verify cleanup
	runtime.ResetRuntime()

	runtime.RuntimeState.RLock()
	if len(State.WebSocketRoutes) != 0 {
		t.Errorf("Expected 0 WebSocket routes after reset, got %d", len(State.WebSocketRoutes))
	}
	if len(State.WebSocketConnections) != 0 {
		t.Errorf("Expected 0 WebSocket connections after reset, got %d", len(State.WebSocketConnections))
	}
	runtime.RuntimeState.RUnlock()
}
