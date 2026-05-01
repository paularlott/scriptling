package scriptling

import (
	"testing"

	"github.com/paularlott/scriptling/stdlib"
)

func TestTripleQuotedFStrings(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected string
	}{
		{
			name:     "basic triple double-quoted fstring",
			code:     `name = "world"; f"""hello {name}"""`,
			expected: "hello world",
		},
		{
			name:     "basic triple single-quoted fstring",
			code:     `name = "world"; f'''hello {name}'''`,
			expected: "hello world",
		},
		{
			name:     "multiline triple fstring",
			code:     "name = \"Paul\"\nf\"\"\"Hello\n{name}\"\"\"",
			expected: "Hello\nPaul",
		},
		{
			name:     "triple fstring with embedded double quotes",
			code:     `name = "Paul"; f"""He said "{name}" is here"""`,
			expected: `He said "Paul" is here`,
		},
		{
			name:     "triple fstring with multiple expressions",
			code:     `a = "foo"; b = "bar"; f"""{a} and {b}"""`,
			expected: "foo and bar",
		},
		{
			name:     "triple fstring with format spec",
			code:     `x = 3.14159; f"""{x:.2f}"""`,
			expected: "3.14",
		},
		{
			name:     "triple fstring empty",
			code:     `f""""""`,
			expected: "",
		},
		{
			name:     "triple fstring no expressions",
			code:     `f"""just plain text"""`,
			expected: "just plain text",
		},
		{
			name:     "triple fstring with newlines and expressions",
			code:     "sender = \"Alice\"\nrecipient = \"Bob\"\nf\"\"\"From: {sender}\nTo: {recipient}\"\"\"",
			expected: "From: Alice\nTo: Bob",
		},
		{
			name:     "uppercase F triple fstring",
			code:     `name = "world"; F"""hello {name}"""`,
			expected: "hello world",
		},
		{
			name:     "triple fstring with integer expression",
			code:     `x = 42; f"""the answer is {x}"""`,
			expected: "the answer is 42",
		},
		{
			name:     "triple fstring with arithmetic",
			code:     `f"""result: {2 + 3}"""`,
			expected: "result: 5",
		},
		{
			name:     "triple fstring with escaped braces",
			code:     `f"""{{literal braces}}"""`,
			expected: "{literal braces}",
		},
		{
			name:     "empty fstring is not triple quoted",
			code:     `x = f""; x`,
			expected: "",
		},
		{
			name:     "empty fstring in list",
			code:     `result = [f"## Title", f"", f"**bold**"]; result[1]`,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New()
			stdlib.RegisterAll(p)
			result, err := p.Eval(tt.code)
			if err != nil {
				t.Fatalf("eval error: %v", err)
			}
			got := result.Inspect()
			if got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}
