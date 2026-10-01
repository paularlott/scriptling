package evaluator

import (
	"context"
	"fmt"
	"strings"

	"github.com/paularlott/scriptling/ast"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// compileNode returns a closure that evaluates node.
//
// The evaluator is a tree of closures: the AST is walked once, here, and every
// node becomes a small function that already holds its children's closures
// and the decisions that never change (which fast path applies, which
// operator, which slot). Running a program is calling the root closure, so
// there is no per-node type switch at run time. The closure tree is built
// once per *ast.Program and cached on it (see EvalWithContext), shared like
// the parsed program itself.
func compileNode(node ast.Node) object.EvalFn {
	switch n := node.(type) {
	case *ast.Program:
		return compileProgramNode(n)
	case *ast.BlockStatement:
		return compileBlock(n)
	case *ast.ExpressionStatement:
		return compileExpressionStatement(n)
	case *ast.AssignStatement:
		return compileAssign(n)
	case *ast.ReturnStatement:
		return compileReturn(n)
	case *ast.IfStatement:
		return compileIf(n)
	case *ast.WhileStatement:
		return compileWhile(n)
	case *ast.ForStatement:
		return compileFor(n)
	case *ast.Identifier:
		return func(ctx context.Context, env *object.Environment) object.Object {
			return evalIdentifier(n, env)
		}
	case *ast.IntegerLiteral:
		value := n.Value
		return func(ctx context.Context, env *object.Environment) object.Object {
			return object.NewInteger(value)
		}
	case *ast.FloatLiteral:
		value := n.Value
		return func(ctx context.Context, env *object.Environment) object.Object {
			return object.NewFloat(value)
		}
	case *ast.StringLiteral:
		// evalStringLiteral reads the node's boxed-value cache, so the node
		// itself is captured, exactly as the old path did.
		return func(ctx context.Context, env *object.Environment) object.Object {
			return evalStringLiteral(n)
		}
	case *ast.Boolean:
		value := n.Value
		return func(ctx context.Context, env *object.Environment) object.Object {
			return nativeBoolToBooleanObject(value)
		}
	case *ast.None:
		return func(ctx context.Context, env *object.Environment) object.Object {
			return NULL
		}
	case *ast.InfixExpression:
		return compileInfix(n)
	case *ast.PrefixExpression:
		return compilePrefix(n)
	case *ast.ConditionalExpression:
		return compileConditional(n)
	case *ast.IndexExpression:
		return compileIndex(n)
	case *ast.CallExpression:
		return compileCall(n)
	case *ast.MethodCallExpression:
		return compileMethodCall(n)
	case *ast.FunctionStatement:
		return compileFunctionStatement(n)
	case *ast.Lambda:
		return compileLambda(n)
	case *ast.ListLiteral:
		return compileListLiteral(n)
	case *ast.DictLiteral:
		return compileDictLiteral(n)
	case *ast.SetLiteral:
		return compileSetLiteral(n)
	case *ast.TupleLiteral:
		return compileTupleLiteral(n)
	case *ast.ListComprehension:
		return compileListComprehension(n)
	case *ast.DictComprehension:
		return compileDictComprehension(n)
	case *ast.SetComprehension:
		return compileSetComprehension(n)
	case *ast.AugmentedAssignStatement:
		return compileAugmentedAssign(n)
	case *ast.MultipleAssignStatement:
		return compileMultipleAssign(n)
	case *ast.SliceExpression:
		return compileSlice(n)
	case *ast.FStringLiteral:
		return compileFString(n)
	case *ast.BreakStatement:
		return func(ctx context.Context, env *object.Environment) object.Object {
			return object.BREAK
		}
	case *ast.ContinueStatement:
		return func(ctx context.Context, env *object.Environment) object.Object {
			return object.CONTINUE
		}
	case *ast.PassStatement:
		return func(ctx context.Context, env *object.Environment) object.Object {
			return NULL
		}
	case *ast.BytesLiteral:
		value := n.Value
		return func(ctx context.Context, env *object.Environment) object.Object {
			return object.NewBytes(value)
		}
	case *ast.WalrusExpression:
		return compileWalrus(n)
	case *ast.DelStatement:
		return compileDel(n)
	case *ast.GlobalStatement:
		return func(ctx context.Context, env *object.Environment) object.Object {
			return evalGlobalStatement(n, env)
		}
	case *ast.NonlocalStatement:
		return func(ctx context.Context, env *object.Environment) object.Object {
			return evalNonlocalStatement(n, env)
		}
	case *ast.AssertStatement:
		return compileAssert(n)
	case *ast.RaiseStatement:
		return compileRaise(n)
	case *ast.WithStatement:
		return compileWith(n)
	case *ast.TryStatement:
		return compileTry(n)
	case *ast.MatchStatement:
		return compileMatch(n)
	case *ast.ImportStatement:
		return func(ctx context.Context, env *object.Environment) object.Object {
			return evalImportStatement(ctx, n, env)
		}
	case *ast.FromImportStatement:
		return func(ctx context.Context, env *object.Environment) object.Object {
			return evalFromImportStatement(ctx, n, env)
		}
	case *ast.ClassStatement:
		return compileClass(n)
	default:
		// Every node type the parser produces has a case above; reaching
		// here means a new node type was added without a compiled form.
		return func(ctx context.Context, env *object.Environment) object.Object {
			return errors.NewError("internal error: no compiled form for %T", n)
		}
	}
}

// cachedExpr returns the compiled form of e, compiling it on first use and
// caching it in slot. It serves the helpers that reach a sub-expression
// through an AST node at run time rather than through a closure built at
// compile time: assignment and deletion targets such as d[k] = v. Compiling
// only reads the immutable AST, so two goroutines racing on first use build
// equivalent closures and one store wins.
func cachedExpr(slot *ast.CompiledSlot, e ast.Expression) object.EvalFn {
	return cachedNode(slot, e)
}

// cachedNode is cachedExpr for any node.
func cachedNode(slot *ast.CompiledSlot, n ast.Node) object.EvalFn {
	if fn, ok := slot.Load().(object.EvalFn); ok && fn != nil {
		return fn
	}
	fn := compileNode(n)
	slot.Store(fn)
	return fn
}

// compileDefaults compiles each parameter default once, at definition time.
func compileDefaults(defaults map[string]ast.Expression) map[string]object.EvalFn {
	if len(defaults) == 0 {
		return nil
	}
	out := make(map[string]object.EvalFn, len(defaults))
	for name, e := range defaults {
		out[name] = compileExpr(e)
	}
	return out
}

// fixErrorPos gives an Error with a zero line or empty file the position of
// the node being evaluated, so error reports name the statement that failed.
func fixErrorPos(ctx context.Context, obj object.Object, line int) object.Object {
	if err, ok := obj.(*object.Error); ok {
		if err.Line == 0 {
			err.Line = line
		}
		if err.File == "" {
			err.File = GetSourceFileFromContext(ctx)
		}
	}
	return obj
}

// compileExpr and compileStmt are the same thing with narrower parameter
// types, so call sites read clearly.
func compileExpr(e ast.Expression) object.EvalFn { return compileNode(e) }

func compileStmt(s ast.Statement) object.EvalFn { return compileNode(s) }

// compileProgram returns the root closure for a program. It is called at most
// once per *ast.Program: the result is cached on the program via SetCompiled
// and shared like the cached parse itself. Compilation only reads the AST,
// which is immutable after parsing, so two goroutines compiling the same
// program concurrently is safe.
func compileProgram(program *ast.Program) object.EvalFn {
	return compileNode(program)
}

// compileProgramNode compiles the program's statement list. Slot setup and the
// per-statement result handling stay at runtime exactly as in evalProgram.
func compileProgramNode(n *ast.Program) object.EvalFn {
	stmts := make([]object.EvalFn, len(n.Statements))
	lines := make([]int, len(n.Statements))
	for i, statement := range n.Statements {
		stmts[i] = compileStmt(statement)
		lines[i] = statement.Line()
	}
	return func(ctx context.Context, env *object.Environment) object.Object {
		// Set up slots for top-level variables to enable fast slot-based access.
		if slotIndex, slotNames := analyzeTopLevelLocals(n); slotIndex != nil {
			if !env.HasSlots() {
				env.SetupSlots(slotIndex, slotNames)
			} else {
				env.ExtendSlots(slotIndex, slotNames)
			}
		}

		var result object.Object = NULL
		cc := newContextChecker(ctx)

		for i := range stmts {
			if err := cc.check(); err != nil {
				return err
			}

			result = stmts[i](ctx, env)

			switch result := result.(type) {
			case *object.ReturnValue:
				val := result.Value
				releaseReturnValue(result)
				return val
			case *object.Error:
				if result.Line == 0 {
					result.Line = lines[i]
				}
				if result.File == "" {
					result.File = GetSourceFileFromContext(ctx)
				}
				return result
			case *object.Exception:
				// A genuinely raised, uncaught exception propagates out of the
				// program as the Exception itself so callers (handleResult, the
				// module-import path) keep its type and can re-raise or report it
				// faithfully. A bare exception *value* (e.g. a final expression
				// `ValueError("x")`) is an ordinary result and flows through
				// unchanged.
				if result.ExceptionType == object.ExceptionTypeSystemExit || result.Raised {
					return result
				}
			}
		}

		return result
	}
}

// compileBlock compiles a block's statement list. The result-type switch and
// the context checker are kept exactly as they are in
// evalBlockStatementWithContext.
func compileBlock(n *ast.BlockStatement) object.EvalFn {
	stmts := make([]object.EvalFn, len(n.Statements))
	lines := make([]int, len(n.Statements))
	for i, statement := range n.Statements {
		stmts[i] = compileStmt(statement)
		lines[i] = statement.Line()
	}
	return func(ctx context.Context, env *object.Environment) object.Object {
		var result object.Object = NULL
		cc := newContextChecker(ctx)

		for i := range stmts {
			if err := cc.check(); err != nil {
				return err
			}

			result = stmts[i](ctx, env)

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
					r.Line = lines[i]
				}
				if r.File == "" {
					r.File = GetSourceFileFromContext(ctx)
				}
				return r
			case nil:
				// nil result, continue
			default:
				// Handle control-flow signals and raised exceptions. A raised
				// exception unwinds the block; a bare exception *value* produced as
				// a statement result is an ordinary value and does not.
				rt := r.Type()
				if rt == object.RETURN_OBJ || rt == object.BREAK_OBJ || rt == object.CONTINUE_OBJ {
					return result
				}
				if rt == object.EXCEPTION_OBJ && isRaised(r) {
					return result
				}
			}
		}

		return result
	}
}

func compileExpressionStatement(n *ast.ExpressionStatement) object.EvalFn {
	expr := compileExpr(n.Expression)
	line := n.Expression.Line()
	return func(ctx context.Context, env *object.Environment) object.Object {
		return fixErrorPos(ctx, expr(ctx, env), line)
	}
}

// compileAssign converts an assignment statement.
func compileAssign(n *ast.AssignStatement) object.EvalFn {
	// Every target kind (identifier, index, attribute) goes through
	// assignToExpression exactly as before — the conversion compiles the value
	// expression and removes the dispatch needed to reach the helper.
	value := compileExpr(n.Value)
	return func(ctx context.Context, env *object.Environment) object.Object {
		val := value(ctx, env)
		if propagates(val) {
			return val
		}
		// Execute chained assignments first (a = b = 5: assign 5 to b, then to a)
		if n.Chained != nil {
			if err := assignToExpression(ctx, n.Chained.Left, val, env); err != nil {
				return assignErrorToObject(err)
			}
			for c := n.Chained.Chained; c != nil; c = c.Chained {
				if err := assignToExpression(ctx, c.Left, val, env); err != nil {
					return assignErrorToObject(err)
				}
			}
		}
		if err := assignToExpression(ctx, n.Left, val, env); err != nil {
			return assignErrorToObject(err)
		}
		return NULL
	}
}

func compileReturn(n *ast.ReturnStatement) object.EvalFn {
	var value object.EvalFn
	if n.ReturnValue != nil {
		value = compileExpr(n.ReturnValue)
	}
	return func(ctx context.Context, env *object.Environment) object.Object {
		val := object.Object(NULL)
		if value != nil {
			val = value(ctx, env)
			if propagates(val) {
				return val
			}
		}
		return acquireReturnValue(env, val)
	}
}

func compileIf(n *ast.IfStatement) object.EvalFn {
	cond := compileExpr(n.Condition)
	then := compileStmt(n.Consequence)
	elifConds := make([]object.EvalFn, len(n.ElifClauses))
	elifBodies := make([]object.EvalFn, len(n.ElifClauses))
	for i, elifClause := range n.ElifClauses {
		elifConds[i] = compileExpr(elifClause.Condition)
		elifBodies[i] = compileStmt(elifClause.Consequence)
	}
	var els object.EvalFn
	if n.Alternative != nil {
		els = compileStmt(n.Alternative)
	}
	return func(ctx context.Context, env *object.Environment) object.Object {
		condition := cond(ctx, env)
		if propagates(condition) {
			return condition
		}

		truthy, errObj := evalTruthy(ctx, condition, env)
		if errObj != nil {
			return errObj
		}
		if truthy {
			return then(ctx, env)
		}

		// Check elif clauses
		for i := range elifConds {
			condition := elifConds[i](ctx, env)
			if propagates(condition) {
				return condition
			}
			elifTruthy, errObj := evalTruthy(ctx, condition, env)
			if errObj != nil {
				return errObj
			}
			if elifTruthy {
				return elifBodies[i](ctx, env)
			}
		}

		// Check else clause
		if els != nil {
			return els(ctx, env)
		}

		return NULL
	}
}

// loopAction classifies what a loop does after one run of its body.
type loopAction uint8

const (
	loopNext  loopAction = iota // run the next iteration
	loopBreak                   // leave the loop, skipping any else clause
	loopExit                    // return the result from the loop at once
)

// loopResult classifies a loop body's result and returns the value the loop
// should carry forward: errors, return values and exceptions exit the loop
// with that value; break leaves it with NULL; continue and ordinary values
// run the next iteration, with continue normalised to NULL.
//
// It switches on the concrete type rather than calling Type() so that it
// stays within the inliner's budget: it runs once per loop iteration.
func loopResult(result object.Object) (loopAction, object.Object) {
	switch result.(type) {
	case nil:
		return loopNext, result
	case *object.Error, *object.ReturnValue, *object.Exception:
		return loopExit, result
	case *object.Break:
		return loopBreak, NULL
	case *object.Continue:
		return loopNext, NULL
	}
	return loopNext, result
}

func compileWhile(n *ast.WhileStatement) object.EvalFn {
	cond := compileExpr(n.Condition)
	body := compileStmt(n.Body)
	// Errors from the body take the body node's own line when they have none.
	bodyLine := n.Body.Line()
	var els object.EvalFn
	elseLine := 0
	if n.Else != nil {
		els = compileStmt(n.Else)
		elseLine = n.Else.Line()
	}
	return func(ctx context.Context, env *object.Environment) object.Object {
		var result object.Object = NULL
		cc := newContextChecker(ctx)
		broke := false

		for {
			if err := cc.check(); err != nil {
				return err
			}

			condition := cond(ctx, env)
			if propagates(condition) {
				return condition
			}

			truthy, errObj := evalTruthy(ctx, condition, env)
			if errObj != nil {
				return errObj
			}
			if !truthy {
				break
			}

			var act loopAction
			act, result = loopResult(fixErrorPos(ctx, body(ctx, env), bodyLine))
			switch act {
			case loopExit:
				return result
			case loopBreak:
				broke = true
				return NULL
			}
		}

		if !broke && els != nil {
			return fixErrorPos(ctx, els(ctx, env), elseLine)
		}
		return result
	}
}

// compileFastRangeParts resolves the static shape of the `for x in range(...)`
// fast path at compile time: single identifier variable, no else clause, and a
// non-overflowing call to something spelled `range`. The shadowing check
// (whether `range` has been rebound in the environment) is dynamic and stays
// in the closure.
func compileFastRangeParts(n *ast.ForStatement) (*ast.Identifier, []object.EvalFn) {
	if len(n.Variables) != 1 || n.Else != nil {
		return nil, nil
	}
	target, ok := n.Variables[0].(*ast.Identifier)
	if !ok {
		return nil, nil
	}
	call, ok := n.Iterable.(*ast.CallExpression)
	if !ok || call.HasOverflow() {
		return nil, nil
	}
	fnIdent, ok := call.Function.(*ast.Identifier)
	if !ok || fnIdent.Value() != "range" {
		return nil, nil
	}
	args := make([]object.EvalFn, len(call.Arguments))
	for i, arg := range call.Arguments {
		args[i] = compileExpr(arg)
	}
	return target, args
}

// compileRangeArgs mirrors evalRangeArgs with the argument expressions already
// compiled. It returns ok=false when the arguments do not fit the fast path
// (wrong count or non-integer at runtime), sending the caller to the generic
// iteration path, exactly as evalRangeArgs does.
func compileRangeArgs(args []object.EvalFn) func(ctx context.Context, env *object.Environment) (start, stop, step int64, errObj object.Object, ok bool) {
	return func(ctx context.Context, env *object.Environment) (int64, int64, int64, object.Object, bool) {
		if len(args) < 1 || len(args) > 3 {
			return 0, 0, 0, nil, false
		}

		values := [3]int64{}
		for i := range args {
			evaluated := args[i](ctx, env)
			if propagates(evaluated) {
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
			return 0, values[0], 1, nil, true
		case 2:
			return values[0], values[1], 1, nil, true
		default:
			if values[2] == 0 {
				return 0, 0, 0, errors.NewError("range step cannot be zero"), true
			}
			return values[0], values[1], values[2], nil, true
		}
	}
}

func compileFor(n *ast.ForStatement) object.EvalFn {
	body := compileStmt(n.Body)
	var els object.EvalFn
	if n.Else != nil {
		els = compileStmt(n.Else)
	}
	iterable := compileExpr(n.Iterable)
	rangeTarget, rangeArgFns := compileFastRangeParts(n)
	var rangeArgs func(ctx context.Context, env *object.Environment) (int64, int64, int64, object.Object, bool)
	if rangeTarget != nil {
		rangeArgs = compileRangeArgs(rangeArgFns)
	}

	return func(ctx context.Context, env *object.Environment) object.Object {
		// Fast range path. Everything static about the shape was resolved at
		// compile time; the shadowing check is dynamic.
		if rangeTarget != nil {
			if _, shadowed := env.Get("range"); !shadowed {
				start, stop, step, errObj, ok := rangeArgs(ctx, env)
				if ok {
					if errObj != nil {
						return errObj
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
							return err
						}

						setIdentifierFast(rangeTarget, object.NewInteger(i), env)
						var act loopAction
						act, result = loopResult(body(ctx, env))
						switch act {
						case loopExit:
							return result
						case loopBreak:
							return NULL // broke: skip else, return NULL
						}
					}
					return result
				}
			}
		}

		iterableVal := iterable(ctx, env)
		if propagates(iterableVal) {
			return iterableVal
		}

		var result object.Object = NULL
		broke := false

		// Fast path: for k, v in d.items() — bind each pair's key/value straight
		// into the two loop variables, skipping the per-iteration Tuple that
		// DictItems.CreateIterator would allocate only for setForVariables to
		// unpack and discard. Only the 2-variable form qualifies; everything else
		// (1 var, 3+ vars, non-DictItems) falls through to the generic path.
		if di, ok := iterableVal.(*object.DictItems); ok && len(n.Variables) == 2 {
			// Snapshot keys so body mutations (e.g. del d[k]) can't corrupt the
			// range, matching DictItems.CreateIterator's view semantics.
			keys := make([]string, 0, len(di.Dict.Pairs))
			for k := range di.Dict.Pairs {
				keys = append(keys, k)
			}
			cc := newContextChecker(ctx)
			for _, key := range keys {
				pair, ok := di.Dict.Pairs[key]
				if !ok {
					continue // key deleted during iteration — view skips it
				}
				if err := cc.check(); err != nil {
					return err
				}
				if err := setForVariable(n.Variables[0], pair.Key, env); err != nil {
					return errors.NewError("%s", err.Error())
				}
				if err := setForVariable(n.Variables[1], pair.Value, env); err != nil {
					return errors.NewError("%s", err.Error())
				}
				var act loopAction
				act, result = loopResult(body(ctx, env))
				switch act {
				case loopExit:
					return result
				case loopBreak:
					return NULL // broke=True: skip else, return NULL
				}
			}
			if els != nil {
				return els(ctx, env)
			}
			return result
		}

		// Handle Iterator objects and Views
		var iter *object.Iterator
		switch o := iterableVal.(type) {
		case *object.Iterator:
			iter = o
		case *object.Dict:
			iter = o.CreateIterator()
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
				if propagates(iterObj) {
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
				return errors.NewTypeError("iterable", iterableVal.Type().String())
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

				// A raised exception or internal error yielded by the iterator
				// (e.g. a user __next__ that raised something other than
				// StopIteration) propagates out of the for-loop as in Python.
				if propagates(element) {
					return element
				}

				if err := setForVariables(n.Variables, element, env); err != nil {
					return errors.NewError("%s", err.Error())
				}

				var act loopAction
				act, result = loopResult(body(ctx, env))
				switch act {
				case loopExit:
					return result
				case loopBreak:
					broke = true
					goto forDone
				}
			}
			goto forDone
		}

		// Get elements to iterate over based on type
		{
			var elements []object.Object
			switch o := iterableVal.(type) {
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
						if err := setForVariables(n.Variables, element, env); err != nil {
							return errors.NewError("%s", err.Error())
						}
						var act loopAction
						act, result = loopResult(body(ctx, env))
						switch act {
						case loopExit:
							return result
						case loopBreak:
							broke = true
							goto forDone
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
					if err := setForVariables(n.Variables, element, env); err != nil {
						return errors.NewError("%s", err.Error())
					}
					var act loopAction
					act, result = loopResult(body(ctx, env))
					switch act {
					case loopExit:
						return result
					case loopBreak:
						broke = true
						goto forDone
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
					if err := setForVariables(n.Variables, element, env); err != nil {
						return errors.NewError("%s", err.Error())
					}

					var act loopAction
					act, result = loopResult(body(ctx, env))
					switch act {
					case loopExit:
						return result
					case loopBreak:
						broke = true
						goto forDone
					}
				}
				goto forDone
			case *object.Bytes:
				// Iterate as integer byte values 0-255, matching Python's bytes iteration.
				cc := newContextChecker(ctx)
				for _, b := range o.BytesValue() {
					if err := cc.check(); err != nil {
						return err
					}

					element := object.NewInteger(int64(b))
					if err := setForVariables(n.Variables, element, env); err != nil {
						return errors.NewError("%s", err.Error())
					}

					var act loopAction
					act, result = loopResult(body(ctx, env))
					switch act {
					case loopExit:
						return result
					case loopBreak:
						broke = true
						goto forDone
					}
				}
				goto forDone
			default:
				return errors.NewTypeError("iterable", iterableVal.Type().String())
			}

			// Single loop for all iterable types
			cc := newContextChecker(ctx)
			for _, element := range elements {
				if err := cc.check(); err != nil {
					return err
				}

				if err := setForVariables(n.Variables, element, env); err != nil {
					return errors.NewError("%s", err.Error())
				}

				var act loopAction
				act, result = loopResult(body(ctx, env))
				switch act {
				case loopExit:
					return result
				case loopBreak:
					broke = true
					goto forDone
				}
			}
		}

	forDone:
		if !broke && els != nil {
			return els(ctx, env)
		}
		return result
	}
}

// compileInfix converts an infix expression. The branch order matters:
// short-circuit first, then the unboxed integer fast path,
// then the string-concatenation chain, then the general path. IntFast and the
// concat chain's shape are static parser annotations, so they are tested at
// compile time; the fast paths themselves still run at runtime and fall back
// to the general path exactly as before.
func compileInfix(n *ast.InfixExpression) object.EvalFn {
	op := n.Operator
	if op == ast.OpAnd || op == ast.OpOr {
		left := compileExpr(n.Left)
		right := compileExpr(n.Right)
		isAnd := op == ast.OpAnd
		return func(ctx context.Context, env *object.Environment) object.Object {
			l := left(ctx, env)
			if propagates(l) {
				return l
			}

			leftTruthy, errObj := evalTruthy(ctx, l, env)
			if errObj != nil {
				return errObj
			}
			if isAnd {
				if !leftTruthy {
					return l
				}
			} else if leftTruthy {
				return l
			}
			return right(ctx, env)
		}
	}

	intFast := n.IntFast != ast.IntFastNone
	useConcat := false
	var concatLeaves []object.EvalFn
	if op == ast.OpAdd {
		if leftExpr, ok := n.Left.(*ast.InfixExpression); ok && leftExpr.Operator == ast.OpAdd {
			useConcat = true
			concatLeaves = flattenConcatChain(n)
		}
	}
	left := compileExpr(n.Left)
	right := compileExpr(n.Right)
	return func(ctx context.Context, env *object.Environment) object.Object {
		if intFast {
			// tryEvalIntInfix reads only static shape off the node and
			// evalIdentifier for identifier operands, so it is used as-is.
			if result, ok := tryEvalIntInfix(n, env); ok {
				return result
			}
		}
		if useConcat {
			return evalConcatChain(ctx, env, concatLeaves)
		}
		// General path: evaluate both sides
		l := left(ctx, env)
		if propagates(l) {
			return l
		}
		r := right(ctx, env)
		if propagates(r) {
			return r
		}
		return evalInfixExpression(ctx, op, l, r, env)
	}
}

// flattenConcatChain collects the operands of a `+` chain left to right, the
// same flattening concatFolder.walk does at run time.
func flattenConcatChain(expr ast.Expression) []object.EvalFn {
	var leaves []object.EvalFn
	var walk func(e ast.Expression)
	walk = func(e ast.Expression) {
		if infix, ok := e.(*ast.InfixExpression); ok && infix.Operator == ast.OpAdd {
			walk(infix.Left)
			walk(infix.Right)
			return
		}
		leaves = append(leaves, compileExpr(e))
	}
	walk(expr)
	return leaves
}

// evalConcatChain is concatFolder over pre-compiled operands. The folding
// rules are identical: a leading run of strings is joined through one buffer;
// the first non-string operand closes the run and folding continues through
// evalInfixExpression; an error stops the walk and is returned. A raised
// exception that is not an Error is folded, not treated as a failure — the
// original checks IsError only.
func evalConcatChain(ctx context.Context, env *object.Environment, leaves []object.EvalFn) object.Object {
	var buf strings.Builder
	buffed := false
	var acc object.Object

	for _, leaf := range leaves {
		val := leaf(ctx, env)
		if object.IsError(val) {
			return val
		}
		if acc != nil {
			acc = evalInfixExpression(ctx, ast.OpAdd, acc, val, env)
			if object.IsError(acc) {
				return acc
			}
			continue
		}
		if s, ok := val.(*object.String); ok {
			buf.WriteString(s.StringValue())
			buffed = true
			continue
		}
		// First non-string operand: close off the string run, then fold normally.
		if buffed {
			acc = object.NewString(buf.String())
			buf.Reset()
			buffed = false
			acc = evalInfixExpression(ctx, ast.OpAdd, acc, val, env)
			if object.IsError(acc) {
				return acc
			}
		} else {
			acc = val
		}
	}

	if acc != nil {
		return acc
	}
	return object.NewString(buf.String())
}

func compilePrefix(n *ast.PrefixExpression) object.EvalFn {
	op := n.Operator
	right := compileExpr(n.Right)
	return func(ctx context.Context, env *object.Environment) object.Object {
		r := right(ctx, env)
		if propagates(r) {
			return r
		}
		return evalPrefixExpression(ctx, op, r, env)
	}
}

func compileConditional(n *ast.ConditionalExpression) object.EvalFn {
	cond := compileExpr(n.Condition)
	trueExpr := compileExpr(n.TrueExpr)
	falseExpr := compileExpr(n.FalseExpr)
	return func(ctx context.Context, env *object.Environment) object.Object {
		condition := cond(ctx, env)
		if propagates(condition) {
			return condition
		}

		truthy, errObj := evalTruthy(ctx, condition, env)
		if errObj != nil {
			return errObj
		}
		if truthy {
			return trueExpr(ctx, env)
		}
		return falseExpr(ctx, env)
	}
}

func compileIndex(n *ast.IndexExpression) object.EvalFn {
	left := compileExpr(n.Left)
	index := compileExpr(n.Index)
	isDot := n.IsDotAccess
	// Whether the dot-access fast path can apply is a static property: the
	// index must be a string literal naming the attribute.
	dotName, hasDotName := "", false
	if isDot {
		if lit, ok := n.Index.(*ast.StringLiteral); ok {
			dotName = lit.Value
			hasDotName = true
		}
	}
	return func(ctx context.Context, env *object.Environment) object.Object {
		leftVal := left(ctx, env)
		if propagates(leftVal) {
			return leftVal
		}
		// Fast path for obj.attr on an instance whose field exists: this is
		// the first thing evalInstanceIndexExpression would do, so answering
		// it here only skips the generic dispatch. A property descriptor or a
		// missing field falls through to the full path unchanged.
		if hasDotName {
			if inst, ok := leftVal.(*object.Instance); ok {
				if val, ok := inst.GetField(dotName); ok {
					if _, isProp := val.(*object.Property); !isProp {
						return val
					}
				}
			}
		}
		indexVal := index(ctx, env)
		if propagates(indexVal) {
			return indexVal
		}
		return evalIndexExpression(ctx, leftVal, indexVal, isDot)
	}
}

// compileExprs compiles a list of child expressions, preserving order.
func compileExprs(exps []ast.Expression) []object.EvalFn {
	if len(exps) == 0 {
		return nil
	}
	fns := make([]object.EvalFn, len(exps))
	for i, e := range exps {
		fns[i] = compileExpr(e)
	}
	return fns
}

// compileKeywordMap compiles keyword-argument values. Iteration order of the
// returned map at run time matches the old code's range over the AST map:
// unspecified, exactly as before.
func compileKeywordMap(kws map[string]ast.Expression) map[string]object.EvalFn {
	if len(kws) == 0 {
		return nil
	}
	fns := make(map[string]object.EvalFn, len(kws))
	for k, v := range kws {
		fns[k] = compileExpr(v)
	}
	return fns
}

// evalCompiledCallArgs is evalCallArgs over pre-compiled arguments, into the
// same pooled per-root buffer. The returned slice is BORROWED: the caller must
// call object.ReleaseArgs(env, args) once the callee has returned.
func evalCompiledCallArgs(ctx context.Context, env *object.Environment, argFns []object.EvalFn) []object.Object {
	n := len(argFns)
	if n == 0 {
		return nil
	}
	result := object.AcquireArgs(env, n)
	for i := range argFns {
		evaluated := argFns[i](ctx, env)
		if propagates(evaluated) {
			object.ReleaseArgs(env, result)
			return []object.Object{evaluated}
		}
		result[i] = evaluated
	}
	return result
}

// applyCompiledBuiltinFast is applyBuiltinFast over pre-compiled arguments.
func applyCompiledBuiltinFast(ctx context.Context, env *object.Environment, fn *object.Builtin, argFns []object.EvalFn) object.Object {
	args := evalCompiledCallArgs(ctx, env, argFns)
	if isPropagatedError(args) {
		return args[0]
	}
	ctxWithEnv := SetEnvInContext(ctx, env)
	res := fn.Fn(ctxWithEnv, object.Kwargs{}, args...)
	object.ReleaseArgs(env, args)
	return res
}

// compileCall converts a call expression. The identifier-callee fast section
// (callee location cache, per-arity user-function fast paths, builtins by
// name) applies only to non-overflowing calls, and both of those properties
// are static, so the section is chosen at compile time. Everything the old
// code resolved at run time — the callee's value and type, the location cache,
// keyword and unpack evaluation — stays in the closure.
func compileCall(n *ast.CallExpression) object.EvalFn {
	argFns := compileExprs(n.Arguments)
	kwFns := compileKeywordMap(n.GetKeywords())
	unpackFns := compileExprs(n.GetArgsUnpack())
	var kwargsUnpackFn object.EvalFn
	if e := n.GetKwargsUnpack(); e != nil {
		kwargsUnpackFn = compileExpr(e)
	}
	callee := compileExpr(n.Function)

	// general is the old general path for complex call expressions. fnFn
	// supplies the callee when it must still be evaluated (nil means the
	// callee object was already resolved by the caller).
	general := func(ctx context.Context, env *object.Environment, function object.Object, fnFn object.EvalFn) object.Object {
		if fnFn != nil {
			function = fnFn(ctx, env)
			if propagates(function) {
				return function
			}
		}

		args := evalCompiledCallArgs(ctx, env, argFns)
		if isPropagatedError(args) {
			return args[0]
		}
		// args is borrowed from the per-root arg-buffer free-list; release it on
		// every return path from here. (If *unpack append below grows args into a
		// fresh backing, the original pooled buffer is still released correctly; the
		// grown backing is simply not pooled — a rare case.)
		defer object.ReleaseArgs(env, args)

		var keywords map[string]object.Object
		if len(kwFns) > 0 {
			keywords = make(map[string]object.Object, len(kwFns))
			for k, vfn := range kwFns {
				val := vfn(ctx, env)
				if propagates(val) {
					return val
				}
				keywords[k] = val
			}
		}

		for _, unpackFn := range unpackFns {
			argsVal := unpackFn(ctx, env)
			if propagates(argsVal) {
				return argsVal
			}
			unpacked, err := unpackArgsFromIterable(argsVal)
			if err != nil {
				return err
			}
			args = append(args, unpacked...)
		}

		if kwargsUnpackFn != nil {
			kwargsVal := kwargsUnpackFn(ctx, env)
			if propagates(kwargsVal) {
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

	if !n.HasOverflow() {
		if ident, ok := n.Function.(*ast.Identifier); ok {
			name := ident.Value()
			return func(ctx context.Context, env *object.Environment) object.Object {
				if val, found := resolveCallee(n, name, env); found {
					switch fn := val.(type) {
					case *object.Function:
						// Fast paths for common arg counts: avoid slice allocation
						nargs := len(argFns)
						nparams := len(fn.Parameters)
						if fn.Variadic == nil && fn.Kwargs == nil && len(fn.DefaultValues) == 0 && nargs == nparams && nargs <= 3 {
							switch nargs {
							case 1:
								a0 := argFns[0](ctx, env)
								if object.IsError(a0) {
									return a0
								}
								return applyUserFunctionDirect(ctx, fn, a0)
							case 2:
								a0 := argFns[0](ctx, env)
								if object.IsError(a0) {
									return a0
								}
								a1 := argFns[1](ctx, env)
								if object.IsError(a1) {
									return a1
								}
								return applyUserFunction2(ctx, fn, a0, a1)
							case 3:
								a0 := argFns[0](ctx, env)
								if object.IsError(a0) {
									return a0
								}
								a1 := argFns[1](ctx, env)
								if object.IsError(a1) {
									return a1
								}
								a2 := argFns[2](ctx, env)
								if object.IsError(a2) {
									return a2
								}
								return applyUserFunctionN(ctx, fn, a0, a1, a2)
							}
						}
						args := evalCompiledCallArgs(ctx, env, argFns)
						if isPropagatedError(args) {
							return args[0]
						}
						res := applyUserFunction(ctx, fn, args, nil, env)
						object.ReleaseArgs(env, args)
						return res
					case *object.Builtin:
						// Use the existing fast builtin path
						return applyCompiledBuiltinFast(ctx, env, fn, argFns)
					case *object.LambdaFunction:
						args := evalCompiledCallArgs(ctx, env, argFns)
						if isPropagatedError(args) {
							return args[0]
						}
						res := applyLambdaFunctionWithContext(ctx, fn, args, nil, env)
						object.ReleaseArgs(env, args)
						return res
					}
					// Class or other callable - fall through to general path
					// (no overflow here, so keywords and unpacks are empty).
					return general(ctx, env, val, nil)
				}
				// Not in env - try fast builtins by name
				if builtin, ok := builtins[name]; ok {
					return applyCompiledBuiltinFast(ctx, env, builtin, argFns)
				}
				// Not found: the old code fell to the general path, which
				// re-evaluated the callee expression and produced the
				// identifier error from there.
				return general(ctx, env, nil, callee)
			}
		}
	}

	return func(ctx context.Context, env *object.Environment) object.Object {
		return general(ctx, env, nil, callee)
	}
}

// compileMethodCall converts a method call. The receiver, the method name and
// the argument list are compiled once; every dispatch decision (dict get,
// dict callable, string upper/lower, plain instance methods, generic path)
// depends on the evaluated receiver and stays in the closure. The overflow
// flag is a static parser annotation, so its tests fold at compile time.
func compileMethodCall(n *ast.MethodCallExpression) object.EvalFn {
	receiver := compileExpr(n.Receiver)
	name := n.Method.Value()
	argFns := compileExprs(n.Arguments)
	kwFns := compileKeywordMap(n.GetKeywords())
	unpackFns := compileExprs(n.GetArgsUnpack())
	var kwargsUnpackFn object.EvalFn
	if e := n.GetKwargsUnpack(); e != nil {
		kwargsUnpackFn = compileExpr(e)
	}
	hasOverflow := n.HasOverflow()

	return func(ctx context.Context, env *object.Environment) object.Object {
		obj := receiver(ctx, env)
		if propagates(obj) {
			return obj
		}

		// Fast path for the hottest production payload access pattern:
		// dict.get(key) and dict.get(key, default) without kwargs or unpacking.
		if dict, ok := obj.(*object.Dict); ok && name == "get" &&
			!dictCallableMethodExists(dict, "get") &&
			!hasOverflow &&
			(len(argFns) == 1 || len(argFns) == 2) {
			return evalFastDictGetCompiled(ctx, dict, argFns, env)
		}

		if dict, ok := obj.(*object.Dict); ok &&
			!hasOverflow &&
			dictCallableMethodExists(dict, name) {
			return evalFastDictCallableMethodCompiled(ctx, dict, name, argFns, env)
		}

		// Fast path for the most common string method calls in hot loops.
		if len(argFns) == 0 && !hasOverflow {
			if str, ok := obj.(*object.String); ok {
				switch name {
				case "upper":
					return object.NewString(fastStringUpper(str.StringValue()))
				case "lower":
					return object.NewString(fastStringLower(str.StringValue()))
				}
			}
		}

		// Fast path for plain instance method calls, which are the object-oriented
		// equivalent of the monomorphic paths in evalCallExpression.
		if inst, ok := obj.(*object.Instance); ok && !hasOverflow {
			if result, handled := tryEvalInstanceMethodFastCompiled(ctx, inst, name, argFns, env); handled {
				return result
			}
		}

		args := evalCompiledCallArgs(ctx, env, argFns)
		if isPropagatedError(args) {
			return args[0]
		}
		// args is borrowed from the per-root arg-buffer free-list; release on return.
		defer object.ReleaseArgs(env, args)

		// Evaluate keyword arguments
		var keywords map[string]object.Object
		if len(kwFns) > 0 {
			keywords = make(map[string]object.Object, len(kwFns))
			for k, vfn := range kwFns {
				val := vfn(ctx, env)
				if propagates(val) {
					return val
				}
				keywords[k] = val
			}
		}

		// Handle *args unpacking (supports multiple)
		for _, unpackFn := range unpackFns {
			argsVal := unpackFn(ctx, env)
			if propagates(argsVal) {
				return argsVal
			}
			unpacked, err := unpackArgsFromIterable(argsVal)
			if err != nil {
				return err
			}
			args = append(args, unpacked...)
		}

		// Handle **kwargs unpacking
		if kwargsUnpackFn != nil {
			kwargsVal := kwargsUnpackFn(ctx, env)
			if propagates(kwargsVal) {
				return kwargsVal
			}
			if dict, ok := kwargsVal.(*object.Dict); ok {
				if keywords == nil {
					keywords = make(map[string]object.Object, len(dict.Pairs))
				}
				for _, pair := range dict.Pairs {
					if str, ok := pair.Key.(*object.String); ok {
						keywords[str.StringValue()] = pair.Value
					} else {
						return errors.NewError("keywords must be strings, not %s", pair.Key.Type())
					}
				}
			} else {
				return errors.NewError("argument after ** must be a dictionary, not %s", kwargsVal.Type())
			}
		}

		return callStringMethodWithKeywords(ctx, obj, name, args, keywords, env)
	}
}

// evalFastDictGetCompiled is evalFastDictGet over pre-compiled arguments.
func evalFastDictGetCompiled(ctx context.Context, dict *object.Dict, argFns []object.EvalFn, env *object.Environment) object.Object {
	keyObj := argFns[0](ctx, env)
	if propagates(keyObj) {
		return keyObj
	}

	var defaultObj object.Object = NULL
	if len(argFns) == 2 {
		defaultObj = argFns[1](ctx, env)
		if propagates(defaultObj) {
			return defaultObj
		}
	}

	if keyStr, ok := keyObj.(*object.String); ok {
		if pair, exists := dict.Pairs[object.DictStringKey(keyStr.StringValue())]; exists {
			return pair.Value
		}
		return defaultObj
	}

	key, rerr := evalHashKeyChecked(ctx, keyObj)
	if rerr != nil {
		return rerr
	}
	if pair, exists := dict.Pairs[key]; exists {
		return pair.Value
	}
	return defaultObj
}

// evalFastDictCallableMethodCompiled is evalFastDictCallableMethod over
// pre-compiled arguments.
func evalFastDictCallableMethodCompiled(ctx context.Context, dict *object.Dict, method string, argFns []object.EvalFn, env *object.Environment) object.Object {
	pair, ok := dict.GetByString(method)
	if !ok {
		return errors.NewError("%s: method %s not found in library", errors.ErrIdentifierNotFound, method)
	}

	args := evalCompiledCallArgs(ctx, env, argFns)
	if isPropagatedError(args) {
		return args[0]
	}
	// args is borrowed from the per-root arg-buffer free-list; release on return.
	defer object.ReleaseArgs(env, args)

	switch fn := pair.Value.(type) {
	case *object.Builtin:
		ctxWithEnv := SetEnvInContext(ctx, env)
		return fn.Fn(ctxWithEnv, object.NewKwargs(nil), args...)
	case *object.Function:
		return applyFunctionWithContext(ctx, fn, args, nil, env)
	case *object.LambdaFunction:
		return applyFunctionWithContext(ctx, fn, args, nil, env)
	case *object.Class:
		return applyFunctionWithContext(ctx, fn, args, nil, env)
	default:
		return errors.NewError("%s: %s is not callable", errors.ErrIdentifierNotFound, method)
	}
}

// tryEvalInstanceMethodFastCompiled is tryEvalInstanceMethodFast over
// pre-compiled arguments, with the method name supplied directly.
func tryEvalInstanceMethodFastCompiled(ctx context.Context, inst *object.Instance, name string, argFns []object.EvalFn, env *object.Environment) (object.Object, bool) {
	// An instance field of the same name shadows the class method and is called
	// without self, so leave that to callInstanceMethod.
	if _, shadowed := inst.GetField(name); shadowed {
		return nil, false
	}
	member, ok := inst.Class.LookupMember(name)
	if !ok {
		return nil, false
	}
	// Only plain functions: staticmethod, classmethod, property and bound
	// methods all bind their first argument differently.
	fn, ok := member.(*object.Function)
	if !ok {
		return nil, false
	}
	if fn.Variadic != nil || fn.Kwargs != nil || len(fn.DefaultValues) != 0 || fn.KeywordOnlyStart != 0 {
		return nil, false
	}
	nargs := len(argFns)
	if nargs > 3 {
		return nil, false
	}
	// Parameters include self, and every one must map to a slot so the argument
	// can be written directly.
	if len(fn.Parameters) != nargs+1 || len(fn.ParamSlotIndexes) != nargs+1 {
		return nil, false
	}

	switch nargs {
	case 0:
		return applyUserFunctionDirect(ctx, fn, inst), true
	case 1:
		a0 := argFns[0](ctx, env)
		if object.IsError(a0) {
			return a0, true
		}
		return applyUserFunction2(ctx, fn, inst, a0), true
	case 2:
		a0 := argFns[0](ctx, env)
		if object.IsError(a0) {
			return a0, true
		}
		a1 := argFns[1](ctx, env)
		if object.IsError(a1) {
			return a1, true
		}
		return applyUserFunctionN(ctx, fn, inst, a0, a1), true
	default:
		a0 := argFns[0](ctx, env)
		if object.IsError(a0) {
			return a0, true
		}
		a1 := argFns[1](ctx, env)
		if object.IsError(a1) {
			return a1, true
		}
		a2 := argFns[2](ctx, env)
		if object.IsError(a2) {
			return a2, true
		}
		return applyUserFunctionN(ctx, fn, inst, a0, a1, a2), true
	}
}

// compileFunctionStatement converts a def statement. The body is not
// compiled here: functionBody compiles it on the first call and caches the
// closure on the body's AST node, so a library that defines a hundred
// functions and has two of them called retains closures for two. Decorators
// and parameter defaults are compiled now, since they run at definition.
func compileFunctionStatement(n *ast.FunctionStatement) object.EvalFn {
	defaults := compileDefaults(n.Function.GetDefaultValues())
	decorators := make([]object.EvalFn, len(n.GetDecorators()))
	for i, d := range n.GetDecorators() {
		decorators[i] = compileExpr(d)
	}
	name := n.Name.Value()
	return func(ctx context.Context, env *object.Environment) object.Object {
		localSlots, localSlotNames := analyzeFunctionLocals(n)
		fn := &object.Function{
			Name:             name,
			Parameters:       n.Function.Parameters,
			DefaultValues:    n.Function.GetDefaultValues(),
			Variadic:         n.Function.GetVariadic(),
			Kwargs:           n.Function.GetKwargs(),
			KeywordOnlyStart: n.Function.GetKeywordOnlyStart() + 1,
			Body:             n.Function.Body,
			Env:              env,
			LocalSlots:       localSlots,
			LocalSlotNames:   localSlotNames,
			ParamSlotIndexes: n.Function.ParamSlotIndexes,
			ReuseCallEnv:     !n.Function.HasNestedFunc,
			CompiledDefaults: defaults,
			CompilerOwned:    true,
		}
		var result object.Object = fn
		for i := len(decorators) - 1; i >= 0; i-- {
			dec := decorators[i](ctx, env)
			if propagates(dec) {
				return dec
			}
			result = applyFunctionWithContext(ctx, dec, []object.Object{result}, nil, env)
			if propagates(result) {
				return result
			}
			// If the decorator returned a Function with a different name, rename it
			// so class method lookup (which keys by Function.Name) still works.
			if wrapped, ok := result.(*object.Function); ok && wrapped.Name != fn.Name {
				wrapped.Name = fn.Name
			}
		}
		env.Set(name, result)
		return result
	}
}

// compileLambda converts a lambda. Its expression body is compiled once and
// stored on every LambdaFunction the node creates.
func compileLambda(n *ast.Lambda) object.EvalFn {
	body := compileExpr(n.Body)
	defaults := compileDefaults(n.GetDefaultValues())
	return func(ctx context.Context, env *object.Environment) object.Object {
		localSlots, localSlotNames := analyzeLambdaLocals(n)
		return &object.LambdaFunction{
			Parameters:       n.Parameters,
			DefaultValues:    n.GetDefaultValues(),
			Variadic:         n.GetVariadic(),
			Kwargs:           n.GetKwargs(),
			KeywordOnlyStart: n.GetKeywordOnlyStart() + 1,
			Body:             n.Body,
			Env:              env,
			LocalSlots:       localSlots,
			LocalSlotNames:   localSlotNames,
			ParamSlotIndexes: n.ParamSlotIndexes,
			CompiledBody:     body,
			CompiledDefaults: defaults,
		}
	}
}

// evalCompiledExpressions is evalExpressionsWithContext over pre-compiled
// expressions: on the first propagating value it returns the one-element
// sentinel slice, exactly like the original.
func evalCompiledExpressions(ctx context.Context, env *object.Environment, fns []object.EvalFn) []object.Object {
	if len(fns) == 0 {
		return nil
	}
	result := make([]object.Object, len(fns))
	for i := range fns {
		evaluated := fns[i](ctx, env)
		if propagates(evaluated) {
			return []object.Object{evaluated}
		}
		result[i] = evaluated
	}
	return result
}

func compileListLiteral(n *ast.ListLiteral) object.EvalFn {
	elements := compileExprs(n.Elements)
	return func(ctx context.Context, env *object.Environment) object.Object {
		vals := evalCompiledExpressions(ctx, env, elements)
		if isPropagatedError(vals) {
			return vals[0]
		}
		return &object.List{Elements: vals}
	}
}

func compileTupleLiteral(n *ast.TupleLiteral) object.EvalFn {
	elements := compileExprs(n.Elements)
	return func(ctx context.Context, env *object.Environment) object.Object {
		vals := evalCompiledExpressions(ctx, env, elements)
		if isPropagatedError(vals) {
			return vals[0]
		}
		return &object.Tuple{Elements: vals}
	}
}

func compileSetLiteral(n *ast.SetLiteral) object.EvalFn {
	elements := compileExprs(n.Elements)
	return func(ctx context.Context, env *object.Environment) object.Object {
		vals := evalCompiledExpressions(ctx, env, elements)
		if isPropagatedError(vals) {
			return vals[0]
		}
		s := object.NewSet()
		for _, elem := range vals {
			if err := evalSetAdd(ctx, s, elem); err != nil {
				return err
			}
		}
		return s
	}
}

func compileDictLiteral(n *ast.DictLiteral) object.EvalFn {
	pairKeys := make([]object.EvalFn, len(n.Pairs))
	pairValues := make([]object.EvalFn, len(n.Pairs))
	for i, pairNode := range n.Pairs {
		pairKeys[i] = compileExpr(pairNode.Key)
		pairValues[i] = compileExpr(pairNode.Value)
	}
	return func(ctx context.Context, env *object.Environment) object.Object {
		if len(pairKeys) == 0 {
			return &object.Dict{Pairs: make(map[string]object.DictPair)}
		}
		pairs := make(map[string]object.DictPair, len(pairKeys))

		for i := range pairKeys {
			key := pairKeys[i](ctx, env)
			if propagates(key) {
				return key
			}

			value := pairValues[i](ctx, env)
			if propagates(value) {
				return value
			}

			hk, raised := evalHashKeyChecked(ctx, key)
			if raised != nil {
				return raised
			}
			pairs[hk] = object.DictPair{Key: key, Value: value}
		}

		return &object.Dict{Pairs: pairs}
	}
}

func compileSlice(n *ast.SliceExpression) object.EvalFn {
	left := compileExpr(n.Left)
	var startFn, endFn, stepFn object.EvalFn
	if n.Start != nil {
		startFn = compileExpr(n.Start)
	}
	if n.End != nil {
		endFn = compileExpr(n.End)
	}
	if n.GetStep() != nil {
		stepFn = compileExpr(n.GetStep())
	}
	return func(ctx context.Context, env *object.Environment) object.Object {
		leftVal := left(ctx, env)
		if propagates(leftVal) {
			return leftVal
		}

		var start, end, step int64
		var hasStart, hasEnd, hasStep bool
		step = 1 // default step

		if startFn != nil {
			startObj := startFn(ctx, env)
			if propagates(startObj) {
				return startObj
			}
			s, err := startObj.AsInt()
			if err != nil {
				return err
			}
			start = s
			hasStart = true
		}

		if endFn != nil {
			endObj := endFn(ctx, env)
			if propagates(endObj) {
				return endObj
			}
			e, err := endObj.AsInt()
			if err != nil {
				return err
			}
			end = e
			hasEnd = true
		}

		if stepFn != nil {
			stepObj := stepFn(ctx, env)
			if propagates(stepObj) {
				return stepObj
			}
			s, err := stepObj.AsInt()
			if err != nil {
				return err
			}
			step = s
			hasStep = true
			if step == 0 {
				return errors.NewError("slice step cannot be zero")
			}
		}

		switch obj := leftVal.(type) {
		case *object.List:
			return sliceList(obj.Elements, start, end, step, hasStart, hasEnd, hasStep)
		case *object.Tuple:
			result := sliceList(obj.Elements, start, end, step, hasStart, hasEnd, hasStep)
			if list, ok := result.(*object.List); ok {
				return &object.Tuple{Elements: list.Elements}
			}
			return result
		case *object.String:
			elements := sliceString(obj.StringValue(), start, end, step, hasStart, hasEnd, hasStep)
			return object.NewString(elements)
		case *object.Bytes:
			return sliceBytes(obj, start, end, step, hasStart, hasEnd, hasStep)
		case *object.FloatArray:
			return sliceFloatArray(obj, start, end, step, hasStart, hasEnd, hasStep)
		default:
			return errors.NewError("slice operator not supported: %s", leftVal.Type())
		}
	}
}

func compileAugmentedAssign(n *ast.AugmentedAssignStatement) object.EvalFn {
	target := n.Left
	if target == nil {
		target = n.Name
	}
	if target == nil {
		errObj := errors.NewError("invalid augmented assignment target")
		return func(ctx context.Context, env *object.Environment) object.Object {
			return errObj
		}
	}
	valueFn := compileExpr(n.Value)
	op := n.Operator

	// Subscript and attribute targets evaluate their container and key once,
	// then read, operate and write through those operands, as Python does:
	// `d[k()] += v` calls k() a single time.
	if idx, ok := target.(*ast.IndexExpression); ok {
		return func(ctx context.Context, env *object.Environment) object.Object {
			ops, errObj := evalAugIndexOperands(ctx, idx, env)
			if errObj != nil {
				return errObj
			}
			currentVal := ops.read(ctx, idx.IsDotAccess)
			if propagates(currentVal) {
				return currentVal
			}
			newVal := valueFn(ctx, env)
			if propagates(newVal) {
				return newVal
			}
			result, done, errObj := applyAugmentedOp(ctx, op, currentVal, newVal, env)
			if errObj != nil {
				return errObj
			}
			if done {
				return NULL
			}
			if err := ops.write(ctx, idx.IsDotAccess, result); err != nil {
				return assignErrorToObject(err)
			}
			return NULL
		}
	}

	targetFn := compileExpr(target)
	return func(ctx context.Context, env *object.Environment) object.Object {
		currentVal := targetFn(ctx, env)
		if propagates(currentVal) {
			return currentVal
		}
		newVal := valueFn(ctx, env)
		if propagates(newVal) {
			return newVal
		}
		result, done, errObj := applyAugmentedOp(ctx, op, currentVal, newVal, env)
		if errObj != nil {
			return errObj
		}
		if done {
			return NULL
		}
		if err := assignToExpression(ctx, target, result, env); err != nil {
			return assignErrorToObject(err)
		}
		return NULL
	}
}

// applyAugmentedOp computes current op= value. done reports that the
// operation mutated current in place (list +=, dict |=, set ops), so the
// caller has nothing to store; otherwise result is the value to assign.
func applyAugmentedOp(ctx context.Context, op ast.Op, currentVal, newVal object.Object, env *object.Environment) (result object.Object, done bool, errObj object.Object) {
	// Fast path: string += string, int += int
	if op == ast.OpAddEq {
		if cur, ok := currentVal.(*object.String); ok {
			if r, ok := newVal.(*object.String); ok {
				return object.NewString(cur.StringValue() + r.StringValue()), false, nil
			}
		}
		if cur, ok := currentVal.(*object.Integer); ok {
			if r, ok := newVal.(*object.Integer); ok {
				return object.NewInteger(cur.IntValue() + r.IntValue()), false, nil
			}
		}
	}

	// Fast path: dict |= dict merges into the left dict in place (PEP 584), so
	// other references to the same dict observe the update, matching Python.
	if op == ast.OpBitOrEq {
		if cur, ok := currentVal.(*object.Dict); ok {
			if r, ok := newVal.(*object.Dict); ok {
				if cur.Pairs == nil {
					cur.Pairs = make(map[string]object.DictPair, len(r.Pairs))
				}
				for k, v := range r.Pairs {
					cur.Pairs[k] = v
				}
				return nil, true, nil
			}
		}
	}

	// Fast path: list += list extends and list *= n repeats in place, since
	// Python's list iadd/imul mutate and aliases observe the update.
	if cur, ok := currentVal.(*object.List); ok {
		switch op {
		case ast.OpAddEq:
			if r, ok := newVal.(*object.List); ok {
				cur.Elements = append(cur.Elements, r.Elements...)
				return nil, true, nil
			}
		case ast.OpMulEq:
			if r, ok := newVal.(*object.Integer); ok {
				elements, errObj := repeatElements(cur.Elements, r.IntValue())
				if errObj != nil {
					return nil, false, errObj
				}
				cur.Elements = elements
				return nil, true, nil
			}
		}
	}

	// Fast path: set augmented operators mutate in place, matching Python
	// (set __ior__ and friends are in-place updates).
	if cur, ok := currentVal.(*object.Set); ok {
		if r, ok := newVal.(*object.Set); ok {
			switch op {
			case ast.OpBitOrEq:
				cur.InPlaceUnion(r)
				return nil, true, nil
			case ast.OpBitAndEq:
				cur.InPlaceIntersection(r)
				return nil, true, nil
			case ast.OpSubEq:
				cur.InPlaceDifference(r)
				return nil, true, nil
			case ast.OpBitXorEq:
				cur.InPlaceSymmetricDifference(r)
				return nil, true, nil
			}
		}
	}

	baseOp := op.BaseOp()
	if baseOp == op {
		return nil, false, errors.NewError("unknown augmented assignment operator: %s", op)
	}
	result = evalInfixExpression(ctx, baseOp, currentVal, newVal, env)
	if object.IsError(result) {
		return nil, false, result
	}
	return result, false, nil
}

// augIndexOperands are the operands of a subscript or attribute target,
// evaluated once per augmented assignment.
type augIndexOperands struct {
	obj, index object.Object
	// A 2-D float array element (`fa[i][j] += v`) is addressed directly:
	// reading fa[i] yields a copy of the row, so a write through it would be
	// lost. fa is nil for every other target.
	fa       *object.FloatArray
	row, col *object.Integer
}

// evalAugIndexOperands evaluates the target's sub-expressions exactly once.
// For a nested subscript it evaluates the base and inner key, then resolves
// the inner subscript itself unless the base is a 2-D float array.
func evalAugIndexOperands(ctx context.Context, t *ast.IndexExpression, env *object.Environment) (augIndexOperands, object.Object) {
	var ops augIndexOperands
	if inner, ok := t.Left.(*ast.IndexExpression); ok {
		base := fixErrorPos(ctx, cachedExpr(&inner.TargetSlots().Left, inner.Left)(ctx, env), inner.Left.Line())
		if propagates(base) {
			return ops, base
		}
		innerKey := cachedExpr(&inner.TargetSlots().Index, inner.Index)(ctx, env)
		if propagates(innerKey) {
			return ops, innerKey
		}
		key := cachedExpr(&t.TargetSlots().Index, t.Index)(ctx, env)
		if propagates(key) {
			return ops, key
		}
		if fa, ok := base.(*object.FloatArray); ok && fa.Is2D() {
			row, rowOK := innerKey.(*object.Integer)
			col, colOK := key.(*object.Integer)
			if !rowOK || !colOK {
				return ops, errors.NewError("float_array index must be integer")
			}
			ops.fa, ops.row, ops.col = fa, row, col
			return ops, nil
		}
		obj := evalIndexExpression(ctx, base, innerKey, inner.IsDotAccess)
		if propagates(obj) {
			return ops, obj
		}
		ops.obj, ops.index = obj, key
		return ops, nil
	}
	obj := fixErrorPos(ctx, cachedExpr(&t.TargetSlots().Left, t.Left)(ctx, env), t.Left.Line())
	if propagates(obj) {
		return ops, obj
	}
	key := cachedExpr(&t.TargetSlots().Index, t.Index)(ctx, env)
	if propagates(key) {
		return ops, key
	}
	ops.obj, ops.index = obj, key
	return ops, nil
}

// read returns the target's current value.
func (ops *augIndexOperands) read(ctx context.Context, isDotAccess bool) object.Object {
	if ops.fa != nil {
		rowObj := evalIndexExpression(ctx, ops.fa, ops.row, false)
		if propagates(rowObj) {
			return rowObj
		}
		return evalIndexExpression(ctx, rowObj, ops.col, false)
	}
	return evalIndexExpression(ctx, ops.obj, ops.index, isDotAccess)
}

// write stores the result in the target.
func (ops *augIndexOperands) write(ctx context.Context, isDotAccess bool, value object.Object) error {
	if ops.fa != nil {
		return assignNestedFloatArrayValue(ops.fa, ops.row, ops.col, value)
	}
	return assignIndexValue(ctx, isDotAccess, ops.obj, ops.index, value)
}

func compileMultipleAssign(n *ast.MultipleAssignStatement) object.EvalFn {
	valueFn := compileExpr(n.Value)
	return func(ctx context.Context, env *object.Environment) object.Object {
		val := valueFn(ctx, env)
		if propagates(val) {
			return val
		}

		var elements []object.Object

		// Value can be a list or tuple; any other iterable (including class
		// instances with __iter__) is materialized, with a raise from the
		// iterator protocol propagating — Python unpacks any iterable.
		switch v := val.(type) {
		case *object.List:
			elements = v.Elements
		case *object.Tuple:
			elements = v.Elements
		default:
			elems, ok, rerr := iterableToSliceChecked(ctx, val, env)
			if rerr != nil {
				return rerr
			}
			if !ok {
				return errors.NewTypeError("list or tuple", val.Type().String())
			}
			elements = elems
		}

		// Handle starred unpacking
		if n.StarredIndex >= 0 {
			// With starred unpacking: a, *b, c = [1, 2, 3, 4, 5]
			// Need at least (len(names) - 1) elements
			minElements := len(n.Names) - 1
			if len(elements) < minElements {
				return errors.NewError("not enough values to unpack (expected at least %d, got %d)", minElements, len(elements))
			}

			// Assign elements before the starred variable
			for i := 0; i < n.StarredIndex; i++ {
				env.Set(n.Names[i].Value(), elements[i])
			}

			// Calculate how many elements go to the starred variable
			elementsAfterStar := len(n.Names) - n.StarredIndex - 1
			starStart := n.StarredIndex
			starEnd := len(elements) - elementsAfterStar

			// Assign starred variable (as a list)
			starredElements := elements[starStart:starEnd]
			env.Set(n.Names[n.StarredIndex].Value(), &object.List{Elements: starredElements})

			// Assign elements after the starred variable
			for i := 0; i < elementsAfterStar; i++ {
				nameIdx := n.StarredIndex + 1 + i
				elemIdx := starEnd + i
				env.Set(n.Names[nameIdx].Value(), elements[elemIdx])
			}
		} else {
			// No starred unpacking - exact length match required
			if len(elements) != len(n.Names) {
				return errors.NewError("cannot unpack %d values to %d variables", len(elements), len(n.Names))
			}

			// Assign each value
			for i, name := range n.Names {
				env.Set(name.Value(), elements[i])
			}
		}

		return NULL
	}
}

func compileFString(n *ast.FStringLiteral) object.EvalFn {
	exprFns := compileExprs(n.Expressions)
	return func(ctx context.Context, env *object.Environment) object.Object {
		var builder strings.Builder

		// Pre-allocate capacity to reduce reallocations
		// Estimate base size from static parts plus some buffer for expressions
		estimatedSize := 0
		for _, part := range n.Parts {
			estimatedSize += len(part)
		}
		// Add buffer for formatted expressions (rough estimate)
		estimatedSize += len(exprFns) * 16
		builder.Grow(estimatedSize)

		conversions := n.GetConversions()
		specs := n.GetFormatSpecs()
		debugTexts := n.GetDebugTexts()
		for i, part := range n.Parts {
			builder.WriteString(part)
			if i < len(exprFns) {
				exprResult := exprFns[i](ctx, env)
				if propagates(exprResult) {
					return exprResult
				}
				spec := ""
				if specs != nil && i < len(specs) {
					spec = specs[i]
				}
				conv := ""
				if conversions != nil && i < len(conversions) {
					conv = conversions[i]
				}
				debugText := ""
				if debugTexts != nil && i < len(debugTexts) {
					debugText = debugTexts[i]
				}
				// Nested spec fields ({x:>{w}}) resolve against the enclosing
				// scope before formatting.
				if strings.Contains(spec, "{") {
					expanded, serr := expandNestedSpecFields(ctx, spec, env)
					if serr != nil {
						return serr
					}
					spec = expanded
				}
				if debugText != "" && conv == "" {
					// f"{x=}" debug: repr-style rendering by default, spec still
					// applies to the value, and the prefix is prepended verbatim.
					rendered, rerr := renderConvertedValue(ctx, exprResult, "r", env)
					if rerr != nil {
						return rerr
					}
					builder.WriteString(debugText)
					builder.WriteString(formatWithSpec(object.NewString(rendered), spec))
					continue
				}
				var formatted string
				if conv != "" {
					// Explicit !r/!s/!a conversion: render to its string form,
					// then apply the spec. Raises from dunders propagate.
					rendered, rerr := renderConvertedValue(ctx, exprResult, conv, env)
					if rerr != nil {
						return rerr
					}
					formatted = formatWithSpec(object.NewString(rendered), spec)
				} else if exc, ok := exprResult.(*object.Exception); ok {
					// Exceptions format as their message (str(e)), like Python.
					formatted = formatWithSpec(object.NewString(exc.Message), spec)
				} else if inst, ok := exprResult.(*object.Instance); ok {
					// Instances convert with str() semantics (__str__ then
					// __repr__); a raise propagates. The spec applies to the
					// converted string, as in Python.
					rendered, rerr := strInstanceChecked(ctx, inst, env)
					if rerr != nil {
						return rerr
					}
					formatted = formatWithSpec(object.NewString(rendered), spec)
				} else {
					// Other types keep their typed value so numeric specs
					// (.2f, >6d) apply directly.
					formatted = formatWithSpec(exprResult, spec)
				}
				builder.WriteString(formatted)
			}
		}

		return object.NewString(builder.String())
	}
}

// compiledClause is a comprehension's additional `for` clause with its
// iterable and optional condition pre-compiled.
type compiledClause struct {
	iterable  object.EvalFn
	condition object.EvalFn // nil when the clause has no `if`
	variables []ast.Expression
}

func compiledClauses(clauses []ast.ComprehensionClause) []compiledClause {
	if len(clauses) == 0 {
		return nil
	}
	out := make([]compiledClause, len(clauses))
	for i, c := range clauses {
		out[i].iterable = compileExpr(c.Iterable)
		if c.Condition != nil {
			out[i].condition = compileExpr(c.Condition)
		}
		out[i].variables = c.Variables
	}
	return out
}

// evalCompiledAdditionalClauses mirrors evalAdditionalClauses over
// pre-compiled clauses.
func evalCompiledAdditionalClauses(ctx context.Context, clauses []compiledClause, idx int, env *object.Environment, action func() object.Object) object.Object {
	if idx >= len(clauses) {
		return action()
	}
	c := clauses[idx]
	iterable := c.iterable(ctx, env)
	if propagates(iterable) {
		return iterable
	}
	return iterateObject(ctx, iterable, func(element object.Object) object.Object {
		if err := setForVariables(c.variables, element, env); err != nil {
			return errors.NewError("%s", err.Error())
		}
		if c.condition != nil {
			cond := c.condition(ctx, env)
			if propagates(cond) {
				return cond
			}
			truthy, errObj := evalTruthy(ctx, cond, env)
			if errObj != nil {
				return errObj
			}
			if !truthy {
				return nil
			}
		}
		return evalCompiledAdditionalClauses(ctx, clauses, idx+1, env, action)
	})
}

// comprehensionFastVar reports whether the comprehension qualifies for the
// single-identifier fast path: no additional clauses and one plain identifier
// variable. Both are static properties of the node.
func comprehensionFastVar(variables []ast.Expression, additional int) *ast.Identifier {
	if additional > 0 || len(variables) != 1 {
		return nil
	}
	ident, _ := variables[0].(*ast.Identifier)
	return ident
}

// compSource is a comprehension's iteration, prepared once at compile time
// and shared by list, dict and set comprehensions.
type compSource struct {
	iterable  object.EvalFn
	variables []ast.Expression
	// fastIdent is set when the comprehension binds one plain name. The
	// comprehension environment then gives that name a slot, so the element
	// and condition expressions read it through the slot cache instead of a
	// map lookup per element.
	fastIdent *ast.Identifier
	slotIndex map[string]int
	slotNames []string
	// rangeArgs is set when the iterable is a literal range(...) call, so the
	// integers are produced directly rather than through a range iterator.
	rangeArgs func(ctx context.Context, env *object.Environment) (start, stop, step int64, errObj object.Object, ok bool)
}

func newCompSource(iterable ast.Expression, variables []ast.Expression, additional int) compSource {
	src := compSource{iterable: compileExpr(iterable), variables: variables}
	if ident := comprehensionFastVar(variables, additional); ident != nil {
		src.fastIdent = ident
		src.slotIndex = map[string]int{ident.Value(): 0}
		src.slotNames = []string{ident.Value()}
	}
	if call, ok := iterable.(*ast.CallExpression); ok && !call.HasOverflow() && len(call.Arguments) >= 1 && len(call.Arguments) <= 3 {
		if fn, ok := call.Function.(*ast.Identifier); ok && fn.Value() == "range" {
			args := make([]object.EvalFn, len(call.Arguments))
			for i, arg := range call.Arguments {
				args[i] = compileExpr(arg)
			}
			src.rangeArgs = compileRangeArgs(args)
		}
	}
	return src
}

// run iterates the source. sized, when not nil, receives the element count
// as soon as it is known so the caller can pre-size its result. body runs
// once per element with the comprehension environment bound; a non-nil
// body result (an error or raised exception) stops the iteration and is
// returned.
func (s *compSource) run(ctx context.Context, env *object.Environment, sized func(int), body func(compEnv *object.Environment) object.Object) object.Object {
	var compEnv *object.Environment
	var step func(element object.Object) object.Object
	if s.fastIdent != nil {
		compEnv = object.NewEnclosedEnvironmentWithSlots(env, s.slotIndex, s.slotNames)
		step = func(element object.Object) object.Object {
			compEnv.SetSlotByIndex(0, element)
			return body(compEnv)
		}
	} else {
		compEnv = object.NewEnclosedEnvironment(env)
		step = func(element object.Object) object.Object {
			if err := setForVariables(s.variables, element, compEnv); err != nil {
				return errors.NewError("%s", err.Error())
			}
			return body(compEnv)
		}
	}

	if s.rangeArgs != nil {
		// A script may rebind range; the fast path applies only to the builtin.
		if _, shadowed := env.Get("range"); !shadowed {
			start, stop, inc, errObj, ok := s.rangeArgs(ctx, env)
			if errObj != nil {
				return errObj
			}
			if ok {
				if sized != nil {
					if n := rangeLen(start, stop, inc); n > 0 {
						sized(n)
					}
				}
				for i := start; (inc > 0 && i < stop) || (inc < 0 && i > stop); i += inc {
					if out := step(object.NewInteger(i)); out != nil {
						return out
					}
				}
				return nil
			}
		}
	}

	iterableVal := s.iterable(ctx, env)
	if propagates(iterableVal) {
		return iterableVal
	}
	if sized != nil {
		switch it := iterableVal.(type) {
		case *object.List:
			sized(len(it.Elements))
		case *object.Tuple:
			sized(len(it.Elements))
		case *object.Set:
			sized(len(it.Elements))
		}
	}
	return iterateObject(ctx, iterableVal, step)
}

// rangeLen is the number of integers range(start, stop, step) produces.
func rangeLen(start, stop, step int64) int {
	if step > 0 && stop > start {
		return int((stop - start + step - 1) / step)
	}
	if step < 0 && start > stop {
		return int((start - stop - step - 1) / (-step))
	}
	return 0
}

func compileListComprehension(n *ast.ListComprehension) object.EvalFn {
	expr := compileExpr(n.Expression)
	var cond object.EvalFn
	if n.Condition != nil {
		cond = compileExpr(n.Condition)
	}
	clauses := compiledClauses(n.AdditionalClauses)
	src := newCompSource(n.Iterable, n.Variables, len(n.AdditionalClauses))

	return func(ctx context.Context, env *object.Environment) object.Object {
		result := []object.Object{}
		runBody := func(compEnv *object.Environment) object.Object {
			if cond != nil {
				c := cond(ctx, compEnv)
				if propagates(c) {
					return c
				}
				truthy, errObj := evalTruthy(ctx, c, compEnv)
				if errObj != nil {
					return errObj
				}
				if !truthy {
					return nil
				}
			}
			emit := func() object.Object {
				v := expr(ctx, compEnv)
				if propagates(v) {
					return v
				}
				result = append(result, v)
				return nil
			}
			if len(clauses) > 0 {
				return evalCompiledAdditionalClauses(ctx, clauses, 0, compEnv, emit)
			}
			return emit()
		}
		if err := src.run(ctx, env, func(size int) { result = make([]object.Object, 0, size) }, runBody); err != nil {
			return err
		}
		return &object.List{Elements: result}
	}
}

func compileDictComprehension(n *ast.DictComprehension) object.EvalFn {
	keyFn := compileExpr(n.Key)
	valueFn := compileExpr(n.Value)
	var cond object.EvalFn
	if n.Condition != nil {
		cond = compileExpr(n.Condition)
	}
	clauses := compiledClauses(n.AdditionalClauses)
	src := newCompSource(n.Iterable, n.Variables, len(n.AdditionalClauses))

	return func(ctx context.Context, env *object.Environment) object.Object {
		result := &object.Dict{Pairs: make(map[string]object.DictPair)}
		runBody := func(compEnv *object.Environment) object.Object {
			if cond != nil {
				c := cond(ctx, compEnv)
				if propagates(c) {
					return c
				}
				truthy, errObj := evalTruthy(ctx, c, compEnv)
				if errObj != nil {
					return errObj
				}
				if !truthy {
					return nil
				}
			}
			emit := func() object.Object {
				k := keyFn(ctx, compEnv)
				if propagates(k) {
					return k
				}
				v := valueFn(ctx, compEnv)
				if propagates(v) {
					return v
				}
				hk, rerr := evalHashKeyChecked(ctx, k)
				if rerr != nil {
					return rerr
				}
				result.Pairs[hk] = object.DictPair{Key: k, Value: v}
				return nil
			}
			if len(clauses) > 0 {
				return evalCompiledAdditionalClauses(ctx, clauses, 0, compEnv, emit)
			}
			return emit()
		}
		if err := src.run(ctx, env, func(size int) { result.Pairs = make(map[string]object.DictPair, size) }, runBody); err != nil {
			return err
		}
		return result
	}
}

func compileSetComprehension(n *ast.SetComprehension) object.EvalFn {
	expr := compileExpr(n.Expression)
	var cond object.EvalFn
	if n.Condition != nil {
		cond = compileExpr(n.Condition)
	}
	clauses := compiledClauses(n.AdditionalClauses)
	src := newCompSource(n.Iterable, n.Variables, len(n.AdditionalClauses))

	return func(ctx context.Context, env *object.Environment) object.Object {
		result := object.NewSet()
		runBody := func(compEnv *object.Environment) object.Object {
			if cond != nil {
				c := cond(ctx, compEnv)
				if propagates(c) {
					return c
				}
				truthy, errObj := evalTruthy(ctx, c, compEnv)
				if errObj != nil {
					return errObj
				}
				if !truthy {
					return nil
				}
			}
			emit := func() object.Object {
				v := expr(ctx, compEnv)
				if propagates(v) {
					return v
				}
				return evalSetAdd(ctx, result, v)
			}
			if len(clauses) > 0 {
				return evalCompiledAdditionalClauses(ctx, clauses, 0, compEnv, emit)
			}
			return emit()
		}
		if err := src.run(ctx, env, nil, runBody); err != nil {
			return err
		}
		return result
	}
}

func compileWalrus(n *ast.WalrusExpression) object.EvalFn {
	value := compileExpr(n.Value)
	return func(ctx context.Context, env *object.Environment) object.Object {
		val := value(ctx, env)
		if propagates(val) {
			return val
		}
		// Bind through the same path as assignment statements so slot caching,
		// global/nonlocal directives, and store fallbacks all behave identically.
		if err := assignToExpression(ctx, n.Target, val, env); err != nil {
			return assignErrorToObject(err)
		}
		return val
	}
}

func compileDel(n *ast.DelStatement) object.EvalFn {
	return func(ctx context.Context, env *object.Environment) object.Object {
		if err := deleteFromExpression(ctx, n.Target, env); err != nil {
			return assignErrorToObject(err)
		}
		for _, target := range n.ExtraTargets {
			if err := deleteFromExpression(ctx, target, env); err != nil {
				return assignErrorToObject(err)
			}
		}
		return NULL
	}
}

func compileAssert(n *ast.AssertStatement) object.EvalFn {
	condition := compileExpr(n.Condition)
	var message object.EvalFn
	if n.Message != nil {
		message = compileExpr(n.Message)
	}
	return func(ctx context.Context, env *object.Environment) object.Object {
		conditionVal := condition(ctx, env)
		if propagates(conditionVal) {
			return conditionVal
		}

		condTruthy, errObj := evalTruthy(ctx, conditionVal, env)
		if errObj != nil {
			return errObj
		}
		if !condTruthy {
			var msg string
			if message != nil {
				msgVal := message(ctx, env)
				if propagates(msgVal) {
					return msgVal
				}
				msg = msgVal.Inspect()
			} else {
				msg = "AssertionError"
			}
			return &object.Error{Message: fmt.Sprintf("AssertionError at line %d: %s", n.Token.Line, msg)}
		}
		return NULL
	}
}

func compileRaise(n *ast.RaiseStatement) object.EvalFn {
	var message object.EvalFn
	if n.Message != nil {
		message = compileExpr(n.Message)
	}
	return func(ctx context.Context, env *object.Environment) object.Object {
		if message != nil {
			msg := message(ctx, env)
			if object.IsError(msg) {
				return msg
			}
			// If it's already an Exception, mark it as actively propagating and
			// return it. This is the point where a constructed exception value
			// becomes a raise.
			if exc, ok := msg.(*object.Exception); ok {
				exc.Raised = true
				return exc
			}
			// Python allows `raise ValueError` — the class without a call: the
			// exception is instantiated with no arguments. Recognized by the raise
			// operand being an identifier naming a builtin exception constructor
			// (a shadowed name evaluates to the user's object above, so this only
			// fires when the builtin itself was raised).
			if _, isBuiltin := msg.(*object.Builtin); isBuiltin {
				if ident, ok := n.Message.(*ast.Identifier); ok {
					if name := ident.Value(); isExceptionConstructorName(name) {
						if entry, ok := builtins[name]; ok {
							res := entry.Fn(SetEnvInContext(ctx, env), object.NewKwargs(nil))
							if exc, ok := res.(*object.Exception); ok {
								exc.Raised = true
								return exc
							}
						}
					}
				}
			}
			// Python 3 doesn't support raise "string", only raise Exception("string")
			return errors.NewError("exceptions must derive from BaseException")
		}

		// Bare raise - re-raise the current exception if one exists
		if currentExc, ok := env.Get("__current_exception__"); ok {
			return markRaised(currentExc)
		}

		// No current exception - error
		return errors.NewError("No active exception to re-raise")
	}
}

func compileWith(n *ast.WithStatement) object.EvalFn {
	contextExpr := compileExpr(n.ContextExpr)
	body := compileStmt(n.Body)
	// Errors from the body take the body node's own line when they have none.
	bodyLine := n.Body.Line()
	return func(ctx context.Context, env *object.Environment) object.Object {
		// Evaluate the context expression
		ctxObj := contextExpr(ctx, env)
		if propagates(ctxObj) {
			return ctxObj
		}

		// Call __enter__
		var enterResult object.Object
		if inst, ok := ctxObj.(*object.Instance); ok {
			enterResult = callDunderMethod(ctx, inst, "__enter__", nil, env)
			if enterResult == nil {
				enterResult = NULL
			}
			if propagates(enterResult) {
				return enterResult
			}
		} else {
			return errors.NewError("with statement requires an object with __enter__ and __exit__ methods")
		}

		// Bind 'as' target if present
		if n.Target != nil {
			env.Set(n.Target.Value(), enterResult)
		}

		// Execute body
		result := fixErrorPos(ctx, body(ctx, env), bodyLine)

		// Call __exit__ — always, even on exception
		// __exit__(exc_type, exc_val, exc_tb) — pass None, None, None on success
		// or exception info on error. If __exit__ returns truthy, suppress the exception.
		inst := ctxObj.(*object.Instance)
		var excType object.Object = NULL
		var excVal object.Object = NULL
		if isRaised(result) || object.IsError(result) {
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
		// A raise (or internal error) from __exit__ itself always propagates and
		// wins over the body's in-flight exception, matching Python (where the
		// __exit__ exception replaces/chains the original). This must be checked
		// before the truthy-suppression test below, since a raised exception is
		// itself truthy and would otherwise be mistaken for "suppress".
		if exitResult != nil && (propagates(exitResult)) {
			return exitResult
		}

		// If the body raised and __exit__ returned a truthy value, suppress the
		// exception (Python's __exit__ contract). A raise from the __exit__
		// result's own __bool__ propagates rather than being coerced.
		if (isRaised(result) || object.IsError(result)) && exitResult != nil {
			suppress, serr := evalTruthy(ctx, exitResult, env)
			if serr != nil {
				return serr
			}
			if suppress {
				return NULL
			}
		}

		return result
	}
}

func compileTry(n *ast.TryStatement) object.EvalFn {
	body := compileStmt(n.Body)
	bodyLine := n.Body.Line()
	exceptBodies := make([]object.EvalFn, len(n.ExceptClauses))
	exceptLines := make([]int, len(n.ExceptClauses))
	for i, exceptClause := range n.ExceptClauses {
		exceptBodies[i] = compileStmt(exceptClause.Body)
		exceptLines[i] = exceptClause.Body.Line()
	}
	var elseFn object.EvalFn
	elseLine := 0
	if n.Else != nil {
		elseFn = compileStmt(n.Else)
		elseLine = n.Else.Line()
	}
	var finallyFn object.EvalFn
	finallyLine := 0
	if n.Finally != nil {
		finallyFn = compileStmt(n.Finally)
		finallyLine = n.Finally.Line()
	}

	// runBlock evaluates one of the compiled blocks, giving errors without a
	// position the block's line.
	runBlock := func(ctx context.Context, env *object.Environment, fn object.EvalFn, line int) object.Object {
		return fixErrorPos(ctx, fn(ctx, env), line)
	}

	return func(ctx context.Context, env *object.Environment) object.Object {
		// Execute try block
		result := runBlock(ctx, env, body, bodyLine)

		exceptionCaught := false

		// Check if a raised exception or an internal error occurred. A bare
		// exception *value* (Raised == false) is not a raise and must not be
		// caught here — only genuinely propagating exceptions are.
		if isRaised(result) || object.IsError(result) {
			// SystemExit exceptions should NOT be caught by except blocks
			// sys.exit() always exits the program, regardless of try/except
			// PermissionError exceptions also bypass try/except — security violations
			// must not be silently swallowed by scripts.
			if exc, ok := result.(*object.Exception); ok && (exc.IsSystemExit() || exc.IsPermissionError()) {
				// Execute finally block before propagating. The protected
				// exception wins over whatever the finally block does.
				if finallyFn != nil {
					result = applyProtectedFinallyResult(result, runBlock(ctx, env, finallyFn, finallyLine))
				}
				return result // always propagates
			}

			// Convert Error to Exception for consistent handling (do this once, before matching)
			var exceptionObj object.Object = result
			if err, ok := result.(*object.Error); ok {
				exceptionObj = &object.Exception{
					Message:       err.Message,
					ExceptionType: errorExceptionType(err),
					Raised:        true,
				}
			}

			// Try each except clause in order
			for i, exceptClause := range n.ExceptClauses {
				// Check if exception type matches (if specified)
				if exceptClause.ExceptType != nil {
					// Python evaluates the except-type expression. Names and dotted
					// names are matched structurally, but any other sub-expression
					// (e.g. a call `except (boom(), ValueError):`) must be evaluated
					// so its raise propagates rather than being silently ignored.
					if raised := evalExceptTypeSideEffects(ctx, exceptClause.ExceptType, env); raised != nil {
						return raised
					}
					if !matchesExceptionType(exceptionObj, exceptClause.ExceptType, env) {
						// Exception type doesn't match, try next except clause
						continue
					}
				}

				// This except clause matches - execute it
				exceptionCaught = true
				// The exception is now caught: it becomes an ordinary value that the
				// handler can inspect (str(e), e.args, re-raise via `raise`). Clear
				// the Raised flag so using it as a value inside the except body does
				// not spuriously unwind the stack. A bare `raise` re-marks it.
				if exc, ok := exceptionObj.(*object.Exception); ok {
					exc.Raised = false
				}
				// Store the current exception for bare raise support
				env.Set("__current_exception__", exceptionObj)

				// Bind exception to variable if specified
				if exceptClause.ExceptVar != nil {
					env.Set(exceptClause.ExceptVar.Value(), exceptionObj)
				}

				// Execute except block in the same environment so variables are accessible
				result = runBlock(ctx, env, exceptBodies[i], exceptLines[i])

				// Clear the current exception after except block
				env.Delete("__current_exception__")

				// If except block didn't re-raise, the exception was handled.
				// Preserve control-flow signals (return, break, continue) so they
				// propagate correctly out of the try/except.
				if !isRaised(result) && !object.IsError(result) {
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
		if elseFn != nil && !exceptionCaught && !isRaised(result) && !object.IsError(result) {
			switch result.(type) {
			case *object.ReturnValue, *object.Break, *object.Continue:
				// don't run else on control flow
			default:
				result = runBlock(ctx, env, elseFn, elseLine)
			}
		}

		// Always execute finally block if present
		// Per Python semantics, return in finally overrides the result, and an
		// exception raised in finally replaces whatever was in flight — unless
		// the in-flight result is a protected exception (SystemExit,
		// PermissionError), which nothing may replace, whichever block raised it.
		if finallyFn != nil {
			if exc, ok := result.(*object.Exception); ok && (exc.IsSystemExit() || exc.IsPermissionError()) {
				result = applyProtectedFinallyResult(result, runBlock(ctx, env, finallyFn, finallyLine))
			} else {
				result = applyFinallyResult(result, runBlock(ctx, env, finallyFn, finallyLine))
			}
		}

		return result
	}
}

func compileMatch(n *ast.MatchStatement) object.EvalFn {
	subject := compileExpr(n.Subject)
	guards := make([]object.EvalFn, len(n.Cases))
	bodies := make([]object.EvalFn, len(n.Cases))
	bodyLines := make([]int, len(n.Cases))
	for i, caseClause := range n.Cases {
		if caseClause.Guard != nil {
			guards[i] = compileExpr(caseClause.Guard)
		}
		bodies[i] = compileStmt(caseClause.Body)
		bodyLines[i] = caseClause.Body.Line()
	}

	return func(ctx context.Context, env *object.Environment) object.Object {
		subjectVal := subject(ctx, env)
		if propagates(subjectVal) {
			return subjectVal
		}

		for i, caseClause := range n.Cases {
			// Track captured variables for this case
			capturedVars := make(map[string]object.Object)

			matched, capturedValue := matchPattern(ctx, subjectVal, caseClause.Pattern, capturedVars, env)
			if propagates(matched) {
				return matched
			}

			if matched == TRUE {
				// Temporarily add captured variables to environment for guard evaluation
				for name, val := range capturedVars {
					env.Set(name, val)
				}

				// Check guard condition if present
				if guards[i] != nil {
					guardResult := guards[i](ctx, env)
					if propagates(guardResult) {
						return guardResult
					}
					guardTruthy, gErr := evalTruthy(ctx, guardResult, env)
					if gErr != nil {
						return gErr
					}
					if !guardTruthy {
						// Guard failed - try next case
						continue
					}
				}

				// Bind explicit capture variable if present
				if caseClause.CaptureAs != nil {
					env.Set(caseClause.CaptureAs.Value(), capturedValue)
				}

				// Execute body in the environment (with captures)
				return fixErrorPos(ctx, bodies[i](ctx, env), bodyLines[i])
			}
		}

		return NULL
	}
}

// compileClass converts a class definition. The class body's statements are
// compiled once; a def inside the body runs through its compiled closure,
// which returns the (decorated) function so it can be registered as a method.
func compileClass(n *ast.ClassStatement) object.EvalFn {
	bodyStmts := make([]object.EvalFn, len(n.Body.Statements))
	for i, s := range n.Body.Statements {
		bodyStmts[i] = compileStmt(s)
	}
	decorators := make([]object.EvalFn, len(n.GetDecorators()))
	for i, d := range n.GetDecorators() {
		decorators[i] = compileExpr(d)
	}
	var baseClassFn object.EvalFn
	if n.BaseClass != nil {
		baseClassFn = compileExpr(n.BaseClass)
	}
	name := n.Name.Value()

	return func(ctx context.Context, env *object.Environment) object.Object {
		class := &object.Class{
			Name:    name,
			Methods: make(map[string]object.Object),
			Env:     env,
		}

		// Handle base class inheritance
		if baseClassFn != nil {
			// Evaluate the base class expression (can be dotted like html.parser.HTMLParser)
			baseClassObj := baseClassFn(ctx, env)
			if propagates(baseClassObj) {
				return baseClassObj
			}
			baseClass, ok := baseClassObj.(*object.Class)
			if !ok {
				return errors.NewError("base class is not a class type, got %s", baseClassObj.Type())
			}
			class.BaseClass = baseClass

			// Copy methods from base class
			for mname, method := range baseClass.Methods {
				class.Methods[mname] = method
			}
		}

		// Create a new environment for the class body
		classEnv := object.NewEnclosedEnvironment(env)
		classEnv.Set("__class__", class)

		// Execute the entire class body in the class environment. Method
		// definitions are collected into class.Methods (overriding inherited ones);
		// every other statement runs for its effect, and any error or raised
		// exception it produces propagates out of the class definition (so a
		// failing attribute expression or a security violation in the body is not
		// silently swallowed). After the body runs, the names it bound in the class
		// environment become class attributes.
		for i, s := range n.Body.Statements {
			if fnStmt, ok := s.(*ast.FunctionStatement); ok {
				// The compiled def closure builds the function, applies its
				// decorators and binds the name in classEnv, then returns the
				// result so it can be registered as a method.
				obj := bodyStmts[i](ctx, classEnv)
				if propagates(obj) {
					return obj
				}
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
					if obj != nil {
						class.Methods[fnStmt.Name.Value()] = obj
					}
				}
				continue
			}
			if res := bodyStmts[i](ctx, classEnv); propagates(res) {
				return res
			}
		}

		// Promote class-body bindings (e.g. `x = 5`) to class attributes. Skip the
		// synthetic __class__ marker and anything already registered as a method.
		classEnv.EachLocal(func(mname string, val object.Object) {
			if mname == "__class__" {
				return
			}
			if _, isMethod := class.Methods[mname]; isMethod {
				return
			}
			class.Methods[mname] = val
		})

		env.Set(name, class)
		var result object.Object = class
		for i := len(decorators) - 1; i >= 0; i-- {
			dec := decorators[i](ctx, env)
			if propagates(dec) {
				return dec
			}
			result = applyFunctionWithContext(ctx, dec, []object.Object{result}, nil, env)
			if propagates(result) {
				return result
			}
		}
		if result != class {
			env.Set(name, result)
		}
		return result
	}
}
