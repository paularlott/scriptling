package secretprovider

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Factory builds a provider from one config block.
type Factory func(Config) (Provider, error)

var (
	factoriesMu sync.RWMutex
	factories   = map[string]Factory{}
)

// RegisterFactory makes a provider type available to NewProvider and
// RegisterConfigs under typeName (the provider = "..." value in config).
//
// The concrete providers live in their own packages so an application links
// only the ones it uses; each has a Register function that calls this, e.g.
// vault.Register() or onepassword.Register().
//
// A type name can be registered once: registering it again is an error, the
// same way Registry.Register rejects a duplicate alias, so two packages can
// never silently replace each other's provider.
func RegisterFactory(typeName string, factory Factory) error {
	if typeName == "" {
		return fmt.Errorf("secret provider type name is empty")
	}
	if factory == nil {
		return fmt.Errorf("secret provider type %q: factory is nil", typeName)
	}

	factoriesMu.Lock()
	defer factoriesMu.Unlock()
	if _, ok := factories[typeName]; ok {
		return fmt.Errorf("secret provider type %q already registered", typeName)
	}
	factories[typeName] = factory
	return nil
}

// RegisteredTypes returns the registered provider type names, sorted.
func RegisteredTypes() []string {
	factoriesMu.RLock()
	defer factoriesMu.RUnlock()
	out := make([]string, 0, len(factories))
	for name := range factories {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func lookupFactory(typeName string) (Factory, bool) {
	factoriesMu.RLock()
	defer factoriesMu.RUnlock()
	f, ok := factories[typeName]
	return f, ok
}

func unregisteredTypeError(typeName string) error {
	registered := "none"
	if types := RegisteredTypes(); len(types) > 0 {
		registered = strings.Join(types, ", ")
	}
	return fmt.Errorf("unsupported secret provider %q: provider type not compiled in or not registered (registered types: %s)", typeName, registered)
}
