package plugin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/paularlott/scriptling/object"
)

// The HTTP plugin transport is unidirectional: unlike the stdio peer it has no
// reverse channel, so host callbacks cannot travel to the plugin. The client
// enforces that in httpCall, which rejects any call carrying callbacks.
//
// These tests characterise where that enforcement happens today: at CALL time,
// not at LOAD time. A plugin whose function needs a callback loads cleanly over
// HTTP and only fails when such a function is actually invoked — and the error
// is a transport-level message, not "this plugin can't run over HTTP". The
// handshake exchanges capability lists but nothing advertises or checks a
// "requires callbacks" need, so the mismatch is a latent, call-time surprise.
//
// If load-time capability negotiation is added, TestHTTPCallbackRejectionIs*
// should be updated to assert a clean load-time refusal instead.

// httpEchoServer answers scriptling.handshake and function.call so an HTTP
// client can be constructed and driven. The handshake advertises no
// capabilities, mirroring the PHP example.
func httpEchoServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case "scriptling.handshake":
			result := handshakeResult{
				Protocol:  ProtocolVersion,
				Transport: "json",
				Library: libraryInfo{
					Name:        "cbdemo",
					Version:     "1.0.0",
					Description: "callback gap test plugin",
				},
				Capabilities: []string{}, // advertises nothing about callbacks
				Schema:       Schema{Functions: []FunctionSchema{{Name: "on_event"}}},
			}
			raw, _ := json.Marshal(result)
			_ = json.NewEncoder(w).Encode(rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: raw})
		case "function.call":
			_ = json.NewEncoder(w).Encode(rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: mustHTTPRawJSON(t, `{"type":"null"}`)})
		default:
			_ = json.NewEncoder(w).Encode(rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: mustHTTPRawJSON(t, "null")})
		}
	}))
}

// TestHTTPPluginLoadsDespiteCallbackFunction shows the gap's first half: a
// plugin declaring a callback-shaped function handshakes and loads over HTTP
// with no error and no warning, because load does not inspect callbacks.
func TestHTTPPluginLoadsDespiteCallbackFunction(t *testing.T) {
	srv := httpEchoServer(t)
	defer srv.Close()

	m := NewManager(nil)
	defer m.Close()

	if err := m.LoadPlugins(context.Background(), []PluginSpec{{Path: srv.URL}}); err != nil {
		t.Fatalf("LoadPlugins over http: %v", err)
	}
	if _, ok := m.Get("plugin.cbdemo"); !ok {
		t.Fatal("expected plugin.cbdemo to be registered")
	}
	if w := m.Warnings(); len(w) != 0 {
		t.Fatalf("expected no load warnings, got %#v", w)
	}
	// This is the crux: the plugin is fully loaded even though one of its
	// functions is callback-shaped, and nothing told the operator that a
	// callback call will later fail. If load-time negotiation is added, this
	// LoadPlugins should fail (or warn) instead.
}

// TestHTTPCallWithoutCallbacksSucceeds is the control: an ordinary HTTP call
// (no callback) goes through, so the rejection below is specific to callbacks.
func TestHTTPCallWithoutCallbacksSucceeds(t *testing.T) {
	srv := httpEchoServer(t)
	defer srv.Close()

	client, err := newHTTPClient(context.Background(), srv.URL, false, true, nil, nil)
	if err != nil {
		t.Fatalf("newHTTPClient: %v", err)
	}
	defer client.doneClose.Do(func() { close(client.done) })

	if _, err := client.CallFunction(context.Background(), "on_event", nil, nil); err != nil {
		t.Fatalf("plain HTTP call should succeed: %v", err)
	}
}

// TestHTTPCallWithCallbacksRejectedAtCallTime pins the gap's second half: the
// same function invoked with a callback argument is rejected only when called,
// with a transport-level message. This is the surprise a load-time capability
// check would prevent.
func TestHTTPCallWithCallbacksRejectedAtCallTime(t *testing.T) {
	srv := httpEchoServer(t)
	defer srv.Close()

	client, err := newHTTPClient(context.Background(), srv.URL, false, true, nil, nil)
	if err != nil {
		t.Fatalf("newHTTPClient: %v", err)
	}
	defer client.doneClose.Do(func() { close(client.done) })

	// A call that carries a host callback (a script passing a function the
	// plugin would invoke). Over stdio this registers a callback owner; over
	// HTTP it must be refused.
	callbacks := newCallbackSet()
	callbacks.add(&object.Builtin{Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
		return &object.Null{}
	}})

	_, err = client.CallFunctionWithCallbacks(context.Background(), "on_event", nil, nil, callbacks)
	if err == nil {
		t.Fatal("expected a callback-over-http rejection, got nil")
	}
	if !strings.Contains(err.Error(), "callbacks are not supported over http") {
		t.Fatalf("expected the transport-level callback rejection, got: %v", err)
	}
	// The error is transport-level and arrives at call time. There is no
	// load-time signal (see TestHTTPPluginLoadsDespiteCallbackFunction), which
	// is the reviewed gap: a clean load-time refusal would be friendlier.
}
