package evaluator

import (
	"github.com/paularlott/scriptling/ast"
	"github.com/paularlott/scriptling/object"
)

// Unboxed integer arithmetic.
//
// Integer results outside the small-int cache are heap-allocated
// *object.Integer values, so an expression like `total + i * 2 - 1` boxes one
// Integer per operator even though only the final value is ever observed.
// Profiling arithmetic-heavy loops showed that ~84% of all allocations were
// exactly these dead intermediates.
//
// evalIntOperand walks a subtree that is provably side-effect free and
// integer-valued and returns a raw int64, so intermediates are never boxed;
// only the caller's final result is.
//
// SAFETY: the walk only ever descends through integer literals, plain
// identifier reads, and integer-valued operators over those. None of those can
// have side effects or call back into script code, so abandoning the walk
// part-way and letting the ordinary evaluation path redo the work is always
// safe. Anything else (calls, indexing, attribute access, floats, strings,
// bools) returns ok=false and the caller falls back to the general path.
//
// Whether a subtree is even eligible is a static property of the AST, recorded
// once by the parser as InfixExpression.IntFast, so nodes that can never use
// this path (`self.x * other.x`, string concatenation, float maths) are rejected
// by a single byte comparison rather than by re-walking the tree on every
// evaluation. That matters: most infix nodes in object-oriented or string-heavy
// code are rejections, and they must not pay for this optimisation.

// evalIntOperand evaluates node to an unboxed int64, reporting ok=false if the
// subtree is not a side-effect-free integer expression.
func evalIntOperand(node ast.Expression, env *object.Environment) (int64, bool) {
	switch n := node.(type) {
	case *ast.IntegerLiteral:
		return n.Value, true
	case *ast.Identifier:
		// Go through evalIdentifier rather than env.Get so this path benefits
		// from the per-node slot cache; env.Get would fall back to a map lookup
		// per scope level and end up slower than the general path.
		i, ok := evalIdentifier(n, env).(*object.Integer)
		if !ok {
			return 0, false
		}
		return i.IntValue(), true
	case *ast.InfixExpression:
		if n.IntFast != ast.IntFastArith {
			return 0, false
		}
		l, ok := evalIntOperand(n.Left, env)
		if !ok {
			return 0, false
		}
		r, ok := evalIntOperand(n.Right, env)
		if !ok {
			return 0, false
		}
		return applyIntFastOp(n.Operator, l, r)
	}
	return 0, false
}

// applyIntFastOp mirrors the integer cases of evalIntegerInfixExpression
// exactly, including its Go-style truncating // and %. Cases that must raise an
// error (division by zero, negative shift counts) return ok=false so the normal
// path produces the error object.
func applyIntFastOp(op ast.Op, l, r int64) (int64, bool) {
	switch op {
	case ast.OpAdd:
		return l + r, true
	case ast.OpSub:
		return l - r, true
	case ast.OpMul:
		return l * r, true
	case ast.OpFloorDiv:
		if r == 0 {
			return 0, false
		}
		return l / r, true
	case ast.OpMod:
		if r == 0 {
			return 0, false
		}
		return l % r, true
	case ast.OpBitAnd:
		return l & r, true
	case ast.OpBitOr:
		return l | r, true
	case ast.OpBitXor:
		return l ^ r, true
	case ast.OpLShift:
		if r < 0 {
			return 0, false
		}
		return l << uint64(r), true
	case ast.OpRShift:
		if r < 0 {
			return 0, false
		}
		return l >> uint64(r), true
	}
	return 0, false
}

// tryEvalIntInfix handles an infix node whose operands are side-effect-free
// integer expressions, boxing at most one result (and none at all for
// comparisons). Returns ok=false when a variable turns out to hold something
// other than an integer, in which case the caller uses the general path.
//
// Callers must check node.IntFast != ast.IntFastNone first so that ineligible
// nodes never reach this function.
func tryEvalIntInfix(node *ast.InfixExpression, env *object.Environment) (object.Object, bool) {
	if node.IntFast == ast.IntFastArith {
		if v, ok := evalIntOperand(node, env); ok {
			return object.NewInteger(v), true
		}
		return nil, false
	}
	// Integer comparison: the result is a boolean singleton, so nothing is boxed.
	l, ok := evalIntOperand(node.Left, env)
	if !ok {
		return nil, false
	}
	r, ok := evalIntOperand(node.Right, env)
	if !ok {
		return nil, false
	}
	switch node.Operator {
	case ast.OpLt:
		return nativeBoolToBooleanObject(l < r), true
	case ast.OpGt:
		return nativeBoolToBooleanObject(l > r), true
	case ast.OpLte:
		return nativeBoolToBooleanObject(l <= r), true
	case ast.OpGte:
		return nativeBoolToBooleanObject(l >= r), true
	case ast.OpEq:
		return nativeBoolToBooleanObject(l == r), true
	case ast.OpNeq:
		return nativeBoolToBooleanObject(l != r), true
	}
	return nil, false
}
