package server

import (
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/paularlott/scriptling/extlibs/fssecurity"
)

// staticRoute is a directory a setup script registered with
// runtime.http.static, and the --allowed-paths set in force when it did.
type staticRoute struct {
	dir     string
	allowed []string // nil = unrestricted
}

// handler serves the route. Unrestricted routes are a plain http.FileServer,
// exactly as before. Under --allowed-paths every file is resolved the way the
// kernel will open it and served only if it lands inside the allowed paths, so
// a symlink in the directory cannot lead out; dotfiles and directory listings
// are hidden.
func (r staticRoute) handler() http.Handler {
	if r.allowed == nil {
		return http.FileServer(http.Dir(r.dir))
	}
	return http.FileServer(containedFS{
		root:  http.Dir(r.dir),
		dir:   r.dir,
		paths: fssecurity.Config{AllowedPaths: r.allowed},
	})
}

// containedFS is an http.FileSystem over a directory that refuses anything
// resolving outside the allowed paths, any path with a dot-prefixed component,
// and directory listings. A refusal is reported as not-found, so the server
// answers 404 and reveals nothing about what exists.
type containedFS struct {
	root  http.Dir
	dir   string
	paths fssecurity.Config
}

func (c containedFS) Open(name string) (http.File, error) {
	// name is a cleaned, slash-separated path rooted at "/".
	for _, part := range strings.Split(name, "/") {
		if strings.HasPrefix(part, ".") {
			return nil, fs.ErrNotExist
		}
	}
	// Check where the file really is before opening it: http.Dir follows
	// symlinks out of the tree.
	full := filepath.Join(c.dir, filepath.FromSlash(name))
	if !c.paths.IsPathAllowed(full) {
		return nil, fs.ErrNotExist
	}
	f, err := c.root.Open(name)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	if info.IsDir() {
		// http.FileServer serves a directory's index.html if it has one and
		// otherwise lists it. Hide the listing: a directory without an index
		// is not found.
		index, err := c.Open(strings.TrimSuffix(name, "/") + "/index.html")
		if err != nil {
			f.Close()
			return nil, fs.ErrNotExist
		}
		index.Close()
	}
	return f, nil
}
