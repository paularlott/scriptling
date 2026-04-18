package secretprovider

import (
	"fmt"
	"time"
)

// Config describes one secret provider registration.
type Config struct {
	Provider string `toml:"provider"`
	Alias    string `toml:"alias"`

	Address      string `toml:"address"`
	Token        string `toml:"token"`
	CacheTTL     string `toml:"cache_ttl"`
	DefaultField string `toml:"default_field"`

	AppRoleID     string `toml:"app_role_id"`
	AppRoleSecret string `toml:"app_role_secret"`
	Namespace     string `toml:"namespace"`
	KVVersion     int    `toml:"kv_version"`

	InsecureSkipTLS bool `toml:"insecure_skip_tls"`
}

// RegisterConfigs creates and registers providers from config blocks.
func RegisterConfigs(registry *Registry, configs []Config) error {
	for _, cfg := range configs {
		provider, err := NewProvider(cfg)
		if err != nil {
			return err
		}

		cacheTTL := DefaultCacheTTL
		if cfg.CacheTTL != "" {
			cacheTTL, err = time.ParseDuration(cfg.CacheTTL)
			if err != nil {
				return fmt.Errorf("secret provider %q has invalid cache_ttl %q: %w", cfg.Provider, cfg.CacheTTL, err)
			}
		}

		if err := registry.Register(provider, cfg.Alias, cacheTTL); err != nil {
			return err
		}
	}

	return nil
}

// NewProvider creates a concrete provider from config.
func NewProvider(cfg Config) (Provider, error) {
	switch cfg.Provider {
	case "vault":
		return NewVaultProvider(cfg)
	case "onepassword":
		return NewOnePasswordProvider(cfg)
	default:
		return nil, fmt.Errorf("unsupported secret provider %q", cfg.Provider)
	}
}
