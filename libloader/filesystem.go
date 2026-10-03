package libloader

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// FilesystemLoader loads libraries from the filesystem.
// It supports Python-style folder structure for nested modules:
//   - libs/knot/groups.py → import knot.groups (preferred)
//   - libs/knot.groups.py → import knot.groups (legacy fallback)
//
// The loader checks the folder structure first, then falls back to flat files.
type FilesystemLoader struct {
	baseDir     string
	extension   string
	followLinks bool
	description string
}

// FilesystemOption configures a FilesystemLoader.
type FilesystemOption func(*FilesystemLoader)

// WithExtension sets a custom file extension (default: ".py").
func WithExtension(ext string) FilesystemOption {
	return func(l *FilesystemLoader) {
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		l.extension = ext
	}
}

// WithFollowLinks enables or disables following symbolic links (default: true).
func WithFollowLinks(follow bool) FilesystemOption {
	return func(l *FilesystemLoader) {
		l.followLinks = follow
	}
}

// WithDescription sets a custom description for the loader.
func WithDescription(desc string) FilesystemOption {
	return func(l *FilesystemLoader) {
		l.description = desc
	}
}

// NewFilesystem creates a new filesystem loader.
// The baseDir is the root directory to search for libraries.
//
// Example:
//
//	loader := NewFilesystem("/app/libs")
//	// Will load:
//	//   import json          -> /app/libs/json.py
//	//   import knot.groups   -> /app/libs/knot/groups.py (preferred)
//	//                          -> /app/libs/knot.groups.py (fallback)
func NewFilesystem(baseDir string, opts ...FilesystemOption) *FilesystemLoader {
	l := &FilesystemLoader{
		baseDir:     baseDir,
		extension:   ".py",
		followLinks: true,
		description: fmt.Sprintf("filesystem:%s", baseDir),
	}

	for _, opt := range opts {
		opt(l)
	}

	return l
}

// Load attempts to load a library from the filesystem.
// It tries the folder structure first, then falls back to flat files.
//
// For a library name "knot.groups", it checks:
//  1. baseDir/knot/groups.py (folder structure - Python style)
//  2. baseDir/knot.groups.py (flat structure - legacy support)
//
// Returns (source, true, nil) if found, ("", false, nil) if not found,
// or ("", false, error) on filesystem errors.
func (l *FilesystemLoader) Load(name string) (string, bool, error) {
	return l.LoadWithContext(context.Background(), name)
}

// LoadWithContext checks caller cancellation between filesystem operations.
func (l *FilesystemLoader) LoadWithContext(ctx context.Context, name string) (string, bool, error) {
	// Convert dotted name to possible file paths
	paths := l.resolvePaths(name)

	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return "", false, err
		}

		// One stat answers the common cases: a missing candidate, or a file
		// unchanged since it was last read. Anything else (errors other than
		// not-exist, changed files) takes the full readFile path.
		info, statErr := os.Stat(path)
		if statErr == nil {
			if info.IsDir() {
				continue
			}
			if content, ok := cachedSource(path, info); ok {
				return content, true, nil
			}
		} else if os.IsNotExist(statErr) {
			continue
		}

		if statErr == nil {
			// A regular file we have already stat'ed: read it directly.
			data, err := os.ReadFile(path)
			if err != nil {
				return "", false, fmt.Errorf("failed to read %s: %w", path, err)
			}
			content := string(data)
			storeSource(path, info, content)
			return content, true, nil
		}

		// An unusual stat error (permissions, a file in place of a
		// directory): readFile classifies it exactly as before.
		content, found, err := l.readFile(path)
		if err != nil {
			return "", false, err
		}
		if found {
			return content, true, nil
		}
	}

	return "", false, nil
}

// sourceCache holds library sources read from disk, shared by every
// FilesystemLoader in the process so that new interpreter instances (which
// often build their own loader) skip re-reading unchanged files. Keyed by the
// candidate path; each hit is revalidated with a single stat. Its total size
// is capped: once full, further files are simply read from disk each time.
var sourceCache = struct {
	sync.RWMutex
	files    map[string]*cachedFile
	bytes    int
	maxBytes int
}{files: make(map[string]*cachedFile), maxBytes: DefaultSourceCacheMaxBytes}

// DefaultSourceCacheMaxBytes is the default limit on library source held by
// the filesystem loaders' shared cache: 32 MiB.
const DefaultSourceCacheMaxBytes = 32 << 20

type cachedFile struct {
	info    os.FileInfo
	content string
}

// racyWindow is how recently a file may have been modified and still be
// cached. Within it, a rewrite could keep the same size and mtime on
// filesystems with coarse timestamps, so such files are always re-read
// (the same rule git uses for its index).
const racyWindow = 2 * time.Second

func cachedSource(path string, info os.FileInfo) (string, bool) {
	sourceCache.RLock()
	c, ok := sourceCache.files[path]
	sourceCache.RUnlock()
	if !ok {
		return "", false
	}
	if os.SameFile(c.info, info) && c.info.Size() == info.Size() && c.info.ModTime().Equal(info.ModTime()) {
		return c.content, true
	}
	sourceCache.Lock()
	if sourceCache.files[path] == c {
		delete(sourceCache.files, path)
		sourceCache.bytes -= len(c.content)
	}
	sourceCache.Unlock()
	return "", false
}

// storeSource caches content read for path. info must come from a stat taken
// before the read, so a concurrent rewrite leaves the entry looking stale
// rather than fresh.
func storeSource(path string, info os.FileInfo, content string) {
	if time.Since(info.ModTime()) < racyWindow {
		return
	}
	sourceCache.Lock()
	defer sourceCache.Unlock()
	old := 0
	if prev, ok := sourceCache.files[path]; ok {
		old = len(prev.content)
	}
	if sourceCache.maxBytes > 0 && sourceCache.bytes-old+len(content) > sourceCache.maxBytes {
		return
	}
	sourceCache.files[path] = &cachedFile{info: info, content: content}
	sourceCache.bytes += len(content) - old
}

// ClearSourceCache drops every cached library source, forcing the next load
// of each file to read it from disk.
func ClearSourceCache() {
	sourceCache.Lock()
	sourceCache.files = make(map[string]*cachedFile)
	sourceCache.bytes = 0
	sourceCache.Unlock()
}

// SetSourceCacheMaxBytes limits the total library source the shared cache
// holds (default DefaultSourceCacheMaxBytes). 0 removes the limit. Lowering
// it clears the cache so the new limit holds at once.
func SetSourceCacheMaxBytes(maxBytes int) {
	if maxBytes < 0 {
		maxBytes = 0
	}
	sourceCache.Lock()
	sourceCache.maxBytes = maxBytes
	if maxBytes > 0 && sourceCache.bytes > maxBytes {
		sourceCache.files = make(map[string]*cachedFile)
		sourceCache.bytes = 0
	}
	sourceCache.Unlock()
}

// resolvePaths returns the possible file paths for a library name.
// Priority: folder structure first, then flat file.
func (l *FilesystemLoader) resolvePaths(name string) []string {
	parts := strings.Split(name, ".")

	var paths []string

	// Priority 1: Folder structure (Python style)
	// knot.groups -> baseDir/knot/groups.py
	// knot.groups.sub -> baseDir/knot/groups/sub.py
	folderPath := filepath.Join(l.baseDir, filepath.Join(parts...)+l.extension)
	paths = append(paths, folderPath)

	// Priority 2: Package __init__.py (for single-part names)
	// telegram -> baseDir/telegram/__init__.py
	// This allows importing a package that has submodules
	initPath := filepath.Join(l.baseDir, filepath.Join(parts...), "__init__.py")
	if initPath != folderPath {
		paths = append(paths, initPath)
	}

	// Priority 3: Flat file (legacy support)
	// knot.groups -> baseDir/knot.groups.py
	if len(parts) > 1 {
		flatPath := filepath.Join(l.baseDir, name+l.extension)
		if flatPath != folderPath && flatPath != initPath {
			paths = append(paths, flatPath)
		}
	}

	return paths
}

// readFile reads a file and returns its content.
func (l *FilesystemLoader) readFile(path string) (string, bool, error) {
	// Resolve symbolic links if enabled
	if l.followLinks {
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			if os.IsNotExist(err) {
				return "", false, nil
			}
			// A path component exists but is not a directory (e.g. a file
			// named "scriptling" blocking resolution of scriptling/runtime/kv.py)
			if strings.Contains(err.Error(), "not a directory") {
				return "", false, nil
			}
			return "", false, fmt.Errorf("failed to resolve symlink %s: %w", path, err)
		}
		path = resolved
	}

	// Check if file exists
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("failed to stat %s: %w", path, err)
	}

	// Make sure it's a regular file (not a directory)
	if info.IsDir() {
		return "", false, nil
	}

	// Read the file
	content, err := os.ReadFile(path)
	if err != nil {
		return "", false, fmt.Errorf("failed to read %s: %w", path, err)
	}

	return string(content), true, nil
}

// Description returns a description of this loader.
func (l *FilesystemLoader) Description() string {
	return l.description
}

// BaseDir returns the base directory this loader searches.
func (l *FilesystemLoader) BaseDir() string {
	return l.baseDir
}

// Extension returns the file extension being used.
func (l *FilesystemLoader) Extension() string {
	return l.extension
}

// MultiFilesystemLoader loads from multiple directories in order.
// Useful for having a user library directory that overrides system libraries.
type MultiFilesystemLoader struct {
	loaders []*FilesystemLoader
	desc    string
}

// NewMultiFilesystem creates a loader that searches multiple directories.
// Directories are searched in the order provided.
//
// Example:
//
//	loader := NewMultiFilesystem("/app/user/libs", "/app/system/libs")
//	// Will check user libs first, then fall back to system libs
func NewMultiFilesystem(dirs ...string) *MultiFilesystemLoader {
	loaders := make([]*FilesystemLoader, len(dirs))
	for i, dir := range dirs {
		loaders[i] = NewFilesystem(dir)
	}

	return &MultiFilesystemLoader{
		loaders: loaders,
		desc:    fmt.Sprintf("multi-filesystem: %v", dirs),
	}
}

// Load tries each directory in order until the library is found.
func (m *MultiFilesystemLoader) Load(name string) (string, bool, error) {
	return m.LoadWithContext(context.Background(), name)
}

// LoadWithContext preserves caller context across each filesystem loader.
func (m *MultiFilesystemLoader) LoadWithContext(ctx context.Context, name string) (string, bool, error) {
	for _, loader := range m.loaders {
		source, found, err := loader.LoadWithContext(ctx, name)
		if err != nil {
			return "", false, err
		}
		if found {
			return source, true, nil
		}
	}
	return "", false, nil
}

// Description returns a description of this loader.
func (m *MultiFilesystemLoader) Description() string {
	return m.desc
}

// AddDir adds another directory to search (appended to the end).
func (m *MultiFilesystemLoader) AddDir(dir string) {
	m.loaders = append(m.loaders, NewFilesystem(dir))
}
