package scriptling

import (
	"os"
	"testing"

	"github.com/paularlott/scriptling/stdlib"
)

func TestFStringEscapeSequences(t *testing.T) {
	p := New()
	stdlib.RegisterAll(p)

	tests := []struct {
		name     string
		code     string
		expected string
	}{
		{
			name:     "newline escape",
			code:     `name = "Paul"; f"\nHello, {name}!"`,
			expected: "\nHello, Paul!",
		},
		{
			name:     "tab escape",
			code:     `value = 42; f"Value:\t{value}"`,
			expected: "Value:\t42",
		},
		{
			name:     "carriage return escape",
			code:     `f"Line1\rLine2"`,
			expected: "Line1\rLine2",
		},
		{
			name:     "backslash escape",
			code:     `name = "Paul"; f"Path: C:\\Users\\{name}"`,
			expected: "Path: C:\\Users\\Paul",
		},
		{
			name:     "double quote escape",
			code:     `name = "Paul"; f"He said: \"{name} is here\""`,
			expected: `He said: "Paul is here"`,
		},
		{
			name:     "single quote in double-quoted f-string",
			code:     `name = "Paul"; f"It's {name}'s book"`,
			expected: "It's Paul's book",
		},
		{
			name:     "multiple escapes",
			code:     `name = "Paul"; value = 42; f"\n\tName: {name}\n\tValue: {value}\n"`,
			expected: "\n\tName: Paul\n\tValue: 42\n",
		},
		{
			name:     "escape before expression",
			code:     `name = "Paul"; f"\nContent: {name}"`,
			expected: "\nContent: Paul",
		},
		{
			name:     "escape after expression",
			code:     `name = "Paul"; f"{name}\nNext line"`,
			expected: "Paul\nNext line",
		},
		{
			name:     "escape between expressions",
			code:     `x = 10; y = 20; f"{x}\n{y}"`,
			expected: "10\n20",
		},
		{
			name:     "escaped braces with newlines",
			code:     `name = "Paul"; f"\n{{{name}}}\n"`,
			expected: "\n{Paul}\n",
		},
		{
			name:     "complex combination",
			code:     `name = "Paul"; value = 42; f"\tUser: \"{name}\"\n\tScore: {value}\n"`,
			expected: "\tUser: \"Paul\"\n\tScore: 42\n",
		},
		{
			name:     "empty f-string with escape",
			code:     `f"\n"`,
			expected: "\n",
		},
		{
			name:     "multiple tabs",
			code:     `name = "Paul"; f"\t\t{name}"`,
			expected: "\t\tPaul",
		},
		{
			name:     "escape at end",
			code:     `name = "Paul"; f"{name}\n"`,
			expected: "Paul\n",
		},
		{
			name:     "mixed quotes and escapes",
			code:     `name = "Paul"; f"Path: \"C:\\Users\\{name}\\Documents\""`,
			expected: `Path: "C:\Users\Paul\Documents"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := p.Eval(tt.code)
			if err != nil {
				t.Fatalf("Eval failed: %v", err)
			}

			if result.Inspect() != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result.Inspect())
			}
		})
	}
}

func TestFStringEscapeSequencesScript(t *testing.T) {
	p := New()
	stdlib.RegisterAll(p)

	content, err := os.ReadFile("tests/test_scripts/fstring_escape_sequences.py")
	if err != nil {
		t.Fatalf("Failed to read test script: %v", err)
	}

	result, err := p.Eval(string(content))
	if err != nil {
		t.Fatalf("Failed to run fstring_escape_sequences.py: %v", err)
	}

	// The script returns True if all tests pass
	resultStr := result.Inspect()
	if resultStr != "True" && resultStr != "true" {
		t.Errorf("Expected True or true, got %s", resultStr)
	}
}

func TestFStringEscapeInLexer(t *testing.T) {
	p := New()
	stdlib.RegisterAll(p)

	// This should not cause a parsing error
	code := `f"He said: \"Hello\""`
	result, err := p.Eval(code)
	if err != nil {
		t.Fatalf("Failed to parse f-string with escaped quotes: %v", err)
	}

	expected := `He said: "Hello"`
	if result.Inspect() != expected {
		t.Errorf("Expected %q, got %q", expected, result.Inspect())
	}
}

func TestFStringEscapeVsRegularString(t *testing.T) {
	p := New()
	stdlib.RegisterAll(p)

	tests := []struct {
		name    string
		fstring string
		regular string
	}{
		{
			name:    "newline",
			fstring: `f"\n"`,
			regular: `"\n"`,
		},
		{
			name:    "tab",
			fstring: `f"\t"`,
			regular: `"\t"`,
		},
		{
			name:    "backslash",
			fstring: `f"\\"`,
			regular: `"\\"`,
		},
		{
			name:    "quote",
			fstring: `f"\""`,
			regular: `"\""`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fResult, err := p.Eval(tt.fstring)
			if err != nil {
				t.Fatalf("F-string eval failed: %v", err)
			}

			rResult, err := p.Eval(tt.regular)
			if err != nil {
				t.Fatalf("Regular string eval failed: %v", err)
			}

			if fResult.Inspect() != rResult.Inspect() {
				t.Errorf("F-string %q and regular string %q should produce same result. Got %q vs %q",
					tt.fstring, tt.regular, fResult.Inspect(), rResult.Inspect())
			}
		})
	}
}
