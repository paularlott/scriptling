package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"testing"
	"time"
)

// Stop shuts the MCP server down before the HTTP server, so an open
// subscriptions/listen stream ends at once rather than holding the HTTP
// shutdown until its deadline.
func TestStopEndsOpenMCPStreams(t *testing.T) {
	s := buildMinimalMCPServer(t, nil)
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	l.Close()
	s.config.Address = addr
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}

	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "subscriptions/listen",
		"params": map[string]any{
			"notifications": map[string]any{"toolsListChanged": true},
			"_meta": map[string]any{
				"io.modelcontextprotocol/protocolVersion":    "2026-07-28",
				"io.modelcontextprotocol/clientCapabilities": map[string]any{},
			},
		},
	})
	req, _ := http.NewRequest(http.MethodPost, "http://"+addr+"/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", "2026-07-28")
	req.Header.Set("Mcp-Method", "subscriptions/listen")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("listen: %d", resp.StatusCode)
	}
	ended := make(chan struct{})
	go func() {
		sc := bufio.NewScanner(resp.Body)
		for sc.Scan() {
		}
		close(ended)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	start := time.Now()
	if err := s.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v (after %s)", err, time.Since(start))
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("Stop took %s: the open MCP stream held the shutdown", d)
	}
	select {
	case <-ended:
	case <-time.After(2 * time.Second):
		t.Fatal("the listen stream is still open after Stop")
	}
}
