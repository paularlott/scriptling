package requests_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/fssecurity"
	"github.com/paularlott/scriptling/extlibs/netsecurity"
	"github.com/paularlott/scriptling/extlibs/requests"
	"github.com/paularlott/scriptling/internal/scripttest"
	"github.com/paularlott/scriptling/stdlib"
)

func TestLibraryNameConstant(t *testing.T) {
	if requests.LibraryName != "requests" {
		t.Errorf("requests.LibraryName = %q, want %q (scripts import it by this name)", requests.LibraryName, "requests")
	}
}

func newJSONServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"hello": "world"}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// loopbackPolicy lets scripts reach an httptest server (a 127.0.0.1 literal).
func loopbackPolicy() *netsecurity.Config {
	return &netsecurity.Config{AllowIPLiterals: true, AllowLoopback: true}
}

func TestRequestsRegisterAndGet(t *testing.T) {
	srv := newJSONServer(t)
	tests := []struct {
		name     string
		register func(*scriptling.Scriptling)
	}{
		{"Register with policy", func(p *scriptling.Scriptling) { requests.Register(p, loopbackPolicy()) }},
		{"Register without policy", func(p *scriptling.Scriptling) { requests.Register(p) }},
		{"RegisterConfigured", func(p *scriptling.Scriptling) {
			requests.RegisterConfigured(p, fssecurity.Config{AllowedPaths: []string{t.TempDir()}}, loopbackPolicy())
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := scriptling.New()
			tt.register(p)
			got := scripttest.Eval(t, p, "import requests\nr = requests.get('"+srv.URL+"')\n[r.status_code, r.json()['hello']]").Inspect()
			if want := "[200, 'world']"; got != want {
				t.Errorf("requests.get = %s, want %s", got, want)
			}
		})
	}
}

func TestRequestsNotImportableWithoutRegister(t *testing.T) {
	p := scriptling.New()
	stdlib.RegisterAll(p)
	scripttest.ImportFails(t, p, "requests", "it is only available after requests.Register(p, policy)")
}

// A request the network policy blocks raises an exception the script can
// catch (unlike filesystem path refusals, which are protected
// PermissionErrors).
func TestRequestsPolicyRefusalIsCatchable(t *testing.T) {
	srv := newJSONServer(t)
	p := scriptling.New()
	requests.Register(p, &netsecurity.Config{}) // default policy: no IP literals, no loopback
	_, msg := scripttest.Raises(t, p, "import requests\nrequests.get('"+srv.URL+"')")
	if !strings.Contains(msg, "network policy") {
		t.Errorf("blocked request raised %q, want a network policy error", msg)
	}
}
