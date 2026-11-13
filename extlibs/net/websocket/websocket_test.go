package websocket

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/object"
)

func TestWebSocketClientConnect(t *testing.T) {
	// Create a test WebSocket server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Logf("Upgrade error: %v", err)
			return
		}
		defer conn.Close()

		// Echo messages
		for {
			msgType, msg, err := conn.ReadMessage()
			if err != nil {
				break
			}
			conn.WriteMessage(msgType, msg)
		}
	}))
	defer server.Close()

	// Convert http:// to ws://
	wsURL := "ws://" + strings.TrimPrefix(server.URL, "http://")

	// Test with scriptling
	p := scriptling.New()
	Register(p)

	script := `
import scriptling.net.websocket as ws

conn = ws.connect("` + wsURL + `", timeout=5)
connected = conn.connected()
conn.close()
connected_after = conn.connected()
[connected, connected_after]
`

	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}

	list, ok := result.(*object.List)
	if !ok {
		t.Fatalf("Expected list result, got %T", result)
	}

	if len(list.Elements) != 2 {
		t.Fatalf("Expected 2 elements, got %d", len(list.Elements))
	}

	// First should be true (connected)
	if b, _ := list.Elements[0].AsBool(); !b {
		t.Error("Expected connected to be true")
	}

	// Second should be false (disconnected after close)
	if b, _ := list.Elements[1].AsBool(); b {
		t.Error("Expected connected to be false after close")
	}
}

func TestWebSocketClientEcho(t *testing.T) {
	// Create a test WebSocket server that echoes
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		for {
			msgType, msg, err := conn.ReadMessage()
			if err != nil {
				break
			}
			conn.WriteMessage(msgType, msg)
		}
	}))
	defer server.Close()

	wsURL := "ws://" + strings.TrimPrefix(server.URL, "http://")

	p := scriptling.New()
	Register(p)

	script := `
import scriptling.net.websocket as ws

conn = ws.connect("` + wsURL + `", timeout=5)
conn.send("Hello, WebSocket!")
msg = conn.receive(timeout=5)
conn.close()
msg
`

	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}

	str, ok := result.(*object.String)
	if !ok {
		t.Fatalf("Expected string result, got %T", result)
	}

	if str.StringValue() != "Hello, WebSocket!" {
		t.Errorf("Expected 'Hello, WebSocket!', got '%s'", str.StringValue())
	}
}

func TestWebSocketClientJSON(t *testing.T) {
	// Create a test WebSocket server that echoes
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		for {
			msgType, msg, err := conn.ReadMessage()
			if err != nil {
				break
			}
			conn.WriteMessage(msgType, msg)
		}
	}))
	defer server.Close()

	wsURL := "ws://" + strings.TrimPrefix(server.URL, "http://")

	p := scriptling.New()
	Register(p)

	script := `
import scriptling.net.websocket as ws

conn = ws.connect("` + wsURL + `", timeout=5)
# Send a dict - should be JSON encoded
conn.send({"type": "test", "value": 42})
msg = conn.receive(timeout=5)
conn.close()
msg
`

	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}

	str, ok := result.(*object.String)
	if !ok {
		t.Fatalf("Expected string result, got %T", result)
	}

	// Check it's valid JSON with the expected content
	if !strings.Contains(str.StringValue(), `"type"`) || !strings.Contains(str.StringValue(), `"test"`) {
		t.Errorf("Expected JSON with type:test, got '%s'", str.StringValue())
	}
}

func TestWebSocketClientTimeout(t *testing.T) {
	// Create a test WebSocket server that doesn't send anything
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Just wait, don't send anything
		time.Sleep(5 * time.Second)
	}))
	defer server.Close()

	wsURL := "ws://" + strings.TrimPrefix(server.URL, "http://")

	p := scriptling.New()
	Register(p)

	script := `
import scriptling.net.websocket as ws

conn = ws.connect("` + wsURL + `", timeout=5)
msg = conn.receive(timeout=1)  # 1 second timeout
still_connected = conn.connected()
conn.close()
[msg, still_connected]
`

	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}

	list, ok := result.(*object.List)
	if !ok {
		t.Fatalf("Expected list result, got %T", result)
	}

	// First should be None (timeout)
	if _, ok := list.Elements[0].(*object.Null); !ok {
		t.Errorf("Expected None on timeout, got %T", list.Elements[0])
	}

	// Second should be true (still connected after timeout)
	if b, _ := list.Elements[1].AsBool(); !b {
		t.Error("Expected to still be connected after timeout")
	}
}

func TestWebSocketIsTextIsBinary(t *testing.T) {
	p := scriptling.New()
	Register(p)

	// Test is_text with string
	script := `
import scriptling.net.websocket as ws

text_msg = "hello"
binary_msg = [1, 2, 3, 255]

[ws.is_text(text_msg), ws.is_binary(text_msg), ws.is_text(binary_msg), ws.is_binary(binary_msg)]
`

	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}

	list, ok := result.(*object.List)
	if !ok {
		t.Fatalf("Expected list result, got %T", result)
	}

	if len(list.Elements) != 4 {
		t.Fatalf("Expected 4 elements, got %d", len(list.Elements))
	}

	// text_msg is text: True
	if b, _ := list.Elements[0].AsBool(); !b {
		t.Error("Expected is_text('hello') to be True")
	}

	// text_msg is binary: False
	if b, _ := list.Elements[1].AsBool(); b {
		t.Error("Expected is_binary('hello') to be False")
	}

	// binary_msg is text: False
	if b, _ := list.Elements[2].AsBool(); b {
		t.Error("Expected is_text([1,2,3,255]) to be False")
	}

	// binary_msg is binary: True
	if b, _ := list.Elements[3].AsBool(); !b {
		t.Error("Expected is_binary([1,2,3,255]) to be True")
	}
}
