package secretconfig

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "secrets.toml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestLoadRegistryFileSupportsCLIProviderTypes: the CLI accepts the vault and
// onepassword (Connect) provider types without any extra registration.
func TestLoadRegistryFileSupportsCLIProviderTypes(t *testing.T) {
	vaultSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Vault-Token") != "hvs.test" || r.URL.Path != "/v1/secret/data/app" {
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"data": map[string]any{"password": "hunter2"}}})
	}))
	t.Cleanup(vaultSrv.Close)

	path := writeConfig(t, `
[[secrets.provider]]
provider = "vault"
alias = "prod_vault"
address = "`+vaultSrv.URL+`"
token = "hvs.test"

[[secrets.provider]]
provider = "onepassword"
alias = "op"
address = "http://op-connect:8080"
token = "example-token"
`)
	registry, err := LoadRegistryFile(path)
	if err != nil {
		t.Fatalf("LoadRegistryFile: %v", err)
	}
	got, err := registry.Resolve(context.Background(), "prod_vault", "secret/data/app", "password")
	if err != nil || got != "hunter2" {
		t.Fatalf("Resolve = %q, %v", got, err)
	}
	if len(registry.Aliases()) != 2 {
		t.Fatalf("aliases = %v, want prod_vault and op", registry.Aliases())
	}
}

func TestLoadRegistryFileUnknownProviderType(t *testing.T) {
	path := writeConfig(t, `
[[secrets.provider]]
provider = "nosuch"
alias = "x"
`)
	_, err := LoadRegistryFile(path)
	if err == nil || !strings.Contains(err.Error(), `"nosuch"`) || !strings.Contains(err.Error(), "not compiled in or not registered") {
		t.Fatalf("err = %v, want unregistered provider type error naming nosuch", err)
	}
}

// TestLoadRegistryFileServiceAccountNeedsExtension: this package does not
// link scriptling-onepassword, so a service account block fails clearly.
func TestLoadRegistryFileServiceAccountNeedsExtension(t *testing.T) {
	path := writeConfig(t, `
[[secrets.provider]]
provider = "onepassword"
alias = "op"
service_account_token = "ops_example"
`)
	_, err := LoadRegistryFile(path)
	if err == nil || !strings.Contains(err.Error(), "not supported by this build") {
		t.Fatalf("err = %v, want not supported by this build", err)
	}
}

func TestRegisterProviderTypesTwice(t *testing.T) {
	if err := RegisterProviderTypes(); err != nil {
		t.Fatal(err)
	}
	if err := RegisterProviderTypes(); err != nil {
		t.Fatalf("second call: %v", err)
	}
}
