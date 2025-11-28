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
	"github.com/paularlott/scriptling/object"
)

// OSLibraryConfig holds the configuration for the OS library
type OSLibraryConfig struct {
	// AllowedPaths is a list of absolute directory paths that file operations are restricted to.
	// If empty, all paths are allowed (no restrictions).
	// All paths must be absolute and will be cleaned/normalized.
	AllowedPaths []string
}

// osLibraryInstance holds the configured OS library instance
type osLibraryInstance struct {
	config OSLibraryConfig
}

// isPathAllowed checks if the given path is within the allowed paths.
// Returns true if the path is allowed, false otherwise.
//
// SECURITY CRITICAL: This function prevents path traversal attacks.
// It handles:
// - Relative paths (./foo, ../foo)
// - Path traversal (../../etc/passwd)
// - Symlink attacks (by evaluating the real path)
// - Prefix attacks (/allowed vs /allowed-other)
func (o *osLibraryInstance) isPathAllowed(path string) bool {
	// If no restrictions, allow all
	if len(o.config.AllowedPaths) == 0 {
		return true
	}

	// Get absolute path to prevent relative path attacks
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	// Clean the path to resolve any .. or . components
	absPath = filepath.Clean(absPath)

	// SECURITY: Evaluate symlinks to get the real path
	// This prevents symlink attacks where a symlink inside allowed dirs
	// points to a location outside allowed dirs.
	// Note: EvalSymlinks also cleans the path and makes it absolute
	realPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		// If the path doesn't exist yet (for write operations), we can't eval symlinks.
		// In this case, we check the parent directory exists and is allowed,
		// and that the final path (after cleaning) is still within allowed dirs.
		parentDir := filepath.Dir(absPath)
		realParent, parentErr := filepath.EvalSymlinks(parentDir)
		if parentErr != nil {
			// Parent doesn't exist either - check if path is within allowed dirs
			// using the cleaned absolute path
			realPath = absPath
		} else {
			// Parent exists, reconstruct the full path with real parent
			realPath = filepath.Join(realParent, filepath.Base(absPath))
		}
	}

	// Check if the real path starts with any of the allowed paths
	for _, allowedPath := range o.config.AllowedPaths {
		// Get real path of allowed directory too (in case it's a symlink)
		realAllowed, err := filepath.EvalSymlinks(allowedPath)
		if err != nil {
			// If allowed path doesn't exist, use it as-is (cleaned)
			realAllowed = filepath.Clean(allowedPath)
		}

		// Ensure allowed path ends with separator for proper prefix matching
		// This prevents /allowed matching /allowed-other
		allowedPrefix := realAllowed
		if !strings.HasSuffix(allowedPrefix, string(os.PathSeparator)) {
			allowedPrefix += string(os.PathSeparator)
		}

		// Check if path is exactly the allowed path or is under it
		if realPath == realAllowed || strings.HasPrefix(realPath+string(os.PathSeparator), allowedPrefix) {
			return true
		}
	}

	return false
}

// checkPathSecurity validates a path and returns an error if access is denied
func (o *osLibraryInstance) checkPathSecurity(path string) object.Object {
	if !o.isPathAllowed(path) {
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
//	// No restrictions - full filesystem access (DANGEROUS for untrusted code)
//	extlibs.RegisterOSLibrary(s, nil)
//
//	// Restricted to specific directories (SECURE)
//	extlibs.RegisterOSLibrary(s, []string{"/tmp/sandbox", "/home/user/data"})
func RegisterOSLibrary(registrar object.LibraryRegistrar, allowedPaths []string) {
	config := OSLibraryConfig{
		AllowedPaths: allowedPaths,
	}
	osLib, osPathLib := NewOSLibrary(config)
	registrar.RegisterLibrary("os", osLib)
	registrar.RegisterLibrary("os.path", osPathLib)
}

// NewOSLibrary creates a new OS library with the given configuration.
// The returned libraries are for "os" and "os.path".
// Prefer using RegisterOSLibrary which handles registration automatically.
func NewOSLibrary(config OSLibraryConfig) (*object.Library, *object.Library) {
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
			Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
				if len(args) < 1 || len(args) > 2 {
					return errors.NewArgumentError(len(args), 1)
				}
				key, ok := args[0].AsString()
				if !ok {
					return errors.NewTypeError("STRING", args[0].Type().String())
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
			Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
				if len(args) != 0 {
					return errors.NewArgumentError(len(args), 0)
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
			Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
				if len(args) != 0 {
					return errors.NewArgumentError(len(args), 0)
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
			Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
				path := "."
				if len(args) > 1 {
					return errors.NewArgumentError(len(args), 1)
				}
				if len(args) == 1 {
					var ok bool
					path, ok = args[0].AsString()
					if !ok {
						return errors.NewTypeError("STRING", args[0].Type().String())
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
			Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
				if len(args) != 1 {
					return errors.NewArgumentError(len(args), 1)
				}
				path, ok := args[0].AsString()
				if !ok {
					return errors.NewTypeError("STRING", args[0].Type().String())
				}

				// Security check
				if err := o.checkPathSecurity(path); err != nil {
					return err
				}

				content, err := os.ReadFile(path)
				if err != nil {
					return errors.NewError("cannot read file: %s", err.Error())
				}
				return &object.String{Value: string(content)}
			},
			HelpText: `read_file(path) - Read entire file contents as string

Returns the contents of the file as a string.`,
		},
		"write_file": {
			Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
				if len(args) != 2 {
					return errors.NewArgumentError(len(args), 2)
				}
				path, ok := args[0].AsString()
				if !ok {
					return errors.NewTypeError("STRING", args[0].Type().String())
				}
				content, ok := args[1].AsString()
				if !ok {
					return errors.NewTypeError("STRING", args[1].Type().String())
				}

				// Security check
				if err := o.checkPathSecurity(path); err != nil {
					return err
				}

				err := os.WriteFile(path, []byte(content), 0644)
				if err != nil {
					return errors.NewError("cannot write file: %s", err.Error())
				}
				return &object.Null{}
			},
			HelpText: `write_file(path, content) - Write content to file

Writes the string content to the file, creating or overwriting it.`,
		},
		"append_file": {
			Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
				if len(args) != 2 {
					return errors.NewArgumentError(len(args), 2)
				}
				path, ok := args[0].AsString()
				if !ok {
					return errors.NewTypeError("STRING", args[0].Type().String())
				}
				content, ok := args[1].AsString()
				if !ok {
					return errors.NewTypeError("STRING", args[1].Type().String())
				}

				// Security check
				if err := o.checkPathSecurity(path); err != nil {
					return err
				}

				f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
				if err != nil {
					return errors.NewError("cannot open file for append: %s", err.Error())
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
			Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
				if len(args) != 1 {
					return errors.NewArgumentError(len(args), 1)
				}
				path, ok := args[0].AsString()
				if !ok {
					return errors.NewTypeError("STRING", args[0].Type().String())
				}

				// Security check
				if err := o.checkPathSecurity(path); err != nil {
					return err
				}

				err := os.Remove(path)
				if err != nil {
					return errors.NewError("cannot remove file: %s", err.Error())
				}
				return &object.Null{}
			},
			HelpText: `remove(path) - Remove a file

Removes the specified file.`,
		},
		"mkdir": {
			Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
				if len(args) != 1 {
					return errors.NewArgumentError(len(args), 1)
				}
				path, ok := args[0].AsString()
				if !ok {
					return errors.NewTypeError("STRING", args[0].Type().String())
				}

				// Security check
				if err := o.checkPathSecurity(path); err != nil {
					return err
				}

				err := os.Mkdir(path, 0755)
				if err != nil {
					return errors.NewError("cannot create directory: %s", err.Error())
				}
				return &object.Null{}
			},
			HelpText: `mkdir(path) - Create a directory

Creates a new directory with the specified path.`,
		},
		"makedirs": {
			Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
				if len(args) != 1 {
					return errors.NewArgumentError(len(args), 1)
				}
				path, ok := args[0].AsString()
				if !ok {
					return errors.NewTypeError("STRING", args[0].Type().String())
				}

				// Security check
				if err := o.checkPathSecurity(path); err != nil {
					return err
				}

				err := os.MkdirAll(path, 0755)
				if err != nil {
					return errors.NewError("cannot create directories: %s", err.Error())
				}
				return &object.Null{}
			},
			HelpText: `makedirs(path) - Create directories recursively

Creates a directory and all parent directories as needed.`,
		},
		"rmdir": {
			Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
				if len(args) != 1 {
					return errors.NewArgumentError(len(args), 1)
				}
				path, ok := args[0].AsString()
				if !ok {
					return errors.NewTypeError("STRING", args[0].Type().String())
				}

				// Security check
				if err := o.checkPathSecurity(path); err != nil {
					return err
				}

				err := os.Remove(path)
				if err != nil {
					return errors.NewError("cannot remove directory: %s", err.Error())
				}
				return &object.Null{}
			},
			HelpText: `rmdir(path) - Remove a directory

Removes the specified empty directory.`,
		},
		"rename": {
			Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
				if len(args) != 2 {
					return errors.NewArgumentError(len(args), 2)
				}
				oldPath, ok := args[0].AsString()
				if !ok {
					return errors.NewTypeError("STRING", args[0].Type().String())
				}
				newPath, ok := args[1].AsString()
				if !ok {
					return errors.NewTypeError("STRING", args[1].Type().String())
				}

				// Security check both paths
				if err := o.checkPathSecurity(oldPath); err != nil {
					return err
				}
				if err := o.checkPathSecurity(newPath); err != nil {
					return err
				}

				err := os.Rename(oldPath, newPath)
				if err != nil {
					return errors.NewError("cannot rename: %s", err.Error())
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
			Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
				if len(args) == 0 {
					return &object.String{Value: ""}
				}
				parts := make([]string, len(args))
				for i, arg := range args {
					s, ok := arg.AsString()
					if !ok {
						return errors.NewTypeError("STRING", arg.Type().String())
					}
					parts[i] = s
				}
				return &object.String{Value: filepath.Join(parts...)}
			},
			HelpText: `join(*paths) - Join path components

Joins path components using the appropriate separator for the OS.`,
		},
		"exists": {
			Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
				if len(args) != 1 {
					return errors.NewArgumentError(len(args), 1)
				}
				path, ok := args[0].AsString()
				if !ok {
					return errors.NewTypeError("STRING", args[0].Type().String())
				}

				// Security check
				if err := o.checkPathSecurity(path); err != nil {
					return err
				}

				_, err := os.Stat(path)
				return &object.Boolean{Value: err == nil}
			},
			HelpText: `exists(path) - Check if path exists

Returns True if the path exists, False otherwise.`,
		},
		"isfile": {
			Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
				if len(args) != 1 {
					return errors.NewArgumentError(len(args), 1)
				}
				path, ok := args[0].AsString()
				if !ok {
					return errors.NewTypeError("STRING", args[0].Type().String())
				}

				// Security check
				if err := o.checkPathSecurity(path); err != nil {
					return err
				}

				info, err := os.Stat(path)
				if err != nil {
					return &object.Boolean{Value: false}
				}
				return &object.Boolean{Value: !info.IsDir()}
			},
			HelpText: `isfile(path) - Check if path is a file

Returns True if the path is a regular file, False otherwise.`,
		},
		"isdir": {
			Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
				if len(args) != 1 {
					return errors.NewArgumentError(len(args), 1)
				}
				path, ok := args[0].AsString()
				if !ok {
					return errors.NewTypeError("STRING", args[0].Type().String())
				}

				// Security check
				if err := o.checkPathSecurity(path); err != nil {
					return err
				}

				info, err := os.Stat(path)
				if err != nil {
					return &object.Boolean{Value: false}
				}
				return &object.Boolean{Value: info.IsDir()}
			},
			HelpText: `isdir(path) - Check if path is a directory

Returns True if the path is a directory, False otherwise.`,
		},
		"basename": {
			Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
				if len(args) != 1 {
					return errors.NewArgumentError(len(args), 1)
				}
				path, ok := args[0].AsString()
				if !ok {
					return errors.NewTypeError("STRING", args[0].Type().String())
				}
				return &object.String{Value: filepath.Base(path)}
			},
			HelpText: `basename(path) - Get the base name of a path

Returns the final component of a pathname.`,
		},
		"dirname": {
			Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
				if len(args) != 1 {
					return errors.NewArgumentError(len(args), 1)
				}
				path, ok := args[0].AsString()
				if !ok {
					return errors.NewTypeError("STRING", args[0].Type().String())
				}
				return &object.String{Value: filepath.Dir(path)}
			},
			HelpText: `dirname(path) - Get the directory name of a path

Returns the directory component of a pathname.`,
		},
		"split": {
			Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
				if len(args) != 1 {
					return errors.NewArgumentError(len(args), 1)
				}
				path, ok := args[0].AsString()
				if !ok {
					return errors.NewTypeError("STRING", args[0].Type().String())
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
			Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
				if len(args) != 1 {
					return errors.NewArgumentError(len(args), 1)
				}
				path, ok := args[0].AsString()
				if !ok {
					return errors.NewTypeError("STRING", args[0].Type().String())
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
			Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
				if len(args) != 1 {
					return errors.NewArgumentError(len(args), 1)
				}
				path, ok := args[0].AsString()
				if !ok {
					return errors.NewTypeError("STRING", args[0].Type().String())
				}
				absPath, err := filepath.Abs(path)
				if err != nil {
					return errors.NewError("cannot get absolute path: %s", err.Error())
				}
				return &object.String{Value: absPath}
			},
			HelpText: `abspath(path) - Get absolute path`,
		},
		"normpath": {
			Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
				if len(args) != 1 {
					return errors.NewArgumentError(len(args), 1)
				}
				path, ok := args[0].AsString()
				if !ok {
					return errors.NewTypeError("STRING", args[0].Type().String())
				}
				return &object.String{Value: filepath.Clean(path)}
			},
			HelpText: `normpath(path) - Normalize path

Normalizes path by collapsing redundant separators and up-level references.`,
		},
		"relpath": {
			Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
				if len(args) < 1 || len(args) > 2 {
					return errors.NewError("relpath() takes 1-2 arguments (%d given)", len(args))
				}
				path, ok := args[0].AsString()
				if !ok {
					return errors.NewTypeError("STRING", args[0].Type().String())
				}
				base := "."
				if len(args) == 2 {
					base, ok = args[1].AsString()
					if !ok {
						return errors.NewTypeError("STRING", args[1].Type().String())
					}
				}
				relPath, err := filepath.Rel(base, path)
				if err != nil {
					return errors.NewError("cannot get relative path: %s", err.Error())
				}
				return &object.String{Value: relPath}
			},
			HelpText: `relpath(path[, start]) - Get relative path

Returns a relative filepath to path either from the current directory or from an optional start directory.`,
		},
		"isabs": {
			Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
				if len(args) != 1 {
					return errors.NewArgumentError(len(args), 1)
				}
				path, ok := args[0].AsString()
				if !ok {
					return errors.NewTypeError("STRING", args[0].Type().String())
				}
				return &object.Boolean{Value: filepath.IsAbs(path)}
			},
			HelpText: `isabs(path) - Check if path is absolute

Returns True if the path is an absolute pathname.`,
		},
		"getsize": {
			Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
				if len(args) != 1 {
					return errors.NewArgumentError(len(args), 1)
				}
				path, ok := args[0].AsString()
				if !ok {
					return errors.NewTypeError("STRING", args[0].Type().String())
				}

				// Security check
				if err := o.checkPathSecurity(path); err != nil {
					return err
				}

				info, err := os.Stat(path)
				if err != nil {
					return errors.NewError("cannot get file size: %s", err.Error())
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
