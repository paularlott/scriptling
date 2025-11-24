package evaluator

import (
	"fmt"
	"github.com/paularlott/scriptling/ast"
	"github.com/paularlott/scriptling/object"
)

var (
	NULL  = &object.Null{}
	TRUE  = &object.Boolean{Value: true}
	FALSE = &object.Boolean{Value: false}
)

func Eval(node ast.Node, env *object.Environment) object.Object {
	switch node := node.(type) {
	case *ast.Program:
		return evalProgram(node, env)
	case *ast.ExpressionStatement:
		return Eval(node.Expression, env)
	case *ast.IntegerLiteral:
		return &object.Integer{Value: node.Value}
	case *ast.FloatLiteral:
		return &object.Float{Value: node.Value}
	case *ast.StringLiteral:
		return &object.String{Value: node.Value}
	case *ast.Boolean:
		return nativeBoolToBooleanObject(node.Value)
	case *ast.None:
		return NULL
	case *ast.PrefixExpression:
		right := Eval(node.Right, env)
		if isError(right) {
			return right
		}
		return evalPrefixExpression(node.Operator, right)
	case *ast.InfixExpression:
		left := Eval(node.Left, env)
		if isError(left) {
			return left
		}
		right := Eval(node.Right, env)
		if isError(right) {
			return right
		}
		return evalInfixExpression(node.Operator, left, right)
	case *ast.BlockStatement:
		return evalBlockStatement(node, env)
	case *ast.IfStatement:
		return evalIfStatement(node, env)
	case *ast.WhileStatement:
		return evalWhileStatement(node, env)
	case *ast.ReturnStatement:
		val := object.Object(NULL)
		if node.ReturnValue != nil {
			val = Eval(node.ReturnValue, env)
			if isError(val) {
				return val
			}
		}
		return &object.ReturnValue{Value: val}
	case *ast.BreakStatement:
		return &object.Break{}
	case *ast.ContinueStatement:
		return &object.Continue{}
	case *ast.PassStatement:
		return NULL
	case *ast.ImportStatement:
		return evalImportStatement(node, env)
	case *ast.AssignStatement:
		val := Eval(node.Value, env)
		if isError(val) {
			return val
		}
		env.Set(node.Name.Value, val)
		return val
	case *ast.AugmentedAssignStatement:
		return evalAugmentedAssignStatement(node, env)
	case *ast.MultipleAssignStatement:
		return evalMultipleAssignStatement(node, env)
	case *ast.Identifier:
		return evalIdentifier(node, env)
	case *ast.FunctionStatement:
		fn := &object.Function{
			Parameters: node.Function.Parameters,
			Body:       node.Function.Body,
			Env:        env,
		}
		env.Set(node.Name.Value, fn)
		return fn
	case *ast.CallExpression:
		function := Eval(node.Function, env)
		if isError(function) {
			return function
		}
		args := evalExpressions(node.Arguments, env)
		if len(args) == 1 && isError(args[0]) {
			return args[0]
		}
		return applyFunction(function, args)
	case *ast.ListLiteral:
		elements := evalExpressions(node.Elements, env)
		if len(elements) == 1 && isError(elements[0]) {
			return elements[0]
		}
		return &object.List{Elements: elements}
	case *ast.DictLiteral:
		return evalDictLiteral(node, env)
	case *ast.IndexExpression:
		left := Eval(node.Left, env)
		if isError(left) {
			return left
		}
		index := Eval(node.Index, env)
		if isError(index) {
			return index
		}
		return evalIndexExpression(left, index)
	case *ast.SliceExpression:
		return evalSliceExpression(node, env)
	case *ast.ForStatement:
		return evalForStatement(node, env)
	case *ast.TryStatement:
		return evalTryStatement(node, env)
	case *ast.RaiseStatement:
		return evalRaiseStatement(node, env)
	case *ast.GlobalStatement:
		return evalGlobalStatement(node, env)
	case *ast.NonlocalStatement:
		return evalNonlocalStatement(node, env)
	case *ast.MethodCallExpression:
		return evalMethodCallExpression(node, env)
	case *ast.ListComprehension:
		return evalListComprehension(node, env)
	}
	return NULL
}

func evalProgram(program *ast.Program, env *object.Environment) object.Object {
	var result object.Object = NULL

	for _, statement := range program.Statements {
		result = Eval(statement, env)

		switch result := result.(type) {
		case *object.ReturnValue:
			return result.Value
		case *Error:
			return result
		}
	}

	return result
}

func evalBlockStatement(block *ast.BlockStatement, env *object.Environment) object.Object {
	var result object.Object = NULL

	for _, statement := range block.Statements {
		result = Eval(statement, env)

		if result != nil {
			rt := result.Type()
			if rt == object.RETURN_OBJ || rt == object.BREAK_OBJ || rt == object.CONTINUE_OBJ || rt == "ERROR" || rt == object.EXCEPTION_OBJ {
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
	default:
		return newError("unknown operator: %s%s", operator, right.Type())
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
		return &object.Integer{Value: -right.Value}
	case *object.Float:
		return &object.Float{Value: -right.Value}
	default:
		return newError("unknown operator: -%s", right.Type())
	}
}

func evalInfixExpression(operator string, left, right object.Object) object.Object {
	// Handle membership operators first
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
	case "and":
		return nativeBoolToBooleanObject(isTruthy(left) && isTruthy(right))
	case "or":
		if isTruthy(left) {
			return TRUE
		}
		return nativeBoolToBooleanObject(isTruthy(right))
	default:
		return newError("type mismatch or unknown operator")
	}
}

func evalIntegerInfixExpression(operator string, leftVal, rightVal int64) object.Object {
	switch operator {
	case "+":
		return &object.Integer{Value: leftVal + rightVal}
	case "-":
		return &object.Integer{Value: leftVal - rightVal}
	case "*":
		return &object.Integer{Value: leftVal * rightVal}
	case "/":
		if rightVal == 0 {
			return newError("division by zero")
		}
		// True division: always return float
		return &object.Float{Value: float64(leftVal) / float64(rightVal)}
	case "%":
		return &object.Integer{Value: leftVal % rightVal}
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
		return newError("unknown operator: INTEGER %s INTEGER", operator)
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
			return newError("division by zero")
		}
		return &object.Float{Value: leftVal / rightVal}
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
		return newError("unknown operator: FLOAT %s FLOAT", operator)
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
		return newError("unknown operator: STRING %s STRING", operator)
	}
}

func evalIfStatement(ie *ast.IfStatement, env *object.Environment) object.Object {
	condition := Eval(ie.Condition, env)
	if isError(condition) {
		return condition
	}

	if isTruthy(condition) {
		return Eval(ie.Consequence, env)
	}

	// Check elif clauses
	for _, elifClause := range ie.ElifClauses {
		condition := Eval(elifClause.Condition, env)
		if isError(condition) {
			return condition
		}
		if isTruthy(condition) {
			return Eval(elifClause.Consequence, env)
		}
	}

	// Check else clause
	if ie.Alternative != nil {
		return Eval(ie.Alternative, env)
	}

	return NULL
}

func evalWhileStatement(ws *ast.WhileStatement, env *object.Environment) object.Object {
	var result object.Object = NULL

	for {
		condition := Eval(ws.Condition, env)
		if isError(condition) {
			return condition
		}

		if !isTruthy(condition) {
			break
		}

		result = Eval(ws.Body, env)
		if isError(result) {
			return result
		}
		if result.Type() == object.RETURN_OBJ {
			return result
		}
		if result.Type() == object.BREAK_OBJ {
			return NULL
		}
		if result.Type() == object.CONTINUE_OBJ {
			continue
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
	return newError("identifier not found: %s", node.Value)
}

func evalExpressions(exps []ast.Expression, env *object.Environment) []object.Object {
	if len(exps) == 0 {
		return nil
	}
	result := make([]object.Object, len(exps))

	for i, e := range exps {
		evaluated := Eval(e, env)
		if isError(evaluated) {
			return []object.Object{evaluated}
		}
		result[i] = evaluated
	}

	return result
}

func applyFunction(fn object.Object, args []object.Object) object.Object {
	switch fn := fn.(type) {
	case *object.Function:
		extendedEnv := extendFunctionEnv(fn, args)
		evaluated := Eval(fn.Body, extendedEnv)
		return unwrapReturnValue(evaluated)
	case *object.Builtin:
		return fn.Fn(args...)
	default:
		return newError("not a function: %s", fn.Type())
	}
}

func extendFunctionEnv(fn *object.Function, args []object.Object) *object.Environment {
	env := object.NewEnclosedEnvironment(fn.Env)

	for paramIdx, param := range fn.Parameters {
		env.Set(param.Value, args[paramIdx])
	}

	return env
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
		return true
	}
}

type Error = object.Error

func newError(format string, a ...interface{}) *Error {
	return &Error{Message: fmt.Sprintf(format, a...)}
}

func isError(obj object.Object) bool {
	if obj != nil {
		return obj.Type() == "ERROR"
	}
	return false
}

func evalDictLiteral(node *ast.DictLiteral, env *object.Environment) object.Object {
	if len(node.Pairs) == 0 {
		return &object.Dict{Pairs: make(map[string]object.DictPair)}
	}
	pairs := make(map[string]object.DictPair, len(node.Pairs))

	for keyNode, valueNode := range node.Pairs {
		key := Eval(keyNode, env)
		if isError(key) {
			return key
		}

		value := Eval(valueNode, env)
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
	case left.Type() == object.DICT_OBJ:
		return evalDictIndexExpression(left, index)
	case left.Type() == object.STRING_OBJ && index.Type() == object.INTEGER_OBJ:
		return evalStringIndexExpression(left, index)
	default:
		return newError("index operator not supported: %s", left.Type())
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

func evalAugmentedAssignStatement(node *ast.AugmentedAssignStatement, env *object.Environment) object.Object {
	currentVal, ok := env.Get(node.Name.Value)
	if !ok {
		return newError("identifier not found: %s", node.Name.Value)
	}

	newVal := Eval(node.Value, env)
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
	default:
		return newError("unknown augmented assignment operator: %s", node.Operator)
	}

	result := evalInfixExpression(operator, currentVal, newVal)
	if isError(result) {
		return result
	}

	env.Set(node.Name.Value, result)
	return result
}

func evalSliceExpression(node *ast.SliceExpression, env *object.Environment) object.Object {
	left := Eval(node.Left, env)
	if isError(left) {
		return left
	}

	var start, end int64
	var hasStart, hasEnd bool

	if node.Start != nil {
		startObj := Eval(node.Start, env)
		if isError(startObj) {
			return startObj
		}
		if startObj.Type() != object.INTEGER_OBJ {
			return newError("slice start must be INTEGER")
		}
		start = startObj.(*object.Integer).Value
		hasStart = true
	}

	if node.End != nil {
		endObj := Eval(node.End, env)
		if isError(endObj) {
			return endObj
		}
		if endObj.Type() != object.INTEGER_OBJ {
			return newError("slice end must be INTEGER")
		}
		end = endObj.(*object.Integer).Value
		hasEnd = true
	}

	switch obj := left.(type) {
	case *object.List:
		length := int64(len(obj.Elements))
		if !hasStart {
			start = 0
		}
		if !hasEnd {
			end = length
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
		return &object.List{Elements: obj.Elements[start:end]}
	case *object.String:
		length := int64(len(obj.Value))
		if !hasStart {
			start = 0
		}
		if !hasEnd {
			end = length
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
		return &object.String{Value: obj.Value[start:end]}
	default:
		return newError("slice operator not supported: %s", left.Type())
	}
}

func evalImportStatement(is *ast.ImportStatement, env *object.Environment) object.Object {
	if importCallback == nil {
		return newError("import not available")
	}
	err := importCallback(is.Name.Value)
	if err != nil {
		return newError("import error: %s", err.Error())
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
		return newError("'in' requires string on left for string container")
	default:
		return newError("'in' operator not supported for %s", right.Type())
	}
}

func evalMultipleAssignStatement(node *ast.MultipleAssignStatement, env *object.Environment) object.Object {
	val := Eval(node.Value, env)
	if isError(val) {
		return val
	}
	
	// Value must be a list
	list, ok := val.(*object.List)
	if !ok {
		return newError("multiple assignment requires list, got %s", val.Type())
	}
	
	// Check length matches
	if len(list.Elements) != len(node.Names) {
		return newError("cannot unpack %d values to %d variables", len(list.Elements), len(node.Names))
	}
	
	// Assign each value
	for i, name := range node.Names {
		env.Set(name.Value, list.Elements[i])
	}
	
	return list
}

func evalTryStatement(ts *ast.TryStatement, env *object.Environment) object.Object {
	// Execute try block
	result := Eval(ts.Body, env)
	
	// Check if exception occurred
	if isException(result) {
		// Execute except block if present
		if ts.Except != nil {
			result = Eval(ts.Except, env)
		}
	}
	
	// Always execute finally block if present
	if ts.Finally != nil {
		Eval(ts.Finally, env)
	}
	
	// Clear exception if it was handled
	if isException(result) && ts.Except != nil {
		return NULL
	}
	
	return result
}

func evalRaiseStatement(rs *ast.RaiseStatement, env *object.Environment) object.Object {
	var message string
	if rs.Message != nil {
		msg := Eval(rs.Message, env)
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
	return obj.Type() == object.EXCEPTION_OBJ || obj.Type() == "ERROR"
}

func evalForStatement(fs *ast.ForStatement, env *object.Environment) object.Object {
	iterable := Eval(fs.Iterable, env)
	if isError(iterable) {
		return iterable
	}

	var result object.Object = NULL

	switch iter := iterable.(type) {
	case *object.List:
		for _, element := range iter.Elements {
			env.Set(fs.Variable.Value, element)
			result = Eval(fs.Body, env)
			if isError(result) {
				return result
			}
			if result.Type() == object.RETURN_OBJ {
				return result
			}
			if result.Type() == object.BREAK_OBJ {
				return NULL
			}
			if result.Type() == object.CONTINUE_OBJ {
				continue
			}
		}
	case *object.String:
		for _, char := range iter.Value {
			env.Set(fs.Variable.Value, &object.String{Value: string(char)})
			result = Eval(fs.Body, env)
			if isError(result) {
				return result
			}
			if result.Type() == object.RETURN_OBJ {
				return result
			}
			if result.Type() == object.BREAK_OBJ {
				return NULL
			}
			if result.Type() == object.CONTINUE_OBJ {
				continue
			}
		}
	default:
		return newError("for loop requires iterable, got %s", iterable.Type())
	}

	return result
}

func evalMethodCallExpression(mce *ast.MethodCallExpression, env *object.Environment) object.Object {
	obj := Eval(mce.Object, env)
	if isError(obj) {
		return obj
	}

	args := evalExpressions(mce.Arguments, env)
	if len(args) == 1 && isError(args[0]) {
		return args[0]
	}

	return callStringMethod(obj, mce.Method.Value, args)
}

func callStringMethod(obj object.Object, method string, args []object.Object) object.Object {
	// Handle library method calls (dictionaries)
	if obj.Type() == object.DICT_OBJ {
		dict := obj.(*object.Dict)
		if pair, ok := dict.Pairs[method]; ok {
			if builtin, ok := pair.Value.(*object.Builtin); ok {
				return builtin.Fn(args...)
			}
		}
		return newError("method %s not found in library", method)
	}
	
	if obj.Type() != object.STRING_OBJ {
		return newError("method %s not supported on %s", method, obj.Type())
	}

	str := obj.(*object.String)

	switch method {
	case "upper":
		if len(args) != 0 {
			return newError("wrong number of arguments. got=%d, want=0", len(args))
		}
		if builtin, ok := builtins["upper"]; ok {
			return builtin.Fn(str)
		}
	case "lower":
		if len(args) != 0 {
			return newError("wrong number of arguments. got=%d, want=0", len(args))
		}
		if builtin, ok := builtins["lower"]; ok {
			return builtin.Fn(str)
		}
	case "split":
		if len(args) != 1 {
			return newError("wrong number of arguments. got=%d, want=1", len(args))
		}
		if builtin, ok := builtins["split"]; ok {
			return builtin.Fn(str, args[0])
		}
	case "replace":
		if len(args) != 2 {
			return newError("wrong number of arguments. got=%d, want=2", len(args))
		}
		if builtin, ok := builtins["replace"]; ok {
			return builtin.Fn(str, args[0], args[1])
		}
	default:
		return newError("unknown method: %s", method)
	}
	return newError("method %s not found", method)
}

func evalListComprehension(lc *ast.ListComprehension, env *object.Environment) object.Object {
	iterable := Eval(lc.Iterable, env)
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
				condition := Eval(lc.Condition, compEnv)
				if isError(condition) {
					return condition
				}
				if !isTruthy(condition) {
					continue
				}
			}
			
			// Evaluate expression
			exprResult := Eval(lc.Expression, compEnv)
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
				condition := Eval(lc.Condition, compEnv)
				if isError(condition) {
					return condition
				}
				if !isTruthy(condition) {
					continue
				}
			}
			
			// Evaluate expression
			exprResult := Eval(lc.Expression, compEnv)
			if isError(exprResult) {
				return exprResult
			}
			result = append(result, exprResult)
		}
	default:
		return newError("list comprehension requires iterable, got %s", iterable.Type())
	}

	return &object.List{Elements: result}
}
