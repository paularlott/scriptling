package secretprovider

import (
	"fmt"
	"sync"
)

var (
	serviceAccountMu      sync.RWMutex
	serviceAccountFactory func(Config) (Provider, error)
)

// RegisterOnePasswordServiceAccount installs the constructor for 1Password
// service account providers. The github.com/paularlott/scriptling-onepassword
// extension calls it from init: the 1Password SDK is large, so it lives
// outside Scriptling, only binaries that import the extension link it, and a
// service_account_token config fails clearly everywhere else.
func RegisterOnePasswordServiceAccount(factory func(Config) (Provider, error)) {
	serviceAccountMu.Lock()
	defer serviceAccountMu.Unlock()
	serviceAccountFactory = factory
}

func newOnePasswordServiceAccountProvider(cfg Config) (Provider, error) {
	if cfg.Address != "" || cfg.Token != "" {
		return nil, fmt.Errorf("onepassword secret provider: use either service_account_token, or address and token for 1Password Connect, not both")
	}

	serviceAccountMu.RLock()
	factory := serviceAccountFactory
	serviceAccountMu.RUnlock()

	if factory == nil {
		return nil, fmt.Errorf("onepassword secret provider: 1Password service accounts are not supported by this build (they need the github.com/paularlott/scriptling-onepassword extension)")
	}
	return factory(cfg)
}
