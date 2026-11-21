package evaluator

import (
	"context"
	"runtime"
	"time"
	"unsafe"

	"github.com/paularlott/scriptling/ast"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// Generators. A generator function (a def whose body contains a yield)
// compiles to a flat list of resumable items — statements, yield boundaries,
// conditional and unconditional jumps, and for-loop iterator steps — and the
// generator's __next__ runs items until the next yield, carrying the cursor
// between calls. Variables live in one environment for the generator's
// lifetime, so locals persist across yields as in Python.
//
// Placement rule: yields may sit anywhere in the body's statement structure:
// if/elif/else, for/while (with else clauses), try/except/else/finally, with
// and match. A compound statement with no yield inside runs as one compiled
// statement; one containing a yield is flattened into items.
//
// try and with blocks flattened this way are regions: each item records the
// innermost region it sits in, and regions record their enclosing region, so
// the handler for an exception (or the cleanup a return, break or continue
// must run on its way out) is found statically from the item that raised it.
// An exception unwinds outward region by region (unwind): a try body tries its
// except clauses, a finally body is entered with the completion parked until
// its end, a with block calls __exit__. close() and throw() unwind from the
// paused yield the same way. A generator with no regions takes none of these
// paths.
//
// A yield is a statement or the whole value of an assignment (x = yield v):
// the assignment is a separate item run on resume, binding the value send()
// delivered (None for next()). `yield from it` is one item the cursor stays
// on while the delegate produces values; a delegated generator is resumed
// directly, so send(), throw(), close() and its return value pass through.

const (
	genRun = iota
	genYield
	genRecv       // target = assignment target for the value sent to the yield before
	genYieldFrom  // fn = iterable expression; target = optional result binding
	genForInit    // fn = iterable expression, slot = loop depth
	genForNext    // targets = loop variables; jump past the loop body when exhausted
	genCond       // fn = condition; jump when false (while, if, elif)
	genJump       // unconditional jump
	genWithEnter  // fn = context expression, target = optional as-binding, slot = manager slot
	genWithExit   // slot = manager slot: __exit__(None, None, None) on leaving normally
	genMatch      // match: sel picks the case; jump = past the statement when none matches
	genExceptEnd  // end of an except clause body: clear the handled exception, jump
	genFinallyEnd // end of a finally body: resume the parked completion (slot)
)

type genItem struct {
	kind    int32
	region  int32            // innermost try/with region (index+1 into plan.regions; 0 = none)
	fn      object.EvalFn    // statement, yield value, iterable, condition
	targets []ast.Expression // for-loop binding
	target  ast.Expression   // genRecv / genYieldFrom / genWithEnter binding
	brk     int              // run: cursor target for break (-1 = top level, -3 = patch me)
	cont    int              // run: cursor target for continue
	jump    int              // loop control: target index
	slot    int              // for-loop nesting depth / try or with slot
	match   *genMatchItem    // genMatch only
}

type genMatchItem struct {
	sel    func(ctx context.Context, env *object.Environment) (int, object.Object)
	starts []int // item index of each case body
}

// Region kinds.
const (
	regTry     = iota + 1 // try body: its except clauses, then its finally
	regHandler            // except clause or else body: the try's finally
	regFinally            // finally body: a completion leaving it replaces the parked one
	regWith               // with body: __exit__
)

type genRegion struct {
	kind    uint8
	except  bool  // regHandler: an except clause body (clears the handled exception)
	parent  int32 // enclosing region (0 = none)
	slot    int   // try: parked-completion slot; with: manager slot
	finally int   // try regions: finally body start, -1 = none
	end     int   // with: index after the statement (where a suppressed exception resumes)
	clauses []*ast.ExceptClause
	starts  []int // regTry: item index of each except clause body
}

// genPlan is a generator function's compiled shape.
type genPlan struct {
	items   []genItem
	regions []genRegion
	nTry    int // try statements flattened (parked-completion slots)
	nWith   int // with statements flattened (manager slots)
}

// buildGeneratorPlan compiles a generator body into resumable items.
func buildGeneratorPlan(body *ast.BlockStatement) *genPlan {
	plan := &genPlan{}
	var cur int32 // region the items being emitted sit in

	emit := func(it genItem) int {
		it.region = cur
		plan.items = append(plan.items, it)
		return len(plan.items) - 1
	}
	addRegion := func(r genRegion) int32 {
		plan.regions = append(plan.regions, r)
		return int32(len(plan.regions))
	}

	// patchBreak fills the -3 break placeholders of a loop body's items once
	// the loop's extent (including any else clause) is known.
	patchBreak := func(start, stop, target int) {
		for i := start; i < stop; i++ {
			if plan.items[i].brk == -3 {
				plan.items[i].brk = target
			}
		}
	}

	var buildList func(stmts []ast.Statement, brk, cont, depth int)
	buildBlock := func(b *ast.BlockStatement, brk, cont, depth int) {
		if b != nil {
			buildList(b.Statements, brk, cont, depth)
		}
	}
	run := func(s ast.Statement, brk, cont int) {
		emit(genItem{kind: genRun, fn: compileStmt(s), brk: brk, cont: cont})
	}
	buildList = func(stmts []ast.Statement, brk, cont, depth int) {
		for _, s := range stmts {
			if !stmtContainsYield(s) {
				run(s, brk, cont)
				continue
			}
			switch st := s.(type) {
			case *ast.YieldStatement:
				if st.From {
					emit(genItem{kind: genYieldFrom, fn: compileExpr(st.Value), target: st.Target})
					continue
				}
				item := genItem{kind: genYield}
				if st.Value != nil {
					item.fn = compileExpr(st.Value)
				}
				emit(item)
				if st.Target != nil {
					emit(genItem{kind: genRecv, target: st.Target})
				}
			case *ast.ForStatement:
				emit(genItem{kind: genForInit, fn: compileExpr(st.Iterable), slot: depth})
				nextIdx := emit(genItem{kind: genForNext, targets: st.Variables, slot: depth})
				bodyStart := len(plan.items)
				buildList(st.Body.Statements, -3, nextIdx, depth+1)
				emit(genItem{kind: genJump, jump: nextIdx})
				bodyEnd := len(plan.items)
				plan.items[nextIdx].jump = bodyEnd
				buildBlock(st.Else, brk, cont, depth)
				patchBreak(bodyStart, bodyEnd, len(plan.items))
			case *ast.WhileStatement:
				condIdx := emit(genItem{kind: genCond, fn: compileExpr(st.Condition)})
				bodyStart := len(plan.items)
				buildList(st.Body.Statements, -3, condIdx, depth)
				emit(genItem{kind: genJump, jump: condIdx})
				bodyEnd := len(plan.items)
				plan.items[condIdx].jump = bodyEnd
				buildBlock(st.Else, brk, cont, depth)
				patchBreak(bodyStart, bodyEnd, len(plan.items))
			case *ast.IfStatement:
				// Each clause: a condition jumping to the next clause when
				// false, the clause body, then a jump past the whole if.
				var toEnd []int
				clause := func(cond ast.Expression, body *ast.BlockStatement, last bool) {
					condIdx := emit(genItem{kind: genCond, fn: compileExpr(cond)})
					buildBlock(body, brk, cont, depth)
					if !last {
						toEnd = append(toEnd, emit(genItem{kind: genJump}))
					}
					plan.items[condIdx].jump = len(plan.items)
				}
				clause(st.Condition, st.Consequence, len(st.ElifClauses) == 0 && st.Alternative == nil)
				for i, cl := range st.ElifClauses {
					clause(cl.Condition, cl.Consequence, i == len(st.ElifClauses)-1 && st.Alternative == nil)
				}
				buildBlock(st.Alternative, brk, cont, depth)
				for _, j := range toEnd {
					plan.items[j].jump = len(plan.items)
				}
			case *ast.TryStatement:
				// Layout: body [else] jump->F; each clause: body, except-end
				// jump->F; F: finally body, finally-end. Without a finally F
				// is the end of the statement.
				parent := cur
				slot := plan.nTry
				plan.nTry++
				tryID := addRegion(genRegion{kind: regTry, parent: parent, slot: slot, finally: -1,
					clauses: st.ExceptClauses, starts: make([]int, len(st.ExceptClauses))})
				ids := []int32{tryID}
				cur = tryID
				buildBlock(st.Body, brk, cont, depth)
				if st.Else != nil {
					id := addRegion(genRegion{kind: regHandler, parent: parent, slot: slot, finally: -1})
					ids = append(ids, id)
					cur = id
					buildBlock(st.Else, brk, cont, depth)
				}
				cur = parent
				toF := []int{emit(genItem{kind: genJump})}
				for i, ec := range st.ExceptClauses {
					id := addRegion(genRegion{kind: regHandler, except: true, parent: parent, slot: slot, finally: -1})
					ids = append(ids, id)
					plan.regions[tryID-1].starts[i] = len(plan.items)
					cur = id
					buildBlock(ec.Body, brk, cont, depth)
					toF = append(toF, emit(genItem{kind: genExceptEnd}))
					cur = parent
				}
				f := len(plan.items)
				if st.Finally != nil {
					cur = addRegion(genRegion{kind: regFinally, parent: parent, slot: slot, finally: -1})
					buildBlock(st.Finally, brk, cont, depth)
					emit(genItem{kind: genFinallyEnd, slot: slot})
					cur = parent
					for _, id := range ids {
						plan.regions[id-1].finally = f
					}
				}
				for _, j := range toF {
					plan.items[j].jump = f
				}
			case *ast.WithStatement:
				parent := cur
				slot := plan.nWith
				plan.nWith++
				enter := genItem{kind: genWithEnter, fn: compileExpr(st.ContextExpr), slot: slot}
				if st.Target != nil {
					enter.target = st.Target
				}
				emit(enter)
				id := addRegion(genRegion{kind: regWith, parent: parent, slot: slot, finally: -1})
				cur = id
				buildBlock(st.Body, brk, cont, depth)
				cur = parent
				emit(genItem{kind: genWithExit, slot: slot})
				plan.regions[id-1].end = len(plan.items)
			case *ast.MatchStatement:
				m := &genMatchItem{sel: compileMatchSelect(st), starts: make([]int, len(st.Cases))}
				matchIdx := emit(genItem{kind: genMatch, match: m})
				var toEnd []int
				for i, cc := range st.Cases {
					m.starts[i] = len(plan.items)
					buildBlock(cc.Body, brk, cont, depth)
					toEnd = append(toEnd, emit(genItem{kind: genJump}))
				}
				for _, j := range toEnd {
					plan.items[j].jump = len(plan.items)
				}
				plan.items[matchIdx].jump = len(plan.items)
			default:
				run(s, brk, cont)
			}
		}
	}

	buildList(body.Statements, -1, -1, 0)
	return plan
}

// containsYield reports whether a block contains a yield statement, looking
// through control flow but not into nested function definitions.
func containsYield(body *ast.BlockStatement) bool {
	if body == nil {
		return false
	}
	for _, s := range body.Statements {
		if stmtContainsYield(s) {
			return true
		}
	}
	return false
}

// stmtContainsYield is containsYield for one statement.
func stmtContainsYield(s ast.Statement) bool {
	switch st := s.(type) {
	case *ast.YieldStatement:
		return true
	case *ast.IfStatement:
		if containsYield(st.Consequence) || containsYield(st.Alternative) {
			return true
		}
		for _, cl := range st.ElifClauses {
			if containsYield(cl.Consequence) {
				return true
			}
		}
	case *ast.ForStatement:
		return containsYield(st.Body) || containsYield(st.Else)
	case *ast.WhileStatement:
		return containsYield(st.Body) || containsYield(st.Else)
	case *ast.TryStatement:
		if containsYield(st.Body) || containsYield(st.Else) || containsYield(st.Finally) {
			return true
		}
		for _, ec := range st.ExceptClauses {
			if containsYield(ec.Body) {
				return true
			}
		}
	case *ast.WithStatement:
		return containsYield(st.Body)
	case *ast.MatchStatement:
		for _, cc := range st.Cases {
			if containsYield(cc.Body) {
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
					g, errObj := generatorArg(args, "send")
					if errObj != nil {
						return errObj
					}
					if len(args) != 2 {
						return errors.NewTypeErrorTagged("send() takes exactly one argument (%d given)", len(args)-1)
					}
					return generatorSend(ctx, g, args[1], GetEnvFromContext(ctx))
				},
				HelpText: "send(value) - Resume the generator, making value the result of the paused yield",
			},
			"throw": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					g, errObj := generatorArg(args, "throw")
					if errObj != nil {
						return errObj
					}
					if len(args) < 2 {
						return errors.NewTypeErrorTagged("throw expected at least 1 argument, got 0")
					}
					env := GetEnvFromContext(ctx)
					exc := toRaisedException(ctx, args[1], args[2:], env)
					return generatorThrow(ctx, g, exc, env)
				},
				HelpText: "throw(exc) - Raise exc at the paused yield and return the next value yielded",
			},
			"close": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					g, errObj := generatorArg(args, "close")
					if errObj != nil {
						return errObj
					}
					v, r := generatorCloseRun(ctx, g, GetEnvFromContext(ctx))
					if r != nil {
						return r
					}
					if v != nil {
						return v
					}
					return NULL
				},
				HelpText: "close() - Raise GeneratorExit at the paused yield, running finally blocks and __exit__, and finish the generator",
			},
		},
	}
}

func generatorArg(args []object.Object, method string) (*object.Instance, object.Object) {
	if len(args) < 1 {
		return nil, errors.NewError("%s() requires the generator", method)
	}
	g, ok := args[0].(*object.Instance)
	if !ok {
		return nil, errors.NewError("%s() must be called on a generator", method)
	}
	return g, nil
}

// genState is the driver state carried on the generator instance's
// NativeData: instance fields are object.Object-typed and cannot hold the
// plan or environment directly.
type genState struct {
	plan   *genPlan
	env    *object.Environment
	iter   *object.Iterator // the outermost for-loop's iterator
	cursor int
	done   bool

	// ext holds the state only some generators need: nested loops,
	// delegation, sent values, and try/with bookkeeping. nil until first
	// needed, so a simple generator does not carry it.
	ext *genExtra

	// depthCtx and depth cache the call-depth tracker of the context the
	// generator was last resumed under: a loop resumes it under the same
	// context every time, and finding the tracker walks the context chain.
	depthCtx context.Context
	depth    *CallDepth
}

type genExtra struct {
	iters   []*object.Iterator // nested for-loops' iterators, by depth-1
	sent    object.Object      // value send() delivers to the paused yield; nil = None
	sub     *object.Instance   // generator being delegated to by yield from
	subIter *object.Iterator   // other iterator being delegated to
	blocks  *genBlocks         // try/with bookkeeping, for generators that have them
}

// genBlocks is the state of a generator's flattened try and with blocks.
type genBlocks struct {
	pend []genCompletion    // by try slot: completion parked while its finally runs
	mgrs []*object.Instance // by with slot: the context manager of an open with
}

// blocks returns the generator's try/with state, creating it on first use.
func (st *genState) blocks() *genBlocks {
	x := st.extra()
	if x.blocks == nil {
		x.blocks = &genBlocks{pend: make([]genCompletion, st.plan.nTry), mgrs: make([]*object.Instance, st.plan.nWith)}
	}
	return x.blocks
}

// Completion kinds: how control leaves a region.
const (
	compNone = iota
	compRaise
	compReturn
	compJump // break or continue: to target, in region
)

type genCompletion struct {
	kind   int
	val    object.Object // raise: the exception; return: the value
	target int           // jump: item index
	region int32         // jump: the region the target sits in
}

func (st *genState) extra() *genExtra {
	if st.ext == nil {
		st.ext = &genExtra{}
	}
	return st.ext
}

// delegating reports whether the generator is paused inside a yield from.
func (st *genState) delegating() bool {
	return st.ext != nil && (st.ext.sub != nil || st.ext.subIter != nil)
}

// finish marks the generator exhausted.
func (st *genState) finish() {
	st.done = true
}

// pausedRegion is the region of the yield (or yield from) the generator is
// suspended at.
func (st *genState) pausedRegion() int32 {
	idx := st.cursor
	if !st.delegating() {
		idx--
	}
	if idx < 0 || idx >= len(st.plan.items) {
		return 0
	}
	return st.plan.items[idx].region
}

// fail handles err, a propagating error raised by an item in region: outside
// any region it ends the generator and is returned (stop); inside one it
// unwinds to the handler.
func (st *genState) fail(ctx context.Context, g *object.Instance, region int32, err object.Object) (object.Object, bool) {
	if region == 0 {
		st.finish()
		return err, true
	}
	return st.unwind(ctx, g, region, genCompletion{kind: compRaise, val: err})
}

// unwind carries completion c outward from region r: a try body's except
// clauses may catch an exception, a finally body is entered with c parked
// until its end, and a with block's __exit__ runs (and may suppress the
// exception). It returns stop=false when execution continues at the cursor,
// or the generator's result once c leaves the body.
func (st *genState) unwind(ctx context.Context, g *object.Instance, r int32, c genCompletion) (object.Object, bool) {
	b := st.blocks()
	for {
		if c.kind == compJump && r == c.region {
			st.cursor = c.target
			return nil, false
		}
		if r == 0 {
			break
		}
		reg := &st.plan.regions[r-1]
		switch reg.kind {
		case regTry:
			if c.kind == compRaise && len(reg.clauses) > 0 && !isProtected(c.val) {
				i, exc, raised := matchExceptClause(ctx, reg.clauses, c.val, st.env)
				if raised != nil {
					c.val = raised
				} else if i >= 0 {
					bindExceptClause(reg.clauses[i], exc, st.env)
					st.cursor = reg.starts[i]
					return nil, false
				}
			}
			if reg.finally >= 0 {
				st.park(reg.slot, c)
				st.cursor = reg.finally
				return nil, false
			}
		case regHandler:
			if reg.except {
				st.env.Delete("__current_exception__")
			}
			if reg.finally >= 0 {
				st.park(reg.slot, c)
				st.cursor = reg.finally
				return nil, false
			}
		case regFinally:
			// A completion leaving a finally body replaces the one parked
			// for it, unless that one is protected (see isProtected).
			p := b.pend[reg.slot]
			b.pend[reg.slot] = genCompletion{}
			if p.kind == compRaise && isProtected(p.val) {
				c = p
			}
		case regWith:
			mgr := b.mgrs[reg.slot]
			b.mgrs[reg.slot] = nil
			if mgr != nil {
				var result object.Object = NULL
				if c.kind == compRaise {
					result = c.val
				}
				res := callWithExit(ctx, mgr, result, st.env)
				switch {
				case c.kind == compRaise && res == c.val:
				case c.kind == compRaise && !propagates(res):
					// __exit__ suppressed the exception: carry on after the with.
					st.cursor = reg.end
					return nil, false
				case propagates(res):
					c = genCompletion{kind: compRaise, val: res}
				}
			}
		}
		r = reg.parent
	}
	st.finish()
	switch c.kind {
	case compRaise:
		return c.val, true
	case compReturn:
		return genStopIterationValue(c.val), true
	}
	return errors.NewError("'break' outside loop"), true
}

// park records completion c for the try in slot while its finally runs.
func (st *genState) park(slot int, c genCompletion) {
	st.blocks().pend[slot] = c
}

// newGenerator constructs the generator object for a call to a generator
// function. Parameters bind eagerly (Python validates the call now); the
// body runs lazily from the first next().
func newGenerator(ctx context.Context, fn *object.Function, args []object.Object, keywords object.Kwargs, env *object.Environment, plan *genPlan) object.Object {
	callEnv, errObj := extendFunctionEnv(ctx, fn, args, keywords, kwArgs{})
	if errObj != nil {
		return errObj
	}
	// The instance and its driver state share one allocation.
	o := &genObject{}
	object.TrackAlloc(o, int64(unsafe.Sizeof(*o)))
	o.inst.Class = generatorClass
	o.st.plan, o.st.env = plan, callEnv
	o.inst.NativeData = &o.st
	if plan.nTry > 0 || plan.nWith > 0 {
		// A generator with try or with blocks may be dropped while paused
		// inside one. Close it when it is collected so its finally blocks
		// and __exit__ calls still run; generators without such blocks have
		// nothing to run and get no finalizer.
		runtime.SetFinalizer(o, finalizeGenerator)
	}
	return &o.inst
}

// finalizeGenerator closes a generator that was collected unfinished. It
// runs on the Go runtime's finalizer goroutine, so it takes the generator's
// interpreter lock, bounds its own run time, and never lets a panic or an
// error from the generator's cleanup escape.
func finalizeGenerator(o *genObject) {
	st := &o.st
	if st.done || (st.cursor == 0 && !st.delegating()) || st.env == nil {
		return // finished, or never started: nothing to clean up
	}
	defer func() { _ = recover() }()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ctx = WithEvaluator(ctx)
	ctx = ContextWithCallDepth(ctx, DefaultMaxCallDepth)
	acquired, entered := st.env.EnterGILWithContext(ctx)
	if !entered {
		return
	}
	if acquired {
		defer st.env.ExitGIL()
	}
	generatorCloseRun(ctx, &o.inst, st.env)
}

// genObject is a generator instance together with its driver state.
type genObject struct {
	inst object.Instance
	st   genState
}

func genStopIteration() object.Object {
	return &object.Exception{Message: "", ExceptionType: object.ExceptionTypeStopIteration, Raised: true}
}

// genStopIterationValue is the StopIteration ending a generator that
// returned v: v is its value (e.value, and the result of yield from).
func genStopIterationValue(v object.Object) object.Object {
	if v == nil || v == NULL {
		return genStopIteration()
	}
	msg, err := v.AsString()
	if err != nil {
		msg = v.Inspect()
	}
	return &object.Exception{Message: msg, ExceptionType: object.ExceptionTypeStopIteration, Args: []object.Object{v}, Raised: true}
}

// generatorClose finishes a generator and the chain it delegates to without
// running any of their code: for generators paused where nothing could see
// a GeneratorExit.
func generatorClose(g *object.Instance) {
	for g != nil {
		st, _ := g.NativeData.(*genState)
		if st == nil {
			return
		}
		st.done = true
		g = nil
		if st.ext != nil {
			g = st.ext.sub
			st.ext.sub, st.ext.subIter = nil, nil
		}
	}
}

const exceptionTypeGeneratorExit = "GeneratorExit"

// generatorCloseRun is close(): GeneratorExit is raised at the paused yield
// so finally blocks and __exit__ run. It returns the value the generator
// returned while closing (nil for None), or the error close() raises: one
// the generator raised while closing, or RuntimeError when it yielded
// instead of finishing.
func generatorCloseRun(ctx context.Context, g *object.Instance, env *object.Environment) (object.Object, object.Object) {
	st, _ := g.NativeData.(*genState)
	if st == nil || st.done || (st.cursor == 0 && !st.delegating()) ||
		((st.ext == nil || st.ext.sub == nil) && st.pausedRegion() == 0) {
		// Not started, finished, or paused outside every try and with:
		// nothing can observe the GeneratorExit.
		generatorClose(g)
		return nil, nil
	}
	r := generatorThrow(ctx, g, &object.Exception{ExceptionType: exceptionTypeGeneratorExit, Raised: true}, env)
	if exc, ok := r.(*object.Exception); ok && exc.Raised {
		switch exc.ExceptionType {
		case exceptionTypeGeneratorExit:
			return nil, nil
		case object.ExceptionTypeStopIteration:
			// The generator returned while closing: close() gives the
			// value it returned (Python 3.13).
			if len(exc.Args) > 0 {
				return exc.Args[0], nil
			}
			return nil, nil
		}
	}
	if propagates(r) {
		return nil, r
	}
	return nil, &object.Exception{Message: "generator ignored GeneratorExit", ExceptionType: object.ExceptionTypeRuntimeError, Raised: true}
}

// generatorThrow is throw(): exc is raised at the paused yield, where an
// enclosing try can catch it; the generator then runs to its next yield,
// whose value is returned. A delegated generator gets the exception first.
func generatorThrow(ctx context.Context, g *object.Instance, exc object.Object, env *object.Environment) object.Object {
	st, _ := g.NativeData.(*genState)
	if st == nil || st.done {
		generatorClose(g)
		return exc
	}
	if st.cursor == 0 && !st.delegating() {
		// Not started: raised before the first statement, where nothing
		// can catch it.
		st.finish()
		return exc
	}
	region := st.pausedRegion()
	if x := st.ext; x != nil && x.sub != nil {
		sub := x.sub
		var r object.Object
		if e, ok := exc.(*object.Exception); ok && e.ExceptionType == exceptionTypeGeneratorExit {
			// Python closes the delegate, then raises GeneratorExit here.
			if _, r = generatorCloseRun(ctx, sub, env); r == nil {
				r = exc
			}
		} else {
			r = generatorThrow(ctx, sub, exc, env)
		}
		if e, ok := r.(*object.Exception); ok && e.Raised && e.ExceptionType == object.ExceptionTypeStopIteration {
			// The delegate returned: yield from evaluates to its value.
			x.sub = nil
			var v object.Object = NULL
			if len(e.Args) > 0 {
				v = e.Args[0]
			}
			it := &st.plan.items[st.cursor]
			st.cursor++
			if it.target != nil {
				if err := assignToExpression(ctx, it.target, v, st.env); err != nil {
					if res, stop := st.fail(ctx, g, region, assignErrorToObject(err)); stop {
						return res
					}
				}
			}
			return generatorNext(ctx, g, env)
		}
		if !propagates(r) {
			return r // the delegate yielded
		}
		x.sub = nil
		exc = r
	} else if x != nil && x.subIter != nil {
		x.subIter = nil
	}
	if res, stop := st.fail(ctx, g, region, exc); stop {
		return res
	}
	return generatorNext(ctx, g, env)
}

// generatorSend resumes g with v as the value of its paused yield.
func generatorSend(ctx context.Context, g *object.Instance, v object.Object, env *object.Environment) object.Object {
	st, _ := g.NativeData.(*genState)
	if v == nil || v == NULL || st == nil {
		return generatorNext(ctx, g, env)
	}
	// Paused at a yield, the cursor is past it (or on a yield from that is
	// delegating): only a generator that never ran sits at item 0.
	if !st.done && st.cursor == 0 && !st.delegating() {
		return errors.NewTypeErrorTagged("can't send non-None value to a just-started generator")
	}
	x := st.extra()
	x.sent = v
	r := generatorNext(ctx, g, env)
	x.sent = nil
	return r
}

// toRaisedException turns throw()'s argument — an exception value, a user
// exception instance or class, or a built-in exception constructor (called
// with any extra arguments) — into a raised exception.
func toRaisedException(ctx context.Context, arg object.Object, extra []object.Object, env *object.Environment) object.Object {
	switch a := arg.(type) {
	case *object.Exception:
		a.Raised = true
		return a
	case *object.Instance:
		if exc, chain, args := exceptionFromClassInstance(ctx, a, env); exc != nil {
			exc.Raised = true
			exc.TypeChain = chain
			exc.Args = args
			return exc
		}
	case *object.Class:
		if a.ExceptionBase != "" {
			inst := applyFunctionWithContext(ctx, a, extra, object.Kwargs{}, env)
			if propagates(inst) {
				return inst
			}
			return toRaisedException(ctx, inst, nil, env)
		}
	case *object.Builtin:
		if _, ok := exceptionBuiltins[a]; ok {
			return toRaisedException(ctx, a.Fn(SetEnvInContext(ctx, env), object.NewKwargs(nil), extra...), nil, env)
		}
	}
	return errors.NewTypeErrorTagged("exceptions must derive from BaseException")
}

// genExhausted is the internal result of resuming a generator that has
// finished without a return value: generatorNext turns it into a fresh
// StopIteration, and a loop over the generator just stops, so ending a loop
// allocates nothing. It never reaches a script.
var genExhausted = &object.Exception{ExceptionType: object.ExceptionTypeStopIteration, Raised: true}

// generatorNext resumes the generator until the next yield value.
func generatorNext(ctx context.Context, g *object.Instance, env *object.Environment) object.Object {
	r := generatorNextRaw(ctx, g, env)
	if r == genExhausted {
		return genStopIteration()
	}
	return r
}

// generatorNextRaw is generatorNext returning genExhausted for a generator
// that finished without a return value.
func generatorNextRaw(ctx context.Context, g *object.Instance, env *object.Environment) object.Object {
	st, ok := g.NativeData.(*genState)
	if !ok || st == nil {
		return errors.NewError("generator has no driver state")
	}
	if st.done {
		return genExhausted
	}
	// Resuming the body is a call: it nests in Go, so a generator whose body
	// resumes another (yield next(g(n + 1))) recursed past the call depth
	// limit to a fatal stack overflow. The charge is here, not where the
	// generator is created, because a generator is often made shallow and
	// resumed deep. At the limit the generator finishes, so the caller sees
	// the error rather than a stopped iteration. The depth is released
	// explicitly, not deferred: with this many returns a defer would not be
	// inlined, and resuming is the hot path.
	if ctx != st.depthCtx {
		st.depthCtx, st.depth = ctx, GetCallDepthFromContext(ctx)
	}
	cd := st.depth
	if cd == nil {
		return generatorResume(ctx, g, env, st)
	}
	if !cd.Enter() {
		st.finish()
		return cd.LimitError()
	}
	r := generatorResume(ctx, g, env, st)
	cd.Exit()
	return r
}

// generatorResume runs a generator's body from its cursor to the next yield,
// return or error. An error from an item goes through st.fail, which either
// ends the generator (stop) or moves the cursor to a handler.
func generatorResume(ctx context.Context, g *object.Instance, env *object.Environment, st *genState) object.Object {
	genv := st.env
	items := st.plan.items

	for {
		idx := st.cursor
		if idx >= len(items) {
			st.finish()
			return genExhausted
		}
		it := &items[idx]
		switch it.kind {
		case genRun:
			st.cursor = idx + 1
			r := it.fn(ctx, genv)
			switch r := r.(type) {
			case *object.Error:
				if res, stop := st.fail(ctx, g, it.region, r); stop {
					return res
				}
			case *object.Exception:
				if r.Raised {
					if res, stop := st.fail(ctx, g, it.region, r); stop {
						return res
					}
				}
			case *object.ReturnValue:
				if it.region == 0 {
					st.finish()
					if r.Value == nil || r.Value == NULL {
						return genExhausted
					}
					return genStopIterationValue(r.Value)
				}
				if res, stop := st.unwind(ctx, g, it.region, genCompletion{kind: compReturn, val: r.Value}); stop {
					return res
				}
			case *object.Break:
				if it.brk < 0 {
					st.finish()
					return errors.NewError("'break' outside loop")
				}
				if res, stop := st.jump(ctx, g, it, it.brk); stop {
					return res
				}
			case *object.Continue:
				if it.cont < 0 {
					st.finish()
					return errors.NewError("'continue' not properly in loop")
				}
				if res, stop := st.jump(ctx, g, it, it.cont); stop {
					return res
				}
			}
		case genYield:
			st.cursor = idx + 1
			if it.fn == nil {
				return NULL
			}
			val := it.fn(ctx, genv)
			if propagates(val) {
				if res, stop := st.fail(ctx, g, it.region, val); stop {
					return res
				}
				continue
			}
			return val
		case genRecv:
			st.cursor = idx + 1
			var sent object.Object = NULL
			if st.ext != nil && st.ext.sent != nil {
				sent, st.ext.sent = st.ext.sent, nil
			}
			if err := assignToExpression(ctx, it.target, sent, genv); err != nil {
				if res, stop := st.fail(ctx, g, it.region, assignErrorToObject(err)); stop {
					return res
				}
			}
		case genYieldFrom:
			r, suspended := generatorDelegate(ctx, it, env, st)
			if suspended {
				return r
			}
			if propagates(r) {
				st.cursor = idx + 1
				if res, stop := st.fail(ctx, g, it.region, r); stop {
					return res
				}
				continue
			}
			st.cursor = idx + 1
			if it.target != nil {
				if err := assignToExpression(ctx, it.target, r, genv); err != nil {
					if res, stop := st.fail(ctx, g, it.region, assignErrorToObject(err)); stop {
						return res
					}
				}
			}
		case genForInit:
			st.cursor = idx + 1
			iterVal := it.fn(ctx, genv)
			if propagates(iterVal) {
				if res, stop := st.fail(ctx, g, it.region, iterVal); stop {
					return res
				}
				continue
			}
			iter, errObj := generatorIterFor(ctx, iterVal, env)
			if errObj != nil {
				if res, stop := st.fail(ctx, g, it.region, errObj); stop {
					return res
				}
				continue
			}
			if it.slot == 0 {
				st.iter = iter
			} else {
				x := st.extra()
				for len(x.iters) < it.slot {
					x.iters = append(x.iters, nil)
				}
				x.iters[it.slot-1] = iter
			}
		case genForNext:
			st.cursor = idx + 1
			iter := st.iter
			if it.slot > 0 {
				iter = st.ext.iters[it.slot-1]
			}
			if iter == nil {
				st.finish()
				return errors.NewError("generator loop has no iterator")
			}
			elem, hasNext := iter.Next()
			if !hasNext {
				if it.slot == 0 {
					st.iter = nil
				} else {
					st.ext.iters[it.slot-1] = nil
				}
				st.cursor = it.jump
				continue
			}
			if propagates(elem) {
				if res, stop := st.fail(ctx, g, it.region, elem); stop {
					return res
				}
				continue
			}
			if err := setForVariables(it.targets, elem, genv); err != nil {
				if res, stop := st.fail(ctx, g, it.region, assignErrorToObject(err)); stop {
					return res
				}
			}
		case genCond:
			st.cursor = idx + 1
			c := it.fn(ctx, genv)
			if propagates(c) {
				if res, stop := st.fail(ctx, g, it.region, c); stop {
					return res
				}
				continue
			}
			truthy, errObj := evalTruthy(ctx, c, genv)
			if errObj != nil {
				if res, stop := st.fail(ctx, g, it.region, errObj); stop {
					return res
				}
				continue
			}
			if !truthy {
				st.cursor = it.jump
			}
		case genJump:
			st.cursor = it.jump
		default:
			if res, stop := generatorStep(ctx, g, st, it, idx); stop {
				return res
			}
		}
	}
}

// jump moves the cursor to a break or continue target, running the cleanup
// of any try or with blocks between the item and the loop; stop is set when
// that cleanup raised out of the generator.
func (st *genState) jump(ctx context.Context, g *object.Instance, it *genItem, target int) (object.Object, bool) {
	loopRegion := st.plan.items[it.cont].region
	if it.region == loopRegion {
		st.cursor = target
		return nil, false
	}
	return st.unwind(ctx, g, it.region, genCompletion{kind: compJump, target: target, region: loopRegion})
}

// generatorStep runs the items only generators with try, with or match
// blocks have, kept out of generatorResume's switch so the common items
// stay compact.
func generatorStep(ctx context.Context, g *object.Instance, st *genState, it *genItem, idx int) (object.Object, bool) {
	genv := st.env
	switch it.kind {
	case genWithEnter:
		st.cursor = idx + 1
		obj := it.fn(ctx, genv)
		if propagates(obj) {
			return st.fail(ctx, g, it.region, obj)
		}
		inst, ok := obj.(*object.Instance)
		if !ok {
			return st.fail(ctx, g, it.region, errors.NewError("with statement requires an object with __enter__ and __exit__ methods"))
		}
		enter := callDunderMethod(ctx, inst, "__enter__", nil, genv)
		if enter == nil {
			enter = NULL
		}
		if propagates(enter) {
			return st.fail(ctx, g, it.region, enter)
		}
		if it.target != nil {
			if err := assignToExpression(ctx, it.target, enter, genv); err != nil {
				return st.fail(ctx, g, it.region, assignErrorToObject(err))
			}
		}
		st.blocks().mgrs[it.slot] = inst
	case genWithExit:
		st.cursor = idx + 1
		b := st.blocks()
		mgr := b.mgrs[it.slot]
		b.mgrs[it.slot] = nil
		if r := callWithExit(ctx, mgr, NULL, genv); propagates(r) {
			return st.fail(ctx, g, it.region, r)
		}
	case genMatch:
		i, errObj := it.match.sel(ctx, genv)
		if errObj != nil {
			st.cursor = idx + 1
			return st.fail(ctx, g, it.region, errObj)
		}
		if i < 0 {
			st.cursor = it.jump
		} else {
			st.cursor = it.match.starts[i]
		}
	case genExceptEnd:
		genv.Delete("__current_exception__")
		st.cursor = it.jump
	case genFinallyEnd:
		st.cursor = idx + 1
		b := st.blocks()
		c := b.pend[it.slot]
		b.pend[it.slot] = genCompletion{}
		if c.kind != compNone {
			return st.unwind(ctx, g, it.region, c)
		}
	default:
		st.finish()
		return errors.NewError("generator plan has an unknown item"), true
	}
	return nil, false
}

// generatorDelegate runs one step of `yield from`: on first arrival it
// evaluates the iterable, then it resumes the delegate (forwarding a sent
// value to a delegated generator). It returns the value to yield with
// suspended set, a propagating error, or — once the delegate is exhausted —
// the delegate's return value (None for a non-generator iterable).
func generatorDelegate(ctx context.Context, it *genItem, env *object.Environment, st *genState) (object.Object, bool) {
	x := st.extra()
	if x.sub == nil && x.subIter == nil {
		src := it.fn(ctx, st.env)
		if propagates(src) {
			return src, false
		}
		if inst, ok := src.(*object.Instance); ok && inst.Class == generatorClass {
			x.sub = inst
		} else {
			iter, errObj := generatorIterFor(ctx, src, env)
			if errObj != nil {
				return errObj, false
			}
			x.subIter = iter
		}
		// The delegate starts with next(): a value sent to the yield that
		// paused before this one belongs to that yield.
		x.sent = nil
	}
	sent := x.sent
	x.sent = nil
	if x.sub != nil {
		r := generatorSend(ctx, x.sub, sent, env)
		if exc, ok := r.(*object.Exception); ok && exc.ExceptionType == object.ExceptionTypeStopIteration && exc.Raised {
			x.sub = nil
			if len(exc.Args) > 0 {
				return exc.Args[0], false
			}
			return NULL, false
		}
		if propagates(r) {
			x.sub = nil
			return r, false
		}
		return r, true
	}
	if sent != nil && sent != NULL {
		x.subIter = nil
		return &object.Exception{Message: "'iterator' object has no attribute 'send'", ExceptionType: "AttributeError", Raised: true}, false
	}
	elem, ok := x.subIter.Next()
	if !ok {
		x.subIter = nil
		return NULL, false
	}
	if propagates(elem) {
		x.subIter = nil
		return elem, false
	}
	return elem, true
}

// generatorIterator adapts generator g to the iterator protocol for loops
// and builtins that consume it.
func generatorIterator(ctx context.Context, g *object.Instance, env *object.Environment) *object.Iterator {
	return object.NewIterator(func() (object.Object, bool) {
		r := generatorNextRaw(ctx, g, env)
		if r == genExhausted {
			return nil, false
		}
		if exc, ok := r.(*object.Exception); ok && exc.Raised && exc.ExceptionType == object.ExceptionTypeStopIteration {
			return nil, false
		}
		return r, true
	})
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
		object.TrackSlice(elements)
		for _, ch := range o.StringValue() {
			elements = append(elements, object.NewRuneString(ch))
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
