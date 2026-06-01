package extlibs

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/extlibs/fssecurity"
	"github.com/paularlott/scriptling/object"
)

// pathNativeData holds the internal path string stored in Instance.NativeData.
type pathNativeData struct {
	path string
}

func pathFrom(inst *object.Instance) (string, object.Object) {
	if nd, ok := inst.NativeData.(*pathNativeData); ok {
		return nd.path, nil
	}
	return "", errors.NewError("Path: invalid native data")
}

func pathArg(args []object.Object) (string, object.Object) {
	inst, ok := args[0].(*object.Instance)
	if !ok {
		return "", errors.NewError("Path: expected a Path instance")
	}
	return pathFrom(inst)
}

// PathlibLibraryInstance holds the configured Pathlib library instance
type PathlibLibraryInstance struct {
	config    fssecurity.Config
	PathClass *object.Class
}

// RegisterPathlibLibrary registers the pathlib library with a Scriptling instance.
func RegisterPathlibLibrary(registrar object.LibraryRegistrar, allowedPaths []string) {
	config := fssecurity.Config{
		AllowedPaths: allowedPaths,
	}
	pathLib := NewPathlibLibrary(config)
	registrar.RegisterLibrary(pathLib)
}

// NewPathlibLibrary creates a new Pathlib library with the given configuration.
func NewPathlibLibrary(config fssecurity.Config) *object.Library {
	if config.AllowedPaths != nil {
		normalizedPaths := make([]string, 0, len(config.AllowedPaths))
		for _, p := range config.AllowedPaths {
			absPath, err := filepath.Abs(p)
			if err != nil {
				continue
			}
			normalizedPaths = append(normalizedPaths, filepath.Clean(absPath))
		}
		config.AllowedPaths = normalizedPaths
	}

	instance := &PathlibLibraryInstance{config: config}
	return instance.createPathlibLibrary()
}

func (p *PathlibLibraryInstance) checkPathSecurity(path string) object.Object {
	if !p.config.IsPathAllowed(path) {
		return errors.NewPermissionError("access denied: path '%s' is outside allowed directories", path)
	}
	return nil
}

func (p *PathlibLibraryInstance) createPathObject(pathStr string) object.Object {
	cleanPath := filepath.Clean(pathStr)

	base := filepath.Base(cleanPath)
	ext := filepath.Ext(cleanPath)
	stem := strings.TrimSuffix(base, ext)
	if base == "/" {
		stem = ""
	}

	parts := strings.Split(cleanPath, string(os.PathSeparator))
	if len(parts) > 1 && parts[0] == "" && parts[1] == "" {
		parts = []string{"/"}
	} else if len(parts) > 0 && parts[0] == "" {
		parts[0] = "/"
	}
	partObjs := make([]object.Object, len(parts))
	for i, part := range parts {
		partObjs[i] = object.NewString(part)
	}

	return &object.Instance{
		Class: p.PathClass,
		Fields: map[string]object.Object{
			"name":    object.NewString(base),
			"stem":    object.NewString(stem),
			"suffix":  object.NewString(ext),
			"parent":  object.NewString(filepath.Dir(cleanPath)),
			"parts":   &object.Tuple{Elements: partObjs},
			"__str__": object.NewString(cleanPath),
		},
		NativeData: &pathNativeData{path: cleanPath},
	}
}

func (p *PathlibLibraryInstance) pathConstructor(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	if err := errors.ExactArgs(args, 1); err != nil {
		return err
	}
	pathStr, err := args[0].AsString()
	if err != nil {
		return err
	}
	return p.createPathObject(pathStr)
}

func (p *PathlibLibraryInstance) createPathlibLibrary() *object.Library {
	p.PathClass = &object.Class{
		Name: "Path",
		Methods: map[string]object.Object{
			"joinpath": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.MinArgs(args, 1); err != nil {
						return err
					}
					cleanPath, errObj := pathArg(args)
					if errObj != nil {
						return errObj
					}
					parts := []string{cleanPath}
					for _, arg := range args[1:] {
						s, err := arg.AsString()
						if err != nil {
							return err
						}
						parts = append(parts, s)
					}
					newPath := parts[0]
					for _, part := range parts[1:] {
						if filepath.IsAbs(part) {
							newPath = part
						} else {
							newPath = filepath.Join(newPath, part)
						}
					}
					return p.createPathObject(newPath)
				},
				HelpText: "joinpath(*other) - Combine this path with other path segments",
			},
			"exists": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.ExactArgs(args, 1); err != nil {
						return err
					}
					cleanPath, errObj := pathArg(args)
					if errObj != nil {
						return errObj
					}
					if err := p.checkPathSecurity(cleanPath); err != nil {
						return err
					}
					_, err := os.Stat(cleanPath)
					return object.NewBoolean(err == nil)
				},
				HelpText: "exists() - Check if the path exists",
			},
			"is_file": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.ExactArgs(args, 1); err != nil {
						return err
					}
					cleanPath, errObj := pathArg(args)
					if errObj != nil {
						return errObj
					}
					if err := p.checkPathSecurity(cleanPath); err != nil {
						return object.NewBoolean(false)
					}
					info, err := os.Stat(cleanPath)
					if err != nil {
						return object.NewBoolean(false)
					}
					return object.NewBoolean(!info.IsDir())
				},
				HelpText: "is_file() - Check if the path is a regular file",
			},
			"is_dir": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.ExactArgs(args, 1); err != nil {
						return err
					}
					cleanPath, errObj := pathArg(args)
					if errObj != nil {
						return errObj
					}
					if err := p.checkPathSecurity(cleanPath); err != nil {
						return object.NewBoolean(false)
					}
					info, err := os.Stat(cleanPath)
					if err != nil {
						return object.NewBoolean(false)
					}
					return object.NewBoolean(info.IsDir())
				},
				HelpText: "is_dir() - Check if the path is a directory",
			},
			"mkdir": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.RangeArgs(args, 1, 2); err != nil {
						return err
					}
					cleanPath, errObj := pathArg(args)
					if errObj != nil {
						return errObj
					}
					mode, errObj := parseFileMode(args, kwargs, 1, 0777)
					if errObj != nil {
						return errObj
					}
					if err := p.checkPathSecurity(cleanPath); err != nil {
						return err
					}
					parents, errObj := kwargs.GetBool("parents", false)
					if errObj != nil {
						return errObj
					}
					existOk, errObj := kwargs.GetBool("exist_ok", false)
					if errObj != nil {
						return errObj
					}
					var err error
					if parents {
						if !existOk {
							if _, statErr := os.Stat(cleanPath); statErr == nil {
								return errors.NewError("cannot create directory: file exists")
							}
						}
						err = os.MkdirAll(cleanPath, mode)
					} else {
						err = os.Mkdir(cleanPath, mode)
						if existOk && os.IsExist(err) {
							if info, statErr := os.Stat(cleanPath); statErr == nil && info.IsDir() {
								return &object.Null{}
							}
						}
					}
					if err != nil {
						return errors.NewError("cannot create directory: %s", err.Error())
					}
					return &object.Null{}
				},
				HelpText: "mkdir(mode=0o777, parents=False, exist_ok=False) - Create a new directory at this given path",
			},
			"chmod": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.RangeArgs(args, 1, 2); err != nil {
						return err
					}
					cleanPath, errObj := pathArg(args)
					if errObj != nil {
						return errObj
					}
					if len(args) == 1 && !kwargs.Has("mode") {
						return errors.NewError("chmod() missing required argument: mode")
					}
					mode, errObj := parseFileMode(args, kwargs, 1, 0)
					if errObj != nil {
						return errObj
					}
					if err := p.checkPathSecurity(cleanPath); err != nil {
						return err
					}
					if err := os.Chmod(cleanPath, mode); err != nil {
						return errors.NewError("cannot change mode: %s", err.Error())
					}
					return &object.Null{}
				},
				HelpText: "chmod(mode) - Change file or directory mode",
			},
			"rmdir": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.ExactArgs(args, 1); err != nil {
						return err
					}
					cleanPath, errObj := pathArg(args)
					if errObj != nil {
						return errObj
					}
					if err := p.checkPathSecurity(cleanPath); err != nil {
						return err
					}
					if err := os.Remove(cleanPath); err != nil {
						return errors.NewError("cannot remove directory: %s", err.Error())
					}
					return &object.Null{}
				},
				HelpText: "rmdir() - Remove the empty directory",
			},
			"unlink": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.ExactArgs(args, 1); err != nil {
						return err
					}
					cleanPath, errObj := pathArg(args)
					if errObj != nil {
						return errObj
					}
					missingOk := false
					if val, ok := kwargs.Kwargs["missing_ok"]; ok {
						if b, err := val.AsBool(); err == nil {
							missingOk = b
						}
					}
					if err := p.checkPathSecurity(cleanPath); err != nil {
						return err
					}
					err := os.Remove(cleanPath)
					if err != nil {
						if missingOk && os.IsNotExist(err) {
							return &object.Null{}
						}
						return errors.NewError("cannot remove file: %s", err.Error())
					}
					return &object.Null{}
				},
				HelpText: "unlink(missing_ok=False) - Remove this file or symbolic link",
			},
			"read_text": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.ExactArgs(args, 1); err != nil {
						return err
					}
					cleanPath, errObj := pathArg(args)
					if errObj != nil {
						return errObj
					}
					if err := p.checkPathSecurity(cleanPath); err != nil {
						return err
					}
					content, err := os.ReadFile(cleanPath)
					if err != nil {
						return errors.NewError("cannot read file: %s", err.Error())
					}
					return object.NewString(string(content))
				},
				HelpText: "read_text() - Read the contents of the file as a string",
			},
			"read_bytes": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.ExactArgs(args, 1); err != nil {
						return err
					}
					cleanPath, errObj := pathArg(args)
					if errObj != nil {
						return errObj
					}
					if err := p.checkPathSecurity(cleanPath); err != nil {
						return err
					}
					content, err := os.ReadFile(cleanPath)
					if err != nil {
						return errors.NewError("cannot read file: %s", err.Error())
					}
					return object.NewString(string(content))
				},
				HelpText: "read_bytes() - Read the contents of the file as bytes",
			},
			"write_text": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.ExactArgs(args, 2); err != nil {
						return err
					}
					cleanPath, errObj := pathArg(args)
					if errObj != nil {
						return errObj
					}
					content, err := args[1].AsString()
					if err != nil {
						return err
					}
					if err := p.checkPathSecurity(cleanPath); err != nil {
						return err
					}
					if err := os.WriteFile(cleanPath, []byte(content), 0644); err != nil {
						return errors.NewError("cannot write file: %s", err.Error())
					}
					return &object.Null{}
				},
				HelpText: "write_text(data) - Write the string data to the file",
			},
			"write_bytes": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.ExactArgs(args, 2); err != nil {
						return err
					}
					cleanPath, errObj := pathArg(args)
					if errObj != nil {
						return errObj
					}
					content, err := args[1].AsString()
					if err != nil {
						return err
					}
					if err := p.checkPathSecurity(cleanPath); err != nil {
						return err
					}
					if err := os.WriteFile(cleanPath, []byte(content), 0644); err != nil {
						return errors.NewError("cannot write file: %s", err.Error())
					}
					return &object.Null{}
				},
				HelpText: "write_bytes(data) - Write bytes to the file",
			},
		},
	}

	return object.NewLibrary(PathlibLibraryName, map[string]*object.Builtin{
		"Path": {
			Fn:         p.pathConstructor,
			Attributes: p.PathClass.Methods,
			HelpText: `Path(path) - Create a new Path object

Path(path) creates a new Path instance representing the filesystem path.

Path instances have the following methods:
  - joinpath(*other) - Combine this path with other path segments
  - exists() - Check if the path exists
  - is_file() - Check if the path is a regular file
  - is_dir() - Check if the path is a directory
  - mkdir(mode=0o777, parents=False, exist_ok=False) - Create a new directory at this given path
  - chmod(mode) - Change file or directory mode
  - rmdir() - Remove the empty directory
  - unlink(missing_ok=False) - Remove this file or symbolic link
  - read_text() - Read the contents of the file as a string
  - write_text(data) - Write the string data to the file
  - read_bytes() - Read the contents of the file as bytes
  - write_bytes(data) - Write bytes to the file

Path instances have the following properties (accessible via indexing):
  - name - The final path component
  - stem - The final path component without its suffix
  - suffix - The final component's last suffix
  - parent - The logical parent of the path
  - parts - A tuple giving access to the path's various components
  - __str__ - String representation of the path

Returns a Path object representing the filesystem path.`,
		},
	}, map[string]object.Object{
		"PathClass": p.PathClass,
	}, "Object-oriented filesystem paths")
}
