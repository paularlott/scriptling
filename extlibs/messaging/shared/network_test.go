package shared

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/paularlott/scriptling/extlibs/netsecurity"
)

func TestNetworkIsPlatform(t *testing.T) {
	n := NewNetwork([]string{"slack.com"}, time.Second)
	cases := map[string]bool{
		"https://slack.com/api/x":         true,
		"https://files.slack.com/f":       true,
		"https://SLACK.COM./x":            true,
		"https://evilslack.com/x":         false,
		"https://slack.com.evil.io/x":     false,
		"http://127.0.0.1/":               false,
		"https://attacker.test/slack.com": false,
		"://bad":                          false,
	}
	for u, want := range cases {
		if got := n.IsPlatform(u); got != want {
			t.Errorf("IsPlatform(%q) = %v, want %v", u, got, want)
		}
	}
}

// Under a network policy every request uses the guarded client, the
// platform's own hosts included: a script supplies the bot token, so a
// platform host is an egress channel like any other (strix 4250 vuln-0006).
// Without a policy the plain client is used.
func TestNetworkClient(t *testing.T) {
	plain := &http.Client{}

	open := NewNetwork([]string{"slack.com"}, time.Second)
	if open.Client(plain) != plain {
		t.Error("without a policy the plain client should be used")
	}

	guarded := NewNetwork([]string{"slack.com"}, time.Second, &netsecurity.Config{})
	c := guarded.Client(plain)
	if c == plain {
		t.Fatal("under a policy every request must use the guarded client")
	}
	if c.Timeout != time.Second {
		t.Errorf("guarded client timeout = %v, want 1s", c.Timeout)
	}

	// The guarded client enforces the policy on platform hosts too.
	deny := NewNetwork([]string{"slack.com"}, time.Second, &netsecurity.Config{DenyHosts: []string{"slack.com"}})
	_, err := deny.Client(plain).Get("https://slack.com/api/auth.test")
	if err == nil || !strings.Contains(err.Error(), "network policy") {
		t.Fatalf("expected a policy refusal for a denied platform host, got %v", err)
	}
}

// Gateway websockets go through the policy too: the URL is checked and the
// dial resolves and validates through the guard.
func TestNetworkDialWebsocket(t *testing.T) {
	deny := NewNetwork([]string{"discord.gg"}, time.Second, &netsecurity.Config{DenyHosts: []string{"gateway.discord.gg"}})
	if _, err := deny.DialWebsocket(context.Background(), "wss://gateway.discord.gg/?v=10"); err == nil || !strings.Contains(err.Error(), "network policy") {
		t.Fatalf("denied gateway host: expected a policy refusal, got %v", err)
	}
	strict := NewNetwork([]string{"slack.com"}, time.Second, &netsecurity.Config{AllowHosts: []string{"only.example"}})
	if _, err := strict.DialWebsocket(context.Background(), "wss://wss-primary.slack.com/link"); err == nil || !strings.Contains(err.Error(), "network policy") {
		t.Fatalf("host outside allow_hosts: expected a policy refusal, got %v", err)
	}
	// Loopback IP literal: denied by the default policy rules.
	if _, err := strict.DialWebsocket(context.Background(), "ws://127.0.0.1:1/"); err == nil || !strings.Contains(err.Error(), "network policy") {
		t.Fatalf("IP literal: expected a policy refusal, got %v", err)
	}
}
