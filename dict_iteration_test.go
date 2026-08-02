package scriptling_test

import (
	"strings"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/stdlib"
)

// TestDictIteration verifies that iterating a dict directly yields its keys,
// matching CPython semantics. Covers for-loops, comprehensions, and *unpacking,
// all of which previously raised "expected iterable, got DICT".
func TestDictIteration(t *testing.T) {
	p := scriptling.New()
	stdlib.RegisterAll(p)

	tests := []struct {
		name string
		code string
		want string
	}{
		{
			name: "for-in over dict yields keys",
			code: "d = {\"b\": 2, \"a\": 1, \"c\": 3}\n" +
				"out = []\n" +
				"for k in d:\n" +
				"    out.append(k)\n" +
				"result = \",\".join(sorted(out))",
			want: "a,b,c",
		},
		{
			name: "for-in over empty dict",
			code: "count = 0\n" +
				"for k in {}:\n" +
				"    count += 1\n" +
				"result = str(count)",
			want: "0",
		},
		{
			name: "list comprehension over dict yields keys",
			code: "d = {\"b\": 2, \"a\": 1, \"c\": 3}\n" +
				"result = \",\".join(sorted([k for k in d]))",
			want: "a,b,c",
		},
		{
			name: "set comprehension over dict yields keys",
			code: "d = {\"b\": 2, \"a\": 1, \"c\": 3}\n" +
				"result = \",\".join(sorted({k for k in d}))",
			want: "a,b,c",
		},
		{
			name: "star-unpacking a dict yields keys",
			code: "d = {\"b\": 2, \"a\": 1, \"c\": 3}\n" +
				"def gather(*args):\n" +
				"    return args\n" +
				"result = \",\".join(sorted(gather(*d)))",
			want: "a,b,c",
		},
		{
			name: "for-in with value lookup via key",
			code: "d = {\"a\": 1, \"b\": 2}\n" +
				"total = 0\n" +
				"for k in d:\n" +
				"    total += d[k]\n" +
				"result = str(total)",
			want: "3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := p.Eval(tt.code); err != nil {
				t.Fatalf("eval error: %v", err)
			}
			got, objErr := p.GetVarAsString("result")
			if objErr != nil {
				t.Fatalf("GetVarAsString error: %v", objErr)
			}
			if strings.TrimSpace(got) != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// TestDictKeysViewNoSort confirms dict_keys has no .sort() method, matching
// CPython (which raises AttributeError). The Pythonic idiom is sorted(d.keys()).
func TestDictKeysViewNoSort(t *testing.T) {
	p := scriptling.New()
	stdlib.RegisterAll(p)

	// d.keys().sort() must fail, like CPython.
	if _, err := p.Eval(`d = {"b": 2, "a": 1}; d.keys().sort()`); err == nil {
		t.Errorf("d.keys().sort() should raise an error (dict_keys has no .sort() in CPython)")
	}

	// sorted(d.keys()) is the supported idiom and must work.
	if _, err := p.Eval(`d = {"b": 2, "a": 1, "c": 3}; result = ",".join(sorted(d.keys()))`); err != nil {
		t.Fatalf("sorted(d.keys()) should succeed: %v", err)
	}
	got, _ := p.GetVarAsString("result")
	if strings.TrimSpace(got) != "a,b,c" {
		t.Errorf("sorted(d.keys()) got %q, want %q", got, "a,b,c")
	}
}
