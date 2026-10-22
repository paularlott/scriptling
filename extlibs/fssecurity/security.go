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

	// SECURITY: resolve the path the way the kernel will when it is opened,
	// following every symlink (dangling ones included), so a link inside the
	// allowed directories cannot lead outside them.
	realPath, ok := ResolvePath(absPath)
	if !ok {
		return false
	}

	// Check if the real path starts with any of the allowed paths
	for _, allowedPath := range c.AllowedPaths {
		// Resolve the allowed directory the same way as the query path
		// (including a possibly-non-existent allowed directory, e.g. before
		// it has been created): otherwise the two sides can disagree about
		// how far a shared prefix like /tmp -> /private/tmp on macOS has
		// been resolved, and an entirely legitimate path is denied.
		realAllowed := ResolveExistingPrefix(filepath.Clean(allowedPath))

		// Ensure allowed path ends with separator for proper prefix matching
		// This prevents /allowed matching /allowed-other
		allowedPrefix := realAllowed
		if !strings.HasSuffix(allowedPrefix, string(os.PathSeparator)) {
			allowedPrefix += string(os.PathSeparator)
		}

		// Check if path is exactly the allowed path or is under it
		if realPath == realAllowed || strings.HasPrefix(realPath+string(os.PathSeparator), allowedPrefix) {
			return true
		}
	}

	return false
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
// dangling symlink is followed to where it points, and components that do not
// exist are appended as written, since a file created there lands there. It
// reports false for a symlink loop.
func ResolvePath(absPath string) (string, bool) {
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
	for len(remaining) > 0 {
		comp := remaining[0]
		remaining = remaining[1:]
		switch comp {
		case "", ".":
			continue
		case "..":
			resolved = filepath.Dir(resolved)
			continue
		}
		next := filepath.Join(resolved, comp)
		fi, err := os.Lstat(next)
		if err != nil {
			// Nothing exists from here on, so nothing further can be a
			// symlink: the rest applies as written.
			return filepath.Join(append([]string{next}, remaining...)...), true
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

// splitPath splits a path into its components on any separator the platform
// accepts. The rune is compared directly, not truncated to a byte: a byte
// cast would make a multi-byte rune whose low byte is 0x2F (e.g. U+012F)
// split like '/', mis-resolving a legitimate filename.
func splitPath(p string) []string {
	return strings.FieldsFunc(p, func(r rune) bool {
		return r == '/' || r == filepath.Separator
	})
}
