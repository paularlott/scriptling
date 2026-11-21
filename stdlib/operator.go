package stdlib

import (
	"context"
	"fmt"
	"strings"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/evaliface"
	"github.com/paularlott/scriptling/object"
)

// The operator module: the language's operators as functions, and the
// itemgetter / attrgetter / methodcaller key-function factories. Every
// operation runs through the evaluator, so dunder methods and Python's
// errors apply exactly as they do to the operators themselves.

func operators(ctx context.Context) (evaliface.Operators, object.Object) {
	ops := evaliface.OperatorsFromContext(ctx)
	if ops == nil {
		return nil, errors.NewError("evaluator not available in context")
	}
	return ops, nil
}

// binaryOperator is operator.<name>(a, b): a <op> b.
func binaryOperator(name, op, doc string) *object.Builtin {
	return &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := operatorArgs(name, args, kwargs, 2); err != nil {
				return err
			}
			ops, err := operators(ctx)
			if err != nil {
				return err
			}
			return ops.BinaryOp(ctx, op, args[0], args[1])
		},
		HelpText: fmt.Sprintf("%s(a, b) - %s", name, doc),
	}
}

// unaryOperator is operator.<name>(a): <op> a.
func unaryOperator(name, op, doc string) *object.Builtin {
	return &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := operatorArgs(name, args, kwargs, 1); err != nil {
				return err
			}
			ops, err := operators(ctx)
			if err != nil {
				return err
			}
			return ops.UnaryOp(ctx, op, args[0])
		},
		HelpText: fmt.Sprintf("%s(a) - %s", name, doc),
	}
}

func operatorArgs(name string, args []object.Object, kwargs object.Kwargs, n int) object.Object {
	if kwargs.Len() > 0 {
		return errors.NewTypeErrorTagged("%s() takes no keyword arguments", name)
	}
	if len(args) != n {
		return errors.NewTypeErrorTagged("%s expected %d arguments, got %d", name, n, len(args))
	}
	return nil
}

func operatorTruth(ctx context.Context, v object.Object) (bool, object.Object) {
	ops, err := operators(ctx)
	if err != nil {
		return false, err
	}
	return ops.Truthy(ctx, v)
}

// newItemgetter is itemgetter(*items): f(obj) is obj[item], or the tuple of
// obj[item] for each item when there are several.
func newItemgetter(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	if kwargs.Len() > 0 {
		return errors.NewTypeErrorTagged("itemgetter() takes no keyword arguments")
	}
	if len(args) == 0 {
		return errors.NewTypeErrorTagged("itemgetter expected 1 argument, got 0")
	}
	items := append([]object.Object(nil), args...)
	reprs := make([]string, len(items))
	for i, it := range items {
		reprs[i] = reprOf(it)
	}
	return &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := operatorArgs("itemgetter", args, kwargs, 1); err != nil {
				return err
			}
			ops, err := operators(ctx)
			if err != nil {
				return err
			}
			if len(items) == 1 {
				return ops.GetItem(ctx, args[0], items[0])
			}
			out := make([]object.Object, len(items))
			for i, it := range items {
				v := ops.GetItem(ctx, args[0], it)
				if object.IsPropagating(v) {
					return v
				}
				out[i] = v
			}
			return &object.Tuple{Elements: out}
		},
		HelpText: "itemgetter object - f(obj) returns obj[item] (a tuple for several items)",
		Repr:     "operator.itemgetter(" + strings.Join(reprs, ", ") + ")",
	}
}

// newAttrgetter is attrgetter(*names): f(obj) is obj.name (dotted names
// follow the chain), or a tuple for several names.
func newAttrgetter(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	if kwargs.Len() > 0 {
		return errors.NewTypeErrorTagged("attrgetter() takes no keyword arguments")
	}
	if len(args) == 0 {
		return errors.NewTypeErrorTagged("attrgetter expected 1 argument, got 0")
	}
	paths := make([][]string, len(args))
	reprs := make([]string, len(args))
	for i, a := range args {
		s, ok := a.(*object.String)
		if !ok {
			return errors.NewTypeErrorTagged("attribute name must be a string")
		}
		paths[i] = strings.Split(s.StringValue(), ".")
		reprs[i] = reprOf(a)
	}
	get := func(ctx context.Context, ops evaliface.Operators, obj object.Object, path []string) object.Object {
		for _, name := range path {
			obj = ops.GetAttr(ctx, obj, name)
			if object.IsPropagating(obj) {
				return obj
			}
		}
		return obj
	}
	return &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := operatorArgs("attrgetter", args, kwargs, 1); err != nil {
				return err
			}
			ops, err := operators(ctx)
			if err != nil {
				return err
			}
			if len(paths) == 1 {
				return get(ctx, ops, args[0], paths[0])
			}
			out := make([]object.Object, len(paths))
			for i, p := range paths {
				v := get(ctx, ops, args[0], p)
				if object.IsPropagating(v) {
					return v
				}
				out[i] = v
			}
			return &object.Tuple{Elements: out}
		},
		HelpText: "attrgetter object - f(obj) returns obj.attr (a tuple for several names; dotted names follow the chain)",
		Repr:     "operator.attrgetter(" + strings.Join(reprs, ", ") + ")",
	}
}

// newMethodcaller is methodcaller(name, *args, **kwargs): f(obj) calls
// obj.name(*args, **kwargs).
func newMethodcaller(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	if len(args) == 0 {
		return errors.NewTypeErrorTagged("methodcaller needs at least one argument, the method name")
	}
	nameObj, ok := args[0].(*object.String)
	if !ok {
		return errors.NewTypeErrorTagged("method name must be a string")
	}
	name := nameObj.StringValue()
	callArgs := append([]object.Object(nil), args[1:]...)
	var callKw map[string]object.Object
	if kwargs.Len() > 0 {
		callKw = make(map[string]object.Object, kwargs.Len())
		for k, v := range kwargs.All() {
			callKw[k] = v
		}
	}
	return &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := operatorArgs("methodcaller", args, kwargs, 1); err != nil {
				return err
			}
			ops, err := operators(ctx)
			if err != nil {
				return err
			}
			method := ops.GetAttr(ctx, args[0], name)
			if object.IsPropagating(method) {
				return method
			}
			eval := evaliface.FromContext(ctx)
			return eval.CallObjectFunction(ctx, method, append([]object.Object(nil), callArgs...), callKw, nil)
		},
		HelpText: "methodcaller object - f(obj) calls obj." + name + "(...)",
		Repr:     "operator.methodcaller(" + reprOf(nameObj) + ")",
	}
}

// reprOf is a value's repr for the factories' own reprs.
func reprOf(o object.Object) string {
	if s, ok := o.(*object.String); ok {
		return object.ReprString(s.StringValue())
	}
	return o.Inspect()
}

func operatorFunctions() map[string]*object.Builtin {
	fns := map[string]*object.Builtin{
		"add":      binaryOperator("add", "+", "Same as a + b"),
		"sub":      binaryOperator("sub", "-", "Same as a - b"),
		"mul":      binaryOperator("mul", "*", "Same as a * b"),
		"truediv":  binaryOperator("truediv", "/", "Same as a / b"),
		"floordiv": binaryOperator("floordiv", "//", "Same as a // b"),
		"mod":      binaryOperator("mod", "%", "Same as a % b"),
		"pow":      binaryOperator("pow", "**", "Same as a ** b"),
		"eq":       binaryOperator("eq", "==", "Same as a == b"),
		"ne":       binaryOperator("ne", "!=", "Same as a != b"),
		"lt":       binaryOperator("lt", "<", "Same as a < b"),
		"le":       binaryOperator("le", "<=", "Same as a <= b"),
		"gt":       binaryOperator("gt", ">", "Same as a > b"),
		"ge":       binaryOperator("ge", ">=", "Same as a >= b"),
		"and_":     binaryOperator("and_", "&", "Same as a & b"),
		"or_":      binaryOperator("or_", "|", "Same as a | b"),
		"xor":      binaryOperator("xor", "^", "Same as a ^ b"),
		"lshift":   binaryOperator("lshift", "<<", "Same as a << b"),
		"rshift":   binaryOperator("rshift", ">>", "Same as a >> b"),
		"is_":      binaryOperator("is_", "is", "Same as a is b"),
		"is_not":   binaryOperator("is_not", "is not", "Same as a is not b"),
		"neg":      unaryOperator("neg", "-", "Same as -a"),
		"pos":      unaryOperator("pos", "+", "Same as +a"),
		"abs":      unaryOperator("abs", "abs", "Same as abs(a)"),
		"invert":   unaryOperator("invert", "~", "Same as ~a"),
		"inv":      unaryOperator("inv", "~", "Same as ~a"),
		"not_":     unaryOperator("not_", "not", "Same as not a"),
		"itemgetter": {
			Fn: newItemgetter,
			HelpText: `itemgetter(item, ...) - Return a callable fetching item(s) from its operand

  f = itemgetter(1)        # f(x) == x[1]
  g = itemgetter("a", "b") # g(d) == (d["a"], d["b"])
  sorted(rows, key=itemgetter(1))`,
		},
		"attrgetter": {
			Fn: newAttrgetter,
			HelpText: `attrgetter(name, ...) - Return a callable fetching attribute(s) from its operand

  f = attrgetter("name")             # f(x) == x.name
  g = attrgetter("name", "address.city")  # g(x) == (x.name, x.address.city)
  sorted(people, key=attrgetter("age"))`,
		},
		"methodcaller": {
			Fn: newMethodcaller,
			HelpText: `methodcaller(name, *args, **kwargs) - Return a callable calling a method on its operand

  f = methodcaller("split", ",")   # f(s) == s.split(",")`,
		},
		"truth": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := operatorArgs("truth", args, kwargs, 1); err != nil {
					return err
				}
				t, err := operatorTruth(ctx, args[0])
				if err != nil {
					return err
				}
				return object.NewBoolean(t)
			},
			HelpText: "truth(a) - Same as bool(a)",
		},
		"contains": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := operatorArgs("contains", args, kwargs, 2); err != nil {
					return err
				}
				ops, err := operators(ctx)
				if err != nil {
					return err
				}
				return ops.BinaryOp(ctx, "in", args[1], args[0])
			},
			HelpText: "contains(a, b) - Same as b in a (note the reversed operands)",
		},
		"countOf": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := operatorArgs("countOf", args, kwargs, 2); err != nil {
					return err
				}
				n := int64(0)
				err := operatorEach(ctx, args[0], func(v object.Object) (bool, object.Object) {
					eq, err := operatorIsOrEq(ctx, v, args[1])
					if eq {
						n++
					}
					return false, err
				})
				if err != nil {
					return err
				}
				return object.NewInteger(n)
			},
			HelpText: "countOf(a, b) - Return the number of items in a which are, or equal, b",
		},
		"indexOf": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := operatorArgs("indexOf", args, kwargs, 2); err != nil {
					return err
				}
				i, found := int64(0), int64(-1)
				err := operatorEach(ctx, args[0], func(v object.Object) (bool, object.Object) {
					eq, err := operatorIsOrEq(ctx, v, args[1])
					if eq {
						found = i
						return true, err
					}
					i++
					return false, err
				})
				if err != nil {
					return err
				}
				if found < 0 {
					return &object.Exception{Message: "sequence.index(x): x not in sequence", ExceptionType: object.ExceptionTypeValueError, Raised: true}
				}
				return object.NewInteger(found)
			},
			HelpText: "indexOf(a, b) - Return the first index of b in a",
		},
		"getitem": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := operatorArgs("getitem", args, kwargs, 2); err != nil {
					return err
				}
				ops, err := operators(ctx)
				if err != nil {
					return err
				}
				return ops.GetItem(ctx, args[0], args[1])
			},
			HelpText: "getitem(a, b) - Same as a[b]",
		},
		"setitem": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := operatorArgs("setitem", args, kwargs, 3); err != nil {
					return err
				}
				ops, err := operators(ctx)
				if err != nil {
					return err
				}
				return ops.SetItem(ctx, args[0], args[1], args[2])
			},
			HelpText: "setitem(a, b, c) - Same as a[b] = c",
		},
		"delitem": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := operatorArgs("delitem", args, kwargs, 2); err != nil {
					return err
				}
				ops, err := operators(ctx)
				if err != nil {
					return err
				}
				return ops.DelItem(ctx, args[0], args[1])
			},
			HelpText: "delitem(a, b) - Same as del a[b]",
		},
		"concat": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := operatorArgs("concat", args, kwargs, 2); err != nil {
					return err
				}
				switch args[0].(type) {
				case *object.List, *object.Tuple, *object.String, *object.Bytes, *object.Instance:
				default:
					return errors.NewTypeErrorTagged("'%s' object can't be concatenated", object.PythonTypeName(args[0]))
				}
				ops, err := operators(ctx)
				if err != nil {
					return err
				}
				return ops.BinaryOp(ctx, "+", args[0], args[1])
			},
			HelpText: "concat(a, b) - Same as a + b, for sequences",
		},
		"index": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := operatorArgs("index", args, kwargs, 1); err != nil {
					return err
				}
				switch v := args[0].(type) {
				case *object.Integer:
					return v
				case *object.Boolean:
					if v.BoolValue() {
						return object.NewInteger(1)
					}
					return object.NewInteger(0)
				case *object.Instance:
					if eval := evaliface.FromContext(ctx); eval != nil {
						if ops, _ := operators(ctx); ops != nil {
							m := ops.GetAttr(ctx, v, "__index__")
							if !object.IsPropagating(m) {
								return eval.CallObjectFunction(ctx, m, nil, nil, nil)
							}
						}
					}
				}
				return errors.NewTypeErrorTagged("'%s' object cannot be interpreted as an integer", object.PythonTypeName(args[0]))
			},
			HelpText: "index(a) - Same as a.__index__(): a as an integer",
		},
	}
	// Python's dunder aliases (operator.__add__ is operator.add).
	for _, n := range []string{"add", "sub", "mul", "truediv", "floordiv", "mod", "pow",
		"eq", "ne", "lt", "le", "gt", "ge", "and_", "or_", "xor", "lshift", "rshift",
		"neg", "pos", "abs", "invert", "not_", "contains", "getitem", "setitem", "delitem", "concat", "index"} {
		fns["__"+strings.TrimSuffix(n, "_")+"__"] = fns[n]
	}
	return fns
}

// operatorEach walks an iterable, stopping when fn says so.
func operatorEach(ctx context.Context, iterable object.Object, fn func(object.Object) (bool, object.Object)) object.Object {
	next, ok := object.IterSource(iterable)
	if !ok {
		return errors.NewTypeErrorTagged("'%s' object is not iterable", object.PythonTypeName(iterable))
	}
	for {
		v, more := next()
		if !more {
			return nil
		}
		if object.IsPropagating(v) {
			return v
		}
		stop, err := fn(v)
		if err != nil {
			return err
		}
		if stop {
			return nil
		}
	}
}

// operatorIsOrEq is the identity-then-equality test countOf/indexOf use.
func operatorIsOrEq(ctx context.Context, a, b object.Object) (bool, object.Object) {
	if a == b {
		return true, nil
	}
	return object.Equal(ctx, a, b)
}

var OperatorLibrary = object.NewLibrary(OperatorLibraryName, operatorFunctions(), nil,
	"Standard operators as functions, and the itemgetter, attrgetter and methodcaller key-function factories")
