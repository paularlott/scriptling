package scriptling

import (
	"testing"

	"github.com/paularlott/scriptling/stdlib"
)

func TestRawFStrings(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected string
	}{
		{
			name:     "rf basic expression",
			code:     `name = "world"; rf"hello {name}"`,
			expected: "hello world",
		},
		{
			name:     "rf preserves backslashes",
			code:     `rf"\d+\.\d+"`,
			expected: `\d+\.\d+`,
		},
		{
			name:     "rf with expression and backslashes",
			code:     `pattern = r"\d+"; rf"match: {pattern}"`,
			expected: `match: \d+`,
		},
		{
			name:     "fr basic expression",
			code:     `name = "world"; fr"hello {name}"`,
			expected: "hello world",
		},
		{
			name:     "fr preserves backslashes",
			code:     `fr"\n\t\r"`,
			expected: `\n\t\r`,
		},
		{
			name:     "rf single quotes",
			code:     `x = 42; rf'value is {x}'`,
			expected: "value is 42",
		},
		{
			name:     "fr single quotes",
			code:     `x = 42; fr'value is {x}'`,
			expected: "value is 42",
		},
		{
			name:     "rf with format spec",
			code:     `x = 3.14159; rf"{x:.2f}"`,
			expected: "3.14",
		},
		{
			name:     "rf empty",
			code:     `rf""`,
			expected: "",
		},
		{
			name:     "rf escaped braces",
			code:     `rf"{{not an expr}}"`,
			expected: "{not an expr}",
		},
		{
			name:     "rf with arithmetic",
			code:     `rf"result: {2 + 3}"`,
			expected: "result: 5",
		},
		{
			name:     "RF uppercase",
			code:     `x = "hi"; RF"{x}"`,
			expected: "hi",
		},
		{
			name:     "Rf mixed case",
			code:     `x = "hi"; Rf"{x}"`,
			expected: "hi",
		},
		{
			name:     "fR mixed case",
			code:     `x = "hi"; fR"{x}"`,
			expected: "hi",
		},
		{
			name:     "rf with regex pattern",
			code:     `import re; p = re.compile(rf"\d{{3}}-\d{{4}}"); p.match("555-1234").group(0)`,
			expected: "555-1234",
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

func TestRawTripleFStrings(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected string
	}{
		{
			name:     "rf triple basic",
			code:     `name = "world"; rf"""hello {name}"""`,
			expected: "hello world",
		},
		{
			name:     "rf triple preserves backslashes",
			code:     `rf"""\d+\.\d+"""`,
			expected: `\d+\.\d+`,
		},
		{
			name:     "rf triple single quotes",
			code:     `x = 42; rf'''value is {x}'''`,
			expected: "value is 42",
		},
		{
			name:     "fr triple basic",
			code:     `name = "world"; fr"""hello {name}"""`,
			expected: "hello world",
		},
		{
			name:     "rf triple multiline",
			code:     "name = \"Paul\"\nrf\"\"\"Hello\n{name}\nWorld\"\"\"",
			expected: "Hello\nPaul\nWorld",
		},
		{
			name:     "rf triple with format spec",
			code:     `x = 3.14159; rf"""{x:.2f}"""`,
			expected: "3.14",
		},
		{
			name:     "rf triple empty",
			code:     `rf""""""`,
			expected: "",
		},
		{
			name:     "rf triple multiple expressions",
			code:     `a = "foo"; b = "bar"; rf"""{a} and {b}"""`,
			expected: "foo and bar",
		},
		{
			name:     "rf triple with escaped braces",
			code:     `rf"""{{literal braces}}"""`,
			expected: "{literal braces}",
		},
		{
			name:     "RF triple uppercase",
			code:     `x = "hi"; RF"""{x}"""`,
			expected: "hi",
		},
		{
			name:     "rf triple with regex",
			code:     `import re; p = re.compile(rf"""\d{{3}}-\d{{4}}"""); p.match("555-1234").group(0)`,
			expected: "555-1234",
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
