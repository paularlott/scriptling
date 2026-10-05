package stdlib

import (
	"context"
	"strings"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// DataclassesLibrary provides @dataclass, field() and MISSING. The decorator
// reads the class's annotated field names (source order) and synthesizes
// __init__/__repr__/__eq__, mirroring Python's generated methods.

// dataclasses.MISSING marks absent defaults; field() instances carry
// default/default_factory through the class attributes.
var dataclassesMissing = object.NewSentinel("MISSING", "MISSING")

var fieldSpecClass = &object.Class{
	Name:    "field",
	Methods: map[string]object.Object{},
}

var DataclassesLibrary = object.NewLibrary(DataclassesLibraryName, map[string]*object.Builtin{
	"dataclass": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			// Both decorator forms: @dataclass (the class itself) and
			// @dataclass(init=...) returning the decorator.
			if len(args) > 0 {
				if cls, ok := args[0].(*object.Class); ok {
					return buildDataclass(cls, kwargs)
				}
			}
			opts := kwargs
			return &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if len(args) != 1 {
						return errors.NewError("dataclass decorator requires the class")
					}
					cls, ok := args[0].(*object.Class)
					if !ok {
						return errors.NewTypeError("class", args[0].Type().String())
					}
					merged := map[string]object.Object{}
					for k, v := range opts.Kwargs {
						merged[k] = v
					}
					for k, v := range kwargs.Kwargs {
						merged[k] = v
					}
					return buildDataclass(cls, object.NewKwargs(merged))
				},
				HelpText: "dataclass(options) - decorator factory",
			}
		},
		HelpText: `dataclass(cls) or dataclass(init=, repr=, eq=) - Generate __init__, __repr__ and __eq__ from annotated class fields

Fields are the class body's annotated names in source order; values are
defaults, and field(default=..., default_factory=...) customises them.`,
	},
	"field": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 0); err != nil {
				return err
			}
			inst := object.NewInstance(fieldSpecClass)
			def, hasDefault := kwargs.Kwargs["default"]
			if !hasDefault {
				def = dataclassesMissing
			}
			inst.SetField("default", def)
			factory, hasFactory := kwargs.Kwargs["default_factory"]
			if !hasFactory {
				factory = &object.Null{}
			}
			inst.SetField("default_factory", factory)
			return inst
		},
		HelpText: `field(default=..., default_factory=...) - Describe a dataclass field

default is the field's default value; default_factory is a zero-argument
callable producing one (for mutable defaults like lists).`,
	},
	"asdict": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			return dataclassesAsDict(args[0])
		},
		HelpText: "asdict(obj) - Recursively convert a dataclass instance to a dict",
	},
	"astuple": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			inst, ok := args[0].(*object.Instance)
			if !ok || inst.Class.FieldNames == nil {
				return errors.NewTypeError("dataclass instance", getTypeNameFor(args[0]))
			}
			elems := []object.Object{}
			for _, name := range inst.Class.FieldNames {
				if v, has := inst.GetField(name); has {
					elems = append(elems, v)
				} else {
					elems = append(elems, &object.Null{})
				}
			}
			return &object.Tuple{Elements: elems}
		},
		HelpText: "astuple(obj) - Convert a dataclass instance to a tuple of field values",
	},
}, map[string]object.Object{
	"MISSING": dataclassesMissing,
}, "Dataclass support (@dataclass, field, MISSING, asdict, astuple)")

// dcField is one @dataclass field with its resolved default.
type dcField struct {
	name    string
	hasDef  bool
	def     object.Object
	factory *object.Builtin
}

// buildDataclass generates the methods on cls per the class's annotated
// fields. init/repr/eq options default to True, as in Python.
func buildDataclass(cls *object.Class, kwargs object.Kwargs) object.Object {
	opt := func(name string) bool {
		if v, ok := kwargs.Kwargs[name]; ok {
			if b, ok := v.(*object.Boolean); ok {
				return b.BoolValue()
			}
		}
		return true
	}

	fields := make([]dcField, 0, len(cls.FieldNames))
	for _, name := range cls.FieldNames {
		f := dcField{name: name}
		if attr, ok := cls.Methods[name]; ok {
			if spec, isSpec := attr.(*object.Instance); isSpec && spec.Class == fieldSpecClass {
				if d, isSent := spec.Field("default").(*object.Sentinel); isSent && d.Name == "MISSING" {
					// default_factory path
					if fac, isFn := spec.Field("default_factory").(*object.Builtin); isFn {
						f.hasDef = true
						f.factory = fac
					}
				} else {
					f.hasDef = true
					f.def = spec.Field("default")
				}
			} else {
				f.hasDef = true
				f.def = attr
			}
		}
		fields = append(fields, f)
	}

	frozen := false
	if v, ok := kwargs.Kwargs["frozen"]; ok {
		if b, ok := v.(*object.Boolean); ok {
			frozen = b.BoolValue()
		}
	}
	if frozen {
		// Frozen dataclasses block field writes after construction and
		// hash by field values (Python: eq=True+FrozenInstanceError path).
		cls.Methods["__frozen__"] = object.NewBoolean(true)
		cls.Methods["__hash__"] = &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				self, ok := args[0].(*object.Instance)
				if !ok {
					return errors.NewError("__hash__ requires an instance")
				}
				h := uint64(14695981039346656037)
				for _, f := range fields {
					if v, has := self.GetField(f.name); has {
						for _, c := range f.name + ":" + v.Inspect() {
							h ^= uint64(c)
							h *= 1099511628211
						}
					}
				}
				return object.NewInteger(int64(h))
			},
			HelpText: "__hash__ - value hash for frozen dataclasses",
		}
	}

	if opt("init") {
		names := make([]string, len(fields))
		for i, f := range fields {
			names[i] = f.name
		}
		cls.Methods["__init__"] = &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) < 1 {
					return errors.NewError("__init__ requires self")
				}
				self, ok := args[0].(*object.Instance)
				if !ok {
					return errors.NewError("__init__ requires instance as first argument")
				}
				positional := args[1:]
				byName := map[string]object.Object{}
				for k, v := range kwargs.Kwargs {
					known := false
					for _, n := range names {
						if n == k {
							known = true
							break
						}
					}
					if !known {
						return &object.Exception{
							Message:       "__init__() got an unexpected keyword argument '" + k + "'",
							ExceptionType: object.ExceptionTypeTypeError,
							Raised:        true,
						}
					}
					byName[k] = v
				}
				for i, f := range fields {
					if i < len(positional) {
						if _, dup := byName[f.name]; dup {
							return &object.Exception{
								Message:       "__init__() got multiple values for argument '" + f.name + "'",
								ExceptionType: object.ExceptionTypeTypeError,
								Raised:        true,
							}
						}
						self.SetField(f.name, positional[i])
						continue
					}
					if v, ok := byName[f.name]; ok {
						self.SetField(f.name, v)
						continue
					}
					if !f.hasDef {
						return &object.Exception{
							Message:       cls.Name + ".__init__() missing 1 required positional argument: '" + f.name + "'",
							ExceptionType: object.ExceptionTypeTypeError,
							Raised:        true,
						}
					}
					if f.factory != nil {
						res := f.factory.Fn(ctx, object.NewKwargs(nil))
						if object.IsError(res) || res.Type() == object.EXCEPTION_OBJ {
							return res
						}
						self.SetField(f.name, res)
					} else {
						self.SetField(f.name, f.def)
					}
				}
				return &object.Null{}
			},
			HelpText: "__init__ - generated by @dataclass",
		}
	}

	if opt("repr") {
		cls.Methods["__repr__"] = &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				self, ok := args[0].(*object.Instance)
				if !ok {
					return errors.NewError("__repr__ requires an instance")
				}
				parts := make([]string, 0, len(fields))
				for _, f := range fields {
					if v, has := self.GetField(f.name); has {
						parts = append(parts, f.name+"="+reprValue(v))
					}
				}
				return object.NewString(cls.Name + "(" + strings.Join(parts, ", ") + ")")
			},
			HelpText: "__repr__ - generated by @dataclass",
		}
	}

	if opt("order") {
		// Tuple-order over the fields, as Python's generated comparison
		// methods. compareFields returns -1/0/1 field-lexicographically.
		compareFields := func(a, b *object.Instance) (int, object.Object) {
			for _, f := range fields {
				av, aok := a.GetField(f.name)
				bv, bok := b.GetField(f.name)
				if !aok || !bok {
					continue
				}
				ai, aIsInt := av.(*object.Integer)
				bi, bIsInt := bv.(*object.Integer)
				if aIsInt && bIsInt {
					switch {
					case ai.IntValue() < bi.IntValue():
						return -1, nil
					case ai.IntValue() > bi.IntValue():
						return 1, nil
					}
					continue
				}
				af, aIsFloat := av.(*object.Float)
				bf, bIsFloat := bv.(*object.Float)
				if aIsFloat && bIsFloat {
					switch {
					case af.FloatValue() < bf.FloatValue():
						return -1, nil
					case af.FloatValue() > bf.FloatValue():
						return 1, nil
					}
					continue
				}
				as, aIsStr := av.(*object.String)
				bs, bIsStr := bv.(*object.String)
				if aIsStr && bIsStr {
					switch {
					case as.StringValue() < bs.StringValue():
						return -1, nil
					case as.StringValue() > bs.StringValue():
						return 1, nil
					}
					continue
				}
				return 0, errors.NewTypeErrorTagged("'<' not supported between instances of '%s' and '%s'", getTypeNameFor(av), getTypeNameFor(bv))
			}
			return 0, nil
		}
		orderMethod := func(name string, keep func(cmp int) bool) object.Object {
			return &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if len(args) != 2 {
						return errors.NewArgumentError(len(args), 2)
					}
					a, okA := args[0].(*object.Instance)
					b, okB := args[1].(*object.Instance)
					if !okA || !okB {
						return errors.NewTypeErrorTagged("'%s' not supported between dataclass and non-dataclass values", name)
					}
					cmp, errObj := compareFields(a, b)
					if errObj != nil {
						return errObj
					}
					return object.NewBoolean(keep(cmp))
				},
				HelpText: name + " - generated by @dataclass(order=True)",
			}
		}
		cls.Methods["__lt__"] = orderMethod("__lt__", func(c int) bool { return c < 0 })
		cls.Methods["__le__"] = orderMethod("__le__", func(c int) bool { return c <= 0 })
		cls.Methods["__gt__"] = orderMethod("__gt__", func(c int) bool { return c > 0 })
		cls.Methods["__ge__"] = orderMethod("__ge__", func(c int) bool { return c >= 0 })
	}

	if opt("eq") {
		cls.Methods["__eq__"] = &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) != 2 {
					return errors.NewArgumentError(len(args), 2)
				}
				a, okA := args[0].(*object.Instance)
				b, okB := args[1].(*object.Instance)
				if !okA || !okB {
					return object.NewBoolean(false)
				}
				for _, f := range fields {
					av, aok := a.GetField(f.name)
					bv, bok := b.GetField(f.name)
					if !aok || !bok || av.Inspect() != bv.Inspect() {
						return object.NewBoolean(false)
					}
				}
				return object.NewBoolean(true)
			},
			HelpText: "__eq__ - generated by @dataclass",
		}
	}

	return cls
}

func getTypeNameFor(v object.Object) string {
	if s, err := v.AsString(); err == nil {
		return s
	}
	return v.Type().String()
}

var _ = context.Background

// dataclassesAsDict recursively converts a dataclass instance (and the
// containers holding them) to plain dicts/lists.
func dataclassesAsDict(obj object.Object) object.Object {
	switch v := obj.(type) {
	case *object.Instance:
		if v.Class != nil && v.Class.FieldNames != nil {
			d := &object.Dict{Pairs: make(map[string]object.DictPair, len(v.Class.FieldNames))}
			for _, name := range v.Class.FieldNames {
				if fv, has := v.GetField(name); has {
					d.SetByString(name, dataclassesAsDict(fv))
				}
			}
			return d
		}
		return v
	case *object.List:
		elems := make([]object.Object, len(v.Elements))
		for i, e := range v.Elements {
			elems[i] = dataclassesAsDict(e)
		}
		return &object.List{Elements: elems}
	case *object.Tuple:
		elems := make([]object.Object, len(v.Elements))
		for i, e := range v.Elements {
			elems[i] = dataclassesAsDict(e)
		}
		return &object.Tuple{Elements: elems}
	case *object.Dict:
		d := object.NewDict()
		for _, pair := range v.OrderedPairs() {
			d.Store(object.DictKey(pair.Key), pair.Key, dataclassesAsDict(pair.Value))
		}
		return d
	}
	return obj
}
