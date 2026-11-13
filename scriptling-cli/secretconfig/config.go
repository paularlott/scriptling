package secretconfig

import (
	"fmt"
	"os"
	"sync"

	"github.com/BurntSushi/toml"
	"github.com/paularlott/scriptling/extlibs/secretprovider"
	"github.com/paularlott/scriptling/extlibs/secretprovider/onepassword"
	"github.com/paularlott/scriptling/extlibs/secretprovider/vault"
	"github.com/paularlott/scriptling/tomllimit"
)

var (
	registerTypesOnce sync.Once
	registerTypesErr  error
)

// RegisterProviderTypes makes the provider types the CLI supports available
// to secret configs: vault and onepassword (1Password Connect). A onepassword
// block with service_account_token additionally needs the
// scriptling-onepassword extension linked in (the -onepassword builds).
// It runs once; later calls return the first result.
func RegisterProviderTypes() error {
	registerTypesOnce.Do(func() {
		if err := vault.Register(); err != nil {
			registerTypesErr = err
			return
		}
		registerTypesErr = onepassword.Register()
	})
	return registerTypesErr
}

type fileConfig struct {
	Secrets struct {
		Providers []secretprovider.Config `toml:"provider"`
	} `toml:"secrets"`
}

// LoadRegistryFile reads a TOML config file and registers the configured providers.
func LoadRegistryFile(path string) (*secretprovider.Registry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read secret config %s: %w", path, err)
	}

	var cfg fileConfig
	if err := tomllimit.Check(string(data)); err != nil {
		return nil, fmt.Errorf("failed to parse secret config %s: %w", path, err)
	}
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse secret config %s: %w", path, err)
	}

	if err := RegisterProviderTypes(); err != nil {
		return nil, err
	}
	registry := secretprovider.NewRegistry()
	if err := secretprovider.RegisterConfigs(registry, cfg.Secrets.Providers); err != nil {
		return nil, fmt.Errorf("failed to register secret providers from %s: %w", path, err)
	}

	return registry, nil
}
