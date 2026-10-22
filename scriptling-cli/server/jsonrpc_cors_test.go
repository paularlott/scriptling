package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/paularlott/scriptling/object"
	scriptlingplugin "github.com/paularlott/scriptling/plugin"
)

// The handler and the script middleware both call probe.hold(), so
// probe.calls counts every script that ran for a request.
const corsHandlerSrc = `import probe

def work(params):
    return probe.hold()

def mw(request):
    probe.hold()
    return None
`

const workCall = `{"jsonrpc":"2.0","id":1,"method":"work"}`

func corsJSONRPCServer(t *testing.T, origins []string) (*httptest.Server, *Server, *handlerProbe) {
	t.Helper()
	probe := &handlerProbe{}
	s, _ := jsonrpcTestServer(t, corsHandlerSrc, map[string]string{"work": "rpcmod.work"}, nil)
	s.config.ExtraLibs = probe.lib
	s.config.JSONRPC = true
	s.config.JSONRPCCorsOrigins = origins
	s.middleware = "rpcmod.mw"
	s.hosts = newHostPolicy("127.0.0.1:0", nil)
	ts := httptest.NewServer(s.buildMux())
	t.Cleanup(ts.Close)
	return ts, s, probe
}

func rpcRequest(t *testing.T, method, url, origin, contentType, body string, hdr ...string) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequest(method, url+"/json-rpc", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for i := 0; i+1 < len(hdr); i += 2 {
		if hdr[i] == "Host" {
			req.Host = hdr[i+1]
		} else {
			req.Header.Set(hdr[i], hdr[i+1])
		}
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	return resp, string(out)
}

// Any web page could call a local server's JSON-RPC methods: a cross-site
// text/plain or form POST needs no preflight, and /json-rpc had no origin
// check, so the handler ran even though the page could not read the reply
// (CWE-352, strix 4250 vuln-0013). A foreign origin is now refused before
// the script middleware or handler runs; same-origin pages and clients that
// send no Origin are unaffected.
func TestJSONRPCCorsDefaultSameOrigin(t *testing.T) {
	ts, _, probe := corsJSONRPCServer(t, nil)

	for _, ct := range []string{"text/plain", "application/x-www-form-urlencoded", "application/json"} {
		resp, body := rpcRequest(t, http.MethodPost, ts.URL, "http://evil.example", ct, workCall)
		if resp.StatusCode != http.StatusForbidden || resp.Header.Get("Access-Control-Allow-Origin") != "" {
			t.Errorf("foreign origin, %s: %d %q ACAO=%q", ct, resp.StatusCode, body, resp.Header.Get("Access-Control-Allow-Origin"))
		}
	}
	resp, _ := rpcRequest(t, http.MethodOptions, ts.URL, "http://evil.example", "", "", "Access-Control-Request-Method", "POST")
	if resp.StatusCode != http.StatusForbidden || resp.Header.Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("foreign preflight: %d ACAO=%q", resp.StatusCode, resp.Header.Get("Access-Control-Allow-Origin"))
	}
	if n := probe.calls.Load(); n != 0 {
		t.Fatalf("%d scripts ran for refused requests", n)
	}

	// No Origin (curl, servers, plugin clients), with or without a JSON
	// content type, is served as before.
	for _, ct := range []string{"application/json", ""} {
		if resp, body := rpcRequest(t, http.MethodPost, ts.URL, "", ct, workCall); resp.StatusCode != 200 || !strings.Contains(body, `"result":"ok"`) {
			t.Errorf("no origin, content type %q: %d %s", ct, resp.StatusCode, body)
		}
	}
	// A page served by this server is same-origin.
	resp, body := rpcRequest(t, http.MethodPost, ts.URL, ts.URL, "application/json", workCall)
	if resp.StatusCode != 200 || !strings.Contains(body, `"result":"ok"`) || resp.Header.Get("Access-Control-Allow-Origin") != ts.URL {
		t.Errorf("same origin: %d %s ACAO=%q", resp.StatusCode, body, resp.Header.Get("Access-Control-Allow-Origin"))
	}
	// A rebound name is refused even though Origin matches Host.
	resp, _ = rpcRequest(t, http.MethodPost, ts.URL, "http://rebind.evil:80", "text/plain", workCall, "Host", "rebind.evil:80")
	if resp.StatusCode != http.StatusMisdirectedRequest {
		t.Errorf("rebound host: %d, want 421", resp.StatusCode)
	}
	// Three requests were served, each running the middleware and handler;
	// the rebound one ran nothing.
	if n := probe.calls.Load(); n != 6 {
		t.Fatalf("%d script runs, want 6", n)
	}
}

// An allowlisted origin can use /json-rpc from a browser: its preflight is
// answered and its responses carry Access-Control-Allow-Origin. Other
// origins are still refused.
func TestJSONRPCCorsAllowlist(t *testing.T) {
	ts, _, probe := corsJSONRPCServer(t, []string{"http://app.example"})

	resp, _ := rpcRequest(t, http.MethodOptions, ts.URL, "http://app.example", "", "",
		"Access-Control-Request-Method", "POST", "Access-Control-Request-Headers", "content-type, x-api-key")
	h := resp.Header
	if resp.StatusCode != http.StatusNoContent || h.Get("Access-Control-Allow-Origin") != "http://app.example" ||
		!strings.Contains(h.Get("Access-Control-Allow-Methods"), "POST") || h.Get("Access-Control-Allow-Headers") != "content-type, x-api-key" ||
		!strings.Contains(h.Get("Vary"), "Origin") {
		t.Errorf("allowed preflight: %d %v", resp.StatusCode, h)
	}
	if n := probe.calls.Load(); n != 0 {
		t.Fatalf("the preflight ran %d scripts", n)
	}

	resp, body := rpcRequest(t, http.MethodPost, ts.URL, "http://app.example", "application/json", workCall)
	if resp.StatusCode != 200 || !strings.Contains(body, `"result":"ok"`) || resp.Header.Get("Access-Control-Allow-Origin") != "http://app.example" {
		t.Errorf("allowed POST: %d %s ACAO=%q", resp.StatusCode, body, resp.Header.Get("Access-Control-Allow-Origin"))
	}

	before := probe.calls.Load()
	if resp, _ := rpcRequest(t, http.MethodPost, ts.URL, "http://other.example", "text/plain", workCall); resp.StatusCode != http.StatusForbidden {
		t.Errorf("unlisted origin: %d", resp.StatusCode)
	}
	if probe.calls.Load() != before {
		t.Fatal("a script ran for an unlisted origin")
	}
}

func TestJSONRPCCorsWildcard(t *testing.T) {
	ts, _, _ := corsJSONRPCServer(t, []string{"*"})
	resp, body := rpcRequest(t, http.MethodPost, ts.URL, "http://anything.example", "application/json", workCall)
	if resp.StatusCode != 200 || !strings.Contains(body, `"result":"ok"`) || resp.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("wildcard: %d %s ACAO=%q", resp.StatusCode, body, resp.Header.Get("Access-Control-Allow-Origin"))
	}
}

// With a bearer token, an allowed origin's preflight is answered without the
// token (browsers never send it on a preflight), while every real request
// and any OPTIONS that is not a preflight still needs it. A preflight-shaped
// OPTIONS without an Origin runs nothing.
func TestJSONRPCCorsWithBearerToken(t *testing.T) {
	ts, s, probe := corsJSONRPCServer(t, []string{"http://app.example"})
	ts.Close()
	s.config.BearerToken = "tok"
	s.bearerExpected = "Bearer tok"
	ts = httptest.NewServer(s.buildMux())
	defer ts.Close()

	if resp, _ := rpcRequest(t, http.MethodOptions, ts.URL, "http://app.example", "", "", "Access-Control-Request-Method", "POST"); resp.StatusCode != http.StatusNoContent {
		t.Errorf("preflight without token: %d", resp.StatusCode)
	}
	if resp, _ := rpcRequest(t, http.MethodOptions, ts.URL, "http://app.example", "", ""); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("non-preflight OPTIONS without token: %d", resp.StatusCode)
	}
	if resp, _ := rpcRequest(t, http.MethodOptions, ts.URL, "", "", "", "Access-Control-Request-Method", "POST"); resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("preflight-shaped OPTIONS without Origin: %d", resp.StatusCode)
	}
	if resp, _ := rpcRequest(t, http.MethodPost, ts.URL, "http://app.example", "application/json", workCall); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("POST without token: %d", resp.StatusCode)
	}
	if n := probe.calls.Load(); n != 0 {
		t.Fatalf("%d scripts ran without the token", n)
	}
	if resp, body := rpcRequest(t, http.MethodPost, ts.URL, "http://app.example", "application/json", workCall, "Authorization", "Bearer tok"); resp.StatusCode != 200 || !strings.Contains(body, `"result":"ok"`) {
		t.Errorf("POST with token: %d %s", resp.StatusCode, body)
	}
}

// A plugin served over HTTP shares the /json-rpc route and its origin policy.
func TestJSONRPCCorsPluginServer(t *testing.T) {
	var calls int
	ps := scriptlingplugin.NewServer("p", "1.0.0", "")
	ps.RegisterBuiltin("hold", func(_ context.Context, _ object.Kwargs, _ ...object.Object) object.Object {
		calls++
		return object.NewString("ok")
	})
	s := &Server{config: ServerConfig{JSONRPC: true}, pluginServer: ps, hosts: newHostPolicy("127.0.0.1:0", nil)}
	ts := httptest.NewServer(s.buildMux())
	defer ts.Close()

	call := `{"jsonrpc":"2.0","id":1,"method":"function.call","params":{"name":"hold"}}`
	if resp, _ := rpcRequest(t, http.MethodPost, ts.URL, "http://evil.example", "text/plain", call); resp.StatusCode != http.StatusForbidden || calls != 0 {
		t.Errorf("foreign origin: %d, %d calls", resp.StatusCode, calls)
	}
	if resp, body := rpcRequest(t, http.MethodPost, ts.URL, "", "application/json", call); resp.StatusCode != 200 || calls != 1 {
		t.Errorf("no origin: %d %s, %d calls", resp.StatusCode, body, calls)
	}
}
