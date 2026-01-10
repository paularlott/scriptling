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

// PathClass is defined per library instance to avoid initialization cycle

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
	registrar.RegisterLibrary(PathlibLibraryName, pathLib)
}

// NewPathlibLibrary creates a new Pathlib library with the given configuration.
func NewPathlibLibrary(config fssecurity.Config) *object.Library {
	// Normalize and validate allowed paths (same as in os.go)
	normalizedPaths := make([]string, 0, len(config.AllowedPaths))
	for _, p := range config.AllowedPaths {
		absPath, err := filepath.Abs(p)
		if err != nil {
			continue
		}
		normalizedPaths = append(normalizedPaths, filepath.Clean(absPath))
	}
	config.AllowedPaths = normalizedPaths

	instance := &PathlibLibraryInstance{config: config}
	return instance.createPathlibLibrary()
}

func (p *PathlibLibraryInstance) createPathlibLibrary() *object.Library {
	// Define PathClass with methods that capture the library instance
	p.PathClass = &object.Class{
		Name: "Path",
		Methods: map[string]object.Object{
			"joinpath": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if len(args) < 1 {
						return errors.NewArgumentError(len(args), 1)
					}
					pathInstance := args[0].(*object.Instance)
					cleanPath, _ := pathInstance.Fields["__path__"].AsString()

					parts := make([]string, 0, len(args))
					parts = append(parts, cleanPath)
					for _, arg := range args[1:] {
						s, ok := arg.AsString()
						if !ok {
							return errors.NewTypeError("STRING", arg.Type().String())
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
					if len(args) != 1 {
						return errors.NewArgumentError(len(args), 1)
					}
					pathInstance := args[0].(*object.Instance)
					cleanPath, _ := pathInstance.Fields["__path__"].AsString()

					if err := p.checkPathSecurity(cleanPath); err != nil {
						return err
					}
					_, err := os.Stat(cleanPath)
					return &object.Boolean{Value: err == nil}
				},
				HelpText: "exists() - Check if the path exists",
			},
			"is_file": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if len(args) != 1 {
						return errors.NewArgumentError(len(args), 1)
					}
					pathInstance := args[0].(*object.Instance)
					cleanPath, _ := pathInstance.Fields["__path__"].AsString()

					if err := p.checkPathSecurity(cleanPath); err != nil {
						return &object.Boolean{Value: false}
					}
					info, err := os.Stat(cleanPath)
					if err != nil {
						return &object.Boolean{Value: false}
					}
					return &object.Boolean{Value: !info.IsDir()}
				},
				HelpText: "is_file() - Check if the path is a regular file",
			},
			"is_dir": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if len(args) != 1 {
						return errors.NewArgumentError(len(args), 1)
					}
					pathInstance := args[0].(*object.Instance)
					cleanPath, _ := pathInstance.Fields["__path__"].AsString()

					if err := p.checkPathSecurity(cleanPath); err != nil {
						return &object.Boolean{Value: false}
					}
					info, err := os.Stat(cleanPath)
					if err != nil {
						return &object.Boolean{Value: false}
					}
					return &object.Boolean{Value: info.IsDir()}
				},
				HelpText: "is_dir() - Check if the path is a directory",
			},
			"mkdir": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if len(args) != 1 {
						return errors.NewArgumentError(len(args), 1)
					}
					pathInstance := args[0].(*object.Instance)
					cleanPath, _ := pathInstance.Fields["__path__"].AsString()

					if err := p.checkPathSecurity(cleanPath); err != nil {
						return err
					}

					parents := false
					if val, ok := kwargs.Kwargs["parents"]; ok {
						if b, ok := val.AsBool(); ok {
							parents = b
						}
					}

					var err error
					if parents {
						err = os.MkdirAll(cleanPath, 0755)
					} else {
						err = os.Mkdir(cleanPath, 0755)
					}

					if err != nil {
						return errors.NewError("cannot create directory: %s", err.Error())
					}
					return &object.Null{}
				},
				HelpText: "mkdir(parents=False) - Create a new directory at this given path",
			},
			"rmdir": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if len(args) != 1 {
						return errors.NewArgumentError(len(args), 1)
					}
					pathInstance := args[0].(*object.Instance)
					cleanPath, _ := pathInstance.Fields["__path__"].AsString()

					if err := p.checkPathSecurity(cleanPath); err != nil {
						return err
					}

					err := os.Remove(cleanPath)
					if err != nil {
						return errors.NewError("cannot remove directory: %s", err.Error())
					}
					return &object.Null{}
				},
				HelpText: "rmdir() - Remove the empty directory",
			},
			"unlink": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if len(args) != 1 {
						return errors.NewArgumentError(len(args), 1)
					}
					pathInstance := args[0].(*object.Instance)
					cleanPath, _ := pathInstance.Fields["__path__"].AsString()

					missingOk := false
					if val, ok := kwargs.Kwargs["missing_ok"]; ok {
						if b, ok := val.AsBool(); ok {
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
					if len(args) != 1 {
						return errors.NewArgumentError(len(args), 1)
					}
					pathInstance := args[0].(*object.Instance)
					cleanPath, _ := pathInstance.Fields["__path__"].AsString()

					if err := p.checkPathSecurity(cleanPath); err != nil {
						return err
					}

					content, err := os.ReadFile(cleanPath)
					if err != nil {
						return errors.NewError("cannot read file: %s", err.Error())
					}
					return &object.String{Value: string(content)}
				},
				HelpText: "read_text() - Read the contents of the file as a string",
			},
			"write_text": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if len(args) != 2 {
						return errors.NewArgumentError(len(args), 2)
					}
					pathInstance := args[0].(*object.Instance)
					cleanPath, _ := pathInstance.Fields["__path__"].AsString()
					content, ok := args[1].AsString()
					if !ok {
						return errors.NewTypeError("STRING", args[1].Type().String())
					}

					if err := p.checkPathSecurity(cleanPath); err != nil {
						return err
					}

					err := os.WriteFile(cleanPath, []byte(content), 0644)
					if err != nil {
						return errors.NewError("cannot write file: %s", err.Error())
					}
					return &object.Null{}
				},
				HelpText: "write_text(data) - Write the string data to the file",
			},
		},
	}

	return object.NewLibrary(map[string]*object.Builtin{
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
  - mkdir(parents=False) - Create a new directory at this given path
  - rmdir() - Remove the empty directory
  - unlink(missing_ok=False) - Remove this file or symbolic link
  - read_text() - Read the contents of the file as a string
  - write_text(data) - Write the string data to the file

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

func (p *PathlibLibraryInstance) pathConstructor(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	if len(args) != 1 {
		return errors.NewArgumentError(len(args), 1)
	}
	pathStr, ok := args[0].AsString()
	if !ok {
		return errors.NewTypeError("STRING", args[0].Type().String())
	}

	return p.createPathObject(pathStr)
}

func (p *PathlibLibraryInstance) createPathObject(pathStr string) object.Object {
	// Clean the path
	cleanPath := filepath.Clean(pathStr)

	// Create the Path instance
	pathInstance := &object.Instance{
		Class:  p.PathClass,
		Fields: make(map[string]object.Object),
	}

	// Store the internal path
	pathInstance.Fields["__path__"] = &object.String{Value: cleanPath}

	// Store allowed paths for security checks
	allowedPaths := make([]object.Object, len(p.config.AllowedPaths))
	for i, path := range p.config.AllowedPaths {
		allowedPaths[i] = &object.String{Value: path}
	}
	pathInstance.Fields["__allowed_paths__"] = &object.List{Elements: allowedPaths}

	// Properties
	base := filepath.Base(cleanPath)
	ext := filepath.Ext(cleanPath)
	stem := strings.TrimSuffix(base, ext)
	if base == "/" {
		stem = ""
	}
	pathInstance.Fields["name"] = &object.String{Value: base}
	pathInstance.Fields["stem"] = &object.String{Value: stem}
	pathInstance.Fields["suffix"] = &object.String{Value: ext}
	pathInstance.Fields["parent"] = &object.String{Value: filepath.Dir(cleanPath)}

	parts := strings.Split(cleanPath, string(os.PathSeparator))
	if len(parts) > 1 && parts[0] == "" && parts[1] == "" {
		parts = []string{"/"}
	} else if len(parts) > 0 && parts[0] == "" {
		parts[0] = "/"
	}
	partObjs := make([]object.Object, len(parts))
	for i, part := range parts {
		partObjs[i] = &object.String{Value: part}
	}
	pathInstance.Fields["parts"] = &object.Tuple{Elements: partObjs}

	pathInstance.Fields["__str__"] = &object.String{Value: cleanPath}

	return pathInstance
}

func (p *PathlibLibraryInstance) checkPathSecurity(path string) object.Object {
	if !p.config.IsPathAllowed(path) {
		return errors.NewError("access denied: path '%s' is outside allowed directories", path)
	}
	return nil
}
