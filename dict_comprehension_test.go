package scriptling_test

import (
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/stdlib"
)

func TestDictComprehension(t *testing.T) {
	p := scriptling.New()
	stdlib.RegisterAll(p)

	tests := []struct {
		name   string
		code   string
		varKey string
		want   interface{}
	}{
		{
			name:   "basic dict comprehension",
			code:   "d = {x: x*x for x in range(4)}\nresult = d[3]",
			varKey: "result",
			want:   int64(9),
		},
		{
			name:   "dict comprehension with condition",
			code:   "d = {x: x*2 for x in range(6) if x % 2 == 0}\nresult = len(d)",
			varKey: "result",
			want:   int64(3),
		},
		{
			name:   "dict comprehension from items",
			code:   "src = {\"a\": 1, \"b\": 2}\nd = {k: v*10 for k, v in src.items()}\nresult = d[\"a\"]",
			varKey: "result",
			want:   int64(10),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.Eval(tt.code)
			if err != nil {
				t.Fatalf("eval error: %v", err)
			}
			switch want := tt.want.(type) {
			case int64:
				got, objErr := p.GetVarAsInt(tt.varKey)
				if objErr != nil {
					t.Fatalf("GetVarAsInt error: %v", objErr)
				}
				if got != want {
					t.Errorf("got %d, want %d", got, want)
				}
			}
		})
	}
}
