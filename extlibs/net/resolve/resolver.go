package resolve

import "net"

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
var globalResolver Resolver

// SetResolver replaces the global resolver.
func SetResolver(r Resolver) {
	if r != nil {
		globalResolver = r
	}
}

// GetResolver returns the current global resolver.
func GetResolver() Resolver {
	return globalResolver
}
