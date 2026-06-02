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

func TestPathlibCopyFile(t *testing.T) {
	tmp := t.TempDir()
	allowedTmp, err := filepath.EvalSymlinks(tmp)
	if err != nil {
		t.Fatalf("eval temp dir: %v", err)
	}
	src := filepath.Join(allowedTmp, "original.txt")
	dst := filepath.Join(allowedTmp, "copy.txt")

	p := scriptling.New()
	RegisterPathlibLibrary(p, []string{allowedTmp})

	code := `import pathlib
src = pathlib.Path("` + src + `")
src.write_text("hello world")
result = src.copy("` + dst + `")
result.name`
	result, err := p.Eval(code)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	if s, ok := result.(*object.String); !ok || s.StringValue() != "copy.txt" {
		t.Fatalf("expected result.name = 'copy.txt', got %T %v", result, result)
	}

	content, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read copied file: %v", err)
	}
	if string(content) != "hello world" {
		t.Fatalf("copied content = %q, want %q", string(content), "hello world")
	}

	originalContent, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read original file: %v", err)
	}
	if string(originalContent) != "hello world" {
		t.Fatalf("original should still exist")
	}
}

func TestPathlibCopyDirectory(t *testing.T) {
	tmp := t.TempDir()
	allowedTmp, err := filepath.EvalSymlinks(tmp)
	if err != nil {
		t.Fatalf("eval temp dir: %v", err)
	}
	srcDir := filepath.Join(allowedTmp, "srcdir")
	dstDir := filepath.Join(allowedTmp, "dstdir")

	os.MkdirAll(filepath.Join(srcDir, "sub"), 0755)
	os.WriteFile(filepath.Join(srcDir, "a.txt"), []byte("aaa"), 0644)
	os.WriteFile(filepath.Join(srcDir, "sub", "b.txt"), []byte("bbb"), 0644)

	p := scriptling.New()
	RegisterPathlibLibrary(p, []string{allowedTmp})

	code := `import pathlib
src = pathlib.Path("` + srcDir + `")
result = src.copy("` + dstDir + `")
result.name`
	result, err := p.Eval(code)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	if s, ok := result.(*object.String); !ok || s.StringValue() != "dstdir" {
		t.Fatalf("expected result.name = 'dstdir', got %T %v", result, result)
	}

	content, err := os.ReadFile(filepath.Join(dstDir, "a.txt"))
	if err != nil {
		t.Fatalf("read copied a.txt: %v", err)
	}
	if string(content) != "aaa" {
		t.Fatalf("a.txt content = %q, want %q", string(content), "aaa")
	}

	subContent, err := os.ReadFile(filepath.Join(dstDir, "sub", "b.txt"))
	if err != nil {
		t.Fatalf("read copied sub/b.txt: %v", err)
	}
	if string(subContent) != "bbb" {
		t.Fatalf("sub/b.txt content = %q, want %q", string(subContent), "bbb")
	}
}

func TestPathlibCopySecurity(t *testing.T) {
	allowedDir := t.TempDir()
	deniedDir := t.TempDir()
	src := filepath.Join(allowedDir, "file.txt")
	os.WriteFile(src, []byte("data"), 0644)

	p := scriptling.New()
	RegisterPathlibLibrary(p, []string{allowedDir})

	_, err := p.Eval(`import pathlib
pathlib.Path("` + src + `").copy("` + filepath.Join(deniedDir, "outside.txt") + `")`)
	if err == nil {
		t.Fatal("expected error copying outside allowed paths")
	}
}

func TestPathlibRenameFile(t *testing.T) {
	tmp := t.TempDir()
	allowedTmp, err := filepath.EvalSymlinks(tmp)
	if err != nil {
		t.Fatalf("eval temp dir: %v", err)
	}
	src := filepath.Join(allowedTmp, "old.txt")
	dst := filepath.Join(allowedTmp, "new.txt")

	p := scriptling.New()
	RegisterPathlibLibrary(p, []string{allowedTmp})

	code := `import pathlib
src = pathlib.Path("` + src + `")
src.write_text("rename me")
result = src.rename("` + dst + `")
result.name`
	result, err := p.Eval(code)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	if s, ok := result.(*object.String); !ok || s.StringValue() != "new.txt" {
		t.Fatalf("expected result.name = 'new.txt', got %T %v", result, result)
	}

	content, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read renamed file: %v", err)
	}
	if string(content) != "rename me" {
		t.Fatalf("renamed content = %q, want %q", string(content), "rename me")
	}

	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatal("source file should no longer exist after rename")
	}
}

func TestPathlibRenameDirectory(t *testing.T) {
	tmp := t.TempDir()
	allowedTmp, err := filepath.EvalSymlinks(tmp)
	if err != nil {
		t.Fatalf("eval temp dir: %v", err)
	}
	srcDir := filepath.Join(allowedTmp, "olddir")
	dstDir := filepath.Join(allowedTmp, "newdir")

	os.MkdirAll(srcDir, 0755)
	os.WriteFile(filepath.Join(srcDir, "file.txt"), []byte("inside"), 0644)

	p := scriptling.New()
	RegisterPathlibLibrary(p, []string{allowedTmp})

	code := `import pathlib
src = pathlib.Path("` + srcDir + `")
result = src.rename("` + dstDir + `")
result.name`
	result, err := p.Eval(code)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	if s, ok := result.(*object.String); !ok || s.StringValue() != "newdir" {
		t.Fatalf("expected result.name = 'newdir', got %T %v", result, result)
	}

	content, err := os.ReadFile(filepath.Join(dstDir, "file.txt"))
	if err != nil {
		t.Fatalf("read renamed dir file: %v", err)
	}
	if string(content) != "inside" {
		t.Fatalf("file content = %q, want %q", string(content), "inside")
	}

	if _, err := os.Stat(srcDir); !os.IsNotExist(err) {
		t.Fatal("source dir should no longer exist after rename")
	}
}

func TestPathlibRenameSecurity(t *testing.T) {
	allowedDir := t.TempDir()
	deniedDir := t.TempDir()
	src := filepath.Join(allowedDir, "file.txt")
	os.WriteFile(src, []byte("data"), 0644)

	p := scriptling.New()
	RegisterPathlibLibrary(p, []string{allowedDir})

	_, err := p.Eval(`import pathlib
pathlib.Path("` + src + `").rename("` + filepath.Join(deniedDir, "outside.txt") + `")`)
	if err == nil {
		t.Fatal("expected error renaming outside allowed paths")
	}
}

func TestPathlibIterdir(t *testing.T) {
	tmp := t.TempDir()
	allowedTmp, err := filepath.EvalSymlinks(tmp)
	if err != nil {
		t.Fatalf("eval temp dir: %v", err)
	}

	os.MkdirAll(filepath.Join(allowedTmp, "subdir"), 0755)
	os.WriteFile(filepath.Join(allowedTmp, "a.txt"), []byte("aaa"), 0644)
	os.WriteFile(filepath.Join(allowedTmp, "b.txt"), []byte("bbb"), 0644)

	p := scriptling.New()
	RegisterPathlibLibrary(p, []string{allowedTmp})

	code := `import pathlib
d = pathlib.Path("` + allowedTmp + `")
children = d.iterdir()
len(children)`
	result, err := p.Eval(code)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	if n, ok := result.(*object.Integer); !ok || n.IntValue() != 3 {
		t.Fatalf("expected 3 children, got %T %v", result, result)
	}

	code2 := `import pathlib
d = pathlib.Path("` + allowedTmp + `")
names = []
for child in d.iterdir():
    names.append(child.name)
sorted(names)`
	result2, err := p.Eval(code2)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	list, ok := result2.(*object.List)
	if !ok {
		t.Fatalf("expected list, got %T", result2)
	}
	if len(list.Elements) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(list.Elements))
	}
	expectedNames := map[string]bool{"a.txt": true, "b.txt": true, "subdir": true}
	for _, el := range list.Elements {
		s, ok := el.(*object.String)
		if !ok {
			t.Fatalf("expected string, got %T", el)
		}
		if !expectedNames[s.StringValue()] {
			t.Fatalf("unexpected name: %s", s.StringValue())
		}
		delete(expectedNames, s.StringValue())
	}
	if len(expectedNames) != 0 {
		t.Fatalf("missing names: %v", expectedNames)
	}
}

func TestPathlibIterdirReturnsPaths(t *testing.T) {
	tmp := t.TempDir()
	allowedTmp, err := filepath.EvalSymlinks(tmp)
	if err != nil {
		t.Fatalf("eval temp dir: %v", err)
	}

	os.WriteFile(filepath.Join(allowedTmp, "file.txt"), []byte("data"), 0644)

	p := scriptling.New()
	RegisterPathlibLibrary(p, []string{allowedTmp})

	code := `import pathlib
d = pathlib.Path("` + allowedTmp + `")
child = d.iterdir()[0]
child.is_file()`
	result, err := p.Eval(code)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	if b, ok := result.(*object.Boolean); !ok || !b.BoolValue() {
		t.Fatalf("expected True, got %T %v", result, result)
	}
}

func TestPathlibIterdirNotExists(t *testing.T) {
	tmp := t.TempDir()
	allowedTmp, err := filepath.EvalSymlinks(tmp)
	if err != nil {
		t.Fatalf("eval temp dir: %v", err)
	}

	p := scriptling.New()
	RegisterPathlibLibrary(p, []string{allowedTmp})

	_, err = p.Eval(`import pathlib
pathlib.Path("` + filepath.Join(allowedTmp, "nope") + `").iterdir()`)
	if err == nil {
		t.Fatal("expected error for non-existent directory")
	}
}

func TestPathlibIterdirSecurity(t *testing.T) {
	allowedDir := t.TempDir()
	deniedDir := t.TempDir()

	p := scriptling.New()
	RegisterPathlibLibrary(p, []string{allowedDir})

	_, err := p.Eval(`import pathlib
pathlib.Path("` + deniedDir + `").iterdir()`)
	if err == nil {
		t.Fatal("expected error for denied directory")
	}
}

func TestPathlibGlob(t *testing.T) {
	tmp := t.TempDir()
	allowedTmp, err := filepath.EvalSymlinks(tmp)
	if err != nil {
		t.Fatalf("eval temp dir: %v", err)
	}

	os.WriteFile(filepath.Join(allowedTmp, "a.txt"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(allowedTmp, "b.txt"), []byte("b"), 0644)
	os.WriteFile(filepath.Join(allowedTmp, "c.py"), []byte("c"), 0644)

	p := scriptling.New()
	RegisterPathlibLibrary(p, []string{allowedTmp})

	code := `import pathlib
d = pathlib.Path("` + allowedTmp + `")
matches = d.glob("*.txt")
len(matches)`
	result, err := p.Eval(code)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	if n, ok := result.(*object.Integer); !ok || n.IntValue() != 2 {
		t.Fatalf("expected 2 matches, got %T %v", result, result)
	}
}

func TestPathlibGlobReturnsPaths(t *testing.T) {
	tmp := t.TempDir()
	allowedTmp, err := filepath.EvalSymlinks(tmp)
	if err != nil {
		t.Fatalf("eval temp dir: %v", err)
	}

	os.WriteFile(filepath.Join(allowedTmp, "test.txt"), []byte("data"), 0644)

	p := scriptling.New()
	RegisterPathlibLibrary(p, []string{allowedTmp})

	code := `import pathlib
d = pathlib.Path("` + allowedTmp + `")
child = d.glob("*.txt")[0]
child.is_file()`
	result, err := p.Eval(code)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	if b, ok := result.(*object.Boolean); !ok || !b.BoolValue() {
		t.Fatalf("expected True, got %T %v", result, result)
	}
}

func TestPathlibGlobNames(t *testing.T) {
	tmp := t.TempDir()
	allowedTmp, err := filepath.EvalSymlinks(tmp)
	if err != nil {
		t.Fatalf("eval temp dir: %v", err)
	}

	os.WriteFile(filepath.Join(allowedTmp, "a.txt"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(allowedTmp, "b.txt"), []byte("b"), 0644)
	os.WriteFile(filepath.Join(allowedTmp, "c.py"), []byte("c"), 0644)

	p := scriptling.New()
	RegisterPathlibLibrary(p, []string{allowedTmp})

	code := `import pathlib
d = pathlib.Path("` + allowedTmp + `")
names = []
for m in d.glob("*.txt"):
    names.append(m.name)
sorted(names)`
	result, err := p.Eval(code)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	list, ok := result.(*object.List)
	if !ok {
		t.Fatalf("expected list, got %T", result)
	}
	if len(list.Elements) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(list.Elements))
	}
	first := list.Elements[0].(*object.String).StringValue()
	second := list.Elements[1].(*object.String).StringValue()
	if first != "a.txt" || second != "b.txt" {
		t.Fatalf("expected [a.txt, b.txt], got [%s, %s]", first, second)
	}
}

func TestPathlibGlobRecursive(t *testing.T) {
	tmp := t.TempDir()
	allowedTmp, err := filepath.EvalSymlinks(tmp)
	if err != nil {
		t.Fatalf("eval temp dir: %v", err)
	}

	os.MkdirAll(filepath.Join(allowedTmp, "sub"), 0755)
	os.WriteFile(filepath.Join(allowedTmp, "a.txt"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(allowedTmp, "sub", "b.txt"), []byte("b"), 0644)
	os.WriteFile(filepath.Join(allowedTmp, "sub", "c.py"), []byte("c"), 0644)

	p := scriptling.New()
	RegisterPathlibLibrary(p, []string{allowedTmp})

	code := `import pathlib
d = pathlib.Path("` + allowedTmp + `")
matches = d.glob("**/*.txt")
len(matches)`
	result, err := p.Eval(code)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	if n, ok := result.(*object.Integer); !ok || n.IntValue() != 2 {
		t.Fatalf("expected 2 matches, got %T %v", result, result)
	}
}

func TestPathlibGlobNoMatch(t *testing.T) {
	tmp := t.TempDir()
	allowedTmp, err := filepath.EvalSymlinks(tmp)
	if err != nil {
		t.Fatalf("eval temp dir: %v", err)
	}

	p := scriptling.New()
	RegisterPathlibLibrary(p, []string{allowedTmp})

	code := `import pathlib
d = pathlib.Path("` + allowedTmp + `")
matches = d.glob("*.xyz")
len(matches)`
	result, err := p.Eval(code)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	if n, ok := result.(*object.Integer); !ok || n.IntValue() != 0 {
		t.Fatalf("expected 0 matches, got %T %v", result, result)
	}
}

func TestPathlibGlobSecurity(t *testing.T) {
	allowedDir := t.TempDir()
	deniedDir := t.TempDir()

	p := scriptling.New()
	RegisterPathlibLibrary(p, []string{allowedDir})

	_, err := p.Eval(`import pathlib
pathlib.Path("` + deniedDir + `").glob("*.txt")`)
	if err == nil {
		t.Fatal("expected error for denied directory")
	}
}
