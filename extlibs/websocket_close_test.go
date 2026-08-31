package extlibs

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// TestWebSocketCloseUnblocksPendingRead pins the wrapper's locking contract:
// a read with no timeout may block forever, and Close must still go through
// and cut the read short. The sole mutex used to be held across ReadMessage,
// so a quiet connection deadlocked Close (and writes) indefinitely.
func TestWebSocketCloseUnblocksPendingRead(t *testing.T) {
	connCh := make(chan *WebSocketServerConn, 1)
	up := websocket.Upgrader{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := up.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		connCh <- NewWebSocketServerConn(conn, "test")
		// Hold the connection open; nothing more to do server-side.
		select {}
	}))
	defer ts.Close()

	dialConn, _, err := websocket.DefaultDialer.Dial("ws"+ts.URL[4:], nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer dialConn.Close()

	var wrapped *WebSocketServerConn
	select {
	case wrapped = <-connCh:
	case <-time.After(5 * time.Second):
		t.Fatal("server never wrapped the connection")
	}

	readDone := make(chan error, 1)
	go func() {
		// No timeout: blocks until Close lands.
		_, _, err := wrapped.ReadWithTimeout(0)
		readDone <- err
	}()

	// Give the reader time to park inside ReadMessage, then close.
	time.Sleep(100 * time.Millisecond)
	closed := make(chan error, 1)
	go func() { closed <- wrapped.Close() }()
	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("Close blocked behind a pending read")
	}
	select {
	case <-readDone:
	case <-time.After(2 * time.Second):
		t.Fatal("pending read did not return after Close")
	}
}
