package pack

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"strings"
)

type requiredSourceKind uint8

const (
	requiredSourceAny requiredSourceKind = iota
	requiredSourceFile
	requiredSourceDir
)

// validateRequiredSourcePath validates bundle-relative syntax and every path
// component without following symlinks. Required declarations must name real
// content beneath the opened source root; incidental links discovered later
// by the walk are handled separately as warnings.
func validateRequiredSourcePath(root *os.Root, declared string, kind requiredSourceKind) (string, os.FileInfo, error) {
	normalized, err := normalizeSourcePath(declared)
	if err != nil {
		return "", nil, err
	}

	parts := strings.Split(normalized, "/")
	var info os.FileInfo
	for i := range parts {
		current := strings.Join(parts[:i+1], "/")
		info, err = root.Lstat(current)
		if err != nil {
			return "", nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", nil, fmt.Errorf("path component %q is a symlink", current)
		}
		if i < len(parts)-1 && !info.IsDir() {
			return "", nil, fmt.Errorf("path component %q is not a directory", current)
		}
	}

	switch kind {
	case requiredSourceFile:
		if !info.Mode().IsRegular() {
			return "", nil, fmt.Errorf("%q is not a regular file", normalized)
		}
	case requiredSourceDir:
		if !info.IsDir() {
			return "", nil, fmt.Errorf("%q is not a directory", normalized)
		}
	case requiredSourceAny:
		if !info.IsDir() && !info.Mode().IsRegular() {
			return "", nil, fmt.Errorf("%q is neither a regular file nor a directory", normalized)
		}
	}
	return normalized, info, nil
}

func normalizeSourcePath(declared string) (string, error) {
	if declared == "" || strings.Contains(declared, "\\") || path.IsAbs(declared) {
		return "", fmt.Errorf("invalid source path %q", declared)
	}
	// Reject Windows volume syntax on every host so a manifest has the same
	// containment semantics when packed on Unix and Windows.
	if len(declared) >= 2 && declared[1] == ':' {
		return "", fmt.Errorf("invalid source path %q", declared)
	}
	cleaned := path.Clean(declared)
	if cleaned != declared || !fs.ValidPath(cleaned) {
		return "", fmt.Errorf("invalid source path %q", declared)
	}
	return cleaned, nil
}

func pathIsWithin(name, dir string) bool {
	return name == dir || strings.HasPrefix(name, dir+"/")
}

func pathIsAncestor(name, target string) bool {
	return strings.HasPrefix(target, name+"/")
}

func shouldIncludePath(rel string, includedDirs, additionalFiles map[string]bool, mainScript string) bool {
	if rel == mainScript || additionalFiles[rel] {
		return true
	}
	for dir := range includedDirs {
		if pathIsWithin(rel, dir) {
			return true
		}
	}
	return false
}

func shouldTraverseSourceDir(rel string, includedDirs, requiredFiles map[string]bool) bool {
	for dir := range includedDirs {
		if pathIsWithin(rel, dir) || pathIsAncestor(rel, dir) {
			return true
		}
	}
	for file := range requiredFiles {
		if pathIsAncestor(rel, file) {
			return true
		}
	}
	return false
}

func isRequiredSourceComponent(rel string, requiredFiles, requiredDirs map[string]bool) bool {
	for file := range requiredFiles {
		if rel == file || pathIsAncestor(rel, file) {
			return true
		}
	}
	for dir := range requiredDirs {
		if rel == dir || pathIsAncestor(rel, dir) {
			return true
		}
	}
	return false
}
