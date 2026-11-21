package evaluator

import (
	"math"

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
// compileIntOperand turns a subtree that is provably side-effect free and
// integer-valued into a closure returning a raw int64, so intermediates are
// never boxed; only the caller's final result is. Compiling the subtree, rather
// than walking it at run time, takes the node-type and IntFast checks out of
// the hot path (measured 7-15% on arithmetic loops).
//
// SAFETY: a compiled subtree only ever reads integer literals and plain
// identifiers and applies integer-valued operators to them. None of those can
// have side effects or call back into script code, so abandoning the
// evaluation part-way and letting the ordinary path redo the work is always
// safe. A variable holding anything but an integer (floats, strings, bools,
// instances) returns ok=false and the caller falls back to the general path.
//
// Whether a subtree is eligible is a static property of the AST, recorded
// once by the parser as InfixExpression.IntFast, so nodes that can never use
// this path (`self.x * other.x`, string concatenation, float maths) get no
// fast-path closure at all and pay nothing for it.

// applyIntFastOp mirrors the integer cases of evalIntegerInfixExpression
// exactly, including its Python-style flooring // and %. Cases that must raise an
// error (division by zero, negative shift counts) return ok=false so the normal
// path produces the error object.
func applyIntFastOp(op ast.Op, l, r int64) (int64, bool) {
	switch op {
	case ast.OpAdd:
		if s, ok := addOv(l, r); ok {
			return s, true
		}
		return 0, false // overflow: the normal path raises OverflowError
	case ast.OpSub:
		if d, ok := subOv(l, r); ok {
			return d, true
		}
		return 0, false
	case ast.OpMul:
		if p, ok := mulOv(l, r); ok {
			return p, true
		}
		return 0, false
	case ast.OpFloorDiv:
		if r == 0 {
			return 0, false
		}
		if l == math.MinInt64 && r == -1 {
			return 0, false // overflow: the normal path raises
		}
		q := l / r
		if l%r != 0 && (l < 0) != (r < 0) {
			q--
		}
		return q, true
	case ast.OpMod:
		if r == 0 {
			return 0, false
		}
		m := l % r
		if m != 0 && (m < 0) != (r < 0) {
			m += r
		}
		return m, true
	case ast.OpBitAnd:
		return l & r, true
	case ast.OpBitOr:
		return l | r, true
	case ast.OpBitXor:
		return l ^ r, true
	case ast.OpLShift:
		if r < 0 || r >= 64 {
			return 0, false
		}
		if l > 0 && l > (math.MaxInt64>>uint(r)) {
			return 0, false
		}
		if l < 0 && l < (math.MinInt64>>uint(r)) {
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

// intEvalFn is a compiled side-effect-free integer subtree: it returns the raw
// int64, or ok=false when an operand is not an integer at run time or the
// operation must raise (overflow, division by zero), in which case the caller
// redoes the work on the general path. See the SAFETY note above.
type intEvalFn func(env *object.Environment) (int64, bool)

// compileIntOperand compiles an integer-shaped subtree (ast.IsIntShaped) into an
// intEvalFn. Returns nil for any other node.
func compileIntOperand(node ast.Expression) intEvalFn {
	switch n := node.(type) {
	case *ast.IntegerLiteral:
		v := n.Value
		return func(*object.Environment) (int64, bool) { return v, true }
	case *ast.Identifier:
		return func(env *object.Environment) (int64, bool) {
			i, ok := evalIdentifier(n, env).(*object.Integer)
			if !ok {
				return 0, false
			}
			return i.IntValue(), true
		}
	case *ast.InfixExpression:
		if n.IntFast != ast.IntFastArith {
			return nil
		}
		op := n.Operator
		// Identifier op literal (i + 1, i * 3, i % n literal) is the common
		// shape; read the identifier inline and keep the constant in the closure.
		if id, ok := n.Left.(*ast.Identifier); ok {
			if lit, ok := n.Right.(*ast.IntegerLiteral); ok {
				c := lit.Value
				return func(env *object.Environment) (int64, bool) {
					i, ok := evalIdentifier(id, env).(*object.Integer)
					if !ok {
						return 0, false
					}
					return applyIntFastOp(op, i.IntValue(), c)
				}
			}
		}
		left := compileIntOperand(n.Left)
		right := compileIntOperand(n.Right)
		if left == nil || right == nil {
			return nil
		}
		return func(env *object.Environment) (int64, bool) {
			l, ok := left(env)
			if !ok {
				return 0, false
			}
			r, ok := right(env)
			if !ok {
				return 0, false
			}
			return applyIntFastOp(op, l, r)
		}
	}
	return nil
}

// compileIntInfix compiles an IntFast infix node to a function that returns
// its boxed result (at most one box, none for comparisons), or ok=false to send
// the caller to the general path. Returns nil if the node is not IntFast.
func compileIntInfix(n *ast.InfixExpression) func(env *object.Environment) (object.Object, bool) {
	switch n.IntFast {
	case ast.IntFastArith:
		fn := compileIntOperand(n)
		if fn == nil {
			return nil
		}
		return func(env *object.Environment) (object.Object, bool) {
			if v, ok := fn(env); ok {
				return object.NewInteger(v), true
			}
			return nil, false
		}
	case ast.IntFastCompare:
		left := compileIntOperand(n.Left)
		right := compileIntOperand(n.Right)
		if left == nil || right == nil {
			return nil
		}
		op := n.Operator
		return func(env *object.Environment) (object.Object, bool) {
			l, ok := left(env)
			if !ok {
				return nil, false
			}
			r, ok := right(env)
			if !ok {
				return nil, false
			}
			switch op {
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
	}
	return nil
}
