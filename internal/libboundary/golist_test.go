package libboundary

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
)

// modulePath is the import path of the module under test.
const modulePath = "github.com/paularlott/scriptling"

// extlibsPrefix is the import path every extension package lives under.
const extlibsPrefix = modulePath + "/extlibs/"

var (
	rootOnce sync.Once
	rootDir  string
	rootErr  error
)

// moduleRoot returns the directory holding the module's go.mod, found by
// walking up from the test's working directory.
func moduleRoot(t testing.TB) string {
	t.Helper()
	rootOnce.Do(func() {
		dir, err := os.Getwd()
		if err != nil {
			rootErr = err
			return
		}
		for {
			data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
			if err == nil && strings.Contains(string(data), "module "+modulePath+"\n") {
				rootDir = dir
				return
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				rootErr = fmt.Errorf("no go.mod for %s above the test directory", modulePath)
				return
			}
			dir = parent
		}
	})
	if rootErr != nil {
		t.Fatalf("locating module root: %v", rootErr)
	}
	return rootDir
}

// goTool returns the path of the go command, skipping the test when it is
// not installed (the checks need `go list`).
func goTool(t testing.TB) string {
	t.Helper()
	path, err := exec.LookPath("go")
	if err != nil {
		t.Skipf("go tool not available: %v", err)
	}
	return path
}

// runGo runs the go command in dir and returns its stdout. extraEnv is
// appended to the environment.
func runGo(t testing.TB, dir string, extraEnv []string, args ...string) []byte {
	t.Helper()
	cmd := exec.Command(goTool(t), args...)
	cmd.Dir = dir
	cmd.Env = append(append(os.Environ(), "GOWORK=off"), extraEnv...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("go %s (in %s) failed: %v\n%s", strings.Join(args, " "), dir, err, stderr.String())
	}
	return stdout.Bytes()
}

// listedPackage is the subset of `go list -json` output the checks use.
type listedPackage struct {
	ImportPath string
	Dir        string
	Standard   bool
	GoFiles    []string
	CgoFiles   []string
	Imports    []string // non-test imports only
	Deps       []string // transitive non-test dependencies
	Error      *struct{ Err string }
}

// listPackages runs `go list -e -json` for the patterns from the module root.
// -e keeps a package that fails to build in the output so one broken package
// does not hide the others.
func listPackages(t testing.TB, patterns ...string) []listedPackage {
	t.Helper()
	args := append([]string{"list", "-e", "-json=ImportPath,Dir,Standard,GoFiles,CgoFiles,Imports,Deps,Error"}, patterns...)
	out := runGo(t, moduleRoot(t), nil, args...)
	dec := json.NewDecoder(bytes.NewReader(out))
	var pkgs []listedPackage
	for {
		var p listedPackage
		if err := dec.Decode(&p); err == io.EOF {
			break
		} else if err != nil {
			t.Fatalf("decoding go list output: %v", err)
		}
		pkgs = append(pkgs, p)
	}
	return pkgs
}

// listOne returns the single package a pattern names.
func listOne(t testing.TB, pattern string) listedPackage {
	t.Helper()
	pkgs := listPackages(t, pattern)
	if len(pkgs) != 1 {
		t.Fatalf("go list %s: want exactly one package, got %d", pattern, len(pkgs))
	}
	if pkgs[0].Error != nil {
		t.Fatalf("go list %s: package has errors, fix the build first: %s", pattern, pkgs[0].Error.Err)
	}
	if len(pkgs[0].Deps) == 0 {
		t.Fatalf("go list %s reported no dependencies; the checker would pass vacuously", pattern)
	}
	return pkgs[0]
}

// lines splits command output into non-empty trimmed lines.
func lines(out []byte) []string {
	var res []string
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	for sc.Scan() {
		if l := strings.TrimSpace(sc.Text()); l != "" {
			res = append(res, l)
		}
	}
	return res
}

// ---------------------------------------------------------------------------
// Pure checkers. They take plain data so the self-tests in checkers_test.go
// can feed them synthetic input and prove they report what they should.
// ---------------------------------------------------------------------------

// pathMatches reports whether import path pkg is pattern or a package below
// it ("net/http" matches "net/http" and "net/http/httptrace", but not
// "golang.org/x/net/http2"; "os" matches "os/exec" but not "osx").
func pathMatches(pattern, pkg string) bool {
	return pkg == pattern || strings.HasPrefix(pkg, pattern+"/")
}

// rule is a banned import path with the reason it is banned.
type rule struct {
	pattern string
	reason  string
}

// bannedImports returns, sorted, "imp (reason)" for every import matching a
// banned rule that is not covered by an allowed exception (exact path).
func bannedImports(imports []string, banned []rule, allowed map[string]string) []string {
	var out []string
	for _, imp := range imports {
		if _, ok := allowed[imp]; ok {
			continue
		}
		for _, r := range banned {
			if pathMatches(r.pattern, imp) {
				out = append(out, fmt.Sprintf("%s (%s)", imp, r.reason))
				break
			}
		}
	}
	sort.Strings(out)
	return out
}

// matchingDeps returns, sorted and de-duplicated, every dep that matches one
// of the patterns.
func matchingDeps(deps []string, patterns []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, d := range deps {
		for _, p := range patterns {
			if pathMatches(p, d) && !seen[d] {
				seen[d] = true
				out = append(out, d)
			}
		}
	}
	sort.Strings(out)
	return out
}

// isThirdParty reports whether import path pkg comes from outside the Go
// standard library and outside module mod. Standard-library paths never have
// a dot in their first element (vendored std copies start with "vendor/").
func isThirdParty(pkg, mod string) bool {
	if pathMatches(mod, pkg) {
		return false
	}
	first, _, _ := strings.Cut(pkg, "/")
	return strings.Contains(first, ".")
}

// thirdPartyDiff compares the third-party packages in deps against an
// allowlist of import-path prefixes. unexpected lists third-party packages no
// prefix covers; unused lists allowlist prefixes no dependency matches (a
// stale entry that should be removed so the list stays exact).
func thirdPartyDiff(deps []string, mod string, allow []string) (unexpected, unused []string) {
	used := map[string]bool{}
	for _, d := range deps {
		if !isThirdParty(d, mod) {
			continue
		}
		covered := false
		for _, a := range allow {
			if pathMatches(a, d) {
				used[a] = true
				covered = true
			}
		}
		if !covered {
			unexpected = append(unexpected, d)
		}
	}
	for _, a := range allow {
		if !used[a] {
			unused = append(unused, a)
		}
	}
	sort.Strings(unexpected)
	sort.Strings(unused)
	return unexpected, unused
}

// edge is a non-test import from one extension package to another, both
// written relative to extlibs/ (e.g. "ai/memory" -> "runtime").
type edge struct{ from, to string }

func (e edge) String() string { return e.from + " -> " + e.to }

// leafViolations applies the leaf rule. imports maps each extension package
// (relative to extlibs/) to its non-test imports (full import paths). An
// import of another package under extlibs/ is a violation unless the target
// is a support package or the edge is in allowed. stale lists allowed edges
// that no longer exist.
func leafViolations(imports map[string][]string, support map[string]string, allowed map[edge]string) (violations, stale []edge) {
	present := map[edge]bool{}
	for from, imps := range imports {
		for _, imp := range imps {
			if !strings.HasPrefix(imp, extlibsPrefix) {
				continue
			}
			to := strings.TrimPrefix(imp, extlibsPrefix)
			if _, ok := support[to]; ok {
				continue
			}
			e := edge{from, to}
			present[e] = true
			if _, ok := allowed[e]; !ok {
				violations = append(violations, e)
			}
		}
	}
	for e := range allowed {
		if !present[e] {
			stale = append(stale, e)
		}
	}
	sortEdges(violations)
	sortEdges(stale)
	return violations, stale
}

func sortEdges(es []edge) {
	sort.Slice(es, func(i, j int) bool { return es[i].String() < es[j].String() })
}

// symbolHits returns, sorted, the forbidden patterns that occur as a
// substring of any symbol line in nm output ("addr type name" per line; the
// whole line is searched because type symbols may contain spaces and the
// address and type columns cannot match an import path).
func symbolHits(nmOutput []byte, forbidden []string) []string {
	hit := map[string]bool{}
	for _, l := range lines(nmOutput) {
		for _, f := range forbidden {
			if strings.Contains(l, f) {
				hit[f] = true
			}
		}
	}
	var out []string
	for f := range hit {
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}
