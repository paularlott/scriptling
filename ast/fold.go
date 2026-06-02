package ast

import (
	"math"
	"strings"
)

// FoldConstants replaces constant sub-expressions with their evaluated literal values.
// It walks the entire AST including nested function bodies, class bodies, and all blocks.
func FoldConstants(program *Program) {
	if program == nil {
		return
	}
	for i, stmt := range program.Statements {
		program.Statements[i] = foldStatement(stmt)
	}
}

func foldStatement(stmt Statement) Statement {
	if stmt == nil {
		return nil
	}
	switch s := stmt.(type) {
	case *AssignStatement:
		s.Value = foldExpression(s.Value)
		if s.Chained != nil {
			s.Chained = foldStatement(s.Chained).(*AssignStatement)
		}
	case *AugmentedAssignStatement:
		s.Value = foldExpression(s.Value)
	case *MultipleAssignStatement:
		s.Value = foldExpression(s.Value)
	case *ExpressionStatement:
		s.Expression = foldExpression(s.Expression)
	case *ReturnStatement:
		s.ReturnValue = foldExpression(s.ReturnValue)
	case *IfStatement:
		s.Condition = foldExpression(s.Condition)
		foldBlock(s.Consequence)
		for _, clause := range s.ElifClauses {
			clause.Condition = foldExpression(clause.Condition)
			foldBlock(clause.Consequence)
		}
		foldBlock(s.Alternative)
	case *WhileStatement:
		s.Condition = foldExpression(s.Condition)
		foldBlock(s.Body)
		foldBlock(s.Else)
	case *ForStatement:
		s.Iterable = foldExpression(s.Iterable)
		foldBlock(s.Body)
		foldBlock(s.Else)
	case *FunctionStatement:
		foldFunctionLiteral(s.Function)
		if s.overflow != nil {
			for i, d := range s.overflow.Decorators {
				s.overflow.Decorators[i] = foldExpression(d)
			}
		}
	case *ClassStatement:
		if s.BaseClass != nil {
			s.BaseClass = foldExpression(s.BaseClass)
		}
		foldBlock(s.Body)
		if s.overflow != nil {
			for i, d := range s.overflow.Decorators {
				s.overflow.Decorators[i] = foldExpression(d)
			}
		}
	case *TryStatement:
		foldBlock(s.Body)
		for _, clause := range s.ExceptClauses {
			if clause.ExceptType != nil {
				clause.ExceptType = foldExpression(clause.ExceptType)
			}
			foldBlock(clause.Body)
		}
		foldBlock(s.Else)
		foldBlock(s.Finally)
	case *RaiseStatement:
		if s.Message != nil {
			s.Message = foldExpression(s.Message)
		}
	case *AssertStatement:
		s.Condition = foldExpression(s.Condition)
		if s.Message != nil {
			s.Message = foldExpression(s.Message)
		}
	case *WithStatement:
		s.ContextExpr = foldExpression(s.ContextExpr)
		foldBlock(s.Body)
	case *MatchStatement:
		s.Subject = foldExpression(s.Subject)
		for _, cc := range s.Cases {
			cc.Pattern = foldExpression(cc.Pattern)
			if cc.Guard != nil {
				cc.Guard = foldExpression(cc.Guard)
			}
			foldBlock(cc.Body)
		}
	case *DelStatement:
		// Don't fold del targets (could be indexed)
	case *ImportStatement, *FromImportStatement:
		// No folding for imports
	case *GlobalStatement, *NonlocalStatement:
		// No expressions to fold
	case *BreakStatement, *ContinueStatement, *PassStatement:
		// No expressions to fold
	}
	return stmt
}

func foldBlock(block *BlockStatement) {
	if block == nil {
		return
	}
	for i, stmt := range block.Statements {
		block.Statements[i] = foldStatement(stmt)
	}
}

func foldFunctionLiteral(fn *FunctionLiteral) {
	if fn == nil {
		return
	}
	foldBlock(fn.Body)
	if fn.overflow != nil {
		for k, v := range fn.overflow.DefaultValues {
			fn.overflow.DefaultValues[k] = foldExpression(v)
		}
	}
}

func foldLambdaExpr(lam *Lambda) {
	if lam == nil {
		return
	}
	lam.Body = foldExpression(lam.Body)
	if lam.overflow != nil {
		for k, v := range lam.overflow.DefaultValues {
			lam.overflow.DefaultValues[k] = foldExpression(v)
		}
	}
}

func foldExpression(expr Expression) Expression {
	if expr == nil {
		return nil
	}

	switch e := expr.(type) {
	case *InfixExpression:
		left := foldExpression(e.Left)
		right := foldExpression(e.Right)
		e.Left = left
		e.Right = right
		if folded := tryFoldInfix(e.Operator, left, right); folded != nil {
			return folded
		}

	case *PrefixExpression:
		right := foldExpression(e.Right)
		e.Right = right
		if folded := tryFoldPrefix(e.Operator, right); folded != nil {
			return folded
		}

	case *ConditionalExpression:
		e.TrueExpr = foldExpression(e.TrueExpr)
		e.Condition = foldExpression(e.Condition)
		e.FalseExpr = foldExpression(e.FalseExpr)

	case *CallExpression:
		e.Function = foldExpression(e.Function)
		for i, arg := range e.Arguments {
			e.Arguments[i] = foldExpression(arg)
		}
		if e.overflow != nil {
			for k, v := range e.overflow.Keywords {
				e.overflow.Keywords[k] = foldExpression(v)
			}
			for i, a := range e.overflow.ArgsUnpack {
				e.overflow.ArgsUnpack[i] = foldExpression(a)
			}
			if e.overflow.KwargsUnpack != nil {
				e.overflow.KwargsUnpack = foldExpression(e.overflow.KwargsUnpack)
			}
		}

	case *MethodCallExpression:
		e.Receiver = foldExpression(e.Receiver)
		for i, arg := range e.Arguments {
			e.Arguments[i] = foldExpression(arg)
		}
		if e.overflow != nil {
			for k, v := range e.overflow.Keywords {
				e.overflow.Keywords[k] = foldExpression(v)
			}
			for i, a := range e.overflow.ArgsUnpack {
				e.overflow.ArgsUnpack[i] = foldExpression(a)
			}
			if e.overflow.KwargsUnpack != nil {
				e.overflow.KwargsUnpack = foldExpression(e.overflow.KwargsUnpack)
			}
		}

	case *IndexExpression:
		e.Left = foldExpression(e.Left)
		e.Index = foldExpression(e.Index)

	case *SliceExpression:
		e.Left = foldExpression(e.Left)
		if e.Start != nil {
			e.Start = foldExpression(e.Start)
		}
		if e.End != nil {
			e.End = foldExpression(e.End)
		}
		if e.overflow != nil && e.overflow.Step != nil {
			e.overflow.Step = foldExpression(e.overflow.Step)
		}

	case *ListLiteral:
		for i, elem := range e.Elements {
			e.Elements[i] = foldExpression(elem)
		}

	case *DictLiteral:
		for i, pair := range e.Pairs {
			e.Pairs[i].Key = foldExpression(pair.Key)
			e.Pairs[i].Value = foldExpression(pair.Value)
		}

	case *SetLiteral:
		for i, elem := range e.Elements {
			e.Elements[i] = foldExpression(elem)
		}

	case *TupleLiteral:
		for i, elem := range e.Elements {
			e.Elements[i] = foldExpression(elem)
		}

	case *ListComprehension:
		e.Expression = foldExpression(e.Expression)
		e.Iterable = foldExpression(e.Iterable)
		if e.Condition != nil {
			e.Condition = foldExpression(e.Condition)
		}
		for i, clause := range e.AdditionalClauses {
			e.AdditionalClauses[i].Iterable = foldExpression(clause.Iterable)
			if clause.Condition != nil {
				e.AdditionalClauses[i].Condition = foldExpression(clause.Condition)
			}
		}

	case *DictComprehension:
		e.Key = foldExpression(e.Key)
		e.Value = foldExpression(e.Value)
		e.Iterable = foldExpression(e.Iterable)
		if e.Condition != nil {
			e.Condition = foldExpression(e.Condition)
		}
		for i, clause := range e.AdditionalClauses {
			e.AdditionalClauses[i].Iterable = foldExpression(clause.Iterable)
			if clause.Condition != nil {
				e.AdditionalClauses[i].Condition = foldExpression(clause.Condition)
			}
		}

	case *SetComprehension:
		e.Expression = foldExpression(e.Expression)
		e.Iterable = foldExpression(e.Iterable)
		if e.Condition != nil {
			e.Condition = foldExpression(e.Condition)
		}
		for i, clause := range e.AdditionalClauses {
			e.AdditionalClauses[i].Iterable = foldExpression(clause.Iterable)
			if clause.Condition != nil {
				e.AdditionalClauses[i].Condition = foldExpression(clause.Condition)
			}
		}

	case *FStringLiteral:
		for i, part := range e.Expressions {
			e.Expressions[i] = foldExpression(part)
		}

	case *Lambda:
		foldLambdaExpr(e)

	case *OrPattern:
		for i, p := range e.Patterns {
			e.Patterns[i] = foldExpression(p)
		}

	// Literals and identifiers are already constants, nothing to fold
	case *IntegerLiteral, *FloatLiteral, *StringLiteral, *Boolean, *None, *Identifier:
		// pass through
	}

	return expr
}

func tryFoldInfix(op Op, left, right Expression) Expression {
	lint, lIsInt := left.(*IntegerLiteral)
	lfloat, lIsFloat := left.(*FloatLiteral)
	rint, rIsInt := right.(*IntegerLiteral)
	rfloat, rIsFloat := right.(*FloatLiteral)
	lstr, lIsStr := left.(*StringLiteral)
	rstr, rIsStr := right.(*StringLiteral)
	lbool, lIsBool := left.(*Boolean)
	rbool, rIsBool := right.(*Boolean)

	// Boolean logic (short-circuit)
	if op == OpAnd {
		if lIsBool {
			if !lbool.Value {
				return BoolFalse // False and X -> False
			}
			return right // True and X -> X (already folded)
		}
		if rIsBool && !rbool.Value {
			return BoolFalse // X and False -> False
		}
	}
	if op == OpOr {
		if lIsBool {
			if lbool.Value {
				return BoolTrue // True or X -> True
			}
			return right // False or X -> X (already folded)
		}
		if rIsBool && rbool.Value {
			return BoolTrue // X or True -> True
		}
	}

	// String concatenation
	if op == OpAdd && lIsStr && rIsStr {
		return &StringLiteral{Value: lstr.Value + rstr.Value}
	}

	// String repetition
	if op == OpMul {
		if lIsStr && rIsInt {
			if rint.Value < 0 {
				return &StringLiteral{Value: ""}
			}
			return &StringLiteral{Value: strings.Repeat(lstr.Value, int(rint.Value))}
		}
		if lIsInt && rIsStr {
			if lint.Value < 0 {
				return &StringLiteral{Value: ""}
			}
			return &StringLiteral{Value: strings.Repeat(rstr.Value, int(lint.Value))}
		}
	}

	// Integer-only bitwise and shift ops
	if lIsInt && rIsInt {
		switch op {
		case OpBitAnd:
			return &IntegerLiteral{Value: lint.Value & rint.Value}
		case OpBitOr:
			return &IntegerLiteral{Value: lint.Value | rint.Value}
		case OpBitXor:
			return &IntegerLiteral{Value: lint.Value ^ rint.Value}
		case OpLShift:
			if rint.Value >= 0 && rint.Value < 64 {
				return &IntegerLiteral{Value: lint.Value << rint.Value}
			}
		case OpRShift:
			if rint.Value >= 0 && rint.Value < 64 {
				return &IntegerLiteral{Value: lint.Value >> rint.Value}
			}
		}
	}

	// Integer arithmetic (both operands are integers)
	if lIsInt && rIsInt {
		switch op {
		case OpAdd:
			return &IntegerLiteral{Value: lint.Value + rint.Value}
		case OpSub:
			return &IntegerLiteral{Value: lint.Value - rint.Value}
		case OpMul:
			return &IntegerLiteral{Value: lint.Value * rint.Value}
		case OpFloorDiv:
			if rint.Value == 0 {
				return nil
			}
			return &IntegerLiteral{Value: floorDivInt(lint.Value, rint.Value)}
		case OpMod:
			if rint.Value == 0 {
				return nil
			}
			return &IntegerLiteral{Value: modInt(lint.Value, rint.Value)}
		case OpPow:
			if result := safeIPow(lint.Value, rint.Value); result != nil {
				return result
			}
			return nil // overflow or invalid, don't fall through to float
		case OpDiv:
			if rint.Value == 0 {
				return nil
			}
			return &FloatLiteral{Value: float64(lint.Value) / float64(rint.Value)}
		case OpLt:
			return &Boolean{Value: lint.Value < rint.Value}
		case OpGt:
			return &Boolean{Value: lint.Value > rint.Value}
		case OpLte:
			return &Boolean{Value: lint.Value <= rint.Value}
		case OpGte:
			return &Boolean{Value: lint.Value >= rint.Value}
		case OpEq:
			return &Boolean{Value: lint.Value == rint.Value}
		case OpNeq:
			return &Boolean{Value: lint.Value != rint.Value}
		}
		return nil // int operands but unhandled operator
	}

	// Float arithmetic (at least one float operand, neither is both-int)
	if (lIsInt || lIsFloat) && (rIsInt || rIsFloat) {
		var lv, rv float64
		if lIsInt {
			lv = float64(lint.Value)
		} else {
			lv = lfloat.Value
		}
		if rIsInt {
			rv = float64(rint.Value)
		} else {
			rv = rfloat.Value
		}

		switch op {
		case OpAdd:
			return &FloatLiteral{Value: lv + rv}
		case OpSub:
			return &FloatLiteral{Value: lv - rv}
		case OpMul:
			return &FloatLiteral{Value: lv * rv}
		case OpDiv:
			if rv == 0 {
				return nil
			}
			return &FloatLiteral{Value: lv / rv}
		case OpFloorDiv:
			if rv == 0 {
				return nil
			}
			return &FloatLiteral{Value: math.Floor(lv / rv)}
		case OpPow:
			if rv >= 0 && rv == math.Floor(rv) {
				v := math.Pow(lv, rv)
				if !math.IsInf(v, 0) && !math.IsNaN(v) {
					return &FloatLiteral{Value: v}
				}
			} else if lv > 0 {
				v := math.Pow(lv, rv)
				if !math.IsInf(v, 0) && !math.IsNaN(v) {
					return &FloatLiteral{Value: v}
				}
			}
		case OpLt:
			return &Boolean{Value: lv < rv}
		case OpGt:
			return &Boolean{Value: lv > rv}
		case OpLte:
			return &Boolean{Value: lv <= rv}
		case OpGte:
			return &Boolean{Value: lv >= rv}
		case OpEq:
			return &Boolean{Value: lv == rv}
		case OpNeq:
			return &Boolean{Value: lv != rv}
		}
	}

	return nil
}

func tryFoldPrefix(op Op, right Expression) Expression {
	rint, rIsInt := right.(*IntegerLiteral)
	rfloat, rIsFloat := right.(*FloatLiteral)
	rbool, rIsBool := right.(*Boolean)

	switch op {
	case OpNot:
		if rIsBool {
			return &Boolean{Value: !rbool.Value}
		}
		if rIsInt {
			return &Boolean{Value: rint.Value == 0}
		}
		if rIsFloat {
			return &Boolean{Value: rfloat.Value == 0}
		}

	case OpBitNot:
		if rIsInt {
			return &IntegerLiteral{Value: ^rint.Value}
		}

	case OpSub:
		if rIsInt {
			return &IntegerLiteral{Value: -rint.Value}
		}
		if rIsFloat {
			return &FloatLiteral{Value: -rfloat.Value}
		}

	case OpPos:
		// +x is already x, no folding needed
	}

	return nil
}

// floorDivInt computes integer floor division matching the evaluator's behavior.
func floorDivInt(a, b int64) int64 {
	return a / b
}

// modInt computes integer modulo matching the evaluator's behavior.
func modInt(a, b int64) int64 {
	return a % b
}

// safeIPow computes a**b for integers. Returns nil on overflow or invalid input.
func safeIPow(base int64, exp int64) Expression {
	if exp < 0 {
		// Negative exponent -> float result, but only if base != 0
		if base == 0 {
			return nil // ZeroDivisionError at runtime
		}
		v := math.Pow(float64(base), float64(exp))
		if !math.IsInf(v, 0) && !math.IsNaN(v) {
			return &FloatLiteral{Value: v}
		}
		return nil // overflow to float
	}

	if exp == 0 {
		return &IntegerLiteral{Value: 1} // 0**0 == 1 in Python
	}

	if base == 0 {
		return &IntegerLiteral{Value: 0}
	}

	if base == 1 {
		return &IntegerLiteral{Value: 1}
	}

	if base == -1 {
		if exp%2 == 0 {
			return &IntegerLiteral{Value: 1}
		}
		return &IntegerLiteral{Value: -1}
	}

	// Check for overflow with large exponents
	if exp > 62 {
		// Could overflow int64, fall through to runtime
		return nil
	}

	result := int64(1)
	for i := int64(0); i < exp; i++ {
		prev := result
		result *= base
		// Detect overflow: if result/base != prev, we overflowed
		if result/base != prev {
			return nil // overflow, evaluate at runtime
		}
	}
	return &IntegerLiteral{Value: result}
}

