package stdlib_test

// Moved from extlibs/shlex_tempfile_shutil_test.go with the shlex library.

import (
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/stdlib"
)

func TestShlexBuiltinRoundTrip(t *testing.T) {
	p := scriptling.New()
	p.RegisterLibrary(stdlib.ShlexLibrary)

	result, err := p.Eval(`import shlex
shlex.join(shlex.split("cmd --flag 'my value'"))`)
	if err != nil {
		t.Fatal(err)
	}
	s, _ := result.AsString()
	if s != `cmd --flag 'my value'` {
		t.Errorf("round trip: got %q", s)
	}
}
