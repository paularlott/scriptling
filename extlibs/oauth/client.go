package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	// maxResponseBytes bounds every response read from the IdP.
	maxResponseBytes = 1 << 20
	// jwksRefetchInterval is the least time between fetches of the JWK set
	// for tokens signed with a key it does not hold.
	jwksRefetchInterval = time.Minute
	// metadataTTL is how long the discovery document and JWK set are kept.
	metadataTTL = time.Hour
)

// client is one oauth.Client: an issuer, this script's client at it, and the
// HTTP client its requests go through (the network policy's, when there is one).
type client struct {
	issuer       string // as configured, without a trailing slash
	clientID     string
	clientSecret string
	http         *http.Client // follows redirects: discovery and keys only
	post         *http.Client // never follows redirects: requests carrying codes, secrets and tokens
	timeout      time.Duration
	now          func() time.Time

	mu              sync.Mutex
	canonicalIssuer string // exactly as the discovery document names it: what iss must be
	metadata        map[string]any
	metadataAt      time.Time
	keys            []publicKey
	keysAt          time.Time
	keysRefetch     time.Time
}

func newClient(issuer, clientID, clientSecret string, httpClient *http.Client, timeout time.Duration) (*client, error) {
	issuer = strings.TrimRight(strings.TrimSpace(issuer), "/")
	if err := checkEndpoint(issuer); err != nil {
		return nil, fmt.Errorf("oauth: issuer: %w", err)
	}
	if u, _ := url.Parse(issuer); u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("oauth: issuer: must have no query or fragment")
	}
	if strings.TrimSpace(clientID) == "" {
		return nil, errors.New("oauth: client_id is required")
	}
	// Discovery and key-set fetches may follow redirects, but only to acceptable
	// endpoints (never https to plain http); requests carrying codes, secrets
	// and tokens never follow one.
	get := *httpClient
	get.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("too many redirects")
		}
		return checkEndpoint(req.URL.String())
	}
	post := *httpClient
	post.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &client{
		issuer: issuer, clientID: clientID, clientSecret: clientSecret,
		http: &get, post: &post, timeout: timeout, now: time.Now,
	}, nil
}

// checkEndpoint accepts https URLs, and http ones only on the loopback
// interface (a development IdP on this machine).
func checkEndpoint(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return errors.New("not an absolute URL")
	}
	switch u.Scheme {
	case "https":
		return nil
	case "http":
		host := u.Hostname()
		if ip := net.ParseIP(host); host == "localhost" || (ip != nil && ip.IsLoopback()) {
			return nil
		}
		return errors.New("must use https (plain http only on localhost)")
	}
	return errors.New("must use https")
}

// oauthError is a refusal by the IdP: its error code and description, never
// the request's codes, secrets or tokens.
type oauthError struct {
	Code        string
	Description string
	Status      int
}

func (e *oauthError) Error() string {
	msg := "oauth: " + e.Code
	if e.Description != "" {
		msg += ": " + e.Description
	}
	return msg
}

// Metadata returns the issuer's discovery document, cached for an hour. It is
// read from OpenID Connect discovery, else RFC 8414 metadata, and must name
// this issuer.
func (c *client) Metadata(ctx context.Context) (map[string]any, error) {
	c.mu.Lock()
	if c.metadata != nil && c.now().Sub(c.metadataAt) < metadataTTL {
		m := c.metadata
		c.mu.Unlock()
		return m, nil
	}
	c.mu.Unlock()

	u, _ := url.Parse(c.issuer)
	candidates := []string{
		c.issuer + "/.well-known/openid-configuration",
		u.Scheme + "://" + u.Host + "/.well-known/oauth-authorization-server" + u.Path,
	}
	var lastErr error
	for _, candidate := range candidates {
		var doc map[string]any
		status, err := c.getJSON(ctx, candidate, &doc)
		if err != nil {
			lastErr = err
			continue
		}
		if status != http.StatusOK {
			lastErr = fmt.Errorf("oauth: discovery %s answered %d", candidate, status)
			continue
		}
		got, _ := doc["issuer"].(string)
		if strings.TrimRight(got, "/") != c.issuer {
			return nil, fmt.Errorf("oauth: the discovery document is for issuer %q, not %q: use that as the issuer", got, c.issuer)
		}
		c.mu.Lock()
		c.canonicalIssuer, c.metadata, c.metadataAt = got, doc, c.now()
		c.mu.Unlock()
		return doc, nil
	}
	return nil, lastErr
}

// endpoint is a URL from the discovery document, which must be acceptable.
func (c *client) endpoint(ctx context.Context, name string) (string, error) {
	doc, err := c.Metadata(ctx)
	if err != nil {
		return "", err
	}
	raw, _ := doc[name].(string)
	if raw == "" {
		return "", fmt.Errorf("oauth: the IdP publishes no %s", name)
	}
	if err := checkEndpoint(raw); err != nil {
		return "", fmt.Errorf("oauth: %s: %w", name, err)
	}
	return raw, nil
}

// Keys returns the issuer's signing keys, cached for an hour; refresh fetches
// them again (at most once a minute), for a token signed with a new key.
func (c *client) Keys(ctx context.Context, refresh bool) ([]publicKey, error) {
	c.mu.Lock()
	now := c.now()
	fresh := c.keys != nil && now.Sub(c.keysAt) < metadataTTL
	if fresh && (!refresh || now.Before(c.keysRefetch)) {
		keys := c.keys
		c.mu.Unlock()
		return keys, nil
	}
	c.mu.Unlock()

	jwksURI, err := c.endpoint(ctx, "jwks_uri")
	if err != nil {
		return nil, err
	}
	raw, status, err := c.get(ctx, jwksURI)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("oauth: jwks_uri answered %d", status)
	}
	keys, err := parseJWKS(raw)
	if err != nil {
		return nil, fmt.Errorf("oauth: %w", err)
	}
	c.mu.Lock()
	c.keys, c.keysAt, c.keysRefetch = keys, now, now.Add(jwksRefetchInterval)
	c.mu.Unlock()
	return keys, nil
}

// PostForm posts form to an endpoint of the IdP, authenticating as this
// client (client_secret_post when it has a secret), and decodes the JSON
// answer. A refusal is an *oauthError.
func (c *client) PostForm(ctx context.Context, endpointName string, form url.Values) (map[string]any, error) {
	endpoint, err := c.endpoint(ctx, endpointName)
	if err != nil {
		return nil, err
	}
	form.Set("client_id", c.clientID)
	if c.clientSecret != "" {
		form.Set("client_secret", c.clientSecret)
	}
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("oauth: %s: %w", endpointName, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := c.post.Do(req)
	if err != nil {
		return nil, fmt.Errorf("oauth: %s: %w", endpointName, sanitize(err))
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("oauth: %s: %w", endpointName, err)
	}
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		code, _ := out["error"].(string)
		desc, _ := out["error_description"].(string)
		if code == "" {
			code = fmt.Sprintf("%s answered %d", endpointName, resp.StatusCode)
		}
		return nil, &oauthError{Code: code, Description: desc, Status: resp.StatusCode}
	}
	if out == nil {
		return nil, fmt.Errorf("oauth: %s did not answer with JSON", endpointName)
	}
	return out, nil
}

func (c *client) getJSON(ctx context.Context, rawURL string, into any) (int, error) {
	raw, status, err := c.get(ctx, rawURL)
	if err != nil || status != http.StatusOK {
		return status, err
	}
	if err := json.Unmarshal(raw, into); err != nil {
		return status, fmt.Errorf("oauth: %s did not answer with JSON", rawURL)
	}
	return status, nil
}

func (c *client) get(ctx context.Context, rawURL string) ([]byte, int, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("oauth: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("oauth: %w", sanitize(err))
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, 0, fmt.Errorf("oauth: %w", err)
	}
	return raw, resp.StatusCode, nil
}

// sanitize drops the URL (and with it any query) from a transport error.
func sanitize(err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return urlErr.Err
	}
	return err
}
