package vault

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/paularlott/scriptling/extlibs/secretprovider"
)

// newFakeVault serves one KV v2 secret at secret/data/app (and its metadata
// listing), plus AppRole login, checking the token and namespace headers.
func newFakeVault(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/auth/approle/login" {
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["role_id"] != "role" || body["secret_id"] != "shh" {
				http.Error(w, "bad credentials", http.StatusForbidden)
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"auth": map[string]any{"client_token": "vault-token"}})
			return
		}
		if r.Header.Get("X-Vault-Token") != "vault-token" {
			http.Error(w, "permission denied", http.StatusForbidden)
			return
		}
		if r.Header.Get("X-Vault-Namespace") != "team" {
			http.Error(w, "wrong namespace", http.StatusBadRequest)
			return
		}
		switch {
		case r.URL.Path == "/v1/secret/data/app":
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"data": map[string]any{"value": "v", "password": "hunter2", "port": 5432},
			}})
		case r.URL.Path == "/v1/secret/metadata/" && r.URL.Query().Get("list") == "true":
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"keys": []string{"app", "db/"}}})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestVaultResolveAndList(t *testing.T) {
	srv := newFakeVault(t)
	if err := Register(); err != nil {
		t.Fatal(err)
	}
	p, err := secretprovider.NewProvider(secretprovider.Config{Provider: "vault", Address: srv.URL + "/", Token: "vault-token", Namespace: "team"})
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	if p.ID() != "vault" {
		t.Errorf("ID() = %q, want vault", p.ID())
	}
	ctx := context.Background()

	for _, tt := range []struct{ field, want string }{{"", "v"}, {"password", "hunter2"}} {
		got, err := p.Resolve(ctx, "/secret/data/app", tt.field)
		if err != nil || got != tt.want {
			t.Errorf("Resolve(field %q) = %q, %v; want %q", tt.field, got, err, tt.want)
		}
	}
	for _, tt := range []struct{ path, field, want string }{
		{"secret/data/app", "nope", `field "nope" not found`},
		{"secret/data/app", "port", "is not a string"},
		{"secret/data/missing", "", "404"},
	} {
		if _, err := p.Resolve(ctx, tt.path, tt.field); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("Resolve(%q, %q) err = %v, want %q", tt.path, tt.field, err, tt.want)
		}
	}

	keys, err := p.List(ctx, "secret/data/app")
	sort.Strings(keys)
	if err != nil || strings.Join(keys, ",") != "password,port,value" {
		t.Errorf("List(leaf) = %v, %v", keys, err)
	}
	keys, err = p.List(ctx, "secret/data/")
	if err != nil || strings.Join(keys, ",") != "app,db/" {
		t.Errorf("List(dir) = %v, %v", keys, err)
	}
	keys, err = p.List(ctx, "secret/data/nothing/")
	if err != nil || keys != nil {
		t.Errorf("List(missing) = %v, %v; want nil, nil", keys, err)
	}
}

func TestVaultAppRoleLogin(t *testing.T) {
	srv := newFakeVault(t)
	p, err := New(secretprovider.Config{Provider: "vault", Address: srv.URL, AppRoleID: "role", AppRoleSecret: "shh", Namespace: "team", KVVersion: 2})
	if err != nil {
		t.Fatalf("New with AppRole: %v", err)
	}
	if got, err := p.Resolve(context.Background(), "secret/data/app", "password"); err != nil || got != "hunter2" {
		t.Fatalf("Resolve after AppRole login = %q, %v", got, err)
	}

	if _, err := New(secretprovider.Config{Provider: "vault", Address: srv.URL, AppRoleID: "role", AppRoleSecret: "wrong"}); err == nil || !strings.Contains(err.Error(), "approle login returned") {
		t.Fatalf("bad AppRole credentials: err = %v", err)
	}
}

func TestVaultNewRejectsBadConfig(t *testing.T) {
	if _, err := New(secretprovider.Config{Provider: "vault", Token: "t"}); err == nil || !strings.Contains(err.Error(), "requires address") {
		t.Errorf("no address: err = %v", err)
	}
	if _, err := New(secretprovider.Config{Provider: "vault", Address: "http://vault:8200"}); err == nil || !strings.Contains(err.Error(), "requires token or app_role") {
		t.Errorf("no credentials: err = %v", err)
	}
}

// TestVaultRegisterIsIdempotent: Register twice neither panics nor errors,
// and a different factory cannot take the vault type name over.
func TestVaultRegisterIsIdempotent(t *testing.T) {
	if err := Register(); err != nil {
		t.Fatal(err)
	}
	if err := Register(); err != nil {
		t.Fatalf("second Register(): %v", err)
	}
	err := secretprovider.RegisterFactory(TypeName, func(secretprovider.Config) (secretprovider.Provider, error) { return nil, nil })
	if err == nil || !strings.Contains(err.Error(), "already registered") {
		t.Fatalf("RegisterFactory(vault) after Register: err = %v, want already registered", err)
	}
}

// TestVaultRegisterConfigs: a config file's vault block resolves through the
// alias registry once the type is registered.
func TestVaultRegisterConfigs(t *testing.T) {
	srv := newFakeVault(t)
	if err := Register(); err != nil {
		t.Fatal(err)
	}
	registry := secretprovider.NewRegistry()
	if err := secretprovider.RegisterConfigs(registry, []secretprovider.Config{
		{Provider: "vault", Alias: "prod_vault", Address: srv.URL, Token: "vault-token", Namespace: "team", CacheTTL: "1m"},
	}); err != nil {
		t.Fatalf("RegisterConfigs: %v", err)
	}
	if got, err := registry.Resolve(context.Background(), "prod_vault", "secret/data/app", "password"); err != nil || got != "hunter2" {
		t.Fatalf("Resolve = %q, %v", got, err)
	}
}
