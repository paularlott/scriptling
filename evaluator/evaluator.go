package evaluator

import (
	"context"
	"math"
	"strings"

	"github.com/paularlott/scriptling/ast"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
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
		env.Set(node.Name.Value, val)
		return NULL
	case *ast.AugmentedAssignStatement:
		return evalAugmentedAssignStatementWithContext(ctx, node, env)
	case *ast.MultipleAssignStatement:
		return evalMultipleAssignStatementWithContext(ctx, node, env)
	case *ast.Identifier:
		return evalIdentifier(node, env)
	case *ast.FunctionStatement:
		fn := &object.Function{
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
	}

	// Type switch is faster than Type() method calls
	switch l := left.(type) {
	case *object.Integer:
		if r, ok := right.(*object.Integer); ok {
			return evalIntegerInfixExpression(operator, l.Value, r.Value)
		}
		return evalFloatInfixExpression(operator, left, right)
	case *object.Float:
		return evalFloatInfixExpression(operator, left, right)
	case *object.String:
		if r, ok := right.(*object.String); ok {
			return evalStringInfixExpression(operator, l.Value, r.Value)
		}
	}

	switch operator {
	case "==":
		return nativeBoolToBooleanObject(left == right)
	case "!=":
		return nativeBoolToBooleanObject(left != right)
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
		if len(keywords) > 0 {
			return errors.NewError("keyword arguments not supported for built-in functions")
		}
		ctxWithEnv := SetEnvInContext(ctx, env)
		return fn.Fn(ctxWithEnv, args...)
	default:
		return errors.NewTypeError("function", fn.Type().String())
	}
}

func extendFunctionEnv(fn *object.Function, args []object.Object, keywords map[string]object.Object) (*object.Environment, object.Object) {
	env := object.NewEnclosedEnvironment(fn.Env)

	// Set provided positional arguments
	numParams := len(fn.Parameters)
	numArgs := len(args)

	for paramIdx := 0; paramIdx < numParams && paramIdx < numArgs; paramIdx++ {
		env.Set(fn.Parameters[paramIdx].Value, args[paramIdx])
	}

	// Check for extra positional arguments
	if numArgs > numParams {
		if fn.Variadic != nil {
			// Collect extra arguments into a list
			variadicArgs := args[numParams:]
			list := &object.List{Elements: variadicArgs}
			env.Set(fn.Variadic.Value, list)
		} else {
			// Calculate min args for error message
			minArgs := numParams
			if fn.DefaultValues != nil {
				minArgs -= len(fn.DefaultValues)
			}
			return nil, errors.NewArgumentError(numArgs, minArgs)
		}
	} else if fn.Variadic != nil {
		// No extra arguments, set variadic to empty list
		env.Set(fn.Variadic.Value, &object.List{Elements: []object.Object{}})
	}

	// Handle keyword arguments if present
	if len(keywords) > 0 {
		setParams := make(map[string]bool, numParams)
		// Mark positional args as set
		for i := 0; i < numParams && i < numArgs; i++ {
			setParams[fn.Parameters[i].Value] = true
		}

		for key, value := range keywords {
			// Check if parameter exists
			paramExists := false
			for _, param := range fn.Parameters {
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

		// Check for missing arguments and apply defaults (when keywords are used)
		for _, param := range fn.Parameters {
			if !setParams[param.Value] {
				// Use default value if available
				if defaultExpr, ok := fn.DefaultValues[param.Value]; ok {
					defaultVal := Eval(defaultExpr, fn.Env)
					env.Set(param.Value, defaultVal)
				} else {
					// Calculate min args for error message
					minArgs := numParams
					if fn.DefaultValues != nil {
						minArgs -= len(fn.DefaultValues)
					}
					return nil, errors.NewArgumentError(numArgs, minArgs)
				}
			}
		}
	} else {
		// No keywords - just check for missing required arguments
		if numArgs < numParams {
			for i := numArgs; i < numParams; i++ {
				param := fn.Parameters[i]
				// Use default value if available
				if defaultExpr, ok := fn.DefaultValues[param.Value]; ok {
					defaultVal := Eval(defaultExpr, fn.Env)
					env.Set(param.Value, defaultVal)
				} else {
					// Calculate min args for error message
					minArgs := numParams
					if fn.DefaultValues != nil {
						minArgs -= len(fn.DefaultValues)
					}
					return nil, errors.NewArgumentError(numArgs, minArgs)
				}
			}
		}
	}

	return env, nil
}

func extendLambdaEnv(fn *object.LambdaFunction, args []object.Object, keywords map[string]object.Object) (*object.Environment, object.Object) {
	env := object.NewEnclosedEnvironment(fn.Env)

	// Set provided positional arguments
	numParams := len(fn.Parameters)
	numArgs := len(args)

	for paramIdx := 0; paramIdx < numParams && paramIdx < numArgs; paramIdx++ {
		env.Set(fn.Parameters[paramIdx].Value, args[paramIdx])
	}

	// Check for extra positional arguments
	if numArgs > numParams {
		if fn.Variadic != nil {
			// Collect extra arguments into a list
			variadicArgs := args[numParams:]
			list := &object.List{Elements: variadicArgs}
			env.Set(fn.Variadic.Value, list)
		} else {
			// Calculate min args for error message
			minArgs := numParams
			if fn.DefaultValues != nil {
				minArgs -= len(fn.DefaultValues)
			}
			return nil, errors.NewArgumentError(numArgs, minArgs)
		}
	} else if fn.Variadic != nil {
		// No extra arguments, set variadic to empty list
		env.Set(fn.Variadic.Value, &object.List{Elements: []object.Object{}})
	}

	// Handle keyword arguments if present
	if len(keywords) > 0 {
		setParams := make(map[string]bool, numParams)
		// Mark positional args as set
		for i := 0; i < numParams && i < numArgs; i++ {
			setParams[fn.Parameters[i].Value] = true
		}

		for key, value := range keywords {
			// Check if parameter exists
			paramExists := false
			for _, param := range fn.Parameters {
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

		// Check for missing arguments and apply defaults (when keywords are used)
		for _, param := range fn.Parameters {
			if !setParams[param.Value] {
				// Use default value if available
				if defaultExpr, ok := fn.DefaultValues[param.Value]; ok {
					defaultVal := Eval(defaultExpr, fn.Env)
					env.Set(param.Value, defaultVal)
				} else {
					// Calculate min args for error message
					minArgs := numParams
					if fn.DefaultValues != nil {
						minArgs -= len(fn.DefaultValues)
					}
					return nil, errors.NewArgumentError(numArgs, minArgs)
				}
			}
		}
	} else {
		// No keywords - just check for missing required arguments
		if numArgs < numParams {
			for i := numArgs; i < numParams; i++ {
				param := fn.Parameters[i]
				// Use default value if available
				if defaultExpr, ok := fn.DefaultValues[param.Value]; ok {
					defaultVal := Eval(defaultExpr, fn.Env)
					env.Set(param.Value, defaultVal)
				} else {
					// Calculate min args for error message
					minArgs := numParams
					if fn.DefaultValues != nil {
						minArgs -= len(fn.DefaultValues)
					}
					return nil, errors.NewArgumentError(numArgs, minArgs)
				}
			}
		}
	}

	return env, nil
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
	max := int64(len(listObject.Elements) - 1)

	if idx < 0 || idx > max {
		return NULL
	}

	return listObject.Elements[idx]
}

func evalTupleIndexExpression(tuple, index object.Object) object.Object {
	tupleObject := tuple.(*object.Tuple)
	idx := index.(*object.Integer).Value
	max := int64(len(tupleObject.Elements) - 1)

	if idx < 0 || idx > max {
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
	max := int64(len(strObject.Value) - 1)

	if idx < 0 || idx > max {
		return NULL
	}

	return &object.String{Value: string(strObject.Value[idx])}
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

func isException(obj object.Object) bool {
	if obj == nil {
		return false
	}
	return obj.Type() == object.EXCEPTION_OBJ
}

func evalForStatementWithContext(ctx context.Context, fs *ast.ForStatement, env *object.Environment) object.Object {
	iterable := evalWithContext(ctx, fs.Iterable, env)
	if isError(iterable) {
		return iterable
	}

	var result object.Object = NULL
	varName := fs.Variable.Value // Cache variable name outside loop

	switch iter := iterable.(type) {
	case *object.List:
		for _, element := range iter.Elements {
			// Check for cancellation in loops
			if err := checkContext(ctx); err != nil {
				return err
			}

			env.Set(varName, element)
			result = evalWithContext(ctx, fs.Body, env)
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
	case *object.Tuple:
		for _, element := range iter.Elements {
			// Check for cancellation in loops
			if err := checkContext(ctx); err != nil {
				return err
			}

			env.Set(varName, element)
			result = evalWithContext(ctx, fs.Body, env)
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
	case *object.String:
		for _, char := range iter.Value {
			// Check for cancellation in loops
			if err := checkContext(ctx); err != nil {
				return err
			}

			env.Set(varName, &object.String{Value: string(char)})
			result = evalWithContext(ctx, fs.Body, env)
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
	default:
		return errors.NewTypeError("iterable", iterable.Type().String())
	}

	return result
}

func evalMethodCallExpression(ctx context.Context, mce *ast.MethodCallExpression, env *object.Environment) object.Object {
	obj := evalWithContext(ctx, mce.Object, env)
	if isError(obj) {
		return obj
	}

	if len(mce.Keywords) > 0 {
		return errors.NewError("keyword arguments not supported for method calls")
	}

	args := evalExpressionsWithContext(ctx, mce.Arguments, env)
	if len(args) == 1 && isError(args[0]) {
		return args[0]
	}

	return callStringMethod(ctx, obj, mce.Method.Value, args, env)
}

func callStringMethod(ctx context.Context, obj object.Object, method string, args []object.Object, env *object.Environment) object.Object {
	// Handle universal methods
	switch method {
	case "type":
		if len(args) != 0 {
			return errors.NewArgumentError(len(args), 0)
		}
		return &object.String{Value: obj.Type().String()}
	}

	// Handle library method calls (dictionaries)
	if obj.Type() == object.DICT_OBJ {
		dict := obj.(*object.Dict)

		// Check for dict instance methods first
		switch method {
		case "keys":
			if len(args) != 0 {
				return errors.NewArgumentError(len(args), 0)
			}
			if builtin, ok := builtins["keys"]; ok {
				ctxWithEnv := SetEnvInContext(ctx, env)
				return builtin.Fn(ctxWithEnv, dict)
			}
		case "values":
			if len(args) != 0 {
				return errors.NewArgumentError(len(args), 0)
			}
			if builtin, ok := builtins["values"]; ok {
				ctxWithEnv := SetEnvInContext(ctx, env)
				return builtin.Fn(ctxWithEnv, dict)
			}
		case "items":
			if len(args) != 0 {
				return errors.NewArgumentError(len(args), 0)
			}
			if builtin, ok := builtins["items"]; ok {
				ctxWithEnv := SetEnvInContext(ctx, env)
				return builtin.Fn(ctxWithEnv, dict)
			}
		}

		// Then check for library methods
		if pair, ok := dict.Pairs[method]; ok {
			switch fn := pair.Value.(type) {
			case *object.Builtin:
				ctxWithEnv := SetEnvInContext(ctx, env)
				return fn.Fn(ctxWithEnv, args...)
			case *object.Function:
				return applyFunctionWithContext(ctx, fn, args, nil, env)
			case *object.LambdaFunction:
				return applyFunctionWithContext(ctx, fn, args, nil, env)
			default:
				// If it's not a callable, just return the value
				if len(args) == 0 {
					return pair.Value
				}
				return errors.NewError("%s: %s is not callable", errors.ErrIdentifierNotFound, method)
			}
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
				return builtin.Fn(ctxWithEnv, list, args[0])
			}
		case "extend":
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			if builtin, ok := builtins["extend"]; ok {
				ctxWithEnv := SetEnvInContext(ctx, env)
				return builtin.Fn(ctxWithEnv, list, args[0])
			}
		default:
			return errors.NewError("%s: list method %s not found", errors.ErrIdentifierNotFound, method)
		}
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
			return builtin.Fn(ctxWithEnv, str)
		}
	case "lower":
		if len(args) != 0 {
			return errors.NewArgumentError(len(args), 0)
		}
		if builtin, ok := builtins["lower"]; ok {
			ctxWithEnv := SetEnvInContext(ctx, env)
			return builtin.Fn(ctxWithEnv, str)
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
			return builtin.Fn(ctxWithEnv, str, args[0])
		}
	case "replace":
		if len(args) != 2 {
			return errors.NewArgumentError(len(args), 2)
		}
		if builtin, ok := builtins["replace"]; ok {
			ctxWithEnv := SetEnvInContext(ctx, env)
			return builtin.Fn(ctxWithEnv, str, args[0], args[1])
		}
	case "join":
		if len(args) != 1 {
			return errors.NewArgumentError(len(args), 1)
		}
		if builtin, ok := builtins["join"]; ok {
			ctxWithEnv := SetEnvInContext(ctx, env)
			// join builtin expects (list, separator), but method is separator.join(list)
			return builtin.Fn(ctxWithEnv, args[0], str)
		}
	case "capitalize":
		if len(args) != 0 {
			return errors.NewArgumentError(len(args), 0)
		}
		if builtin, ok := builtins["capitalize"]; ok {
			ctxWithEnv := SetEnvInContext(ctx, env)
			return builtin.Fn(ctxWithEnv, str)
		}
	case "title":
		if len(args) != 0 {
			return errors.NewArgumentError(len(args), 0)
		}
		if builtin, ok := builtins["title"]; ok {
			ctxWithEnv := SetEnvInContext(ctx, env)
			return builtin.Fn(ctxWithEnv, str)
		}
	case "strip":
		if len(args) != 0 {
			return errors.NewArgumentError(len(args), 0)
		}
		if builtin, ok := builtins["strip"]; ok {
			ctxWithEnv := SetEnvInContext(ctx, env)
			return builtin.Fn(ctxWithEnv, str)
		}
	case "lstrip":
		if len(args) != 0 {
			return errors.NewArgumentError(len(args), 0)
		}
		if builtin, ok := builtins["lstrip"]; ok {
			ctxWithEnv := SetEnvInContext(ctx, env)
			return builtin.Fn(ctxWithEnv, str)
		}
	case "rstrip":
		if len(args) != 0 {
			return errors.NewArgumentError(len(args), 0)
		}
		if builtin, ok := builtins["rstrip"]; ok {
			ctxWithEnv := SetEnvInContext(ctx, env)
			return builtin.Fn(ctxWithEnv, str)
		}
	case "startswith":
		if len(args) != 1 {
			return errors.NewArgumentError(len(args), 1)
		}
		if builtin, ok := builtins["startswith"]; ok {
			ctxWithEnv := SetEnvInContext(ctx, env)
			return builtin.Fn(ctxWithEnv, str, args[0])
		}
	case "endswith":
		if len(args) != 1 {
			return errors.NewArgumentError(len(args), 1)
		}
		if builtin, ok := builtins["endswith"]; ok {
			ctxWithEnv := SetEnvInContext(ctx, env)
			return builtin.Fn(ctxWithEnv, str, args[0])
		}
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
