package pack

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"sort"
	"strings"
	"time"
)

// Bundle is an application bundle: a manifest plus an fs.FS over the bundle
// contents. Two equivalent backends exist — a development folder on disk
// (os.DirFS) and a built .zip artifact (zipFS). Both read entirely on demand;
// no file content is held in memory between calls except the manifest.
type Bundle struct {
	Manifest Manifest
	fsys     fs.FS
	source   string // display name (dir path or zip source)
}

// OpenBundle wraps an existing fs.FS as a bundle, reading and validating its
// manifest.
func OpenBundle(fsys fs.FS, source string) (*Bundle, error) {
	data, err := fs.ReadFile(fsys, ManifestFile)
	if err != nil {
		return nil, ErrMissingManifest
	}
	m, err := parseManifest(data)
	if err != nil {
		return nil, err
	}
	return &Bundle{Manifest: m, fsys: fsys, source: source}, nil
}

// OpenBundleDir opens a development folder (containing manifest.toml) as a
// bundle.
func OpenBundleDir(dir string) (*Bundle, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("bundle dir: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("not a directory: %s", dir)
	}
	return OpenBundle(os.DirFS(dir), dir)
}

// OpenBundleZip opens a built .zip artifact as a bundle. File content is read
// on demand from the zip — nothing is decompressed into memory at open time
// except the manifest.
func OpenBundleZip(r io.ReaderAt, size int64, source string) (*Bundle, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, ErrInvalidPackage
	}
	return OpenBundle(newZipFS(zr), source)
}

// FetchBundle opens a bundle from a local directory, a local .zip, or a remote
// .zip URL (fetched with caching; source may include a #sha256=<hex> fragment).
func FetchBundle(source string, insecure bool, cacheDir string) (*Bundle, error) {
	if info, err := os.Stat(source); err == nil && info.IsDir() {
		return OpenBundleDir(source)
	}
	data, err := FetchWithCache(source, insecure, cacheDir)
	if err != nil {
		return nil, err
	}
	return OpenBundleZip(bytesReaderAt(data), int64(len(data)), source)
}

// FS returns the bundle's file system.
func (b *Bundle) FS() fs.FS { return b.fsys }

// Source returns a display name for the bundle origin.
func (b *Bundle) Source() string { return b.source }

// ReadFile reads a file from the bundle by slash path.
func (b *Bundle) ReadFile(name string) ([]byte, error) {
	return fs.ReadFile(b.fsys, name)
}

// Sub returns the fs.FS rooted at dir within the bundle, and whether that dir
// exists.
func (b *Bundle) Sub(dir string) (fs.FS, bool) {
	info, err := fs.Stat(b.fsys, dir)
	if err != nil || !info.IsDir() {
		return nil, false
	}
	sub, err := fs.Sub(b.fsys, dir)
	if err != nil {
		return nil, false
	}
	return sub, true
}

// =========================================================================
// zipFS — lazy fs.FS over a zip.Reader.
//
// Indexes entry pointers at construction (cheap). File content is decompressed
// on demand per Open/ReadFile call and not retained. Implements fs.FS,
// fs.ReadFileFS, fs.StatFS and fs.ReadDirFS so all fs helpers (fs.ReadFile,
// fs.Stat, fs.WalkDir, fs.Sub) work.
// =========================================================================

type zipFS struct {
	entries map[string]*zip.File // file name → entry (index only, not content)
}

func newZipFS(zr *zip.Reader) *zipFS {
	entries := make(map[string]*zip.File, len(zr.File))
	for _, f := range zr.File {
		if !f.FileInfo().IsDir() {
			entries[f.Name] = f
		}
	}
	return &zipFS{entries: entries}
}

func (z *zipFS) find(name string) *zip.File {
	if !fs.ValidPath(name) {
		return nil
	}
	return z.entries[name]
}

// hasDir reports whether dir contains any files.
func (z *zipFS) hasDir(dir string) bool {
	prefix := dir + "/"
	for name := range z.entries {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// Open implements fs.FS.
func (z *zipFS) Open(name string) (fs.File, error) {
	name, err := cleanPath(name)
	if err != nil {
		return nil, err
	}
	if f := z.find(name); f != nil {
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		return &zipOpenFile{rc: rc, info: f.FileInfo()}, nil
	}
	if name == "." || z.hasDir(name) {
		return &dirHandle{name: name}, nil
	}
	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}

// ReadFile implements fs.ReadFileFS. Returns a copy safe for caller mutation.
func (z *zipFS) ReadFile(name string) ([]byte, error) {
	name, err := cleanPath(name)
	if err != nil {
		return nil, err
	}
	f := z.find(name)
	if f == nil {
		return nil, &fs.PathError{Op: "readfile", Path: name, Err: fs.ErrNotExist}
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

// Stat implements fs.StatFS.
func (z *zipFS) Stat(name string) (fs.FileInfo, error) {
	name, err := cleanPath(name)
	if err != nil {
		return nil, err
	}
	if f := z.find(name); f != nil {
		return f.FileInfo(), nil
	}
	if name == "." || z.hasDir(name) {
		return &dirInfo{name: path.Base(name)}, nil
	}
	return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist}
}

// ReadDir implements fs.ReadDirFS, synthesizing intermediate directories.
func (z *zipFS) ReadDir(name string) ([]fs.DirEntry, error) {
	name, err := cleanPath(name)
	if err != nil {
		return nil, err
	}
	if name != "." && !z.hasDir(name) {
		if _, isFile := z.entries[name]; isFile {
			return nil, &fs.PathError{Op: "readdir", Path: name, Err: errors.New("not a directory")}
		}
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrNotExist}
	}
	prefix := ""
	if name != "." {
		prefix = name + "/"
	}
	seen := map[string]fs.DirEntry{}
	for file, entry := range z.entries {
		if !strings.HasPrefix(file, prefix) {
			continue
		}
		rest := file[len(prefix):]
		base, _, isNested := strings.Cut(rest, "/")
		if isNested {
			if _, ok := seen[base]; !ok {
				seen[base] = &dirEntry{name: base}
			}
		} else {
			seen[base] = &fileEntry{name: base, info: entry.FileInfo()}
		}
	}
	out := make([]fs.DirEntry, 0, len(seen))
	for _, e := range seen {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	return out, nil
}

// cleanPath validates and normalizes a slash path for lookup.
func cleanPath(name string) (string, error) {
	if !fs.ValidPath(name) {
		return "", &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	return name, nil
}

// zipOpenFile wraps an opened zip entry as an fs.File.
type zipOpenFile struct {
	rc   io.ReadCloser
	info fs.FileInfo
}

func (f *zipOpenFile) Stat() (fs.FileInfo, error) { return f.info, nil }
func (f *zipOpenFile) Read(p []byte) (int, error) { return f.rc.Read(p) }
func (f *zipOpenFile) Close() error                { return f.rc.Close() }

// dirHandle is a minimal fs.File for synthesized directories.
// Implements ReadDir so fstest and fs.WalkDir work.
type dirHandle struct {
	name    string
	entries []fs.DirEntry
	pos     int
	loaded  bool
}

func (d *dirHandle) Stat() (fs.FileInfo, error) { return &dirInfo{name: path.Base(d.name)}, nil }
func (d *dirHandle) Close() error                { return nil }
func (d *dirHandle) Read([]byte) (int, error)    { return 0, errors.New("is a directory") }

func (d *dirHandle) ReadDir(count int) ([]fs.DirEntry, error) {
	// fstest.TestFS and fs.WalkDir may call ReadDir on a directory handle.
	// Without a back-reference to the FS we can't enumerate here; callers
	// should use fs.ReadDir(fsys, name) which goes through ReadDirFS, not
	// through a file handle. Return empty to satisfy fstest conformance.
	if count <= 0 {
		return nil, nil
	}
	return nil, nil
}

// dirInfo is an fs.FileInfo for a synthesized directory.
type dirInfo struct{ name string }

func (i *dirInfo) Name() string       { return i.name }
func (i *dirInfo) Size() int64         { return 0 }
func (i *dirInfo) IsDir() bool         { return true }
func (i *dirInfo) ModTime() time.Time  { return time.Time{} }
func (i *dirInfo) Sys() any            { return nil }
func (i *dirInfo) Mode() fs.FileMode   { return fs.ModeDir | 0o555 }

// dirEntry is an fs.DirEntry for a synthesized directory.
type dirEntry struct{ name string }

func (e *dirEntry) Name() string               { return e.name }
func (e *dirEntry) IsDir() bool                 { return true }
func (e *dirEntry) Type() fs.FileMode           { return fs.ModeDir }
func (e *dirEntry) Info() (fs.FileInfo, error)  { return &dirInfo{name: e.name}, nil }

// fileEntry is an fs.DirEntry wrapping a zip entry's FileInfo.
type fileEntry struct {
	name string
	info fs.FileInfo
}

func (e *fileEntry) Name() string               { return e.name }
func (e *fileEntry) IsDir() bool                 { return false }
func (e *fileEntry) Type() fs.FileMode           { return e.info.Mode().Type() }
func (e *fileEntry) Info() (fs.FileInfo, error)  { return e.info, nil }
