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

// Pack creates a package from srcDir, writing to dst.
// Reads manifest.toml from srcDir. Use force to overwrite an existing dst.
// Returns the SHA-256 hex hash of the written package.
func Pack(srcDir, dst string, force bool) (string, error) {
	// Validate source
	info, err := os.Stat(srcDir)
	if err != nil {
		return "", fmt.Errorf("source not found: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("source must be a directory: %s", srcDir)
	}

	// Require manifest.toml
	if _, err := os.Stat(filepath.Join(srcDir, ManifestFile)); err != nil {
		return "", ErrMissingManifest
	}

	// Check destination
	if !force {
		if _, err := os.Stat(dst); err == nil {
			return "", fmt.Errorf("destination already exists (use -f to overwrite): %s", dst)
		}
	}

	f, err := os.Create(dst)
	if err != nil {
		return "", fmt.Errorf("failed to create package: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	zw := zip.NewWriter(io.MultiWriter(f, h))
	err = filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)

		if rel != ManifestFile &&
			!strings.HasPrefix(rel, LibDir+"/") &&
			!strings.HasPrefix(rel, DocsDir+"/") {
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
		defer src.Close()

		_, err = io.Copy(w, src)
		return err
	})
	if err != nil {
		return "", err
	}
	// Flush zip before reading hash
	if err := zw.Close(); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
