package evaluator

import (
	"context"

	"github.com/paularlott/scriptling/object"
)

// The evaluator's == and ordering, for Go-backed classes outside the
// evaluator (see object.SetCompareHooks). Both continue from the comparison
// depth the context carries, so values nested through such a class are still
// held to maxCompareDepth.
func init() {
	object.SetCompareHooks(hookEqual, hookCompare)
	object.SetReprHook(func(ctx context.Context, obj object.Object) (string, object.Object) {
		return renderConvertedValue(ctx, obj, "r", GetEnvFromContext(ctx))
	})
	object.SetStrHook(func(ctx context.Context, obj object.Object) (string, object.Object) {
		return renderConvertedValue(ctx, obj, "s", GetEnvFromContext(ctx))
	})
}

func hookEqual(ctx context.Context, a, b object.Object) (bool, object.Object) {
	depth := object.CompareDepth(ctx)
	if depth >= maxCompareDepth {
		return false, errCompareTooDeep()
	}
	if depth == 0 && (isInstanceOperand(a) || isInstanceOperand(b)) {
		return evalObjectsEqualChecked(ctx, a, b, GetEnvFromContext(ctx))
	}
	budget := cmpBudget{ctx: ctx}
	return deepEqual(&budget, a, b, depth)
}

func hookCompare(ctx context.Context, a, b object.Object) (int, object.Object) {
	depth := object.CompareDepth(ctx)
	if depth >= maxCompareDepth {
		return 0, errCompareTooDeep()
	}
	budget := cmpBudget{ctx: ctx}
	return compareForSortDepth(ctx, a, b, GetEnvFromContext(ctx), depth, &budget)
}

// cmpCheckEvery is how many containers a structural comparison walks between
// cancellation checks.
const cmpCheckEvery = 1 << 12

// cmpBudget lets one structural comparison (==, ordering) honour its call's
// cancellation. Values that share containers can be exponentially larger
// than the memory they hold: x = [x0, x0] nested 40 deep compares 2**40
// elements against an equal copy built the same way, all within one
// operator, so the walk checks the context every cmpCheckEvery containers.
type cmpBudget struct {
	ctx context.Context
	n   int
}

// step counts one container and, every cmpCheckEvery, checks the context
// (and the memory-limit stop, as the evaluator's loops do). b must not be
// nil.
func (b *cmpBudget) step() object.Object {
	if b.n++; b.n&(cmpCheckEvery-1) != 0 {
		return nil
	}
	return b.check()
}

// check is step's slow path, kept out of line so step inlines.
//
//go:noinline
func (b *cmpBudget) check() object.Object {
	if b.ctx == nil {
		return nil
	}
	cc := newContextChecker(b.ctx)
	return cc.checkBatch()
}
