package fssecurity

import (
	"os"
	"path/filepath"
	"strings"
)

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

	// Get absolute path to prevent relative path attacks
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	// Clean the path to resolve any .. or . components
	absPath = filepath.Clean(absPath)

	// SECURITY: Evaluate symlinks to get the real path
	// This prevents symlink attacks where a symlink inside allowed dirs
	// points to a location outside allowed dirs.
	// Note: EvalSymlinks also cleans the path and makes it absolute
	realPath := resolveExistingPrefix(absPath)

	// Check if the real path starts with any of the allowed paths
	for _, allowedPath := range c.AllowedPaths {
		// Resolve the allowed directory the same way as the query path
		// (including a possibly-non-existent allowed directory, e.g. before
		// it has been created): otherwise the two sides can disagree about
		// how far a shared prefix like /tmp -> /private/tmp on macOS has
		// been resolved, and an entirely legitimate path is denied.
		realAllowed := resolveExistingPrefix(filepath.Clean(allowedPath))

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

// resolveExistingPrefix resolves symlinks in path for a write-style check
// where path itself (and possibly several trailing components) may not
// exist yet. It walks up from path until it finds the nearest ancestor that
// does exist, resolves that ancestor's symlinks, and rejoins the
// not-yet-existing suffix onto the resolved ancestor.
//
// A single level of "check the immediate parent" is not enough: creating a
// new file several directories deep in one call (a fresh extraction
// directory, a multi-level config path) means neither the file nor its
// immediate parent exist yet, so stopping at one level silently falls back
// to the unresolved, uncleaned path — which can either wrongly deny a
// legitimate nested path (e.g. on a host where an allowed directory itself
// sits under a symlink, such as macOS's /tmp -> /private/tmp) or, more
// importantly, wrongly ALLOW a path where an intermediate, not-yet-visible
// component is actually a symlink that escapes the allowed directories.
// Walking to the nearest existing ancestor closes both gaps at every depth,
// not just one level.
func resolveExistingPrefix(absPath string) string {
	if real, err := filepath.EvalSymlinks(absPath); err == nil {
		return real
	}

	suffix := ""
	dir := absPath
	for {
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached the filesystem root without finding an existing
			// ancestor; nothing left to resolve against.
			return absPath
		}
		suffix = filepath.Join(filepath.Base(dir), suffix)
		dir = parent

		if realDir, err := filepath.EvalSymlinks(dir); err == nil {
			return filepath.Join(realDir, suffix)
		}
	}
}
