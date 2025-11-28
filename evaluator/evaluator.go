package evaluator

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode"

	"github.com/paularlott/scriptling/ast"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/stdlib"
)

var (
	NULL  = &object.Null{}
	TRUE  = &object.Boolean{Value: true}
	FALSE = &object.Boolean{Value: false}
)

// envContextKey is used to store environment in context
var envContextKey = struct{}{}

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

// init sets up callbacks for stdlib functions that need to call user functions
func init() {
	// Set up the function caller for functools.reduce
	stdlib.SetFunctionCaller(func(ctx context.Context, fn *object.Function, args []object.Object) object.Object {
		return applyFunctionWithContext(ctx, fn, args, nil, fn.Env)
	})
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
		fn := &object.Function{
			Name:          node.Name.Value,
			Parameters:    node.Function.Parameters,
			DefaultValues: node.Function.DefaultValues,
			Variadic:      node.Function.Variadic,
			Body:          node.Function.Body,
			Env:           env,
		}
		env.Set(node.Name.Value, fn)
		return fn
	case *ast.CallExpression:
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

func applyFunctionWithContext(ctx context.Context, fn object.Object, args []object.Object, keywords map[string]object.Object, env *object.Environment) object.Object {
	switch fn := fn.(type) {
	case *object.Function:
		extendedEnv, err := extendFunctionEnv(fn, args, keywords)
		if err != nil {
			return err
		}
		evaluated := evalWithContext(ctx, fn.Body, extendedEnv)
		return unwrapReturnValue(evaluated)
	case *object.LambdaFunction:
		extendedEnv, err := extendLambdaEnv(fn, args, keywords)
		if err != nil {
			return err
		}
		evaluated := evalWithContext(ctx, fn.Body, extendedEnv)
		return evaluated // No unwrapping needed for lambda expressions
	case *object.Builtin:
		ctxWithEnv := SetEnvInContext(ctx, env)
		return fn.Fn(ctxWithEnv, keywords, args...)
	default:
		return errors.NewTypeError("function", fn.Type().String())
	}
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

func evalDictLiteralWithContext(ctx context.Context, node *ast.DictLiteral, env *object.Environment) object.Object {
	if len(node.Pairs) == 0 {
		return &object.Dict{Pairs: make(map[string]object.DictPair)}
	}
	pairs := make(map[string]object.DictPair, len(node.Pairs))

	for keyNode, valueNode := range node.Pairs {
		key := evalWithContext(ctx, keyNode, env)
		if isError(key) {
			return key
		}

		value := evalWithContext(ctx, valueNode, env)
		if isError(value) {
			return value
		}

		pairs[key.Inspect()] = object.DictPair{Key: key, Value: value}
	}

	return &object.Dict{Pairs: pairs}
}

func evalIndexExpression(left, index object.Object) object.Object {
	switch {
	case left.Type() == object.LIST_OBJ && index.Type() == object.INTEGER_OBJ:
		return evalListIndexExpression(left, index)
	case left.Type() == object.TUPLE_OBJ && index.Type() == object.INTEGER_OBJ:
		return evalTupleIndexExpression(left, index)
	case left.Type() == object.DICT_OBJ:
		return evalDictIndexExpression(left, index)
	case left.Type() == object.REGEX_OBJ:
		return evalRegexIndexExpression(left, index)
	case left.Type() == object.STRING_OBJ && index.Type() == object.INTEGER_OBJ:
		return evalStringIndexExpression(left, index)
	default:
		return errors.NewError("index operator not supported: %s", left.Type())
	}
}

func evalDictMemberAccess(dict *object.Dict, member string) object.Object {
	pair, ok := dict.Pairs[member]
	if !ok {
		return NULL
	}
	return pair.Value
}

func evalListIndexExpression(list, index object.Object) object.Object {
	listObject := list.(*object.List)
	idx := index.(*object.Integer).Value
	length := int64(len(listObject.Elements))

	// Handle negative indices
	if idx < 0 {
		idx += length
	}

	if idx < 0 || idx >= length {
		return NULL
	}

	return listObject.Elements[idx]
}

func evalTupleIndexExpression(tuple, index object.Object) object.Object {
	tupleObject := tuple.(*object.Tuple)
	idx := index.(*object.Integer).Value
	length := int64(len(tupleObject.Elements))

	// Handle negative indices
	if idx < 0 {
		idx += length
	}

	if idx < 0 || idx >= length {
		return NULL
	}

	return tupleObject.Elements[idx]
}

func evalDictIndexExpression(dict, index object.Object) object.Object {
	dictObject := dict.(*object.Dict)
	key := index.Inspect()

	pair, ok := dictObject.Pairs[key]
	if !ok {
		return NULL
	}

	return pair.Value
}

func evalStringIndexExpression(str, index object.Object) object.Object {
	strObject := str.(*object.String)
	idx := index.(*object.Integer).Value
	length := int64(len(strObject.Value))

	// Handle negative indices
	if idx < 0 {
		idx += length
	}

	if idx < 0 || idx >= length {
		return NULL
	}

	return &object.String{Value: string(strObject.Value[idx])}
}

func evalRegexIndexExpression(regex, index object.Object) object.Object {
	if index.Type() != object.STRING_OBJ {
		return errors.NewError("regex index must be string")
	}
	method := index.(*object.String).Value
	builtin, ok := regexBuiltins[method]
	if !ok {
		return errors.NewError("regex has no method %s", method)
	}
	return builtin
}

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

func evalSliceExpressionWithContext(ctx context.Context, node *ast.SliceExpression, env *object.Environment) object.Object {
	left := evalWithContext(ctx, node.Left, env)
	if isError(left) {
		return left
	}

	var start, end, step int64
	var hasStart, hasEnd, hasStep bool
	step = 1 // default step

	if node.Start != nil {
		startObj := evalWithContext(ctx, node.Start, env)
		if isError(startObj) {
			return startObj
		}
		if startObj.Type() != object.INTEGER_OBJ {
			return errors.NewTypeError("INTEGER", startObj.Type().String())
		}
		start = startObj.(*object.Integer).Value
		hasStart = true
	}

	if node.End != nil {
		endObj := evalWithContext(ctx, node.End, env)
		if isError(endObj) {
			return endObj
		}
		if endObj.Type() != object.INTEGER_OBJ {
			return errors.NewTypeError("INTEGER", endObj.Type().String())
		}
		end = endObj.(*object.Integer).Value
		hasEnd = true
	}

	if node.Step != nil {
		stepObj := evalWithContext(ctx, node.Step, env)
		if isError(stepObj) {
			return stepObj
		}
		if stepObj.Type() != object.INTEGER_OBJ {
			return errors.NewTypeError("INTEGER", stepObj.Type().String())
		}
		step = stepObj.(*object.Integer).Value
		hasStep = true
		if step == 0 {
			return errors.NewError("slice step cannot be zero")
		}
	}

	switch obj := left.(type) {
	case *object.List:
		return sliceList(obj.Elements, start, end, step, hasStart, hasEnd, hasStep)
	case *object.String:
		elements := sliceString(obj.Value, start, end, step, hasStart, hasEnd, hasStep)
		return &object.String{Value: elements}
	default:
		return errors.NewError("slice operator not supported: %s", left.Type())
	}
}

func sliceList(elements []object.Object, start, end, step int64, hasStart, hasEnd, hasStep bool) object.Object {
	length := int64(len(elements))

	// Handle negative step (reverse iteration)
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

		// Bounds checking
		if start >= length {
			start = length - 1
		}
		if start < 0 {
			start = -1
		}
		if end >= length {
			end = length - 1
		}

		result := []object.Object{}
		for i := start; i > end; i += step {
			if i >= 0 && i < length {
				result = append(result, elements[i])
			}
		}
		return &object.List{Elements: result}
	}

	// Positive step (forward iteration)
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

	// Bounds checking
	if start < 0 {
		start = 0
	}
	if end > length {
		end = length
	}
	if start > end {
		start = end
	}

	// If step is 1, use simple slicing
	if step == 1 {
		return &object.List{Elements: elements[start:end]}
	}

	// Step > 1
	result := []object.Object{}
	for i := start; i < end; i += step {
		result = append(result, elements[i])
	}
	return &object.List{Elements: result}
}

func sliceString(str string, start, end, step int64, hasStart, hasEnd, hasStep bool) string {
	length := int64(len(str))

	// Handle negative step (reverse iteration)
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

		// Bounds checking
		if start >= length {
			start = length - 1
		}
		if start < 0 {
			start = -1
		}
		if end >= length {
			end = length - 1
		}

		result := ""
		for i := start; i > end; i += step {
			if i >= 0 && i < length {
				result += string(str[i])
			}
		}
		return result
	}

	// Positive step (forward iteration)
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

	// Bounds checking
	if start < 0 {
		start = 0
	}
	if end > length {
		end = length
	}
	if start > end {
		start = end
	}

	// If step is 1, use simple slicing
	if step == 1 {
		return str[start:end]
	}

	// Step > 1
	result := ""
	for i := start; i < end; i += step {
		result += string(str[i])
	}
	return result
}

func evalImportStatement(is *ast.ImportStatement, env *object.Environment) object.Object {
	importCallback := env.GetImportCallback()
	if importCallback == nil {
		return errors.NewError(errors.ErrImportError)
	}
	err := importCallback(is.Name.Value)
	if err != nil {
		return errors.NewError("%s: %s", errors.ErrImportError, err.Error())
	}

	// Import additional libraries if any
	for _, name := range is.AdditionalNames {
		if err := importCallback(name.Value); err != nil {
			return errors.NewError("%s: %s", errors.ErrImportError, err.Error())
		}
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
		return &object.Error{Message: "AssertionError: " + message}
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

func evalMethodCallExpression(ctx context.Context, mce *ast.MethodCallExpression, env *object.Environment) object.Object {
	obj := evalWithContext(ctx, mce.Object, env)
	if isError(obj) {
		return obj
	}

	args := evalExpressionsWithContext(ctx, mce.Arguments, env)
	if len(args) == 1 && isError(args[0]) {
		return args[0]
	}

	// Evaluate keyword arguments
	keywords := make(map[string]object.Object)
	for k, v := range mce.Keywords {
		val := evalWithContext(ctx, v, env)
		if isError(val) {
			return val
		}
		keywords[k] = val
	}

	return callStringMethodWithKeywords(ctx, obj, mce.Method.Value, args, keywords, env)
}

func callStringMethodWithKeywords(ctx context.Context, obj object.Object, method string, args []object.Object, keywords map[string]object.Object, env *object.Environment) object.Object {
	// Handle universal methods
	switch method {
	case "type":
		if len(args) != 0 {
			return errors.NewArgumentError(len(args), 0)
		}
		if len(keywords) > 0 {
			return errors.NewError("type() does not accept keyword arguments")
		}
		return &object.String{Value: obj.Type().String()}
	}

	// Handle library method calls (dictionaries)
	if obj.Type() == object.DICT_OBJ {
		dict := obj.(*object.Dict)

		// First check for library methods (callable functions stored in dict)
		// This takes priority over dict instance methods like get, pop, etc.
		if pair, ok := dict.Pairs[method]; ok {
			switch fn := pair.Value.(type) {
			case *object.Builtin:
				ctxWithEnv := SetEnvInContext(ctx, env)
				return fn.Fn(ctxWithEnv, keywords, args...)
			case *object.Function:
				return applyFunctionWithContext(ctx, fn, args, keywords, env)
			case *object.LambdaFunction:
				return applyFunctionWithContext(ctx, fn, args, keywords, env)
			}
			// If it's not a callable, fall through to dict instance methods
		}

		// Check for dict instance methods
		switch method {
		case "keys":
			if len(args) != 0 {
				return errors.NewArgumentError(len(args), 0)
			}
			if len(keywords) > 0 {
				return errors.NewError("keys() does not accept keyword arguments")
			}
			if builtin, ok := builtins["keys"]; ok {
				ctxWithEnv := SetEnvInContext(ctx, env)
				return builtin.Fn(ctxWithEnv, nil, dict)
			}
		case "values":
			if len(args) != 0 {
				return errors.NewArgumentError(len(args), 0)
			}
			if len(keywords) > 0 {
				return errors.NewError("values() does not accept keyword arguments")
			}
			if builtin, ok := builtins["values"]; ok {
				ctxWithEnv := SetEnvInContext(ctx, env)
				return builtin.Fn(ctxWithEnv, nil, dict)
			}
		case "items":
			if len(args) != 0 {
				return errors.NewArgumentError(len(args), 0)
			}
			if len(keywords) > 0 {
				return errors.NewError("items() does not accept keyword arguments")
			}
			if builtin, ok := builtins["items"]; ok {
				ctxWithEnv := SetEnvInContext(ctx, env)
				return builtin.Fn(ctxWithEnv, nil, dict)
			}
		case "get":
			if len(args) < 1 || len(args) > 2 {
				return errors.NewError("get() takes 1-2 arguments (%d given)", len(args))
			}
			if len(keywords) > 0 {
				return errors.NewError("get() does not accept keyword arguments")
			}
			key := args[0].Inspect()
			if pair, ok := dict.Pairs[key]; ok {
				return pair.Value
			}
			if len(args) == 2 {
				return args[1]
			}
			return NULL
		case "pop":
			if len(args) < 1 || len(args) > 2 {
				return errors.NewError("pop() takes 1-2 arguments (%d given)", len(args))
			}
			if len(keywords) > 0 {
				return errors.NewError("pop() does not accept keyword arguments")
			}
			key := args[0].Inspect()
			if pair, ok := dict.Pairs[key]; ok {
				delete(dict.Pairs, key)
				return pair.Value
			}
			if len(args) == 2 {
				return args[1]
			}
			return errors.NewError("key '%s' not found", key)
		case "update":
			if len(args) > 1 {
				return errors.NewError("update() takes at most 1 argument (%d given)", len(args))
			}
			// Handle kwargs
			for k, v := range keywords {
				dict.Pairs[k] = object.DictPair{Key: &object.String{Value: k}, Value: v}
			}
			// Handle positional argument (another dict or list of pairs)
			if len(args) == 1 {
				switch other := args[0].(type) {
				case *object.Dict:
					for k, v := range other.Pairs {
						dict.Pairs[k] = v
					}
				case *object.List:
					for _, elem := range other.Elements {
						var pair []object.Object
						switch p := elem.(type) {
						case *object.List:
							pair = p.Elements
						case *object.Tuple:
							pair = p.Elements
						default:
							return errors.NewError("dictionary update sequence element must be [key, value] pair")
						}
						if len(pair) != 2 {
							return errors.NewError("dictionary update sequence element must be [key, value] pair")
						}
						dict.Pairs[pair[0].Inspect()] = object.DictPair{Key: pair[0], Value: pair[1]}
					}
				default:
					return errors.NewTypeError("DICT or LIST of pairs", args[0].Type().String())
				}
			}
			return NULL
		case "clear":
			if len(args) != 0 {
				return errors.NewArgumentError(len(args), 0)
			}
			if len(keywords) > 0 {
				return errors.NewError("clear() does not accept keyword arguments")
			}
			dict.Pairs = make(map[string]object.DictPair)
			return NULL
		case "copy":
			if len(args) != 0 {
				return errors.NewArgumentError(len(args), 0)
			}
			if len(keywords) > 0 {
				return errors.NewError("copy() does not accept keyword arguments")
			}
			newPairs := make(map[string]object.DictPair, len(dict.Pairs))
			for k, v := range dict.Pairs {
				newPairs[k] = v
			}
			return &object.Dict{Pairs: newPairs}
		case "setdefault":
			if len(args) < 1 || len(args) > 2 {
				return errors.NewError("setdefault() takes 1-2 arguments (%d given)", len(args))
			}
			if len(keywords) > 0 {
				return errors.NewError("setdefault() does not accept keyword arguments")
			}
			key := args[0].Inspect()
			if pair, ok := dict.Pairs[key]; ok {
				return pair.Value
			}
			var defaultVal object.Object = NULL
			if len(args) == 2 {
				defaultVal = args[1]
			}
			dict.Pairs[key] = object.DictPair{Key: args[0], Value: defaultVal}
			return defaultVal
		case "fromkeys":
			// dict.fromkeys(iterable[, value]) - create new dict with keys from iterable
			if len(args) < 1 || len(args) > 2 {
				return errors.NewError("fromkeys() takes 1-2 arguments (%d given)", len(args))
			}
			if len(keywords) > 0 {
				return errors.NewError("fromkeys() does not accept keyword arguments")
			}
			var defaultVal object.Object = NULL
			if len(args) == 2 {
				defaultVal = args[1]
			}
			newPairs := make(map[string]object.DictPair)
			switch iter := args[0].(type) {
			case *object.List:
				for _, elem := range iter.Elements {
					key := elem.Inspect()
					newPairs[key] = object.DictPair{Key: elem, Value: defaultVal}
				}
			case *object.Tuple:
				for _, elem := range iter.Elements {
					key := elem.Inspect()
					newPairs[key] = object.DictPair{Key: elem, Value: defaultVal}
				}
			case *object.String:
				for _, ch := range iter.Value {
					s := string(ch)
					newPairs[s] = object.DictPair{Key: &object.String{Value: s}, Value: defaultVal}
				}
			default:
				return errors.NewTypeError("iterable (LIST, TUPLE, STRING)", args[0].Type().String())
			}
			return &object.Dict{Pairs: newPairs}
		}

		// Check for non-callable dict values (for accessing dict attributes)
		if pair, ok := dict.Pairs[method]; ok {
			// If it's not a callable, just return the value
			if len(args) == 0 && len(keywords) == 0 {
				return pair.Value
			}
			return errors.NewError("%s: %s is not callable", errors.ErrIdentifierNotFound, method)
		}
		return errors.NewError("%s: method %s not found in library", errors.ErrIdentifierNotFound, method)

	}

	// Handle list methods
	if obj.Type() == object.LIST_OBJ {
		list := obj.(*object.List)
		switch method {
		case "append":
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			if builtin, ok := builtins["append"]; ok {
				ctxWithEnv := SetEnvInContext(ctx, env)
				return builtin.Fn(ctxWithEnv, nil, list, args[0])
			}
		case "extend":
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			if builtin, ok := builtins["extend"]; ok {
				ctxWithEnv := SetEnvInContext(ctx, env)
				return builtin.Fn(ctxWithEnv, nil, list, args[0])
			}
		case "index":
			if len(args) < 1 || len(args) > 3 {
				return errors.NewError("index() takes 1-3 arguments (%d given)", len(args))
			}
			value := args[0]
			start := 0
			end := len(list.Elements)
			if len(args) >= 2 {
				if s, ok := args[1].(*object.Integer); ok {
					start = int(s.Value)
					if start < 0 {
						start = len(list.Elements) + start
						if start < 0 {
							start = 0
						}
					}
				} else {
					return errors.NewTypeError("INTEGER", args[1].Type().String())
				}
			}
			if len(args) == 3 {
				if e, ok := args[2].(*object.Integer); ok {
					end = int(e.Value)
					if end < 0 {
						end = len(list.Elements) + end
					}
				} else {
					return errors.NewTypeError("INTEGER", args[2].Type().String())
				}
			}
			if start > len(list.Elements) {
				start = len(list.Elements)
			}
			if end > len(list.Elements) {
				end = len(list.Elements)
			}
			for i := start; i < end; i++ {
				if objectsEqual(list.Elements[i], value) {
					return object.NewInteger(int64(i))
				}
			}
			return errors.NewError("value not in list")
		case "count":
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			value := args[0]
			count := int64(0)
			for _, elem := range list.Elements {
				if objectsEqual(elem, value) {
					count++
				}
			}
			return object.NewInteger(count)
		case "pop":
			if len(args) > 1 {
				return errors.NewError("pop() takes at most 1 argument (%d given)", len(args))
			}
			if len(list.Elements) == 0 {
				return errors.NewError("pop from empty list")
			}
			idx := len(list.Elements) - 1
			if len(args) == 1 {
				if i, ok := args[0].(*object.Integer); ok {
					idx = int(i.Value)
					if idx < 0 {
						idx = len(list.Elements) + idx
					}
					if idx < 0 || idx >= len(list.Elements) {
						return errors.NewError("pop index out of range")
					}
				} else {
					return errors.NewTypeError("INTEGER", args[0].Type().String())
				}
			}
			result := list.Elements[idx]
			list.Elements = append(list.Elements[:idx], list.Elements[idx+1:]...)
			return result
		case "insert":
			if len(args) != 2 {
				return errors.NewArgumentError(len(args), 2)
			}
			if idx, ok := args[0].(*object.Integer); ok {
				i := int(idx.Value)
				if i < 0 {
					i = len(list.Elements) + i + 1
					if i < 0 {
						i = 0
					}
				}
				if i > len(list.Elements) {
					i = len(list.Elements)
				}
				list.Elements = append(list.Elements[:i], append([]object.Object{args[1]}, list.Elements[i:]...)...)
				return NULL
			}
			return errors.NewTypeError("INTEGER", args[0].Type().String())
		case "remove":
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			value := args[0]
			for i, elem := range list.Elements {
				if objectsEqual(elem, value) {
					list.Elements = append(list.Elements[:i], list.Elements[i+1:]...)
					return NULL
				}
			}
			return errors.NewError("value not in list")
		case "clear":
			if len(args) != 0 {
				return errors.NewArgumentError(len(args), 0)
			}
			list.Elements = []object.Object{}
			return NULL
		case "copy":
			if len(args) != 0 {
				return errors.NewArgumentError(len(args), 0)
			}
			elements := make([]object.Object, len(list.Elements))
			copy(elements, list.Elements)
			return &object.List{Elements: elements}
		case "reverse":
			if len(args) != 0 {
				return errors.NewArgumentError(len(args), 0)
			}
			for i, j := 0, len(list.Elements)-1; i < j; i, j = i+1, j-1 {
				list.Elements[i], list.Elements[j] = list.Elements[j], list.Elements[i]
			}
			return NULL
		case "sort":
			if len(args) != 0 {
				return errors.NewArgumentError(len(args), 0)
			}
			// Check for key and reverse kwargs
			var keyFunc object.Object
			reverse := false
			if keywords != nil {
				if kf, ok := keywords["key"]; ok {
					keyFunc = kf
				}
				if rev, ok := keywords["reverse"]; ok {
					if b, ok := rev.(*object.Boolean); ok {
						reverse = b.Value
					}
				}
			}
			// Sort in place using Go's efficient sort (O(n log n))
			n := len(list.Elements)
			if n > 1 {
				// Pre-compute keys if key function is provided
				var keys []object.Object
				if keyFunc != nil {
					keys = make([]object.Object, n)
					for i, elem := range list.Elements {
						key := applyFunctionWithContext(ctx, keyFunc, []object.Object{elem}, nil, env)
						if isError(key) {
							return key
						}
						keys[i] = key
					}
				}
				// Create index array to track original positions
				indices := make([]int, n)
				for i := range indices {
					indices[i] = i
				}
				// Sort indices based on element/key values
				sort.Slice(indices, func(i, j int) bool {
					var left, right object.Object
					if keys != nil {
						left, right = keys[indices[i]], keys[indices[j]]
					} else {
						left, right = list.Elements[indices[i]], list.Elements[indices[j]]
					}
					cmp := compareObjects(left, right)
					if reverse {
						return cmp > 0
					}
					return cmp < 0
				})
				// Reorder elements according to sorted indices
				newElements := make([]object.Object, n)
				for i, idx := range indices {
					newElements[i] = list.Elements[idx]
				}
				copy(list.Elements, newElements)
			}
			return NULL
		default:
			return errors.NewError("%s: list method %s not found", errors.ErrIdentifierNotFound, method)
		}
	}

	// Handle Regex method calls
	if obj.Type() == object.REGEX_OBJ {
		regex := obj.(*object.Regex)
		if builtin, ok := regexBuiltins[method]; ok {
			ctxWithEnv := SetEnvInContext(ctx, env)
			// Prepend the regex object to args
			allArgs := make([]object.Object, len(args)+1)
			allArgs[0] = regex
			copy(allArgs[1:], args)
			return builtin.Fn(ctxWithEnv, keywords, allArgs...)
		}
		return errors.NewError("regex has no method %s", method)
	}

	if obj.Type() != object.STRING_OBJ {
		return errors.NewTypeError("STRING", obj.Type().String())
	}

	str := obj.(*object.String)

	switch method {
	case "upper":
		if len(args) != 0 {
			return errors.NewArgumentError(len(args), 0)
		}
		if builtin, ok := builtins["upper"]; ok {
			ctxWithEnv := SetEnvInContext(ctx, env)
			return builtin.Fn(ctxWithEnv, nil, str)
		}
	case "lower":
		if len(args) != 0 {
			return errors.NewArgumentError(len(args), 0)
		}
		if builtin, ok := builtins["lower"]; ok {
			ctxWithEnv := SetEnvInContext(ctx, env)
			return builtin.Fn(ctxWithEnv, nil, str)
		}
	case "split":
		if len(args) > 1 {
			return errors.NewArgumentError(len(args), 1)
		}
		// If no argument, split on whitespace
		if len(args) == 0 {
			parts := strings.Fields(str.Value)
			elements := make([]object.Object, len(parts))
			for i, part := range parts {
				elements[i] = &object.String{Value: part}
			}
			return &object.List{Elements: elements}
		}
		// With separator argument
		if builtin, ok := builtins["split"]; ok {
			ctxWithEnv := SetEnvInContext(ctx, env)
			return builtin.Fn(ctxWithEnv, nil, str, args[0])
		}
	case "replace":
		if len(args) != 2 {
			return errors.NewArgumentError(len(args), 2)
		}
		if builtin, ok := builtins["replace"]; ok {
			ctxWithEnv := SetEnvInContext(ctx, env)
			return builtin.Fn(ctxWithEnv, nil, str, args[0], args[1])
		}
	case "join":
		if len(args) != 1 {
			return errors.NewArgumentError(len(args), 1)
		}
		if builtin, ok := builtins["join"]; ok {
			ctxWithEnv := SetEnvInContext(ctx, env)
			// join builtin expects (list, separator), but method is separator.join(list)
			return builtin.Fn(ctxWithEnv, nil, args[0], str)
		}
	case "capitalize":
		if len(args) != 0 {
			return errors.NewArgumentError(len(args), 0)
		}
		if builtin, ok := builtins["capitalize"]; ok {
			ctxWithEnv := SetEnvInContext(ctx, env)
			return builtin.Fn(ctxWithEnv, nil, str)
		}
	case "title":
		if len(args) != 0 {
			return errors.NewArgumentError(len(args), 0)
		}
		if builtin, ok := builtins["title"]; ok {
			ctxWithEnv := SetEnvInContext(ctx, env)
			return builtin.Fn(ctxWithEnv, nil, str)
		}
	case "strip":
		if len(args) != 0 {
			return errors.NewArgumentError(len(args), 0)
		}
		if builtin, ok := builtins["strip"]; ok {
			ctxWithEnv := SetEnvInContext(ctx, env)
			return builtin.Fn(ctxWithEnv, nil, str)
		}
	case "lstrip":
		if len(args) != 0 {
			return errors.NewArgumentError(len(args), 0)
		}
		if builtin, ok := builtins["lstrip"]; ok {
			ctxWithEnv := SetEnvInContext(ctx, env)
			return builtin.Fn(ctxWithEnv, nil, str)
		}
	case "rstrip":
		if len(args) != 0 {
			return errors.NewArgumentError(len(args), 0)
		}
		if builtin, ok := builtins["rstrip"]; ok {
			ctxWithEnv := SetEnvInContext(ctx, env)
			return builtin.Fn(ctxWithEnv, nil, str)
		}
	case "startswith":
		if len(args) != 1 {
			return errors.NewArgumentError(len(args), 1)
		}
		if builtin, ok := builtins["startswith"]; ok {
			ctxWithEnv := SetEnvInContext(ctx, env)
			return builtin.Fn(ctxWithEnv, nil, str, args[0])
		}
	case "endswith":
		if len(args) != 1 {
			return errors.NewArgumentError(len(args), 1)
		}
		if builtin, ok := builtins["endswith"]; ok {
			ctxWithEnv := SetEnvInContext(ctx, env)
			return builtin.Fn(ctxWithEnv, nil, str, args[0])
		}
	case "find":
		if len(args) < 1 || len(args) > 3 {
			return errors.NewError("find() takes 1-3 arguments (%d given)", len(args))
		}
		if substr, ok := args[0].(*object.String); ok {
			start := 0
			end := len(str.Value)
			if len(args) >= 2 {
				if s, ok := args[1].(*object.Integer); ok {
					start = int(s.Value)
					if start < 0 {
						start = len(str.Value) + start
						if start < 0 {
							start = 0
						}
					}
				} else {
					return errors.NewTypeError("INTEGER", args[1].Type().String())
				}
			}
			if len(args) == 3 {
				if e, ok := args[2].(*object.Integer); ok {
					end = int(e.Value)
					if end < 0 {
						end = len(str.Value) + end
					}
				} else {
					return errors.NewTypeError("INTEGER", args[2].Type().String())
				}
			}
			if start > len(str.Value) {
				start = len(str.Value)
			}
			if end > len(str.Value) {
				end = len(str.Value)
			}
			if start > end {
				return object.NewInteger(-1)
			}
			searchStr := str.Value[start:end]
			idx := strings.Index(searchStr, substr.Value)
			if idx == -1 {
				return object.NewInteger(-1)
			}
			return object.NewInteger(int64(start + idx))
		}
		return errors.NewTypeError("STRING", args[0].Type().String())
	case "rfind":
		if len(args) < 1 || len(args) > 3 {
			return errors.NewError("rfind() takes 1-3 arguments (%d given)", len(args))
		}
		if substr, ok := args[0].(*object.String); ok {
			start := 0
			end := len(str.Value)
			if len(args) >= 2 {
				if s, ok := args[1].(*object.Integer); ok {
					start = int(s.Value)
					if start < 0 {
						start = len(str.Value) + start
						if start < 0 {
							start = 0
						}
					}
				} else {
					return errors.NewTypeError("INTEGER", args[1].Type().String())
				}
			}
			if len(args) == 3 {
				if e, ok := args[2].(*object.Integer); ok {
					end = int(e.Value)
					if end < 0 {
						end = len(str.Value) + end
					}
				} else {
					return errors.NewTypeError("INTEGER", args[2].Type().String())
				}
			}
			if start > len(str.Value) {
				start = len(str.Value)
			}
			if end > len(str.Value) {
				end = len(str.Value)
			}
			if start > end {
				return object.NewInteger(-1)
			}
			searchStr := str.Value[start:end]
			idx := strings.LastIndex(searchStr, substr.Value)
			if idx == -1 {
				return object.NewInteger(-1)
			}
			return object.NewInteger(int64(start + idx))
		}
		return errors.NewTypeError("STRING", args[0].Type().String())
	case "rindex":
		if len(args) < 1 || len(args) > 3 {
			return errors.NewError("rindex() takes 1-3 arguments (%d given)", len(args))
		}
		if substr, ok := args[0].(*object.String); ok {
			start := 0
			end := len(str.Value)
			if len(args) >= 2 {
				if s, ok := args[1].(*object.Integer); ok {
					start = int(s.Value)
					if start < 0 {
						start = len(str.Value) + start
						if start < 0 {
							start = 0
						}
					}
				} else {
					return errors.NewTypeError("INTEGER", args[1].Type().String())
				}
			}
			if len(args) == 3 {
				if e, ok := args[2].(*object.Integer); ok {
					end = int(e.Value)
					if end < 0 {
						end = len(str.Value) + end
					}
				} else {
					return errors.NewTypeError("INTEGER", args[2].Type().String())
				}
			}
			if start > len(str.Value) {
				start = len(str.Value)
			}
			if end > len(str.Value) {
				end = len(str.Value)
			}
			if start > end {
				return errors.NewError("substring not found")
			}
			searchStr := str.Value[start:end]
			idx := strings.LastIndex(searchStr, substr.Value)
			if idx == -1 {
				return errors.NewError("substring not found")
			}
			return object.NewInteger(int64(start + idx))
		}
		return errors.NewTypeError("STRING", args[0].Type().String())
	case "index":
		if len(args) < 1 || len(args) > 3 {
			return errors.NewError("index() takes 1-3 arguments (%d given)", len(args))
		}
		if substr, ok := args[0].(*object.String); ok {
			start := 0
			end := len(str.Value)
			if len(args) >= 2 {
				if s, ok := args[1].(*object.Integer); ok {
					start = int(s.Value)
					if start < 0 {
						start = len(str.Value) + start
						if start < 0 {
							start = 0
						}
					}
				} else {
					return errors.NewTypeError("INTEGER", args[1].Type().String())
				}
			}
			if len(args) == 3 {
				if e, ok := args[2].(*object.Integer); ok {
					end = int(e.Value)
					if end < 0 {
						end = len(str.Value) + end
					}
				} else {
					return errors.NewTypeError("INTEGER", args[2].Type().String())
				}
			}
			if start > len(str.Value) {
				start = len(str.Value)
			}
			if end > len(str.Value) {
				end = len(str.Value)
			}
			if start > end {
				return errors.NewError("substring not found")
			}
			searchStr := str.Value[start:end]
			idx := strings.Index(searchStr, substr.Value)
			if idx == -1 {
				return errors.NewError("substring not found")
			}
			return object.NewInteger(int64(start + idx))
		}
		return errors.NewTypeError("STRING", args[0].Type().String())
	case "count":
		if len(args) < 1 || len(args) > 3 {
			return errors.NewError("count() takes 1-3 arguments (%d given)", len(args))
		}
		if substr, ok := args[0].(*object.String); ok {
			start := 0
			end := len(str.Value)
			if len(args) >= 2 {
				if s, ok := args[1].(*object.Integer); ok {
					start = int(s.Value)
					if start < 0 {
						start = len(str.Value) + start
						if start < 0 {
							start = 0
						}
					}
				} else {
					return errors.NewTypeError("INTEGER", args[1].Type().String())
				}
			}
			if len(args) == 3 {
				if e, ok := args[2].(*object.Integer); ok {
					end = int(e.Value)
					if end < 0 {
						end = len(str.Value) + end
					}
				} else {
					return errors.NewTypeError("INTEGER", args[2].Type().String())
				}
			}
			if start > len(str.Value) {
				start = len(str.Value)
			}
			if end > len(str.Value) {
				end = len(str.Value)
			}
			if start > end {
				return object.NewInteger(0)
			}
			searchStr := str.Value[start:end]
			return object.NewInteger(int64(strings.Count(searchStr, substr.Value)))
		}
		return errors.NewTypeError("STRING", args[0].Type().String())
	case "format":
		// Simple positional formatting: "{} {}".format("hello", "world")
		result := str.Value
		for i, arg := range args {
			placeholder := fmt.Sprintf("{%d}", i)
			result = strings.Replace(result, placeholder, arg.Inspect(), 1)
			// Also support {} for positional
			result = strings.Replace(result, "{}", arg.Inspect(), 1)
		}
		return &object.String{Value: result}
	case "isdigit":
		if len(args) != 0 {
			return errors.NewArgumentError(len(args), 0)
		}
		if len(str.Value) == 0 {
			return FALSE
		}
		for _, ch := range str.Value {
			if ch < '0' || ch > '9' {
				return FALSE
			}
		}
		return TRUE
	case "isalpha":
		if len(args) != 0 {
			return errors.NewArgumentError(len(args), 0)
		}
		if len(str.Value) == 0 {
			return FALSE
		}
		for _, ch := range str.Value {
			if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')) {
				return FALSE
			}
		}
		return TRUE
	case "isalnum":
		if len(args) != 0 {
			return errors.NewArgumentError(len(args), 0)
		}
		if len(str.Value) == 0 {
			return FALSE
		}
		for _, ch := range str.Value {
			if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9')) {
				return FALSE
			}
		}
		return TRUE
	case "isspace":
		if len(args) != 0 {
			return errors.NewArgumentError(len(args), 0)
		}
		if len(str.Value) == 0 {
			return FALSE
		}
		for _, ch := range str.Value {
			if ch != ' ' && ch != '\t' && ch != '\n' && ch != '\r' && ch != '\v' && ch != '\f' {
				return FALSE
			}
		}
		return TRUE
	case "isupper":
		if len(args) != 0 {
			return errors.NewArgumentError(len(args), 0)
		}
		hasUpper := false
		for _, ch := range str.Value {
			if ch >= 'a' && ch <= 'z' {
				return FALSE
			}
			if ch >= 'A' && ch <= 'Z' {
				hasUpper = true
			}
		}
		if hasUpper {
			return TRUE
		}
		return FALSE
	case "islower":
		if len(args) != 0 {
			return errors.NewArgumentError(len(args), 0)
		}
		hasLower := false
		for _, ch := range str.Value {
			if ch >= 'A' && ch <= 'Z' {
				return FALSE
			}
			if ch >= 'a' && ch <= 'z' {
				hasLower = true
			}
		}
		if hasLower {
			return TRUE
		}
		return FALSE
	case "zfill":
		if len(args) != 1 {
			return errors.NewArgumentError(len(args), 1)
		}
		if width, ok := args[0].(*object.Integer); ok {
			w := int(width.Value)
			if w <= len(str.Value) {
				return str
			}
			// Handle negative sign
			if len(str.Value) > 0 && (str.Value[0] == '-' || str.Value[0] == '+') {
				return &object.String{Value: string(str.Value[0]) + strings.Repeat("0", w-len(str.Value)) + str.Value[1:]}
			}
			return &object.String{Value: strings.Repeat("0", w-len(str.Value)) + str.Value}
		}
		return errors.NewTypeError("INTEGER", args[0].Type().String())
	case "center":
		if len(args) < 1 || len(args) > 2 {
			return errors.NewError("center() takes 1-2 arguments (%d given)", len(args))
		}
		if width, ok := args[0].(*object.Integer); ok {
			w := int(width.Value)
			if w <= len(str.Value) {
				return str
			}
			fillChar := " "
			if len(args) == 2 {
				if fill, ok := args[1].(*object.String); ok {
					if len(fill.Value) != 1 {
						return errors.NewError("fill character must be exactly one character")
					}
					fillChar = fill.Value
				} else {
					return errors.NewTypeError("STRING", args[1].Type().String())
				}
			}
			padding := w - len(str.Value)
			leftPad := padding / 2
			rightPad := padding - leftPad
			return &object.String{Value: strings.Repeat(fillChar, leftPad) + str.Value + strings.Repeat(fillChar, rightPad)}
		}
		return errors.NewTypeError("INTEGER", args[0].Type().String())
	case "ljust":
		if len(args) < 1 || len(args) > 2 {
			return errors.NewError("ljust() takes 1-2 arguments (%d given)", len(args))
		}
		if width, ok := args[0].(*object.Integer); ok {
			w := int(width.Value)
			if w <= len(str.Value) {
				return str
			}
			fillChar := " "
			if len(args) == 2 {
				if fill, ok := args[1].(*object.String); ok {
					if len(fill.Value) != 1 {
						return errors.NewError("fill character must be exactly one character")
					}
					fillChar = fill.Value
				} else {
					return errors.NewTypeError("STRING", args[1].Type().String())
				}
			}
			return &object.String{Value: str.Value + strings.Repeat(fillChar, w-len(str.Value))}
		}
		return errors.NewTypeError("INTEGER", args[0].Type().String())
	case "rjust":
		if len(args) < 1 || len(args) > 2 {
			return errors.NewError("rjust() takes 1-2 arguments (%d given)", len(args))
		}
		if width, ok := args[0].(*object.Integer); ok {
			w := int(width.Value)
			if w <= len(str.Value) {
				return str
			}
			fillChar := " "
			if len(args) == 2 {
				if fill, ok := args[1].(*object.String); ok {
					if len(fill.Value) != 1 {
						return errors.NewError("fill character must be exactly one character")
					}
					fillChar = fill.Value
				} else {
					return errors.NewTypeError("STRING", args[1].Type().String())
				}
			}
			return &object.String{Value: strings.Repeat(fillChar, w-len(str.Value)) + str.Value}
		}
		return errors.NewTypeError("INTEGER", args[0].Type().String())
	case "splitlines":
		keepends := false
		if len(args) > 1 {
			return errors.NewError("splitlines() takes at most 1 argument (%d given)", len(args))
		}
		if len(args) == 1 {
			if b, ok := args[0].(*object.Boolean); ok {
				keepends = b.Value
			} else {
				return errors.NewTypeError("BOOLEAN", args[0].Type().String())
			}
		}
		lines := []object.Object{}
		text := str.Value
		start := 0
		for i := 0; i < len(text); i++ {
			if text[i] == '\n' {
				if keepends {
					lines = append(lines, &object.String{Value: text[start : i+1]})
				} else {
					lines = append(lines, &object.String{Value: text[start:i]})
				}
				start = i + 1
			} else if text[i] == '\r' {
				end := i
				if i+1 < len(text) && text[i+1] == '\n' {
					i++
				}
				if keepends {
					lines = append(lines, &object.String{Value: text[start : i+1]})
				} else {
					lines = append(lines, &object.String{Value: text[start:end]})
				}
				start = i + 1
			}
		}
		if start < len(text) {
			lines = append(lines, &object.String{Value: text[start:]})
		}
		return &object.List{Elements: lines}
	case "swapcase":
		if len(args) != 0 {
			return errors.NewError("swapcase() takes no arguments (%d given)", len(args))
		}
		result := make([]rune, len(str.Value))
		for i, r := range str.Value {
			if r >= 'A' && r <= 'Z' {
				result[i] = r + 32
			} else if r >= 'a' && r <= 'z' {
				result[i] = r - 32
			} else {
				result[i] = r
			}
		}
		return &object.String{Value: string(result)}
	case "partition":
		if len(args) != 1 {
			return errors.NewArgumentError(len(args), 1)
		}
		sep, ok := args[0].(*object.String)
		if !ok {
			return errors.NewTypeError("STRING", args[0].Type().String())
		}
		idx := strings.Index(str.Value, sep.Value)
		if idx < 0 {
			return &object.Tuple{Elements: []object.Object{
				str,
				&object.String{Value: ""},
				&object.String{Value: ""},
			}}
		}
		return &object.Tuple{Elements: []object.Object{
			&object.String{Value: str.Value[:idx]},
			sep,
			&object.String{Value: str.Value[idx+len(sep.Value):]},
		}}
	case "rpartition":
		if len(args) != 1 {
			return errors.NewArgumentError(len(args), 1)
		}
		sep, ok := args[0].(*object.String)
		if !ok {
			return errors.NewTypeError("STRING", args[0].Type().String())
		}
		idx := strings.LastIndex(str.Value, sep.Value)
		if idx < 0 {
			return &object.Tuple{Elements: []object.Object{
				&object.String{Value: ""},
				&object.String{Value: ""},
				str,
			}}
		}
		return &object.Tuple{Elements: []object.Object{
			&object.String{Value: str.Value[:idx]},
			sep,
			&object.String{Value: str.Value[idx+len(sep.Value):]},
		}}
	case "removeprefix":
		if len(args) != 1 {
			return errors.NewArgumentError(len(args), 1)
		}
		prefix, ok := args[0].(*object.String)
		if !ok {
			return errors.NewTypeError("STRING", args[0].Type().String())
		}
		if strings.HasPrefix(str.Value, prefix.Value) {
			return &object.String{Value: str.Value[len(prefix.Value):]}
		}
		return str
	case "removesuffix":
		if len(args) != 1 {
			return errors.NewArgumentError(len(args), 1)
		}
		suffix, ok := args[0].(*object.String)
		if !ok {
			return errors.NewTypeError("STRING", args[0].Type().String())
		}
		if strings.HasSuffix(str.Value, suffix.Value) {
			return &object.String{Value: str.Value[:len(str.Value)-len(suffix.Value)]}
		}
		return str
	case "encode":
		if len(args) > 1 {
			return errors.NewError("encode() takes at most 1 argument (%d given)", len(args))
		}
		// In Scriptling, encode just returns a list of byte values
		// as we don't have a bytes type
		bytes := []object.Object{}
		for _, b := range []byte(str.Value) {
			bytes = append(bytes, object.NewInteger(int64(b)))
		}
		return &object.List{Elements: bytes}
	case "expandtabs":
		tabsize := 8
		if len(args) > 1 {
			return errors.NewError("expandtabs() takes at most 1 argument (%d given)", len(args))
		}
		if len(args) == 1 {
			if ts, ok := args[0].(*object.Integer); ok {
				tabsize = int(ts.Value)
			} else {
				return errors.NewTypeError("INTEGER", args[0].Type().String())
			}
		}
		var result strings.Builder
		col := 0
		for _, ch := range str.Value {
			if ch == '\t' {
				spaces := tabsize - (col % tabsize)
				result.WriteString(strings.Repeat(" ", spaces))
				col += spaces
			} else if ch == '\n' || ch == '\r' {
				result.WriteRune(ch)
				col = 0
			} else {
				result.WriteRune(ch)
				col++
			}
		}
		return &object.String{Value: result.String()}
	case "casefold":
		if len(args) != 0 {
			return errors.NewArgumentError(len(args), 0)
		}
		// casefold is more aggressive than lower() for Unicode
		// For ASCII, it's equivalent to lower()
		return &object.String{Value: strings.ToLower(str.Value)}
	case "maketrans":
		if len(args) < 1 || len(args) > 3 {
			return errors.NewError("maketrans() takes 1, 2, or 3 arguments (%d given)", len(args))
		}
		transMap := &object.Dict{Pairs: make(map[string]object.DictPair)}
		if len(args) == 1 {
			// Single argument: must be a dict
			if d, ok := args[0].(*object.Dict); ok {
				for k, v := range d.Pairs {
					transMap.Pairs[k] = v
				}
				return transMap
			}
			return errors.NewTypeError("DICT", args[0].Type().String())
		}
		// Two arguments: from and to strings
		from, okFrom := args[0].(*object.String)
		to, okTo := args[1].(*object.String)
		if !okFrom || !okTo {
			return errors.NewError("maketrans() arguments must be strings")
		}
		fromRunes := []rune(from.Value)
		toRunes := []rune(to.Value)
		if len(fromRunes) != len(toRunes) {
			return errors.NewError("maketrans() arguments must have equal length")
		}
		for i, ch := range fromRunes {
			key := string(ch)
			transMap.Pairs[key] = object.DictPair{
				Key:   &object.String{Value: key},
				Value: &object.String{Value: string(toRunes[i])},
			}
		}
		// Third argument: characters to delete
		if len(args) == 3 {
			if del, ok := args[2].(*object.String); ok {
				for _, ch := range del.Value {
					key := string(ch)
					transMap.Pairs[key] = object.DictPair{
						Key:   &object.String{Value: key},
						Value: NULL,
					}
				}
			} else {
				return errors.NewTypeError("STRING", args[2].Type().String())
			}
		}
		return transMap
	case "translate":
		if len(args) != 1 {
			return errors.NewArgumentError(len(args), 1)
		}
		transMap, ok := args[0].(*object.Dict)
		if !ok {
			return errors.NewTypeError("DICT", args[0].Type().String())
		}
		var result strings.Builder
		for _, ch := range str.Value {
			key := string(ch)
			if pair, exists := transMap.Pairs[key]; exists {
				if pair.Value == NULL || pair.Value.Type() == object.NULL_OBJ {
					// Delete character
					continue
				}
				if s, ok := pair.Value.(*object.String); ok {
					result.WriteString(s.Value)
				} else {
					result.WriteRune(ch)
				}
			} else {
				result.WriteRune(ch)
			}
		}
		return &object.String{Value: result.String()}
	case "isnumeric":
		// Returns True if all characters are numeric (0-9, superscripts, fractions, etc.)
		// For simplicity, we check for Unicode numeric characters
		if len(args) != 0 {
			return errors.NewArgumentError(len(args), 0)
		}
		if len(str.Value) == 0 {
			return FALSE
		}
		for _, ch := range str.Value {
			// Check if character is in Unicode numeric categories
			if !unicode.IsNumber(ch) {
				return FALSE
			}
		}
		return TRUE
	case "isdecimal":
		// Returns True if all characters are decimal digits (0-9)
		if len(args) != 0 {
			return errors.NewArgumentError(len(args), 0)
		}
		if len(str.Value) == 0 {
			return FALSE
		}
		for _, ch := range str.Value {
			if ch < '0' || ch > '9' {
				return FALSE
			}
		}
		return TRUE
	case "istitle":
		// Returns True if string is titlecased
		if len(args) != 0 {
			return errors.NewArgumentError(len(args), 0)
		}
		if len(str.Value) == 0 {
			return FALSE
		}
		// Title case: first char of each word is uppercase, rest are lowercase
		prevCased := false
		hasCased := false
		for _, ch := range str.Value {
			isUpper := ch >= 'A' && ch <= 'Z'
			isLower := ch >= 'a' && ch <= 'z'
			isCased := isUpper || isLower
			if isCased {
				hasCased = true
				if prevCased {
					// Previous char was cased, this one should be lowercase
					if !isLower {
						return FALSE
					}
				} else {
					// Previous char was not cased, this one should be uppercase
					if !isUpper {
						return FALSE
					}
				}
			}
			prevCased = isCased
		}
		if hasCased {
			return TRUE
		}
		return FALSE
	case "isidentifier":
		// Returns True if string is a valid identifier
		if len(args) != 0 {
			return errors.NewArgumentError(len(args), 0)
		}
		if len(str.Value) == 0 {
			return FALSE
		}
		for i, ch := range str.Value {
			if i == 0 {
				// First character must be letter or underscore
				if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_') {
					return FALSE
				}
			} else {
				// Subsequent characters can also be digits
				if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_') {
					return FALSE
				}
			}
		}
		return TRUE
	case "isprintable":
		// Returns True if all characters are printable
		if len(args) != 0 {
			return errors.NewArgumentError(len(args), 0)
		}
		// Empty string is considered printable
		for _, ch := range str.Value {
			if !unicode.IsPrint(ch) && ch != ' ' {
				return FALSE
			}
		}
		return TRUE
	default:
		return errors.NewError("%s: %s", errors.ErrIdentifierNotFound, method)
	}
	return errors.NewError("%s: %s", errors.ErrIdentifierNotFound, method)
}

func evalListComprehension(ctx context.Context, lc *ast.ListComprehension, env *object.Environment) object.Object {
	iterable := evalWithContext(ctx, lc.Iterable, env)
	if isError(iterable) {
		return iterable
	}

	result := []object.Object{}

	// Create new scope for comprehension variable
	compEnv := object.NewEnclosedEnvironment(env)

	switch iter := iterable.(type) {
	case *object.List:
		for _, element := range iter.Elements {
			compEnv.Set(lc.Variable.Value, element)

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
	case *object.String:
		for _, char := range iter.Value {
			compEnv.Set(lc.Variable.Value, &object.String{Value: string(char)})

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
	default:
		return errors.NewTypeError("iterable", iterable.Type().String())
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
			result += exprResult.Inspect()
		}
	}

	return &object.String{Value: result}
}
