package plugin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPClientCall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		var req rpcRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		if req.Method != "echo" {
			t.Errorf("expected method echo, got %s", req.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: mustHTTPRawJSON(t, `{"ok":true}`)})
	}))
	defer srv.Close()

	client, err := newHTTPClient(context.Background(), srv.URL, false, false, nil)
	if err != nil {
		t.Fatalf("newHTTPClient: %v", err)
	}
	defer client.doneClose.Do(func() { close(client.done) })

	var result map[string]bool
	if err := client.Call(context.Background(), "echo", map[string]any{"hello": "world"}, &result); err != nil {
		t.Fatalf("Call: %v", err)
	}
	if !result["ok"] {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestHTTPClientHeaders(t *testing.T) {
	const token = "Bearer scriptling-test"
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if got := r.Header.Get("Authorization"); got != token {
			t.Errorf("expected Authorization header %q, got %q", token, got)
		}
		if got := r.Header.Get("X-Scriptling-Test"); got != "yes" {
			t.Errorf("expected X-Scriptling-Test header yes, got %q", got)
		}
		var req rpcRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: mustHTTPRawJSON(t, `"ok"`)})
	}))
	defer srv.Close()

	client, err := newHTTPClient(context.Background(), srv.URL, false, false, nil, map[string]string{
		"Authorization":     token,
		"X-Scriptling-Test": "yes",
		"X-Original-Header": "original",
		"X-Another-Header":  "another",
	})
	if err != nil {
		t.Fatalf("newHTTPClient: %v", err)
	}
	defer client.doneClose.Do(func() { close(client.done) })

	var result string
	if err := client.Call(context.Background(), "ping", nil, &result); err != nil {
		t.Fatalf("Call: %v", err)
	}
	if result != "ok" {
		t.Fatalf("expected ok, got %q", result)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestHTTPClientBatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqs []rpcRequest
		if err := json.NewDecoder(r.Body).Decode(&reqs); err != nil {
			t.Errorf("decode batch: %v", err)
			return
		}
		if len(reqs) != 2 {
			t.Errorf("expected 2 requests, got %d", len(reqs))
		}
		responses := make([]rpcResponse, len(reqs))
		for i, req := range reqs {
			raw, err := json.Marshal(map[string]string{"method": req.Method})
			if err != nil {
				t.Errorf("marshal response: %v", err)
				return
			}
			responses[len(reqs)-1-i] = rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: raw}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(responses)
	}))
	defer srv.Close()

	client, err := newHTTPClient(context.Background(), srv.URL, false, false, nil)
	if err != nil {
		t.Fatalf("newHTTPClient: %v", err)
	}
	defer client.doneClose.Do(func() { close(client.done) })

	results, err := client.Batch(context.Background(), []batchRequest{
		{Method: "first", Params: map[string]any{"n": 1}},
		{Method: "second", Params: map[string]any{"n": 2}},
	})
	if err != nil {
		t.Fatalf("Batch: %v", err)
	}
	var first, second map[string]string
	if err := json.Unmarshal(results[0], &first); err != nil {
		t.Fatalf("unmarshal first: %v", err)
	}
	if err := json.Unmarshal(results[1], &second); err != nil {
		t.Fatalf("unmarshal second: %v", err)
	}
	if first["method"] != "first" || second["method"] != "second" {
		t.Fatalf("batch results not matched by id: first=%#v second=%#v", first, second)
	}
}

func TestHTTPClientInsecureSkipTLS(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: mustHTTPRawJSON(t, `"ok"`)})
	}))
	defer srv.Close()

	secureClient, err := newHTTPClient(context.Background(), srv.URL, false, false, nil)
	if err != nil {
		t.Fatalf("new secure HTTP client: %v", err)
	}
	var ignored string
	if err := secureClient.Call(context.Background(), "ping", nil, &ignored); err == nil {
		t.Fatal("expected TLS verification error without insecure_skip_tls")
	}
	secureClient.doneClose.Do(func() { close(secureClient.done) })

	insecureClient, err := newHTTPClient(context.Background(), srv.URL, false, false, newSharedHTTPTransport(true))
	if err != nil {
		t.Fatalf("new insecure HTTP client: %v", err)
	}
	defer insecureClient.doneClose.Do(func() { close(insecureClient.done) })
	var result string
	if err := insecureClient.Call(context.Background(), "ping", nil, &result); err != nil {
		t.Fatalf("Call with insecure skip TLS: %v", err)
	}
	if result != "ok" {
		t.Fatalf("expected ok, got %q", result)
	}
}

// TestHTTPClientSharedTransport verifies that a Manager's shared transport is
// used when creating HTTP clients, enabling connection pooling across calls.
func TestHTTPClientSharedTransport(t *testing.T) {
	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		var req rpcRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: mustHTTPRawJSON(t, `"pooled"`)})
	}))
	defer srv.Close()

	shared := newSharedHTTPTransport(false)

	// Both clients share the same transport — connections are pooled.
	c1, err := newHTTPClient(context.Background(), srv.URL, false, false, shared)
	if err != nil {
		t.Fatalf("c1: %v", err)
	}
	defer c1.doneClose.Do(func() { close(c1.done) })

	c2, err := newHTTPClient(context.Background(), srv.URL, false, false, shared)
	if err != nil {
		t.Fatalf("c2: %v", err)
	}
	defer c2.doneClose.Do(func() { close(c2.done) })

	for _, c := range []*Client{c1, c2} {
		var result string
		if err := c.Call(context.Background(), "ping", nil, &result); err != nil {
			t.Fatalf("Call: %v", err)
		}
		if result != "pooled" {
			t.Fatalf("expected pooled, got %q", result)
		}
	}
	if requests != 2 {
		t.Fatalf("expected 2 requests, got %d", requests)
	}
}

func mustHTTPRawJSON(t *testing.T, s string) json.RawMessage {
	t.Helper()
	var raw json.RawMessage
	if err := json.Unmarshal([]byte(s), &raw); err != nil {
		t.Fatalf("invalid raw json: %v", err)
	}
	return raw
}
