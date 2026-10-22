package telegram

import (
	"strings"
	"testing"
	"time"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/messaging/shared"
	"github.com/paularlott/scriptling/extlibs/netsecurity"
)

// The Telegram library ignored the network policy: a plain HTTP client made
// every request, so a script could reach api.telegram.org (and through a bot
// it controls, send data anywhere) under any policy (CWE-918, strix 4250
// vuln-0006). Every request now goes through the policy; the refusal comes
// before any connection is made.
func TestTelegramHonorsNetworkPolicy(t *testing.T) {
	for name, cfg := range map[string]*netsecurity.Config{
		"deny_hosts":  {DenyHosts: []string{"api.telegram.org"}},
		"allow_hosts": {AllowHosts: []string{"only.example"}},
	} {
		p := scriptling.New()
		Register(p, nil, cfg)
		_, err := p.Eval(`import scriptling.messaging.telegram as telegram
c = telegram.client("123456:FAKE")
telegram.send_message(c, "1", "x")`)
		if err == nil || !strings.Contains(err.Error(), "network policy") {
			t.Errorf("%s: expected a network policy refusal, got %v", name, err)
		}
	}
}

// Without a policy the plain client is used, as before; with one, the
// guarded client, for every request.
func TestTelegramClientSelection(t *testing.T) {
	plain := newClient("t", nil)
	if plain.network.Client(plain.httpClient) != plain.httpClient {
		t.Error("without a policy the plain client should be used")
	}
	c := newClient("t", nil)
	c.network = shared.NewNetwork(platformDomains, 60*time.Second, &netsecurity.Config{})
	if c.network.Client(c.httpClient) == c.httpClient {
		t.Error("under a policy the guarded client must be used")
	}
}
