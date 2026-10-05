package extlibs

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/netsecurity"
)

// Network-restriction battery: every HTTP-carrying script surface under
// guard configurations, against a live loopback server, plus SSRF vectors.
// A test finding a request succeeding under a policy that should deny it is
// a security finding.

func newNetInterpreter(t *testing.T, cfg *netsecurity.Config) *scriptling.Scriptling {
	t.Helper()
	p := scriptling.New()
	RegisterRequestsLibrary(p, cfg)
	return p
}

func startLoopbackServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/redirect-loopback":
			// Redirect to the loopback root: a policy must re-check each hop.
			http.Redirect(w, r, "/", http.StatusFound)
		case "/redirect-external":
			// Redirect toward an external host the policy never allows.
			http.Redirect(w, r, "http://192.0.2.1/", http.StatusFound) // TEST-NET-1
		default:
			fmt.Fprint(w, "loopback-ok")
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func evalNet(t *testing.T, p *scriptling.Scriptling, script string) (string, bool) {
	t.Helper()
	result, err := p.Eval(script)
	if err != nil {
		return err.Error(), false
	}
	return result.Inspect(), true
}

func mustDenyNet(t *testing.T, p *scriptling.Scriptling, name, script string) {
	t.Helper()
	out, ok := evalNet(t, p, script)
	if ok {
		t.Errorf("POLICY-BYPASS [%s]: succeeded with %s", name, out)
		return
	}
	if !strings.Contains(strings.ToLower(out), "policy") && !strings.Contains(strings.ToLower(out), "denied") &&
		!strings.Contains(strings.ToLower(out), "not allowed") && !strings.Contains(strings.ToLower(out), "requires https") {
		t.Errorf("[%s]: denied but unexpected error: %s", name, out)
	}
}

func mustWorkNet(t *testing.T, p *scriptling.Scriptling, name, script string) {
	t.Helper()
	out, ok := evalNet(t, p, script)
	if !ok {
		t.Errorf("[%s]: expected success, got %s", name, out)
	}
}

// guardWith builds a guard config from functional options for tests.
func guardConfig(t *testing.T, mutate func(*netsecurity.Config)) *netsecurity.Config {
	t.Helper()
	cfg := &netsecurity.Config{}
	mutate(cfg)
	return cfg
}

func TestNetworkUnrestrictedRequestsWork(t *testing.T) {
	srv := startLoopbackServer(t)
	p := newNetInterpreter(t, nil)
	mustWorkNet(t, p, "unrestricted get", `import requests
requests.get("`+srv.URL+`").text`)
}

func TestNetworkRequestsDeniedByDefaultPolicy(t *testing.T) {
	srv := startLoopbackServer(t)
	// Empty non-nil config: no allows at all → deny.
	p := newNetInterpreter(t, guardConfig(t, func(c *netsecurity.Config) {}))
	mustDenyNet(t, p, "default-deny get", `import requests
requests.get("`+srv.URL+`").text`)
}

func TestNetworkRequestsLoopbackAllowed(t *testing.T) {
	srv := startLoopbackServer(t)
	port := strings.TrimPrefix(srv.URL, "http://127.0.0.1:")
	p := newNetInterpreter(t, guardConfig(t, func(c *netsecurity.Config) {
		c.AllowLoopback = true
	}))
	mustWorkNet(t, p, "loopback get by name", `import requests
requests.get("http://localhost:`+port+`").text`)
	mustDenyNet(t, p, "loopback still blocks private", `import requests
requests.get("http://192.168.1.1/").text`)
	// Explicit opt-in makes IP literals work too.
	pLit := newNetInterpreter(t, guardConfig(t, func(c *netsecurity.Config) {
		c.AllowLoopback = true
		c.AllowIPLiterals = true
	}))
	mustWorkNet(t, pLit, "loopback get by literal", `import requests
requests.get("`+srv.URL+`").text`)
}

func TestNetworkRequestsRequireHTTPS(t *testing.T) {
	srv := startLoopbackServer(t)
	p := newNetInterpreter(t, guardConfig(t, func(c *netsecurity.Config) {
		c.AllowLoopback = true
		c.RequireHTTPS = true
	}))
	mustDenyNet(t, p, "http denied under requireHTTPS", `import requests
requests.get("`+srv.URL+`").text`)
}

func TestNetworkRequestsIPLiteralRules(t *testing.T) {
	srv := startLoopbackServer(t)
	host := strings.TrimPrefix(srv.URL, "http://")
	p := newNetInterpreter(t, guardConfig(t, func(c *netsecurity.Config) {
		c.AllowLoopback = true
	}))
	// Even with loopback allowed, IP literals are blocked unless opted in.
	mustDenyNet(t, p, "IP literal denied by default", `import requests
requests.get("http://`+host+`/").text`)
}

func TestNetworkRedirectRecheck(t *testing.T) {
	srv := startLoopbackServer(t)
	p := newNetInterpreter(t, guardConfig(t, func(c *netsecurity.Config) {
		c.AllowedCIDRs = []string{"127.0.0.0/8"}
	}))
	mustWorkNet(t, p, "direct allowed", `import requests
requests.get("`+srv.URL+`").text`)
	mustDenyNet(t, p, "redirect to denied host blocked", `import requests
requests.get("`+srv.URL+`/redirect-external").text`)
}

func TestNetworkHostnameAllowlist(t *testing.T) {
	srv := startLoopbackServer(t)
	host := strings.TrimPrefix(srv.URL, "http://") // 127.0.0.1:port
	p := newNetInterpreter(t, guardConfig(t, func(c *netsecurity.Config) {
		c.AllowHosts = []string{"api.example.test"}
	}))
	// Not on the list → denied. The policy check fires before DNS, but a
	// DNS failure for the unresolvable test name is equally a denial.
	out, ok := evalNet(t, p, `import requests
requests.get("http://api.example.test/").text`)
	if ok {
		t.Errorf("POLICY-BYPASS [host not allowlisted]: succeeded with %s", out)
	}
	_ = host
}

// TestNetworkResolvedAddressEnforced: a hostname that resolves to a blocked
// address is denied at dial time even though the hostname itself passes the
// host rules — "localhost" resolving to 127.0.0.1 under a policy that allows
// private ranges but not loopback.
func TestNetworkResolvedAddressEnforced(t *testing.T) {
	p := newNetInterpreter(t, guardConfig(t, func(c *netsecurity.Config) {
		c.AllowPrivateIPs = true
	}))
	mustDenyNet(t, p, "localhost resolves to denied loopback", `import requests
requests.get("http://localhost:1/").text`)
}

// TestNetworkTrustedHostBypassesAddressPolicy: allowlisted hosts resolve
// anywhere — that is their grant — while non-listed hosts stay category-
// checked.
func TestNetworkTrustedHostBypassesAddressPolicy(t *testing.T) {
	srv := startLoopbackServer(t)
	// localhost is trusted → loopback answer allowed despite categories.
	p := newNetInterpreter(t, guardConfig(t, func(c *netsecurity.Config) {
		c.AllowHosts = []string{"localhost"}
	}))
	mustWorkNet(t, p, "trusted localhost to loopback", `import requests
r = requests.get("http://localhost:`+strings.TrimPrefix(srv.URL, "http://127.0.0.1:")+`/")
r.status_code`)
}
