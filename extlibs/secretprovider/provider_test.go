package secretprovider

import (
	"context"
	"strings"
	"testing"
	"time"
)

type testProvider struct {
	id    string
	value string
	calls int
}

func (p *testProvider) ID() string { return p.id }

func (p *testProvider) Resolve(_ context.Context, path, field string) (string, error) {
	p.calls++
	return p.value + ":" + path + ":" + field, nil
}

func (p *testProvider) List(_ context.Context, path string) ([]string, error) {
	return []string{"key1", "key2", path}, nil
}

func TestRegistryResolveCachesByAlias(t *testing.T) {
	registry := NewRegistry()
	provider := &testProvider{id: "vault", value: "secret"}
	if err := registry.Register(provider, "prod_vault", time.Minute); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	first, err := registry.Resolve(context.Background(), "prod_vault", "secret/data/app", "password")
	if err != nil {
		t.Fatalf("first Resolve() error = %v", err)
	}
	second, err := registry.Resolve(context.Background(), "prod_vault", "secret/data/app", "password")
	if err != nil {
		t.Fatalf("second Resolve() error = %v", err)
	}

	if first != second {
		t.Fatalf("cached values differ: %q vs %q", first, second)
	}
	if provider.calls != 1 {
		t.Fatalf("provider calls = %d, want 1", provider.calls)
	}
}

func TestRegistryResolveCacheExpires(t *testing.T) {
	registry := NewRegistry()
	provider := &testProvider{id: "vault", value: "secret"}
	if err := registry.Register(provider, "prod_vault", 20*time.Millisecond); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	if _, err := registry.Resolve(context.Background(), "prod_vault", "secret/data/app", "password"); err != nil {
		t.Fatalf("first Resolve() error = %v", err)
	}

	time.Sleep(30 * time.Millisecond)

	if _, err := registry.Resolve(context.Background(), "prod_vault", "secret/data/app", "password"); err != nil {
		t.Fatalf("second Resolve() error = %v", err)
	}

	if provider.calls != 2 {
		t.Fatalf("provider calls = %d, want 2", provider.calls)
	}
}

func TestRegistryRejectsDuplicateAlias(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register(&testProvider{id: "vault"}, "shared", time.Minute); err != nil {
		t.Fatalf("first Register() error = %v", err)
	}

	err := registry.Register(&testProvider{id: "onepassword"}, "shared", time.Minute)
	if err == nil {
		t.Fatal("second Register() error = nil, want duplicate alias error")
	}
}

func TestRegistryRejectsUnknownAlias(t *testing.T) {
	registry := NewRegistry()

	_, err := registry.Resolve(context.Background(), "missing", "secret/data/app", "password")
	if err == nil {
		t.Fatal("Resolve() error = nil, want unknown alias error")
	}
	if !strings.Contains(err.Error(), "not registered") {
		t.Fatalf("Resolve() error = %v, want unknown alias error", err)
	}
}

func TestRegistryList(t *testing.T) {
	registry := NewRegistry()
	provider := &testProvider{id: "vault", value: "secret"}
	if err := registry.Register(provider, "prod_vault", time.Minute); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	keys, err := registry.List(context.Background(), "prod_vault", "secret/data/app")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(keys) != 3 {
		t.Fatalf("List() keys = %d, want 3", len(keys))
	}
	if keys[0] != "key1" || keys[1] != "key2" || keys[2] != "secret/data/app" {
		t.Fatalf("List() keys = %v, want [key1 key2 secret/data/app]", keys)
	}
}

func TestRegistryListUnknownAlias(t *testing.T) {
	registry := NewRegistry()

	_, err := registry.List(context.Background(), "missing", "secret/data/app")
	if err == nil {
		t.Fatal("List() error = nil, want unknown alias error")
	}
	if !strings.Contains(err.Error(), "not registered") {
		t.Fatalf("List() error = %v, want unknown alias error", err)
	}
}

// TestNewProviderUnregisteredType: this test binary links no concrete
// provider package, so the built-in type names are not registered and a
// config naming them fails clearly instead of silently doing nothing.
func TestNewProviderUnregisteredType(t *testing.T) {
	for _, typeName := range []string{"vault", "onepassword", "nosuch"} {
		_, err := NewProvider(Config{Provider: typeName, Address: "http://127.0.0.1:1", Token: "t"})
		if err == nil {
			t.Fatalf("NewProvider(%q) error = nil, want not registered", typeName)
		}
		if !strings.Contains(err.Error(), `"`+typeName+`"`) || !strings.Contains(err.Error(), "not compiled in or not registered") {
			t.Fatalf("NewProvider(%q) error = %v, want it to name the type and say it is not registered", typeName, err)
		}
	}

	registry := NewRegistry()
	err := RegisterConfigs(registry, []Config{{Provider: "vault", Alias: "v", Address: "http://127.0.0.1:1", Token: "t"}})
	if err == nil || !strings.Contains(err.Error(), "not compiled in or not registered") {
		t.Fatalf("RegisterConfigs error = %v, want not registered", err)
	}
	if registry.HasProviders() {
		t.Fatal("a failed RegisterConfigs must not register the alias")
	}
}

func TestRegisterFactory(t *testing.T) {
	const typeName = "test_factory_stub"
	var got Config
	err := RegisterFactory(typeName, func(cfg Config) (Provider, error) {
		got = cfg
		return &testProvider{id: typeName, value: "v"}, nil
	})
	if err != nil {
		t.Fatalf("RegisterFactory: %v", err)
	}

	registry := NewRegistry()
	if err := RegisterConfigs(registry, []Config{{Provider: typeName, Alias: "stub", Address: "addr", CacheTTL: "1m"}}); err != nil {
		t.Fatalf("RegisterConfigs: %v", err)
	}
	if got.Address != "addr" {
		t.Fatalf("factory received %+v, want the config block", got)
	}
	if v, err := registry.Resolve(context.Background(), "stub", "p", "f"); err != nil || v != "v:p:f" {
		t.Fatalf("Resolve = %q, %v", v, err)
	}

	found := false
	for _, name := range RegisteredTypes() {
		found = found || name == typeName
	}
	if !found {
		t.Fatalf("RegisteredTypes() = %v, want %q", RegisteredTypes(), typeName)
	}
	if _, err := NewProvider(Config{Provider: "nosuch"}); err == nil || !strings.Contains(err.Error(), typeName) {
		t.Fatalf("unregistered-type error = %v, want it to list the registered types", err)
	}

	// Duplicate type names are rejected like duplicate aliases, never swapped.
	if err := RegisterFactory(typeName, func(Config) (Provider, error) { return nil, nil }); err == nil || !strings.Contains(err.Error(), "already registered") {
		t.Fatalf("duplicate RegisterFactory error = %v, want already registered", err)
	}
	if v, err := registry.Resolve(context.Background(), "stub", "p", "f"); err != nil || v != "v:p:f" {
		t.Fatalf("after rejected duplicate: Resolve = %q, %v", v, err)
	}

	if err := RegisterFactory("", func(Config) (Provider, error) { return nil, nil }); err == nil {
		t.Fatal("empty type name should be rejected")
	}
	if err := RegisterFactory("test_factory_nil", nil); err == nil {
		t.Fatal("nil factory should be rejected")
	}
}

func TestOnePasswordServiceAccountNeedsSDKPackage(t *testing.T) {
	serviceAccountMu.Lock()
	previous := serviceAccountFactory
	serviceAccountFactory = nil
	serviceAccountMu.Unlock()
	t.Cleanup(func() { RegisterOnePasswordServiceAccount(previous) })

	_, err := NewProvider(Config{Provider: "onepassword", ServiceAccountToken: "ops_x"})
	if err == nil || !strings.Contains(err.Error(), "not supported by this build") {
		t.Fatalf("err = %v, want not supported by this build", err)
	}
}
