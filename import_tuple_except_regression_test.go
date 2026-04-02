package scriptling

import (
	"testing"

	"github.com/paularlott/scriptling/libloader"
	"github.com/paularlott/scriptling/stdlib"
)

func TestImportLibraryWithTupleExcept(t *testing.T) {
	p := New()
	stdlib.RegisterAll(p)
	p.SetLibraryLoader(libloader.NewMemoryLoader(map[string]string{
		"buglib": `
def run():
    try:
        raise ValueError("bad")
    except (TypeError, ValueError):
        return "caught"
`,
	}))

	if err := p.Import("buglib"); err != nil {
		t.Fatalf("import failed: %v", err)
	}

	result, err := p.Eval(`
import buglib
buglib.run()
`)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}

	str, objErr := result.AsString()
	if objErr != nil {
		t.Fatalf("result is not a string: %v", objErr)
	}
	if str != "caught" {
		t.Fatalf("result = %q, want %q", str, "caught")
	}
}
