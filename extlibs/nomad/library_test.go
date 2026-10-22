package nomad

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/netsecurity"
)

// jobsServer answers the jobs list endpoint and counts every request.
func jobsServer(hits *atomic.Int32) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"ID":"web","Name":"web"}]`))
	}
}

func evalNomad(t *testing.T, cfg *netsecurity.Config, script string) (string, error) {
	t.Helper()
	p := scriptling.New()
	if cfg != nil {
		Register(p, cfg)
	} else {
		Register(p)
	}
	result, err := p.Eval("import scriptling.nomad as nomad\n" + script)
	if err != nil {
		return "", err
	}
	return result.Inspect(), nil
}

func TestNomadWithoutPolicy(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(jobsServer(&hits))
	defer srv.Close()

	out, err := evalNomad(t, nil, `nomad.Client("`+srv.URL+`").jobs_list()[0]["id"]`)
	if err != nil || out != "web" {
		t.Fatalf("jobs_list without policy: out=%q err=%v", out, err)
	}
}

func TestNomadRejectsQueryAndFragment(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(jobsServer(&hits))
	defer srv.Close()

	for _, addr := range []string{srv.URL + "/secret#", srv.URL + "/secret?x=", srv.URL + "?"} {
		_, err := evalNomad(t, nil, `nomad.Client("`+addr+`").jobs_list()`)
		if err == nil || !strings.Contains(err.Error(), "query or fragment") {
			t.Errorf("Client(%q) should be rejected, got: %v", addr, err)
		}
	}
	if hits.Load() != 0 {
		t.Errorf("rejected addresses must not send requests, got %d", hits.Load())
	}
}

// With a network policy, Client() requests must not reach denied addresses,
// whether named directly, by a name resolving to them, or via a redirect.
func TestNomadNetworkPolicyBlocksDenied(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(jobsServer(&hits))
	defer srv.Close()
	tlsSrv := httptest.NewTLSServer(jobsServer(&hits))
	defer tlsSrv.Close()
	// localhost is trusted by the policy below, but redirects to a literal.
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, srv.URL+r.URL.Path, http.StatusFound)
	}))
	defer redirect.Close()
	redirectByName := strings.Replace(redirect.URL, "127.0.0.1", "localhost", 1)

	cases := []struct {
		name   string
		cfg    *netsecurity.Config
		script string
	}{
		{"ip literal", &netsecurity.Config{AllowLoopback: true}, `nomad.Client("` + srv.URL + `").jobs_list()`},
		{"loopback name", &netsecurity.Config{}, `nomad.Client("` + strings.Replace(srv.URL, "127.0.0.1", "localhost", 1) + `").jobs_list()`},
		{"insecure tls", &netsecurity.Config{}, `nomad.Client("` + tlsSrv.URL + `", insecure=True).jobs_list()`},
		{"redirect", &netsecurity.Config{AllowHosts: []string{"localhost"}}, `nomad.Client("` + redirectByName + `").jobs_list()`},
	}
	for _, tc := range cases {
		_, err := evalNomad(t, tc.cfg, tc.script)
		if err == nil || !strings.Contains(err.Error(), "network policy") {
			t.Errorf("%s: expected network policy error, got: %v", tc.name, err)
		}
	}
	if hits.Load() != 0 {
		t.Errorf("POLICY-BYPASS [nomad]: denied requests reached the server %d times", hits.Load())
	}
}

// A policy that permits the address still lets the client work, including
// with insecure=True against a self-signed certificate.
func TestNomadNetworkPolicyAllowsPermitted(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(jobsServer(&hits))
	defer srv.Close()
	tlsSrv := httptest.NewTLSServer(jobsServer(&hits))
	defer tlsSrv.Close()

	cfg := &netsecurity.Config{AllowLoopback: true, AllowIPLiterals: true}
	for _, script := range []string{
		`nomad.Client("` + srv.URL + `").jobs_list()[0]["id"]`,
		`nomad.Client("` + tlsSrv.URL + `", insecure=True).jobs_list()[0]["id"]`,
	} {
		out, err := evalNomad(t, cfg, script)
		if err != nil || out != "web" {
			t.Errorf("%s: out=%q err=%v", script, out, err)
		}
	}

	// Without insecure=True the self-signed certificate is still verified.
	if _, err := evalNomad(t, cfg, `nomad.Client("`+tlsSrv.URL+`").jobs_list()`); err == nil || !strings.Contains(err.Error(), "certificate") {
		t.Errorf("TLS verification should apply without insecure=True, got: %v", err)
	}
}
