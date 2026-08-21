package netsecurity

import (
	"context"
	"net"
	"net/url"
	"os"
	"strings"
	"testing"
)

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse %q: %v", raw, err)
	}
	return u
}

func mustGuard(t *testing.T, cfg *Config) *Guard {
	t.Helper()
	g, err := NewGuard(cfg)
	if err != nil {
		t.Fatalf("NewGuard: %v", err)
	}
	return g
}

func TestNewGuardRejectsInvalidConfig(t *testing.T) {
	if _, err := NewGuard(&Config{AllowedCIDRs: []string{"10.0.0.0/8", "not-a-cidr"}}); err == nil {
		t.Error("expected error for invalid AllowedCIDRs")
	}
	if _, err := NewGuard(&Config{DeniedCIDRs: []string{"300.0.0.0/8"}}); err == nil {
		t.Error("expected error for invalid DeniedCIDRs")
	}
	if _, err := NewGuard(&Config{DNSServers: []string{" "}}); err == nil {
		t.Error("expected error for empty DNS server")
	}
}

func TestFailClosedRejectsEverything(t *testing.T) {
	g := FailClosed(errTest("bad cidr"))
	if err := g.CheckURL(mustURL(t, "https://example.com/")); err == nil || !strings.Contains(err.Error(), "invalid configuration") {
		t.Errorf("CheckURL error = %v, want invalid configuration error", err)
	}
	if _, err := g.DialContext(context.Background(), "tcp", "example.com:443"); err == nil {
		t.Error("DialContext should fail on a fail-closed guard")
	}
}

type errTest string

func (e errTest) Error() string { return string(e) }

func TestCheckURLSchemes(t *testing.T) {
	g := mustGuard(t, &Config{})
	if err := g.CheckURL(mustURL(t, "http://example.com/")); err != nil {
		t.Errorf("http allowed by default, got %v", err)
	}
	if err := g.CheckURL(mustURL(t, "ftp://example.com/")); err == nil {
		t.Error("ftp must be rejected")
	}
	ghttps := mustGuard(t, &Config{RequireHTTPS: true})
	if err := ghttps.CheckURL(mustURL(t, "http://example.com/")); err == nil {
		t.Error("RequireHTTPS must reject http")
	}
	if err := ghttps.CheckURL(mustURL(t, "ws://example.com/")); err == nil {
		t.Error("RequireHTTPS must reject ws")
	}
	if err := ghttps.CheckURL(mustURL(t, "https://example.com/")); err != nil {
		t.Errorf("https allowed, got %v", err)
	}
}

func TestCheckURLIPLiterals(t *testing.T) {
	g := mustGuard(t, &Config{})
	if err := g.CheckURL(mustURL(t, "http://8.8.8.8/")); err == nil {
		t.Error("IP literals must be blocked by default")
	}
	gAllow := mustGuard(t, &Config{AllowIPLiterals: true})
	if err := gAllow.CheckURL(mustURL(t, "http://8.8.8.8/")); err != nil {
		t.Errorf("public literal allowed with AllowIPLiterals, got %v", err)
	}
	// A private literal is still blocked by the address categories.
	if err := gAllow.CheckURL(mustURL(t, "http://10.0.0.1/")); err == nil {
		t.Error("private literal must stay blocked")
	}
	// A literal inside AllowedCIDRs is trusted even without the flag.
	gCIDR := mustGuard(t, &Config{AllowedCIDRs: []string{"10.1.2.0/24"}})
	if err := gCIDR.CheckURL(mustURL(t, "http://10.1.2.3/")); err != nil {
		t.Errorf("literal in AllowedCIDRs should pass, got %v", err)
	}
}

func TestCheckURLAddressCategories(t *testing.T) {
	blocked := []string{
		"http://127.0.0.1/",           // loopback
		"http://[::1]/",               // IPv6 loopback
		"http://169.254.169.254/",     // link-local / cloud metadata
		"http://[fe80::1]/",           // IPv6 link-local
		"http://10.0.0.1/",            // private
		"http://192.168.1.1/",         // private
		"http://[fc00::1]/",           // IPv6 unique-local
		"http://0.0.0.0/",             // unspecified
		"http://224.0.0.1/",           // multicast
		"http://::ffff:127.0.0.1:80/", // IPv4-mapped IPv6 (parsed as host)
	}
	g := mustGuard(t, &Config{AllowIPLiterals: true})
	for _, raw := range blocked {
		u, err := url.Parse(raw)
		if err != nil {
			continue
		}
		if err := g.CheckURL(u); err == nil {
			t.Errorf("%s should be blocked", raw)
		}
	}

	gOpen := mustGuard(t, &Config{AllowIPLiterals: true, AllowLoopback: true, AllowPrivateIPs: true})
	if err := gOpen.CheckURL(mustURL(t, "http://127.0.0.1/")); err != nil {
		t.Errorf("loopback opt-in failed: %v", err)
	}
	if err := gOpen.CheckURL(mustURL(t, "http://10.0.0.1/")); err != nil {
		t.Errorf("private opt-in failed: %v", err)
	}
	// Link-local has no toggle: only explicit CIDRs permit it.
	gLL := mustGuard(t, &Config{AllowIPLiterals: true, AllowedCIDRs: []string{"169.254.0.0/16"}})
	if err := gLL.CheckURL(mustURL(t, "http://169.254.169.254/")); err != nil {
		t.Errorf("link-local via AllowedCIDRs should pass, got %v", err)
	}
}

func TestCheckURLHostLists(t *testing.T) {
	g := mustGuard(t, &Config{
		AllowHosts: []string{"api.example.com", ".trusted.org"},
		DenyHosts:  []string{"evil.com", ".bad.io"},
	})
	blocked := []string{
		"https://evil.com/",
		"https://sub.evil.com/", // no: exact rule only matches evil.com — see below
		"https://x.bad.io/",
		"https://other.example.com/", // not in allowlist
	}
	for _, raw := range blocked {
		if raw == "https://sub.evil.com/" {
			continue // exact entries do not match subdomains
		}
		if err := g.CheckURL(mustURL(t, raw)); err == nil {
			t.Errorf("%s should be blocked", raw)
		}
	}
	allowed := []string{
		"https://api.example.com/",
		"https://api.trusted.org/",
		"https://deep.sub.trusted.org/",
	}
	for _, raw := range allowed {
		if err := g.CheckURL(mustURL(t, raw)); err != nil {
			t.Errorf("%s should be allowed: %v", raw, err)
		}
	}

	// Deny always wins over allow.
	gBoth := mustGuard(t, &Config{
		AllowHosts: []string{"evil.com"},
		DenyHosts:  []string{"evil.com"},
	})
	if err := gBoth.CheckURL(mustURL(t, "https://evil.com/")); err == nil {
		t.Error("deny must win over allow")
	}
}

func TestDeniedCIDRWinsOverAllowedCIDR(t *testing.T) {
	g := mustGuard(t, &Config{
		AllowIPLiterals: true,
		AllowedCIDRs:    []string{"10.0.0.0/8"},
		DeniedCIDRs:     []string{"10.66.0.0/16"},
	})
	if err := g.CheckURL(mustURL(t, "http://10.1.2.3/")); err != nil {
		t.Errorf("10.1.2.3 allowed by CIDR, got %v", err)
	}
	if err := g.CheckURL(mustURL(t, "http://10.66.1.1/")); err == nil {
		t.Error("10.66.1.1 must be denied")
	}
}

// DialContext must reject a hostname when ANY resolved address fails policy —
// the mixed-answer (DNS rebinding aid) case — and permit trusted allowlisted
// hosts to resolve anywhere.
func TestDialContextValidatesAllResolvedIPs(t *testing.T) {
	g := mustGuard(t, &Config{})
	g.lookupFn = func(ctx context.Context, host string) ([]net.IPAddr, error) {
		return []net.IPAddr{
			{IP: net.ParseIP("93.184.216.34")}, // public
			{IP: net.ParseIP("10.0.0.5")},      // private — must poison the set
		}, nil
	}
	if _, err := g.DialContext(context.Background(), "tcp", "rebind.example.com:80"); err == nil {
		t.Fatal("mixed public/private answers must be rejected")
	} else if !strings.Contains(err.Error(), "private") {
		t.Errorf("error should name the blocked address, got: %v", err)
	}
}

func TestDialContextTrustedHostBypassesCategories(t *testing.T) {
	g := mustGuard(t, &Config{AllowHosts: []string{"internal.corp"}})
	g.lookupFn = func(ctx context.Context, host string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, nil
	}
	// The trusted host dials the loopback IP directly: the connection to a
	// closed port fails with a dial error, NOT a policy error.
	_, err := g.DialContext(context.Background(), "tcp", "internal.corp:1")
	if err == nil {
		t.Fatal("expected dial error to closed port")
	}
	if strings.Contains(err.Error(), "network policy") {
		t.Errorf("trusted host should not be policy-blocked, got: %v", err)
	}

	// The same resolution for a non-trusted host is policy-blocked.
	if _, err := g.DialContext(context.Background(), "tcp", "other.corp:1"); err == nil || !strings.Contains(err.Error(), "network policy") {
		t.Errorf("untrusted host resolving to loopback must be policy-blocked, got: %v", err)
	}
}

func TestDialContextBlocksLiteralDial(t *testing.T) {
	g := mustGuard(t, &Config{})
	_, err := g.DialContext(context.Background(), "tcp", "169.254.169.254:80")
	if err == nil || !strings.Contains(err.Error(), "link-local") {
		t.Errorf("literal metadata address must be policy-blocked, got: %v", err)
	}
}

func TestNormalizeDNSAddr(t *testing.T) {
	cases := map[string]string{
		"1.1.1.1":      "1.1.1.1:53",
		"1.1.1.1:5353": "1.1.1.1:5353",
		"::1":          "[::1]:53",
		"[::1]:53":     "[::1]:53",
	}
	for in, want := range cases {
		got, err := normalizeDNSAddr(in)
		if err != nil {
			t.Errorf("%q: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("normalizeDNSAddr(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/policy.toml"
	if err := os.WriteFile(path, []byte(`
https_only = true
allow_ip_literals = true
allow_loopback = true
allow_private_ips = true
allow_hosts = ["api.example.com", ".internal.corp"]
deny_hosts = ["evil.example"]
allow_cidrs = ["10.1.0.0/16"]
deny_cidrs = ["10.66.0.0/16"]
dns_servers = ["1.1.1.1", "8.8.8.8:53"]
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if !cfg.RequireHTTPS || !cfg.AllowIPLiterals || !cfg.AllowLoopback || !cfg.AllowPrivateIPs {
		t.Errorf("boolean flags not loaded: %+v", cfg)
	}
	if len(cfg.AllowHosts) != 2 || len(cfg.DenyHosts) != 1 || len(cfg.AllowedCIDRs) != 1 || len(cfg.DeniedCIDRs) != 1 || len(cfg.DNSServers) != 2 {
		t.Errorf("lists not loaded: %+v", cfg)
	}
	// Compiles and enforces as expected.
	g, err := NewGuard(cfg)
	if err != nil {
		t.Fatalf("NewGuard: %v", err)
	}
	if err := g.CheckURL(mustURL(t, "https://10.1.2.3/")); err != nil {
		t.Errorf("allowed CIDR should pass, got %v", err)
	}
	if err := g.CheckURL(mustURL(t, "http://10.1.2.3/")); err == nil {
		t.Error("https_only should still reject plain http")
	}
}

func TestLoadConfigEmptyFileIsValid(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/policy.toml"
	if err := os.WriteFile(path, []byte("# safe defaults only\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg == nil {
		t.Fatal("cfg should be non-nil")
	}
}

func TestLoadConfigErrors(t *testing.T) {
	dir := t.TempDir()

	badToml := dir + "/bad.toml"
	_ = os.WriteFile(badToml, []byte("this is not toml = = ="), 0o644)
	if _, err := LoadConfig(badToml); err == nil {
		t.Error("invalid TOML should error")
	}

	badCIDR := dir + "/badcidr.toml"
	_ = os.WriteFile(badCIDR, []byte("allow_cidrs = [\"not-a-cidr\"]\n"), 0o644)
	if _, err := LoadConfig(badCIDR); err == nil {
		t.Error("invalid CIDR should error")
	}

	if _, err := LoadConfig(dir + "/missing.toml"); err == nil {
		t.Error("missing file should error")
	}
}

// AllowAll is the resolver-only mode: no address or host checks, so hosts
// can configure shared DNS servers without imposing a policy.
func TestAllowAllResolverOnlyMode(t *testing.T) {
	g := mustGuard(t, &Config{AllowAll: true})
	for _, raw := range []string{
		"http://127.0.0.1/",
		"http://10.0.0.1/",
		"http://169.254.169.254/",
		"http://[fc00::1]/",
		"http://8.8.8.8/",
	} {
		if err := g.CheckURL(mustURL(t, raw)); err != nil {
			t.Errorf("AllowAll should pass %s: %v", raw, err)
		}
	}
	// Scheme validation still applies.
	if err := g.CheckURL(mustURL(t, "ftp://example.com/")); err == nil {
		t.Error("AllowAll must still reject unsupported schemes")
	}
	// Dial-time checks are disabled too: a literal dial reaches the dialer
	// (connection refused, not a policy error).
	_, err := g.DialContext(context.Background(), "tcp", "127.0.0.1:1")
	if err == nil || strings.Contains(err.Error(), "network policy") {
		t.Errorf("AllowAll dial should not be policy-blocked, got: %v", err)
	}
}

func TestResolverAccessorAndNewResolver(t *testing.T) {
	g := mustGuard(t, &Config{})
	if g.Resolver() == nil {
		t.Error("Resolver() should never be nil")
	}
	if r, err := NewResolver(nil); err != nil || r != net.DefaultResolver {
		t.Errorf("NewResolver(nil) should return the system resolver, got %v, %v", r, err)
	}
	if _, err := NewResolver([]string{" "}); err == nil {
		t.Error("NewResolver should reject empty server entries")
	}
	// AllowAll configs are never parsed from files, so LoadConfig cannot
	// enable it; verify the field stays host-only by schema absence.
	_, err := NewGuard(&Config{AllowAll: true, DNSServers: []string{"1.1.1.1", "bad server"}})
	if err == nil {
		t.Error("NewGuard should still validate DNS servers in AllowAll mode")
	}
}
