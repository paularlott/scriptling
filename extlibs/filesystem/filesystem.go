// Package filesystem provides Scriptling's filesystem extension libraries:
// os and os.path, pathlib, glob, fs, shutil, tempfile, zipfile, tarfile,
// scriptling.grep, scriptling.find and scriptling.sed. Every library checks
// paths against the allowed paths it is registered with (nil allows all,
// an empty slice denies all). The package also exports a plain-Go API
// (Grep, Find, SedReplace, EditFile, ...) that needs no interpreter.
package filesystem

import "github.com/paularlott/scriptling/object"

// Library names.
const (
	OSLibraryName       = "os"
	OSPathLibraryName   = "os.path"
	PathlibLibraryName  = "pathlib"
	GlobLibraryName     = "glob"
	FSLibraryName       = "fs"
	ShutilLibraryName   = "shutil"
	TempfileLibraryName = "tempfile"
	ZipfileLibraryName  = "zipfile"
	TarfileLibraryName  = "tarfile"
	GrepLibraryName     = "scriptling.grep"
	FindLibraryName     = "scriptling.find"
	SedLibraryName      = "scriptling.sed"
)

// RegisterAll registers every filesystem library (os and os.path, pathlib,
// glob, fs, shutil, tempfile, zipfile, tarfile, scriptling.grep,
// scriptling.find and scriptling.sed), all restricted to allowedPaths.
func RegisterAll(registrar object.LibraryRegistrar, allowedPaths []string) {
	RegisterOS(registrar, allowedPaths)
	RegisterPathlib(registrar, allowedPaths)
	RegisterGlob(registrar, allowedPaths)
	RegisterFS(registrar, allowedPaths)
	RegisterShutil(registrar, allowedPaths)
	RegisterTempfile(registrar, allowedPaths)
	RegisterZipfile(registrar, allowedPaths)
	RegisterTarfile(registrar, allowedPaths)
	RegisterGrep(registrar, allowedPaths)
	RegisterFind(registrar, allowedPaths)
	RegisterSed(registrar, allowedPaths)
}
