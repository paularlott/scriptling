package websocket

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/paularlott/scriptling"
)

// A WebSocket connection a script leaves open is closed when the interpreter
// is closed.
func TestWebSocketClosedWithInterpreter(t *testing.T) {
	ended := make(chan struct{})
	up := websocket.Upgrader{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				close(ended)
				return
			}
		}
	}))
	defer ts.Close()

	p := scriptling.New()
	Register(p)
	if _, err := p.Eval(`
import scriptling.net.websocket as ws
c = ws.connect("ws` + ts.URL[4:] + `")
`); err != nil {
		t.Fatal(err)
	}
	select {
	case <-ended:
		t.Fatal("the connection ended while the script still held it")
	case <-time.After(100 * time.Millisecond):
	}
	p.Close()
	select {
	case <-ended:
	case <-time.After(5 * time.Second):
		t.Fatal("the connection is still open after the interpreter closed")
	}
}
