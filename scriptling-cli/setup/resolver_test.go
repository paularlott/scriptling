package setup

import (
	"strings"
	"testing"

	"github.com/paularlott/scriptling/extlibs/netsecurity"
)

// With a network configuration active, scriptling.net.resolve must answer
// from the configuration's resolver — dead nameservers prove the wiring.
func TestScriptResolverUsesConfiguredServers(t *testing.T) {
	r := scriptResolver([]*netsecurity.Config{{AllowAll: true, DNSServers: []string{"127.0.0.1:1"}}})
	if _, err := r.LookupIP("not-in-hosts.test"); err == nil {
		t.Fatal("dead nameserver should fail the lookup")
	}

	// No configuration: the system resolver answers as before.
	r = scriptResolver(nil)
	ips, err := r.LookupIP("localhost")
	if err != nil {
		t.Fatalf("system resolver should resolve localhost: %v", err)
	}
	if len(ips) == 0 {
		t.Fatal("expected at least one address for localhost")
	}
	found := false
	for _, ip := range ips {
		if ip == "127.0.0.1" || ip == "::1" {
			found = true
		}
	}
	if !found {
		t.Errorf("localhost should resolve to loopback, got %v", strings.Join(ips, ","))
	}
}
