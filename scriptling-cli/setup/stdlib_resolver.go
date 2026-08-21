package setup

import (
	"context"
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	scriptlingresolve "github.com/paularlott/scriptling/extlibs/net/resolve"
	"github.com/paularlott/scriptling/extlibs/netsecurity"
)

// stdlibResolver uses Go's net package for DNS resolution, through either
// the system resolver or an injected one (the network configuration's
// resolver, so every script network path resolves via the same servers).
type stdlibResolver struct {
	timeout  time.Duration
	resolver *net.Resolver // nil = system default
}

var defaultNetResolver = &net.Resolver{}

func (r stdlibResolver) netResolver() *net.Resolver {
	if r.resolver != nil {
		return r.resolver
	}
	return defaultNetResolver
}

func (r stdlibResolver) LookupIP(host string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()
	return r.netResolver().LookupHost(ctx, host)
}

func (r stdlibResolver) LookupSRV(service string) ([]*net.TCPAddr, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()
	_, srvs, err := r.netResolver().LookupSRV(ctx, "", "", service)
	if err != nil {
		return nil, err
	}
	var tcpAddrs []*net.TCPAddr
	for _, srv := range srvs {
		target := strings.TrimSuffix(srv.Target, ".")
		ips, err := r.netResolver().LookupHost(ctx, target)
		if err != nil {
			continue
		}
		for _, ip := range ips {
			if parsed := net.ParseIP(ip); parsed != nil {
				tcpAddrs = append(tcpAddrs, &net.TCPAddr{IP: parsed, Port: int(srv.Port)})
			}
		}
	}
	if len(tcpAddrs) == 0 {
		return nil, errors.New("no addresses found")
	}
	return tcpAddrs, nil
}

func (r stdlibResolver) ResolveSRVHttp(uri string) string {
	if !strings.HasPrefix(uri, "srv+") && !strings.HasPrefix(uri, "SRV+") {
		if !strings.HasPrefix(uri, "http://") && !strings.HasPrefix(uri, "https://") {
			return "https://" + uri
		}
		return uri
	}

	u, err := url.Parse(uri[4:])
	if err != nil {
		return uri[4:]
	}

	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()
	_, srvs, err := r.netResolver().LookupSRV(ctx, "", "", u.Host)
	if err != nil || len(srvs) == 0 {
		return uri[4:]
	}

	port := int(srvs[0].Port)
	if port <= 0 {
		return uri[4:]
	}

	u.Host = net.JoinHostPort(u.Hostname(), strconv.Itoa(port))
	return u.String()
}

// scriptResolver picks the resolver for the resolve library: the network
// configuration's resolver when one is active, otherwise the system one.
func scriptResolver(netPolicy []*netsecurity.Config) scriptlingresolve.Resolver {
	if len(netPolicy) > 0 && netPolicy[0] != nil {
		if g, err := netsecurity.NewGuard(netPolicy[0]); err == nil && g != nil {
			return stdlibResolver{timeout: 2 * time.Second, resolver: g.Resolver()}
		}
	}
	return stdlibResolver{timeout: 2 * time.Second}
}
