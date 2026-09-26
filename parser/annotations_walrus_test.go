package parser

import (
	"strings"
	"testing"

	"github.com/paularlott/scriptling/ast"
	"github.com/paularlott/scriptling/lexer"
)

func TestParseTypeAnnotationsOnParameters(t *testing.T) {
	l := lexer.New(`def add(a: int, b: int = 2, c: "Forward" = 3) -> int:
    return a + b + c
`)
	p := New(l)
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		t.Fatalf("expected no parse errors, got: %v", errs)
	}
	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(program.Statements))
	}
	fn, ok := program.Statements[0].(*ast.FunctionStatement)
	if !ok {
		t.Fatalf("expected FunctionStatement, got %T", program.Statements[0])
	}
	names := make([]string, 0, len(fn.Function.Parameters))
	for _, param := range fn.Function.Parameters {
		names = append(names, param.Value())
	}
	if strings.Join(names, ",") != "a,b,c" {
		t.Fatalf("annotations should not leak into parameter names, got: %v", names)
	}
	if len(fn.Function.GetDefaultValues()) != 2 {
		t.Fatalf("expected defaults for b and c, got %d", len(fn.Function.GetDefaultValues()))
	}
}

func TestParseTypeAnnotationsVariadicKwargs(t *testing.T) {
	l := lexer.New(`def f(*args: tuple, **kw: dict) -> None:
    return args
`)
	p := New(l)
	p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		t.Fatalf("expected no parse errors, got: %v", errs)
	}
}

func TestParseTypeAnnotationGenericType(t *testing.T) {
	// Subscripted generics contain commas and ellipses; both must stay inside
	// the discarded annotation.
	l := lexer.New(`def f(items: dict[str, int], pairs: tuple[int, ...], flag: int | None = None) -> list[str]:
    return items
`)
	p := New(l)
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		t.Fatalf("expected no parse errors, got: %v", errs)
	}
	fn := program.Statements[0].(*ast.FunctionStatement)
	if len(fn.Function.Parameters) != 3 {
		t.Fatalf("expected 3 parameters, got %d", len(fn.Function.Parameters))
	}
}

func TestParseAnnotatedAssignStatement(t *testing.T) {
	l := lexer.New(`count: int = 5
name: str
x: dict[str, int] = {}
`)
	p := New(l)
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		t.Fatalf("expected no parse errors, got: %v", errs)
	}
	if len(program.Statements) != 3 {
		t.Fatalf("expected 3 statements, got %d", len(program.Statements))
	}
	if _, ok := program.Statements[0].(*ast.AssignStatement); !ok {
		t.Fatalf("count: int = 5 should parse as an assignment, got %T", program.Statements[0])
	}
	if _, ok := program.Statements[1].(*ast.PassStatement); !ok {
		t.Fatalf("bare name: str should parse as a no-op, got %T", program.Statements[1])
	}
	if _, ok := program.Statements[2].(*ast.AssignStatement); !ok {
		t.Fatalf("annotated assignment with dict annotation should parse as an assignment, got %T", program.Statements[2])
	}
}

func TestParseWalrusExpression(t *testing.T) {
	l := lexer.New(`if (n := 10) > 5:
    print(n)
`)
	p := New(l)
	p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		t.Fatalf("expected no parse errors, got: %v", errs)
	}
}

func TestParseWalrusInvalidTarget(t *testing.T) {
	l := lexer.New(`x = (1 := 2)
`)
	p := New(l)
	p.ParseProgram()
	errs := p.Errors()
	if len(errs) == 0 {
		t.Fatal("walrus with a literal target should report parser errors")
	}
	if !strings.Contains(errs[0], "walrus") {
		t.Fatalf("first error should name the walrus problem, got: %v", errs[0])
	}
}

func TestParseEllipsis(t *testing.T) {
	l := lexer.New(`def stub() -> None: ...
x = ...
`)
	p := New(l)
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		t.Fatalf("expected no parse errors, got: %v", errs)
	}
	if len(program.Statements) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(program.Statements))
	}
}
