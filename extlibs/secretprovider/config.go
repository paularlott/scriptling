package secretprovider

import (
	"fmt"
	"time"
)

// Config describes one secret provider registration.
type Config struct {
	Provider string `toml:"provider"`
	Alias    string `toml:"alias"`

	Address string `toml:"address"`
	Token   string `toml:"token"`
	// ServiceAccountToken selects a 1Password service account instead of
	// 1Password Connect for the onepassword provider. Needs the
	// github.com/paularlott/scriptling-onepassword extension linked in.
	ServiceAccountToken string `toml:"service_account_token"`
	CacheTTL            string `toml:"cache_ttl"`
	DefaultField        string `toml:"default_field"`

	AppRoleID     string `toml:"app_role_id"`
	AppRoleSecret string `toml:"app_role_secret"`
	Namespace     string `toml:"namespace"`
	KVVersion     int    `toml:"kv_version"`

	InsecureSkipTLS bool `toml:"insecure_skip_tls"`
}

// RegisterConfigs creates and registers providers from config blocks. Each
// block's provider type must have been registered with RegisterFactory (for
// example by vault.Register or onepassword.Register).
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

// NewProvider creates a concrete provider from config, using the factory
// registered for cfg.Provider. A onepassword config with
// service_account_token goes to the 1Password service-account constructor
// (see RegisterOnePasswordServiceAccount) instead.
func NewProvider(cfg Config) (Provider, error) {
	if cfg.Provider == onePasswordTypeName && cfg.ServiceAccountToken != "" {
		return newOnePasswordServiceAccountProvider(cfg)
	}
	factory, ok := lookupFactory(cfg.Provider)
	if !ok {
		return nil, unregisteredTypeError(cfg.Provider)
	}
	return factory(cfg)
}
