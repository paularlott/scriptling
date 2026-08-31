package plugin

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/paularlott/scriptling/object"
)

// TestHTTPPluginConnectionsArePooled pins the transport contract for HTTP
// plugins: every client of a manager shares one pooled *http.Transport, so
// sequential calls reuse a single keep-alive connection instead of opening a
// TCP connection (and, for https, a TLS handshake) per JSON-RPC request. A
// regression that builds a fresh transport or client per call trips this.
func TestHTTPPluginConnectionsArePooled(t *testing.T) {
	echo := object.NewFunctionBuilder()
	echo.Function(func(v any) any { return v })
	server := NewServer("poolprobe", "1.0.0", "connection pooling probe").RegisterFunc("echo", echo)

	ts := httptest.NewUnstartedServer(server)
	var connections int64
	ts.Config.ConnState = func(c net.Conn, state http.ConnState) {
		if state == http.StateNew {
			atomic.AddInt64(&connections, 1)
		}
	}
	ts.Start()
	t.Cleanup(ts.Close)

	m := NewManager(nil)
	defer m.Close()
	if err := m.LoadPlugins(context.Background(), []PluginSpec{{Path: ts.URL}}); err != nil {
		t.Fatalf("LoadPlugins: %v", err)
	}
	client, ok := m.Get("plugin.poolprobe")
	if !ok {
		t.Fatal("plugin.poolprobe not registered")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	for i := 0; i < 10; i++ {
		result, err := client.CallFunction(ctx, "echo", []Value{{Type: valueInt, Value: int64(i)}}, nil)
		if err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
		if result.Type != valueInt || fmt.Sprint(result.Value) != fmt.Sprint(i) {
			t.Fatalf("call %d: unexpected result %#v", i, result)
		}
	}

	if got := atomic.LoadInt64(&connections); got != 1 {
		t.Fatalf("10 sequential calls used %d TCP connections, want 1 (keep-alive pooling broken)", got)
	}

	// The shared transport outlives individual plugins: loading a second HTTP
	// plugin through the same manager reuses the manager's transport (and its
	// pool) rather than building another.
	before := atomic.LoadInt64(&connections)
	second := NewServer("poolprobe2", "1.0.0", "second probe").RegisterFunc("echo", echo)
	ts2 := httptest.NewServer(second)
	t.Cleanup(ts2.Close)
	if err := m.LoadPlugins(context.Background(), []PluginSpec{{Path: ts2.URL}}); err != nil {
		t.Fatalf("LoadPlugins second: %v", err)
	}
	if _, ok := m.Get("plugin.poolprobe2"); !ok {
		t.Fatal("plugin.poolprobe2 not registered")
	}
	if got := atomic.LoadInt64(&connections); got < before {
		t.Fatal("connection count went down?!")
	}
}
