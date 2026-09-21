package server

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paularlott/logger"
	"github.com/paularlott/scriptling/extlibs"
	"github.com/paularlott/scriptling/extlibs/secretprovider"
	"github.com/paularlott/scriptling/scriptling-cli/setup"
)

// buildMinimalMCPServer builds a Server exposing one folder tool over HTTP,
// enough to exercise routing (buildMux) without any middleware/script.
// corsOrigins sets MCPCorsOrigins; pass nil for the same-origin default.
func buildMinimalMCPServer(t *testing.T, corsOrigins []string) *Server {
	t.Helper()
	libDir := t.TempDir()
	setup.Factories([]string{libDir}, nil, nil, secretprovider.NewRegistry(), logger.NewNullLogger(), "", "")
	extlibs.ResetRuntime()

	toolsDir := t.TempDir()
	writeFile(t, filepath.Join(toolsDir, "greet.toml"), []byte("description = \"Greet\"\n"))
	writeFile(t, filepath.Join(toolsDir, "greet.py"), []byte("import scriptling.mcp.tool as tool\ntool.return_string('hi')\n"))

	s := &Server{
		config:     ServerConfig{MCPToolsDir: toolsDir, LibDirs: []string{libDir}, MCPCorsOrigins: corsOrigins},
		mcpHandler: &reloadableMCPHandler{},
	}
	server, err := s.createMCPServer()
	if err != nil {
		t.Fatalf("createMCPServer: %v", err)
	}
	s.mcpHandler.server.Store(server)
	return s
}

func corsPreflight(t *testing.T, tsURL string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodOptions, tsURL+"/mcp", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "content-type")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

// TestMCPRoute_OPTIONS_CORSPreflight_DefaultDeniesForeignOrigin is the
// regression test for the security finding this middleware fixes: by
// default (no MCPCorsOrigins configured), a cross-origin browser preflight
// from a foreign origin must NOT get a matching Access-Control-Allow-Origin
// — the mcpCorsMiddleware wrapper overrides the * that
// mcp.Server.HandleRequest sets unconditionally. Without this, any web page
// could complete the CORS dance and drive tools/call (including
// execute_script) against this unauthenticated server.
func TestMCPRoute_OPTIONS_CORSPreflight_DefaultDeniesForeignOrigin(t *testing.T) {
	s := buildMinimalMCPServer(t, nil)
	ts := httptest.NewServer(s.buildMux())
	defer ts.Close()

	resp := corsPreflight(t, ts.URL)
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusMethodNotAllowed {
		t.Fatalf("OPTIONS /mcp = 405 Method Not Allowed; the preflight never reached the MCP handler")
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Access-Control-Allow-Origin = %q, want empty (foreign origin denied by the same-origin default) — a browser would incorrectly complete the preflight", got)
	}
}

// TestMCPRoute_OPTIONS_CORSPreflight_WildcardOptIn proves the escape hatch
// still works: an operator who explicitly opts in with MCPCorsOrigins =
// ["*"] (e.g. to use examples/mcp-app-host-harness from a different origin)
// gets the same unrestricted preflight this route existed for originally.
func TestMCPRoute_OPTIONS_CORSPreflight_WildcardOptIn(t *testing.T) {
	s := buildMinimalMCPServer(t, []string{"*"})
	ts := httptest.NewServer(s.buildMux())
	defer ts.Close()

	resp := corsPreflight(t, ts.URL)
	defer resp.Body.Close()

	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("Access-Control-Allow-Origin = %q, want \"*\" (explicit opt-in)", got)
	}
	if got := resp.Header.Get("Access-Control-Allow-Methods"); got == "" {
		t.Error("Access-Control-Allow-Methods is empty, want POST etc.")
	}
}

// TestMCPRoute_POST_CrossOrigin_DefaultDeniesForeignOrigin is the actual-
// request counterpart of the preflight test above: a real cross-origin POST
// from a foreign origin is now rejected outright (403) by mcpCorsMiddleware
// itself, rather than allowed to reach the MCP handler and rely solely on a
// stripped Access-Control-Allow-Origin header to stop the browser from
// reading the response. The latter is CORS enforcement, not a server-side
// block — it does nothing against a request that already had side effects
// by the time a browser would refuse to expose the response (or against
// any non-browser caller that simply ignores CORS headers), which is
// exactly the DNS-rebinding-style gap the MCP spec's Origin-check
// requirement targets. See mcpCorsMiddleware's doc comment.
func TestMCPRoute_POST_CrossOrigin_DefaultDeniesForeignOrigin(t *testing.T) {
	s := buildMinimalMCPServer(t, nil)
	ts := httptest.NewServer(s.buildMux())
	defer ts.Close()

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://example.com")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("POST /mcp status = %d, want 403 (foreign origin denied server-side, not just via a stripped CORS header)", resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Access-Control-Allow-Origin = %q, want empty", got)
	}
}

// TestMCPRoute_DELETE_Routed proves DELETE /mcp still reaches
// mcp.Server.HandleRequest (rather than ServeMux's own unrouted 405) without
// relying on the CORS header for that signal, since the header's presence
// is now origin-dependent: this server has no session manager configured,
// so a request that reached HandleRequest gets its specific "Session
// management not enabled" message, not net/http's generic 405 body.
func TestMCPRoute_DELETE_Routed(t *testing.T) {
	s := buildMinimalMCPServer(t, nil)
	ts := httptest.NewServer(s.buildMux())
	defer ts.Close()

	req, err := http.NewRequest(http.MethodDelete, ts.URL+"/mcp", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405 (no session manager configured)", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !bytes.Contains(body, []byte("Session management not enabled")) {
		t.Errorf("body = %q, want HandleRequest's own \"Session management not enabled\" message (DELETE /mcp may not be routed to the MCP handler)", body)
	}
}

// TestMCPRoute_BearerTokenAndCORSCoexist proves a CORS preflight (OPTIONS)
// succeeds without an Authorization header even when a bearer token is
// configured — browsers never attach a custom Authorization header to the
// preflight itself, only to the real follow-up request once the preflight
// allows it, so requiring one on OPTIONS would 401 every cross-origin
// preflight unconditionally and make MCPCorsOrigins unusable together with
// a bearer token. The real POST must still be rejected without the token.
func TestMCPRoute_BearerTokenAndCORSCoexist(t *testing.T) {
	s := buildMinimalMCPServer(t, []string{"*"})
	s.config.BearerToken = "secret"
	s.bearerExpected = "Bearer secret"
	ts := httptest.NewServer(s.buildMux())
	defer ts.Close()

	// Preflight: no Authorization header, must not be 401.
	resp := corsPreflight(t, ts.URL)
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		t.Fatalf("OPTIONS /mcp = 401; a CORS preflight must not require Authorization")
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("Access-Control-Allow-Origin = %q, want \"*\"", got)
	}

	// The real request still enforces the token.
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	postResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer postResp.Body.Close()
	if postResp.StatusCode != http.StatusUnauthorized {
		t.Errorf("POST /mcp without token = %d, want 401 (bearer token must still be enforced on the real request)", postResp.StatusCode)
	}
}

// TestBearerToken_OPTIONSExemptionIsNarrow is the regression test for the
// vulnerability an earlier version of bearerTokenMiddleware introduced: it
// exempted every OPTIONS request from the token check, not just a genuine
// CORS preflight to /mcp. Since OPTIONS still reached the real handler
// afterward (next.ServeHTTP), that let anyone skip the token by sending
// OPTIONS instead of GET — serving a protected static file's full content,
// or running the not-found/fallback handler. Both conditions of the real
// fix are asserted: Access-Control-Request-Method must be present (not
// just the OPTIONS method), and the exemption must not extend to routes
// other than /mcp even when that header is present.
func TestBearerToken_OPTIONSExemptionIsNarrow(t *testing.T) {
	webRoot := t.TempDir()
	const secret = "top secret file contents"
	if err := os.WriteFile(filepath.Join(webRoot, "secret.txt"), []byte(secret), 0o644); err != nil {
		t.Fatal(err)
	}

	s := &Server{config: ServerConfig{WebRoot: webRoot, BearerToken: "tok"}}
	s.bearerExpected = "Bearer tok"
	ts := httptest.NewServer(s.buildMux())
	defer ts.Close()

	doOptions := func(t *testing.T, path string, withPreflightHeader bool) *http.Response {
		t.Helper()
		req, err := http.NewRequest(http.MethodOptions, ts.URL+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Origin", "http://example.com")
		if withPreflightHeader {
			req.Header.Set("Access-Control-Request-Method", "GET")
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}

	t.Run("OPTIONS a protected static file without the preflight header is still blocked", func(t *testing.T) {
		resp := doOptions(t, "/secret.txt", false)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", resp.StatusCode)
		}
	})

	t.Run("OPTIONS a protected static file WITH the preflight header is still blocked — the exemption is /mcp-only", func(t *testing.T) {
		resp := doOptions(t, "/secret.txt", true)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401 (a fake preflight to a non-/mcp route must not bypass auth)", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		if strings.Contains(string(body), secret) {
			t.Fatalf("response leaked the protected file's contents: %q", body)
		}
	})

	t.Run("OPTIONS an undefined path (the not-found/fallback handler) is still blocked", func(t *testing.T) {
		resp := doOptions(t, "/does-not-exist", true)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401 (the fallback handler must not run for an unauthenticated request)", resp.StatusCode)
		}
	})

	t.Run("a real GET still needs the token", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/secret.txt")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", resp.StatusCode)
		}
	})

	t.Run("a real GET with the token succeeds, proving the route itself still works", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, ts.URL+"/secret.txt", nil)
		req.Header.Set("Authorization", "Bearer tok")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK || string(body) != secret {
			t.Fatalf("status = %d, body = %q, want 200 with the file's contents", resp.StatusCode, body)
		}
	})
}
