package evaluator

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/paularlott/scriptling/ast"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/extlibs"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/stdlib"
)

var (
	NULL  = &object.Null{}
	TRUE  = &object.Boolean{Value: true}
	FALSE = &object.Boolean{Value: false}
)

// envContextKey is used to store environment in context
const envContextKey = "scriptling-env"

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
func init() {
	// Set up the function caller for functools.reduce
	stdlib.SetFunctionCaller(func(ctx context.Context, fn *object.Function, args []object.Object, keywords map[string]object.Object) object.Object {
		return applyFunctionWithContext(ctx, fn, args, keywords, fn.Env)
	})

	// Set up the method caller for html.parser (and other extlibs that need to call user methods)
	extlibs.ApplyMethodFunc = func(ctx context.Context, instance *object.Instance, method *object.Function, args []object.Object) object.Object {
		// Prepend instance (self) to args
		allArgs := append([]object.Object{instance}, args...)
		return applyFunctionWithContext(ctx, method, allArgs, nil, method.Env)
	}
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

func evalWithContext(ctx context.Context, node ast.Node, env *object.Environment) object.Object {
	obj := evalNode(ctx, node, env)
	if err, ok := obj.(*object.Error); ok {
		if err.Line == 0 {
			err.Line = node.Line()
		}
	}
	return obj
}

func evalNode(ctx context.Context, node ast.Node, env *object.Environment) object.Object {
	// Check for cancellation
	if err := checkContext(ctx); err != nil {
		return err
	}
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
		if isError(right) {
			return right
		}
		return evalPrefixExpression(node.Operator, right)
	case *ast.InfixExpression:
		left := evalWithContext(ctx, node.Left, env)
		if isError(left) {
			return left
		}
		right := evalWithContext(ctx, node.Right, env)
		if isError(right) {
			return right
		}
		return evalInfixExpression(node.Operator, left, right)
	case *ast.ConditionalExpression:
		return evalConditionalExpression(ctx, node, env)
	case *ast.BlockStatement:
		return evalBlockStatementWithContext(ctx, node, env)
	case *ast.IfStatement:
		return evalIfStatementWithContext(ctx, node, env)
	case *ast.WhileStatement:
		return evalWhileStatementWithContext(ctx, node, env)
	case *ast.ReturnStatement:
		val := object.Object(NULL)
		if node.ReturnValue != nil {
			val = evalWithContext(ctx, node.ReturnValue, env)
			if isError(val) {
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
		if isError(val) || isException(val) {
			return val
		}
		if err := assignToExpression(node.Left, val, env); err != nil {
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
		if len(elements) == 1 && isError(elements[0]) {
			return elements[0]
		}
		return &object.List{Elements: elements}
	case *ast.DictLiteral:
		return evalDictLiteralWithContext(ctx, node, env)
	case *ast.IndexExpression:
		left := evalWithContext(ctx, node.Left, env)
		if isError(left) {
			return left
		}
		index := evalWithContext(ctx, node.Index, env)
		if isError(index) {
			return index
		}
		return evalIndexExpression(left, index)
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
	case *ast.MethodCallExpression:
		return evalMethodCallExpression(ctx, node, env)
	case *ast.ListComprehension:
		return evalListComprehension(ctx, node, env)
	case *ast.Lambda:
		return evalLambda(node, env)
	case *ast.TupleLiteral:
		elements := evalExpressionsWithContext(ctx, node.Elements, env)
		if len(elements) == 1 && isError(elements[0]) {
			return elements[0]
		}
		return &object.Tuple{Elements: elements}
	}
	return NULL
}

func evalProgram(ctx context.Context, program *ast.Program, env *object.Environment) object.Object {
	var result object.Object = NULL

	for _, statement := range program.Statements {
		// Check for cancellation in loops
		if err := checkContext(ctx); err != nil {
			return err
		}

		result = evalWithContext(ctx, statement, env)

		switch result := result.(type) {
		case *object.ReturnValue:
			return result.Value
		case *object.Error:
			return result
		}
	}

	return result
}

func evalBlockStatementWithContext(ctx context.Context, block *ast.BlockStatement, env *object.Environment) object.Object {
	var result object.Object = NULL

	for _, statement := range block.Statements {
		// Check for cancellation in loops
		if err := checkContext(ctx); err != nil {
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

func evalInfixExpression(operator string, left, right object.Object) object.Object {
	// Handle boolean operators first (they work with any type)
	switch operator {
	case "and":
		// Short-circuit: return first falsy value or last value
		if !isTruthy(left) {
			return left
		}
		return right
	case "or":
		// Short-circuit: return first truthy value or last value
		if isTruthy(left) {
			return left
		}
		return right
	}

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
	if isError(condition) {
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
		// Integer exponentiation
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
		return &object.String{Value: leftVal + rightVal}
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

func evalIfStatementWithContext(ctx context.Context, ie *ast.IfStatement, env *object.Environment) object.Object {
	condition := evalWithContext(ctx, ie.Condition, env)
	if isError(condition) {
		return condition
	}

	if isTruthy(condition) {
		return evalWithContext(ctx, ie.Consequence, env)
	}

	// Check elif clauses
	for _, elifClause := range ie.ElifClauses {
		condition := evalWithContext(ctx, elifClause.Condition, env)
		if isError(condition) {
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

	for {
		// Check for cancellation in loops
		if err := checkContext(ctx); err != nil {
			return err
		}

		condition := evalWithContext(ctx, ws.Condition, env)
		if isError(condition) {
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
		Body:          stmt.Function.Body,
		Env:           env,
	}
	env.Set(stmt.Name.Value, fn)
	return fn
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
		if isError(baseClassObj) {
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
			if fn, ok := obj.(*object.Function); ok {
				class.Methods[fn.Name] = fn
			}
		}
	}

	env.Set(stmt.Name.Value, class)
	return class
}

func evalCallExpression(ctx context.Context, node *ast.CallExpression, env *object.Environment) object.Object {
	function := evalWithContext(ctx, node.Function, env)
	if isError(function) {
		return function
	}
	args := evalExpressionsWithContext(ctx, node.Arguments, env)
	if len(args) == 1 && isError(args[0]) {
		return args[0]
	}

	keywords := make(map[string]object.Object)
	for k, v := range node.Keywords {
		val := evalWithContext(ctx, v, env)
		if isError(val) {
			return val
		}
		keywords[k] = val
	}

	return applyFunctionWithContext(ctx, function, args, keywords, env)
}

func createInstance(ctx context.Context, class *object.Class, args []object.Object, keywords map[string]object.Object, env *object.Environment) object.Object {
	instance := &object.Instance{
		Class:  class,
		Fields: make(map[string]object.Object),
	}

	// Call __init__ if it exists
	if initMethod, ok := class.Methods["__init__"]; ok {
		// Bind 'self' to the instance
		// We need to call the method, but applyFunction expects a Function object.
		// We can reuse applyFunctionWithContext but we need to prepend 'self' to args.
		newArgs := append([]object.Object{instance}, args...)
		result := applyFunctionWithContext(ctx, initMethod, newArgs, keywords, env)
		if isError(result) {
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
		if isError(evaluated) {
			return []object.Object{evaluated}
		}
		result[i] = evaluated
	}

	return result
}

func applyUserFunction(ctx context.Context, fn *object.Function, args []object.Object, keywords map[string]object.Object, env *object.Environment) object.Object {
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

func applyFunctionWithContext(ctx context.Context, fn object.Object, args []object.Object, keywords map[string]object.Object, env *object.Environment) object.Object {
	switch fn := fn.(type) {
	case *object.Function:
		return applyUserFunction(ctx, fn, args, keywords, env)
	case *object.LambdaFunction:
		return applyLambdaFunctionWithContext(ctx, fn, args, keywords, env)
	case *object.Builtin:
		ctxWithEnv := SetEnvInContext(ctx, env)
		return fn.Fn(ctxWithEnv, keywords, args...)
	case *object.Class:
		return createInstance(ctx, fn, args, keywords, env)
	default:
		return errors.NewError("not a function or class: %s", fn.Type())
	}
}

func applyLambdaFunctionWithContext(ctx context.Context, fn *object.LambdaFunction, args []object.Object, keywords map[string]object.Object, env *object.Environment) object.Object {
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
		setParams := make(map[string]bool, numParams)
		// Mark positional args as set
		for i := 0; i < numParams && i < numArgs; i++ {
			setParams[fp.parameters[i].Value] = true
		}

		for key, value := range keywords {
			// Check if parameter exists
			paramExists := false
			for _, param := range fp.parameters {
				if param.Value == key {
					paramExists = true
					break
				}
			}

			if !paramExists {
				return nil, errors.NewError("got an unexpected keyword argument '%s'", key)
			}

			if setParams[key] {
				return nil, errors.NewError("multiple values for argument '%s'", key)
			}

			env.Set(key, value)
			setParams[key] = true
		}

		// Check for missing arguments and apply defaults
		for _, param := range fp.parameters {
			if !setParams[param.Value] {
				if defaultExpr, ok := fp.defaultValues[param.Value]; ok {
					defaultVal := Eval(defaultExpr, fp.parentEnv)
					env.Set(param.Value, defaultVal)
				} else {
					minArgs := numParams - len(fp.defaultValues)
					return nil, errors.NewArgumentError(numArgs, minArgs)
				}
			}
		}
	} else if numArgs < numParams {
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

	return env, nil
}

func extendFunctionEnv(fn *object.Function, args []object.Object, keywords map[string]object.Object) (*object.Environment, object.Object) {
	return extendEnvWithParams(funcParams{
		parameters:    fn.Parameters,
		defaultValues: fn.DefaultValues,
		variadic:      fn.Variadic,
		parentEnv:     fn.Env,
	}, args, keywords)
}

func extendLambdaEnv(fn *object.LambdaFunction, args []object.Object, keywords map[string]object.Object) (*object.Environment, object.Object) {
	return extendEnvWithParams(funcParams{
		parameters:    fn.Parameters,
		defaultValues: fn.DefaultValues,
		variadic:      fn.Variadic,
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
		default:
			return true
		}
	}
}

func isError(obj object.Object) bool {
	if obj != nil {
		return obj.Type() == object.ERROR_OBJ
	}
	return false
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
	if isError(newVal) {
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

	result := evalInfixExpression(operator, currentVal, newVal)
	if isError(result) {
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

	// Import additional libraries if any
	for _, name := range is.AdditionalNames {
		if err := importCallback(name.Value); err != nil {
			return errors.NewError("%s: %s", errors.ErrImportError, err.Error())
		}
	}

	return NULL
}

func evalFromImportStatement(fis *ast.FromImportStatement, env *object.Environment) object.Object {
	importCallback := env.GetImportCallback()
	if importCallback == nil {
		return errors.NewError(errors.ErrImportError)
	}

	// First, import the module
	moduleName := fis.Module.Value
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
				if pair, exists := m.Pairs[parts[i]]; exists {
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
			if pair, exists := m.Pairs[name.Value]; exists {
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

	return NULL
}

func evalInOperator(left, right object.Object) object.Object {
	switch container := right.(type) {
	case *object.List:
		for _, elem := range container.Elements {
			if left == elem || (left.Inspect() == elem.Inspect()) {
				return TRUE
			}
		}
		return FALSE
	case *object.Dict:
		key := left.Inspect()
		_, ok := container.Pairs[key]
		return nativeBoolToBooleanObject(ok)
	case *object.String:
		if needle, ok := left.(*object.String); ok {
			for i := 0; i <= len(container.Value)-len(needle.Value); i++ {
				if container.Value[i:i+len(needle.Value)] == needle.Value {
					return TRUE
				}
			}
			return FALSE
		}
		return errors.NewTypeError("STRING", "non-string type")
	case *object.DictKeys:
		key := left.Inspect()
		_, ok := container.Dict.Pairs[key]
		return nativeBoolToBooleanObject(ok)
	case *object.DictValues:
		for _, pair := range container.Dict.Pairs {
			if left == pair.Value || (left.Inspect() == pair.Value.Inspect()) {
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
			keyStr := key.Inspect()
			if pair, ok := container.Dict.Pairs[keyStr]; ok {
				if val == pair.Value || (val.Inspect() == pair.Value.Inspect()) {
					return TRUE
				}
			}
		}
		return FALSE
	case *object.Set:
		return nativeBoolToBooleanObject(container.Contains(left))
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
	if isError(val) {
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

	// Check length matches
	if len(elements) != len(node.Names) {
		return errors.NewError("cannot unpack %d values to %d variables", len(elements), len(node.Names))
	}

	// Assign each value
	for i, name := range node.Names {
		env.Set(name.Value, elements[i])
	}

	return NULL
}

func evalTryStatementWithContext(ctx context.Context, ts *ast.TryStatement, env *object.Environment) object.Object {
	// Execute try block
	result := evalWithContext(ctx, ts.Body, env)

	// Check if exception or error occurred
	if isException(result) || isError(result) {
		// Execute except block if present
		if ts.Except != nil {
			// Bind exception to variable if specified
			if ts.ExceptVar != nil {
				env.Set(ts.ExceptVar.Value, result)
			}

			// Execute except block in the same environment so variables are accessible
			result = evalWithContext(ctx, ts.Except, env)
		}
	}

	// Always execute finally block if present
	if ts.Finally != nil {
		evalWithContext(ctx, ts.Finally, env)
	}

	// Clear exception if it was handled
	if (isException(result) || isError(result)) && ts.Except != nil {
		return NULL
	}

	return result
}

func evalRaiseStatementWithContext(ctx context.Context, rs *ast.RaiseStatement, env *object.Environment) object.Object {
	var message string
	if rs.Message != nil {
		msg := evalWithContext(ctx, rs.Message, env)
		if isError(msg) {
			return msg
		}
		message = msg.Inspect()
	} else {
		message = "Exception raised"
	}
	return &object.Exception{Message: message}
}

func evalAssertStatementWithContext(ctx context.Context, as *ast.AssertStatement, env *object.Environment) object.Object {
	condition := evalWithContext(ctx, as.Condition, env)
	if isError(condition) {
		return condition
	}

	if !isTruthy(condition) {
		var message string
		if as.Message != nil {
			msg := evalWithContext(ctx, as.Message, env)
			if isError(msg) {
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

func isException(obj object.Object) bool {
	if obj == nil {
		return false
	}
	return obj.Type() == object.EXCEPTION_OBJ
}

func assignToExpression(expr ast.Expression, value object.Object, env *object.Environment) error {
	switch left := expr.(type) {
	case *ast.Identifier:
		env.Set(left.Value, value)
		return nil
	case *ast.IndexExpression:
		obj := evalWithContext(context.Background(), left.Left, env)
		if isError(obj) {
			return fmt.Errorf("assignment error")
		}
		index := evalWithContext(context.Background(), left.Index, env)
		if isError(index) {
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
			key := index.Inspect()
			o.Pairs[key] = object.DictPair{Key: index, Value: value}
			return nil
		case *object.Instance:
			if key, ok := index.(*object.String); ok {
				o.Fields[key.Value] = value
				return nil
			}
			return fmt.Errorf("instance attribute must be string")
		}
		return fmt.Errorf("cannot assign to index")
	default:
		return fmt.Errorf("cannot assign to expression")
	}
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

func evalForStatementWithContext(ctx context.Context, fs *ast.ForStatement, env *object.Environment) object.Object {
	iterable := evalWithContext(ctx, fs.Iterable, env)
	if isError(iterable) {
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
	}

	if iter != nil {
		for {
			if err := checkContext(ctx); err != nil {
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
		// Convert string chars to objects
		elements = make([]object.Object, len(iter.Value))
		for i, char := range iter.Value {
			elements[i] = &object.String{Value: string(char)}
		}
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
	if isError(iterable) {
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
				if isError(condition) {
					return condition
				}
				if !isTruthy(condition) {
					continue
				}
			}

			// Evaluate expression
			exprResult := evalWithContext(ctx, lc.Expression, compEnv)
			if isError(exprResult) {
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
		elements = make([]object.Object, len(iter.Value))
		for i, char := range iter.Value {
			elements[i] = &object.String{Value: string(char)}
		}
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
			if isError(condition) {
				return condition
			}
			if !isTruthy(condition) {
				continue
			}
		}

		// Evaluate expression
		exprResult := evalWithContext(ctx, lc.Expression, compEnv)
		if isError(exprResult) {
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
		Body:          lambda.Body,
		Env:           env,
	}
}

func evalFStringLiteral(ctx context.Context, fstr *ast.FStringLiteral, env *object.Environment) object.Object {
	result := ""

	for i, part := range fstr.Parts {
		result += part
		if i < len(fstr.Expressions) {
			exprResult := evalWithContext(ctx, fstr.Expressions[i], env)
			if isError(exprResult) {
				return exprResult
			}
			formatted := formatWithSpec(exprResult, fstr.FormatSpecs[i])
			result += formatted
		}
	}

	return &object.String{Value: result}
}

func formatWithSpec(obj object.Object, spec string) string {
	if spec == "" {
		// Check for integers first (before float conversion)
		if intVal, ok := obj.AsInt(); ok {
			return fmt.Sprintf("%d", intVal)
		}
		// For floats, provide a more Python-like representation
		if floatVal, ok := obj.AsFloat(); ok {
			// Check if it's a whole number
			if floatVal == float64(int64(floatVal)) {
				return fmt.Sprintf("%.1f", floatVal)
			}
			return fmt.Sprintf("%g", floatVal)
		}
		return obj.Inspect()
	}

	// Handle integer formatting like :2d, :02d
	if strings.HasSuffix(spec, "d") {
		widthStr := strings.TrimSuffix(spec, "d")
		if widthStr == "" {
			if intVal, ok := obj.AsInt(); ok {
				return fmt.Sprintf("%d", intVal)
			}
		} else if widthStr[0] == '0' {
			// Zero-padded
			widthStr = widthStr[1:]
			if width, err := strconv.Atoi(widthStr); err == nil {
				if intVal, ok := obj.AsInt(); ok {
					return fmt.Sprintf("%0*d", width, intVal)
				}
			}
		} else {
			// Space-padded
			if width, err := strconv.Atoi(widthStr); err == nil {
				if intVal, ok := obj.AsInt(); ok {
					return fmt.Sprintf("%*d", width, intVal)
				}
			}
		}
	}

	// Fallback to inspect
	return obj.Inspect()
}
