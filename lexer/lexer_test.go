package lexer

import (
	"testing"

	"github.com/paularlott/scriptling/token"
)

func TestNextToken(t *testing.T) {
	input := `x = 5
y = 10
z = x + y`

	tests := []struct {
		expectedType    token.TokenType
		expectedLiteral string
	}{
		{token.IDENT, "x"},
		{token.ASSIGN, "="},
		{token.INT, "5"},
		{token.NEWLINE, "\\n"},
		{token.IDENT, "y"},
		{token.ASSIGN, "="},
		{token.INT, "10"},
		{token.NEWLINE, "\\n"},
		{token.IDENT, "z"},
		{token.ASSIGN, "="},
		{token.IDENT, "x"},
		{token.PLUS, "+"},
		{token.IDENT, "y"},
		{token.EOF, ""},
	}

	l := New(input)

	for i, tt := range tests {
		tok := l.NextToken()
		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q",
				i, tt.expectedType, tok.Type)
		}
		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestOperators(t *testing.T) {
	input := `+ - * / % == != < > <= >=`

	tests := []token.TokenType{
		token.PLUS, token.MINUS, token.ASTERISK, token.SLASH, token.PERCENT,
		token.EQ, token.NOT_EQ, token.LT, token.GT, token.LTE, token.GTE,
		token.EOF,
	}

	l := New(input)
	for i, tt := range tests {
		tok := l.NextToken()
		if tok.Type != tt {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q",
				i, tt, tok.Type)
		}
	}
}

func TestKeywords(t *testing.T) {
	input := `if else while def return and or not True False`

	tests := []token.TokenType{
		token.IF, token.ELSE, token.WHILE, token.DEF, token.RETURN,
		token.AND, token.OR, token.NOT, token.TRUE, token.FALSE, token.EOF,
	}

	l := New(input)
	for i, tt := range tests {
		tok := l.NextToken()
		if tok.Type != tt {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q",
				i, tt, tok.Type)
		}
	}
}

func TestStrings(t *testing.T) {
	input := `"hello" 'world'`

	tests := []struct {
		expectedType    token.TokenType
		expectedLiteral string
	}{
		{token.STRING, "hello"},
		{token.STRING, "world"},
		{token.EOF, ""},
	}

	l := New(input)
	for i, tt := range tests {
		tok := l.NextToken()
		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q",
				i, tt.expectedType, tok.Type)
		}
		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestNumbers(t *testing.T) {
	input := `42 3.14 0 100.5`

	tests := []struct {
		expectedType    token.TokenType
		expectedLiteral string
	}{
		{token.INT, "42"},
		{token.FLOAT, "3.14"},
		{token.INT, "0"},
		{token.FLOAT, "100.5"},
		{token.EOF, ""},
	}

	l := New(input)
	for i, tt := range tests {
		tok := l.NextToken()
		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q",
				i, tt.expectedType, tok.Type)
		}
		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestIndentation(t *testing.T) {
	input := `if True:
    x = 1
    y = 2`

	tests := []token.TokenType{
		token.IF, token.TRUE, token.COLON, token.INDENT,
		token.IDENT, token.ASSIGN, token.INT, token.NEWLINE,
		token.IDENT, token.ASSIGN, token.INT, token.DEDENT, token.EOF,
	}

	l := New(input)
	for i, tt := range tests {
		tok := l.NextToken()
		if tok.Type != tt {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q (literal: %q)",
				i, tt, tok.Type, tok.Literal)
		}
	}
}

func TestComments(t *testing.T) {
	input := `x = 5  # This is a comment
y = 10`

	tests := []token.TokenType{
		token.IDENT, token.ASSIGN, token.INT, token.NEWLINE,
		token.IDENT, token.ASSIGN, token.INT, token.EOF,
	}

	l := New(input)
	for i, tt := range tests {
		tok := l.NextToken()
		if tok.Type != tt {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q",
				i, tt, tok.Type)
		}
	}
}

func TestRawAndTripleStrings(t *testing.T) {
	input := `"""a
b
c""" r"a\b\c" r'href=["\'](.*?)[\'"]'`

	tests := []struct {
		expectedType    token.TokenType
		expectedLiteral string
	}{
		{token.STRING, "a\nb\nc"},
		{token.STRING, "a\\b\\c"},
		{token.STRING, "href=[\"\\'](.*?)[\\'\"]"},
		{token.EOF, ""},
	}

	l := New(input)
	for i, tt := range tests {
		tok := l.NextToken()
		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q",
				i, tt.expectedType, tok.Type)
		}
		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestRawFStrings(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		expectedType    token.TokenType
		expectedLiteral string
	}{
		{"rf double quote", `rf"hello\n{world}"`, token.RF_STRING, "hello\\n{world}"},
		{"rf single quote", `rf'hello\n{world}'`, token.RF_STRING, "hello\\n{world}"},
		{"fr double quote", `fr"hello\n{world}"`, token.RF_STRING, "hello\\n{world}"},
		{"fr single quote", `fr'hello\n{world}'`, token.RF_STRING, "hello\\n{world}"},
		{"RF double quote", `RF"hello\n"`, token.RF_STRING, "hello\\n"},
		{"Fr single quote", `Fr'hello\t'`, token.RF_STRING, "hello\\t"},
		{"fR double quote", `fR"hello\d+"`, token.RF_STRING, "hello\\d+"},
		{"Rf single quote", `Rf'hello\s+'`, token.RF_STRING, "hello\\s+"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := New(tt.input)
			tok := l.NextToken()
			if tok.Type != tt.expectedType {
				t.Fatalf("tokentype wrong. expected=%q, got=%q", tt.expectedType, tok.Type)
			}
			if tok.Literal != tt.expectedLiteral {
				t.Fatalf("literal wrong. expected=%q, got=%q", tt.expectedLiteral, tok.Literal)
			}
		})
	}
}

func TestRawTripleFStrings(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		expectedType    token.TokenType
		expectedLiteral string
	}{
		{"rf triple double", `rf"""hello\n{world}"""`, token.RF_STRING, "hello\\n{world}"},
		{"rf triple single", `rf'''hello\n{world}'''`, token.RF_STRING, "hello\\n{world}"},
		{"fr triple double", `fr"""hello\n{world}"""`, token.RF_STRING, "hello\\n{world}"},
		{"fr triple single", `fr'''hello\n{world}'''`, token.RF_STRING, "hello\\n{world}"},
		{"RF triple double", `RF"""hello\tworld"""`, token.RF_STRING, "hello\\tworld"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := New(tt.input)
			tok := l.NextToken()
			if tok.Type != tt.expectedType {
				t.Fatalf("tokentype wrong. expected=%q, got=%q", tt.expectedType, tok.Type)
			}
			if tok.Literal != tt.expectedLiteral {
				t.Fatalf("literal wrong. expected=%q, got=%q", tt.expectedLiteral, tok.Literal)
			}
		})
	}
}

func TestLineNumbers(t *testing.T) {
	tests := []struct {
		name  string
		input string
		// Each entry is (tokenType, expectedLine). Only tokens we care about checking.
		checks []struct {
			typ  token.TokenType
			line int
		}
	}{
		{
			name:  "simple single-line tokens all on line 1",
			input: "x = 5",
			checks: []struct {
				typ  token.TokenType
				line int
			}{
				{token.IDENT, 1},
				{token.ASSIGN, 1},
				{token.INT, 1},
			},
		},
		{
			name:  "tokens on multiple lines",
			input: "x = 1\ny = 2\nz = 3",
			checks: []struct {
				typ  token.TokenType
				line int
			}{
				{token.IDENT, 1}, // x
				{token.ASSIGN, 1},
				{token.INT, 1},
				{token.NEWLINE, 1},
				{token.IDENT, 2}, // y
				{token.ASSIGN, 2},
				{token.INT, 2},
				{token.NEWLINE, 2},
				{token.IDENT, 3}, // z
				{token.ASSIGN, 3},
				{token.INT, 3},
			},
		},
		{
			name:  "triple-quoted string advances line counter",
			input: "x = \"\"\"\nline2\nline3\n\"\"\"\ny = 1",
			checks: []struct {
				typ  token.TokenType
				line int
			}{
				{token.IDENT, 1},   // x
				{token.ASSIGN, 1},
				{token.STRING, 1},  // the triple-quoted string token itself
				{token.NEWLINE, 4}, // newline after closing """
				{token.IDENT, 5},   // y — must be on line 5, not line 2
				{token.ASSIGN, 5},
				{token.INT, 5},
			},
		},
		{
			name:  "single-quoted triple string advances line counter",
			input: "a = '''\nfoo\nbar\n'''\nb = 2",
			checks: []struct {
				typ  token.TokenType
				line int
			}{
				{token.IDENT, 1},  // a
				{token.STRING, 1}, // triple-quoted string
				{token.NEWLINE, 4},
				{token.IDENT, 5},  // b
				{token.INT, 5},
			},
		},
		{
			name:  "raw triple-quoted string advances line counter",
			input: "a = r\"\"\"\nfoo\nbar\n\"\"\"\nb = 2",
			checks: []struct {
				typ  token.TokenType
				line int
			}{
				{token.IDENT, 1},  // a
				{token.STRING, 1}, // raw triple-quoted string
				{token.NEWLINE, 4},
				{token.IDENT, 5},  // b
				{token.INT, 5},
			},
		},
		{
			name:  "code after triple-quoted string has correct line numbers",
			input: "def foo():\n    x = \"\"\"\n    line2\n    line3\n    \"\"\"\n    return x\n\ny = 1",
			checks: []struct {
				typ  token.TokenType
				line int
			}{
				{token.DEF, 1},
				{token.IDENT, 1},   // foo
				{token.STRING, 2},  // triple-quoted string starts on line 2
				{token.RETURN, 6},  // return is on line 6
				{token.IDENT, 6},   // x in "return x" is on line 6
				{token.IDENT, 8},   // y is on line 8
				{token.INT, 8},
			},
		},
		{
			name:  "multiple triple-quoted strings accumulate line counts",
			input: "a = \"\"\"\none\n\"\"\"\nb = \"\"\"\ntwo\nthree\n\"\"\"\nc = 1",
			checks: []struct {
				typ  token.TokenType
				line int
			}{
				{token.IDENT, 1},  // a
				{token.STRING, 1}, // first triple string
				{token.NEWLINE, 3},
				{token.IDENT, 4},  // b
				{token.STRING, 4}, // second triple string
				{token.NEWLINE, 7},
				{token.IDENT, 8},  // c
				{token.INT, 8},
			},
		},
		{
			name:  "regular string does not affect line counting",
			input: "x = \"hello\"\ny = 2",
			checks: []struct {
				typ  token.TokenType
				line int
			}{
				{token.IDENT, 1},
				{token.STRING, 1},
				{token.NEWLINE, 1},
				{token.IDENT, 2},
				{token.INT, 2},
			},
		},
		{
			name:  "comment lines are counted",
			input: "# comment\nx = 1\n# another\ny = 2",
			checks: []struct {
				typ  token.TokenType
				line int
			}{
				{token.IDENT, 2}, // x — comment on line 1 is skipped
				{token.INT, 2},
				{token.NEWLINE, 2},
				{token.IDENT, 4}, // y — comment on line 3 is skipped
				{token.INT, 4},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := New(tt.input)
			checkIdx := 0
			for checkIdx < len(tt.checks) {
				tok := l.NextToken()
				if tok.Type == token.EOF {
					break
				}
				// Skip tokens we're not checking
				if tok.Type != tt.checks[checkIdx].typ {
					continue
				}
				want := tt.checks[checkIdx].line
				if tok.Line != want {
					t.Errorf("check[%d]: token %q: expected line %d, got %d",
						checkIdx, tok.Literal, want, tok.Line)
				}
				checkIdx++
			}
			if checkIdx < len(tt.checks) {
				t.Errorf("only matched %d of %d expected tokens", checkIdx, len(tt.checks))
			}
		})
	}
}
