package extlibs

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/netsecurity"
)

// TestNetworkWebSocketPolicy: the websocket library dials through the guard.
func TestNetworkWebSocketPolicy(t *testing.T) {
	// A plain HTTP server suffices: the denial must happen at policy check,
	// before any protocol negotiation.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	// IP-literal URL, no opt-in → denied despite the server existing.
	p := scriptling.New()
	RegisterWebSocketLibrary(p, &netsecurity.Config{AllowLoopback: true})
	out, ok := pEvalErr(t, p, `import scriptling.net.websocket as websocket
websocket.connect("`+strings.Replace(srv.URL, "http", "ws", 1)+`")`)
	if ok || !strings.Contains(out, "IP literals") {
		t.Errorf("websocket should be policy-guarded, got ok=%v err=%s", ok, out)
	}
}

// TestNetworkWaitForPolicy: wait_for's HTTP and port probes share the guard,
// and a policy denial raises instead of being reported as "not ready".
func TestNetworkWaitForPolicy(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1) }))
	defer srv.Close()
	port := srv.Listener.Addr().(*net.TCPAddr).Port

	// Loopback is allowed, so only the IP-literal rule can deny the first two;
	// the third is denied by name resolving to loopback.
	pLiteral := scriptling.New()
	RegisterWaitForLibrary(pLiteral, &netsecurity.Config{AllowLoopback: true})
	pDefault := scriptling.New()
	RegisterWaitForLibrary(pDefault, &netsecurity.Config{})
	for _, tc := range []struct {
		p      *scriptling.Scriptling
		script string
	}{
		{pLiteral, `wait_for.http("` + srv.URL + `", timeout=5)`},
		{pLiteral, `wait_for.port("127.0.0.1", ` + strconv.Itoa(port) + `, timeout=5)`},
		{pDefault, `wait_for.port("localhost", ` + strconv.Itoa(port) + `, timeout=5)`},
	} {
		p, script := tc.p, tc.script
		start := time.Now()
		out, ok := pEvalErr(t, p, "import scriptling.wait_for as wait_for\n"+script)
		if ok || !strings.Contains(out, "network policy") {
			t.Errorf("POLICY-BYPASS [wait_for]: %s: ok=%v err=%s", script, ok, out)
		}
		if time.Since(start) > 3*time.Second {
			t.Errorf("%s: policy denial should fail fast, took %v", script, time.Since(start))
		}
	}
	if hits.Load() != 0 {
		t.Errorf("POLICY-BYPASS [wait_for]: denied probe reached the server %d times", hits.Load())
	}
}

// TestNetworkWaitForPolicyPermits: addresses the policy allows still probe
// normally, open or closed.
func TestNetworkWaitForPolicyPermits(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port

	p := scriptling.New()
	RegisterWaitForLibrary(p, &netsecurity.Config{AllowLoopback: true, AllowIPLiterals: true})
	result, err := p.Eval(`import scriptling.wait_for as wait_for
wait_for.port("127.0.0.1", ` + strconv.Itoa(port) + `, timeout=2)`)
	if err != nil || result.Inspect() != "True" {
		t.Fatalf("open permitted port: result=%v err=%v", result, err)
	}

	ln.Close()
	result, err = p.Eval(`import scriptling.wait_for as wait_for
wait_for.port("127.0.0.1", ` + strconv.Itoa(port) + `, timeout=1, poll_rate=0.2)`)
	if err != nil || result.Inspect() != "False" {
		t.Fatalf("closed permitted port: result=%v err=%v", result, err)
	}
}

func pEvalErr(t *testing.T, p *scriptling.Scriptling, script string) (string, bool) {
	t.Helper()
	result, err := p.Eval(script)
	if err != nil {
		return err.Error(), false
	}
	return result.Inspect(), true
}
