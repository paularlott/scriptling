package extlibs

import (
	"strings"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/stdlib"
)

func newXMLScriptling() *scriptling.Scriptling {
	p := scriptling.New()
	stdlib.RegisterAll(p)
	RegisterXmlLibrary(p)
	return p
}

func nestedXML(n int) string {
	return strings.Repeat("<a>", n) + "x" + strings.Repeat("</a>", n)
}

// A deeply nested document is an ordinary, catchable error rather than a
// stack overflow that ends the process (CWE-674, vuln-0017).
func TestXMLLoadsRejectsDeepNesting(t *testing.T) {
	p := newXMLScriptling()
	p.SetVar("doc", nestedXML(200000))
	result, err := p.Eval(`
import xml
try:
    xml.loads(doc)
    r = "parsed"
except Exception as e:
    r = str(e)
r`)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if got := result.Inspect(); !strings.Contains(got, "nests more than 1000 levels deep") {
		t.Fatalf("expected a catchable depth error, got %s", got)
	}
}

// The cap is exact: a document at the cap parses, one level deeper does not.
func TestXMLLoadsDepthBoundary(t *testing.T) {
	p := newXMLScriptling()
	p.SetVar("ok", nestedXML(maxXMLDepth))
	p.SetVar("deep", nestedXML(maxXMLDepth+1))
	if _, err := p.Eval("import xml\nxml.loads(ok)"); err != nil {
		t.Fatalf("a document at the cap should parse: %v", err)
	}
	if _, err := p.Eval("import xml\nxml.loads(deep)"); err == nil || !strings.Contains(err.Error(), "nests more than") {
		t.Fatalf("one level past the cap should be rejected, got: %v", err)
	}
}

// Normal documents parse as before, and round-trip through dumps.
func TestXMLLoadsDumpsNormalDocuments(t *testing.T) {
	p := newXMLScriptling()
	result, err := p.Eval(`
import xml
d = xml.loads('<root id="1"><item>a</item><item>b</item><nested><leaf>v</leaf></nested></root>')
back = xml.loads(xml.dumps(d))
[d["root"]["@id"], d["root"]["item"], d["root"]["nested"]["leaf"], back == d]`)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if got := result.Inspect(); got != "['1', ['a', 'b'], 'v', True]" {
		t.Fatalf("got %s", got)
	}
}

// dumps is capped the same way: deeply nested dicts, nested lists (which
// recurse without indenting) and a value that contains itself all raise an
// error instead of overflowing the stack.
func TestXMLDumpsRejectsDeepNesting(t *testing.T) {
	p := newXMLScriptling()
	for name, build := range map[string]string{
		"nested dicts":   "v = 'x'\nfor i in range(200000):\n    v = {'a': v}\nxml.dumps({'root': v})",
		"nested lists":   "v = 'x'\nfor i in range(200000):\n    v = [v]\nxml.dumps({'root': v})",
		"self reference": "d = {}\nd['a'] = d\nxml.dumps({'root': d})",
	} {
		_, err := p.Eval("import xml\n" + build)
		if err == nil || !strings.Contains(err.Error(), "nests more than") {
			t.Errorf("%s: expected a depth error, got: %v", name, err)
		}
	}
}
