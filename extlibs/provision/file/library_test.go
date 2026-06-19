package file

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paularlott/scriptling"
)

func TestProvisionFileRegistration(t *testing.T) {
	p := scriptling.New()
	Register(p)

	_, err := p.Eval(`
import scriptling.provision.file as file
`)
	if err != nil {
		t.Fatalf("Failed to import provision.file library: %v", err)
	}
}

func TestEnsureCreate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "test.txt")

	p := scriptling.New()
	Register(p)

	_, err := p.Eval(`
import scriptling.provision.file as file
status = file.ensure("` + path + `", "hello world")
if status != file.CREATED:
    raise Exception("expected CREATED")
`)
	if err != nil {
		t.Fatalf("Failed to run ensure: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	if string(content) != "hello world" {
		t.Errorf("Expected 'hello world', got %s", string(content))
	}
}

func TestEnsureUnchanged(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte("same content"), 0o644)

	p := scriptling.New()
	Register(p)

	_, err := p.Eval(`
import scriptling.provision.file as file
status = file.ensure("` + path + `", "same content")
if status != file.UNCHANGED:
    raise Exception("expected UNCHANGED")
`)
	if err != nil {
		t.Fatalf("Failed to run ensure: %v", err)
	}

	status, _ := p.GetVar("status")
	if status.(string) != "unchanged" {
		t.Errorf("Expected 'unchanged', got %s", status)
	}
}

func TestEnsureUpdated(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte("old content"), 0o644)

	p := scriptling.New()
	Register(p)

	_, err := p.Eval(`
import scriptling.provision.file as file
status = file.ensure("` + path + `", "new content")
if status != file.UPDATED:
    raise Exception("expected UPDATED")
`)
	if err != nil {
		t.Fatalf("Failed to run ensure: %v", err)
	}

	content, _ := os.ReadFile(path)
	if string(content) != "new content" {
		t.Errorf("Expected 'new content', got %s", string(content))
	}
}

func TestEnsureWithMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secret.txt")

	p := scriptling.New()
	Register(p)

	_, err := p.Eval(`
import scriptling.provision.file as file
status = file.ensure("` + path + `", "secret", mode=0o600)
`)
	if err != nil {
		t.Fatalf("Failed to run ensure with mode: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("Expected mode 0o600, got %o", info.Mode().Perm())
	}
}

func TestEnsureCreatesParentDirs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a", "b", "c", "test.txt")

	p := scriptling.New()
	Register(p)

	_, err := p.Eval(`
import scriptling.provision.file as file
status = file.ensure("` + path + `", "nested")
`)
	if err != nil {
		t.Fatalf("Failed to run ensure with nested dirs: %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("Expected file to exist at %s", path)
	}
}

func TestEnsureWrongArgCount(t *testing.T) {
	p := scriptling.New()
	Register(p)

	_, err := p.Eval(`
import scriptling.provision.file as file
try:
    file.ensure("only_one_arg")
    error_caught = False
except:
    error_caught = True
`)
	if err != nil {
		t.Fatalf("Failed to run wrong arg count test: %v", err)
	}

	caught, _ := p.GetVar("error_caught")
	if caught.(bool) != true {
		t.Errorf("Expected error for missing argument")
	}
}

func TestEnsureOverwriteDifferentMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte("same"), 0o644)

	p := scriptling.New()
	Register(p)

	_, err := p.Eval(`
import scriptling.provision.file as file
status = file.ensure("` + path + `", "same", mode=0o600)
if status != file.UNCHANGED:
    raise Exception("expected UNCHANGED")
`)
	if err != nil {
		t.Fatalf("Failed to run ensure: %v", err)
	}
}

func TestEnsureCreateOnlySkipsExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte("original"), 0o644)

	p := scriptling.New()
	Register(p)

	_, err := p.Eval(`
import scriptling.provision.file as file
status = file.ensure("` + path + `", "different", create_only=True)
if status != file.UNCHANGED:
    raise Exception("expected UNCHANGED")
`)
	if err != nil {
		t.Fatalf("Failed to run ensure with create_only: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	if string(content) != "original" {
		t.Errorf("create_only should not modify existing file, got %s", string(content))
	}
}

func TestEnsureCreateOnlyCreatesNew(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "test.txt")

	p := scriptling.New()
	Register(p)

	_, err := p.Eval(`
import scriptling.provision.file as file
status = file.ensure("` + path + `", "fresh", create_only=True)
if status != file.CREATED:
    raise Exception("expected CREATED")
`)
	if err != nil {
		t.Fatalf("Failed to run ensure with create_only: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	if string(content) != "fresh" {
		t.Errorf("Expected 'fresh', got %s", string(content))
	}
}

func TestEnsureCreateOnlyFalseUpdates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte("old"), 0o644)

	p := scriptling.New()
	Register(p)

	_, err := p.Eval(`
import scriptling.provision.file as file
status = file.ensure("` + path + `", "new", create_only=False)
if status != file.UPDATED:
    raise Exception("expected UPDATED")
`)
	if err != nil {
		t.Fatalf("Failed to run ensure: %v", err)
	}

	content, _ := os.ReadFile(path)
	if string(content) != "new" {
		t.Errorf("Expected 'new', got %s", string(content))
	}
}

func TestAbsentRemoves(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte("data"), 0o644)

	p := scriptling.New()
	Register(p)

	_, err := p.Eval(`
import scriptling.provision.file as file
status = file.absent("` + path + `")
if status != file.REMOVED:
    raise Exception("expected REMOVED")
`)
	if err != nil {
		t.Fatalf("Failed to run absent: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("Expected file to be removed")
	}
}

func TestAbsentAlreadyGone(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.txt")

	p := scriptling.New()
	Register(p)

	_, err := p.Eval(`
import scriptling.provision.file as file
status = file.absent("` + path + `")
if status != file.ABSENT:
    raise Exception("expected ABSENT")
`)
	if err != nil {
		t.Fatalf("Failed to run absent: %v", err)
	}
}

func TestDirectoryCreate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a", "b", "c")

	p := scriptling.New()
	Register(p)

	_, err := p.Eval(`
import scriptling.provision.file as file
status = file.ensure_directory("` + path + `")
if status != file.CREATED:
    raise Exception("expected CREATED")
`)
	if err != nil {
		t.Fatalf("Failed to run ensure_directory: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Expected dir to exist: %v", err)
	}
	if !info.IsDir() {
		t.Errorf("Expected %s to be a directory", path)
	}
}

func TestEnsureDirectoryExists(t *testing.T) {
	dir := t.TempDir()

	p := scriptling.New()
	Register(p)

	_, err := p.Eval(`
import scriptling.provision.file as file
status = file.ensure_directory("` + dir + `")
if status != file.EXISTS:
    raise Exception("expected EXISTS")
`)
	if err != nil {
		t.Fatalf("Failed to run ensure_directory: %v", err)
	}
}

func TestEnsureDirectoryNotADir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	os.WriteFile(path, []byte("data"), 0o644)

	p := scriptling.New()
	Register(p)

	_, err := p.Eval(`
import scriptling.provision.file as file
try:
    file.ensure_directory("` + path + `")
    error_caught = False
except:
    error_caught = True
`)
	if err != nil {
		t.Fatalf("Failed to run ensure_directory: %v", err)
	}

	caught, _ := p.GetVar("error_caught")
	if caught.(bool) != true {
		t.Errorf("Expected error when path is a file")
	}
}

func TestEnsureDirectoryWithMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secretdir")

	p := scriptling.New()
	Register(p)

	_, err := p.Eval(`
import scriptling.provision.file as file
status = file.ensure_directory("` + path + `", mode=0o700)
if status != file.CREATED:
    raise Exception("expected CREATED")
`)
	if err != nil {
		t.Fatalf("Failed to run ensure_directory: %v", err)
	}

	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0o700 {
		t.Errorf("Expected mode 0o700, got %o", info.Mode().Perm())
	}
}

func TestAbsentDirectoryRemoves(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "emptydir")
	os.Mkdir(path, 0o755)

	p := scriptling.New()
	Register(p)

	_, err := p.Eval(`
import scriptling.provision.file as file
status = file.absent_directory("` + path + `")
if status != file.REMOVED:
    raise Exception("expected REMOVED")
`)
	if err != nil {
		t.Fatalf("Failed to run absent_directory: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("Expected directory to be removed")
	}
}

func TestAbsentDirectoryAlreadyGone(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent")

	p := scriptling.New()
	Register(p)

	_, err := p.Eval(`
import scriptling.provision.file as file
status = file.absent_directory("` + path + `")
if status != file.ABSENT:
    raise Exception("expected ABSENT")
`)
	if err != nil {
		t.Fatalf("Failed to run absent_directory: %v", err)
	}
}

func TestAbsentDirectoryNotEmpty(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "mydir")
	os.Mkdir(subdir, 0o755)
	os.WriteFile(filepath.Join(subdir, "file.txt"), []byte("data"), 0o644)

	p := scriptling.New()
	Register(p)

	_, err := p.Eval(`
import scriptling.provision.file as file
try:
    file.absent_directory("` + subdir + `")
    error_caught = False
except:
    error_caught = True
`)
	if err != nil {
		t.Fatalf("Failed to run absent_directory: %v", err)
	}

	caught, _ := p.GetVar("error_caught")
	if caught.(bool) != true {
		t.Errorf("Expected error for non-empty directory")
	}
}

// evalBlockScript runs src against a fresh provision.file-enabled evaluator and
// fails the test on any script error. Returns the instance for var inspection.
func evalBlockScript(t *testing.T, src string) *scriptling.Scriptling {
	t.Helper()
	p := scriptling.New()
	Register(p)
	if _, err := p.Eval(src); err != nil {
		t.Fatalf("script failed: %v\n--- script ---\n%s", err, src)
	}
	return p
}

// expectBlockError runs src which must catch an error internally, failing if no
// error was caught.
func expectBlockError(t *testing.T, src string) {
	t.Helper()
	p := evalBlockScript(t, src)
	caught, _ := p.GetVar("error_caught")
	if caught == nil || caught.(bool) != true {
		t.Fatalf("Expected an error to be caught, but none was.\n--- script ---\n%s", src)
	}
}

func TestEnsureBlockCreatesNewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "cfg.conf")

	evalBlockScript(t, `
import scriptling.provision.file as file
status = file.ensure_block("`+path+`", "foo=1\n", id="app")
if status != file.CREATED:
    raise Exception("expected CREATED, got " + status)
`)

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}
	want := "# >>> scriptling managed: app >>>\nfoo=1\n# <<< scriptling managed: app <<<\n"
	if string(raw) != want {
		t.Errorf("unexpected file content:\n got: %q\nwant: %q", string(raw), want)
	}
}

func TestEnsureBlockAppendsDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	os.WriteFile(path, []byte("line1\nline2\n"), 0o644)

	evalBlockScript(t, `
import scriptling.provision.file as file
status = file.ensure_block("`+path+`", "managed\n")
if status != file.CREATED:
    raise Exception("expected CREATED, got " + status)
`)

	raw, _ := os.ReadFile(path)
	want := "line1\nline2\n# >>> scriptling managed: managed >>>\nmanaged\n# <<< scriptling managed: managed <<<\n"
	if string(raw) != want {
		t.Errorf("unexpected content:\n got: %q\nwant: %q", string(raw), want)
	}
}

func TestEnsureBlockPrepend(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	os.WriteFile(path, []byte("existing\n"), 0o644)

	evalBlockScript(t, `
import scriptling.provision.file as file
status = file.ensure_block("`+path+`", "header\n", position="start")
if status != file.CREATED:
    raise Exception("expected CREATED, got " + status)
`)

	raw, _ := os.ReadFile(path)
	want := "# >>> scriptling managed: managed >>>\nheader\n# <<< scriptling managed: managed <<<\nexisting\n"
	if string(raw) != want {
		t.Errorf("unexpected content:\n got: %q\nwant: %q", string(raw), want)
	}
}

func TestEnsureBlockInsertAfterFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	os.WriteFile(path, []byte("alpha\nbeta\ngamma\n"), 0o644)

	evalBlockScript(t, `
import scriptling.provision.file as file
status = file.ensure_block("`+path+`", "injected\n", insert_after="beta")
if status != file.CREATED:
    raise Exception("expected CREATED, got " + status)
`)

	raw, _ := os.ReadFile(path)
	want := "alpha\nbeta\n# >>> scriptling managed: managed >>>\ninjected\n# <<< scriptling managed: managed <<<\ngamma\n"
	if string(raw) != want {
		t.Errorf("unexpected content:\n got: %q\nwant: %q", string(raw), want)
	}
}

func TestEnsureBlockInsertAfterNotFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	os.WriteFile(path, []byte("alpha\n"), 0o644)

	expectBlockError(t, `
import scriptling.provision.file as file
try:
    file.ensure_block("`+path+`", "x\n", insert_after="zzz")
    error_caught = False
except:
    error_caught = True
`)

	raw, _ := os.ReadFile(path)
	if string(raw) != "alpha\n" {
		t.Errorf("file should be unchanged on anchor miss, got %q", string(raw))
	}
}

func TestEnsureBlockInsertAfterPrecedenceOverPosition(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	os.WriteFile(path, []byte("top\nbottom\n"), 0o644)

	evalBlockScript(t, `
import scriptling.provision.file as file
status = file.ensure_block("`+path+`", "mid\n", position="start", insert_after="top")
if status != file.CREATED:
    raise Exception("expected CREATED, got " + status)
`)

	raw, _ := os.ReadFile(path)
	// insert_after wins: block appears after "top", not at start
	want := "top\n# >>> scriptling managed: managed >>>\nmid\n# <<< scriptling managed: managed <<<\nbottom\n"
	if string(raw) != want {
		t.Errorf("unexpected content:\n got: %q\nwant: %q", string(raw), want)
	}
}

func TestEnsureBlockUpdatesContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	os.WriteFile(path, []byte("before\n# >>> scriptling managed: app >>>\nold\n# <<< scriptling managed: app <<<\nafter\n"), 0o644)

	evalBlockScript(t, `
import scriptling.provision.file as file
status = file.ensure_block("`+path+`", "new\n", id="app")
if status != file.UPDATED:
    raise Exception("expected UPDATED, got " + status)
`)

	raw, _ := os.ReadFile(path)
	want := "before\n# >>> scriptling managed: app >>>\nnew\n# <<< scriptling managed: app <<<\nafter\n"
	if string(raw) != want {
		t.Errorf("unexpected content:\n got: %q\nwant: %q", string(raw), want)
	}
}

func TestEnsureBlockUnchangedWhenSame(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	os.WriteFile(path, []byte("preamble\n# >>> scriptling managed: app >>>\nsame\n# <<< scriptling managed: app <<<\n"), 0o644)
	orig, _ := os.ReadFile(path)

	evalBlockScript(t, `
import scriptling.provision.file as file
status = file.ensure_block("`+path+`", "same\n", id="app")
if status != file.UNCHANGED:
    raise Exception("expected UNCHANGED, got " + status)
`)

	raw, _ := os.ReadFile(path)
	if string(raw) != string(orig) {
		t.Errorf("file should be unchanged:\n got: %q\norig: %q", string(raw), string(orig))
	}
}

func TestEnsureBlockUpdatePreservesSurroundingAndTrailingNewline(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	orig := "x\n# >>> scriptling managed: m >>>\na\nb\n# <<< scriptling managed: m <<<\ny\n"
	os.WriteFile(path, []byte(orig), 0o644)

	evalBlockScript(t, `
import scriptling.provision.file as file
status = file.ensure_block("`+path+`", "c\nd\n", id="m")
if status != file.UPDATED:
    raise Exception("expected UPDATED, got " + status)
`)

	want := "x\n# >>> scriptling managed: m >>>\nc\nd\n# <<< scriptling managed: m <<<\ny\n"
	raw, _ := os.ReadFile(path)
	if string(raw) != want {
		t.Errorf("unexpected content:\n got: %q\nwant: %q", string(raw), want)
	}
}

func TestEnsureBlockMultipleIDsCoexist(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	os.WriteFile(path, []byte("base\n"), 0o644)

	evalBlockScript(t, `
import scriptling.provision.file as file
s1 = file.ensure_block("`+path+`", "one\n", id="a")
s2 = file.ensure_block("`+path+`", "two\n", id="b")
if s1 != file.CREATED or s2 != file.CREATED:
    raise Exception("expected both CREATED")
`)

	raw, _ := os.ReadFile(path)
	want := "base\n" +
		"# >>> scriptling managed: a >>>\none\n# <<< scriptling managed: a <<<\n" +
		"# >>> scriptling managed: b >>>\ntwo\n# <<< scriptling managed: b <<<\n"
	if string(raw) != want {
		t.Errorf("unexpected content:\n got: %q\nwant: %q", string(raw), want)
	}
}

func TestEnsureBlockCreateOnlySkipsExistingBlock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	os.WriteFile(path, []byte("# >>> scriptling managed: app >>>\nold\n# <<< scriptling managed: app <<<\n"), 0o644)

	evalBlockScript(t, `
import scriptling.provision.file as file
status = file.ensure_block("`+path+`", "new\n", id="app", create_only=True)
if status != file.UNCHANGED:
    raise Exception("expected UNCHANGED, got " + status)
`)

	raw, _ := os.ReadFile(path)
	if strings.Contains(string(raw), "new") {
		t.Errorf("create_only must not modify block; got %q", string(raw))
	}
}

func TestEnsureBlockCreateOnlyStillCreates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")

	evalBlockScript(t, `
import scriptling.provision.file as file
status = file.ensure_block("`+path+`", "fresh\n", create_only=True)
if status != file.CREATED:
    raise Exception("expected CREATED, got " + status)
`)

	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), "fresh") {
		t.Errorf("create_only should still create new block; got %q", string(raw))
	}
}

func TestEnsureBlockEmptyContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")

	evalBlockScript(t, `
import scriptling.provision.file as file
s1 = file.ensure_block("`+path+`", "", id="e")
if s1 != file.CREATED:
    raise Exception("expected CREATED, got " + s1)
s2 = file.ensure_block("`+path+`", "", id="e")
if s2 != file.UNCHANGED:
    raise Exception("expected UNCHANGED on re-run, got " + s2)
`)

	raw, _ := os.ReadFile(path)
	want := "# >>> scriptling managed: e >>>\n# <<< scriptling managed: e <<<\n"
	if string(raw) != want {
		t.Errorf("unexpected content:\n got: %q\nwant: %q", string(raw), want)
	}
}

func TestEnsureBlockTrailingNewlineNormalization(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")

	// Create with trailing newline included in content
	evalBlockScript(t, `
import scriptling.provision.file as file
file.ensure_block("`+path+`", "foo\n", id="n")
`)

	// Re-call with content lacking the trailing newline; should be UNCHANGED
	p := evalBlockScript(t, `
import scriptling.provision.file as file
status = file.ensure_block("`+path+`", "foo", id="n")
if status != file.UNCHANGED:
    raise Exception("expected UNCHANGED, got " + status)
`)
	_ = p
}

func TestEnsureBlockContentContainsEndMarker(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")

	expectBlockError(t, `
import scriptling.provision.file as file
end_marker = "# <<< scriptling managed: m <<<"
try:
    file.ensure_block("`+path+`", end_marker + "\n", id="m")
    error_caught = False
except:
    error_caught = True
`)
}

func TestEnsureBlockContentContainsBeginMarker(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")

	expectBlockError(t, `
import scriptling.provision.file as file
begin_marker = "# >>> scriptling managed: m >>>"
try:
    file.ensure_block("`+path+`", begin_marker + "\n", id="m")
    error_caught = False
except:
    error_caught = True
`)
}

func TestEnsureBlockOrphanBeginOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	os.WriteFile(path, []byte("# >>> scriptling managed: m >>>\nstuff\n"), 0o644)

	expectBlockError(t, `
import scriptling.provision.file as file
try:
    file.ensure_block("`+path+`", "x\n", id="m")
    error_caught = False
except:
    error_caught = True
`)
}

func TestEnsureBlockOrphanEndOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	os.WriteFile(path, []byte("stuff\n# <<< scriptling managed: m <<<\n"), 0o644)

	expectBlockError(t, `
import scriptling.provision.file as file
try:
    file.ensure_block("`+path+`", "x\n", id="m")
    error_caught = False
except:
    error_caught = True
`)
}

func TestEnsureBlockOrphanTwoBegins(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	os.WriteFile(path, []byte("# >>> scriptling managed: m >>>\n# >>> scriptling managed: m >>>\n# <<< scriptling managed: m <<<\n"), 0o644)

	expectBlockError(t, `
import scriptling.provision.file as file
try:
    file.ensure_block("`+path+`", "x\n", id="m")
    error_caught = False
except:
    error_caught = True
`)
}

func TestEnsureBlockInvalidIDNewline(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")

	expectBlockError(t, `
import scriptling.provision.file as file
try:
    file.ensure_block("`+path+`", "x\n", id="bad\nid")
    error_caught = False
except:
    error_caught = True
`)
}

func TestEnsureBlockEmptyID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")

	expectBlockError(t, `
import scriptling.provision.file as file
try:
    file.ensure_block("`+path+`", "x\n", id="")
    error_caught = False
except:
    error_caught = True
`)
}

func TestEnsureBlockEmptyComment(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")

	expectBlockError(t, `
import scriptling.provision.file as file
try:
    file.ensure_block("`+path+`", "x\n", comment="")
    error_caught = False
except:
    error_caught = True
`)
}

func TestEnsureBlockInvalidPosition(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")

	expectBlockError(t, `
import scriptling.provision.file as file
try:
    file.ensure_block("`+path+`", "x\n", position="middle")
    error_caught = False
except:
    error_caught = True
`)
}

func TestEnsureBlockCustomComment(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")

	evalBlockScript(t, `
import scriptling.provision.file as file
file.ensure_block("`+path+`", "x\n", comment="//", id="js")
`)

	raw, _ := os.ReadFile(path)
	want := "// >>> scriptling managed: js >>>\nx\n// <<< scriptling managed: js <<<\n"
	if string(raw) != want {
		t.Errorf("unexpected content:\n got: %q\nwant: %q", string(raw), want)
	}
}

func TestEnsureBlockModeOnNewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secret.txt")

	evalBlockScript(t, `
import scriptling.provision.file as file
file.ensure_block("`+path+`", "x\n", mode=0o600)
`)

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("expected mode 0o600, got %o", info.Mode().Perm())
	}
}

func TestEnsureBlockCreatesParentDirs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a", "b", "c", "f.txt")

	evalBlockScript(t, `
import scriptling.provision.file as file
file.ensure_block("`+path+`", "x\n")
`)

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file not created at nested path: %v", err)
	}
}

func TestEnsureBlockWrongArgCount(t *testing.T) {
	expectBlockError(t, `
import scriptling.provision.file as file
try:
    file.ensure_block("/tmp/whatever")
    error_caught = False
except:
    error_caught = True
`)
}

func TestEnsureBlockNewFileHasTrailingNewline(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")

	evalBlockScript(t, `
import scriptling.provision.file as file
file.ensure_block("`+path+`", "x\n")
`)

	raw, _ := os.ReadFile(path)
	if !strings.HasSuffix(string(raw), "\n") {
		t.Errorf("new file should end with newline; got %q", string(raw))
	}
}

func TestEnsureBlockAppendToNoTrailingNewline(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	os.WriteFile(path, []byte("nofinalnl"), 0o644)

	evalBlockScript(t, `
import scriptling.provision.file as file
file.ensure_block("`+path+`", "x\n")
`)

	raw, _ := os.ReadFile(path)
	// The original file's trailing-newline state (absent) is preserved: the
	// block still starts on its own line, but no new trailing newline is added.
	want := "nofinalnl\n# >>> scriptling managed: managed >>>\nx\n# <<< scriptling managed: managed <<<"
	if string(raw) != want {
		t.Errorf("unexpected content:\n got: %q\nwant: %q", string(raw), want)
	}
}

func TestAbsentBlockRemoves(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	os.WriteFile(path, []byte("before\n# >>> scriptling managed: app >>>\ngone\n# <<< scriptling managed: app <<<\nafter\n"), 0o644)

	evalBlockScript(t, `
import scriptling.provision.file as file
status = file.absent_block("`+path+`", id="app")
if status != file.REMOVED:
    raise Exception("expected REMOVED, got " + status)
`)

	raw, _ := os.ReadFile(path)
	want := "before\nafter\n"
	if string(raw) != want {
		t.Errorf("unexpected content after removal:\n got: %q\nwant: %q", string(raw), want)
	}
}

func TestAbsentBlockNoBlockPresent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	os.WriteFile(path, []byte("nothing managed here\n"), 0o644)

	evalBlockScript(t, `
import scriptling.provision.file as file
status = file.absent_block("`+path+`", id="app")
if status != file.UNCHANGED:
    raise Exception("expected UNCHANGED, got " + status)
`)
}

func TestAbsentBlockFileDoesNotExist(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing.txt")

	evalBlockScript(t, `
import scriptling.provision.file as file
status = file.absent_block("`+path+`", id="app")
if status != file.UNCHANGED:
    raise Exception("expected UNCHANGED, got " + status)
`)
}

func TestAbsentBlockOrphanMarkers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	os.WriteFile(path, []byte("# >>> scriptling managed: m >>>\nstuff\n"), 0o644)

	expectBlockError(t, `
import scriptling.provision.file as file
try:
    file.absent_block("`+path+`", id="m")
    error_caught = False
except:
    error_caught = True
`)
}

func TestAbsentBlockRemovesOnlyTargetID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	os.WriteFile(path, []byte(
		"# >>> scriptling managed: a >>>\none\n# <<< scriptling managed: a <<<\n"+
			"# >>> scriptling managed: b >>>\ntwo\n# <<< scriptling managed: b <<<\n"), 0o644)

	evalBlockScript(t, `
import scriptling.provision.file as file
status = file.absent_block("`+path+`", id="a")
if status != file.REMOVED:
    raise Exception("expected REMOVED, got " + status)
`)

	raw, _ := os.ReadFile(path)
	want := "# >>> scriptling managed: b >>>\ntwo\n# <<< scriptling managed: b <<<\n"
	if string(raw) != want {
		t.Errorf("unexpected content:\n got: %q\nwant: %q", string(raw), want)
	}
}

func TestAbsentBlockCustomComment(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	os.WriteFile(path, []byte("// >>> scriptling managed: js >>>\nx\n// <<< scriptling managed: js <<<\n"), 0o644)

	evalBlockScript(t, `
import scriptling.provision.file as file
status = file.absent_block("`+path+`", id="js", comment="//")
if status != file.REMOVED:
    raise Exception("expected REMOVED, got " + status)
`)

	raw, _ := os.ReadFile(path)
	if string(raw) != "" {
		t.Errorf("expected empty file after removal, got %q", string(raw))
	}
}

func TestAbsentBlockIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	os.WriteFile(path, []byte("plain\n"), 0o644)

	evalBlockScript(t, `
import scriptling.provision.file as file
s1 = file.absent_block("`+path+`", id="m")
s2 = file.absent_block("`+path+`", id="m")
if s1 != file.UNCHANGED or s2 != file.UNCHANGED:
    raise Exception("expected both UNCHANGED")
`)
}

func TestAbsentBlockPreservesFileMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	os.WriteFile(path, []byte("# >>> scriptling managed: m >>>\nx\n# <<< scriptling managed: m <<<\n"), 0o600)

	evalBlockScript(t, `
import scriptling.provision.file as file
file.absent_block("`+path+`", id="m")
`)

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("mode should be preserved as 0o600, got %o", info.Mode().Perm())
	}
}

func TestAbsentBlockWrongArgCount(t *testing.T) {
	expectBlockError(t, `
import scriptling.provision.file as file
try:
    file.absent_block()
    error_caught = False
except:
    error_caught = True
`)
}
