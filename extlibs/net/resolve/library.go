package resolve

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/paularlott/scriptling/conversion"
	"github.com/paularlott/scriptling/object"
)

const (
	LibraryName = "scriptling.net.resolve"
	LibraryDesc = "DNS resolution utilities for IP lookup, SRV record resolution, and srv+http URL resolution"
)

var (
	library     *object.Library
	libraryOnce sync.Once
)

// Register registers the resolve library with the given registrar.
// resolver is required and must not be nil.
func Register(registrar interface{ RegisterLibrary(*object.Library) }, resolver Resolver) {
	if resolver == nil {
		panic("resolve.Register: resolver is required")
	}
	SetResolver(resolver)
	libraryOnce.Do(func() {
		library = buildLibrary()
	})
	registrar.RegisterLibrary(library)
}

func buildLibrary() *object.Library {
	return object.NewLibrary(LibraryName, map[string]*object.Builtin{
		"lookup_ip": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) != 1 {
					return &object.Error{Message: fmt.Sprintf("lookup_ip expected 1 argument, got %d", len(args))}
				}
				host, coerceErr := args[0].CoerceString()
				if coerceErr != nil {
					return &object.Error{Message: "lookup_ip: host must be a string"}
				}

			var ips []string
			var lookupErr error
			object.RunBlocking(ctx, func() { ips, lookupErr = GetResolver().LookupIP(host) })
			if lookupErr != nil {
				return &object.Error{Message: fmt.Sprintf("lookup_ip: %s", lookupErr.Error())}
			}

				return conversion.FromGo(ips)
			},
			HelpText: `lookup_ip(host) - Resolve a hostname to a list of IP addresses

Parameters:
  host (str): The hostname to resolve

Returns:
  list[str]: List of IP address strings

Example:
  import scriptling.net.resolve as resolve
  ips = resolve.lookup_ip("example.com")
  print(ips)  # ["93.184.216.34"]`,
		},
		"lookup_srv": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) != 1 {
					return &object.Error{Message: fmt.Sprintf("lookup_srv expected 1 argument, got %d", len(args))}
				}
				service, coerceErr := args[0].CoerceString()
				if coerceErr != nil {
					return &object.Error{Message: "lookup_srv: service must be a string"}
				}

			var addrs []*net.TCPAddr
			var lookupErr error
			object.RunBlocking(ctx, func() { addrs, lookupErr = GetResolver().LookupSRV(service) })
			if lookupErr != nil {
				return &object.Error{Message: fmt.Sprintf("lookup_srv: %s", lookupErr.Error())}
			}

				result := make([]any, 0, len(addrs))
				for _, addr := range addrs {
					entry := map[string]any{
						"ip":   addr.IP.String(),
						"port": addr.Port,
					}
					result = append(result, entry)
				}

				return conversion.FromGo(result)
			},
			HelpText: `lookup_srv(service) - Resolve an SRV record to a list of addresses

Parameters:
  service (str): The SRV service name (e.g. "_myservice._tcp.example.com")

Returns:
  list[dict]: List of address dicts with "ip" (str) and "port" (int) keys

Example:
  import scriptling.net.resolve as resolve
  addrs = resolve.lookup_srv("_myservice._tcp.example.com")
  for addr in addrs:
      print(addr.ip, addr.port)`,
		},
		"resolve_srv_http": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) != 1 {
					return &object.Error{Message: fmt.Sprintf("resolve_srv_http expected 1 argument, got %d", len(args))}
				}
				uri, coerceErr := args[0].CoerceString()
				if coerceErr != nil {
					return &object.Error{Message: "resolve_srv_http: uri must be a string"}
				}

			var result string
			object.RunBlocking(ctx, func() { result = GetResolver().ResolveSRVHttp(uri) })
			return object.NewString(result)
			},
			HelpText: `resolve_srv_http(uri) - Resolve a srv+http(s):// URI to a concrete URL

Strips the srv+ prefix, resolves the SRV record for the host, and returns
the URL with the correct port substituted. The original hostname is preserved
for SNI/TLS. If the URI does not start with srv+, it is returned unchanged
(with an https:// prefix added if no scheme is present).

Parameters:
  uri (str): The URI to resolve (e.g. "srv+https://service.example.com/path")

Returns:
  str: The resolved URL

Example:
  import scriptling.net.resolve as resolve
  url = resolve.resolve_srv_http("srv+https://api.example.com/v1")
  print(url)  # "https://api.example.com:8443/v1"`,
		},
	}, nil, LibraryDesc)
}
