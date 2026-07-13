// Package extlibs provides external libraries that need explicit registration
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

// tempfileLibraryInstance holds the configured tempfile library instance.
type tempfileLibraryInstance struct {
	config fssecurity.Config
}

// RegisterTempfileLibrary registers the tempfile library with a Scriptling instance.
func RegisterTempfileLibrary(registrar object.LibraryRegistrar, allowedPaths []string) {
	registrar.RegisterLibrary(NewTempfileLibrary(fssecurity.Config{AllowedPaths: allowedPaths}))
}

// NewTempfileLibrary creates a new tempfile library with the given configuration.
func NewTempfileLibrary(config fssecurity.Config) *object.Library {
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
	inst := &tempfileLibraryInstance{config: config}
	return inst.createLibrary()
}

func (t *tempfileLibraryInstance) createLibrary() *object.Library {
	return object.NewLibrary(TempfileLibraryName, map[string]*object.Builtin{
		"mkstemp": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				suffix, prefix, dir, oErr := parseTempfileKwargs(kwargs)
				if oErr != nil {
					return oErr
				}
				path, oErr := t.makeTempFile(ctx, suffix, prefix, dir)
				if oErr != nil {
					return oErr
				}
				return object.NewString(path)
			},
			HelpText: `mkstemp(suffix="", prefix="tmp", dir=None) - Create a temporary file

Creates and returns the path to a new temporary file. The file is created with
restrictive permissions (0600) and is readable/writable only by the owner.
Unlike Python's mkstemp, which returns (fd, path), this returns just the path
(the file is created and immediately closed).

Parameters:
  suffix  Suffix for the temporary file name (default "")
  prefix  Prefix for the temporary file name (default "tmp")
  dir     Directory to create the file in (default: system temp directory)`,
		},
		"mkdtemp": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				suffix, prefix, dir, oErr := parseTempfileKwargs(kwargs)
				if oErr != nil {
					return oErr
				}
				path, oErr := t.makeTempDir(ctx, suffix, prefix, dir)
				if oErr != nil {
					return oErr
				}
				return object.NewString(path)
			},
			HelpText: `mkdtemp(suffix="", prefix="tmp", dir=None) - Create a temporary directory

Creates and returns the path to a new temporary directory. The directory is
created with restrictive permissions (0700).

Parameters:
  suffix  Suffix for the temporary directory name (default "")
  prefix  Prefix for the temporary directory name (default "tmp")
  dir     Parent directory (default: system temp directory)`,
		},
		"gettempdir": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 0); err != nil {
					return err
				}
				return object.NewString(t.resolvedTempDir())
			},
			HelpText: `gettempdir() - Return the default temporary directory`,
		},
		"gettempprefix": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 0); err != nil {
					return err
				}
				return object.NewString("tmp")
			},
			HelpText: `gettempprefix() - Return the default temporary file name prefix ("tmp")`,
		},
	}, nil, "Temporary file and directory creation")
}

// resolvedTempDir returns the system temp dir if it is within the allowed
// paths, otherwise the first allowed path (or "." as a last resort).
func (t *tempfileLibraryInstance) resolvedTempDir() string {
	sys := os.TempDir()
	if t.config.IsPathAllowed(sys) {
		return sys
	}
	if t.config.AllowedPaths != nil && len(t.config.AllowedPaths) > 0 {
		return t.config.AllowedPaths[0]
	}
	return "."
}

func (t *tempfileLibraryInstance) makeTempFile(ctx context.Context, suffix, prefix, dir string) (string, object.Object) {
	if dir == "" {
		dir = t.resolvedTempDir()
	}
	if err := checkPathSecurity(t.config, dir); err != nil {
		return "", err
	}

	pattern := sanitizeTempPattern(prefix) + "*" + sanitizeTempPattern(suffix)
	var path string
	var createErr error
	object.RunBlocking(ctx, func() {
		f, err := os.CreateTemp(dir, pattern)
		if err != nil {
			createErr = err
			return
		}
		path = f.Name()
		f.Close()
	})
	if createErr != nil {
		return "", errors.NewError("cannot create temporary file: %s", createErr.Error())
	}
	if !t.config.IsPathAllowed(path) {
		os.Remove(path)
		return "", errors.NewPermissionError("access denied: temporary file '%s' is outside allowed directories", path)
	}
	return path, nil
}

func (t *tempfileLibraryInstance) makeTempDir(ctx context.Context, suffix, prefix, dir string) (string, object.Object) {
	if dir == "" {
		dir = t.resolvedTempDir()
	}
	if err := checkPathSecurity(t.config, dir); err != nil {
		return "", err
	}

	pattern := sanitizeTempPattern(prefix) + "*" + sanitizeTempPattern(suffix)
	var path string
	var mkdirErr error
	object.RunBlocking(ctx, func() {
		p, err := os.MkdirTemp(dir, pattern)
		if err != nil {
			mkdirErr = err
			return
		}
		path = p
	})
	if mkdirErr != nil {
		return "", errors.NewError("cannot create temporary directory: %s", mkdirErr.Error())
	}
	if !t.config.IsPathAllowed(path) {
		os.RemoveAll(path)
		return "", errors.NewPermissionError("access denied: temporary directory '%s' is outside allowed directories", path)
	}
	return path, nil
}

// parseTempfileKwargs reads the suffix/prefix/dir keyword arguments shared by
// mkstemp and mkdtemp.
func parseTempfileKwargs(kwargs object.Kwargs) (suffix, prefix, dir string, errObj object.Object) {
	if v := kwargs.Get("suffix"); v != nil {
		s, err := v.AsString()
		if err != nil {
			return "", "", "", err
		}
		suffix = s
	}
	if v := kwargs.Get("prefix"); v != nil {
		s, err := v.AsString()
		if err != nil {
			return "", "", "", err
		}
		prefix = s
	}
	if v := kwargs.Get("dir"); v != nil {
		if _, isNull := v.(*object.Null); isNull {
			return suffix, prefix, "", nil
		}
		s, err := v.AsString()
		if err != nil {
			return "", "", "", err
		}
		dir = s
	}
	if prefix == "" {
		prefix = "tmp"
	}
	return suffix, prefix, dir, nil
}

// sanitizeTempPattern removes "*" from prefix/suffix so os.CreateTemp/MkdirTemp
// use the * we insert as the sole random placeholder.
func sanitizeTempPattern(s string) string {
	return strings.ReplaceAll(s, "*", "")
}
