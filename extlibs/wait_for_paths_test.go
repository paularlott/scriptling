package extlibs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/paularlott/scriptling"
)

// waitForJail lays out an allowed directory and, outside it, a secret file, a
// sibling directory sharing the allowed directory's name as a prefix, and
// symlinks inside the allowed directory that lead out.
func waitForJail(t *testing.T) (allowed, outside string) {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	allowed = filepath.Join(base, "allowed")
	outside = filepath.Join(base, "outside")
	for _, d := range []string{filepath.Join(allowed, "sub"), outside, base + "/allowed-other"} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(p, c string) {
		if err := os.WriteFile(p, []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(allowed, "in.txt"), "inside content")
	write(filepath.Join(outside, "secret.txt"), "root:x:0:0")
	write(base+"/allowed-other/o.txt", "sibling")
	if err := os.Symlink(filepath.Join(outside, "secret.txt"), filepath.Join(allowed, "link.txt")); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(allowed, "dirlink")); err != nil {
		t.Fatal(err)
	}
	return allowed, outside
}

func newWaitForScriptling(allowedPaths []string) *scriptling.Scriptling {
	p := scriptling.New()
	RegisterWaitForLibraryConfigured(p, allowedPaths)
	return p
}

// Under --allowed-paths, file, dir and file_content refuse any path that
// resolves outside the allowed directories, before touching it; they were an
// existence and content oracle for every file on the host (CWE-22,
// strix 4250 vuln-0001).
func TestWaitForHonorsAllowedPaths(t *testing.T) {
	allowed, outside := waitForJail(t)
	base := filepath.Dir(allowed)
	secret := filepath.Join(outside, "secret.txt")
	p := newWaitForScriptling([]string{allowed})

	for _, call := range []string{
		`w.file("` + secret + `", timeout=0)`,
		`w.dir("` + outside + `", timeout=0)`,
		`w.file_content("` + secret + `", "root:", timeout=0)`,
		`w.file("` + allowed + `/../outside/secret.txt", timeout=0)`,
		`w.file("` + allowed + `/nope/../../outside/secret.txt", timeout=0)`,
		`w.file("` + allowed + `/link.txt", timeout=0)`,
		`w.file_content("` + allowed + `/link.txt", "root:", timeout=0)`,
		`w.dir("` + allowed + `/dirlink", timeout=0)`,
		`w.file("` + base + `/allowed-other/o.txt", timeout=0)`,
	} {
		// A denial is a PermissionError the script cannot catch.
		_, err := p.Eval("import scriptling.wait_for as w\ntry:\n    " + call + "\nexcept Exception:\n    pass\n'continued'")
		if err == nil || !strings.Contains(err.Error(), "outside allowed directories") {
			t.Errorf("%s: expected an uncatchable access denied error, got %v", call, err)
		}
	}

	// Inside the allowed directory everything works as before.
	result, err := p.Eval(`
import scriptling.wait_for as w
[w.file("` + allowed + `/in.txt", timeout=0), w.dir("` + allowed + `/sub", timeout=0),
 w.file_content("` + allowed + `/in.txt", "inside", timeout=0), w.file("` + allowed + `/missing", timeout=0),
 w.file_content("` + allowed + `/in.txt", "absent", timeout=0)]`)
	if err != nil {
		t.Fatalf("allowed paths: %v", err)
	}
	if got := result.Inspect(); got != "[True, True, True, False, False]" {
		t.Fatalf("allowed paths: got %s", got)
	}
}

// The policy is checked on every poll, not just once: a path that is fine
// when the wait starts but later appears as a symlink out is refused.
func TestWaitForRechecksEachPoll(t *testing.T) {
	allowed, outside := waitForJail(t)
	late := filepath.Join(allowed, "late.txt")
	go func() {
		time.Sleep(150 * time.Millisecond)
		os.Symlink(filepath.Join(outside, "secret.txt"), late)
	}()
	p := newWaitForScriptling([]string{allowed})
	_, err := p.Eval(`import scriptling.wait_for as w
w.file_content("` + late + `", "root:", timeout=3, poll_rate=0.05)`)
	if err == nil || !strings.Contains(err.Error(), "outside allowed directories") {
		t.Fatalf("expected the late symlink to be refused, got %v", err)
	}
}

// With no allowed paths (nil) wait_for is unrestricted, as before; an empty
// list denies everything.
func TestWaitForAllowedPathsNilAndEmpty(t *testing.T) {
	_, outside := waitForJail(t)
	secret := filepath.Join(outside, "secret.txt")

	for _, p := range []*scriptling.Scriptling{newWaitForScriptling(nil), func() *scriptling.Scriptling {
		p := scriptling.New()
		RegisterWaitForLibrary(p)
		return p
	}()} {
		result, err := p.Eval(`import scriptling.wait_for as w
[w.file("` + secret + `", timeout=0), w.file_content("` + secret + `", "root:", timeout=0)]`)
		if err != nil || result.Inspect() != "[True, True]" {
			t.Fatalf("unrestricted: %v %v", result, err)
		}
	}

	p := newWaitForScriptling([]string{})
	if _, err := p.Eval(`import scriptling.wait_for as w
w.file("` + secret + `", timeout=0)`); err == nil || !strings.Contains(err.Error(), "outside allowed directories") {
		t.Fatalf("deny-all: expected access denied, got %v", err)
	}
}
