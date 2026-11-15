package conversion_test

import (
	"runtime"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/conversion"
	"github.com/paularlott/scriptling/object"
)

// ToGoLimited counts the bytes of shared strings: a list holding one 1 MB
// string a hundred times converts to a Go value sharing the string, but
// encodes to 100 MB, and is refused under an 8 MB value limit, as is a
// doubling chain of shared lists standing for 2**60 values, both within a
// small multiple of the limit.
func TestToGoLimitedBoundsSharedValues(t *testing.T) {
	p := scriptling.New(scriptling.WithMemoryLimit(256<<20), scriptling.WithMaxValueBytes(8<<20))
	for _, script := range []string{
		`s = "x" * 1000000
[s] * 100`,
		`x = [0]
for i in range(60):
    x = [x, x]
x`,
		`d = {"k": "v" * 100000}
[d] * 1000`,
	} {
		v, err := p.Eval(script)
		if err != nil {
			t.Fatal(err)
		}
		release := object.BindGuard(p.ResourceGuard())
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		before := m.TotalAlloc
		_, errObj := conversion.ToGoLimited(v)
		_, checkedErr := conversion.ToGoChecked(v)
		runtime.ReadMemStats(&m)
		release()
		if errObj == nil || checkedErr == nil {
			t.Fatalf("%s: converted without a MemoryError (%v, %v)", script, errObj, checkedErr)
		}
		if allocated := m.TotalAlloc - before; allocated > 256<<20 {
			t.Fatalf("%s: allocated %d MB", script, allocated>>20)
		}
	}
}

// With no limits the conversion is complete.
func TestToGoLimitedNoLimits(t *testing.T) {
	p := scriptling.New()
	v, err := p.Eval(`s = "x" * 1000
{"a": [s] * 100, "b": (1, 2.5, None, True)}`)
	if err != nil {
		t.Fatal(err)
	}
	g, errObj := conversion.ToGoLimited(v)
	if errObj != nil {
		t.Fatal(errObj)
	}
	m := g.(map[string]interface{})
	if len(m["a"].([]interface{})) != 100 || len(m["b"].([]interface{})) != 4 {
		t.Fatalf("got %v", g)
	}
}
