package object

import "context"

// Comparison hooks. The evaluator registers its == and ordering here so that
// Go-backed classes outside it (collections.deque) compare their elements
// exactly as scripts do: deeply, through user dunders, and within the same
// nesting limit, which the comparison depth carried on the context enforces
// across the hop out of the evaluator and back.
var (
	equalHook   func(ctx context.Context, a, b Object) (bool, Object)
	compareHook func(ctx context.Context, a, b Object) (int, Object)
)

// SetCompareHooks registers the evaluator's equality and ordering.
func SetCompareHooks(equal func(ctx context.Context, a, b Object) (bool, Object), compare func(ctx context.Context, a, b Object) (int, Object)) {
	equalHook, compareHook = equal, compare
}

// Equal reports whether a == b as a script would see it. The second return
// is an error to propagate (a raising __eq__, or a RecursionError).
func Equal(ctx context.Context, a, b Object) (bool, Object) {
	if equalHook == nil {
		return a == b, nil
	}
	return equalHook(ctx, a, b)
}

// Compare orders a and b as sorted() does: negative, zero or positive, or an
// error to propagate (a TypeError for values that cannot be ordered).
func Compare(ctx context.Context, a, b Object) (int, Object) {
	if compareHook == nil {
		return 0, nil
	}
	return compareHook(ctx, a, b)
}

type compareDepthKey struct{}

// WithCompareDepth returns ctx carrying the nesting depth of a comparison in
// progress, for a dunder called while comparing container elements.
func WithCompareDepth(ctx context.Context, depth int) context.Context {
	return context.WithValue(ctx, compareDepthKey{}, depth)
}

// CompareDepth returns the comparison nesting depth carried by ctx, 0 when
// none is in progress.
func CompareDepth(ctx context.Context) int {
	if ctx == nil {
		return 0
	}
	d, _ := ctx.Value(compareDepthKey{}).(int)
	return d
}
