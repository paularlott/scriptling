// Package extlibs provides external libraries that need explicit registration
package extlibs

import (
	"context"
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
	return checkPathSecurity(o.config, path)
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
	registrar.RegisterLibrary(osLib)
	registrar.RegisterLibrary(osPathLib)
}

// NewOSLibrary creates a new OS library with the given configuration.
// The returned libraries are for "os" and "os.path".
// Prefer using RegisterOSLibrary which handles registration automatically.
func NewOSLibrary(config fssecurity.Config) (*object.Library, *object.Library) {
	// Normalize and validate allowed paths
	// IMPORTANT: nil means no restrictions, empty slice means deny all
	config = normalizeFileIOAllowedPaths(config)

	instance := &osLibraryInstance{config: config}

	osLib := instance.createOSLibrary()
	osPathLib := instance.createOSPathLibrary()

	return osLib, osPathLib
}

func (o *osLibraryInstance) createOSLibrary() *object.Library {
	// Build environ dict - this happens when the library is registered/imported
	// Environment variables are captured at that time
	environDict := &object.Dict{Pairs: make(map[string]object.DictPair)}
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			environDict.SetByString(parts[0], object.NewString(parts[1]))
		}
	}

	return object.NewLibrary(OSLibraryName, map[string]*object.Builtin{
		"getenv": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.RangeArgs(args, 1, 2); err != nil {
					return err
				}
				key, err := args[0].AsString()
				if err != nil {
					return err
				}
				value, found := os.LookupEnv(key)
				if !found {
					if len(args) == 2 {
						return args[1]
					}
					return &object.Null{}
				}
				return object.NewString(value)
			},
			HelpText: `getenv(key[, default]) - Get environment variable

Returns the value of the environment variable key if it exists, None if not set (or default if provided).`,
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
				return object.NewString(cwd)
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
					elements[i] = object.NewString(entry.Name())
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

				content, errObj := readFileBytes(ctx, o.config, path)
				if errObj != nil {
					return errObj
				}
				return object.NewString(string(content))
			},
			HelpText: `read_file(path) - Read entire file contents as string

Returns the contents of the file as a string.`,
		},
		"write_file": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.RangeArgs(args, 2, 3); err != nil {
					return err
				}
				path, err := args[0].AsString()
				if err != nil {
					return err
				}
				content, err := args[1].AsString()
				if err != nil {
					return errors.NewTypeError("STRING", args[1].Type().String())
				}
				mode, errObj := parseFileMode(args, kwargs, 2, 0644)
				if errObj != nil {
					return errObj
				}

				return writeFileBytes(ctx, o.config, path, []byte(content), mode)
			},
			HelpText: `write_file(path, content[, mode]) - Write content to file

Writes the string content to the file, creating or overwriting it.`,
		},
		"append_file": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 2); err != nil {
					return err
				}
				path, err := args[0].AsString()
				if err != nil {
					return err
				}
				content, err := args[1].AsString()
				if err != nil {
					return errors.NewTypeError("STRING", args[1].Type().String())
				}

				return appendFileBytes(ctx, o.config, path, []byte(content), 0644)
			},
			HelpText: `append_file(path, content) - Append content to file

Appends the string content to the file, creating it if it doesn't exist.`,
		},
		"remove": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}
				path, err := args[0].AsString()
				if err != nil {
					return err
				}

				return removePath(o.config, path, "file", false)
			},
			HelpText: `remove(path) - Remove a file

Removes the specified file.`,
		},
		"chmod": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.RangeArgs(args, 1, 2); err != nil {
					return err
				}
				path, err := args[0].AsString()
				if err != nil {
					return err
				}
				if len(args) == 1 && !kwargs.Has("mode") {
					return errors.NewError("chmod() missing required argument: mode")
				}
				mode, errObj := parseFileMode(args, kwargs, 1, 0)
				if errObj != nil {
					return errObj
				}

				return chmodPath(o.config, path, mode)
			},
			HelpText: `chmod(path, mode) - Change file or directory mode

Changes the permissions of the specified file or directory.`,
		},
		"mkdir": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.RangeArgs(args, 1, 2); err != nil {
					return err
				}
				path, err := args[0].AsString()
				if err != nil {
					return err
				}
				mode, errObj := parseFileMode(args, kwargs, 1, 0777)
				if errObj != nil {
					return errObj
				}

				return mkdirPath(o.config, path, mode, false, false)
			},
			HelpText: `mkdir(path[, mode]) - Create a directory

Creates a new directory with the specified path.`,
		},
		"makedirs": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.RangeArgs(args, 1, 2); err != nil {
					return err
				}
				path, err := args[0].AsString()
				if err != nil {
					return err
				}
				mode, errObj := parseFileMode(args, kwargs, 1, 0777)
				if errObj != nil {
					return errObj
				}
				existOk, errObj := kwargs.GetBool("exist_ok", false)
				if errObj != nil {
					return errObj
				}

				return mkdirPath(o.config, path, mode, true, existOk)
			},
			HelpText: `makedirs(path[, mode], exist_ok=False) - Create directories recursively

Creates a directory and all parent directories as needed.`,
		},
		"rmdir": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}
				path, err := args[0].AsString()
				if err != nil {
					return err
				}

				return removePath(o.config, path, "directory", false)
			},
			HelpText: `rmdir(path) - Remove a directory

Removes the specified empty directory.`,
		},
		"removedirs": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}
				path, err := args[0].AsString()
				if err != nil {
					return err
				}
				return removeDirs(o.config, path)
			},
			HelpText: `removedirs(name) - Remove empty directory and empty parent directories

Removes the leaf directory, then removes empty parent directories until a parent cannot be removed.`,
		},
		"rename": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 2); err != nil {
					return err
				}
				oldPath, err := args[0].AsString()
				if err != nil {
					return err
				}
				newPath, err := args[1].AsString()
				if err != nil {
					return err
				}

				return renamePath(o.config, oldPath, newPath)
			},
			HelpText: `rename(old, new) - Rename a file or directory

Renames the file or directory from old to new.`,
		},
	}, map[string]object.Object{
		"sep":      object.NewString(string(os.PathSeparator)),
		"linesep":  object.NewString(getLineSep()),
		"name":     object.NewString(getOSName()),
		"platform": object.NewString(runtime.GOOS),
		"environ":  environDict,
	}, "Operating system interface")
}

func (o *osLibraryInstance) createOSPathLibrary() *object.Library {
	return object.NewLibrary(OSPathLibraryName, map[string]*object.Builtin{
		"join": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) == 0 {
					return object.NewString("")
				}
				parts := make([]string, len(args))
				for i, arg := range args {
					s, err := arg.AsString()
					if err != nil {
						return err
					}
					parts[i] = s
				}
				return object.NewString(filepath.Join(parts...))
			},
			HelpText: `join(*paths) - Join path components

Joins path components using the appropriate separator for the OS.`,
		},
		"exists": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}
				path, err := args[0].AsString()
				if err != nil {
					return err
				}

				info, errObj := statPath(o.config, path, "")
				if errObj != nil {
					return errObj
				}
				return object.NewBoolean(info != nil)
			},
			HelpText: `exists(path) - Check if path exists

Returns True if the path exists, False otherwise.`,
		},
		"isfile": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}
				path, err := args[0].AsString()
				if err != nil {
					return err
				}

				info, errObj := statPath(o.config, path, "")
				if errObj != nil {
					return errObj
				}
				if info == nil {
					return object.NewBoolean(false)
				}
				return object.NewBoolean(!info.IsDir())
			},
			HelpText: `isfile(path) - Check if path is a file

Returns True if the path is a regular file, False otherwise.`,
		},
		"isdir": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}
				path, err := args[0].AsString()
				if err != nil {
					return err
				}

				info, errObj := statPath(o.config, path, "")
				if errObj != nil {
					return errObj
				}
				if info == nil {
					return object.NewBoolean(false)
				}
				return object.NewBoolean(info.IsDir())
			},
			HelpText: `isdir(path) - Check if path is a directory

Returns True if the path is a directory, False otherwise.`,
		},
		"basename": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}
				path, err := args[0].AsString()
				if err != nil {
					return err
				}
				return object.NewString(filepath.Base(path))
			},
			HelpText: `basename(path) - Get the base name of a path

Returns the final component of a pathname.`,
		},
		"dirname": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}
				path, err := args[0].AsString()
				if err != nil {
					return err
				}
				return object.NewString(filepath.Dir(path))
			},
			HelpText: `dirname(path) - Get the directory name of a path

Returns the directory component of a pathname.`,
		},
		"split": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}
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
					object.NewString(dir),
					object.NewString(file),
				}}
			},
			HelpText: `split(path) - Split path into (directory, filename) tuple`,
		},
		"splitext": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}
				path, err := args[0].AsString()
				if err != nil {
					return err
				}
				ext := filepath.Ext(path)
				root := path[:len(path)-len(ext)]
				return &object.Tuple{Elements: []object.Object{
					object.NewString(root),
					object.NewString(ext),
				}}
			},
			HelpText: `splitext(path) - Split path into (root, extension) tuple`,
		},
		"abspath": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}
				path, err := args[0].AsString()
				if err != nil {
					return err
				}
				absPath, fsErr := filepath.Abs(path)
				if fsErr != nil {
					return errors.NewError("cannot get absolute path: %s", fsErr.Error())
				}
				return object.NewString(absPath)
			},
			HelpText: `abspath(path) - Get absolute path`,
		},
		"normpath": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}
				path, err := args[0].AsString()
				if err != nil {
					return err
				}
				return object.NewString(filepath.Clean(path))
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
				return object.NewString(relPath)
			},
			HelpText: `relpath(path[, start]) - Get relative path

Returns a relative filepath to path either from the current directory or from an optional start directory.`,
		},
		"isabs": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}
				path, err := args[0].AsString()
				if err != nil {
					return err
				}
				return object.NewBoolean(filepath.IsAbs(path))
			},
			HelpText: `isabs(path) - Check if path is absolute

Returns True if the path is an absolute pathname.`,
		},
		"getsize": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}
				path, err := args[0].AsString()
				if err != nil {
					return err
				}

				info, errObj := statPath(o.config, path, "cannot get file size")
				if errObj != nil {
					return errObj
				}
				return object.NewInteger(info.Size())
			},
			HelpText: `getsize(path) - Get file size in bytes

Returns the size in bytes of the specified file.`,
		},
		"getmtime": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}
				path, err := args[0].AsString()
				if err != nil {
					return err
				}

				info, errObj := statPath(o.config, path, "cannot get file mtime")
				if errObj != nil {
					return errObj
				}
				return object.NewFloat(float64(info.ModTime().Unix()))
			},
			HelpText: `getmtime(path) - Get file modification time

Returns the time of last modification of path as a Unix timestamp (seconds since epoch).`,
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
	switch runtime.GOOS {
	case "windows":
		return "nt"
	default:
		return "posix"
	}
}
