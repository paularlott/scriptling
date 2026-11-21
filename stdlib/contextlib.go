package stdlib

import (
	"context"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/evaliface"
	"github.com/paularlott/scriptling/object"
)

const suppressKey = "__suppress_types__"

// resolveExcTypeName returns the exception type name string for an object passed to suppress().
// Builtins (e.g. ValueError) are called with no args to obtain their ExceptionType field.
func resolveExcTypeName(ctx context.Context, obj object.Object) string {
	switch v := obj.(type) {
	case *object.String:
		return v.StringValue()
	case *object.Class:
		return v.Name
	case *object.Builtin:
		result := v.Fn(ctx, object.NewKwargs(nil))
		if exc, ok := object.AsException(result); ok {
			return exc.ExceptionType
		}
	}
	return ""
}

var suppressClass = &object.Class{
	Name: "suppress",
	Methods: map[string]object.Object{
		"__init__": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) < 1 {
					return errors.NewError("suppress.__init__ requires self")
				}
				inst, ok := args[0].(*object.Instance)
				if !ok {
					return errors.NewError("suppress.__init__: self must be an instance")
				}
				// Copy args[1:] so the stored List doesn't alias the caller's buffer.
				excTypes := make([]object.Object, len(args)-1)
				copy(excTypes, args[1:])
				inst.SetField(suppressKey, &object.List{Elements: excTypes})
				return &object.Null{}
			},
		},
		"__enter__": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) < 1 {
					return errors.NewError("suppress.__enter__ requires self")
				}
				return args[0]
			},
		},
		"__exit__": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				// args: self, exc_type, exc_val, exc_tb
				if len(args) < 2 {
					return object.NewBoolean(false)
				}
				inst, ok := args[0].(*object.Instance)
				if !ok {
					return object.NewBoolean(false)
				}
				excType := args[1]
				if excType == nil || excType.Type() == object.NULL_OBJ {
					return object.NewBoolean(false)
				}
				typesObj, ok := inst.GetField(suppressKey)
				if !ok {
					return object.NewBoolean(false)
				}
				typesList, ok := typesObj.(*object.List)
				if !ok {
					return object.NewBoolean(false)
				}
				if len(typesList.Elements) == 0 {
					return object.NewBoolean(true)
				}
				excTypeName := resolveExcTypeName(ctx, excType)
				for _, t := range typesList.Elements {
					name := resolveExcTypeName(ctx, t)
					if name == excTypeName || name == object.ExceptionTypeException {
						return object.NewBoolean(true)
					}
				}
				return object.NewBoolean(false)
			},
		},
	},
}

func suppressNew(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	inst := object.NewInstanceWithFields(suppressClass, nil)
	// Copy args so the stored List doesn't alias the caller's buffer.
	excTypes := make([]object.Object, len(args))
	copy(excTypes, args)
	inst.SetField(suppressKey, &object.List{Elements: excTypes})
	return inst
}

// selfInstance returns the instance a Go-defined method was called on.
func selfInstance(args []object.Object, method string) (*object.Instance, object.Object) {
	if len(args) < 1 {
		return nil, errors.NewError("%s requires self", method)
	}
	inst, ok := args[0].(*object.Instance)
	if !ok {
		return nil, errors.NewError("%s: self must be an instance", method)
	}
	return inst, nil
}

// callMember calls obj.name(args...) for an instance obj; found is false
// when obj has no such member.
func callMember(ctx context.Context, obj *object.Instance, name string, args ...object.Object) (result object.Object, found bool) {
	m, ok := obj.Class.LookupMember(name)
	if !ok {
		return nil, false
	}
	eval := evaliface.FromContext(ctx)
	if eval == nil {
		return errors.NewError("evaluator not available in context"), true
	}
	return eval.CallObjectFunction(ctx, m, append([]object.Object{obj}, args...), nil, nil), true
}

func isStopIteration(o object.Object) bool {
	exc, ok := o.(*object.Exception)
	return ok && exc.ExceptionType == object.ExceptionTypeStopIteration
}

// generatorCMClass is the context manager contextmanager-decorated functions
// return: __enter__ runs the generator to its yield and returns the yielded
// value; __exit__ resumes it so the code after the yield runs. When the with
// body raised, __exit__ throws the exception into the generator at its yield,
// as Python does: a try/finally around the yield runs its cleanup, and a
// generator that catches the exception and finishes suppresses it.
var generatorCMClass = &object.Class{
	Name: "_GeneratorContextManager",
	Methods: map[string]object.Object{
		"__enter__": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				self, err := selfInstance(args, "__enter__")
				if err != nil {
					return err
				}
				gen, _ := self.Field("_gen").(*object.Instance)
				if gen == nil {
					return errors.NewTypeErrorTagged("contextmanager function did not return a generator")
				}
				v, found := callMember(ctx, gen, "__next__")
				if !found {
					return errors.NewTypeErrorTagged("'%s' object is not an iterator", gen.Class.Name)
				}
				if isStopIteration(v) {
					return &object.Exception{Message: "generator didn't yield", ExceptionType: object.ExceptionTypeRuntimeError, Raised: true}
				}
				return v
			},
			HelpText: "__enter__() - Run the generator to its yield and return the yielded value",
		},
		"__exit__": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				self, err := selfInstance(args, "__exit__")
				if err != nil {
					return err
				}
				gen, _ := self.Field("_gen").(*object.Instance)
				if gen == nil {
					return object.NewBoolean(false)
				}
				if len(args) > 1 && args[1].Type() != object.NULL_OBJ {
					return generatorCMThrow(ctx, gen, args)
				}
				v, _ := callMember(ctx, gen, "__next__")
				if isStopIteration(v) {
					return object.NewBoolean(false)
				}
				if object.IsPropagating(v) {
					return v
				}
				return generatorCMDidntStop(ctx, gen, "generator didn't stop")
			},
			HelpText: "__exit__(exc_type, exc, tb) - Resume the generator past its yield",
		},
	},
}

// generatorCMThrow is __exit__ for a with body that raised: the exception
// (args[2]) is thrown into the generator. The generator finishing suppresses
// it (true); the same exception coming back propagates it unchanged (false);
// another exception replaces it.
func generatorCMThrow(ctx context.Context, gen *object.Instance, args []object.Object) object.Object {
	var exc *object.Exception
	if len(args) > 2 {
		exc, _ = args[2].(*object.Exception)
	}
	if exc == nil {
		// No exception value to throw (__exit__ called by hand with only a
		// type): close the generator and let the exception propagate.
		if r, _ := callMember(ctx, gen, "close"); r != nil && object.IsPropagating(r) {
			return r
		}
		return object.NewBoolean(false)
	}
	wasRaised := exc.Raised
	r, found := callMember(ctx, gen, "throw", exc)
	exc.Raised = wasRaised
	if !found {
		return object.NewBoolean(false)
	}
	if isStopIteration(r) && object.IsPropagating(r) {
		return object.NewBoolean(r != object.Object(exc))
	}
	if r == object.Object(exc) {
		return object.NewBoolean(false)
	}
	if object.IsPropagating(r) {
		return r
	}
	return generatorCMDidntStop(ctx, gen, "generator didn't stop after throw()")
}

// generatorCMDidntStop closes a generator that yielded a second time and
// returns the RuntimeError reporting it (or a raise from closing it).
func generatorCMDidntStop(ctx context.Context, gen *object.Instance, msg string) object.Object {
	if r, _ := callMember(ctx, gen, "close"); r != nil && object.IsPropagating(r) {
		return r
	}
	return &object.Exception{Message: msg, ExceptionType: object.ExceptionTypeRuntimeError, Raised: true}
}

// contextmanagerFn is contextlib.contextmanager(func).
func contextmanagerFn(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	if err := errors.ExactArgs(args, 1); err != nil {
		return err
	}
	fn := args[0]
	name := "<unknown>"
	if f, ok := fn.(*object.Function); ok {
		name = f.Name
	}
	return &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			eval := evaliface.FromContext(ctx)
			if eval == nil {
				return errors.NewError("evaluator not available in context")
			}
			gen := eval.CallObjectFunction(ctx, fn, append([]object.Object(nil), args...), kwargs.Map(), nil)
			if object.IsPropagating(gen) {
				return gen
			}
			return object.NewInstanceWithFields(generatorCMClass, map[string]object.Object{"_gen": gen})
		},
		HelpText:   "context manager factory made by @contextmanager from " + name,
		Attributes: map[string]object.Object{"__name__": object.NewString(name), "__wrapped__": fn},
	}
}

// nullcontextClass is contextlib.nullcontext: __enter__ returns the value
// given, __exit__ does nothing.
var nullcontextClass = &object.Class{
	Name: "nullcontext",
	Methods: map[string]object.Object{
		"__enter__": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				self, err := selfInstance(args, "__enter__")
				if err != nil {
					return err
				}
				return self.Field("enter_result")
			},
			HelpText: "__enter__() - Return the enter_result given to nullcontext",
		},
		"__exit__": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				return object.NewBoolean(false)
			},
			HelpText: "__exit__(exc_type, exc, tb) - Do nothing",
		},
	},
}

// closingClass is contextlib.closing: __exit__ calls thing.close().
var closingClass = &object.Class{
	Name: "closing",
	Methods: map[string]object.Object{
		"__enter__": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				self, err := selfInstance(args, "__enter__")
				if err != nil {
					return err
				}
				return self.Field("thing")
			},
			HelpText: "__enter__() - Return the thing being closed",
		},
		"__exit__": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				self, err := selfInstance(args, "__exit__")
				if err != nil {
					return err
				}
				eval := evaliface.FromContext(ctx)
				ops := evaliface.OperatorsFromContext(ctx)
				if eval == nil || ops == nil {
					return errors.NewError("evaluator not available in context")
				}
				closeFn := ops.GetAttr(ctx, self.Field("thing"), "close")
				if object.IsPropagating(closeFn) {
					return closeFn
				}
				if r := eval.CallObjectFunction(ctx, closeFn, nil, nil, nil); object.IsPropagating(r) {
					return r
				}
				return object.NewBoolean(false)
			},
			HelpText: "__exit__(exc_type, exc, tb) - Call thing.close()",
		},
	},
}

var ContextlibLibrary = object.NewLibrary(ContextlibLibraryName, map[string]*object.Builtin{
	"contextmanager": {
		Fn: contextmanagerFn,
		HelpText: `contextmanager(func) - Decorator making a generator function a context manager

The code before the yield runs on entering the with block, the yielded value
is bound by "as", and the code after the yield runs on leaving it:

  @contextlib.contextmanager
  def opened(path):
      f = open(path)
      yield f
      f.close()

  with opened("data.txt") as f:
      ...

If the with block raises, the exception is raised in the generator at the
yield: put the cleanup in a try/finally around the yield to run it either
way, or catch the exception there to suppress it.`,
	},
	"nullcontext": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			var v object.Object = &object.Null{}
			if len(args) > 0 {
				v = args[0]
			} else if r := kwargs.Get("enter_result"); r != nil {
				v = r
			}
			return object.NewInstanceWithFields(nullcontextClass, map[string]object.Object{"enter_result": v})
		},
		HelpText: `nullcontext(enter_result=None) - A context manager that does nothing

  with (open(p) if p else contextlib.nullcontext(default)) as f: ...`,
	},
	"closing": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			return object.NewInstanceWithFields(closingClass, map[string]object.Object{"thing": args[0]})
		},
		HelpText: `closing(thing) - Context manager calling thing.close() on exit

  with contextlib.closing(make_conn()) as conn: ...`,
	},
	"suppress": {
		Fn: suppressNew,
		HelpText: `suppress(*exc_types) - Context manager that silently suppresses the given exception types.

If no exception types are given, all exceptions are suppressed.
Use Exception to suppress all standard exceptions.

Example:
    with contextlib.suppress(ValueError):
        int("not a number")  # silently ignored`,
	},
}, nil, "Utilities for common tasks involving the with statement")
