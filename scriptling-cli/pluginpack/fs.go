package pluginpack

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/paularlott/scriptling/plugin"
)

// pluginFS is an fs.FS over one fetcher plugin client for one source, the
// RPC-backed sibling of pack's dir and zip backends. Every ReadFile is a
// fetch.read round trip guarded by the per-file disk cache; ReadDir results
// are memoized for the file system's lifetime. Implements fs.FS,
// fs.ReadFileFS, fs.StatFS and fs.ReadDirFS so all fs helpers (fs.ReadFile,
// fs.Stat, fs.WalkDir, fs.Sub) work, mirroring pack's zipFS.
type pluginFS struct {
	client   *plugin.Client
	source   string
	cacheDir string

	mu   sync.Mutex
	dirs map[string][]fs.DirEntry // memoized fetch.list results ("." → root)
}

func newPluginFS(client *plugin.Client, source, cacheDir string) *pluginFS {
	return &pluginFS{
		client:   client,
		source:   source,
		cacheDir: cacheDir,
		dirs:     map[string][]fs.DirEntry{},
	}
}

// ReadFile implements fs.ReadFileFS. Returns a copy safe for caller mutation.
func (p *pluginFS) ReadFile(name string) ([]byte, error) {
	name, err := cleanPath(name)
	if err != nil {
		return nil, err
	}
	return p.cachedRead(name)
}

// Open implements fs.FS.
func (p *pluginFS) Open(name string) (fs.File, error) {
	name, err := cleanPath(name)
	if err != nil {
		return nil, err
	}
	data, isDir, err := p.statOrRead(name)
	if err != nil {
		return nil, err
	}
	if isDir {
		entries, err := p.ReadDir(name)
		if err != nil {
			return nil, err
		}
		return &dirHandle{name: name, entries: entries}, nil
	}
	info := &fileInfo{name: path.Base(name), size: int64(len(data))}
	return &openFile{reader: bytes.NewReader(data), info: info}, nil
}

// Stat implements fs.StatFS.
func (p *pluginFS) Stat(name string) (fs.FileInfo, error) {
	name, err := cleanPath(name)
	if err != nil {
		return nil, err
	}
	data, isDir, err := p.statOrRead(name)
	if err != nil {
		return nil, err
	}
	if isDir {
		return &fileInfo{name: path.Base(name), size: 0, dir: true}, nil
	}
	return &fileInfo{name: path.Base(name), size: int64(len(data))}, nil
}

// ReadDir implements fs.ReadDirFS.
func (p *pluginFS) ReadDir(name string) ([]fs.DirEntry, error) {
	name, err := cleanPath(name)
	if err != nil {
		return nil, err
	}
	p.mu.Lock()
	if entries, ok := p.dirs[name]; ok {
		p.mu.Unlock()
		return entries, nil
	}
	p.mu.Unlock()

	entries, err := p.client.FetchList(context.Background(), p.source, name)
	if err != nil {
		if errors.Is(err, plugin.ErrFetchNotFound) {
			return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrNotExist}
		}
		return nil, err
	}
	out := make([]fs.DirEntry, len(entries))
	for i, e := range entries {
		out[i] = &dirEntry{name: e.Name, dir: e.IsDir}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })

	p.mu.Lock()
	p.dirs[name] = out
	p.mu.Unlock()
	return out, nil
}

// statOrRead resolves name to either its content (data != nil, isDir false),
// a directory (data == nil, isDir true) or an error. It consults the
// memoized directory listings first so stats and globs do not fetch content.
func (p *pluginFS) statOrRead(name string) (data []byte, isDir bool, err error) {
	if name == "." {
		return nil, true, nil
	}
	if entry, ok := p.lookupEntry(name); ok {
		if entry.dir {
			return nil, true, nil
		}
		content, err := p.cachedRead(name)
		if err != nil {
			return nil, false, err
		}
		return content, false, nil
	}
	// Unknown to any parent listing — but the fetcher may serve files its
	// listings omit (some do for probe-heavy paths), so confirm with a read
	// before declaring the path missing.
	content, _, err := p.cachedReadResult(name)
	if err != nil {
		return nil, false, err
	}
	return content, false, nil
}

// lookupEntry reports the memoized listing entry for name's base under its
// parent directory, populating the parent's listing if needed.
func (p *pluginFS) lookupEntry(name string) (*dirEntry, bool) {
	parent := path.Dir(name)
	if parent == name { // "."
		return nil, false
	}
	entries, err := p.ReadDir(parent)
	if err != nil {
		return nil, false
	}
	base := path.Base(name)
	for _, e := range entries {
		if entry, ok := e.(*dirEntry); ok && entry.name == base {
			return entry, true
		}
	}
	return nil, false
}

// cachedRead returns file content through the conditional disk cache.
func (p *pluginFS) cachedRead(name string) ([]byte, error) {
	data, _, err := p.cachedReadResult(name)
	return data, err
}

// cachedReadResult is cachedRead but also reports the validators observed
// with the returned bytes (used by tests to assert revalidation).
func (p *pluginFS) cachedReadResult(name string) ([]byte, plugin.FetchResult, error) {
	cacheDir, skip := resolveCacheDir(p.cacheDir)
	if skip {
		// No usable cache dir: straight fetch, no validators.
		data, res, err := fetchFile(p.client, p.source, name, "", "")
		return data, res, err
	}
	key := fetchCacheKey(p.source, name)
	dataFile := filepath.Join(cacheDir, key+".pfile")
	metaFile := filepath.Join(cacheDir, key+".meta")

	etag, lastMod := readCacheMeta(metaFile)
	if _, statErr := os.Stat(dataFile); statErr == nil {
		data, res, err := fetchFile(p.client, p.source, name, etag, lastMod)
		if err != nil {
			return nil, res, err
		}
		if res.NotModified {
			now := time.Now()
			_ = os.Chtimes(dataFile, now, now)
			cached, readErr := os.ReadFile(dataFile)
			if readErr != nil {
				return nil, res, readErr
			}
			return cached, res, nil
		}
		writeCachePair(dataFile, metaFile, data, res)
		return data, res, nil
	}

	data, res, err := fetchFile(p.client, p.source, name, "", "")
	if err != nil {
		return nil, res, err
	}
	writeCachePair(dataFile, metaFile, data, res)
	return data, res, nil
}

// =========================================================================
// Cache layout — same directory and .meta format as pack's URL cache, with
// .pfile data files keyed by source+path.
// =========================================================================

// resolveCacheDir returns the cache directory to use, or skip=true when no
// cache directory is available (fall back to uncached fetches).
func resolveCacheDir(cacheDir string) (string, bool) {
	if cacheDir == "" {
		var err error
		cacheDir, err = defaultCacheDir()
		if err != nil {
			return "", true
		}
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", true
	}
	return cacheDir, false
}

// defaultCacheDir mirrors pack.DefaultCacheDir without the import cycle
// (pack does not export it; the layout is shared by convention).
func defaultCacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine cache directory: %w", err)
	}
	return filepath.Join(base, "scriptling", "packages"), nil
}

// fetchCacheKey returns a stable filename-safe key for a source+path pair.
func fetchCacheKey(source, path string) string {
	h := sha256.Sum256([]byte(source + "\x00" + path))
	return hex.EncodeToString(h[:])
}

// readCacheMeta reads the etag\nlast-modified validators from a meta file.
func readCacheMeta(path string) (etag, lastMod string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", ""
	}
	parts := strings.SplitN(string(data), "\n", 2)
	if len(parts) == 2 {
		return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	}
	return strings.TrimSpace(string(data)), ""
}

func writeCachePair(dataFile, metaFile string, data []byte, res plugin.FetchResult) {
	_ = os.WriteFile(dataFile, data, 0o644)
	if res.ETag != "" || res.LastModified != "" {
		_ = os.WriteFile(metaFile, []byte(res.ETag+"\n"+res.LastModified), 0o644)
	}
}

// =========================================================================
// Minimal fs plumbing (mirrors pack's zipFS helpers).
// =========================================================================

func cleanPath(name string) (string, error) {
	if !fs.ValidPath(name) {
		return "", &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	return name, nil
}

type fileInfo struct {
	name string
	size int64
	dir  bool
}

func (i *fileInfo) Name() string       { return i.name }
func (i *fileInfo) Size() int64        { return i.size }
func (i *fileInfo) ModTime() time.Time { return time.Time{} }
func (i *fileInfo) IsDir() bool        { return i.dir }
func (i *fileInfo) Sys() any           { return nil }
func (i *fileInfo) Mode() fs.FileMode {
	if i.dir {
		return fs.ModeDir | 0o555
	}
	return 0o444
}

type dirEntry struct {
	name string
	dir  bool
}

func (e *dirEntry) Name() string { return e.name }
func (e *dirEntry) IsDir() bool  { return e.dir }

func (e *dirEntry) Type() fs.FileMode {
	if e.dir {
		return fs.ModeDir
	}
	return 0
}

func (e *dirEntry) Info() (fs.FileInfo, error) { return &fileInfo{name: e.name, dir: e.dir}, nil }

type openFile struct {
	reader *bytes.Reader
	info   fs.FileInfo
}

func (f *openFile) Stat() (fs.FileInfo, error) { return f.info, nil }
func (f *openFile) Read(p []byte) (int, error) { return f.reader.Read(p) }
func (f *openFile) Close() error               { return nil }

type dirHandle struct {
	name    string
	entries []fs.DirEntry
	pos     int
}

func (d *dirHandle) Stat() (fs.FileInfo, error) {
	return &fileInfo{name: path.Base(d.name), dir: true}, nil
}
func (d *dirHandle) Close() error             { return nil }
func (d *dirHandle) Read([]byte) (int, error) { return 0, errors.New("is a directory") }

func (d *dirHandle) ReadDir(count int) ([]fs.DirEntry, error) {
	if d.pos >= len(d.entries) {
		if count <= 0 {
			return nil, nil
		}
		return nil, io.EOF
	}
	end := d.pos + count
	if end > len(d.entries) || count <= 0 {
		end = len(d.entries)
	}
	out := d.entries[d.pos:end]
	d.pos = end
	return out, nil
}
