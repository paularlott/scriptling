package server

import (
	"archive/zip"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
)

// webRootCandidates returns the names a request path may be served from in a
// web root: the file itself, then the index.html of a directory by that name.
// Every candidate is a valid fs.FS name (fs.ValidPath), so ".." and absolute
// paths are never looked up, whichever web root serves the request.
func webRootCandidates(urlPath string) []string {
	name := path.Clean(strings.TrimPrefix(urlPath, "/"))
	if name == "." {
		return []string{"index.html"}
	}
	if !fs.ValidPath(name) {
		return nil
	}
	return []string{name, name + "/index.html"}
}

// webRootBufferedMax is the largest web root file read whole before it is
// written: one exact-size buffer is cheaper than a copy loop for the small
// assets most requests are for. Larger files stream.
const webRootBufferedMax = 64 << 10

// serveWebRoot serves a request from a web root: the app bundle's webroot/,
// a --web-root directory or a --web-root zip. The first candidate that is a
// file is written with its content type; otherwise the not-found handler
// runs. what names the web root in trace logs.
func (s *Server) serveWebRoot(w http.ResponseWriter, r *http.Request, fsys fs.FS, what string) {
	for _, candidate := range webRootCandidates(r.URL.Path) {
		f, err := fsys.Open(candidate)
		if err != nil {
			continue
		}
		info, err := f.Stat()
		if err != nil || info.IsDir() {
			f.Close()
			continue
		}
		var data []byte
		if size := info.Size(); size >= 0 && size <= webRootBufferedMax {
			data = make([]byte, size)
			if _, err := io.ReadFull(f, data); err != nil {
				f.Close()
				continue
			}
		}
		Log.Trace("Serving file from "+what, "file", candidate)
		if ct := mime.TypeByExtension(path.Ext(candidate)); ct != "" {
			w.Header().Set("Content-Type", ct)
		}
		if data != nil {
			w.Write(data)
		} else {
			io.Copy(w, f)
		}
		f.Close()
		return
	}
	Log.Trace(what+" entry not found", "path", r.URL.Path)
	s.serveNotFound(w, r)
}

// zipWebRoot is an fs.FS over the files of a --web-root zip, indexed by name
// when the archive is opened so a request is one map lookup. Member names are
// arbitrary strings; only those that are valid fs.FS names are indexed, so a
// member named "/etc/passwd" or "../x" can never be served. Directories are
// not indexed: a request for one falls through to its index.html.
type zipWebRoot struct {
	files map[string]*zip.File
}

// newZipWebRoot indexes the archive's files. It returns the names of members
// it left out because they are not valid relative paths, for the caller to
// report. When a name appears twice the first member wins, as it always has.
func newZipWebRoot(r *zip.Reader) (zipWebRoot, []string) {
	files := make(map[string]*zip.File, len(r.File))
	var skipped []string
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if !fs.ValidPath(f.Name) {
			skipped = append(skipped, f.Name)
			continue
		}
		if _, dup := files[f.Name]; !dup {
			files[f.Name] = f
		}
	}
	return zipWebRoot{files: files}, skipped
}

func (z zipWebRoot) Open(name string) (fs.File, error) {
	f, ok := z.files[name]
	if !ok {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	rc, err := f.Open()
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: err}
	}
	return zipWebRootFile{ReadCloser: rc, info: f.FileInfo()}, nil
}

// zipWebRootFile is an open zip member.
type zipWebRootFile struct {
	io.ReadCloser
	info fs.FileInfo
}

func (f zipWebRootFile) Stat() (fs.FileInfo, error) { return f.info, nil }
