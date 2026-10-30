package extlibs

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/paularlott/scriptling/conversion"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/extlibs/fssecurity"
	"github.com/paularlott/scriptling/extlibs/netsecurity"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/pool"
)

// parallelRequestOverhead estimates the memory requests.parallel holds per
// request besides the response and the request's entries: the batch's slice
// entries and the converted map's base size.
const parallelRequestOverhead = 256

// parallelRequestEntryBytes estimates the memory of one entry copied from a
// request dict into its converted map.
const parallelRequestEntryBytes = 64

// parallelResultSlotBytes is the size of one slot of the result list.
const parallelResultSlotBytes = int64(unsafe.Sizeof(object.Object(nil)))

// RegisterRequestsLibrary registers the requests library, under the network
// policy cfg when one is given. A CA bundle a script names with verify="path"
// may be any file: use RegisterRequestsLibraryConfigured to confine it.
func RegisterRequestsLibrary(registrar interface{ RegisterLibrary(*object.Library) }, cfg ...*netsecurity.Config) {
	var policy *netsecurity.Config
	if len(cfg) > 0 {
		policy = cfg[0]
	}
	RegisterRequestsLibraryConfigured(registrar, fssecurity.Config{}, policy)
}

// RegisterRequestsLibraryConfigured registers the requests library with a
// filesystem allow-path policy and a network policy, as the os library and the
// CLI's --allowed-paths and --network-policy use them: a CA bundle a script
// names with verify="path" must lie within paths' allowed directories (else an
// uncatchable PermissionError), and every request passes the network policy.
// A zero fssecurity.Config and a nil policy register it unrestricted.
func RegisterRequestsLibraryConfigured(registrar interface{ RegisterLibrary(*object.Library) }, paths fssecurity.Config, policy *netsecurity.Config) {
	paths.AllowedPaths = fssecurity.NormalizeAllowedPaths(paths.AllowedPaths)
	if paths.AllowedPaths == nil && policy == nil {
		registrar.RegisterLibrary(RequestsLibrary)
		return
	}
	lib := &requestsLib{paths: paths}
	if policy != nil {
		guard, gerr := netsecurity.NewGuard(policy)
		if gerr != nil {
			guard = netsecurity.FailClosed(gerr)
		}
		lib.guard = guard
		lib.client = guard.HTTPClient()
		lib.insecureClient = guard.InsecureHTTPClient()
	}
	registrar.RegisterLibrary(newRequestsLibrary(lib))
}

// Response class for HTTP responses

// ResponseClass defines the Response class with its methods
var ResponseClass = &object.Class{
	Name: "Response",
	Methods: map[string]object.Object{
		"json": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}
				if instance, ok := args[0].(*object.Instance); ok {
					if body, err := instance.Field("body").AsString(); err == nil {
						return conversion.MustParseJSON(body)
					}
				}
				return errors.NewError("json() called on non-Response object")
			},
			HelpText: `json() - Parses the response body as JSON and returns the parsed object`,
		},
		"raise_for_status": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}
				if instance, ok := args[0].(*object.Instance); ok {
					if statusCode, err := instance.Field("status_code").AsInt(); err == nil {
						if statusCode >= 400 {
							kind := "Client"
							if statusCode >= 500 {
								kind = "Server"
							}
							return &object.Exception{
								ExceptionType: "HTTPError",
								Message:       fmt.Sprintf("HTTPError: %d %s Error", statusCode, kind),
								// raise_for_status() raises on 4xx/5xx: the returned
								// exception is propagating, not a mere value.
								Raised: true,
							}
						}
						return &object.Null{}
					}
				}
				return errors.NewError("raise_for_status() called on non-Response object")
			},
			HelpText: `raise_for_status() - Raises an exception if the status code indicates an error`,
		},
	},
}

// createResponseInstance creates a Response for a request in a batch, where
// every failure is a response with status 0: a response that does not fit
// the limits becomes one too.
func createResponseInstance(statusCode int, headers map[string]string, body []byte, url string) *object.Instance {
	resp, errObj := newResponseInstance(statusCode, headers, body, url)
	if errObj != nil {
		resp, _ = newResponseInstance(0, nil, []byte(errObj.(*object.Error).Message), url)
	}
	return resp
}

// newResponseInstance creates a Response, or returns the error when the
// response would not fit the limits.
func newResponseInstance(statusCode int, headers map[string]string, body []byte, url string) (*object.Instance, object.Object) {
	// The body was admitted as read; the response holds two more copies (the
	// text and the bytes value), so admit those together before making them,
	// and the headers as one value.
	if errObj := object.CheckRoom("response", 2*int64(len(body))); errObj != nil {
		return nil, errObj
	}
	var headerBytes int64
	for k, v := range headers {
		headerBytes += int64(len(k) + len(v))
	}
	if errObj := object.CheckAllocFor("response headers", headerBytes); errObj != nil {
		return nil, errObj
	}
	text := object.NewString(string(body))
	// Convert headers to object.Dict
	headerDict := &object.Dict{Pairs: make(map[string]object.DictPair)}
	for k, v := range headers {
		headerDict.SetByString(k, object.NewString(v))
	}

	instance := object.NewInstanceWithFields(ResponseClass, map[string]object.Object{
		"status_code": object.NewInteger(int64(statusCode)),
		"text":        text,
		"content":     object.NewBytes(body),
		"headers":     headerDict,
		"body":        text, // deprecated alias of .text, sharing its copy
		"url":         object.NewString(url),
	})
	return instance, nil
}

// Exception types for requests library
var requestExceptionType = object.NewString("RequestException")
var httpErrorType = object.NewString("HTTPError")

// Create exceptions namespace dict
var exceptionsNamespace = object.NewStringDict(map[string]object.Object{
	"RequestException": requestExceptionType,
	"HTTPError":        httpErrorType,
})

// tlsVerify is a request's verify option, as in Python's requests: by default
// the server's certificate is checked against the system's CA certificates;
// off accepts any certificate; caBundle names a PEM file of CA certificates to
// trust instead of the system's.
type tlsVerify struct {
	off      bool
	caBundle string
}

// parseVerify reads the verify option: True (the default), False, or the path
// of a CA bundle.
func parseVerify(options map[string]object.Object) (tlsVerify, object.Object) {
	v, ok := options["verify"]
	if !ok {
		return tlsVerify{}, nil
	}
	switch val := v.(type) {
	case *object.Null:
		return tlsVerify{}, nil // None is the default, as in Python
	case *object.Boolean:
		on, _ := val.AsBool()
		return tlsVerify{off: !on}, nil
	case *object.String:
		path, _ := val.AsString()
		if path == "" {
			return tlsVerify{}, errors.NewValueError("verify: the CA bundle path is empty")
		}
		return tlsVerify{caBundle: path}, nil
	}
	return tlsVerify{}, errors.NewTypeError("BOOLEAN or STRING", v.Type().String())
}

// parseRequestOptions parses the options dict and returns timeout, headers, params, user, pass, verify
func parseRequestOptions(options map[string]object.Object) (int, map[string]string, map[string]string, string, string, tlsVerify, object.Object) {
	verify, errObj := parseVerify(options)
	if errObj != nil {
		return 0, nil, nil, "", "", tlsVerify{}, errObj
	}
	timeout := 5
	headers := make(map[string]string)
	params := make(map[string]string)
	user := ""
	pass := ""
	if timeoutPair, ok := options["timeout"]; ok {
		if timeoutInt, err := timeoutPair.AsInt(); err == nil {
			timeout = int(timeoutInt)
		}
	}
	if headersPair, ok := options["headers"]; ok {
		if headersDict, err := headersPair.AsDict(); err == nil {
			headers = extractHeaders(headersDict)
		}
	}
	if paramsPair, ok := options["params"]; ok {
		if paramsDict, err := paramsPair.AsDict(); err == nil {
			params = extractParams(paramsDict)
		}
	}
	if authPair, ok := options["auth"]; ok {
		if authList, err := authPair.AsList(); err == nil {
			if len(authList) == 2 {
				if u, err := authList[0].AsString(); err == nil {
					user = u
				}
				if p, err := authList[1].AsString(); err == nil {
					pass = p
				}
			}
		}
	}
	if errObj := checkRequestFields(headers, params); errObj != nil {
		return 0, nil, nil, "", "", tlsVerify{}, errObj
	}
	return timeout, headers, params, user, pass, verify, nil
}

// extractParams extracts params from a dict, converting all values to strings
func extractParams(dict map[string]object.Object) map[string]string {
	params := make(map[string]string, len(dict))
	object.TrackMap(params, int64(len(dict))*parallelRequestEntryBytes)
	for key, value := range dict {
		// Convert any value type to string (int, float, bool, etc.)
		if strVal, err := object.CoerceWireString(value); err == nil {
			params[key] = strVal
		}
	}
	return params
}

// buildURLWithParams appends query parameters to a URL
func buildURLWithParams(baseURL string, params map[string]string) (string, object.Object) {
	full := buildURLWithParamsRaw(baseURL, params)
	// Encoding can triple a parameter's size: the URL is a value too.
	if errObj := object.CheckAllocFor("request URL", int64(len(full))); errObj != nil {
		return "", errObj
	}
	return full, nil
}

// checkRequestFields holds the converted headers and parameters, which the
// request copies, to the interpreter's value limit as one value.
// checkRequestFields admits the headers and parameters of a request: each
// is a value on its own, and together they must fit in the memory left.
func checkRequestFields(headers, params map[string]string) object.Object {
	var total int64
	for k, v := range headers {
		n := int64(len(k) + len(v))
		if errObj := object.CheckAllocFor("request header", n); errObj != nil {
			return errObj
		}
		total += n
	}
	for k, v := range params {
		n := int64(len(k) + len(v))
		if errObj := object.CheckAllocFor("request parameter", n); errObj != nil {
			return errObj
		}
		total += n
	}
	return object.CheckRoom("request headers and parameters", total)
}

func buildURLWithParamsRaw(baseURL string, params map[string]string) string {
	if len(params) == 0 {
		return baseURL
	}

	// Parse existing URL to preserve any existing query params
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		// If parsing fails, just append params naively
		return baseURL + "?" + encodeParams(params)
	}

	// Add new params to existing query
	query := parsedURL.Query()
	for key, value := range params {
		query.Set(key, value)
	}
	parsedURL.RawQuery = query.Encode()
	return parsedURL.String()
}

// encodeParams encodes params to URL query string format
func encodeParams(params map[string]string) string {
	values := url.Values{}
	for key, value := range params {
		values.Set(key, value)
	}
	return values.Encode()
}

// extractRequestArgs extracts URL, optional data, and remaining options from kwargs and args.
// hasData should be true for POST/PUT/PATCH methods, false for GET/DELETE.
// Returns (url, data, options, errorOrNil).
func extractRequestArgs(kwargs object.Kwargs, args []object.Object, hasData bool) (string, string, map[string]object.Object, object.Object) {
	var url, data string
	options := make(map[string]object.Object)

	// 1. Handle kwargs
	for k, v := range kwargs.Kwargs {
		if k == "url" {
			if s, err := v.AsString(); err == nil {
				url = s
			} else {
				return "", "", nil, errors.NewTypeError("STRING", v.Type().String())
			}
		} else if hasData && k == "data" {
			// Accept String (UTF-8 encoded) or Bytes (raw). The body is
			// threaded through the request pipeline as a Go string because Go
			// strings are byte sequences — round-tripping bytes through
			// string() is lossless for HTTP body transmission.
			if s, err := v.AsString(); err == nil {
				data = s
			} else if b, ok := v.(*object.Bytes); ok {
				data = string(b.BytesValue())
			} else {
				return "", "", nil, errors.NewTypeError("STRING or BYTES", v.Type().String())
			}
		} else if hasData && k == "json" {
			// Handle json parameter - convert to JSON string, under the
			// interpreter's limits: shared values convert once per reference
			// and the encoded body is a value too.
			goValue, limitErr := conversion.ToGoLimited(v)
			if limitErr != nil {
				return "", "", nil, limitErr
			}
			jsonBytes, err := json.Marshal(goValue)
			if err != nil {
				return "", "", nil, errors.NewError("failed to encode json: %s", err.Error())
			}
			if errObj := object.CheckAllocFor("request body", int64(len(jsonBytes))); errObj != nil {
				return "", "", nil, errObj
			}
			object.TrackSlice(jsonBytes)
			data = string(jsonBytes)
			// Set Content-Type header if not already set
			if _, hasHeaders := options["headers"]; !hasHeaders {
				contentTypeDict := &object.Dict{Pairs: make(map[string]object.DictPair)}
				contentTypeDict.SetByString("Content-Type", object.NewString("application/json"))
				options["headers"] = contentTypeDict
			}
		} else {
			options[k] = v
		}
	}

	// 2. Handle positional args
	argIdx := 0
	if url == "" && len(args) > argIdx {
		if s, err := args[argIdx].AsString(); err == nil {
			url = s
			argIdx++
		} else {
			return "", "", nil, errors.NewTypeError("STRING", args[argIdx].Type().String())
		}
	}

	if hasData && data == "" && len(args) > argIdx {
		if s, err := args[argIdx].AsString(); err == nil {
			data = s
			argIdx++
		} else if b, ok := args[argIdx].(*object.Bytes); ok {
			// Bytes input — preserve raw bytes via Go's byte-sequence strings.
			data = string(b.BytesValue())
			argIdx++
		} else if args[argIdx].Type() != object.DICT_OBJ {
			// Not a string, bytes, or dict (options), error
			return "", "", nil, errors.NewTypeError("STRING or BYTES", args[argIdx].Type().String())
		}
		// If it's a dict, we'll process it as options below
	}

	// Check for legacy options dict
	if len(args) > argIdx {
		if d, err := args[argIdx].AsDict(); err == nil {
			for k, v := range d {
				options[k] = v
			}
		}
	}

	if url == "" {
		return "", "", nil, errors.NewArgumentError(0, 1)
	}

	return url, data, options, nil
}

var RequestsLibrary = newRequestsLibrary(&requestsLib{})

// maxCABundleClients bounds the clients kept for CA bundles. A script names a
// handful; past the bound the oldest is dropped (a later request builds it
// again).
const maxCABundleClients = 16

// requestsLib is one registration of the requests library: its HTTP clients,
// its network guard and its filesystem jail.
type requestsLib struct {
	// client and insecureClient serve verify=True and verify=False. Without a
	// network policy both are nil: the shared scriptling pool and its
	// separate insecure pool. With one they are the guard's, so every policy
	// check applies whatever verify says.
	client, insecureClient *http.Client
	guard                  *netsecurity.Guard
	// paths confines the CA bundles a script names with verify="path".
	paths fssecurity.Config

	mu        sync.Mutex
	caClients map[[sha256.Size]byte]*http.Client // by the bundle's content
	caOrder   [][sha256.Size]byte                // oldest first
}

// checkCABundlePath refuses a CA bundle outside the allowed paths with the
// uncatchable PermissionError of every jail denial.
func (l *requestsLib) checkCABundlePath(path string) object.Object {
	if l.paths.CheckPath(path) != nil {
		return errors.NewPermissionError("access denied: CA bundle '%s' is outside allowed directories", path)
	}
	return nil
}

// clientFor returns the client for a request's verify option. A guarded
// library never falls back to the unguarded pool.
func (l *requestsLib) clientFor(ctx context.Context, v tlsVerify) (*http.Client, object.Object) {
	switch {
	case v.caBundle != "":
		return l.caBundleClient(ctx, v.caBundle)
	case !v.off:
		return l.client, nil // nil: the shared pool
	case l.insecureClient != nil:
		return l.insecureClient, nil
	default:
		return pool.GetInsecureHTTPClient(), nil
	}
}

// caBundleClient returns a client trusting only the CA certificates in the PEM
// file at path, reading it through the library's jail and the interpreter's
// limits. Clients are kept by the bundle's content, so a bundle named again
// reuses its connections and an edited one gets a new client.
func (l *requestsLib) caBundleClient(ctx context.Context, path string) (*http.Client, object.Object) {
	if errObj := l.checkCABundlePath(path); errObj != nil {
		return nil, errObj
	}
	pem, errObj := readFileBytes(ctx, l.paths, path)
	if errObj != nil {
		return nil, errObj
	}
	key := sha256.Sum256(pem)

	l.mu.Lock()
	defer l.mu.Unlock()
	if c, ok := l.caClients[key]; ok {
		return c, nil
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(pem) {
		return nil, errors.NewValueError("verify: no PEM certificates in CA bundle '%s'", path)
	}
	var c *http.Client
	if l.guard != nil {
		c = l.guard.HTTPClientWithRoots(roots)
	} else {
		c = pool.NewHTTPClientWithRoots(roots)
	}
	if l.caClients == nil {
		l.caClients = make(map[[sha256.Size]byte]*http.Client)
	}
	if len(l.caOrder) >= maxCABundleClients {
		oldest := l.caOrder[0]
		l.caOrder = l.caOrder[1:]
		l.caClients[oldest].CloseIdleConnections()
		delete(l.caClients, oldest)
	}
	l.caClients[key] = c
	l.caOrder = append(l.caOrder, key)
	return c, nil
}

// newRequestsLibrary builds the requests library for one registration.
func newRequestsLibrary(lib *requestsLib) *object.Library {
	return object.NewLibrary(RequestsLibraryName, map[string]*object.Builtin{
		// Exceptions namespace - returns dict with exception types
		"exceptions": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				return exceptionsNamespace
			},
		},
		"get": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				rawURL, _, options, err := extractRequestArgs(kwargs, args, false)
				if err != nil {
					return err
				}
				timeout, headers, params, user, pass, verify, errObj := parseRequestOptions(options)
				if errObj != nil {
					return errObj
				}
				// Build URL with query params
				fullURL, errObj := buildURLWithParams(rawURL, params)
				if errObj != nil {
					return errObj
				}
				client, errObj := lib.clientFor(ctx, verify)
				if errObj != nil {
					return errObj
				}
				return httpRequestWithContext(ctx, "GET", fullURL, "", timeout, headers, user, pass, client)
			},
			HelpText: `get(url, **kwargs) - Send a GET request

Sends an HTTP GET request to the specified URL.

Parameters:
  url (string): The URL to send the request to
  **kwargs: Optional arguments
    - timeout (int): Request timeout in seconds (default: 5)
    - headers (dict): HTTP headers as key-value pairs
    - params (dict): Query parameters to append to URL
    - auth (tuple/list): Basic authentication as (username, password)
    - verify (bool or str): Verify the server's TLS certificate (default: True). False accepts any certificate, for a development server only; a path names a PEM bundle of CA certificates to trust instead of the system's

Returns:
  Response object with status_code, text, headers, body, url, and json() method`,
		},
		"post": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				url, data, options, err := extractRequestArgs(kwargs, args, true)
				if err != nil {
					return err
				}
				timeout, headers, params, user, pass, verify, errObj := parseRequestOptions(options)
				if errObj != nil {
					return errObj
				}
				// POST can also have query params
				fullURL, errObj := buildURLWithParams(url, params)
				if errObj != nil {
					return errObj
				}
				client, errObj := lib.clientFor(ctx, verify)
				if errObj != nil {
					return errObj
				}
				return httpRequestWithContext(ctx, "POST", fullURL, data, timeout, headers, user, pass, client)
			},
			HelpText: `post(url, data=None, json=None, **kwargs) - Send a POST request

Sends an HTTP POST request to the specified URL with the given data.

Parameters:
  url (string): The URL to send the request to
  data (string, optional): The request body data as a string
  json (dict/list, optional): Data to be JSON-encoded and sent (sets Content-Type to application/json)
  **kwargs: Optional arguments
    - timeout (int): Request timeout in seconds (default: 5)
    - headers (dict): HTTP headers as key-value pairs
    - auth (tuple/list): Basic authentication as (username, password)
    - verify (bool or str): Verify the server's TLS certificate (default: True). False accepts any certificate, for a development server only; a path names a PEM bundle of CA certificates to trust instead of the system's

Note: Use either 'data' or 'json', not both.

Returns:
  Response object with status_code, text, headers, body, url, and json() method`,
		},
		"put": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				url, data, options, err := extractRequestArgs(kwargs, args, true)
				if err != nil {
					return err
				}
				timeout, headers, params, user, pass, verify, errObj := parseRequestOptions(options)
				if errObj != nil {
					return errObj
				}
				fullURL, errObj := buildURLWithParams(url, params)
				if errObj != nil {
					return errObj
				}
				client, errObj := lib.clientFor(ctx, verify)
				if errObj != nil {
					return errObj
				}
				return httpRequestWithContext(ctx, "PUT", fullURL, data, timeout, headers, user, pass, client)
			},
			HelpText: `put(url, data=None, json=None, **kwargs) - Send a PUT request

Sends an HTTP PUT request to the specified URL with the given data.

Parameters:
  url (string): The URL to send the request to
  data (string, optional): The request body data as a string
  json (dict/list, optional): Data to be JSON-encoded and sent (sets Content-Type to application/json)
  **kwargs: Optional arguments
    - timeout (int): Request timeout in seconds (default: 5)
    - headers (dict): HTTP headers as key-value pairs
    - auth (tuple/list): Basic authentication as (username, password)
    - verify (bool or str): Verify the server's TLS certificate (default: True). False accepts any certificate, for a development server only; a path names a PEM bundle of CA certificates to trust instead of the system's

Note: Use either 'data' or 'json', not both.

Returns:
  Response object with status_code, text, headers, body, url, and json() method`,
		},
		"delete": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				rawURL, _, options, err := extractRequestArgs(kwargs, args, false)
				if err != nil {
					return err
				}
				timeout, headers, params, user, pass, verify, errObj := parseRequestOptions(options)
				if errObj != nil {
					return errObj
				}
				fullURL, errObj := buildURLWithParams(rawURL, params)
				if errObj != nil {
					return errObj
				}
				client, errObj := lib.clientFor(ctx, verify)
				if errObj != nil {
					return errObj
				}
				return httpRequestWithContext(ctx, "DELETE", fullURL, "", timeout, headers, user, pass, client)
			},
			HelpText: `delete(url, **kwargs) - Send a DELETE request

Sends an HTTP DELETE request to the specified URL.

Parameters:
  url (string): The URL to send the request to
  **kwargs: Optional arguments
    - timeout (int): Request timeout in seconds (default: 5)
    - headers (dict): HTTP headers as key-value pairs
    - auth (tuple/list): Basic authentication as (username, password)
    - verify (bool or str): Verify the server's TLS certificate (default: True). False accepts any certificate, for a development server only; a path names a PEM bundle of CA certificates to trust instead of the system's

Returns:
  Response object with status_code, text, headers, body, url, and json() method`,
		},
		"patch": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				url, data, options, err := extractRequestArgs(kwargs, args, true)
				if err != nil {
					return err
				}
				timeout, headers, params, user, pass, verify, errObj := parseRequestOptions(options)
				if errObj != nil {
					return errObj
				}
				fullURL, errObj := buildURLWithParams(url, params)
				if errObj != nil {
					return errObj
				}
				client, errObj := lib.clientFor(ctx, verify)
				if errObj != nil {
					return errObj
				}
				return httpRequestWithContext(ctx, "PATCH", fullURL, data, timeout, headers, user, pass, client)
			},
			HelpText: `patch(url, data=None, json=None, **kwargs) - Send a PATCH request

Sends an HTTP PATCH request to the specified URL with the given data.

Parameters:
  url (string): The URL to send the request to
  data (string, optional): The request body data as a string
  json (dict/list, optional): Data to be JSON-encoded and sent (sets Content-Type to application/json)
  **kwargs: Optional arguments
    - timeout (int): Request timeout in seconds (default: 5)
    - headers (dict): HTTP headers as key-value pairs
    - auth (tuple/list): Basic authentication as (username, password)
    - verify (bool or str): Verify the server's TLS certificate (default: True). False accepts any certificate, for a development server only; a path names a PEM bundle of CA certificates to trust instead of the system's

Note: Use either 'data' or 'json', not both.

Returns:
  Response object with status_code, text, headers, body, url, and json() method`,
		},
		"parallel": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) < 1 {
					return errors.NewArgumentError(1, 0)
				}
				requestList, err := args[0].AsList()
				if err != nil {
					return errors.NewTypeError("LIST", args[0].Type().String())
				}
				if len(requestList) == 0 {
					return &object.List{Elements: []object.Object{}}
				}

				maxParallel := int64(4)
				if mp, mpErr := kwargs.GetInt("max_parallel", 4); mpErr == nil {
					maxParallel = mp
				}
				maxParallel = clampRequestsMaxParallel(maxParallel)

				// The batch's bookkeeping grows with the request count: the
				// result list (a value, so held to the value limit), plus a
				// converted map per request (held to the memory limit). Check
				// both before building any of it.
				if errObj := object.CheckElements(int64(len(requestList))); errObj != nil {
					return errObj
				}
				// Everything the batch holds at once, as one total: the
				// result list and the batch slices, plus each converted map,
				// which copies every entry of its request dict.
				batchBytes := int64(len(requestList)) * parallelRequestOverhead
				for _, reqObj := range requestList {
					if d, ok := reqObj.(*object.Dict); ok {
						batchBytes += int64(len(d.Pairs)) * parallelRequestEntryBytes
					}
				}
				// The result list is a value: its slots are checked above and
				// charged below, so the preflight and the charges agree.
				resultBytes := int64(len(requestList)) * parallelResultSlotBytes
				if errObj := object.CheckRoom("parallel request batch", batchBytes+resultBytes); errObj != nil {
					return errObj
				}
				results := make([]object.Object, len(requestList))
				object.TrackSlice(results)
				var wg sync.WaitGroup

				// Workers fetch for the calling interpreter: bind them to its
				// guard so response bodies are charged to it and held to its
				// limits.
				guard := object.CurrentGuard()
				if guard == nil {
					guard = getEnvFromContext(ctx).ResourceGuard()
				}

				// Workers do pure-Go HTTP, not script — detach the environment from
				// their context so the blocking HTTP call inside executeRequest does
				// not try to release an interpreter lock the worker doesn't hold.
				workerCtx := context.WithValue(ctx, "scriptling-env", (*object.Environment)(nil))

				// Read the request dicts while this goroutine still holds the
				// interpreter lock; workers only touch the converted maps.
				reqDicts := make([]map[string]object.Object, len(requestList))
				// The converted maps have no pointer of their own to charge;
				// their estimated size rides on the slice that holds them.
				object.TrackAlloc(&reqDicts[:1][0], batchBytes)
				pending := make([]int, 0, len(requestList))
				for i, reqObj := range requestList {
					reqDict, dictErr := reqObj.AsDict()
					if dictErr != nil {
						results[i] = errors.NewTypeError("DICT", reqObj.Type().String())
						continue
					}
					reqDicts[i] = reqDict
					pending = append(pending, i)
				}

				// Each request's client, from its verify option. A CA bundle is
				// read here, on the script's goroutine, under the interpreter's
				// limits: one outside the allowed paths raises, as every jail
				// denial does; any other problem fails only that request.
				clients := make([]*http.Client, len(requestList))
				object.TrackSlice(clients)
				ready := make([]int, 0, len(pending))
				for _, i := range pending {
					v, errObj := parseVerify(reqDicts[i])
					if errObj == nil && v.caBundle != "" {
						if denied := lib.checkCABundlePath(v.caBundle); denied != nil {
							return denied
						}
					}
					if errObj == nil {
						clients[i], errObj = lib.clientFor(ctx, v)
					}
					if errObj != nil {
						results[i] = createResponseInstance(0, nil, []byte(objectErrorMessage(errObj)), requestURL(reqDicts[i]))
						continue
					}
					ready = append(ready, i)
				}
				pending = ready

				// Release this goroutine's interpreter lock while the parallel batch
				// runs, so other script goroutines can proceed meanwhile.
				object.RunBlocking(ctx, func() {
					// A fixed pool of max_parallel workers takes requests from
					// a queue, so a long request list costs no goroutine each.
					jobs := make(chan int)
					workers := min(int(maxParallel), len(pending))
					for range workers {
						wg.Add(1)
						go func() {
							defer wg.Done()
							defer object.BindGuard(guard)()
							for idx := range jobs {
								results[idx] = executeParallelRequest(workerCtx, reqDicts[idx], clients[idx])
							}
						}()
					}
					for _, i := range pending {
						jobs <- i
					}
					close(jobs)
					wg.Wait()
				})
				return &object.List{Elements: results}
			},
			HelpText: `parallel(requests, max_parallel=4) - Execute multiple HTTP requests in parallel

Sends multiple HTTP requests concurrently with a configurable concurrency limit.
Results are returned in the same order as the input requests.

Parameters:
  requests (list): List of request dicts, each with:
    - method (string): HTTP method ("GET", "POST", "PUT", "DELETE", "PATCH")
    - url (string): The URL to send the request to
    - data (string, optional): Request body as string
    - json (dict/list, optional): Data to JSON-encode as body
    - headers (dict, optional): HTTP headers
    - params (dict, optional): Query parameters
    - auth (list/tuple, optional): [username, password] for basic auth
    - timeout (int, optional): Timeout in seconds (default: 30)
    - verify (bool or str, optional): Verify the server's TLS certificate (default: True), or a CA bundle path
  max_parallel (int): Maximum concurrent requests (default: 4). The host may cap this; a larger request is reduced to the cap.

Returns:
  list: List of Response objects in the same order as input requests.
        Failed requests return a Response with status_code=0 and the error in body.

Example:
  results = requests.parallel([
      {"method": "GET", "url": "https://api.example.com/item/1"},
      {"method": "POST", "url": "https://api.example.com/item/2", "json": {"key": "val"}},
  ], max_parallel=4)
  for resp in results:
      if resp.status_code == 200:
          data = resp.json()`,
		},
	}, map[string]object.Object{
		// Exception types as constants (for except clause matching)
		"RequestException": requestExceptionType,
		"HTTPError":        httpErrorType,
		"Response":         ResponseClass,
	}, "HTTP requests library")
}

func extractHeaders(dict map[string]object.Object) map[string]string {
	headers := make(map[string]string, len(dict))
	object.TrackMap(headers, int64(len(dict))*parallelRequestEntryBytes)
	for key, value := range dict {
		if strVal, err := value.AsString(); err == nil {
			headers[key] = strVal
		}
	}
	return headers
}

// checkOutgoingRequest admits everything an outgoing request is built from:
// the URL, the body, each header and, when a user is given, the Basic Auth
// header exactly as it will be sent. Each is a value on its own; together
// they must fit in the memory left.
func checkOutgoingRequest(url, body string, headers map[string]string, user, pass string) object.Object {
	if errObj := object.CheckAllocFor("request URL", int64(len(url))); errObj != nil {
		return errObj
	}
	if errObj := object.CheckAllocFor("request body", int64(len(body))); errObj != nil {
		return errObj
	}
	total := int64(len(url) + len(body))
	for k, v := range headers {
		n := int64(len(k) + len(v))
		if errObj := object.CheckAllocFor("request header", n); errObj != nil {
			return errObj
		}
		total += n
	}
	if user != "" {
		n := basicAuthHeaderBytes(user, pass)
		if errObj := object.CheckAllocFor("request authorization", n); errObj != nil {
			return errObj
		}
		total += n
	}
	return object.CheckRoom("request", total)
}

// basicAuthHeaderBytes is the size of the Authorization header SetBasicAuth
// adds: the name, "Basic " and the padded Base64 of "user:pass".
func basicAuthHeaderBytes(user, pass string) int64 {
	credentials := len(user) + 1 + len(pass)
	return int64(len("Authorization: Basic ") + base64.StdEncoding.EncodedLen(credentials))
}

func httpRequestWithContext(parentCtx context.Context, method, url, body string, timeoutSecs int, headers map[string]string, user, pass string, client *http.Client) object.Object {
	if errObj := checkOutgoingRequest(url, body, headers, user, pass); errObj != nil {
		return errObj
	}
	// Combine parent context with timeout
	ctx, cancel := context.WithTimeout(parentCtx, time.Duration(timeoutSecs)*time.Second)
	defer cancel()

	var req *http.Request
	var err error

	if body != "" {
		req, err = http.NewRequestWithContext(ctx, method, url, strings.NewReader(body))
		if err != nil {
			return errors.NewError("http request error: %s", err.Error())
		}
		if _, hasContentType := headers["Content-Type"]; !hasContentType {
			req.Header.Set("Content-Type", "application/json")
		}
	} else {
		req, err = http.NewRequestWithContext(ctx, method, url, nil)
		if err != nil {
			return errors.NewError("http request error: %s", err.Error())
		}
	}

	// Set custom headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// Set basic auth if provided
	if user != "" {
		req.SetBasicAuth(user, pass)
	}

	var resp *http.Response
	reqClient := client
	if reqClient == nil {
		reqClient = pool.GetHTTPClient()
	}
	object.RunBlocking(ctx, func() { resp, err = reqClient.Do(req) })
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return errors.NewError("http timeout after %d seconds", timeoutSecs)
		}
		return errors.NewError("http error: %s", err.Error())
	}
	defer resp.Body.Close()

	var respBody []byte
	var limitErr object.Object
	object.RunBlocking(ctx, func() { respBody, limitErr, err = object.ReadAllLimited(resp.Body, "http response body") })
	if err != nil {
		return errors.NewError("http read error: %s", err.Error())
	}
	if limitErr != nil {
		return limitErr
	}

	respHeaders := make(map[string]string)
	for key, values := range resp.Header {
		if len(values) > 0 {
			respHeaders[key] = values[0]
		}
	}

	instance, errObj := newResponseInstance(resp.StatusCode, respHeaders, respBody, url)
	if errObj != nil {
		return errObj
	}
	return instance
}

// executeParallelRequest executes a single request from a parallel batch spec dict.
func executeParallelRequest(ctx context.Context, spec map[string]object.Object, client *http.Client) object.Object {
	// Extract method
	method := "GET"
	if methodObj, ok := spec["method"]; ok {
		if m, err := methodObj.AsString(); err == nil {
			method = strings.ToUpper(m)
		}
	}

	// Extract URL
	urlStr := ""
	if urlObj, ok := spec["url"]; ok {
		if u, err := urlObj.AsString(); err == nil {
			urlStr = u
		}
	}
	if urlStr == "" {
		return createResponseInstance(0, nil, []byte("parallel request missing 'url'"), "")
	}

	// Extract timeout (default 30s for parallel — longer than individual default)
	timeout := 30
	if timeoutObj, ok := spec["timeout"]; ok {
		if t, err := timeoutObj.AsInt(); err == nil {
			timeout = int(t)
		}
	}

	// Extract headers
	headers := make(map[string]string)
	if headersObj, ok := spec["headers"]; ok {
		if d, err := headersObj.AsDict(); err == nil {
			headers = extractHeaders(d)
		}
	}

	// Extract query params
	params := map[string]string{}
	if paramsObj, ok := spec["params"]; ok {
		if d, err := paramsObj.AsDict(); err == nil {
			params = extractParams(d)
		}
	}
	// The worker is bound to the calling interpreter: converted fields and
	// the encoded URL are held to its limits like the body below.
	if errObj := checkRequestFields(headers, params); errObj != nil {
		return createResponseInstance(0, nil, []byte(errObj.(*object.Error).Message), urlStr)
	}
	if len(params) > 0 {
		full, errObj := buildURLWithParams(urlStr, params)
		if errObj != nil {
			return createResponseInstance(0, nil, []byte(errObj.(*object.Error).Message), urlStr)
		}
		urlStr = full
	}

	// Extract auth
	user, pass := "", ""
	if authObj, ok := spec["auth"]; ok {
		if authList, err := authObj.AsList(); err == nil && len(authList) == 2 {
			if u, err := authList[0].AsString(); err == nil {
				user = u
			}
			if p, err := authList[1].AsString(); err == nil {
				pass = p
			}
		}
	}

	// Extract body — either 'data' (string) or 'json' (dict/list to encode)
	body := ""
	if jsonObj, ok := spec["json"]; ok {
		// The worker is bound to the calling interpreter, so the converted
		// value and the encoded body are held to its limits and charged.
		goValue, limitErr := conversion.ToGoLimited(jsonObj)
		if limitErr != nil {
			return createResponseInstance(0, nil, []byte(limitErr.Message), urlStr)
		}
		jsonBytes, err := json.Marshal(goValue)
		if err != nil {
			return createResponseInstance(0, nil, []byte(fmt.Sprintf("failed to encode json: %s", err.Error())), urlStr)
		}
		if errObj := object.CheckAllocFor("request body", int64(len(jsonBytes))); errObj != nil {
			return createResponseInstance(0, nil, []byte(errObj.(*object.Error).Message), urlStr)
		}
		object.TrackSlice(jsonBytes)
		body = string(jsonBytes)
		if _, hasContentType := headers["Content-Type"]; !hasContentType {
			headers["Content-Type"] = "application/json"
		}
	} else if dataObj, ok := spec["data"]; ok {
		if d, err := dataObj.AsString(); err == nil {
			body = d
		}
	}

	// In a batch every failure is a response with status 0, as documented.
	result := httpRequestWithContext(ctx, method, urlStr, body, timeout, headers, user, pass, client)
	if errObj, ok := result.(*object.Error); ok {
		return createResponseInstance(0, nil, []byte(errObj.Message), urlStr)
	}
	return result
}

// requestURL is a parallel request's url, for a status 0 response that fails it
// before it is sent.
func requestURL(spec map[string]object.Object) string {
	if u, ok := spec["url"]; ok {
		if s, err := u.AsString(); err == nil {
			return s
		}
	}
	return ""
}

// objectErrorMessage is the message of an error or exception object.
func objectErrorMessage(obj object.Object) string {
	switch e := obj.(type) {
	case *object.Error:
		return e.Message
	case *object.Exception:
		return e.Message
	}
	return obj.Inspect()
}
