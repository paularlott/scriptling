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
}, map[string]object.Object{
	"MISSING": dataclassesMissing,
}, "Dataclass support (@dataclass, field, MISSING)")

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

var _ = context.Background
