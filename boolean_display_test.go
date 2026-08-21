package scriptling

import (
	"testing"

	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/stdlib"
)

// Boolean display follows Python 3: print, str(), repr() and f-strings all
// render True/False. Machine-facing formats are covered separately below.
func TestBooleanDisplayMatchesPython(t *testing.T) {
	p := New()
	stdlib.RegisterAll(p)
	p.EnableOutputCapture()
	_, err := p.Eval(`
print(True, False)
s = str(True) + "," + str(False)
r = repr(False)
f = f"{True}-{False}"
lst = str([True, False])
`)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	if got := p.GetOutput(); got != "True False\n" {
		t.Errorf("print output = %q, want %q", got, "True False\n")
	}
	s, objErr := p.GetVar("s")
	if objErr != nil || s != "True,False" {
		t.Errorf("str() = %v, want True,False", s)
	}
	r, objErr := p.GetVar("r")
	if objErr != nil || r != "False" {
		t.Errorf("repr() = %v, want False", r)
	}
	f, objErr := p.GetVar("f")
	if objErr != nil || f != "True-False" {
		t.Errorf("f-string = %v, want True-False", f)
	}
	lst, objErr := p.GetVar("lst")
	if objErr != nil || lst != "[True, False]" {
		t.Errorf("str([True, False]) = %v, want [True, False]", lst)
	}
}

// json.dumps must keep emitting lowercase true/false regardless of the
// display form.
func TestJSONDumpsBooleanStillLowercase(t *testing.T) {
	p := New()
	stdlib.RegisterAll(p)
	p.EnableOutputCapture()
	_, err := p.Eval(`
import json
print(json.dumps(True))
print(json.dumps({"ok": False}))
`)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	want := "true\n{\"ok\":false}\n"
	if got := p.GetOutput(); got != want {
		t.Errorf("json.dumps output = %q, want %q", got, want)
	}
}

// CoerceWireString is the machine-facing form used by wire formats: booleans
// render lowercase while other types coerce exactly like CoerceString.
func TestCoerceWireStringBooleansLowercase(t *testing.T) {
	cases := []struct {
		obj      object.Object
		expected string
	}{
		{object.NewBoolean(true), "true"},
		{object.NewBoolean(false), "false"},
		{object.NewInteger(42), "42"},
		{object.NewFloat(3.5), "3.5"},
		{object.NewString("hello"), "hello"},
	}
	for _, tt := range cases {
		s, errObj := object.CoerceWireString(tt.obj)
		if errObj != nil {
			t.Errorf("CoerceWireString(%s) error: %v", tt.obj.Inspect(), errObj)
			continue
		}
		if s != tt.expected {
			t.Errorf("CoerceWireString(%s) = %q, want %q", tt.obj.Inspect(), s, tt.expected)
		}
	}
}
