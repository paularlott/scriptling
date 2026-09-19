package plugin

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/paularlott/scriptling/extlibs/netsecurity"
	"github.com/paularlott/scriptling/object"
)

// TestTransportNoneBlocksAllLoadingAndUnloading proves every way to change a
// TransportNone scope's plugin set fails: LoadPlugin, LoadPath, LoadURL,
// LoadPlugins (the batch path used for directory scans), and Unload.
func TestTransportNoneBlocksAllLoadingAndUnloading(t *testing.T) {
	echo := object.NewFunctionBuilder()
	echo.Function(func(v any) any { return v })
	server := NewServer("noneload", "1.0.0", "transport-none demo").RegisterFunc("echo", echo)
	srv := httptestServer(t, server)
	dir := t.TempDir()
	helper := writeEnvProbeHelper(t, dir)

	none := NewManager(nil).NewScope(WithTransport(TransportNone))
	defer none.Close()
	ctx := context.Background()

	assertDisabled := func(t *testing.T, err error) {
		t.Helper()
		if err == nil || !strings.Contains(err.Error(), "disabled") {
			t.Fatalf("expected a 'disabled' error, got: %v", err)
		}
	}

	if _, err := none.LoadPlugin(ctx, helper, nil); true {
		assertDisabled(t, err)
	}
	if _, err := none.LoadPath(ctx, "x", helper, false, nil); true {
		assertDisabled(t, err)
	}
	if _, err := none.LoadPath(ctx, "y", srv.URL, false, nil); true {
		assertDisabled(t, err)
	}
	if _, err := none.LoadURL(ctx, "z", srv.URL, false, false); true {
		assertDisabled(t, err)
	}
	assertDisabled(t, none.LoadPlugins(ctx, []PluginSpec{{Path: helper}}))
	assertDisabled(t, none.LoadPlugins(ctx, []PluginSpec{{Path: srv.URL}}))

	// Load() (directory discovery) treats a per-plugin start failure as a
	// warning, not a returned error — matching every other startOne failure
	// reason (a bad executable, a naming conflict, etc.), so it returns nil
	// here too. What matters is that nothing actually got loaded.
	none.AddDir(dir)
	if err := none.Load(ctx); err != nil {
		t.Fatalf("Load() should warn, not fail, on a disabled scope: %v", err)
	}
	warned := false
	for _, w := range none.Warnings() {
		if strings.Contains(w, "disabled") {
			warned = true
		}
	}
	if !warned {
		t.Fatalf("expected a warning mentioning 'disabled', got: %v", none.Warnings())
	}

	assertDisabled(t, none.Unload("anything"))

	if len(none.List()) != 0 {
		t.Fatalf("expected nothing to have loaded, got: %v", none.List())
	}
}

// TestTransportNoneStillExposesParentPlugins proves the point of
// TransportNone: plugins the admin loaded on an unrestricted parent manager
// stay fully visible and callable — list, describe, and call_function all
// keep working — through a child scope that itself can load or unload
// nothing.
func TestTransportNoneStillExposesParentPlugins(t *testing.T) {
	echo := object.NewFunctionBuilder()
	echo.Function(func(v any) any { return v })
	server := NewServer("trusted", "1.0.0", "admin-preloaded demo").RegisterFunc("echo", echo)
	srv := httptestServer(t, server)

	parent := NewManager(nil)
	defer parent.Close()
	if _, err := parent.LoadURL(context.Background(), "trusted", srv.URL, false, false); err != nil {
		t.Fatalf("parent LoadURL: %v", err)
	}

	scope := parent.NewScope(WithTransport(TransportNone))
	defer scope.Close()

	client, ok := scope.Get("plugin.trusted")
	if !ok {
		t.Fatal("plugin.trusted not visible through the TransportNone scope")
	}
	result, err := client.CallFunction(context.Background(), "echo",
		[]Value{{Type: valueString, Value: "still-works"}}, nil)
	if err != nil {
		t.Fatalf("CallFunction through TransportNone scope: %v", err)
	}
	if result.Type != valueString || result.Value != "still-works" {
		t.Fatalf("unexpected result: %#v", result)
	}

	found := false
	for _, meta := range scope.List() {
		if meta.Name == "plugin.trusted" {
			found = true
		}
	}
	if !found {
		t.Fatalf("List() through TransportNone scope did not include the parent's plugin: %v", scope.List())
	}

	// The scope cannot unload the parent's plugin either.
	if err := scope.Unload("trusted"); err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("expected unload to be disabled, got: %v", err)
	}
	if _, ok := parent.Get("plugin.trusted"); !ok {
		t.Fatal("parent's plugin should be unaffected by the scope's disabled unload")
	}
}

// recordingTransport wraps http.Transport's dialer to count how many times
// it was actually asked to dial, so tests can prove a given transport (and
// not some other one) is the one actually used.
type recordingTransport struct {
	*http.Transport
	dials int
}

func newRecordingTransport(fail bool) *recordingTransport {
	rt := &recordingTransport{}
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	rt.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // test-only
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			rt.dials++
			if fail {
				return nil, errors.New("recordingTransport: dial refused by test double")
			}
			return dialer.DialContext(ctx, network, addr)
		},
	}
	return rt
}

// TestWithHTTPTransportIsUsedForCalls proves a scope's injected transport —
// not the Manager's own default pooled transport — is what an HTTP(S)
// plugin's calls actually dial through. LoadURL itself doesn't dial for a
// non-handshake (scriptling=false) peer — only call_function does — so this
// exercises the transport via CallFunction, which every real script call
// goes through: a deliberately-refusing transport fails the call, and the
// working one succeeds and shows a nonzero dial count.
func TestWithHTTPTransportIsUsedForCalls(t *testing.T) {
	echo := object.NewFunctionBuilder()
	echo.Function(func(v any) any { return v })
	server := NewServer("guarded", "1.0.0", "guarded transport demo").RegisterFunc("echo", echo)
	srv := httptestServer(t, server)

	refusing := newRecordingTransport(true)
	blocked := NewManager(nil).NewScope(WithTransport(TransportAll), WithHTTPTransport(refusing.Transport))
	defer blocked.Close()
	blockedClient, err := blocked.LoadURL(context.Background(), "guarded", srv.URL, false, false)
	if err != nil {
		t.Fatalf("LoadURL (no handshake, should not dial): %v", err)
	}
	if _, err := blockedClient.CallFunction(context.Background(), "echo",
		[]Value{{Type: valueString, Value: "x"}}, nil); err == nil {
		t.Fatal("expected the refusing transport to fail the call")
	}
	if refusing.dials == 0 {
		t.Fatal("expected the injected refusing transport's DialContext to have been invoked at least once")
	}

	working := newRecordingTransport(false)
	allowed := NewManager(nil).NewScope(WithTransport(TransportAll), WithHTTPTransport(working.Transport))
	defer allowed.Close()
	client, err := allowed.LoadURL(context.Background(), "guarded", srv.URL, false, false)
	if err != nil {
		t.Fatalf("LoadURL with working injected transport: %v", err)
	}
	result, err := client.CallFunction(context.Background(), "echo",
		[]Value{{Type: valueString, Value: "works"}}, nil)
	if err != nil {
		t.Fatalf("CallFunction with working injected transport: %v", err)
	}
	if result.Type != valueString || result.Value != "works" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if working.dials == 0 {
		t.Fatal("expected the injected working transport's DialContext to have been invoked")
	}
}

// TestWithHTTPTransportAppliesToInsecureRequestsToo proves a script asking
// for insecure_skip_tls=True still goes through the injected transport
// rather than the Manager's own default insecure transport — the injected
// transport is a one-way ratchet, never bypassable by that flag.
func TestWithHTTPTransportAppliesToInsecureRequestsToo(t *testing.T) {
	echo := object.NewFunctionBuilder()
	echo.Function(func(v any) any { return v })
	server := NewServer("insecuredemo", "1.0.0", "insecure demo").RegisterFunc("echo", echo)
	srv := httptestServer(t, server)

	refusing := newRecordingTransport(true)
	scope := NewManager(nil).NewScope(WithTransport(TransportAll), WithHTTPTransport(refusing.Transport))
	defer scope.Close()

	// insecureSkipTLS=true would normally select the Manager's separate
	// httpInsecureTransport; with WithHTTPTransport both slots point at the
	// same injected transport, so a call must still fail through it.
	client, err := scope.LoadURL(context.Background(), "insecuredemo", srv.URL, false, true)
	if err != nil {
		t.Fatalf("LoadURL (no handshake, should not dial): %v", err)
	}
	if _, err := client.CallFunction(context.Background(), "echo",
		[]Value{{Type: valueString, Value: "x"}}, nil); err == nil {
		t.Fatal("expected the injected transport to govern the insecure-skip-tls path too")
	}
	if refusing.dials == 0 {
		t.Fatal("expected the injected transport to have been used even for the insecure request")
	}
}

// TestWithHTTPTransportEnforcesARealNetworkPolicy proves the pattern
// documented for embedders — WithHTTPTransport(guard.HTTPClient().Transport)
// — actually enforces a real netsecurity policy for plugin HTTP calls, not
// just a hand-rolled test double: the same host is reachable when the
// policy allows it and blocked when it doesn't, through the exact guard
// construction requests/scriptling.ai/scriptling.mcp use.
func TestWithHTTPTransportEnforcesARealNetworkPolicy(t *testing.T) {
	echo := object.NewFunctionBuilder()
	echo.Function(func(v any) any { return v })
	server := NewServer("policydemo", "1.0.0", "real network policy demo").RegisterFunc("echo", echo)
	srv := httptestServer(t, server) // httptest always serves on loopback

	deniedGuard, err := netsecurity.NewGuard(&netsecurity.Config{})
	if err != nil {
		t.Fatalf("NewGuard: %v", err)
	}
	denied := NewManager(nil).NewScope(WithTransport(TransportHTTP), WithHTTPTransport(deniedGuard.HTTPClient().Transport))
	defer denied.Close()
	deniedClient, err := denied.LoadURL(context.Background(), "policydemo", srv.URL, false, false)
	if err != nil {
		t.Fatalf("LoadURL (no handshake, should not dial): %v", err)
	}
	if _, err := deniedClient.CallFunction(context.Background(), "echo",
		[]Value{{Type: valueString, Value: "x"}}, nil); err == nil || !strings.Contains(err.Error(), "network policy") {
		t.Fatalf("expected the real guard's default policy to deny this address, got: %v", err)
	}

	// httptest always serves on a loopback IP literal, so both categories
	// need lifting to actually connect — matching real production use,
	// where a host would grant an address via allow_cidrs/allow_hosts
	// instead of these two narrow flags.
	allowedGuard, err := netsecurity.NewGuard(&netsecurity.Config{AllowLoopback: true, AllowIPLiterals: true})
	if err != nil {
		t.Fatalf("NewGuard: %v", err)
	}
	allowed := NewManager(nil).NewScope(WithTransport(TransportHTTP), WithHTTPTransport(allowedGuard.HTTPClient().Transport))
	defer allowed.Close()
	allowedClient, err := allowed.LoadURL(context.Background(), "policydemo", srv.URL, false, false)
	if err != nil {
		t.Fatalf("LoadURL: %v", err)
	}
	result, err := allowedClient.CallFunction(context.Background(), "echo",
		[]Value{{Type: valueString, Value: "policy-allowed"}}, nil)
	if err != nil {
		t.Fatalf("CallFunction with AllowLoopback: %v", err)
	}
	if result.Type != valueString || result.Value != "policy-allowed" {
		t.Fatalf("unexpected result: %#v", result)
	}

	// Stdio/exec loading stays impossible no matter what the network policy
	// says — TransportHTTP never re-opens it.
	helper := writeEnvProbeHelper(t, t.TempDir())
	if _, err := allowed.LoadPath(context.Background(), "x", helper, false, nil); err == nil ||
		!strings.Contains(err.Error(), "not permitted") {
		t.Fatalf("expected stdio loading to stay refused under TransportHTTP, got: %v", err)
	}
}
