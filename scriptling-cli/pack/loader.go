package pack

import (
	"fmt"
	"strings"

	"github.com/paularlott/scriptling/libloader"
)

// Loader implements libloader.LibraryLoader over a set of bundles.
// Bundles are searched in reverse order (last added = highest priority);
// within a bundle, each manifest libs dir is searched in declared order.
type Loader struct {
	bundles  []*Bundle
	fallback libloader.LibraryLoader
	cacheDir string // empty = OS default
}

// NewLoader creates a new Loader.
func NewLoader() *Loader {
	return &Loader{}
}

// SetCacheDir overrides the default OS cache directory for remote packages.
func (l *Loader) SetCacheDir(dir string) {
	l.cacheDir = dir
}

// AddBundle adds a bundle to the loader.
func (l *Loader) AddBundle(b *Bundle) {
	l.bundles = append(l.bundles, b)
}

// Bundles returns the bundles added to the loader, in add order.
func (l *Loader) Bundles() []*Bundle {
	return l.bundles
}

// AddFromPath loads a bundle from a local directory, a local .zip, or a URL.
// source may include a #sha256=<hex> fragment for integrity verification.
func (l *Loader) AddFromPath(source string, insecure bool) error {
	b, err := FetchBundle(source, insecure, l.cacheDir)
	if err != nil {
		return err
	}
	l.AddBundle(b)
	return nil
}

// SetFallback sets the fallback loader used when no bundle provides the module.
func (l *Loader) SetFallback(fallback libloader.LibraryLoader) {
	l.fallback = fallback
}

// Load implements libloader.LibraryLoader.
// Searches bundles in reverse order (last = highest priority), then fallback.
func (l *Loader) Load(name string) (string, bool, error) {
	for i := len(l.bundles) - 1; i >= 0; i-- {
		if src, ok := loadFromBundle(l.bundles[i], name); ok {
			return src, true, nil
		}
	}
	if l.fallback != nil {
		return l.fallback.Load(name)
	}
	return "", false, nil
}

// Description implements libloader.LibraryLoader.
func (l *Loader) Description() string {
	return "pack loader"
}

// MainEntry describes a bundle's resolved main entry point.
type MainEntry struct {
	// Script is the content of a .py file within the bundle, run as top-level
	// code. Set when main ends in .py and the file exists.
	Script []byte
	// ScriptName is the slash path of the script within the bundle (for error
	// messages).
	ScriptName string
	// Module and Function name the module.function entry point, used when
	// Script is nil.
	Module   string
	Function string
}

// ResolveMain determines the main entry point of the last bundle that declares
// one, using lookup-order resolution: a main ending in .py that exists as a
// file in the bundle is a script; otherwise main is treated as module.function.
// found is false when no bundle declares main; an error is returned when main
// is declared but unresolvable.
func (l *Loader) ResolveMain() (entry MainEntry, found bool, err error) {
	for i := len(l.bundles) - 1; i >= 0; i-- {
		b := l.bundles[i]
		main := b.Manifest.Main
		if main == "" {
			continue
		}
		if strings.HasSuffix(main, ".py") {
			if data, ferr := b.ReadFile(main); ferr == nil {
				return MainEntry{Script: data, ScriptName: main}, true, nil
			}
		}
		parts := strings.SplitN(main, ".", 2)
		if len(parts) == 2 {
			return MainEntry{Module: parts[0], Function: parts[1]}, true, nil
		}
		return MainEntry{}, false, fmt.Errorf("bundle %s: main %q is neither a .py file in the bundle nor module.function", b.Source(), main)
	}
	return MainEntry{}, false, nil
}

// loadFromBundle tries to resolve a dotted module name from a bundle's libs
// dirs, searched in declared order. Mirrors the resolution order of
// FilesystemLoader within each dir:
//  1. <dir>/a/b.py
//  2. <dir>/a/b/__init__.py
//  3. <dir>/a.b.py  (flat fallback)
func loadFromBundle(b *Bundle, name string) (string, bool) {
	parts := strings.Split(name, ".")
	joined := strings.Join(parts, "/")

	for _, dir := range b.Manifest.LibDirs() {
		candidates := []string{
			dir + "/" + joined + ".py",
			dir + "/" + joined + "/__init__.py",
		}
		if len(parts) > 1 {
			candidates = append(candidates, dir+"/"+name+".py")
		}
		for _, path := range candidates {
			if data, err := b.ReadFile(path); err == nil {
				return string(data), true
			}
		}
	}
	return "", false
}
