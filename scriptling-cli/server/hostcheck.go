package server

import (
	"net"
	"net/http"
	"strings"
)

// hostPolicy decides which Host headers the server answers, the standard
// defence against DNS rebinding: a page on an attacker's name that the
// attacker re-points at this machine is same-origin with every endpoint, so
// Origin checks alone cannot stop it, but the browser still sends the
// attacker's name as Host.
//
//   - Bound to loopback (127.0.0.0/8, ::1, localhost): only loopback names
//     (localhost, *.localhost, loopback IPs) and AllowedHosts are answered.
//   - Bound anywhere else: with AllowedHosts set, loopback names, IP literals
//     (a browser cannot rebind an IP literal) and AllowedHosts are answered;
//     without it, every Host is (the server cannot know its public names).
type hostPolicy struct {
	enforce bool
	allowIP bool     // IP literals are answered (non-loopback binds)
	allowed []string // AllowedHosts: exact names, or ".suffix" for a domain and its subdomains
}

func newHostPolicy(listenAddr string, allowedHosts []string) hostPolicy {
	allowed := make([]string, 0, len(allowedHosts))
	for _, h := range allowedHosts {
		if h = normalizeHostName(h); h != "" {
			allowed = append(allowed, h)
		}
	}
	if isLoopbackListen(listenAddr) {
		return hostPolicy{enforce: true, allowed: allowed}
	}
	return hostPolicy{enforce: len(allowed) > 0, allowIP: true, allowed: allowed}
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
// answer, before any route runs.
func (s *Server) hostCheckMiddleware(next http.Handler) http.Handler {
	if !s.hosts.enforce {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.hosts.allows(r.Host) {
			Log.Debug("Refused request for a host this server does not answer", "host", r.Host, "path", r.URL.Path, "remote", r.RemoteAddr)
			http.Error(w, "Host not allowed", http.StatusMisdirectedRequest)
			return
		}
		next.ServeHTTP(w, r)
	})
}
