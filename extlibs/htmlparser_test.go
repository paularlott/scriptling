package extlibs

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/conversion"
	"github.com/paularlott/scriptling/stdlib"
)

type htmlGoldenCase struct {
	Input           string `json:"input"`
	ConvertCharrefs bool   `json:"convert_charrefs"`
	Events          []any  `json:"events"`
}

func newHTMLParserScriptling(t testing.TB) *scriptling.Scriptling {
	t.Helper()
	p := scriptling.New()
	stdlib.RegisterAll(p)
	RegisterHTMLParserLibrary(p)
	return p
}

// newHTMLHarness loads the recorder harness gen.py also runs under CPython.
func newHTMLHarness(t testing.TB) *scriptling.Scriptling {
	t.Helper()
	harness, err := os.ReadFile("testdata/htmlparser/harness.py")
	if err != nil {
		t.Fatal(err)
	}
	p := newHTMLParserScriptling(t)
	if _, err := p.Eval(string(harness)); err != nil {
		t.Fatalf("loading harness: %v", err)
	}
	return p
}

func runHTMLHarness(t testing.TB, p *scriptling.Scriptling, input string, convert bool) (any, error) {
	t.Helper()
	p.SetVar("input_data", input)
	p.SetVar("convert", convert)
	result, err := p.Eval("run(input_data, convert)")
	if err != nil {
		return nil, err
	}
	// Round-trip through JSON so tuples, lists and None compare with the
	// golden data exactly as CPython wrote it.
	raw, jerr := json.Marshal(conversion.ToGo(result))
	if jerr != nil {
		t.Fatal(jerr)
	}
	var events any
	if jerr := json.Unmarshal(raw, &events); jerr != nil {
		t.Fatal(jerr)
	}
	return events, nil
}

// Every corpus input produces exactly the callbacks CPython 3.14 does, with
// and without convert_charrefs. The corpus covers ordinary documents and the
// malformed markup the parser used to panic on or diverge from Python for
// (vuln-0020). Regenerate the golden data with gen.py.
func TestHTMLParserMatchesCPython(t *testing.T) {
	raw, err := os.ReadFile("testdata/htmlparser/golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []htmlGoldenCase
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) < 200 {
		t.Fatalf("golden data has only %d cases", len(cases))
	}
	p := newHTMLHarness(t)
	for _, c := range cases {
		got, err := runHTMLHarness(t, p, c.Input, c.ConvertCharrefs)
		if err != nil {
			t.Errorf("%q convert_charrefs=%v: %v", c.Input, c.ConvertCharrefs, err)
			continue
		}
		want := any(c.Events)
		if c.Events == nil {
			want = []any{}
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%q convert_charrefs=%v\n got  %v\n want %v", c.Input, c.ConvertCharrefs, got, want)
		}
	}
}

// The inputs that panicked (index and slice bounds out of range) parse
// without error (vuln-0020).
func TestHTMLParserMalformedInputsDoNotPanic(t *testing.T) {
	p := newHTMLHarness(t)
	for _, input := range []string{"a &; b", "<!-->", "<!--->", "<?>"} {
		for _, convert := range []bool{true, false} {
			if _, err := runHTMLHarness(t, p, input, convert); err != nil {
				t.Errorf("%q convert_charrefs=%v: %v", input, convert, err)
			}
		}
	}
}

// An exception raised in any handler propagates out of feed(), as in Python;
// it used to be swallowed and parsing carried on.
func TestHTMLParserHandlerExceptionsPropagate(t *testing.T) {
	p := newHTMLParserScriptling(t)
	for handler, doc := range map[string]string{
		"handle_starttag":    "<a>",
		"handle_endtag":      "</a>",
		"handle_startendtag": "<br/>",
		"handle_data":        "text",
		"handle_comment":     "<!--c-->",
		"handle_decl":        "<!DOCTYPE html>",
		"handle_pi":          "<?pi>",
		"unknown_decl":       "<![CDATA[x]]>",
		"handle_entityref":   "&amp;",
		"handle_charref":     "&#65;",
	} {
		result, err := p.Eval(`
from html.parser import HTMLParser
class P(HTMLParser):
    def ` + handler + `(self, *args):
        self.calls = self.calls + 1
        raise ValueError("boom")
parser = P(convert_charrefs=False)
parser.calls = 0
try:
    parser.feed("` + doc + doc + `")
    r = "no exception"
except ValueError as e:
    r = "caught " + str(e) + " after " + str(parser.calls)
r`)
		if err != nil {
			t.Fatalf("%s: %v", handler, err)
		}
		if got := result.Inspect(); got != "caught boom after 1" {
			t.Errorf("%s: %s", handler, got)
		}
	}
}

// The default handle_startendtag calls handle_starttag then handle_endtag;
// an exception in either propagates.
func TestHTMLParserDefaultStartendtagPropagates(t *testing.T) {
	p := newHTMLParserScriptling(t)
	result, err := p.Eval(`
from html.parser import HTMLParser
class P(HTMLParser):
    def handle_starttag(self, tag, attrs):
        self.seen.append("start " + tag)
    def handle_endtag(self, tag):
        raise KeyError(tag)
parser = P()
parser.seen = []
try:
    parser.feed("<br/><hr/>")
    r = "no exception"
except KeyError as e:
    r = "caught"
[r, parser.seen]`)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Inspect(); got != "['caught', ['start br']]" {
		t.Fatalf("got %s", got)
	}
}

// sys.exit() in a handler ends the script, rather than being swallowed while
// the script carries on.
func TestHTMLParserHandlerSysExitPropagates(t *testing.T) {
	p := newHTMLParserScriptling(t)
	RegisterSysLibrary(p, nil, nil)
	p.EnableOutputCapture()
	_, err := p.Eval(`
import sys
from html.parser import HTMLParser
class P(HTMLParser):
    def handle_starttag(self, tag, attrs):
        sys.exit(3)
P().feed("<a>")
print("continued")`)
	if err == nil || !strings.Contains(err.Error(), "SystemExit") {
		t.Fatalf("expected SystemExit, got %v", err)
	}
	if out := p.GetOutput(); strings.Contains(out, "continued") {
		t.Fatalf("script carried on after sys.exit: %q", out)
	}
}

// A timeout hit inside a handler stops the script: feed() no longer turns it
// into a normal return that lets the script run on (the vuln-0010 guarantee).
func TestHTMLParserHandlerTimeoutPropagates(t *testing.T) {
	p := newHTMLParserScriptling(t)
	p.EnableOutputCapture()
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_, err := p.EvalWithContext(ctx, `
from html.parser import HTMLParser
class P(HTMLParser):
    def handle_starttag(self, tag, attrs):
        while True:
            pass
try:
    P().feed("<a><b><c>")
except Exception:
    pass
print("continued")`)
	if err == nil {
		t.Fatal("expected the timeout to stop the script")
	}
	if out := p.GetOutput(); strings.Contains(out, "continued") {
		t.Fatalf("script carried on after its timeout: %q", out)
	}
}

// Parsing never panics, whatever the input: a recovered panic would surface
// as a "script panic" error.
func FuzzHTMLParserFeed(f *testing.F) {
	raw, err := os.ReadFile("testdata/htmlparser/golden.json")
	if err != nil {
		f.Fatal(err)
	}
	var cases []htmlGoldenCase
	if err := json.Unmarshal(raw, &cases); err != nil {
		f.Fatal(err)
	}
	for _, c := range cases {
		f.Add(c.Input, c.ConvertCharrefs)
	}
	p := newHTMLHarness(f)
	f.Fuzz(func(t *testing.T, input string, convert bool) {
		if _, err := runHTMLHarness(t, p, input, convert); err != nil {
			t.Fatalf("%q convert_charrefs=%v: %v", input, convert, err)
		}
	})
}
