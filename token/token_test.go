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
	tests := []struct {
		type_   TokenType
		literal string
		line    int
	}{
		{IDENT, "x", 1},
		{INT, "42", 1},
		{STRING, `"hello"`, 2},
		{PLUS, "+", 3},
		{MINUS, "-", 4},
	}

	for _, tt := range tests {
		tok := Token{Type: tt.type_, Literal: tt.literal, Line: tt.line}
		if tok.Type != tt.type_ {
			t.Errorf("Token.Type = %v, want %v", tok.Type, tt.type_)
		}
		if tok.Literal != tt.literal {
			t.Errorf("Token.Literal = %v, want %v", tok.Literal, tt.literal)
		}
		if tok.Line != tt.line {
			t.Errorf("Token.Line = %v, want %v", tok.Line, tt.line)
		}
	}
}

func TestLookupIdent(t *testing.T) {
	tests := []struct {
		input    string
		expected TokenType
	}{
		// Keywords
		{"True", TRUE},
		{"False", FALSE},
		{"None", NONE},
		{"import", IMPORT},
		{"from", FROM},
		{"if", IF},
		{"elif", ELIF},
		{"else", ELSE},
		{"while", WHILE},
		{"for", FOR},
		{"in", IN},
		{"def", DEF},
		{"class", CLASS},
		{"return", RETURN},
		{"break", BREAK},
		{"continue", CONTINUE},
		{"pass", PASS},
		{"and", AND},
		{"or", OR},
		{"not", NOT},
		{"is", IS},
		{"try", TRY},
		{"except", EXCEPT},
		{"finally", FINALLY},
		{"raise", RAISE},
		{"global", GLOBAL},
		{"nonlocal", NONLOCAL},
		{"lambda", LAMBDA},
		{"as", AS},
		{"assert", ASSERT},
		{"match", MATCH},
		{"case", CASE},

		// Identifiers
		{"x", IDENT},
		{"foo", IDENT},
		{"myVar", IDENT},
		{"_private", IDENT},
		{"with_underscore", IDENT},
	}

	for _, tt := range tests {
		tok := LookupIdent(tt.input)
		if tok != tt.expected {
			t.Errorf("LookupIdent(%q) = %v, want %v", tt.input, tok, tt.expected)
		}
	}
}

func TestAdditionalTokenTypes(t *testing.T) {
	// Test additional token types not in original test
	additionalTests := []struct {
		token    TokenType
		expected string
	}{
		{F_STRING, "F_STRING"},
		{PLUS_EQ, "+="},
		{MINUS_EQ, "-="},
		{MUL_EQ, "*="},
		{DIV_EQ, "/="},
		{FLOORDIV_EQ, "//="},
		{MOD_EQ, "%="},
		{POW, "**"},
		{FLOORDIV, "//"},
		{PERCENT, "%"},
		{TILDE, "~"},
		{AMPERSAND, "&"},
		{PIPE, "|"},
		{CARET, "^"},
		{LSHIFT, "<<"},
		{RSHIFT, ">>"},
		{AND_EQ, "&="},
		{OR_EQ, "|="},
		{XOR_EQ, "^="},
		{LSHIFT_EQ, "<<="},
		{RSHIFT_EQ, ">>="},
		{LTE, "<="},
		{GTE, ">="},
		{DOT, "."},
		{SEMICOLON, ";"},
		{WHILE, "WHILE"},
		{FOR, "FOR"},
		{IN, "IN"},
		{DEF, "DEF"},
		{CLASS, "CLASS"},
		{BREAK, "BREAK"},
		{CONTINUE, "CONTINUE"},
		{PASS, "PASS"},
		{AND, "AND"},
		{OR, "OR"},
		{NOT, "NOT"},
		{NOT_IN, "NOT_IN"},
		{IS_NOT, "IS_NOT"},
		{TRY, "TRY"},
		{EXCEPT, "EXCEPT"},
		{FINALLY, "FINALLY"},
		{RAISE, "RAISE"},
		{GLOBAL, "GLOBAL"},
		{NONLOCAL, "NONLOCAL"},
		{LAMBDA, "LAMBDA"},
		{AS, "AS"},
		{ASSERT, "ASSERT"},
		{MATCH, "MATCH"},
		{CASE, "CASE"},
	}

	for _, tt := range additionalTests {
		if string(tt.token) != tt.expected {
			t.Errorf("TokenType string mismatch: got %q, want %q", string(tt.token), tt.expected)
		}
	}
}