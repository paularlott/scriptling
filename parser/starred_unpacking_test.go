package parser

import (
	"testing"

	"github.com/paularlott/scriptling/lexer"
)

func TestStarredUnpackingAfterStatement(t *testing.T) {
	input := `a = 1
*b, c = [2, 3]`

	l := lexer.New(input)
	
	// Print all tokens
	t.Log("Tokens:")
	for {
		tok := l.NextToken()
		t.Logf("%s: %q", tok.Type, tok.Literal)
		if tok.Type == "EOF" {
			break
		}
	}
	
	// Re-create lexer for parsing
	l = lexer.New(input)
	p := New(l)
	program := p.ParseProgram()

	if len(p.Errors()) > 0 {
		t.Logf("parser has %d errors:", len(p.Errors()))
		for _, msg := range p.Errors() {
			t.Logf("parser error: %q", msg)
		}
		t.FailNow()
	}

	if len(program.Statements) != 2 {
		t.Fatalf("program should have 2 statements, got %d", len(program.Statements))
	}
}
