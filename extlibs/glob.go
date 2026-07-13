// Package extlibs provides external libraries that need explicit registration
package extlibs

import (
	"context"
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
	// IMPORTANT: nil means no restrictions, empty slice means deny all
	if config.AllowedPaths != nil {
		normalizedPaths := make([]string, 0, len(config.AllowedPaths))
		for _, p := range config.AllowedPaths {
			absPath, err := filepath.Abs(p)
			if err != nil {
				continue
			}
			normalizedPaths = append(normalizedPaths, filepath.Clean(absPath))
		}
		config.AllowedPaths = normalizedPaths
	}

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

				recursive, includeHidden, oErr := parseGlobKwargs(kwargs)
				if oErr != nil {
					return oErr
				}

				matches := globMatches(ctx, g.config, pattern, rootDir, recursive, includeHidden)

				elements := make([]object.Object, len(matches))
				for i, match := range matches {
					elements[i] = object.NewString(match)
				}
				return &object.List{Elements: elements}
			},
			HelpText: globGlobHelp,
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

				recursive, includeHidden, oErr := parseGlobKwargs(kwargs)
				if oErr != nil {
					return oErr
				}

				// Pre-compute all matches so the iterator is a thin cursor; the
				// recursive path already runs a bounded parallel walk internally.
				matches := globMatches(ctx, g.config, pattern, rootDir, recursive, includeHidden)

				index := 0
				return object.NewIterator(func() (object.Object, bool) {
					if index >= len(matches) {
						return nil, false
					}
					result := object.NewString(matches[index])
					index++
					return result, true
				})
			},
			HelpText: iglobHelp,
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

				return object.NewString(result.String())
			},
			HelpText: escapeHelp,
		},
	}, nil, "Unix shell-style wildcards")
}

// parseGlobKwargs reads the keyword-only options shared by glob and iglob.
func parseGlobKwargs(kwargs object.Kwargs) (recursive bool, includeHidden bool, errObj object.Object) {
	if v := kwargs.Get("recursive"); v != nil {
		b, err := v.AsBool()
		if err != nil {
			return false, false, err
		}
		recursive = b
	}
	if v := kwargs.Get("include_hidden"); v != nil {
		b, err := v.AsBool()
		if err != nil {
			return false, false, err
		}
		includeHidden = b
	}
	return recursive, includeHidden, nil
}

const globGlobHelp = `glob(pattern[, root_dir="."], *, recursive=False, include_hidden=False) -> list

Find all pathnames matching a shell-style wildcard pattern.

Returns a list of filenames matching the given pattern. The pattern is a
shell-style wildcard pattern where * matches everything except a path
separator, ? matches any single character, [seq] matches any character in
seq, and [!seq] matches any character not in seq. Results are returned in
arbitrary order; an empty list is returned when there are no matches.

Parameters:
  pattern        Shell-style wildcard pattern to match.
  root_dir       Directory to search from (default: current directory).
  recursive      When True, ** matches files and directories recursively,
                 descending into every subdirectory (default: False). When
                 False, ** is treated as *.
  include_hidden When True, entries whose name starts with "." are matched;
                 when False (the default) they are skipped.

Recursive searches use a bounded parallel directory walk, the same worker
model as scriptling.grep.`

const iglobHelp = `iglob(pattern[, root_dir="."], *, recursive=False, include_hidden=False) -> iterator

Find all pathnames matching a shell-style wildcard pattern, returned as an
iterator instead of a list. This is memory efficient for large result sets.
See glob() for pattern syntax and parameter details.`

const escapeHelp = `escape(pattern) - Escape special characters in a pattern

Returns a string with all special characters (*, ?, [, ]) escaped so they
are treated as literal characters rather than wildcards.`

// checkPathSecurity validates a path and returns an error if access is denied
func (g *GlobLibraryInstance) checkPathSecurity(path string) object.Object {
	if !g.config.IsPathAllowed(path) {
		return errors.NewPermissionError("access denied: path '%s' is outside allowed directories", path)
	}
	return nil
}
