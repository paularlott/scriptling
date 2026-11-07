package server

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	mcplib "github.com/paularlott/mcp"
	"github.com/paularlott/scriptling/conversion"
	"github.com/paularlott/scriptling/object"
)

// protectedResourceWellKnown is the RFC 9728 metadata location MCP clients
// try when the challenge does not name one.
const protectedResourceWellKnown = "/.well-known/oauth-protected-resource"

// staticTokenSubject is the TokenInfo subject a request authenticated with
// the configured --bearer-token carries.
const staticTokenSubject = "bearer"

// authConfigured reports whether the HTTP server requires a credential.
func (c ServerConfig) authConfigured() bool {
	return c.BearerToken != "" || c.OIDCIssuer != ""
}

// buildResourceServer turns the auth configuration into the mcp library's
// OAuth protected resource: a static --bearer-token, an identity provider
// (--oidc-issuer, whose JWT access tokens are verified against its JWKS), or
// both — a request is then accepted with either. It returns nil when no
// credential is configured (the server is unauthenticated, as before).
func buildResourceServer(c ServerConfig) (*mcplib.ResourceServer, error) {
	if c.OIDCIssuer == "" {
		if c.OIDCJWKSURL != "" || c.OIDCAudience != "" || len(c.OIDCRequiredScopes) > 0 || c.OIDCRequireATJWT || c.OIDCCAFile != "" || c.OIDCTLSInsecure {
			return nil, errors.New("--oidc-jwks-url, --oidc-audience, --oidc-required-scopes, --oidc-require-at-jwt, --oidc-ca-file and --oidc-tls-insecure need --oidc-issuer")
		}
	}
	if !c.authConfigured() {
		return nil, nil
	}

	rs := &mcplib.ResourceServer{ResourceName: "scriptling"}
	var validators []mcplib.TokenValidator
	if c.BearerToken != "" {
		// The static token is the operator's own all-access credential: it
		// is granted the required scopes so RequiredScopes (which apply to
		// identity-provider tokens) never shut it out.
		validators = append(validators, mcplib.StaticTokenValidator(map[string]*mcplib.TokenInfo{
			c.BearerToken: {Subject: staticTokenSubject, Scopes: c.OIDCRequiredScopes},
		}))
	}
	if c.OIDCIssuer != "" {
		// There is no public-URL setting to derive the resource identifier
		// from, so it must be given: it is what tokens' aud must name and
		// what the protected resource metadata advertises.
		if c.OIDCAudience == "" {
			return nil, errors.New("--oidc-issuer requires --oidc-audience: the URL clients reach this server's /mcp endpoint at (e.g. https://mcp.example.com/mcp)")
		}
		resource, err := canonicalResource(c.OIDCAudience)
		if err != nil {
			return nil, err
		}
		client, err := oidcHTTPClient(c)
		if err != nil {
			return nil, err
		}
		jwt, err := mcplib.NewJWTValidator(mcplib.JWTValidatorConfig{
			Issuer:                 c.OIDCIssuer,
			JWKSURL:                c.OIDCJWKSURL,
			HTTPClient:             client,
			RequireAccessTokenType: c.OIDCRequireATJWT,
		})
		if err != nil {
			return nil, err
		}
		validators = append(validators, requireAudience(jwt.Validate))
		rs.Resource = resource
		rs.AuthorizationServers = []string{c.OIDCIssuer}
		rs.RequiredScopes = c.OIDCRequiredScopes
		rs.ScopesSupported = c.OIDCRequiredScopes
	}
	rs.ValidateToken = firstValidToken(validators...)
	return rs, nil
}

// oidcHTTPClient builds the client the JWT validator fetches the issuer's
// discovery document and JWKS with. --oidc-ca-file adds the PEM file's
// certificates to the system roots (an identity provider with a private or
// self-signed CA); --oidc-tls-insecure skips verification altogether and
// wins when both are set. The default transport is cloned so
// proxy-from-environment and the platform's tuning survive.
func oidcHTTPClient(c ServerConfig) (*http.Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if c.OIDCCAFile != "" || c.OIDCTLSInsecure {
		tlsCfg := &tls.Config{MinVersion: tls.VersionTLS12}
		if transport.TLSClientConfig != nil {
			tlsCfg = transport.TLSClientConfig.Clone()
		}
		if c.OIDCCAFile != "" {
			pem, err := os.ReadFile(c.OIDCCAFile)
			if err != nil {
				return nil, fmt.Errorf("--oidc-ca-file: %w", err)
			}
			pool, err := x509.SystemCertPool()
			if err != nil || pool == nil {
				pool = x509.NewCertPool()
			}
			if !pool.AppendCertsFromPEM(pem) {
				return nil, fmt.Errorf("--oidc-ca-file %s: no PEM certificates found", c.OIDCCAFile)
			}
			tlsCfg.RootCAs = pool
		}
		if c.OIDCTLSInsecure {
			Log.Warn("TLS certificate verification is disabled for identity provider discovery and JWKS fetches (--oidc-tls-insecure); use --oidc-ca-file instead outside development", "issuer", c.OIDCIssuer)
			tlsCfg.InsecureSkipVerify = true //nolint:gosec // intentional: --oidc-tls-insecure
		}
		transport.TLSClientConfig = tlsCfg
	}
	return &http.Client{Transport: transport, Timeout: 10 * time.Second}, nil
}

// canonicalResource checks the resource identifier is an absolute http(s)
// URL and normalises it as RFC 8707 asks: lowercase scheme and host, no
// fragment, no trailing slash.
func canonicalResource(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return "", fmt.Errorf("--oidc-audience %q must be an absolute http(s) URL (the URL of this server's /mcp endpoint)", raw)
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("--oidc-audience %q must not have a query or fragment", raw)
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	u.Path = strings.TrimSuffix(u.Path, "/")
	u.RawPath = ""
	return u.String(), nil
}

// requireAudience refuses a JWT with no aud claim. ResourceServer treats an
// empty audience as "issued for this resource" (right for the static
// token), but an identity provider's token without one could have been
// issued for any service, so it is not accepted here.
func requireAudience(v mcplib.TokenValidator) mcplib.TokenValidator {
	return func(r *http.Request, token string) (*mcplib.TokenInfo, error) {
		info, err := v(r, token)
		if err != nil {
			return nil, err
		}
		if info == nil || len(info.Audience) == 0 {
			return nil, mcplib.ErrWrongAudience
		}
		return info, nil
	}
}

// firstValidToken tries each validator in turn and accepts the token with
// the first that does (the static token first: it is cheap and needs no
// network).
func firstValidToken(validators ...mcplib.TokenValidator) mcplib.TokenValidator {
	if len(validators) == 1 {
		return validators[0]
	}
	return func(r *http.Request, token string) (*mcplib.TokenInfo, error) {
		err := error(mcplib.ErrInvalidToken)
		for _, v := range validators {
			info, verr := v(r, token)
			if verr == nil && info != nil {
				return info, nil
			}
			if verr != nil {
				err = verr
			}
		}
		return nil, err
	}
}

// authMiddleware requires a valid credential on every route (the MCP and
// JSON-RPC endpoints, script routes, static files, the webroot and the
// not-found handler alike), answering a refusal as the MCP authorization
// spec and RFC 6750 require: 401 with a WWW-Authenticate Bearer challenge
// (naming the protected resource metadata when an identity provider is
// configured), or 403 insufficient_scope. The accepted token's TokenInfo is
// put on the request context (mcplib.TokenInfoFromContext) and exposed to
// scripts as request.auth.
//
// Two kinds of request pass without a token:
//   - The protected resource metadata (RFC 9728), when an identity provider
//     is configured: it is how a client learns where to get a token.
//   - A CORS preflight to /mcp or /json-rpc, recognised by the
//     Access-Control-Request-Method header a browser always sends on one,
//     not by method alone — any client can send a bare OPTIONS, and an
//     earlier version that exempted every OPTIONS let anyone skip the token
//     and reach static files or the not-found handler. Only those two
//     routes are built to answer a preflight safely (headers only, no body,
//     no side effect, never reaching a script); any other route has no such
//     handling, so a preflight-shaped request there still needs the token.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	rs := s.resourceServer
	var metadata http.Handler
	if rs.Resource != "" {
		metadata = rs.MetadataHandler()
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if metadata != nil && (r.URL.Path == rs.MetadataPath() || r.URL.Path == protectedResourceWellKnown) {
			metadata.ServeHTTP(w, r)
			return
		}
		isPreflight := r.Method == http.MethodOptions &&
			r.Header.Get("Access-Control-Request-Method") != "" &&
			((s.mcpHandler != nil && r.URL.Path == "/mcp") || (s.config.JSONRPC && r.URL.Path == "/json-rpc"))
		if isPreflight {
			next.ServeHTTP(w, r)
			return
		}
		info, err := rs.Authenticate(r)
		if err != nil {
			s.rejectAuth(w, r, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(mcplib.WithTokenInfo(r.Context(), info)))
	})
}

// rejectAuth answers a refused request. With an identity provider the
// library's Reject names the metadata document in the challenge; with only
// a static token there is no metadata to point at (and no authorization
// server to get a token from), so the challenge is a plain RFC 6750 one.
func (s *Server) rejectAuth(w http.ResponseWriter, r *http.Request, err error) {
	if s.resourceServer.Resource != "" {
		s.resourceServer.Reject(w, r, err)
		return
	}
	challenge := `Bearer realm="scriptling"`
	if !errors.Is(err, mcplib.ErrNoToken) {
		challenge += `, error="invalid_token", error_description="the token is not valid for this server"`
	}
	w.Header().Set("WWW-Authenticate", challenge)
	w.Header().Set("Cache-Control", "no-store")
	http.Error(w, "Unauthorized", http.StatusUnauthorized)
}

// tokenInfoObject converts the request's accepted token to the dict scripts
// see as request.auth: subject, audience, scopes, expiry (Unix seconds, or
// None when the token does not expire) and claims (the JWT's claims; empty
// for the static token). None when the request carries no token info (no
// auth configured, or a preflight).
func tokenInfoObject(info *mcplib.TokenInfo) object.Object {
	if info == nil {
		return &object.Null{}
	}
	audience := make([]object.Object, len(info.Audience))
	for i, a := range info.Audience {
		audience[i] = object.NewString(a)
	}
	scopes := make([]object.Object, len(info.Scopes))
	for i, sc := range info.Scopes {
		scopes[i] = object.NewString(sc)
	}
	var expiry object.Object = &object.Null{}
	if !info.Expiry.IsZero() {
		expiry = object.NewInteger(info.Expiry.Unix())
	}
	claims := map[string]any{}
	for k, v := range info.Claims {
		claims[k] = v
	}
	return object.NewStringDict(map[string]object.Object{
		"subject":  object.NewString(info.Subject),
		"audience": &object.List{Elements: audience},
		"scopes":   &object.List{Elements: scopes},
		"expiry":   expiry,
		"claims":   conversion.FromGo(claims),
	})
}
