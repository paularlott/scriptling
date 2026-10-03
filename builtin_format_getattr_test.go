package scriptling

import "testing"

// format(v, spec) must render exactly like f"{v:spec}".
func TestFormatBuiltinMatchesFString(t *testing.T) {
	cases := []struct{ value, spec string }{
		{"42", "05d"}, {"7", "03"}, {"-7", "04d"}, {"1234567", ","}, {"255", "x"}, {"255", "#x"},
		{"255", "08b"}, {"3.14159", ".2f"}, {"3.5", "08.2f"}, {"0.256", ".1%"}, {"12345.678", ",.2f"},
		{"1e20", "e"}, {"5", "+d"}, {"'x'", ">3"}, {"'x'", "*^5"}, {"'abc'", "<6"}, {"42", ""},
		{"True", ""}, {"[1, 2]", ""},
	}
	for _, c := range cases {
		script := "a = format(" + c.value + ", \"" + c.spec + "\")\nb = f\"{" + c.value + ":" + c.spec + "}\"\n"
		if c.spec == "" {
			script = "a = format(" + c.value + ")\nb = f\"{" + c.value + "}\"\n"
		}
		p := New()
		if _, err := p.Eval(script); err != nil {
			t.Fatalf("%s: %v", script, err)
		}
		a, _ := p.GetVarAsString("a")
		b, _ := p.GetVarAsString("b")
		if a != b {
			t.Errorf("format(%s, %q) = %q, f-string gives %q", c.value, c.spec, a, b)
		}
	}
}

// As in Python, format() with a spec needs __format__ on an instance; an
// empty spec falls back to str().
func TestFormatBuiltinInstances(t *testing.T) {
	p := New()
	script := `class P:
    def __str__(self):
        return "pt"
class F:
    def __format__(self, spec):
        return "F" + spec
plain = format(P())
custom = format(F(), ">4")
try:
    format(P(), ">4")
    err = ""
except TypeError as e:
    err = str(e)
`
	if _, err := p.Eval(script); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"plain": "pt", "custom": "F>4", "err": "unsupported format string passed to P.__format__"} {
		if got, _ := p.GetVarAsString(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}

// A missing attribute in getattr/delattr raises a catchable AttributeError
// naming the class.
func TestGetattrDelattrRaiseAttributeError(t *testing.T) {
	for name, call := range map[string]string{
		"getattr": "getattr(p, \"nope\")",
		"delattr": "delattr(p, \"nope\")",
	} {
		t.Run(name, func(t *testing.T) {
			p := New()
			script := "class P:\n    pass\np = P()\nresult = \"\"\ntry:\n    " + call + "\nexcept AttributeError as e:\n    result = str(e)\n"
			if _, err := p.Eval(script); err != nil {
				t.Fatal(err)
			}
			if got, _ := p.GetVarAsString("result"); got != "'P' object has no attribute 'nope'" {
				t.Fatalf("got %q", got)
			}
		})
	}

	p := New()
	if _, err := p.Eval("class P:\n    pass\nresult = getattr(P(), \"nope\", 4)\n"); err != nil {
		t.Fatal(err)
	}
	if got, _ := p.GetVarAsInt("result"); got != 4 {
		t.Fatalf("default: got %d, want 4", got)
	}
}
