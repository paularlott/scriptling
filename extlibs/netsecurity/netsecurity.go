// Package netsecurity restricts outbound network access for script-facing
// libraries (requests, wait_for, websocket).
//
// # SECURITY MODEL
//
// All enforcement happens in two layers:
//
//  1. CheckURL rejects a request before it starts: unsupported schemes,
//     https-only violations, denied/allowed host rules, and (unless
//     explicitly permitted) IP-literal URLs.
//  2. DialContext resolves the hostname via the configured resolver,
//     validates EVERY resolved IP against the policy, and dials the
//     validated IP directly. Dialing the validated IP (rather than the
//     hostname) is what defeats DNS rebinding: Go never re-resolves after
//     the check, so a resolver that flips its answer between check and
//     connect cannot reach a blocked address. Validating every answer
//     defeats multi-answer tricks where one record is public and another
//     is private.
//
// Redirects are covered because every hop produces a new RoundTrip, which
// re-runs CheckURL, and every connection goes through DialContext.
//
// A nil Config (or nil *Guard) means no restrictions: existing embedders
// keep today's behavior. A non-nil Config enables the policy with safe
// defaults — loopback, link-local (including cloud metadata endpoints),
// private, unspecified and multicast addresses are all denied unless
// explicitly allowed.
package netsecurity

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"golang.org/x/net/http2"
)

// Config holds the outbound network policy for a library registration.
// The zero value plus a non-nil pointer enables safe-default blocking.
type Config struct {
	// RequireHTTPS rejects plain http:// and ws:// URLs.
	RequireHTTPS bool

	// AllowIPLiterals permits URLs that name an IP directly (http://1.2.3.4/).
	// Off by default: literals are the classic SSRF vector. An IP literal
	// inside AllowedCIDRs is always permitted regardless of this flag.
	AllowIPLiterals bool

	// AllowLoopback permits 127.0.0.0/8 and ::1 (off by default).
	AllowLoopback bool

	// AllowPrivateIPs permits RFC1918 and IPv6 unique-local ranges (off by
	// default). This is the explicit "this script may reach the LAN" switch.
	AllowPrivateIPs bool

	// AllowHosts, when non-empty, is an allowlist: only listed hosts may be
	// contacted. Listed hosts are trusted — their resolved IPs bypass the
	// loopback/private/link-local categories (that is the way to grant a
	// script access to an internal service by name). Entries are exact
	// hostnames (case-insensitive) or domain suffixes written with a leading
	// dot (".corp.example.com" matches any subdomain).
	AllowHosts []string

	// DenyHosts always wins, regardless of any other setting. Same entry
	// syntax as AllowHosts.
	DenyHosts []string

	// AllowedCIDRs permits explicit address ranges (e.g. a corporate block),
	// overriding the built-in loopback/private/link-local categories.
	AllowedCIDRs []string

	// DeniedCIDRs blocks explicit ranges and wins over AllowedCIDRs and every
	// other allow.
	DeniedCIDRs []string

	// DNSServers resolves hostnames through these servers instead of the
	// host's resolver (e.g. "1.1.1.1", "8.8.8.8:53"). Plain DNS (53/udp and
	// 53/tcp) only. Resolution and validation use the same resolver, so
	// policy decisions and connections see the same answers.
	DNSServers []string

	// AllowAll disables every address and host check, leaving only the
	// shared DNS resolver. Hosts use it to configure nameservers without
	// imposing a policy; it cannot be set from a policy file.
	AllowAll bool
}

// hostRule is one compiled host-list entry: exact match or domain suffix.
type hostRule struct {
	exact  string
	suffix string // including leading dot, e.g. ".corp.example.com"
}

// Guard is an immutable, compiled network policy. It is safe for concurrent
// use by multiple libraries and interpreters.
type Guard struct {
	// failErr is non-nil only on a FailClosed guard: an invalid config must
	// never degrade into an open policy, so every check fails with it.
	failErr error
	// lookupFn resolves hostnames; swapped only in tests to simulate
	// attacker-controlled answers (DNS rebinding, mixed record sets).
	lookupFn   func(ctx context.Context, host string) ([]net.IPAddr, error)
	denied     []*net.IPNet
	allowed    []*net.IPNet
	allowHosts []hostRule
	denyHosts  []hostRule
	resolver   *net.Resolver
	dialer     *net.Dialer
	requireHTTPS,
	allowIPLiterals,
	allowLoopback,
	allowPrivateIPs,
	allowAll bool
}

// Resolver returns the resolver this guard dials with: the configured DNS
// servers when set, otherwise the system resolver. Hosts can hand it to
// other lookups (scriptling.net.resolve) so the whole system resolves
// through the same servers.
func (g *Guard) Resolver() *net.Resolver {
	if g == nil {
		return net.DefaultResolver
	}
	return g.resolver
}

// FailClosed returns a Guard that rejects every URL and dial with an error
// derived from err. Use it when a Config fails to compile.
func FailClosed(err error) *Guard {
	return &Guard{failErr: fmt.Errorf("network policy disabled by invalid configuration: %w", err)}
}

// NewGuard compiles a Config into a Guard. Invalid CIDRs or malformed DNS
// server entries are reported as errors; callers should treat a config error
// as a registration failure rather than falling back to an open policy.
func NewGuard(cfg *Config) (*Guard, error) {
	if cfg == nil {
		return nil, nil
	}
	g := &Guard{
		allowHosts:      compileHostRules(cfg.AllowHosts),
		denyHosts:       compileHostRules(cfg.DenyHosts),
		requireHTTPS:    cfg.RequireHTTPS,
		allowIPLiterals: cfg.AllowIPLiterals,
		allowLoopback:   cfg.AllowLoopback,
		allowPrivateIPs: cfg.AllowPrivateIPs,
		allowAll:        cfg.AllowAll,
		dialer: &net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		},
	}

	var err error
	if g.denied, err = parseCIDRs(cfg.DeniedCIDRs); err != nil {
		return nil, fmt.Errorf("DeniedCIDRs: %w", err)
	}
	if g.allowed, err = parseCIDRs(cfg.AllowedCIDRs); err != nil {
		return nil, fmt.Errorf("AllowedCIDRs: %w", err)
	}
	if g.resolver, err = NewResolver(cfg.DNSServers); err != nil {
		return nil, err
	}
	resolver := g.resolver
	g.lookupFn = func(ctx context.Context, host string) ([]net.IPAddr, error) {
		return resolver.LookupIPAddr(ctx, host)
	}

	return g, nil
}

// policyFile is the TOML schema accepted by LoadConfig.
type policyFile struct {
	HTTPSOnly       bool     `toml:"https_only"`
	AllowIPLiterals bool     `toml:"allow_ip_literals"`
	AllowLoopback   bool     `toml:"allow_loopback"`
	AllowPrivateIPs bool     `toml:"allow_private_ips"`
	AllowHosts      []string `toml:"allow_hosts"`
	DenyHosts       []string `toml:"deny_hosts"`
	AllowCIDRs      []string `toml:"allow_cidrs"`
	DenyCIDRs       []string `toml:"deny_cidrs"`
	DNSServers      []string `toml:"dns_servers"`
}

// LoadConfig reads a TOML policy file. Every key is optional, for example:
//
//	https_only = true
//	allow_hosts = ["api.example.com", ".internal.corp"]
//	allow_cidrs = ["10.1.0.0/16"]
//	dns_servers = ["1.1.1.1", "8.8.8.8:53"]
//
// The file must parse and compile: an invalid policy is an error, never an
// open policy.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f policyFile
	if _, err := toml.Decode(string(data), &f); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	cfg := &Config{
		RequireHTTPS:    f.HTTPSOnly,
		AllowIPLiterals: f.AllowIPLiterals,
		AllowLoopback:   f.AllowLoopback,
		AllowPrivateIPs: f.AllowPrivateIPs,
		AllowHosts:      f.AllowHosts,
		DenyHosts:       f.DenyHosts,
		AllowedCIDRs:    f.AllowCIDRs,
		DeniedCIDRs:     f.DenyCIDRs,
		DNSServers:      f.DNSServers,
	}
	if _, err := NewGuard(cfg); err != nil {
		return nil, fmt.Errorf("invalid policy in %s: %w", path, err)
	}
	return cfg, nil
}

// CheckURL validates a request URL before it is sent: scheme, host list
// rules, and IP-literal rules. Hostname resolution is NOT checked here —
// that happens at dial time. It returns a descriptive error suitable for
// surfacing to the script.
func (g *Guard) CheckURL(u *url.URL) error {
	if g.failErr != nil {
		return g.failErr
	}
	if u == nil {
		return fmt.Errorf("network policy: no URL")
	}

	switch u.Scheme {
	case "https", "wss":
	case "http", "ws":
		if g.requireHTTPS && !g.allowAll {
			return fmt.Errorf("network policy: %s requires https", u.Scheme)
		}
	default:
		return fmt.Errorf("network policy: unsupported scheme %q", u.Scheme)
	}

	if g.allowAll {
		return nil
	}

	host := strings.ToLower(u.Hostname())
	if host == "" {
		return fmt.Errorf("network policy: URL has no host")
	}

	if matchHost(g.denyHosts, host) {
		return fmt.Errorf("network policy: host %q is denied", host)
	}

	if ip := net.ParseIP(host); ip != nil {
		// IP literal: blocked outright unless allowed, then still subject to
		// the address categories.
		if !g.allowIPLiterals && !g.containsIP(g.allowed, ip) {
			return fmt.Errorf("network policy: IP literals are not allowed")
		}
		if err := g.checkIP(ip); err != nil {
			return err
		}
		return nil
	}

	if len(g.allowHosts) > 0 && !matchHost(g.allowHosts, host) {
		return fmt.Errorf("network policy: host %q is not in the allowed host list", host)
	}

	return nil
}

// DialContext implements the resolve-validate-dial pattern described in the
// package comment. It is suitable as an http.Transport.DialContext and as a
// gorilla websocket NetDialContext.
func (g *Guard) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	if g.failErr != nil {
		return nil, g.failErr
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("network policy: invalid address %q", addr)
	}
	host = strings.ToLower(host)

	if matchHost(g.denyHosts, host) {
		return nil, fmt.Errorf("network policy: host %q is denied", host)
	}

	if ip := net.ParseIP(host); ip != nil {
		if err := g.checkIP(ip); err != nil {
			return nil, err
		}
		return g.dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
	}

	ips, err := g.lookupFn(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("DNS lookup failed for %q: %w", host, err)
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("no addresses resolved for %q", host)
	}

	// Trusted (allowlisted) hosts may resolve anywhere — that is their grant.
	// Every other host must have every answer pass the address policy, so a
	// mixed public/private answer set cannot slip a blocked address through.
	if !matchHost(g.allowHosts, host) {
		for _, ia := range ips {
			if cerr := g.checkIP(ia.IP); cerr != nil {
				return nil, cerr
			}
		}
	}

	// Dial validated IPs directly, never the hostname: Go must not
	// re-resolve. Try each answer in order so dual-stack hosts (localhost
	// resolving to ::1 and 127.0.0.1) connect on whichever stack answers.
	var lastErr error
	for _, ia := range ips {
		conn, derr := g.dialer.DialContext(ctx, network, net.JoinHostPort(ia.IP.String(), port))
		if derr == nil {
			return conn, nil
		}
		lastErr = derr
	}
	return nil, lastErr
}

// HTTPClient returns a guarded client for script HTTP requests. The transport
// disables proxy environment variables: an HTTP(S)_PROXY would tunnel the
// connection to the proxy host, bypassing the address policy for the target.
func (g *Guard) HTTPClient() *http.Client {
	return &http.Client{
		Transport: &guardTransport{
			inner: g.NewTransport(),
			guard: g,
		},
		Timeout: 30 * time.Second,
	}
}

// NewTransport builds the guarded base transport (pooled, like the shared
// scriptling HTTP pool, but with policy-controlled dialing and no proxies).
func (g *Guard) NewTransport() *http.Transport {
	t := &http.Transport{
		DialContext:           g.DialContext,
		Proxy:                 nil,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: time.Second,
		TLSClientConfig:       nil, // default verification, TLS 1.3 minimum below
	}
	t.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS13}
	http2.ConfigureTransport(t)
	// HTTP/2 configuration errors are non-fatal; the client falls back to HTTP/1.1.
	return t
}

// guardTransport re-runs CheckURL for every request, including each redirect
// hop, before handing off to the guarded dial transport.
type guardTransport struct {
	inner http.RoundTripper
	guard *Guard
}

func (t *guardTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := t.guard.CheckURL(req.URL); err != nil {
		return nil, err
	}
	return t.inner.RoundTrip(req)
}

// checkIP applies the address policy to a single IP. Order matters:
// explicit denies win, then explicit allows (the escape hatch for ranges the
// built-in categories block), then the built-in categories.
func (g *Guard) checkIP(ip net.IP) error {
	if g.allowAll {
		return nil
	}

	if v4 := ip.To4(); v4 != nil {
		ip = v4 // normalize IPv4-mapped IPv6, e.g. ::ffff:10.0.0.1
	}

	if g.containsIP(g.denied, ip) {
		return fmt.Errorf("network policy: address %s is in a denied range", ip)
	}
	if g.containsIP(g.allowed, ip) {
		return nil
	}

	switch {
	case ip.IsUnspecified():
		return fmt.Errorf("network policy: unspecified address %s is not allowed", ip)
	case ip.IsLoopback():
		if !g.allowLoopback {
			return fmt.Errorf("network policy: loopback address %s is not allowed", ip)
		}
	case ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast():
		// No toggle: link-local hosts the cloud metadata endpoints among
		// other infrastructure. Allow via an explicit AllowedCIDRs entry.
		return fmt.Errorf("network policy: link-local address %s is not allowed", ip)
	case ip.IsPrivate():
		if !g.allowPrivateIPs {
			return fmt.Errorf("network policy: private address %s is not allowed", ip)
		}
	case ip.IsMulticast():
		return fmt.Errorf("network policy: multicast address %s is not allowed", ip)
	}
	return nil
}

func (g *Guard) containsIP(nets []*net.IPNet, ip net.IP) bool {
	for _, n := range nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

func compileHostRules(hosts []string) []hostRule {
	if len(hosts) == 0 {
		return nil
	}
	rules := make([]hostRule, 0, len(hosts))
	for _, h := range hosts {
		h = strings.ToLower(strings.TrimSpace(h))
		if h == "" {
			continue
		}
		if strings.HasPrefix(h, ".") {
			rules = append(rules, hostRule{suffix: h})
		} else {
			rules = append(rules, hostRule{exact: h})
		}
	}
	return rules
}

func matchHost(rules []hostRule, host string) bool {
	for _, r := range rules {
		if r.exact != "" {
			if host == r.exact {
				return true
			}
		} else if strings.HasSuffix(host, r.suffix) {
			return true
		}
	}
	return false
}

func parseCIDRs(cidrs []string) ([]*net.IPNet, error) {
	if len(cidrs) == 0 {
		return nil, nil
	}
	nets := make([]*net.IPNet, 0, len(cidrs))
	for _, c := range cidrs {
		_, n, err := net.ParseCIDR(strings.TrimSpace(c))
		if err != nil {
			return nil, fmt.Errorf("invalid CIDR %q: %w", c, err)
		}
		nets = append(nets, n)
	}
	return nets, nil
}

// NewResolver returns a *net.Resolver that queries the given DNS servers
// (plain DNS, port 53; entries like "1.1.1.1" or "8.8.8.8:53"). An empty
// list returns the system resolver. Guards use it internally; hosts can
// use it directly to resolve through the same servers everywhere.
func NewResolver(servers []string) (*net.Resolver, error) {
	if len(servers) == 0 {
		return net.DefaultResolver, nil
	}
	addrs := make([]string, 0, len(servers))
	for _, srv := range servers {
		addr, err := normalizeDNSAddr(srv)
		if err != nil {
			return nil, fmt.Errorf("DNSServers: %w", err)
		}
		addrs = append(addrs, addr)
	}
	return &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			var lastErr error
			for _, addr := range addrs {
				conn, derr := (&net.Dialer{Timeout: 3 * time.Second}).DialContext(ctx, network, addr)
				if derr == nil {
					return conn, nil
				}
				lastErr = derr
			}
			if lastErr == nil {
				lastErr = fmt.Errorf("no DNS servers configured")
			}
			return nil, lastErr
		},
	}, nil
}

func normalizeDNSAddr(server string) (string, error) {
	server = strings.TrimSpace(server)
	if server == "" {
		return "", fmt.Errorf("empty DNS server")
	}
	// Already host:port (including bracketed IPv6 with port)? Keep as is;
	// otherwise append the default port, bracketing bare IPv6 correctly.
	addr := server
	if _, _, err := net.SplitHostPort(server); err != nil {
		addr = net.JoinHostPort(server, "53")
	}
	// DNS servers are addresses, not names: a typo must fail loudly rather
	// than produce a resolver that dials a hostname.
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return "", fmt.Errorf("invalid DNS server %q: %w", server, err)
	}
	if net.ParseIP(host) == nil {
		return "", fmt.Errorf("invalid DNS server %q: not an IP address", server)
	}
	return addr, nil
}
