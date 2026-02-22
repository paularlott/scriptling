package evaluator

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/paularlott/scriptling/ast"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

var (
	NULL  = &object.Null{}
	TRUE  = &object.Boolean{Value: true}
	FALSE = &object.Boolean{Value: false}
)

// Pool for strings.Builder to reduce allocations in string operations
var builderPool = sync.Pool{
	New: func() any {
		b := &strings.Builder{}
		// Pre-allocate a reasonable capacity to reduce reallocations
		b.Grow(64)
		return b
	},
}

// getStringBuilder retrieves a builder from the pool
func getStringBuilder() *strings.Builder {
	return builderPool.Get().(*strings.Builder)
}

// putStringBuilder returns a builder to the pool after resetting it
func putStringBuilder(b *strings.Builder) {
	b.Reset()
	builderPool.Put(b)
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
	v := atomic.AddInt32(&cd.current, 1)
	return v <= cd.max
}

// Exit decrements call depth
func (cd *CallDepth) Exit() {
	atomic.AddInt32(&cd.current, -1)
}

// Depth returns current call depth
func (cd *CallDepth) Depth() int {
	return int(atomic.LoadInt32(&cd.current))
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
	// Add evaluator to context if not present
	ctx = WithEvaluator(ctx)

	// Check for cancellation - batched via context checker in the top-level EvalWithContext
	// For leaf nodes, we skip the check to reduce overhead
	switch node := node.(type) {
	case *ast.Program:
		return evalProgram(ctx, node, env)
	case *ast.ExpressionStatement:
		return evalWithContext(ctx, node.Expression, env)
	case *ast.IntegerLiteral:
		return object.NewInteger(node.Value)
	case *ast.FloatLiteral:
		return &object.Float{Value: node.Value}
	case *ast.StringLiteral:
		return &object.String{Value: node.Value}
	case *ast.FStringLiteral:
		return evalFStringLiteral(ctx, node, env)
	case *ast.Boolean:
		return nativeBoolToBooleanObject(node.Value)
	case *ast.None:
		return NULL
	case *ast.PrefixExpression:
		right := evalWithContext(ctx, node.Right, env)
		if object.IsError(right) {
			return right
		}
		return evalPrefixExpression(node.Operator, right)
	case *ast.InfixExpression:
		// Handle short-circuit operators (and, or) specially
		if node.Operator == "and" || node.Operator == "or" {
			return evalShortCircuitInfixExpression(ctx, node, env)
		}
		// For other operators, evaluate both sides
		left := evalWithContext(ctx, node.Left, env)
		if object.IsError(left) {
			return left
		}
		right := evalWithContext(ctx, node.Right, env)
		if object.IsError(right) {
			return right
		}
		return evalInfixExpression(ctx, node.Operator, left, right, env)
	case *ast.ConditionalExpression:
		return evalConditionalExpression(ctx, node, env)
	case *ast.BlockStatement:
		return evalBlockStatementWithContext(ctx, node, env)
	case *ast.IfStatement:
		return evalIfStatementWithContext(ctx, node, env)
	case *ast.MatchStatement:
		return evalMatchStatementWithContext(ctx, node, env)
	case *ast.WhileStatement:
		return evalWhileStatementWithContext(ctx, node, env)
	case *ast.ReturnStatement:
		val := object.Object(NULL)
		if node.ReturnValue != nil {
			val = evalWithContext(ctx, node.ReturnValue, env)
			if object.IsError(val) {
				return val
			}
		}
		return &object.ReturnValue{Value: val}
	case *ast.BreakStatement:
		return object.BREAK
	case *ast.ContinueStatement:
		return object.CONTINUE
	case *ast.PassStatement:
		return NULL
	case *ast.ImportStatement:
		return evalImportStatement(node, env)
	case *ast.FromImportStatement:
		return evalFromImportStatement(node, env)
	case *ast.AssignStatement:
		val := evalWithContext(ctx, node.Value, env)
		if object.IsError(val) || isException(val) {
			return val
		}
		if err := assignToExpression(ctx, node.Left, val, env); err != nil {
			return errors.NewError("%s", err.Error())
		}
		return NULL
	case *ast.AugmentedAssignStatement:
		return evalAugmentedAssignStatementWithContext(ctx, node, env)
	case *ast.MultipleAssignStatement:
		return evalMultipleAssignStatementWithContext(ctx, node, env)
	case *ast.Identifier:
		return evalIdentifier(node, env)
	case *ast.FunctionStatement:
		return evalFunctionStatement(ctx, node, env)
	case *ast.ClassStatement:
		return evalClassStatement(ctx, node, env)
	case *ast.CallExpression:
		return evalCallExpression(ctx, node, env)
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
			s.Add(elem)
		}
		return s
	case *ast.IndexExpression:
		left := evalWithContext(ctx, node.Left, env)
		if object.IsError(left) {
			return left
		}
		index := evalWithContext(ctx, node.Index, env)
		if object.IsError(index) {
			return index
		}
		return evalIndexExpression(ctx, left, index)
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
	case *ast.MethodCallExpression:
		return evalMethodCallExpression(ctx, node, env)
	case *ast.ListComprehension:
		return evalListComprehension(ctx, node, env)
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
	var result object.Object = NULL
	cc := newContextChecker(ctx)

	for _, statement := range program.Statements {
		// Check for cancellation less frequently
		if err := cc.check(); err != nil {
			return err
		}

		result = evalWithContext(ctx, statement, env)

		switch result := result.(type) {
		case *object.ReturnValue:
			return result.Value
		case *object.Error:
			return result
		case *object.Exception:
			// Uncaught exception at program level
			// SystemExit exceptions should be returned as-is for proper error handling
			// Other exceptions are converted to errors
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
		// Check for cancellation less frequently
		if err := cc.check(); err != nil {
			return err
		}

		result = evalWithContext(ctx, statement, env)

		if result != nil {
			rt := result.Type()
			if rt == object.RETURN_OBJ || rt == object.BREAK_OBJ || rt == object.CONTINUE_OBJ {
				return result
			}
			// Don't return errors immediately - let try/catch handle them
			if rt == object.ERROR_OBJ || rt == object.EXCEPTION_OBJ {
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

// objectsEqual checks if two objects are equal
func objectsEqual(a, b object.Object) bool {
	if a.Type() != b.Type() {
		return false
	}
	switch av := a.(type) {
	case *object.Integer:
		return av.Value == b.(*object.Integer).Value
	case *object.Float:
		return av.Value == b.(*object.Float).Value
	case *object.String:
		return av.Value == b.(*object.String).Value
	case *object.Boolean:
		return av.Value == b.(*object.Boolean).Value
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
		return av.Value == b.(*object.Integer).Value
	case *object.Float:
		return av.Value == b.(*object.Float).Value
	case *object.String:
		return av.Value == b.(*object.String).Value
	case *object.Boolean:
		return av.Value == b.(*object.Boolean).Value
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

func evalPrefixExpression(operator string, right object.Object) object.Object {
	switch operator {
	case "not":
		return evalNotOperatorExpression(right)
	case "-":
		return evalMinusPrefixOperatorExpression(right)
	case "~":
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
		return object.NewInteger(-right.Value)
	case *object.Float:
		return &object.Float{Value: -right.Value}
	default:
		return errors.NewError("%s: -%s", errors.ErrUnknownOperator, right.Type())
	}
}

func evalBitwiseNotOperatorExpression(right object.Object) object.Object {
	switch right := right.(type) {
	case *object.Integer:
		return object.NewInteger(^right.Value)
	default:
		return errors.NewError("%s: ~%s", errors.ErrUnknownOperator, right.Type())
	}
}

// evalShortCircuitInfixExpression handles and/or operators with proper short-circuit evaluation
func evalShortCircuitInfixExpression(ctx context.Context, node *ast.InfixExpression, env *object.Environment) object.Object {
	left := evalWithContext(ctx, node.Left, env)
	if object.IsError(left) {
		return left
	}

	switch node.Operator {
	case "and":
		// Short-circuit: if left is falsy, return it without evaluating right
		if !isTruthy(left) {
			return left
		}
		// Left is truthy, evaluate and return right
		return evalWithContext(ctx, node.Right, env)
	case "or":
		// Short-circuit: if left is truthy, return it without evaluating right
		if isTruthy(left) {
			return left
		}
		// Left is falsy, evaluate and return right
		return evalWithContext(ctx, node.Right, env)
	default:
		return errors.NewError("unknown operator: %s", node.Operator)
	}
}

func evalInfixExpression(ctx context.Context, operator string, left, right object.Object, env *object.Environment) object.Object {

	// Handle membership operators
	switch operator {
	case "in":
		return evalInOperator(left, right)
	case "not in":
		result := evalInOperator(left, right)
		if result == TRUE {
			return FALSE
		}
		return TRUE
	case "is":
		return evalIsOperator(left, right)
	case "is not":
		result := evalIsOperator(left, right)
		if result == TRUE {
			return FALSE
		}
		return TRUE
	}

	// Type switch is faster than Type() method calls
	switch l := left.(type) {
	case *object.Integer:
		if r, ok := right.(*object.Integer); ok {
			return evalIntegerInfixExpression(operator, l.Value, r.Value)
		}
		// Handle int * string
		if r, ok := right.(*object.String); ok && operator == "*" {
			return evalStringMultiplication(r.Value, l.Value)
		}
		return evalFloatInfixExpression(operator, left, right)
	case *object.Float:
		return evalFloatInfixExpression(operator, left, right)
	case *object.String:
		if r, ok := right.(*object.String); ok {
			return evalStringInfixExpression(operator, l.Value, r.Value)
		}
		// Handle string * int
		if r, ok := right.(*object.Integer); ok && operator == "*" {
			return evalStringMultiplication(l.Value, r.Value)
		}
	case *object.Instance:
		// Handle instance operators via dunder methods (__lt__, __gt__, __eq__, __sub__, __add__, etc.)
		if result := evalInstanceInfixExpression(ctx, operator, l, right, env); result != nil {
			return result
		}
	}

	switch operator {
	case "==":
		return nativeBoolToBooleanObject(objectsDeepEqual(left, right))
	case "!=":
		return nativeBoolToBooleanObject(!objectsDeepEqual(left, right))
	default:
		return errors.NewError("%s: type mismatch", errors.ErrTypeError)
	}
}

func evalConditionalExpression(ctx context.Context, node *ast.ConditionalExpression, env *object.Environment) object.Object {
	condition := evalWithContext(ctx, node.Condition, env)
	if object.IsError(condition) {
		return condition
	}

	if isTruthy(condition) {
		return evalWithContext(ctx, node.TrueExpr, env)
	} else {
		return evalWithContext(ctx, node.FalseExpr, env)
	}
}

func evalIntegerInfixExpression(operator string, leftVal, rightVal int64) object.Object {
	switch operator {
	case "+":
		return object.NewInteger(leftVal + rightVal)
	case "-":
		return object.NewInteger(leftVal - rightVal)
	case "*":
		return object.NewInteger(leftVal * rightVal)
	case "/":
		if rightVal == 0 {
			return errors.NewError(errors.ErrDivisionByZero)
		}
		// True division: always return float
		return &object.Float{Value: float64(leftVal) / float64(rightVal)}
	case "//":
		if rightVal == 0 {
			return errors.NewError(errors.ErrDivisionByZero)
		}
		// Floor division: return integer for integers
		return object.NewInteger(leftVal / rightVal)
	case "**":
		// Power operator - use float for negative exponents
		if rightVal < 0 {
			return evalFloatInfixExpression("**", object.NewInteger(leftVal), object.NewInteger(rightVal))
		}
		// Integer exponentiation with overflow detection.
		// Unlike Python 3 which has arbitrary precision integers, scriptling uses
		// int64. If the result would overflow, we fall back to float64.
		if rightVal > 63 || (leftVal > 1 && rightVal > 40) || (leftVal < -1 && rightVal > 40) {
			// Definitely overflows int64, use float
			return &object.Float{Value: math.Pow(float64(leftVal), float64(rightVal))}
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
	case "%":
		if rightVal == 0 {
			return errors.NewError(errors.ErrDivisionByZero)
		}
		return object.NewInteger(leftVal % rightVal)
	case "&":
		return object.NewInteger(leftVal & rightVal)
	case "|":
		return object.NewInteger(leftVal | rightVal)
	case "^":
		return object.NewInteger(leftVal ^ rightVal)
	case "<<":
		if rightVal < 0 {
			return errors.NewError("negative shift count")
		}
		return object.NewInteger(leftVal << uint64(rightVal))
	case ">>":
		if rightVal < 0 {
			return errors.NewError("negative shift count")
		}
		return object.NewInteger(leftVal >> uint64(rightVal))
	case "<":
		return nativeBoolToBooleanObject(leftVal < rightVal)
	case ">":
		return nativeBoolToBooleanObject(leftVal > rightVal)
	case "<=":
		return nativeBoolToBooleanObject(leftVal <= rightVal)
	case ">=":
		return nativeBoolToBooleanObject(leftVal >= rightVal)
	case "==":
		return nativeBoolToBooleanObject(leftVal == rightVal)
	case "!=":
		return nativeBoolToBooleanObject(leftVal != rightVal)
	default:
		return errors.NewError("unknown operator: INTEGER %s INTEGER", operator)
	}
}

func evalFloatInfixExpression(operator string, left, right object.Object) object.Object {
	var leftVal, rightVal float64

	switch left := left.(type) {
	case *object.Float:
		leftVal = left.Value
	case *object.Integer:
		leftVal = float64(left.Value)
	}

	switch right := right.(type) {
	case *object.Float:
		rightVal = right.Value
	case *object.Integer:
		rightVal = float64(right.Value)
	default:
		// For == and != with non-numeric types, different types are never equal (Python behavior)
		switch operator {
		case "==":
			return FALSE
		case "!=":
			return TRUE
		}
		return errors.NewTypeError("NUMBER", right.Type().String())
	}

	switch operator {
	case "+":
		return &object.Float{Value: leftVal + rightVal}
	case "-":
		return &object.Float{Value: leftVal - rightVal}
	case "*":
		return &object.Float{Value: leftVal * rightVal}
	case "/":
		if rightVal == 0 {
			return errors.NewError(errors.ErrDivisionByZero)
		}
		return &object.Float{Value: leftVal / rightVal}
	case "//":
		if rightVal == 0 {
			return errors.NewError(errors.ErrDivisionByZero)
		}
		return &object.Float{Value: math.Floor(leftVal / rightVal)}
	case "**":
		return &object.Float{Value: math.Pow(leftVal, rightVal)}
	case "<":
		return nativeBoolToBooleanObject(leftVal < rightVal)
	case ">":
		return nativeBoolToBooleanObject(leftVal > rightVal)
	case "<=":
		return nativeBoolToBooleanObject(leftVal <= rightVal)
	case ">=":
		return nativeBoolToBooleanObject(leftVal >= rightVal)
	case "==":
		return nativeBoolToBooleanObject(leftVal == rightVal)
	case "!=":
		return nativeBoolToBooleanObject(leftVal != rightVal)
	default:
		return errors.NewError("unknown operator: FLOAT %s FLOAT", operator)
	}
}

func evalStringInfixExpression(operator string, leftVal, rightVal string) object.Object {
	switch operator {
	case "+":
		if len(leftVal) == 0 {
			return &object.String{Value: rightVal}
		}
		if len(rightVal) == 0 {
			return &object.String{Value: leftVal}
		}
		// For small strings, use direct builder allocation (faster than pooling)
		// For large strings, use pooling to reduce allocations
		totalLen := len(leftVal) + len(rightVal)
		if totalLen < 128 { // Threshold for small strings
			var builder strings.Builder
			builder.Grow(totalLen)
			builder.WriteString(leftVal)
			builder.WriteString(rightVal)
			return &object.String{Value: builder.String()}
		}
		// Use pooled strings.Builder for larger concatenations
		builder := getStringBuilder()
		builder.Grow(totalLen)
		builder.WriteString(leftVal)
		builder.WriteString(rightVal)
		result := builder.String()
		putStringBuilder(builder)
		return &object.String{Value: result}
	case "==":
		return nativeBoolToBooleanObject(leftVal == rightVal)
	case "!=":
		return nativeBoolToBooleanObject(leftVal != rightVal)
	default:
		return errors.NewError("%s: STRING %s STRING", errors.ErrUnknownOperator, operator)
	}
}

func evalStringMultiplication(str string, multiplier int64) object.Object {
	if multiplier < 0 {
		return &object.String{Value: ""}
	}
	return &object.String{Value: strings.Repeat(str, int(multiplier))}
}

// callDunderMethod calls a dunder method on an instance, returning nil if not defined.
// Returns the result string object for __str__/__repr__, or the raw result for others.
func callDunderMethod(ctx context.Context, inst *object.Instance, method string, args []object.Object, env *object.Environment) object.Object {
	// Walk the class hierarchy
	currentClass := inst.Class
	for currentClass != nil {
		if fn, ok := currentClass.Methods[method]; ok {
			newArgs := prependSelf(inst, args)
			result := applyFunctionWithContext(ctx, fn, newArgs, nil, env)
			if object.IsError(result) {
				return result
			}
			return result
		}
		currentClass = currentClass.BaseClass
	}
	return nil
}

// operatorToDunderMethod maps operators to their corresponding dunder method names
var operatorToDunderMethod = map[string]string{
	"<":  "__lt__",
	">":  "__gt__",
	"<=": "__le__",
	">=": "__ge__",
	"==": "__eq__",
	"!=": "__ne__",
	"+":  "__add__",
	"-":  "__sub__",
	"*":  "__mul__",
	"/":  "__truediv__",
	"//": "__floordiv__",
	"%":  "__mod__",
}

// evalInstanceInfixExpression handles operators on instances by calling dunder methods
// Returns nil if no dunder method is found (falls through to default handling)
func evalInstanceInfixExpression(ctx context.Context, operator string, left *object.Instance, right object.Object, env *object.Environment) object.Object {
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
	condition := evalWithContext(ctx, ie.Condition, env)
	if object.IsError(condition) {
		return condition
	}

	if isTruthy(condition) {
		return evalWithContext(ctx, ie.Consequence, env)
	}

	// Check elif clauses
	for _, elifClause := range ie.ElifClauses {
		condition := evalWithContext(ctx, elifClause.Condition, env)
		if object.IsError(condition) {
			return condition
		}
		if isTruthy(condition) {
			return evalWithContext(ctx, elifClause.Consequence, env)
		}
	}

	// Check else clause
	if ie.Alternative != nil {
		return evalWithContext(ctx, ie.Alternative, env)
	}

	return NULL
}

func evalWhileStatementWithContext(ctx context.Context, ws *ast.WhileStatement, env *object.Environment) object.Object {
	var result object.Object = NULL
	cc := newContextChecker(ctx)

	for {
		// Always check in loops for responsiveness, but batch the checks
		if err := cc.checkAlways(); err != nil {
			return err
		}

		condition := evalWithContext(ctx, ws.Condition, env)
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
				return NULL
			case object.CONTINUE_OBJ:
				continue
			}
		}
	}

	return result
}

func evalIdentifier(node *ast.Identifier, env *object.Environment) object.Object {
	if val, ok := env.Get(node.Value); ok {
		return val
	}
	if builtin, ok := builtins[node.Value]; ok {
		return builtin
	}
	return errors.NewIdentifierError(node.Value)
}

func evalFunctionStatement(ctx context.Context, stmt *ast.FunctionStatement, env *object.Environment) object.Object {
	fn := &object.Function{
		Name:          stmt.Name.Value,
		Parameters:    stmt.Function.Parameters,
		DefaultValues: stmt.Function.DefaultValues,
		Variadic:      stmt.Function.Variadic,
		Kwargs:        stmt.Function.Kwargs,
		Body:          stmt.Function.Body,
		Env:           env,
	}
	var result object.Object = fn
	// Apply decorators right-to-left (innermost first)
	for i := len(stmt.Decorators) - 1; i >= 0; i-- {
		dec := evalWithContext(ctx, stmt.Decorators[i], env)
		if object.IsError(dec) {
			return dec
		}
		result = applyFunctionWithContext(ctx, dec, []object.Object{result}, nil, env)
		if object.IsError(result) {
			return result
		}
	}
	env.Set(stmt.Name.Value, result)
	return result
}

func evalClassStatement(ctx context.Context, stmt *ast.ClassStatement, env *object.Environment) object.Object {
	class := &object.Class{
		Name:    stmt.Name.Value,
		Methods: make(map[string]object.Object),
		Env:     env,
	}

	// Handle base class inheritance
	if stmt.BaseClass != nil {
		// Evaluate the base class expression (can be dotted like html.parser.HTMLParser)
		baseClassObj := evalWithContext(ctx, stmt.BaseClass, env)
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
				class.Methods[fnStmt.Name.Value] = m
			case *object.StaticMethod:
				class.Methods[fnStmt.Name.Value] = m
			}
		}
	}

	env.Set(stmt.Name.Value, class)
	var result object.Object = class
	// Apply decorators right-to-left (innermost first)
	for i := len(stmt.Decorators) - 1; i >= 0; i-- {
		dec := evalWithContext(ctx, stmt.Decorators[i], env)
		if object.IsError(dec) {
			return dec
		}
		result = applyFunctionWithContext(ctx, dec, []object.Object{result}, nil, env)
		if object.IsError(result) {
			return result
		}
	}
	if result != class {
		env.Set(stmt.Name.Value, result)
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
		for _, r := range val.Value {
			unpacked = append(unpacked, &object.String{Value: string(r)})
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

func evalCallExpression(ctx context.Context, node *ast.CallExpression, env *object.Environment) object.Object {
	function := evalWithContext(ctx, node.Function, env)
	if object.IsError(function) {
		return function
	}
	args := evalExpressionsWithContext(ctx, node.Arguments, env)
	if len(args) == 1 && object.IsError(args[0]) {
		return args[0]
	}

	var keywords map[string]object.Object
	if len(node.Keywords) > 0 {
		keywords = make(map[string]object.Object, len(node.Keywords))
		for k, v := range node.Keywords {
			val := evalWithContext(ctx, v, env)
			if object.IsError(val) {
				return val
			}
			keywords[k] = val
		}
	}

	// Handle *args unpacking (supports multiple)
	for _, argsUnpackExpr := range node.ArgsUnpack {
		argsVal := evalWithContext(ctx, argsUnpackExpr, env)
		if object.IsError(argsVal) {
			return argsVal
		}
		unpacked, err := unpackArgsFromIterable(argsVal)
		if err != nil {
			return err
		}
		args = append(args, unpacked...)
	}

	// Handle **kwargs unpacking
	if node.KwargsUnpack != nil {
		kwargsVal := evalWithContext(ctx, node.KwargsUnpack, env)
		if object.IsError(kwargsVal) {
			return kwargsVal
		}
		if dict, ok := kwargsVal.(*object.Dict); ok {
			if keywords == nil {
				keywords = make(map[string]object.Object, len(dict.Pairs))
			}
			for _, pair := range dict.Pairs {
				// Use the original string key, not the DictKey-formatted map key
				keywords[pair.StringKey()] = pair.Value
			}
		} else {
			return errors.NewError("argument after ** must be a dictionary, not %s", kwargsVal.Type())
		}
	}

	return applyFunctionWithContext(ctx, function, args, keywords, env)
}

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

	return instance
}

func evalExpressionsWithContext(ctx context.Context, exps []ast.Expression, env *object.Environment) []object.Object {
	if len(exps) == 0 {
		return nil
	}
	result := make([]object.Object, len(exps))

	for i, e := range exps {
		evaluated := evalWithContext(ctx, e, env)
		if object.IsError(evaluated) {
			return []object.Object{evaluated}
		}
		result[i] = evaluated
	}

	return result
}

func applyUserFunction(ctx context.Context, fn *object.Function, args []object.Object, keywords map[string]object.Object, env *object.Environment) object.Object {
	// Check call depth to prevent stack overflow
	if cd := GetCallDepthFromContext(ctx); cd != nil {
		if !cd.Enter() {
			return errors.NewCallDepthExceededError(int(cd.max))
		}
		defer cd.Exit()
	}

	extendedEnv, err := extendFunctionEnv(fn, args, keywords)
	if err != nil {
		return err
	}

	evaluated := evalWithContext(ctx, fn.Body, extendedEnv)
	if err, ok := evaluated.(*object.Error); ok {
		if err.Function == "" {
			err.Function = fn.Name
		}
	}
	return unwrapReturnValue(evaluated)
}

// ApplyFunction calls a function object with arguments and keyword arguments.
// This is exported for use by other packages that need to call script functions directly.
func ApplyFunction(ctx context.Context, fn object.Object, args []object.Object, keywords map[string]object.Object, env *object.Environment) object.Object {
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
		return ApplyFunction(ctx, bm.Method, newArgs, keywords, env)
	}
	return ApplyFunction(ctx, fn, args, keywords, env)
}

func applyLambdaFunctionWithContext(ctx context.Context, fn *object.LambdaFunction, args []object.Object, keywords map[string]object.Object, env *object.Environment) object.Object {
	// Check call depth to prevent stack overflow
	if cd := GetCallDepthFromContext(ctx); cd != nil {
		if !cd.Enter() {
			return errors.NewCallDepthExceededError(int(cd.max))
		}
		defer cd.Exit()
	}

	extendedEnv, err := extendLambdaEnv(fn, args, keywords)
	if err != nil {
		return err
	}

	evaluated := evalWithContext(ctx, fn.Body, extendedEnv)
	return evaluated // No unwrapping needed for lambda expressions
}

// funcParams abstracts the common parts of Function and LambdaFunction for parameter handling
type funcParams struct {
	parameters    []*ast.Identifier
	defaultValues map[string]ast.Expression
	variadic      *ast.Identifier
	kwargs        *ast.Identifier
	parentEnv     *object.Environment
}

// extendEnvWithParams handles the common logic for extending environments with function arguments
func extendEnvWithParams(fp funcParams, args []object.Object, keywords map[string]object.Object) (*object.Environment, object.Object) {
	env := object.NewEnclosedEnvironment(fp.parentEnv)

	numParams := len(fp.parameters)
	numArgs := len(args)

	// Set provided positional arguments
	for paramIdx := 0; paramIdx < numParams && paramIdx < numArgs; paramIdx++ {
		env.Set(fp.parameters[paramIdx].Value, args[paramIdx])
	}

	// Check for extra positional arguments
	if numArgs > numParams {
		if fp.variadic != nil {
			// Collect extra arguments into a list
			list := &object.List{Elements: args[numParams:]}
			env.Set(fp.variadic.Value, list)
		} else {
			minArgs := numParams - len(fp.defaultValues)
			return nil, errors.NewArgumentError(numArgs, minArgs)
		}
	} else if fp.variadic != nil {
		// No extra arguments, set variadic to empty list
		env.Set(fp.variadic.Value, &object.List{Elements: []object.Object{}})
	}

	// Handle keyword arguments if present
	if len(keywords) > 0 {
		// Use a stack-allocated array for small param counts to track which params are set
		var setSmall [8]bool
		var setParams map[string]bool
		if numParams <= 8 {
			// Mark positional args as set via index
			for i := 0; i < numParams && i < numArgs; i++ {
				setSmall[i] = true
			}
		} else {
			setParams = make(map[string]bool, numParams)
			for i := 0; i < numParams && i < numArgs; i++ {
				setParams[fp.parameters[i].Value] = true
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
				if param.Value == key {
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
				kwargsDict.Pairs[object.DictKey(&object.String{Value: key})] = object.DictPair{
					Key:   &object.String{Value: key},
					Value: value,
				}
			}
			env.Set(fp.kwargs.Value, kwargsDict)
		}

		// Check for missing arguments and apply defaults
		for pi, param := range fp.parameters {
			if !isParamSet(pi, param.Value) {
				if defaultExpr, ok := fp.defaultValues[param.Value]; ok {
					defaultVal := Eval(defaultExpr, fp.parentEnv)
					env.Set(param.Value, defaultVal)
				} else {
					minArgs := numParams - len(fp.defaultValues)
					return nil, errors.NewArgumentError(numArgs, minArgs)
				}
			}
		}
	} else {
		// No keywords - set empty **kwargs dict if defined
		if fp.kwargs != nil {
			env.Set(fp.kwargs.Value, &object.Dict{Pairs: make(map[string]object.DictPair)})
		}

		if numArgs < numParams {
			// No keywords - check for missing required arguments
			for i := numArgs; i < numParams; i++ {
				param := fp.parameters[i]
				if defaultExpr, ok := fp.defaultValues[param.Value]; ok {
					defaultVal := Eval(defaultExpr, fp.parentEnv)
					env.Set(param.Value, defaultVal)
				} else {
					minArgs := numParams - len(fp.defaultValues)
					return nil, errors.NewArgumentError(numArgs, minArgs)
				}
			}
		}
	}

	return env, nil
}

func extendFunctionEnv(fn *object.Function, args []object.Object, keywords map[string]object.Object) (*object.Environment, object.Object) {
	return extendEnvWithParams(funcParams{
		parameters:    fn.Parameters,
		defaultValues: fn.DefaultValues,
		variadic:      fn.Variadic,
		kwargs:        fn.Kwargs,
		parentEnv:     fn.Env,
	}, args, keywords)
}

func extendLambdaEnv(fn *object.LambdaFunction, args []object.Object, keywords map[string]object.Object) (*object.Environment, object.Object) {
	return extendEnvWithParams(funcParams{
		parameters:    fn.Parameters,
		defaultValues: fn.DefaultValues,
		variadic:      fn.Variadic,
		kwargs:        fn.Kwargs,
		parentEnv:     fn.Env,
	}, args, keywords)
}

func unwrapReturnValue(obj object.Object) object.Object {
	if returnValue, ok := obj.(*object.ReturnValue); ok {
		return returnValue.Value
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
			return v.Value
		case *object.Integer:
			return v.Value != 0
		case *object.Float:
			return v.Value != 0.0
		case *object.String:
			return v.Value != ""
		case *object.List:
			return len(v.Elements) > 0
		case *object.Dict:
			return len(v.Pairs) > 0
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
	currentClass := inst.Class
	for currentClass != nil {
		if fn, ok := currentClass.Methods[method]; ok {
			return fn, true
		}
		currentClass = currentClass.BaseClass
	}
	return nil, false
}

func init() {
	isTruthyInstanceFn = func(inst *object.Instance) bool {
		if fn, ok := findDunderMethod(inst, "__bool__"); ok {
			result := applyFunctionWithContext(context.Background(), fn, prependSelf(inst, nil), nil, inst.Class.Env)
			if b, ok := result.(*object.Boolean); ok {
				return b.Value
			}
		}
		if fn, ok := findDunderMethod(inst, "__len__"); ok {
			result := applyFunctionWithContext(context.Background(), fn, prependSelf(inst, nil), nil, inst.Class.Env)
			if i, ok := result.(*object.Integer); ok {
				return i.Value != 0
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
	currentVal, ok := env.Get(node.Name.Value)
	if !ok {
		return errors.NewIdentifierError(node.Name.Value)
	}

	newVal := evalWithContext(ctx, node.Value, env)
	if object.IsError(newVal) {
		return newVal
	}

	var operator string
	switch node.Operator {
	case "+=":
		operator = "+"
	case "-=":
		operator = "-"
	case "*=":
		operator = "*"
	case "/=":
		operator = "/"
	case "//=":
		operator = "//"
	case "%=":
		operator = "%"
	case "&=":
		operator = "&"
	case "|=":
		operator = "|"
	case "^=":
		operator = "^"
	case "<<=":
		operator = "<<"
	case ">>=":
		operator = ">>"
	default:
		return errors.NewError("unknown augmented assignment operator: %s", node.Operator)
	}

	result := evalInfixExpression(ctx, operator, currentVal, newVal, env)
	if object.IsError(result) {
		return result
	}

	env.Set(node.Name.Value, result)
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
	err := importCallback(is.Name.Value)
	if err != nil {
		return errors.NewError("%s at line %d: %s", errors.ErrImportError, is.Token.Line, err.Error())
	}

	// Handle alias if present
	if is.Alias != nil {
		moduleObj := getModuleByPath(env, is.Name.Value)
		if moduleObj != nil {
			env.Set(is.Alias.Value, moduleObj)
		}
	}

	// Import additional libraries if any
	for i, name := range is.AdditionalNames {
		if err := importCallback(name.Value); err != nil {
			return errors.NewError("%s: %s", errors.ErrImportError, err.Error())
		}

		// Handle alias for this additional import if present
		if i < len(is.AdditionalAliases) && is.AdditionalAliases[i] != nil {
			moduleObj := getModuleByPath(env, name.Value)
			if moduleObj != nil {
				env.Set(is.AdditionalAliases[i].Value, moduleObj)
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

	// First, import the module
	moduleName := fis.Module.Value

	// Check if module was already in the environment before importing
	// (e.g. user did `import json` before `from json import dumps`)
	_, wasPresent := env.Get(moduleName)

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
			case *object.Library:
				if sub := m.SubLibraries(); sub != nil {
					if subLib, exists := sub[parts[i]]; exists {
						moduleObj = subLib
					} else {
						return errors.NewError("%s: cannot find '%s' in module '%s'", errors.ErrImportError, parts[i], strings.Join(parts[:i], "."))
					}
				} else if funcs := m.Functions(); funcs != nil {
					if fn, exists := funcs[parts[i]]; exists {
						moduleObj = fn
					} else {
						return errors.NewError("%s: cannot find '%s' in module '%s'", errors.ErrImportError, parts[i], strings.Join(parts[:i], "."))
					}
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
			if pair, exists := m.GetByString(name.Value); exists {
				value = pair.Value
				found = true
			}
		case *object.Library:
			// Check functions first
			if funcs := m.Functions(); funcs != nil {
				if fn, exists := funcs[name.Value]; exists {
					value = fn
					found = true
				}
			}
			// Check constants
			if !found {
				if consts := m.Constants(); consts != nil {
					if c, exists := consts[name.Value]; exists {
						value = c
						found = true
					}
				}
			}
			// Check sub-libraries (for classes like BeautifulSoup)
			if !found {
				if subs := m.SubLibraries(); subs != nil {
					if sub, exists := subs[name.Value]; exists {
						value = sub
						found = true
					}
				}
			}
		case *object.Instance:
			if field, exists := m.Fields[name.Value]; exists {
				value = field
				found = true
			}
		}

		if !found {
			return errors.NewError("%s: cannot import name '%s' from '%s'", errors.ErrImportError, name.Value, moduleName)
		}

		// Use alias if provided, otherwise use the original name
		bindName := name.Value
		if fis.Aliases[i] != nil {
			bindName = fis.Aliases[i].Value
		}

		env.Set(bindName, value)
	}

	// Remove the module from the environment (from X import Y should not make X available)
	// But only if the module was NOT already in the environment before this from-import.
	// This preserves modules that were explicitly imported (e.g. `import json` before `from json import dumps`).
	// For dotted imports (e.g. from a.b.c import X), only the full dotted name is deleted.
	// The root module (parts[0]) is NOT deleted as it may be needed by other imports.
	if !wasPresent {
		env.Delete(moduleName)
	}

	return NULL
}

func evalInOperator(left, right object.Object) object.Object {
	switch container := right.(type) {
	case *object.List:
		for _, elem := range container.Elements {
			if left == elem || objectsDeepEqual(left, elem) {
				return TRUE
			}
		}
		return FALSE
	case *object.Dict:
		key := object.DictKey(left)
		_, ok := container.Pairs[key]
		return nativeBoolToBooleanObject(ok)
	case *object.String:
		if needle, ok := left.(*object.String); ok {
			return nativeBoolToBooleanObject(strings.Contains(container.Value, needle.Value))
		}
		return errors.NewTypeError("STRING", "non-string type")
	case *object.DictKeys:
		key := object.DictKey(left)
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
		// Expect left to be a tuple/list of [key, value]
		// Actually, Python allows (key, value) tuple.
		// Let's check if left is a tuple/list of 2 elements.
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
			// Check if key exists and value matches
			keyStr := object.DictKey(key)
			if pair, ok := container.Dict.Pairs[keyStr]; ok {
				if val == pair.Value || objectsDeepEqual(val, pair.Value) {
					return TRUE
				}
			}
		}
		return FALSE
	case *object.Set:
		return nativeBoolToBooleanObject(container.Contains(left))
	case *object.Instance:
		// Call __contains__ dunder method if defined
		if fn, ok := findDunderMethod(container, "__contains__"); ok {
			result := applyFunctionWithContext(context.Background(), fn, prependSelf(container, []object.Object{left}), nil, container.Class.Env)
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

	// For immutable types like small integers and strings, Python caches them
	// so we check both pointer equality and value equality for these types
	switch l := left.(type) {
	case *object.Integer:
		if r, ok := right.(*object.Integer); ok {
			// Python caches small integers (-5 to 256)
			if l.Value >= -5 && l.Value <= 256 && l.Value == r.Value {
				return TRUE
			}
			// Otherwise check pointer equality
			return nativeBoolToBooleanObject(left == right)
		}
		return FALSE
	case *object.String:
		if r, ok := right.(*object.String); ok {
			// Python interns short strings
			if len(l.Value) <= 20 && l.Value == r.Value {
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
	val := evalWithContext(ctx, node.Value, env)
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
			env.Set(node.Names[i].Value, elements[i])
		}

		// Calculate how many elements go to the starred variable
		elementsAfterStar := len(node.Names) - node.StarredIndex - 1
		starStart := node.StarredIndex
		starEnd := len(elements) - elementsAfterStar

		// Assign starred variable (as a list)
		starredElements := elements[starStart:starEnd]
		env.Set(node.Names[node.StarredIndex].Value, &object.List{Elements: starredElements})

		// Assign elements after the starred variable
		for i := 0; i < elementsAfterStar; i++ {
			nameIdx := node.StarredIndex + 1 + i
			elemIdx := starEnd + i
			env.Set(node.Names[nameIdx].Value, elements[elemIdx])
		}
	} else {
		// No starred unpacking - exact length match required
		if len(elements) != len(node.Names) {
			return errors.NewError("cannot unpack %d values to %d variables", len(elements), len(node.Names))
		}

		// Assign each value
		for i, name := range node.Names {
			env.Set(name.Value, elements[i])
		}
	}

	return NULL
}

func evalTryStatementWithContext(ctx context.Context, ts *ast.TryStatement, env *object.Environment) object.Object {
	// Execute try block
	result := evalWithContext(ctx, ts.Body, env)

	// Check if exception or error occurred
	if isException(result) || object.IsError(result) {
		// SystemExit exceptions should NOT be caught by except blocks
		// sys.exit() always exits the program, regardless of try/except
		if exc, ok := result.(*object.Exception); ok && exc.IsSystemExit() {
			// Execute finally block before propagating SystemExit
			if ts.Finally != nil {
				evalWithContext(ctx, ts.Finally, env)
			}
			return result // SystemExit always propagates
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
			// Store the current exception for bare raise support
			env.Set("__current_exception__", exceptionObj)

			// Bind exception to variable if specified
			if exceptClause.ExceptVar != nil {
				env.Set(exceptClause.ExceptVar.Value, exceptionObj)
			}

			// Execute except block in the same environment so variables are accessible
			result = evalWithContext(ctx, exceptClause.Body, env)

			// Clear the current exception after except block
			env.Delete("__current_exception__")

			// If except block didn't re-raise, the exception was handled
			if !isException(result) && !object.IsError(result) {
				result = NULL
			}

			// Exception was handled (or re-raised), don't try other except clauses
			break
		}
	}

	// Always execute finally block if present
	if ts.Finally != nil {
		evalWithContext(ctx, ts.Finally, env)
	}

	return result
}

func evalRaiseStatementWithContext(ctx context.Context, rs *ast.RaiseStatement, env *object.Environment) object.Object {
	if rs.Message != nil {
		msg := evalWithContext(ctx, rs.Message, env)
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
	condition := evalWithContext(ctx, as.Condition, env)
	if object.IsError(condition) {
		return condition
	}

	if !isTruthy(condition) {
		var message string
		if as.Message != nil {
			msg := evalWithContext(ctx, as.Message, env)
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
	ctxObj := evalWithContext(ctx, ws.ContextExpr, env)
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
		env.Set(ws.Target.Value, enterResult)
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
			excType = &object.String{Value: exc.ExceptionType}
			excVal = &object.String{Value: exc.Message}
		} else if err, ok := result.(*object.Error); ok {
			excType = &object.String{Value: "Exception"}
			excVal = &object.String{Value: err.Message}
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
	if obj == nil {
		return false
	}
	return obj.Type() == object.EXCEPTION_OBJ
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

	// Evaluate the exception type expression to get the type name
	var expectedType string
	switch expr := exceptTypeExpr.(type) {
	case *ast.Identifier:
		expectedType = expr.Value
	case *ast.IndexExpression:
		// Handle dotted names like requests.HTTPError — match on the last component
		dotted := buildDottedName(expr)
		parts := strings.Split(dotted, ".")
		expectedType = parts[len(parts)-1]
	default:
		return false
	}

	// "Exception" catches all errors and exceptions
	if expectedType == "Exception" {
		return true
	}

	// Check for exact match
	if exceptionType == expectedType {
		return true
	}

	return false
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
			parts = append([]string{ident.Value}, parts...)
			break
		} else {
			break
		}
	}

	return strings.Join(parts, ".")
}

func assignToExpression(ctx context.Context, expr ast.Expression, value object.Object, env *object.Environment) error {
	switch left := expr.(type) {
	case *ast.Identifier:
		env.Set(left.Value, value)
		return nil
	case *ast.IndexExpression:
		obj := evalWithContext(ctx, left.Left, env)
		if object.IsError(obj) {
			return fmt.Errorf("assignment error")
		}
		index := evalWithContext(ctx, left.Index, env)
		if object.IsError(index) {
			return fmt.Errorf("assignment error")
		}
		switch o := obj.(type) {
		case *object.List:
			if idx, ok := index.(*object.Integer); ok {
				i := idx.Value
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
			key := object.DictKey(index)
			o.Pairs[key] = object.DictPair{Key: index, Value: value}
			return nil
		case *object.Instance:
			if key, ok := index.(*object.String); ok {
				// Check class hierarchy for a property descriptor before writing to Fields
				if p := findPropertyInClass(key.Value, o.Class); p != nil {
					if p.Setter == nil {
						return fmt.Errorf("can't set attribute '%s': property is read-only", key.Value)
					}
					result := applyFunctionWithContext(ctx, p.Setter, []object.Object{o, value}, nil, nil)
					if object.IsError(result) {
						return fmt.Errorf("%s", result.(*object.Error).Message)
					}
					return nil
				}
				o.Fields[key.Value] = value
				return nil
			}
			return fmt.Errorf("instance attribute must be string")
		case *object.Class:
			if key, ok := index.(*object.String); ok {
				o.Methods[key.Value] = value
				return nil
			}
			return fmt.Errorf("class attribute must be string")
		}
		return fmt.Errorf("cannot assign to index")
	default:
		return fmt.Errorf("cannot assign to expression")
	}
}

// findPropertyInClass walks the class hierarchy looking for a Property descriptor.
func findPropertyInClass(name string, class *object.Class) *object.Property {
	for c := class; c != nil; c = c.BaseClass {
		if fn, ok := c.Methods[name]; ok {
			if prop, ok := fn.(*object.Property); ok {
				return prop
			}
			return nil // found as non-property; stop
		}
	}
	return nil
}

func setForVariables(variables []ast.Expression, value object.Object, env *object.Environment) error {
	if len(variables) == 1 {
		if ident, ok := variables[0].(*ast.Identifier); ok {
			env.Set(ident.Value, value)
			return nil
		}
		return fmt.Errorf("for loop variable must be an identifier")
	}

	// Unpacking - get elements from tuple or list
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
		if ident, ok := varExpr.(*ast.Identifier); ok {
			env.Set(ident.Value, elements[i])
		} else {
			return fmt.Errorf("for loop variables must be identifiers")
		}
	}
	return nil
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
	iterable := evalWithContext(ctx, fs.Iterable, env)
	if object.IsError(iterable) {
		return iterable
	}

	var result object.Object = NULL

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
			// Check context frequently in loops for responsiveness
			if err := cc.checkAlways(); err != nil {
				return err
			}

			element, hasNext := iter.Next()
			if !hasNext {
				break
			}

			if err := setForVariables(fs.Variables, element, env); err != nil {
				return errors.NewError("%s", err.Error())
			}

			result = evalWithContext(ctx, fs.Body, env)
			if result != nil {
				switch result.Type() {
				case object.ERROR_OBJ, object.RETURN_OBJ:
					return result
				case object.BREAK_OBJ:
					return NULL
				case object.CONTINUE_OBJ:
					continue
				}
			}
		}
		return result
	}

	// Get elements to iterate over based on type
	var elements []object.Object
	switch iter := iterable.(type) {
	case *object.List:
		elements = iter.Elements
	case *object.Tuple:
		elements = iter.Elements
	case *object.String:
		// Iterate over string runes lazily to avoid pre-allocating all characters
		cc := newContextChecker(ctx)
		for _, char := range iter.Value {
			if err := cc.checkAlways(); err != nil {
				return err
			}

			element := &object.String{Value: string(char)}
			if err := setForVariables(fs.Variables, element, env); err != nil {
				return errors.NewError("%s", err.Error())
			}

			result = evalWithContext(ctx, fs.Body, env)
			if result != nil {
				switch result.Type() {
				case object.ERROR_OBJ, object.RETURN_OBJ:
					return result
				case object.BREAK_OBJ:
					return NULL
				case object.CONTINUE_OBJ:
					continue
				}
			}
		}
		return result
	default:
		return errors.NewTypeError("iterable", iterable.Type().String())
	}

	// Single loop for all iterable types
	for _, element := range elements {
		if err := checkContext(ctx); err != nil {
			return err
		}

		if err := setForVariables(fs.Variables, element, env); err != nil {
			return errors.NewError("%s", err.Error())
		}

		result = evalWithContext(ctx, fs.Body, env)
		if result != nil {
			switch result.Type() {
			case object.ERROR_OBJ, object.RETURN_OBJ:
				return result
			case object.BREAK_OBJ:
				return NULL
			case object.CONTINUE_OBJ:
				continue
			}
		}
	}

	return result
}

// evalMethodCallExpression is in methods.go
// callStringMethodWithKeywords is in methods.go

func evalListComprehension(ctx context.Context, lc *ast.ListComprehension, env *object.Environment) object.Object {
	iterable := evalWithContext(ctx, lc.Iterable, env)
	if object.IsError(iterable) {
		return iterable
	}

	result := []object.Object{}

	// Create new scope for comprehension variable(s)
	compEnv := object.NewEnclosedEnvironment(env)

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
		for {
			element, hasNext := iter.Next()
			if !hasNext {
				break
			}

			// Set variable(s) - supports tuple unpacking
			if err := setForVariables(lc.Variables, element, compEnv); err != nil {
				return errors.NewError("%s", err.Error())
			}

			// Check condition if present
			if lc.Condition != nil {
				condition := evalWithContext(ctx, lc.Condition, compEnv)
				if object.IsError(condition) {
					return condition
				}
				if !isTruthy(condition) {
					continue
				}
			}

			// Evaluate expression
			exprResult := evalWithContext(ctx, lc.Expression, compEnv)
			if object.IsError(exprResult) {
				return exprResult
			}
			result = append(result, exprResult)
		}
		return &object.List{Elements: result}
	}

	// Get elements based on iterable type
	var elements []object.Object
	switch iter := iterable.(type) {
	case *object.List:
		elements = iter.Elements
	case *object.Tuple:
		elements = iter.Elements
	case *object.String:
		// Iterate over string runes lazily to avoid pre-allocating all characters
		for _, char := range iter.Value {
			element := &object.String{Value: string(char)}
			if err := setForVariables(lc.Variables, element, compEnv); err != nil {
				return errors.NewError("%s", err.Error())
			}

			if lc.Condition != nil {
				condition := evalWithContext(ctx, lc.Condition, compEnv)
				if object.IsError(condition) {
					return condition
				}
				if !isTruthy(condition) {
					continue
				}
			}

			exprResult := evalWithContext(ctx, lc.Expression, compEnv)
			if object.IsError(exprResult) {
				return exprResult
			}
			result = append(result, exprResult)
		}
		return &object.List{Elements: result}
	default:
		return errors.NewTypeError("iterable", iterable.Type().String())
	}

	for _, element := range elements {
		// Set variable(s) - supports tuple unpacking
		if err := setForVariables(lc.Variables, element, compEnv); err != nil {
			return errors.NewError("%s", err.Error())
		}

		// Check condition if present
		if lc.Condition != nil {
			condition := evalWithContext(ctx, lc.Condition, compEnv)
			if object.IsError(condition) {
				return condition
			}
			if !isTruthy(condition) {
				continue
			}
		}

		// Evaluate expression
		exprResult := evalWithContext(ctx, lc.Expression, compEnv)
		if object.IsError(exprResult) {
			return exprResult
		}
		result = append(result, exprResult)
	}

	return &object.List{Elements: result}
}

func evalLambda(lambda *ast.Lambda, env *object.Environment) object.Object {
	return &object.LambdaFunction{
		Parameters:    lambda.Parameters,
		DefaultValues: lambda.DefaultValues,
		Variadic:      lambda.Variadic,
		Kwargs:        lambda.Kwargs,
		Body:          lambda.Body,
		Env:           env,
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
			exprResult := evalWithContext(ctx, fstr.Expressions[i], env)
			if object.IsError(exprResult) {
				return exprResult
			}
			// Call __str__ on instances for f-string formatting
			if inst, ok := exprResult.(*object.Instance); ok && fstr.FormatSpecs[i] == "" {
				if result := callDunderMethod(ctx, inst, "__str__", nil, env); result != nil {
					exprResult = result
				}
			}
			formatted := formatWithSpec(exprResult, fstr.FormatSpecs[i])
			builder.WriteString(formatted)
		}
	}

	return &object.String{Value: builder.String()}
}

func formatWithSpec(obj object.Object, spec string) string {
	if spec == "" {
		// Check the actual object type first to preserve float representation
		switch obj.Type() {
		case object.INTEGER_OBJ:
			if intVal, err := obj.AsInt(); err == nil {
				return fmt.Sprintf("%d", intVal)
			}
		case object.FLOAT_OBJ:
			if floatVal, err := obj.AsFloat(); err == nil {
				// Check if it's a whole number
				if floatVal == float64(int64(floatVal)) {
					return fmt.Sprintf("%.1f", floatVal)
				}
				return fmt.Sprintf("%g", floatVal)
			}
		}
		return obj.Inspect()
	}

	// Handle integer formatting like :2d, :02d
	if strings.HasSuffix(spec, "d") {
		widthStr := strings.TrimSuffix(spec, "d")
		if widthStr == "" {
			if intVal, err := obj.AsInt(); err == nil {
				return fmt.Sprintf("%d", intVal)
			}
		} else if widthStr[0] == '0' {
			// Zero-padded
			widthStr = widthStr[1:]
			if width, err := strconv.Atoi(widthStr); err == nil {
				if intVal, err := obj.AsInt(); err == nil {
					return fmt.Sprintf("%0*d", width, intVal)
				}
			}
		} else {
			// Space-padded
			if width, err := strconv.Atoi(widthStr); err == nil {
				if intVal, err := obj.AsInt(); err == nil {
					return fmt.Sprintf("%*d", width, intVal)
				}
			}
		}
	}

	// Fallback to inspect
	return obj.Inspect()
}

func evalMatchStatementWithContext(ctx context.Context, ms *ast.MatchStatement, env *object.Environment) object.Object {
	subject := evalWithContext(ctx, ms.Subject, env)
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
				guardResult := evalWithContext(ctx, caseClause.Guard, env)
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
				env.Set(caseClause.CaptureAs.Value, capturedValue)
			}

			// Execute body in the environment (with captures)
			return evalWithContext(ctx, caseClause.Body, env)
		}
	}

	return NULL
}

func matchPattern(ctx context.Context, subject object.Object, pattern ast.Expression, capturedVars map[string]object.Object) (object.Object, object.Object) {
	switch p := pattern.(type) {
	case *ast.Identifier:
		// Wildcard pattern
		if p.Value == "_" {
			return TRUE, subject
		}

		// All other identifiers are capture variables (always match)
		// Bind the captured value to the identifier name
		capturedVars[p.Value] = subject
		return TRUE, subject

	case *ast.CallExpression:
		// Handle type patterns like int(), str(), list(), dict()
		if ident, ok := p.Function.(*ast.Identifier); ok {
			// Check if it's a type constructor with no arguments
			if len(p.Arguments) == 0 && len(p.Keywords) == 0 {
				typeName := ident.Value
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
			if intObj.Value == p.Value {
				return TRUE, subject
			}
		}
		return FALSE, NULL

	case *ast.FloatLiteral:
		if floatObj, ok := subject.(*object.Float); ok {
			if floatObj.Value == p.Value {
				return TRUE, subject
			}
		}
		return FALSE, NULL

	case *ast.StringLiteral:
		if strObj, ok := subject.(*object.String); ok {
			if strObj.Value == p.Value {
				return TRUE, subject
			}
		}
		return FALSE, NULL

	case *ast.Boolean:
		if boolObj, ok := subject.(*object.Boolean); ok {
			if boolObj.Value == p.Value {
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
		for keyExpr, valueExpr := range p.Pairs {
			keyObj := evalWithContext(ctx, keyExpr, object.NewEnvironment())
			if object.IsError(keyObj) {
				return keyObj, NULL
			}

			keyStr := object.DictKey(keyObj)
			pair, exists := dictObj.Pairs[keyStr]
			if !exists {
				return FALSE, NULL
			}

			// If pattern value is an identifier (not _), it's a capture variable
			if ident, ok := valueExpr.(*ast.Identifier); ok && ident.Value != "_" {
				// Store the captured value
				capturedVars[ident.Value] = pair.Value
			} else {
				// Otherwise, it must match exactly
				matched, _ := matchPattern(ctx, pair.Value, valueExpr, capturedVars)
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
