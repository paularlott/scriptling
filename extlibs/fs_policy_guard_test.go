package extlibs

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// fsCall matches Go calls that touch the filesystem by path.
var fsCall = regexp.MustCompile(`\bos\.(Stat|Lstat|Open|OpenFile|OpenRoot|ReadFile|ReadDir|Create|CreateTemp|MkdirTemp|WriteFile|Remove|RemoveAll|Rename|Mkdir|MkdirAll|Chmod|Chown|Lchown|Chtimes|Symlink|Link|Readlink|Truncate|DirFS|CopyFS)\(|\bReadFileLimited(Err)?\(|\bfilepath\.(Walk|WalkDir|Glob|EvalSymlinks)\(|\bioutil\.`)

// pathCheck matches the calls that apply the --allowed-paths policy:
// fssecurity's, and the extlibs helpers built on them.
var pathCheck = regexp.MustCompile(`\bIsPathAllowed\(|\bCheckPath\(|\bcheckPathSecurity\(|\bcheckAllowed\(`)

// fsPolicyExempt lists files that touch the filesystem without the
// allowed-paths policy, each reviewed: none can be given a path by a script.
var fsPolicyExempt = map[string]string{
	"netsecurity/netsecurity.go": "LoadConfig reads the operator's network policy file",
}

// Every library file that touches the filesystem must also apply the
// --allowed-paths policy (IsPathAllowed, CheckPath or a helper built on them),
// or be listed in fsPolicyExempt with the reason it is safe. wait_for and the
// messaging uploads read files with no check, so scripts could probe and
// exfiltrate files outside the jail (strix 4250 vuln-0001); this catches the
// next one.
//
// It works on source text, file by file: a file that checks one path and
// forgets another still passes, so it is a backstop for review, not a proof.
func TestFilesystemCallsApplyPathPolicy(t *testing.T) {
	var missing []string
	for _, root := range []string{".", "../stdlib", "../plugins"} {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == "testdata" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			src, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			rel := filepath.ToSlash(path)
			if _, ok := fsPolicyExempt[rel]; ok {
				return nil
			}
			if fsCall.Match(src) && !pathCheck.Match(src) {
				missing = append(missing, rel)
			}
			return nil
		})
		if err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
	sort.Strings(missing)
	for _, f := range missing {
		t.Errorf("%s touches the filesystem but never applies the allowed-paths policy: check script-supplied paths with fssecurity (IsPathAllowed/CheckPath/checkPathSecurity), or add the file to fsPolicyExempt with the reason it is safe", f)
	}

	// The exemptions must still exist and still need exempting.
	for f := range fsPolicyExempt {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Errorf("fsPolicyExempt lists %s, which is gone: %v", f, err)
			continue
		}
		if !fsCall.Match(src) || pathCheck.Match(src) {
			t.Errorf("fsPolicyExempt lists %s, which no longer needs exempting", f)
		}
	}
}
