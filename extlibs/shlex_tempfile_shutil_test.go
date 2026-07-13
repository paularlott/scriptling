package extlibs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/object"
)

// ===================== shlex =====================

func TestShlexQuote(t *testing.T) {
	cases := map[string]string{
		"":          "''",
		"safe":      "safe",
		"file.txt":  "file.txt",
		"a/b/c-d_e": "a/b/c-d_e",
		"has space": "'has space'",
		"it's":      "'it'\"'\"'s'",
		"$HOME":     "'$HOME'",
		"a@b.com":   "a@b.com",
		"100%":      "100%",
	}
	for input, want := range cases {
		got := shlexQuote(input)
		if got != want {
			t.Errorf("quote(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestShlexSplit(t *testing.T) {
	cases := []struct {
		input string
		want  []string
	}{
		{"a b c", []string{"a", "b", "c"}},
		{"  spaced  out  ", []string{"spaced", "out"}},
		{`'single quoted'`, []string{"single quoted"}},
		{`"double quoted"`, []string{"double quoted"}},
		{`arg\ with\ spaces`, []string{"arg with spaces"}},
		{`--flag="quoted value"`, []string{"--flag=quoted value"}},
		{`one 'two three' four`, []string{"one", "two three", "four"}},
		{``, nil},
	}
	for _, c := range cases {
		got, err := shlexSplit(c.input)
		if err != nil {
			t.Errorf("split(%q) error: %v", c.input, err)
			continue
		}
		if !equalStrSlices(got, c.want) {
			t.Errorf("split(%q) = %v, want %v", c.input, got, c.want)
		}
	}
}

func TestShlexSplitErrors(t *testing.T) {
	if _, err := shlexSplit("unterminated 'quote"); err == nil {
		t.Error("expected error for unterminated single quote")
	}
	if _, err := shlexSplit(`unterminated "quote`); err == nil {
		t.Error("expected error for unterminated double quote")
	}
	if _, err := shlexSplit("trailing\\"); err == nil {
		t.Error("expected error for trailing backslash")
	}
}

func TestShlexJoin(t *testing.T) {
	got := shlexJoin([]string{"safe", "has space", ""})
	want := "safe 'has space' ''"
	if got != want {
		t.Errorf("join = %q, want %q", got, want)
	}
}

func TestShlexBuiltinRoundTrip(t *testing.T) {
	p := scriptling.New()
	RegisterShlexLibrary(p)

	result, err := p.Eval(`import shlex
shlex.join(shlex.split("cmd --flag 'my value'"))`)
	if err != nil {
		t.Fatal(err)
	}
	s, _ := result.AsString()
	if s != `cmd --flag 'my value'` {
		t.Errorf("round trip: got %q", s)
	}
}

// ===================== tempfile =====================

func TestTempfileMkstemp(t *testing.T) {
	p := scriptling.New()
	RegisterTempfileLibrary(p, nil)

	result, err := p.Eval(`import tempfile
tempfile.mkstemp(prefix="test_")`)
	if err != nil {
		t.Fatal(err)
	}
	path, _ := result.AsString()
	if path == "" {
		t.Fatal("mkstemp should return a path")
	}
	info, statErr := os.Stat(path)
	if statErr != nil || info.IsDir() {
		t.Error("mkstemp should create a regular file")
	}
	os.Remove(path)
}

func TestTempfileMkdtemp(t *testing.T) {
	p := scriptling.New()
	RegisterTempfileLibrary(p, nil)

	result, err := p.Eval(`import tempfile
tempfile.mkdtemp(prefix="testdir_")`)
	if err != nil {
		t.Fatal(err)
	}
	path, _ := result.AsString()
	if path == "" {
		t.Fatal("mkdtemp should return a path")
	}
	info, statErr := os.Stat(path)
	if statErr != nil || !info.IsDir() {
		t.Error("mkdtemp should create a directory")
	}
	os.RemoveAll(path)
}

func TestTempfileSuffix(t *testing.T) {
	p := scriptling.New()
	RegisterTempfileLibrary(p, nil)

	result, err := p.Eval(`import tempfile
f = tempfile.mkstemp(prefix="data_", suffix=".json")
f.endswith(".json")`)
	if err != nil {
		t.Fatal(err)
	}
	if !evalBool(result) {
		t.Error("mkstemp should respect suffix")
	}
}

func TestTempfileSecurityRestricted(t *testing.T) {
	allowed := t.TempDir()
	p := scriptling.New()
	RegisterTempfileLibrary(p, []string{allowed})

	// mkstemp with explicit dir outside allowed
	other := t.TempDir()
	_, err := p.Eval(`import tempfile
tempfile.mkstemp(dir="` + other + `")`)
	if err == nil {
		t.Error("expected error for temp file outside allowed paths")
	}

	// mkdtemp with no dir should succeed in allowed path
	result, err := p.Eval(`import tempfile
d = tempfile.mkdtemp()
d.startswith("` + allowed + `")`)
	if err != nil {
		t.Fatal(err)
	}
	if !evalBool(result) {
		t.Error("mkdtemp should create within allowed path when no dir given")
	}
}

func TestTempfileGettempdir(t *testing.T) {
	p := scriptling.New()
	RegisterTempfileLibrary(p, nil)

	result, err := p.Eval(`import tempfile
len(tempfile.gettempdir()) > 0`)
	if err != nil {
		t.Fatal(err)
	}
	if !evalBool(result) {
		t.Error("gettempdir should return a non-empty string")
	}
}

// ===================== shutil =====================

func setupShutilTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "sub"), 0755)
	os.WriteFile(filepath.Join(root, "a.txt"), []byte("hello"), 0644)
	os.WriteFile(filepath.Join(root, "sub", "b.txt"), []byte("world"), 0644)
	return root
}

func TestShutilCopyFile(t *testing.T) {
	root := setupShutilTree(t)
	dst := filepath.Join(root, "copy.txt")
	p := scriptling.New()
	RegisterShutilLibrary(p, nil)

	_, err := p.Eval(`import shutil
shutil.copy("` + filepath.Join(root, "a.txt") + `", "` + dst + `")`)
	if err != nil {
		t.Fatal(err)
	}
	if _, e := os.Stat(dst); e != nil {
		t.Error("copy should create the destination file")
	}
}

func TestShutilCopyTree(t *testing.T) {
	root := setupShutilTree(t)
	dst := filepath.Join(root, "copied")
	p := scriptling.New()
	RegisterShutilLibrary(p, nil)

	_, err := p.Eval(`import shutil
shutil.copytree("` + filepath.Join(root, "sub") + `", "` + dst + `")`)
	if err != nil {
		t.Fatal(err)
	}
	if _, e := os.Stat(filepath.Join(dst, "b.txt")); e != nil {
		t.Error("copytree should copy subdirectory contents")
	}
}

func TestShutilRmtree(t *testing.T) {
	root := setupShutilTree(t)
	sub := filepath.Join(root, "sub")
	p := scriptling.New()
	RegisterShutilLibrary(p, nil)

	_, err := p.Eval(`import shutil
shutil.rmtree("` + sub + `")`)
	if err != nil {
		t.Fatal(err)
	}
	if _, e := os.Stat(sub); !os.IsNotExist(e) {
		t.Error("rmtree should remove the directory")
	}
}

func TestShutilMove(t *testing.T) {
	root := setupShutilTree(t)
	src := filepath.Join(root, "a.txt")
	dst := filepath.Join(root, "moved.txt")
	p := scriptling.New()
	RegisterShutilLibrary(p, nil)

	_, err := p.Eval(`import shutil
shutil.move("` + src + `", "` + dst + `")`)
	if err != nil {
		t.Fatal(err)
	}
	if _, e := os.Stat(dst); e != nil {
		t.Error("move should create the destination")
	}
	if _, e := os.Stat(src); !os.IsNotExist(e) {
		t.Error("move should remove the source")
	}
}

func TestShutilDiskUsage(t *testing.T) {
	p := scriptling.New()
	RegisterShutilLibrary(p, nil)

	result, err := p.Eval(`import shutil
du = shutil.disk_usage("/")
du["total"] > 0 and du["free"] > 0`)
	if err != nil {
		t.Fatal(err)
	}
	if !evalBool(result) {
		t.Error("disk_usage should return positive values")
	}
}

func TestShutilSecurityDenied(t *testing.T) {
	allowed := t.TempDir()
	denied := t.TempDir()
	src := filepath.Join(denied, "secret.txt")
	os.WriteFile(src, []byte("x"), 0644)

	p := scriptling.New()
	RegisterShutilLibrary(p, []string{allowed})

	_, err := p.Eval(`import shutil
shutil.copy("` + src + `", "` + filepath.Join(allowed, "out.txt") + `")`)
	if err == nil {
		t.Error("expected permission error copying from denied path")
	}

	_, err = p.Eval(`import shutil
shutil.rmtree("` + denied + `")`)
	if err == nil {
		t.Error("expected permission error rmtree on denied path")
	}
}

// evalBool is a test helper that coerces an Object to bool.
func evalBool(obj object.Object) bool {
	b, _ := obj.AsBool()
	return b
}

// equalStrSlices is a test helper.
func equalStrSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
