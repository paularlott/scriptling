package filesystem

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// Pure path operations of pathlib.Path: they compute new paths and touch no
// filesystem, so they need no path-security check.

// pathSuffixes is Path.suffixes for a final component.
func pathSuffixes(name string) object.Object {
	var out []object.Object
	if !strings.HasSuffix(name, ".") {
		parts := strings.Split(strings.TrimLeft(name, "."), ".")
		for _, s := range parts[1:] {
			out = append(out, object.NewString("."+s))
		}
	}
	return &object.List{Elements: out}
}

// pathValueError is a raised ValueError.
func pathValueError(msg string) object.Object {
	return &object.Exception{Message: msg, ExceptionType: object.ExceptionTypeValueError, Raised: true}
}

// pathOrString reads a Path or a string argument.
func pathOrString(arg object.Object) (string, object.Object) {
	if inst, ok := arg.(*object.Instance); ok {
		return pathFrom(inst)
	}
	s, ok := arg.(*object.String)
	if !ok {
		return "", errors.NewTypeErrorTagged("expected str or Path, not %s", object.PythonTypeName(arg))
	}
	return s.StringValue(), nil
}

func pathRepr(path string) string {
	return "PosixPath(" + object.ReprString(path) + ")"
}

func (p *PathlibLibraryInstance) addPurePathMethods() {
	m := p.PathClass.Methods
	// withName replaces the final component, or fails as Python does for a
	// path with no name.
	withName := func(path, name string) object.Object {
		base := filepath.Base(path)
		if path == "." || base == string(filepath.Separator) || base == "" {
			return pathValueError(pathRepr(path) + " has an empty name")
		}
		return p.createPathObject(filepath.Join(filepath.Dir(path), name))
	}
	m["with_name"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}
			path, errObj := pathArg(args)
			if errObj != nil {
				return errObj
			}
			name, err := args[1].AsString()
			if err != nil {
				return err
			}
			if name == "" || name == "." || strings.ContainsRune(name, filepath.Separator) {
				return pathValueError("Invalid name " + object.ReprString(name))
			}
			return withName(path, name)
		},
		HelpText: "with_name(name) - A new path with the final component replaced",
	}
	m["with_suffix"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}
			path, errObj := pathArg(args)
			if errObj != nil {
				return errObj
			}
			suffix, err := args[1].AsString()
			if err != nil {
				return err
			}
			if suffix != "" && (!strings.HasPrefix(suffix, ".") || strings.ContainsRune(suffix, filepath.Separator)) {
				return pathValueError("Invalid suffix " + object.ReprString(suffix))
			}
			base := filepath.Base(path)
			stem := strings.TrimSuffix(base, filepath.Ext(base))
			return withName(path, stem+suffix)
		},
		HelpText: `with_suffix(suffix) - A new path with the suffix changed (added, or removed with "")

  Path("a/b.txt").with_suffix(".md")   # a/b.md`,
	}
	m["with_stem"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}
			path, errObj := pathArg(args)
			if errObj != nil {
				return errObj
			}
			stem, err := args[1].AsString()
			if err != nil {
				return err
			}
			ext := filepath.Ext(filepath.Base(path))
			if stem == "" && ext != "" {
				return pathValueError(pathRepr(path) + " has a non-empty suffix")
			}
			if stem == "" || strings.ContainsRune(stem, filepath.Separator) {
				return pathValueError("Invalid name " + object.ReprString(stem+ext))
			}
			return withName(path, stem+ext)
		},
		HelpText: "with_stem(stem) - A new path with the stem changed, keeping the suffix",
	}
	m["is_absolute"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			path, errObj := pathArg(args)
			if errObj != nil {
				return errObj
			}
			return object.NewBoolean(filepath.IsAbs(path))
		},
		HelpText: "is_absolute() - True for a path starting at the root",
	}
	m["as_posix"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			path, errObj := pathArg(args)
			if errObj != nil {
				return errObj
			}
			return object.NewString(filepath.ToSlash(path))
		},
		HelpText: "as_posix() - The path as a string with forward slashes",
	}
	m["relative_to"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.MinArgs(args, 2); err != nil {
				return err
			}
			path, errObj := pathArg(args)
			if errObj != nil {
				return errObj
			}
			other, errObj := pathOrString(args[1])
			if errObj != nil {
				return errObj
			}
			other = filepath.Clean(other)
			rel, err := filepath.Rel(other, path)
			if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) ||
				filepath.IsAbs(path) != filepath.IsAbs(other) {
				return pathValueError(object.ReprString(path) + " is not in the subpath of " + object.ReprString(other))
			}
			return p.createPathObject(rel)
		},
		HelpText: "relative_to(other) - This path relative to other; ValueError when it is not under other",
	}
	m["is_relative_to"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}
			path, errObj := pathArg(args)
			if errObj != nil {
				return errObj
			}
			other, errObj := pathOrString(args[1])
			if errObj != nil {
				return errObj
			}
			rel, err := filepath.Rel(filepath.Clean(other), path)
			ok := err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) &&
				filepath.IsAbs(path) == filepath.IsAbs(other)
			return object.NewBoolean(ok)
		},
		HelpText: "is_relative_to(other) - True when this path is under other",
	}
	// Paths compare and hash by their path string, so they work as dict
	// keys and in sets, and sort.
	m["__eq__"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) != 2 {
				return object.NewBoolean(false)
			}
			a, errObj := pathArg(args)
			if errObj != nil {
				return errObj
			}
			other, ok := args[1].(*object.Instance)
			if !ok {
				return object.NewBoolean(false)
			}
			b, errObj := pathFrom(other)
			if errObj != nil {
				return object.NewBoolean(false)
			}
			return object.NewBoolean(a == b)
		},
		HelpText: "__eq__(other) - Paths are equal when their paths are",
	}
	m["__lt__"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) != 2 {
				return errors.NewTypeErrorTagged("__lt__ expects one argument")
			}
			a, errObj := pathArg(args)
			if errObj != nil {
				return errObj
			}
			other, ok := args[1].(*object.Instance)
			if !ok {
				return errors.NewTypeErrorTagged("'<' not supported between instances of 'PosixPath' and '%s'", object.PythonTypeName(args[1]))
			}
			b, errObj := pathFrom(other)
			if errObj != nil {
				return errObj
			}
			return object.NewBoolean(a < b)
		},
		HelpText: "__lt__(other) - Order paths by their path strings",
	}
	m["__hash__"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			path, errObj := pathArg(args)
			if errObj != nil {
				return errObj
			}
			var h int64 = 1469598103934665603
			for i := 0; i < len(path); i++ {
				h = (h ^ int64(path[i])) * 1099511628211
			}
			return object.NewInteger(h)
		},
		HelpText: "__hash__() - Hash of the path string",
	}
}
