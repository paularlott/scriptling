package extlibs

import (
	"context"
	"testing"
	"time"

	"github.com/paularlott/scriptling/extlibs/secretprovider"
	"github.com/paularlott/scriptling/object"
)

type testSecretProvider struct {
	id    string
	value string
	calls int
}

func (p *testSecretProvider) ID() string { return p.id }

func (p *testSecretProvider) Resolve(_ context.Context, path, field string) (string, error) {
	p.calls++
	if field == "" {
		return p.value + ":" + path, nil
	}
	return p.value + ":" + path + ":" + field, nil
}

func (p *testSecretProvider) List(_ context.Context, path string) ([]string, error) {
	return []string{"db_password", "api_key"}, nil
}

func TestSecretLibraryGet(t *testing.T) {
	registry := secretprovider.NewRegistry()
	if err := registry.Register(&testSecretProvider{id: "vault", value: "resolved"}, "prod_vault", time.Minute); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	lib := NewSecretLibrary(registry)
	result := lib.Functions()["get"].Fn(context.Background(), object.NewKwargs(nil),
		&object.String{Value: "prod_vault"},
		&object.String{Value: "secret/data/app"},
		&object.String{Value: "password"},
	)

	str, ok := result.(*object.String)
	if !ok {
		t.Fatalf("result type = %T, want *object.String", result)
	}
	if str.Value != "resolved:secret/data/app:password" {
		t.Fatalf("result = %q, want resolved secret", str.Value)
	}
}

func TestSecretLibraryGetUnknownAlias(t *testing.T) {
	lib := NewSecretLibrary(secretprovider.NewRegistry())
	result := lib.Functions()["get"].Fn(context.Background(), object.NewKwargs(nil),
		&object.String{Value: "missing"},
		&object.String{Value: "secret/data/app"},
	)

	if _, ok := result.(*object.Error); !ok {
		t.Fatalf("result type = %T, want *object.Error", result)
	}
}

func TestSecretLibraryList(t *testing.T) {
	registry := secretprovider.NewRegistry()
	if err := registry.Register(&testSecretProvider{id: "vault", value: "resolved"}, "prod_vault", time.Minute); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	lib := NewSecretLibrary(registry)
	result := lib.Functions()["list"].Fn(context.Background(), object.NewKwargs(nil),
		&object.String{Value: "prod_vault"},
		&object.String{Value: "secret/data/app"},
	)

	list, ok := result.(*object.List)
	if !ok {
		t.Fatalf("result type = %T, want *object.List", result)
	}
	if len(list.Elements) != 2 {
		t.Fatalf("list length = %d, want 2", len(list.Elements))
	}
	first, _ := list.Elements[0].AsString()
	if first != "db_password" {
		t.Fatalf("first element = %q, want %q", first, "db_password")
	}
}

func TestSecretLibraryListUnknownAlias(t *testing.T) {
	lib := NewSecretLibrary(secretprovider.NewRegistry())
	result := lib.Functions()["list"].Fn(context.Background(), object.NewKwargs(nil),
		&object.String{Value: "missing"},
		&object.String{Value: "secret/data/app"},
	)

	if _, ok := result.(*object.Error); !ok {
		t.Fatalf("result type = %T, want *object.Error", result)
	}
}
