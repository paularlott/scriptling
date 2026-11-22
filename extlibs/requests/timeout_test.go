package requests

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/netsecurity"
	"github.com/paularlott/scriptling/pool"
	"github.com/paularlott/scriptling/stdlib"
)

// slowServer answers after delay.
func slowServer(t *testing.T, delay time.Duration) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(delay):
		case <-r.Context().Done():
		}
		w.WriteHeader(200)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func evalRequests(t *testing.T, script string) (string, error) {
	t.Helper()
	p := scriptling.New()
	stdlib.RegisterAll(p)
	Register(p)
	result, err := p.Eval("import requests\n" + script)
	if err != nil {
		return "", err
	}
	return result.Inspect(), nil
}

// The timeout is seconds as in Python's requests: a fraction is not cut to
// zero, None means no timeout of its own, and a wrong type is an error, not a
// silent fall back to the default.
func TestRequestTimeoutValues(t *testing.T) {
	srv := slowServer(t, 300*time.Millisecond)

	got, err := evalRequests(t, `requests.get("`+srv.URL+`", timeout=2.5).status_code`)
	if err != nil || got != "200" {
		t.Fatalf("timeout=2.5: got %q, %v", got, err)
	}
	got, err = evalRequests(t, `requests.get("`+srv.URL+`", timeout=None).status_code`)
	if err != nil || got != "200" {
		t.Fatalf("timeout=None: got %q, %v", got, err)
	}

	start := time.Now()
	_, err = evalRequests(t, `requests.get("`+srv.URL+`", timeout=0.1)`)
	if err == nil || !strings.Contains(err.Error(), "http timeout after 0.1 seconds") {
		t.Fatalf("timeout=0.1: want a 0.1 second timeout, got %v", err)
	}
	if d := time.Since(start); d < 80*time.Millisecond {
		t.Fatalf("timeout=0.1 gave up after %v: the fraction was dropped", d)
	}

	for _, bad := range []string{`"10"`, `True`, `0`, `-1`, `0.0`} {
		_, err := evalRequests(t, `requests.get("`+srv.URL+`", timeout=`+bad+`)`)
		if err == nil || !strings.Contains(err.Error(), "timeout must be") {
			t.Errorf("timeout=%s: want an error, got %v", bad, err)
		}
	}
}

func TestParallelTimeoutValues(t *testing.T) {
	srv := slowServer(t, 300*time.Millisecond)
	got, err := evalRequests(t, `[r.status_code for r in requests.parallel([
    {"url": "`+srv.URL+`", "timeout": 2.5},
    {"url": "`+srv.URL+`", "timeout": 0.1},
    {"url": "`+srv.URL+`", "timeout": "10"},
])]`)
	if err != nil || got != "[200, 0, 0]" {
		t.Fatalf("got %q, %v", got, err)
	}
}

// The shared pool's general client timeout must not cut a longer request
// timeout short: it used to cap every request at 30 seconds.
func TestRequestTimeoutNotCappedByPool(t *testing.T) {
	// The pool is built once per process, so shorten its client's timeout
	// in place rather than through SetConfig.
	shared := pool.GetHTTPClient()
	old := shared.Timeout
	shared.Timeout = 100 * time.Millisecond
	t.Cleanup(func() { shared.Timeout = old })

	srv := slowServer(t, 300*time.Millisecond)
	got, err := evalRequests(t, `requests.get("`+srv.URL+`", timeout=2).status_code`)
	if err != nil || got != "200" {
		t.Fatalf("get: got %q, %v", got, err)
	}
	got, err = evalRequests(t, `requests.parallel([{"url": "`+srv.URL+`", "timeout": 2}])[0].status_code`)
	if err != nil || got != "200" {
		t.Fatalf("parallel: got %q, %v", got, err)
	}
}

// A network policy's client_timeout is the operator's cap and still applies,
// reported as a timeout rather than a bare client error.
func TestRequestTimeoutPolicyCapStays(t *testing.T) {
	srv := slowServer(t, 300*time.Millisecond)
	p := scriptling.New()
	Register(p, &netsecurity.Config{AllowAll: true, ClientTimeout: 100 * time.Millisecond})
	_, err := p.Eval(`import requests
requests.get("` + srv.URL + `", timeout=2)`)
	if err == nil || !strings.Contains(err.Error(), "http timeout after 0.1 seconds (the network policy's client_timeout)") {
		t.Fatalf("want the policy's timeout, got %v", err)
	}
}
