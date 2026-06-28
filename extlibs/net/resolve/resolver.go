package resolve

import (
	"net"
	"sync"
)

// Resolver defines the interface for DNS resolution.
// Each consumer must provide its own implementation when calling Register.
type Resolver interface {
	// LookupIP resolves a hostname to a list of IP address strings.
	LookupIP(host string) ([]string, error)

	// LookupSRV resolves an SRV service name to a list of TCP addresses.
	LookupSRV(service string) ([]*net.TCPAddr, error)

	// ResolveSRVHttp resolves a srv+http(s):// URI to a concrete URL,
	// preserving the original hostname for SNI/TLS while substituting the SRV-resolved port.
	ResolveSRVHttp(uri string) string
}

// globalResolver is the resolver used by the scriptling library functions.
// It is set by Register and must not be nil when library functions are called.
// It is guarded by resolverMu because the servers call Register once per
// request from many goroutines (setupScriptling runs per connection).
var (
	resolverMu    sync.RWMutex
	globalResolver Resolver
)

// SetResolver replaces the global resolver.
func SetResolver(r Resolver) {
	if r != nil {
		resolverMu.Lock()
		globalResolver = r
		resolverMu.Unlock()
	}
}

// GetResolver returns a snapshot of the current global resolver.
func GetResolver() Resolver {
	resolverMu.RLock()
	r := globalResolver
	resolverMu.RUnlock()
	return r
}
