package pack

import (
	"context"
	"io/fs"
	"path"
	"strings"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

const PackageLibraryName = "scriptling.package"

// RegisterPackageLibrary registers the scriptling.package library on the given
// Scriptling instance, providing read-only access to files inside loaded
// packages (app bundles and library bundles). Every function takes the package
// name (from manifest.toml) as its first argument.
func RegisterPackageLibrary(p interface{ RegisterLibrary(*object.Library) }, loader *Loader) {
	if loader == nil {
		return
	}

	funcs := map[string]*object.Builtin{
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
  str: File contents as a string

Example:
  import scriptling.package as package
  spec = package.read_file("myapp", "data/spec.md")`,
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
				_ = fs.WalkDir(b.FS(), ".", func(p string, d fs.DirEntry, err error) error {
					if err != nil || d.IsDir() {
						return nil
					}
					if globMatch(pattern, p) {
						matches = append(matches, object.NewString(p))
					}
					return nil
				})
				if matches == nil {
					matches = []object.Object{}
				}
				return &object.List{Elements: matches}
			},
			HelpText: `glob(name, pattern) - Find files matching a glob pattern in a package

Parameters:
  name (str): Package name from manifest.toml
  pattern (str): Glob pattern (* and ? wildcards, ** for recursive)

Returns:
  list of str: Matching file paths relative to the package root

Example:
  import scriptling.package as package
  py_files = package.glob("myapp", "**/*.py")`,
		},
	}

	lib := object.NewLibrary(PackageLibraryName, funcs, nil,
		"Read-only access to files inside loaded packages")
	p.RegisterLibrary(lib)
}

// globMatch matches a glob pattern against a slash-separated path.
// Supports * (within a segment), ? (single char), and ** (any number of
// path segments).
func globMatch(pattern, p string) bool {
	if !strings.Contains(pattern, "**") {
		matched, _ := path.Match(pattern, p)
		return matched
	}
	// Handle ** patterns: split on **, match prefix and suffix.
	parts := strings.SplitN(pattern, "**", 2)
	prefix := strings.TrimPrefix(parts[0], "/")
	suffix := ""
	if len(parts) > 1 {
		suffix = strings.TrimPrefix(parts[1], "/")
	}
	// If prefix is empty, match any start. If suffix is empty, match any end.
	ok := true
	if prefix != "" {
		ok = strings.HasPrefix(p, prefix)
	}
	if ok && suffix != "" {
		ok = strings.HasSuffix(p, suffix)
	}
	return ok
}
