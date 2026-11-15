package template

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/stdlib"
)

const (
	deadlineTestTimeout = 300 * time.Millisecond
	deadlineTestSlack   = 1500 * time.Millisecond
)

// expectDeadline runs script under deadlineTestTimeout and fails unless it
// stops with a timeout soon after the deadline. A render cut short reports
// the context's error as a template render error.
func expectDeadline(t *testing.T, script string) {
	t.Helper()
	p := scriptling.New()
	stdlib.RegisterAll(p)
	RegisterHTML(p)
	RegisterText(p)
	ctx, cancel := context.WithTimeout(context.Background(), deadlineTestTimeout)
	defer cancel()
	start := time.Now()
	result, err := p.EvalWithContext(ctx, script)
	elapsed := time.Since(start)
	msg := ""
	if err != nil {
		msg = err.Error()
	} else if e, ok := result.(*object.Error); ok {
		msg = e.Message
	} else {
		t.Fatalf("expected a timeout, script finished in %v with a %s", elapsed, result.Type())
	}
	lower := strings.ToLower(msg)
	if !strings.Contains(lower, "timeout") && !strings.Contains(lower, "deadline") {
		t.Fatalf("expected a timeout error, got %q after %v", msg, elapsed)
	}
	if elapsed > deadlineTestTimeout+deadlineTestSlack {
		t.Fatalf("call ran %v past a %v deadline", elapsed-deadlineTestTimeout, deadlineTestTimeout)
	}
}

const nestedRanges = `{{range .}}{{range $}}{{range $}}{{range $}}{{end}}{{end}}{{end}}{{end}}`

func TestTextRangesWithoutOutputHonourDeadline(t *testing.T) {
	expectDeadline(t, "import scriptling.template.text as text\nt = text.Set()\nt.add('"+nestedRanges+"')\nt.render(list(range(150)))\n")
}

func TestHTMLRangesWithoutOutputHonourDeadline(t *testing.T) {
	expectDeadline(t, "import scriptling.template.html as html\nt = html.Set()\nt.add('"+nestedRanges+"')\nt.render(list(range(150)))\n")
}

func TestRangeOverIntegerHonoursDeadline(t *testing.T) {
	expectDeadline(t, "import scriptling.template.text as text\nt = text.Set()\nt.add('{{range 100000000000}}{{end}}')\nt.render(None)\n")
}

// Template calls that recurse without output: each level calls the next
// twice, 2**40 calls in all.
func TestRecursiveTemplateCallsHonourDeadline(t *testing.T) {
	var b strings.Builder
	b.WriteString("import scriptling.template.text as text\nt = text.Set()\n")
	for i := 0; i < 40; i++ {
		b.WriteString("t.add('{{define \"t")
		b.WriteString(itoa(i))
		b.WriteString("\"}}{{template \"t")
		b.WriteString(itoa(i + 1))
		b.WriteString("\"}}{{template \"t")
		b.WriteString(itoa(i + 1))
		b.WriteString("\"}}{{end}}')\n")
	}
	b.WriteString("t.add('{{define \"t40\"}}{{end}}')\nt.render(\"t0\", None)\n")
	expectDeadline(t, b.String())
}

func itoa(i int) string { return strconv.Itoa(i) }

// Instrumenting a set leaves its output unchanged, and adding to it again
// does not instrument a tree twice.
func TestCheckpointsLeaveOutputUnchanged(t *testing.T) {
	p := scriptling.New()
	RegisterHTML(p)
	RegisterText(p)
	result, err := p.Eval(`import scriptling.template.text as text
import scriptling.template.html as html
t = text.Set()
t.add('{{define "row"}}[{{.}}]{{end}}')
t.add('{{range $i, $v := .}}{{if $i}},{{end}}{{template "row" $v}}{{else}}none{{end}}')
h = html.Set()
h.add('<ul>{{range .}}<li>{{.}}</li>{{end}}</ul>')
t.render([1, 2, "<x>"]) + "|" + t.render([]) + "|" + h.render(["a", "<b>"])
`)
	if err != nil {
		t.Fatal(err)
	}
	want := "[1],[2],[<x>]|none|<ul><li>a</li><li>&lt;b&gt;</li></ul>"
	if s, _ := result.AsString(); s != want {
		t.Fatalf("got %q, want %q", s, want)
	}
}
