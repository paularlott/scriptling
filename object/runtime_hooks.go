package object

import (
	"context"
	"sync/atomic"
)

// IsInstance is the isinstance builtin (obj against a type, class, union or
// tuple of them; it returns a Boolean or a raised error), installed by the
// evaluator for libraries that cannot import it. Nil until then.
var IsInstance func(ctx context.Context, obj, classinfo Object) Object

// reprHook and strHook are the evaluator's repr() and str(), installed for
// libraries that cannot import it.
var reprHook, strHook func(ctx context.Context, obj Object) (string, Object)

// SetReprHook registers the evaluator's repr().
func SetReprHook(repr func(ctx context.Context, obj Object) (string, Object)) {
	reprHook = repr
}

// SetStrHook registers the evaluator's str().
func SetStrHook(str func(ctx context.Context, obj Object) (string, Object)) {
	strHook = str
}

// Str renders obj as str() does in a script: an instance's __str__, else its
// __repr__; a container's elements with repr. The second return is an error
// to propagate.
func Str(ctx context.Context, obj Object) (string, Object) {
	if s, ok := obj.(*String); ok {
		return s.value, nil
	}
	if strHook == nil || ctx == nil {
		if IsReprContainer(obj) {
			return InspectRepr(obj, nil)
		}
		return obj.Inspect(), nil
	}
	return strHook(ctx, obj)
}

type reprChainKey struct{}

// reprChain links the values whose repr is being built on this call path,
// across the hops between the evaluator and library code.
type reprChain struct {
	obj   Object
	prev  *reprChain
	depth int
}

// Repr renders obj as repr() does in a script, calling __repr__ on instances
// (nested ones too). An instance met again while its own repr is being built
// renders as "...", as in Python. The second return is an error to
// propagate.
func Repr(ctx context.Context, obj Object) (string, Object) {
	if reprHook == nil || ctx == nil {
		if IsReprContainer(obj) {
			return InspectRepr(obj, nil)
		}
		return obj.Inspect(), nil
	}
	return reprHook(ctx, obj)
}

// reprPaths counts the repr paths being built anywhere (EnterRepr calls not
// yet matched by LeaveRepr), so ReprPathOf skips the context lookup in the
// common case of a repr that no instance's repr has started.
var reprPaths atomic.Int32

// EnterRepr marks obj (an instance or a container) as having its repr built
// on this call path. entered is false when it already is: the caller
// renders "..." (or CyclicRepr) instead of recursing. errObj is the
// RecursionError for a path nested past MaxReprDepth. A caller that entered
// calls LeaveRepr when its repr is built.
func EnterRepr(ctx context.Context, obj Object) (next context.Context, entered bool, errObj Object) {
	if ctx == nil {
		reprPaths.Add(1)
		return ctx, true, nil
	}
	chain, _ := ctx.Value(reprChainKey{}).(*reprChain)
	depth := 0
	if chain != nil {
		depth = chain.depth + 1
		if depth >= MaxReprDepth {
			return ctx, false, errReprTooDeep()
		}
	}
	for c := chain; c != nil; c = c.prev {
		if c.obj == obj {
			return ctx, false, nil
		}
	}
	reprPaths.Add(1)
	return context.WithValue(ctx, reprChainKey{}, &reprChain{obj: obj, prev: chain, depth: depth}), true, nil
}

// LeaveRepr ends what an EnterRepr that entered started.
func LeaveRepr() {
	reprPaths.Add(-1)
}

// ReprPath is the chain of values whose repr is being built on a call path
// (EnterRepr). The zero value is an empty path.
type ReprPath struct{ chain *reprChain }

// ReprPathOf returns ctx's repr path. It allocates nothing.
func ReprPathOf(ctx context.Context) ReprPath {
	if ctx == nil || reprPaths.Load() == 0 {
		return ReprPath{}
	}
	chain, _ := ctx.Value(reprChainKey{}).(*reprChain)
	return ReprPath{chain}
}

// Has reports whether obj's repr is being built on the path.
func (p ReprPath) Has(obj Object) bool {
	for c := p.chain; c != nil; c = c.prev {
		if c.obj == obj {
			return true
		}
	}
	return false
}

// CyclicRepr is what Python prints for a container met again inside its own
// repr: [...], {...} or (...).
func CyclicRepr(obj Object) string {
	return cyclicPlaceholder(obj)
}
