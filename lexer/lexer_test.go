package lexer

import (
	"github.com/paularlott/scriptling/token"
	"testing"
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
