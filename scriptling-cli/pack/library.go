package pack

import (
	"context"
	"io/fs"
	"path"
	"strings"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/plugin"
)

const PackageLibraryName = "scriptling.package"

// NewPackageLibrary builds the scriptling.package library bound to the given
// loader. Exposed so embedders and tests can register it on a custom
// registrar or inspect it directly.
func NewPackageLibrary(loader *Loader) *object.Library {
	funcs := buildPackageLibraryFuncs(loader)
	return object.NewLibrary(PackageLibraryName, funcs, nil,
		"Read-only access to files inside loaded packages")
}

// RegisterPackageLibrary registers the scriptling.package library on the given
// Scriptling instance. Convenience wrapper around NewPackageLibrary.
func RegisterPackageLibrary(p interface{ RegisterLibrary(*object.Library) }, loader *Loader) {
	if loader == nil {
		return
	}
	p.RegisterLibrary(NewPackageLibrary(loader))
}

// buildPackageLibraryFuncs returns the function map for the scriptling.package
// library, bound to the given loader.
func buildPackageLibraryFuncs(loader *Loader) map[string]*object.Builtin {
	return map[string]*object.Builtin{
		"names": {
			Fn: func(_ context.Context, _ object.Kwargs, _ ...object.Object) object.Object {
				names := loader.BundleNames()
				elems := make([]object.Object, len(names))
				for i, n := range names {
					elems[i] = object.NewString(n)
				}
				return &object.List{Elements: elems}
			},
			HelpText: `names() - List all loaded package names

Returns:
  list of strings: the manifest name of each loaded package`,
		},

		"version": {
			Fn: func(_ context.Context, _ object.Kwargs, args ...object.Object) object.Object {
				if err := errors.MinArgs(args, 1); err != nil {
					return err
				}
				name, err := args[0].AsString()
				if err != nil {
					return err
				}
				b := loader.BundleByName(name)
				if b == nil {
					return errors.NewError("unknown package: %s", name)
				}
				return object.NewString(b.Manifest.Version)
			},
			HelpText: `version(name) - Get the version of a loaded package

Parameters:
  name (str): Package name from manifest.toml

Returns:
  str: Version string (e.g. "1.0.0")`,
		},

		"exists": {
			Fn: func(_ context.Context, _ object.Kwargs, args ...object.Object) object.Object {
				if err := errors.MinArgs(args, 1); err != nil {
					return err
				}
				name, err := args[0].AsString()
				if err != nil {
					return err
				}
				return object.NewBoolean(loader.BundleByName(name) != nil)
			},
			HelpText: `exists(name) - Check if a package is loaded

Parameters:
  name (str): Package name from manifest.toml

Returns:
  bool: True if the package is loaded`,
		},

		"file_exists": {
			Fn: func(_ context.Context, _ object.Kwargs, args ...object.Object) object.Object {
				if err := errors.MinArgs(args, 2); err != nil {
					return err
				}
				pkgName, err := args[0].AsString()
				if err != nil {
					return err
				}
				filePath, err := args[1].AsString()
				if err != nil {
					return err
				}
				b := loader.BundleByName(pkgName)
				if b == nil {
					return object.NewBoolean(false)
				}
				clean := path.Clean(strings.TrimPrefix(filePath, "/"))
				_, statErr := fs.Stat(b.FS(), clean)
				return object.NewBoolean(statErr == nil)
			},
			HelpText: `file_exists(name, path) - Check if a file exists in a package

Parameters:
  name (str): Package name from manifest.toml
  path (str): File path relative to the package root

Returns:
  bool: True if the file exists`,
		},

		"read_file": {
			Fn: func(_ context.Context, _ object.Kwargs, args ...object.Object) object.Object {
				if err := errors.MinArgs(args, 2); err != nil {
					return err
				}
				pkgName, err := args[0].AsString()
				if err != nil {
					return err
				}
				filePath, err := args[1].AsString()
				if err != nil {
					return err
				}
				b := loader.BundleByName(pkgName)
				if b == nil {
					return errors.NewError("unknown package: %s", pkgName)
				}
				clean := path.Clean(strings.TrimPrefix(filePath, "/"))
				data, readErr := fs.ReadFile(b.FS(), clean)
				if readErr != nil {
					return errors.NewError("file not found in package %s: %s", pkgName, clean)
				}
				return object.NewString(string(data))
			},
			HelpText: `read_file(name, path) - Read a file from a package

Parameters:
  name (str): Package name from manifest.toml
  path (str): File path relative to the package root

Returns:
  str: File contents as a string. Use read_bytes() for binary files.

Example:
  import scriptling.package as package
  spec = package.read_file("myapp", "data/spec.md")`,
		},
		"read_bytes": {
			Fn: func(_ context.Context, _ object.Kwargs, args ...object.Object) object.Object {
				if err := errors.MinArgs(args, 2); err != nil {
					return err
				}
				pkgName, err := args[0].AsString()
				if err != nil {
					return err
				}
				filePath, err := args[1].AsString()
				if err != nil {
					return err
				}
				b := loader.BundleByName(pkgName)
				if b == nil {
					return errors.NewError("unknown package: %s", pkgName)
				}
				clean := path.Clean(strings.TrimPrefix(filePath, "/"))
				data, readErr := fs.ReadFile(b.FS(), clean)
				if readErr != nil {
					return errors.NewError("file not found in package %s: %s", pkgName, clean)
				}
				return object.NewBytes(data)
			},
			HelpText: `read_bytes(name, path) - Read a file from a package as bytes

Parameters:
  name (str): Package name from manifest.toml
  path (str): File path relative to the package root

Returns:
  bytes: File contents as a Bytes value.

Example:
  import scriptling.package as package
  import msgpack
  data = msgpack.unpackb(package.read_bytes("myapp", "data/payload.msgpack"))`,
		},

		"list": {
			Fn: func(_ context.Context, _ object.Kwargs, args ...object.Object) object.Object {
				if err := errors.MinArgs(args, 2); err != nil {
					return err
				}
				pkgName, err := args[0].AsString()
				if err != nil {
					return err
				}
				dirPath, err := args[1].AsString()
				if err != nil {
					return err
				}
				b := loader.BundleByName(pkgName)
				if b == nil {
					return errors.NewError("unknown package: %s", pkgName)
				}
				clean := path.Clean(strings.TrimPrefix(dirPath, "/"))
				if clean == "." {
					clean = "."
				}
				entries, dirErr := fs.ReadDir(b.FS(), clean)
				if dirErr != nil {
					return errors.NewError("directory not found in package %s: %s", pkgName, clean)
				}
				var names []object.Object
				for _, e := range entries {
					suffix := ""
					if e.IsDir() {
						suffix = "/"
					}
					names = append(names, object.NewString(e.Name()+suffix))
				}
				if names == nil {
					names = []object.Object{}
				}
				return &object.List{Elements: names}
			},
			HelpText: `list(name, path) - List files in a directory within a package

Parameters:
  name (str): Package name from manifest.toml
  path (str): Directory path relative to the package root (use "" or "." for root)

Returns:
  list of str: File and directory names (directories end with /)`,
		},

		"glob": {
			Fn: func(_ context.Context, _ object.Kwargs, args ...object.Object) object.Object {
				if err := errors.MinArgs(args, 2); err != nil {
					return err
				}
				pkgName, err := args[0].AsString()
				if err != nil {
					return err
				}
				pattern, err := args[1].AsString()
				if err != nil {
					return err
				}
				b := loader.BundleByName(pkgName)
				if b == nil {
					return errors.NewError("unknown package: %s", pkgName)
				}
				var matches []object.Object
				walkErr := fs.WalkDir(b.FS(), ".", func(p string, d fs.DirEntry, err error) error {
					if err != nil {
						return err
					}
					if d.IsDir() {
						return nil
					}
					if plugin.MatchGlob(pattern, p) {
						matches = append(matches, object.NewString(p))
					}
					return nil
				})
				if walkErr != nil {
					return errors.NewError("cannot walk package %s: %v", pkgName, walkErr)
				}
				if matches == nil {
					matches = []object.Object{}
				}
				return &object.List{Elements: matches}
			},
			HelpText: `glob(name, pattern) - Find files matching a glob pattern in a package

Parameters:
  name (str): Package name from manifest.toml
  pattern (str): Glob pattern; * and ? stay within a segment, ** crosses
    segments (so "**/*.md" also matches a file at the package root)

Returns:
  list of str: Matching file paths relative to the package root

Example:
  import scriptling.package as package
  py_files = package.glob("myapp", "**/*.py")`,
		},
	}
}

// The glob pattern language is plugin.MatchGlob, the same one fetch.glob
// speaks: * and ? stay within a path segment, ** crosses any number of
// segments (including none), so "**/*.md" matches root files too.
