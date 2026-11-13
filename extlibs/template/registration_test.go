package template_test

import (
	"fmt"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/template"
	"github.com/paularlott/scriptling/internal/scripttest"
	"github.com/paularlott/scriptling/stdlib"
)

func TestLibraryNameConstants(t *testing.T) {
	if template.HTMLLibraryName != "scriptling.template.html" {
		t.Errorf("template.HTMLLibraryName = %q, want scriptling.template.html", template.HTMLLibraryName)
	}
	if template.TextLibraryName != "scriptling.template.text" {
		t.Errorf("template.TextLibraryName = %q, want scriptling.template.text", template.TextLibraryName)
	}
}

const renderSrc = `import %s as tpl
s = tpl.Set()
s.add("Hi {{.Name}}")
s.render({"Name": "<b>"})`

func TestTemplateHTMLRegisterAndRender(t *testing.T) {
	p := scriptling.New()
	template.RegisterHTML(p)
	if got := scripttest.EvalString(t, p, fmt.Sprintf(renderSrc, "scriptling.template.html")); got != "Hi &lt;b&gt;" {
		t.Errorf("html template render = %q, want the value HTML-escaped", got)
	}
	scripttest.ImportFails(t, p, "scriptling.template.text", "RegisterHTML must not register the text library")
}

func TestTemplateTextRegisterAndRender(t *testing.T) {
	p := scriptling.New()
	template.RegisterText(p)
	if got := scripttest.EvalString(t, p, fmt.Sprintf(renderSrc, "scriptling.template.text")); got != "Hi <b>" {
		t.Errorf("text template render = %q, want the value unescaped", got)
	}
	scripttest.ImportFails(t, p, "scriptling.template.html", "RegisterText must not register the html library")
}

func TestTemplateNotImportableWithoutRegister(t *testing.T) {
	for _, name := range []string{"scriptling.template.html", "scriptling.template.text"} {
		p := scriptling.New()
		stdlib.RegisterAll(p)
		scripttest.ImportFails(t, p, name, "it is only available after template.RegisterHTML/RegisterText(p)")
	}
}

func TestTemplateBadSourceIsCatchable(t *testing.T) {
	p := scriptling.New()
	template.RegisterText(p)
	if typ, _ := scripttest.Raises(t, p, "import scriptling.template.text as tpl\ns = tpl.Set()\ns.add('{{.Name')\ns.render({})"); typ == "" {
		t.Error("a malformed template must raise")
	}
}
