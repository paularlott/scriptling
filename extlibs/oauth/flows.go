package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"
)

const (
	grantDeviceCode        = "urn:ietf:params:oauth:grant-type:device_code"
	defaultDeviceInterval  = 5 * time.Second
	slowDownStep           = 5 * time.Second // RFC 8628 §3.5
	defaultExpiryMargin    = 30 * time.Second
	defaultVerifyLeeway    = 60 * time.Second
	maxDeviceFlowLifetime  = time.Hour
	maxDeviceFlowIntervalS = 60
)

// ── Tokens ──────────────────────────────────────────────────────────────────

// token is an access token and what came with it. Kept with the client that
// obtained it, so it can refresh itself.
type token struct {
	mu           sync.Mutex
	refreshMu    sync.Mutex // one refresh at a time: a refresh token may be good for one use only
	c            *client
	accessToken  string
	tokenType    string
	refreshToken string
	scope        string
	idToken      string
	expiresAt    time.Time // zero: the IdP did not say
}

// newToken builds a token from a token endpoint answer.
func newToken(c *client, m map[string]any) (*token, error) {
	t := &token{c: c}
	if err := t.set(m); err != nil {
		return nil, err
	}
	return t, nil
}

// set takes the fields of a token endpoint answer (or a saved token dict).
// A refresh answer without a refresh token keeps the old one (RFC 6749 §6).
func (t *token) set(m map[string]any) error {
	access, _ := m["access_token"].(string)
	if access == "" {
		return errors.New("oauth: the token endpoint answered without an access_token")
	}
	t.accessToken = access
	t.tokenType = "Bearer"
	if typ, _ := m["token_type"].(string); typ != "" && !strings.EqualFold(typ, "bearer") {
		t.tokenType = typ
	}
	if rt, _ := m["refresh_token"].(string); rt != "" {
		t.refreshToken = rt
	}
	if s, ok := m["scope"].(string); ok {
		t.scope = s
	}
	if id, _ := m["id_token"].(string); id != "" {
		t.idToken = id
	}
	t.expiresAt = time.Time{}
	if at, ok := number(m["expires_at"]); ok && at > 0 {
		t.expiresAt = time.Unix(int64(at), 0)
	} else if in, ok := number(m["expires_in"]); ok && in > 0 {
		t.expiresAt = t.c.now().Add(time.Duration(in) * time.Second)
	}
	return nil
}

// expired reports whether the token expires within margin. A token whose
// lifetime is unknown is never taken as expired.
func (t *token) expired(margin time.Duration) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return !t.expiresAt.IsZero() && !t.c.now().Add(margin).Before(t.expiresAt)
}

// Refresh exchanges the refresh token for a new access token, in place.
func (t *token) Refresh(ctx context.Context, scope string) error {
	t.refreshMu.Lock()
	defer t.refreshMu.Unlock()
	return t.refreshLocked(ctx, scope)
}

// refreshLocked is Refresh with refreshMu held.
func (t *token) refreshLocked(ctx context.Context, scope string) error {
	t.mu.Lock()
	rt := t.refreshToken
	t.mu.Unlock()
	if rt == "" {
		return errors.New("oauth: the token has no refresh_token to refresh with")
	}
	form := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {rt}}
	if scope != "" {
		form.Set("scope", scope)
	}
	m, err := t.c.PostForm(ctx, "token_endpoint", form)
	if err != nil {
		return err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.set(m)
}

// Header is the Authorization header value, refreshing the token first when
// it is about to expire and can be refreshed.
func (t *token) Header(ctx context.Context) (string, error) {
	if t.expired(defaultExpiryMargin) {
		t.refreshMu.Lock()
		// Another caller may have refreshed it while this one waited.
		t.mu.Lock()
		canRefresh := t.refreshToken != ""
		t.mu.Unlock()
		if canRefresh && t.expired(defaultExpiryMargin) {
			if err := t.refreshLocked(ctx, ""); err != nil {
				t.refreshMu.Unlock()
				return "", err
			}
		}
		t.refreshMu.Unlock()
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.tokenType + " " + t.accessToken, nil
}

// dict is the token in the shape Python's OAuth libraries use, for a script to
// keep between runs if it chooses to: it holds the tokens themselves.
func (t *token) dict() map[string]any {
	t.mu.Lock()
	defer t.mu.Unlock()
	d := map[string]any{"access_token": t.accessToken, "token_type": t.tokenType}
	if !t.expiresAt.IsZero() {
		d["expires_at"] = t.expiresAt.Unix()
		d["expires_in"] = max(int64(0), int64(math.Ceil(t.expiresAt.Sub(t.c.now()).Seconds())))
	}
	for k, v := range map[string]string{"refresh_token": t.refreshToken, "scope": t.scope, "id_token": t.idToken} {
		if v != "" {
			d[k] = v
		}
	}
	return d
}

// claims decodes the access token's payload without verifying it, to show
// what it says. Never a basis for trusting it: that is verify().
func (t *token) claims() (map[string]any, error) {
	t.mu.Lock()
	access := t.accessToken
	t.mu.Unlock()
	parsed, err := parseJWT(access)
	if err != nil {
		return nil, errors.New("oauth: the access token is not a JWT")
	}
	return parsed.claims, nil
}

// ── Device sign-in (RFC 8628) ──────────────────────────────────────────────

// deviceFlow is a device sign-in waiting for the person to approve it.
type deviceFlow struct {
	mu                      sync.Mutex
	c                       *client
	deviceCode              string
	userCode                string
	verificationURI         string
	verificationURIComplete string
	expiresAt               time.Time
	interval                time.Duration
	nextPoll                time.Time
	finished                bool
	polling                 bool // a poll is waiting on the IdP: no second one meanwhile
}

// DeviceStart asks the IdP for a device code.
func (c *client) DeviceStart(ctx context.Context, scope string, resources []string) (*deviceFlow, error) {
	form := url.Values{}
	if scope != "" {
		form.Set("scope", scope)
	}
	for _, r := range resources {
		form.Add("resource", r)
	}
	m, err := c.PostForm(ctx, "device_authorization_endpoint", form)
	if err != nil {
		return nil, err
	}
	f := &deviceFlow{c: c}
	f.deviceCode, _ = m["device_code"].(string)
	f.userCode, _ = m["user_code"].(string)
	f.verificationURI, _ = m["verification_uri"].(string)
	f.verificationURIComplete, _ = m["verification_uri_complete"].(string)
	if f.deviceCode == "" || f.userCode == "" || f.verificationURI == "" {
		return nil, errors.New("oauth: the device authorization answer is incomplete")
	}
	expiresIn, ok := number(m["expires_in"])
	if !ok || expiresIn <= 0 || expiresIn > maxDeviceFlowLifetime.Seconds() {
		expiresIn = 600
	}
	f.expiresAt = c.now().Add(time.Duration(expiresIn) * time.Second)
	f.interval = defaultDeviceInterval
	if i, ok := number(m["interval"]); ok && i >= 1 && i <= maxDeviceFlowIntervalS {
		f.interval = time.Duration(i) * time.Second
	}
	f.nextPoll = c.now().Add(f.interval)
	return f, nil
}

var (
	errDeviceDenied  = errors.New("oauth: the sign-in was denied")
	errDeviceExpired = errors.New("oauth: the code expired before it was approved")
	errDeviceDone    = errors.New("oauth: this device sign-in has already finished")
)

// Poll asks the IdP once whether the sign-in was approved: the token once it
// is, nil while it is not. It does not ask sooner than the IdP's interval: an
// early call (a web page reloading) returns nil without a request.
func (f *deviceFlow) Poll(ctx context.Context) (*token, error) {
	// The lock is not held while the IdP answers: reading the flow's fields
	// (a page showing user_code) must not wait on the network.
	f.mu.Lock()
	if f.finished {
		f.mu.Unlock()
		return nil, errDeviceDone
	}
	now := f.c.now()
	if !now.Before(f.expiresAt) {
		f.finished = true
		f.mu.Unlock()
		return nil, errDeviceExpired
	}
	if f.polling || now.Before(f.nextPoll) {
		f.mu.Unlock()
		return nil, nil
	}
	f.polling = true
	deviceCode := f.deviceCode
	f.mu.Unlock()

	m, err := f.c.PostForm(ctx, "token_endpoint", url.Values{"grant_type": {grantDeviceCode}, "device_code": {deviceCode}})

	f.mu.Lock()
	defer f.mu.Unlock()
	f.polling = false
	var oe *oauthError
	switch {
	case err == nil:
		f.finished = true
		return newToken(f.c, m)
	case errors.As(err, &oe) && oe.Code == "authorization_pending":
		f.nextPoll = f.c.now().Add(f.interval)
		return nil, nil
	case errors.As(err, &oe) && oe.Code == "slow_down":
		f.interval = min(f.interval+slowDownStep, maxDeviceFlowIntervalS*time.Second)
		f.nextPoll = f.c.now().Add(f.interval)
		return nil, nil
	case errors.As(err, &oe) && oe.Code == "access_denied":
		f.finished = true
		return nil, errDeviceDenied
	case errors.As(err, &oe) && oe.Code == "expired_token":
		f.finished = true
		return nil, errDeviceExpired
	case errors.As(err, &oe):
		// Any other refusal ends the sign-in (RFC 8628 §3.5): polling again cannot help.
		f.finished = true
		return nil, err
	}
	// A network failure or timeout: try again at the next interval.
	f.nextPoll = f.c.now().Add(f.interval)
	return nil, err
}

// Wait polls until the sign-in is approved, refused or expired, or timeout
// (zero: until the code expires) passes. It sleeps between polls; call it with
// the interpreter lock released.
func (f *deviceFlow) Wait(ctx context.Context, timeout time.Duration) (*token, error) {
	var deadline <-chan time.Time
	if timeout > 0 {
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		deadline = timer.C
	}
	for {
		tok, err := f.Poll(ctx)
		if err != nil || tok != nil {
			return tok, err
		}
		f.mu.Lock()
		wait := max(f.nextPoll.Sub(f.c.now()), 50*time.Millisecond)
		f.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-deadline:
			return nil, errors.New("oauth: timed out waiting for the sign-in to be approved")
		case <-time.After(wait):
		}
	}
}

// dict is the flow for a script that keeps it between requests (a web page
// polling on each load). It holds the device code: keep it server side.
func (f *deviceFlow) dict() map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()
	d := map[string]any{
		"device_code":      f.deviceCode,
		"user_code":        f.userCode,
		"verification_uri": f.verificationURI,
		"expires_at":       f.expiresAt.Unix(),
		"interval":         int64(f.interval / time.Second),
	}
	if f.verificationURIComplete != "" {
		d["verification_uri_complete"] = f.verificationURIComplete
	}
	return d
}

// resumeDeviceFlow rebuilds a flow from its dict.
func (c *client) resumeDeviceFlow(m map[string]any) (*deviceFlow, error) {
	f := &deviceFlow{c: c, interval: defaultDeviceInterval}
	f.deviceCode, _ = m["device_code"].(string)
	f.userCode, _ = m["user_code"].(string)
	f.verificationURI, _ = m["verification_uri"].(string)
	f.verificationURIComplete, _ = m["verification_uri_complete"].(string)
	at, ok := number(m["expires_at"])
	if f.deviceCode == "" || f.userCode == "" || f.verificationURI == "" || !ok {
		return nil, errors.New("oauth: not a device sign-in dict (see DeviceFlow.to_dict)")
	}
	// A dict may have been kept anywhere: never longer than a code can live.
	f.expiresAt = time.Unix(int64(min(at, float64(c.now().Add(maxDeviceFlowLifetime).Unix()))), 0)
	if i, ok := number(m["interval"]); ok && i >= 1 && i <= maxDeviceFlowIntervalS {
		f.interval = time.Duration(i) * time.Second
	}
	return f, nil
}

// ── Client credentials (RFC 6749 §4.4) ─────────────────────────────────────

// ClientCredentials gets a token for the client itself, with no user.
func (c *client) ClientCredentials(ctx context.Context, scope string, resources []string) (*token, error) {
	if c.clientSecret == "" {
		return nil, errors.New("oauth: client_credentials needs the client's client_secret")
	}
	form := url.Values{"grant_type": {"client_credentials"}}
	if scope != "" {
		form.Set("scope", scope)
	}
	for _, r := range resources {
		form.Add("resource", r)
	}
	m, err := c.PostForm(ctx, "token_endpoint", form)
	if err != nil {
		return nil, err
	}
	return newToken(c, m)
}

// ── Verifying access tokens (a script serving an API) ───────────────────────

// Verify checks a JWT access token the issuer signed for audience and returns
// its claims: the signature against the issuer's published keys, iss, aud,
// exp and nbf (within leeway), typ when asked for, and no key binding (cnf),
// which this cannot check. Errors never include the token.
func (c *client) Verify(ctx context.Context, raw, audience, typ string, leeway time.Duration) (map[string]any, error) {
	refused := func(format string, args ...any) error {
		return fmt.Errorf("oauth: token refused: "+format, args...)
	}
	if audience == "" {
		return nil, errors.New("oauth: verify needs the audience the token must be for")
	}
	t, err := parseJWT(raw)
	if err != nil {
		return nil, refused("%v", err)
	}
	if typ != "" {
		got, _ := t.header["typ"].(string)
		if !strings.EqualFold(got, typ) && !strings.EqualFold(got, "application/"+typ) {
			return nil, refused("its type is %q, not %q", got, typ)
		}
	}
	keys, err := c.Keys(ctx, false)
	if err != nil {
		return nil, err
	}
	err = verifySignature(t, keys)
	if errors.Is(err, errUnknownKey) {
		if keys, err = c.Keys(ctx, true); err == nil {
			err = verifySignature(t, keys)
		}
	}
	if err != nil {
		return nil, refused("%v", err)
	}

	claims := t.claims
	now := c.now()
	c.mu.Lock()
	issuer := c.canonicalIssuer
	c.mu.Unlock()
	if iss, _ := claims["iss"].(string); iss != issuer {
		return nil, refused("it is from another issuer")
	}
	if !slices.Contains(audiences(claims["aud"]), audience) {
		return nil, refused("it is not for %s", audience)
	}
	exp, ok := number(claims["exp"])
	if !ok {
		return nil, refused("it has no expiry")
	}
	if !now.Add(-leeway).Before(time.Unix(int64(exp), 0)) {
		return nil, refused("it has expired")
	}
	if nbf, ok := number(claims["nbf"]); ok && now.Add(leeway).Before(time.Unix(int64(nbf), 0)) {
		return nil, refused("it is not valid yet")
	}
	if _, bound := claims["cnf"]; bound {
		return nil, refused("it is bound to a key, which this cannot check")
	}
	return claims, nil
}

// audiences is the aud claim as a list: a string or a list of strings.
func audiences(v any) []string {
	switch a := v.(type) {
	case string:
		return []string{a}
	case []any:
		out := make([]string, 0, len(a))
		for _, x := range a {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// number reads a JSON number (json.Number or float64), or a numeric Go value.
func number(v any) (float64, bool) {
	switch n := v.(type) {
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	case float64:
		return n, true
	case int64:
		return float64(n), true
	case int:
		return float64(n), true
	}
	return 0, false
}
