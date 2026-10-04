package evaluator

import (
	"context"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// sentinelConstructorFn implements sentinel(name, *, repr=None) from PEP 661.
// Each call returns a new distinct Sentinel; identity (is) is the equality
// that matters, and == agrees by falling back to reference equality for
// unknown types.
func sentinelConstructorFn(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	if err := errors.ExactArgs(args, 1); err != nil {
		return err
	}
	name, err := args[0].AsString()
	if err != nil {
		return newSentinelTypeError("name", args[0])
	}
	repr := ""
	if r := kwargs.Get("repr"); r != nil {
		s, err := r.AsString()
		if err != nil {
			return newSentinelTypeError("repr", r)
		}
		repr = s
	}
	return object.NewSentinel(name, repr)
}

// newSentinelTypeError builds Python's TypeError for a non-str argument,
// as a typed Error so `except TypeError:` catches it.
func newSentinelTypeError(arg string, got object.Object) object.Object {
	err := errors.NewError("sentinel() argument '%s' must be str, not %s", arg, getTypeName(got))
	err.ExceptionType = object.ExceptionTypeTypeError
	return err
}

// SentinelBuiltin is the global sentinel builtin (PEP 661, Python 3.15).
var SentinelBuiltin = &object.Builtin{
	Fn: sentinelConstructorFn,
	HelpText: `sentinel(name, *, repr=None) - Create a unique sentinel value

Returns a new object that is equal only to itself, for use as a "value was
not supplied" marker that cannot collide with real data:

  MISSING = sentinel("MISSING")

  def fetch(key, default=MISSING):
      if default is MISSING:
          ...

Parameters:
  name (str): The sentinel's name, used for repr() and the __name__ attribute.
  repr (str, optional): Overrides the repr() output; defaults to the name.

Each call returns a new, distinct sentinel. Sentinels are truthy, are not
callable, have no ordering comparisons, and can be stored in sets. Check them
with the is operator (== is true only against the same sentinel).`,
}
