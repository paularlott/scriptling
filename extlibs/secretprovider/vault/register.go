package vault

import (
	"sync"

	"github.com/paularlott/scriptling/extlibs/secretprovider"
)

// TypeName is the provider = "..." config value for the HashiCorp Vault provider.
const TypeName = "vault"

var (
	registerOnce sync.Once
	registerErr  error
)

// Register adds the HashiCorp Vault provider to the secretprovider factory table
// under TypeName, so secretprovider.NewProvider and RegisterConfigs accept
// provider = "vault" config blocks. Calling it more than once is a no-op;
// it returns an error only if another package already registered TypeName.
func Register() error {
	registerOnce.Do(func() {
		registerErr = secretprovider.RegisterFactory(TypeName, New)
	})
	return registerErr
}
