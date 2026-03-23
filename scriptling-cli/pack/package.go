package pack

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)


const (
	Extension    = ".zip"
	ManifestFile = "manifest.toml"
	LibDir       = "lib"
	DocsDir      = "docs"
)

var (
	ErrInvalidPackage  = errors.New("invalid package format")
	ErrMissingManifest = errors.New("missing manifest.toml")
	ErrInvalidManifest = errors.New("invalid manifest format")
	ErrModuleNotFound  = errors.New("module not found in package")
	ErrFetchFailed     = errors.New("failed to fetch package")
)

// Manifest describes package metadata.
type Manifest struct {
	Name        string `toml:"name"`
	Version     string `toml:"version"`
	Description string `toml:"description,omitempty"`
	Main        string `toml:"main,omitempty"` // module.function entry point
}

// Package represents a loaded package.
// All file contents are decompressed into memory at Open time.
// docs/ entries are intentionally excluded; use ZipDocReader for those.
type Package struct {
	Manifest Manifest
	hasDocs  bool
	files    map[string][]byte
}

// ReadManifestFromDir reads manifest.toml from a source directory.
func ReadManifestFromDir(dir string) (Manifest, error) {
	data, err := os.ReadFile(filepath.Join(dir, ManifestFile))
	if err != nil {
		return Manifest{}, ErrMissingManifest
	}
	var m Manifest
	if _, err := toml.NewDecoder(bytes.NewReader(data)).Decode(&m); err != nil {
		return Manifest{}, ErrInvalidManifest
	}
	return m, nil
}

// bytesReaderAt wraps a byte slice as an io.ReaderAt.
type bytesReaderAt []byte

func (b bytesReaderAt) ReadAt(p []byte, off int64) (int, error) {
	if off >= int64(len(b)) {
		return 0, nil
	}
	return copy(p, b[off:]), nil
}
func Open(r io.ReaderAt, size int64) (*Package, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, ErrInvalidPackage
	}

	p := &Package{
		files: make(map[string][]byte, len(zr.File)),
	}
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
			// Track docs presence but skip loading — loaded on demand by the docs viewer.
		if strings.HasPrefix(f.Name, DocsDir+"/") {
			p.hasDocs = true
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", f.Name, err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", f.Name, err)
		}
		p.files[f.Name] = data
	}

	// Parse manifest
	manifestData, ok := p.files[ManifestFile]
	if !ok {
		return nil, ErrMissingManifest
	}
	if _, err := toml.NewDecoder(bytes.NewReader(manifestData)).Decode(&p.Manifest); err != nil {
		return nil, ErrInvalidManifest
	}

	return p, nil
}

// OpenFile opens a package from a local file path.
func OpenFile(path string) (*Package, error) {
	data, err := FetchFile(path)
	if err != nil {
		return nil, err
	}
	return Open(bytes.NewReader(data), int64(len(data)))
}

// OpenURL opens a package from a URL.
func OpenURL(url string, insecure bool) (*Package, error) {
	data, err := Fetch(url, insecure)
	if err != nil {
		return nil, err
	}
	return Open(bytesReaderAt(data), int64(len(data)))
}

// ReadFile reads a file from the package by path.
func (p *Package) ReadFile(name string) ([]byte, error) {
	data, ok := p.files[name]
	if !ok {
		return nil, ErrModuleNotFound
	}
	return data, nil
}

// List returns file names under a directory prefix within the package.
func (p *Package) List(dir string) []string {
	prefix := dir + "/"
	var result []string
	for name := range p.files {
		if strings.HasPrefix(name, prefix) {
			result = append(result, name)
		}
	}
	return result
}

// HasDocs returns true if the package contains a docs folder.
// Since docs/ is not loaded into p.files, we track this separately.
func (p *Package) HasDocs() bool {
	return p.hasDocs
}

// DocReader provides access to docs/ content from a package source.
type DocReader interface {
	// Name returns a display name for this source.
	Name() string
	// ListDocs returns all doc file paths relative to docs/ (e.g. "guide.md").
	ListDocs() []string
	// ReadDoc reads a doc file by its relative path.
	ReadDoc(name string) ([]byte, error)
}

// ZipDocReader reads docs from a zip package file.
type ZipDocReader struct {
	name string
	files map[string][]byte // docs/ relative path -> content
}

// NewZipDocReader opens a zip and extracts only the docs/ entries.
func NewZipDocReader(src string, insecure bool) (*ZipDocReader, error) {
	data, err := Fetch(src, insecure)
	if err != nil {
		return nil, err
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, ErrInvalidPackage
	}
	r := &ZipDocReader{
		name:  filepath.Base(src),
		files: make(map[string][]byte),
	}
	prefix := DocsDir + "/"
	for _, f := range zr.File {
		if f.FileInfo().IsDir() || !strings.HasPrefix(f.Name, prefix) {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, err
		}
		r.files[f.Name[len(prefix):]] = content
	}
	return r, nil
}

func (r *ZipDocReader) Name() string { return r.name }

func (r *ZipDocReader) ListDocs() []string {
	out := make([]string, 0, len(r.files))
	for k := range r.files {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func (r *ZipDocReader) ReadDoc(name string) ([]byte, error) {
	if data, ok := r.files[name]; ok {
		return data, nil
	}
	return nil, ErrModuleNotFound
}

// DirDocReader reads docs from an unpacked package directory.
type DirDocReader struct {
	root string
}

// NewDirDocReader creates a DocReader for an unpacked package directory.
func NewDirDocReader(dir string) *DirDocReader {
	return &DirDocReader{root: dir}
}

func (r *DirDocReader) Name() string { return filepath.Base(r.root) }

func (r *DirDocReader) ListDocs() []string {
	docsDir := filepath.Join(r.root, DocsDir)
	var out []string
	_ = filepath.WalkDir(docsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(docsDir, path)
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	sort.Strings(out)
	return out
}

func (r *DirDocReader) ReadDoc(name string) ([]byte, error) {
	path := filepath.Join(r.root, DocsDir, filepath.FromSlash(name))
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, ErrModuleNotFound
	}
	return data, nil
}
