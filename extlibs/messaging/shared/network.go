package shared

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/paularlott/scriptling/extlibs/netsecurity"
)

// Network is how a messaging client reaches the network. Two separate rules:
//
//   - The bot credential goes only to the platform's own hosts (IsPlatform):
//     a URL handed over by a script for any other host is fetched without it.
//   - When a network policy is configured, every request and gateway
//     connection goes through it, the platform's own hosts included. A script
//     supplies the bot token, so a platform host is an egress channel like any
//     other; an operator who uses messaging under a strict allow_hosts lists
//     the platform's hosts there.
type Network struct {
	domains  []string
	gateways []string           // extra hosts gateway connections may use (see WithGateways)
	guard    *netsecurity.Guard // nil = no network policy
	guarded  *http.Client
	dialer   *websocket.Dialer
}

// NewNetwork builds the rules for a platform whose hosts are the given domains
// and their subdomains. timeout caps the guarded client's request time; cfg is
// the optional network policy.
func NewNetwork(domains []string, timeout time.Duration, cfg ...*netsecurity.Config) *Network {
	n := &Network{domains: domains}
	if len(cfg) > 0 && cfg[0] != nil {
		guard, err := netsecurity.NewGuard(cfg[0])
		if err != nil {
			guard = netsecurity.FailClosed(err)
		}
		client := guard.HTTPClient()
		if client.Timeout == 0 || client.Timeout > timeout {
			client.Timeout = timeout
		}
		d := *websocket.DefaultDialer
		d.NetDialContext = guard.DialContext
		d.Proxy = nil // never tunnel through proxy env vars past the policy
		n.guard, n.guarded, n.dialer = guard, client, &d
	}
	return n
}

// IsPlatform reports whether rawURL's host is one of the platform's domains
// or a subdomain of one, and so may be sent the bot credential. Unparseable
// URLs are not platform URLs.
func (n *Network) IsPlatform(rawURL string) bool {
	if n == nil {
		return false
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.TrimRight(strings.ToLower(u.Hostname()), ".")
	return hostIn(host, n.domains)
}

// WithGateways adds domains (and their subdomains) that gateway connections
// may dial besides the platform's own: a platform whose WebSocket gateway
// lives on another domain (Discord's discord.gg) lists it here. Unlike the
// platform domains, these never receive the bot credential over HTTP.
func (n *Network) WithGateways(domains ...string) *Network {
	n.gateways = append(n.gateways, domains...)
	return n
}

func hostIn(host string, domains []string) bool {
	for _, d := range domains {
		if host == d || strings.HasSuffix(host, "."+d) {
			return true
		}
	}
	return false
}

// Client returns the client for every request: the policy-guarded client
// when a policy is configured, otherwise plain.
func (n *Network) Client(plain *http.Client) *http.Client {
	if n == nil || n.guarded == nil {
		return plain
	}
	return n.guarded
}

// DialWebsocket opens a gateway connection to rawURL, through the network
// policy when one is configured.
//
// The URL comes from the platform's API (Discord's /gateway/bot and
// resume_gateway_url, Slack's apps.connections.open), and the bot then
// authenticates over the connection, so a tampered or spoofed response must
// not steer it elsewhere: only wss:// URLs on the platform's own domains or
// its gateway domains (WithGateways) are dialled, policy or not.
func (n *Network) DialWebsocket(ctx context.Context, rawURL string) (*websocket.Conn, error) {
	if n == nil {
		return nil, fmt.Errorf("gateway: no network configured")
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	host := strings.TrimRight(strings.ToLower(u.Hostname()), ".")
	if u.Scheme != "wss" || !(hostIn(host, n.domains) || hostIn(host, n.gateways)) {
		return nil, fmt.Errorf("gateway: refusing to connect to %q: not a wss:// URL on the platform's own hosts", u.Redacted())
	}
	if n.guard == nil {
		conn, _, err := websocket.DefaultDialer.DialContext(ctx, u.String(), nil)
		return conn, err
	}
	if err := n.guard.CheckURL(u); err != nil {
		return nil, err
	}
	conn, _, err := n.dialer.DialContext(ctx, u.String(), nil)
	return conn, err
}
