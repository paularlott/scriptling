package onepassword

import (
	"context"
	"strings"
	"testing"

	"github.com/paularlott/scriptling/extlibs/secretprovider"
)

func TestOnePasswordConnectConfigUnchanged(t *testing.T) {
	if err := Register(); err != nil {
		t.Fatal(err)
	}
	p, err := secretprovider.NewProvider(secretprovider.Config{Provider: "onepassword", Address: "http://connect:8080", Token: "connect-token"})
	if err != nil {
		t.Fatalf("Connect config: %v", err)
	}
	if _, ok := p.(*onePasswordProvider); !ok {
		t.Fatalf("Connect config built %T, want the Connect provider", p)
	}
}

// TestRegisterIsIdempotent: calling Register again (an embedder and a
// library both enabling the provider) neither panics nor errors.
func TestRegisterIsIdempotent(t *testing.T) {
	for i := 0; i < 3; i++ {
		if err := Register(); err != nil {
			t.Fatalf("Register() call %d: %v", i+1, err)
		}
	}
	found := false
	for _, name := range secretprovider.RegisteredTypes() {
		if name == TypeName {
			found = true
		}
	}
	if !found {
		t.Fatalf("RegisteredTypes() = %v, want it to include %q", secretprovider.RegisteredTypes(), TypeName)
	}
	// A different factory under the same type name is rejected, not swapped in.
	err := secretprovider.RegisterFactory(TypeName, func(secretprovider.Config) (secretprovider.Provider, error) { return nil, nil })
	if err == nil || !strings.Contains(err.Error(), "already registered") {
		t.Fatalf("RegisterFactory(%q) twice: err = %v, want already registered", TypeName, err)
	}
}

// TestRegisterConfigsResolvesThroughConnect is the end-to-end happy path: a
// config block naming the onepassword type, after Register, resolves a
// secret from a Connect server through the alias registry.
func TestRegisterConfigsResolvesThroughConnect(t *testing.T) {
	_, srv := newFakeConnect(t)
	if err := Register(); err != nil {
		t.Fatal(err)
	}
	registry := secretprovider.NewRegistry()
	err := secretprovider.RegisterConfigs(registry, []secretprovider.Config{
		{Provider: "onepassword", Alias: "op", Address: srv.URL, Token: "connect-token"},
	})
	if err != nil {
		t.Fatalf("RegisterConfigs: %v", err)
	}
	got, err := registry.Resolve(context.Background(), "op", "Engineering/Database", "username")
	if err != nil || got != "admin" {
		t.Fatalf("Resolve = %q, %v; want admin", got, err)
	}
}

func TestNewRejectsBadConfig(t *testing.T) {
	for _, tt := range []struct {
		name string
		cfg  secretprovider.Config
		want string
	}{
		{"no address", secretprovider.Config{Provider: "onepassword", Token: "t"}, "requires address"},
		{"no token", secretprovider.Config{Provider: "onepassword", Address: "http://connect:8080"}, "requires token"},
		{"bad scheme", secretprovider.Config{Provider: "onepassword", Address: "ftp://connect", Token: "t"}, "http or https"},
	} {
		if _, err := New(tt.cfg); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s: err = %v, want %q", tt.name, err, tt.want)
		}
	}
}
