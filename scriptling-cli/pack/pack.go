package pack

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// conventionDirs are the fixed top-level dirs included in a bundle when
// present: MCP tools/resources/prompts, static web assets, and docs.
var conventionDirs = []string{"tools", "resources", "prompts", "webroot", DocsDir}

// Pack creates a package from srcDir, writing to dst. Use force to overwrite
// an existing dst. Returns the SHA-256 hex hash of the written package and a
// list of warnings for skipped files.
//
// Inclusion is manifest-driven: manifest.toml, every dir in libs, the main
// script file (when main names a .py file), and the convention dirs
// (tools/, resources/, prompts/, webroot/, docs/) when present. Dotfiles are
// skipped silently; anything else at the top level produces a warning.
//
// A libs dir listed in the manifest but missing, or a main script file that
// does not exist, is a build error.
func Pack(srcDir, dst string, force bool) (string, []string, error) {
	info, err := os.Stat(srcDir)
	if err != nil {
		return "", nil, fmt.Errorf("source not found: %w", err)
	}
	if !info.IsDir() {
		return "", nil, fmt.Errorf("source must be a directory: %s", srcDir)
	}
	sourceRoot, err := os.OpenRoot(srcDir)
	if err != nil {
		return "", nil, fmt.Errorf("source not readable: %w", err)
	}
	defer sourceRoot.Close()

	// Check the destination early for a useful error, then enforce the same
	// policy atomically again when the completed temporary file is published.
	if !force {
		if _, err := os.Lstat(dst); err == nil {
			return "", nil, fmt.Errorf("destination already exists (use -f to overwrite): %s", dst)
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", nil, fmt.Errorf("destination not accessible: %w", err)
		}
	}

	// An output living inside the source tree would include itself in the
	// walk once the archive outlives it. Compare canonical paths: a symlinked
	// destination parent can route an apparently outside path back inside.
	canonicalSrc, err := filepath.EvalSymlinks(srcDir)
	if err != nil {
		return "", nil, fmt.Errorf("source not readable: %w", err)
	}
	dstParent := filepath.Dir(dst)
	if parentInfo, statErr := os.Stat(dstParent); statErr != nil {
		return "", nil, fmt.Errorf("destination parent not readable: %w", statErr)
	} else if !parentInfo.IsDir() {
		return "", nil, fmt.Errorf("destination parent is not a directory: %s", dstParent)
	}
	canonicalParent, err := filepath.EvalSymlinks(dstParent)
	if err != nil {
		return "", nil, fmt.Errorf("destination parent not readable: %w", err)
	}
	canonicalDst := filepath.Join(canonicalParent, filepath.Base(dst))
	canonicalSrc = filepath.Clean(canonicalSrc)
	if canonicalDst == canonicalSrc || strings.HasPrefix(canonicalDst, canonicalSrc+string(os.PathSeparator)) {
		return "", nil, fmt.Errorf("output %s lives inside the source tree %s", dst, srcDir)
	}

	if _, _, err := validateRequiredSourcePath(sourceRoot, ManifestFile, requiredSourceFile); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil, ErrMissingManifest
		}
		return "", nil, fmt.Errorf("invalid required source %q: %w", ManifestFile, err)
	}
	manifestData, err := sourceRoot.ReadFile(ManifestFile)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read required source %q: %w", ManifestFile, err)
	}
	manifest, err := parseManifest(manifestData)
	if err != nil {
		return "", nil, err
	}

	mainScript := ""
	if strings.HasSuffix(manifest.Main, ".py") {
		mainScript, _, err = validateRequiredSourcePath(sourceRoot, manifest.Main, requiredSourceFile)
		if err != nil {
			return "", nil, fmt.Errorf("main script %q is invalid in %s: %w", manifest.Main, srcDir, err)
		}
	}

	includedDirs := map[string]bool{}
	requiredDirs := map[string]bool{}
	requiredFiles := map[string]bool{ManifestFile: true}
	if mainScript != "" {
		requiredFiles[mainScript] = true
	}

	// Explicit libs are required. The implicit default lib remains optional.
	if len(manifest.Libs) == 0 {
		includedDirs[LibDir] = true
	} else {
		for _, lib := range manifest.Libs {
			normalized, _, err := validateRequiredSourcePath(sourceRoot, lib, requiredSourceDir)
			if err != nil {
				return "", nil, fmt.Errorf("libs dir %q is invalid in %s: %w", lib, srcDir, err)
			}
			includedDirs[normalized] = true
			requiredDirs[normalized] = true
		}
	}
	for _, dir := range conventionDirs {
		includedDirs[dir] = true
	}

	additionalFiles := map[string]bool{}
	for _, declared := range manifest.AdditionalFiles {
		normalized, sourceInfo, err := validateRequiredSourcePath(sourceRoot, strings.TrimRight(declared, "/"), requiredSourceAny)
		if err != nil {
			return "", nil, fmt.Errorf("additional_files entry %q is invalid in %s: %w", declared, srcDir, err)
		}
		if sourceInfo.IsDir() {
			includedDirs[normalized] = true
			requiredDirs[normalized] = true
		} else {
			additionalFiles[normalized] = true
			requiredFiles[normalized] = true
		}
	}

	// Write to a unique temporary sibling. Failed builds leave old artifacts
	// untouched and concurrent builds cannot share a staging path.
	tmpF, err := os.CreateTemp(dstParent, filepath.Base(dst)+".*.tmp")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create package: %w", err)
	}
	tmp := tmpF.Name()
	f := tmpF
	defer func() {
		_ = f.Close()
		_ = os.Remove(tmp)
	}()

	var warnings []string
	h := sha256.New()
	zw := zip.NewWriter(io.MultiWriter(f, h))
	err = filepath.Walk(srcDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		top, _, _ := strings.Cut(rel, "/")

		// Preserve the existing policy of silently omitting dot-prefixed roots.
		if strings.HasPrefix(top, ".") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if info.Mode()&os.ModeSymlink != 0 {
			if isRequiredSourceComponent(rel, requiredFiles, requiredDirs) {
				return fmt.Errorf("required source path %q contains a symlink", rel)
			}
			if shouldIncludePath(rel, includedDirs, additionalFiles, mainScript) {
				warnings = append(warnings, fmt.Sprintf("skipping %s: symlink (pack real files, not links)", rel))
			}
			return nil
		}

		if info.IsDir() {
			if !shouldTraverseSourceDir(rel, includedDirs, requiredFiles) {
				warnings = append(warnings, fmt.Sprintf("skipping %s/: not part of the bundle (declare it in libs or use a convention dir)", rel))
				return filepath.SkipDir
			}
			return nil
		}

		if !shouldIncludePath(rel, includedDirs, additionalFiles, mainScript) && rel != ManifestFile {
			warnings = append(warnings, fmt.Sprintf("skipping %s: not part of the bundle", rel))
			return nil
		}
		if !info.Mode().IsRegular() {
			warnings = append(warnings, fmt.Sprintf("skipping %s: not a regular file", rel))
			return nil
		}

		// Re-check immediately before opening so a required source cannot be
		// validated and then silently skipped if it changes during the walk.
		current, err := sourceRoot.Lstat(rel)
		if err != nil {
			return err
		}
		if current.Mode()&os.ModeSymlink != 0 {
			if isRequiredSourceComponent(rel, requiredFiles, requiredDirs) {
				return fmt.Errorf("required source path %q became a symlink", rel)
			}
			warnings = append(warnings, fmt.Sprintf("skipping %s: symlink (pack real files, not links)", rel))
			return nil
		}
		if !current.Mode().IsRegular() {
			return fmt.Errorf("source path %q is not a regular file", rel)
		}

		w, err := zw.Create(rel)
		if err != nil {
			return err
		}
		src, err := sourceRoot.Open(rel)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(w, src)
		if closeErr := src.Close(); copyErr == nil {
			copyErr = closeErr
		}
		return copyErr
	})
	if err != nil {
		return "", nil, err
	}
	if err := zw.Close(); err != nil {
		return "", nil, err
	}
	if err := f.Close(); err != nil {
		return "", nil, err
	}
	if err := publishPackage(tmp, dst, force); err != nil {
		if !force && errors.Is(err, os.ErrExist) {
			return "", nil, fmt.Errorf("destination already exists (use -f to overwrite): %s", dst)
		}
		return "", nil, fmt.Errorf("failed to publish package: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), warnings, nil
}
