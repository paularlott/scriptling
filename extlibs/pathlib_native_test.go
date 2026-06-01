package extlibs

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/fssecurity"
	"github.com/paularlott/scriptling/object"
)

func TestPathMethodsRejectMissingNativeData(t *testing.T) {
	lib := NewPathlibLibrary(fssecurity.Config{})
	pathClassObj := lib.Constants()["PathClass"]
	pathClass, ok := pathClassObj.(*object.Class)
	if !ok {
		t.Fatalf("expected PathClass constant, got %T", pathClassObj)
	}

	path := &object.Instance{
		Class:  pathClass,
		Fields: map[string]object.Object{},
	}

	for _, name := range []string{"joinpath", "exists"} {
		method, ok := pathClass.Methods[name].(*object.Builtin)
		if !ok {
			t.Fatalf("expected %s builtin, got %T", name, pathClass.Methods[name])
		}

		result := method.Fn(context.Background(), object.NewKwargs(nil), path)
		errObj, ok := result.(*object.Error)
		if !ok {
			t.Fatalf("%s returned %T, expected error", name, result)
		}
		if !strings.Contains(errObj.Message, "invalid native data") {
			t.Fatalf("%s error = %q, expected invalid native data", name, errObj.Message)
		}
	}
}

func TestPathlibChmodAndMkdirMode(t *testing.T) {
	tmp := t.TempDir()
	allowedTmp, err := filepath.EvalSymlinks(tmp)
	if err != nil {
		t.Fatalf("eval temp dir: %v", err)
	}
	tmp = allowedTmp
	dir := filepath.Join(tmp, "mode-dir")
	nested := filepath.Join(tmp, "nested", "child")
	file := filepath.Join(tmp, "file.txt")

	p := scriptling.New()
	RegisterPathlibLibrary(p, []string{allowedTmp})

	code := `import pathlib
d = pathlib.Path("` + dir + `")
d.mkdir(0o700)
n = pathlib.Path("` + nested + `")
n.mkdir(parents=True, exist_ok=True)
f = pathlib.Path("` + file + `")
f.write_text("content")
f.chmod(mode=0o600)
d.chmod(0o711)
True`
	result, err := p.Eval(code)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	if boolean, ok := result.(*object.Boolean); !ok || !boolean.BoolValue() {
		t.Fatalf("expected True, got %T %v", result, result)
	}

	fileInfo, err := os.Stat(file)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	if got := fileInfo.Mode().Perm(); got != 0600 {
		t.Fatalf("file mode = %o, want 0600", got)
	}
	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if got := dirInfo.Mode().Perm(); got != 0711 {
		t.Fatalf("dir mode = %o, want 0711", got)
	}
}

func TestPathlibReadWriteBytes(t *testing.T) {
	tmp := t.TempDir()
	allowedTmp, err := filepath.EvalSymlinks(tmp)
	if err != nil {
		t.Fatalf("eval temp dir: %v", err)
	}
	file := filepath.Join(allowedTmp, "data.bin")

	p := scriptling.New()
	RegisterPathlibLibrary(p, []string{allowedTmp})

	code := `import pathlib
p = pathlib.Path("` + file + `")
p.write_bytes("ABC")
p.read_bytes() == "ABC"`
	result, err := p.Eval(code)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	if boolean, ok := result.(*object.Boolean); !ok || !boolean.BoolValue() {
		t.Fatalf("expected True, got %T %v", result, result)
	}
	content, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(content) != "ABC" {
		t.Fatalf("file content = %q, want binary payload", string(content))
	}
}
