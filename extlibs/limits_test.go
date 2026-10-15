package extlibs

import (
	"archive/tar"
	"archive/zip"
	"context"
	stderrors "errors"
	htmltemplate "html/template"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	texttemplate "text/template"
	"time"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/fssecurity"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/stdlib"
)

// limitsTestLimit is the value limit used by the tests in this file: small
// enough that a few KB of data goes over it.
const limitsTestLimit = 1024

// limitsBigSize is comfortably over limitsTestLimit.
const limitsBigSize = 4096

// limitsNewInterp returns an interpreter with the stdlib registered and the
// given value limit (0 = no limit).
func limitsNewInterp(t *testing.T, maxValue int64) *scriptling.Scriptling {
	t.Helper()
	p := scriptling.New(scriptling.WithMaxValueBytes(maxValue))
	stdlib.RegisterAll(p)
	return p
}

// limitsCatch runs setup followed by expr inside a try/except MemoryError and
// returns "ok:<str(expr)>" or "MemoryError:<message>".
func limitsCatch(t *testing.T, p *scriptling.Scriptling, setup, expr string) string {
	t.Helper()
	var b strings.Builder
	b.WriteString(setup)
	b.WriteString("\ntry:\n    __r = \"ok:\" + str(")
	b.WriteString(expr)
	b.WriteString(")\nexcept MemoryError as e:\n    __r = \"MemoryError:\" + str(e)\n__r\n")
	result, err := p.Eval(b.String())
	if err != nil {
		t.Fatalf("eval failed: %v\nscript:\n%s", err, b.String())
	}
	s, errObj := result.AsString()
	if errObj != nil {
		t.Fatalf("result is not a string: %v", result.Inspect())
	}
	return s
}

func limitsExpectOK(t *testing.T, got, want string) {
	t.Helper()
	if got != "ok:"+want {
		t.Fatalf("got %q, want %q", got, "ok:"+want)
	}
}

func limitsExpectMemoryError(t *testing.T, got, msgPart string) {
	t.Helper()
	if !strings.HasPrefix(got, "MemoryError:") {
		t.Fatalf("expected a MemoryError, got %q", got)
	}
	if msgPart != "" && !strings.Contains(got, msgPart) {
		t.Fatalf("MemoryError message %q does not contain %q", got, msgPart)
	}
}

// ===================== requests =====================

func TestLimitsRequestsResponseBody(t *testing.T) {
	small := strings.Repeat("a", 100)
	big := strings.Repeat("b", limitsBigSize)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/small":
			w.Write([]byte(small))
		case "/big":
			w.Write([]byte(big))
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	setup := "import requests"

	t.Run("within limit", func(t *testing.T) {
		p := limitsNewInterp(t, limitsTestLimit)
		RegisterRequestsLibrary(p)
		got := limitsCatch(t, p, setup, `requests.get("`+srv.URL+`/small", timeout=5).text`)
		limitsExpectOK(t, got, small)
	})

	t.Run("over limit", func(t *testing.T) {
		p := limitsNewInterp(t, limitsTestLimit)
		RegisterRequestsLibrary(p)
		got := limitsCatch(t, p, setup, `len(requests.get("`+srv.URL+`/big", timeout=5).text)`)
		limitsExpectMemoryError(t, got, "too large")
	})

	t.Run("no limit", func(t *testing.T) {
		p := limitsNewInterp(t, limitsTestLimit)
		p.SetMaxValueBytes(0)
		RegisterRequestsLibrary(p)
		got := limitsCatch(t, p, setup, `len(requests.get("`+srv.URL+`/big", timeout=5).text)`)
		limitsExpectOK(t, got, "4096")
	})
}

// ===================== file reads =====================

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
		RegisterOSLibrary(p, []string{dir})
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

// ===================== zipfile / tarfile =====================

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
		RegisterZipfileLibrary(p, nil)
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
		RegisterTarfileLibrary(p, nil)
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

// ===================== secrets =====================

func TestLimitsSecrets(t *testing.T) {
	// CheckElements charges one interface slot (16 bytes on 64-bit) per
	// requested byte, so with a 1024 byte limit 32 bytes is fine and 1000 is not.
	newP := func(t *testing.T, limit int64) *scriptling.Scriptling {
		p := limitsNewInterp(t, limit)
		RegisterSecretsLibrary(p)
		return p
	}

	t.Run("within limit", func(t *testing.T) {
		p := newP(t, limitsTestLimit)
		limitsExpectOK(t, limitsCatch(t, p, "import secrets", `len(secrets.token_bytes(32))`), "32")
		limitsExpectOK(t, limitsCatch(t, p, "import secrets", `len(secrets.token_hex(32))`), "64")
		limitsExpectOK(t, limitsCatch(t, p, "import secrets", `len(secrets.token_urlsafe(30))`), "40")
	})

	for _, fn := range []string{"token_bytes", "token_hex", "token_urlsafe"} {
		t.Run(fn+" over limit", func(t *testing.T) {
			got := limitsCatch(t, newP(t, limitsTestLimit), "import secrets", `len(secrets.`+fn+`(1000))`)
			limitsExpectMemoryError(t, got, "too large")
		})
	}

	t.Run("no limit", func(t *testing.T) {
		p := newP(t, 0)
		limitsExpectOK(t, limitsCatch(t, p, "import secrets", `len(secrets.token_hex(1000))`), "2000")
		limitsExpectOK(t, limitsCatch(t, p, "import secrets", `len(secrets.token_bytes(1000))`), "1000")
	})
}

// ===================== template =====================

func TestLimitsTemplateRender(t *testing.T) {
	// The data stays under the limit; only the rendered output (the value
	// repeated by the range) goes over it.
	setup := func(lib string) string {
		return `import scriptling.template.` + lib + ` as tpl
t = tpl.Set()
t.add("{{range .L}}{{$.S}}{{end}}")
s = "x" * 200`
	}

	for _, lib := range []string{"html", "text"} {
		newP := func(t *testing.T, limit int64) *scriptling.Scriptling {
			p := limitsNewInterp(t, limit)
			RegisterTemplateHTMLLibrary(p)
			RegisterTemplateTextLibrary(p)
			return p
		}

		t.Run(lib+" within limit", func(t *testing.T) {
			got := limitsCatch(t, newP(t, limitsTestLimit), setup(lib), `len(t.render({"S": s, "L": [1, 2, 3]}))`)
			limitsExpectOK(t, got, "600")
		})
		t.Run(lib+" over limit", func(t *testing.T) {
			got := limitsCatch(t, newP(t, limitsTestLimit), setup(lib), `len(t.render({"S": s, "L": list(range(10))}))`)
			limitsExpectMemoryError(t, got, "rendered template too large")
		})
		t.Run(lib+" no limit", func(t *testing.T) {
			got := limitsCatch(t, newP(t, 0), setup(lib), `len(t.render({"S": s, "L": list(range(20))}))`)
			limitsExpectOK(t, got, "4000")
		})
	}
}

func TestLimitsTemplateCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	for _, pt := range []*parsedTemplateSet{
		{html: mustHTMLSet(t, "Hello {{.}}")},
		{text: mustTextSet(t, "Hello {{.}}")},
	} {
		out, err := renderTemplateSet(ctx, pt, "", "world")
		if !stderrors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got out=%q err=%v", out, err)
		}
		if out != "" {
			t.Fatalf("expected no output after cancellation, got %q", out)
		}

		// Through the render method: a cancelled call is a render error.
		inst := newSetInstance(pt)
		res := methodFn(t, "render")(ctx, object.NewKwargs(nil), inst, object.NewString("world"))
		if !object.IsError(res) {
			t.Fatalf("expected an error from render with a cancelled context, got %s", res.Inspect())
		}

		// The same set renders normally with a live context.
		out, err = renderTemplateSet(context.Background(), pt, "", "world")
		if err != nil || out != "Hello world" {
			t.Fatalf("live context: out=%q err=%v", out, err)
		}
	}
}

func TestLimitsTemplateRenderWriter(t *testing.T) {
	w := &renderWriter{ctx: context.Background(), limit: 10}
	if n, err := w.Write([]byte("0123456789")); err != nil || n != 10 {
		t.Fatalf("write to exactly the limit: n=%d err=%v", n, err)
	}
	if _, err := w.Write([]byte("x")); !stderrors.Is(err, errRenderTooLarge) {
		t.Fatalf("expected errRenderTooLarge, got %v", err)
	}
	if w.buf.String() != "0123456789" {
		t.Fatalf("buffer changed after rejected write: %q", w.buf.String())
	}

	unlimited := &renderWriter{ctx: context.Background(), limit: 0}
	if _, err := unlimited.Write(make([]byte, limitsBigSize)); err != nil {
		t.Fatalf("no limit: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	<-ctx.Done()
	expired := &renderWriter{ctx: ctx, limit: 0}
	if _, err := expired.Write([]byte("x")); !stderrors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded, got %v", err)
	}
}

// ===================== sys.stdin =====================

func TestLimitsSysStdinRead(t *testing.T) {
	newP := func(t *testing.T, limit int64, stdin string) *scriptling.Scriptling {
		p := limitsNewInterp(t, limit)
		RegisterSysLibrary(p, []string{"script"}, strings.NewReader(stdin))
		return p
	}

	t.Run("within limit", func(t *testing.T) {
		got := limitsCatch(t, newP(t, limitsTestLimit, "line one\nline two\n"), "import sys", `sys.stdin.read()`)
		limitsExpectOK(t, got, "line one\nline two\n")
	})
	t.Run("over limit", func(t *testing.T) {
		got := limitsCatch(t, newP(t, limitsTestLimit, strings.Repeat("i", limitsBigSize)), "import sys", `len(sys.stdin.read())`)
		limitsExpectMemoryError(t, got, "too large")
	})
	t.Run("no limit", func(t *testing.T) {
		got := limitsCatch(t, newP(t, 0, strings.Repeat("i", limitsBigSize)), "import sys", `len(sys.stdin.read())`)
		limitsExpectOK(t, got, "4096")
	})
}

// ===================== helpers =====================

// fsConfigAllowAll is a filesystem config with no path restrictions.
func fsConfigAllowAll() fssecurity.Config { return fssecurity.Config{} }

func mustHTMLSet(t *testing.T, src string) *htmltemplate.Template {
	t.Helper()
	tmpl, err := htmltemplate.New("").Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	return tmpl
}

func mustTextSet(t *testing.T, src string) *texttemplate.Template {
	t.Helper()
	tmpl, err := texttemplate.New("").Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	return tmpl
}
