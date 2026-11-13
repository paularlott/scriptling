package filesystem_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/filesystem"
	"github.com/paularlott/scriptling/internal/scripttest"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/stdlib"
)

// registerFunc is the shape shared by every filesystem Register function.
type registerFunc func(object.LibraryRegistrar, []string)

// fsLibraries maps each Register function to the script names it provides.
var fsLibraries = []struct {
	register registerFunc
	fn       string
	names    []string
}{
	{filesystem.RegisterOS, "RegisterOS", []string{"os", "os.path"}},
	{filesystem.RegisterPathlib, "RegisterPathlib", []string{"pathlib"}},
	{filesystem.RegisterGlob, "RegisterGlob", []string{"glob"}},
	{filesystem.RegisterFS, "RegisterFS", []string{"fs"}},
	{filesystem.RegisterShutil, "RegisterShutil", []string{"shutil"}},
	{filesystem.RegisterTempfile, "RegisterTempfile", []string{"tempfile"}},
	{filesystem.RegisterZipfile, "RegisterZipfile", []string{"zipfile"}},
	{filesystem.RegisterTarfile, "RegisterTarfile", []string{"tarfile"}},
	{filesystem.RegisterGrep, "RegisterGrep", []string{"scriptling.grep"}},
	{filesystem.RegisterFind, "RegisterFind", []string{"scriptling.find"}},
	{filesystem.RegisterSed, "RegisterSed", []string{"scriptling.sed"}},
}

// recorder collects the names a Register function registers.
type recorder struct{ names []string }

func (r *recorder) RegisterLibrary(lib *object.Library) { r.names = append(r.names, lib.Name()) }

// slash makes a path safe to embed in a script string literal.
func slash(p string) string { return filepath.ToSlash(p) }

func TestLibraryNameConstants(t *testing.T) {
	want := map[string]string{
		filesystem.OSLibraryName: "os", filesystem.OSPathLibraryName: "os.path",
		filesystem.PathlibLibraryName: "pathlib", filesystem.GlobLibraryName: "glob",
		filesystem.FSLibraryName: "fs", filesystem.ShutilLibraryName: "shutil",
		filesystem.TempfileLibraryName: "tempfile", filesystem.ZipfileLibraryName: "zipfile",
		filesystem.TarfileLibraryName: "tarfile", filesystem.GrepLibraryName: "scriptling.grep",
		filesystem.FindLibraryName: "scriptling.find", filesystem.SedLibraryName: "scriptling.sed",
	}
	for got, w := range want {
		if got != w {
			t.Errorf("library name constant is %q, want %q (scripts import it by this name)", got, w)
		}
	}
}

func TestRegisterAllRegistersExactlyTheFilesystemLibraries(t *testing.T) {
	var r recorder
	filesystem.RegisterAll(&r, nil)
	var want []string
	for _, l := range fsLibraries {
		want = append(want, l.names...)
	}
	got := slices.Clone(r.names)
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("filesystem.RegisterAll registered %v,\nwant exactly the eleven libraries (twelve names with os.path) %v", got, want)
	}
	if len(want) != 12 {
		t.Fatalf("test table lists %d names, want 12", len(want))
	}
}

func TestEachRegisterProvidesOnlyItsNames(t *testing.T) {
	for _, l := range fsLibraries {
		t.Run(l.fn, func(t *testing.T) {
			var r recorder
			l.register(&r, nil)
			got := slices.Clone(r.names)
			slices.Sort(got)
			want := slices.Clone(l.names)
			slices.Sort(want)
			if !slices.Equal(got, want) {
				t.Errorf("filesystem.%s registered %v, want %v", l.fn, got, want)
			}

			p := scriptling.New()
			l.register(p, nil)
			for _, n := range l.names {
				scripttest.Imports(t, p, n)
			}
		})
	}
}

func TestUnregisteredFilesystemLibrariesAreNotImportable(t *testing.T) {
	for _, l := range fsLibraries {
		for _, n := range l.names {
			t.Run(n, func(t *testing.T) {
				p := scriptling.New()
				stdlib.RegisterAll(p)
				scripttest.ImportFails(t, p, n, "it is only available after filesystem."+l.fn+"(p, allowedPaths)")
			})
		}
	}
}

func TestRegisteringOneLibraryDoesNotProvideTheOthers(t *testing.T) {
	p := scriptling.New()
	filesystem.RegisterPathlib(p, nil)
	scripttest.Imports(t, p, "pathlib")
	for _, n := range []string{"os", "glob", "shutil", "scriptling.sed"} {
		scripttest.ImportFails(t, p, n, "RegisterPathlib must register pathlib only")
	}
}

func TestFilesystemRepresentativeCalls(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(file, []byte("hello world\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	d, f := slash(dir), slash(file)

	tests := []struct {
		name     string
		register registerFunc
		src      string
		want     string
		check    func(t *testing.T)
	}{
		{"os", filesystem.RegisterOS,
			"import os\nos.write_file('" + d + "/w.txt', 'hi')\nos.read_file('" + d + "/w.txt')", "hi", nil},
		{"os.path", filesystem.RegisterOS, "import os.path\nos.path.basename('" + f + "')", "a.txt", nil},
		{"pathlib", filesystem.RegisterPathlib, "import pathlib\npathlib.Path('" + f + "').read_text()", "hello world\n", nil},
		{"glob", filesystem.RegisterGlob, "import glob\nstr(len(glob.glob('" + d + "/a.*')))", "1", nil},
		{"fs", filesystem.RegisterFS, "import fs\nstr(len(fs.read_bytes('" + f + "', 0, 5)))", "5", nil},
		{"shutil", filesystem.RegisterShutil, "import shutil\nshutil.copy('" + f + "', '" + d + "/copy.txt')\n'ok'", "ok",
			func(t *testing.T) {
				if b, err := os.ReadFile(filepath.Join(dir, "copy.txt")); err != nil || string(b) != "hello world\n" {
					t.Errorf("shutil.copy did not produce the copy: %q, %v", b, err)
				}
			}},
		{"tempfile", filesystem.RegisterTempfile, "import tempfile\ntempfile.mkdtemp(prefix='reg', dir='" + d + "')", "", nil},
		{"zipfile", filesystem.RegisterZipfile,
			"import zipfile\nz = zipfile.ZipFile('" + d + "/t.zip', 'w')\nz.writestr('in.txt', 'x')\nz.close()\n" +
				"z = zipfile.ZipFile('" + d + "/t.zip')\nn = z.namelist()[0]\nz.close()\nn", "in.txt", nil},
		{"tarfile", filesystem.RegisterTarfile,
			"import tarfile\nt = tarfile.TarFile('" + d + "/t.tar', 'w')\nt.addstr('in.txt', 'x')\nt.close()\n" +
				"t = tarfile.TarFile('" + d + "/t.tar')\nn = t.getnames()[0]\nt.close()\nn", "in.txt", nil},
		{"scriptling.grep", filesystem.RegisterGrep, "import scriptling.grep as grep\nstr(len(grep.pattern('hel+o', '" + f + "')))", "1", nil},
		{"scriptling.find", filesystem.RegisterFind, "import scriptling.find as find\nstr(len(find.path('" + d + "', name='a.txt')))", "1", nil},
		{"scriptling.sed", filesystem.RegisterSed, "import scriptling.sed as sed\nsed.replace('world', 'there', '" + d + "/s.txt')\n'ok'", "ok",
			func(t *testing.T) {
				if b, err := os.ReadFile(filepath.Join(dir, "s.txt")); err != nil || string(b) != "hello there\n" {
					t.Errorf("sed.replace did not rewrite the file: %q, %v", b, err)
				}
			}},
	}
	if err := os.WriteFile(filepath.Join(dir, "s.txt"), []byte("hello world\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := scriptling.New()
			tt.register(p, []string{dir}) // inside the allowed path
			got := scripttest.EvalString(t, p, tt.src)
			if tt.want != "" && got != tt.want {
				t.Errorf("%s: got %q, want %q", tt.name, got, tt.want)
			}
			if tt.name == "tempfile" && !strings.HasPrefix(slash(got), d+"/reg") {
				t.Errorf("tempfile.mkdtemp returned %q, want a directory under %s", got, d)
			}
			if tt.check != nil {
				tt.check(t)
			}
		})
	}
}

// deniedMessage reports whether msg is a path-policy refusal.
func deniedMessage(msg string) bool {
	m := strings.ToLower(msg)
	return strings.Contains(m, "denied") || strings.Contains(m, "outside allowed") || strings.Contains(m, "not permitted")
}

// A path outside allowedPaths is refused with a PermissionError. Security
// refusals are protected: a script handler cannot swallow them, so the
// embedder always sees the violation.
func TestFilesystemPolicyRefusalCannotBeSwallowed(t *testing.T) {
	allowed := t.TempDir()
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(secret, []byte("TOPSECRET"), 0o644); err != nil {
		t.Fatal(err)
	}
	o, s := slash(outside), slash(secret)

	tests := []struct {
		name     string
		register registerFunc
		src      string
	}{
		{"os", filesystem.RegisterOS, "import os\nos.read_file('" + s + "')"},
		{"pathlib", filesystem.RegisterPathlib, "import pathlib\npathlib.Path('" + s + "').read_text()"},
		{"glob", filesystem.RegisterGlob, "import glob\nglob.glob('*', '" + o + "')"},
		{"fs", filesystem.RegisterFS, "import fs\nfs.read_bytes('" + s + "', 0, 2)"},
		{"shutil", filesystem.RegisterShutil, "import shutil\nshutil.copy('" + s + "', '" + slash(allowed) + "/x.txt')"},
		{"tempfile", filesystem.RegisterTempfile, "import tempfile\ntempfile.mkdtemp(dir='" + o + "')"},
		{"zipfile", filesystem.RegisterZipfile, "import zipfile\nzipfile.ZipFile('" + o + "/x.zip', 'w')"},
		{"tarfile", filesystem.RegisterTarfile, "import tarfile\ntarfile.TarFile('" + o + "/x.tar', 'w')"},
		{"scriptling.grep", filesystem.RegisterGrep, "import scriptling.grep as grep\ngrep.pattern('TOP', '" + s + "')"},
		{"scriptling.find", filesystem.RegisterFind, "import scriptling.find as find\nfind.path('" + o + "')"},
		{"scriptling.sed", filesystem.RegisterSed, "import scriptling.sed as sed\nsed.replace('TOP', 'x', '" + s + "')"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := scriptling.New()
			tt.register(p, []string{allowed})
			if msg := scripttest.Refused(t, p, tt.src); !deniedMessage(msg) {
				t.Errorf("%s outside allowedPaths: got %q, want a path-policy refusal", tt.name, msg)
			}
		})
	}
	if b, _ := os.ReadFile(secret); string(b) != "TOPSECRET" {
		t.Errorf("a refused call modified the file outside the allowed paths: %q", b)
	}
}

func TestFilesystemAllowedPathsNilVersusEmpty(t *testing.T) {
	file := filepath.Join(t.TempDir(), "f.txt")
	if err := os.WriteFile(file, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	src := "import os\nos.read_file('" + slash(file) + "')"

	// nil: unrestricted.
	p := scriptling.New()
	filesystem.RegisterOS(p, nil)
	if got := scripttest.EvalString(t, p, src); got != "data" {
		t.Errorf("nil allowedPaths must be unrestricted, read %q", got)
	}

	// empty non-nil: deny everything.
	p = scriptling.New()
	filesystem.RegisterOS(p, []string{})
	if msg := scripttest.Refused(t, p, src); !deniedMessage(msg) {
		t.Errorf("empty allowedPaths must deny all access, got %q", msg)
	}
}

func TestRegisterOSTwice(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(b, "f.txt"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := scriptling.New()
	filesystem.RegisterOS(p, []string{a})
	filesystem.RegisterOS(p, []string{b})
	// The later registration replaces the earlier one rather than adding a
	// second copy: its policy is the one in force.
	if got := scripttest.EvalString(t, p, "import os\nos.read_file('"+slash(filepath.Join(b, "f.txt"))+"')"); got != "b" {
		t.Errorf("after re-registering with a new allowed path, read %q", got)
	}
}
