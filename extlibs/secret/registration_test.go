package secret_test

import (
	"context"
	"testing"
	"time"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/secret"
	"github.com/paularlott/scriptling/extlibs/secretprovider"
	"github.com/paularlott/scriptling/internal/scripttest"
	"github.com/paularlott/scriptling/stdlib"
)

// stubProvider resolves every secret to "<path>/<field>".
type stubProvider struct{}

func (stubProvider) ID() string { return "stub" }
func (stubProvider) Resolve(_ context.Context, path, field string) (string, error) {
	return path + "/" + field, nil
}
func (stubProvider) List(context.Context, string) ([]string, error) { return []string{"a", "b"}, nil }

func TestLibraryNameConstant(t *testing.T) {
	if secret.LibraryName != "scriptling.secret" {
		t.Errorf("secret.LibraryName = %q, want %q (scripts import it by this name)", secret.LibraryName, "scriptling.secret")
	}
}

func TestSecretRegisterAndGet(t *testing.T) {
	reg := secretprovider.NewRegistry()
	if err := reg.Register(stubProvider{}, "vault", time.Minute); err != nil {
		t.Fatal(err)
	}
	p := scriptling.New()
	secret.Register(p, reg)
	got := scripttest.Eval(t, p, `import scriptling.secret as secret
[secret.get("vault", "app/db", "password"), secret.list("vault", "app")]`).Inspect()
	if want := "['app/db/password', ['a', 'b']]"; got != want {
		t.Errorf("secret.get/list = %s, want %s", got, want)
	}
}

func TestSecretUnknownAliasIsCatchable(t *testing.T) {
	p := scriptling.New()
	secret.Register(p, secretprovider.NewRegistry())
	if typ, _ := scripttest.Raises(t, p, "import scriptling.secret as secret\nsecret.get('missing', 'x')"); typ == "" {
		t.Error("an unknown provider alias must raise")
	}
}

func TestSecretNotImportableWithoutRegister(t *testing.T) {
	p := scriptling.New()
	stdlib.RegisterAll(p)
	scripttest.ImportFails(t, p, "scriptling.secret", "it is only available after secret.Register(p, registry)")
}
