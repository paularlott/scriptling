// Package oauth is the scriptling.oauth library: OAuth 2.0 and OpenID Connect
// for scripts, against any identity provider that publishes discovery
// metadata. A script signs a person in with device sign-in (RFC 8628, waiting
// in the CLI or polling from a web page), gets a token for itself with client
// credentials, keeps tokens fresh with refresh tokens, and verifies the access
// tokens sent to an API it serves. It uses Go's standard library only.
package oauth

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/paularlott/scriptling/conversion"
	scriptlingerrors "github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/extlibs/fssecurity"
	"github.com/paularlott/scriptling/extlibs/netsecurity"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/pool"
)

const (
	LibraryName = "scriptling.oauth"
	LibraryDesc = "OAuth 2.0 and OpenID Connect: device sign-in, client credentials, refresh and token verification"

	defaultTimeout = 10 * time.Second
)

var (
	library     *object.Library
	libraryOnce sync.Once
)

// Register registers scriptling.oauth, under the network policy cfg when one
// is given. A CA bundle a script names with verify="path" may be any file: use
// RegisterConfigured to confine it.
func Register(registrar interface{ RegisterLibrary(*object.Library) }, cfg ...*netsecurity.Config) {
	var policy *netsecurity.Config
	if len(cfg) > 0 {
		policy = cfg[0]
	}
	RegisterConfigured(registrar, fssecurity.Config{}, policy)
}

// RegisterConfigured registers scriptling.oauth with a filesystem allow-path
// policy and a network policy, as the CLI's --allowed-paths and
// --network-policy give them: every request to an IdP passes the network
// policy, and a CA bundle named with verify="path" must lie within the allowed
// paths. A zero fssecurity.Config and a nil policy register it unrestricted.
func RegisterConfigured(registrar interface{ RegisterLibrary(*object.Library) }, paths fssecurity.Config, policy *netsecurity.Config) {
	paths.AllowedPaths = fssecurity.NormalizeAllowedPaths(paths.AllowedPaths)
	if paths.AllowedPaths == nil && policy == nil {
		libraryOnce.Do(func() { library = buildLibrary(&transports{}) })
		registrar.RegisterLibrary(library)
		return
	}
	t := &transports{paths: paths}
	if policy != nil {
		guard, gerr := netsecurity.NewGuard(policy)
		if gerr != nil {
			guard = netsecurity.FailClosed(gerr)
		}
		t.guard = guard
	}
	registrar.RegisterLibrary(buildLibrary(t))
}

// transports picks a client's HTTP client from its verify option: the shared
// pool's (or, under a network policy, the guard's, whatever verify says).
type transports struct {
	guard *netsecurity.Guard
	paths fssecurity.Config
}

func (t *transports) client(ctx context.Context, verify object.Object) (*http.Client, object.Object) {
	switch v := verify.(type) {
	case nil, *object.Null:
		return t.secure(), nil
	case *object.Boolean:
		if on, _ := v.AsBool(); on {
			return t.secure(), nil
		}
		if t.guard != nil {
			return t.guard.InsecureHTTPClient(), nil
		}
		return pool.GetInsecureHTTPClient(), nil
	case *object.String:
		path, _ := v.AsString()
		return t.withCABundle(ctx, path)
	}
	return nil, scriptlingerrors.NewTypeError("BOOLEAN or STRING", verify.Type().String())
}

func (t *transports) secure() *http.Client {
	if t.guard != nil {
		return t.guard.HTTPClient()
	}
	return pool.GetHTTPClient()
}

// withCABundle is a client trusting only the CA certificates in the PEM file
// at path, read through the allowed paths and the interpreter's limits.
func (t *transports) withCABundle(ctx context.Context, path string) (*http.Client, object.Object) {
	if path == "" {
		return nil, scriptlingerrors.NewValueError("verify: the CA bundle path is empty")
	}
	if t.paths.CheckPath(path) != nil {
		return nil, scriptlingerrors.NewPermissionError("access denied: CA bundle '%s' is outside allowed directories", path)
	}
	var pem []byte
	var limitErr object.Object
	var err error
	object.RunBlocking(ctx, func() { pem, limitErr, err = object.ReadFileLimited(path) })
	if limitErr != nil {
		return nil, limitErr
	}
	if err != nil {
		return nil, &object.Error{Message: fmt.Sprintf("verify: cannot read CA bundle: %v", err)}
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(pem) {
		return nil, scriptlingerrors.NewValueError("verify: no PEM certificates in CA bundle '%s'", path)
	}
	if t.guard != nil {
		return t.guard.HTTPClientWithRoots(roots), nil
	}
	return pool.NewHTTPClientWithRoots(roots), nil
}

func buildLibrary(t *transports) *object.Library {
	b := object.NewLibraryBuilder(LibraryName, LibraryDesc)

	b.FunctionWithHelp("Client", func(ctx context.Context, kwargs object.Kwargs, issuer string, rest ...string) object.Object {
		clientID, errObj := kwargs.GetString("client_id", "")
		if errObj != nil {
			return errObj
		}
		clientSecret, errObj := optionalStringArg(kwargs, "client_secret")
		if errObj != nil {
			return errObj
		}
		switch {
		case len(rest) > 1:
			return scriptlingerrors.NewArgumentError(len(rest)+1, 2)
		case len(rest) == 1 && kwargs.Has("client_id"):
			return scriptlingerrors.NewValueError("client_id given both by position and by name")
		case len(rest) == 1:
			clientID = rest[0]
		}
		httpClient, errObj := t.client(ctx, kwargs.Get("verify"))
		if errObj != nil {
			return errObj
		}
		timeout, errObj := kwargs.GetFloat("timeout", defaultTimeout.Seconds())
		if errObj != nil {
			return errObj
		}
		if timeout <= 0 {
			return scriptlingerrors.NewValueError("timeout must be positive")
		}
		c, err := newClient(issuer, clientID, clientSecret,
			httpClient, time.Duration(timeout*float64(time.Second)))
		if err != nil {
			return &object.Error{Message: err.Error()}
		}
		return newInstance(getClientClass(), "OAuthClient", c)
	}, `Client(issuer, client_id, **kwargs) - An OAuth 2.0 / OpenID Connect client at an identity provider

Parameters:
  issuer (str): The IdP's issuer URL, e.g. "https://id.example.com/o/acme". Its endpoints are read from its
    discovery document. https only (plain http only on localhost)
  client_id (str): This script's client ID at the IdP
  client_secret (str, optional): The client's secret, for a confidential client (client credentials). Keep it
    out of the script: read it with the secret library or os.getenv
  verify (bool or str, optional): Verify the IdP's TLS certificate (default: True); False accepts any
    certificate (development only); a path names a PEM bundle of CA certificates to trust
  timeout (float, optional): Per-request timeout in seconds. Default: 10

Returns:
  OAuthClient

Example:
  idp = oauth.Client("https://id.example.com/o/acme", client_id="c_cli")
  flow = idp.device_start(scope="openid profile", resource="https://api.example.com/")
  print("Open", flow.verification_uri, "and enter", flow.user_code)
  token = flow.wait()
  requests.get("https://api.example.com/items", headers=token.headers())`)

	return b.Build()
}

// ── Instances ───────────────────────────────────────────────────────────────

func newInstance(class *object.Class, typeName string, v any) *object.Instance {
	return object.NewInstanceWithFields(class, map[string]object.Object{
		"_impl": &object.ClientWrapper{TypeName: typeName, Client: v},
	})
}

func impl[T any](self *object.Instance) (T, object.Object) {
	var zero T
	w, ok := object.GetClientField(self, "_impl")
	if !ok || w.Client == nil {
		return zero, &object.Error{Message: "oauth: missing internal state"}
	}
	v, ok := w.Client.(T)
	if !ok {
		return zero, &object.Error{Message: "oauth: invalid internal state"}
	}
	return v, nil
}

// blocking runs fn with the interpreter lock released (it waits on the network).
func blocking[T any](ctx context.Context, fn func() (T, error)) (T, error) {
	var v T
	var err error
	object.RunBlocking(ctx, func() { v, err = fn() })
	return v, err
}

func errObject(err error) object.Object {
	return &object.Error{Message: err.Error()}
}

func isNull(v object.Object) bool {
	_, ok := v.(*object.Null)
	return ok
}

// optionalStringArg reads a string keyword argument that may be None (not given).
func optionalStringArg(kwargs object.Kwargs, name string) (string, object.Object) {
	if v := kwargs.Get(name); v == nil || isNull(v) {
		return "", nil
	}
	return kwargs.GetString(name, "")
}

func optionalString(s string) object.Object {
	if s == "" {
		return &object.Null{}
	}
	return object.NewString(s)
}

// scopeArg reads scope: a space-separated string or a list of strings.
func scopeArg(kwargs object.Kwargs) (string, object.Object) {
	v := kwargs.Get("scope")
	parts, errObj := stringOrList(v, "scope")
	if errObj != nil {
		return "", errObj
	}
	return strings.Join(parts, " "), nil
}

// resourceArg reads resource: one URL or a list of them (RFC 8707).
func resourceArg(kwargs object.Kwargs) ([]string, object.Object) {
	return stringOrList(kwargs.Get("resource"), "resource")
}

func stringOrList(v object.Object, name string) ([]string, object.Object) {
	switch x := v.(type) {
	case nil, *object.Null:
		return nil, nil
	case *object.String:
		s, _ := x.AsString()
		if s == "" {
			return nil, nil
		}
		return []string{s}, nil
	case *object.List:
		out := make([]string, 0, len(x.Elements))
		for _, e := range x.Elements {
			s, err := e.AsString()
			if err != nil {
				return nil, scriptlingerrors.NewValueError("%s: every item must be a string", name)
			}
			out = append(out, s)
		}
		return out, nil
	}
	return nil, scriptlingerrors.NewTypeError("STRING or LIST", v.Type().String())
}

func dictArg(v object.Object, name string) (map[string]any, object.Object) {
	if v == nil {
		return nil, scriptlingerrors.NewValueError("%s: a dict is required", name)
	}
	d, err := v.AsDict()
	if err != nil {
		return nil, scriptlingerrors.NewTypeError("DICT", v.Type().String())
	}
	out := make(map[string]any, len(d))
	for k, val := range d {
		out[k] = conversion.ToGo(val)
	}
	return out, nil
}

// ── OAuthClient ─────────────────────────────────────────────────────────────

var (
	clientClass     *object.Class
	clientClassOnce sync.Once
)

func getClientClass() *object.Class {
	clientClassOnce.Do(func() { clientClass = buildClientClass() })
	return clientClass
}

func buildClientClass() *object.Class {
	cb := object.NewClassBuilder("OAuthClient")

	cb.Property("issuer", func(self *object.Instance) object.Object {
		c, errObj := impl[*client](self)
		if errObj != nil {
			return errObj
		}
		return object.NewString(c.issuer)
	})
	cb.Property("client_id", func(self *object.Instance) object.Object {
		c, errObj := impl[*client](self)
		if errObj != nil {
			return errObj
		}
		return object.NewString(c.clientID)
	})

	cb.MethodWithHelp("metadata", func(self *object.Instance, ctx context.Context) object.Object {
		c, errObj := impl[*client](self)
		if errObj != nil {
			return errObj
		}
		doc, err := blocking(ctx, func() (map[string]any, error) { return c.Metadata(ctx) })
		if err != nil {
			return errObject(err)
		}
		return conversion.FromGo(doc)
	}, `metadata() - The IdP's discovery document (its endpoints and what it supports), read once an hour

Returns:
  dict`)

	cb.MethodWithHelp("device_start", func(self *object.Instance, ctx context.Context, kwargs object.Kwargs) object.Object {
		c, errObj := impl[*client](self)
		if errObj != nil {
			return errObj
		}
		scope, errObj := scopeArg(kwargs)
		if errObj != nil {
			return errObj
		}
		resources, errObj := resourceArg(kwargs)
		if errObj != nil {
			return errObj
		}
		f, err := blocking(ctx, func() (*deviceFlow, error) { return c.DeviceStart(ctx, scope, resources) })
		if err != nil {
			return errObject(err)
		}
		return newInstance(getFlowClass(), "DeviceFlow", f)
	}, `device_start(**kwargs) - Start a device sign-in (RFC 8628)

The person opens verification_uri on any device, enters user_code and approves; the script waits with
wait() (a CLI) or checks with poll() (a web page that shows the link and reloads). No browser is opened.

Parameters:
  scope (str or list, optional): Scopes to ask for, e.g. "openid profile"
  resource (str or list, optional): The API(s) the token is for (RFC 8707); it becomes the token's audience

Returns:
  DeviceFlow`)

	cb.MethodWithHelp("device_resume", func(self *object.Instance, flow object.Object) object.Object {
		c, errObj := impl[*client](self)
		if errObj != nil {
			return errObj
		}
		d, errObj := dictArg(flow, "flow")
		if errObj != nil {
			return errObj
		}
		f, err := c.resumeDeviceFlow(d)
		if err != nil {
			return errObject(err)
		}
		return newInstance(getFlowClass(), "DeviceFlow", f)
	}, `device_resume(flow) - Pick up a device sign-in from DeviceFlow.to_dict(), e.g. on a later web request

Parameters:
  flow (dict): A dict from DeviceFlow.to_dict()

Returns:
  DeviceFlow`)

	cb.MethodWithHelp("client_credentials", func(self *object.Instance, ctx context.Context, kwargs object.Kwargs) object.Object {
		c, errObj := impl[*client](self)
		if errObj != nil {
			return errObj
		}
		scope, errObj := scopeArg(kwargs)
		if errObj != nil {
			return errObj
		}
		resources, errObj := resourceArg(kwargs)
		if errObj != nil {
			return errObj
		}
		tok, err := blocking(ctx, func() (*token, error) { return c.ClientCredentials(ctx, scope, resources) })
		if err != nil {
			return errObject(err)
		}
		return newInstance(getTokenClass(), "Token", tok)
	}, `client_credentials(**kwargs) - A token for the client itself, with no user (needs client_secret)

Parameters:
  scope (str or list, optional): Scopes to ask for
  resource (str or list, optional): The API(s) the token is for (RFC 8707)

Returns:
  Token`)

	cb.MethodWithHelp("token_from_dict", func(self *object.Instance, saved object.Object) object.Object {
		c, errObj := impl[*client](self)
		if errObj != nil {
			return errObj
		}
		d, errObj := dictArg(saved, "token")
		if errObj != nil {
			return errObj
		}
		tok, err := newToken(c, d)
		if err != nil {
			return errObject(err)
		}
		return newInstance(getTokenClass(), "Token", tok)
	}, `token_from_dict(token) - A token saved earlier with Token.to_dict(), able to refresh itself with this client

Parameters:
  token (dict): A dict from Token.to_dict()

Returns:
  Token`)

	cb.MethodWithHelp("verify", func(self *object.Instance, ctx context.Context, kwargs object.Kwargs, raw string, audience string) object.Object {
		c, errObj := impl[*client](self)
		if errObj != nil {
			return errObj
		}
		typ, errObj := kwargs.GetString("typ", "")
		if errObj != nil {
			return errObj
		}
		leeway, errObj := kwargs.GetFloat("leeway", defaultVerifyLeeway.Seconds())
		if errObj != nil {
			return errObj
		}
		if leeway < 0 {
			return scriptlingerrors.NewValueError("leeway must not be negative")
		}
		claims, err := blocking(ctx, func() (map[string]any, error) {
			return c.Verify(ctx, raw, audience, typ, time.Duration(leeway*float64(time.Second)))
		})
		if err != nil {
			return errObject(err)
		}
		return conversion.FromGo(claims)
	}, `verify(token, audience, **kwargs) - Check an access token sent to an API this script serves

Checks the signature against the issuer's published keys (refetched when a new key appears), that the issuer
is this client's, that aud contains audience, exp and nbf, and that the token is not bound to a key (cnf).
Raises on any failure; the error never contains the token.

Parameters:
  token (str): The access token (the Bearer value, without "Bearer ")
  audience (str): This API's identifier, as the IdP puts it in aud
  typ (str, optional): Require this JWT type, e.g. "at+jwt" (RFC 9068). Default: not checked
  leeway (float, optional): Allowed clock skew in seconds. Default: 60

Returns:
  dict: The token's claims (sub, scope, client_id, ...)`)

	return cb.Build()
}

// ── DeviceFlow ──────────────────────────────────────────────────────────────

var (
	flowClass     *object.Class
	flowClassOnce sync.Once
)

func getFlowClass() *object.Class {
	flowClassOnce.Do(func() { flowClass = buildFlowClass() })
	return flowClass
}

func buildFlowClass() *object.Class {
	cb := object.NewClassBuilder("DeviceFlow")

	field := func(get func(f *deviceFlow) object.Object) func(self *object.Instance) object.Object {
		return func(self *object.Instance) object.Object {
			f, errObj := impl[*deviceFlow](self)
			if errObj != nil {
				return errObj
			}
			f.mu.Lock()
			defer f.mu.Unlock()
			return get(f)
		}
	}
	cb.Property("user_code", field(func(f *deviceFlow) object.Object { return object.NewString(f.userCode) }))
	cb.Property("verification_uri", field(func(f *deviceFlow) object.Object { return object.NewString(f.verificationURI) }))
	cb.Property("verification_uri_complete", field(func(f *deviceFlow) object.Object {
		return optionalString(f.verificationURIComplete)
	}))
	cb.Property("expires_at", field(func(f *deviceFlow) object.Object { return object.NewInteger(f.expiresAt.Unix()) }))
	cb.Property("interval", field(func(f *deviceFlow) object.Object { return object.NewInteger(int64(f.interval / time.Second)) }))

	cb.MethodWithHelp("poll", func(self *object.Instance, ctx context.Context) object.Object {
		f, errObj := impl[*deviceFlow](self)
		if errObj != nil {
			return errObj
		}
		tok, err := blocking(ctx, func() (*token, error) { return f.Poll(ctx) })
		if err != nil {
			return errObject(err)
		}
		if tok == nil {
			return &object.Null{}
		}
		return newInstance(getTokenClass(), "Token", tok)
	}, `poll() - Check once whether the sign-in was approved

Asks the IdP no sooner than its interval allows; an earlier call (a page reloading) just returns None.

Returns:
  Token once approved, None while waiting. Raises if the sign-in was denied or the code expired.`)

	cb.MethodWithHelp("wait", func(self *object.Instance, ctx context.Context, kwargs object.Kwargs) object.Object {
		f, errObj := impl[*deviceFlow](self)
		if errObj != nil {
			return errObj
		}
		var timeout float64
		if v := kwargs.Get("timeout"); v != nil && !isNull(v) {
			if timeout, errObj = kwargs.GetFloat("timeout", 0); errObj != nil {
				return errObj
			}
		}
		tok, err := blocking(ctx, func() (*token, error) {
			return f.Wait(ctx, time.Duration(timeout*float64(time.Second)))
		})
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return scriptlingerrors.NewCancelledError()
			}
			return errObject(err)
		}
		return newInstance(getTokenClass(), "Token", tok)
	}, `wait(**kwargs) - Wait until the sign-in is approved, polling at the IdP's interval

Parameters:
  timeout (float, optional): Give up after this many seconds. Default: until the code expires

Returns:
  Token. Raises if the sign-in was denied, the code expired or the timeout passed.`)

	cb.MethodWithHelp("to_dict", func(self *object.Instance) object.Object {
		f, errObj := impl[*deviceFlow](self)
		if errObj != nil {
			return errObj
		}
		return conversion.FromGo(f.dict())
	}, `to_dict() - The flow as a dict, to keep between web requests and resume with OAuthClient.device_resume()

It holds the device code: keep it server side, never in a page or cookie.

Returns:
  dict`)

	return cb.Build()
}

// ── Token ───────────────────────────────────────────────────────────────────

var (
	tokenClass     *object.Class
	tokenClassOnce sync.Once
)

func getTokenClass() *object.Class {
	tokenClassOnce.Do(func() { tokenClass = buildTokenClass() })
	return tokenClass
}

func buildTokenClass() *object.Class {
	cb := object.NewClassBuilder("Token")

	field := func(get func(t *token) object.Object) func(self *object.Instance) object.Object {
		return func(self *object.Instance) object.Object {
			t, errObj := impl[*token](self)
			if errObj != nil {
				return errObj
			}
			t.mu.Lock()
			defer t.mu.Unlock()
			return get(t)
		}
	}
	cb.Property("access_token", field(func(t *token) object.Object { return object.NewString(t.accessToken) }))
	cb.Property("token_type", field(func(t *token) object.Object { return object.NewString(t.tokenType) }))
	cb.Property("refresh_token", field(func(t *token) object.Object { return optionalString(t.refreshToken) }))
	cb.Property("scope", field(func(t *token) object.Object { return optionalString(t.scope) }))
	cb.Property("id_token", field(func(t *token) object.Object { return optionalString(t.idToken) }))
	cb.Property("expires_at", field(func(t *token) object.Object {
		if t.expiresAt.IsZero() {
			return &object.Null{}
		}
		return object.NewInteger(t.expiresAt.Unix())
	}))

	cb.MethodWithHelp("headers", func(self *object.Instance, ctx context.Context) object.Object {
		t, errObj := impl[*token](self)
		if errObj != nil {
			return errObj
		}
		header, err := blocking(ctx, func() (string, error) { return t.Header(ctx) })
		if err != nil {
			return errObject(err)
		}
		return object.NewStringDict(map[string]object.Object{"Authorization": object.NewString(header)})
	}, `headers() - {"Authorization": "Bearer <access token>"}, for requests

Refreshes the token first when it is about to expire and has a refresh token.

Returns:
  dict`)

	cb.MethodWithHelp("is_expired", func(self *object.Instance, kwargs object.Kwargs) object.Object {
		t, errObj := impl[*token](self)
		if errObj != nil {
			return errObj
		}
		margin, errObj := kwargs.GetFloat("margin", defaultExpiryMargin.Seconds())
		if errObj != nil {
			return errObj
		}
		return object.NewBoolean(t.expired(time.Duration(margin * float64(time.Second))))
	}, `is_expired(**kwargs) - Whether the access token expires within margin seconds

Parameters:
  margin (float, optional): Seconds. Default: 30

Returns:
  bool: False when the IdP did not say when it expires`)

	cb.MethodWithHelp("refresh", func(self *object.Instance, ctx context.Context, kwargs object.Kwargs) object.Object {
		t, errObj := impl[*token](self)
		if errObj != nil {
			return errObj
		}
		scope, errObj := scopeArg(kwargs)
		if errObj != nil {
			return errObj
		}
		if _, err := blocking(ctx, func() (struct{}, error) { return struct{}{}, t.Refresh(ctx, scope) }); err != nil {
			return errObject(err)
		}
		return &object.Null{}
	}, `refresh(**kwargs) - Get a new access token with the refresh token, in place

Parameters:
  scope (str or list, optional): Narrower scopes to ask for. Default: the same

Raises if the token has no refresh token or the IdP refuses it.`)

	cb.MethodWithHelp("to_dict", func(self *object.Instance) object.Object {
		t, errObj := impl[*token](self)
		if errObj != nil {
			return errObj
		}
		return conversion.FromGo(t.dict())
	}, `to_dict() - The token as a dict, to save between runs if the script chooses to

Holds the access and refresh tokens: store it where only this script can read it, and load it again with
OAuthClient.token_from_dict().

Returns:
  dict: access_token, token_type, expires_at, expires_in, refresh_token, scope, id_token (when present)`)

	cb.MethodWithHelp("claims", func(self *object.Instance) object.Object {
		t, errObj := impl[*token](self)
		if errObj != nil {
			return errObj
		}
		claims, err := t.claims()
		if err != nil {
			return errObject(err)
		}
		return conversion.FromGo(claims)
	}, `claims() - What a JWT access token says (iss, aud, sub, exp, ...), NOT verified

For showing or debugging only; an API checks a token it receives with OAuthClient.verify().

Returns:
  dict`)

	return cb.Build()
}
