package scriptling

import "testing"

// Comprehensions iterate through one shared driver that gives the bound
// variable a slot and produces range() integers directly. These cases pin
// the behaviour that must survive that: a rebound range is honoured, nested
// clauses bind, the variable does not leak, user iterators and raising
// iterators behave as in a for loop.
func TestComprehensionDriverSemantics(t *testing.T) {
	cases := []struct {
		name, script, want string
	}{
		{"range rebound", `
def range(n):
    return ["a", "b"]
result = [x + "!" for x in range(5)]
`, "[a!, b!]"},
		{"range with step and negative step", `
result = [[i for i in range(1, 10, 3)], [i for i in range(5, 0, -2)], [i for i in range(3, 3)]]
`, "[[1, 4, 7], [5, 3, 1], []]"},
		{"nested clauses with condition", `
result = [(a, b) for a in range(3) for b in [10, 20] if a % 2 == 0]
`, "[(0, 10), (0, 20), (2, 10), (2, 20)]"},
		{"variable does not leak and outer survives", `
x = "outer"
squares = [x * x for x in range(4)]
result = [squares, x]
`, "[[0, 1, 4, 9], outer]"},
		{"tuple unpacking target", `
result = [k + str(v) for k, v in {"a": 1, "b": 2}.items()]
result.sort()
`, "[a1, b2]"},
		{"dict and set over range", `
d = {i: i * i for i in range(4) if i}
s = sorted({i % 3 for i in range(10)})
result = [sorted(d.keys()), s]
`, "[[1, 2, 3], [0, 1, 2]]"},
		{"user iterator", `
class Count:
    def __init__(self, n):
        self.n = n
        self.i = 0
    def __iter__(self):
        return self
    def __next__(self):
        if self.i >= self.n:
            raise StopIteration
        self.i += 1
        return self.i
result = [v * 2 for v in Count(3)]
`, "[2, 4, 6]"},
		{"raising iterator propagates", `
class Bad:
    def __iter__(self):
        return self
    def __next__(self):
        raise ValueError("boom")
try:
    [v for v in Bad()]
    result = "no raise"
except ValueError as e:
    result = "caught " + str(e)
`, "caught boom"},
		{"range of non-integers errors like the builtin", `
try:
    [i for i in range("x")]
    result = "no error"
except Exception as e:
    result = "error"
`, "error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := New()
			if _, err := p.Eval(tc.script); err != nil {
				t.Fatalf("eval: %v", err)
			}
			got, errObj := p.GetVarAsObject("result")
			if errObj != nil {
				t.Fatalf("result: %v", errObj)
			}
			if got.Inspect() != tc.want {
				t.Fatalf("got %s, want %s", got.Inspect(), tc.want)
			}
		})
	}
}
