package token

import "testing"

func TestTokenType(t *testing.T) {
	tests := []struct {
		token    TokenType
		expected string
	}{
		{ILLEGAL, "ILLEGAL"},
		{EOF, "EOF"},
		{IDENT, "IDENT"},
		{INT, "INT"},
		{FLOAT, "FLOAT"},
		{STRING, "STRING"},
		{ASSIGN, "="},
		{PLUS, "+"},
		{MINUS, "-"},
		{ASTERISK, "*"},
		{SLASH, "/"},
		{LT, "<"},
		{GT, ">"},
		{COMMA, ","},
		{LPAREN, "("},
		{RPAREN, ")"},
		{LBRACE, "{"},
		{RBRACE, "}"},
		{LBRACKET, "["},
		{RBRACKET, "]"},
		{TRUE, "TRUE"},
		{FALSE, "FALSE"},
		{IF, "IF"},
		{ELSE, "ELSE"},
		{RETURN, "RETURN"},
		{EQ, "=="},
		{NOT_EQ, "!="},
		{COLON, ":"},
		{NEWLINE, "NEWLINE"},
		{INDENT, "INDENT"},
		{DEDENT, "DEDENT"},
	}

	for _, tt := range tests {
		if string(tt.token) != tt.expected {
			t.Errorf("TokenType string mismatch: got %q, want %q", string(tt.token), tt.expected)
		}
	}
}

func TestToken(t *testing.T) {
	tok := Token{
		Type:    IDENT,
		Literal: "test",
	}

	if tok.Type != IDENT {
		t.Errorf("Token.Type = %q, want %q", tok.Type, IDENT)
	}

	if tok.Literal != "test" {
		t.Errorf("Token.Literal = %q, want %q", tok.Literal, "test")
	}
}