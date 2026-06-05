package file

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/paularlott/scriptling/object"
)

const (
	LibraryName = "scriptling.provision.file"
	LibraryDesc = "File provisioning utilities for creating and updating files with correct permissions"
)

var (
	library     *object.Library
	libraryOnce sync.Once
)

const (
	defaultFileMode = 0o644
	defaultDirMode  = 0o755

	StatusCreated   = "created"
	StatusUpdated   = "updated"
	StatusUnchanged = "unchanged"
	StatusRemoved   = "removed"
	StatusAbsent    = "absent"
	StatusExists    = "exists"
)

func Register(registrar interface{ RegisterLibrary(*object.Library) }) {
	libraryOnce.Do(func() {
		library = buildLibrary()
	})
	registrar.RegisterLibrary(library)
}

func buildLibrary() *object.Library {
	return object.NewLibrary(LibraryName, map[string]*object.Builtin{
		"ensure": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) != 2 {
					return &object.Error{Message: fmt.Sprintf("ensure expected 2 arguments, got %d", len(args))}
				}

				path, coerceErr := args[0].CoerceString()
				if coerceErr != nil {
					return &object.Error{Message: "ensure: path must be a string"}
				}

				content, coerceErr := args[1].CoerceString()
				if coerceErr != nil {
					return &object.Error{Message: "ensure: content must be a string"}
				}

				mode := int(kwargs.MustGetInt("mode", defaultFileMode))

				path = expandPath(path)

				existing, err := os.ReadFile(path)
			if err == nil && bytes.Equal(existing, []byte(content)) {
				return object.NewString(StatusUnchanged)
			}

				fileExisted := err == nil

				dir := filepath.Dir(path)
				if dir != "" && dir != "." {
					if err := os.MkdirAll(dir, defaultDirMode); err != nil {
						return &object.Error{Message: fmt.Sprintf("ensure: failed to create directory %s: %s", dir, err.Error())}
					}
				}

				if err := os.WriteFile(path, []byte(content), os.FileMode(mode)); err != nil {
					return &object.Error{Message: fmt.Sprintf("ensure: failed to write %s: %s", path, err.Error())}
				}

				if err := os.Chmod(path, os.FileMode(mode)); err != nil {
					return &object.Error{Message: fmt.Sprintf("ensure: failed to set mode on %s: %s", path, err.Error())}
				}

			if fileExisted {
				return object.NewString(StatusUpdated)
			}
			return object.NewString(StatusCreated)
			},
			HelpText: `ensure(path, content, mode=0o644) - Ensure a file exists with the given content

Creates parent directories if needed. If the file already exists with the
same content, it is left unchanged. Otherwise the file is written with the
specified mode.

Parameters:
  path (str): Path to the file (supports ~ expansion)
  content (str): File contents
  mode (int): File permission mode (default 0o644)

Returns:
  str: "created" if the file was newly written,
       "updated" if the file existed but content differed,
       "unchanged" if the file existed with identical content

Example:
  import scriptling.provision.file as file
  status = file.ensure("~/.gitconfig", "[user]\\nname = Jane\\n", mode=0o600)
  if status == file.CREATED:
      print("File created")
  elif status == file.UPDATED:
      print("File updated")`,
		},
		"absent": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) != 1 {
					return &object.Error{Message: fmt.Sprintf("absent expected 1 argument, got %d", len(args))}
				}

				path, coerceErr := args[0].CoerceString()
				if coerceErr != nil {
					return &object.Error{Message: "absent: path must be a string"}
				}

				path = expandPath(path)

				if _, err := os.Stat(path); os.IsNotExist(err) {
					return object.NewString(StatusAbsent)
				}

				if err := os.Remove(path); err != nil {
					return &object.Error{Message: fmt.Sprintf("absent: failed to remove %s: %s", path, err.Error())}
				}

				return object.NewString(StatusRemoved)
			},
			HelpText: `absent(path) - Ensure a file does not exist

Removes the file if it exists. Does nothing if the file is already absent.

Parameters:
  path (str): Path to the file (supports ~ expansion)

Returns:
  str: file.REMOVED if the file was deleted,
       file.ABSENT if the file did not exist

Example:
  import scriptling.provision.file as file
  status = file.absent("~/.old_config")
  if status == file.REMOVED:
      print("File removed")`,
		},
		"ensure_directory": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) != 1 {
					return &object.Error{Message: fmt.Sprintf("ensure_directory expected 1 argument, got %d", len(args))}
				}

				path, coerceErr := args[0].CoerceString()
				if coerceErr != nil {
					return &object.Error{Message: "ensure_directory: path must be a string"}
				}

				mode := int(kwargs.MustGetInt("mode", defaultDirMode))

				path = expandPath(path)

				info, err := os.Stat(path)
				if err == nil {
					if !info.IsDir() {
						return &object.Error{Message: fmt.Sprintf("ensure_directory: %s exists but is not a directory", path)}
					}
					return object.NewString(StatusExists)
				}

				if err := os.MkdirAll(path, os.FileMode(mode)); err != nil {
					return &object.Error{Message: fmt.Sprintf("ensure_directory: failed to create %s: %s", path, err.Error())}
				}

				return object.NewString(StatusCreated)
			},
			HelpText: `ensure_directory(path, mode=0o755) - Ensure a directory exists

Creates the directory and all parent directories if needed.

Parameters:
  path (str): Path to the directory (supports ~ expansion)
  mode (int): Directory permission mode (default 0o755)

Returns:
  str: file.CREATED if the directory was newly created,
       file.EXISTS if the directory already existed

Example:
  import scriptling.provision.file as file
  status = file.ensure_directory("~/.config/myapp", mode=0o700)
  if status == file.CREATED:
      print("Directory created")`,
		},
		"absent_directory": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) != 1 {
					return &object.Error{Message: fmt.Sprintf("absent_directory expected 1 argument, got %d", len(args))}
				}

				path, coerceErr := args[0].CoerceString()
				if coerceErr != nil {
					return &object.Error{Message: "absent_directory: path must be a string"}
				}

				path = expandPath(path)

				info, err := os.Stat(path)
				if os.IsNotExist(err) {
					return object.NewString(StatusAbsent)
				}
				if err != nil {
					return &object.Error{Message: fmt.Sprintf("absent_directory: %s", err.Error())}
				}
				if !info.IsDir() {
					return &object.Error{Message: fmt.Sprintf("absent_directory: %s is not a directory", path)}
				}

				if err := os.Remove(path); err != nil {
					return &object.Error{Message: fmt.Sprintf("absent_directory: failed to remove %s: %s", path, err.Error())}
				}

				return object.NewString(StatusRemoved)
			},
			HelpText: `absent_directory(path) - Ensure an empty directory does not exist

Removes the directory if it exists and is empty. Returns an error if the
directory is not empty.

Parameters:
  path (str): Path to the directory (supports ~ expansion)

Returns:
  str: file.REMOVED if the directory was deleted,
       file.ABSENT if the directory did not exist

Example:
  import scriptling.provision.file as file
  status = file.absent_directory("~/old/empty/dir")
  if status == file.REMOVED:
      print("Directory removed")`,
		},
	}, map[string]object.Object{
		"CREATED":   object.NewString(StatusCreated),
		"UPDATED":   object.NewString(StatusUpdated),
		"UNCHANGED": object.NewString(StatusUnchanged),
		"REMOVED":   object.NewString(StatusRemoved),
		"ABSENT":    object.NewString(StatusAbsent),
		"EXISTS":    object.NewString(StatusExists),
	}, LibraryDesc)
}

func expandPath(path string) string {
	if path == "" {
		return path
	}
	if path[0] == '~' {
		home, err := os.UserHomeDir()
		if err == nil {
			if len(path) == 1 || path[1] == '/' {
				return filepath.Join(home, path[1:])
			}
		}
	}
	return path
}
