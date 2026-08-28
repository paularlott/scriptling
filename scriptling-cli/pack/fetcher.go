package pack

import (
	"fmt"
	"strings"
	"sync"
)

// The scheme registry routes sources with a custom <scheme>:// prefix — such
// as knot://libs, served by a fetcher plugin — to an opener instead of the
// built-in directory / zip / http logic. Built-in schemes cannot be
// registered; http(s) and local paths keep their existing behavior.

// SchemeOpener opens a bundle from a source using the registered scheme. The
// signature matches FetchBundle so openers compose with every existing caller.
type SchemeOpener func(source string, insecure bool, cacheDir string) (*Bundle, error)

var (
	schemeMu      sync.RWMutex
	schemeOpeners = map[string]SchemeOpener{}
)

// builtinSchemes are owned by FetchBundle itself and never routable.
var builtinSchemes = map[string]bool{"http": true, "https": true, "file": true}

// RegisterScheme routes sources with the given scheme to opener. Registering a
// scheme twice, or a built-in (http, https, file), is an error.
func RegisterScheme(scheme string, opener SchemeOpener) error {
	if builtinSchemes[scheme] || !validSchemeName(scheme) {
		return fmt.Errorf("cannot register scheme %q", scheme)
	}
	if opener == nil {
		return fmt.Errorf("cannot register scheme %q with a nil opener", scheme)
	}
	schemeMu.Lock()
	defer schemeMu.Unlock()
	if _, exists := schemeOpeners[scheme]; exists {
		return fmt.Errorf("scheme %q is already registered", scheme)
	}
	schemeOpeners[scheme] = opener
	return nil
}

// RegisteredSchemes returns the currently registered custom schemes.
func RegisteredSchemes() []string {
	schemeMu.RLock()
	defer schemeMu.RUnlock()
	schemes := make([]string, 0, len(schemeOpeners))
	for scheme := range schemeOpeners {
		schemes = append(schemes, scheme)
	}
	return schemes
}

// SchemeFor reports the custom scheme a source carries, if any. It returns
// ("", false) for http(s) URLs, local paths and malformed sources.
func SchemeFor(source string) (string, bool) {
	scheme, rest, found := strings.Cut(source, "://")
	if !found || !validSchemeName(scheme) || builtinSchemes[scheme] {
		return "", false
	}
	if rest == "" || strings.ContainsAny(rest, " \t") {
		return "", false
	}
	schemeMu.RLock()
	_, registered := schemeOpeners[scheme]
	schemeMu.RUnlock()
	if !registered {
		return "", false
	}
	return scheme, true
}

// lookupSchemeOpener returns the opener for a custom-scheme source, or nil.
func lookupSchemeOpener(scheme string) SchemeOpener {
	schemeMu.RLock()
	defer schemeMu.RUnlock()
	return schemeOpeners[scheme]
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
