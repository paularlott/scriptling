package server

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
)

// hostPolicy decides which Host headers the server answers, the standard
// defence against DNS rebinding: a page on an attacker's name that the
// attacker re-points at this machine is same-origin with every endpoint, so
// Origin checks alone cannot stop it, but the browser still sends the
// attacker's name as Host.
//
//   - Bound to loopback (127.0.0.0/8, ::1, localhost): only loopback names
//     (localhost, *.localhost, loopback IPs) and AllowedHosts are answered.
//     The check costs nothing there, since a loopback server is reached by
//     those names anyway, and it is where rebinding bites: a page on any
//     website could otherwise call a developer's local server.
//   - Bound anywhere else: without AllowedHosts, every Host is answered, as
//     with every other restriction that is off until configured; the server
//     cannot know its public names. With AllowedHosts set, loopback names,
//     IP literals (a browser cannot rebind an IP literal), the machine's own
//     host name as the OS reports it and its first label, and the listed
//     names are answered, and the refusal names the flag.
type hostPolicy struct {
	enforce bool
	allowIP bool     // IP literals are answered (non-loopback binds)
	allowed []string // AllowedHosts: exact names, or ".suffix" for a domain and its subdomains
}

// open reports whether the policy answers every Host: a non-loopback bind
// with no AllowedHosts.
func (p hostPolicy) open() bool { return !p.enforce }

// localHostname reports the machine's host name; a variable so tests can
// set it.
var localHostname = os.Hostname

func newHostPolicy(listenAddr string, allowedHosts []string) hostPolicy {
	allowed := make([]string, 0, len(allowedHosts)+2)
	for _, h := range allowedHosts {
		if h = normalizeAllowedHost(h); h != "" {
			allowed = append(allowed, h)
		}
	}
	if isLoopbackListen(listenAddr) {
		return hostPolicy{enforce: true, allowed: allowed}
	}
	if len(allowed) == 0 {
		return hostPolicy{}
	}
	if name, err := localHostname(); err == nil {
		if name = normalizeHostName(name); name != "" {
			allowed = append(allowed, name)
			if short, _, found := strings.Cut(name, "."); found && short != "" {
				allowed = append(allowed, short)
			}
		}
	}
	return hostPolicy{enforce: true, allowIP: true, allowed: allowed}
}

// normalizeAllowedHost prepares an AllowedHosts entry: lowercased, without
// a trailing dot, brackets or a port, and with "*.example.com" taken as
// ".example.com" (the domain and its subdomains). An entry that is only a
// port, or empty, is dropped.
func normalizeAllowedHost(entry string) string {
	entry = strings.TrimSpace(entry)
	if host, _, err := net.SplitHostPort(entry); err == nil {
		entry = host
	}
	entry = strings.TrimPrefix(entry, "*")
	return normalizeHostName(entry)
}

// isLoopbackListen reports whether a listen address only accepts loopback
// connections. An empty host (":8000") listens on every interface.
func isLoopbackListen(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	host = normalizeHostName(host)
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// normalizeHostName lowercases a host name, strips IPv6 brackets and a
// trailing dot.
func normalizeHostName(h string) string {
	h = strings.ToLower(strings.TrimSpace(h))
	h = strings.TrimPrefix(strings.TrimSuffix(h, "]"), "[")
	return strings.TrimSuffix(h, ".")
}

// allows reports whether the server answers a request with this Host header.
func (p hostPolicy) allows(hostHeader string) bool {
	if !p.enforce {
		return true
	}
	host, _, err := net.SplitHostPort(hostHeader)
	if err != nil {
		host = hostHeader
	}
	host = normalizeHostName(host)
	if host == "" {
		return false
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback() || p.allowIP
	}
	for _, a := range p.allowed {
		if host == a || (strings.HasPrefix(a, ".") && (host == a[1:] || strings.HasSuffix(host, a))) {
			return true
		}
	}
	return false
}

// hostCheckMiddleware refuses a request whose Host the server does not
// answer, before any route runs. The first refusal of each Host name is
// logged at warning level, so an operator whose server is reached by a name
// it does not know (a DNS name, a reverse proxy forwarding the public name)
// sees what to list in --allowed-hosts; repeats stay at debug level so a
// rebinding attempt cannot flood the log.
func (s *Server) hostCheckMiddleware(next http.Handler) http.Handler {
	if s.hosts.open() {
		return next
	}
	var refused sync.Map // host name -> struct{}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.hosts.allows(r.Host) {
			if _, seen := refused.LoadOrStore(r.Host, struct{}{}); !seen {
				Log.Warn("Refused request for a host this server does not answer; list it with --allowed-hosts if it is yours", "host", r.Host, "path", r.URL.Path, "remote", r.RemoteAddr)
			} else {
				Log.Debug("Refused request for a host this server does not answer", "host", r.Host, "path", r.URL.Path, "remote", r.RemoteAddr)
			}
			http.Error(w, fmt.Sprintf("Host %q is not a name this server answers; list it with --allowed-hosts", r.Host), http.StatusMisdirectedRequest)
			return
		}
		next.ServeHTTP(w, r)
	})
}
