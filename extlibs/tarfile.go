// Package extlibs provides external libraries that need explicit registration
package extlibs

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/extlibs/fssecurity"
	"github.com/paularlott/scriptling/object"
)

// tarfileLibraryInstance holds the configured tarfile library instance.
type tarfileLibraryInstance struct {
	config   fssecurity.Config
	TarClass *object.Class
}

func RegisterTarfileLibrary(registrar object.LibraryRegistrar, allowedPaths []string) {
	registrar.RegisterLibrary(NewTarfileLibrary(fssecurity.Config{AllowedPaths: allowedPaths}))
}

func NewTarfileLibrary(config fssecurity.Config) *object.Library {
	if config.AllowedPaths != nil {
		normalized := make([]string, 0, len(config.AllowedPaths))
		for _, p := range config.AllowedPaths {
			abs, err := filepath.Abs(p)
			if err != nil {
				continue
			}
			normalized = append(normalized, filepath.Clean(abs))
		}
		config.AllowedPaths = normalized
	}
	inst := &tarfileLibraryInstance{config: config}
	return inst.createLibrary()
}

// tarFileNativeData holds the open archive state for a TarFile instance.
type tarFileNativeData struct {
	reader      *tar.Reader
	closer      io.Closer // underlying file (and optional gzip reader) to close
	writer      *tar.Writer
	zipWriter   *gzip.Writer // non-nil when writing gzipped tar
	file        *os.File
	headerIndex map[string]*tar.Header
	headers     []*tar.Header
	path        string
	mode        string
	closed      bool
}

func (t *tarfileLibraryInstance) createLibrary() *object.Library {
	t.TarClass = &object.Class{
		Name: "TarFile",
		Methods: map[string]object.Object{
			"getnames":   &object.Builtin{Fn: t.fnGetnames, HelpText: "getnames() - Return a list of archive member names"},
			"read":       &object.Builtin{Fn: t.fnRead, HelpText: "read(name) - Read a member from the archive as a string"},
			"extract":    &object.Builtin{Fn: t.fnExtract, HelpText: "extract(member, path='.') - Extract a single member"},
			"extractall": &object.Builtin{Fn: t.fnExtractall, HelpText: "extractall(path='.') - Extract all members"},
			"add":        &object.Builtin{Fn: t.fnAdd, HelpText: "add(filename, arcname=None) - Add a file to the archive"},
			"addstr":     &object.Builtin{Fn: t.fnAddstr, HelpText: "addstr(name, data) - Write a string as a member"},
			"close":      &object.Builtin{Fn: t.fnClose, HelpText: "close() - Close the archive"},
		},
	}

	return object.NewLibrary(TarfileLibraryName, map[string]*object.Builtin{
		"TarFile": {
			Fn:         t.tarConstructor,
			Attributes: t.TarClass.Methods,
			HelpText: `TarFile(path, mode="r") - Open a TAR archive

Modes: "r" (uncompressed), "r:gz" (gzipped), "w" (write uncompressed),
"w:gz" (write gzipped).`,
		},
		"is_tarfile": {
			Fn:       t.fnIsTarfile,
			HelpText: `is_tarfile(path) - Return True if path is a valid TAR archive`,
		},
	}, nil, "TAR archive reading and writing")
}

func (t *tarfileLibraryInstance) tarConstructor(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	if err := errors.RangeArgs(args, 1, 2); err != nil {
		return err
	}
	path, err := args[0].AsString()
	if err != nil {
		return err
	}
	mode := "r"
	if len(args) == 2 {
		mode, err = args[1].AsString()
		if err != nil {
			return err
		}
	}
	if err := checkPathSecurity(t.config, path); err != nil {
		return err
	}

	nd := &tarFileNativeData{path: path, mode: mode, headerIndex: make(map[string]*tar.Header)}

	switch mode {
	case "r", "r:gz":
		var f *os.File
		var openErr error
		object.RunBlocking(ctx, func() {
			f, openErr = os.Open(path)
		})
		if openErr != nil {
			return errors.NewError("cannot open tar: %s", openErr.Error())
		}
		nd.file = f
		var src io.Reader = f
		if mode == "r:gz" {
			gz, err := gzip.NewReader(f)
			if err != nil {
				f.Close()
				return errors.NewError("cannot open gzip: %s", err.Error())
			}
			nd.zipWriter = nil // track gzip reader for closing
			nd.closer = &tarGzipCloser{f: f, gz: gz}
			src = gz
		} else {
			nd.closer = f
		}
		nd.reader = tar.NewReader(src)
		// Pre-read all headers so getnames/read/extract can seek by name.
		object.RunBlocking(ctx, func() {
			for {
				hdr, err := nd.reader.Next()
				if err != nil {
					break
				}
				nd.headers = append(nd.headers, hdr)
				nd.headerIndex[hdr.Name] = hdr
			}
		})

	case "w", "w:gz":
		var f *os.File
		var createErr error
		object.RunBlocking(ctx, func() {
			f, createErr = os.Create(path)
		})
		if createErr != nil {
			return errors.NewError("cannot create tar: %s", createErr.Error())
		}
		nd.file = f
		if mode == "w:gz" {
			gz := gzip.NewWriter(f)
			nd.zipWriter = gz
			nd.writer = tar.NewWriter(gz)
			nd.closer = &tarGzipCloser{f: f, gz: gz}
		} else {
			nd.writer = tar.NewWriter(f)
			nd.closer = f
		}

	default:
		return errors.NewError("TarFile: mode must be 'r', 'r:gz', 'w', or 'w:gz', got '%s'", mode)
	}

	return object.NewInstanceWithData(t.TarClass, map[string]object.Object{
		"__str__": object.NewString(filepath.Base(path) + " (mode=" + mode + ")"),
	}, nd)
}

// tarGzipCloser closes both the gzip writer/reader and the underlying file.
type tarGzipCloser struct {
	f  *os.File
	gz interface{ Close() error }
}

func (c *tarGzipCloser) Close() error {
	c.gz.Close()
	return c.f.Close()
}

func tarFromArgs(args []object.Object) (*tarFileNativeData, object.Object) {
	if len(args) < 1 {
		return nil, errors.NewError("expected TarFile instance")
	}
	inst, ok := args[0].(*object.Instance)
	if !ok {
		return nil, errors.NewError("expected TarFile instance")
	}
	nd, ok := inst.NativeData.(*tarFileNativeData)
	if !ok {
		return nil, errors.NewError("TarFile: invalid instance")
	}
	return nd, nil
}

func (t *tarfileLibraryInstance) fnGetnames(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	nd, errObj := tarFromArgs(args)
	if errObj != nil {
		return errObj
	}
	if nd.reader == nil {
		return errors.NewError("TarFile.getnames: archive not open for reading")
	}
	elements := make([]object.Object, len(nd.headers))
	for i, hdr := range nd.headers {
		elements[i] = object.NewString(hdr.Name)
	}
	return &object.List{Elements: elements}
}

func (t *tarfileLibraryInstance) fnRead(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	if err := errors.ExactArgs(args, 2); err != nil {
		return err
	}
	nd, errObj := tarFromArgs(args)
	if errObj != nil {
		return errObj
	}
	name, err := args[1].AsString()
	if err != nil {
		return err
	}
	if nd.reader == nil {
		return errors.NewError("TarFile.read: archive not open for reading")
	}

	// Re-open and stream to the target member (tar is sequential).
	var content string
	var readErr error
	object.RunBlocking(ctx, func() {
		f, err := os.Open(nd.path)
		if err != nil {
			readErr = err
			return
		}
		defer f.Close()
		var src io.Reader = f
		if nd.mode == "r:gz" {
			gz, err := gzip.NewReader(f)
			if err != nil {
				readErr = err
				return
			}
			defer gz.Close()
			src = gz
		}
		tr := tar.NewReader(src)
		for {
			hdr, err := tr.Next()
			if err != nil {
				readErr = fmt.Errorf("file %q not found in archive", name)
				return
			}
			if hdr.Name == name {
				data, err := io.ReadAll(tr)
				if err != nil {
					readErr = err
					return
				}
				content = string(data)
				return
			}
		}
	})
	if readErr != nil {
		return errors.NewError("TarFile.read: %s", readErr.Error())
	}
	return object.NewString(content)
}

func (t *tarfileLibraryInstance) fnExtract(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	if err := errors.RangeArgs(args, 2, 3); err != nil {
		return err
	}
	nd, errObj := tarFromArgs(args)
	if errObj != nil {
		return errObj
	}
	member, err := args[1].AsString()
	if err != nil {
		return err
	}
	dest := "."
	if len(args) == 3 {
		dest, err = args[2].AsString()
		if err != nil {
			return err
		}
	}
	if err := checkPathSecurity(t.config, dest); err != nil {
		return err
	}
	if nd.reader == nil {
		return errors.NewError("TarFile.extract: archive not open for reading")
	}

	hdr, ok := nd.headerIndex[member]
	if !ok {
		return errors.NewError("TarFile.extract: file %q not found in archive", member)
	}

	var extractedPath string
	var extractErr error
	object.RunBlocking(ctx, func() {
		extractedPath, extractErr = t.extractTarMember(nd, hdr, dest)
	})
	if extractErr != nil {
		return errors.NewError("TarFile.extract: %s", extractErr.Error())
	}
	return object.NewString(extractedPath)
}

func (t *tarfileLibraryInstance) fnExtractall(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	if err := errors.RangeArgs(args, 1, 2); err != nil {
		return err
	}
	nd, errObj := tarFromArgs(args)
	if errObj != nil {
		return errObj
	}
	dest := "."
	if len(args) == 2 {
		var err object.Object
		dest, err = args[1].AsString()
		if err != nil {
			return err
		}
	}
	if err := checkPathSecurity(t.config, dest); err != nil {
		return err
	}
	if nd.reader == nil {
		return errors.NewError("TarFile.extractall: archive not open for reading")
	}

	var paths []string
	var extractErr error
	object.RunBlocking(ctx, func() {
		for _, hdr := range nd.headers {
			p, err := t.extractTarMember(nd, hdr, dest)
			if err != nil {
				extractErr = err
				return
			}
			paths = append(paths, p)
		}
	})
	if extractErr != nil {
		return errors.NewError("TarFile.extractall: %s", extractErr.Error())
	}
	elements := make([]object.Object, len(paths))
	for i, p := range paths {
		elements[i] = object.NewString(p)
	}
	return &object.List{Elements: elements}
}

// extractTarMember re-opens the archive and streams to the target header.
func (t *tarfileLibraryInstance) extractTarMember(nd *tarFileNativeData, hdr *tar.Header, dest string) (string, error) {
	target := filepath.Join(dest, hdr.Name)
	if !isWithinDir(dest, target) {
		return "", fmt.Errorf("tar entry %q escapes destination directory", hdr.Name)
	}
	if !t.config.IsPathAllowed(target) {
		return "", fmt.Errorf("access denied: path '%s' is outside allowed directories", target)
	}

	f, err := os.Open(nd.path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	var src io.Reader = f
	if nd.mode == "r:gz" {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return "", err
		}
		defer gz.Close()
		src = gz
	}
	tr := tar.NewReader(src)
	for {
		h, err := tr.Next()
		if err != nil {
			return "", fmt.Errorf("file %q not found", hdr.Name)
		}
		if h.Name != hdr.Name {
			continue
		}
		if h.Typeflag == tar.TypeDir {
			return target, os.MkdirAll(target, os.FileMode(h.Mode))
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return "", err
		}
		w, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(h.Mode))
		if err != nil {
			return "", err
		}
		defer w.Close()
		_, err = io.Copy(w, tr)
		return target, err
	}
}

func (t *tarfileLibraryInstance) fnAdd(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	if err := errors.RangeArgs(args, 2, 3); err != nil {
		return err
	}
	nd, errObj := tarFromArgs(args)
	if errObj != nil {
		return errObj
	}
	filename, err := args[1].AsString()
	if err != nil {
		return err
	}
	arcname := filename
	if len(args) == 3 {
		arcname, err = args[2].AsString()
		if err != nil {
			return err
		}
	}
	if err := checkPathSecurity(t.config, filename); err != nil {
		return err
	}
	if nd.writer == nil {
		return errors.NewError("TarFile.add: archive not open for writing")
	}

	var writeErr error
	object.RunBlocking(ctx, func() {
		writeErr = addPathToTar(nd.writer, filename, arcname, t.config)
	})
	if writeErr != nil {
		return errors.NewError("TarFile.add: %s", writeErr.Error())
	}
	return &object.Null{}
}

func (t *tarfileLibraryInstance) fnAddstr(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	if err := errors.ExactArgs(args, 3); err != nil {
		return err
	}
	nd, errObj := tarFromArgs(args)
	if errObj != nil {
		return errObj
	}
	name, err := args[1].AsString()
	if err != nil {
		return err
	}
	data, err := args[2].AsString()
	if err != nil {
		return err
	}
	if nd.writer == nil {
		return errors.NewError("TarFile.addstr: archive not open for writing")
	}

	var writeErr error
	object.RunBlocking(ctx, func() {
		hdr := &tar.Header{Name: name, Mode: 0644, Size: int64(len(data)), Typeflag: tar.TypeReg, Format: tar.FormatGNU}
		if err := nd.writer.WriteHeader(hdr); err != nil {
			writeErr = err
			return
		}
		_, writeErr = nd.writer.Write([]byte(data))
	})
	if writeErr != nil {
		return errors.NewError("TarFile.addstr: %s", writeErr.Error())
	}
	return &object.Null{}
}

func (t *tarfileLibraryInstance) fnClose(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	nd, errObj := tarFromArgs(args)
	if errObj != nil {
		return errObj
	}
	if nd.closed {
		return &object.Null{}
	}
	object.RunBlocking(ctx, func() {
		if nd.writer != nil {
			nd.writer.Close()
		}
		if nd.closer != nil {
			nd.closer.Close()
		}
	})
	nd.closed = true
	return &object.Null{}
}

func (t *tarfileLibraryInstance) fnIsTarfile(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	if err := errors.ExactArgs(args, 1); err != nil {
		return err
	}
	path, err := args[0].AsString()
	if err != nil {
		return err
	}
	if err := checkPathSecurity(t.config, path); err != nil {
		return err
	}
	var valid bool
	object.RunBlocking(ctx, func() {
		f, err := os.Open(path)
		if err != nil {
			valid = false
			return
		}
		defer f.Close()
		// Try plain tar first.
		tr := tar.NewReader(f)
		if _, err := tr.Next(); err == nil {
			valid = true
			return
		}
		// Try gzipped tar.
		f.Seek(0, io.SeekStart)
		gz, err := gzip.NewReader(f)
		if err != nil {
			valid = false
			return
		}
		defer gz.Close()
		gzTr := tar.NewReader(gz)
		_, err = gzTr.Next()
		valid = err == nil
	})
	return object.NewBoolean(valid)
}

// addPathToTar adds a file or directory tree to the tar writer.
func addPathToTar(w *tar.Writer, srcPath, arcname string, config fssecurity.Config) error {
	info, err := os.Stat(srcPath)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return filepath.WalkDir(srcPath, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			if !config.IsPathAllowed(path) {
				return nil
			}
			rel, err := filepath.Rel(srcPath, path)
			if err != nil {
				return err
			}
			return addFileToTar(w, path, filepath.Join(arcname, rel))
		})
	}
	return addFileToTar(w, srcPath, arcname)
}

func addFileToTar(w *tar.Writer, srcPath, arcname string) error {
	info, err := os.Stat(srcPath)
	if err != nil {
		return err
	}
	hdr, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	hdr.Name = filepath.ToSlash(arcname)
	hdr.Format = tar.FormatGNU
	if err := w.WriteHeader(hdr); err != nil {
		return err
	}
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}
