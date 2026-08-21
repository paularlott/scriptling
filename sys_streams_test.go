package scriptling

import (
	"bufio"
	"bytes"
	"testing"

	"github.com/paularlott/scriptling/extlibs"
)

// newSysStreamTestInterpreter returns an interpreter with the sys library
// registered and both output writers pointed at buffers so stream routing
// can be asserted.
func newSysStreamTestInterpreter(t *testing.T) (*Scriptling, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	p := New()
	extlibs.RegisterSysLibrary(p, []string{}, nil)
	var out, errOut bytes.Buffer
	p.SetOutputWriter(&out)
	p.SetErrorWriter(&errOut)
	return p, &out, &errOut
}

func TestSysStreamsExistWithoutStdin(t *testing.T) {
	p := New()
	extlibs.RegisterSysLibrary(p, []string{}, nil)
	_, err := p.Eval(`
import sys
n_out = sys.stdout.write("")
n_err = sys.stderr.write("")
`)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	nOut, objErr := p.GetVar("n_out")
	if objErr != nil || nOut != int64(0) {
		t.Errorf("sys.stdout.write(\"\") = %v, want 0", nOut)
	}
	nErr, objErr := p.GetVar("n_err")
	if objErr != nil || nErr != int64(0) {
		t.Errorf("sys.stderr.write(\"\") = %v, want 0", nErr)
	}
}

func TestSysStdoutAndStderrAreSeparated(t *testing.T) {
	p, out, errOut := newSysStreamTestInterpreter(t)
	_, err := p.Eval(`
import sys
sys.stdout.write("report line\n")
sys.stderr.write("warning: disk low\n")
`)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	if out.String() != "report line\n" {
		t.Errorf("stdout = %q, want %q", out.String(), "report line\n")
	}
	if errOut.String() != "warning: disk low\n" {
		t.Errorf("stderr = %q, want %q", errOut.String(), "warning: disk low\n")
	}
}

func TestSysStreamWriteReturnsCount(t *testing.T) {
	p, _, errOut := newSysStreamTestInterpreter(t)
	_, err := p.Eval(`
import sys
n = sys.stderr.write("hello")
`)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	n, objErr := p.GetVar("n")
	if objErr != nil || n != int64(5) {
		t.Errorf("sys.stderr.write(\"hello\") = %v, want 5", n)
	}
	if errOut.String() != "hello" {
		t.Errorf("stderr = %q, want %q", errOut.String(), "hello")
	}
}

func TestSysStreamWriteInsideFunction(t *testing.T) {
	p, out, errOut := newSysStreamTestInterpreter(t)
	_, err := p.Eval(`
import sys
def emit(msg):
    sys.stderr.write(msg)
emit("from function\n")
`)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	if errOut.String() != "from function\n" {
		t.Errorf("stderr = %q, want %q", errOut.String(), "from function\n")
	}
	if out.String() != "" {
		t.Errorf("stdout = %q, want empty", out.String())
	}
}

func TestSysStreamWriteRespectsOutputCapture(t *testing.T) {
	p, _, errOut := newSysStreamTestInterpreter(t)
	p.SetErrorWriter(errOut)
	p.EnableOutputCapture()
	_, err := p.Eval(`
import sys
print("report data")
sys.stderr.write("warning text")
`)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	if got := p.GetOutput(); got != "report data\n" {
		t.Errorf("captured output = %q, want %q", got, "report data\n")
	}
	if errOut.String() != "warning text" {
		t.Errorf("stderr = %q, want %q", errOut.String(), "warning text")
	}
}

func TestPrintFileStderr(t *testing.T) {
	p, out, errOut := newSysStreamTestInterpreter(t)
	_, err := p.Eval(`
import sys
print("fatal: boom", file=sys.stderr)
print("a", "b", sep="-", end="!", file=sys.stderr)
`)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	want := "fatal: boom\na-b!"
	if errOut.String() != want {
		t.Errorf("stderr = %q, want %q", errOut.String(), want)
	}
	if out.String() != "" {
		t.Errorf("stdout = %q, want empty", out.String())
	}
}

func TestPrintFileStdout(t *testing.T) {
	p, out, errOut := newSysStreamTestInterpreter(t)
	_, err := p.Eval(`
import sys
print("via stream", file=sys.stdout)
`)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	if out.String() != "via stream\n" {
		t.Errorf("stdout = %q, want %q", out.String(), "via stream\n")
	}
	if errOut.String() != "" {
		t.Errorf("stderr = %q, want empty", errOut.String())
	}
}

func TestPrintFileUserWriteClass(t *testing.T) {
	p, _, _ := newSysStreamTestInterpreter(t)
	_, err := p.Eval(`
class Collector:
    def __init__(self):
        self.chunks = []

    def write(self, s):
        self.chunks.append(s)
        return len(s)

c = Collector()
print("hello", "world", file=c)
result = c.chunks
`)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	chunks, objErr := p.GetVarAsList("result")
	if objErr != nil {
		t.Fatalf("GetVarAsList error: %v", objErr)
	}
	if len(chunks) != 1 {
		t.Fatalf("chunks = %v, want single write call", chunks)
	}
	s, sErr := chunks[0].AsString()
	if sErr != nil || s != "hello world\n" {
		t.Errorf("chunk = %q, want %q", s, "hello world\n")
	}
}

func TestSysStreamWritelines(t *testing.T) {
	p, _, errOut := newSysStreamTestInterpreter(t)
	_, err := p.Eval(`
import sys
sys.stderr.writelines(["line1\n", "line2\n"])
`)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	want := "line1\nline2\n"
	if errOut.String() != want {
		t.Errorf("stderr = %q, want %q", errOut.String(), want)
	}
}

func TestSysStreamWritelinesRejectsNonList(t *testing.T) {
	p, _, _ := newSysStreamTestInterpreter(t)
	_, err := p.Eval(`
import sys
sys.stdout.writelines("nope")
`)
	if err == nil {
		t.Error("expected error for writelines with non-list argument")
	}
}

func TestSysStreamWriteRejectsNonString(t *testing.T) {
	p, _, _ := newSysStreamTestInterpreter(t)
	_, err := p.Eval(`
import sys
sys.stderr.write(42)
`)
	if err == nil {
		t.Error("expected error for write with non-string argument")
	}
}

func TestSysStreamFlush(t *testing.T) {
	p, _, _ := newSysStreamTestInterpreter(t)
	_, err := p.Eval(`
import sys
is_none = sys.stdout.flush() is None
`)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	isNone, objErr := p.GetVar("is_none")
	if objErr != nil || isNone != true {
		t.Errorf("sys.stdout.flush() is None = %v, want true", isNone)
	}
}

func TestSysStreamFlushFlushesBufferedWriter(t *testing.T) {
	p := New()
	extlibs.RegisterSysLibrary(p, []string{}, nil)
	var raw bytes.Buffer
	buffered := bufio.NewWriter(&raw)
	p.SetOutputWriter(buffered)

	_, err := p.Eval(`
import sys
sys.stdout.write("buffered content")
`)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	if raw.String() != "" {
		t.Fatalf("writer flushed before flush(): %q", raw.String())
	}

	_, err = p.Eval(`
import sys
sys.stdout.flush()
`)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	if raw.String() != "buffered content" {
		t.Errorf("after flush() = %q, want %q", raw.String(), "buffered content")
	}
}

func TestSysStreamIsatty(t *testing.T) {
	p, _, _ := newSysStreamTestInterpreter(t)
	_, err := p.Eval(`
import sys
tty = sys.stdout.isatty()
`)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	isTty, objErr := p.GetVar("tty")
	if objErr != nil || isTty != false {
		t.Errorf("sys.stdout.isatty() = %v, want false (buffer writer)", isTty)
	}
}

func TestSysStreamWithStatement(t *testing.T) {
	p, out, _ := newSysStreamTestInterpreter(t)
	_, err := p.Eval(`
import sys
with sys.stdout as f:
    f.write("in context")
`)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	if out.String() != "in context" {
		t.Errorf("stdout = %q, want %q", out.String(), "in context")
	}
}

func TestSysStreamsDiscardedInSandbox(t *testing.T) {
	extlibs.ResetRuntime()
	var out, errOut bytes.Buffer
	extlibs.SetSandboxFactory(func() extlibs.SandboxInstance {
		q := New()
		extlibs.RegisterSysLibrary(q, []string{}, nil)
		q.SetOutputWriter(&out)
		q.SetErrorWriter(&errOut)
		return q
	})
	defer extlibs.SetSandboxFactory(nil)

	p := New()
	extlibs.RegisterRuntimeLibraryAll(p, nil)
	_, err := p.Eval(`
import scriptling.runtime as runtime

env = runtime.sandbox.create()
env.exec("import sys\nsys.stdout.write('leak-out')\nsys.stderr.write('leak-err')\nprint('leak-print')")
`)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	if out.String() != "" {
		t.Errorf("sandbox stdout leaked: %q", out.String())
	}
	if errOut.String() != "" {
		t.Errorf("sandbox stderr leaked: %q", errOut.String())
	}
}
