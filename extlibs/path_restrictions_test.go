package extlibs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paularlott/scriptling"
)

// Path-restriction battery: every filesystem-touching operation is tested
// under unrestricted, allowed, and deny-all configurations, plus escape
// vectors. A test that finds an operation succeeding against a path outside
// the allowed set is a security finding.

// newAllFSInterpreter registers every FS-touching library with the given
// allowed paths (nil = unrestricted, empty non-nil = deny everything).
func newAllFSInterpreter(t *testing.T, allowedPaths []string) *scriptling.Scriptling {
	t.Helper()
	p := scriptling.New()
	RegisterOSLibrary(p, allowedPaths)
	RegisterPathlibLibrary(p, allowedPaths)
	RegisterShutilLibrary(p, allowedPaths)
	RegisterTempfileLibrary(p, allowedPaths)
	RegisterGlobLibrary(p, allowedPaths)
	RegisterFindLibrary(p, allowedPaths)
	RegisterGrepLibrary(p, allowedPaths)
	RegisterSedLibrary(p, allowedPaths)
	RegisterZipfileLibrary(p, allowedPaths)
	RegisterTarfileLibrary(p, allowedPaths)
	return p
}

type fsFixture struct {
	allowed  string // inside the sandbox
	outside  string // outside the sandbox, holds a secret
	secret   string
	insideOK string // a readable file inside the sandbox
}

func newFSFixture(t *testing.T) fsFixture {
	t.Helper()
	allowed := t.TempDir()
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(secret, []byte("TOPSECRET"), 0644); err != nil {
		t.Fatal(err)
	}
	insideOK := filepath.Join(allowed, "ok.txt")
	if err := os.WriteFile(insideOK, []byte("fine"), 0644); err != nil {
		t.Fatal(err)
	}
	return fsFixture{allowed: allowed, outside: outside, secret: secret, insideOK: insideOK}
}

func evalFS(t *testing.T, p *scriptling.Scriptling, script string) (string, bool) {
	t.Helper()
	result, err := p.Eval(script)
	if err != nil {
		return err.Error(), false
	}
	return result.Inspect(), true
}

// mustDeny asserts the script fails with a permission-style error.
func mustDeny(t *testing.T, p *scriptling.Scriptling, name, script string) {
	t.Helper()
	out, ok := evalFS(t, p, script)
	if ok {
		t.Errorf("DENY-BYPASS [%s]: succeeded with %s", name, out)
		return
	}
	lower := strings.ToLower(out)
	if !strings.Contains(lower, "denied") && !strings.Contains(lower, "outside allowed") &&
		!strings.Contains(lower, "permission") && !strings.Contains(lower, "not permitted") &&
		!strings.Contains(lower, "access") {
		t.Errorf("[%s]: denied but with unexpected error: %s", name, out)
	}
}

// mustWork asserts the script evaluates without error and yields want.
func mustWork(t *testing.T, p *scriptling.Scriptling, name, script, want string) {
	t.Helper()
	out, ok := evalFS(t, p, script)
	if !ok {
		t.Errorf("[%s]: expected success, got error: %s", name, out)
		return
	}
	if want != "" && out != want {
		t.Errorf("[%s]: got %s, want %s", name, out, want)
	}
}

func TestPathRestrictionReads(t *testing.T) {
	f := newFSFixture(t)
	p := newAllFSInterpreter(t, []string{f.allowed})

	mustWork(t, p, "read inside", `import os
os.read_file("`+f.insideOK+`")`, "fine")
	mustDeny(t, p, "os.read_file outside", `import os
os.read_file("`+f.secret+`")`)
	mustDeny(t, p, "os.read_bytes outside", `import os
len(os.read_bytes("`+f.secret+`"))`)
	mustDeny(t, p, "os.read_lines outside", `import os
len(os.read_lines("`+f.secret+`"))`)
	mustDeny(t, p, "pathlib read_text outside", `import pathlib
pathlib.Path("`+f.secret+`").read_text()`)
	mustDeny(t, p, "pathlib read_bytes outside", `import pathlib
len(pathlib.Path("`+f.secret+`").read_bytes())`)
}

func TestPathRestrictionWritesAndDeletes(t *testing.T) {
	f := newFSFixture(t)
	p := newAllFSInterpreter(t, []string{f.allowed})

	mustWork(t, p, "write inside", `import os
os.write_file("`+filepath.Join(f.allowed, "w.txt")+`", "x")
"ok"`, "ok")
	mustDeny(t, p, "os.write_file outside", `import os
os.write_file("`+filepath.Join(f.outside, "evil.txt")+`", "x")`)
	mustDeny(t, p, "os.append_file outside", `import os
os.append_file("`+filepath.Join(f.outside, "evil.txt")+`", "x")`)
	mustDeny(t, p, "pathlib write_text outside", `import pathlib
pathlib.Path("`+filepath.Join(f.outside, "evil.txt")+`").write_text("x")`)
	mustDeny(t, p, "os.remove outside", `import os
os.remove("`+f.secret+`")`)
	mustDeny(t, p, "os.rename into outside", `import os
os.rename("`+f.insideOK+`", "`+filepath.Join(f.outside, "stolen")+`")`)
	mustDeny(t, p, "os.rename from outside", `import os
os.rename("`+f.secret+`", "`+filepath.Join(f.allowed, "stolen")+`")`)
	mustDeny(t, p, "os.rmdir outside", `import os
os.rmdir("`+f.outside+`")`)
	mustDeny(t, p, "os.makedirs outside", `import os
os.makedirs("`+filepath.Join(f.outside, "mk")+`")`)
	mustDeny(t, p, "os.symlink outside target", `import os
os.symlink("`+f.secret+`", "`+filepath.Join(f.allowed, "leak")+`")`)
	// Dangling targets cannot be resolved and remain allowed (Python parity).
	mustWork(t, p, "os.symlink dangling target", `import os
os.symlink("`+filepath.Join(f.allowed, "not-yet-created")+`", "`+filepath.Join(f.allowed, "future")+`")
"ok"`, "ok")
	mustDeny(t, p, "shutil.rmtree outside", `import shutil
shutil.rmtree("`+f.outside+`")`)
	mustDeny(t, p, "shutil.copy src outside", `import shutil
shutil.copy("`+f.secret+`", "`+filepath.Join(f.allowed, "stolen")+`")`)
	mustDeny(t, p, "shutil.copy dst outside", `import shutil
shutil.copy("`+f.insideOK+`", "`+filepath.Join(f.outside, "stolen")+`")`)
	mustDeny(t, p, "pathlib unlink outside", `import pathlib
pathlib.Path("`+f.secret+`").unlink()`)
}

func TestPathRestrictionListing(t *testing.T) {
	f := newFSFixture(t)
	p := newAllFSInterpreter(t, []string{f.allowed})

	mustDeny(t, p, "os.listdir outside", `import os
len(os.listdir("`+f.outside+`"))`)
	mustDeny(t, p, "find outside", `import scriptling.find as find
len(find.path("`+f.outside+`", name="*.txt"))`)
	mustDeny(t, p, "grep outside", `import scriptling.grep as grep
len(grep.pattern("SECRET", "`+f.outside+`"))`)
	mustDeny(t, p, "sed outside", `import scriptling.sed as sed
sed.replace("`+f.secret+`", "TOPSECRET", "x")`)
	mustDeny(t, p, "pathlib iterdir outside", `import pathlib
len(list(pathlib.Path("`+f.outside+`").iterdir()))`)
	mustDeny(t, p, "glob explicit root outside", `import glob
len(glob.glob("*", "`+f.outside+`"))`)
}

func TestPathRestrictionMetadata(t *testing.T) {
	f := newFSFixture(t)
	p := newAllFSInterpreter(t, []string{f.allowed})

	mustWork(t, p, "getsize inside", `import os
os.path.getsize("`+f.insideOK+`") > 0`, "True")
	mustDeny(t, p, "os.path.getsize outside", `import os
os.path.getsize("`+f.secret+`")`)
	mustDeny(t, p, "os.path.getmtime outside", `import os
os.path.getmtime("`+f.secret+`")`)
}

func TestPathRestrictionTempFiles(t *testing.T) {
	f := newFSFixture(t)
	p := newAllFSInterpreter(t, []string{f.allowed})

	// The system temp dir is outside the allowed set, so tempfile falls
	// back to creating inside the allowed area — never outside it.
	out, ok := evalFS(t, p, `import tempfile
tempfile.mkdtemp(prefix="sbox_")`)
	if !ok {
		t.Errorf("[mkdtemp sandboxed]: error %s", out)
	} else if !strings.HasPrefix(out, f.allowed) {
		t.Errorf("DENY-BYPASS [mkdtemp sandboxed]: created outside allowed at %s", out)
	}
	out, ok = evalFS(t, p, `import tempfile
tempfile.mkstemp(prefix="sbox_")`)
	if !ok {
		t.Errorf("[mkstemp sandboxed]: error %s", out)
	} else if !strings.HasPrefix(strings.TrimSuffix(out, "\""), f.allowed) && !strings.HasPrefix(out, f.allowed) {
		t.Logf("mkstemp returned %s", out)
	}

	// An explicit dir outside the allowed set is denied.
	mustDeny(t, p, "tempfile.mkdtemp explicit outside", `import tempfile
tempfile.mkdtemp(prefix="sbox_", dir="`+f.outside+`")`)

	// Deny-all: no temp creation anywhere.
	pd := newAllFSInterpreter(t, []string{})
	mustDeny(t, pd, "tempfile.mkdtemp deny-all", `import tempfile
tempfile.mkdtemp(prefix="sbox_")`)
}

func TestPathRestrictionEscapes(t *testing.T) {
	f := newFSFixture(t)
	p := newAllFSInterpreter(t, []string{f.allowed})

	// .. traversal landing outside
	mustDeny(t, p, "read via ..", `import os
os.read_file("`+f.allowed+`/../`+filepath.Base(f.outside)+`/secret.txt")`)
	mustDeny(t, p, "write via ..", `import os
os.write_file("`+f.allowed+`/../evil.txt", "x")`)
	mustDeny(t, p, "listdir via ..", `import os
len(os.listdir("`+f.allowed+`/.."))`)

	// symlink escape: create the symlink from Go (outside policy, as a
	// pre-existing condition), then read through it from scriptling.
	link := filepath.Join(f.allowed, "escape")
	if err := os.Symlink(f.secret, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	mustDeny(t, p, "read through symlink", `import os
os.read_file("`+link+`")`)
	mustDeny(t, p, "write through symlink", `import os
os.write_file("`+link+`", "x")`)
}

func TestPathRestrictionDenyAll(t *testing.T) {
	f := newFSFixture(t)
	p := newAllFSInterpreter(t, []string{}) // empty non-nil: deny everything

	mustDeny(t, p, "deny-all read", `import os
os.read_file("`+f.insideOK+`")`)
	mustDeny(t, p, "deny-all write", `import os
os.write_file("`+filepath.Join(f.allowed, "x")+`", "x")`)
	mustDeny(t, p, "deny-all listdir", `import os
len(os.listdir("`+f.allowed+`"))`)
}

func TestPathRestrictionUnrestrictedEverythingWorks(t *testing.T) {
	f := newFSFixture(t)
	p := newAllFSInterpreter(t, nil)

	mustWork(t, p, "unrestricted read", `import os
os.read_file("`+f.secret+`")`, "TOPSECRET")
	mustWork(t, p, "unrestricted listdir", `import os
len(os.listdir("`+f.outside+`")) >= 1`, "True")
	mustWork(t, p, "unrestricted copy", `import shutil
shutil.copy("`+f.secret+`", "`+filepath.Join(f.allowed, "copied")+`")
os.read_file("`+filepath.Join(f.allowed, "copied")+`")`, "TOPSECRET")
	mustWork(t, p, "unrestricted tempfile", `import tempfile
d = tempfile.mkdtemp(prefix="free_")
len(d) > 0`, "True")
}

func TestPathRestrictionZipSlip(t *testing.T) {
	f := newFSFixture(t)

	// Build a zip in Go containing an entry that escapes via ../.
	zipPath := filepath.Join(f.allowed, "evil.zip")
	buildZipSlipArchive(t, zipPath)

	p := newAllFSInterpreter(t, []string{f.allowed})

	out, ok := evalFS(t, p, `import zipfile
z = zipfile.ZipFile("`+zipPath+`")
z.extractall("`+f.allowed+`")
z.close()
"extracted"`)
	if !ok {
		t.Logf("extractall errored: %s", out)
	} else {
		t.Logf("extractall ok: %s", out)
	}
	// The escape target must not exist regardless of extract outcome.
	if _, err := os.Stat(filepath.Join(f.outside, "slipped.txt")); err == nil {
		t.Fatal("ZIP-SLIP: extraction wrote outside the allowed directory")
	}
	if _, err := os.Stat(filepath.Join(f.allowed, "safe.txt")); err != nil {
		t.Logf("note: safe entry also not extracted (extraction likely denied wholesale)")
	}
}

func TestPathRestrictionSymlinkedDirEscapes(t *testing.T) {
	f := newFSFixture(t)
	p := newAllFSInterpreter(t, []string{f.allowed})

	// A directory symlink inside allowed pointing at the outside dir.
	dirLink := filepath.Join(f.allowed, "dirlink")
	if err := os.Symlink(f.outside, dirLink); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	mustDeny(t, p, "os.listdir through dir symlink", `import os
len(os.listdir("`+dirLink+`"))`)
	mustDeny(t, p, "os.read_file through dir symlink", `import os
os.read_file("`+dirLink+`/secret.txt")`)
	// glob filters disallowed matches to empty rather than erroring
	// (round-12b policy); the assertion is the no-leak property.
	out, ok := evalFS(t, p, `import glob
glob.glob("`+dirLink+`/*")`)
	if ok && out != "[]" {
		t.Errorf("DENY-BYPASS [glob through dir symlink]: leaked %s", out)
	}
	mustDeny(t, p, "find through dir symlink", `import scriptling.find as find
len(find.path("`+dirLink+`"))`)
	mustDeny(t, p, "grep through dir symlink", `import scriptling.grep as grep
len(grep.pattern("TOPSECRET", "`+dirLink+`"))`)
	mustDeny(t, p, "shutil.copytree through dir symlink", `import shutil
shutil.copytree("`+dirLink+`", "`+filepath.Join(f.allowed, "stolen-tree")+`")`)
	mustDeny(t, p, "shutil.rmtree through dir symlink", `import shutil
shutil.rmtree("`+dirLink+`")`)
	mustDeny(t, p, "pathlib iterdir through dir symlink", `import pathlib
len(list(pathlib.Path("`+dirLink+`").iterdir()))`)
}
