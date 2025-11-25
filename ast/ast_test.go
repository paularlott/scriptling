package ast

import "testing"

func TestProgram(t *testing.T) {
	program := &Program{
		Statements: []Statement{
			&AssignStatement{
				Name: &Identifier{Value: "myVar"},
				Value: &Identifier{Value: "anotherVar"},
			},
		},
	}

	if len(program.Statements) != 1 {
		t.Errorf("program has wrong number of statements. got=%d", len(program.Statements))
	}
}

func TestIdentifier(t *testing.T) {
	ident := &Identifier{Value: "foobar"}
	
	if ident.Value != "foobar" {
		t.Errorf("ident.Value = %q, want %q", ident.Value, "foobar")
	}
}

func TestIntegerLiteral(t *testing.T) {
	lit := &IntegerLiteral{Value: 5}
	
	if lit.Value != 5 {
		t.Errorf("lit.Value = %d, want %d", lit.Value, 5)
	}
}

func TestFloatLiteral(t *testing.T) {
	lit := &FloatLiteral{Value: 3.14}
	
	if lit.Value != 3.14 {
		t.Errorf("lit.Value = %f, want %f", lit.Value, 3.14)
	}
}

func TestStringLiteral(t *testing.T) {
	lit := &StringLiteral{Value: "hello world"}
	
	if lit.Value != "hello world" {
		t.Errorf("lit.Value = %q, want %q", lit.Value, "hello world")
	}
}

func TestBoolean(t *testing.T) {
	trueBool := &Boolean{Value: true}
	falseBool := &Boolean{Value: false}
	
	if trueBool.Value != true {
		t.Errorf("trueBool.Value = %t, want %t", trueBool.Value, true)
	}
	
	if falseBool.Value != false {
		t.Errorf("falseBool.Value = %t, want %t", falseBool.Value, false)
	}
}