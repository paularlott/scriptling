package server

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestHostPolicy(t *testing.T) {
	localHostname = func() (string, error) { return "WEB01.corp.example", nil }
	t.Cleanup(func() { localHostname = os.Hostname })
	for _, tc := range []struct {
		listen  string
		allowed []string
		host    string
		want    bool
	}{
		// Bound to loopback: loopback names only.
		{"127.0.0.1:8000", nil, "127.0.0.1:8000", true},
		{"127.0.0.1:8000", nil, "localhost:8000", true},
		{"127.0.0.1:8000", nil, "LOCALHOST.:8000", true},
		{"127.0.0.1:8000", nil, "app.localhost:8000", true},
		{"127.0.0.1:8000", nil, "[::1]:8000", true},
		{"[::1]:8000", nil, "localhost", true},
		{"localhost:8000", nil, "127.0.0.2:8000", true},
		{"127.0.0.1:8000", nil, "rebind.evil:8000", false},
		{"127.0.0.1:8000", nil, "localhost.evil:8000", false},
		{"127.0.0.1:8000", nil, "192.168.1.5:8000", false},
		{"127.0.0.1:8000", nil, "", false},
		{"127.0.0.1:8000", []string{"app.example.com"}, "app.example.com:8000", true},
		{"127.0.0.1:8000", []string{".example.com"}, "x.example.com", true},
		{"127.0.0.1:8000", []string{".example.com"}, "example.com", true},
		{"127.0.0.1:8000", []string{".example.com"}, "evilexample.com", false},
		// Bound elsewhere: loopback names, IP literals, the machine's own
		// name and AllowedHosts; every other name is refused.
		{":8000", nil, "rebind.evil:8000", false},
		{"0.0.0.0:8000", nil, "anything", false},
		{"[::]:8000", nil, "anything", false},
		{":8000", nil, "", false},
		{":8000", nil, "192.168.1.5:8000", true},
		{":8000", nil, "[2001:db8::7]:8000", true},
		{":8000", nil, "localhost:8000", true},
		{":8000", nil, "web01.corp.example:8000", true},
		{":8000", nil, "WEB01.corp.example.:8000", true},
		{":8000", nil, "web01:8000", true},
		{":8000", nil, "web01.other.example:8000", false},
		{":8000", nil, "corp.example:8000", false},
		{":8000", []string{"app.example.com"}, "app.example.com", true},
		{":8000", []string{"app.example.com"}, "192.168.1.5:8000", true},
		{":8000", []string{"app.example.com"}, "localhost:8000", true},
		{":8000", []string{"app.example.com"}, "rebind.evil:8000", false},
		// AllowedHosts entries: ports are ignored, "*.x" is ".x".
		{":8000", []string{"app.example.com:8443"}, "app.example.com:8000", true},
		{":8000", []string{"*.example.com"}, "x.example.com", true},
		{":8000", []string{"*.example.com"}, "example.com", true},
		{":8000", []string{"*.example.com"}, "evilexample.com", false},
		{":8000", []string{"[2001:db8::7]:8000"}, "[2001:db8::7]:8000", true},
		{":8000", []string{"*", "", ":8000"}, "rebind.evil", false},
	} {
		if got := newHostPolicy(tc.listen, tc.allowed).allows(tc.host); got != tc.want {
			t.Errorf("listen %q allowed %v: Host %q = %v, want %v", tc.listen, tc.allowed, tc.host, got, tc.want)
		}
	}
}

func rebindRequest(t *testing.T, tsURL, method, path, host, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, tsURL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Host = host
	req.Header.Set("Origin", "http://"+host)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

// DNS rebinding made a page on an attacker's name same-origin with a local
// server: the browser sends the attacker's name as both Origin and Host, and
// the same-origin default accepted Origin == Host, so any website could call
// /mcp (execute_script included) and read the result (CWE-346, strix 4250
// vuln-0010). A loopback-bound server now answers only loopback Host names,
// on every endpoint, before any route runs.
func TestDNSRebindingRefusedOnLoopbackBind(t *testing.T) {
	s := buildMinimalMCPServer(t, nil)
	s.config.Address = "127.0.0.1:8000"
	s.config.JSONRPC = true
	s.hosts = newHostPolicy(s.config.Address, nil)
	s.handlers = map[string]string{"GET /hello": "lib.hello"}
	s.wsHandlers = map[string]string{"/ws": "lib.ws"}
	ts := httptest.NewServer(s.buildMux())
	defer ts.Close()

	list := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`
	for _, rq := range []struct{ method, path, body string }{
		{"POST", "/mcp", list},
		{"POST", "/json-rpc", `{"jsonrpc":"2.0","id":1,"method":"x"}`},
		{"GET", "/health", ""},
		{"GET", "/hello", ""},
		{"GET", "/ws", ""},
	} {
		resp := rebindRequest(t, ts.URL, rq.method, rq.path, "rebind.evil:8000", rq.body)
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusMisdirectedRequest {
			t.Errorf("%s %s with a rebound Host = %d %q, want 421", rq.method, rq.path, resp.StatusCode, body)
		}
	}

	// The legitimate local client still works, by IP and by localhost.
	for _, host := range []string{"127.0.0.1:8000", "localhost:8000"} {
		resp := rebindRequest(t, ts.URL, "POST", "/mcp", host, list)
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK || !bytes.Contains(body, []byte("greet")) {
			t.Errorf("same-origin %s: %d %q", host, resp.StatusCode, body)
		}
	}
}

// A server bound to every interface answers every Host by default (it cannot
// know its public names); --allowed-hosts opts it into checking.
// A server bound to every interface used to answer any Host unless
// --allowed-hosts was set, which left the most common bind (":8000") open to
// rebinding. It now answers loopback names, IP literals, its own host name
// and the listed names only, and the refusal names the flag to set.
func TestHostCheckOnWildcardBind(t *testing.T) {
	localHostname = func() (string, error) { return "myserver.lan", nil }
	t.Cleanup(func() { localHostname = os.Hostname })
	list := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`
	s := buildMinimalMCPServer(t, nil)
	s.config.Address = ":8000"
	s.hosts = newHostPolicy(s.config.Address, nil)
	ts := httptest.NewServer(s.buildMux())
	for host, want := range map[string]int{
		"myserver.lan:8000":   http.StatusOK,
		"myserver:8000":       http.StatusOK,
		"192.168.1.5:8000":    http.StatusOK,
		"localhost:8000":      http.StatusOK,
		"rebind.evil:8000":    http.StatusMisdirectedRequest,
		"app.example.com:443": http.StatusMisdirectedRequest,
	} {
		resp := rebindRequest(t, ts.URL, "POST", "/mcp", host, list)
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != want {
			t.Errorf("no allowed hosts: Host %s = %d, want %d", host, resp.StatusCode, want)
		}
		if want == http.StatusMisdirectedRequest && !strings.Contains(string(body), "--allowed-hosts") {
			t.Errorf("refusal for %s does not name the flag: %q", host, body)
		}
	}
	ts.Close()

	s.hosts = newHostPolicy(":8000", []string{"app.example.com"})
	ts = httptest.NewServer(s.buildMux())
	defer ts.Close()
	for host, want := range map[string]int{
		"app.example.com:443": http.StatusOK,
		"myserver.lan:8000":   http.StatusOK,
		"192.168.1.5:8000":    http.StatusOK,
		"rebind.evil:8000":    http.StatusMisdirectedRequest,
		"localhost:8000":      http.StatusOK,
	} {
		resp := rebindRequest(t, ts.URL, "POST", "/mcp", host, list)
		resp.Body.Close()
		if resp.StatusCode != want {
			t.Errorf("Host %s = %d, want %d", host, resp.StatusCode, want)
		}
	}
}

// The same-origin rule no longer accepts Origin == Host by itself.
func TestSameOriginRequiresAnsweredHost(t *testing.T) {
	r := httptest.NewRequest("POST", "/mcp", nil)
	r.Host = "rebind.evil:8000"
	r.Header.Set("Origin", "http://rebind.evil:8000")
	if browserOriginAllowed(r, nil, newHostPolicy("127.0.0.1:8000", nil)) {
		t.Fatal("a rebound Origin == Host was accepted as same-origin on a loopback bind")
	}
	r.Host = "localhost:8000"
	r.Header.Set("Origin", "http://localhost:8000")
	if !browserOriginAllowed(r, nil, newHostPolicy("127.0.0.1:8000", nil)) {
		t.Fatal("a genuine same-origin request was refused")
	}
}

// The script execution tool over HTTP requires a bearer token; over stdio it
// does not.
func TestExecToolOverHTTPRequiresBearerToken(t *testing.T) {
	_, err := NewServer(ServerConfig{Address: "127.0.0.1:0", MCPExecTool: true})
	if err == nil || !strings.Contains(err.Error(), "requires --bearer-token") {
		t.Fatalf("expected a bearer-token error, got %v", err)
	}
	for name, cfg := range map[string]ServerConfig{
		"with token": {Address: "127.0.0.1:0", MCPExecTool: true, BearerToken: "t"},
		"stdio":      {MCPExecTool: true},
	} {
		if _, err := NewServer(cfg); err != nil && strings.Contains(err.Error(), "bearer-token") {
			t.Errorf("%s: %v", name, err)
		}
	}
}
