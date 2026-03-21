package pack

import (
	"path/filepath"
	"strings"

	"github.com/paularlott/scriptling/libloader"
)

// Loader implements libloader.LibraryLoader over a set of packages and single files.
// Packages are searched in reverse order (last added = highest priority).
type Loader struct {
	packages []*Package
	files    map[string]string // module name -> source
	fallback libloader.LibraryLoader
	cacheDir string // empty = OS default
}

// NewLoader creates a new Loader.
func NewLoader() *Loader {
	return &Loader{files: make(map[string]string)}
}

// SetCacheDir overrides the default OS cache directory for remote packages.
func (l *Loader) SetCacheDir(dir string) {
	l.cacheDir = dir
}

// AddPackage adds a package to the loader.
func (l *Loader) AddPackage(p *Package) {
	l.packages = append(l.packages, p)
}

// AddFromPath loads a .zip package or .py file from a local path or URL.
// source may include a #sha256:<hex> fragment for integrity verification.
func (l *Loader) AddFromPath(source string, insecure bool) error {
	cleanSource, _ := splitHash(source)
	data, err := FetchWithCache(source, insecure, l.cacheDir)
	if err != nil {
		return err
	}
	if strings.HasSuffix(cleanSource, Extension) {
		p, err := Open(bytesReaderAt(data), int64(len(data)))
		if err != nil {
			return err
		}
		l.AddPackage(p)
		return nil
	}
	// Single .py file
	name := strings.TrimSuffix(filepath.Base(cleanSource), ".py")
	l.files[name] = string(data)
	return nil
}

// SetFallback sets the fallback loader used when no package provides the module.
func (l *Loader) SetFallback(fallback libloader.LibraryLoader) {
	l.fallback = fallback
}

// Load implements libloader.LibraryLoader.
// Searches single files first, then packages in reverse order, then fallback.
func (l *Loader) Load(name string) (string, bool, error) {
	// Single files take priority
	if src, ok := l.files[name]; ok {
		return src, true, nil
	}

	// Packages in reverse order (last = highest priority)
	for i := len(l.packages) - 1; i >= 0; i-- {
		if src, ok := loadFromPackage(l.packages[i], name); ok {
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

// GetMainEntry returns the main entry point from the last package that defines one.
// Returns module, function, and whether one was found.
func (l *Loader) GetMainEntry() (module, function string, found bool) {
	for i := len(l.packages) - 1; i >= 0; i-- {
		main := l.packages[i].Manifest.Main
		if main == "" {
			continue
		}
		parts := strings.SplitN(main, ".", 2)
		if len(parts) == 2 {
			return parts[0], parts[1], true
		}
	}
	return "", "", false
}

// loadFromPackage tries to resolve a dotted module name from a package's lib/ dir.
// Mirrors the resolution order of FilesystemLoader:
//  1. lib/a/b.py
//  2. lib/a/b/__init__.py
//  3. lib/a.b.py  (flat fallback)
func loadFromPackage(p *Package, name string) (string, bool) {
	parts := strings.Split(name, ".")
	joined := strings.Join(parts, "/")

	candidates := []string{
		LibDir + "/" + joined + ".py",
		LibDir + "/" + joined + "/__init__.py",
	}
	if len(parts) > 1 {
		candidates = append(candidates, LibDir+"/"+name+".py")
	}

	for _, path := range candidates {
		data, err := p.ReadFile(path)
		if err == nil {
			return string(data), true
		}
	}
	return "", false
}


