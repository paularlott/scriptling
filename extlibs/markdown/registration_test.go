package markdown_test

import (
	"strings"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/markdown"
	"github.com/paularlott/scriptling/internal/scripttest"
	"github.com/paularlott/scriptling/stdlib"
)

func TestLibraryNameConstant(t *testing.T) {
	if markdown.LibraryName != "scriptling.markdown" {
		t.Errorf("markdown.LibraryName = %q, want %q (scripts import it by this name)", markdown.LibraryName, "scriptling.markdown")
	}
}

func TestMarkdownRegisterAndRender(t *testing.T) {
	p := scriptling.New()
	markdown.Register(p)
	got := scripttest.EvalString(t, p, "import scriptling.markdown as md\nmd.to_html('# Title\\n\\n**bold**')")
	for _, want := range []string{"<h1", "Title</h1>", "<strong>bold</strong>"} {
		if !strings.Contains(got, want) {
			t.Errorf("markdown.to_html output %q lacks %q", got, want)
		}
	}
}

func TestMarkdownNotImportableWithoutRegister(t *testing.T) {
	p := scriptling.New()
	stdlib.RegisterAll(p)
	scripttest.ImportFails(t, p, "scriptling.markdown", "it is only available after markdown.Register(p)")
}
