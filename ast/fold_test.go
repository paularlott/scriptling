package ast

import (
	"math"
	"strings"
	"testing"
)

func intLit(v int64) *IntegerLiteral {
	return &IntegerLiteral{Value: v}
}

func floatLit(v float64) *FloatLiteral {
	return &FloatLiteral{Value: v}
}

func strLit(v string) *StringLiteral {
	return &StringLiteral{Value: v}
}

func boolLit(v bool) *Boolean {
	return &Boolean{Value: v}
}

func infix(op Op, left, right Expression) *InfixExpression {
	return &InfixExpression{Operator: op, Left: left, Right: right}
}

func prefix(op Op, right Expression) *PrefixExpression {
	return &PrefixExpression{Operator: op, Right: right}
}

func exprStmt(expr Expression) *ExpressionStatement {
	return &ExpressionStatement{Expression: expr}
}

func foldExpr(expr Expression) Expression {
	prog := &Program{
		Statements: []Statement{exprStmt(expr)},
	}
	FoldConstants(prog)
	return prog.Statements[0].(*ExpressionStatement).Expression
}

func TestFoldNilProgram(t *testing.T) {
	FoldConstants(nil)
}

func TestFoldIntegerArithmetic(t *testing.T) {
	tests := []struct {
		name     string
		input    Expression
		expected int64
	}{
		{"add", infix(OpAdd, intLit(3), intLit(4)), 7},
		{"sub", infix(OpSub, intLit(10), intLit(3)), 7},
		{"mul", infix(OpMul, intLit(3), intLit(4)), 12},
		{"neg add", infix(OpAdd, intLit(-3), intLit(4)), 1},
		{"neg mul", infix(OpMul, intLit(-3), intLit(4)), -12},
		{"nested", infix(OpAdd, infix(OpMul, intLit(2), intLit(3)), intLit(4)), 10},
		{"deep nested", infix(OpAdd, infix(OpMul, infix(OpAdd, intLit(1), intLit(2)), intLit(3)), intLit(4)), 13},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := foldExpr(tt.input)
			lit, ok := result.(*IntegerLiteral)
			if !ok {
				t.Fatalf("expected *IntegerLiteral, got %T", result)
			}
			if lit.Value != tt.expected {
				t.Errorf("got %d, want %d", lit.Value, tt.expected)
			}
		})
	}
}

func TestFoldIntegerDivision(t *testing.T) {
	t.Run("true div", func(t *testing.T) {
		result := foldExpr(infix(OpDiv, intLit(10), intLit(3)))
		lit, ok := result.(*FloatLiteral)
		if !ok {
			t.Fatalf("expected *FloatLiteral, got %T", result)
		}
		if lit.Value != 10.0/3.0 {
			t.Errorf("got %f, want %f", lit.Value, 10.0/3.0)
		}
	})

	t.Run("floor div", func(t *testing.T) {
		result := foldExpr(infix(OpFloorDiv, intLit(10), intLit(3)))
		lit, ok := result.(*IntegerLiteral)
		if !ok {
			t.Fatalf("expected *IntegerLiteral, got %T", result)
		}
		if lit.Value != 3 {
			t.Errorf("got %d, want 3", lit.Value)
		}
	})

	t.Run("floor div negative", func(t *testing.T) {
		result := foldExpr(infix(OpFloorDiv, intLit(-7), intLit(2)))
		lit, ok := result.(*IntegerLiteral)
		if !ok {
			t.Fatalf("expected *IntegerLiteral, got %T", result)
		}
		if lit.Value != -3 {
			t.Errorf("got %d, want -3 (matching runtime truncation)", lit.Value)
		}
	})

	t.Run("mod", func(t *testing.T) {
		result := foldExpr(infix(OpMod, intLit(10), intLit(3)))
		lit, ok := result.(*IntegerLiteral)
		if !ok {
			t.Fatalf("expected *IntegerLiteral, got %T", result)
		}
		if lit.Value != 1 {
			t.Errorf("got %d, want 1", lit.Value)
		}
	})

	t.Run("mod negative", func(t *testing.T) {
		result := foldExpr(infix(OpMod, intLit(-7), intLit(2)))
		lit, ok := result.(*IntegerLiteral)
		if !ok {
			t.Fatalf("expected *IntegerLiteral, got %T", result)
		}
		if lit.Value != -1 {
			t.Errorf("got %d, want -1 (matching runtime)", lit.Value)
		}
	})

	t.Run("div by zero not folded", func(t *testing.T) {
		expr := infix(OpDiv, intLit(10), intLit(0))
		result := foldExpr(expr)
		infixResult, ok := result.(*InfixExpression)
		if !ok {
			t.Fatalf("expected *InfixExpression (not folded), got %T", result)
		}
		if infixResult.Operator != OpDiv {
			t.Errorf("expected OpDiv, got %v", infixResult.Operator)
		}
	})

	t.Run("floor div by zero not folded", func(t *testing.T) {
		expr := infix(OpFloorDiv, intLit(10), intLit(0))
		result := foldExpr(expr)
		_, ok := result.(*InfixExpression)
		if !ok {
			t.Fatalf("expected *InfixExpression (not folded), got %T", result)
		}
	})

	t.Run("mod by zero not folded", func(t *testing.T) {
		expr := infix(OpMod, intLit(10), intLit(0))
		result := foldExpr(expr)
		_, ok := result.(*InfixExpression)
		if !ok {
			t.Fatalf("expected *InfixExpression (not folded), got %T", result)
		}
	})
}

func TestFoldIntegerPower(t *testing.T) {
	tests := []struct {
		name     string
		input    Expression
		expected int64
	}{
		{"2^10", infix(OpPow, intLit(2), intLit(10)), 1024},
		{"3^3", infix(OpPow, intLit(3), intLit(3)), 27},
		{"0^0", infix(OpPow, intLit(0), intLit(0)), 1},
		{"0^5", infix(OpPow, intLit(0), intLit(5)), 0},
		{"1^100", infix(OpPow, intLit(1), intLit(100)), 1},
		{"(-1)^2", infix(OpPow, intLit(-1), intLit(2)), 1},
		{"(-1)^3", infix(OpPow, intLit(-1), intLit(3)), -1},
		{"5^1", infix(OpPow, intLit(5), intLit(1)), 5},
		{"7^0", infix(OpPow, intLit(7), intLit(0)), 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := foldExpr(tt.input)
			lit, ok := result.(*IntegerLiteral)
			if !ok {
				t.Fatalf("expected *IntegerLiteral, got %T", result)
			}
			if lit.Value != tt.expected {
				t.Errorf("got %d, want %d", lit.Value, tt.expected)
			}
		})
	}

	t.Run("negative exponent returns float", func(t *testing.T) {
		result := foldExpr(infix(OpPow, intLit(2), intLit(-1)))
		lit, ok := result.(*FloatLiteral)
		if !ok {
			t.Fatalf("expected *FloatLiteral, got %T", result)
		}
		if lit.Value != 0.5 {
			t.Errorf("got %f, want 0.5", lit.Value)
		}
	})

	t.Run("overflow not folded", func(t *testing.T) {
		expr := infix(OpPow, intLit(2), intLit(63))
		result := foldExpr(expr)
		_, ok := result.(*InfixExpression)
		if !ok {
			t.Fatalf("expected *InfixExpression (not folded), got %T", result)
		}
	})

	t.Run("large exponent not folded", func(t *testing.T) {
		expr := infix(OpPow, intLit(2), intLit(63))
		result := foldExpr(expr)
		_, ok := result.(*InfixExpression)
		if !ok {
			t.Fatalf("expected *InfixExpression (not folded), got %T", result)
		}
	})

	t.Run("0^negative not folded (runtime error)", func(t *testing.T) {
		expr := infix(OpPow, intLit(0), intLit(-1))
		result := foldExpr(expr)
		_, ok := result.(*InfixExpression)
		if !ok {
			t.Fatalf("expected *InfixExpression (not folded), got %T", result)
		}
	})
}

func TestFoldFloatArithmetic(t *testing.T) {
	tests := []struct {
		name     string
		input    Expression
		expected float64
	}{
		{"add", infix(OpAdd, floatLit(3.5), floatLit(2.5)), 6.0},
		{"sub", infix(OpSub, floatLit(10.0), floatLit(3.0)), 7.0},
		{"mul", infix(OpMul, floatLit(2.5), floatLit(4.0)), 10.0},
		{"div", infix(OpDiv, floatLit(10.0), floatLit(4.0)), 2.5},
		{"mixed add", infix(OpAdd, intLit(3), floatLit(2.5)), 5.5},
		{"mixed mul", infix(OpMul, intLit(3), floatLit(2.0)), 6.0},
		{"mixed div", infix(OpDiv, intLit(10), floatLit(4.0)), 2.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := foldExpr(tt.input)
			lit, ok := result.(*FloatLiteral)
			if !ok {
				t.Fatalf("expected *FloatLiteral, got %T", result)
			}
			if lit.Value != tt.expected {
				t.Errorf("got %f, want %f", lit.Value, tt.expected)
			}
		})
	}

	t.Run("float floor div", func(t *testing.T) {
		result := foldExpr(infix(OpFloorDiv, floatLit(7.5), floatLit(2.0)))
		lit, ok := result.(*FloatLiteral)
		if !ok {
			t.Fatalf("expected *FloatLiteral, got %T", result)
		}
		if lit.Value != 3.0 {
			t.Errorf("got %f, want 3.0", lit.Value)
		}
	})

	t.Run("float div by zero not folded", func(t *testing.T) {
		expr := infix(OpDiv, floatLit(10.0), floatLit(0.0))
		result := foldExpr(expr)
		_, ok := result.(*InfixExpression)
		if !ok {
			t.Fatalf("expected *InfixExpression (not folded), got %T", result)
		}
	})

	t.Run("float pow", func(t *testing.T) {
		result := foldExpr(infix(OpPow, floatLit(2.0), floatLit(3.0)))
		lit, ok := result.(*FloatLiteral)
		if !ok {
			t.Fatalf("expected *FloatLiteral, got %T", result)
		}
		if lit.Value != 8.0 {
			t.Errorf("got %f, want 8.0", lit.Value)
		}
	})

	t.Run("float pow negative base", func(t *testing.T) {
		result := foldExpr(infix(OpPow, floatLit(-2.0), floatLit(3.0)))
		lit, ok := result.(*FloatLiteral)
		if !ok {
			t.Fatalf("expected *FloatLiteral, got %T", result)
		}
		if lit.Value != -8.0 {
			t.Errorf("got %f, want -8.0", lit.Value)
		}
	})
}

func TestFoldStringConcat(t *testing.T) {
	t.Run("simple concat", func(t *testing.T) {
		result := foldExpr(infix(OpAdd, strLit("hello "), strLit("world")))
		lit, ok := result.(*StringLiteral)
		if !ok {
			t.Fatalf("expected *StringLiteral, got %T", result)
		}
		if lit.Value != "hello world" {
			t.Errorf("got %q, want %q", lit.Value, "hello world")
		}
	})

	t.Run("nested concat", func(t *testing.T) {
		result := foldExpr(infix(OpAdd, infix(OpAdd, strLit("a"), strLit("b")), strLit("c")))
		lit, ok := result.(*StringLiteral)
		if !ok {
			t.Fatalf("expected *StringLiteral, got %T", result)
		}
		if lit.Value != "abc" {
			t.Errorf("got %q, want %q", lit.Value, "abc")
		}
	})

	t.Run("string + non-literal not folded", func(t *testing.T) {
		ident := &Identifier{}
		expr := infix(OpAdd, strLit("hello "), ident)
		result := foldExpr(expr)
		infixResult, ok := result.(*InfixExpression)
		if !ok {
			t.Fatalf("expected *InfixExpression, got %T", result)
		}
		if infixResult.Operator != OpAdd {
			t.Errorf("expected OpAdd, got %v", infixResult.Operator)
		}
	})
}

func TestFoldStringRepetition(t *testing.T) {
	t.Run("str * int", func(t *testing.T) {
		result := foldExpr(infix(OpMul, strLit("a"), intLit(3)))
		lit, ok := result.(*StringLiteral)
		if !ok {
			t.Fatalf("expected *StringLiteral, got %T", result)
		}
		if lit.Value != "aaa" {
			t.Errorf("got %q, want %q", lit.Value, "aaa")
		}
	})

	t.Run("int * str", func(t *testing.T) {
		result := foldExpr(infix(OpMul, intLit(3), strLit("ab")))
		lit, ok := result.(*StringLiteral)
		if !ok {
			t.Fatalf("expected *StringLiteral, got %T", result)
		}
		if lit.Value != "ababab" {
			t.Errorf("got %q, want %q", lit.Value, "ababab")
		}
	})

	t.Run("str * 0", func(t *testing.T) {
		result := foldExpr(infix(OpMul, strLit("a"), intLit(0)))
		lit, ok := result.(*StringLiteral)
		if !ok {
			t.Fatalf("expected *StringLiteral, got %T", result)
		}
		if lit.Value != "" {
			t.Errorf("got %q, want %q", lit.Value, "")
		}
	})

	t.Run("str * negative", func(t *testing.T) {
		result := foldExpr(infix(OpMul, strLit("a"), intLit(-3)))
		lit, ok := result.(*StringLiteral)
		if !ok {
			t.Fatalf("expected *StringLiteral, got %T", result)
		}
		if lit.Value != "" {
			t.Errorf("got %q, want empty string", lit.Value)
		}
	})

	t.Run("negative * str", func(t *testing.T) {
		result := foldExpr(infix(OpMul, intLit(-3), strLit("a")))
		lit, ok := result.(*StringLiteral)
		if !ok {
			t.Fatalf("expected *StringLiteral, got %T", result)
		}
		if lit.Value != "" {
			t.Errorf("got %q, want empty string", lit.Value)
		}
	})

	t.Run("str * 1", func(t *testing.T) {
		result := foldExpr(infix(OpMul, strLit("hello"), intLit(1)))
		lit, ok := result.(*StringLiteral)
		if !ok {
			t.Fatalf("expected *StringLiteral, got %T", result)
		}
		if lit.Value != "hello" {
			t.Errorf("got %q, want %q", lit.Value, "hello")
		}
	})

	t.Run("nested: (\"a\" * 2) * 3", func(t *testing.T) {
		inner := infix(OpMul, strLit("a"), intLit(2))
		result := foldExpr(infix(OpMul, inner, intLit(3)))
		lit, ok := result.(*StringLiteral)
		if !ok {
			t.Fatalf("expected *StringLiteral, got %T", result)
		}
		if lit.Value != "aaaaaa" {
			t.Errorf("got %q, want %q", lit.Value, "aaaaaa")
		}
	})

	t.Run("large repetition", func(t *testing.T) {
		result := foldExpr(infix(OpMul, strLit("x"), intLit(1000)))
		lit, ok := result.(*StringLiteral)
		if !ok {
			t.Fatalf("expected *StringLiteral, got %T", result)
		}
		expected := strings.Repeat("x", 1000)
		if lit.Value != expected {
			t.Errorf("got len %d, want len %d", len(lit.Value), len(expected))
		}
	})
}

func TestFoldBooleanLogic(t *testing.T) {
	t.Run("True and True", func(t *testing.T) {
		result := foldExpr(infix(OpAnd, boolLit(true), boolLit(true)))
		lit, ok := result.(*Boolean)
		if !ok {
			t.Fatalf("expected *Boolean, got %T", result)
		}
		if lit.Value != true {
			t.Errorf("got %v, want true", lit.Value)
		}
	})

	t.Run("True and False", func(t *testing.T) {
		result := foldExpr(infix(OpAnd, boolLit(true), boolLit(false)))
		lit, ok := result.(*Boolean)
		if !ok {
			t.Fatalf("expected *Boolean, got %T", result)
		}
		if lit.Value != false {
			t.Errorf("got %v, want false", lit.Value)
		}
	})

	t.Run("False and True", func(t *testing.T) {
		result := foldExpr(infix(OpAnd, boolLit(false), boolLit(true)))
		lit, ok := result.(*Boolean)
		if !ok {
			t.Fatalf("expected *Boolean, got %T", result)
		}
		if lit.Value != false {
			t.Errorf("got %v, want false", lit.Value)
		}
	})

	t.Run("False or True", func(t *testing.T) {
		result := foldExpr(infix(OpOr, boolLit(false), boolLit(true)))
		lit, ok := result.(*Boolean)
		if !ok {
			t.Fatalf("expected *Boolean, got %T", result)
		}
		if lit.Value != true {
			t.Errorf("got %v, want true", lit.Value)
		}
	})

	t.Run("True or False", func(t *testing.T) {
		result := foldExpr(infix(OpOr, boolLit(true), boolLit(false)))
		lit, ok := result.(*Boolean)
		if !ok {
			t.Fatalf("expected *Boolean, got %T", result)
		}
		if lit.Value != true {
			t.Errorf("got %v, want true", lit.Value)
		}
	})

	t.Run("False or False", func(t *testing.T) {
		result := foldExpr(infix(OpOr, boolLit(false), boolLit(false)))
		lit, ok := result.(*Boolean)
		if !ok {
			t.Fatalf("expected *Boolean, got %T", result)
		}
		if lit.Value != false {
			t.Errorf("got %v, want false", lit.Value)
		}
	})

	t.Run("True and non-bool returns non-bool", func(t *testing.T) {
		ident := &Identifier{}
		result := foldExpr(infix(OpAnd, boolLit(true), ident))
		_, ok := result.(*Identifier)
		if !ok {
			t.Fatalf("expected *Identifier (True and X -> X), got %T", result)
		}
	})

	t.Run("False and non-bool returns False", func(t *testing.T) {
		ident := &Identifier{}
		result := foldExpr(infix(OpAnd, boolLit(false), ident))
		lit, ok := result.(*Boolean)
		if !ok {
			t.Fatalf("expected *Boolean, got %T", result)
		}
		if lit.Value != false {
			t.Errorf("got %v, want false", lit.Value)
		}
	})

	t.Run("False or non-bool returns non-bool", func(t *testing.T) {
		ident := &Identifier{}
		result := foldExpr(infix(OpOr, boolLit(false), ident))
		_, ok := result.(*Identifier)
		if !ok {
			t.Fatalf("expected *Identifier (False or X -> X), got %T", result)
		}
	})

	t.Run("True or non-bool returns True", func(t *testing.T) {
		ident := &Identifier{}
		result := foldExpr(infix(OpOr, boolLit(true), ident))
		lit, ok := result.(*Boolean)
		if !ok {
			t.Fatalf("expected *Boolean, got %T", result)
		}
		if lit.Value != true {
			t.Errorf("got %v, want true", lit.Value)
		}
	})

	t.Run("non-bool and False", func(t *testing.T) {
		ident := &Identifier{}
		result := foldExpr(infix(OpAnd, ident, boolLit(false)))
		lit, ok := result.(*Boolean)
		if !ok {
			t.Fatalf("expected *Boolean, got %T", result)
		}
		if lit.Value != false {
			t.Errorf("got %v, want false", lit.Value)
		}
	})

	t.Run("non-bool or True", func(t *testing.T) {
		ident := &Identifier{}
		result := foldExpr(infix(OpOr, ident, boolLit(true)))
		lit, ok := result.(*Boolean)
		if !ok {
			t.Fatalf("expected *Boolean, got %T", result)
		}
		if lit.Value != true {
			t.Errorf("got %v, want true", lit.Value)
		}
	})
}

func TestFoldBitwise(t *testing.T) {
	tests := []struct {
		name     string
		input    Expression
		expected int64
	}{
		{"and", infix(OpBitAnd, intLit(0xFF), intLit(0x0F)), 0x0F},
		{"or", infix(OpBitOr, intLit(0xF0), intLit(0x0F)), 0xFF},
		{"xor", infix(OpBitXor, intLit(0xFF), intLit(0x0F)), 0xF0},
		{"lshift", infix(OpLShift, intLit(1), intLit(8)), 256},
		{"rshift", infix(OpRShift, intLit(256), intLit(8)), 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := foldExpr(tt.input)
			lit, ok := result.(*IntegerLiteral)
			if !ok {
				t.Fatalf("expected *IntegerLiteral, got %T", result)
			}
			if lit.Value != tt.expected {
				t.Errorf("got %d, want %d", lit.Value, tt.expected)
			}
		})
	}

	t.Run("lshift too large not folded", func(t *testing.T) {
		expr := infix(OpLShift, intLit(1), intLit(64))
		result := foldExpr(expr)
		_, ok := result.(*InfixExpression)
		if !ok {
			t.Fatalf("expected *InfixExpression (not folded), got %T", result)
		}
	})

	t.Run("rshift negative not folded", func(t *testing.T) {
		expr := infix(OpRShift, intLit(1), intLit(-1))
		result := foldExpr(expr)
		_, ok := result.(*InfixExpression)
		if !ok {
			t.Fatalf("expected *InfixExpression (not folded), got %T", result)
		}
	})
}

func TestFoldComparisons(t *testing.T) {
	tests := []struct {
		name     string
		input    Expression
		expected bool
	}{
		{"3 < 5", infix(OpLt, intLit(3), intLit(5)), true},
		{"5 < 3", infix(OpLt, intLit(5), intLit(3)), false},
		{"5 > 3", infix(OpGt, intLit(5), intLit(3)), true},
		{"3 > 5", infix(OpGt, intLit(3), intLit(5)), false},
		{"3 <= 3", infix(OpLte, intLit(3), intLit(3)), true},
		{"3 >= 3", infix(OpGte, intLit(3), intLit(3)), true},
		{"3 == 3", infix(OpEq, intLit(3), intLit(3)), true},
		{"3 != 4", infix(OpNeq, intLit(3), intLit(4)), true},
		{"3 == 4", infix(OpEq, intLit(3), intLit(4)), false},
		{"float lt", infix(OpLt, floatLit(3.0), floatLit(3.5)), true},
		{"mixed eq", infix(OpEq, intLit(3), floatLit(3.0)), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := foldExpr(tt.input)
			lit, ok := result.(*Boolean)
			if !ok {
				t.Fatalf("expected *Boolean, got %T", result)
			}
			if lit.Value != tt.expected {
				t.Errorf("got %v, want %v", lit.Value, tt.expected)
			}
		})
	}
}

func TestFoldPrefix(t *testing.T) {
	t.Run("negate int", func(t *testing.T) {
		result := foldExpr(prefix(OpSub, intLit(5)))
		lit, ok := result.(*IntegerLiteral)
		if !ok {
			t.Fatalf("expected *IntegerLiteral, got %T", result)
		}
		if lit.Value != -5 {
			t.Errorf("got %d, want -5", lit.Value)
		}
	})

	t.Run("negate float", func(t *testing.T) {
		result := foldExpr(prefix(OpSub, floatLit(3.14)))
		lit, ok := result.(*FloatLiteral)
		if !ok {
			t.Fatalf("expected *FloatLiteral, got %T", result)
		}
		if lit.Value != -3.14 {
			t.Errorf("got %f, want -3.14", lit.Value)
		}
	})

	t.Run("double negate", func(t *testing.T) {
		result := foldExpr(prefix(OpSub, prefix(OpSub, intLit(5))))
		lit, ok := result.(*IntegerLiteral)
		if !ok {
			t.Fatalf("expected *IntegerLiteral, got %T", result)
		}
		if lit.Value != 5 {
			t.Errorf("got %d, want 5", lit.Value)
		}
	})

	t.Run("not true", func(t *testing.T) {
		result := foldExpr(prefix(OpNot, boolLit(true)))
		lit, ok := result.(*Boolean)
		if !ok {
			t.Fatalf("expected *Boolean, got %T", result)
		}
		if lit.Value != false {
			t.Errorf("got %v, want false", lit.Value)
		}
	})

	t.Run("not false", func(t *testing.T) {
		result := foldExpr(prefix(OpNot, boolLit(false)))
		lit, ok := result.(*Boolean)
		if !ok {
			t.Fatalf("expected *Boolean, got %T", result)
		}
		if lit.Value != true {
			t.Errorf("got %v, want true", lit.Value)
		}
	})

	t.Run("not 0", func(t *testing.T) {
		result := foldExpr(prefix(OpNot, intLit(0)))
		lit, ok := result.(*Boolean)
		if !ok {
			t.Fatalf("expected *Boolean, got %T", result)
		}
		if lit.Value != true {
			t.Errorf("got %v, want true", lit.Value)
		}
	})

	t.Run("not 1", func(t *testing.T) {
		result := foldExpr(prefix(OpNot, intLit(1)))
		lit, ok := result.(*Boolean)
		if !ok {
			t.Fatalf("expected *Boolean, got %T", result)
		}
		if lit.Value != false {
			t.Errorf("got %v, want false", lit.Value)
		}
	})

	t.Run("not 0.0", func(t *testing.T) {
		result := foldExpr(prefix(OpNot, floatLit(0.0)))
		lit, ok := result.(*Boolean)
		if !ok {
			t.Fatalf("expected *Boolean, got %T", result)
		}
		if lit.Value != true {
			t.Errorf("got %v, want true", lit.Value)
		}
	})

	t.Run("bitnot", func(t *testing.T) {
		result := foldExpr(prefix(OpBitNot, intLit(0)))
		lit, ok := result.(*IntegerLiteral)
		if !ok {
			t.Fatalf("expected *IntegerLiteral, got %T", result)
		}
		if lit.Value != -1 {
			t.Errorf("got %d, want -1", lit.Value)
		}
	})

	t.Run("not identifier not folded", func(t *testing.T) {
		ident := &Identifier{}
		result := foldExpr(prefix(OpNot, ident))
		prefixResult, ok := result.(*PrefixExpression)
		if !ok {
			t.Fatalf("expected *PrefixExpression, got %T", result)
		}
		if prefixResult.Operator != OpNot {
			t.Errorf("expected OpNot, got %v", prefixResult.Operator)
		}
	})
}

func TestFoldNonConstant(t *testing.T) {
	t.Run("identifier + identifier", func(t *testing.T) {
		ident := &Identifier{}
		expr := infix(OpAdd, ident, ident)
		result := foldExpr(expr)
		infixResult, ok := result.(*InfixExpression)
		if !ok {
			t.Fatalf("expected *InfixExpression (not folded), got %T", result)
		}
		if infixResult.Operator != OpAdd {
			t.Errorf("expected OpAdd, got %v", infixResult.Operator)
		}
	})

	t.Run("identifier + literal", func(t *testing.T) {
		ident := &Identifier{}
		expr := infix(OpAdd, ident, intLit(5))
		result := foldExpr(expr)
		infixResult, ok := result.(*InfixExpression)
		if !ok {
			t.Fatalf("expected *InfixExpression (not folded), got %T", result)
		}
		if infixResult.Operator != OpAdd {
			t.Errorf("expected OpAdd, got %v", infixResult.Operator)
		}
	})

	t.Run("nested: (1+2) + identifier", func(t *testing.T) {
		ident := &Identifier{}
		expr := infix(OpAdd, infix(OpAdd, intLit(1), intLit(2)), ident)
		result := foldExpr(expr)
		infixResult, ok := result.(*InfixExpression)
		if !ok {
			t.Fatalf("expected *InfixExpression (not folded), got %T", result)
		}
		left, ok := infixResult.Left.(*IntegerLiteral)
		if !ok {
			t.Fatalf("expected left to be *IntegerLiteral, got %T", infixResult.Left)
		}
		if left.Value != 3 {
			t.Errorf("left got %d, want 3", left.Value)
		}
	})
}

func TestFoldNilExpression(t *testing.T) {
	result := foldExpr(nil)
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestFoldNilStatement(t *testing.T) {
	result := foldStatement(nil)
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestFoldEmptyBlock(t *testing.T) {
	foldBlock(nil)
}

func TestFoldEmptyFunctionLiteral(t *testing.T) {
	foldFunctionLiteral(nil)
}

func TestFoldEmptyLambda(t *testing.T) {
	foldLambdaExpr(nil)
}

func TestFoldListLiteral(t *testing.T) {
	list := &ListLiteral{
		Elements: []Expression{
			infix(OpAdd, intLit(1), intLit(2)),
			infix(OpMul, intLit(3), intLit(4)),
			&Identifier{},
		},
	}
	result := foldExpr(list)
	resultList, ok := result.(*ListLiteral)
	if !ok {
		t.Fatalf("expected *ListLiteral, got %T", result)
	}
	first, ok := resultList.Elements[0].(*IntegerLiteral)
	if !ok || first.Value != 3 {
		t.Errorf("first element: got %v, want 3", resultList.Elements[0])
	}
	second, ok := resultList.Elements[1].(*IntegerLiteral)
	if !ok || second.Value != 12 {
		t.Errorf("second element: got %v, want 12", resultList.Elements[1])
	}
	if _, ok := resultList.Elements[2].(*Identifier); !ok {
		t.Errorf("third element: got %T, want *Identifier", resultList.Elements[2])
	}
}

func TestFoldDictLiteral(t *testing.T) {
	dict := &DictLiteral{
		Pairs: []DictPairLiteral{
			{Key: strLit("a"), Value: infix(OpAdd, intLit(1), intLit(2))},
		},
	}
	result := foldExpr(dict)
	resultDict, ok := result.(*DictLiteral)
	if !ok {
		t.Fatalf("expected *DictLiteral, got %T", result)
	}
	val, ok := resultDict.Pairs[0].Value.(*IntegerLiteral)
	if !ok || val.Value != 3 {
		t.Errorf("value: got %v, want 3", resultDict.Pairs[0].Value)
	}
}

func TestFoldSetLiteral(t *testing.T) {
	set := &SetLiteral{
		Elements: []Expression{
			infix(OpAdd, intLit(1), intLit(2)),
		},
	}
	result := foldExpr(set)
	resultSet, ok := result.(*SetLiteral)
	if !ok {
		t.Fatalf("expected *SetLiteral, got %T", result)
	}
	elem, ok := resultSet.Elements[0].(*IntegerLiteral)
	if !ok || elem.Value != 3 {
		t.Errorf("element: got %v, want 3", resultSet.Elements[0])
	}
}

func TestFoldTupleLiteral(t *testing.T) {
	tuple := &TupleLiteral{
		Elements: []Expression{
			infix(OpAdd, intLit(1), intLit(2)),
		},
	}
	result := foldExpr(tuple)
	resultTuple, ok := result.(*TupleLiteral)
	if !ok {
		t.Fatalf("expected *TupleLiteral, got %T", result)
	}
	elem, ok := resultTuple.Elements[0].(*IntegerLiteral)
	if !ok || elem.Value != 3 {
		t.Errorf("element: got %v, want 3", resultTuple.Elements[0])
	}
}

func TestFoldSafeIPow(t *testing.T) {
	t.Run("2^10 = 1024", func(t *testing.T) {
		result := safeIPow(2, 10)
		lit, ok := result.(*IntegerLiteral)
		if !ok {
			t.Fatalf("expected *IntegerLiteral, got %T", result)
		}
		if lit.Value != 1024 {
			t.Errorf("got %d, want 1024", lit.Value)
		}
	})

	t.Run("0^0 = 1", func(t *testing.T) {
		result := safeIPow(0, 0)
		lit, ok := result.(*IntegerLiteral)
		if !ok {
			t.Fatalf("expected *IntegerLiteral, got %T", result)
		}
		if lit.Value != 1 {
			t.Errorf("got %d, want 1", lit.Value)
		}
	})

	t.Run("overflow returns nil", func(t *testing.T) {
		result := safeIPow(2, 63)
		if result != nil {
			t.Errorf("expected nil for overflow, got %v", result)
		}
	})

	t.Run("exp > 62 returns nil", func(t *testing.T) {
		result := safeIPow(2, 100)
		if result != nil {
			t.Errorf("expected nil for large exp, got %v", result)
		}
	})

	t.Run("0^negative returns nil", func(t *testing.T) {
		result := safeIPow(0, -1)
		if result != nil {
			t.Errorf("expected nil, got %v", result)
		}
	})

	t.Run("2^-1 = 0.5", func(t *testing.T) {
		result := safeIPow(2, -1)
		lit, ok := result.(*FloatLiteral)
		if !ok {
			t.Fatalf("expected *FloatLiteral, got %T", result)
		}
		if lit.Value != 0.5 {
			t.Errorf("got %f, want 0.5", lit.Value)
		}
	})

	t.Run("base 1 always 1", func(t *testing.T) {
		result := safeIPow(1, 1000)
		lit, ok := result.(*IntegerLiteral)
		if !ok {
			t.Fatalf("expected *IntegerLiteral, got %T", result)
		}
		if lit.Value != 1 {
			t.Errorf("got %d, want 1", lit.Value)
		}
	})

	t.Run("base -1 even exp", func(t *testing.T) {
		result := safeIPow(-1, 4)
		lit, ok := result.(*IntegerLiteral)
		if !ok {
			t.Fatalf("expected *IntegerLiteral, got %T", result)
		}
		if lit.Value != 1 {
			t.Errorf("got %d, want 1", lit.Value)
		}
	})

	t.Run("base -1 odd exp", func(t *testing.T) {
		result := safeIPow(-1, 3)
		lit, ok := result.(*IntegerLiteral)
		if !ok {
			t.Fatalf("expected *IntegerLiteral, got %T", result)
		}
		if lit.Value != -1 {
			t.Errorf("got %d, want -1", lit.Value)
		}
	})

	t.Run("0^positive = 0", func(t *testing.T) {
		result := safeIPow(0, 5)
		lit, ok := result.(*IntegerLiteral)
		if !ok {
			t.Fatalf("expected *IntegerLiteral, got %T", result)
		}
		if lit.Value != 0 {
			t.Errorf("got %d, want 0", lit.Value)
		}
	})

	t.Run("any^0 = 1", func(t *testing.T) {
		result := safeIPow(42, 0)
		lit, ok := result.(*IntegerLiteral)
		if !ok {
			t.Fatalf("expected *IntegerLiteral, got %T", result)
		}
		if lit.Value != 1 {
			t.Errorf("got %d, want 1", lit.Value)
		}
	})

	t.Run("negative exp overflow to float returns nil", func(t *testing.T) {
		result := safeIPow(999999999, -1)
		if result != nil {
			flit, ok := result.(*FloatLiteral)
			if ok {
				if math.IsInf(flit.Value, 0) || math.IsNaN(flit.Value) {
					t.Errorf("should return nil for overflow, got %f", flit.Value)
				}
			}
		}
	})
}

func TestFoldFloorDivInt(t *testing.T) {
	tests := []struct {
		name     string
		a, b     int64
		expected int64
	}{
		{"10 / 3", 10, 3, 3},
		{"-7 / 2", -7, 2, -3},
		{"7 / -2", 7, -2, -3},
		{"-7 / -2", -7, -2, 3},
		{"0 / 5", 0, 5, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := floorDivInt(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("got %d, want %d", result, tt.expected)
			}
		})
	}
}

func TestFoldModInt(t *testing.T) {
	tests := []struct {
		name     string
		a, b     int64
		expected int64
	}{
		{"10 % 3", 10, 3, 1},
		{"-7 % 2", -7, 2, -1},
		{"7 % -2", 7, -2, 1},
		{"-7 % -2", -7, -2, -1},
		{"0 % 5", 0, 5, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := modInt(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("got %d, want %d", result, tt.expected)
			}
		})
	}
}

func TestFoldFloatEdgeCases(t *testing.T) {
	t.Run("float mod int", func(t *testing.T) {
		ident := &Identifier{}
		result := tryFoldInfix(OpMod, floatLit(7.0), ident)
		if result != nil {
			t.Errorf("expected nil for float mod identifier, got %T", result)
		}
	})

	t.Run("float pow inf not folded", func(t *testing.T) {
		result := tryFoldInfix(OpPow, floatLit(1e300), floatLit(1e300))
		if result != nil {
			t.Errorf("expected nil for inf pow, got %T", result)
		}
	})

	t.Run("float pow negative base fractional exp", func(t *testing.T) {
		result := tryFoldInfix(OpPow, floatLit(-2.0), floatLit(0.5))
		if result != nil {
			t.Errorf("expected nil for negative base fractional exp, got %T", result)
		}
	})

	t.Run("float floor div by zero not folded", func(t *testing.T) {
		result := tryFoldInfix(OpFloorDiv, floatLit(10.0), floatLit(0.0))
		if result != nil {
			t.Errorf("expected nil for float floor div by zero, got %T", result)
		}
	})

	t.Run("string * string not folded", func(t *testing.T) {
		result := tryFoldInfix(OpMul, strLit("a"), strLit("b"))
		if result != nil {
			t.Errorf("expected nil for string * string, got %T", result)
		}
	})

	t.Run("unhandled operator on ints", func(t *testing.T) {
		result := tryFoldInfix(OpIn, intLit(1), intLit(2))
		if result != nil {
			t.Errorf("expected nil for OpIn on ints, got %T", result)
		}
	})

	t.Run("unhandled operator on floats", func(t *testing.T) {
		result := tryFoldInfix(OpIn, floatLit(1.0), floatLit(2.0))
		if result != nil {
			t.Errorf("expected nil for OpIn on floats, got %T", result)
		}
	})

	t.Run("non-numeric operands", func(t *testing.T) {
		result := tryFoldInfix(OpAdd, &None{}, &None{})
		if result != nil {
			t.Errorf("expected nil for None + None, got %T", result)
		}
	})
}

func TestFoldAssignStatement(t *testing.T) {
	prog := &Program{
		Statements: []Statement{
			&AssignStatement{
				Left:  &Identifier{},
				Value: infix(OpAdd, intLit(1), intLit(2)),
			},
		},
	}
	FoldConstants(prog)
	assign := prog.Statements[0].(*AssignStatement)
	lit, ok := assign.Value.(*IntegerLiteral)
	if !ok || lit.Value != 3 {
		t.Errorf("expected folded value 3, got %v", assign.Value)
	}
}

func TestFoldReturnStatement(t *testing.T) {
	block := &BlockStatement{
		Statements: []Statement{
			&ReturnStatement{
				ReturnValue: infix(OpAdd, intLit(1), intLit(2)),
			},
		},
	}
	foldBlock(block)
	ret := block.Statements[0].(*ReturnStatement)
	lit, ok := ret.ReturnValue.(*IntegerLiteral)
	if !ok || lit.Value != 3 {
		t.Errorf("expected folded value 3, got %v", ret.ReturnValue)
	}
}

func TestFoldIfStatement(t *testing.T) {
	prog := &Program{
		Statements: []Statement{
			&IfStatement{
				Condition: infix(OpLt, intLit(1), intLit(2)),
				Consequence: &BlockStatement{
					Statements: []Statement{
						&ExpressionStatement{Expression: infix(OpAdd, intLit(1), intLit(2))},
					},
				},
			},
		},
	}
	FoldConstants(prog)
	ifStmt := prog.Statements[0].(*IfStatement)
	cond, ok := ifStmt.Condition.(*Boolean)
	if !ok || cond.Value != true {
		t.Errorf("expected folded condition true, got %v", ifStmt.Condition)
	}
	expr := ifStmt.Consequence.Statements[0].(*ExpressionStatement).Expression
	lit, ok := expr.(*IntegerLiteral)
	if !ok || lit.Value != 3 {
		t.Errorf("expected folded body 3, got %v", expr)
	}
}

func TestFoldAugmentedAssignStatement(t *testing.T) {
	prog := &Program{
		Statements: []Statement{
			&AugmentedAssignStatement{
				Value: infix(OpAdd, intLit(1), intLit(2)),
			},
		},
	}
	FoldConstants(prog)
	aug := prog.Statements[0].(*AugmentedAssignStatement)
	lit, ok := aug.Value.(*IntegerLiteral)
	if !ok || lit.Value != 3 {
		t.Errorf("expected folded value 3, got %v", aug.Value)
	}
}

func TestFoldMultipleAssignStatement(t *testing.T) {
	prog := &Program{
		Statements: []Statement{
			&MultipleAssignStatement{
				Value: infix(OpAdd, intLit(1), intLit(2)),
			},
		},
	}
	FoldConstants(prog)
	ma := prog.Statements[0].(*MultipleAssignStatement)
	lit, ok := ma.Value.(*IntegerLiteral)
	if !ok || lit.Value != 3 {
		t.Errorf("expected folded value 3, got %v", ma.Value)
	}
}

func BenchmarkFoldConstants(b *testing.B) {
	buildProgram := func() *Program {
		return &Program{
			Statements: []Statement{
				&ExpressionStatement{Expression: infix(OpAdd, infix(OpMul, intLit(2), intLit(3)), intLit(4))},
				&ExpressionStatement{Expression: infix(OpAdd, strLit("hello "), strLit("world"))},
				&ExpressionStatement{Expression: infix(OpMul, strLit("a"), intLit(100))},
				&ExpressionStatement{Expression: infix(OpPow, intLit(2), intLit(10))},
				&ExpressionStatement{Expression: prefix(OpSub, intLit(42))},
				&IfStatement{
					Condition:   infix(OpLt, intLit(1), intLit(2)),
					Consequence: &BlockStatement{Statements: []Statement{&PassStatement{}}},
				},
			},
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		prog := buildProgram()
		FoldConstants(prog)
	}
}
