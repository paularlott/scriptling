package plugin

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/fssecurity"
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

// TestTransportNoneBlocksMultiSpecBatch proves the check applies uniformly
// across every spec in one LoadPlugins call, not just a single-spec case —
// a mixed stdio+HTTP batch on a TransportNone scope must load nothing at
// all rather than silently accepting some specs.
func TestTransportNoneBlocksMultiSpecBatch(t *testing.T) {
	echo := object.NewFunctionBuilder()
	echo.Function(func(v any) any { return v })
	server := NewServer("batchdemo", "1.0.0", "batch demo").RegisterFunc("echo", echo)
	srv := httptestServer(t, server)
	dir := t.TempDir()
	helper := writeEnvProbeHelper(t, dir)

	none := NewManager(nil).NewScope(WithTransport(TransportNone))
	defer none.Close()

	err := none.LoadPlugins(context.Background(), []PluginSpec{
		{Path: helper},
		{Path: srv.URL},
	})
	if err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("expected a 'disabled' error for the batch, got: %v", err)
	}
	if len(none.List()) != 0 {
		t.Fatalf("expected nothing from the batch to have loaded, got: %v", none.List())
	}
}

// TestTransportNoneHoldsUnderConcurrentAccess hammers Load/LoadPlugin/
// LoadURL/Unload from many goroutines at once on a single TransportNone
// scope, run under -race: proves the check is not just correct in the
// single-threaded case but genuinely race-free and never lets a single
// attempt slip through under contention, while a concurrently-running
// legitimate caller against the (separate, unrestricted) parent manager is
// unaffected.
func TestTransportNoneHoldsUnderConcurrentAccess(t *testing.T) {
	echo := object.NewFunctionBuilder()
	echo.Function(func(v any) any { return v })
	server := NewServer("racedemo", "1.0.0", "race demo").RegisterFunc("echo", echo)
	srv := httptestServer(t, server)
	dir := t.TempDir()
	helper := writeEnvProbeHelper(t, dir)

	parent := NewManager(nil)
	defer parent.Close()
	if _, err := parent.LoadURL(context.Background(), "trusted", srv.URL, false, false); err != nil {
		t.Fatalf("parent LoadURL: %v", err)
	}

	none := parent.NewScope(WithTransport(TransportNone))
	defer none.Close()

	const goroutines = 50
	var wg sync.WaitGroup
	errs := make(chan error, goroutines*4)
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			if _, err := none.LoadPlugin(context.Background(), helper, nil); err == nil {
				errs <- fmt.Errorf("goroutine %d: LoadPlugin unexpectedly succeeded", n)
			}
			if _, err := none.LoadURL(context.Background(), fmt.Sprintf("x%d", n), srv.URL, false, false); err == nil {
				errs <- fmt.Errorf("goroutine %d: LoadURL unexpectedly succeeded", n)
			}
			if err := none.Unload("trusted"); err == nil {
				errs <- fmt.Errorf("goroutine %d: Unload unexpectedly succeeded", n)
			}
			// The scope itself sees the parent's plugin throughout.
			if _, ok := none.Get("plugin.trusted"); !ok {
				errs <- fmt.Errorf("goroutine %d: lost sight of the parent's plugin mid-race", n)
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}

	// none.List() legitimately still shows "trusted" — that's the parent's
	// plugin via the documented chain lookup, not something the scope
	// loaded itself. What matters is that it's still exactly one entry (no
	// goroutine's failed LoadPlugin/LoadURL slipped a second one in) and
	// the parent's own copy is untouched by the scope's failed Unload calls.
	if got := none.List(); len(got) != 1 || got[0].Name != "plugin.trusted" {
		t.Fatalf("expected only the parent's plugin visible after the race, got: %v", got)
	}
	if _, ok := parent.Get("plugin.trusted"); !ok {
		t.Fatal("parent's plugin should still be present and untouched by the scope's failed attempts")
	}
}

// TestTransportNoneExposesBothDiskAndHTTPPreloadedPlugins closes a real gap:
// every prior TransportNone test pre-loaded only an HTTP(S) plugin on the
// parent. Startup pre-loading commonly mixes both a directory of stdio
// executables and specific HTTP(S) endpoints (mirroring the CLI's
// --plugin-dir plus --plugin) — this proves both kinds survive identically
// through a TransportNone scope: listed, callable, and never load/unload-able.
func TestTransportNoneExposesBothDiskAndHTTPPreloadedPlugins(t *testing.T) {
	echoHTTP := object.NewFunctionBuilder()
	echoHTTP.Function(func(v any) any { return v })
	server := NewServer("httppreload", "1.0.0", "http preload demo").RegisterFunc("echo", echoHTTP)
	srv := httptestServer(t, server)
	helper := writeEnvProbeHelper(t, t.TempDir())

	parent := NewManager(nil)
	defer parent.Close()
	if _, err := parent.LoadURL(context.Background(), "httppreload", srv.URL, false, false); err != nil {
		t.Fatalf("parent LoadURL (http preload): %v", err)
	}
	if _, err := parent.LoadPlugin(context.Background(), helper, nil); err != nil {
		t.Fatalf("parent LoadPlugin (disk preload): %v", err)
	}

	scope := parent.NewScope(WithTransport(TransportNone))
	defer scope.Close()

	names := map[string]bool{}
	for _, meta := range scope.List() {
		names[meta.Name] = true
	}
	if !names["plugin.httppreload"] || !names["plugin.envprobe"] {
		t.Fatalf("expected both disk- and HTTP-preloaded plugins visible, got: %v", names)
	}

	httpClient, ok := scope.Get("plugin.httppreload")
	if !ok {
		t.Fatal("plugin.httppreload not reachable through the scope")
	}
	if _, err := httpClient.CallFunction(context.Background(), "echo",
		[]Value{{Type: valueString, Value: "disk-and-http"}}, nil); err != nil {
		t.Fatalf("CallFunction on the HTTP-preloaded plugin: %v", err)
	}

	diskClient, ok := scope.Get("plugin.envprobe")
	if !ok {
		t.Fatal("plugin.envprobe not reachable through the scope")
	}
	if _, err := diskClient.CallFunction(context.Background(), "env",
		[]Value{{Type: valueString, Value: "PATH"}}, nil); err != nil {
		t.Fatalf("CallFunction on the disk-preloaded plugin: %v", err)
	}

	// The scope still can't load a third plugin, nor unload either preloaded one.
	if _, err := scope.LoadPlugin(context.Background(), helper, nil); err == nil {
		t.Fatal("expected LoadPlugin to stay disabled even with plugins already preloaded")
	}
	if err := scope.Unload("httppreload"); err == nil {
		t.Fatal("expected Unload of the HTTP-preloaded plugin to stay disabled")
	}
	if err := scope.Unload("envprobe"); err == nil {
		t.Fatal("expected Unload of the disk-preloaded plugin to stay disabled")
	}
}

// TestPreloadedPluginsSurviveAlongsideHTTPOnlyLoading closes the second gap:
// no prior test combined "the host preloaded trusted plugins at startup"
// with "the host also lets scripts load *new* HTTP(S) plugins under a
// network policy" on the same scope. Proves both halves hold together: the
// startup-preloaded plugin (loaded via disk, matching --plugin-dir) stays
// fully usable, a new HTTP(S) load the policy allows succeeds, one the
// policy denies fails, and stdio loading remains impossible throughout even
// though the preloaded plugin itself came from disk.
func TestPreloadedPluginsSurviveAlongsideHTTPOnlyLoading(t *testing.T) {
	echo := object.NewFunctionBuilder()
	echo.Function(func(v any) any { return v })
	srv := httptestServer(t, NewServer("remote", "1.0.0", "remote demo").RegisterFunc("echo", echo))

	helper := writeEnvProbeHelper(t, t.TempDir())

	// Startup: preload a trusted plugin from disk on the unrestricted parent.
	parent := NewManager(nil)
	defer parent.Close()
	if _, err := parent.LoadPlugin(context.Background(), helper, nil); err != nil {
		t.Fatalf("parent LoadPlugin (startup disk preload): %v", err)
	}

	// httptest always serves on loopback, so both the "allowed" and "denied"
	// cases below necessarily target the exact same address — the two guards
	// differ only in whether loopback/IP-literal access is granted at all,
	// which is enough to prove the policy is actually consulted rather than
	// both cases coincidentally passing or failing the same blanket check.
	deniedGuard, err := netsecurity.NewGuard(&netsecurity.Config{})
	if err != nil {
		t.Fatalf("NewGuard (denied): %v", err)
	}
	allowedGuard, err := netsecurity.NewGuard(&netsecurity.Config{AllowLoopback: true, AllowIPLiterals: true})
	if err != nil {
		t.Fatalf("NewGuard (allowed): %v", err)
	}

	deniedScope := parent.NewScope(WithTransport(TransportHTTP), WithHTTPTransport(deniedGuard.HTTPClient().Transport))
	defer deniedScope.Close()
	allowedScope := parent.NewScope(WithTransport(TransportHTTP), WithHTTPTransport(allowedGuard.HTTPClient().Transport))
	defer allowedScope.Close()

	// The startup-preloaded plugin is fully usable through *both* scopes —
	// it's the parent's, inherited via the chain regardless of which HTTP
	// policy this particular scope enforces for anything new.
	for _, scope := range []*Manager{deniedScope, allowedScope} {
		diskClient, ok := scope.Get("plugin.envprobe")
		if !ok {
			t.Fatal("plugin.envprobe (startup preload) not visible on the HTTP-loading scope")
		}
		if _, err := diskClient.CallFunction(context.Background(), "env",
			[]Value{{Type: valueString, Value: "PATH"}}, nil); err != nil {
			t.Fatalf("CallFunction on the startup-preloaded plugin: %v", err)
		}
	}

	// A new HTTP(S) load the policy allows succeeds.
	allowedClient, err := allowedScope.LoadURL(context.Background(), "remote", srv.URL, false, false)
	if err != nil {
		t.Fatalf("LoadURL (no handshake, should not dial yet): %v", err)
	}
	if _, err := allowedClient.CallFunction(context.Background(), "echo",
		[]Value{{Type: valueString, Value: "ok"}}, nil); err != nil {
		t.Fatalf("CallFunction on the policy-allowed new load: %v", err)
	}

	// The identical new HTTP(S) load fails under the denying policy.
	deniedClient, err := deniedScope.LoadURL(context.Background(), "remote", srv.URL, false, false)
	if err != nil {
		t.Fatalf("LoadURL (no handshake, should not dial yet): %v", err)
	}
	if _, err := deniedClient.CallFunction(context.Background(), "echo",
		[]Value{{Type: valueString, Value: "x"}}, nil); err == nil || !strings.Contains(err.Error(), "network policy") {
		t.Fatalf("expected the policy-denied load's call to fail with a network policy error, got: %v", err)
	}

	// Stdio loading is still impossible on either scope, even though the
	// preloaded plugin itself is a stdio executable and TransportHTTP is
	// active.
	scope := deniedScope
	if _, err := scope.LoadPlugin(context.Background(), helper, nil); err == nil || !strings.Contains(err.Error(), "not permitted") {
		t.Fatalf("expected stdio loading to stay refused, got: %v", err)
	}
}

// TestNestedScopeInheritsTransportModeAndExecPaths is a regression test for
// a real bug: NewScope used to leave transportMode (and now execPaths) at
// their zero values regardless of the parent's restriction, so a nested
// scope created with no options was silently *more* permissive than its
// parent — the exact opposite of what scoping down should mean. Both fields
// must now be inherited by default, while an explicit option on the child
// still overrides.
func TestNestedScopeInheritsTransportModeAndExecPaths(t *testing.T) {
	helper := writeEnvProbeHelper(t, t.TempDir())

	restricted := NewManager(nil).NewScope(
		WithTransport(TransportHTTP),
		WithExecPaths(&fssecurity.Config{AllowedPaths: []string{}}), // deny-all
	)
	defer restricted.Close()

	// A child created with no options inherits both restrictions.
	inheriting := restricted.NewScope()
	defer inheriting.Close()
	if _, err := inheriting.LoadPlugin(context.Background(), helper, nil); err == nil {
		t.Fatal("expected the inheriting child to stay stdio-refused (TransportHTTP inherited)")
	}

	// A child can still explicitly loosen its own transport mode...
	reopened := restricted.NewScope(WithTransport(TransportAll))
	defer reopened.Close()
	// ...but execPaths (deny-all, inherited) still blocks the actual spawn,
	// proving the two restrictions are independent knobs.
	if _, err := reopened.LoadPlugin(context.Background(), helper, nil); err == nil ||
		!strings.Contains(err.Error(), "not in the allowed paths") {
		t.Fatalf("expected the deny-all execPaths to still block the load, got: %v", err)
	}
}

// TestWithExecPathsRestrictsDynamicStdioLoading proves the actual new
// feature: a scope can permit stdio/exec loading, but only for executables
// under specific paths — the exec-side counterpart to WithHTTPTransport.
// The unrestricted parent (representing boot-time preloading by the host
// application itself) can still load from anywhere, regardless of what its
// script-facing child scope allows.
func TestWithExecPathsRestrictsDynamicStdioLoading(t *testing.T) {
	allowedDir := t.TempDir()
	allowedHelper := writeEnvProbeHelper(t, allowedDir)
	outsideHelper := writeEnvProbeHelper(t, t.TempDir())

	parent := NewManager(nil)
	defer parent.Close()
	// Boot-time: the host application preloads from anywhere, unrestricted.
	if _, err := parent.LoadPlugin(context.Background(), outsideHelper, nil); err != nil {
		t.Fatalf("unrestricted parent LoadPlugin: %v", err)
	}

	scope := parent.NewScope(
		WithTransport(TransportAll),
		WithExecPaths(&fssecurity.Config{AllowedPaths: []string{allowedDir}}),
	)
	defer scope.Close()

	// The boot-time preloaded plugin (outside the allowlist) is still fully
	// usable through the scope — execPaths only gates *new* loads.
	if _, ok := scope.Get("plugin.envprobe"); !ok {
		t.Fatal("expected the boot-time preloaded plugin to remain visible")
	}

	// A new load from the allowed directory succeeds.
	name := "allowed-" + filepath.Base(allowedDir)
	if _, err := scope.LoadPath(context.Background(), name, allowedHelper, false, nil); err != nil {
		t.Fatalf("LoadPath from an allowed path: %v", err)
	}

	// A new load from outside the allowed directory fails, even though the
	// exact same binary already loaded fine on the unrestricted parent.
	if _, err := scope.LoadPath(context.Background(), "outside", outsideHelper, false, nil); err == nil ||
		!strings.Contains(err.Error(), "not in the allowed paths") {
		t.Fatalf("expected a path outside the allowlist to be refused, got: %v", err)
	}
}

// TestWithExecPathsCombinesWithHTTPTransport proves both restrictions can
// be active on the same scope simultaneously and are independent: stdio
// loading is allowed but only from a specific directory, and HTTP(S)
// loading is allowed but only within a real network policy, at once.
func TestWithExecPathsCombinesWithHTTPTransport(t *testing.T) {
	allowedDir := t.TempDir()
	allowedHelper := writeEnvProbeHelper(t, allowedDir)
	outsideHelper := writeEnvProbeHelper(t, t.TempDir())

	echo := object.NewFunctionBuilder()
	echo.Function(func(v any) any { return v })
	srv := httptestServer(t, NewServer("combodemo", "1.0.0", "combined restriction demo").RegisterFunc("echo", echo))

	guard, err := netsecurity.NewGuard(&netsecurity.Config{})
	if err != nil {
		t.Fatalf("NewGuard: %v", err)
	}

	scope := NewManager(nil).NewScope(
		WithTransport(TransportAll),
		WithExecPaths(&fssecurity.Config{AllowedPaths: []string{allowedDir}}),
		WithHTTPTransport(guard.HTTPClient().Transport),
	)
	defer scope.Close()

	// Stdio: allowed path succeeds, disallowed path fails.
	if _, err := scope.LoadPath(context.Background(), "allowed", allowedHelper, false, nil); err != nil {
		t.Fatalf("LoadPath from an allowed path: %v", err)
	}
	if _, err := scope.LoadPath(context.Background(), "outside", outsideHelper, false, nil); err == nil ||
		!strings.Contains(err.Error(), "not in the allowed paths") {
		t.Fatalf("expected a path outside the allowlist to be refused, got: %v", err)
	}

	// HTTP: the default (nothing allowed) guard still blocks this loopback call.
	httpClient, err := scope.LoadURL(context.Background(), "combodemo", srv.URL, false, false)
	if err != nil {
		t.Fatalf("LoadURL (no handshake, should not dial yet): %v", err)
	}
	if _, err := httpClient.CallFunction(context.Background(), "echo",
		[]Value{{Type: valueString, Value: "x"}}, nil); err == nil || !strings.Contains(err.Error(), "network policy") {
		t.Fatalf("expected the network policy to still block this call, got: %v", err)
	}
}

// TestWithExecPathsScriptLevel proves the WithExecPaths doc example works
// through the actual scriptling.plugin control library, not just the Go
// Manager API: a startup-preloaded plugin stays fully usable from a script,
// a script-initiated load() from inside the allowlist succeeds, and one
// from outside it fails, exactly as host-integration.md documents.
func TestWithExecPathsScriptLevel(t *testing.T) {
	allowedDir := t.TempDir()
	allowedHelper := filepath.Join(allowedDir, "loader")
	writeScriptlingHelper(t, allowedHelper)

	outsideDir := t.TempDir()
	outsideHelper := filepath.Join(outsideDir, "loader")
	writeScriptlingHelper(t, outsideHelper)

	manager := NewManager(nil)
	defer manager.Close()
	if _, err := manager.LoadPlugin(context.Background(), outsideHelper, nil); err != nil {
		t.Fatalf("unrestricted startup LoadPlugin: %v", err)
	}

	scope := manager.NewScope(WithExecPaths(&fssecurity.Config{AllowedPaths: []string{allowedDir}}))
	defer scope.Close()

	p := scriptling.New()
	RegisterLibraries(p, scope)

	t.Run("preloaded_plugin_still_usable", func(t *testing.T) {
		result, err := p.Eval(`
import scriptling.plugin
scriptling.plugin.call_function("declared", "add", 18, 24)
`)
		if err != nil {
			t.Fatalf("Eval: %v", err)
		}
		if i, ok := result.(*object.Integer); !ok || i.IntValue() != 42 {
			t.Fatalf("expected int 42, got %#v", result)
		}
	})

	t.Run("load_from_allowed_dir_succeeds", func(t *testing.T) {
		result, err := p.Eval(`
import scriptling.plugin
scriptling.plugin.load("extra", ` + strconv.Quote(allowedHelper) + `, scriptling=True)
`)
		if err != nil {
			t.Fatalf("Eval: %v", err)
		}
		if s, ok := result.(*object.String); !ok || s.StringValue() != "plugin.extra" {
			t.Fatalf("expected plugin.extra, got %#v", result)
		}
	})

	t.Run("load_from_outside_allowed_dir_fails", func(t *testing.T) {
		_, err := p.Eval(`
import scriptling.plugin
scriptling.plugin.load("evil", ` + strconv.Quote(outsideHelper) + `, scriptling=True)
`)
		if err == nil || !strings.Contains(err.Error(), "not in the allowed paths") {
			t.Fatalf("expected a not-in-the-allowed-paths error, got: %v", err)
		}
	})
}
