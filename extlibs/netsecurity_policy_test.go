package extlibs

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/netsecurity"
)

func TestRequestsNetworkPolicyAllowsLoopback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status": "ok"}`))
	}))
	defer srv.Close()

	p := scriptling.New()
	RegisterRequestsLibrary(p, &netsecurity.Config{
		AllowIPLiterals: true, // httptest URLs are 127.0.0.1 literals
		AllowLoopback:   true,
	})

	result, err := p.Eval("import requests\nresp = requests.get('" + srv.URL + "')\nresp.status_code")
	if err != nil {
		t.Fatalf("allowed request failed: %v", err)
	}
	if status, serr := result.AsInt(); serr != nil || status != 200 {
		t.Errorf("status = %v, want 200", result.Inspect())
	}
}

func TestRequestsNetworkPolicyBlocksLiteralByDefault(t *testing.T) {
	p := scriptling.New()
	RegisterRequestsLibrary(p, &netsecurity.Config{AllowLoopback: true})

	_, err := p.Eval("import requests\nrequests.get('http://127.0.0.1:8080/')")
	if err == nil || !strings.Contains(err.Error(), "IP literals") {
		t.Errorf("expected IP-literal block, got: %v", err)
	}
}

func TestRequestsNetworkPolicyBlocksResolvedLoopback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	// Rewrite the URL to use the "localhost" hostname so the request goes
	// through the resolve-validate-dial path rather than the literal rule.
	hostURL := strings.Replace(srv.URL, "127.0.0.1", "localhost", 1)

	// Blocked: localhost resolves to a loopback address.
	p := scriptling.New()
	RegisterRequestsLibrary(p, &netsecurity.Config{})
	_, err := p.Eval("import requests\nrequests.get('" + hostURL + "')")
	if err == nil || !strings.Contains(err.Error(), "loopback") {
		t.Errorf("expected loopback block via resolution, got: %v", err)
	}

	// Allowed with the opt-in.
	p2 := scriptling.New()
	RegisterRequestsLibrary(p2, &netsecurity.Config{AllowLoopback: true})
	if _, err := p2.Eval("import requests\nrequests.get('" + hostURL + "')"); err != nil {
		t.Errorf("loopback opt-in should allow localhost: %v", err)
	}
}

func TestRequestsNetworkPolicyRequiresHTTPS(t *testing.T) {
	p := scriptling.New()
	RegisterRequestsLibrary(p, &netsecurity.Config{RequireHTTPS: true})

	_, err := p.Eval("import requests\nrequests.get('http://example.com/')")
	if err == nil || !strings.Contains(err.Error(), "requires https") {
		t.Errorf("expected https-only block, got: %v", err)
	}
}

func TestRequestsNetworkPolicyBlocksRedirectToBlockedTarget(t *testing.T) {
	// Server that redirects every request to the cloud metadata address.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://169.254.169.254/latest/meta-data/", http.StatusFound)
	}))
	defer srv.Close()

	p := scriptling.New()
	RegisterRequestsLibrary(p, &netsecurity.Config{
		AllowIPLiterals: true,
		AllowLoopback:   true,
	})

	_, err := p.Eval("import requests\nrequests.get('" + srv.URL + "')")
	if err == nil || !strings.Contains(err.Error(), "network policy") {
		t.Errorf("redirect to blocked target must be policy-blocked, got: %v", err)
	}
}

func TestRequestsNetworkPolicyFailClosedOnBadConfig(t *testing.T) {
	p := scriptling.New()
	RegisterRequestsLibrary(p, &netsecurity.Config{AllowedCIDRs: []string{"not-a-cidr"}})

	_, err := p.Eval("import requests\nrequests.get('http://93.184.216.34/')")
	if err == nil || !strings.Contains(err.Error(), "invalid configuration") {
		t.Errorf("bad config must fail closed, got: %v", err)
	}
}

func TestWaitForHTTPNetworkPolicy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p := scriptling.New()
	RegisterWaitForLibrary(p, &netsecurity.Config{
		AllowIPLiterals: true,
		AllowLoopback:   true,
	})
	result, err := p.Eval("import scriptling.wait_for as wait_for\nwait_for.http('" + srv.URL + "')")
	if err != nil {
		t.Fatalf("wait_for eval: %v", err)
	}
	if result.Inspect() != "True" {
		t.Errorf("allowed wait_for.http should return True, got %s", result.Inspect())
	}

	p2 := scriptling.New()
	RegisterWaitForLibrary(p2, &netsecurity.Config{})
	result, err = p2.Eval("import scriptling.wait_for as wait_for\nwait_for.http('" + srv.URL + "', timeout=1)")
	if err != nil {
		t.Fatalf("wait_for eval: %v", err)
	}
	if result.Inspect() != "False" {
		t.Error("policy-blocked wait_for.http should return False, not error")
	}
}

func TestWebSocketNetworkPolicyBlocksDial(t *testing.T) {
	p := scriptling.New()
	RegisterWebSocketLibrary(p, &netsecurity.Config{})

	_, err := p.Eval("import scriptling.net.websocket as websocket\nwebsocket.connect('ws://10.0.0.1:8080/', timeout=1)")
	if err == nil || !strings.Contains(err.Error(), "network policy") {
		t.Errorf("policy-blocked websocket dial should fail clearly, got: %v", err)
	}

	_, err = p.Eval("import scriptling.net.websocket as websocket\nwebsocket.connect('ws://127.0.0.1:1/', timeout=1)")
	if err == nil || !strings.Contains(err.Error(), "IP literals") {
		t.Errorf("websocket literal dial should be blocked, got: %v", err)
	}
}

func TestNilPolicyKeepsExistingBehavior(t *testing.T) {
	// No config: registration compiles and requests still work (httptest is
	// a loopback literal, which only a policy would restrict).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p := scriptling.New()
	RegisterRequestsLibrary(p)
	RegisterWaitForLibrary(p)
	RegisterWebSocketLibrary(p)

	result, err := p.Eval("import requests\nresp = requests.get('" + srv.URL + "')\nresp.status_code")
	if err != nil {
		t.Fatalf("unrestricted request failed: %v", err)
	}
	if status, serr := result.AsInt(); serr != nil || status != 200 {
		t.Errorf("status = %v, want 200", result.Inspect())
	}
}

// Resolver-only mode (nameservers without a policy): requests must resolve
// through the configured servers and no address checks apply.
func TestRequestsResolverOnlyModeUsesConfiguredDNS(t *testing.T) {
	p := scriptling.New()
	RegisterRequestsLibrary(p, &netsecurity.Config{
		AllowAll:   true,
		DNSServers: []string{"127.0.0.1:1"}, // nothing listens: lookups must fail
	})

	_, err := p.Eval("import requests\nrequests.get('http://example.test/', timeout=2)")
	if err == nil {
		t.Fatal("expected DNS failure via dead nameserver")
	}
	if msg := err.Error(); strings.Contains(msg, "network policy") {
		t.Errorf("resolver-only mode must not policy-block, got: %v", err)
	}
}
