package bootstrap

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/libloader"
	"github.com/paularlott/scriptling/scriptling-cli/pack"
)

// BaseDir returns the script directory, or the current working directory when no file is provided.
func BaseDir(file string) (string, error) {
	if file != "" {
		return filepath.Dir(file), nil
	}
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to determine current working directory: %w", err)
	}
	return dir, nil
}

// BuildLibDirs constructs the ordered list of library search directories.
func BuildLibDirs(baseDir string, extra []string) []string {
	var dirs []string
	if baseDir != "" {
		dirs = append(dirs, baseDir)
	}
	for _, d := range extra {
		if d != "" {
			dirs = append(dirs, d)
		}
	}
	return dirs
}

// ParseAllowedPaths parses a comma-separated list of paths.
// Returns nil for no restrictions, empty slice for deny-all (paths == "-").
func ParseAllowedPaths(paths string) []string {
	if paths == "" {
		return nil
	}
	if paths == "-" {
		return []string{}
	}
	var result []string
	for _, p := range strings.Split(paths, ",") {
		if p = strings.TrimSpace(p); p != "" {
			result = append(result, p)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// NewPackLoader loads package sources into a loader.
func NewPackLoader(sources []string, insecure bool, cacheDir string) (*pack.Loader, error) {
	if len(sources) == 0 {
		return nil, nil
	}

	loader := pack.NewLoader()
	loader.SetCacheDir(cacheDir)
	for _, src := range sources {
		if err := loader.AddFromPath(src, insecure); err != nil {
			return nil, fmt.Errorf("failed to load package %s: %w", src, err)
		}
	}
	return loader, nil
}

// ChainLoader combines a primary and fallback loader.
func ChainLoader(primary, fallback scriptling.LibraryLoader) scriptling.LibraryLoader {
	switch {
	case primary == nil:
		return fallback
	case fallback == nil:
		return primary
	default:
		return libloader.NewChain(primary, fallback)
	}
}

// ApplyPackLoader adds the package loader behind the current interpreter loader.
func ApplyPackLoader(p *scriptling.Scriptling, packLoader *pack.Loader) {
	if packLoader == nil {
		return
	}
	p.SetLibraryLoader(ChainLoader(p.GetLibraryLoader(), packLoader))
}
