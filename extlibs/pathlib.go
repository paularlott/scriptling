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

// PathlibLibraryInstance holds the configured Pathlib library instance
type PathlibLibraryInstance struct {
	config fssecurity.Config
}

// RegisterPathlibLibrary registers the pathlib library with a Scriptling instance.
func RegisterPathlibLibrary(registrar object.LibraryRegistrar, allowedPaths []string) {
	config := fssecurity.Config{
		AllowedPaths: allowedPaths,
	}
	pathLib := NewPathlibLibrary(config)
	registrar.RegisterLibrary("pathlib", pathLib)
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
	return object.NewLibrary(map[string]*object.Builtin{
		"Path": {
			Fn: p.pathConstructor,
			HelpText: `Path(path) - Create a new Path object

Returns a Path object representing the filesystem path.`,
		},
	}, nil, "Object-oriented filesystem paths")
}

func (p *PathlibLibraryInstance) pathConstructor(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
	if len(args) != 1 {
		return errors.NewArgumentError(len(args), 1)
	}
	pathStr, ok := args[0].AsString()
	if !ok {
		return errors.NewTypeError("STRING", args[0].Type().String())
	}

	return p.createPathObject(pathStr)
}

func (p *PathlibLibraryInstance) createPathObject(pathStr string) *object.Dict {
	// Clean the path
	cleanPath := filepath.Clean(pathStr)

	// Create the dictionary that will represent the Path object
	pathDict := &object.Dict{
		Pairs: make(map[string]object.DictPair),
	}

	// Helper to add methods to the dict
	addMethod := func(name string, fn object.BuiltinFunction, help string) {
		pathDict.Pairs[name] = object.DictPair{
			Key: &object.String{Value: name},
			Value: &object.Builtin{
				Fn:       fn,
				HelpText: help,
			},
		}
	}

	// Helper to add properties (as values)
	addProperty := func(name string, val object.Object) {
		pathDict.Pairs[name] = object.DictPair{
			Key:   &object.String{Value: name},
			Value: val,
		}
	}

	// Properties
	base := filepath.Base(cleanPath)
	ext := filepath.Ext(cleanPath)
	stem := strings.TrimSuffix(base, ext)
	if base == "/" {
		stem = ""
	}
	addProperty("name", &object.String{Value: base})
	addProperty("stem", &object.String{Value: stem})
	addProperty("suffix", &object.String{Value: ext})
	addProperty("parent", &object.String{Value: filepath.Dir(cleanPath)}) // Returning string for parent to avoid infinite recursion/complexity for now

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
	addProperty("parts", &object.Tuple{Elements: partObjs})

	// __str__ equivalent (for printing) - though Scriptling doesn't auto-call it yet
	addProperty("__str__", &object.String{Value: cleanPath})

	// Methods

	// joinpath(*args)
	addMethod("joinpath", func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
		parts := make([]string, 0, len(args)+1)
		parts = append(parts, cleanPath)
		for _, arg := range args {
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
	}, "joinpath(*other) - Combine this path with other path segments")

	// exists()
	addMethod("exists", func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
		if err := p.checkPathSecurity(cleanPath); err != nil {
			return err
		}
		_, err := os.Stat(cleanPath)
		return &object.Boolean{Value: err == nil}
	}, "exists() - Check if the path exists")

	// is_file()
	addMethod("is_file", func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
		if err := p.checkPathSecurity(cleanPath); err != nil {
			return err
		}
		info, err := os.Stat(cleanPath)
		if err != nil {
			return &object.Boolean{Value: false}
		}
		return &object.Boolean{Value: !info.IsDir()}
	}, "is_file() - Check if the path is a regular file")

	// is_dir()
	addMethod("is_dir", func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
		if err := p.checkPathSecurity(cleanPath); err != nil {
			return err
		}
		info, err := os.Stat(cleanPath)
		if err != nil {
			return &object.Boolean{Value: false}
		}
		return &object.Boolean{Value: info.IsDir()}
	}, "is_dir() - Check if the path is a directory")

	// mkdir(mode=0o777, parents=False, exist_ok=False)
	addMethod("mkdir", func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
		if err := p.checkPathSecurity(cleanPath); err != nil {
			return err
		}

		parents := false
		if val, ok := kwargs["parents"]; ok {
			if b, ok := val.AsBool(); ok {
				parents = b
			}
		}

		// We ignore exist_ok and mode for simplicity in this minimal version,
		// but could implement them if needed.

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
	}, "mkdir(parents=False) - Create a new directory at this given path")

	// rmdir()
	addMethod("rmdir", func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
		if err := p.checkPathSecurity(cleanPath); err != nil {
			return err
		}
		err := os.Remove(cleanPath)
		if err != nil {
			return errors.NewError("cannot remove directory: %s", err.Error())
		}
		return &object.Null{}
	}, "rmdir() - Remove the empty directory")

	// unlink(missing_ok=False)
	addMethod("unlink", func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
		if err := p.checkPathSecurity(cleanPath); err != nil {
			return err
		}

		missingOk := false
		if val, ok := kwargs["missing_ok"]; ok {
			if b, ok := val.AsBool(); ok {
				missingOk = b
			}
		}

		err := os.Remove(cleanPath)
		if err != nil {
			if missingOk && os.IsNotExist(err) {
				return &object.Null{}
			}
			return errors.NewError("cannot remove file: %s", err.Error())
		}
		return &object.Null{}
	}, "unlink(missing_ok=False) - Remove this file or symbolic link")

	// read_text()
	addMethod("read_text", func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
		if err := p.checkPathSecurity(cleanPath); err != nil {
			return err
		}
		content, err := os.ReadFile(cleanPath)
		if err != nil {
			return errors.NewError("cannot read file: %s", err.Error())
		}
		return &object.String{Value: string(content)}
	}, "read_text() - Read the contents of the file as a string")

	// write_text(data)
	addMethod("write_text", func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
		if len(args) != 1 {
			return errors.NewArgumentError(len(args), 1)
		}
		content, ok := args[0].AsString()
		if !ok {
			return errors.NewTypeError("STRING", args[0].Type().String())
		}

		if err := p.checkPathSecurity(cleanPath); err != nil {
			return err
		}

		err := os.WriteFile(cleanPath, []byte(content), 0644)
		if err != nil {
			return errors.NewError("cannot write file: %s", err.Error())
		}
		return &object.Null{}
	}, "write_text(data) - Write the string data to the file")

	return pathDict
}

func (p *PathlibLibraryInstance) checkPathSecurity(path string) object.Object {
	if !p.config.IsPathAllowed(path) {
		return errors.NewError("access denied: path '%s' is outside allowed directories", path)
	}
	return nil
}
