package evaluator

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/paularlott/scriptling/ast"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

var (
	NULL  = &object.Null{}
	TRUE  = object.NewBoolean(true)
	FALSE = object.NewBoolean(false)
)

func acquireReturnValue(env *object.Environment, val object.Object) *object.ReturnValue {
	return object.AcquireReturnValue(env, val)
}

func releaseReturnValue(rv *object.ReturnValue) {
	object.ReleaseReturnValue(rv)
}

// envContextKey is used to store environment in context
const envContextKey = "scriptling-env"

// callDepthKey is used to store call depth counter in context
type callDepthKey struct{}

// DefaultMaxCallDepth is the default maximum call depth (1000)
const DefaultMaxCallDepth = 1000

// CallDepth tracks function call depth to prevent stack overflow
type CallDepth struct {
	current int32
	max     int32
}

// NewCallDepth creates a new CallDepth tracker with the specified max depth
func NewCallDepth(maxDepth int) *CallDepth {
	return &CallDepth{max: int32(maxDepth)}
}

// Enter increments call depth and returns true if within limits
func (cd *CallDepth) Enter() bool {
	cd.current++
	return cd.current <= cd.max
}

// Exit decrements call depth
func (cd *CallDepth) Exit() {
	cd.current--
}

// Depth returns current call depth
func (cd *CallDepth) Depth() int {
	return int(cd.current)
}

// SetEnvInContext stores environment in context for builtin functions.
//
// Every builtin call goes through here, and context.WithValue allocates, so the
// result is memoised on the environment: the base context is stable for the
// duration of an evaluation, so the same derived context can be handed out
// repeatedly instead of rebuilt per call.
func SetEnvInContext(ctx context.Context, env *object.Environment) context.Context {
	if env == nil {
		return context.WithValue(ctx, envContextKey, env)
	}
	if cached, ok := env.CachedEnvContext(ctx); ok {
		return cached
	}
	derived := context.WithValue(ctx, envContextKey, env)
	env.StoreEnvContext(ctx, derived)
	return derived
}

// GetEnvFromContext retrieves environment from context for external functions
func GetEnvFromContext(ctx context.Context) *object.Environment {
	if env, ok := ctx.Value(envContextKey).(*object.Environment); ok {
		return env
	}
	return object.NewEnvironment() // fallback
}

// enterScript acquires the interpreter lock for env's tree at a Go->script
// boundary, unless the calling goroutine already holds it (a re-entrant call,
// e.g. a builtin invoking a script callback). Ownership is tracked by goroutine
// id, so re-entrancy and "am I the holder?" checks are exact regardless of how
// the context was derived. Returns the env whose GIL was acquired (the caller
// defers its ExitGIL), or nil when the call was re-entrant (caller does nothing).
// Returning an env pointer rather than a func() avoids per-call closure
// allocation on the hot Go->script boundary.
func enterScript(env *object.Environment) *object.Environment {
	if env != nil && env.EnterGIL() {
		return env
	}
	return nil
}

// SetCallDepthInContext stores call depth tracker in context
func SetCallDepthInContext(ctx context.Context, cd *CallDepth) context.Context {
	return context.WithValue(ctx, callDepthKey{}, cd)
}

// GetCallDepthFromContext retrieves call depth tracker from context
func GetCallDepthFromContext(ctx context.Context) *CallDepth {
	if cd, ok := ctx.Value(callDepthKey{}).(*CallDepth); ok {
		return cd
	}
	return nil
}

// ContextWithCallDepth creates a context with a call depth tracker
func ContextWithCallDepth(ctx context.Context, maxDepth int) context.Context {
	return SetCallDepthInContext(ctx, NewCallDepth(maxDepth))
}

// sourceFileKey is used to store source file name in context for error reporting
type sourceFileKey struct{}

// ContextWithSourceFile creates a context with source file info for error reporting
func ContextWithSourceFile(ctx context.Context, filename string) context.Context {
	return context.WithValue(ctx, sourceFileKey{}, filename)
}

// GetSourceFileFromContext retrieves source file name from context
func GetSourceFileFromContext(ctx context.Context) string {
	if f, ok := ctx.Value(sourceFileKey{}).(string); ok {
		return f
	}
	return ""
}

// Eval executes without context (backwards compatible)
func Eval(node ast.Node, env *object.Environment) object.Object {
	return EvalWithContext(context.Background(), node, env)
}

// EvalWithContext executes with context for timeout/cancellation
func EvalWithContext(ctx context.Context, node ast.Node, env *object.Environment) object.Object {
	// Check for cancellation at start of each evaluation
	select {
	case <-ctx.Done():
		if ctx.Err() == context.DeadlineExceeded {
			return errors.NewTimeoutError()
		}
		return errors.NewCancelledError()
	default:
	}

	ctx = WithEvaluator(ctx)
	// Add the default call-depth tracker last (outermost) when a caller hasn't
	// supplied one, so the per-call GetCallDepthFromContext lookup on the hot
	// path is O(1) rather than walking past the evaluator/source-file wrappers.
	if GetCallDepthFromContext(ctx) == nil {
		ctx = ContextWithCallDepth(ctx, DefaultMaxCallDepth)
	}
	// Acquire the interpreter lock for the duration of this top-level run so the
	// whole tree is touched by one goroutine at a time.
	if gilEnv := enterScript(env); gilEnv != nil {
		defer gilEnv.ExitGIL()
	}
	// Programs run through their compiled closure tree, built once per
	// *ast.Program and shared like the cached parse itself. A racing goroutine
	// may store an equivalent closure; the extra store is harmless.
	if program, ok := node.(*ast.Program); ok {
		fn, _ := program.Compiled().(object.EvalFn)
		if fn == nil {
			fn = compileProgram(program)
			program.SetCompiled(fn)
		}
		return fn(ctx, env)
	}
	// Any other node (hosts and tests may evaluate a bare statement or
	// expression) is compiled on the spot; this path is not performance
	// sensitive.
	return fixErrorPos(ctx, compileNode(node)(ctx, env), node.Line())
}

// checkContext checks for cancellation and returns error if cancelled
func checkContext(ctx context.Context) object.Object {
	select {
	case <-ctx.Done():
		if ctx.Err() == context.DeadlineExceeded {
			return errors.NewTimeoutError()
		}
		return errors.NewCancelledError()
	default:
		return nil
	}
}

// contextCheckBatch is how many loop iterations or statements run between
// cancellation checks.
const contextCheckBatch = 10

// contextChecker helps batch context checks in loops to reduce overhead.
//
// The context's Done channel is resolved lazily on the first real check and
// then cached: ctx.Done() walks the whole chain of value/cancel wrappers on
// every call, and for a context that can never be cancelled (the common case
// for embedded use) it returns nil, so once resolved a check is a counter
// compare and, at most, a nil test. Resolving lazily rather than in
// newContextChecker keeps short blocks, which never reach the batch size,
// free of the Done() walk entirely.
type contextChecker struct {
	ctx      context.Context
	done     <-chan struct{}
	counter  int
	resolved bool
}

func newContextChecker(ctx context.Context) contextChecker {
	return contextChecker{ctx: ctx}
}

func (cc *contextChecker) check() object.Object {
	cc.counter++
	if cc.counter < contextCheckBatch {
		return nil
	}
	return cc.checkBatch()
}

// checkBatch runs once per batch. It is kept out of check so that check stays
// small enough to inline at every statement and loop iteration.
func (cc *contextChecker) checkBatch() object.Object {
	cc.counter = 0
	if !cc.resolved {
		cc.done = cc.ctx.Done()
		cc.resolved = true
	}
	if cc.done == nil {
		return nil
	}
	select {
	case <-cc.done:
		return checkContext(cc.ctx)
	default:
		return nil
	}
}

// checkAlways checks context every time (for critical sections)
func (cc *contextChecker) checkAlways() object.Object {
	return checkContext(cc.ctx)
}

// assignErrorToObject converts an assignment or deletion error into the object
// the evaluator propagates: a script-level exception passes through as an
// Exception so `except` clauses can catch it, anything else becomes a plain
// Error.
func assignErrorToObject(err error) object.Object {
	if ae, ok := err.(*assignmentExceptionError); ok {
		// Surfacing from a failed del/assignment is a raise, so it must unwind
		// the stack and be catchable by an enclosing except clause.
		ae.ex.Raised = true
		return ae.ex
	}
	return errors.NewError("%s", err.Error())
}

// evalStringLiteral returns the runtime value of a string literal, reusing the
// one cached on the AST node.
//
// Strings are immutable in Scriptling — nothing mutates an *object.String in
// place — so a single value can safely back every evaluation of a given literal.
// This matters most for attribute access: `obj.attr` desugars to an index by a
// string literal, so without this every field read allocated its own copy of the
// field name.
func evalStringLiteral(node *ast.StringLiteral) object.Object {
	if s, ok := node.Boxed().(*object.String); ok {
		return s
	}
	s := object.NewString(node.Value)
	node.SetBoxed(s)
	return s
}

func nativeBoolToBooleanObject(input bool) *object.Boolean {
	if input {
		return TRUE
	}
	return FALSE
}

func objectsEqual(a, b object.Object) bool {
	if a.Type() != b.Type() {
		return false
	}
	switch av := a.(type) {
	case *object.Integer:
		return av.IntValue() == b.(*object.Integer).IntValue()
	case *object.Float:
		return av.FloatValue() == b.(*object.Float).FloatValue()
	case *object.String:
		return av.StringValue() == b.(*object.String).StringValue()
	case *object.Bytes:
		return av.Equal(b.(*object.Bytes))
	case *object.Boolean:
		return av.BoolValue() == b.(*object.Boolean).BoolValue()
	case *object.Null:
		return true
	default:
		return a == b // Reference equality for complex types
	}
}

// evalObjectsEqualChecked compares a and b for equality, dispatching to a
// user-defined __eq__ when either side is an instance (left operand first,
// then the reflected right side, mirroring Python's rich comparisons) and
// falling back to structural equality otherwise. The second return value is
// non-nil when the __eq__ call produced a propagating exception or internal
// error; the caller must propagate it instead of using the bool.
func evalObjectsEqualChecked(ctx context.Context, a, b object.Object, env *object.Environment) (bool, object.Object) {
	if aInst, ok := a.(*object.Instance); ok {
		if method, has := aInst.Class.Methods["__eq__"]; has {
			result := applyFunctionWithContext(ctx, method, []object.Object{a, b}, nil, env)
			if propagates(result) {
				return false, result
			}
			if bl, ok := result.(*object.Boolean); ok {
				return bl.BoolValue(), nil
			}
			return false, nil
		}
	}
	if bInst, ok := b.(*object.Instance); ok {
		if method, has := bInst.Class.Methods["__eq__"]; has {
			result := applyFunctionWithContext(ctx, method, []object.Object{b, a}, nil, env)
			if propagates(result) {
				return false, result
			}
			if bl, ok := result.(*object.Boolean); ok {
				return bl.BoolValue(), nil
			}
			return false, nil
		}
	}
	return objectsEqual(a, b), nil
}

// isInstanceOperand reports whether obj is a class instance (and therefore
// participates in dunder-driven comparison/hashing).
func isInstanceOperand(obj object.Object) bool {
	_, ok := obj.(*object.Instance)
	return ok
}

// objectsDeepEqual compares two objects for deep equality (handles lists, tuples, dicts)
func objectsDeepEqual(a, b object.Object) bool {
	if a.Type() != b.Type() {
		return false
	}
	switch av := a.(type) {
	case *object.Integer:
		return av.IntValue() == b.(*object.Integer).IntValue()
	case *object.Float:
		return av.FloatValue() == b.(*object.Float).FloatValue()
	case *object.String:
		return av.StringValue() == b.(*object.String).StringValue()
	case *object.Bytes:
		return av.Equal(b.(*object.Bytes))
	case *object.Boolean:
		return av.BoolValue() == b.(*object.Boolean).BoolValue()
	case *object.Null:
		return true
	case *object.List:
		bv := b.(*object.List)
		if len(av.Elements) != len(bv.Elements) {
			return false
		}
		for i, elem := range av.Elements {
			if !objectsDeepEqual(elem, bv.Elements[i]) {
				return false
			}
		}
		return true
	case *object.Tuple:
		bv := b.(*object.Tuple)
		if len(av.Elements) != len(bv.Elements) {
			return false
		}
		for i, elem := range av.Elements {
			if !objectsDeepEqual(elem, bv.Elements[i]) {
				return false
			}
		}
		return true
	case *object.Dict:
		bv := b.(*object.Dict)
		if len(av.Pairs) != len(bv.Pairs) {
			return false
		}
		for key, pairA := range av.Pairs {
			pairB, ok := bv.Pairs[key]
			if !ok {
				return false
			}
			if !objectsDeepEqual(pairA.Value, pairB.Value) {
				return false
			}
		}
		return true
	case *object.Set:
		// Sets are keyed by DictKey (identity hash), so equal key sets means
		// equal sets regardless of insertion/iteration order.
		bv := b.(*object.Set)
		if len(av.Elements) != len(bv.Elements) {
			return false
		}
		for key := range av.Elements {
			if _, ok := bv.Elements[key]; !ok {
				return false
			}
		}
		return true
	default:
		return a == b // Reference equality for other types
	}
}

func evalPrefixExpression(ctx context.Context, operator ast.Op, right object.Object, env *object.Environment) object.Object {
	switch operator {
	case ast.OpNot:
		return evalNotOperatorExpression(ctx, right, env)
	case ast.OpSub:
		return evalMinusPrefixOperatorExpression(right)
	case ast.OpBitNot:
		return evalBitwiseNotOperatorExpression(right)
	default:
		return errors.NewError("%s: %s%s", errors.ErrUnknownOperator, operator, right.Type())
	}
}

func evalNotOperatorExpression(ctx context.Context, right object.Object, env *object.Environment) object.Object {
	truthy, errObj := evalTruthy(ctx, right, env)
	if errObj != nil {
		return errObj
	}
	if truthy {
		return FALSE
	}
	return TRUE
}

func evalMinusPrefixOperatorExpression(right object.Object) object.Object {
	switch right := right.(type) {
	case *object.Integer:
		return object.NewInteger(-right.IntValue())
	case *object.Float:
		return object.NewFloat(-right.FloatValue())
	default:
		return errors.NewError("%s: -%s", errors.ErrUnknownOperator, right.Type())
	}
}

func evalBitwiseNotOperatorExpression(right object.Object) object.Object {
	switch right := right.(type) {
	case *object.Integer:
		return object.NewInteger(^right.IntValue())
	default:
		return errors.NewError("%s: ~%s", errors.ErrUnknownOperator, right.Type())
	}
}

func evalInfixExpression(ctx context.Context, operator ast.Op, left, right object.Object, env *object.Environment) object.Object {

	switch operator {
	case ast.OpIn:
		return evalInOperator(ctx, left, right, env)
	case ast.OpNotIn:
		result := evalInOperator(ctx, left, right, env)
		if propagates(result) {
			return result
		}
		if result == TRUE {
			return FALSE
		}
		return TRUE
	case ast.OpIs:
		return evalIsOperator(left, right)
	case ast.OpIsNot:
		result := evalIsOperator(left, right)
		if result == TRUE {
			return FALSE
		}
		return TRUE
	}

	switch l := left.(type) {
	case *object.Integer:
		switch r := right.(type) {
		case *object.Integer:
			return evalIntegerInfixExpression(operator, l.IntValue(), r.IntValue())
		case *object.Float:
			return evalFloatInfixValues(operator, float64(l.IntValue()), r.FloatValue())
		case *object.Boolean:
			rv := int64(0)
			if r.BoolValue() {
				rv = 1
			}
			return evalIntegerInfixExpression(operator, l.IntValue(), rv)
		case *object.String:
			if operator == ast.OpMul {
				return evalStringMultiplication(r.StringValue(), l.IntValue())
			}
		case *object.Bytes:
			if operator == ast.OpMul {
				return evalBytesMultiplication(r, l.IntValue())
			}
		case *object.List:
			if operator == ast.OpMul {
				elements, errObj := repeatElements(r.Elements, l.IntValue())
				if errObj != nil {
					return errObj
				}
				return &object.List{Elements: elements}
			}
		case *object.Tuple:
			if operator == ast.OpMul {
				elements, errObj := repeatElements(r.Elements, l.IntValue())
				if errObj != nil {
					return errObj
				}
				return &object.Tuple{Elements: elements}
			}
		}
		return evalFloatInfixExpression(operator, left, right)
	case *object.Float:
		switch r := right.(type) {
		case *object.Float:
			return evalFloatInfixValues(operator, l.FloatValue(), r.FloatValue())
		case *object.Integer:
			return evalFloatInfixValues(operator, l.FloatValue(), float64(r.IntValue()))
		case *object.Boolean:
			rv := float64(0)
			if r.BoolValue() {
				rv = 1
			}
			return evalFloatInfixValues(operator, l.FloatValue(), rv)
		default:
			return evalFloatInfixExpression(operator, left, right)
		}
	case *object.Boolean:
		lv := int64(0)
		if l.BoolValue() {
			lv = 1
		}
		if rb, ok := right.(*object.Boolean); ok {
			rv := int64(0)
			if rb.BoolValue() {
				rv = 1
			}
			return evalIntegerInfixExpression(operator, lv, rv)
		}
		return evalInfixExpression(ctx, operator, object.NewInteger(lv), right, env)
	case *object.String:
		if operator == ast.OpMod {
			return evalStringPercentFormat(ctx, l.StringValue(), right, env)
		}
		if r, ok := right.(*object.String); ok {
			return evalStringInfixExpression(operator, l.StringValue(), r.StringValue())
		}
		if r, ok := right.(*object.Integer); ok && operator == ast.OpMul {
			return evalStringMultiplication(l.StringValue(), r.IntValue())
		}
	case *object.Bytes:
		return evalBytesInfixExpression(operator, l, right)
	case *object.FloatArray:
		if operator == ast.OpAdd {
			if r, ok := right.(*object.FloatArray); ok {
				newData := make([]float64, len(l.Data)+len(r.Data))
				copy(newData, l.Data)
				copy(newData[len(l.Data):], r.Data)
				if l.Is2D() && r.Is2D() {
					return object.NewFloatArray2D(newData, l.Rows()+r.Rows(), l.Cols())
				}
				return object.NewFloatArray1D(newData)
			}
			return errors.NewTypeError("FLOAT_ARRAY", right.Type().String())
		}
	case *object.Instance:
		if result := evalInstanceInfixExpression(ctx, operator, l, right, env); result != nil {
			return result
		}
		// Reflected comparison (Python rich comparisons): when the left
		// operand defines no dunder for this operator, try the mirrored
		// dunder on the right operand.
		if result := evalReflectedInstanceComparison(ctx, operator, left, right, env); result != nil {
			return result
		}
	case *object.Tuple:
		switch operator {
		case ast.OpLt, ast.OpGt, ast.OpLte, ast.OpGte:
			// Python ordering for tuples: element-wise, tuples only.
			if r, ok := right.(*object.Tuple); ok {
				cmp, errObj := compareForSort(ctx, l, r, env)
				if errObj != nil {
					return errObj
				}
				return orderResult(operator, cmp)
			}
		case ast.OpAdd:
			if r, ok := right.(*object.Tuple); ok {
				result := make([]object.Object, len(l.Elements)+len(r.Elements))
				copy(result, l.Elements)
				copy(result[len(l.Elements):], r.Elements)
				return &object.Tuple{Elements: result}
			}
			return errors.NewTypeError("tuple", right.Type().String())
		case ast.OpMul:
			if r, ok := right.(*object.Integer); ok {
				elements, errObj := repeatElements(l.Elements, r.IntValue())
				if errObj != nil {
					return errObj
				}
				return &object.Tuple{Elements: elements}
			}
			return errors.NewTypeError("int", right.Type().String())
		case ast.OpEq:
			return nativeBoolToBooleanObject(objectsDeepEqual(left, right))
		case ast.OpNeq:
			return nativeBoolToBooleanObject(!objectsDeepEqual(left, right))
		}
	case *object.List:
		switch operator {
		case ast.OpLt, ast.OpGt, ast.OpLte, ast.OpGte:
			// Python ordering for sequences: element-wise, lists only
			// compare with lists (mixed list/tuple is a TypeError).
			if r, ok := right.(*object.List); ok {
				cmp, errObj := compareForSort(ctx, l, r, env)
				if errObj != nil {
					return errObj
				}
				return orderResult(operator, cmp)
			}
		case ast.OpAdd:
			var rightElems []object.Object
			switch r := right.(type) {
			case *object.List:
				rightElems = r.Elements
			case *object.Tuple:
				rightElems = r.Elements
			default:
				return errors.NewTypeError("list", right.Type().String())
			}
			result := make([]object.Object, len(l.Elements)+len(rightElems))
			copy(result, l.Elements)
			copy(result[len(l.Elements):], rightElems)
			return &object.List{Elements: result}
		case ast.OpMul:
			if r, ok := right.(*object.Integer); ok {
				elements, errObj := repeatElements(l.Elements, r.IntValue())
				if errObj != nil {
					return errObj
				}
				return &object.List{Elements: elements}
			}
			return errors.NewTypeError("int", right.Type().String())
		}
	case *object.Dict:
		// Dict merge (PEP 584): d1 | d2 builds a new dict with d2's keys
		// winning on conflicts; both operands are left unchanged. The |= form
		// flows through here too via the augmented-assignment base op.
		if operator == ast.OpBitOr {
			r, ok := right.(*object.Dict)
			if !ok {
				return errors.NewTypeError("dict", right.Type().String())
			}
			merged := &object.Dict{Pairs: make(map[string]object.DictPair, len(l.Pairs)+len(r.Pairs))}
			for k, v := range l.Pairs {
				merged.Pairs[k] = v
			}
			for k, v := range r.Pairs {
				merged.Pairs[k] = v
			}
			return merged
		}
	case *object.Set:
		// Set algebra operators. Both operands must be sets (matching Python);
		// for iterable operands use the .intersection()/.union()/etc. methods.
		switch operator {
		case ast.OpBitAnd:
			if r, ok := right.(*object.Set); ok {
				return l.Intersection(r)
			}
			return errors.NewTypeError("set", right.Type().String())
		case ast.OpBitOr:
			if r, ok := right.(*object.Set); ok {
				return l.Union(r)
			}
			return errors.NewTypeError("set", right.Type().String())
		case ast.OpSub:
			if r, ok := right.(*object.Set); ok {
				return l.Difference(r)
			}
			return errors.NewTypeError("set", right.Type().String())
		case ast.OpBitXor:
			if r, ok := right.(*object.Set); ok {
				return l.SymmetricDifference(r)
			}
			return errors.NewTypeError("set", right.Type().String())
		}
	}

	if rb, ok := right.(*object.Boolean); ok {
		if operator >= ast.OpAdd && operator <= ast.OpNeq {
			rv := int64(0)
			if rb.BoolValue() {
				rv = 1
			}
			return evalInfixExpression(ctx, operator, left, object.NewInteger(rv), env)
		}
	}

	switch operator {
	case ast.OpEq:
		if la, ok := left.(*object.FloatArray); ok {
			if ra, ok := right.(*object.FloatArray); ok {
				return nativeBoolToBooleanObject(floatArraysEqual(la, ra))
			}
			return FALSE
		}
		return nativeBoolToBooleanObject(objectsDeepEqual(left, right))
	case ast.OpNeq:
		if la, ok := left.(*object.FloatArray); ok {
			if ra, ok := right.(*object.FloatArray); ok {
				return nativeBoolToBooleanObject(!floatArraysEqual(la, ra))
			}
			return TRUE
		}
		return nativeBoolToBooleanObject(!objectsDeepEqual(left, right))
	default:
		return newUnsupportedOperandError(operator, left, right)
	}
}

func floatArraysEqual(left, right *object.FloatArray) bool {
	if len(left.Shape) != len(right.Shape) {
		return false
	}
	for i := range left.Shape {
		if left.Shape[i] != right.Shape[i] {
			return false
		}
	}
	if len(left.Data) != len(right.Data) {
		return false
	}
	for i := range left.Data {
		if left.Data[i] != right.Data[i] {
			return false
		}
	}
	return true
}

// orderResult maps a three-way comparison to the boolean result of an
// ordering operator (<, <=, >, >=), Python-style.
func orderResult(operator ast.Op, cmp int) object.Object {
	switch operator {
	case ast.OpLt:
		return nativeBoolToBooleanObject(cmp < 0)
	case ast.OpGt:
		return nativeBoolToBooleanObject(cmp > 0)
	case ast.OpLte:
		return nativeBoolToBooleanObject(cmp <= 0)
	case ast.OpGte:
		return nativeBoolToBooleanObject(cmp >= 0)
	}
	return errors.NewError("%s: type mismatch", errors.ErrTypeError)
}

// newUnsupportedOperandError builds Python's arithmetic TypeError
// ("unsupported operand type(s) for +: 'int' and 'str'") as a typed Error so
// it keeps line/library stamping and converts to a catchable exception.
func newUnsupportedOperandError(operator ast.Op, left, right object.Object) object.Object {
	err := errors.NewError("unsupported operand type(s) for %s: '%s' and '%s'", operator.String(), getTypeName(left), getTypeName(right))
	err.ExceptionType = object.ExceptionTypeTypeError
	return err
}

func evalIntegerInfixExpression(operator ast.Op, leftVal, rightVal int64) object.Object {
	switch operator {
	case ast.OpAdd:
		return object.NewInteger(leftVal + rightVal)
	case ast.OpSub:
		return object.NewInteger(leftVal - rightVal)
	case ast.OpMul:
		return object.NewInteger(leftVal * rightVal)
	case ast.OpDiv:
		if rightVal == 0 {
			return errors.NewZeroDivisionError()
		}
		return object.NewFloat(float64(leftVal) / float64(rightVal))
	case ast.OpFloorDiv:
		if rightVal == 0 {
			return errors.NewZeroDivisionError()
		}
		// Python floors toward negative infinity; Go truncates toward zero,
		// so adjust when the signs differ and the division is inexact.
		q := leftVal / rightVal
		if leftVal%rightVal != 0 && (leftVal < 0) != (rightVal < 0) {
			q--
		}
		return object.NewInteger(q)
	case ast.OpPow:
		if rightVal < 0 {
			return evalFloatInfixValues(ast.OpPow, float64(leftVal), float64(rightVal))
		}
		if rightVal > 63 || (leftVal > 1 && rightVal > 40) || (leftVal < -1 && rightVal > 40) {
			return object.NewFloat(math.Pow(float64(leftVal), float64(rightVal)))
		}
		result := int64(1)
		base := leftVal
		exp := rightVal
		for exp > 0 {
			if exp%2 == 1 {
				result *= base
			}
			base *= base
			exp /= 2
		}
		return object.NewInteger(result)
	case ast.OpMod:
		if rightVal == 0 {
			return errors.NewZeroDivisionError()
		}
		// Python's % takes the sign of the divisor; Go's takes the sign of
		// the dividend, so fold the remainder over when they disagree.
		m := leftVal % rightVal
		if m != 0 && (m < 0) != (rightVal < 0) {
			m += rightVal
		}
		return object.NewInteger(m)
	case ast.OpBitAnd:
		return object.NewInteger(leftVal & rightVal)
	case ast.OpBitOr:
		return object.NewInteger(leftVal | rightVal)
	case ast.OpBitXor:
		return object.NewInteger(leftVal ^ rightVal)
	case ast.OpLShift:
		if rightVal < 0 {
			return errors.NewError("negative shift count")
		}
		return object.NewInteger(leftVal << uint64(rightVal))
	case ast.OpRShift:
		if rightVal < 0 {
			return errors.NewError("negative shift count")
		}
		return object.NewInteger(leftVal >> uint64(rightVal))
	case ast.OpLt:
		return nativeBoolToBooleanObject(leftVal < rightVal)
	case ast.OpGt:
		return nativeBoolToBooleanObject(leftVal > rightVal)
	case ast.OpLte:
		return nativeBoolToBooleanObject(leftVal <= rightVal)
	case ast.OpGte:
		return nativeBoolToBooleanObject(leftVal >= rightVal)
	case ast.OpEq:
		return nativeBoolToBooleanObject(leftVal == rightVal)
	case ast.OpNeq:
		return nativeBoolToBooleanObject(leftVal != rightVal)
	default:
		return errors.NewError("unknown operator: INTEGER %s INTEGER", operator)
	}
}

func evalFloatInfixExpression(operator ast.Op, left, right object.Object) object.Object {
	leftVal, ok := numericFloatValue(left)
	if !ok {
		return newUnsupportedOperandError(operator, left, right)
	}
	rightVal, ok := numericFloatValue(right)
	if !ok {
		switch operator {
		case ast.OpEq:
			return FALSE
		case ast.OpNeq:
			return TRUE
		}
		return newUnsupportedOperandError(operator, left, right)
	}
	return evalFloatInfixValues(operator, leftVal, rightVal)
}

func evalFloatInfixValues(operator ast.Op, leftVal, rightVal float64) object.Object {
	switch operator {
	case ast.OpAdd:
		return object.NewFloat(leftVal + rightVal)
	case ast.OpSub:
		return object.NewFloat(leftVal - rightVal)
	case ast.OpMul:
		return object.NewFloat(leftVal * rightVal)
	case ast.OpDiv:
		if rightVal == 0 {
			return errors.NewZeroDivisionError()
		}
		return object.NewFloat(leftVal / rightVal)
	case ast.OpFloorDiv:
		if rightVal == 0 {
			return errors.NewZeroDivisionError()
		}
		return object.NewFloat(math.Floor(leftVal / rightVal))
	case ast.OpMod:
		if rightVal == 0 {
			return errors.NewZeroDivisionError()
		}
		// Python's % takes the sign of the divisor, like the integer path.
		m := math.Mod(leftVal, rightVal)
		if m != 0 && (m < 0) != (rightVal < 0) {
			m += rightVal
		}
		return object.NewFloat(m)
	case ast.OpPow:
		return object.NewFloat(math.Pow(leftVal, rightVal))
	case ast.OpLt:
		return nativeBoolToBooleanObject(leftVal < rightVal)
	case ast.OpGt:
		return nativeBoolToBooleanObject(leftVal > rightVal)
	case ast.OpLte:
		return nativeBoolToBooleanObject(leftVal <= rightVal)
	case ast.OpGte:
		return nativeBoolToBooleanObject(leftVal >= rightVal)
	case ast.OpEq:
		return nativeBoolToBooleanObject(leftVal == rightVal)
	case ast.OpNeq:
		return nativeBoolToBooleanObject(leftVal != rightVal)
	default:
		return errors.NewError("unknown operator: FLOAT %s FLOAT", operator)
	}
}

func numericFloatValue(obj object.Object) (float64, bool) {
	switch v := obj.(type) {
	case *object.Float:
		return v.FloatValue(), true
	case *object.Integer:
		return float64(v.IntValue()), true
	default:
		return 0, false
	}
}

func evalStringInfixExpression(operator ast.Op, leftVal, rightVal string) object.Object {
	switch operator {
	case ast.OpAdd:
		if len(leftVal) == 0 {
			return object.NewString(rightVal)
		}
		if len(rightVal) == 0 {
			return object.NewString(leftVal)
		}
		return object.NewString(leftVal + rightVal)
	case ast.OpEq:
		return nativeBoolToBooleanObject(leftVal == rightVal)
	case ast.OpNeq:
		return nativeBoolToBooleanObject(leftVal != rightVal)
	case ast.OpLt:
		return nativeBoolToBooleanObject(leftVal < rightVal)
	case ast.OpGt:
		return nativeBoolToBooleanObject(leftVal > rightVal)
	case ast.OpLte:
		return nativeBoolToBooleanObject(leftVal <= rightVal)
	case ast.OpGte:
		return nativeBoolToBooleanObject(leftVal >= rightVal)
	default:
		return errors.NewError("%s: STRING %s STRING", errors.ErrUnknownOperator, operator)
	}
}

// evalStringPercentFormat implements Python-style % string formatting.
// Supports: %s, %d, %i, %f, %e, %g, %x, %X, %o, %c, %r, %%
// With width/precision: %10s, %-10s, %.2f, %05d, etc.
// Right side can be a single value or a tuple of values.
func evalStringPercentFormat(ctx context.Context, format string, right object.Object, env *object.Environment) object.Object {
	// Collect values: if right is a tuple, use its elements; otherwise single value
	var values []object.Object
	if tuple, ok := right.(*object.Tuple); ok {
		values = tuple.Elements
	} else {
		values = []object.Object{right}
	}

	var result strings.Builder
	valueIdx := 0
	usedNamedKey := false
	i := 0

	for i < len(format) {
		if format[i] == '%' {
			i++
			if i >= len(format) {
				return errors.NewError("incomplete format string ending with %%")
			}
			// Literal %%
			if format[i] == '%' {
				result.WriteByte('%')
				i++
				continue
			}

			// Parse format specifier: %[(key)][flags][width][.precision]type
			specStart := i - 1

			// Named form: %(key)s reads the value from a dict right-hand
			// side (the logging/template idiom).
			key := ""
			hasKey := false
			if format[i] == '(' {
				end := strings.IndexByte(format[i:], ')')
				if end < 0 {
					return errors.NewError("incomplete format key in format string")
				}
				key = format[i+1 : i+end]
				hasKey = true
				i += end + 1
				if i >= len(format) {
					return errors.NewError("incomplete format string")
				}
				// The spec handed to the formatter excludes the key.
				specStart = i
			}

			// Flags
			for i < len(format) && (format[i] == '-' || format[i] == '+' || format[i] == ' ' || format[i] == '#' || format[i] == '0') {
				i++
			}
			// Width (number or *)
			for i < len(format) && format[i] >= '0' && format[i] <= '9' {
				i++
			}
			// Precision
			if i < len(format) && format[i] == '.' {
				i++
				for i < len(format) && format[i] >= '0' && format[i] <= '9' {
					i++
				}
			}

			if i >= len(format) {
				return errors.NewError("incomplete format string")
			}

			// Without a key, specStart sits on the '%' itself; with a named
			// key it was advanced past the key and needs the '%' back.
			spec := format[specStart : i+1]
			if hasKey {
				spec = "%" + spec
			}
			conversion := format[i]
			i++

			var val object.Object
			if hasKey {
				dict, ok := right.(*object.Dict)
				if !ok {
					return &object.Exception{
						Message:       "format requires a mapping",
						ExceptionType: object.ExceptionTypeTypeError,
						Raised:        true,
					}
				}
				pair, found := dict.GetByString(key)
				if !found {
					return &object.Exception{
						Message:       object.ReprString(key),
						ExceptionType: object.ExceptionTypeKeyError,
						Raised:        true,
					}
				}
				val = pair.Value
				usedNamedKey = true
			} else {
				if valueIdx >= len(values) {
					return errors.NewError("not enough arguments for format string")
				}
				val = values[valueIdx]
				valueIdx++
			}

			formatted, err := formatPercentValue(ctx, spec, conversion, val, env)
			if err != nil {
				return err
			}
			result.WriteString(formatted)
		} else {
			result.WriteByte(format[i])
			i++
		}
	}

	if valueIdx < len(values) && !usedNamedKey {
		return errors.NewError("not all arguments converted during string formatting")
	}

	return object.NewString(result.String())
}

// applyStringSpec applies a % format's width/precision/flags to a string
// body (for %s and %r): zero-padding is not a string conversion in Python, so
// the 0 flag is dropped; width pads with spaces, '-' left-justifies, and
// precision truncates.
func applyStringSpec(spec string, body string) string {
	goSpec := spec[:len(spec)-1]
	// Drop only a flag '0' (zero-padding), not the '0' digits of the width:
	// flags sit between '%' and the width.
	if i := 1; i < len(goSpec) {
		for i < len(goSpec) && (goSpec[i] == '-' || goSpec[i] == '+' || goSpec[i] == ' ' || goSpec[i] == '#' || goSpec[i] == '0') {
			i++
		}
		flags := goSpec[1:i]
		flags = strings.ReplaceAll(flags, "0", "")
		goSpec = "%" + flags + goSpec[i:]
	}
	return fmt.Sprintf(goSpec+"s", body)
}

// formatPercentValue formats a single value according to a Python % format specifier.
func formatPercentValue(ctx context.Context, spec string, conversion byte, val object.Object, env *object.Environment) (string, object.Object) {
	switch conversion {
	case 's', 'r':
		// %s and %r convert exactly like str() and repr(); a raise from a
		// dunder propagates, and the width/precision spec applies after.
		rendered, rerr := renderConvertedValue(ctx, val, string(conversion), env)
		if rerr != nil {
			return "", rerr
		}
		return applyStringSpec(spec, rendered), nil
	case 'd', 'i':
		var intVal int64
		switch v := val.(type) {
		case *object.Integer:
			intVal = v.IntValue()
		case *object.Float:
			intVal = int64(v.FloatValue())
		case *object.Boolean:
			if v.BoolValue() {
				intVal = 1
			}
		default:
			return "", errors.NewError("%%d format: a number is required, not %s", val.Type().String())
		}
		return fmt.Sprintf(spec[:len(spec)-1]+"d", intVal), nil
	case 'f':
		floatVal, err := val.AsFloat()
		if err != nil {
			return "", errors.NewError("%%f format: a number is required, not %s", val.Type().String())
		}
		return fmt.Sprintf(spec[:len(spec)-1]+"f", floatVal), nil
	case 'e':
		floatVal, err := val.AsFloat()
		if err != nil {
			return "", errors.NewError("%%e format: a number is required, not %s", val.Type().String())
		}
		return fmt.Sprintf(spec[:len(spec)-1]+"e", floatVal), nil
	case 'g':
		floatVal, err := val.AsFloat()
		if err != nil {
			return "", errors.NewError("%%g format: a number is required, not %s", val.Type().String())
		}
		return fmt.Sprintf(spec[:len(spec)-1]+"g", floatVal), nil
	case 'x':
		intVal, err := val.AsInt()
		if err != nil {
			return "", errors.NewError("%%x format: an integer is required, not %s", val.Type().String())
		}
		return fmt.Sprintf(spec[:len(spec)-1]+"x", intVal), nil
	case 'X':
		intVal, err := val.AsInt()
		if err != nil {
			return "", errors.NewError("%%X format: an integer is required, not %s", val.Type().String())
		}
		return fmt.Sprintf(spec[:len(spec)-1]+"X", intVal), nil
	case 'o':
		intVal, err := val.AsInt()
		if err != nil {
			return "", errors.NewError("%%o format: an integer is required, not %s", val.Type().String())
		}
		return fmt.Sprintf(spec[:len(spec)-1]+"o", intVal), nil
	case 'c':
		switch v := val.(type) {
		case *object.Integer:
			if v.IntValue() < 0 || v.IntValue() > 0x10ffff {
				return "", errors.NewError("%%c: ordinal out of range")
			}
			return string(rune(v.IntValue())), nil
		case *object.String:
			if len(v.StringValue()) != 1 {
				return "", errors.NewError("%%c requires int or char")
			}
			return v.StringValue(), nil
		default:
			return "", errors.NewError("%%c requires int or char")
		}
	default:
		return "", errors.NewError("unsupported format character: %c", conversion)
	}
}

// Repetition result quotas: a repetition is how a script asks for a huge
// allocation in one expression, so the result size is bounded and the refusal
// is a clean error rather than an opaque runtime panic (or, for strings, a
// panic that escapes evaluation) or an OOM kill.
const (
	maxRepeatBytes    = 1 << 30 // 1 GiB per string/bytes repetition result
	maxRepeatElements = 1 << 27 // ~134M elements (~1 GiB of object pointers)
)

// checkRepetition validates multiplier*unitLen against limit. Returns an
// error object to hand back from the fast paths, or nil when the repetition
// may proceed (including the trivially-empty cases).
func checkRepetition(multiplier, unitLen, limit int64) object.Object {
	if multiplier <= 0 || unitLen == 0 {
		return nil
	}
	if multiplier > limit/unitLen {
		return errors.NewError("repetition result too large (over %d units)", limit)
	}
	return nil
}

// repeatElements returns elems repeated multiplier times, an empty slice for a
// non-positive multiplier or empty elems, or the repetition error when the
// result would exceed maxRepeatElements. checkRepetition bounds the element
// count, so neither the int conversion nor slices.Repeat's internal size
// arithmetic can overflow.
func repeatElements(elems []object.Object, multiplier int64) ([]object.Object, object.Object) {
	if multiplier <= 0 || len(elems) == 0 {
		return []object.Object{}, nil
	}
	if errObj := checkRepetition(multiplier, int64(len(elems)), maxRepeatElements); errObj != nil {
		return nil, errObj
	}
	return slices.Repeat(elems, int(multiplier)), nil
}

func evalStringMultiplication(str string, multiplier int64) object.Object {
	if multiplier < 0 {
		return object.NewString("")
	}
	// An empty operand stays empty whatever the multiplier: without this the
	// repeat loop runs multiplier times over nothing, a pure CPU burn.
	if len(str) == 0 {
		return object.NewString("")
	}
	if errObj := checkRepetition(multiplier, int64(len(str)), maxRepeatBytes); errObj != nil {
		return errObj
	}
	return object.NewString(strings.Repeat(str, int(multiplier)))
}

// evalBytesInfixExpression handles binary operators where the left operand is
// Bytes. Supports concatenation (+), repetition (*), equality/ordering
// comparisons against other Bytes, and int * Bytes (mirroring String * int).
// Mixing Bytes with String raises a type error (strict mode).
func evalBytesInfixExpression(operator ast.Op, left *object.Bytes, right object.Object) object.Object {
	// int * bytes → repetition (so 3 * b"ab" == b"ababab")
	if r, ok := right.(*object.Integer); ok && operator == ast.OpMul {
		return evalBytesMultiplication(left, r.IntValue())
	}
	if r, ok := right.(*object.Bytes); ok {
		switch operator {
		case ast.OpAdd:
			joined := make([]byte, 0, left.Len()+r.Len())
			joined = append(joined, left.BytesValue()...)
			joined = append(joined, r.BytesValue()...)
			return object.NewBytes(joined)
		case ast.OpEq:
			return nativeBoolToBooleanObject(left.Equal(r))
		case ast.OpNeq:
			return nativeBoolToBooleanObject(!left.Equal(r))
		case ast.OpLt:
			return nativeBoolToBooleanObject(bytes.Compare(left.BytesValue(), r.BytesValue()) < 0)
		case ast.OpGt:
			return nativeBoolToBooleanObject(bytes.Compare(left.BytesValue(), r.BytesValue()) > 0)
		case ast.OpLte:
			return nativeBoolToBooleanObject(bytes.Compare(left.BytesValue(), r.BytesValue()) <= 0)
		case ast.OpGte:
			return nativeBoolToBooleanObject(bytes.Compare(left.BytesValue(), r.BytesValue()) >= 0)
		}
		return errors.NewError("%s: BYTES %s BYTES", errors.ErrUnknownOperator, operator)
	}
	return errors.NewTypeError("BYTES", right.Type().String())
}

// evalBytesMultiplication returns a Bytes value whose content is the input
// repeated n times. A negative count yields empty bytes (matching String).
func evalBytesMultiplication(b *object.Bytes, multiplier int64) object.Object {
	if multiplier <= 0 {
		return object.NewBytes(nil)
	}
	src := b.BytesValue()
	srcLen := len(src)
	if srcLen == 0 {
		return object.NewBytes(nil)
	}
	if errObj := checkRepetition(multiplier, int64(srcLen), maxRepeatBytes); errObj != nil {
		return errObj
	}
	// checkRepetition bounds the byte count, so bytes.Repeat's internal size
	// arithmetic cannot overflow.
	return object.NewBytes(bytes.Repeat(src, int(multiplier)))
}

// callDunderMethod calls a dunder method on an instance, returning nil if not defined.
// Returns the result string object for __str__/__repr__, or the raw result for others.
func callDunderMethod(ctx context.Context, inst *object.Instance, method string, args []object.Object, env *object.Environment) object.Object {
	if fn, ok := inst.Class.LookupMember(method); ok {
		newArgs := prependSelf(inst, args)
		result := applyFunctionWithContext(ctx, fn, newArgs, nil, env)
		if object.IsError(result) {
			return result
		}
		return result
	}
	return nil
}

// operatorToDunderMethod maps operators to their corresponding dunder method names
var operatorToDunderMethod = map[ast.Op]string{
	ast.OpLt:       "__lt__",
	ast.OpGt:       "__gt__",
	ast.OpLte:      "__le__",
	ast.OpGte:      "__ge__",
	ast.OpEq:       "__eq__",
	ast.OpNeq:      "__ne__",
	ast.OpAdd:      "__add__",
	ast.OpSub:      "__sub__",
	ast.OpMul:      "__mul__",
	ast.OpDiv:      "__truediv__",
	ast.OpFloorDiv: "__floordiv__",
	ast.OpMod:      "__mod__",
	ast.OpBitOr:    "__or__",
	ast.OpBitAnd:   "__and__",
}

// mirroredComparisonDunder maps a comparison operator to the dunder that
// expresses the same comparison when its operands are swapped (a < b is
// b.__gt__(a), a <= b is b.__ge__(a), and so on).
var mirroredComparisonDunder = map[ast.Op]string{
	ast.OpLt:  "__gt__",
	ast.OpGt:  "__lt__",
	ast.OpLte: "__ge__",
	ast.OpGte: "__le__",
	ast.OpEq:  "__eq__",
	ast.OpNeq: "__ne__",
}

// evalReflectedInstanceComparison tries the mirrored comparison dunder on the
// right operand when it is an instance and the left operand did not handle the
// operator. Returns nil when not applicable (non-comparison operator, or the
// right operand defines no mirrored dunder).
func evalReflectedInstanceComparison(ctx context.Context, operator ast.Op, left, right object.Object, env *object.Environment) object.Object {
	rInst, ok := right.(*object.Instance)
	if !ok {
		return nil
	}
	methodName, ok := mirroredComparisonDunder[operator]
	if !ok {
		return nil
	}
	method, has := rInst.Class.Methods[methodName]
	if !has {
		return nil
	}
	return applyFunctionWithContext(ctx, method, []object.Object{right, left}, nil, env)
}

// evalInstanceInfixExpression handles operators on instances by calling dunder methods
// Returns nil if no dunder method is found (falls through to default handling)
func evalInstanceInfixExpression(ctx context.Context, operator ast.Op, left *object.Instance, right object.Object, env *object.Environment) object.Object {
	methodName, ok := operatorToDunderMethod[operator]
	if !ok {
		return nil // No dunder method for this operator
	}

	// Look up the dunder method in the instance's class
	method, ok := left.Class.Methods[methodName]
	if !ok {
		return nil // No dunder method defined
	}

	// Call the dunder method with self and the right operand
	args := []object.Object{left, right}
	return applyFunctionWithContext(ctx, method, args, nil, env)
}

func evalIdentifier(node *ast.Identifier, env *object.Environment) object.Object {
	// Fast path: use cached slot index to skip the slotIndex map lookup.
	// SlotCache encoding: 0=uncached, -1=not a local slot, >0=slot index+1.
	if cached := node.SlotCache.Load(); cached > 0 {
		if val, ok := env.GetCachedSlot(int(cached-1), node.Value()); ok {
			return val
		}
		// Cache miss (wrong scope or stale index), fall through to full lookup.
		node.SlotCache.Store(0)
	}

	if val, ok := env.Get(node.Value()); ok {
		// Cache the slot index if this variable is in the local scope's slots.
		if idx, ok := env.GetSlotIndex(node.Value()); ok {
			if slotVal, slotOK := env.GetSlotByIndex(idx); slotOK && slotVal == val {
				node.SlotCache.Store(int32(idx + 1))
			}
		} else if node.SlotCache.Load() == 0 {
			node.SlotCache.Store(-1) // not a local slot
		}
		return val
	}
	if builtin, ok := builtins[node.Value()]; ok {
		return builtin
	}
	return errors.NewIdentifierError(node.Value())
}

// unpackArgsFromIterable unpacks an iterable object into a slice of arguments
func unpackArgsFromIterable(argsVal object.Object) ([]object.Object, object.Object) {
	var unpacked []object.Object
	switch val := argsVal.(type) {
	case *object.List:
		unpacked = val.Elements
	case *object.Tuple:
		unpacked = val.Elements
	case *object.String:
		for _, r := range val.StringValue() {
			unpacked = append(unpacked, object.NewString(string(r)))
		}
	case *object.Iterator:
		for {
			elem, hasNext := val.Next()
			if !hasNext {
				break
			}
			unpacked = append(unpacked, elem)
		}
	case *object.Dict:
		iter := val.CreateIterator()
		for {
			elem, hasNext := iter.Next()
			if !hasNext {
				break
			}
			unpacked = append(unpacked, elem)
		}
	case *object.DictKeys:
		iter := val.CreateIterator()
		for {
			elem, hasNext := iter.Next()
			if !hasNext {
				break
			}
			unpacked = append(unpacked, elem)
		}
	case *object.DictValues:
		iter := val.CreateIterator()
		for {
			elem, hasNext := iter.Next()
			if !hasNext {
				break
			}
			unpacked = append(unpacked, elem)
		}
	case *object.DictItems:
		iter := val.CreateIterator()
		for {
			elem, hasNext := iter.Next()
			if !hasNext {
				break
			}
			unpacked = append(unpacked, elem)
		}
	case *object.Set:
		iter := val.CreateIterator()
		for {
			elem, hasNext := iter.Next()
			if !hasNext {
				break
			}
			unpacked = append(unpacked, elem)
		}
	default:
		return nil, errors.NewError("argument after * must be iterable, not %s", argsVal.Type())
	}
	return unpacked, nil
}

// resolveCallee resolves an identifier callee using the call node's location
// cache to skip the environment-chain map lookups on repeat calls. The cache
// stores a location (hops + slot index), not a value, so callee reassignment is
// reflected automatically; GetAtLocation revalidates the slot name to guard
// against stale layouts from AST shared across instances via the parse cache.
func resolveCallee(node *ast.CallExpression, name string, env *object.Environment) (object.Object, bool) {
	if c := node.CalleeCache(); c > 0 {
		hops, slotIdx := ast.DecodeCalleeLocation(c)
		if val, ok := env.GetAtLocation(hops, slotIdx, name); ok {
			return val, true
		}
		// Stale layout - fall through to re-resolve and re-cache.
	} else if c < 0 {
		// Known-uncacheable callee (lives in a store map / resolved via builtins).
		return env.Get(name)
	}
	val, hops, slotIdx, ok := env.GetWithLocation(name)
	if !ok {
		return nil, false
	}
	if slotIdx >= 0 {
		node.SetCalleeCache(ast.EncodeCalleeLocation(hops, slotIdx))
	} else {
		node.SetCalleeCache(-1)
	}
	return val, true
}

// assignIndexValue stores value at obj[index] (or obj.attr for dot access)
// given operands that have already been evaluated. assignToExpression and
// augmented assignment share it so each evaluates the operands exactly once.
func assignIndexValue(ctx context.Context, isDotAccess bool, obj, index, value object.Object) error {
	switch o := obj.(type) {
	case *object.List:
		if idx, ok := index.(*object.Integer); ok {
			i := idx.IntValue()
			length := int64(len(o.Elements))
			// Handle negative indices
			if i < 0 {
				i += length
			}
			if i < 0 || i >= length {
				return raisedAssignmentError(object.ExceptionTypeIndexError, "list assignment index out of range")
			}
			o.Elements[i] = value
			return nil
		}
	case *object.Dict:
		key, rerr := evalHashKeyChecked(ctx, index)
		if rerr != nil {
			return hashKeyAssignError(rerr)
		}
		o.Pairs[key] = object.DictPair{Key: index, Value: value}
		return nil
	case *object.Instance:
		// For explicit bracket access (not dot), call __setitem__ if defined
		if !isDotAccess {
			if setitem, ok := o.Class.Methods["__setitem__"]; ok {
				result := applyFunctionWithContext(ctx, setitem, []object.Object{obj, index, value}, nil, nil)
				if object.IsError(result) {
					return fmt.Errorf("%s", result.(*object.Error).Message)
				}
				if isRaised(result) {
					return &assignmentExceptionError{ex: result.(*object.Exception)}
				}
				return nil
			}
		}
		if key, ok := index.(*object.String); ok {
			// Check class hierarchy for a property descriptor before writing to Fields
			if p := findPropertyInClass(key.StringValue(), o.Class); p != nil {
				if p.Setter == nil {
					return fmt.Errorf("can't set attribute '%s': property is read-only", key.StringValue())
				}
				result := applyFunctionWithContext(ctx, p.Setter, []object.Object{o, value}, nil, nil)
				if object.IsError(result) {
					return fmt.Errorf("%s", result.(*object.Error).Message)
				}
				if isRaised(result) {
					return &assignmentExceptionError{ex: result.(*object.Exception)}
				}
				return nil
			}
			o.SetField(key.StringValue(), value)
			o.InvalidateBoundMethod(key.StringValue())
			return nil
		}
		return fmt.Errorf("instance attribute must be string")
	case *object.Class:
		if key, ok := index.(*object.String); ok {
			o.Methods[key.StringValue()] = value
			o.InvalidateLookupCache()
			return nil
		}
		return fmt.Errorf("class attribute must be string")
	case *object.FloatArray:
		idx, ok := index.(*object.Integer)
		if !ok {
			return fmt.Errorf("float_array index must be integer")
		}
		i := idx.IntValue()
		if o.Is2D() {
			rows := int64(o.Rows())
			if i < 0 {
				i += rows
			}
			if i < 0 || i >= rows {
				return fmt.Errorf("index out of range")
			}
			switch v := value.(type) {
			case *object.List:
				cols := o.Cols()
				if len(v.Elements) != cols {
					return fmt.Errorf("row length mismatch: expected %d, got %d", cols, len(v.Elements))
				}
				off := int(i) * cols
				for j, el := range v.Elements {
					f, err := el.AsFloat()
					if err != nil {
						return fmt.Errorf("row element must be a number")
					}
					o.Data[off+j] = f
				}
			case *object.FloatArray:
				cols := o.Cols()
				if v.Is2D() {
					return fmt.Errorf("float_array row assignment requires a 1D FloatArray")
				}
				if len(v.Data) != cols {
					return fmt.Errorf("row length mismatch: expected %d, got %d", cols, len(v.Data))
				}
				off := int(i) * cols
				copy(o.Data[off:off+cols], v.Data)
			default:
				return fmt.Errorf("float_array row assignment requires a list or FloatArray")
			}
			return nil
		}
		length := int64(len(o.Data))
		if i < 0 {
			i += length
		}
		if i < 0 || i >= length {
			return fmt.Errorf("index out of range")
		}
		f, err := value.AsFloat()
		if err != nil {
			return fmt.Errorf("float_array element must be a number")
		}
		o.Data[i] = f
		return nil
	}
	return fmt.Errorf("cannot assign to index")
}

// tryEvalFastBuiltinCall handles fast-path builtin calls (len, type, str, etc.).
// Returns (result, envFn, ok):
//   - ok=true:   result is the builtin's return value, envFn is nil.
//   - ok=false, envFn!=nil: name was found in the environment (not a builtin),
//     envFn holds the resolved value so the caller can skip a redundant lookup.
//   - ok=false, envFn==nil: not applicable, caller should use normal resolution.

func createInstance(ctx context.Context, class *object.Class, args []object.Object, keywords map[string]object.Object, env *object.Environment) object.Object {
	instance := object.NewInstanceWithFields(class, make(map[string]object.Object))

	// Call __init__ if it exists, walking the base class chain
	var initMethod object.Object
	for c := class; c != nil; c = c.BaseClass {
		if m, ok := c.Methods["__init__"]; ok {
			initMethod = m
			break
		}
	}
	if initMethod != nil {
		// Bind 'self' to the instance
		n := len(args) + 1
		var newArgs []object.Object
		if n <= 8 {
			var buf [8]object.Object
			buf[0] = instance
			copy(buf[1:], args)
			newArgs = buf[:n]
		} else {
			newArgs = make([]object.Object, n)
			newArgs[0] = instance
			copy(newArgs[1:], args)
		}
		result := applyFunctionWithContext(ctx, initMethod, newArgs, keywords, env)
		// A raised exception (including an uncatchable security violation such as
		// PermissionError) from __init__ must propagate out of instantiation, not
		// be swallowed so the object constructs cleanly.
		if propagates(result) {
			return result
		}
	}

	// Install GC finalizer for __del__ if the class defines one
	if del, ok := class.LookupMember("__del__"); ok {
		delMethod := del
		// A script destructor owns its lexical environment/GIL domain. Builtin
		// destructors use the construction domain. Retain only that domain in an
		// independent call environment: retaining the full construction store can
		// keep an otherwise unreachable instance alive through its last binding.
		gilOwnerEnv := env
		switch destructor := delMethod.(type) {
		case *object.Function:
			gilOwnerEnv = destructor.Env
		case *object.LambdaFunction:
			gilOwnerEnv = destructor.Env
		}
		finalizerEnv := object.NewEnvironmentWithGILDomain(gilOwnerEnv)
		runtime.SetFinalizer(instance, func(inst *object.Instance) {
			// A finalizer runs on the runtime's own goroutine with no
			// caller to recover for it: a panic here is fatal to the host
			// process, so give the destructor its own boundary and a
			// bounded context. It still has no caller cancellation to
			// inherit and runs at the GC's whim, not the script's.
			defer func() { _ = recover() }()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			ctx = WithEvaluator(ctx)
			ctx = ContextWithCallDepth(ctx, DefaultMaxCallDepth)
			ApplyFunctionGIL(ctx, delMethod, []object.Object{inst}, nil, finalizerEnv)
		})
	}

	return instance
}

// applyUserFunctionDirect is a fast path for calling a 1-parameter function with
// a single argument, bypassing slice allocation and the generic params path.
func applyUserFunctionDirect(ctx context.Context, fn *object.Function, arg object.Object) object.Object {
	if cd := GetCallDepthFromContext(ctx); cd != nil {
		if !cd.Enter() {
			return errors.NewCallDepthExceededError(int(cd.max))
		}
		defer cd.Exit()
	}

	var extendedEnv *object.Environment
	if fn.ReuseCallEnv {
		extendedEnv = object.AcquireCallEnvironment(fn.Env, fn.LocalSlots, fn.LocalSlotNames)
	} else {
		extendedEnv = object.NewEnclosedEnvironmentWithSlots(fn.Env, fn.LocalSlots, fn.LocalSlotNames)
	}
	defer object.ReleaseCallEnvironment(extendedEnv)

	// Set the single argument directly into its slot
	if len(fn.ParamSlotIndexes) == 1 {
		extendedEnv.SetSlotByIndex(fn.ParamSlotIndexes[0], arg)
	} else {
		extendedEnv.Set(fn.Parameters[0].Value(), arg)
	}

	evaluated := functionBody(fn)(ctx, extendedEnv)
	if err, ok := evaluated.(*object.Error); ok {
		if err.Function == "" {
			err.Function = fn.Name
		}
	}
	return unwrapReturnValue(evaluated)
}

// applyUserFunction2 is a fast path for 2-parameter calls, avoiding slice allocation.
func applyUserFunction2(ctx context.Context, fn *object.Function, a0, a1 object.Object) object.Object {
	if cd := GetCallDepthFromContext(ctx); cd != nil {
		if !cd.Enter() {
			return errors.NewCallDepthExceededError(int(cd.max))
		}
		defer cd.Exit()
	}

	var extendedEnv *object.Environment
	if fn.ReuseCallEnv {
		extendedEnv = object.AcquireCallEnvironment(fn.Env, fn.LocalSlots, fn.LocalSlotNames)
	} else {
		extendedEnv = object.NewEnclosedEnvironmentWithSlots(fn.Env, fn.LocalSlots, fn.LocalSlotNames)
	}
	defer object.ReleaseCallEnvironment(extendedEnv)

	// Set arguments directly into slots
	if len(fn.ParamSlotIndexes) == 2 {
		extendedEnv.SetSlotByIndex(fn.ParamSlotIndexes[0], a0)
		extendedEnv.SetSlotByIndex(fn.ParamSlotIndexes[1], a1)
	} else {
		extendedEnv.Set(fn.Parameters[0].Value(), a0)
		extendedEnv.Set(fn.Parameters[1].Value(), a1)
	}

	evaluated := functionBody(fn)(ctx, extendedEnv)
	if err, ok := evaluated.(*object.Error); ok {
		if err.Function == "" {
			err.Function = fn.Name
		}
	}
	return unwrapReturnValue(evaluated)
}

// applyUserFunctionN is a fast path for N-parameter calls (N <= 3), using stack-allocated args.
func applyUserFunctionN(ctx context.Context, fn *object.Function, args ...object.Object) object.Object {
	if cd := GetCallDepthFromContext(ctx); cd != nil {
		if !cd.Enter() {
			return errors.NewCallDepthExceededError(int(cd.max))
		}
		defer cd.Exit()
	}

	var extendedEnv *object.Environment
	if fn.ReuseCallEnv {
		extendedEnv = object.AcquireCallEnvironment(fn.Env, fn.LocalSlots, fn.LocalSlotNames)
	} else {
		extendedEnv = object.NewEnclosedEnvironmentWithSlots(fn.Env, fn.LocalSlots, fn.LocalSlotNames)
	}
	defer object.ReleaseCallEnvironment(extendedEnv)

	// Set arguments directly into slots
	if len(fn.ParamSlotIndexes) == len(args) {
		for i, slotIdx := range fn.ParamSlotIndexes {
			extendedEnv.SetSlotByIndex(slotIdx, args[i])
		}
	} else {
		for i := range args {
			extendedEnv.Set(fn.Parameters[i].Value(), args[i])
		}
	}

	evaluated := functionBody(fn)(ctx, extendedEnv)
	if err, ok := evaluated.(*object.Error); ok {
		if err.Function == "" {
			err.Function = fn.Name
		}
	}
	return unwrapReturnValue(evaluated)
}

// functionBody returns the compiled body of fn, compiling it on the first
// call and caching the closure on the body's AST node, so closures exist only
// for functions that actually run. Function objects the compiler created also
// memoise the closure in CompiledBody, keeping the per-call cost to a field
// read; objects assembled elsewhere may be shared between independent
// interpreter trees, so they are read from the node cache and never written.
func functionBody(fn *object.Function) object.EvalFn {
	if fn.CompiledBody != nil {
		return fn.CompiledBody
	}
	body := cachedNode(&fn.Body.Compiled, fn.Body)
	if fn.CompilerOwned {
		fn.CompiledBody = body
	}
	return body
}

func applyUserFunction(ctx context.Context, fn *object.Function, args []object.Object, keywords map[string]object.Object, env *object.Environment) object.Object {
	// Check call depth to prevent stack overflow
	if cd := GetCallDepthFromContext(ctx); cd != nil {
		if !cd.Enter() {
			return errors.NewCallDepthExceededError(int(cd.max))
		}
		defer cd.Exit()
	}

	extendedEnv, err := extendFunctionEnv(ctx, fn, args, keywords)
	if err != nil {
		return err
	}
	defer object.ReleaseCallEnvironment(extendedEnv)

	evaluated := fixErrorPos(ctx, functionBody(fn)(ctx, extendedEnv), fn.Body.Line())
	if err, ok := evaluated.(*object.Error); ok {
		if err.Function == "" {
			err.Function = fn.Name
		}
	}
	return unwrapReturnValue(evaluated)
}

// applyFunction calls a function object with arguments and keyword arguments.
//
// It is unexported because it does NOT acquire the interpreter lock: it is the
// internal dispatch used while the lock is already held (dunder-method dispatch,
// recursion, etc.). The locking boundaries are EvalWithContext, ApplyFunctionGIL,
// and the evaliface adapter (CallFunction/CallObjectFunction/CallMethod). Go
// callers outside the evaluator package must use ApplyFunctionGIL.
//
// ApplyFunctionGIL acquires the environment's interpreter lock (reentrant via
// goroutine-id ownership, so nested calls from the same goroutine don't
// deadlock) and then dispatches via applyFunction. Host code that calls script
// functions — especially concurrently against a shared environment — must use
// this rather than the unexported applyFunction, which assumes the lock is held.
func ApplyFunctionGIL(ctx context.Context, fn object.Object, args []object.Object, keywords map[string]object.Object, env *object.Environment) object.Object {
	if env != nil {
		acquired, entered := env.EnterGILWithContext(ctx)
		if !entered {
			if err := checkContext(ctx); err != nil {
				return err
			}
			return errors.NewCancelledError()
		}
		if acquired {
			defer env.ExitGIL()
		}
	}
	return applyFunction(ctx, fn, args, keywords, env)
}

func applyFunction(ctx context.Context, fn object.Object, args []object.Object, keywords map[string]object.Object, env *object.Environment) object.Object {
	switch fn := fn.(type) {
	case *object.Function:
		return applyUserFunction(ctx, fn, args, keywords, env)
	case *object.LambdaFunction:
		return applyLambdaFunctionWithContext(ctx, fn, args, keywords, env)
	case *object.Builtin:
		ctxWithEnv := SetEnvInContext(ctx, env)
		return fn.Fn(ctxWithEnv, object.NewKwargs(keywords), args...)
	case *object.Class:
		return createInstance(ctx, fn, args, keywords, env)
	case *object.Instance:
		// Callable instances: obj(...) dispatches __call__ (Python
		// functors, strategies, partial application).
		if call, ok := fn.Class.Methods["__call__"]; ok {
			return applyFunctionWithContext(ctx, call, append([]object.Object{fn}, args...), keywords, env)
		}
		return &object.Exception{
			Message:       fmt.Sprintf("'%s' object is not callable", fn.Class.Name),
			ExceptionType: object.ExceptionTypeTypeError,
			Raised:        true,
		}
	case *object.ClientWrapper:
		if callable, ok := fn.Client.(object.ScriptCallable); ok {
			return callable.ScriptCall(ctx, args, keywords)
		}
		return errors.NewError("cannot call %s: not callable", fn.Inspect())
	default:
		return notCallableError(fn)
	}
}

func applyFunctionWithContext(ctx context.Context, fn object.Object, args []object.Object, keywords map[string]object.Object, env *object.Environment) object.Object {
	// Handle BoundMethod - prepend self to args
	if bm, ok := fn.(*object.BoundMethod); ok {
		if len(args) == 0 {
			return applyFunction(ctx, bm.Method, bm.SelfArgs(), keywords, env)
		}
		n := len(args) + 1
		var newArgs []object.Object
		if n <= 8 {
			var buf [8]object.Object
			buf[0] = bm.Instance
			copy(buf[1:], args)
			newArgs = buf[:n]
		} else {
			newArgs = make([]object.Object, n)
			newArgs[0] = bm.Instance
			copy(newArgs[1:], args)
		}
		return applyFunction(ctx, bm.Method, newArgs, keywords, env)
	}
	return applyFunction(ctx, fn, args, keywords, env)
}

func applyLambdaFunctionWithContext(ctx context.Context, fn *object.LambdaFunction, args []object.Object, keywords map[string]object.Object, env *object.Environment) object.Object {
	// Check call depth to prevent stack overflow
	if cd := GetCallDepthFromContext(ctx); cd != nil {
		if !cd.Enter() {
			return errors.NewCallDepthExceededError(int(cd.max))
		}
		defer cd.Exit()
	}

	extendedEnv, err := extendLambdaEnv(ctx, fn, args, keywords)
	if err != nil {
		return err
	}
	defer object.ReleaseCallEnvironment(extendedEnv)

	body := fn.CompiledBody
	if body == nil {
		// Only lambdas assembled outside the compiler lack a body closure. The
		// expression node has no cache slot, so compile it for this call
		// rather than write to a function object that may be shared.
		body = compileExpr(fn.Body)
	}
	return fixErrorPos(ctx, body(ctx, extendedEnv), fn.Body.Line()) // No unwrapping needed for lambda expressions
}

// evalDefault evaluates a parameter default in the defining scope, through the
// closure compiled at definition time when the function carries one.
// Functions assembled without compiled defaults (none in the evaluator today)
// compile the expression on the spot.
func (fp *funcParams) evalDefault(ctx context.Context, name string, defaultExpr ast.Expression) object.Object {
	if fn, ok := fp.compiledDefaults[name]; ok {
		return fn(ctx, fp.parentEnv)
	}
	return compileExpr(defaultExpr)(ctx, fp.parentEnv)
}

// funcParams abstracts the common parts of Function and LambdaFunction for parameter handling
type funcParams struct {
	parameters       []*ast.Identifier
	defaultValues    map[string]ast.Expression
	compiledDefaults map[string]object.EvalFn
	variadic         *ast.Identifier
	kwargs           *ast.Identifier
	keywordOnlyStart int
	parentEnv        *object.Environment
	localSlots       map[string]int
	localSlotNames   []string
	paramSlotIndexes []int
	reuseCallEnv     bool
}

// extendEnvWithParams handles the common logic for extending environments with function arguments
func extendEnvWithParams(ctx context.Context, fp funcParams, args []object.Object, keywords map[string]object.Object) (*object.Environment, object.Object) {
	var env *object.Environment
	if fp.reuseCallEnv {
		env = object.AcquireCallEnvironment(fp.parentEnv, fp.localSlots, fp.localSlotNames)
	} else {
		env = object.NewEnclosedEnvironmentWithSlots(fp.parentEnv, fp.localSlots, fp.localSlotNames)
	}

	numParams := len(fp.parameters)
	numArgs := len(args)
	positionalLimit := numParams
	if fp.keywordOnlyStart > 0 && fp.keywordOnlyStart-1 < positionalLimit {
		positionalLimit = fp.keywordOnlyStart - 1
	}

	// Fast path for the common case: exact positional arguments with no defaults,
	// variadics, kwargs, or keyword arguments.
	if len(keywords) == 0 && fp.keywordOnlyStart == 0 && fp.variadic == nil && fp.kwargs == nil && len(fp.defaultValues) == 0 && numArgs == numParams {
		if len(fp.paramSlotIndexes) == numParams {
			for paramIdx, slotIdx := range fp.paramSlotIndexes {
				if !env.SetSlotByIndex(slotIdx, args[paramIdx]) {
					env.Set(fp.parameters[paramIdx].Value(), args[paramIdx])
				}
			}
		} else {
			for paramIdx := 0; paramIdx < numParams; paramIdx++ {
				env.Set(fp.parameters[paramIdx].Value(), args[paramIdx])
			}
		}
		return env, nil
	}

	// Set provided positional arguments
	for paramIdx := 0; paramIdx < positionalLimit && paramIdx < numArgs; paramIdx++ {
		env.Set(fp.parameters[paramIdx].Value(), args[paramIdx])
	}

	// Check for extra positional arguments
	if numArgs > positionalLimit {
		if fp.variadic != nil {
			// Collect extra arguments into a list. Copy so the varargs list
			// doesn't alias the caller's args buffer (which may be reused).
			varArgs := make([]object.Object, numArgs-positionalLimit)
			copy(varArgs, args[positionalLimit:])
			list := &object.List{Elements: varArgs}
			env.Set(fp.variadic.Value(), list)
		} else {
			minArgs := positionalLimit
			return nil, errors.NewArgumentError(numArgs, minArgs)
		}
	} else if fp.variadic != nil {
		// No extra arguments, set variadic to empty list
		env.Set(fp.variadic.Value(), &object.List{Elements: []object.Object{}})
	}

	// Handle keyword arguments if present
	if len(keywords) > 0 {
		// Use a stack-allocated array for small param counts to track which params are set
		var setSmall [8]bool
		var setParams map[string]bool
		if numParams <= 8 {
			// Mark positional args as set via index
			for i := 0; i < positionalLimit && i < numArgs; i++ {
				setSmall[i] = true
			}
		} else {
			setParams = make(map[string]bool, numParams)
			for i := 0; i < positionalLimit && i < numArgs; i++ {
				setParams[fp.parameters[i].Value()] = true
			}
		}

		isParamSet := func(idx int, name string) bool {
			if numParams <= 8 {
				return setSmall[idx]
			}
			return setParams[name]
		}
		markParamSet := func(idx int, name string) {
			if numParams <= 8 {
				setSmall[idx] = true
			} else {
				setParams[name] = true
			}
		}

		var extraKwargs map[string]object.Object

		for key, value := range keywords {
			// Check if parameter exists
			paramIdx := -1
			for pi, param := range fp.parameters {
				if param.Value() == key {
					paramIdx = pi
					break
				}
			}

			if paramIdx == -1 {
				// If **kwargs is defined, collect extra keyword arguments
				if fp.kwargs != nil {
					if extraKwargs == nil {
						extraKwargs = make(map[string]object.Object, len(keywords))
					}
					extraKwargs[key] = value
					continue
				}
				return nil, errors.NewError("got an unexpected keyword argument '%s'", key)
			}

			if isParamSet(paramIdx, key) {
				return nil, errors.NewError("multiple values for argument '%s'", key)
			}

			env.Set(key, value)
			markParamSet(paramIdx, key)
		}

		// Set **kwargs dict if defined
		if fp.kwargs != nil {
			kwargsDict := &object.Dict{Pairs: make(map[string]object.DictPair, len(extraKwargs))}
			for key, value := range extraKwargs {
				kwargsDict.Pairs[object.DictKey(object.NewString(key))] = object.DictPair{
					Key:   object.NewString(key),
					Value: value,
				}
			}
			env.Set(fp.kwargs.Value(), kwargsDict)
		}

		// Check for missing arguments and apply defaults
		for pi, param := range fp.parameters {
			if !isParamSet(pi, param.Value()) {
				if defaultExpr, ok := fp.defaultValues[param.Value()]; ok {
					defaultVal := fp.evalDefault(ctx, param.Value(), defaultExpr)
					env.Set(param.Value(), defaultVal)
				} else {
					minArgs := numParams - len(fp.defaultValues)
					return nil, errors.NewArgumentError(numArgs, minArgs)
				}
			}
		}
	} else {
		// No keywords - set empty **kwargs dict if defined
		if fp.kwargs != nil {
			env.Set(fp.kwargs.Value(), &object.Dict{Pairs: make(map[string]object.DictPair)})
		}

		if numArgs < numParams {
			// No keywords - check for missing required arguments
			for i := numArgs; i < numParams; i++ {
				param := fp.parameters[i]
				if defaultExpr, ok := fp.defaultValues[param.Value()]; ok {
					defaultVal := fp.evalDefault(ctx, param.Value(), defaultExpr)
					env.Set(param.Value(), defaultVal)
				} else {
					minArgs := numParams - len(fp.defaultValues)
					return nil, errors.NewArgumentError(numArgs, minArgs)
				}
			}
		}
	}

	return env, nil
}

func extendFunctionEnv(ctx context.Context, fn *object.Function, args []object.Object, keywords map[string]object.Object) (*object.Environment, object.Object) {
	return extendEnvWithParams(ctx, funcParams{
		parameters:       fn.Parameters,
		defaultValues:    fn.DefaultValues,
		compiledDefaults: fn.CompiledDefaults,
		variadic:         fn.Variadic,
		kwargs:           fn.Kwargs,
		keywordOnlyStart: fn.KeywordOnlyStart,
		parentEnv:        fn.Env,
		localSlots:       fn.LocalSlots,
		localSlotNames:   fn.LocalSlotNames,
		paramSlotIndexes: fn.ParamSlotIndexes,
		reuseCallEnv:     fn.ReuseCallEnv,
	}, args, keywords)
}

func extendLambdaEnv(ctx context.Context, fn *object.LambdaFunction, args []object.Object, keywords map[string]object.Object) (*object.Environment, object.Object) {
	return extendEnvWithParams(ctx, funcParams{
		parameters:       fn.Parameters,
		defaultValues:    fn.DefaultValues,
		compiledDefaults: fn.CompiledDefaults,
		variadic:         fn.Variadic,
		kwargs:           fn.Kwargs,
		keywordOnlyStart: fn.KeywordOnlyStart,
		parentEnv:        fn.Env,
		localSlots:       fn.LocalSlots,
		localSlotNames:   fn.LocalSlotNames,
		paramSlotIndexes: fn.ParamSlotIndexes,
		reuseCallEnv:     true,
	}, args, keywords)
}

func analyzeFunctionLocals(stmt *ast.FunctionStatement) (map[string]int, []string) {
	if stmt.Function.LocalSlots != nil {
		return stmt.Function.LocalSlots, stmt.Function.LocalSlotNames
	}
	return nil, nil
}

// analyzeTopLevelLocals finds all assigned variables in a top-level program
// and returns slot index mapping and ordered names.
func analyzeTopLevelLocals(program *ast.Program) (map[string]int, []string) {
	return program.LocalSlots, program.LocalSlotNames
}

func analyzeLambdaLocals(lambda *ast.Lambda) (map[string]int, []string) {
	return lambda.LocalSlots, lambda.LocalSlotNames
}

func unwrapReturnValue(obj object.Object) object.Object {
	if returnValue, ok := obj.(*object.ReturnValue); ok {
		val := returnValue.Value
		releaseReturnValue(returnValue)
		return val
	}
	return obj
}

func isTruthy(obj object.Object) bool {
	switch obj {
	case NULL:
		return false
	case TRUE:
		return true
	case FALSE:
		return false
	default:
		// Check for Python-style falsy values
		switch v := obj.(type) {
		case *object.Boolean:
			return v.BoolValue()
		case *object.Integer:
			return v.IntValue() != 0
		case *object.Float:
			return v.FloatValue() != 0.0
		case *object.String:
			return v.StringValue() != ""
		case *object.Bytes:
			return v.Len() > 0
		case *object.List:
			return len(v.Elements) > 0
		case *object.Tuple:
			return len(v.Elements) > 0
		case *object.Dict:
			return len(v.Pairs) > 0
		case *object.Set:
			return len(v.Elements) > 0
		case *object.DictKeys:
			return len(v.Dict.Pairs) > 0
		case *object.DictValues:
			return len(v.Dict.Pairs) > 0
		case *object.DictItems:
			return len(v.Dict.Pairs) > 0
		case *object.FloatArray:
			return len(v.Data) > 0
		case *object.Instance:
			// Try __bool__ first, then __len__ via the function variable (avoids init cycle)
			if isTruthyInstanceFn != nil {
				return isTruthyInstanceFn(v)
			}
			return true
		default:
			return true
		}
	}
}

// isTruthyInstanceFn is set in init() to break the initialization cycle
var isTruthyInstanceFn func(inst *object.Instance) bool

// evalTruthyFn is a package-level indirection to evalTruthy, set in init() so
// that builtins (any/all/bool/filter/map) defined in the package-level map can
// reference it without creating a static initialization cycle.
var evalTruthyFn func(ctx context.Context, obj object.Object, env *object.Environment) (bool, object.Object)

// renderConvertedValue applies a Python !r/!s/!a conversion to a format
// value: "r" and "a" use repr semantics (instances dispatch __repr__ then
// __str__; exceptions show their full Inspect), "s" uses str semantics
// (exceptions show their message, instances dispatch __str__). A raise from a
// dunder propagates as the second return.
func renderConvertedValue(ctx context.Context, val object.Object, conv string, env *object.Environment) (string, object.Object) {
	switch conv {
	case "r", "a":
		switch v := val.(type) {
		case *object.Instance:
			return reprInstanceChecked(ctx, v, env)
		case *object.String:
			return object.ReprString(v.StringValue()), nil
		case *object.Exception:
			return object.ReprException(v), nil
		}
	default: // "s" or none: str semantics
		switch v := val.(type) {
		case *object.Exception:
			return v.Message, nil
		case *object.Instance:
			return strInstanceChecked(ctx, v, env)
		}
	}
	// A container renders its elements with repr in both cases, as Python
	// does: str(["a"]) is "['a']" and nested instances use __repr__.
	if object.IsReprContainer(val) {
		return containerReprChecked(ctx, val, env)
	}
	return val.Inspect(), nil
}

// isPlainFormattable reports whether val takes a format spec directly.
func isPlainFormattable(val object.Object) bool {
	switch val.(type) {
	case *object.String, *object.Integer, *object.Float, *object.Boolean:
		return true
	}
	return false
}

// formatValueChecked applies a format spec to a value as Python's format()
// does. Numbers and strings take the spec directly; an instance uses its
// __format__ when it has one; anything else accepts only an empty spec (and
// renders with str semantics), raising TypeError for a non-empty one.
func formatValueChecked(ctx context.Context, val object.Object, spec string, env *object.Environment) (string, object.Object) {
	if isPlainFormattable(val) {
		return formatWithSpec(val, spec), nil
	}
	switch v := val.(type) {
	case *object.Instance:
		if _, ok := v.Class.LookupMember("__format__"); ok {
			result := callDunderMethodFn(ctx, v, "__format__", []object.Object{object.NewString(spec)}, env)
			if propagates(result) {
				return "", result
			}
			if s, ok := result.(*object.String); ok {
				return s.StringValue(), nil
			}
			return "", &object.Exception{
				Message:       fmt.Sprintf("__format__ must return a str, not %s", getTypeName(result)),
				ExceptionType: object.ExceptionTypeTypeError,
				Raised:        true,
			}
		}
	}
	if spec != "" {
		return "", &object.Exception{
			Message:       fmt.Sprintf("unsupported format string passed to %s.__format__", getTypeName(val)),
			ExceptionType: object.ExceptionTypeTypeError,
			Raised:        true,
		}
	}
	return renderConvertedValue(ctx, val, "s", env)
}

// reprInstanceChecked renders an instance with repr() semantics: __repr__,
// else the default <Class object at 0x...>. Unlike str(), __str__ is never
// used. A raise from __repr__ is returned.
func reprInstanceChecked(ctx context.Context, inst *object.Instance, env *object.Environment) (string, object.Object) {
	if result := callDunderMethodFn(ctx, inst, "__repr__", nil, env); result != nil {
		if propagates(result) {
			return "", result
		}
		if s, ok := result.(*object.String); ok {
			return s.StringValue(), nil
		}
		return "", &object.Exception{
			Message:       fmt.Sprintf("__repr__ returned non-string (type %s)", getTypeName(result)),
			ExceptionType: object.ExceptionTypeTypeError,
			Raised:        true,
		}
	}
	return inst.Inspect(), nil
}

// containerReprChecked renders a list, tuple, dict, set or dict view as
// Python does, calling __repr__ on nested instances.
func containerReprChecked(ctx context.Context, val object.Object, env *object.Environment) (string, object.Object) {
	return object.InspectRepr(val, func(inst *object.Instance) (string, object.Object) {
		return reprInstanceChecked(ctx, inst, env)
	})
}

// strInstanceChecked renders an instance with str() semantics: __str__ first,
// falling back to __repr__ (Python's object.__str__ delegates to __repr__),
// then the default representation. A raise from either dunder propagates.
func strInstanceChecked(ctx context.Context, inst *object.Instance, env *object.Environment) (string, object.Object) {
	for _, name := range []string{"__str__", "__repr__"} {
		if result := callDunderMethodFn(ctx, inst, name, nil, env); result != nil {
			if propagates(result) {
				return "", result
			}
			if s, ok := result.(*object.String); ok {
				return s.StringValue(), nil
			}
		}
	}
	return inst.Inspect(), nil
}

// expandNestedSpecFields resolves nested replacement fields inside a format
// spec (dynamic widths like "{x:>{w}}"): each {identifier} is replaced by the
// value of that name in env, rendered via str semantics. Nested fields are
// the common plain-variable form; anything else is left untouched for
// formatWithSpec to render literally.
func expandNestedSpecFields(ctx context.Context, spec string, env *object.Environment) (string, object.Object) {
	if !strings.Contains(spec, "{") {
		return spec, nil
	}
	var b strings.Builder
	for i := 0; i < len(spec); {
		c := spec[i]
		if c != '{' {
			b.WriteByte(c)
			i++
			continue
		}
		end := strings.IndexByte(spec[i:], '}')
		if end < 0 {
			b.WriteByte(c)
			i++
			continue
		}
		name := spec[i+1 : i+end]
		i += end + 1
		if name == "" {
			return "", errors.NewError("empty nested format field")
		}
		val, ok := env.Get(name)
		if !ok {
			return "", errors.NewError("nested format field '%s' is not defined", name)
		}
		rendered, rerr := renderConvertedValue(ctx, val, "s", env)
		if rerr != nil {
			return "", rerr
		}
		b.WriteString(rendered)
	}
	return b.String(), nil
}

// evalTruthy evaluates the truthiness of obj, propagating any exception raised
// (or internal error produced) by a user-defined __bool__ or __len__. The
// second return value is non-nil exactly when such a raise/error occurred, in
// which case the bool is meaningless and the caller must return the object.
// Non-instance values (and instances without __bool__/__len__) never raise, so
// the fast isTruthy path is reused for them.
func evalTruthy(ctx context.Context, obj object.Object, env *object.Environment) (bool, object.Object) {
	inst, ok := obj.(*object.Instance)
	if !ok {
		return isTruthy(obj), nil
	}
	if fn, ok := findDunderMethod(inst, "__bool__"); ok {
		result := applyFunctionWithContext(ctx, fn, prependSelf(inst, nil), nil, inst.Class.Env)
		if propagates(result) {
			return false, result
		}
		if b, ok := result.(*object.Boolean); ok {
			return b.BoolValue(), nil
		}
	}
	if fn, ok := findDunderMethod(inst, "__len__"); ok {
		result := applyFunctionWithContext(ctx, fn, prependSelf(inst, nil), nil, inst.Class.Env)
		if propagates(result) {
			return false, result
		}
		if i, ok := result.(*object.Integer); ok {
			return i.IntValue() != 0, nil
		}
	}
	return true, nil
}

// findDunderMethod looks up a dunder method in the instance's class hierarchy
func findDunderMethod(inst *object.Instance, method string) (object.Object, bool) {
	return inst.Class.LookupMember(method)
}

func init() {
	evalTruthyFn = evalTruthy
	iterableToSliceCheckedFn = iterableToSliceChecked
	acceptInstanceIterableFn = acceptInstanceIterable
	isTruthyInstanceFn = func(inst *object.Instance) bool {
		if fn, ok := findDunderMethod(inst, "__bool__"); ok {
			result := applyFunctionWithContext(context.Background(), fn, prependSelf(inst, nil), nil, inst.Class.Env)
			if b, ok := result.(*object.Boolean); ok {
				return b.BoolValue()
			}
		}
		if fn, ok := findDunderMethod(inst, "__len__"); ok {
			result := applyFunctionWithContext(context.Background(), fn, prependSelf(inst, nil), nil, inst.Class.Env)
			if i, ok := result.(*object.Integer); ok {
				return i.IntValue() != 0
			}
		}
		return true
	}
}

// evalDictLiteralWithContext is in data_structures.go
// evalIndexExpression is in data_structures.go
// evalDictMemberAccess is in data_structures.go
// evalListIndexExpression is in data_structures.go
// evalTupleIndexExpression is in data_structures.go
// evalDictIndexExpression is in data_structures.go
// evalStringIndexExpression is in data_structures.go
// evalRegexIndexExpression is in data_structures.go

// evalSliceExpressionWithContext is in data_structures.go
// sliceList is in data_structures.go
// sliceString is in data_structures.go

// raisedExceptionCarrier is implemented by an import error that wraps an
// exception raised by a module's top-level code, so the import site can
// re-raise the original exception with its type intact instead of flattening it
// into an ImportError. The host's module loader implements this.
type raisedExceptionCarrier interface {
	RaisedException() *object.Exception
}

// importErrorToObject turns an import-callback error into the object to
// propagate: if the module raised an exception at import time, re-raise that
// original exception (Python semantics); otherwise report an ImportError with
// the given prefix message.
func importErrorToObject(err error, prefix string) object.Object {
	if carrier, ok := err.(raisedExceptionCarrier); ok {
		exc := carrier.RaisedException()
		exc.Raised = true
		return exc
	}
	return errors.NewError("%s: %s", prefix, err.Error())
}

func evalImportStatement(ctx context.Context, is *ast.ImportStatement, env *object.Environment) object.Object {
	importCallback := env.GetImportCallbackWithContext()
	if importCallback == nil {
		return errors.NewError("%s at line %d", errors.ErrImportError, is.Token.Line)
	}
	err := importCallback(ctx, is.Name.Value())
	if err != nil {
		return importErrorToObject(err, fmt.Sprintf("%s at line %d", errors.ErrImportError, is.Token.Line))
	}

	// Handle alias if present
	if is.GetAlias() != nil {
		moduleObj := getModuleByPath(env, is.Name.Value())
		if moduleObj != nil {
			env.Set(is.GetAlias().Value(), moduleObj)
			if _, ok := moduleObj.(*object.Dict); ok {
				env.MarkImportedBinding(is.GetAlias().Value())
			}
		}
	}

	for i, name := range is.GetAdditionalNames() {
		if err := importCallback(ctx, name.Value()); err != nil {
			return importErrorToObject(err, errors.ErrImportError)
		}

		if i < len(is.GetAdditionalAliases()) && is.GetAdditionalAliases()[i] != nil {
			moduleObj := getModuleByPath(env, name.Value())
			if moduleObj != nil {
				env.Set(is.GetAdditionalAliases()[i].Value(), moduleObj)
				if _, ok := moduleObj.(*object.Dict); ok {
					env.MarkImportedBinding(is.GetAdditionalAliases()[i].Value())
				}
			}
		}
	}

	return NULL
}

// getModuleByPath gets a module from the environment, handling dotted paths
func getModuleByPath(env *object.Environment, name string) object.Object {
	// First try direct lookup
	if obj, ok := env.Get(name); ok {
		return obj
	}

	// For dotted paths, navigate from the root
	parts := strings.Split(name, ".")
	if len(parts) < 2 {
		return nil
	}

	// Get the root module
	rootObj, ok := env.Get(parts[0])
	if !ok {
		return nil
	}

	// Navigate through the path
	current := rootObj
	for i := 1; i < len(parts); i++ {
		dict, ok := current.(*object.Dict)
		if !ok {
			return nil
		}
		pair, ok := dict.GetByString(parts[i])
		if !ok {
			return nil
		}
		current = pair.Value
	}

	return current
}

func evalFromImportStatement(ctx context.Context, fis *ast.FromImportStatement, env *object.Environment) object.Object {
	importCallback := env.GetImportCallbackWithContext()
	if importCallback == nil {
		return errors.NewError(errors.ErrImportError)
	}

	// Resolve the base module name, handling relative imports
	var baseModuleName string
	if fis.RelativeLevel > 0 {
		// Relative import: resolve based on current module
		currentModule := env.GetCurrentModule()
		if currentModule == "" {
			return errors.NewError("%s: relative import outside of module context", errors.ErrImportError)
		}

		// Split current module path into parts
		parts := strings.Split(currentModule, ".")

		// Calculate how many levels we can go up
		// e.g., currentModule = "a.b.c", relativeLevel = 1 -> go up 1 level -> "a.b"
		// e.g., currentModule = "a.b.c", relativeLevel = 2 -> go up 2 levels -> "a"
		// e.g., currentModule = "a.b.c", relativeLevel = 3 -> error (can't go beyond root)
		if fis.RelativeLevel > len(parts) {
			return errors.NewError("%s: relative import level %d exceeds module depth for '%s'", errors.ErrImportError, fis.RelativeLevel, currentModule)
		}

		// Strip the appropriate number of levels from the current module
		resolvedParts := parts[:len(parts)-fis.RelativeLevel]

		// Build the resolved base module name
		if fis.Module != nil {
			// from .module import X or from ..module import X
			baseModuleName = strings.Join(resolvedParts, ".") + "." + fis.Module.Value()
		} else {
			// from . import X or from .. import X (no additional module)
			// In this case, each name to import is a submodule of the parent
			baseModuleName = strings.Join(resolvedParts, ".")
			// If empty after stripping, it means we're at the package root - this is an error
			if baseModuleName == "" {
				return errors.NewError("%s: relative import at package root has no target", errors.ErrImportError)
			}
		}
	} else {
		// Absolute import
		if fis.Module == nil {
			return errors.NewError("%s: missing module name in from-import", errors.ErrImportError)
		}
		baseModuleName = fis.Module.Value()
	}

	// For "from . import X" (no module specified), we need to import each name as a submodule
	// For "from .module import X" (module specified), we import the module once
	if fis.Module == nil && fis.RelativeLevel > 0 {
		// "from . import X, Y" - each name is a submodule to import
		return evalFromImportMultipleSubmodules(ctx, fis, baseModuleName, env, importCallback)
	}

	// Standard from-import: import the module and extract names
	return evalFromImportStandard(ctx, fis, baseModuleName, env, importCallback)
}

// evalFromImportMultipleSubmodules handles "from . import X, Y" where each name is a submodule
func evalFromImportMultipleSubmodules(ctx context.Context, fis *ast.FromImportStatement, baseModule string, env *object.Environment, importCallback func(context.Context, string) error) object.Object {
	for i, name := range fis.Names {
		// Build the full module name: base + "." + name
		fullModuleName := baseModule + "." + name.Value()

		// Import the submodule
		err := importCallback(ctx, fullModuleName)
		if err != nil {
			return importErrorToObject(err, errors.ErrImportError)
		}

		// Get the imported submodule
		moduleObj, ok := env.Get(fullModuleName)
		if !ok {
			// Try getting from parent module
			parentObj, parentOk := env.Get(baseModule)
			if parentOk {
				switch p := parentObj.(type) {
				case *object.Dict:
					if pair, exists := p.GetByString(name.Value()); exists {
						moduleObj = pair.Value
						ok = true
					}
				}
			}
			if !ok {
				return errors.NewError("%s: cannot import name '%s' from '%s'", errors.ErrImportError, name.Value(), baseModule)
			}
		}

		// Use alias if provided, otherwise use the original name
		bindName := name.Value()
		if fis.Aliases[i] != nil {
			bindName = fis.Aliases[i].Value()
		}

		env.Set(bindName, moduleObj)
		if _, ok := moduleObj.(*object.Dict); ok {
			env.MarkImportedBinding(bindName)
		}
	}

	return NULL
}

// evalFromImportStandard handles standard "from module import X, Y"
func evalFromImportStandard(ctx context.Context, fis *ast.FromImportStatement, moduleName string, env *object.Environment, importCallback func(context.Context, string) error) object.Object {
	// Check if module was already in the environment before importing
	// (e.g. user did `import json` before `from json import dumps`)
	_, wasPresent := env.Get(moduleName)

	// Import the module
	err := importCallback(ctx, moduleName)
	if err != nil {
		return importErrorToObject(err, errors.ErrImportError)
	}

	// Get the imported module from the environment
	moduleObj, ok := env.Get(moduleName)
	if !ok {
		// Try getting just the first part for dotted imports
		parts := strings.Split(moduleName, ".")
		moduleObj, ok = env.Get(parts[0])
		if !ok {
			return errors.NewError("%s: module '%s' not found after import", errors.ErrImportError, moduleName)
		}
		// Navigate to the sub-module
		for i := 1; i < len(parts); i++ {
			switch m := moduleObj.(type) {
			case *object.Dict:
				if pair, exists := m.GetByString(parts[i]); exists {
					moduleObj = pair.Value
				} else {
					return errors.NewError("%s: cannot find '%s' in module '%s'", errors.ErrImportError, parts[i], strings.Join(parts[:i], "."))
				}
			default:
				return errors.NewError("%s: '%s' is not a module", errors.ErrImportError, strings.Join(parts[:i], "."))
			}
		}
	}

	// Now extract the requested names from the module and bind them
	for i, name := range fis.Names {
		var value object.Object
		var found bool

		switch m := moduleObj.(type) {
		case *object.Dict:
			if pair, exists := m.GetByString(name.Value()); exists {
				value = pair.Value
				found = true
			}
		case *object.Library:
			// Check functions first
			if funcs := m.Functions(); funcs != nil {
				if fn, exists := funcs[name.Value()]; exists {
					value = fn
					found = true
				}
			}
			// Check constants
			if !found {
				if consts := m.Constants(); consts != nil {
					if c, exists := consts[name.Value()]; exists {
						value = c
						found = true
					}
				}
			}
		case *object.Instance:
			if field, exists := m.GetField(name.Value()); exists {
				value = field
				found = true
			}
		}

		if !found {
			return errors.NewError("%s: cannot import name '%s' from '%s'", errors.ErrImportError, name.Value(), moduleName)
		}

		// Use alias if provided, otherwise use the original name
		bindName := name.Value()
		if fis.Aliases[i] != nil {
			bindName = fis.Aliases[i].Value()
		}

		env.Set(bindName, value)
		if _, ok := value.(*object.Dict); ok {
			env.MarkImportedBinding(bindName)
		}
	}

	// Remove the module from the environment (from X import Y should not make X available)
	// But only if the module was NOT already in the environment before this from-import.
	// This preserves modules that were explicitly imported (e.g. `import json` before `from json import dumps`).
	// For dotted imports (e.g. from a.b.c import X), only the full dotted name is deleted.
	// The root module (parts[0]) is NOT deleted as it may be needed by other imports.
	// IMPORTANT: Don't delete if one of the imported names (or aliases) matches the module name,
	// as that would delete the just-imported value (e.g. `from datetime import datetime`).
	if !wasPresent {
		shouldDelete := true
		for i, name := range fis.Names {
			bindName := name.Value()
			if fis.Aliases[i] != nil {
				bindName = fis.Aliases[i].Value()
			}
			if bindName == moduleName {
				shouldDelete = false
				break
			}
		}
		if shouldDelete {
			env.Delete(moduleName)
		}
	}

	return NULL
}

func evalInOperator(ctx context.Context, left, right object.Object, env *object.Environment) object.Object {
	switch container := right.(type) {
	case *object.List:
		for _, elem := range container.Elements {
			if left == elem {
				return TRUE
			}
			// Instances compare via __eq__ (a raising __eq__ propagates);
			// other types keep deep structural equality.
			if isInstanceOperand(left) || isInstanceOperand(elem) {
				eq, rerr := evalObjectsEqualChecked(ctx, left, elem, env)
				if rerr != nil {
					return rerr
				}
				if eq {
					return TRUE
				}
				continue
			}
			if objectsDeepEqual(left, elem) {
				return TRUE
			}
		}
		return FALSE
	case *object.Tuple:
		for _, elem := range container.Elements {
			if left == elem {
				return TRUE
			}
			if isInstanceOperand(left) || isInstanceOperand(elem) {
				eq, rerr := evalObjectsEqualChecked(ctx, left, elem, env)
				if rerr != nil {
					return rerr
				}
				if eq {
					return TRUE
				}
				continue
			}
			if objectsDeepEqual(left, elem) {
				return TRUE
			}
		}
		return FALSE
	case *object.Dict:
		key, rerr := evalHashKeyChecked(ctx, left)
		if rerr != nil {
			return rerr
		}
		_, ok := container.Pairs[key]
		return nativeBoolToBooleanObject(ok)
	case *object.String:
		if needle, ok := left.(*object.String); ok {
			return nativeBoolToBooleanObject(strings.Contains(container.StringValue(), needle.StringValue()))
		}
		return errors.NewTypeError("STRING", "non-string type")
	case *object.Bytes:
		// An integer in 0-255 checks for a byte value; a Bytes needle does
		// substring containment. Anything else is a type error.
		switch needle := left.(type) {
		case *object.Integer:
			target := byte(needle.IntValue())
			for _, b := range container.BytesValue() {
				if b == target {
					return TRUE
				}
			}
			return FALSE
		case *object.Bytes:
			return nativeBoolToBooleanObject(bytes.Contains(container.BytesValue(), needle.BytesValue()))
		}
		return errors.NewTypeError("INTEGER or BYTES", left.Type().String())
	case *object.DictKeys:
		key, rerr := evalHashKeyChecked(ctx, left)
		if rerr != nil {
			return rerr
		}
		_, ok := container.Dict.Pairs[key]
		return nativeBoolToBooleanObject(ok)
	case *object.DictValues:
		for _, pair := range container.Dict.Pairs {
			if left == pair.Value {
				return TRUE
			}
			if isInstanceOperand(left) || isInstanceOperand(pair.Value) {
				eq, rerr := evalObjectsEqualChecked(ctx, left, pair.Value, env)
				if rerr != nil {
					return rerr
				}
				if eq {
					return TRUE
				}
				continue
			}
			if objectsDeepEqual(left, pair.Value) {
				return TRUE
			}
		}
		return FALSE
	case *object.DictItems:
		var key, val object.Object
		switch l := left.(type) {
		case *object.Tuple:
			if len(l.Elements) == 2 {
				key, val = l.Elements[0], l.Elements[1]
			}
		case *object.List:
			if len(l.Elements) == 2 {
				key, val = l.Elements[0], l.Elements[1]
			}
		}
		if key != nil {
			keyStr, rerr := evalHashKeyChecked(ctx, key)
			if rerr != nil {
				return rerr
			}
			if pair, ok := container.Dict.Pairs[keyStr]; ok {
				if val == pair.Value {
					return TRUE
				}
				if isInstanceOperand(val) || isInstanceOperand(pair.Value) {
					eq, rerr := evalObjectsEqualChecked(ctx, val, pair.Value, env)
					if rerr != nil {
						return rerr
					}
					if eq {
						return TRUE
					}
				} else if objectsDeepEqual(val, pair.Value) {
					return TRUE
				}
			}
		}
		return FALSE
	case *object.Set:
		key, rerr := evalHashKeyChecked(ctx, left)
		if rerr != nil {
			return rerr
		}
		return nativeBoolToBooleanObject(container.ContainsKeyed(key))
	case *object.FloatArray:
		if f, err := left.AsFloat(); err == nil {
			for _, v := range container.Data {
				if v == f {
					return TRUE
				}
			}
		}
		return FALSE
	case *object.Instance:
		if fn, ok := findDunderMethod(container, "__contains__"); ok {
			result := applyFunctionWithContext(ctx, fn, prependSelf(container, []object.Object{left}), nil, container.Class.Env)
			if propagates(result) {
				return result
			}
			return nativeBoolToBooleanObject(isTruthy(result))
		}
		return errors.NewTypeError("iterable", right.Type().String())
	default:
		return errors.NewTypeError("iterable", right.Type().String())
	}
}

// evalIsOperator checks if two objects are the same instance (identity comparison)
func evalIsOperator(left, right object.Object) object.Object {
	// Special handling for None - there's only one None
	if left == NULL && right == NULL {
		return TRUE
	}
	if left == NULL || right == NULL {
		return FALSE
	}

	// Special handling for boolean singletons
	if left == TRUE && right == TRUE {
		return TRUE
	}
	if left == FALSE && right == FALSE {
		return TRUE
	}

	// Booleans: compare by value (like Python, True is always True)
	if l, ok := left.(*object.Boolean); ok {
		if r, ok := right.(*object.Boolean); ok {
			return nativeBoolToBooleanObject(l.BoolValue() == r.BoolValue())
		}
		return FALSE
	}

	// For immutable types like small integers and strings, Python caches them
	// so we check both pointer equality and value equality for these types
	switch l := left.(type) {
	case *object.Integer:
		if r, ok := right.(*object.Integer); ok {
			// Python caches small integers (-5 to 256)
			if l.IntValue() >= -5 && l.IntValue() <= 256 && l.IntValue() == r.IntValue() {
				return TRUE
			}
			// Otherwise check pointer equality
			return nativeBoolToBooleanObject(left == right)
		}
		return FALSE
	case *object.String:
		if r, ok := right.(*object.String); ok {
			// Python interns short strings
			if len(l.StringValue()) <= 20 && l.StringValue() == r.StringValue() {
				return TRUE
			}
			return nativeBoolToBooleanObject(left == right)
		}
		return FALSE
	}

	// For other types, check pointer equality
	return nativeBoolToBooleanObject(left == right)
}

// applyFinallyResult folds a finally block's outcome into the try statement's
// pending result. A return still overrides (kept as a ReturnValue marker so a
// later statement cannot replace it); an exception raised in finally replaces
// the in-flight one (Python semantics); break and continue propagate rather
// than being silently dropped. A finally that ends normally changes nothing.
func applyFinallyResult(result, finallyResult object.Object) object.Object {
	if finallyResult == nil {
		return result
	}
	switch finallyResult.(type) {
	case *object.ReturnValue, *object.Exception, *object.Error, *object.Break, *object.Continue:
		return finallyResult
	default:
		return result
	}
}

// applyProtectedFinallyResult runs cleanup for exceptions that must not be
// caught or replaced: SystemExit and PermissionError. Whatever the finally
// block does, the protected exception keeps propagating — letting a raise in
// finally replace it would give a handler the chance to swallow an exit or a
// security refusal.
func applyProtectedFinallyResult(result, finallyResult object.Object) object.Object {
	return result
}

// exceptionConstructorNames lists the builtin exception constructors accepted
// by the bare `raise Name` form (Python instantiates the class with no
// arguments when the raise operand is not already an exception instance).
var exceptionConstructorNames = []string{
	"Exception", "ValueError", "TypeError", "NameError", "ImportError",
	"StopIteration", "RuntimeError", "ZeroDivisionError", "IndexError",
	"KeyError", "AttributeError", "OSError",
}

// isExceptionConstructorName reports whether name is a builtin exception
// constructor accepted by the bare `raise Name` form.
func isExceptionConstructorName(name string) bool {
	for _, n := range exceptionConstructorNames {
		if n == name {
			return true
		}
	}
	return false
}

// isRaised reports whether obj is an exception that is actively propagating and
// must unwind the call stack. A constructed or caught exception value
// (ex.Raised == false) is NOT raised — it is an ordinary value. Internal
// *object.Error values always propagate and are handled separately by
// object.IsError. Use isRaised at expression-level guards (call arguments,
// operands, list/tuple/dict elements) where a bare exception may legitimately
// be a value rather than a propagating raise.
func isRaised(obj object.Object) bool {
	ex, ok := obj.(*object.Exception)
	return ok && ex.Raised
}

// propagates reports whether obj must be handed straight back to the caller:
// an internal error or a raised exception.
func propagates(obj object.Object) bool {
	return object.IsError(obj) || isRaised(obj)
}

// markRaised flags an exception as actively propagating and returns it. Safe to
// call on any object; non-exceptions are returned unchanged.
func markRaised(obj object.Object) object.Object {
	if ex, ok := obj.(*object.Exception); ok {
		ex.Raised = true
	}
	return obj
}

// isPropagatedError reports whether the slice returned by evalCallArgs /
// evalExpressionsWithContext is the one-element sentinel those helpers return
// when evaluating a sub-expression produced a propagating value: an internal
// *object.Error or a raised *object.Exception. Callers use it to propagate that
// value instead of treating it as a real argument/element.
func isPropagatedError(vals []object.Object) bool {
	return len(vals) == 1 && (object.IsError(vals[0]) || isRaised(vals[0]))
}

// matchesExceptionType checks if an exception matches the specified exception type
// Supports: Exception (catches all), specific types (ValueError, TypeError, etc.),
// and dotted names (requests.HTTPError)
// errorExceptionType determines which Python exception type an *object.Error
// should be matched as by an except clause.
//
// Errors tagged at the point they were raised (see errors.NewZeroDivisionError)
// carry their type explicitly and are used as-is. Everything else falls back to
// inferring a type from the message text. That inference is a legacy heuristic:
// it only exists for errors that have not been tagged yet, and the right way to
// classify a new error is to set Error.ExceptionType where it is created rather
// than to add another pattern here. Anything unclassified is reported as a plain
// Exception, which `except Exception:` still catches.
func errorExceptionType(err *object.Error) string {
	if err.ExceptionType != "" {
		return err.ExceptionType
	}
	msg := err.Message
	switch {
	case strings.HasPrefix(msg, "type error:"), strings.Contains(msg, "type mismatch"):
		return object.ExceptionTypeTypeError
	case strings.Contains(msg, "value error"), strings.Contains(msg, "invalid value"):
		return object.ExceptionTypeValueError
	case strings.Contains(msg, "identifier not found"),
		strings.Contains(msg, "name") && strings.Contains(msg, "not defined"):
		return object.ExceptionTypeNameError
	case strings.HasPrefix(msg, errors.ErrImportError):
		return object.ExceptionTypeImportError
	}
	return object.ExceptionTypeException
}

func matchesExceptionType(exception object.Object, exceptTypeExpr ast.Expression, env *object.Environment) bool {
	// Get the exception type string
	var exceptionType string
	if exc, ok := exception.(*object.Exception); ok {
		exceptionType = exc.ExceptionType
		if exceptionType == "" {
			exceptionType = "Exception" // Default to Exception if not set
		}
	} else if _, ok := exception.(*object.Error); ok {
		// Errors are treated as generic exceptions
		exceptionType = "Exception"
	} else {
		return false
	}

	return matchesExceptionTypeExpr(exceptionType, exceptTypeExpr)
}

// evalExceptTypeSideEffects evaluates the parts of an except-type expression
// that are not plain names or dotted names (which are matched structurally),
// so that a raise from e.g. `except (boom(), ValueError):` propagates. Returns
// the raised exception / internal error if one occurred, otherwise nil.
func evalExceptTypeSideEffects(ctx context.Context, expr ast.Expression, env *object.Environment) object.Object {
	switch e := expr.(type) {
	case *ast.Identifier:
		return nil // a bare name is matched structurally, not evaluated
	case *ast.IndexExpression:
		return nil // dotted name (requests.HTTPError) matched structurally
	case *ast.TupleLiteral:
		for _, elem := range e.Elements {
			if raised := evalExceptTypeSideEffects(ctx, elem, env); raised != nil {
				return raised
			}
		}
		return nil
	case *ast.ListLiteral:
		for _, elem := range e.Elements {
			if raised := evalExceptTypeSideEffects(ctx, elem, env); raised != nil {
				return raised
			}
		}
		return nil
	default:
		// Any other expression (a call, operator, etc.) is evaluated for its
		// side effects; only a raise or internal error is propagated. Such
		// except types are rare, so the expression is compiled on the spot.
		result := compileExpr(expr)(ctx, env)
		if propagates(result) {
			return result
		}
		return nil
	}
}

func matchesExceptionTypeExpr(exceptionType string, exceptTypeExpr ast.Expression) bool {
	switch expr := exceptTypeExpr.(type) {
	case *ast.Identifier:
		return matchesNamedExceptionType(exceptionType, expr.Value())
	case *ast.IndexExpression:
		// Handle dotted names like requests.HTTPError — match on the last component
		dotted := buildDottedName(expr)
		parts := strings.Split(dotted, ".")
		return matchesNamedExceptionType(exceptionType, parts[len(parts)-1])
	case *ast.TupleLiteral:
		for _, elem := range expr.Elements {
			if matchesExceptionTypeExpr(exceptionType, elem) {
				return true
			}
		}
		return false
	case *ast.ListLiteral:
		for _, elem := range expr.Elements {
			if matchesExceptionTypeExpr(exceptionType, elem) {
				return true
			}
		}
		return false
	default:
		return false
	}
}

func matchesNamedExceptionType(exceptionType, expectedType string) bool {
	if expectedType == "Exception" {
		return true
	}
	return exceptionType == expectedType
}

// buildDottedName constructs a dotted name from nested IndexExpression nodes
// e.g., requests.HTTPError becomes "requests.HTTPError"
func buildDottedName(expr *ast.IndexExpression) string {
	parts := []string{}

	// Walk the chain of index expressions
	current := ast.Expression(expr)
	for {
		if idx, ok := current.(*ast.IndexExpression); ok {
			// Get the rightmost part
			if str, ok := idx.Index.(*ast.StringLiteral); ok {
				parts = append([]string{str.Value}, parts...)
			}
			current = idx.Left
		} else if ident, ok := current.(*ast.Identifier); ok {
			// Base identifier
			parts = append([]string{ident.Value()}, parts...)
			break
		} else {
			break
		}
	}

	return strings.Join(parts, ".")
}

// assignmentExceptionError wraps an object.Exception so it can travel through
// the Go error interface returned by assignToExpression.
type assignmentExceptionError struct{ ex *object.Exception }

func (e *assignmentExceptionError) Error() string { return e.ex.Message }

func raisedAssignmentError(exceptionType, message string) error {
	return &assignmentExceptionError{
		ex: &object.Exception{
			Message:       message,
			ExceptionType: exceptionType,
		},
	}
}

// hashKeyAssignError converts a propagating object produced while hashing an
// assignment/del target key (a raised exception or internal error from
// evalHashKeyChecked) into the Go error that assignToExpression and
// deleteFromExpression return, preserving exception type information.
func hashKeyAssignError(rerr object.Object) error {
	if exc, ok := rerr.(*object.Exception); ok {
		return &assignmentExceptionError{ex: exc}
	}
	if err, ok := rerr.(*object.Error); ok {
		return fmt.Errorf("%s", err.Message)
	}
	return fmt.Errorf("assignment error")
}

func evalSliceObjectWithContext(ctx context.Context, node *ast.SliceExpression, env *object.Environment) (*object.Slice, object.Object) {
	sliceObj := &object.Slice{}

	if node.Start != nil {
		startObj := cachedExpr(&node.TargetSlots().Start, node.Start)(ctx, env)
		if propagates(startObj) {
			return nil, startObj
		}
		start, err := startObj.AsInt()
		if err != nil {
			return nil, err
		}
		sliceObj.Start = object.NewInteger(start)
	}

	if node.End != nil {
		endObj := cachedExpr(&node.TargetSlots().End, node.End)(ctx, env)
		if propagates(endObj) {
			return nil, endObj
		}
		end, err := endObj.AsInt()
		if err != nil {
			return nil, err
		}
		sliceObj.End = object.NewInteger(end)
	}

	if node.GetStep() != nil {
		stepObj := cachedExpr(&node.TargetSlots().Step, node.GetStep())(ctx, env)
		if propagates(stepObj) {
			return nil, stepObj
		}
		step, err := stepObj.AsInt()
		if err != nil {
			return nil, err
		}
		if step == 0 {
			return nil, errors.NewError("slice step cannot be zero")
		}
		sliceObj.Step = object.NewInteger(step)
	}

	return sliceObj, nil
}

func sliceDeleteIndices(length, start, end, step int64, hasStart, hasEnd, hasStep bool) []int64 {
	if !hasStep {
		step = 1
	}

	indices := []int64{}

	if step < 0 {
		if !hasStart {
			start = length - 1
		} else if start < 0 {
			start = length + start
		}
		if !hasEnd {
			end = -1
		} else if end < 0 {
			end = length + end
		}

		if start >= length {
			start = length - 1
		}
		if start < 0 {
			start = -1
		}
		if end >= length {
			end = length - 1
		}

		for i := start; i > end; i += step {
			if i >= 0 && i < length {
				indices = append(indices, i)
			}
		}
		return indices
	}

	if !hasStart {
		start = 0
	} else if start < 0 {
		start = length + start
		if start < 0 {
			start = 0
		}
	}
	if !hasEnd {
		end = length
	} else if end < 0 {
		end = length + end
		if end < 0 {
			end = 0
		}
	}

	if start < 0 {
		start = 0
	}
	if end > length {
		end = length
	}
	if start > end {
		start = end
	}

	for i := start; i < end; i += step {
		indices = append(indices, i)
	}

	return indices
}

func deleteListIndices(listObj *object.List, indices []int64) {
	if len(indices) == 0 {
		return
	}

	toDelete := make(map[int64]struct{}, len(indices))
	for _, idx := range indices {
		toDelete[idx] = struct{}{}
	}

	newElements := make([]object.Object, 0, len(listObj.Elements)-len(toDelete))
	for i, elem := range listObj.Elements {
		if _, shouldDelete := toDelete[int64(i)]; shouldDelete {
			continue
		}
		newElements = append(newElements, elem)
	}
	listObj.Elements = newElements
}

func deleteFromExpression(ctx context.Context, expr ast.Expression, env *object.Environment) error {
	switch target := expr.(type) {
	case *ast.Identifier:
		if _, ok := env.Get(target.Value()); !ok {
			return fmt.Errorf("%s", errors.NewIdentifierError(target.Value()).Message)
		}
		env.Delete(target.Value())
		return nil
	case *ast.IndexExpression:
		obj := cachedExpr(&target.TargetSlots().Left, target.Left)(ctx, env)
		if object.IsError(obj) {
			return fmt.Errorf("deletion error")
		}
		if isRaised(obj) {
			return &assignmentExceptionError{ex: obj.(*object.Exception)}
		}

		index := cachedExpr(&target.TargetSlots().Index, target.Index)(ctx, env)
		if object.IsError(index) {
			return fmt.Errorf("deletion error")
		}
		if isRaised(index) {
			return &assignmentExceptionError{ex: index.(*object.Exception)}
		}

		switch o := obj.(type) {
		case *object.List:
			idx, ok := index.(*object.Integer)
			if !ok {
				return fmt.Errorf("list index must be integer")
			}
			i := idx.IntValue()
			length := int64(len(o.Elements))
			if i < 0 {
				i += length
			}
			if i < 0 || i >= length {
				return raisedAssignmentError(object.ExceptionTypeIndexError, "list index out of range")
			}
			o.Elements = append(o.Elements[:i], o.Elements[i+1:]...)
			return nil
		case *object.Dict:
			key, rerr := evalHashKeyChecked(ctx, index)
			if rerr != nil {
				return hashKeyAssignError(rerr)
			}
			if _, ok := o.Pairs[key]; !ok {
				return raisedAssignmentError(object.ExceptionTypeKeyError, index.Inspect())
			}
			delete(o.Pairs, key)
			return nil
		case *object.Instance:
			if !target.IsDotAccess {
				if delitem, ok := o.Class.Methods["__delitem__"]; ok {
					result := applyFunctionWithContext(ctx, delitem, []object.Object{obj, index}, nil, nil)
					if object.IsError(result) {
						return fmt.Errorf("%s", result.(*object.Error).Message)
					}
					if isRaised(result) {
						return &assignmentExceptionError{ex: result.(*object.Exception)}
					}
					return nil
				}
				return fmt.Errorf("cannot delete index")
			}
			key, ok := index.(*object.String)
			if !ok {
				return fmt.Errorf("instance attribute must be string")
			}
			if _, exists := o.GetField(key.StringValue()); exists {
				o.DeleteField(key.StringValue())
				o.InvalidateBoundMethod(key.StringValue())
				return nil
			}
			if findPropertyInClass(key.StringValue(), o.Class) != nil {
				return raisedAssignmentError(object.ExceptionTypeAttributeError, fmt.Sprintf("can't delete attribute '%s'", key.StringValue()))
			}
			return &assignmentExceptionError{ex: attributeError(o, key.StringValue()).(*object.Exception)}
		case *object.Class:
			key, ok := index.(*object.String)
			if !ok {
				return fmt.Errorf("class attribute must be string")
			}
			if _, exists := o.Methods[key.StringValue()]; !exists {
				return &assignmentExceptionError{ex: attributeError(o, key.StringValue()).(*object.Exception)}
			}
			delete(o.Methods, key.StringValue())
			o.InvalidateLookupCache()
			return nil
		default:
			return fmt.Errorf("cannot delete index")
		}
	case *ast.SliceExpression:
		obj := cachedExpr(&target.TargetSlots().Left, target.Left)(ctx, env)
		if object.IsError(obj) {
			return fmt.Errorf("deletion error")
		}
		if isRaised(obj) {
			return &assignmentExceptionError{ex: obj.(*object.Exception)}
		}

		sliceObj, errObj := evalSliceObjectWithContext(ctx, target, env)
		if errObj != nil {
			if exc, ok := errObj.(*object.Exception); ok {
				return &assignmentExceptionError{ex: exc}
			}
			if evalErr, ok := errObj.(*object.Error); ok {
				return fmt.Errorf("%s", evalErr.Message)
			}
			return fmt.Errorf("deletion error")
		}

		switch o := obj.(type) {
		case *object.List:
			var start, end, step int64
			hasStart := sliceObj.Start != nil
			hasEnd := sliceObj.End != nil
			hasStep := sliceObj.Step != nil
			if hasStart {
				start = sliceObj.Start.IntValue()
			}
			if hasEnd {
				end = sliceObj.End.IntValue()
			}
			if hasStep {
				step = sliceObj.Step.IntValue()
			}
			deleteListIndices(o, sliceDeleteIndices(int64(len(o.Elements)), start, end, step, hasStart, hasEnd, hasStep))
			return nil
		case *object.Instance:
			if delitem, ok := o.Class.Methods["__delitem__"]; ok {
				result := applyFunctionWithContext(ctx, delitem, []object.Object{obj, sliceObj}, nil, nil)
				if object.IsError(result) {
					return fmt.Errorf("%s", result.(*object.Error).Message)
				}
				if isRaised(result) {
					return &assignmentExceptionError{ex: result.(*object.Exception)}
				}
				return nil
			}
			return fmt.Errorf("cannot delete slice")
		default:
			return fmt.Errorf("cannot delete slice")
		}
	default:
		return fmt.Errorf("cannot delete expression")
	}
}

// assignToSliceExpression implements slice assignment: l[start:end] =
// values splices the list (length may change), and stepped slices replace
// element-by-element with a length match required, as in Python.
func assignToSliceExpression(ctx context.Context, target *ast.SliceExpression, value object.Object, env *object.Environment) error {
	obj := fixErrorPos(ctx, cachedExpr(&target.TargetSlots().Left, target.Left)(ctx, env), target.Left.Line())
	if object.IsError(obj) {
		return fmt.Errorf("assignment error")
	}
	if isRaised(obj) {
		return &assignmentExceptionError{ex: obj.(*object.Exception)}
	}

	sliceObj, errObj := evalSliceObjectWithContext(ctx, target, env)
	if errObj != nil {
		if exc, ok := errObj.(*object.Exception); ok {
			return &assignmentExceptionError{ex: exc}
		}
		if evalErr, ok := errObj.(*object.Error); ok {
			return fmt.Errorf("%s", evalErr.Message)
		}
		return fmt.Errorf("assignment error")
	}

	var values []object.Object
	switch v := value.(type) {
	case *object.List:
		values = v.Elements
	case *object.Tuple:
		values = v.Elements
	default:
		return raisedAssignmentError(object.ExceptionTypeTypeError, "can only assign an iterable")
	}

	switch o := obj.(type) {
	case *object.List:
		var step int64 = 1
		hasStep := sliceObj.Step != nil
		if hasStep {
			step = sliceObj.Step.IntValue()
			if step == 0 {
				return raisedAssignmentError(object.ExceptionTypeValueError, "slice step cannot be zero")
			}
		}
		length := int64(len(o.Elements))
		if step == 1 {
			start, end := resolveListSliceBounds(length, sliceObj)
			merged := make([]object.Object, 0, length-int64(end-start)+int64(len(values)))
			merged = append(merged, o.Elements[:start]...)
			merged = append(merged, values...)
			merged = append(merged, o.Elements[end:]...)
			o.Elements = merged
			return nil
		}
		var rawStart, rawEnd int64
		if sliceObj.Start != nil {
			rawStart = sliceObj.Start.IntValue()
		}
		if sliceObj.End != nil {
			rawEnd = sliceObj.End.IntValue()
		}
		positions := sliceDeleteIndices(length, rawStart, rawEnd, step, sliceObj.Start != nil, sliceObj.End != nil, hasStep)
		if int64(len(values)) != int64(len(positions)) {
			return raisedAssignmentError(object.ExceptionTypeValueError,
				fmt.Sprintf("attempt to assign sequence of size %d to extended slice of size %d", len(values), len(positions)))
		}
		for i, pos := range positions {
			o.Elements[pos] = values[i]
		}
		return nil
	case *object.Instance:
		if setitem, ok := o.Class.Methods["__setitem__"]; ok {
			result := applyFunctionWithContext(ctx, setitem, []object.Object{obj, sliceObj, value}, nil, nil)
			if object.IsError(result) {
				return fmt.Errorf("%s", result.(*object.Error).Message)
			}
			if isRaised(result) {
				return &assignmentExceptionError{ex: result.(*object.Exception)}
			}
			return nil
		}
		return fmt.Errorf("cannot assign to slice")
	default:
		return fmt.Errorf("cannot assign to slice")
	}
}

// resolveListSliceBounds clamps a step-1 slice's start/end against the list
// length the way Python does (used by slice assignment).
func resolveListSliceBounds(length int64, sliceObj *object.Slice) (int64, int64) {
	start, end := int64(0), length
	if sliceObj.Start != nil {
		start = sliceObj.Start.IntValue()
		if start < 0 {
			start += length
			if start < 0 {
				start = 0
			}
		}
	}
	if sliceObj.End != nil {
		end = sliceObj.End.IntValue()
		if end < 0 {
			end += length
			if end < 0 {
				end = 0
			}
		}
	}
	if end > length {
		end = length
	}
	if start > end {
		start = end
	}
	return start, end
}

func assignToExpression(ctx context.Context, expr ast.Expression, value object.Object, env *object.Environment) error {
	switch left := expr.(type) {
	case *ast.Identifier:
		// Fast path: use cached slot index with name validation.
		if cached := left.SlotCache.Load(); cached > 0 {
			if env.SetCachedSlot(int(cached-1), left.Value(), value) {
				return nil
			}
		}
		env.Set(left.Value(), value)
		// Cache the slot index for future writes.
		if left.SlotCache.Load() == 0 {
			if idx, ok := env.GetSlotIndex(left.Value()); ok {
				left.SlotCache.Store(int32(idx + 1))
			}
		}
		return nil
	case *ast.IndexExpression:
		if err := assignToNestedFloatArrayIndex(ctx, left, value, env); err != nil {
			if err == errNotNestedFloatArrayAssignment {
				// Fall through to regular assignment handling.
			} else {
				return err
			}
		} else {
			return nil
		}
		obj := fixErrorPos(ctx, cachedExpr(&left.TargetSlots().Left, left.Left)(ctx, env), left.Left.Line())
		if isRaised(obj) {
			return &assignmentExceptionError{ex: obj.(*object.Exception)}
		}
		if object.IsError(obj) {
			return fmt.Errorf("assignment error")
		}
		index := cachedExpr(&left.TargetSlots().Index, left.Index)(ctx, env)
		if isRaised(index) {
			return &assignmentExceptionError{ex: index.(*object.Exception)}
		}
		if object.IsError(index) {
			return fmt.Errorf("assignment error")
		}
		return assignIndexValue(ctx, left.IsDotAccess, obj, index, value)
	case *ast.SliceExpression:
		return assignToSliceExpression(ctx, left, value, env)
	default:
		return fmt.Errorf("cannot assign to expression")
	}
}

var errNotNestedFloatArrayAssignment = fmt.Errorf("not nested float_array assignment")

func assignToNestedFloatArrayIndex(ctx context.Context, expr *ast.IndexExpression, value object.Object, env *object.Environment) error {
	rowExpr, ok := expr.Left.(*ast.IndexExpression)
	if !ok {
		return errNotNestedFloatArrayAssignment
	}

	baseObj := fixErrorPos(ctx, cachedExpr(&rowExpr.TargetSlots().Left, rowExpr.Left)(ctx, env), rowExpr.Left.Line())
	if object.IsError(baseObj) {
		return fmt.Errorf("assignment error")
	}
	fa, ok := baseObj.(*object.FloatArray)
	if !ok || !fa.Is2D() {
		return errNotNestedFloatArrayAssignment
	}

	rowIndexObj := fixErrorPos(ctx, cachedExpr(&rowExpr.TargetSlots().Index, rowExpr.Index)(ctx, env), rowExpr.Index.Line())
	if object.IsError(rowIndexObj) {
		return fmt.Errorf("assignment error")
	}
	rowIndex, ok := rowIndexObj.(*object.Integer)
	if !ok {
		return fmt.Errorf("float_array index must be integer")
	}

	colIndexObj := fixErrorPos(ctx, cachedExpr(&expr.TargetSlots().Index, expr.Index)(ctx, env), expr.Index.Line())
	if object.IsError(colIndexObj) {
		return fmt.Errorf("assignment error")
	}
	colIndex, ok := colIndexObj.(*object.Integer)
	if !ok {
		return fmt.Errorf("float_array index must be integer")
	}

	return assignNestedFloatArrayValue(fa, rowIndex, colIndex, value)
}

// assignNestedFloatArrayValue stores value at fa[row][col] of a 2-D float
// array from already-evaluated indices.
func assignNestedFloatArrayValue(fa *object.FloatArray, rowIndex, colIndex *object.Integer, value object.Object) error {
	row := rowIndex.IntValue()
	rows := int64(fa.Rows())
	if row < 0 {
		row += rows
	}
	if row < 0 || row >= rows {
		return fmt.Errorf("index out of range")
	}

	col := colIndex.IntValue()
	cols := int64(fa.Cols())
	if col < 0 {
		col += cols
	}
	if col < 0 || col >= cols {
		return fmt.Errorf("index out of range")
	}

	f, err := value.AsFloat()
	if err != nil {
		return fmt.Errorf("float_array element must be a number")
	}
	fa.Data[int(row)*fa.Cols()+int(col)] = f
	return nil
}

// findPropertyInClass walks the class hierarchy looking for a Property descriptor.
func findPropertyInClass(name string, class *object.Class) *object.Property {
	if fn, ok := class.LookupMember(name); ok {
		if prop, ok := fn.(*object.Property); ok {
			return prop
		}
	}
	return nil
}

func setForVariables(variables []ast.Expression, value object.Object, env *object.Environment) error {
	if len(variables) == 1 {
		return setForVariable(variables[0], value, env)
	}

	// Flat unpacking: a, b in ...
	var elements []object.Object
	switch v := value.(type) {
	case *object.Tuple:
		elements = v.Elements
	case *object.List:
		elements = v.Elements
	default:
		return fmt.Errorf("cannot unpack non-tuple/list value")
	}

	if len(elements) != len(variables) {
		return fmt.Errorf("cannot unpack %d values into %d variables", len(elements), len(variables))
	}

	for i, varExpr := range variables {
		if err := setForVariable(varExpr, elements[i], env); err != nil {
			return err
		}
	}
	return nil
}

// setForVariable assigns a single for-loop target expression to a value.
// Supports identifiers and nested tuple/list unpacking, e.g. for a, (b, c) in ...
func setForVariable(varExpr ast.Expression, value object.Object, env *object.Environment) error {
	switch target := varExpr.(type) {
	case *ast.Identifier:
		setIdentifierFast(target, value, env)
		return nil
	case *ast.TupleLiteral:
		return setForVariables(target.Elements, value, env)
	case *ast.ListLiteral:
		return setForVariables(target.Elements, value, env)
	default:
		return fmt.Errorf("for loop variables must be identifiers")
	}
}

func setIdentifierFast(target *ast.Identifier, value object.Object, env *object.Environment) {
	if cached := target.SlotCache.Load(); cached > 0 {
		if env.SetCachedSlot(int(cached-1), target.Value(), value) {
			return
		}
	}
	env.Set(target.Value(), value)
	if target.SlotCache.Load() == 0 {
		if idx, ok := env.GetSlotIndex(target.Value()); ok {
			target.SlotCache.Store(int32(idx + 1))
		}
	}
}

// iterableToSliceChecked materializes an iterable into a slice of elements for
// builtins that accept any iterable (list, tuple, sorted, map, ...). Class
// instances run the full iterator protocol exactly like a for-loop: __iter__
// must return an iterator, __next__ runs to StopIteration, and a raised
// exception or internal error (from the protocol itself or yielded as an
// element) propagates as the third return. Non-instance types defer to
// object.IterableToSlice; ok=false without a raise means "not iterable" and
// the caller reports its usual type error.
// acceptInstanceIterableFn is set in init() to break the initialization
// cycle between the builtins map and the evaluator (see evalTruthyFn).
var acceptInstanceIterableFn func(ctx context.Context, obj object.Object) (object.Object, object.Object)

// acceptInstanceIterable validates one enumerate/zip argument. Materialized
// builtin iterables and raw iterators pass through unchanged: the zip and
// enumerate iterators pull iterator inputs lazily (so infinite iterators such
// as itertools.cycle work, as in Python) and pass a yielded error or raised
// exception straight to the consumer. Class instances get __iter__ called
// now (a raise there surfaces at construction, like Python) and the
// resulting iterator is likewise pulled lazily. It returns (nil, nil) when
// the object is not an iterable type at all, so the caller reports its usual
// type error.
func acceptInstanceIterable(ctx context.Context, obj object.Object) (object.Object, object.Object) {
	switch o := obj.(type) {
	case *object.List, *object.Tuple, *object.String, *object.FloatArray, *object.Iterator:
		return obj, nil
	case *object.Instance:
		if fn, has := findDunderMethod(o, "__iter__"); has {
			env := GetEnvFromContext(ctx)
			iterObj := applyFunctionWithContext(ctx, fn, prependSelf(o, nil), nil, env)
			if propagates(iterObj) {
				return nil, iterObj
			}
			switch it := iterObj.(type) {
			case *object.Iterator:
				return it, nil
			case *object.Instance:
				return instanceToIterator(ctx, it, env), nil
			default:
				return nil, errors.NewError("__iter__ must return an iterator")
			}
		}
		elems, ok, rerr := iterableToSliceChecked(ctx, obj, GetEnvFromContext(ctx))
		if rerr != nil {
			return nil, rerr
		}
		if !ok {
			return nil, nil
		}
		return &object.List{Elements: elems}, nil
	default:
		return nil, nil
	}
}

// iterableToSliceCheckedFn is set in init() to break the initialization
// cycle between the builtins map and the evaluator (see evalTruthyFn).
var iterableToSliceCheckedFn func(ctx context.Context, obj object.Object, env *object.Environment) ([]object.Object, bool, object.Object)

func iterableToSliceChecked(ctx context.Context, obj object.Object, env *object.Environment) ([]object.Object, bool, object.Object) {
	if iter, isIter := obj.(*object.Iterator); isIter {
		// A raw iterator may yield raised exceptions or internal errors
		// (e.g. wrapped user iterators); check every element while pulling.
		// Context is checked periodically so an infinite user iterator stays
		// cancellable by timeout, like a for-loop over it would be.
		elements := []object.Object{}
		cc := newContextChecker(ctx)
		for {
			if err := cc.check(); err != nil {
				return nil, false, err
			}
			val, hasNext := iter.Next()
			if !hasNext {
				break
			}
			if propagates(val) {
				return nil, false, val
			}
			elements = append(elements, val)
		}
		return elements, true, nil
	}
	inst, isInst := obj.(*object.Instance)
	if !isInst {
		elems, ok := object.IterableToSlice(obj)
		return elems, ok, nil
	}
	fn, has := findDunderMethod(inst, "__iter__")
	if !has {
		return nil, false, nil
	}
	iterObj := applyFunctionWithContext(ctx, fn, prependSelf(inst, nil), nil, env)
	if propagates(iterObj) {
		return nil, false, iterObj
	}
	var iter *object.Iterator
	if iterInst, ok := iterObj.(*object.Instance); ok {
		iter = instanceToIterator(ctx, iterInst, env)
	} else if iterIter, ok := iterObj.(*object.Iterator); ok {
		iter = iterIter
	} else {
		return nil, false, errors.NewError("__iter__ must return an iterator")
	}
	elements := []object.Object{}
	cc := newContextChecker(ctx)
	for {
		if err := cc.check(); err != nil {
			return nil, false, err
		}
		val, hasNext := iter.Next()
		if !hasNext {
			break
		}
		if propagates(val) {
			return nil, false, val
		}
		elements = append(elements, val)
	}
	return elements, true, nil
}

// instanceToIterator wraps an instance with __next__ as an object.Iterator.
//
// End-of-iteration is signalled ONLY by a StopIteration exception (or a nil
// result) → (nil, false). Any other raised exception or internal *object.Error
// from __next__ is a real failure and must propagate out of the for-loop, so it
// is yielded as the "next" element with ok=true; the loop driver detects the
// error/raise before binding and returns it. Previously these were mapped to
// end-of-iteration, which either looped forever (a raised value the body didn't
// consume) or silently ended the loop (an internal error such as a NameError).
func instanceToIterator(ctx context.Context, inst *object.Instance, env *object.Environment) *object.Iterator {
	return object.NewIterator(func() (object.Object, bool) {
		result := callDunderMethod(ctx, inst, "__next__", nil, env)
		if result == nil {
			return nil, false
		}
		// StopIteration signals normal end of iteration.
		if exc, ok := result.(*object.Exception); ok && exc.ExceptionType == object.ExceptionTypeStopIteration {
			return nil, false
		}
		// A raised exception or an internal error propagates: yield it so the
		// loop driver can detect and return it.
		if propagates(result) {
			return result, true
		}
		return result, true
	})
}

// evalMethodCallExpression is in methods.go
// callStringMethodWithKeywords is in methods.go

func iterateObject(ctx context.Context, obj object.Object, fn func(object.Object) object.Object) object.Object {
	switch o := obj.(type) {
	case *object.List:
		for _, el := range o.Elements {
			if err := fn(el); err != nil {
				return err
			}
		}
	case *object.Tuple:
		for _, el := range o.Elements {
			if err := fn(el); err != nil {
				return err
			}
		}
	case *object.Iterator:
		for {
			el, ok := o.Next()
			if !ok {
				break
			}
			// A user __next__ that raised (or errored) yields the exception as
			// the element; propagate it instead of feeding it to the body.
			if propagates(el) {
				return el
			}
			if err := fn(el); err != nil {
				return err
			}
		}
	case *object.Dict:
		iter := o.CreateIterator()
		for {
			el, ok := iter.Next()
			if !ok {
				break
			}
			if err := fn(el); err != nil {
				return err
			}
		}
	case *object.DictKeys:
		iter := o.CreateIterator()
		for {
			el, ok := iter.Next()
			if !ok {
				break
			}
			if err := fn(el); err != nil {
				return err
			}
		}
	case *object.DictValues:
		iter := o.CreateIterator()
		for {
			el, ok := iter.Next()
			if !ok {
				break
			}
			if err := fn(el); err != nil {
				return err
			}
		}
	case *object.DictItems:
		iter := o.CreateIterator()
		for {
			el, ok := iter.Next()
			if !ok {
				break
			}
			if err := fn(el); err != nil {
				return err
			}
		}
	case *object.Set:
		iter := o.CreateIterator()
		for {
			el, ok := iter.Next()
			if !ok {
				break
			}
			if err := fn(el); err != nil {
				return err
			}
		}
	case *object.String:
		for _, ch := range o.StringValue() {
			if err := fn(object.NewString(string(ch))); err != nil {
				return err
			}
		}
	case *object.Bytes:
		for _, b := range o.BytesValue() {
			if err := fn(object.NewInteger(int64(b))); err != nil {
				return err
			}
		}
	case *object.FloatArray:
		if o.Is2D() {
			rows := o.Rows()
			cols := o.Cols()
			for i := 0; i < rows; i++ {
				off := i * cols
				rowData := make([]float64, cols)
				copy(rowData, o.Data[off:off+cols])
				row := object.NewFloatArray1D(rowData)
				if err := fn(row); err != nil {
					return err
				}
			}
		} else {
			for _, v := range o.Data {
				if err := fn(object.NewFloat(v)); err != nil {
					return err
				}
			}
		}
	case *object.Instance:
		if iterFn, ok := findDunderMethod(o, "__iter__"); ok {
			iterObj := applyFunctionWithContext(ctx, iterFn, prependSelf(o, nil), nil, nil)
			if propagates(iterObj) {
				return iterObj
			}
			var iter *object.Iterator
			if iterInst, ok := iterObj.(*object.Instance); ok {
				iter = instanceToIterator(ctx, iterInst, nil)
			} else if iterIter, ok := iterObj.(*object.Iterator); ok {
				iter = iterIter
			} else {
				return errors.NewError("__iter__ must return an iterator")
			}
			for {
				el, ok := iter.Next()
				if !ok {
					break
				}
				// Propagate a raised exception / internal error yielded by a
				// user __next__ instead of feeding it to the body.
				if propagates(el) {
					return el
				}
				if err := fn(el); err != nil {
					return err
				}
			}
		} else {
			return errors.NewTypeError("iterable", obj.Type().String())
		}
	default:
		return errors.NewTypeError("iterable", obj.Type().String())
	}
	return nil
}

func formatWithSpec(obj object.Object, spec string) string {
	if spec == "" {
		switch v := obj.(type) {
		case *object.Integer:
			return strconv.FormatInt(v.IntValue(), 10)
		case *object.Float:
			return object.FloatStr(v.FloatValue())
		}
		return obj.Inspect()
	}

	// Parse the format spec: [[fill]align][sign][#][0][width][grouping][.precision][type]
	// We support: [fill]align, 0width, width, .precision, type
	// Types: d, f, e, E, g, G, x, X, o, b, s, %
	// Align: <, >, ^, = (with optional fill char)
	// Grouping: ,

	var fill rune = ' '
	var align rune
	var sign rune // '+', '-', or ' '
	var zero bool
	var width int
	var precision int = -1
	var grouping bool
	var typeChar byte

	i := 0
	runes := []rune(spec)
	n := len(runes)

	// Check for fill+align (2 chars: fill then align)
	if n >= 2 && (runes[1] == '<' || runes[1] == '>' || runes[1] == '^' || runes[1] == '=') {
		fill = runes[0]
		align = runes[1]
		i = 2
	} else if n >= 1 && (runes[0] == '<' || runes[0] == '>' || runes[0] == '^' || runes[0] == '=') {
		align = runes[0]
		i = 1
	}

	// Sign (+, -, space)
	if i < n && (runes[i] == '+' || runes[i] == '-' || runes[i] == ' ') {
		sign = runes[i]
		i++
	}

	// Skip # (alternate form)
	if i < n && runes[i] == '#' {
		i++
	}

	// Zero padding
	if i < n && runes[i] == '0' && align == 0 {
		zero = true
		i++
	}

	// Width
	for i < n && runes[i] >= '0' && runes[i] <= '9' {
		width = width*10 + int(runes[i]-'0')
		i++
	}

	// Grouping
	if i < n && runes[i] == ',' {
		grouping = true
		i++
	}

	// Precision
	if i < n && runes[i] == '.' {
		i++
		precision = 0
		for i < n && runes[i] >= '0' && runes[i] <= '9' {
			precision = precision*10 + int(runes[i]-'0')
			i++
		}
	}

	// Type
	if i < n {
		typeChar = byte(runes[i])
	}

	// Format the value
	var formatted string
	switch typeChar {
	case 'd', 0:
		if typeChar == 0 && obj.Type() == object.FLOAT_OBJ {
			// No type char with float: use 'g' or precision
			if floatVal, ok := numericFloatValue(obj); ok {
				if precision >= 0 {
					formatted = strconv.FormatFloat(floatVal, 'f', precision, 64)
				} else {
					formatted = object.FloatStr(floatVal)
				}
				formatted = applySign(formatted, floatVal >= 0, sign)
			} else {
				formatted = obj.Inspect()
			}
		} else if typeChar == 0 && obj.Type() == object.STRING_OBJ {
			// No type char with string: apply precision as truncation
			if s, err := obj.AsString(); err == nil {
				formatted = s
				if precision >= 0 {
					runes := []rune(formatted)
					if len(runes) > precision {
						formatted = string(runes[:precision])
					}
				}
			} else {
				formatted = obj.Inspect()
			}
		} else if intVal, err := obj.AsInt(); err == nil {
			if zero && width > 0 {
				formatted = formatZeroPaddedInt(intVal, width)
				formatted = applySign(formatted, intVal >= 0, sign)
			} else {
				formatted = strconv.FormatInt(intVal, 10)
				formatted = applySign(formatted, intVal >= 0, sign)
			}
		} else {
			formatted = obj.Inspect()
		}
	case 'f', 'F':
		if floatVal, ok := numericFloatValue(obj); ok {
			prec := 6
			if precision >= 0 {
				prec = precision
			}
			formatted = strconv.FormatFloat(floatVal, 'f', prec, 64)
			formatted = applySign(formatted, floatVal >= 0, sign)
		} else {
			formatted = obj.Inspect()
		}
	case 'e':
		if floatVal, ok := numericFloatValue(obj); ok {
			prec := 6
			if precision >= 0 {
				prec = precision
			}
			formatted = strconv.FormatFloat(floatVal, 'e', prec, 64)
			formatted = applySign(formatted, floatVal >= 0, sign)
		} else {
			formatted = obj.Inspect()
		}
	case 'E':
		if floatVal, ok := numericFloatValue(obj); ok {
			prec := 6
			if precision >= 0 {
				prec = precision
			}
			formatted = strings.ToUpper(strconv.FormatFloat(floatVal, 'e', prec, 64))
			formatted = applySign(formatted, floatVal >= 0, sign)
		} else {
			formatted = obj.Inspect()
		}
	case 'g', 'G':
		if floatVal, ok := numericFloatValue(obj); ok {
			if precision >= 0 {
				if typeChar == 'G' {
					formatted = strings.ToUpper(strconv.FormatFloat(floatVal, 'g', precision, 64))
				} else {
					formatted = strconv.FormatFloat(floatVal, 'g', precision, 64)
				}
			} else {
				formatted = strconv.FormatFloat(floatVal, 'g', -1, 64)
				if typeChar == 'G' {
					formatted = strings.ToUpper(formatted)
				}
			}
			formatted = applySign(formatted, floatVal >= 0, sign)
		} else {
			formatted = obj.Inspect()
		}
	case 'x':
		if intVal, err := obj.AsInt(); err == nil {
			formatted = formatBaseInt(intVal, 16, false, width, zero)
		} else {
			formatted = obj.Inspect()
		}
	case 'X':
		if intVal, err := obj.AsInt(); err == nil {
			formatted = formatBaseInt(intVal, 16, true, width, zero)
		} else {
			formatted = obj.Inspect()
		}
	case 'o':
		if intVal, err := obj.AsInt(); err == nil {
			formatted = formatBaseInt(intVal, 8, false, width, zero)
		} else {
			formatted = obj.Inspect()
		}
	case 'b':
		if intVal, err := obj.AsInt(); err == nil {
			formatted = formatBaseInt(intVal, 2, false, width, zero)
		} else {
			formatted = obj.Inspect()
		}
	case 's':
		if s, err := obj.AsString(); err == nil {
			formatted = s
		} else {
			formatted = obj.Inspect()
		}
		if precision >= 0 {
			runes := []rune(formatted)
			if len(runes) > precision {
				formatted = string(runes[:precision])
			}
		}
	case '%':
		if floatVal, ok := numericFloatValue(obj); ok {
			prec := 6
			if precision >= 0 {
				prec = precision
			}
			formatted = strconv.FormatFloat(floatVal*100, 'f', prec, 64) + "%"
			formatted = applySign(formatted, floatVal >= 0, sign)
		} else {
			formatted = obj.Inspect()
		}
	default:
		formatted = obj.Inspect()
	}

	// Apply thousands grouping
	if grouping && (typeChar == 'd' || typeChar == 0) {
		if obj.Type() == object.FLOAT_OBJ {
			// Group the integer part and keep the fraction, like Python's
			// format(x, ",").
			if floatVal, ok := numericFloatValue(obj); ok {
				commaStr := formatWithCommas(int64(floatVal))
				frac := ""
				digits := object.FloatStr(math.Abs(floatVal))
				if idx := strings.IndexByte(digits, '.'); idx >= 0 {
					frac = digits[idx:]
				}
				commaStr = applySign(commaStr+frac, floatVal >= 0, sign)
				formatted = commaStr
			}
		} else if intVal, err := obj.AsInt(); err == nil {
			commaStr := formatWithCommas(intVal)
			commaStr = applySign(commaStr, intVal >= 0, sign)
			formatted = commaStr
		}
	} else if grouping && (typeChar == 'f' || typeChar == 'F') {
		parts := strings.SplitN(formatted, ".", 2)
		if intVal, err := strconv.ParseInt(strings.TrimLeft(parts[0], "-"), 10, 64); err == nil {
			commaInt := formatWithCommas(intVal)
			if strings.HasPrefix(parts[0], "-") {
				commaInt = "-" + commaInt
			}
			if len(parts) == 2 {
				formatted = commaInt + "." + parts[1]
			} else {
				formatted = commaInt
			}
		}
	}

	// Apply width and alignment (use rune-aware length for Unicode)
	if width > 0 && len([]rune(formatted)) < width {
		padding := width - len([]rune(formatted))
		switch align {
		case '<':
			formatted = formatted + strings.Repeat(string(fill), padding)
		case '>':
			formatted = strings.Repeat(string(fill), padding) + formatted
		case '^':
			left := padding / 2
			right := padding - left
			formatted = strings.Repeat(string(fill), left) + formatted + strings.Repeat(string(fill), right)
		default:
			// Default alignment: right for numbers, left for strings
			if zero {
				// Already handled in the type formatting above for integers/hex/oct/bin
				// For floats, apply zero padding (preserving sign)
				if obj.Type() == object.FLOAT_OBJ {
					if len(formatted) > 0 && (formatted[0] == '+' || formatted[0] == '-' || formatted[0] == ' ') {
						formatted = string(formatted[0]) + strings.Repeat("0", padding) + formatted[1:]
					} else {
						formatted = strings.Repeat("0", padding) + formatted
					}
				} else {
					formatted = strings.Repeat(" ", padding) + formatted
				}
			} else if obj.Type() == object.STRING_OBJ || typeChar == 's' {
				formatted = formatted + strings.Repeat(string(fill), padding)
			} else {
				formatted = strings.Repeat(string(fill), padding) + formatted
			}
		}
	}

	return formatted
}

// applySign prepends a sign character to a formatted number string.
// sign: '+' always show sign, ' ' show space for positive, 0 means no sign prefix.
// The formatted string may already have a '-' prefix for negative numbers.
func applySign(formatted string, positive bool, sign rune) string {
	if !positive {
		return formatted // already has '-'
	}
	switch sign {
	case '+':
		return "+" + formatted
	case ' ':
		return " " + formatted
	}
	return formatted
}

// formatWithCommas formats an integer with thousands separators
func formatWithCommas(n int64) string {
	if n < 0 {
		return "-" + formatWithCommas(-n)
	}
	s := strconv.FormatInt(n, 10)
	if len(s) <= 3 {
		return s
	}
	var result strings.Builder
	start := len(s) % 3
	if start > 0 {
		result.WriteString(s[:start])
	}
	for i := start; i < len(s); i += 3 {
		if i > 0 || start > 0 {
			result.WriteByte(',')
		}
		result.WriteString(s[i : i+3])
	}
	return result.String()
}

func formatZeroPaddedInt(n int64, width int) string {
	if width <= 0 {
		return strconv.FormatInt(n, 10)
	}
	if n < 0 {
		digits := strconv.FormatInt(-n, 10)
		if len(digits)+1 >= width {
			return "-" + digits
		}
		return "-" + strings.Repeat("0", width-len(digits)-1) + digits
	}
	digits := strconv.FormatInt(n, 10)
	if len(digits) >= width {
		return digits
	}
	return strings.Repeat("0", width-len(digits)) + digits
}

func formatBaseInt(n int64, base int, upper bool, width int, zero bool) string {
	formatted := strconv.FormatInt(n, base)
	if upper {
		formatted = strings.ToUpper(formatted)
	}
	if !zero || width <= 0 || len(formatted) >= width {
		return formatted
	}
	if n < 0 {
		return "-" + strings.Repeat("0", width-len(formatted)) + formatted[1:]
	}
	return strings.Repeat("0", width-len(formatted)) + formatted
}

func matchPattern(ctx context.Context, subject object.Object, pattern ast.Expression, capturedVars map[string]object.Object, env *object.Environment) (object.Object, object.Object) {
	switch p := pattern.(type) {
	case *ast.OrPattern:
		for _, alt := range p.Patterns {
			matched, val := matchPattern(ctx, subject, alt, capturedVars, env)
			if propagates(matched) {
				return matched, NULL
			}
			if matched == TRUE {
				return TRUE, val
			}
		}
		return FALSE, NULL

	case *ast.Identifier:
		// Wildcard pattern
		if p.Value() == "_" {
			return TRUE, subject
		}

		// All other identifiers are capture variables (always match)
		// Bind the captured value to the identifier name
		capturedVars[p.Value()] = subject
		return TRUE, subject

	case *ast.CallExpression:
		// Handle type patterns like int(), str(), list(), dict()
		if ident, ok := p.Function.(*ast.Identifier); ok {
			// Check if it's a type constructor with no arguments
			if len(p.Arguments) == 0 && !p.HasOverflow() {
				typeName := ident.Value()
				subjectType := getTypeName(subject)
				if typeName == subjectType {
					return TRUE, subject
				}
				return FALSE, NULL
			}
		}
		return &object.Error{Message: "call expressions in patterns must be type constructors with no arguments"}, NULL

	case *ast.IntegerLiteral:
		if intObj, ok := subject.(*object.Integer); ok {
			if intObj.IntValue() == p.Value {
				return TRUE, subject
			}
		}
		return FALSE, NULL

	case *ast.FloatLiteral:
		if floatObj, ok := subject.(*object.Float); ok {
			if floatObj.FloatValue() == p.Value {
				return TRUE, subject
			}
		}
		return FALSE, NULL

	case *ast.StringLiteral:
		if strObj, ok := subject.(*object.String); ok {
			if strObj.StringValue() == p.Value {
				return TRUE, subject
			}
		}
		return FALSE, NULL

	case *ast.Boolean:
		if boolObj, ok := subject.(*object.Boolean); ok {
			if boolObj.BoolValue() == p.Value {
				return TRUE, subject
			}
		}
		return FALSE, NULL

	case *ast.None:
		if subject == NULL {
			return TRUE, subject
		}
		return FALSE, NULL

	case *ast.DictLiteral:
		// Structural matching for dictionaries
		dictObj, ok := subject.(*object.Dict)
		if !ok {
			return FALSE, NULL
		}

		// Match all keys in pattern
		for _, patternPair := range p.Pairs {
			// Mapping-pattern keys are value expressions evaluated at match
			// time in the enclosing scope (PEP 634), so variables are visible.
			// Mapping patterns are rare, so the key expression is compiled
			// on the spot rather than cached on the pattern node.
			keyObj := compileExpr(patternPair.Key)(ctx, env)
			if propagates(keyObj) {
				return keyObj, NULL
			}

			keyStr, rerr := evalHashKeyChecked(ctx, keyObj)
			if rerr != nil {
				return rerr, NULL
			}
			dictPair, exists := dictObj.Pairs[keyStr]
			if !exists {
				return FALSE, NULL
			}

			// If pattern value is an identifier (not _), it's a capture variable
			if ident, ok := patternPair.Value.(*ast.Identifier); ok && ident.Value() != "_" {
				// Store the captured value
				capturedVars[ident.Value()] = dictPair.Value
			} else {
				// Otherwise, it must match exactly
				matched, _ := matchPattern(ctx, dictPair.Value, patternPair.Value, capturedVars, env)
				if propagates(matched) {
					return matched, NULL
				}
				if matched == FALSE {
					return FALSE, NULL
				}
			}
		}

		return TRUE, subject

	case *ast.ListLiteral:
		// Simple list matching
		listObj, ok := subject.(*object.List)
		if !ok {
			return FALSE, NULL
		}

		if len(p.Elements) != len(listObj.Elements) {
			return FALSE, NULL
		}

		for i, elemExpr := range p.Elements {
			matched, _ := matchPattern(ctx, listObj.Elements[i], elemExpr, capturedVars, env)
			if propagates(matched) {
				return matched, NULL
			}
			if matched == FALSE {
				return FALSE, NULL
			}
		}

		return TRUE, subject

	default:
		return &object.Error{Message: fmt.Sprintf("unsupported pattern type: %T", pattern)}, NULL
	}
}

func getTypeName(obj object.Object) string {
	switch obj.Type() {
	case object.INTEGER_OBJ:
		return "int"
	case object.FLOAT_OBJ:
		return "float"
	case object.STRING_OBJ:
		return "str"
	case object.BOOLEAN_OBJ:
		return "bool"
	case object.LIST_OBJ:
		return "list"
	case object.DICT_OBJ:
		if obj.(*object.Dict).Module != "" {
			return "module"
		}
		return "dict"
	case object.TUPLE_OBJ:
		return "tuple"
	case object.SET_OBJ:
		return "set"
	case object.NULL_OBJ:
		return "NoneType"
	case object.BYTES_OBJ:
		return "bytes"
	case object.DICT_KEYS_OBJ:
		return "dict_keys"
	case object.DICT_VALUES_OBJ:
		return "dict_values"
	case object.DICT_ITEMS_OBJ:
		return "dict_items"
	case object.FUNCTION_OBJ, object.LAMBDA_OBJ:
		return "function"
	case object.BUILTIN_OBJ:
		return "builtin_function_or_method"
	case object.CLASS_OBJ:
		return "type"
	case object.INSTANCE_OBJ:
		return obj.(*object.Instance).Class.Name
	case object.EXCEPTION_OBJ:
		if exc, ok := obj.(*object.Exception); ok && exc.ExceptionType != "" {
			return exc.ExceptionType
		}
		return "Exception"
	case object.SUPER_OBJ:
		return "super"
	case object.ITERATOR_OBJ:
		return "iterator"
	case object.SLICE_OBJ:
		return "slice"
	case object.PROPERTY_OBJ:
		return "property"
	case object.STATICMETHOD_OBJ:
		return "staticmethod"
	case object.CLASSMETHOD_OBJ:
		return "classmethod"
	case object.FLOAT_ARRAY_OBJ:
		return "FloatArray"
	default:
		return obj.Type().String()
	}
}
