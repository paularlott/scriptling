package pack

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// The scheme registry routes sources with a custom <scheme>:// prefix — such
// as knot://libs, served by a fetcher plugin — to an opener instead of the
// built-in directory / zip / http logic. Built-in schemes cannot be
// registered; http(s) and local paths keep their existing behavior.
//
// A SchemeRegistry is an independent routing table. Hosts that embed
// scriptling can create their own, register a plugin bridge into it, and
// Unregister on teardown so the next set of plugins can claim the same
// schemes. The CLI (and anything calling the package-level functions) shares
// the process-wide DefaultSchemeRegistry.

// SchemeOpener opens a bundle from a source using the registered scheme. The
// signature matches FetchBundle so openers compose with every existing caller.
// Openers that need a context capture one at registration; see
// pluginpack.Bridge.
type SchemeOpener func(source string, insecure bool, cacheDir string) (*Bundle, error)

// builtinSchemes are owned by FetchBundle itself and never routable.
var builtinSchemes = map[string]bool{"http": true, "https": true, "file": true}

// ErrUnknownScheme reports a source that looks like <scheme>://… but whose
// scheme has no registered opener — almost always a fetcher plugin that was
// never loaded. Callers match on it to add context only they can supply, such
// as the flags or configuration that load a plugin in their environment.
var ErrUnknownScheme = errors.New("no plugin provides the source scheme")

// SchemeRegistry maps custom source schemes to bundle openers. It is safe for
// concurrent use. The zero value is not usable; call NewSchemeRegistry.
type SchemeRegistry struct {
	mu      sync.RWMutex
	openers map[string]SchemeOpener
}

// NewSchemeRegistry returns an empty scheme registry.
func NewSchemeRegistry() *SchemeRegistry {
	return &SchemeRegistry{openers: map[string]SchemeOpener{}}
}

// defaultRegistry is the process-wide registry used by the package-level
// functions and by FetchBundle.
var defaultRegistry = NewSchemeRegistry()

// DefaultSchemeRegistry returns the process-wide registry that FetchBundle and
// the package-level Register/Unregister functions use.
func DefaultSchemeRegistry() *SchemeRegistry { return defaultRegistry }

// Register routes sources with the given scheme to opener. Registering a
// scheme twice, or a built-in (http, https, file), is an error — one scheme
// has one owner. Use Unregister to release it first.
func (r *SchemeRegistry) Register(scheme string, opener SchemeOpener) error {
	if builtinSchemes[scheme] || !validSchemeName(scheme) {
		return fmt.Errorf("cannot register scheme %q", scheme)
	}
	if opener == nil {
		return fmt.Errorf("cannot register scheme %q with a nil opener", scheme)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.openers[scheme]; exists {
		return fmt.Errorf("scheme %q is already registered", scheme)
	}
	r.openers[scheme] = opener
	return nil
}

// Unregister releases a scheme so it can be claimed again, and reports whether
// it was registered. Hosts that reload plugins call this on teardown.
func (r *SchemeRegistry) Unregister(scheme string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, existed := r.openers[scheme]
	delete(r.openers, scheme)
	return existed
}

// Registered returns the registered custom schemes in sorted order.
func (r *SchemeRegistry) Registered() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	schemes := make([]string, 0, len(r.openers))
	for scheme := range r.openers {
		schemes = append(schemes, scheme)
	}
	sort.Strings(schemes)
	return schemes
}

// Lookup returns the opener for a scheme, or nil when it is not registered.
func (r *SchemeRegistry) Lookup(scheme string) SchemeOpener {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.openers[scheme]
}

// FetchBundle opens source through this registry's openers, falling back to
// the built-in directory / zip / URL handling for non-scheme sources.
func (r *SchemeRegistry) FetchBundle(source string, insecure bool, cacheDir string) (*Bundle, error) {
	if scheme, ok := SchemeSyntax(source); ok {
		opener := r.Lookup(scheme)
		if opener == nil {
			return nil, r.unknownSchemeError(scheme, source)
		}
		return opener(source, insecure, cacheDir)
	}
	return fetchBuiltinBundle(source, insecure, cacheDir)
}

// unknownSchemeError explains that a scheme-shaped source has no opener, which
// almost always means the plugin that serves it was not loaded.
//
// The message stays audience-neutral: this package is used by the CLI and by
// embedding hosts alike, and naming CLI flags here would be wrong advice for a
// host with its own registry. Callers that know how plugins get loaded in their
// context match on ErrUnknownScheme and add that detail themselves.
// The advice is deliberately the last clause, so a caller can extend it into a
// sentence that names how plugins load in its context ("…load the plugin that
// serves it with --plugin").
func (r *SchemeRegistry) unknownSchemeError(scheme, source string) error {
	const fix = "load the plugin that serves it"
	if available := r.Registered(); len(available) > 0 {
		return fmt.Errorf("%w %q for %s (available schemes: %s): %s",
			ErrUnknownScheme, scheme, source, strings.Join(available, ", "), fix)
	}
	return fmt.Errorf("%w %q for %s: %s", ErrUnknownScheme, scheme, source, fix)
}

// =========================================================================
// Scheme syntax
// =========================================================================

// SchemeSyntax reports whether source looks like a custom <scheme>://rest
// source, regardless of whether any opener is registered for it. It returns
// ("", false) for http(s) URLs (owned by FetchBundle), local paths, and
// malformed sources. Callers use it to tell a missing plugin apart from a
// missing file.
func SchemeSyntax(source string) (string, bool) {
	scheme, rest, found := strings.Cut(source, "://")
	if !found || !validSchemeName(scheme) || builtinSchemes[scheme] {
		return "", false
	}
	if rest == "" || strings.ContainsAny(rest, " \t") {
		return "", false
	}
	return scheme, true
}

// validSchemeName reports whether scheme is a plausible URI scheme: a letter
// followed by letters, digits, "+" "-" ".".
func validSchemeName(scheme string) bool {
	if scheme == "" {
		return false
	}
	for i, r := range scheme {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
		case i > 0 && (r >= '0' && r <= '9' || r == '+' || r == '-' || r == '.'):
		default:
			return false
		}
	}
	return true
}
