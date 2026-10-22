package fssecurity

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrPathNotAllowed is the error a path outside the allowed paths wraps.
var ErrPathNotAllowed = errors.New("path is outside allowed directories")

// DeniedError reports a path outside the allowed paths. It wraps
// ErrPathNotAllowed, so callers that turn Go errors into script errors can
// raise it as a PermissionError.
type DeniedError struct {
	Path string
}

func (e *DeniedError) Error() string {
	return fmt.Sprintf("access denied: path '%s' is outside allowed directories", e.Path)
}

func (e *DeniedError) Unwrap() error { return ErrPathNotAllowed }

// CheckPath returns a *DeniedError when path is outside the allowed paths,
// nil otherwise. A nil Config allows everything.
func (c *Config) CheckPath(path string) error {
	if c == nil || c.IsPathAllowed(path) {
		return nil
	}
	return &DeniedError{Path: path}
}

// Config holds the configuration for file system security
type Config struct {
	// AllowedPaths is a list of absolute directory paths that file operations are restricted to.
	// If nil, all paths are allowed (no restrictions).
	// If empty slice (not nil), no paths are allowed (deny all).
	// All paths must be absolute and will be cleaned/normalized.
	AllowedPaths []string
}

// IsPathAllowed checks if the given path is within the allowed paths.
// Returns true if the path is allowed, false otherwise.
//
// SECURITY CRITICAL: This function prevents path traversal attacks.
// It handles:
// - Relative paths (./foo, ../foo)
// - Path traversal (../../etc/passwd)
// - Symlink attacks (by evaluating the real path)
// - Prefix attacks (/allowed vs /allowed-other)
func (c *Config) IsPathAllowed(path string) bool {
	// If nil, no restrictions - allow all
	if c.AllowedPaths == nil {
		return true
	}
	// If empty slice (not nil), deny all
	if len(c.AllowedPaths) == 0 {
		return false
	}

	// Make the path absolute WITHOUT cleaning it: "link/.." means the parent
	// of the link's target to the kernel, so ".." must only be applied after
	// the components before it are resolved (see ResolvePath).
	absPath, err := absNoClean(path)
	if err != nil {
		return false
	}

	// Resolve each allowed directory the same way as the query path
	// (including a possibly-non-existent allowed directory, e.g. before it has
	// been created): otherwise the two sides can disagree about how far a
	// shared prefix like /tmp -> /private/tmp on macOS has been resolved, and
	// an entirely legitimate path is denied. Resolved lazily, once per call.
	realAllowed := make([]string, len(c.AllowedPaths))
	allowedAt := func(i int) string {
		if realAllowed[i] == "" {
			realAllowed[i] = ResolveExistingPrefix(filepath.Clean(c.AllowedPaths[i]))
		}
		return realAllowed[i]
	}
	inside := func(p string) bool {
		for i := range c.AllowedPaths {
			if isWithin(p, allowedAt(i)) {
				return true
			}
		}
		return false
	}

	// SECURITY: resolve the path the way the kernel will when it is opened,
	// following every symlink (dangling ones included), so a link inside the
	// allowed directories cannot lead outside them. A component that does
	// not exist is walked as the directory os.MkdirAll would create there, so
	// each one must itself be inside the allowed directories (or be an
	// ancestor of one, which creating a not-yet-existing allowed directory
	// needs): "/etc/new/../../jail/x" ends inside the jail, but MkdirAll on it
	// would create /etc/new.
	realPath, ok := resolvePath(absPath, func(dir string) bool {
		if inside(dir) {
			return true
		}
		for i := range c.AllowedPaths {
			if isWithin(allowedAt(i), dir) {
				return true
			}
		}
		return false
	})
	return ok && inside(realPath)
}

// isWithin reports whether the clean absolute path p is dir or under it. The
// separator is appended before comparing so /allowed does not match
// /allowed-other.
func isWithin(p, dir string) bool {
	if p == dir {
		return true
	}
	prefix := dir
	if !strings.HasSuffix(prefix, string(os.PathSeparator)) {
		prefix += string(os.PathSeparator)
	}
	return strings.HasPrefix(p, prefix)
}

// NormalizeAllowedPaths makes every entry absolute and clean, relative to the
// working directory now, as each library does when it is registered.
// IsPathAllowed compares absolute resolved paths, so a relative entry (".",
// "./data", a script's own directory) would otherwise never match. Nil (no
// restriction) stays nil and an empty slice (deny all) stays empty; an entry
// that cannot be made absolute is dropped.
func NormalizeAllowedPaths(paths []string) []string {
	if paths == nil {
		return nil
	}
	normalized := make([]string, 0, len(paths))
	for _, p := range paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			continue
		}
		normalized = append(normalized, abs)
	}
	return normalized
}

// ResolveExistingPrefix resolves absPath through symlinks as ResolvePath
// does, returning absPath unchanged when it cannot be resolved (a symlink
// loop).
func ResolveExistingPrefix(absPath string) string {
	if real, ok := ResolvePath(absPath); ok {
		return real
	}
	return absPath
}

// maxSymlinks bounds symlink expansion, as the kernel does, so a loop fails.
const maxSymlinks = 40

// absNoClean makes path absolute without cleaning it.
func absNoClean(path string) (string, error) {
	if filepath.IsAbs(path) {
		return path, nil
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return wd + string(os.PathSeparator) + path, nil
}

// ResolvePath resolves an absolute path the way the kernel does when it is
// opened: component by component, following each symlink (a relative target
// from the link's own directory) before applying any later "..". Unlike
// filepath.EvalSymlinks it also resolves paths that do not fully exist: a
// dangling symlink is followed to where it points, and a component that does
// not exist is taken as the plain directory os.MkdirAll would create there,
// so a later ".." pops it and later symlinks are still followed. It reports
// false for a symlink loop.
func ResolvePath(absPath string) (string, bool) {
	return resolvePath(absPath, nil)
}

// resolvePath is ResolvePath, calling missing (when not nil) with the
// resolved path of each component that does not exist; a false return fails
// the resolution.
func resolvePath(absPath string, missing func(string) bool) (string, bool) {
	// A relative path is taken from the working directory, never from the
	// filesystem root.
	if !filepath.IsAbs(absPath) {
		abs, err := absNoClean(absPath)
		if err != nil {
			return "", false
		}
		absPath = abs
	}

	// Fast path: the whole path exists. EvalSymlinks applies ".." to the
	// resolved prefix, as the kernel does.
	if real, err := filepath.EvalSymlinks(absPath); err == nil {
		return real, true
	}

	vol := filepath.VolumeName(absPath)
	root := vol + string(os.PathSeparator)
	resolved := root
	remaining := splitPath(absPath[len(vol):])
	links := 0
	// absent counts the trailing components of resolved that do not exist.
	// Nothing below a missing directory exists either, so those components
	// need no Lstat, and a ".." that pops one returns to real directories.
	absent := 0
	for len(remaining) > 0 {
		comp := remaining[0]
		remaining = remaining[1:]
		switch comp {
		case "", ".":
			continue
		case "..":
			// Dir of the root is the root, as ".." at "/" is.
			resolved = filepath.Dir(resolved)
			if absent > 0 {
				absent--
			}
			continue
		}
		next := filepath.Join(resolved, comp)
		if absent > 0 {
			if missing != nil && !missing(next) {
				return "", false
			}
			resolved = next
			absent++
			continue
		}
		fi, err := os.Lstat(next)
		if err != nil {
			// Not there (or not reachable): walk on as the directory a
			// create would make, never a lexical join of the rest, so a
			// later ".." and symlink resolve as the kernel will once it
			// exists.
			if missing != nil && !missing(next) {
				return "", false
			}
			resolved = next
			absent = 1
			continue
		}
		if fi.Mode()&os.ModeSymlink == 0 {
			resolved = next
			continue
		}
		if links++; links > maxSymlinks {
			return "", false
		}
		target, err := os.Readlink(next)
		if err != nil {
			return "", false
		}
		if filepath.IsAbs(target) {
			tvol := filepath.VolumeName(target)
			resolved = tvol + string(os.PathSeparator)
			target = target[len(tvol):]
		}
		// A relative target continues from the link's directory, which is
		// resolved as it stands.
		remaining = append(splitPath(target), remaining...)
	}
	return resolved, true
}

// RawDir returns path without its final element, like filepath.Dir but
// without cleaning: filepath.Dir("a/l/../b/f") is "a/b", which the kernel
// resolves differently from the "a/l/../b" that opening the path walks
// through when l is a symlink. Creating RawDir(path) with os.MkdirAll
// therefore makes only directories that checking path with IsPathAllowed
// has already walked. It returns "." for a path with no separator.
func RawDir(path string) string {
	vol := filepath.VolumeName(path)
	i := len(path) - 1
	for i >= len(vol) && !os.IsPathSeparator(path[i]) {
		i--
	}
	dir := path[len(vol) : i+1]
	for len(dir) > 1 && os.IsPathSeparator(dir[len(dir)-1]) {
		dir = dir[:len(dir)-1]
	}
	if dir == "" {
		return vol + "."
	}
	return vol + dir
}

// splitPath splits a path into its components on any separator the platform
// accepts. The rune is compared directly, not truncated to a byte: a byte
// cast would make a multi-byte rune whose low byte is 0x2F (e.g. U+012F)
// split like '/', mis-resolving a legitimate filename.
func splitPath(p string) []string {
	return strings.FieldsFunc(p, func(r rune) bool {
		return r == '/' || r == filepath.Separator
	})
}
