package netsecurity

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"
)

func resolvingTo(g *Guard, ips ...string) {
	g.lookupFn = func(ctx context.Context, host string) ([]net.IPAddr, error) {
		out := make([]net.IPAddr, len(ips))
		for i, s := range ips {
			out[i] = net.IPAddr{IP: net.ParseIP(s)}
		}
		return out, nil
	}
}

// An allowlisted host's answers skipped the address checks entirely, so
// allow_hosts silently switched off deny_cidrs and the link-local block for
// that host; with a suffix entry an attacker who can point a name under it
// reached cloud metadata or a denied range (CWE-918, strix 4250 vuln-0007).
// deny_cidrs now always wins, and link-local needs an allow_cidrs entry.
func TestTrustedHostStillHonorsDeniesAndLinkLocal(t *testing.T) {
	ctx := context.Background()
	for name, tc := range map[string]struct {
		cfg     *Config
		answers []string
		wantErr string
	}{
		"denied range":           {&Config{AllowHosts: []string{".nip.io"}, DeniedCIDRs: []string{"127.0.0.0/8"}}, []string{"127.0.0.1"}, "denied range"},
		"denied range, mixed":    {&Config{AllowHosts: []string{".nip.io"}, DeniedCIDRs: []string{"10.66.0.0/16"}}, []string{"10.1.1.1", "10.66.0.5"}, "denied range"},
		"metadata":               {&Config{AllowHosts: []string{".nip.io"}}, []string{"169.254.169.254"}, "link-local"},
		"metadata, mapped":       {&Config{AllowHosts: []string{".nip.io"}}, []string{"::ffff:169.254.169.254"}, "link-local"},
		"link-local v6":          {&Config{AllowHosts: []string{".nip.io"}}, []string{"fe80::1"}, "link-local"},
		"deny beats allow_cidrs": {&Config{AllowHosts: []string{".nip.io"}, AllowedCIDRs: []string{"169.254.0.0/16"}, DeniedCIDRs: []string{"169.254.169.254/32"}}, []string{"169.254.169.254"}, "denied range"},
	} {
		g := mustGuard(t, tc.cfg)
		resolvingTo(g, tc.answers...)
		_, err := g.DialContext(ctx, "tcp", "x.127.0.0.1.nip.io:1")
		if err == nil || !strings.Contains(err.Error(), "network policy") || !strings.Contains(err.Error(), tc.wantErr) {
			t.Errorf("%s: expected a %q policy refusal, got %v", name, tc.wantErr, err)
		}
	}
}

// The allowlist grant itself is unchanged: an allowlisted host may resolve
// to loopback, private or reserved addresses, and to a link-local address the
// operator grants explicitly with allow_cidrs. A refused dial to a closed port
// is a dial error, not a policy error.
func TestTrustedHostGrantPreserved(t *testing.T) {
	ctx := context.Background()
	for name, tc := range map[string]struct {
		cfg     *Config
		answers []string
	}{
		"loopback":            {&Config{AllowHosts: []string{"internal.corp"}}, []string{"127.0.0.1"}},
		"private":             {&Config{AllowHosts: []string{"internal.corp"}}, []string{"10.0.0.5"}},
		"granted link-local":  {&Config{AllowHosts: []string{"internal.corp"}, AllowedCIDRs: []string{"169.254.169.254/32"}}, []string{"169.254.169.254"}},
		"deny elsewhere only": {&Config{AllowHosts: []string{"internal.corp"}, DeniedCIDRs: []string{"10.66.0.0/16"}}, []string{"127.0.0.1"}},
	} {
		g := mustGuard(t, tc.cfg)
		resolvingTo(g, tc.answers...)
		g.dialer.Timeout = 200 * time.Millisecond // unroutable granted addresses must not stall the test
		_, err := g.DialContext(ctx, "tcp", "internal.corp:1")
		if err != nil && strings.Contains(err.Error(), "network policy") {
			t.Errorf("%s: the allowlist grant was refused by the policy: %v", name, err)
		}
	}
}
