// Package extlibs provides external libraries that need explicit registration
package extlibs

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/extlibs/fssecurity"
	"github.com/paularlott/scriptling/object"
)

// zipfileLibraryInstance holds the configured zipfile library instance.
type zipfileLibraryInstance struct {
	config   fssecurity.Config
	ZipClass *object.Class
}

func RegisterZipfileLibrary(registrar object.LibraryRegistrar, allowedPaths []string) {
	registrar.RegisterLibrary(NewZipfileLibrary(fssecurity.Config{AllowedPaths: allowedPaths}))
}

func NewZipfileLibrary(config fssecurity.Config) *object.Library {
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
	inst := &zipfileLibraryInstance{config: config}
	return inst.createLibrary()
}

// zipFileNativeData holds the open archive state for a ZipFile instance.
type zipFileNativeData struct {
	reader *zip.ReadCloser
	writer *zip.Writer
	file   *os.File
	path   string
	mode   string
	closed bool
}

func (z *zipfileLibraryInstance) createLibrary() *object.Library {
	z.ZipClass = &object.Class{
		Name: "ZipFile",
		Methods: map[string]object.Object{
			"namelist":   &object.Builtin{Fn: z.fnNamelist, HelpText: "namelist() - Return a list of archive member names"},
			"read":       &object.Builtin{Fn: z.fnRead, HelpText: "read(name) - Read a member from the archive as a string"},
			"extract":    &object.Builtin{Fn: z.fnExtract, HelpText: "extract(member, path='.') - Extract a single member"},
			"extractall": &object.Builtin{Fn: z.fnExtractall, HelpText: "extractall(path='.') - Extract all members"},
			"write":      &object.Builtin{Fn: z.fnWrite, HelpText: "write(filename, arcname=None) - Add a file to the archive"},
			"writestr":   &object.Builtin{Fn: z.fnWritestr, HelpText: "writestr(name, data) - Write a string as a member"},
			"close":      &object.Builtin{Fn: z.fnClose, HelpText: "close() - Close the archive"},
		},
	}

	return object.NewLibrary(ZipfileLibraryName, map[string]*object.Builtin{
		"ZipFile": {
			Fn:         z.zipConstructor,
			Attributes: z.ZipClass.Methods,
			HelpText: `ZipFile(path, mode="r") - Open a ZIP archive

mode "r" opens an existing archive for reading; mode "w" creates a new
archive for writing. Close the archive with close() when done.`,
		},
		"is_zipfile": {
			Fn:       z.fnIsZipfile,
			HelpText: `is_zipfile(path) - Return True if path is a valid ZIP archive`,
		},
	}, nil, "ZIP archive reading and writing")
}

func (z *zipfileLibraryInstance) zipConstructor(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
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
	if err := checkPathSecurity(z.config, path); err != nil {
		return err
	}

	nd := &zipFileNativeData{path: path, mode: mode}

	switch mode {
	case "r":
		var openErr error
		object.RunBlocking(ctx, func() {
			nd.reader, openErr = zip.OpenReader(path)
		})
		if openErr != nil {
			return errors.NewError("cannot open zip: %s", openErr.Error())
		}
	case "w":
		var createErr error
		object.RunBlocking(ctx, func() {
			nd.file, createErr = os.Create(path)
		})
		if createErr != nil {
			return errors.NewError("cannot create zip: %s", createErr.Error())
		}
		nd.writer = zip.NewWriter(nd.file)
	default:
		return errors.NewError("ZipFile: mode must be 'r' or 'w', got '%s'", mode)
	}

	return object.NewInstanceWithData(z.ZipClass, map[string]object.Object{
		"__str__": object.NewString(filepath.Base(path) + " (mode=" + mode + ")"),
	}, nd)
}

func zipFromArgs(args []object.Object) (*zipFileNativeData, object.Object) {
	if len(args) < 1 {
		return nil, errors.NewError("expected ZipFile instance")
	}
	inst, ok := args[0].(*object.Instance)
	if !ok {
		return nil, errors.NewError("expected ZipFile instance")
	}
	nd, ok := inst.NativeData.(*zipFileNativeData)
	if !ok {
		return nil, errors.NewError("ZipFile: invalid instance")
	}
	return nd, nil
}

func (z *zipfileLibraryInstance) fnNamelist(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	nd, errObj := zipFromArgs(args)
	if errObj != nil {
		return errObj
	}
	if nd.mode != "r" || nd.reader == nil {
		return errors.NewError("ZipFile.namelist: archive not open for reading")
	}
	elements := make([]object.Object, len(nd.reader.File))
	for i, f := range nd.reader.File {
		elements[i] = object.NewString(f.Name)
	}
	return &object.List{Elements: elements}
}

func (z *zipfileLibraryInstance) fnRead(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	if err := errors.ExactArgs(args, 2); err != nil {
		return err
	}
	nd, errObj := zipFromArgs(args)
	if errObj != nil {
		return errObj
	}
	name, err := args[1].AsString()
	if err != nil {
		return err
	}
	if nd.mode != "r" || nd.reader == nil {
		return errors.NewError("ZipFile.read: archive not open for reading")
	}

	var content string
	var readErr error
	object.RunBlocking(ctx, func() {
		for _, f := range nd.reader.File {
			if f.Name == name {
				rc, err := f.Open()
				if err != nil {
					readErr = err
					return
				}
				defer rc.Close()
				data, err := io.ReadAll(rc)
				if err != nil {
					readErr = err
					return
				}
				content = string(data)
				return
			}
		}
		readErr = fmt.Errorf("file %q not found in archive", name)
	})
	if readErr != nil {
		return errors.NewError("ZipFile.read: %s", readErr.Error())
	}
	return object.NewString(content)
}

func (z *zipfileLibraryInstance) fnExtract(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	if err := errors.RangeArgs(args, 2, 3); err != nil {
		return err
	}
	nd, errObj := zipFromArgs(args)
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
	if err := checkPathSecurity(z.config, dest); err != nil {
		return err
	}
	if nd.mode != "r" || nd.reader == nil {
		return errors.NewError("ZipFile.extract: archive not open for reading")
	}

	var extractedPath string
	var extractErr error
	object.RunBlocking(ctx, func() {
		for _, f := range nd.reader.File {
			if f.Name == member {
				extractedPath, extractErr = z.extractZipFile(f, dest)
				return
			}
		}
		extractErr = fmt.Errorf("file %q not found in archive", member)
	})
	if extractErr != nil {
		return errors.NewError("ZipFile.extract: %s", extractErr.Error())
	}
	return object.NewString(extractedPath)
}

func (z *zipfileLibraryInstance) fnExtractall(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	if err := errors.RangeArgs(args, 1, 2); err != nil {
		return err
	}
	nd, errObj := zipFromArgs(args)
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
	if err := checkPathSecurity(z.config, dest); err != nil {
		return err
	}
	if nd.mode != "r" || nd.reader == nil {
		return errors.NewError("ZipFile.extractall: archive not open for reading")
	}

	var paths []string
	var extractErr error
	object.RunBlocking(ctx, func() {
		for _, f := range nd.reader.File {
			p, err := z.extractZipFile(f, dest)
			if err != nil {
				extractErr = err
				return
			}
			paths = append(paths, p)
		}
	})
	if extractErr != nil {
		return errors.NewError("ZipFile.extractall: %s", extractErr.Error())
	}
	elements := make([]object.Object, len(paths))
	for i, p := range paths {
		elements[i] = object.NewString(p)
	}
	return &object.List{Elements: elements}
}

// extractZipFile extracts a single zip.File to dest, with zip-slip protection.
func (z *zipfileLibraryInstance) extractZipFile(f *zip.File, dest string) (string, error) {
	target := filepath.Join(dest, f.Name)
	if !isWithinDir(dest, target) {
		return "", fmt.Errorf("zip entry %q escapes destination directory", f.Name)
	}
	if !z.config.IsPathAllowed(target) {
		return "", fmt.Errorf("access denied: path '%s' is outside allowed directories", target)
	}

	if f.FileInfo().IsDir() {
		return target, os.MkdirAll(target, f.Mode())
	}
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return "", err
	}
	rc, err := f.Open()
	if err != nil {
		return "", err
	}
	defer rc.Close()
	w, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
	if err != nil {
		return "", err
	}
	defer w.Close()
	_, err = io.Copy(w, rc)
	return target, err
}

func (z *zipfileLibraryInstance) fnWrite(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	if err := errors.RangeArgs(args, 2, 3); err != nil {
		return err
	}
	nd, errObj := zipFromArgs(args)
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
	if err := checkPathSecurity(z.config, filename); err != nil {
		return err
	}
	if nd.mode != "w" || nd.writer == nil {
		return errors.NewError("ZipFile.write: archive not open for writing")
	}

	var writeErr error
	object.RunBlocking(ctx, func() {
		writeErr = addPathToZip(nd.writer, filename, arcname, z.config)
	})
	if writeErr != nil {
		return errors.NewError("ZipFile.write: %s", writeErr.Error())
	}
	return &object.Null{}
}

func (z *zipfileLibraryInstance) fnWritestr(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	if err := errors.ExactArgs(args, 3); err != nil {
		return err
	}
	nd, errObj := zipFromArgs(args)
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
	if nd.mode != "w" || nd.writer == nil {
		return errors.NewError("ZipFile.writestr: archive not open for writing")
	}

	var writeErr error
	object.RunBlocking(ctx, func() {
		f, err := nd.writer.Create(name)
		if err != nil {
			writeErr = err
			return
		}
		_, writeErr = f.Write([]byte(data))
	})
	if writeErr != nil {
		return errors.NewError("ZipFile.writestr: %s", writeErr.Error())
	}
	return &object.Null{}
}

func (z *zipfileLibraryInstance) fnClose(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	nd, errObj := zipFromArgs(args)
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
		if nd.reader != nil {
			nd.reader.Close()
		}
		if nd.file != nil {
			nd.file.Close()
		}
	})
	nd.closed = true
	return &object.Null{}
}

func (z *zipfileLibraryInstance) fnIsZipfile(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	if err := errors.ExactArgs(args, 1); err != nil {
		return err
	}
	path, err := args[0].AsString()
	if err != nil {
		return err
	}
	if err := checkPathSecurity(z.config, path); err != nil {
		return err
	}
	var valid bool
	object.RunBlocking(ctx, func() {
		r, err := zip.OpenReader(path)
		if err != nil {
			valid = false
			return
		}
		r.Close()
		valid = true
	})
	return object.NewBoolean(valid)
}

// addPathToZip adds a file or directory tree to the zip writer.
func addPathToZip(w *zip.Writer, srcPath, arcname string, config fssecurity.Config) error {
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
			return addFileToZip(w, path, filepath.ToSlash(filepath.Join(arcname, rel)))
		})
	}
	return addFileToZip(w, srcPath, filepath.ToSlash(arcname))
}

func addFileToZip(w *zip.Writer, srcPath, arcname string) error {
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}
	f, err := w.Create(arcname)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	return err
}

// isWithinDir checks that target is inside dest (prevents zip/tar-slip).
func isWithinDir(dest, target string) bool {
	cleanDest := filepath.Clean(dest)
	cleanTarget := filepath.Clean(target)
	if cleanTarget == cleanDest {
		return true
	}
	return strings.HasPrefix(cleanTarget, cleanDest+string(os.PathSeparator))
}
