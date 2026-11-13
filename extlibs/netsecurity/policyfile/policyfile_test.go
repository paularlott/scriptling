package policyfile

import (
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/paularlott/scriptling/extlibs/netsecurity"
)

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse %q: %v", raw, err)
	}
	return u
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
client_timeout = "45s"
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
	if cfg.ClientTimeout != 45*time.Second {
		t.Errorf("client_timeout not loaded: %+v", cfg)
	}
	// Compiles and enforces as expected.
	g, err := netsecurity.NewGuard(cfg)
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

// TestLoadConfigClientTimeoutValidation checks the policy-file spelling of
// ClientTimeout: a positive Go duration string, anything else is an error —
// an invalid policy never degrades into a different one.
func TestLoadConfigClientTimeoutValidation(t *testing.T) {
	dir := t.TempDir()

	path := dir + "/bad.toml"
	if err := os.WriteFile(path, []byte("client_timeout = \"soon\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(path); err == nil {
		t.Error("client_timeout = \"soon\" should fail to load")
	}

	path = dir + "/zero.toml"
	if err := os.WriteFile(path, []byte("client_timeout = \"0s\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(path); err == nil {
		t.Error("client_timeout = \"0s\" should fail to load")
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

func TestLoadConfigAllowReservedIPs(t *testing.T) {
	path := t.TempDir() + "/policy.toml"
	if err := os.WriteFile(path, []byte("allow_reserved_ips = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if !cfg.AllowReservedIPs {
		t.Errorf("allow_reserved_ips not loaded: %+v", cfg)
	}
}
