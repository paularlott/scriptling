package pack

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
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
	// Validate source
	info, err := os.Stat(srcDir)
	if err != nil {
		return "", nil, fmt.Errorf("source not found: %w", err)
	}
	if !info.IsDir() {
		return "", nil, fmt.Errorf("source must be a directory: %s", srcDir)
	}

	manifest, err := ReadManifestFromDir(srcDir)
	if err != nil {
		return "", nil, err
	}

	// The main script (when main names a .py file) is included verbatim.
	mainScript := ""
	if strings.HasSuffix(manifest.Main, ".py") {
		mainScript = manifest.Main
	}

	// Validate explicitly declared libs dirs exist. The default ["lib"] is
	// not validated — a minimal app may have no lib/ dir at all.
	for _, d := range manifest.Libs {
		di, err := os.Stat(filepath.Join(srcDir, filepath.FromSlash(d)))
		if err != nil || !di.IsDir() {
			return "", nil, fmt.Errorf("libs dir %q not found in %s", d, srcDir)
		}
	}
	if mainScript != "" {
		if _, err := os.Stat(filepath.Join(srcDir, filepath.FromSlash(mainScript))); err != nil {
			return "", nil, fmt.Errorf("main script %q not found in %s", mainScript, srcDir)
		}
	}

	includedDirs := map[string]bool{}
	for _, d := range manifest.LibDirs() {
		includedDirs[d] = true
	}
	for _, d := range conventionDirs {
		includedDirs[d] = true
	}

	// Check destination
	if !force {
		if _, err := os.Stat(dst); err == nil {
			return "", nil, fmt.Errorf("destination already exists (use -f to overwrite): %s", dst)
		}
	}

	f, err := os.Create(dst)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create package: %w", err)
	}
	defer f.Close()

	var warnings []string
	h := sha256.New()
	zw := zip.NewWriter(io.MultiWriter(f, h))
	err = filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil // the walk root itself
		}
		top, _, _ := strings.Cut(rel, "/")

		// Dotfiles and dot-dirs are skipped silently (.git, .DS_Store, ...).
		if strings.HasPrefix(top, ".") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if info.IsDir() {
			// Unknown top-level dirs are excluded with a warning (once).
			// Exception: don't skip a dir that contains the main script.
			if rel != "." && !strings.Contains(rel, "/") && !includedDirs[rel] {
				if mainScript != "" && strings.HasPrefix(mainScript, rel+"/") {
					return nil // main script lives under here — keep walking
				}
				warnings = append(warnings, fmt.Sprintf("skipping %s/: not part of the bundle (declare it in libs or use a convention dir)", rel))
				return filepath.SkipDir
			}
			return nil
		}

		if rel != ManifestFile && rel != mainScript && !includedDirs[top] {
			warnings = append(warnings, fmt.Sprintf("skipping %s: not part of the bundle", rel))
			return nil
		}

		w, err := zw.Create(rel)
		if err != nil {
			return err
		}

		src, err := os.Open(path)
		if err != nil {
			return err
		}
		_, err = io.Copy(w, src)
		src.Close()
		return err
	})
	if err != nil {
		return "", nil, err
	}
	// Flush zip before reading hash
	if err := zw.Close(); err != nil {
		return "", nil, err
	}
	return hex.EncodeToString(h.Sum(nil)), warnings, nil
}
