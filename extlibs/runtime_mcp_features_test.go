package extlibs_test

import (
	"testing"

	"github.com/paularlott/scriptling/extlibs"
	"github.com/paularlott/scriptling/extlibs/mcp"
)

func TestMCPDecoratorCompleteKwarg_Errors(t *testing.T) {
	for label, src := range map[string]string{
		"prompt complete not a dict": `
@mcp.prompt(description="d", complete=["x"])
def p(x):
    return x
`,
		"resource complete without template": `
@mcp.resource("docs://a", complete={"x": "m.f"})
def r():
    return "a"
`,
	} {
		t.Run(label, func(t *testing.T) {
			p := newTestScriptling()
			if _, err := p.Eval("import scriptling.runtime.mcp as mcp\n" + src); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestMCPNotifyResourceUpdated(t *testing.T) {
	t.Cleanup(func() { extlibs.SetMCPResourceNotifier(nil) })

	extlibs.SetMCPResourceNotifier(nil)
	p := newTestScriptling()
	result, err := p.Eval("import scriptling.runtime.mcp as mcp\nmcp.notify_resource_updated(\"notes://1\")")
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if b, _ := result.AsBool(); b {
		t.Errorf("with no server = %v, want False", result.Inspect())
	}

	var got []string
	extlibs.SetMCPResourceNotifier(func(uri string) { got = append(got, uri) })
	result, err = p.Eval("mcp.notify_resource_updated(\"notes://1\")")
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if b, _ := result.AsBool(); !b || len(got) != 1 || got[0] != "notes://1" {
		t.Errorf("result %v, notified %v", result.Inspect(), got)
	}

	if _, err := p.Eval("mcp.notify_resource_updated(\"\")"); err == nil {
		t.Error("empty uri should be an error")
	}
}

func TestMCPToolProgress_OutsideARequestIsANoOp(t *testing.T) {
	p := newTestScriptling()
	mcp.RegisterToolHelpers(p)
	if _, err := p.Eval(`
import scriptling.mcp.tool as tool
tool.progress(1)
tool.progress(2, total=10, message="working")
tool.progress(0.5, 4, "positional")
`); err != nil {
		t.Fatalf("eval: %v", err)
	}
	for _, call := range []string{`tool.progress("1")`, `tool.progress(1, total="2")`, `tool.progress(1, total=0)`, `tool.progress(1, message=3)`, `tool.progress()`} {
		if _, err := p.Eval(call); err == nil {
			t.Errorf("%s: expected an error", call)
		}
	}
}
