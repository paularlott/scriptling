// Package extlibs provides external libraries that need explicit registration
package extlibs

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/extlibs/fssecurity"
	"github.com/paularlott/scriptling/object"
)

// osLibraryInstance holds the configured OS library instance
type osLibraryInstance struct {
	config fssecurity.Config
}

// checkPathSecurity validates a path and returns an error if access is denied
func (o *osLibraryInstance) checkPathSecurity(path string) object.Object {
	if !o.config.IsPathAllowed(path) {
		return errors.NewError("access denied: path '%s' is outside allowed directories", path)
	}
	return nil
}

// RegisterOSLibrary registers the os and os.path libraries with a Scriptling instance.
// If allowedPaths is empty or nil, all paths are allowed (no restrictions).
// If allowedPaths contains paths, all file operations are restricted to those directories.
//
// SECURITY: When running untrusted scripts, ALWAYS provide allowedPaths to restrict
// file system access. The security checks prevent:
// - Reading/writing files outside allowed directories
// - Path traversal attacks (../../../etc/passwd)
// - Symlink attacks (symlinks pointing outside allowed dirs)
//
// Example:
//
//	No restrictions - full filesystem access (DANGEROUS for untrusted code)
//	extlibs.RegisterOSLibrary(s, nil)
//
//	Restricted to specific directories (SECURE)
//	extlibs.RegisterOSLibrary(s, []string{"/tmp/sandbox", "/home/user/data"})
func RegisterOSLibrary(registrar object.LibraryRegistrar, allowedPaths []string) {
	config := fssecurity.Config{
		AllowedPaths: allowedPaths,
	}
	osLib, osPathLib := NewOSLibrary(config)
	registrar.RegisterLibrary(OSLibraryName, osLib)
	registrar.RegisterLibrary(OSPathLibraryName, osPathLib)
}

// NewOSLibrary creates a new OS library with the given configuration.
// The returned libraries are for "os" and "os.path".
// Prefer using RegisterOSLibrary which handles registration automatically.
func NewOSLibrary(config fssecurity.Config) (*object.Library, *object.Library) {
	// Normalize and validate allowed paths
	normalizedPaths := make([]string, 0, len(config.AllowedPaths))
	for _, p := range config.AllowedPaths {
		absPath, err := filepath.Abs(p)
		if err != nil {
			continue
		}
		normalizedPaths = append(normalizedPaths, filepath.Clean(absPath))
	}
	config.AllowedPaths = normalizedPaths

	instance := &osLibraryInstance{config: config}

	osLib := instance.createOSLibrary()
	osPathLib := instance.createOSPathLibrary()

	return osLib, osPathLib
}

func (o *osLibraryInstance) createOSLibrary() *object.Library {
	return object.NewLibrary(map[string]*object.Builtin{
		"getenv": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.RangeArgs(args, 1, 2); err != nil {
					return err
				}
				key, err := args[0].AsString()
				if err != nil {
					return err
				}
				value := os.Getenv(key)
				if value == "" && len(args) == 2 {
					return args[1]
				}
				return &object.String{Value: value}
			},
			HelpText: `getenv(key[, default]) - Get environment variable

Returns the value of the environment variable key if it exists, or default if provided.`,
		},
		"environ": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 0); err != nil {
					return err
				}
				pairs := make(map[string]object.DictPair)
				for _, env := range os.Environ() {
					parts := strings.SplitN(env, "=", 2)
					if len(parts) == 2 {
						pairs[parts[0]] = object.DictPair{
							Key:   &object.String{Value: parts[0]},
							Value: &object.String{Value: parts[1]},
						}
					}
				}
				return &object.Dict{Pairs: pairs}
			},
			HelpText: `environ() - Get all environment variables as a dictionary`,
		},
		"getcwd": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 0); err != nil {
					return err
				}
				cwd, err := os.Getwd()
				if err != nil {
					return errors.NewError("cannot get current directory: %s", err.Error())
				}
				return &object.String{Value: cwd}
			},
			HelpText: `getcwd() - Get current working directory`,
		},
		"listdir": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.MaxArgs(args, 1); err != nil {
					return err
				}
				path := "."
				if len(args) == 1 {
					var err object.Object
					path, err = args[0].AsString()
					if err != nil {
						return err
					}
				}

				// Security check
				if err := o.checkPathSecurity(path); err != nil {
					return err
				}

				entries, err := os.ReadDir(path)
				if err != nil {
					return errors.NewError("cannot read directory: %s", err.Error())
				}

				elements := make([]object.Object, len(entries))
				for i, entry := range entries {
					elements[i] = &object.String{Value: entry.Name()}
				}
				return &object.List{Elements: elements}
			},
			HelpText: `listdir(path=".") - List directory contents

Returns a list of the names of the entries in the given directory.`,
		},
		"read_file": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}
				path, err := args[0].AsString()
				if err != nil {
					return err
				}

				// Security check
				if err := o.checkPathSecurity(path); err != nil {
					return err
				}

				content, fsErr := os.ReadFile(path)
				if fsErr != nil {
					return errors.NewError("cannot read file: %s", fsErr.Error())
				}
				return &object.String{Value: string(content)}
			},
			HelpText: `read_file(path) - Read entire file contents as string

Returns the contents of the file as a string.`,
		},
		"write_file": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 2); err != nil { return err }
				path, err := args[0].AsString()
				if err != nil {
					return err
				}
				content, err := args[1].AsString()
				if err != nil {
					return errors.NewTypeError("STRING", args[1].Type().String())
				}

				// Security check
				if err := o.checkPathSecurity(path); err != nil {
					return err
				}

				fsErr := os.WriteFile(path, []byte(content), 0644)
				if fsErr != nil {
					return errors.NewError("cannot write file: %s", fsErr.Error())
				}
				return &object.Null{}
			},
			HelpText: `write_file(path, content) - Write content to file

Writes the string content to the file, creating or overwriting it.`,
		},
		"append_file": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 2); err != nil { return err }
				path, err := args[0].AsString()
				if err != nil {
					return err
				}
				content, err := args[1].AsString()
				if err != nil {
					return errors.NewTypeError("STRING", args[1].Type().String())
				}

				// Security check
				if err := o.checkPathSecurity(path); err != nil {
					return err
				}

				f, fsErr := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
				if fsErr != nil {
					return errors.NewError("cannot open file for append: %s", fsErr.Error())
				}
				defer f.Close()

				if _, err := io.WriteString(f, content); err != nil {
					return errors.NewError("cannot append to file: %s", err.Error())
				}
				return &object.Null{}
			},
			HelpText: `append_file(path, content) - Append content to file

Appends the string content to the file, creating it if it doesn't exist.`,
		},
		"remove": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil { return err }
				path, err := args[0].AsString()
				if err != nil {
					return err
				}

				// Security check
				if err := o.checkPathSecurity(path); err != nil {
					return err
				}

				fsErr := os.Remove(path)
				if fsErr != nil {
					return errors.NewError("cannot remove file: %s", fsErr.Error())
				}
				return &object.Null{}
			},
			HelpText: `remove(path) - Remove a file

Removes the specified file.`,
		},
		"mkdir": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil { return err }
				path, err := args[0].AsString()
				if err != nil {
					return err
				}

				// Security check
				if err := o.checkPathSecurity(path); err != nil {
					return err
				}

				fsErr := os.Mkdir(path, 0755)
				if fsErr != nil {
					return errors.NewError("cannot create directory: %s", fsErr.Error())
				}
				return &object.Null{}
			},
			HelpText: `mkdir(path) - Create a directory

Creates a new directory with the specified path.`,
		},
		"makedirs": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil { return err }
				path, err := args[0].AsString()
				if err != nil {
					return err
				}

				// Security check
				if err := o.checkPathSecurity(path); err != nil {
					return err
				}

				fsErr := os.MkdirAll(path, 0755)
				if fsErr != nil {
					return errors.NewError("cannot create directories: %s", fsErr.Error())
				}
				return &object.Null{}
			},
			HelpText: `makedirs(path) - Create directories recursively

Creates a directory and all parent directories as needed.`,
		},
		"rmdir": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil { return err }
				path, err := args[0].AsString()
				if err != nil {
					return err
				}

				// Security check
				if err := o.checkPathSecurity(path); err != nil {
					return err
				}

				fsErr := os.Remove(path)
				if fsErr != nil {
					return errors.NewError("cannot remove directory: %s", fsErr.Error())
				}
				return &object.Null{}
			},
			HelpText: `rmdir(path) - Remove a directory

Removes the specified empty directory.`,
		},
		"rename": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 2); err != nil { return err }
				oldPath, err := args[0].AsString()
				if err != nil {
					return err
				}
				newPath, err := args[1].AsString()
				if err != nil {
					return err
				}

				// Security check both paths
				if err := o.checkPathSecurity(oldPath); err != nil {
					return err
				}
				if err := o.checkPathSecurity(newPath); err != nil {
					return err
				}

				fsErr := os.Rename(oldPath, newPath)
				if fsErr != nil {
					return errors.NewError("cannot rename: %s", fsErr.Error())
				}
				return &object.Null{}
			},
			HelpText: `rename(old, new) - Rename a file or directory

Renames the file or directory from old to new.`,
		},
	}, map[string]object.Object{
		"sep":     &object.String{Value: string(os.PathSeparator)},
		"linesep": &object.String{Value: getLineSep()},
		"name":    &object.String{Value: getOSName()},
	}, "Operating system interface")
}

func (o *osLibraryInstance) createOSPathLibrary() *object.Library {
	return object.NewLibrary(map[string]*object.Builtin{
		"join": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) == 0 {
					return &object.String{Value: ""}
				}
				parts := make([]string, len(args))
				for i, arg := range args {
					s, err := arg.AsString()
					if err != nil {
						return err
					}
					parts[i] = s
				}
				return &object.String{Value: filepath.Join(parts...)}
			},
			HelpText: `join(*paths) - Join path components

Joins path components using the appropriate separator for the OS.`,
		},
		"exists": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil { return err }
				path, err := args[0].AsString()
				if err != nil {
					return err
				}

				// Security check
				if err := o.checkPathSecurity(path); err != nil {
					return err
				}

				_, statErr := os.Stat(path)
				return &object.Boolean{Value: statErr == nil}
			},
			HelpText: `exists(path) - Check if path exists

Returns True if the path exists, False otherwise.`,
		},
		"isfile": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil { return err }
				path, err := args[0].AsString()
				if err != nil {
					return err
				}

				// Security check
				if err := o.checkPathSecurity(path); err != nil {
					return err
				}

				info, statErr := os.Stat(path)
				if statErr != nil {
					return &object.Boolean{Value: false}
				}
				return &object.Boolean{Value: !info.IsDir()}
			},
			HelpText: `isfile(path) - Check if path is a file

Returns True if the path is a regular file, False otherwise.`,
		},
		"isdir": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil { return err }
				path, err := args[0].AsString()
				if err != nil {
					return err
				}

				// Security check
				if err := o.checkPathSecurity(path); err != nil {
					return err
				}

				info, statErr := os.Stat(path)
				if statErr != nil {
					return &object.Boolean{Value: false}
				}
				return &object.Boolean{Value: info.IsDir()}
			},
			HelpText: `isdir(path) - Check if path is a directory

Returns True if the path is a directory, False otherwise.`,
		},
		"basename": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil { return err }
				path, err := args[0].AsString()
				if err != nil {
					return err
				}
				return &object.String{Value: filepath.Base(path)}
			},
			HelpText: `basename(path) - Get the base name of a path

Returns the final component of a pathname.`,
		},
		"dirname": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil { return err }
				path, err := args[0].AsString()
				if err != nil {
					return err
				}
				return &object.String{Value: filepath.Dir(path)}
			},
			HelpText: `dirname(path) - Get the directory name of a path

Returns the directory component of a pathname.`,
		},
		"split": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil { return err }
				path, err := args[0].AsString()
				if err != nil {
					return err
				}
				dir, file := filepath.Split(path)
				// Remove trailing slash from dir (Python behavior) unless it's the root
				if len(dir) > 1 && (dir[len(dir)-1] == '/' || dir[len(dir)-1] == '\\') {
					dir = dir[:len(dir)-1]
				}
				return &object.Tuple{Elements: []object.Object{
					&object.String{Value: dir},
					&object.String{Value: file},
				}}
			},
			HelpText: `split(path) - Split path into (directory, filename) tuple`,
		},
		"splitext": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil { return err }
				path, err := args[0].AsString()
				if err != nil {
					return err
				}
				ext := filepath.Ext(path)
				root := path[:len(path)-len(ext)]
				return &object.Tuple{Elements: []object.Object{
					&object.String{Value: root},
					&object.String{Value: ext},
				}}
			},
			HelpText: `splitext(path) - Split path into (root, extension) tuple`,
		},
		"abspath": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil { return err }
				path, err := args[0].AsString()
				if err != nil {
					return err
				}
				absPath, fsErr := filepath.Abs(path)
				if fsErr != nil {
					return errors.NewError("cannot get absolute path: %s", fsErr.Error())
				}
				return &object.String{Value: absPath}
			},
			HelpText: `abspath(path) - Get absolute path`,
		},
		"normpath": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil { return err }
				path, err := args[0].AsString()
				if err != nil {
					return err
				}
				return &object.String{Value: filepath.Clean(path)}
			},
			HelpText: `normpath(path) - Normalize path

Normalizes path by collapsing redundant separators and up-level references.`,
		},
		"relpath": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) < 1 || len(args) > 2 {
					return errors.NewError("relpath() takes 1-2 arguments (%d given)", len(args))
				}
				path, err := args[0].AsString()
				if err != nil {
					return err
				}
				base := "."
				if len(args) == 2 {
					base, err = args[1].AsString()
					if err != nil {
						return err
					}
				}
				relPath, fsErr := filepath.Rel(base, path)
				if fsErr != nil {
					return errors.NewError("cannot get relative path: %s", fsErr.Error())
				}
				return &object.String{Value: relPath}
			},
			HelpText: `relpath(path[, start]) - Get relative path

Returns a relative filepath to path either from the current directory or from an optional start directory.`,
		},
		"isabs": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil { return err }
				path, err := args[0].AsString()
				if err != nil {
					return err
				}
				return &object.Boolean{Value: filepath.IsAbs(path)}
			},
			HelpText: `isabs(path) - Check if path is absolute

Returns True if the path is an absolute pathname.`,
		},
		"getsize": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil { return err }
				path, err := args[0].AsString()
				if err != nil {
					return err
				}

				// Security check
				if err := o.checkPathSecurity(path); err != nil {
					return err
				}

				info, fsErr := os.Stat(path)
				if fsErr != nil {
					return errors.NewError("cannot get file size: %s", fsErr.Error())
				}
				return object.NewInteger(info.Size())
			},
			HelpText: `getsize(path) - Get file size in bytes

Returns the size in bytes of the specified file.`,
		},
	}, nil, "Common pathname manipulations")
}

func getLineSep() string {
	if runtime.GOOS == "windows" {
		return "\r\n"
	}
	return "\n"
}

func getOSName() string {
	if runtime.GOOS == "windows" {
		return "nt"
	}
	return "posix"
}
