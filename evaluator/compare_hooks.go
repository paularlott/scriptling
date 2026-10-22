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
}

func hookEqual(ctx context.Context, a, b object.Object) (bool, object.Object) {
	depth := object.CompareDepth(ctx)
	if depth >= maxCompareDepth {
		return false, errCompareTooDeep()
	}
	if depth == 0 && (isInstanceOperand(a) || isInstanceOperand(b)) {
		return evalObjectsEqualChecked(ctx, a, b, GetEnvFromContext(ctx))
	}
	return deepEqual(a, b, depth)
}

func hookCompare(ctx context.Context, a, b object.Object) (int, object.Object) {
	depth := object.CompareDepth(ctx)
	if depth >= maxCompareDepth {
		return 0, errCompareTooDeep()
	}
	return compareForSortDepth(ctx, a, b, GetEnvFromContext(ctx), depth)
}
