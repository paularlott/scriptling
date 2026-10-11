package netsecurity

import (
	"context"
	"crypto/tls"
	"net"
	"os"
	"strings"
	"testing"
)

var reservedSamples = []string{
	"100.64.0.1", "100.100.100.200", "0.0.0.1", "192.0.0.8", "192.0.2.1",
	"198.18.0.1", "198.51.100.1", "203.0.113.1", "240.0.0.1", "255.255.255.255",
	"[::7f00:1]", "[64:ff9b::a9fe:a9fe]", "[64:ff9b:1::1]", "[100::1]",
	"[2001::1]", "[2001:2::1]", "[2001:db8::1]", "[2002:7f00:1::1]",
	"[3fff::1]", "[fec0::1]",
}

func TestReservedRangesBlockedByDefault(t *testing.T) {
	g := mustGuard(t, &Config{AllowIPLiterals: true})
	for _, h := range reservedSamples {
		err := g.CheckURL(mustURL(t, "http://"+h+"/"))
		if err == nil {
			t.Errorf("%s should be blocked", h)
		} else if !strings.Contains(err.Error(), "reserved") {
			t.Errorf("%s: error should mention reserved, got: %v", h, err)
		}
	}
}

func TestPublicAddressesStillPass(t *testing.T) {
	g := mustGuard(t, &Config{AllowIPLiterals: true})
	for _, h := range []string{"1.1.1.1", "8.8.8.8", "[2606:4700:4700::1111]"} {
		if err := g.CheckURL(mustURL(t, "http://"+h+"/")); err != nil {
			t.Errorf("%s should pass, got %v", h, err)
		}
	}
}

func TestAllowReservedIPs(t *testing.T) {
	g := mustGuard(t, &Config{AllowIPLiterals: true, AllowReservedIPs: true})
	for _, h := range reservedSamples {
		if err := g.CheckURL(mustURL(t, "http://"+h+"/")); err != nil {
			t.Errorf("%s should pass with AllowReservedIPs, got %v", h, err)
		}
	}
}

func TestAllowedCIDRPermitsOnlyThatReservedRange(t *testing.T) {
	g := mustGuard(t, &Config{AllowIPLiterals: true, AllowedCIDRs: []string{"100.64.0.0/10"}})
	for _, h := range []string{"100.64.0.1", "100.100.100.200"} {
		if err := g.CheckURL(mustURL(t, "http://"+h+"/")); err != nil {
			t.Errorf("%s should pass via AllowedCIDRs, got %v", h, err)
		}
	}
	for _, h := range []string{"198.18.0.1", "192.0.2.1", "[2001:db8::1]"} {
		if err := g.CheckURL(mustURL(t, "http://"+h+"/")); err == nil {
			t.Errorf("%s should remain blocked", h)
		}
	}
}

func TestDeniedCIDRWinsOverAllowReservedIPs(t *testing.T) {
	g := mustGuard(t, &Config{AllowIPLiterals: true, AllowReservedIPs: true, DeniedCIDRs: []string{"100.64.0.0/10"}})
	if err := g.CheckURL(mustURL(t, "http://100.64.0.1/")); err == nil {
		t.Error("DeniedCIDRs must win over AllowReservedIPs")
	}
	if err := g.CheckURL(mustURL(t, "http://198.18.0.1/")); err != nil {
		t.Errorf("other reserved ranges should pass, got %v", err)
	}
}

func TestReservedDoesNotShadowLoopbackOrUnspecified(t *testing.T) {
	g := mustGuard(t, &Config{AllowIPLiterals: true})
	if err := g.CheckURL(mustURL(t, "http://[::1]/")); err == nil || !strings.Contains(err.Error(), "loopback") {
		t.Errorf("::1 should be reported as loopback, got %v", err)
	}
	if err := g.CheckURL(mustURL(t, "http://[::]/")); err == nil || !strings.Contains(err.Error(), "unspecified") {
		t.Errorf(":: should be reported as unspecified, got %v", err)
	}
	gl := mustGuard(t, &Config{AllowIPLiterals: true, AllowLoopback: true})
	if err := gl.CheckURL(mustURL(t, "http://[::1]/")); err != nil {
		t.Errorf("::1 should pass with AllowLoopback, got %v", err)
	}
}

func TestDialContextBlocksReservedResolution(t *testing.T) {
	g := mustGuard(t, &Config{AllowHosts: []string{"trusted.corp"}})
	g.lookupFn = func(ctx context.Context, host string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("100.64.0.5")}}, nil
	}
	if _, err := g.DialContext(context.Background(), "tcp", "other.corp:1"); err == nil || !strings.Contains(err.Error(), "reserved") {
		t.Errorf("host resolving to reserved address must be refused, got %v", err)
	}
	// Trusted host bypasses the category check: failure (if any) is a dial error.
	if _, err := g.DialContext(context.Background(), "tcp", "trusted.corp:1"); err != nil && strings.Contains(err.Error(), "network policy") {
		t.Errorf("trusted host should not be policy-blocked, got %v", err)
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

func TestNewTransportMinTLS12(t *testing.T) {
	g := mustGuard(t, &Config{})
	if got := g.NewTransport().TLSClientConfig.MinVersion; got != tls.VersionTLS12 {
		t.Errorf("MinVersion = %#x, want TLS 1.2", got)
	}
}
