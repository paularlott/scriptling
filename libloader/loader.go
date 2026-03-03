// Package libloader provides a flexible library loading system for Scriptling.
// It supports chaining multiple loaders together, allowing libraries to be loaded
// from various sources (filesystem, API, memory, etc.) in a prioritized order.
//
// The package follows Python's module loading conventions, supporting both:
//   - Folder structure: libs/knot/groups.py → import knot.groups (preferred)
//   - Flat structure: libs/knot.groups.py → import knot.groups (legacy)
package libloader

import "strings"

// LibraryLoader attempts to load a library by name.
// Implementations can load from various sources: filesystem, API, memory, etc.
type LibraryLoader interface {
	// Load attempts to load a library by name.
	// Returns the library source code, whether it was found, and any error.
	// If the library is not found, returns (nil, false, nil).
	// If there's an error (e.g., network failure), returns (nil, false, error).
	Load(name string) (source string, found bool, err error)

	// Description returns a human-readable description of this loader.
	// Used for debugging and logging.
	Description() string
}

// Chain tries multiple loaders in sequence until one succeeds.
// Loaders are tried in the order they are added.
type Chain struct {
	loaders []LibraryLoader
}

// NewChain creates a new loader chain with the given loaders.
// Loaders are tried in the order provided.
func NewChain(loaders ...LibraryLoader) *Chain {
	return &Chain{loaders: loaders}
}

// Add appends a loader to the end of the chain.
func (c *Chain) Add(loader LibraryLoader) {
	c.loaders = append(c.loaders, loader)
}

// Load tries each loader in sequence until one finds the library.
// Returns the first successful result, or (nil, false, nil) if no loader found it.
// Returns an error immediately if any loader encounters an error.
func (c *Chain) Load(name string) (string, bool, error) {
	for _, loader := range c.loaders {
		source, found, err := loader.Load(name)
		if err != nil {
			return "", false, err
		}
		if found {
			return source, true, nil
		}
	}
	return "", false, nil
}

// Description returns a description of all loaders in the chain.
func (c *Chain) Description() string {
	if len(c.loaders) == 0 {
		return "empty loader chain"
	}
	if len(c.loaders) == 1 {
		return c.loaders[0].Description()
	}

	parts := make([]string, 0, len(c.loaders))
	for _, loader := range c.loaders {
		parts = append(parts, loader.Description())
	}
	return "chain: " + strings.Join(parts, " → ")
}

// Loaders returns the list of loaders in the chain.
func (c *Chain) Loaders() []LibraryLoader {
	return c.loaders
}

// MemoryLoader loads libraries from an in-memory map.
// Useful for testing and for registering libraries programmatically.
type MemoryLoader struct {
	libraries map[string]string
	desc      string
}

// NewMemoryLoader creates a new memory loader with the given libraries.
func NewMemoryLoader(libraries map[string]string) *MemoryLoader {
	return &MemoryLoader{
		libraries: libraries,
		desc:      "memory",
	}
}

// NewMemoryLoaderWithDescription creates a memory loader with a custom description.
func NewMemoryLoaderWithDescription(libraries map[string]string, description string) *MemoryLoader {
	return &MemoryLoader{
		libraries: libraries,
		desc:      description,
	}
}

// Load returns the library source if it exists in memory.
func (m *MemoryLoader) Load(name string) (string, bool, error) {
	source, found := m.libraries[name]
	return source, found, nil
}

// Description returns the loader description.
func (m *MemoryLoader) Description() string {
	return m.desc
}

// Set adds or updates a library in memory.
func (m *MemoryLoader) Set(name, source string) {
	if m.libraries == nil {
		m.libraries = make(map[string]string)
	}
	m.libraries[name] = source
}

// Remove removes a library from memory.
func (m *MemoryLoader) Remove(name string) {
	delete(m.libraries, name)
}

// FuncLoader is a loader that uses a function to load libraries.
// Useful for simple custom loaders without implementing the full interface.
type FuncLoader struct {
	fn   func(name string) (string, bool, error)
	desc string
}

// NewFuncLoader creates a loader from a function.
func NewFuncLoader(fn func(name string) (string, bool, error), description string) *FuncLoader {
	return &FuncLoader{fn: fn, desc: description}
}

// Load calls the wrapped function.
func (f *FuncLoader) Load(name string) (string, bool, error) {
	return f.fn(name)
}

// Description returns the loader description.
func (f *FuncLoader) Description() string {
	return f.desc
}
