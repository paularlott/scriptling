package plugin

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"strings"
	"sync"
	"testing"

	"github.com/paularlott/jsonrpc"
	"github.com/paularlott/scriptling/extlibs/netsecurity"
	"github.com/paularlott/scriptling/object"
)

// policyPipeServer runs a Server over in-process pipes and connects a Client
// that carries the given policy in its handshake, mirroring the stdio
// transport (LoadClientFromIO cannot be used because it handshakes before a
// policy could be attached).
func policyPipeServer(t *testing.T, server *Server, policy *Policy) *Client {
	t.Helper()
	clientIn, serverOut := io.Pipe()
	serverIn, clientOut := io.Pipe()
	go func() { _ = server.RunIO(serverIn, serverOut) }()
	client := &Client{
		path:           "<pipe>",
		policy:         policy,
		callbackOwners: make(map[string]*callbackOwner),
		done:           make(chan struct{}),
	}
	client.peer = jsonrpc.NewPeer(clientIn, clientOut, newPluginPeerServer(client), jsonrpc.WithPeerCloseFunc(func() error {
		return clientOut.Close()
	}))
	go func() { _ = client.peer.Serve() }()
	go func() {
		<-client.peer.Done()
		client.markDone()
	}()
	if err := client.handshake(context.Background()); err != nil {
		t.Fatalf("handshake with policy: %v", err)
	}
	return client
}

func TestHandshakeDeliversPolicyToServer(t *testing.T) {
	server := NewServer("policycheck", "1.0.0", "policy test")
	policy := &Policy{
		AllowedPaths: []string{"/tmp/db"},
		Network: &NetworkPolicy{
			AllowPrivateIPs: true,
			AllowHosts:      []string{"db.internal"},
		},
	}
	client := policyPipeServer(t, server, policy)
	defer client.Close()

	got := server.Policy()
	if got == nil {
		t.Fatal("expected server to receive a policy")
	}
	if len(got.AllowedPaths) != 1 || got.AllowedPaths[0] != "/tmp/db" {
		t.Fatalf("allowed paths not delivered: %#v", got.AllowedPaths)
	}
	if got.Network == nil {
		t.Fatal("network policy not delivered")
	}
	if !got.Network.AllowPrivateIPs || len(got.Network.AllowHosts) != 1 || got.Network.AllowHosts[0] != "db.internal" {
		t.Fatalf("network policy fields not delivered: %#v", got.Network)
	}
}

func TestHandshakeWithoutPolicyLeavesServerUnrestricted(t *testing.T) {
	server := NewServer("policyfree", "1.0.0", "no policy test")
	client := policyPipeServer(t, server, nil)
	defer client.Close()

	if got := server.Policy(); got != nil {
		t.Fatalf("expected nil policy, got %#v", got)
	}
}

func TestHandshakeLegacyParamsWithoutPolicy(t *testing.T) {
	// A host that predates the policy block sends the old params shape. The
	// server must accept the handshake and keep a nil policy.
	server := NewServer("legacy", "1.0.0", "legacy handshake test")
	raw := json.RawMessage(`{"protocol":"1.0","host":"scriptling","host_version":"0.0","transports":["json"],"capabilities":["remote_objects"]}`)
	var params any
	if err := json.Unmarshal(raw, &params); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, err := server.dispatch(context.Background(), "scriptling.handshake", params); err != nil {
		t.Fatalf("dispatch handshake: %v", err)
	}
	if got := server.Policy(); got != nil {
		t.Fatalf("expected nil policy from legacy params, got %#v", got)
	}
}

func TestServerAdvertisesPolicyCapability(t *testing.T) {
	server := NewServer("capped", "1.0.0", "capability test")
	client := policyPipeServer(t, server, nil)
	defer client.Close()

	found := false
	for _, capability := range client.Metadata().Capabilities {
		if capability == "policy" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected policy capability, got %#v", client.Metadata().Capabilities)
	}
}

func TestManagerPolicySnapshotWalksParentChain(t *testing.T) {
	root := NewManager(nil)
	if got := root.policySnapshot(); got != nil {
		t.Fatalf("fresh manager policy must be nil, got %#v", got)
	}
	policy := &Policy{AllowedPaths: []string{"/data"}}
	root.SetPolicy(policy)
	if got := root.policySnapshot(); got != policy {
		t.Fatalf("snapshot after SetPolicy wrong: %#v", got)
	}
	scope := root.NewScope()
	if got := scope.policySnapshot(); got != policy {
		t.Fatal("scope must inherit parent policy")
	}
}

func TestPolicyPathAllowed(t *testing.T) {
	var unrestricted *Policy
	if !unrestricted.PathAllowed("/etc/passwd") {
		t.Fatal("nil policy must allow everything")
	}

	empty := &Policy{}
	if !empty.PathAllowed("/anywhere/x.db") {
		t.Fatal("nil AllowedPaths must mean unrestricted")
	}

	restricted := &Policy{AllowedPaths: []string{"/tmp/db"}}
	if restricted.PathAllowed("/etc/app.db") {
		t.Fatal("path outside allowed roots must be denied")
	}
	if !restricted.PathAllowed("/tmp/db/app.db") {
		t.Fatal("path inside allowed root must be allowed")
	}
}

func TestPolicyGuardReflectsNetworkPolicy(t *testing.T) {
	var unrestricted *Policy
	guard, err := unrestricted.Guard()
	if err != nil {
		t.Fatalf("nil policy guard: %v", err)
	}
	if guard != nil {
		t.Fatal("nil policy must yield a nil guard")
	}

	restricted := &Policy{Network: &NetworkPolicy{AllowHosts: []string{"db.example.com"}}}
	guard, err = restricted.Guard()
	if err != nil {
		t.Fatalf("guard: %v", err)
	}
	if guard == nil {
		t.Fatal("network policy must yield a guard")
	}
	// The guard dials validated IPs only; loopback is denied unless allowed.
	if _, dialErr := guard.DialContext(context.Background(), "tcp", "127.0.0.1:1"); dialErr == nil {
		t.Fatal("loopback must be denied unless allowed")
	}

	allowing := &Policy{Network: &NetworkPolicy{AllowLoopback: true}}
	guard, err = allowing.Guard()
	if err != nil {
		t.Fatalf("guard: %v", err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()
	conn, dialErr := guard.DialContext(context.Background(), "tcp", listener.Addr().String())
	if dialErr != nil {
		t.Fatalf("loopback must dial cleanly when allowed: %v", dialErr)
	}
	_ = conn.Close()
}

func TestPolicyFromSecurity(t *testing.T) {
	if got := PolicyFromSecurity(nil, nil); got != nil {
		t.Fatalf("nil config must yield nil policy, got %#v", got)
	}

	pathsOnly := PolicyFromSecurity(nil, []string{"/data"})
	if pathsOnly == nil || len(pathsOnly.AllowedPaths) != 1 || pathsOnly.Network != nil {
		t.Fatalf("paths-only policy wrong: %#v", pathsOnly)
	}

	cfg := &netsecurity.Config{
		RequireHTTPS:    true,
		AllowPrivateIPs: true,
		AllowHosts:      []string{"db.internal"},
		DeniedCIDRs:     []string{"10.0.0.0/8"},
	}
	netOnly := PolicyFromSecurity(cfg, nil)
	if netOnly == nil || netOnly.AllowedPaths != nil || netOnly.Network == nil {
		t.Fatalf("network-only policy wrong: %#v", netOnly)
	}
	if !netOnly.Network.RequireHTTPS || !netOnly.Network.AllowPrivateIPs {
		t.Fatalf("network flags lost: %#v", netOnly.Network)
	}
	if len(netOnly.Network.DeniedCIDRs) != 1 || netOnly.Network.DeniedCIDRs[0] != "10.0.0.0/8" {
		t.Fatalf("cidrs lost: %#v", netOnly.Network.DeniedCIDRs)
	}

	// Round-trip: the guard rebuilt from the wire form enforces the same
	// allow rule as the original config.
	guard, err := netOnly.Guard()
	if err != nil {
		t.Fatalf("guard: %v", err)
	}
	if _, err := guard.DialContext(context.Background(), "tcp", "127.0.0.1:1"); err == nil {
		t.Fatal("loopback must remain denied after round-trip")
	}
}

// ---- compiled-in registry ----

type stubRegistrar struct {
	mu   sync.Mutex
	libs []*object.Library
}

func (s *stubRegistrar) RegisterLibrary(lib *object.Library) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.libs = append(s.libs, lib)
}

func (s *stubRegistrar) lookup(name string) *object.Library {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, lib := range s.libs {
		if lib.Name() == name {
			return lib
		}
	}
	return nil
}

func TestCompiledInRegistration(t *testing.T) {
	restore := snapshotCompiledIn()
	defer restore()

	RegisterCompiledIn("gadget", "test gadget", func(policy *Policy) (*object.Library, string) {
		builder := object.NewLibraryBuilder(NormalizeLibraryName("gadget"), "test gadget")
		builder.Function("ping", func() string { return "pong" })
		return builder.Build(), ""
	})

	registrar := &stubRegistrar{}
	RegisterLibraries(registrar, nil)

	lib := registrar.lookup("plugin.gadget")
	if lib == nil {
		t.Fatalf("compiled-in library not registered under plugin.gadget; got %v", CompiledInNames())
	}
	if lib.Functions()["ping"] == nil {
		t.Fatal("compiled-in function missing")
	}
}

func TestCompiledInTakesPrecedenceOverDiscovered(t *testing.T) {
	restore := snapshotCompiledIn()
	defer restore()

	RegisterCompiledIn("dupe", "compiled-in dupe", func(policy *Policy) (*object.Library, string) {
		builder := object.NewLibraryBuilder(NormalizeLibraryName("dupe"), "compiled-in dupe")
		builder.Function("who", func() string { return "compiled-in" })
		return builder.Build(), ""
	})

	server := NewServer("dupe", "1.0.0", "external dupe")
	fb := object.NewFunctionBuilder()
	fb.Function(func() string { return "external" })
	server.RegisterFunc("who", fb)
	client := policyPipeServer(t, server, nil)
	defer client.Close()

	manager := NewManager(nil)
	manager.mu.Lock()
	manager.clients["plugin.dupe"] = client
	manager.mu.Unlock()

	registrar := &stubRegistrar{}
	RegisterLibraries(registrar, manager)

	lib := registrar.lookup("plugin.dupe")
	if lib == nil {
		t.Fatal("plugin.dupe not registered")
	}
	if lib.Functions()["who"] == nil {
		t.Fatal("compiled-in must win the plugin.dupe namespace")
	}
	warned := false
	for _, warning := range manager.Warnings() {
		if strings.Contains(warning, "takes precedence") {
			warned = true
		}
	}
	if !warned {
		t.Fatalf("expected a precedence warning, got %v", manager.Warnings())
	}
}

// snapshotCompiledIn isolates the compiled-in registry for one test and
// returns a restore func.
func snapshotCompiledIn() func() {
	compiledInMu.Lock()
	prev := make([]compiledInEntry, len(compiledInComps))
	copy(prev, compiledInComps)
	compiledInComps = nil
	compiledInMu.Unlock()
	return func() {
		compiledInMu.Lock()
		compiledInComps = prev
		compiledInMu.Unlock()
	}
}

func TestNormalizeLibraryNameNamespaces(t *testing.T) {
	for _, tt := range []struct{ in, want string }{
		{"widgets", "plugin.widgets"},
		{"knot", "plugin.knot"},
		// dotted names are the author's namespace — used verbatim
		{"scriptling.sqlite", "scriptling.sqlite"},
		{"paul.hello", "paul.hello"},
		{"plugin.hello", "plugin.hello"},
	} {
		if got := NormalizeLibraryName(tt.in); got != tt.want {
			t.Fatalf("NormalizeLibraryName(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
