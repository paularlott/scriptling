package pluginpack

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/paularlott/scriptling/plugin"
)

// pluginFS is an fs.FS over one fetcher plugin client for one source, the
// RPC-backed sibling of pack's dir and zip backends. Implements fs.FS,
// fs.ReadFileFS, fs.StatFS and fs.ReadDirFS so all fs helpers (fs.ReadFile,
// fs.Stat, fs.WalkDir, fs.Sub) work, mirroring pack's zipFS.
//
// File content is never retained — every read is a fetch. Directory listings
// are the one exception: they are memoized for dirTTL, because resolving a
// single path consults its parent's listing, so a WalkDir over an app bundle
// would otherwise re-list the same directory once per entry. Structure is
// cheap to hold and bounded; content is not held at all.
//
// ctx is the host's context: cancelling it aborts in-flight fetches. fs.FS has
// no per-call context, so it is captured once here.
type pluginFS struct {
	ctx    context.Context
	client *plugin.Client
	source string
	dirTTL time.Duration

	mu     sync.Mutex
	dirs   map[string]dirCacheEntry  // fetch.glob listings, keyed by directory
	exists map[string]dirExistsEntry // exact-path probes: does this directory exist
}

// dirCacheEntry is a directory listing with the time it was fetched.
type dirCacheEntry struct {
	entries []fs.DirEntry
	fetched time.Time
}

// dirExistsEntry caches an exact-path directory probe (a wildcard-free
// fetch.glob answering the directory entry itself, so empty directories are
// distinguishable from missing ones).
type dirExistsEntry struct {
	ok      bool
	fetched time.Time
}

func newPluginFS(ctx context.Context, client *plugin.Client, source string, dirTTL time.Duration) *pluginFS {
	if ctx == nil {
		ctx = context.Background()
	}
	return &pluginFS{
		ctx:    ctx,
		client: client,
		source: source,
		dirTTL: dirTTL,
		dirs:   map[string]dirCacheEntry{},
		exists: map[string]dirExistsEntry{},
	}
}

// ReadFile implements fs.ReadFileFS. Returns a copy safe for caller mutation.
func (p *pluginFS) ReadFile(name string) ([]byte, error) {
	name, err := cleanPath(name)
	if err != nil {
		return nil, err
	}
	return p.readFile(name)
}

// Open implements fs.FS. A directory is one whose listing succeeds; anything
// else is read as a file — there is no stat round trip, we ask for the file
// and get it.
func (p *pluginFS) Open(name string) (fs.File, error) {
	name, err := cleanPath(name)
	if err != nil {
		return nil, err
	}
	if name == "." || p.isDir(name) {
		entries, err := p.ReadDir(name)
		if err != nil {
			return nil, err
		}
		return &dirHandle{name: name, entries: entries}, nil
	}
	data, err := p.readFile(name)
	if err != nil {
		return nil, err
	}
	info := &fileInfo{name: path.Base(name), size: int64(len(data))}
	return &openFile{reader: bytes.NewReader(data), info: info}, nil
}

// Stat implements fs.StatFS. A directory is one whose listing succeeds;
// anything else is read as a file — stat-ing means asking for it.
func (p *pluginFS) Stat(name string) (fs.FileInfo, error) {
	name, err := cleanPath(name)
	if err != nil {
		return nil, err
	}
	if name == "." || p.isDir(name) {
		return &fileInfo{name: path.Base(name), size: 0, dir: true}, nil
	}
	data, err := p.readFile(name)
	if err != nil {
		return nil, err
	}
	return &fileInfo{name: path.Base(name), size: int64(len(data))}, nil
}

// isDir reports whether name is a directory of the source: an exact-path
// fetch.glob that answers the directory entry itself (so an empty directory
// still resolves, where a listing of nothing would not).
func (p *pluginFS) isDir(name string) bool {
	if name == "." {
		return true
	}
	if _, ok := p.cachedDir(name); ok {
		return true
	}
	if ok, cached := p.cachedExists(name); cached {
		return ok
	}
	entries, err := p.client.FetchGlob(p.ctx, p.source, name)
	if err != nil {
		// An error is not an answer: an unavailable backend must not cache
		// a real directory as missing for the whole TTL. Fail the probe
		// uncached so the next call asks again.
		return false
	}
	is := false
	for _, entry := range entries {
		if entry.Name == name && entry.IsDir {
			is = true
		}
	}
	p.mu.Lock()
	p.exists[name] = dirExistsEntry{ok: is, fetched: time.Now()}
	p.mu.Unlock()
	return is
}

// cachedExists returns a directory probe still within dirTTL.
func (p *pluginFS) cachedExists(name string) (bool, bool) {
	if p.dirTTL < 0 {
		return false, false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	entry, ok := p.exists[name]
	if !ok {
		return false, false
	}
	if time.Since(entry.fetched) > p.dirTTL {
		delete(p.exists, name)
		return false, false
	}
	return entry.ok, true
}

// ReadDir implements fs.ReadDirFS. Listings are reused for dirTTL; a negative
// dirTTL disables reuse so every call is a fresh fetch.glob.
func (p *pluginFS) ReadDir(name string) ([]fs.DirEntry, error) {
	name, err := cleanPath(name)
	if err != nil {
		return nil, err
	}
	if entries, ok := p.cachedDir(name); ok {
		return entries, nil
	}
	if !p.isDir(name) {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrNotExist}
	}

	pattern := name + "/*"
	if name == "." {
		pattern = "*"
	}
	listed, err := p.client.FetchGlob(p.ctx, p.source, pattern)
	if err != nil {
		if errors.Is(err, plugin.ErrFetchNotFound) {
			return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrNotExist}
		}
		if errors.Is(err, plugin.ErrFetchDenied) {
			return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrPermission}
		}
		return nil, err
	}
	prefix := ""
	if name != "." {
		prefix = name + "/"
	}
	out := make([]fs.DirEntry, 0, len(listed))
	for _, e := range listed {
		if !strings.HasPrefix(e.Name, prefix) || strings.Contains(strings.TrimPrefix(e.Name, prefix), "/") {
			continue // the pattern asked for one level; keep exactly that
		}
		out = append(out, &dirEntry{name: strings.TrimPrefix(e.Name, prefix), dir: e.IsDir})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })

	p.mu.Lock()
	p.dirs[name] = dirCacheEntry{entries: out, fetched: time.Now()}
	p.mu.Unlock()
	return out, nil
}

// cachedDir returns a listing that is still within dirTTL.
func (p *pluginFS) cachedDir(name string) ([]fs.DirEntry, bool) {
	if p.dirTTL < 0 {
		return nil, false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	entry, ok := p.dirs[name]
	if !ok {
		return nil, false
	}
	if time.Since(entry.fetched) > p.dirTTL {
		delete(p.dirs, name)
		return nil, false
	}
	return entry.entries, true
}

// cleanPattern normalizes the cosmetic spellings fs.ValidPath rejects (a
// leading "./", interior "." segments) while keeping the safety property:
// ".." anywhere stays an error. fs.Glob callers reasonably write "./lib/*".
func cleanPattern(pattern string) (string, error) {
	if fs.ValidPath(pattern) {
		return pattern, nil
	}
	parts := strings.Split(pattern, "/")
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		switch part {
		case "", ".":
			continue
		case "..":
			return "", &fs.PathError{Op: "glob", Path: pattern, Err: fs.ErrInvalid}
		default:
			cleaned = append(cleaned, part)
		}
	}
	return strings.Join(cleaned, "/"), nil
}

// Glob implements fs.GlobFS: one fetch.glob round trip for any pattern,
// instead of the per-level walk fs.Glob would otherwise perform. Matches are
// full paths relative to the source, which is exactly what fs.Glob returns.
func (p *pluginFS) Glob(pattern string) ([]string, error) {
	pattern, err := cleanPattern(pattern)
	if err != nil {
		return nil, err
	}
	entries, err := p.client.FetchGlob(p.ctx, p.source, pattern)
	if err != nil {
		if errors.Is(err, plugin.ErrFetchNotFound) {
			return nil, &fs.PathError{Op: "glob", Path: pattern, Err: fs.ErrNotExist}
		}
		if errors.Is(err, plugin.ErrFetchDenied) {
			return nil, &fs.PathError{Op: "glob", Path: pattern, Err: fs.ErrPermission}
		}
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name)
	}
	sort.Strings(names)
	return names, nil
}

// readFile fetches file content from the plugin. Nothing is cached: the host
// holds no plugin-served bytes, on disk or in memory, so every read is a fetch.
//
// Caching content here would buy very little. Server modes build a fresh
// interpreter per request, so a handler module is re-read on every request
// whatever we do, and the files are source modules measured in hundreds of
// bytes. The parse cache upstream is keyed on the source text, so it already
// removes the expensive part: compiling. What is left is a small, predictable
// RPC, with always-fresh semantics and no invalidation logic to get wrong.
//
// A plugin whose backend is genuinely slow caches behind its own fetcher, where
// the credentials and freshness rules live.
func (p *pluginFS) readFile(name string) ([]byte, error) {
	return fetchFile(p.ctx, p.client, p.source, name)
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
