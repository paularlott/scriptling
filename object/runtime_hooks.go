package object

import "context"

// IsInstance is the isinstance builtin (obj against a type, class, union or
// tuple of them; it returns a Boolean or a raised error), installed by the
// evaluator for libraries that cannot import it. Nil until then.
var IsInstance func(ctx context.Context, obj, classinfo Object) Object
