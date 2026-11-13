package template

import (
	"context"
	stderrors "errors"
	htmltemplate "html/template"
	"strings"
	"testing"
	texttemplate "text/template"
	"time"

	"github.com/paularlott/scriptling"
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
			RegisterHTML(p)
			RegisterText(p)
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

// Template data is converted under the interpreter's limits, so a small
// value sharing one list many times over cannot expand without bound.
func TestTemplateDataSharedExpansion(t *testing.T) {
	p := scriptling.New(scriptling.WithMaxValueBytes(1 << 20))
	stdlib.RegisterAll(p)
	RegisterText(p)
	result, err := p.Eval(`
import scriptling.template.text as tpl
t = tpl.Set()
t.add("{{len .}}")
a = [0] * 100
b = [a] * 100
c = [b] * 100
d = [c] * 100
ok = t.render([a] * 10)
try:
    t.render(d)
    r = "no error"
except MemoryError:
    r = "caught"
[ok, r]
`)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Inspect(); got != "['10', 'caught']" {
		t.Fatalf("got %s", got)
	}
}
