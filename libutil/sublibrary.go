package libutil

import (
	"path/filepath"
	"strings"

	"github.com/paularlott/scriptling/object"
)

// LibraryLookup is implemented by registrars that can return a library
// already registered with them (scriptling.Scriptling does). Sub-library
// registration uses it to expose a sub-library as an attribute of its parent
// library, so `import parent as p; p.sub.fn()` works without a separate
// `import parent.sub`.
type LibraryLookup interface {
	RegisteredLibrary(name string) (*object.Library, bool)
}

// RegisterSubLibrary registers lib with registrar and, when the parent
// library (lib's name up to its last dot) is already registered there, sets
// the sub-library's dict as the parent's attribute named after the last
// component of lib's name. Parent libraries registered later pick up
// already-registered sub-libraries with AttachSubLibraries, so the two can be
// registered in either order. A registrar that cannot look libraries up only
// gets lib registered; `import parent.sub` still works then.
func RegisterSubLibrary(registrar object.LibraryRegistrar, lib *object.Library) {
	registrar.RegisterLibrary(lib)
	lookup, ok := registrar.(LibraryLookup)
	if !ok {
		return
	}
	name := lib.Name()
	dot := strings.LastIndexByte(name, '.')
	if dot < 0 {
		return
	}
	parent, ok := lookup.RegisteredLibrary(name[:dot])
	if !ok || parent == nil || parent.Constants() == nil {
		return
	}
	// Only attach to the library registered under that name: a registrar
	// that filters registrations (or replaced lib) must not see it attached.
	if registered, ok := lookup.RegisteredLibrary(name); !ok || registered != lib {
		return
	}
	parent.Constants()[name[dot+1:]] = lib.GetDict()
}

// AttachSubLibraries sets, on parent, an attribute for each of the named
// sub-libraries (parent.Name()+"."+attr) that is already registered with
// registrar. parent must have a non-nil constants map and should be a library
// built for this registrar alone, since attaching mutates it. A registrar that
// cannot look libraries up attaches nothing.
func AttachSubLibraries(registrar object.LibraryRegistrar, parent *object.Library, attrs ...string) {
	lookup, ok := registrar.(LibraryLookup)
	if !ok || parent.Constants() == nil {
		return
	}
	for _, attr := range attrs {
		if lib, ok := lookup.RegisteredLibrary(parent.Name() + "." + attr); ok && lib != nil {
			parent.Constants()[attr] = lib.GetDict()
		}
	}
}

// ModuleRef derives the "module.function" reference for a decorated function
// defined in env. It uses __name__, falling back to the base name of __file__
// when __name__ is "__main__" (as in a setup script). It returns "" when
// neither gives a module name.
func ModuleRef(env *object.Environment, funcName string) string {
	if env != nil {
		if nameObj, ok := env.Get("__name__"); ok {
			if name, e := nameObj.AsString(); e == nil && name != "" && name != "__main__" {
				return name + "." + funcName
			}
		}
		if fileObj, ok := env.Get("__file__"); ok {
			if file, e := fileObj.AsString(); e == nil && file != "" {
				base := filepath.Base(file)
				ext := filepath.Ext(base)
				if module := strings.TrimSuffix(base, ext); module != "" {
					return module + "." + funcName
				}
			}
		}
	}
	return ""
}
