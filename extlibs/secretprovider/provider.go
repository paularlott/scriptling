package secretprovider

import (
	"context"
	"fmt"
	"regexp"
	"sync"
	"time"
)

const DefaultCacheTTL = 5 * time.Minute

var aliasPattern = regexp.MustCompile(`^[a-z0-9_]+$`)

// Provider resolves secrets from an external source.
type Provider interface {
	ID() string
	Resolve(ctx context.Context, path string, field string) (string, error)
	List(ctx context.Context, path string) ([]string, error)
}

type registration struct {
	provider Provider
	alias    string
	cacheTTL time.Duration
}

type cacheEntry struct {
	value     string
	expiresAt time.Time
}

// Registry stores secret providers by alias and caches resolved values.
type Registry struct {
	mu      sync.RWMutex
	byAlias map[string]registration
	cache   sync.Map
}

// NewRegistry creates an empty secret provider registry.
func NewRegistry() *Registry {
	return &Registry{
		byAlias: make(map[string]registration),
	}
}

// Register adds a provider under an alias.
func (r *Registry) Register(p Provider, alias string, cacheTTL time.Duration) error {
	if r == nil {
		return fmt.Errorf("secret registry is nil")
	}
	if p == nil {
		return fmt.Errorf("secret provider is nil")
	}
	if alias == "" {
		alias = p.ID()
	}
	if !aliasPattern.MatchString(alias) {
		return fmt.Errorf("invalid secret provider alias %q", alias)
	}
	if cacheTTL <= 0 {
		cacheTTL = DefaultCacheTTL
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if existing, ok := r.byAlias[alias]; ok {
		return fmt.Errorf("secret provider alias %q already registered for %q", alias, existing.provider.ID())
	}

	r.byAlias[alias] = registration{
		provider: p,
		alias:    alias,
		cacheTTL: cacheTTL,
	}

	return nil
}

// Resolve fetches a secret using the provider registered for alias.
func (r *Registry) Resolve(ctx context.Context, alias, path, field string) (string, error) {
	if r == nil {
		return "", fmt.Errorf("secret registry is nil")
	}

	r.mu.RLock()
	entry, ok := r.byAlias[alias]
	r.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("secret provider alias %q not registered", alias)
	}

	cacheKey := fmt.Sprintf("%s:%s:%s", entry.alias, path, field)
	if cached, ok := r.cache.Load(cacheKey); ok {
		existing := cached.(cacheEntry)
		if time.Now().Before(existing.expiresAt) {
			return existing.value, nil
		}
		r.cache.Delete(cacheKey)
	}

	value, err := entry.provider.Resolve(ctx, path, field)
	if err != nil {
		return "", err
	}

	r.cache.Store(cacheKey, cacheEntry{
		value:     value,
		expiresAt: time.Now().Add(entry.cacheTTL),
	})

	return value, nil
}

// List returns the keys at a path using the provider registered for alias.
func (r *Registry) List(ctx context.Context, alias, path string) ([]string, error) {
	if r == nil {
		return nil, fmt.Errorf("secret registry is nil")
	}

	r.mu.RLock()
	entry, ok := r.byAlias[alias]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("secret provider alias %q not registered", alias)
	}

	return entry.provider.List(ctx, path)
}

// HasProviders reports whether any aliases are registered.
func (r *Registry) HasProviders() bool {
	if r == nil {
		return false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.byAlias) > 0
}

// Aliases returns the registered aliases in unspecified order.
func (r *Registry) Aliases() []string {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]string, 0, len(r.byAlias))
	for alias := range r.byAlias {
		out = append(out, alias)
	}
	return out
}

// ResetForTests clears registrations and cache. Intended for tests only.
func (r *Registry) ResetForTests() {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.byAlias = make(map[string]registration)
	r.mu.Unlock()
	r.cache.Range(func(key, _ any) bool {
		r.cache.Delete(key)
		return true
	})
}
