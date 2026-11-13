package filesystem

import (
	"archive/tar"
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/fssecurity"
	"github.com/paularlott/scriptling/object"
)

// limitsTestLimit is the value limit used by the tests in this file: small
// enough that a few KB of data goes over it.
const limitsTestLimit = 1024

// limitsBigSize is comfortably over limitsTestLimit.
const limitsBigSize = 4096

func TestLimitsFileRead(t *testing.T) {
	dir := t.TempDir()
	smallPath := filepath.Join(dir, "small.txt")
	bigPath := filepath.Join(dir, "big.txt")
	small := strings.Repeat("s", 200)
	if err := os.WriteFile(smallPath, []byte(small), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bigPath, []byte(strings.Repeat("B", limitsBigSize)), 0o644); err != nil {
		t.Fatal(err)
	}

	// Exactly at the limit is allowed.
	exactPath := filepath.Join(dir, "exact.txt")
	if err := os.WriteFile(exactPath, []byte(strings.Repeat("e", limitsTestLimit)), 0o644); err != nil {
		t.Fatal(err)
	}

	newP := func(t *testing.T, limit int64) *scriptling.Scriptling {
		p := limitsNewInterp(t, limit)
		RegisterOS(p, []string{dir})
		return p
	}

	t.Run("read_file within limit", func(t *testing.T) {
		got := limitsCatch(t, newP(t, limitsTestLimit), "import os", `os.read_file("`+smallPath+`")`)
		limitsExpectOK(t, got, small)
	})

	t.Run("read_file exactly at limit", func(t *testing.T) {
		got := limitsCatch(t, newP(t, limitsTestLimit), "import os", `len(os.read_file("`+exactPath+`"))`)
		limitsExpectOK(t, got, "1024")
	})

	t.Run("read_file over limit", func(t *testing.T) {
		got := limitsCatch(t, newP(t, limitsTestLimit), "import os", `len(os.read_file("`+bigPath+`"))`)
		limitsExpectMemoryError(t, got, "too large")
	})

	t.Run("read_bytes over limit", func(t *testing.T) {
		got := limitsCatch(t, newP(t, limitsTestLimit), "import os", `len(os.read_bytes("`+bigPath+`"))`)
		limitsExpectMemoryError(t, got, "too large")
	})

	t.Run("no limit", func(t *testing.T) {
		got := limitsCatch(t, newP(t, 0), "import os", `len(os.read_file("`+bigPath+`"))`)
		limitsExpectOK(t, got, "4096")
	})

	t.Run("missing file is not a MemoryError", func(t *testing.T) {
		p := newP(t, limitsTestLimit)
		result, err := p.Eval(`import os
try:
    os.read_file("` + filepath.Join(dir, "missing.txt") + `")
    r = "no error"
except FileNotFoundError:
    r = "FileNotFoundError"
except MemoryError:
    r = "MemoryError"
r`)
		if err != nil {
			t.Fatal(err)
		}
		if s, _ := result.AsString(); s != "FileNotFoundError" {
			t.Fatalf("got %q, want FileNotFoundError", s)
		}
	})
}

// Direct test of readFileBytes, the shared helper behind os/pathlib reads,
// run under an interpreter lock so the guard applies to this goroutine.
func TestLimitsReadFileBytesDirect(t *testing.T) {
	dir := t.TempDir()
	bigPath := filepath.Join(dir, "big.bin")
	if err := os.WriteFile(bigPath, make([]byte, limitsBigSize), 0o644); err != nil {
		t.Fatal(err)
	}

	env := object.NewEnvironment()
	g := object.NewResourceGuard()
	g.SetMaxValueBytes(limitsTestLimit)
	env.SetResourceGuard(g)
	if env.EnterGIL() {
		defer env.ExitGIL()
	}

	data, errObj := readFileBytes(context.Background(), fsConfigAllowAll(), bigPath)
	if errObj == nil {
		t.Fatalf("expected a MemoryError, got %d bytes", len(data))
	}
	e, ok := errObj.(*object.Error)
	if !ok {
		t.Fatalf("expected *object.Error, got %T", errObj)
	}
	if e.ExceptionType != object.ExceptionTypeMemoryError || !strings.Contains(e.Message, "too large") {
		t.Fatalf("got %s %q", e.ExceptionType, e.Message)
	}

	g.SetMaxValueBytes(0)
	data, errObj = readFileBytes(context.Background(), fsConfigAllowAll(), bigPath)
	if errObj != nil {
		t.Fatalf("no limit: unexpected error %v", errObj.Inspect())
	}
	if len(data) != limitsBigSize {
		t.Fatalf("no limit: got %d bytes", len(data))
	}
}

func TestLimitsZipfileRead(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "test.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for name, content := range map[string]string{
		"small.txt": "small entry",
		"big.txt":   strings.Repeat("z", limitsBigSize),
	} {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(content))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()

	setup := "import zipfile\nzf = zipfile.ZipFile(\"" + zipPath + "\")"
	newP := func(t *testing.T, limit int64) *scriptling.Scriptling {
		p := limitsNewInterp(t, limit)
		RegisterZipfile(p, nil)
		return p
	}

	t.Run("within limit", func(t *testing.T) {
		limitsExpectOK(t, limitsCatch(t, newP(t, limitsTestLimit), setup, `zf.read("small.txt")`), "small entry")
	})
	t.Run("over limit", func(t *testing.T) {
		limitsExpectMemoryError(t, limitsCatch(t, newP(t, limitsTestLimit), setup, `len(zf.read("big.txt"))`), "too large")
	})
	t.Run("no limit", func(t *testing.T) {
		limitsExpectOK(t, limitsCatch(t, newP(t, 0), setup, `len(zf.read("big.txt"))`), "4096")
	})
}

func TestLimitsTarfileRead(t *testing.T) {
	dir := t.TempDir()
	tarPath := filepath.Join(dir, "test.tar")
	f, err := os.Create(tarPath)
	if err != nil {
		t.Fatal(err)
	}
	tw := tar.NewWriter(f)
	for _, e := range []struct{ name, content string }{
		{"small.txt", "small tar entry"},
		{"big.txt", strings.Repeat("t", limitsBigSize)},
	} {
		if err := tw.WriteHeader(&tar.Header{Name: e.name, Mode: 0o644, Size: int64(len(e.content))}); err != nil {
			t.Fatal(err)
		}
		tw.Write([]byte(e.content))
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()

	setup := "import tarfile\ntf = tarfile.TarFile(\"" + tarPath + "\")"
	newP := func(t *testing.T, limit int64) *scriptling.Scriptling {
		p := limitsNewInterp(t, limit)
		RegisterTarfile(p, nil)
		return p
	}

	t.Run("within limit", func(t *testing.T) {
		limitsExpectOK(t, limitsCatch(t, newP(t, limitsTestLimit), setup, `tf.read("small.txt")`), "small tar entry")
	})
	t.Run("over limit", func(t *testing.T) {
		limitsExpectMemoryError(t, limitsCatch(t, newP(t, limitsTestLimit), setup, `len(tf.read("big.txt"))`), "too large")
	})
	t.Run("no limit", func(t *testing.T) {
		limitsExpectOK(t, limitsCatch(t, newP(t, 0), setup, `len(tf.read("big.txt"))`), "4096")
	})
}

// fsConfigAllowAll is a filesystem config with no path restrictions.
func fsConfigAllowAll() fssecurity.Config { return fssecurity.Config{} }
