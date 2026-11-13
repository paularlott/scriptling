package onepassword

import (
	"sync"

	"github.com/paularlott/scriptling/extlibs/secretprovider"
)

// TypeName is the provider = "..." config value for the 1Password Connect provider.
const TypeName = "onepassword"

var (
	registerOnce sync.Once
	registerErr  error
)

// Register adds the 1Password Connect provider to the secretprovider factory table
// under TypeName, so secretprovider.NewProvider and RegisterConfigs accept
// provider = "onepassword" config blocks. Calling it more than once is a no-op;
// it returns an error only if another package already registered TypeName.
func Register() error {
	registerOnce.Do(func() {
		registerErr = secretprovider.RegisterFactory(TypeName, New)
	})
	return registerErr
}
