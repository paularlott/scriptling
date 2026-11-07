package server

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	mcplib "github.com/paularlott/mcp"
)

// mustResourceServer builds the auth layer for a hand-assembled test Server.
func mustResourceServer(t *testing.T, c ServerConfig) *mcplib.ResourceServer {
	t.Helper()
	rs, err := buildResourceServer(c)
	if err != nil {
		t.Fatalf("buildResourceServer: %v", err)
	}
	return rs
}

// testIdP is a minimal OpenID provider: discovery document and JWKS for one
// RSA key, plus a minting helper.
type testIdP struct {
	*httptest.Server
	key *rsa.PrivateKey
}

func newTestIdP(t *testing.T) *testIdP {
	t.Helper()
	return newTestIdPServer(t, false)
}

// newTestIdPServer is newTestIdP, served over https with httptest's
// self-signed certificate when useTLS is set.
func newTestIdPServer(t *testing.T, useTLS bool) *testIdP {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	idp := &testIdP{key: key}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"issuer": idp.URL, "jwks_uri": idp.URL + "/jwks"})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		b64 := base64.RawURLEncoding.EncodeToString
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kty": "RSA", "kid": "k1", "use": "sig", "alg": "RS256",
			"n": b64(key.N.Bytes()), "e": b64(big.NewInt(int64(key.E)).Bytes()),
		}}})
	})
	if useTLS {
		idp.Server = httptest.NewTLSServer(mux)
	} else {
		idp.Server = httptest.NewServer(mux)
	}
	t.Cleanup(idp.Close)
	return idp
}

func (idp *testIdP) mint(t *testing.T, claims map[string]any) string {
	t.Helper()
	enc := func(v any) string {
		b, _ := json.Marshal(v)
		return base64.RawURLEncoding.EncodeToString(b)
	}
	input := enc(map[string]string{"alg": "RS256", "kid": "k1", "typ": "at+jwt"}) + "." + enc(claims)
	sum := sha256.Sum256([]byte(input))
	sig, err := rsa.SignPKCS1v15(rand.Reader, idp.key, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatal(err)
	}
	return input + "." + base64.RawURLEncoding.EncodeToString(sig)
}

const testResource = "https://mcp.example.com/mcp"

// authTestServer serves a whoami tool (reporting request.auth) and a tool
// that fails, behind the given auth configuration.
func authTestServer(t *testing.T, c ServerConfig) *httptest.Server {
	t.Helper()
	toolsDir := t.TempDir()
	writeFile(t, toolsDir+"/whoami.toml", []byte("description = \"Who am I\"\n"))
	writeFile(t, toolsDir+"/whoami.py", []byte(`import scriptling.mcp.tool as tool
a = tool.get_request().auth
tool.return_string(a["subject"] + "|" + ",".join(a["scopes"]))
`))
	writeFile(t, toolsDir+"/boom.toml", []byte("description = \"Fails\"\n"))
	writeFile(t, toolsDir+"/boom.py", []byte("import scriptling.mcp.tool as tool\ntool.return_error('the widget is broken')\n"))

	c.ScriptFile = writeSetup(t, `
import scriptling.runtime as runtime
runtime.start_server(wait=False)
while runtime.server_running():
    yield_now()
`)
	c.MCPToolsDir = toolsDir
	s, err := NewServer(c)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	t.Cleanup(func() { signalShutdown(t, s) })
	ts := httptest.NewServer(s.buildMux())
	t.Cleanup(ts.Close)
	return ts
}

func postMCP(t *testing.T, ts *httptest.Server, body, token string) (*http.Response, map[string]any) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var parsed map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&parsed)
	return resp, parsed
}

func callTool(t *testing.T, ts *httptest.Server, name, token string) (*http.Response, map[string]any) {
	return postMCP(t, ts, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"`+name+`","arguments":{}}}`, token)
}

func firstText(t *testing.T, body map[string]any) (string, bool) {
	t.Helper()
	result, ok := body["result"].(map[string]any)
	if !ok {
		t.Fatalf("no result: %#v", body)
	}
	content, _ := result["content"].([]any)
	if len(content) == 0 {
		t.Fatalf("no content: %#v", body)
	}
	isErr, _ := result["isError"].(bool)
	return content[0].(map[string]any)["text"].(string), isErr
}

func TestStaticBearerToken401Challenge(t *testing.T) {
	ts := authTestServer(t, ServerConfig{BearerToken: "seekrit"})

	resp, _ := callTool(t, ts, "whoami", "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no token: %d, want 401", resp.StatusCode)
	}
	if got := resp.Header.Get("WWW-Authenticate"); got != `Bearer realm="scriptling"` {
		t.Errorf("no token challenge = %q", got)
	}

	resp, _ = callTool(t, ts, "whoami", "wrong")
	if resp.StatusCode != http.StatusUnauthorized || !strings.Contains(resp.Header.Get("WWW-Authenticate"), `error="invalid_token"`) {
		t.Fatalf("wrong token: %d %q, want 401 invalid_token", resp.StatusCode, resp.Header.Get("WWW-Authenticate"))
	}

	// No identity provider: nothing to advertise.
	if r, err := http.Get(ts.URL + "/.well-known/oauth-protected-resource"); err != nil || r.StatusCode != http.StatusUnauthorized {
		t.Fatalf("metadata without an issuer should not be served unauthenticated: %v %v", r, err)
	}

	resp, body := callTool(t, ts, "whoami", "seekrit")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("good token: %d", resp.StatusCode)
	}
	if text, _ := firstText(t, body); text != "bearer|" {
		t.Fatalf("request.auth = %q, want bearer|", text)
	}
}

// A tool's own failure is a tool execution error (isError result), not a
// JSON-RPC error.
func TestToolFailureIsErrorResult(t *testing.T) {
	ts := authTestServer(t, ServerConfig{BearerToken: "seekrit"})
	resp, body := callTool(t, ts, "boom", "seekrit")
	if resp.StatusCode != http.StatusOK || body["error"] != nil {
		t.Fatalf("boom: %d %#v, want a result", resp.StatusCode, body)
	}
	text, isErr := firstText(t, body)
	if !isErr || !strings.Contains(text, "the widget is broken") {
		t.Fatalf("boom = %q isError=%v", text, isErr)
	}
}

func TestOIDCResourceServer(t *testing.T) {
	idp := newTestIdP(t)
	ts := authTestServer(t, ServerConfig{
		BearerToken:        "seekrit",
		OIDCIssuer:         idp.URL,
		OIDCAudience:       testResource + "/",
		OIDCRequiredScopes: []string{"mcp:use"},
		OIDCRequireATJWT:   true,
	})
	exp := time.Now().Add(time.Hour).Unix()

	t.Run("challenge names the metadata", func(t *testing.T) {
		resp, _ := callTool(t, ts, "whoami", "")
		want := `Bearer resource_metadata="https://mcp.example.com/.well-known/oauth-protected-resource/mcp", scope="mcp:use"`
		if resp.StatusCode != http.StatusUnauthorized || resp.Header.Get("WWW-Authenticate") != want {
			t.Fatalf("%d %q, want 401 %q", resp.StatusCode, resp.Header.Get("WWW-Authenticate"), want)
		}
	})

	t.Run("metadata is public at both well-known paths", func(t *testing.T) {
		for _, path := range []string{"/.well-known/oauth-protected-resource/mcp", "/.well-known/oauth-protected-resource"} {
			resp, err := http.Get(ts.URL + path)
			if err != nil {
				t.Fatal(err)
			}
			b, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			var doc map[string]any
			_ = json.Unmarshal(b, &doc)
			as, _ := doc["authorization_servers"].([]any)
			if resp.StatusCode != http.StatusOK || doc["resource"] != testResource || len(as) != 1 || as[0] != idp.URL {
				t.Fatalf("%s: %d %s", path, resp.StatusCode, b)
			}
		}
	})

	t.Run("valid JWT", func(t *testing.T) {
		tok := idp.mint(t, map[string]any{"iss": idp.URL, "sub": "alice", "aud": testResource, "exp": exp, "scope": "mcp:use other"})
		resp, body := callTool(t, ts, "whoami", tok)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%d %#v", resp.StatusCode, body)
		}
		if text, _ := firstText(t, body); text != "alice|mcp:use,other" {
			t.Fatalf("request.auth = %q", text)
		}
	})

	t.Run("static token still accepted", func(t *testing.T) {
		resp, body := callTool(t, ts, "whoami", "seekrit")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%d", resp.StatusCode)
		}
		if text, _ := firstText(t, body); text != "bearer|mcp:use" {
			t.Fatalf("request.auth = %q", text)
		}
	})

	t.Run("wrong audience", func(t *testing.T) {
		tok := idp.mint(t, map[string]any{"iss": idp.URL, "sub": "alice", "aud": "https://other.example/mcp", "exp": exp, "scope": "mcp:use"})
		if resp, _ := callTool(t, ts, "whoami", tok); resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("%d, want 401", resp.StatusCode)
		}
	})

	t.Run("no audience", func(t *testing.T) {
		tok := idp.mint(t, map[string]any{"iss": idp.URL, "sub": "alice", "exp": exp, "scope": "mcp:use"})
		if resp, _ := callTool(t, ts, "whoami", tok); resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("%d, want 401", resp.StatusCode)
		}
	})

	t.Run("missing scope", func(t *testing.T) {
		tok := idp.mint(t, map[string]any{"iss": idp.URL, "sub": "alice", "aud": testResource, "exp": exp, "scope": "other"})
		resp, _ := callTool(t, ts, "whoami", tok)
		if resp.StatusCode != http.StatusForbidden || !strings.Contains(resp.Header.Get("WWW-Authenticate"), `error="insufficient_scope"`) {
			t.Fatalf("%d %q, want 403 insufficient_scope", resp.StatusCode, resp.Header.Get("WWW-Authenticate"))
		}
	})

	t.Run("expired", func(t *testing.T) {
		tok := idp.mint(t, map[string]any{"iss": idp.URL, "sub": "alice", "aud": testResource, "exp": time.Now().Add(-time.Hour).Unix(), "scope": "mcp:use"})
		if resp, _ := callTool(t, ts, "whoami", tok); resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("%d, want 401", resp.StatusCode)
		}
	})

	t.Run("non-MCP routes need a token too", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/health")
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("/health without token: %d", resp.StatusCode)
		}
	})
}

func TestResourceServerConfigErrors(t *testing.T) {
	for name, c := range map[string]ServerConfig{
		"issuer without audience": {OIDCIssuer: "https://idp.example"},
		"audience without issuer": {OIDCAudience: testResource},
		"relative audience":       {OIDCIssuer: "https://idp.example", OIDCAudience: "mcp"},
		"CA file without issuer":  {OIDCCAFile: "ca.pem"},
		"insecure without issuer": {OIDCTLSInsecure: true},
	} {
		if _, err := buildResourceServer(c); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	if rs, err := buildResourceServer(ServerConfig{}); rs != nil || err != nil {
		t.Errorf("no auth configured: %v %v", rs, err)
	}
}

// TestOIDCTLSOptions checks --oidc-ca-file and --oidc-tls-insecure reach the
// discovery and JWKS fetches: against an https identity provider with a
// self-signed certificate, a token validates only when its certificate is
// trusted through the CA file or verification is skipped.
func TestOIDCTLSOptions(t *testing.T) {
	idp := newTestIdPServer(t, true)
	caFile := filepath.Join(t.TempDir(), "ca.pem")
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: idp.Certificate().Raw})
	if err := os.WriteFile(caFile, certPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	tok := idp.mint(t, map[string]any{"iss": idp.URL, "sub": "alice", "aud": testResource, "exp": time.Now().Add(time.Hour).Unix()})
	validate := func(c ServerConfig) error {
		t.Helper()
		c.OIDCIssuer = idp.URL
		c.OIDCAudience = testResource
		rs := mustResourceServer(t, c)
		info, err := rs.ValidateToken(httptest.NewRequest(http.MethodPost, "/mcp", nil), tok)
		if err == nil && (info == nil || info.Subject != "alice") {
			t.Fatalf("token info = %#v", info)
		}
		return err
	}

	t.Run("untrusted certificate refused", func(t *testing.T) {
		if err := validate(ServerConfig{}); err == nil {
			t.Fatal("validated against an untrusted certificate")
		}
	})
	t.Run("CA file", func(t *testing.T) {
		if err := validate(ServerConfig{OIDCCAFile: caFile}); err != nil {
			t.Fatalf("with --oidc-ca-file: %v", err)
		}
	})
	t.Run("insecure", func(t *testing.T) {
		if err := validate(ServerConfig{OIDCTLSInsecure: true}); err != nil {
			t.Fatalf("with --oidc-tls-insecure: %v", err)
		}
	})
	t.Run("CA file and insecure", func(t *testing.T) {
		if err := validate(ServerConfig{OIDCCAFile: caFile, OIDCTLSInsecure: true}); err != nil {
			t.Fatalf("with both: %v", err)
		}
	})

	empty := filepath.Join(t.TempDir(), "empty.pem")
	if err := os.WriteFile(empty, []byte("not a certificate\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, path := range map[string]string{
		"unreadable CA file": filepath.Join(t.TempDir(), "missing.pem"),
		"empty CA file":      empty,
	} {
		_, err := buildResourceServer(ServerConfig{OIDCIssuer: idp.URL, OIDCAudience: testResource, OIDCCAFile: path})
		if err == nil || !strings.Contains(err.Error(), "--oidc-ca-file") {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}
