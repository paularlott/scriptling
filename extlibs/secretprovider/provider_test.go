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
