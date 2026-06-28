package evaluator

import (
	"context"
	"fmt"
	"math"
	"runtime"
	"strconv"
	"strings"

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

// SetEnvInContext stores environment in context for builtin functions
func SetEnvInContext(ctx context.Context, env *object.Environment) context.Context {
	return context.WithValue(ctx, envContextKey, env)
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
// the context was derived. Returns a release function (a no-op when re-entrant).
func enterScript(ctx context.Context, env *object.Environment) (context.Context, func()) {
	if env != nil && env.EnterGIL() {
		return ctx, env.ExitGIL
	}
	return ctx, func() {}
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
	// Acquire the interpreter lock for the duration of this top-level run so the
	// whole tree is touched by one goroutine at a time.
	ctx, release := enterScript(ctx, env)
	defer release()
	return evalWithContext(ctx, node, env)
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

// contextChecker helps batch context checks in loops to reduce overhead
type contextChecker struct {
	ctx       context.Context
	counter   int
	batchSize int
}

func newContextChecker(ctx context.Context) contextChecker {
	return contextChecker{
		ctx:       ctx,
		batchSize: 10, // Check context every 10 operations in loops
	}
}

func (cc *contextChecker) check() object.Object {
	cc.counter++
	if cc.counter >= cc.batchSize {
		cc.counter = 0
		return checkContext(cc.ctx)
	}
	return nil
}

// checkAlways checks context every time (for critical sections)
func (cc *contextChecker) checkAlways() object.Object {
	return checkContext(cc.ctx)
}

func evalWithContext(ctx context.Context, node ast.Node, env *object.Environment) object.Object {
	obj := evalNode(ctx, node, env)
	if err, ok := obj.(*object.Error); ok {
		if err.Line == 0 {
			err.Line = node.Line()
		}
		if err.File == "" {
			err.File = GetSourceFileFromContext(ctx)
		}
	}
	return obj
}

func evalNode(ctx context.Context, node ast.Node, env *object.Environment) object.Object {
	// Check for cancellation - batched via context checker in the top-level EvalWithContext
	// For leaf nodes, we skip the check to reduce overhead
	switch node := node.(type) {
	case *ast.ExpressionStatement:
		return evalWithContext(ctx, node.Expression, env)
	case *ast.InfixExpression:
		if node.Operator == ast.OpAnd || node.Operator == ast.OpOr {
			return evalShortCircuitInfixExpression(ctx, node, env)
		}
		if node.Operator == ast.OpAdd {
			if left, ok := node.Left.(*ast.InfixExpression); ok && left.Operator == ast.OpAdd {
				if result, ok := tryEvalStringConcatChain(ctx, node, env); ok {
					return result
				}
			}
		}
		if node.Operator >= ast.OpAdd && node.Operator <= ast.OpNeq {
			if lid, ok := node.Left.(*ast.Identifier); ok {
				if rid, ok := node.Right.(*ast.Identifier); ok {
					if lv, ok := env.Get(lid.Value()); ok {
						if li, ok := lv.(*object.Integer); ok {
							if rv, ok := env.Get(rid.Value()); ok {
								if ri, ok := rv.(*object.Integer); ok {
									return evalIntegerInfixExpression(node.Operator, li.IntValue(), ri.IntValue())
								}
							}
						}
					}
				}
			}
		}
		// General path: evaluate both sides
		left := evalNode(ctx, node.Left, env)
		if object.IsError(left) {
			return left
		}
		right := evalNode(ctx, node.Right, env)
		if object.IsError(right) {
			return right
		}
		return evalInfixExpression(ctx, node.Operator, left, right, env)
	case *ast.ReturnStatement:
		val := object.Object(NULL)
		if node.ReturnValue != nil {
			val = evalNode(ctx, node.ReturnValue, env)
			if object.IsError(val) || isException(val) {
				return val
			}
		}
		return acquireReturnValue(env, val)
	case *ast.CallExpression:
		return evalCallExpression(ctx, node, env)
	case *ast.MethodCallExpression:
		return evalMethodCallExpression(ctx, node, env)
	case *ast.Identifier:
		return evalIdentifier(node, env)
	case *ast.IntegerLiteral:
		return object.NewInteger(node.Value)
	case *ast.IfStatement:
		return evalIfStatementWithContext(ctx, node, env)
	case *ast.BlockStatement:
		return evalBlockStatementWithContext(ctx, node, env)
	case *ast.Program:
		return evalProgram(ctx, node, env)
	case *ast.FloatLiteral:
		return object.NewFloat(node.Value)
	case *ast.StringLiteral:
		return object.NewString(node.Value)
	case *ast.FStringLiteral:
		return evalFStringLiteral(ctx, node, env)
	case *ast.Boolean:
		return nativeBoolToBooleanObject(node.Value)
	case *ast.None:
		return NULL
	case *ast.PrefixExpression:
		right := evalNode(ctx, node.Right, env)
		if object.IsError(right) {
			return right
		}
		return evalPrefixExpression(node.Operator, right)
	case *ast.ConditionalExpression:
		return evalConditionalExpression(ctx, node, env)
	case *ast.MatchStatement:
		return evalMatchStatementWithContext(ctx, node, env)
	case *ast.WhileStatement:
		return evalWhileStatementWithContext(ctx, node, env)
	case *ast.BreakStatement:
		return object.BREAK
	case *ast.ContinueStatement:
		return object.CONTINUE
	case *ast.PassStatement:
		return NULL
	case *ast.DelStatement:
		if err := deleteFromExpression(ctx, node.Target, env); err != nil {
			if ae, ok := err.(*assignmentExceptionError); ok {
				return ae.ex
			}
			return errors.NewError("%s", err.Error())
		}
		return NULL
	case *ast.ImportStatement:
		return evalImportStatement(node, env)
	case *ast.FromImportStatement:
		return evalFromImportStatement(node, env)
	case *ast.AssignStatement:
		val := evalNode(ctx, node.Value, env)
		if object.IsError(val) || isException(val) {
			return val
		}
		// Execute chained assignments first (a = b = 5: assign 5 to b, then to a)
		if node.Chained != nil {
			if err := assignToExpression(ctx, node.Chained.Left, val, env); err != nil {
				if ae, ok := err.(*assignmentExceptionError); ok {
					return ae.ex
				}
				return errors.NewError("%s", err.Error())
			}
			for c := node.Chained.Chained; c != nil; c = c.Chained {
				if err := assignToExpression(ctx, c.Left, val, env); err != nil {
					if ae, ok := err.(*assignmentExceptionError); ok {
						return ae.ex
					}
					return errors.NewError("%s", err.Error())
				}
			}
		}
		if err := assignToExpression(ctx, node.Left, val, env); err != nil {
			if ae, ok := err.(*assignmentExceptionError); ok {
				return ae.ex
			}
			return errors.NewError("%s", err.Error())
		}
		return NULL
	case *ast.AugmentedAssignStatement:
		return evalAugmentedAssignStatementWithContext(ctx, node, env)
	case *ast.MultipleAssignStatement:
		return evalMultipleAssignStatementWithContext(ctx, node, env)
	case *ast.FunctionStatement:
		return evalFunctionStatement(ctx, node, env)
	case *ast.ClassStatement:
		return evalClassStatement(ctx, node, env)
	case *ast.ListLiteral:
		elements := evalExpressionsWithContext(ctx, node.Elements, env)
		if len(elements) == 1 && object.IsError(elements[0]) {
			return elements[0]
		}
		return &object.List{Elements: elements}
	case *ast.DictLiteral:
		return evalDictLiteralWithContext(ctx, node, env)
	case *ast.SetLiteral:
		elements := evalExpressionsWithContext(ctx, node.Elements, env)
		if len(elements) == 1 && object.IsError(elements[0]) {
			return elements[0]
		}
		s := object.NewSet()
		for _, elem := range elements {
			if err := evalSetAdd(ctx, s, elem); err != nil {
				return err
			}
		}
		return s
	case *ast.IndexExpression:
		left := evalNode(ctx, node.Left, env)
		if object.IsError(left) {
			return left
		}
		index := evalNode(ctx, node.Index, env)
		if object.IsError(index) {
			return index
		}
		return evalIndexExpression(ctx, left, index, node.IsDotAccess)
	case *ast.SliceExpression:
		return evalSliceExpressionWithContext(ctx, node, env)
	case *ast.ForStatement:
		return evalForStatementWithContext(ctx, node, env)
	case *ast.TryStatement:
		return evalTryStatementWithContext(ctx, node, env)
	case *ast.RaiseStatement:
		return evalRaiseStatementWithContext(ctx, node, env)
	case *ast.GlobalStatement:
		return evalGlobalStatement(node, env)
	case *ast.NonlocalStatement:
		return evalNonlocalStatement(node, env)
	case *ast.AssertStatement:
		return evalAssertStatementWithContext(ctx, node, env)
	case *ast.WithStatement:
		return evalWithStatementWithContext(ctx, node, env)
	case *ast.ListComprehension:
		return evalListComprehension(ctx, node, env)
	case *ast.DictComprehension:
		return evalDictComprehension(ctx, node, env)
	case *ast.SetComprehension:
		return evalSetComprehension(ctx, node, env)
	case *ast.Lambda:
		return evalLambda(node, env)
	case *ast.TupleLiteral:
		elements := evalExpressionsWithContext(ctx, node.Elements, env)
		if len(elements) == 1 && object.IsError(elements[0]) {
			return elements[0]
		}
		return &object.Tuple{Elements: elements}
	}
	return NULL
}

func evalProgram(ctx context.Context, program *ast.Program, env *object.Environment) object.Object {
	// Set up slots for top-level variables to enable fast slot-based access.
	if slotIndex, slotNames := analyzeTopLevelLocals(program); slotIndex != nil {
		if !env.HasSlots() {
			env.SetupSlots(slotIndex, slotNames)
		} else {
			env.ExtendSlots(slotIndex, slotNames)
		}
	}

	var result object.Object = NULL
	cc := newContextChecker(ctx)

	for _, statement := range program.Statements {
		if err := cc.check(); err != nil {
			return err
		}

		result = evalNode(ctx, statement, env)

		switch result := result.(type) {
		case *object.ReturnValue:
			val := result.Value
			releaseReturnValue(result)
			return val
		case *object.Error:
			if result.Line == 0 {
				result.Line = statement.Line()
			}
			if result.File == "" {
				result.File = GetSourceFileFromContext(ctx)
			}
			return result
		case *object.Exception:
			if result.ExceptionType == object.ExceptionTypeSystemExit {
				return result
			}
			return errors.NewError("Uncaught exception: %s", result.Message)
		}
	}

	return result
}

func evalBlockStatementWithContext(ctx context.Context, block *ast.BlockStatement, env *object.Environment) object.Object {
	var result object.Object = NULL
	cc := newContextChecker(ctx)

	for _, statement := range block.Statements {
		if err := cc.check(); err != nil {
			return err
		}

		result = evalNode(ctx, statement, env)

		// Fast path: most statements return NULL, a value, or a simple type.
		// Avoid the type switch overhead for the common case.
		switch r := result.(type) {
		case *object.Null:
			// Most common: statement returns NULL, continue
		case *object.Integer, *object.String, *object.Float, *object.Boolean,
			*object.List, *object.Dict, *object.Tuple, *object.Set,
			*object.Function, *object.LambdaFunction, *object.Builtin,
			*object.Instance, *object.Class, *object.BoundMethod,
			*object.FloatArray:
			// Normal value, continue
		case *object.ReturnValue:
			return result
		case *object.Error:
			if r.Line == 0 {
				r.Line = statement.Line()
			}
			if r.File == "" {
				r.File = GetSourceFileFromContext(ctx)
			}
			return r
		case nil:
			// nil result, continue
		default:
			// Handle BREAK, CONTINUE, EXCEPTION via type method
			rt := r.Type()
			if rt == object.RETURN_OBJ || rt == object.BREAK_OBJ || rt == object.CONTINUE_OBJ || rt == object.EXCEPTION_OBJ {
				return result
			}
		}
	}

	return result
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
	case *object.Boolean:
		return av.BoolValue() == b.(*object.Boolean).BoolValue()
	case *object.Null:
		return true
	default:
		return a == b // Reference equality for complex types
	}
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
	default:
		return a == b // Reference equality for other types
	}
}

func evalPrefixExpression(operator ast.Op, right object.Object) object.Object {
	switch operator {
	case ast.OpNot:
		return evalNotOperatorExpression(right)
	case ast.OpSub:
		return evalMinusPrefixOperatorExpression(right)
	case ast.OpBitNot:
		return evalBitwiseNotOperatorExpression(right)
	default:
		return errors.NewError("%s: %s%s", errors.ErrUnknownOperator, operator, right.Type())
	}
}

func evalNotOperatorExpression(right object.Object) object.Object {
	if isTruthy(right) {
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

// evalShortCircuitInfixExpression handles and/or operators with proper short-circuit evaluation
func evalShortCircuitInfixExpression(ctx context.Context, node *ast.InfixExpression, env *object.Environment) object.Object {
	left := evalNode(ctx, node.Left, env)
	if object.IsError(left) {
		return left
	}

	switch node.Operator {
	case ast.OpAnd:
		if !isTruthy(left) {
			return left
		}
		return evalNode(ctx, node.Right, env)
	case ast.OpOr:
		if isTruthy(left) {
			return left
		}
		return evalNode(ctx, node.Right, env)
	default:
		return errors.NewError("unknown operator: %s", node.Operator)
	}
}

func evalInfixExpression(ctx context.Context, operator ast.Op, left, right object.Object, env *object.Environment) object.Object {

	switch operator {
	case ast.OpIn:
		return evalInOperator(ctx, left, right)
	case ast.OpNotIn:
		result := evalInOperator(ctx, left, right)
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
		case *object.List:
			if operator == ast.OpMul {
				if l.IntValue() <= 0 {
					return &object.List{Elements: []object.Object{}}
				}
				result := make([]object.Object, int(l.IntValue())*len(r.Elements))
				for i := range int(l.IntValue()) {
					copy(result[i*len(r.Elements):], r.Elements)
				}
				return &object.List{Elements: result}
			}
		case *object.Tuple:
			if operator == ast.OpMul {
				if l.IntValue() <= 0 {
					return &object.Tuple{Elements: []object.Object{}}
				}
				result := make([]object.Object, int(l.IntValue())*len(r.Elements))
				for i := range int(l.IntValue()) {
					copy(result[i*len(r.Elements):], r.Elements)
				}
				return &object.Tuple{Elements: result}
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
			return evalStringPercentFormat(l.StringValue(), right)
		}
		if r, ok := right.(*object.String); ok {
			return evalStringInfixExpression(operator, l.StringValue(), r.StringValue())
		}
		if r, ok := right.(*object.Integer); ok && operator == ast.OpMul {
			return evalStringMultiplication(l.StringValue(), r.IntValue())
		}
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
	case *object.Tuple:
		switch operator {
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
				if r.IntValue() <= 0 {
					return &object.Tuple{Elements: []object.Object{}}
				}
				result := make([]object.Object, int(r.IntValue())*len(l.Elements))
				for i := range int(r.IntValue()) {
					copy(result[i*len(l.Elements):], l.Elements)
				}
				return &object.Tuple{Elements: result}
			}
			return errors.NewTypeError("int", right.Type().String())
		case ast.OpEq:
			return nativeBoolToBooleanObject(objectsDeepEqual(left, right))
		case ast.OpNeq:
			return nativeBoolToBooleanObject(!objectsDeepEqual(left, right))
		}
	case *object.List:
		switch operator {
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
				if r.IntValue() <= 0 {
					return &object.List{Elements: []object.Object{}}
				}
				result := make([]object.Object, int(r.IntValue())*len(l.Elements))
				for i := range int(r.IntValue()) {
					copy(result[i*len(l.Elements):], l.Elements)
				}
				return &object.List{Elements: result}
			}
			return errors.NewTypeError("int", right.Type().String())
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
		return errors.NewError("%s: type mismatch", errors.ErrTypeError)
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

func evalConditionalExpression(ctx context.Context, node *ast.ConditionalExpression, env *object.Environment) object.Object {
	condition := evalNode(ctx, node.Condition, env)
	if object.IsError(condition) {
		return condition
	}

	if isTruthy(condition) {
		return evalNode(ctx, node.TrueExpr, env)
	} else {
		return evalNode(ctx, node.FalseExpr, env)
	}
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
			return errors.NewError(errors.ErrDivisionByZero)
		}
		return object.NewFloat(float64(leftVal) / float64(rightVal))
	case ast.OpFloorDiv:
		if rightVal == 0 {
			return errors.NewError(errors.ErrDivisionByZero)
		}
		return object.NewInteger(leftVal / rightVal)
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
			return errors.NewError(errors.ErrDivisionByZero)
		}
		return object.NewInteger(leftVal % rightVal)
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
		return errors.NewTypeError("NUMBER", left.Type().String())
	}
	rightVal, ok := numericFloatValue(right)
	if !ok {
		switch operator {
		case ast.OpEq:
			return FALSE
		case ast.OpNeq:
			return TRUE
		}
		return errors.NewTypeError("NUMBER", right.Type().String())
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
			return errors.NewError(errors.ErrDivisionByZero)
		}
		return object.NewFloat(leftVal / rightVal)
	case ast.OpFloorDiv:
		if rightVal == 0 {
			return errors.NewError(errors.ErrDivisionByZero)
		}
		return object.NewFloat(math.Floor(leftVal / rightVal))
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

func tryEvalStringConcatChain(ctx context.Context, expr *ast.InfixExpression, env *object.Environment) (object.Object, bool) {
	var leaves []ast.Expression
	if !collectStringConcatLeaves(expr, &leaves) {
		return nil, false
	}

	values := make([]object.Object, len(leaves))
	allStrings := true
	total := 0
	for i, leaf := range leaves {
		val := evalNode(ctx, leaf, env)
		if object.IsError(val) {
			return val, true
		}
		values[i] = val
		if s, ok := val.(*object.String); ok {
			total += len(s.StringValue())
		} else {
			allStrings = false
		}
	}

	if allStrings {
		var b strings.Builder
		b.Grow(total)
		for _, val := range values {
			b.WriteString(val.(*object.String).StringValue())
		}
		return object.NewString(b.String()), true
	}

	result := values[0]
	for i := 1; i < len(values); i++ {
		result = evalInfixExpression(ctx, ast.OpAdd, result, values[i], env)
		if object.IsError(result) {
			return result, true
		}
	}
	return result, true
}

func collectStringConcatLeaves(expr ast.Expression, leaves *[]ast.Expression) bool {
	infix, ok := expr.(*ast.InfixExpression)
	if ok && infix.Operator == ast.OpAdd {
		return collectStringConcatLeaves(infix.Left, leaves) &&
			collectStringConcatLeaves(infix.Right, leaves)
	}
	*leaves = append(*leaves, expr)
	return true
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
func evalStringPercentFormat(format string, right object.Object) object.Object {
	// Collect values: if right is a tuple, use its elements; otherwise single value
	var values []object.Object
	if tuple, ok := right.(*object.Tuple); ok {
		values = tuple.Elements
	} else {
		values = []object.Object{right}
	}

	var result strings.Builder
	valueIdx := 0
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

			// Parse format specifier: %[flags][width][.precision]type
			specStart := i - 1

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

			spec := format[specStart : i+1]
			conversion := format[i]
			i++

			if valueIdx >= len(values) {
				return errors.NewError("not enough arguments for format string")
			}
			val := values[valueIdx]
			valueIdx++

			formatted, err := formatPercentValue(spec, conversion, val)
			if err != nil {
				return err
			}
			result.WriteString(formatted)
		} else {
			result.WriteByte(format[i])
			i++
		}
	}

	if valueIdx < len(values) {
		return errors.NewError("not all arguments converted during string formatting")
	}

	return object.NewString(result.String())
}

// formatPercentValue formats a single value according to a Python % format specifier.
func formatPercentValue(spec string, conversion byte, val object.Object) (string, object.Object) {
	switch conversion {
	case 's':
		return val.Inspect(), nil
	case 'r':
		return fmt.Sprintf("%#v", val.Inspect()), nil
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

func evalStringMultiplication(str string, multiplier int64) object.Object {
	if multiplier < 0 {
		return object.NewString("")
	}
	return object.NewString(strings.Repeat(str, int(multiplier)))
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

func evalIfStatementWithContext(ctx context.Context, ie *ast.IfStatement, env *object.Environment) object.Object {
	condition := evalNode(ctx, ie.Condition, env)
	if object.IsError(condition) {
		return condition
	}

	if isTruthy(condition) {
		return evalBlockStatementWithContext(ctx, ie.Consequence, env)
	}

	// Check elif clauses
	for _, elifClause := range ie.ElifClauses {
		condition := evalNode(ctx, elifClause.Condition, env)
		if object.IsError(condition) {
			return condition
		}
		if isTruthy(condition) {
			return evalBlockStatementWithContext(ctx, elifClause.Consequence, env)
		}
	}

	// Check else clause
	if ie.Alternative != nil {
		return evalBlockStatementWithContext(ctx, ie.Alternative, env)
	}

	return NULL
}

func evalWhileStatementWithContext(ctx context.Context, ws *ast.WhileStatement, env *object.Environment) object.Object {
	var result object.Object = NULL
	cc := newContextChecker(ctx)
	broke := false

	for {
		if err := cc.check(); err != nil {
			return err
		}

		condition := evalNode(ctx, ws.Condition, env)
		if object.IsError(condition) {
			return condition
		}

		if !isTruthy(condition) {
			break
		}

		result = evalWithContext(ctx, ws.Body, env)
		if result != nil {
			switch result.Type() {
			case object.ERROR_OBJ:
				return result
			case object.RETURN_OBJ:
				return result
			case object.BREAK_OBJ:
				broke = true
				return NULL
			case object.CONTINUE_OBJ:
				result = NULL
				continue
			}
		}
	}

	if !broke && ws.Else != nil {
		return evalWithContext(ctx, ws.Else, env)
	}
	return result
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

func evalFunctionStatement(ctx context.Context, stmt *ast.FunctionStatement, env *object.Environment) object.Object {
	localSlots, localSlotNames := analyzeFunctionLocals(stmt)
	fn := &object.Function{
		Name:             stmt.Name.Value(),
		Parameters:       stmt.Function.Parameters,
		DefaultValues:    stmt.Function.GetDefaultValues(),
		Variadic:         stmt.Function.GetVariadic(),
		Kwargs:           stmt.Function.GetKwargs(),
		KeywordOnlyStart: stmt.Function.GetKeywordOnlyStart() + 1,
		Body:             stmt.Function.Body,
		Env:              env,
		LocalSlots:       localSlots,
		LocalSlotNames:   localSlotNames,
		ParamSlotIndexes: stmt.Function.ParamSlotIndexes,
		ReuseCallEnv:     !stmt.Function.HasNestedFunc,
	}
	var result object.Object = fn
	for i := len(stmt.GetDecorators()) - 1; i >= 0; i-- {
		dec := evalNode(ctx, stmt.GetDecorators()[i], env)
		if object.IsError(dec) {
			return dec
		}
		result = applyFunctionWithContext(ctx, dec, []object.Object{result}, nil, env)
		if object.IsError(result) {
			return result
		}
		// If the decorator returned a Function with a different name, rename it
		// so class method lookup (which keys by Function.Name) still works.
		if wrapped, ok := result.(*object.Function); ok && wrapped.Name != fn.Name {
			wrapped.Name = fn.Name
		}
	}
	env.Set(stmt.Name.Value(), result)
	return result
}

func evalClassStatement(ctx context.Context, stmt *ast.ClassStatement, env *object.Environment) object.Object {
	class := &object.Class{
		Name:    stmt.Name.Value(),
		Methods: make(map[string]object.Object),
		Env:     env,
	}

	// Handle base class inheritance
	if stmt.BaseClass != nil {
		// Evaluate the base class expression (can be dotted like html.parser.HTMLParser)
		baseClassObj := evalNode(ctx, stmt.BaseClass, env)
		if object.IsError(baseClassObj) {
			return baseClassObj
		}
		baseClass, ok := baseClassObj.(*object.Class)
		if !ok {
			return errors.NewError("base class is not a class type, got %s", baseClassObj.Type())
		}
		class.BaseClass = baseClass

		// Copy methods from base class
		for name, method := range baseClass.Methods {
			class.Methods[name] = method
		}
	}

	// Create a new environment for the class body
	classEnv := object.NewEnclosedEnvironment(env)
	classEnv.Set("__class__", class)

	// Evaluate the class body to find methods (will override inherited methods)
	for _, s := range stmt.Body.Statements {
		if fnStmt, ok := s.(*ast.FunctionStatement); ok {
			obj := evalFunctionStatement(ctx, fnStmt, classEnv)
			switch m := obj.(type) {
			case *object.Function:
				class.Methods[m.Name] = m
			case *object.Property:
				class.Methods[fnStmt.Name.Value()] = m
			case *object.StaticMethod:
				class.Methods[fnStmt.Name.Value()] = m
			case *object.ClassMethod:
				class.Methods[fnStmt.Name.Value()] = m
			default:
				// Decorator returned something other than a bare Function
				// (e.g. a wrapper closure). Store under the original method name.
				if obj != nil && !object.IsError(obj) {
					class.Methods[fnStmt.Name.Value()] = obj
				}
			}
		}
	}

	env.Set(stmt.Name.Value(), class)
	var result object.Object = class
	for i := len(stmt.GetDecorators()) - 1; i >= 0; i-- {
		dec := evalNode(ctx, stmt.GetDecorators()[i], env)
		if object.IsError(dec) {
			return dec
		}
		result = applyFunctionWithContext(ctx, dec, []object.Object{result}, nil, env)
		if object.IsError(result) {
			return result
		}
	}
	if result != class {
		env.Set(stmt.Name.Value(), result)
	}
	return result
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

func evalCallExpression(ctx context.Context, node *ast.CallExpression, env *object.Environment) object.Object {
	if !node.HasOverflow() {
		if ident, ok := node.Function.(*ast.Identifier); ok {
			name := ident.Value()
			if val, found := resolveCallee(node, name, env); found {
				switch fn := val.(type) {
				case *object.Function:
					// Fast paths for common arg counts: avoid slice allocation
					nargs := len(node.Arguments)
					nparams := len(fn.Parameters)
					if fn.Variadic == nil && fn.Kwargs == nil && len(fn.DefaultValues) == 0 && nargs == nparams && nargs <= 3 {
						switch nargs {
						case 1:
							a0 := evalNode(ctx, node.Arguments[0], env)
							if object.IsError(a0) {
								return a0
							}
							return applyUserFunctionDirect(ctx, fn, a0)
						case 2:
							a0 := evalNode(ctx, node.Arguments[0], env)
							if object.IsError(a0) {
								return a0
							}
							a1 := evalNode(ctx, node.Arguments[1], env)
							if object.IsError(a1) {
								return a1
							}
							return applyUserFunction2(ctx, fn, a0, a1)
						case 3:
							a0 := evalNode(ctx, node.Arguments[0], env)
							if object.IsError(a0) {
								return a0
							}
							a1 := evalNode(ctx, node.Arguments[1], env)
							if object.IsError(a1) {
								return a1
							}
							a2 := evalNode(ctx, node.Arguments[2], env)
							if object.IsError(a2) {
								return a2
							}
							return applyUserFunctionN(ctx, fn, a0, a1, a2)
						}
					}
					args := evalArgsFast(ctx, node.Arguments, env)
					if len(args) == 1 && object.IsError(args[0]) {
						return args[0]
					}
					return applyUserFunction(ctx, fn, args, nil, env)
				case *object.Builtin:
					// Use the existing fast builtin path
					return applyBuiltinFast(ctx, node, env, fn)
				case *object.LambdaFunction:
					args := evalArgsFast(ctx, node.Arguments, env)
					if len(args) == 1 && object.IsError(args[0]) {
						return args[0]
					}
					return applyLambdaFunctionWithContext(ctx, fn, args, nil, env)
				}
				// Class or other callable - fall through to general path
				return applyFunctionWithContext(ctx, val,
					evalExpressionsWithContext(ctx, node.Arguments, env), nil, env)
			}
			// Not in env - try fast builtins by name
			if builtin, ok := builtins[name]; ok {
				return applyBuiltinFast(ctx, node, env, builtin)
			}
		}
	}

	// General path for complex call expressions
	function := evalNode(ctx, node.Function, env)
	if object.IsError(function) {
		return function
	}

	args := evalExpressionsWithContext(ctx, node.Arguments, env)
	if len(args) == 1 && object.IsError(args[0]) {
		return args[0]
	}

	var keywords map[string]object.Object
	keywordsMap := node.GetKeywords()
	if len(keywordsMap) > 0 {
		keywords = make(map[string]object.Object, len(keywordsMap))
		for k, v := range keywordsMap {
			val := evalNode(ctx, v, env)
			if object.IsError(val) {
				return val
			}
			keywords[k] = val
		}
	}

	for _, argsUnpackExpr := range node.GetArgsUnpack() {
		argsVal := evalNode(ctx, argsUnpackExpr, env)
		if object.IsError(argsVal) {
			return argsVal
		}
		unpacked, err := unpackArgsFromIterable(argsVal)
		if err != nil {
			return err
		}
		args = append(args, unpacked...)
	}

	kwargsUnpack := node.GetKwargsUnpack()
	if kwargsUnpack != nil {
		kwargsVal := evalNode(ctx, kwargsUnpack, env)
		if object.IsError(kwargsVal) {
			return kwargsVal
		}
		if dict, ok := kwargsVal.(*object.Dict); ok {
			if keywords == nil {
				keywords = make(map[string]object.Object, len(dict.Pairs))
			}
			for _, pair := range dict.Pairs {
				keywords[pair.StringKey()] = pair.Value
			}
		} else {
			return errors.NewError("argument after ** must be a dictionary, not %s", kwargsVal.Type())
		}
	}

	return applyFunctionWithContext(ctx, function, args, keywords, env)
}

// evalArgsFast evaluates call arguments with reduced allocation overhead.
func evalArgsFast(ctx context.Context, exps []ast.Expression, env *object.Environment) []object.Object {
	n := len(exps)
	if n == 0 {
		return nil
	}
	// For single-arg calls (the most common case for recursive functions),
	// avoid the slice allocation by using a stack-allocated array.
	if n == 1 {
		result := evalNode(ctx, exps[0], env)
		// Use a fixed-size array to avoid heap allocation for the slice
		return []object.Object{result}
	}
	result := make([]object.Object, n)
	for i, e := range exps {
		evaluated := evalNode(ctx, e, env)
		if object.IsError(evaluated) {
			return []object.Object{evaluated}
		}
		result[i] = evaluated
	}
	return result
}

// applyBuiltinFast dispatches builtin calls, matching the fast builtin path.
func applyBuiltinFast(ctx context.Context, node *ast.CallExpression, env *object.Environment, fn *object.Builtin) object.Object {
	args := evalExpressionsWithContext(ctx, node.Arguments, env)
	if len(args) == 1 && object.IsError(args[0]) {
		return args[0]
	}
	ctxWithEnv := SetEnvInContext(ctx, env)
	return fn.Fn(ctxWithEnv, object.Kwargs{}, args...)
}

// tryEvalFastBuiltinCall handles fast-path builtin calls (len, type, str, etc.).
// Returns (result, envFn, ok):
//   - ok=true:   result is the builtin's return value, envFn is nil.
//   - ok=false, envFn!=nil: name was found in the environment (not a builtin),
//     envFn holds the resolved value so the caller can skip a redundant lookup.
//   - ok=false, envFn==nil: not applicable, caller should use normal resolution.

func createInstance(ctx context.Context, class *object.Class, args []object.Object, keywords map[string]object.Object, env *object.Environment) object.Object {
	instance := &object.Instance{
		Class:  class,
		Fields: make(map[string]object.Object),
	}

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
		if object.IsError(result) {
			return result
		}
	}

	// Install GC finalizer for __del__ if the class defines one
	if del, ok := class.LookupMember("__del__"); ok {
		delMethod := del
		runtime.SetFinalizer(instance, func(inst *object.Instance) {
			applyFunction(context.Background(), delMethod, []object.Object{inst}, nil, object.NewEnvironment())
		})
	}

	return instance
}

func evalExpressionsWithContext(ctx context.Context, exps []ast.Expression, env *object.Environment) []object.Object {
	if len(exps) == 0 {
		return nil
	}
	result := make([]object.Object, len(exps))

	for i, e := range exps {
		evaluated := evalNode(ctx, e, env)
		if object.IsError(evaluated) {
			return []object.Object{evaluated}
		}
		result[i] = evaluated
	}

	return result
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

	// Call evalBlockStatementWithContext directly, skipping evalWithContext/evalNode overhead
	// since fn.Body is always a *ast.BlockStatement and block already handles errors.
	evaluated := evalBlockStatementWithContext(ctx, fn.Body, extendedEnv)
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

	evaluated := evalBlockStatementWithContext(ctx, fn.Body, extendedEnv)
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

	evaluated := evalBlockStatementWithContext(ctx, fn.Body, extendedEnv)
	if err, ok := evaluated.(*object.Error); ok {
		if err.Function == "" {
			err.Function = fn.Name
		}
	}
	return unwrapReturnValue(evaluated)
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

	evaluated := evalWithContext(ctx, fn.Body, extendedEnv)
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
	ctx, release := enterScript(ctx, env)
	defer release()
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
	default:
		return errors.NewError("not a function or class: %s", fn.Type())
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

	evaluated := evalWithContext(ctx, fn.Body, extendedEnv)
	return evaluated // No unwrapping needed for lambda expressions
}

// funcParams abstracts the common parts of Function and LambdaFunction for parameter handling
type funcParams struct {
	parameters       []*ast.Identifier
	defaultValues    map[string]ast.Expression
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
			// Collect extra arguments into a list
			list := &object.List{Elements: args[positionalLimit:]}
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
					defaultVal := evalNode(ctx, defaultExpr, fp.parentEnv)
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
					defaultVal := evalNode(ctx, defaultExpr, fp.parentEnv)
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
		case *object.List:
			return len(v.Elements) > 0
		case *object.Dict:
			return len(v.Pairs) > 0
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

// findDunderMethod looks up a dunder method in the instance's class hierarchy
func findDunderMethod(inst *object.Instance, method string) (object.Object, bool) {
	return inst.Class.LookupMember(method)
}

func init() {
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

func evalAugmentedAssignStatementWithContext(ctx context.Context, node *ast.AugmentedAssignStatement, env *object.Environment) object.Object {
	left := node.Left
	if left == nil {
		left = node.Name
	}
	if left == nil {
		return errors.NewError("invalid augmented assignment target")
	}

	currentVal := evalNode(ctx, left, env)
	if object.IsError(currentVal) || isException(currentVal) {
		return currentVal
	}

	newVal := evalNode(ctx, node.Value, env)
	if object.IsError(newVal) || isException(newVal) {
		return newVal
	}

	// Fast path: string += string, int += int
	if node.Operator == ast.OpAddEq {
		if cur, ok := currentVal.(*object.String); ok {
			if r, ok := newVal.(*object.String); ok {
				if err := assignToExpression(ctx, left, object.NewString(cur.StringValue()+r.StringValue()), env); err != nil {
					if ae, ok := err.(*assignmentExceptionError); ok {
						return ae.ex
					}
					return errors.NewError("%s", err.Error())
				}
				return NULL
			}
		}
		if cur, ok := currentVal.(*object.Integer); ok {
			if r, ok := newVal.(*object.Integer); ok {
				if err := assignToExpression(ctx, left, object.NewInteger(cur.IntValue()+r.IntValue()), env); err != nil {
					if ae, ok := err.(*assignmentExceptionError); ok {
						return ae.ex
					}
					return errors.NewError("%s", err.Error())
				}
				return NULL
			}
		}
	}

	operator := node.Operator.BaseOp()
	if operator == node.Operator {
		return errors.NewError("unknown augmented assignment operator: %s", node.Operator)
	}

	result := evalInfixExpression(ctx, operator, currentVal, newVal, env)
	if object.IsError(result) {
		return result
	}

	if err := assignToExpression(ctx, left, result, env); err != nil {
		if ae, ok := err.(*assignmentExceptionError); ok {
			return ae.ex
		}
		return errors.NewError("%s", err.Error())
	}
	return NULL
}

// evalSliceExpressionWithContext is in data_structures.go
// sliceList is in data_structures.go
// sliceString is in data_structures.go

func evalImportStatement(is *ast.ImportStatement, env *object.Environment) object.Object {
	importCallback := env.GetImportCallback()
	if importCallback == nil {
		return errors.NewError("%s at line %d", errors.ErrImportError, is.Token.Line)
	}
	err := importCallback(is.Name.Value())
	if err != nil {
		return errors.NewError("%s at line %d: %s", errors.ErrImportError, is.Token.Line, err.Error())
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
		if err := importCallback(name.Value()); err != nil {
			return errors.NewError("%s: %s", errors.ErrImportError, err.Error())
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

func evalFromImportStatement(fis *ast.FromImportStatement, env *object.Environment) object.Object {
	importCallback := env.GetImportCallback()
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
		return evalFromImportMultipleSubmodules(fis, baseModuleName, env, importCallback)
	}

	// Standard from-import: import the module and extract names
	return evalFromImportStandard(fis, baseModuleName, env, importCallback)
}

// evalFromImportMultipleSubmodules handles "from . import X, Y" where each name is a submodule
func evalFromImportMultipleSubmodules(fis *ast.FromImportStatement, baseModule string, env *object.Environment, importCallback func(string) error) object.Object {
	for i, name := range fis.Names {
		// Build the full module name: base + "." + name
		fullModuleName := baseModule + "." + name.Value()

		// Import the submodule
		err := importCallback(fullModuleName)
		if err != nil {
			return errors.NewError("%s: %s", errors.ErrImportError, err.Error())
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
func evalFromImportStandard(fis *ast.FromImportStatement, moduleName string, env *object.Environment, importCallback func(string) error) object.Object {
	// Check if module was already in the environment before importing
	// (e.g. user did `import json` before `from json import dumps`)
	_, wasPresent := env.Get(moduleName)

	// Import the module
	err := importCallback(moduleName)
	if err != nil {
		return errors.NewError("%s: %s", errors.ErrImportError, err.Error())
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
			if field, exists := m.Fields[name.Value()]; exists {
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

func evalInOperator(ctx context.Context, left, right object.Object) object.Object {
	switch container := right.(type) {
	case *object.List:
		for _, elem := range container.Elements {
			if left == elem || objectsDeepEqual(left, elem) {
				return TRUE
			}
		}
		return FALSE
	case *object.Tuple:
		for _, elem := range container.Elements {
			if left == elem || objectsDeepEqual(left, elem) {
				return TRUE
			}
		}
		return FALSE
	case *object.Dict:
		key := evalHashKey(ctx, left)
		_, ok := container.Pairs[key]
		return nativeBoolToBooleanObject(ok)
	case *object.String:
		if needle, ok := left.(*object.String); ok {
			return nativeBoolToBooleanObject(strings.Contains(container.StringValue(), needle.StringValue()))
		}
		return errors.NewTypeError("STRING", "non-string type")
	case *object.DictKeys:
		key := evalHashKey(ctx, left)
		_, ok := container.Dict.Pairs[key]
		return nativeBoolToBooleanObject(ok)
	case *object.DictValues:
		for _, pair := range container.Dict.Pairs {
			if left == pair.Value || objectsDeepEqual(left, pair.Value) {
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
			keyStr := evalHashKey(ctx, key)
			if pair, ok := container.Dict.Pairs[keyStr]; ok {
				if val == pair.Value || objectsDeepEqual(val, pair.Value) {
					return TRUE
				}
			}
		}
		return FALSE
	case *object.Set:
		return nativeBoolToBooleanObject(container.ContainsKeyed(evalHashKey(ctx, left)))
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
			if object.IsError(result) {
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

func evalMultipleAssignStatementWithContext(ctx context.Context, node *ast.MultipleAssignStatement, env *object.Environment) object.Object {
	val := evalNode(ctx, node.Value, env)
	if object.IsError(val) {
		return val
	}

	var elements []object.Object

	// Value can be a list or tuple
	switch v := val.(type) {
	case *object.List:
		elements = v.Elements
	case *object.Tuple:
		elements = v.Elements
	default:
		return errors.NewTypeError("list or tuple", val.Type().String())
	}

	// Handle starred unpacking
	if node.StarredIndex >= 0 {
		// With starred unpacking: a, *b, c = [1, 2, 3, 4, 5]
		// Need at least (len(names) - 1) elements
		minElements := len(node.Names) - 1
		if len(elements) < minElements {
			return errors.NewError("not enough values to unpack (expected at least %d, got %d)", minElements, len(elements))
		}

		// Assign elements before the starred variable
		for i := 0; i < node.StarredIndex; i++ {
			env.Set(node.Names[i].Value(), elements[i])
		}

		// Calculate how many elements go to the starred variable
		elementsAfterStar := len(node.Names) - node.StarredIndex - 1
		starStart := node.StarredIndex
		starEnd := len(elements) - elementsAfterStar

		// Assign starred variable (as a list)
		starredElements := elements[starStart:starEnd]
		env.Set(node.Names[node.StarredIndex].Value(), &object.List{Elements: starredElements})

		// Assign elements after the starred variable
		for i := 0; i < elementsAfterStar; i++ {
			nameIdx := node.StarredIndex + 1 + i
			elemIdx := starEnd + i
			env.Set(node.Names[nameIdx].Value(), elements[elemIdx])
		}
	} else {
		// No starred unpacking - exact length match required
		if len(elements) != len(node.Names) {
			return errors.NewError("cannot unpack %d values to %d variables", len(elements), len(node.Names))
		}

		// Assign each value
		for i, name := range node.Names {
			env.Set(name.Value(), elements[i])
		}
	}

	return NULL
}

func evalTryStatementWithContext(ctx context.Context, ts *ast.TryStatement, env *object.Environment) object.Object {
	// Execute try block
	result := evalWithContext(ctx, ts.Body, env)

	exceptionCaught := false

	// Check if exception or error occurred
	if isException(result) || object.IsError(result) {
		// SystemExit exceptions should NOT be caught by except blocks
		// sys.exit() always exits the program, regardless of try/except
		// PermissionError exceptions also bypass try/except — security violations
		// must not be silently swallowed by scripts.
		if exc, ok := result.(*object.Exception); ok && (exc.IsSystemExit() || exc.IsPermissionError()) {
			// Execute finally block before propagating
			if ts.Finally != nil {
				if finallyResult := evalWithContext(ctx, ts.Finally, env); finallyResult != nil {
					if rv, ok := finallyResult.(*object.ReturnValue); ok {
						result = unwrapReturnValue(rv)
					}
				}
			}
			return result // always propagates
		}

		// Convert Error to Exception for consistent handling (do this once, before matching)
		var exceptionObj object.Object = result
		if err, ok := result.(*object.Error); ok {
			// Try to infer exception type from error message
			exceptionType := object.ExceptionTypeException
			msg := err.Message
			if strings.HasPrefix(msg, "type error:") || strings.Contains(msg, "type mismatch") {
				exceptionType = object.ExceptionTypeTypeError
			} else if strings.Contains(msg, "value error") || strings.Contains(msg, "invalid value") {
				exceptionType = object.ExceptionTypeValueError
			} else if strings.Contains(msg, "identifier not found") || strings.Contains(msg, "name") && strings.Contains(msg, "not defined") {
				exceptionType = object.ExceptionTypeNameError
			} else if strings.HasPrefix(msg, errors.ErrImportError) {
				exceptionType = object.ExceptionTypeImportError
			}
			exceptionObj = &object.Exception{
				Message:       msg,
				ExceptionType: exceptionType,
			}
		}

		// Try each except clause in order
		for _, exceptClause := range ts.ExceptClauses {
			// Check if exception type matches (if specified)
			if exceptClause.ExceptType != nil {
				if !matchesExceptionType(exceptionObj, exceptClause.ExceptType, env) {
					// Exception type doesn't match, try next except clause
					continue
				}
			}

			// This except clause matches - execute it
			exceptionCaught = true
			// Store the current exception for bare raise support
			env.Set("__current_exception__", exceptionObj)

			// Bind exception to variable if specified
			if exceptClause.ExceptVar != nil {
				env.Set(exceptClause.ExceptVar.Value(), exceptionObj)
			}

			// Execute except block in the same environment so variables are accessible
			result = evalWithContext(ctx, exceptClause.Body, env)

			// Clear the current exception after except block
			env.Delete("__current_exception__")

			// If except block didn't re-raise, the exception was handled.
			// Preserve control-flow signals (return, break, continue) so they
			// propagate correctly out of the try/except.
			if !isException(result) && !object.IsError(result) {
				switch result.(type) {
				case *object.ReturnValue, *object.Break, *object.Continue:
					// keep result as-is
				default:
					result = NULL
				}
			}

			// Exception was handled (or re-raised), don't try other except clauses
			break
		}
	}

	// Execute else block only if no exception was raised (and not re-raised)
	if ts.Else != nil && !exceptionCaught && !isException(result) && !object.IsError(result) {
		switch result.(type) {
		case *object.ReturnValue, *object.Break, *object.Continue:
			// don't run else on control flow
		default:
			result = evalWithContext(ctx, ts.Else, env)
		}
	}

	// Always execute finally block if present
	// Per Python semantics, return in finally overrides the result.
	if ts.Finally != nil {
		if finallyResult := evalWithContext(ctx, ts.Finally, env); finallyResult != nil {
			if rv, ok := finallyResult.(*object.ReturnValue); ok {
				result = unwrapReturnValue(rv)
			}
		}
	}

	return result
}

func evalRaiseStatementWithContext(ctx context.Context, rs *ast.RaiseStatement, env *object.Environment) object.Object {
	if rs.Message != nil {
		msg := evalNode(ctx, rs.Message, env)
		if object.IsError(msg) {
			return msg
		}
		// If it's already an Exception, return it as-is
		if exc, ok := msg.(*object.Exception); ok {
			return exc
		}
		// Python 3 doesn't support raise "string", only raise Exception("string")
		return errors.NewError("exceptions must derive from BaseException")
	}

	// Bare raise - re-raise the current exception if one exists
	if currentExc, ok := env.Get("__current_exception__"); ok {
		return currentExc
	}

	// No current exception - error
	return errors.NewError("No active exception to re-raise")
}

func evalAssertStatementWithContext(ctx context.Context, as *ast.AssertStatement, env *object.Environment) object.Object {
	condition := evalNode(ctx, as.Condition, env)
	if object.IsError(condition) {
		return condition
	}

	if !isTruthy(condition) {
		var message string
		if as.Message != nil {
			msg := evalNode(ctx, as.Message, env)
			if object.IsError(msg) {
				return msg
			}
			message = msg.Inspect()
		} else {
			message = "AssertionError"
		}
		return &object.Error{Message: fmt.Sprintf("AssertionError at line %d: %s", as.Token.Line, message)}
	}

	return NULL
}

func evalWithStatementWithContext(ctx context.Context, ws *ast.WithStatement, env *object.Environment) object.Object {
	// Evaluate the context expression
	ctxObj := evalNode(ctx, ws.ContextExpr, env)
	if object.IsError(ctxObj) {
		return ctxObj
	}

	// Call __enter__
	var enterResult object.Object
	if inst, ok := ctxObj.(*object.Instance); ok {
		enterResult = callDunderMethod(ctx, inst, "__enter__", nil, env)
		if enterResult == nil {
			enterResult = NULL
		}
		if object.IsError(enterResult) {
			return enterResult
		}
	} else {
		return errors.NewError("with statement requires an object with __enter__ and __exit__ methods")
	}

	// Bind 'as' target if present
	if ws.Target != nil {
		env.Set(ws.Target.Value(), enterResult)
	}

	// Execute body
	result := evalWithContext(ctx, ws.Body, env)

	// Call __exit__ — always, even on exception
	// __exit__(exc_type, exc_val, exc_tb) — pass None, None, None on success
	// or exception info on error. If __exit__ returns truthy, suppress the exception.
	inst := ctxObj.(*object.Instance)
	var excType object.Object = NULL
	var excVal object.Object = NULL
	if isException(result) || object.IsError(result) {
		if exc, ok := result.(*object.Exception); ok {
			excType = object.NewString(exc.ExceptionType)
			excVal = object.NewString(exc.Message)
		} else if err, ok := result.(*object.Error); ok {
			excType = object.NewString("Exception")
			excVal = object.NewString(err.Message)
		}
	}
	exitArgs := []object.Object{excType, excVal, NULL}

	exitResult := callDunderMethod(ctx, inst, "__exit__", exitArgs, env)
	if exitResult != nil && object.IsError(exitResult) {
		return exitResult
	}

	// If body raised and __exit__ returned truthy, suppress the exception
	if (isException(result) || object.IsError(result)) && exitResult != nil && isTruthy(exitResult) {
		return NULL
	}

	return result
}

func isException(obj object.Object) bool {
	// Concrete type assertion avoids a non-inlinable interface method call
	// (obj.Type()) on this hot path; only *object.Exception has EXCEPTION_OBJ.
	_, ok := obj.(*object.Exception)
	return ok
}

// matchesExceptionType checks if an exception matches the specified exception type
// Supports: Exception (catches all), specific types (ValueError, TypeError, etc.),
// and dotted names (requests.HTTPError)
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

func exceptionDeleteError(exceptionType, message string) error {
	return &assignmentExceptionError{
		ex: &object.Exception{
			Message:       message,
			ExceptionType: exceptionType,
		},
	}
}

func evalSliceObjectWithContext(ctx context.Context, node *ast.SliceExpression, env *object.Environment) (*object.Slice, object.Object) {
	sliceObj := &object.Slice{}

	if node.Start != nil {
		startObj := evalNode(ctx, node.Start, env)
		if object.IsError(startObj) || isException(startObj) {
			return nil, startObj
		}
		start, err := startObj.AsInt()
		if err != nil {
			return nil, err
		}
		sliceObj.Start = object.NewInteger(start)
	}

	if node.End != nil {
		endObj := evalNode(ctx, node.End, env)
		if object.IsError(endObj) || isException(endObj) {
			return nil, endObj
		}
		end, err := endObj.AsInt()
		if err != nil {
			return nil, err
		}
		sliceObj.End = object.NewInteger(end)
	}

	if node.GetStep() != nil {
		stepObj := evalNode(ctx, node.GetStep(), env)
		if object.IsError(stepObj) || isException(stepObj) {
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
		obj := evalNode(ctx, target.Left, env)
		if object.IsError(obj) {
			return fmt.Errorf("deletion error")
		}
		if isException(obj) {
			return &assignmentExceptionError{ex: obj.(*object.Exception)}
		}

		index := evalNode(ctx, target.Index, env)
		if object.IsError(index) {
			return fmt.Errorf("deletion error")
		}
		if isException(index) {
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
				return exceptionDeleteError(object.ExceptionTypeIndexError, "list index out of range")
			}
			o.Elements = append(o.Elements[:i], o.Elements[i+1:]...)
			return nil
		case *object.Dict:
			key := evalHashKey(ctx, index)
			if _, ok := o.Pairs[key]; !ok {
				return exceptionDeleteError(object.ExceptionTypeKeyError, index.Inspect())
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
					if isException(result) {
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
			if _, exists := o.Fields[key.StringValue()]; exists {
				delete(o.Fields, key.StringValue())
				o.InvalidateBoundMethod(key.StringValue())
				return nil
			}
			if findPropertyInClass(key.StringValue(), o.Class) != nil {
				return exceptionDeleteError(object.ExceptionTypeAttributeError, fmt.Sprintf("can't delete attribute '%s'", key.StringValue()))
			}
			return exceptionDeleteError(object.ExceptionTypeAttributeError, fmt.Sprintf("'%s' object has no attribute '%s'", o.Class.Name, key.StringValue()))
		case *object.Class:
			key, ok := index.(*object.String)
			if !ok {
				return fmt.Errorf("class attribute must be string")
			}
			if _, exists := o.Methods[key.StringValue()]; !exists {
				return exceptionDeleteError(object.ExceptionTypeAttributeError, fmt.Sprintf("type object '%s' has no attribute '%s'", o.Name, key.StringValue()))
			}
			delete(o.Methods, key.StringValue())
			o.InvalidateLookupCache()
			return nil
		default:
			return fmt.Errorf("cannot delete index")
		}
	case *ast.SliceExpression:
		obj := evalNode(ctx, target.Left, env)
		if object.IsError(obj) {
			return fmt.Errorf("deletion error")
		}
		if isException(obj) {
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
				if isException(result) {
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
		obj := evalWithContext(ctx, left.Left, env)
		if object.IsError(obj) {
			return fmt.Errorf("assignment error")
		}
		index := evalNode(ctx, left.Index, env)
		if object.IsError(index) {
			return fmt.Errorf("assignment error")
		}
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
					return fmt.Errorf("index out of range")
				}
				o.Elements[i] = value
				return nil
			}
		case *object.Dict:
			key := evalHashKey(ctx, index)
			o.Pairs[key] = object.DictPair{Key: index, Value: value}
			return nil
		case *object.Instance:
			// For explicit bracket access (not dot), call __setitem__ if defined
			if !left.IsDotAccess {
				if setitem, ok := o.Class.Methods["__setitem__"]; ok {
					result := applyFunctionWithContext(ctx, setitem, []object.Object{obj, index, value}, nil, nil)
					if object.IsError(result) {
						return fmt.Errorf("%s", result.(*object.Error).Message)
					}
					if isException(result) {
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
					if isException(result) {
						return &assignmentExceptionError{ex: result.(*object.Exception)}
					}
					return nil
				}
				o.Fields[key.StringValue()] = value
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

	baseObj := evalWithContext(ctx, rowExpr.Left, env)
	if object.IsError(baseObj) {
		return fmt.Errorf("assignment error")
	}
	fa, ok := baseObj.(*object.FloatArray)
	if !ok || !fa.Is2D() {
		return errNotNestedFloatArrayAssignment
	}

	rowIndexObj := evalWithContext(ctx, rowExpr.Index, env)
	if object.IsError(rowIndexObj) {
		return fmt.Errorf("assignment error")
	}
	rowIndex, ok := rowIndexObj.(*object.Integer)
	if !ok {
		return fmt.Errorf("float_array index must be integer")
	}

	colIndexObj := evalWithContext(ctx, expr.Index, env)
	if object.IsError(colIndexObj) {
		return fmt.Errorf("assignment error")
	}
	colIndex, ok := colIndexObj.(*object.Integer)
	if !ok {
		return fmt.Errorf("float_array index must be integer")
	}

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

// instanceToIterator wraps an instance with __next__ as an object.Iterator
func instanceToIterator(ctx context.Context, inst *object.Instance, env *object.Environment) *object.Iterator {
	return object.NewIterator(func() (object.Object, bool) {
		result := callDunderMethod(ctx, inst, "__next__", nil, env)
		if result == nil {
			return nil, false
		}
		// StopIteration is signalled by returning an Exception with type StopIteration
		if exc, ok := result.(*object.Exception); ok && exc.ExceptionType == object.ExceptionTypeStopIteration {
			return nil, false
		}
		if object.IsError(result) {
			return nil, false
		}
		return result, true
	})
}

func evalForStatementWithContext(ctx context.Context, fs *ast.ForStatement, env *object.Environment) object.Object {
	if result, ok := evalFastRangeForStatement(ctx, fs, env); ok {
		return result
	}

	iterable := evalNode(ctx, fs.Iterable, env)
	if object.IsError(iterable) {
		return iterable
	}

	var result object.Object = NULL
	broke := false

	// Handle Iterator objects and Views
	var iter *object.Iterator
	switch o := iterable.(type) {
	case *object.Iterator:
		iter = o
	case *object.DictKeys:
		iter = o.CreateIterator()
	case *object.DictValues:
		iter = o.CreateIterator()
	case *object.DictItems:
		iter = o.CreateIterator()
	case *object.Set:
		iter = o.CreateIterator()
	case *object.Instance:
		if fn, ok := findDunderMethod(o, "__iter__"); ok {
			iterObj := applyFunctionWithContext(ctx, fn, prependSelf(o, nil), nil, env)
			if object.IsError(iterObj) {
				return iterObj
			}
			if iterInst, ok := iterObj.(*object.Instance); ok {
				iter = instanceToIterator(ctx, iterInst, env)
			} else if iterIter, ok := iterObj.(*object.Iterator); ok {
				iter = iterIter
			} else {
				return errors.NewError("__iter__ must return an iterator")
			}
		} else {
			return errors.NewTypeError("iterable", iterable.Type().String())
		}
	}

	if iter != nil {
		cc := newContextChecker(ctx)
		for {
			// Check context periodically in loops for responsiveness
			if err := cc.check(); err != nil {
				return err
			}

			element, hasNext := iter.Next()
			if !hasNext {
				break
			}

			if err := setForVariables(fs.Variables, element, env); err != nil {
				return errors.NewError("%s", err.Error())
			}

			result = evalBlockStatementWithContext(ctx, fs.Body, env)
			if result != nil {
				switch result.Type() {
				case object.ERROR_OBJ, object.RETURN_OBJ:
					return result
				case object.BREAK_OBJ:
					broke = true
					result = NULL
					goto forDone
				case object.CONTINUE_OBJ:
					result = NULL
					continue
				}
			}
		}
		goto forDone
	}

	// Get elements to iterate over based on type
	{
		var elements []object.Object
		switch o := iterable.(type) {
		case *object.List:
			elements = o.Elements
		case *object.Tuple:
			elements = o.Elements
		case *object.FloatArray:
			if o.Is2D() {
				rows := o.Rows()
				cols := o.Cols()
				cc := newContextChecker(ctx)
				for i := 0; i < rows; i++ {
					if err := cc.check(); err != nil {
						return err
					}
					off := i * cols
					rowData := make([]float64, cols)
					copy(rowData, o.Data[off:off+cols])
					element := object.NewFloatArray1D(rowData)
					if err := setForVariables(fs.Variables, element, env); err != nil {
						return errors.NewError("%s", err.Error())
					}
					result = evalBlockStatementWithContext(ctx, fs.Body, env)
					if result != nil {
						switch result.Type() {
						case object.ERROR_OBJ, object.RETURN_OBJ:
							return result
						case object.BREAK_OBJ:
							broke = true
							result = NULL
							goto forDone
						case object.CONTINUE_OBJ:
							result = NULL
							continue
						}
					}
				}
				goto forDone
			}
			cc := newContextChecker(ctx)
			for _, v := range o.Data {
				if err := cc.check(); err != nil {
					return err
				}
				element := object.NewFloat(v)
				if err := setForVariables(fs.Variables, element, env); err != nil {
					return errors.NewError("%s", err.Error())
				}
				result = evalBlockStatementWithContext(ctx, fs.Body, env)
				if result != nil {
					switch result.Type() {
					case object.ERROR_OBJ, object.RETURN_OBJ:
						return result
					case object.BREAK_OBJ:
						broke = true
						result = NULL
						goto forDone
					case object.CONTINUE_OBJ:
						result = NULL
						continue
					}
				}
			}
			goto forDone
		case *object.String:
			// Iterate over string runes lazily to avoid pre-allocating all characters
			cc := newContextChecker(ctx)
			for _, char := range o.StringValue() {
				if err := cc.check(); err != nil {
					return err
				}

				element := object.NewString(string(char))
				if err := setForVariables(fs.Variables, element, env); err != nil {
					return errors.NewError("%s", err.Error())
				}

				result = evalBlockStatementWithContext(ctx, fs.Body, env)
				if result != nil {
					switch result.Type() {
					case object.ERROR_OBJ, object.RETURN_OBJ:
						return result
					case object.BREAK_OBJ:
						broke = true
						result = NULL
						goto forDone
					case object.CONTINUE_OBJ:
						result = NULL
						continue
					}
				}
			}
			goto forDone
		default:
			return errors.NewTypeError("iterable", iterable.Type().String())
		}

		// Single loop for all iterable types
		cc := newContextChecker(ctx)
		for _, element := range elements {
			if err := cc.check(); err != nil {
				return err
			}

			if err := setForVariables(fs.Variables, element, env); err != nil {
				return errors.NewError("%s", err.Error())
			}

			result = evalBlockStatementWithContext(ctx, fs.Body, env)
			if result != nil {
				switch result.Type() {
				case object.ERROR_OBJ, object.RETURN_OBJ:
					return result
				case object.BREAK_OBJ:
					broke = true
					result = NULL
					goto forDone
				case object.CONTINUE_OBJ:
					result = NULL
					continue
				}
			}
		}
	}

forDone:
	if !broke && fs.Else != nil {
		return evalBlockStatementWithContext(ctx, fs.Else, env)
	}
	return result
}

func evalFastRangeForStatement(ctx context.Context, fs *ast.ForStatement, env *object.Environment) (object.Object, bool) {
	if len(fs.Variables) != 1 || fs.Else != nil {
		return nil, false
	}
	target, ok := fs.Variables[0].(*ast.Identifier)
	if !ok {
		return nil, false
	}
	call, ok := fs.Iterable.(*ast.CallExpression)
	if !ok || call.HasOverflow() {
		return nil, false
	}
	fnIdent, ok := call.Function.(*ast.Identifier)
	if !ok || fnIdent.Value() != "range" {
		return nil, false
	}
	// If range is shadowed in the environment, preserve the normal call path.
	if _, shadowed := env.Get("range"); shadowed {
		return nil, false
	}

	start, stop, step, errObj, ok := evalRangeArgs(ctx, call.Arguments, env)
	if !ok {
		return nil, false
	}
	if errObj != nil {
		return errObj, true
	}

	var result object.Object = NULL
	cc := newContextChecker(ctx)
	for i := start; ; i += step {
		if step > 0 {
			if i >= stop {
				break
			}
		} else if i <= stop {
			break
		}

		if err := cc.check(); err != nil {
			return err, true
		}

		setIdentifierFast(target, object.NewInteger(i), env)
		result = evalBlockStatementWithContext(ctx, fs.Body, env)
		if result != nil {
			switch result.Type() {
			case object.ERROR_OBJ, object.RETURN_OBJ:
				return result, true
			case object.BREAK_OBJ:
				return NULL, true
			case object.CONTINUE_OBJ:
				result = NULL
				continue
			}
		}
	}
	return result, true
}

func evalRangeArgs(ctx context.Context, args []ast.Expression, env *object.Environment) (start, stop, step int64, errObj object.Object, ok bool) {
	if len(args) < 1 || len(args) > 3 {
		return 0, 0, 0, nil, false
	}

	values := [3]int64{}
	for i, arg := range args {
		evaluated := evalNode(ctx, arg, env)
		if object.IsError(evaluated) {
			return 0, 0, 0, evaluated, true
		}
		intObj, isInt := evaluated.(*object.Integer)
		if !isInt {
			return 0, 0, 0, nil, false
		}
		values[i] = intObj.IntValue()
	}

	switch len(args) {
	case 1:
		start, stop, step = 0, values[0], 1
	case 2:
		start, stop, step = values[0], values[1], 1
	case 3:
		start, stop, step = values[0], values[1], values[2]
		if step == 0 {
			return 0, 0, 0, errors.NewError("range step cannot be zero"), true
		}
	}
	return start, stop, step, nil, true
}

// evalMethodCallExpression is in methods.go
// callStringMethodWithKeywords is in methods.go

func evalAdditionalClauses(ctx context.Context, clauses []ast.ComprehensionClause, idx int, env *object.Environment, action func() object.Object) object.Object {
	if idx >= len(clauses) {
		return action()
	}
	c := clauses[idx]
	iterable := evalNode(ctx, c.Iterable, env)
	if object.IsError(iterable) {
		return iterable
	}
	return iterateObject(ctx, iterable, func(element object.Object) object.Object {
		if err := setForVariables(c.Variables, element, env); err != nil {
			return errors.NewError("%s", err.Error())
		}
		if c.Condition != nil {
			cond := evalNode(ctx, c.Condition, env)
			if object.IsError(cond) {
				return cond
			}
			if !isTruthy(cond) {
				return nil
			}
		}
		return evalAdditionalClauses(ctx, clauses, idx+1, env, action)
	})
}

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
			if object.IsError(iterObj) {
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

func evalListComprehension(ctx context.Context, lc *ast.ListComprehension, env *object.Environment) object.Object {
	iterable := evalNode(ctx, lc.Iterable, env)
	if object.IsError(iterable) {
		return iterable
	}
	if result, ok := tryEvalFastListComprehension(ctx, lc, iterable, env); ok {
		return result
	}
	result := []object.Object{}
	compEnv := object.NewEnclosedEnvironment(env)
	emit := func() object.Object {
		v := evalNode(ctx, lc.Expression, compEnv)
		if object.IsError(v) {
			return v
		}
		result = append(result, v)
		return nil
	}
	runBody := func(element object.Object) object.Object {
		if err := setForVariables(lc.Variables, element, compEnv); err != nil {
			return errors.NewError("%s", err.Error())
		}
		if lc.Condition != nil {
			cond := evalNode(ctx, lc.Condition, compEnv)
			if object.IsError(cond) {
				return cond
			}
			if !isTruthy(cond) {
				return nil
			}
		}
		if len(lc.AdditionalClauses) > 0 {
			return evalAdditionalClauses(ctx, lc.AdditionalClauses, 0, compEnv, emit)
		}
		return emit()
	}
	if err := iterateObject(ctx, iterable, runBody); err != nil {
		return err
	}
	return &object.List{Elements: result}
}

func tryEvalFastListComprehension(ctx context.Context, lc *ast.ListComprehension, iterable object.Object, env *object.Environment) (object.Object, bool) {
	if len(lc.AdditionalClauses) > 0 || len(lc.Variables) != 1 {
		return nil, false
	}
	ident, ok := lc.Variables[0].(*ast.Identifier)
	if !ok {
		return nil, false
	}

	compEnv := object.NewEnclosedEnvironment(env)
	result := make([]object.Object, 0)
	runElement := func(element object.Object) object.Object {
		compEnv.Set(ident.Value(), element)
		if lc.Condition != nil {
			cond := evalNode(ctx, lc.Condition, compEnv)
			if object.IsError(cond) {
				return cond
			}
			if !isTruthy(cond) {
				return nil
			}
		}
		value := evalNode(ctx, lc.Expression, compEnv)
		if object.IsError(value) {
			return value
		}
		result = append(result, value)
		return nil
	}

	switch it := iterable.(type) {
	case *object.List:
		result = make([]object.Object, 0, len(it.Elements))
		for _, element := range it.Elements {
			if out := runElement(element); out != nil {
				return out, true
			}
		}
	case *object.Tuple:
		result = make([]object.Object, 0, len(it.Elements))
		for _, element := range it.Elements {
			if out := runElement(element); out != nil {
				return out, true
			}
		}
	case *object.Set:
		result = make([]object.Object, 0, len(it.Elements))
		for _, element := range it.Elements {
			if out := runElement(element); out != nil {
				return out, true
			}
		}
	case *object.Iterator:
		for {
			element, ok := it.Next()
			if !ok {
				break
			}
			if out := runElement(element); out != nil {
				return out, true
			}
		}
	case *object.FloatArray:
		if it.Is2D() {
			rows := it.Rows()
			cols := it.Cols()
			result = make([]object.Object, 0, rows)
			for i := 0; i < rows; i++ {
				off := i * cols
				rowData := make([]float64, cols)
				copy(rowData, it.Data[off:off+cols])
				row := object.NewFloatArray1D(rowData)
				if out := runElement(row); out != nil {
					return out, true
				}
			}
		} else {
			result = make([]object.Object, 0, len(it.Data))
			for _, v := range it.Data {
				if out := runElement(object.NewFloat(v)); out != nil {
					return out, true
				}
			}
		}
	default:
		return nil, false
	}

	return &object.List{Elements: result}, true
}

func evalDictComprehension(ctx context.Context, dc *ast.DictComprehension, env *object.Environment) object.Object {
	iterable := evalNode(ctx, dc.Iterable, env)
	if object.IsError(iterable) {
		return iterable
	}
	result := &object.Dict{Pairs: make(map[string]object.DictPair)}
	compEnv := object.NewEnclosedEnvironment(env)
	emit := func() object.Object {
		k := evalNode(ctx, dc.Key, compEnv)
		if object.IsError(k) {
			return k
		}
		v := evalNode(ctx, dc.Value, compEnv)
		if object.IsError(v) {
			return v
		}
		result.Pairs[evalHashKey(ctx, k)] = object.DictPair{Key: k, Value: v}
		return nil
	}
	runBody := func(element object.Object) object.Object {
		if err := setForVariables(dc.Variables, element, compEnv); err != nil {
			return errors.NewError("%s", err.Error())
		}
		if dc.Condition != nil {
			cond := evalNode(ctx, dc.Condition, compEnv)
			if object.IsError(cond) {
				return cond
			}
			if !isTruthy(cond) {
				return nil
			}
		}
		if len(dc.AdditionalClauses) > 0 {
			return evalAdditionalClauses(ctx, dc.AdditionalClauses, 0, compEnv, emit)
		}
		return emit()
	}
	if err := iterateObject(ctx, iterable, runBody); err != nil {
		return err
	}
	return result
}

func evalSetComprehension(ctx context.Context, sc *ast.SetComprehension, env *object.Environment) object.Object {
	iterable := evalNode(ctx, sc.Iterable, env)
	if object.IsError(iterable) {
		return iterable
	}
	result := object.NewSet()
	compEnv := object.NewEnclosedEnvironment(env)
	emit := func() object.Object {
		v := evalNode(ctx, sc.Expression, compEnv)
		if object.IsError(v) {
			return v
		}
		return evalSetAdd(ctx, result, v)
	}
	runBody := func(element object.Object) object.Object {
		if err := setForVariables(sc.Variables, element, compEnv); err != nil {
			return errors.NewError("%s", err.Error())
		}
		if sc.Condition != nil {
			cond := evalNode(ctx, sc.Condition, compEnv)
			if object.IsError(cond) {
				return cond
			}
			if !isTruthy(cond) {
				return nil
			}
		}
		if len(sc.AdditionalClauses) > 0 {
			return evalAdditionalClauses(ctx, sc.AdditionalClauses, 0, compEnv, emit)
		}
		return emit()
	}
	if err := iterateObject(ctx, iterable, runBody); err != nil {
		return err
	}
	return result
}

func evalLambda(lambda *ast.Lambda, env *object.Environment) object.Object {
	localSlots, localSlotNames := analyzeLambdaLocals(lambda)
	return &object.LambdaFunction{
		Parameters:       lambda.Parameters,
		DefaultValues:    lambda.GetDefaultValues(),
		Variadic:         lambda.GetVariadic(),
		Kwargs:           lambda.GetKwargs(),
		KeywordOnlyStart: lambda.GetKeywordOnlyStart() + 1,
		Body:             lambda.Body,
		Env:              env,
		LocalSlots:       localSlots,
		LocalSlotNames:   localSlotNames,
		ParamSlotIndexes: lambda.ParamSlotIndexes,
	}
}

func evalFStringLiteral(ctx context.Context, fstr *ast.FStringLiteral, env *object.Environment) object.Object {
	var builder strings.Builder

	// Pre-allocate capacity to reduce reallocations
	// Estimate base size from static parts plus some buffer for expressions
	estimatedSize := 0
	for _, part := range fstr.Parts {
		estimatedSize += len(part)
	}
	// Add buffer for formatted expressions (rough estimate)
	estimatedSize += len(fstr.Expressions) * 16
	builder.Grow(estimatedSize)

	for i, part := range fstr.Parts {
		builder.WriteString(part)
		if i < len(fstr.Expressions) {
			exprResult := evalNode(ctx, fstr.Expressions[i], env)
			if object.IsError(exprResult) {
				return exprResult
			}
			// Call __str__ on instances for f-string formatting
			specs := fstr.GetFormatSpecs()
			specEmpty := specs == nil || specs[i] == ""
			if inst, ok := exprResult.(*object.Instance); ok && specEmpty {
				if result := callDunderMethod(ctx, inst, "__str__", nil, env); result != nil {
					exprResult = result
				}
			}
			spec := ""
			if specs != nil {
				spec = specs[i]
			}
			formatted := formatWithSpec(exprResult, spec)
			builder.WriteString(formatted)
		}
	}

	return object.NewString(builder.String())
}

func formatWithSpec(obj object.Object, spec string) string {
	if spec == "" {
		switch v := obj.(type) {
		case *object.Integer:
			return strconv.FormatInt(v.IntValue(), 10)
		case *object.Float:
			if v.FloatValue() == float64(int64(v.FloatValue())) {
				return strconv.FormatFloat(v.FloatValue(), 'f', 1, 64)
			}
			return strconv.FormatFloat(v.FloatValue(), 'g', -1, 64)
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
					formatted = strconv.FormatFloat(floatVal, 'g', -1, 64)
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
		if intVal, err := obj.AsInt(); err == nil {
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

func evalMatchStatementWithContext(ctx context.Context, ms *ast.MatchStatement, env *object.Environment) object.Object {
	subject := evalNode(ctx, ms.Subject, env)
	if object.IsError(subject) {
		return subject
	}

	for _, caseClause := range ms.Cases {
		// Track captured variables for this case
		capturedVars := make(map[string]object.Object)

		matched, capturedValue := matchPattern(ctx, subject, caseClause.Pattern, capturedVars)
		if object.IsError(matched) {
			return matched
		}

		if matched == TRUE {
			// Temporarily add captured variables to environment for guard evaluation
			for name, val := range capturedVars {
				env.Set(name, val)
			}

			// Check guard condition if present
			if caseClause.Guard != nil {
				guardResult := evalNode(ctx, caseClause.Guard, env)
				if object.IsError(guardResult) {
					return guardResult
				}
				if !isTruthy(guardResult) {
					// Guard failed - try next case
					continue
				}
			}

			// Bind explicit capture variable if present
			if caseClause.CaptureAs != nil {
				env.Set(caseClause.CaptureAs.Value(), capturedValue)
			}

			// Execute body in the environment (with captures)
			return evalWithContext(ctx, caseClause.Body, env)
		}
	}

	return NULL
}

func matchPattern(ctx context.Context, subject object.Object, pattern ast.Expression, capturedVars map[string]object.Object) (object.Object, object.Object) {
	switch p := pattern.(type) {
	case *ast.OrPattern:
		for _, alt := range p.Patterns {
			matched, val := matchPattern(ctx, subject, alt, capturedVars)
			if object.IsError(matched) {
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
			keyObj := evalNode(ctx, patternPair.Key, object.NewEnvironment())
			if object.IsError(keyObj) {
				return keyObj, NULL
			}

			keyStr := evalHashKey(ctx, keyObj)
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
				matched, _ := matchPattern(ctx, dictPair.Value, patternPair.Value, capturedVars)
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
			matched, _ := matchPattern(ctx, listObj.Elements[i], elemExpr, capturedVars)
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
		return "dict"
	case object.TUPLE_OBJ:
		return "tuple"
	case object.SET_OBJ:
		return "set"
	case object.NULL_OBJ:
		return "NoneType"
	default:
		return obj.Type().String()
	}
}
