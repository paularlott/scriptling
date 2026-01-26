// Package extlibs provides external libraries that need explicit registration
package extlibs

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/extlibs/fssecurity"
	"github.com/paularlott/scriptling/object"
)

// GlobLibraryInstance holds the configured Glob library instance
type GlobLibraryInstance struct {
	config fssecurity.Config
}

// RegisterGlobLibrary registers the glob library with a Scriptling instance.
// If allowedPaths is empty or nil, all paths are allowed (no restrictions).
// If allowedPaths contains paths, all glob operations are restricted to those directories.
//
// SECURITY: When running untrusted scripts, ALWAYS provide allowedPaths to restrict
// file system access. The security checks prevent:
// - Reading files outside allowed directories
// - Path traversal attacks (../../../etc/passwd)
// - Symlink attacks (symlinks pointing outside allowed dirs)
//
// Example:
//
//	No restrictions - full filesystem access (DANGEROUS for untrusted code)
//	extlibs.RegisterGlobLibrary(s, nil)
//
//	Restricted to specific directories (SECURE)
//	extlibs.RegisterGlobLibrary(s, []string{"/tmp/sandbox", "/home/user/data"})
func RegisterGlobLibrary(registrar object.LibraryRegistrar, allowedPaths []string) {
	config := fssecurity.Config{
		AllowedPaths: allowedPaths,
	}
	globLib := NewGlobLibrary(config)
	registrar.RegisterLibrary(globLib)
}

// NewGlobLibrary creates a new Glob library with the given configuration.
func NewGlobLibrary(config fssecurity.Config) *object.Library {
	// Normalize and validate allowed paths
	normalizedPaths := make([]string, 0, len(config.AllowedPaths))
	for _, p := range config.AllowedPaths {
		absPath, err := filepath.Abs(p)
		if err != nil {
			continue
		}
		normalizedPaths = append(normalizedPaths, filepath.Clean(absPath))
	}
	config.AllowedPaths = normalizedPaths

	instance := &GlobLibraryInstance{config: config}
	return instance.createGlobLibrary()
}

func (g *GlobLibraryInstance) createGlobLibrary() *object.Library {
	return object.NewLibrary(GlobLibraryName, map[string]*object.Builtin{
		"glob": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.RangeArgs(args, 1, 2); err != nil {
					return err
				}
				pattern, err := args[0].AsString()
				if err != nil {
					return err
				}

				rootDir := "."
				if len(args) == 2 {
					rootDir, err = args[1].AsString()
					if err != nil {
						return err
					}
				}

				// Security check on root directory
				if err := g.checkPathSecurity(rootDir); err != nil {
					return err
				}

				matches, globErr := g.glob(pattern, rootDir, false)
				if globErr != nil {
					return globErr
				}

				elements := make([]object.Object, len(matches))
				for i, match := range matches {
					elements[i] = &object.String{Value: match}
				}
				return &object.List{Elements: elements}
			},
			HelpText: `glob(pattern[, root_dir="."]) - Find all pathnames matching a pattern

Returns a list of filenames matching the given pattern. The pattern is a shell-style
wildcard pattern where * matches everything, ? matches any single character,
and [seq] matches any character in seq.

Optional root_dir specifies the directory to search from (default: current directory).`,
		},
		"iglob": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.RangeArgs(args, 1, 2); err != nil {
					return err
				}
				pattern, err := args[0].AsString()
				if err != nil {
					return err
				}

				rootDir := "."
				if len(args) == 2 {
					rootDir, err = args[1].AsString()
					if err != nil {
						return err
					}
				}

				// Security check on root directory
				if err := g.checkPathSecurity(rootDir); err != nil {
					return err
				}

				// Pre-compute all matches
				matches := g.globMatches(pattern, rootDir)
				// Filter through security
				filteredMatches := make([]string, 0, len(matches))
				for _, match := range matches {
					if g.config.IsPathAllowed(match) {
						filteredMatches = append(filteredMatches, match)
					}
				}

				index := 0
				return object.NewIterator(func() (object.Object, bool) {
					if index >= len(filteredMatches) {
						return nil, false
					}
					result := &object.String{Value: filteredMatches[index]}
					index++
					return result, true
				})
			},
			HelpText: `iglob(pattern[, root_dir="."]) - Find all pathnames matching a pattern (returns iterator)

Returns an iterator over the filenames matching the given pattern. This is memory
efficient for large result sets. See glob() for pattern syntax details.`,
		},
		"escape": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}
				pattern, err := args[0].AsString()
				if err != nil {
					return err
				}

				// Escape special characters by building result character by character
				// This prevents double-escaping
				specialChars := map[rune]bool{'*': true, '?': true, '[': true, ']': true}
				var result strings.Builder
				result.Grow(len(pattern) * 2) // Pre-allocate to avoid reallocations

				for _, ch := range pattern {
					if specialChars[ch] {
						result.WriteRune('[')
						result.WriteRune(ch)
						result.WriteRune(']')
					} else {
						result.WriteRune(ch)
					}
				}

				return &object.String{Value: result.String()}
			},
			HelpText: `escape(pattern) - Escape special characters in a pattern

Returns a string with all special characters (*, ?, [, ]) escaped so they
are treated as literal characters rather than wildcards.`,
		},
	}, nil, "Unix shell-style wildcards")
}

// glob returns all files matching pattern
func (g *GlobLibraryInstance) glob(pattern, rootDir string, recursive bool) ([]string, object.Object) {
	matches := g.globMatches(pattern, rootDir)

	// Filter results through security check
	filtered := make([]string, 0, len(matches))
	for _, match := range matches {
		if g.config.IsPathAllowed(match) {
			filtered = append(filtered, match)
		}
	}

	return filtered, nil
}

// globMatches performs the actual glob matching without security filtering
func (g *GlobLibraryInstance) globMatches(pattern, rootDir string) []string {
	// Handle recursive glob pattern **
	var matches []string
	if strings.Contains(pattern, "**") {
		matches = g.doubleStarGlob(pattern, rootDir)
	} else {
		fullPattern := filepath.Join(rootDir, pattern)
		matches, _ = filepath.Glob(fullPattern)
	}
	return matches
}

// doubleStarGlob handles the ** recursive pattern
func (g *GlobLibraryInstance) doubleStarGlob(pattern, rootDir string) []string {
	var results []string

	// Split pattern by **
	parts := strings.Split(pattern, "**")
	if len(parts) != 2 {
		// Fall back to regular glob for malformed patterns
		fullPattern := filepath.Join(rootDir, pattern)
		matches, _ := filepath.Glob(fullPattern)
		return matches
	}

	prefix := strings.TrimSuffix(filepath.Join(rootDir, parts[0]), string(filepath.Separator))
	suffix := strings.TrimPrefix(parts[1], string(filepath.Separator))

	// Find all directories matching prefix
	prefixMatches, _ := filepath.Glob(prefix)
	if len(prefixMatches) == 0 {
		prefixMatches = []string{prefix}
	}

	for _, base := range prefixMatches {
		results = append(results, g.walkAndMatch(base, suffix)...)
	}

	return results
}

// walkAndMatch recursively walks directories and matches suffix pattern
func (g *GlobLibraryInstance) walkAndMatch(base, suffix string) []string {
	var results []string

	// Try direct match (no recursion needed)
	directPath := filepath.Join(base, suffix)
	matches, _ := filepath.Glob(directPath)
	results = append(results, matches...)

	// Recurse into subdirectories
	entries, _ := filepath.Glob(filepath.Join(base, "*"))
	for _, entry := range entries {
		info, err := os.Stat(entry)
		if err != nil {
			continue
		}
		if info.IsDir() {
			results = append(results, g.walkAndMatch(entry, suffix)...)
		}
	}

	return results
}

// checkPathSecurity validates a path and returns an error if access is denied
func (g *GlobLibraryInstance) checkPathSecurity(path string) object.Object {
	if !g.config.IsPathAllowed(path) {
		return errors.NewError("access denied: path '%s' is outside allowed directories", path)
	}
	return nil
}
