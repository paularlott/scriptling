package libloader

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// containFixture lays out:
//
//	base/libs/            the library directory
//	base/libs2/           a second library directory
//	base/shared/          an allowed data directory
//	base/secret/          outside everything
type containFixture struct {
	libs, libs2, shared, secret string
}

func newContainFixture(t *testing.T) containFixture {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f := containFixture{
		libs:   filepath.Join(base, "libs"),
		libs2:  filepath.Join(base, "libs2"),
		shared: filepath.Join(base, "shared"),
		secret: filepath.Join(base, "secret"),
	}
	for _, d := range []string{f.libs, f.libs2, f.shared, f.secret} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func link(t *testing.T, target, name string) {
	t.Helper()
	if err := os.Symlink(target, name); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}
}

func load(t *testing.T, l interface {
	Load(string) (string, bool, error)
}, name string) (string, bool) {
	t.Helper()
	src, found, err := l.Load(name)
	if err != nil {
		t.Fatalf("Load(%q): %v", name, err)
	}
	return src, found
}

// A symlink in a library directory that leads outside the allowed set must not
// be followed (CWE-59, vuln-0012); an unrestricted loader still follows it.
func TestRestrictedLoaderDoesNotFollowSymlinkOutside(t *testing.T) {
	f := newContainFixture(t)
	writeFile(t, filepath.Join(f.secret, "token.txt"), "TOPSECRET")
	link(t, filepath.Join(f.secret, "token.txt"), filepath.Join(f.libs, "leak.py"))
	link(t, filepath.Join("..", "secret", "token.txt"), filepath.Join(f.libs, "leakrel.py"))
	link(t, f.secret, filepath.Join(f.libs, "leakpkg"))
	writeFile(t, filepath.Join(f.secret, "mod.py"), "X = 1")

	// Control: no restriction, the link is followed as before.
	open := NewMultiFilesystem(f.libs)
	if src, found := load(t, open, "leak"); !found || src != "TOPSECRET" {
		t.Fatalf("unrestricted loader should follow the link, got found=%v %q", found, src)
	}

	for _, allowed := range [][]string{{}, {f.shared}} {
		m := NewMultiFilesystem(f.libs)
		m.RestrictTo(allowed)
		for _, name := range []string{"leak", "leakrel", "leakpkg.mod"} {
			if src, found := load(t, m, name); found {
				t.Errorf("allowed=%v: %q escaped the library directory and loaded %q", allowed, name, src)
			}
		}
	}
}

// Files in the library directory, packages and nested modules, and links that
// stay inside it, keep working under a restriction.
func TestRestrictedLoaderLoadsNormalLibraries(t *testing.T) {
	f := newContainFixture(t)
	writeFile(t, filepath.Join(f.libs, "plain.py"), "A = 1")
	writeFile(t, filepath.Join(f.libs, "pkg", "__init__.py"), "B = 2")
	writeFile(t, filepath.Join(f.libs, "pkg", "sub.py"), "C = 3")
	writeFile(t, filepath.Join(f.libs, "flat.dotted.py"), "D = 4")
	link(t, "plain.py", filepath.Join(f.libs, "alias.py"))
	link(t, filepath.Join(f.libs, "pkg"), filepath.Join(f.libs, "pkglink"))

	for _, allowed := range [][]string{{}, {f.shared}} {
		m := NewMultiFilesystem(f.libs)
		m.RestrictTo(allowed)
		for name, want := range map[string]string{
			"plain": "A = 1", "pkg": "B = 2", "pkg.sub": "C = 3",
			"flat.dotted": "D = 4", "alias": "A = 1", "pkglink.sub": "C = 3",
		} {
			if src, found := load(t, m, name); !found || src != want {
				t.Errorf("allowed=%v: %q = (%v, %q), want %q", allowed, name, found, src, want)
			}
		}
		if _, found := load(t, m, "missing"); found {
			t.Error("a library that does not exist was found")
		}
	}
}

// A link into another library directory, or into an allowed path, is permitted:
// the operator named both.
func TestRestrictedLoaderAllowsLibDirsAndAllowedPaths(t *testing.T) {
	f := newContainFixture(t)
	writeFile(t, filepath.Join(f.libs2, "common.py"), "COMMON = 1")
	writeFile(t, filepath.Join(f.shared, "data.py"), "DATA = 1")
	link(t, filepath.Join(f.libs2, "common.py"), filepath.Join(f.libs, "common.py"))
	link(t, filepath.Join(f.shared, "data.py"), filepath.Join(f.libs, "data.py"))

	m := NewMultiFilesystem(f.libs, f.libs2)
	m.RestrictTo([]string{f.shared})
	if src, found := load(t, m, "common"); !found || src != "COMMON = 1" {
		t.Errorf("link into another library directory: (%v, %q)", found, src)
	}
	if src, found := load(t, m, "data"); !found || src != "DATA = 1" {
		t.Errorf("link into an allowed path: (%v, %q)", found, src)
	}

	// Without the allowed path the data link is refused, the lib link is not.
	m2 := NewMultiFilesystem(f.libs, f.libs2)
	m2.RestrictTo([]string{})
	if _, found := load(t, m2, "data"); found {
		t.Error("a link into a directory that is not allowed was followed")
	}
	if _, found := load(t, m2, "common"); !found {
		t.Error("a link into another library directory was refused")
	}
}

// The source cache is shared by every loader in the process and keyed by the
// candidate path. A file an unrestricted loader read and cached must not then be
// served to a restricted loader whose limits it breaks.
func TestRestrictedLoaderIgnoresSourceCachedByUnrestrictedLoader(t *testing.T) {
	f := newContainFixture(t)
	secret := filepath.Join(f.secret, "cached.txt")
	writeFile(t, secret, "CACHED-SECRET")
	// Not within the cache's "recently modified" window, so it is cached.
	old := mustOld(t, secret)
	link(t, secret, filepath.Join(f.libs, "cached.py"))

	open := NewMultiFilesystem(f.libs)
	if src, found := load(t, open, "cached"); !found || src != "CACHED-SECRET" {
		t.Fatalf("setup: unrestricted load = (%v, %q)", found, src)
	}
	if src, found := load(t, open, "cached"); !found || src != "CACHED-SECRET" {
		t.Fatalf("setup: second load = (%v, %q) (mtime %v)", found, src, old)
	}

	m := NewMultiFilesystem(f.libs)
	m.RestrictTo([]string{})
	if src, found := load(t, m, "cached"); found {
		t.Fatalf("the restricted loader was served a cached file from outside its limits: %q", src)
	}
}

func mustOld(t *testing.T, path string) any {
	t.Helper()
	past := timeAgo()
	if err := os.Chtimes(path, past, past); err != nil {
		t.Fatal(err)
	}
	return past
}

// A library directory added later is a library directory like the others.
func TestRestrictedLoaderAddDirInheritsRestriction(t *testing.T) {
	f := newContainFixture(t)
	writeFile(t, filepath.Join(f.libs2, "late.py"), "LATE = 1")
	writeFile(t, filepath.Join(f.secret, "out.txt"), "OUT")
	link(t, filepath.Join(f.secret, "out.txt"), filepath.Join(f.libs2, "out.py"))

	m := NewMultiFilesystem(f.libs)
	m.RestrictTo([]string{})
	m.AddDir(f.libs2)
	if _, found := load(t, m, "late"); !found {
		t.Error("a file in an added library directory was refused")
	}
	if _, found := load(t, m, "out"); found {
		t.Error("an escaping link in an added library directory was followed")
	}

	// RestrictTo(nil) lifts the restriction again.
	m.RestrictTo(nil)
	if _, found := load(t, m, "out"); !found {
		t.Error("RestrictTo(nil) should leave the loader unrestricted")
	}
}

// A single FilesystemLoader restricted through its option behaves the same.
func TestFilesystemLoaderWithAllowedPaths(t *testing.T) {
	f := newContainFixture(t)
	writeFile(t, filepath.Join(f.secret, "t.txt"), "S")
	link(t, filepath.Join(f.secret, "t.txt"), filepath.Join(f.libs, "leak.py"))
	writeFile(t, filepath.Join(f.libs, "ok.py"), "OK")

	l := NewFilesystem(f.libs, WithAllowedPaths([]string{}))
	if _, found := load(t, l, "leak"); found {
		t.Error("restricted FilesystemLoader followed an escaping link")
	}
	if src, found := load(t, l, "ok"); !found || src != "OK" {
		t.Errorf("restricted FilesystemLoader refused a normal library: (%v, %q)", found, src)
	}
	if _, found := load(t, NewFilesystem(f.libs, WithAllowedPaths(nil)), "leak"); !found {
		t.Error("WithAllowedPaths(nil) should leave the loader unrestricted")
	}
}

func timeAgo() time.Time { return time.Now().Add(-time.Hour) }
