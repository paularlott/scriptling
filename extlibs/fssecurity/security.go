package fssecurity

import (
	"os"
	"path/filepath"
	"strings"
)

// Config holds the configuration for file system security
type Config struct {
	// AllowedPaths is a list of absolute directory paths that file operations are restricted to.
	// If empty, all paths are allowed (no restrictions).
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
	// If no restrictions, allow all
	if len(c.AllowedPaths) == 0 {
		return true
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
	realPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		// If the path doesn't exist yet (for write operations), we can't eval symlinks.
		// In this case, we check the parent directory exists and is allowed,
		// and that the final path (after cleaning) is still within allowed dirs.
		parentDir := filepath.Dir(absPath)
		realParent, parentErr := filepath.EvalSymlinks(parentDir)
		if parentErr != nil {
			// Parent doesn't exist either - check if path is within allowed dirs
			// using the cleaned absolute path
			realPath = absPath
		} else {
			// Parent exists, reconstruct the full path with real parent
			realPath = filepath.Join(realParent, filepath.Base(absPath))
		}
	}

	// Check if the real path starts with any of the allowed paths
	for _, allowedPath := range c.AllowedPaths {
		// Get real path of allowed directory too (in case it's a symlink)
		realAllowed, err := filepath.EvalSymlinks(allowedPath)
		if err != nil {
			// If allowed path doesn't exist, use it as-is (cleaned)
			realAllowed = filepath.Clean(allowedPath)
		}

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
