package stdlib_test

import (
	"strings"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/stdlib"
)

// The value limit used here: small enough that a few KB of writes go over it.
const stringIOTestLimit = 1024

func stringIOInterp(limit int64) *scriptling.Scriptling {
	p := scriptling.New(scriptling.WithMaxValueBytes(limit))
	stdlib.RegisterAll(p)
	return p
}

// evalString runs script and returns its string result.
func evalString(t *testing.T, p *scriptling.Scriptling, script string) string {
	t.Helper()
	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("eval failed: %v\nscript:\n%s", err, script)
	}
	s, errObj := result.AsString()
	if errObj != nil {
		t.Fatalf("result is not a string: %s", result.Inspect())
	}
	return s
}

func TestLimitsStringIOWrite(t *testing.T) {
	// Each write is under the limit; it is the buffer's total that goes over.
	script := `import io
buf = io.StringIO()
chunk = "w" * 300
n = 0
try:
    for i in range(COUNT):
        buf.write(chunk)
        n += 1
    r = "ok:" + str(len(buf.getvalue()))
except MemoryError as e:
    r = "MemoryError:" + str(n) + ":" + str(len(buf.getvalue())) + ":" + str(e)
r`

	t.Run("within limit", func(t *testing.T) {
		got := evalString(t, stringIOInterp(stringIOTestLimit), strings.Replace(script, "COUNT", "3", 1))
		if got != "ok:900" {
			t.Fatalf("got %q, want ok:900", got)
		}
	})

	t.Run("exactly at limit", func(t *testing.T) {
		got := evalString(t, stringIOInterp(stringIOTestLimit), `import io
buf = io.StringIO()
buf.write("a" * 1000)
buf.write("b" * 24)
str(len(buf.getvalue()))`)
		if got != "1024" {
			t.Fatalf("got %q, want 1024", got)
		}
	})

	t.Run("over limit", func(t *testing.T) {
		got := evalString(t, stringIOInterp(stringIOTestLimit), strings.Replace(script, "COUNT", "10", 1))
		// Three writes fit (900 bytes); the fourth (1200) is refused and
		// leaves the buffer unchanged.
		if !strings.HasPrefix(got, "MemoryError:3:900:") {
			t.Fatalf("got %q, want a MemoryError after 3 writes with 900 bytes buffered", got)
		}
		if !strings.Contains(got, "too large") {
			t.Fatalf("MemoryError message %q does not contain \"too large\"", got)
		}
	})

	t.Run("no limit", func(t *testing.T) {
		got := evalString(t, stringIOInterp(0), strings.Replace(script, "COUNT", "20", 1))
		if got != "ok:6000" {
			t.Fatalf("got %q, want ok:6000", got)
		}
	})

	t.Run("uncaught error propagates", func(t *testing.T) {
		_, err := stringIOInterp(stringIOTestLimit).Eval(`import io
buf = io.StringIO()
for i in range(10):
    buf.write("u" * 300)`)
		if err == nil {
			t.Fatal("expected an error")
		}
		if !strings.Contains(err.Error(), "too large") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestLimitsPrintToStringIO(t *testing.T) {
	script := `import io
buf = io.StringIO()
line = "p" * 299
n = 0
try:
    for i in range(COUNT):
        print(line, file=buf)
        n += 1
    r = "ok:" + str(len(buf.getvalue()))
except MemoryError as e:
    r = "MemoryError:" + str(n) + ":" + str(len(buf.getvalue())) + ":" + str(e)
r`

	t.Run("within limit", func(t *testing.T) {
		got := evalString(t, stringIOInterp(stringIOTestLimit), strings.Replace(script, "COUNT", "3", 1))
		if got != "ok:900" {
			t.Fatalf("got %q, want ok:900", got)
		}
	})

	t.Run("over limit", func(t *testing.T) {
		got := evalString(t, stringIOInterp(stringIOTestLimit), strings.Replace(script, "COUNT", "10", 1))
		// print emits text and end in one write, so three lines fit (900
		// bytes), the fourth is refused whole and the buffer is unchanged.
		if !strings.HasPrefix(got, "MemoryError:3:900:") {
			t.Fatalf("got %q, want a MemoryError after 3 prints with 900 bytes buffered", got)
		}
		if !strings.Contains(got, "too large") {
			t.Fatalf("MemoryError message %q does not contain %q", got, "too large")
		}
	})

	t.Run("no limit", func(t *testing.T) {
		got := evalString(t, stringIOInterp(0), strings.Replace(script, "COUNT", "20", 1))
		if got != "ok:6000" {
			t.Fatalf("got %q, want ok:6000", got)
		}
	})
}

// The Go-side writer used by print(file=buf) reports ErrOutputLimitExceeded
// once the buffer would pass the limit of the interpreter on this goroutine.
func TestLimitsStringIOWriterDirect(t *testing.T) {
	env := object.NewEnvironment()
	g := object.NewResourceGuard()
	g.SetMaxValueBytes(16)
	env.SetResourceGuard(g)
	if env.EnterGIL() {
		defer env.ExitGIL()
	}

	inst, ok := stdlib.IOLibrary.Functions()["StringIO"].Fn(t.Context(), object.NewKwargs(nil)).(*object.Instance)
	if !ok {
		t.Fatal("StringIO() did not return an instance")
	}
	w, ok := stdlib.GetStringIOWriter(inst)
	if !ok {
		t.Fatal("GetStringIOWriter failed")
	}
	if n, err := w.Write([]byte("0123456789abcdef")); err != nil || n != 16 {
		t.Fatalf("write to exactly the limit: n=%d err=%v", n, err)
	}
	if _, err := w.Write([]byte("x")); err != object.ErrOutputLimitExceeded {
		t.Fatalf("expected ErrOutputLimitExceeded, got %v", err)
	}

	g.SetMaxValueBytes(0)
	if _, err := w.Write(make([]byte, 4096)); err != nil {
		t.Fatalf("no limit: %v", err)
	}
}

// Content given to StringIO when it is created is held to the value limit.
func TestLimitsStringIOInitialContent(t *testing.T) {
	p := scriptling.New(scriptling.WithMaxValueBytes(1024))
	stdlib.RegisterAll(p)
	result, err := p.Eval(`
import io
out = [len(io.StringIO("x" * 1000).getvalue())]
try:
    io.StringIO("x" * 2000)
    out.append("no error")
except MemoryError:
    out.append("caught")
out
`)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Inspect(); got != "[1000, 'caught']" {
		t.Fatalf("got %s", got)
	}
}

// A StringIO buffer is charged for its backing storage, and the charge is
// released when truncate (or the buffer's growth) lets old storage go, so
// reusing one buffer never adds up to a false memory limit cancellation.
func TestLimitsStringIOAccountingFollowsStorage(t *testing.T) {
	p := scriptling.New(scriptling.WithMemoryLimit(8<<20), scriptling.WithMaxValueBytes(0))
	stdlib.RegisterAll(p)
	if _, err := p.Eval(`
import io
n = 3 << 20
buf = io.StringIO("y" * 1000)
for i in range(10):
    buf.write(str(i % 10) * n)
    buf.truncate(0)
    buf.seek(0)
`); err != nil {
		t.Fatalf("30 MiB written through one buffer, truncated each time, under an 8 MiB limit: %v", err)
	}
	if p.ResourceUsage().MemoryLimitCancellations != 0 {
		t.Fatal("cancelled for memory already released")
	}
	if p.ResourceUsage().AllocatedBytes < 30<<20 {
		t.Fatalf("buffer storage charged only %d bytes", p.ResourceUsage().AllocatedBytes)
	}
}

// json.dumps renders a shared value once per reference, so a small script
// value can expand into a huge document; the walk is held to the value limit.
func TestLimitsJSONDumpsSharedExpansion(t *testing.T) {
	p := scriptling.New(scriptling.WithMaxValueBytes(1 << 20))
	stdlib.RegisterAll(p)
	result, err := p.Eval(`
import json
a = [0] * 100
b = [a] * 100
c = [b] * 100
d = [c] * 100
ok = len(json.dumps([a] * 10))
try:
    json.dumps(d)
    r = "no error"
except MemoryError:
    r = "caught"
[ok, r]
`)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Inspect(); got != "[3020, 'caught']" {
		t.Fatalf("got %s", got)
	}
}

// json.dumps refuses deep nesting and oversized container text without
// building the whole document first.
func TestLimitsJSONDumpsDepthAndContainerText(t *testing.T) {
	p := scriptling.New(scriptling.WithMaxValueBytes(1 << 20))
	stdlib.RegisterAll(p)
	result, err := p.Eval(`
import json
out = []
x = []
for i in range(20000):
    x = [x]
try:
    json.dumps(x)
    out.append("no error")
except MemoryError as e:
    out.append("deep" if "deeply nested" in str(e) else str(e))
`)
	if err != nil {
		t.Fatal(err)
	}
	p.SetMaxValueBytes(1024) // a container's text is checked as it is assembled
	result, err = p.Eval(`
try:
    json.dumps({"k": ["x" * 500] * 3})
    out.append("no error")
except MemoryError:
    out.append("text")
out
`)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Inspect(); got != "['deep', 'text']" {
		t.Fatalf("got %s", got)
	}
}

// Indented output is held to the value limit by its exact size, not by the
// estimate used to admit building it.
func TestLimitsJSONDumpsIndentExact(t *testing.T) {
	p := scriptling.New()
	stdlib.RegisterAll(p)
	// The data is built with no limit; the limit then applies to the dump.
	result, err := p.Eval(`
import json
data = {"a": [1, 2, 3], "b": {"c": "d"}}
json.dumps(data, indent=2)
`)
	if err != nil {
		t.Fatal(err)
	}
	exact := int64(len(result.Inspect()))
	p.SetMaxValueBytes(exact)
	if _, err := p.Eval(`json.dumps(data, indent=2)`); err != nil {
		t.Fatalf("output of exactly the limit (%d bytes) refused: %v", exact, err)
	}
	p.SetMaxValueBytes(exact - 1)
	if _, err := p.Eval(`json.dumps(data, indent=2)`); err == nil {
		t.Fatal("output one byte over the limit accepted")
	}
}
