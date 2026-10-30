package oauth

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/fssecurity"
	"github.com/paularlott/scriptling/extlibs/netsecurity"
	"github.com/paularlott/scriptling/pool"
	"github.com/paularlott/scriptling/stdlib"
)

// ── A fake identity provider ────────────────────────────────────────────────

type fakeIdP struct {
	srv    *httptest.Server
	issuer string
	key    *ecdsa.PrivateKey

	mu           sync.Mutex
	jwks         []map[string]any
	approveAfter int // device polls answered authorization_pending before approval
	polls        int
	slowDownOnce bool
	deny         bool
	lastForm     map[string]string
	tokenCalls   atomic.Int32
	jwksFetches  atomic.Int32
	refreshCalls atomic.Int32
	refreshDelay time.Duration
	deviceError  string        // answer device polls with this error
	deviceGate   chan struct{} // when set, device polls wait for it
	issuerSlash  bool          // the issuer's canonical form ends in "/"
}

func newFakeIdP(t *testing.T) *fakeIdP {
	t.Helper()
	f := &fakeIdP{key: newECKey(t)}
	f.jwks = []map[string]any{ecJWK(&f.key.PublicKey, "k1")}
	mux := http.NewServeMux()
	f.srv = httptest.NewTLSServer(mux)
	t.Cleanup(f.srv.Close)
	f.issuer = f.srv.URL + "/o/acme"

	mux.HandleFunc("/o/acme/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		issuer := f.issuer
		if f.issuerSlash {
			issuer += "/"
		}
		f.mu.Unlock()
		writeJSON(w, 200, map[string]any{
			"issuer":                        issuer,
			"token_endpoint":                f.issuer + "/oauth/token",
			"device_authorization_endpoint": f.issuer + "/oauth/device_authorization",
			"jwks_uri":                      f.issuer + "/oauth/jwks",
		})
	})
	mux.HandleFunc("/o/acme/oauth/jwks", func(w http.ResponseWriter, r *http.Request) {
		f.jwksFetches.Add(1)
		f.mu.Lock()
		defer f.mu.Unlock()
		writeJSON(w, 200, map[string]any{"keys": f.jwks})
	})
	mux.HandleFunc("/o/acme/oauth/device_authorization", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		f.record(r)
		writeJSON(w, 200, map[string]any{
			"device_code": "dev-code-secret", "user_code": "WDJB-MJHT",
			"verification_uri": f.issuer + "/device", "verification_uri_complete": f.issuer + "/device?user_code=WDJB-MJHT",
			"expires_in": 600, "interval": 1,
		})
	})
	mux.HandleFunc("/o/acme/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		f.record(r)
		f.tokenCalls.Add(1)
		f.mu.Lock()
		gate, delay := f.deviceGate, f.refreshDelay
		f.mu.Unlock()
		if gate != nil && r.PostForm.Get("grant_type") == grantDeviceCode {
			<-gate
		}
		if r.PostForm.Get("grant_type") == "refresh_token" {
			f.refreshCalls.Add(1)
			time.Sleep(delay)
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		switch r.PostForm.Get("grant_type") {
		case grantDeviceCode:
			f.polls++
			switch {
			case f.deviceError != "":
				writeJSON(w, 400, map[string]any{"error": f.deviceError})
			case f.deny:
				writeJSON(w, 400, map[string]any{"error": "access_denied"})
			case f.slowDownOnce:
				f.slowDownOnce = false
				writeJSON(w, 400, map[string]any{"error": "slow_down"})
			case f.polls <= f.approveAfter:
				writeJSON(w, 400, map[string]any{"error": "authorization_pending"})
			default:
				writeJSON(w, 200, map[string]any{"access_token": "at-1", "token_type": "Bearer", "expires_in": 600,
					"refresh_token": "rt-1", "scope": "openid profile"})
			}
		case "client_credentials":
			if r.PostForm.Get("client_secret") != "s3cret" {
				writeJSON(w, 401, map[string]any{"error": "invalid_client", "error_description": "wrong secret"})
				return
			}
			writeJSON(w, 200, map[string]any{"access_token": "svc-token", "token_type": "Bearer", "expires_in": 600})
		case "refresh_token":
			if r.PostForm.Get("refresh_token") != "rt-1" {
				writeJSON(w, 400, map[string]any{"error": "invalid_grant"})
				return
			}
			writeJSON(w, 200, map[string]any{"access_token": "at-2", "token_type": "Bearer", "expires_in": 600})
		default:
			writeJSON(w, 400, map[string]any{"error": "unsupported_grant_type"})
		}
	})
	return f
}

func (f *fakeIdP) record(r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastForm = map[string]string{}
	for k, v := range r.PostForm {
		f.lastForm[k] = strings.Join(v, " ")
	}
}

func (f *fakeIdP) form() map[string]string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lastForm
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// client is a Go-level client of the fake IdP with a controllable clock.
func (f *fakeIdP) client(t *testing.T, secret string) (*client, *time.Time) {
	t.Helper()
	c, err := newClient(f.issuer, "c_cli", secret, f.srv.Client(), 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	c.now = func() time.Time { return now }
	return c, &now
}

// ── Keys and signing ────────────────────────────────────────────────────────

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func newECKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func ecJWK(pub *ecdsa.PublicKey, kid string) map[string]any {
	return map[string]any{"kty": "EC", "crv": "P-256", "kid": kid, "alg": "ES256", "use": "sig",
		"x": b64(pub.X.FillBytes(make([]byte, 32))), "y": b64(pub.Y.FillBytes(make([]byte, 32)))}
}

// sign makes a compact JWT with header and claims, signed by key for alg.
func sign(t *testing.T, key crypto.Signer, alg string, header, claims map[string]any) string {
	t.Helper()
	hb, _ := json.Marshal(header)
	cb, _ := json.Marshal(claims)
	input := b64(hb) + "." + b64(cb)
	var sig []byte
	switch alg {
	case "ES256":
		d := sha256.Sum256([]byte(input))
		r, s, err := ecdsa.Sign(rand.Reader, key.(*ecdsa.PrivateKey), d[:])
		if err != nil {
			t.Fatal(err)
		}
		sig = append(r.FillBytes(make([]byte, 32)), s.FillBytes(make([]byte, 32))...)
	case "RS256":
		d := sha256.Sum256([]byte(input))
		sig, _ = rsa.SignPKCS1v15(rand.Reader, key.(*rsa.PrivateKey), crypto.SHA256, d[:])
	case "PS256":
		d := sha256.Sum256([]byte(input))
		sig, _ = rsa.SignPSS(rand.Reader, key.(*rsa.PrivateKey), crypto.SHA256, d[:],
			&rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash})
	case "EdDSA":
		sig = ed25519.Sign(key.(ed25519.PrivateKey), []byte(input))
	case "none":
		sig = nil
	}
	return input + "." + b64(sig)
}

func (f *fakeIdP) accessToken(t *testing.T, over map[string]any) string {
	t.Helper()
	claims := map[string]any{"iss": f.issuer, "sub": "user-1", "aud": []string{"c_cli", "https://api.example.com/"},
		"exp": time.Now().Add(10 * time.Minute).Unix(), "iat": time.Now().Unix(), "scope": "openid"}
	for k, v := range over {
		if v == nil {
			delete(claims, k)
		} else {
			claims[k] = v
		}
	}
	return sign(t, f.key, "ES256", map[string]any{"alg": "ES256", "kid": "k1", "typ": "at+jwt"}, claims)
}

// ── Scripts ─────────────────────────────────────────────────────────────────

func evalScript(t *testing.T, register func(*scriptling.Scriptling), script string) (string, error) {
	t.Helper()
	p := scriptling.New()
	stdlib.RegisterAll(p)
	register(p)
	result, err := p.Eval("import scriptling.oauth as oauth\n" + script)
	if err != nil {
		return "", err
	}
	return result.Inspect(), nil
}

func plain(p *scriptling.Scriptling) { Register(p) }

func (f *fakeIdP) newClientCall(extra string) string {
	return `oauth.Client("` + f.issuer + `", client_id="c_cli", verify=False` + extra + `)`
}

// ── Discovery and the client ────────────────────────────────────────────────

func TestClientRefusesPlainHTTPAndBadIssuers(t *testing.T) {
	for issuer, want := range map[string]string{
		"http://id.example.com/o/acme":      "must use https",
		"https://id.example.com/o/acme?x=1": "no query or fragment",
		"id.example.com":                    "not an absolute URL",
	} {
		if _, err := newClient(issuer, "c", "", pool.GetHTTPClient(), time.Second); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: want %q, got %v", issuer, want, err)
		}
	}
	for _, ok := range []string{"http://localhost:8080/o/acme", "http://127.0.0.1:9/o", "https://id.example.com/"} {
		if _, err := newClient(ok, "c", "", pool.GetHTTPClient(), time.Second); err != nil {
			t.Errorf("%s: %v", ok, err)
		}
	}
	if _, err := newClient("https://id.example.com", "", "", pool.GetHTTPClient(), time.Second); err == nil {
		t.Error("an empty client_id was accepted")
	}
}

func TestMetadataMustNameTheIssuer(t *testing.T) {
	f := newFakeIdP(t)
	c, _ := f.client(t, "")
	doc, err := c.Metadata(context.Background())
	if err != nil || doc["token_endpoint"] != f.issuer+"/oauth/token" {
		t.Fatalf("metadata: %v %v", doc, err)
	}

	other, err := newClient(f.srv.URL+"/o/acme/", "c", "", f.srv.Client(), time.Second) // trailing slash is the same issuer
	if err != nil {
		t.Fatal(err)
	}
	if _, err := other.Metadata(context.Background()); err != nil {
		t.Errorf("issuer with a trailing slash: %v", err)
	}

	wrong := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"issuer": "https://elsewhere.example.com"})
	}))
	defer wrong.Close()
	c2, _ := newClient(wrong.URL, "c", "", wrong.Client(), time.Second)
	if _, err := c2.Metadata(context.Background()); err == nil || !strings.Contains(err.Error(), "is for issuer") {
		t.Errorf("discovery for another issuer: %v", err)
	}
}

// ── Device sign-in ──────────────────────────────────────────────────────────

func TestDeviceFlowPollsAtTheIntervalUntilApproved(t *testing.T) {
	f := newFakeIdP(t)
	f.approveAfter = 1
	c, now := f.client(t, "")
	ctx := context.Background()

	flow, err := c.DeviceStart(ctx, "openid profile", []string{"https://api.example.com/"})
	if err != nil {
		t.Fatal(err)
	}
	if got := f.form(); got["scope"] != "openid profile" || got["resource"] != "https://api.example.com/" || got["client_id"] != "c_cli" {
		t.Errorf("device request form: %v", got)
	}

	// Too soon: no request to the IdP.
	if tok, err := flow.Poll(ctx); tok != nil || err != nil || f.tokenCalls.Load() != 0 {
		t.Fatalf("early poll: %v %v (calls %d)", tok, err, f.tokenCalls.Load())
	}
	*now = now.Add(time.Second)
	if tok, err := flow.Poll(ctx); tok != nil || err != nil {
		t.Fatalf("pending poll: %v %v", tok, err)
	}
	*now = now.Add(time.Second)
	tok, err := flow.Poll(ctx)
	if err != nil || tok == nil || tok.accessToken != "at-1" || tok.refreshToken != "rt-1" {
		t.Fatalf("approved poll: %+v %v", tok, err)
	}
	if f.form()["device_code"] != "dev-code-secret" || f.form()["grant_type"] != grantDeviceCode {
		t.Errorf("token request form: %v", f.form())
	}
	if _, err := flow.Poll(ctx); err != errDeviceDone {
		t.Errorf("poll after approval: %v", err)
	}
}

func TestDeviceFlowSlowDownDeniedAndExpired(t *testing.T) {
	f := newFakeIdP(t)
	f.slowDownOnce = true
	c, now := f.client(t, "")
	ctx := context.Background()
	flow, _ := c.DeviceStart(ctx, "", nil)

	*now = now.Add(time.Second)
	if tok, err := flow.Poll(ctx); tok != nil || err != nil || flow.interval != 6*time.Second {
		t.Fatalf("slow_down: %v %v interval %v", tok, err, flow.interval)
	}

	f.mu.Lock()
	f.deny = true
	f.mu.Unlock()
	*now = now.Add(6 * time.Second)
	if _, err := flow.Poll(ctx); err != errDeviceDenied {
		t.Errorf("denied: %v", err)
	}

	expiring, _ := c.DeviceStart(ctx, "", nil)
	*now = now.Add(11 * time.Minute)
	if _, err := expiring.Poll(ctx); err != errDeviceExpired {
		t.Errorf("expired: %v", err)
	}
}

func TestDeviceFlowInScript(t *testing.T) {
	f := newFakeIdP(t)
	f.approveAfter = 1
	out, err := evalScript(t, plain, `
idp = `+f.newClientCall("")+`
flow = idp.device_start(scope=["openid", "profile"], resource="https://api.example.com/")
info = [flow.user_code, flow.verification_uri_complete, flow.interval]
saved = flow.to_dict()
again = idp.device_resume(saved)
token = again.wait(timeout=10)
[info, token.access_token, token.headers()["Authorization"], token.refresh_token, token.is_expired()]`)
	if err != nil {
		t.Fatal(err)
	}
	want := `[['WDJB-MJHT', '` + f.issuer + `/device?user_code=WDJB-MJHT', 1], 'at-1', 'Bearer at-1', 'rt-1', False]`
	if out != want {
		t.Errorf("got  %s\nwant %s", out, want)
	}
}

// ── Client credentials, refresh, saving ─────────────────────────────────────

func TestClientCredentialsAndErrorsKeepSecretsOut(t *testing.T) {
	f := newFakeIdP(t)
	out, err := evalScript(t, plain, `
svc = `+f.newClientCall(`, client_secret="s3cret"`)+`
svc.client_credentials(scope="reports:read", resource=["https://a.example.com/", "https://b.example.com/"]).access_token`)
	if err != nil || out != `svc-token` {
		t.Fatalf("client credentials: %s %v", out, err)
	}
	if f.form()["resource"] != "https://a.example.com/ https://b.example.com/" || f.form()["client_secret"] != "s3cret" {
		t.Errorf("form: %v", f.form())
	}

	_, err = evalScript(t, plain, f.newClientCall(`, client_secret="wrong-secret-value"`)+`.client_credentials()`)
	if err == nil || !strings.Contains(err.Error(), "invalid_client: wrong secret") || strings.Contains(err.Error(), "wrong-secret-value") {
		t.Errorf("refused client credentials: %v", err)
	}
	if _, err := evalScript(t, plain, f.newClientCall("")+`.client_credentials()`); err == nil ||
		!strings.Contains(err.Error(), "needs the client's client_secret") {
		t.Errorf("without a secret: %v", err)
	}
}

func TestTokenRefreshesAndRoundTrips(t *testing.T) {
	f := newFakeIdP(t)
	c, now := f.client(t, "")
	ctx := context.Background()
	tok, err := newToken(c, map[string]any{"access_token": "at-1", "refresh_token": "rt-1", "expires_in": float64(600)})
	if err != nil {
		t.Fatal(err)
	}
	if h, _ := tok.Header(ctx); h != "Bearer at-1" || f.tokenCalls.Load() != 0 {
		t.Fatalf("fresh token: %q, %d calls", h, f.tokenCalls.Load())
	}
	*now = now.Add(590 * time.Second) // inside the 30s margin
	if h, err := tok.Header(ctx); h != "Bearer at-2" || err != nil {
		t.Fatalf("expiring token: %q %v", h, err)
	}
	if tok.refreshToken != "rt-1" {
		t.Error("a refresh answer without a refresh token dropped the old one")
	}

	d := tok.dict()
	again, err := newToken(c, d)
	if err != nil || again.accessToken != "at-2" || again.refreshToken != "rt-1" || !again.expiresAt.Equal(time.Unix(tok.expiresAt.Unix(), 0)) {
		t.Errorf("round trip: %+v %v", again, err)
	}
	if _, err := newToken(c, map[string]any{"token_type": "Bearer"}); err == nil {
		t.Error("a dict without access_token was accepted")
	}

	out, err := evalScript(t, plain, `
idp = `+f.newClientCall("")+`
tok = idp.token_from_dict({"access_token": "at-1", "refresh_token": "rt-1", "expires_at": 1})
tok.refresh()
[tok.access_token, tok.to_dict()["refresh_token"]]`)
	if err != nil || out != `['at-2', 'rt-1']` {
		t.Errorf("script refresh: %s %v", out, err)
	}
}

// ── Verification ────────────────────────────────────────────────────────────

func TestVerifyAccessTokens(t *testing.T) {
	f := newFakeIdP(t)
	c, _ := f.client(t, "")
	ctx := context.Background()
	aud := "https://api.example.com/"

	claims, err := c.Verify(ctx, f.accessToken(t, nil), aud, "at+jwt", time.Minute)
	if err != nil || claims["sub"] != "user-1" {
		t.Fatalf("good token: %v %v", claims, err)
	}
	if _, err := c.Verify(ctx, f.accessToken(t, map[string]any{"aud": aud}), aud, "", time.Minute); err != nil {
		t.Errorf("aud as a string: %v", err)
	}

	other := newECKey(t)
	cases := map[string]string{
		"another issuer": f.accessToken(t, map[string]any{"iss": "https://elsewhere.example.com"}),
		"another API":    f.accessToken(t, map[string]any{"aud": "https://other.example.com/"}),
		"expired":        f.accessToken(t, map[string]any{"exp": time.Now().Add(-2 * time.Minute).Unix()}),
		"no expiry":      f.accessToken(t, map[string]any{"exp": nil}),
		"not yet valid":  f.accessToken(t, map[string]any{"nbf": time.Now().Add(5 * time.Minute).Unix()}),
		"key bound":      f.accessToken(t, map[string]any{"cnf": map[string]any{"jkt": "x"}}),
		"wrong key": sign(t, other, "ES256", map[string]any{"alg": "ES256", "kid": "k1"},
			map[string]any{"iss": f.issuer, "aud": aud, "exp": time.Now().Add(time.Hour).Unix()}),
		"alg none": sign(t, f.key, "none", map[string]any{"alg": "none"},
			map[string]any{"iss": f.issuer, "aud": aud, "exp": time.Now().Add(time.Hour).Unix()}),
		"not a JWT": "abc.def",
	}
	for name, tok := range cases {
		_, err := c.Verify(ctx, tok, aud, "", time.Minute)
		if err == nil {
			t.Errorf("%s: accepted", name)
			continue
		}
		if strings.Contains(err.Error(), tok) {
			t.Errorf("%s: the error contains the token", name)
		}
	}
	if _, err := c.Verify(ctx, f.accessToken(t, nil), aud, "logout+jwt", time.Minute); err == nil {
		t.Error("a typ requirement was not checked")
	}
	if _, err := c.Verify(ctx, f.accessToken(t, nil), "", "", time.Minute); err == nil {
		t.Error("verify without an audience was allowed")
	}
}

// A token signed with a key the cached set lacks fetches the set again, at
// most once a minute: the IdP rotated its keys.
func TestVerifyRefetchesKeysForANewKey(t *testing.T) {
	f := newFakeIdP(t)
	c, now := f.client(t, "")
	ctx := context.Background()
	aud := "https://api.example.com/"
	if _, err := c.Verify(ctx, f.accessToken(t, nil), aud, "", time.Minute); err != nil {
		t.Fatal(err)
	}

	newKey := newECKey(t)
	f.mu.Lock()
	f.jwks = append(f.jwks, ecJWK(&newKey.PublicKey, "k2"))
	f.mu.Unlock()
	rotated := sign(t, newKey, "ES256", map[string]any{"alg": "ES256", "kid": "k2"},
		map[string]any{"iss": f.issuer, "aud": aud, "exp": time.Now().Add(time.Hour).Unix()})

	*now = now.Add(2 * time.Minute)
	if _, err := c.Verify(ctx, rotated, aud, "", time.Minute); err != nil {
		t.Fatalf("rotated key: %v", err)
	}
	fetches := f.jwksFetches.Load()
	unknown := sign(t, newECKey(t), "ES256", map[string]any{"alg": "ES256", "kid": "k9"},
		map[string]any{"iss": f.issuer, "aud": aud, "exp": time.Now().Add(time.Hour).Unix()})
	for range 3 {
		_, _ = c.Verify(ctx, unknown, aud, "", time.Minute)
	}
	if f.jwksFetches.Load() != fetches {
		t.Errorf("unknown keys fetched the set %d more times within a minute", f.jwksFetches.Load()-fetches)
	}
}

func TestVerifyRSAAndEd25519(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pub, edKey, _ := ed25519.GenerateKey(rand.Reader)
	keys := []publicKey{
		{kid: "r", key: &rsaKey.PublicKey},
		{kid: "e", alg: "EdDSA", key: pub},
	}
	claims := map[string]any{"iss": "x"}
	for _, tc := range []struct {
		alg, kid string
		key      crypto.Signer
	}{{"RS256", "r", rsaKey}, {"PS256", "r", rsaKey}, {"EdDSA", "e", edKey}} {
		parsed, err := parseJWT(sign(t, tc.key, tc.alg, map[string]any{"alg": tc.alg, "kid": tc.kid}, claims))
		if err != nil {
			t.Fatal(err)
		}
		if err := verifySignature(parsed, keys); err != nil {
			t.Errorf("%s: %v", tc.alg, err)
		}
		parsed.signature[0] ^= 1
		if verifySignature(parsed, keys) == nil {
			t.Errorf("%s: a tampered signature verified", tc.alg)
		}
	}
	// HS256 is never accepted, whatever the key set holds.
	parsed, _ := parseJWT(b64([]byte(`{"alg":"HS256"}`)) + "." + b64([]byte(`{}`)) + ".c2ln")
	if err := verifySignature(parsed, keys); err == nil || !strings.Contains(err.Error(), "not accepted") {
		t.Errorf("HS256: %v", err)
	}
}

func TestParseJWKSSkipsWhatItCannotUse(t *testing.T) {
	k := newECKey(t)
	set, _ := json.Marshal(map[string]any{"keys": []map[string]any{
		{"kty": "RSA", "use": "enc", "n": "AQAB", "e": "AQAB"},
		{"kty": "oct", "k": "c2VjcmV0"},
		{"kty": "EC", "crv": "P-256", "x": b64(make([]byte, 32)), "y": b64(make([]byte, 32))}, // not on the curve
		ecJWK(&k.PublicKey, "good"),
	}})
	keys, err := parseJWKS(set)
	if err != nil || len(keys) != 1 || keys[0].kid != "good" {
		t.Fatalf("keys: %+v %v", keys, err)
	}
	if _, err := parseJWKS([]byte(`{"keys":[{"kty":"oct","k":"x"}]}`)); err == nil {
		t.Error("a set without usable keys was accepted")
	}
}

func TestVerifyInScript(t *testing.T) {
	f := newFakeIdP(t)
	good := f.accessToken(t, nil)
	out, err := evalScript(t, plain, `
idp = `+f.newClientCall("")+`
claims = idp.verify("`+good+`", "https://api.example.com/", typ="at+jwt")
[claims["sub"], claims["scope"]]`)
	if err != nil || out != `['user-1', 'openid']` {
		t.Fatalf("verify: %s %v", out, err)
	}
	out, err = evalScript(t, plain, `
idp = `+f.newClientCall("")+`
tok = idp.token_from_dict({"access_token": "`+good+`"})
tok.claims()["aud"]`)
	if err != nil || !strings.Contains(out, "https://api.example.com/") {
		t.Errorf("claims: %s %v", out, err)
	}
}

// ── Policy, TLS and the jail ────────────────────────────────────────────────

func TestNetworkPolicyApplies(t *testing.T) {
	f := newFakeIdP(t)
	blocked := func(p *scriptling.Scriptling) { Register(p, &netsecurity.Config{AllowLoopback: true}) }
	if _, err := evalScript(t, blocked, f.newClientCall("")+`.metadata()`); err == nil || !strings.Contains(err.Error(), "IP literals") {
		t.Errorf("policy without IP literals: %v", err)
	}
	allowed := func(p *scriptling.Scriptling) {
		Register(p, &netsecurity.Config{AllowLoopback: true, AllowIPLiterals: true})
	}
	if out, err := evalScript(t, allowed, f.newClientCall("")+`.metadata()["issuer"]`); err != nil || !strings.Contains(out, f.issuer) {
		t.Errorf("policy allowing it: %s %v", out, err)
	}
}

func TestTLSIsVerifiedByDefault(t *testing.T) {
	f := newFakeIdP(t)
	_, err := evalScript(t, plain, `oauth.Client("`+f.issuer+`", client_id="c").metadata()`)
	if err == nil || !strings.Contains(err.Error(), "certificate") {
		t.Errorf("self-signed IdP without verify=False: %v", err)
	}
}

func TestCABundleIsConfined(t *testing.T) {
	f := newFakeIdP(t)
	allowed := t.TempDir()
	write := func(dir string) string {
		p := filepath.Join(dir, "ca.pem")
		_ = os.WriteFile(p, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: f.srv.Certificate().Raw}), 0o600)
		return p
	}
	inside, outside := write(allowed), write(t.TempDir())
	jailed := func(p *scriptling.Scriptling) {
		RegisterConfigured(p, fssecurity.Config{AllowedPaths: []string{allowed}}, nil)
	}
	call := func(path string) string {
		return `oauth.Client("` + f.issuer + `", client_id="c", verify="` + path + `").metadata()["issuer"]`
	}
	if out, err := evalScript(t, jailed, call(inside)); err != nil || !strings.Contains(out, f.issuer) {
		t.Errorf("CA bundle inside the allowed paths: %s %v", out, err)
	}
	if _, err := evalScript(t, jailed, call(outside)); err == nil || !strings.Contains(err.Error(), "outside allowed directories") {
		t.Errorf("CA bundle outside the allowed paths: %v", err)
	}
}

// Requests carrying codes, secrets or tokens never follow a redirect.
func TestTokenRequestsDoNotFollowRedirects(t *testing.T) {
	var stolen atomic.Int32
	thief := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { stolen.Add(1) }))
	defer thief.Close()
	var srvURL string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "openid-configuration") {
			writeJSON(w, 200, map[string]any{"issuer": srvURL, "token_endpoint": srvURL + "/token"})
			return
		}
		http.Redirect(w, r, thief.URL+"/token", http.StatusTemporaryRedirect) // 307 would resend the body
	}))
	defer srv.Close()
	srvURL = srv.URL
	c, _ := newClient(srv.URL, "c", "s3cret", pool.GetInsecureHTTPClient(), 5*time.Second)
	if _, err := c.ClientCredentials(context.Background(), "", nil); err == nil {
		t.Error("a redirected token request succeeded")
	}
	if stolen.Load() != 0 {
		t.Errorf("the redirect was followed %d times", stolen.Load())
	}
}

func TestClientIDByPositionOrName(t *testing.T) {
	f := newFakeIdP(t)
	for call, want := range map[string]string{
		`oauth.Client("` + f.issuer + `", "c_pos", verify=False).client_id`:          "c_pos",
		`oauth.Client("` + f.issuer + `", client_id="c_kw", verify=False).client_id`: "c_kw",
	} {
		if out, err := evalScript(t, plain, call); err != nil || out != want {
			t.Errorf("%s: %s %v", call, out, err)
		}
	}
	if _, err := evalScript(t, plain, `oauth.Client("`+f.issuer+`", "a", client_id="b")`); err == nil ||
		!strings.Contains(err.Error(), "both by position and by name") {
		t.Errorf("client_id twice: %v", err)
	}
}

// ── Fixes from review ───────────────────────────────────────────────────────

// An issuer whose canonical form ends in "/" (as some IdPs' does) is matched
// exactly: iss must be the discovery document's issuer, slash and all.
func TestVerifyUsesTheCanonicalIssuer(t *testing.T) {
	f := newFakeIdP(t)
	f.issuerSlash = true
	c, _ := f.client(t, "")
	ctx := context.Background()
	aud := "https://api.example.com/"
	if _, err := c.Verify(ctx, f.accessToken(t, map[string]any{"iss": f.issuer + "/"}), aud, "", time.Minute); err != nil {
		t.Errorf("iss with the canonical trailing slash: %v", err)
	}
	if _, err := c.Verify(ctx, f.accessToken(t, nil), aud, "", time.Minute); err == nil {
		t.Error("iss without the canonical trailing slash was accepted")
	}
}

// Concurrent headers() on an expiring token refresh it once: a second use of
// a one-time refresh token would make the IdP revoke the grant.
func TestConcurrentHeadersRefreshOnce(t *testing.T) {
	f := newFakeIdP(t)
	f.refreshDelay = 100 * time.Millisecond
	c, now := f.client(t, "")
	tok, _ := newToken(c, map[string]any{"access_token": "at-1", "refresh_token": "rt-1", "expires_in": float64(600)})
	*now = now.Add(590 * time.Second)

	var wg sync.WaitGroup
	headers := make([]string, 8)
	for i := range headers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			headers[i], _ = tok.Header(context.Background())
		}()
	}
	wg.Wait()
	if n := f.refreshCalls.Load(); n != 1 {
		t.Errorf("%d refreshes, want 1", n)
	}
	for _, h := range headers {
		if h != "Bearer at-2" {
			t.Errorf("header %q, want the refreshed token", h)
		}
	}
}

// Reading a flow does not wait while a poll is on the network.
func TestPollDoesNotHoldTheFlowAcrossTheNetwork(t *testing.T) {
	f := newFakeIdP(t)
	f.approveAfter = 99
	c, now := f.client(t, "")
	ctx := context.Background()
	flow, _ := c.DeviceStart(ctx, "", nil)
	gate := make(chan struct{})
	release := sync.OnceFunc(func() { close(gate) })
	defer release() // a failing check must not leave the fake IdP waiting
	f.mu.Lock()
	f.deviceGate = gate
	f.mu.Unlock()
	*now = now.Add(time.Second)

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = flow.Poll(ctx)
	}()
	for f.tokenCalls.Load() == 0 {
		time.Sleep(5 * time.Millisecond)
	}
	read := make(chan struct{})
	go func() {
		_ = flow.dict()
		if tok, err := flow.Poll(ctx); tok != nil || err != nil {
			t.Errorf("a poll while one is in flight: %v %v", tok, err)
		}
		close(read)
	}()
	select {
	case <-read:
	case <-time.After(2 * time.Second):
		t.Fatal("reading the flow waited for the poll on the network")
	}
	release()
	<-done
	if n := f.tokenCalls.Load(); n != 1 {
		t.Errorf("%d polls reached the IdP, want 1", n)
	}
}

// Any refusal other than pending or slow_down ends the sign-in (RFC 8628 §3.5).
func TestOtherDeviceErrorsEndTheFlow(t *testing.T) {
	f := newFakeIdP(t)
	f.deviceError = "invalid_grant"
	c, now := f.client(t, "")
	ctx := context.Background()
	flow, _ := c.DeviceStart(ctx, "", nil)
	*now = now.Add(time.Second)
	if _, err := flow.Poll(ctx); err == nil || !strings.Contains(err.Error(), "invalid_grant") {
		t.Fatalf("invalid_grant: %v", err)
	}
	*now = now.Add(10 * time.Second)
	if _, err := flow.Poll(ctx); err != errDeviceDone {
		t.Errorf("poll after a refusal: %v", err)
	}
}

func TestResumedFlowAndSlowDownAreBounded(t *testing.T) {
	f := newFakeIdP(t)
	c, now := f.client(t, "")
	flow, err := c.resumeDeviceFlow(map[string]any{"device_code": "d", "user_code": "u", "verification_uri": "https://x",
		"expires_at": float64(now.Add(1000 * time.Hour).Unix())})
	if err != nil {
		t.Fatal(err)
	}
	if flow.expiresAt.After(now.Add(maxDeviceFlowLifetime)) {
		t.Errorf("a resumed flow lives until %v", flow.expiresAt)
	}

	f.slowDownOnce = true
	ctx := context.Background()
	started, _ := c.DeviceStart(ctx, "", nil)
	started.interval = (maxDeviceFlowIntervalS - 1) * time.Second
	*now = now.Add(time.Second)
	f.mu.Lock()
	f.slowDownOnce = true
	f.mu.Unlock()
	_, _ = started.Poll(ctx)
	if started.interval != maxDeviceFlowIntervalS*time.Second {
		t.Errorf("slow_down took the interval to %v", started.interval)
	}
}

func TestCriticalHeadersAreRefused(t *testing.T) {
	f := newFakeIdP(t)
	c, _ := f.client(t, "")
	tok := sign(t, f.key, "ES256", map[string]any{"alg": "ES256", "kid": "k1", "crit": []string{"exp"}, "exp": 1},
		map[string]any{"iss": f.issuer, "aud": "https://api.example.com/", "exp": time.Now().Add(time.Hour).Unix()})
	if _, err := c.Verify(context.Background(), tok, "https://api.example.com/", "", time.Minute); err == nil ||
		!strings.Contains(err.Error(), "critical") {
		t.Errorf("crit header: %v", err)
	}
}

// Discovery and key-set fetches follow redirects only to acceptable endpoints.
func TestKeySetRedirectToPlainHTTPIsRefused(t *testing.T) {
	plainKeys := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"keys": []any{}})
	}))
	defer plainKeys.Close()
	var srvURL string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "openid-configuration") {
			writeJSON(w, 200, map[string]any{"issuer": srvURL, "jwks_uri": srvURL + "/jwks"})
			return
		}
		// A plain-http address that is not loopback by name: refused.
		http.Redirect(w, r, strings.Replace(plainKeys.URL, "127.0.0.1", "keys.example.com", 1)+"/jwks", http.StatusFound)
	}))
	defer srv.Close()
	srvURL = srv.URL
	c, _ := newClient(srv.URL, "c", "", pool.GetInsecureHTTPClient(), 5*time.Second)
	if _, err := c.Keys(context.Background(), false); err == nil || !strings.Contains(err.Error(), "https") {
		t.Errorf("redirect to plain http: %v", err)
	}
}

func TestArgumentTypesAndNone(t *testing.T) {
	f := newFakeIdP(t)
	for call, want := range map[string]string{
		`oauth.Client("` + f.issuer + `", client_id=1)`:                                        "client_id: must be a string",
		`oauth.Client("` + f.issuer + `", client_id="c", timeout="x")`:                         "timeout: must be a number",
		f.newClientCall("") + `.verify("a.b.c", "https://x/", leeway="x")`:                     "leeway: must be a number",
		f.newClientCall("") + `.token_from_dict({"access_token": "a"}).is_expired(margin="x")`: "margin: must be a number",
	} {
		if _, err := evalScript(t, plain, call); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: want %q, got %v", call, want, err)
		}
	}
	if out, err := evalScript(t, plain, `oauth.Client("`+f.issuer+`", "c", client_secret=None, verify=False).client_id`); err != nil || out != "c" {
		t.Errorf("client_secret=None: %s %v", out, err)
	}
}

func TestTypeErrors(t *testing.T) {
	f := newFakeIdP(t)
	for call, want := range map[string]string{
		`oauth.Client("` + f.issuer + `", client_id="c", verify=1)`:  "BOOLEAN or STRING",
		f.newClientCall("") + `.device_start(scope=1)`:               "STRING or LIST",
		f.newClientCall("") + `.token_from_dict("x")`:                "DICT",
		f.newClientCall("") + `.device_resume({"user_code": "x"})`:   "not a device sign-in dict",
		`oauth.Client("` + f.issuer + `", client_id="c", timeout=0)`: "timeout must be positive",
	} {
		if _, err := evalScript(t, plain, call); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: want %q, got %v", call, want, err)
		}
	}
}
