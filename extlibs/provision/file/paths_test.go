package file

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/fssecurity"
)

// A path outside the allowed directories is a PermissionError the script
// cannot catch, as in the os and pathlib libraries; these used to return an
// ordinary error a script could catch and carry on from (vuln-0004).
func TestProvisionFileDenialIsUncatchable(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	allowed := filepath.Join(base, "allowed")
	outside := filepath.Join(base, "outside")
	os.MkdirAll(allowed, 0o755)
	os.MkdirAll(outside, 0o755)

	for _, call := range []string{
		`file.ensure("` + outside + `/f", "x")`,
		`file.absent("` + outside + `/f")`,
		`file.ensure_directory("` + outside + `/d")`,
		`file.absent_directory("` + outside + `/d")`,
		`file.ensure_block("` + outside + `/f", "x")`,
		`file.absent_block("` + outside + `/f")`,
	} {
		p := scriptling.New()
		RegisterConfigured(p, fssecurity.Config{AllowedPaths: []string{allowed}})
		p.EnableOutputCapture()
		_, err := p.Eval("import scriptling.provision.file as file\ntry:\n    " + call + "\nexcept Exception:\n    pass\nprint('continued')")
		if err == nil || !strings.Contains(err.Error(), "outside allowed directories") {
			t.Errorf("%s: expected access denied, got %v", call, err)
		}
		if strings.Contains(p.GetOutput(), "continued") {
			t.Errorf("%s: the script caught the denial and carried on", call)
		}
	}
	if entries, _ := os.ReadDir(outside); len(entries) != 0 {
		t.Fatalf("wrote outside the allowed directories: %v", entries)
	}

	// Inside the allowed directory the calls work as before.
	p := scriptling.New()
	RegisterConfigured(p, fssecurity.Config{AllowedPaths: []string{allowed}})
	result, err := p.Eval(`import scriptling.provision.file as file
[file.ensure("` + allowed + `/f", "x"), file.absent("` + allowed + `/f")]`)
	if err != nil || result.Inspect() != "['created', 'removed']" {
		t.Fatalf("inside: %v %v", result, err)
	}
}
