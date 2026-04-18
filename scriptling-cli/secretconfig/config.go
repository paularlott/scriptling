package secretconfig

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/paularlott/scriptling/extlibs/secretprovider"
)

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
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse secret config %s: %w", path, err)
	}

	registry := secretprovider.NewRegistry()
	if err := secretprovider.RegisterConfigs(registry, cfg.Secrets.Providers); err != nil {
		return nil, fmt.Errorf("failed to register secret providers from %s: %w", path, err)
	}

	return registry, nil
}
