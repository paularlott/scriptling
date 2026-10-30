package extlibs

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/fssecurity"
	"github.com/paularlott/scriptling/extlibs/netsecurity"
	"github.com/paularlott/scriptling/stdlib"
)

// newSelfSignedServer is an https server whose certificate nothing trusts.
func newSelfSignedServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newRequestsInterpreter(cfg ...*netsecurity.Config) *scriptling.Scriptling {
	p := scriptling.New()
	stdlib.RegisterAll(p)
	RegisterRequestsLibrary(p, cfg...)
	return p
}

// The certificate is checked unless the script turns it off with verify=False.
func TestRequestsVerify(t *testing.T) {
	srv := newSelfSignedServer(t)

	cases := []struct {
		name, call string
		ok         bool
	}{
		{"default", `requests.get("` + srv.URL + `")`, false},
		{"verify True", `requests.get("` + srv.URL + `", verify=True)`, false},
		{"verify None is the default", `requests.get("` + srv.URL + `", verify=None)`, false},
		{"verify False", `requests.get("` + srv.URL + `", verify=False)`, true},
		{"post verify False", `requests.post("` + srv.URL + `", json={"a": 1}, verify=False)`, true},
		{"put verify False", `requests.put("` + srv.URL + `", data="x", verify=False)`, true},
		{"patch verify False", `requests.patch("` + srv.URL + `", data="x", verify=False)`, true},
		{"delete verify False", `requests.delete("` + srv.URL + `", verify=False)`, true},
	}
	for _, tc := range cases {
		result, err := newRequestsInterpreter().Eval("import requests\n" + tc.call + ".status_code")
		if !tc.ok {
			if err == nil || !strings.Contains(err.Error(), "certificate") {
				t.Errorf("%s: want a certificate error, got %v, %v", tc.name, result, err)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: %v", tc.name, err)
			continue
		}
		if status, _ := result.AsInt(); status != 200 {
			t.Errorf("%s: status %d, want 200", tc.name, status)
		}
	}
}

func TestRequestsVerifyMustBeABoolOrAPath(t *testing.T) {
	srv := newSelfSignedServer(t)
	cases := map[string]string{`0`: "BOOLEAN or STRING", `[]`: "BOOLEAN or STRING", `""`: "CA bundle path is empty"}
	for value, want := range cases {
		_, err := newRequestsInterpreter().Eval(`import requests
requests.get("` + srv.URL + `", verify=` + value + `)`)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("verify=%s: want %q, got %v", value, want, err)
		}
	}
}

// Under a network policy verify=False only stops the certificate being
// checked: every rule of the policy still applies.
func TestRequestsVerifyFalseKeepsTheNetworkPolicy(t *testing.T) {
	srv := newSelfSignedServer(t)
	call := "import requests\nrequests.get('" + srv.URL + "', verify=False).status_code"

	allowed := newRequestsInterpreter(&netsecurity.Config{AllowIPLiterals: true, AllowLoopback: true})
	result, err := allowed.Eval(call)
	if err != nil {
		t.Fatalf("allowed by the policy: %v", err)
	}
	if status, _ := result.AsInt(); status != 200 {
		t.Errorf("allowed by the policy: status %d, want 200", status)
	}

	blocked := newRequestsInterpreter(&netsecurity.Config{AllowLoopback: true})
	if _, err := blocked.Eval(call); err == nil || !strings.Contains(err.Error(), "IP literals") {
		t.Errorf("verify=False got past the policy: %v", err)
	}
}

// In a batch each request has its own verify; a failure is a status 0 response.
func TestParallelVerify(t *testing.T) {
	srv := newSelfSignedServer(t)
	result, err := newRequestsInterpreter().Eval(`import requests
rs = requests.parallel([
    {"method": "GET", "url": "` + srv.URL + `", "verify": False},
    {"method": "GET", "url": "` + srv.URL + `"},
])
[rs[0].status_code, rs[1].status_code]`)
	if err != nil {
		t.Fatal(err)
	}
	list, lerr := result.AsList()
	if lerr != nil || len(list) != 2 {
		t.Fatalf("unexpected result %v", result.Inspect())
	}
	if s, _ := list[0].AsInt(); s != 200 {
		t.Errorf("verify False: status %d, want 200", s)
	}
	if s, _ := list[1].AsInt(); s != 0 {
		t.Errorf("default: status %d, want 0 (certificate refused)", s)
	}
}

// writeCABundle writes srv's certificate as a PEM CA bundle in dir.
func writeCABundle(t *testing.T, srv *httptest.Server, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "ca.pem")
	data := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// writeOtherCA writes a PEM bundle of a freshly made CA certificate in dir:
// every httptest TLS server shares one certificate, so this is a CA none of
// them is signed by.
func writeOtherCA(t *testing.T, dir string) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	serial, _ := rand.Int(rand.Reader, big.NewInt(1<<62))
	tmpl := &x509.Certificate{
		SerialNumber: serial, Subject: pkix.Name{CommonName: "other test CA"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "other-ca.pem")
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func statusOf(t *testing.T, p *scriptling.Scriptling, script string) (int64, error) {
	t.Helper()
	result, err := p.Eval(script)
	if err != nil {
		return 0, err
	}
	status, _ := result.AsInt()
	return status, nil
}

// verify="path" trusts the CA certificates in that file instead of the system's,
// as in Python's requests: the server's own certificate works, another does not.
func TestRequestsVerifyCABundle(t *testing.T) {
	srv := newSelfSignedServer(t)
	bundle := writeCABundle(t, srv, t.TempDir())
	otherBundle := writeOtherCA(t, t.TempDir())

	for _, method := range []string{"get", "post", "put", "patch", "delete"} {
		status, err := statusOf(t, newRequestsInterpreter(),
			"import requests\nrequests."+method+"('"+srv.URL+"', verify='"+bundle+"').status_code")
		if err != nil || status != 200 {
			t.Errorf("%s with its own CA: status %d, err %v", method, status, err)
		}
	}
	if _, err := statusOf(t, newRequestsInterpreter(),
		"import requests\nrequests.get('"+srv.URL+"', verify='"+otherBundle+"').status_code"); err == nil ||
		!strings.Contains(err.Error(), "certificate") {
		t.Errorf("another CA: want a certificate error, got %v", err)
	}
}

func TestRequestsVerifyCABundleErrors(t *testing.T) {
	srv := newSelfSignedServer(t)
	dir := t.TempDir()
	notPEM := filepath.Join(dir, "not.pem")
	if err := os.WriteFile(notPEM, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		notPEM:                            "no PEM certificates",
		filepath.Join(dir, "missing.pem"): "no such file",
	}
	for path, want := range cases {
		_, err := statusOf(t, newRequestsInterpreter(), "import requests\nrequests.get('"+srv.URL+"', verify='"+path+"')")
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: want %q, got %v", filepath.Base(path), want, err)
		}
	}
}

// Under --allowed-paths a CA bundle must lie within them: outside, the read is a
// jail denial, an uncatchable PermissionError, as in the os library.
func TestRequestsVerifyCABundleIsConfined(t *testing.T) {
	srv := newSelfSignedServer(t)
	allowed := t.TempDir()
	inside := writeCABundle(t, srv, allowed)
	outside := writeCABundle(t, srv, t.TempDir())

	jailed := func() *scriptling.Scriptling {
		p := scriptling.New()
		stdlib.RegisterAll(p)
		RegisterRequestsLibraryConfigured(p, fssecurity.Config{AllowedPaths: []string{allowed}}, nil)
		return p
	}
	if status, err := statusOf(t, jailed(),
		"import requests\nrequests.get('"+srv.URL+"', verify='"+inside+"').status_code"); err != nil || status != 200 {
		t.Errorf("bundle inside the allowed paths: status %d, err %v", status, err)
	}
	_, err := jailed().Eval(`import requests
caught = False
try:
    requests.get("` + srv.URL + `", verify="` + outside + `")
except Exception:
    caught = True
caught`)
	if err == nil || !strings.Contains(err.Error(), "outside allowed directories") {
		t.Errorf("bundle outside the allowed paths: want an uncatchable PermissionError, got %v", err)
	}

	link := filepath.Join(allowed, "link.pem")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	if _, err := statusOf(t, jailed(), "import requests\nrequests.get('"+srv.URL+"', verify='"+link+"')"); err == nil ||
		!strings.Contains(err.Error(), "outside allowed directories") {
		t.Errorf("symlink in the allowed paths to a bundle outside them: want a PermissionError, got %v", err)
	}
}

// A CA bundle changes only what is trusted: the network policy still applies.
func TestRequestsVerifyCABundleKeepsTheNetworkPolicy(t *testing.T) {
	srv := newSelfSignedServer(t)
	bundle := writeCABundle(t, srv, t.TempDir())
	call := "import requests\nrequests.get('" + srv.URL + "', verify='" + bundle + "').status_code"

	allowed := scriptling.New()
	RegisterRequestsLibraryConfigured(allowed, fssecurity.Config{}, &netsecurity.Config{AllowIPLiterals: true, AllowLoopback: true})
	if status, err := statusOf(t, allowed, call); err != nil || status != 200 {
		t.Errorf("allowed by the policy: status %d, err %v", status, err)
	}

	blocked := scriptling.New()
	RegisterRequestsLibraryConfigured(blocked, fssecurity.Config{}, &netsecurity.Config{AllowLoopback: true})
	if _, err := statusOf(t, blocked, call); err == nil || !strings.Contains(err.Error(), "IP literals") {
		t.Errorf("a CA bundle got past the policy: %v", err)
	}
}

// Clients are kept by the bundle's content: reused, rebuilt when the file
// changes, and bounded.
func TestRequestsCABundleClientCache(t *testing.T) {
	srv := newSelfSignedServer(t)
	dir := t.TempDir()
	path := writeCABundle(t, srv, dir)
	lib := &requestsLib{}
	ctx := context.Background()

	first, errObj := lib.caBundleClient(ctx, path)
	if errObj != nil {
		t.Fatal(errObj.Inspect())
	}
	again, _ := lib.caBundleClient(ctx, path)
	if again != first {
		t.Error("the same bundle built a second client")
	}
	// same path, new content
	if err := os.Rename(writeOtherCA(t, dir), path); err != nil {
		t.Fatal(err)
	}
	edited, _ := lib.caBundleClient(ctx, path)
	if edited == first {
		t.Error("an edited bundle reused the old client")
	}

	editedKey := lib.caOrder[len(lib.caOrder)-1]
	for i := 0; i < maxCABundleClients+5; i++ {
		if _, errObj := lib.caBundleClient(ctx, writeOtherCA(t, t.TempDir())); errObj != nil {
			t.Fatal(errObj.Inspect())
		}
	}
	if n := len(lib.caClients); n != maxCABundleClients || len(lib.caOrder) != n {
		t.Errorf("%d clients kept (%d in order), want %d", n, len(lib.caOrder), maxCABundleClients)
	}
	if _, kept := lib.caClients[editedKey]; kept {
		t.Error("the oldest bundle's client was not the one dropped")
	}
}

// In a batch: a CA bundle per request; a jail denial raises; any other problem
// with a bundle fails only its request (status 0).
func TestParallelVerifyCABundle(t *testing.T) {
	srv := newSelfSignedServer(t)
	allowed := t.TempDir()
	bundle := writeCABundle(t, srv, allowed)
	outside := writeCABundle(t, srv, t.TempDir())
	p := scriptling.New()
	stdlib.RegisterAll(p)
	RegisterRequestsLibraryConfigured(p, fssecurity.Config{AllowedPaths: []string{allowed}}, nil)

	result, err := p.Eval(`import requests
rs = requests.parallel([
    {"method": "GET", "url": "` + srv.URL + `", "verify": "` + bundle + `"},
    {"method": "GET", "url": "` + srv.URL + `", "verify": "` + filepath.Join(allowed, "missing.pem") + `"},
])
[rs[0].status_code, rs[1].status_code]`)
	if err != nil {
		t.Fatal(err)
	}
	list, _ := result.AsList()
	if len(list) != 2 {
		t.Fatalf("unexpected result %v", result.Inspect())
	}
	if s, _ := list[0].AsInt(); s != 200 {
		t.Errorf("with its CA bundle: status %d, want 200", s)
	}
	if s, _ := list[1].AsInt(); s != 0 {
		t.Errorf("missing bundle: status %d, want 0", s)
	}

	if _, err := p.Eval(`import requests
requests.parallel([{"method": "GET", "url": "` + srv.URL + `", "verify": "` + outside + `"}])`); err == nil ||
		!strings.Contains(err.Error(), "outside allowed directories") {
		t.Errorf("bundle outside the allowed paths in a batch: want a PermissionError, got %v", err)
	}
}
