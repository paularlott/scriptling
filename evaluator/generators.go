package evaluator

import (
	"context"

	"github.com/paularlott/scriptling/ast"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// Phase-1 generators. A generator function (a def whose body contains a
// yield statement) compiles to a flat list of resumable items — statements,
// yield boundaries, and loop control (init/pull/cond/end with explicit jump
// targets) — and the generator's __next__ runs items until the next yield,
// carrying the cursor between calls. Variables live in one environment for
// the generator's lifetime, so locals persist across yields as in Python.
//
// Phase-1 placement rule: yield statements are supported at the top level of
// the generator body and directly in the immediate body of a top-level
// for/while loop. Deeper yields (inside if/try/nested loops) are a clear
// error at definition time; the filter idiom rewrites as a continue-guard
// (if not p(x): continue / yield ...).

const (
	genRun = iota
	genYield
	genForInit // fn = iterable expression, targets = loop variables
	genForNext // targets = loop variables; jump past the loop when exhausted
	genWhile   // fn = condition; jump past the loop when false
	genLoopEnd // jump back to the loop's pull/cond item
)

type genItem struct {
	kind    int
	fn      object.EvalFn    // statement, yield value, iterable, condition
	targets []ast.Expression // for-loop binding
	brk     int              // run: cursor target for break (-1 = top level, -3 = patch me)
	cont    int              // run: cursor target for continue
	jump    int              // loop control: target index
}

// genPlan is a generator function's compiled shape plus its validation
// error, surfaced when the def executes.
type genPlan struct {
	items   []genItem
	invalid string // non-empty: phase-1 placement violation
}

// buildGeneratorPlan compiles a generator body into resumable items.
func buildGeneratorPlan(body *ast.BlockStatement) *genPlan {
	plan := &genPlan{}

	// validate reports the first yield that is not at a legal phase-1
	// position: yields may sit in the function body's top-level statement
	// list, or in the immediate statement list of a top-level for/while.
	// legalHere: yields in this list are legal; inLoopBody: this list is the
	// immediate body of the function's top-level loop.
	var validate func(stmts []ast.Statement, legalHere, inLoopBody bool) string
	validate = func(stmts []ast.Statement, legalHere, inLoopBody bool) string {
		for _, s := range stmts {
			switch st := s.(type) {
			case *ast.YieldStatement:
				if !legalHere {
					return "a nested position (keep yields at the top level of the function body or of its top-level loop)"
				}
			case *ast.IfStatement:
				for _, cl := range st.ElifClauses {
					if msg := validate(cl.Consequence.Statements, false, false); msg != "" {
						return msg
					}
				}
				if st.Consequence != nil {
					if msg := validate(st.Consequence.Statements, false, false); msg != "" {
						return msg
					}
				}
				if st.Alternative != nil {
					if msg := validate(st.Alternative.Statements, false, false); msg != "" {
						return msg
					}
				}
			case *ast.ForStatement:
				if legalHere && !inLoopBody {
					if msg := validate(st.Body.Statements, true, true); msg != "" {
						return msg
					}
				} else if msg := validate(st.Body.Statements, false, false); msg != "" {
					return msg
				}
			case *ast.WhileStatement:
				if legalHere && !inLoopBody {
					if msg := validate(st.Body.Statements, true, true); msg != "" {
						return msg
					}
				} else if msg := validate(st.Body.Statements, false, false); msg != "" {
					return msg
				}
			case *ast.TryStatement:
				if msg := validate(st.Body.Statements, false, false); msg != "" {
					return msg
				}
				for _, ec := range st.ExceptClauses {
					if msg := validate(ec.Body.Statements, false, false); msg != "" {
						return msg
					}
				}
			}
		}
		return ""
	}

	// patchBreak fills the -3 break placeholder once the loop's extent is
	// known, and returns the index just past the loop.
	patchBreak := func(start int) int {
		end := len(plan.items)
		for i := start; i < end; i++ {
			if plan.items[i].brk == -3 {
				plan.items[i].brk = end
			}
		}
		return end
	}

	var buildList func(stmts []ast.Statement, brk, cont int)
	buildList = func(stmts []ast.Statement, brk, cont int) {
		for _, s := range stmts {
			switch st := s.(type) {
			case *ast.YieldStatement:
				item := genItem{kind: genYield}
				if st.Value != nil {
					item.fn = compileExpr(st.Value)
				}
				plan.items = append(plan.items, item)
			case *ast.ForStatement:
				if containsYield(st.Body) {
					plan.items = append(plan.items, genItem{kind: genForInit, fn: compileExpr(st.Iterable), targets: st.Variables})
					nextIdx := len(plan.items)
					plan.items = append(plan.items, genItem{kind: genForNext, targets: st.Variables})
					bodyStart := len(plan.items)
					buildList(st.Body.Statements, -3, nextIdx)
					plan.items = append(plan.items, genItem{kind: genLoopEnd, jump: nextIdx})
					end := patchBreak(bodyStart)
					plan.items[nextIdx].jump = end
				} else {
					plan.items = append(plan.items, genItem{kind: genRun, fn: compileStmt(s), brk: brk, cont: cont})
				}
			case *ast.WhileStatement:
				if containsYield(st.Body) {
					condIdx := len(plan.items)
					plan.items = append(plan.items, genItem{kind: genWhile, fn: compileExpr(st.Condition)})
					bodyStart := len(plan.items)
					buildList(st.Body.Statements, -3, condIdx)
					plan.items = append(plan.items, genItem{kind: genLoopEnd, jump: condIdx})
					end := patchBreak(bodyStart)
					plan.items[condIdx].jump = end
				} else {
					plan.items = append(plan.items, genItem{kind: genRun, fn: compileStmt(s), brk: brk, cont: cont})
				}
			default:
				plan.items = append(plan.items, genItem{kind: genRun, fn: compileStmt(s), brk: brk, cont: cont})
			}
		}
	}

	if msg := validate(body.Statements, true, false); msg != "" {
		plan.invalid = msg
	}
	buildList(body.Statements, -1, -1)
	return plan
}

// containsYield reports whether a block contains a yield statement, looking
// through control flow but not into nested function definitions.
func containsYield(body *ast.BlockStatement) bool {
	if body == nil {
		return false
	}
	for _, s := range body.Statements {
		switch st := s.(type) {
		case *ast.YieldStatement:
			return true
		case *ast.IfStatement:
			if containsYield(st.Consequence) {
				return true
			}
			for _, cl := range st.ElifClauses {
				if containsYield(cl.Consequence) {
					return true
				}
			}
			if containsYield(st.Alternative) {
				return true
			}
		case *ast.ForStatement:
			if containsYield(st.Body) {
				return true
			}
		case *ast.WhileStatement:
			if containsYield(st.Body) {
				return true
			}
		case *ast.TryStatement:
			if containsYield(st.Body) {
				return true
			}
			for _, ec := range st.ExceptClauses {
				if containsYield(ec.Body) {
					return true
				}
			}
			if containsYield(st.Else) || containsYield(st.Finally) {
				return true
			}
		}
	}
	return false
}

// generatorClass is the shared class of generator objects; driver state
// lives in NativeData. Built lazily to break an initialization cycle with
// the evaluator's function-application machinery.
var generatorClass *object.Class

func init() {
	generatorClass = &object.Class{
	Name: "generator",
	Methods: map[string]object.Object{
		"__iter__": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				return args[0]
			},
			HelpText: "__iter__() - Generators are their own iterator",
		},
		"__next__": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) < 1 {
					return errors.NewError("__next__() requires the generator")
				}
				g, ok := args[0].(*object.Instance)
				if !ok {
					return errors.NewError("__next__() must be called on a generator")
				}
				return generatorNext(ctx, g, GetEnvFromContext(ctx))
			},
			HelpText: "__next__() - Resume the generator to the next yield",
		},
		"send": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				return &object.Exception{
					Message:       "generator.send() is not supported yet (scriptling generators yield values only)",
					ExceptionType: "NotImplementedError",
					Raised:        true,
				}
			},
			HelpText: "send(value) - Not supported yet",
		},
		"close": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) < 1 {
					return errors.NewError("close() requires the generator")
				}
				g, ok := args[0].(*object.Instance)
				if !ok {
					return errors.NewError("close() must be called on a generator")
				}
				g.SetField("_done", TRUE)
				return NULL
			},
			HelpText: "close() - Mark the generator finished",
		},
	},
}
}

// genState is the driver state carried on the generator instance's
// NativeData: instance fields are object.Object-typed and cannot hold the
// plan or environment directly.
type genState struct {
	plan    *genPlan
	env     *object.Environment
	iter    *object.Iterator
	cursor  int
	started bool
	done    bool
}

// newGenerator constructs the generator object for a call to a generator
// function. Parameters bind eagerly (Python validates the call now); the
// body runs lazily from the first next().
func newGenerator(ctx context.Context, fn *object.Function, args []object.Object, keywords map[string]object.Object, env *object.Environment, plan *genPlan) object.Object {
	callEnv, errObj := extendFunctionEnv(ctx, fn, args, keywords)
	if errObj != nil {
		return errObj
	}
	g := object.NewInstance(generatorClass)
	g.SetField("_done", FALSE)
	g.NativeData = &genState{plan: plan, env: callEnv}
	return g
}

func genStopIteration() object.Object {
	return &object.Exception{Message: "", ExceptionType: object.ExceptionTypeStopIteration, Raised: true}
}

// generatorNext resumes the generator until the next yield value.
func generatorNext(ctx context.Context, g *object.Instance, env *object.Environment) object.Object {
	if done, _ := g.Field("_done").(*object.Boolean); done != nil && done.BoolValue() {
		return genStopIteration()
	}
	st, ok := g.NativeData.(*genState)
	if !ok || st == nil {
		return errors.NewError("generator has no driver state")
	}
	finish := func() {
		st.done = true
		g.SetField("_done", TRUE)
	}
	if st.done {
		return genStopIteration()
	}
	genv := st.env

	for {
		idx := st.cursor
		if idx >= len(st.plan.items) {
			finish()
			return genStopIteration()
		}
		it := st.plan.items[idx]
		switch it.kind {
		case genRun:
			st.cursor = idx + 1
			r := it.fn(ctx, genv)
			switch r.(type) {
			case *object.Error, *object.Exception:
				finish()
				return r
			case *object.ReturnValue:
				finish()
				return genStopIteration()
			case *object.Break:
				if it.brk < 0 {
					finish()
					return errors.NewError("'break' outside loop")
				}
				st.cursor = it.brk
			case *object.Continue:
				if it.cont < 0 {
					finish()
					return errors.NewError("'continue' not properly in loop")
				}
				st.cursor = it.cont
			}
		case genYield:
			st.cursor = idx + 1
			if it.fn == nil {
				return NULL
			}
			val := it.fn(ctx, genv)
			if propagates(val) {
				finish()
				return val
			}
			return val
		case genForInit:
			st.cursor = idx + 1
			iterVal := it.fn(ctx, genv)
			if propagates(iterVal) {
				finish()
				return iterVal
			}
			iter, errObj := generatorIterFor(ctx, iterVal, env)
			if errObj != nil {
				finish()
				return errObj
			}
			st.iter = iter
		case genForNext:
			st.cursor = idx + 1
			if st.iter == nil {
				finish()
				return errors.NewError("generator loop has no iterator")
			}
			elem, hasNext := st.iter.Next()
			if !hasNext {
				st.iter = nil
				st.cursor = it.jump
				continue
			}
			if err := setForVariables(it.targets, elem, genv); err != nil {
				finish()
				return errors.NewError("%s", err.Error())
			}
		case genWhile:
			st.cursor = idx + 1
			c := it.fn(ctx, genv)
			if propagates(c) {
				finish()
				return c
			}
			truthy, errObj := evalTruthy(ctx, c, genv)
			if errObj != nil {
				finish()
				return errObj
			}
			if !truthy {
				st.cursor = it.jump
			}
		case genLoopEnd:
			st.cursor = it.jump
		default:
			finish()
			return errors.NewError("generator plan has an unknown item")
		}
	}
}

// generatorIterFor builds the iterator for a generator's for-loop, accepting
// every iterable a normal for-loop accepts.
func generatorIterFor(ctx context.Context, val object.Object, env *object.Environment) (*object.Iterator, object.Object) {
	switch o := val.(type) {
	case *object.Iterator:
		return o, nil
	case *object.List:
		return sliceIterator(o.Elements), nil
	case *object.Tuple:
		return sliceIterator(o.Elements), nil
	case *object.String:
		elements := make([]object.Object, 0, len(o.StringValue()))
		for _, ch := range o.StringValue() {
			elements = append(elements, object.NewString(string(ch)))
		}
		return sliceIterator(elements), nil
	case *object.Bytes:
		elements := make([]object.Object, len(o.BytesValue()))
		for i, b := range o.BytesValue() {
			elements[i] = object.NewInteger(int64(b))
		}
		return sliceIterator(elements), nil
	case *object.Set:
		return o.CreateIterator(), nil
	case *object.Dict:
		return o.CreateIterator(), nil
	case *object.DictKeys:
		return o.CreateIterator(), nil
	case *object.DictValues:
		return o.CreateIterator(), nil
	case *object.DictItems:
		return o.CreateIterator(), nil
	case *object.FloatArray:
		return sliceIterator(o.ToList().Elements), nil
	case *object.Instance:
		if fn, ok := findDunderMethod(o, "__iter__"); ok {
			return callIter(ctx, o, fn, env)
		}
	}
	return nil, errors.NewTypeError("iterable", val.Type().String())
}

// sliceIterator adapts a materialised slice to the iterator protocol.
func sliceIterator(elements []object.Object) *object.Iterator {
	i := 0
	return object.NewIterator(func() (object.Object, bool) {
		if i >= len(elements) {
			return nil, false
		}
		v := elements[i]
		i++
		return v, true
	})
}
