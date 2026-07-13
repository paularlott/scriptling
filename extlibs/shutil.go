// Package extlibs provides external libraries that need explicit registration
package extlibs

import (
	"context"
	"os"
	"path/filepath"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/extlibs/fssecurity"
	"github.com/paularlott/scriptling/object"
)

// shutilLibraryInstance holds the configured shutil library instance.
type shutilLibraryInstance struct {
	config fssecurity.Config
}

// RegisterShutilLibrary registers the shutil library with a Scriptling instance.
func RegisterShutilLibrary(registrar object.LibraryRegistrar, allowedPaths []string) {
	registrar.RegisterLibrary(NewShutilLibrary(fssecurity.Config{AllowedPaths: allowedPaths}))
}

// NewShutilLibrary creates a new shutil library with the given configuration.
func NewShutilLibrary(config fssecurity.Config) *object.Library {
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
	inst := &shutilLibraryInstance{config: config}
	return inst.createLibrary()
}

func (s *shutilLibraryInstance) createLibrary() *object.Library {
	return object.NewLibrary(ShutilLibraryName, map[string]*object.Builtin{
		"copy": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				return s.fnCopy(ctx, args)
			},
			HelpText: `copy(src, dst) - Copy a file or directory tree

Copies src to dst, preserving file modes. If src is a directory the entire
tree is copied recursively. Returns the destination path.

Both src and dst must be within the allowed paths.`,
		},
		"copy2": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				return s.fnCopy(ctx, args)
			},
			HelpText: `copy2(src, dst) - Copy a file with metadata

Identical to copy() — file mode is always preserved. Provided for Python
compatibility. Returns the destination path.`,
		},
		"copytree": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 2); err != nil {
					return err
				}
				src, err := args[0].AsString()
				if err != nil {
					return err
				}
				dst, err := args[1].AsString()
				if err != nil {
					return err
				}
				if err := checkPathSecurity(s.config, src); err != nil {
					return err
				}
				if err := checkPathSecurity(s.config, dst); err != nil {
					return err
				}

				info, statErr := os.Stat(src)
				if statErr != nil {
					return errors.NewError("cannot copy: %s", statErr.Error())
				}
				if !info.IsDir() {
					return errors.NewError("copytree: src must be a directory: %s", src)
				}

				var result object.Object
				object.RunBlocking(ctx, func() {
					result = copyDir(s.config, src, dst, info.Mode())
				})
				if isObjectError(result) {
					return result
				}
				return object.NewString(dst)
			},
			HelpText: `copytree(src, dst) - Recursively copy a directory tree

Copies the entire directory tree rooted at src to dst. File modes are
preserved. Returns the destination path.`,
		},
		"rmtree": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}
				path, err := args[0].AsString()
				if err != nil {
					return err
				}
				if err := checkPathSecurity(s.config, path); err != nil {
					return err
				}

				var rmErr error
				object.RunBlocking(ctx, func() {
					rmErr = os.RemoveAll(path)
				})
				if rmErr != nil {
					return errors.NewError("cannot remove directory tree: %s", rmErr.Error())
				}
				return &object.Null{}
			},
			HelpText: `rmtree(path) - Recursively delete a directory tree

Deletes the directory at path and all of its contents (files, subdirectories,
symlinks). Unlike os.removedirs, the directory does not need to be empty.`,
		},
		"move": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 2); err != nil {
					return err
				}
				src, err := args[0].AsString()
				if err != nil {
					return err
				}
				dst, err := args[1].AsString()
				if err != nil {
					return err
				}
				if err := checkPathSecurity(s.config, src); err != nil {
					return err
				}
				if err := checkPathSecurity(s.config, dst); err != nil {
					return err
				}

				var mvErr error
				object.RunBlocking(ctx, func() {
					mvErr = os.Rename(src, dst)
				})
				if mvErr != nil {
					return errors.NewError("cannot move: %s", mvErr.Error())
				}
				return object.NewString(dst)
			},
			HelpText: `move(src, dst) - Move or rename a file or directory

Atomically moves src to dst (same as os.rename). Returns the destination
path.`,
		},
		"disk_usage": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}
				path, err := args[0].AsString()
				if err != nil {
					return err
				}
				if err := checkPathSecurity(s.config, path); err != nil {
					return err
				}

				total, used, free, duErr := diskUsageStat(path)
				if duErr != nil {
					return errors.NewError("cannot get disk usage: %s", duErr.Error())
				}

				d := &object.Dict{Pairs: make(map[string]object.DictPair)}
				d.SetByString("total", object.NewInteger(total))
				d.SetByString("used", object.NewInteger(used))
				d.SetByString("free", object.NewInteger(free))
				return d
			},
			HelpText: `disk_usage(path) - Return disk usage statistics

Returns a dict with total, used, and free space (in bytes) for the file
system containing the given path:

  {"total": int, "used": int, "free": int}`,
		},
	}, nil, "High-level file and directory operations")
}

// fnCopy is the shared implementation for copy and copy2.
func (s *shutilLibraryInstance) fnCopy(ctx context.Context, args []object.Object) object.Object {
	if err := errors.ExactArgs(args, 2); err != nil {
		return err
	}
	src, err := args[0].AsString()
	if err != nil {
		return err
	}
	dst, err := args[1].AsString()
	if err != nil {
		return err
	}

	var result object.Object
	object.RunBlocking(ctx, func() {
		result = copyPath(s.config, src, dst)
	})
	if isObjectError(result) {
		return result
	}
	return object.NewString(dst)
}
