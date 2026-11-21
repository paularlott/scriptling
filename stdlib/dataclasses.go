package stdlib

import (
	"context"
	"slices"

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
			if kwargs.Len() > 0 {
				if err := errors.CheckKwargs("dataclass", kwargs, "init", "repr", "eq", "order", "frozen", "unsafe_hash", "match_args", "kw_only", "slots", "weakref_slot"); err != nil {
					return err
				}
			}
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
					if kwargs.Len() > 0 {
						if err := errors.CheckKwargs("dataclass", kwargs, "init", "repr", "eq", "order", "frozen", "unsafe_hash", "match_args", "kw_only", "slots", "weakref_slot"); err != nil {
							return err
						}
					}
					if len(args) != 1 {
						return errors.NewTypeErrorTagged("dataclass decorator requires the class")
					}
					cls, ok := args[0].(*object.Class)
					if !ok {
						return errors.NewTypeError("class", args[0].Type().String())
					}
					merged := map[string]object.Object{}
					for k, v := range opts.All() {
						merged[k] = v
					}
					for k, v := range kwargs.All() {
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
			if kwargs.Len() > 0 {
				if err := errors.CheckKwargs("field", kwargs, "default", "default_factory", "init", "repr", "hash", "compare", "metadata", "kw_only"); err != nil {
					return err
				}
			}
			if err := errors.ExactArgs(args, 0); err != nil {
				return err
			}
			inst := object.NewInstance(fieldSpecClass)
			def, hasDefault := kwargs.Lookup("default")
			if !hasDefault {
				def = dataclassesMissing
			}
			inst.SetField("default", def)
			factory, hasFactory := kwargs.Lookup("default_factory")
			if !hasFactory {
				factory = &object.Null{}
			}
			inst.SetField("default_factory", factory)
			// init=False leaves the field out of __init__, repr=False out of
			// repr(), compare=False out of == and ordering.
			for _, opt := range []string{"init", "repr", "compare"} {
				v := object.Object(object.NewBoolean(true))
				if o, ok := kwargs.Lookup(opt); ok {
					v = object.NewBoolean(isTruthy(o))
				}
				inst.SetField(opt, v)
			}
			return inst
		},
		HelpText: `field(default=..., default_factory=...) - Describe a dataclass field

default is the field's default value; default_factory is a zero-argument
callable producing one (for mutable defaults like lists).`,
	},
	"asdict": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("asdict", kwargs); err != nil {
				return err
			}
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			if inst, ok := args[0].(*object.Instance); !ok || inst.Class.FieldNames == nil {
				return errors.NewTypeErrorTagged("asdict() should be called on dataclass instances")
			}
			return dataclassesAsDict(args[0])
		},
		HelpText: "asdict(obj) - Recursively convert a dataclass instance to a dict",
	},
	"astuple": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("astuple", kwargs); err != nil {
				return err
			}
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
			object.TrackSlice(elems)
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
	factory object.Object // a zero-argument callable, or nil
	noInit  bool
	noRepr  bool
	noCmp   bool
}

// buildDataclass generates the methods on cls per the class's annotated
// fields. init/repr/eq options default to True, as in Python.
func buildDataclass(cls *object.Class, kwargs object.Kwargs) object.Object {
	opt := func(name string) bool {
		if v, ok := kwargs.Lookup(name); ok {
			if b, ok := v.(*object.Boolean); ok {
				return b.BoolValue()
			}
		}
		return true
	}
	// own reports whether the class body itself defines name (methods
	// are copied down from bases, so an inherited one equals the base's).
	// Python never replaces a method the class defines.
	own := func(name string) bool {
		m, ok := cls.Methods[name]
		if !ok {
			return false
		}
		return cls.BaseClass == nil || cls.BaseClass.Methods[name] != m
	}
	order := false
	if v, ok := kwargs.Lookup("order"); ok {
		if b, ok := v.(*object.Boolean); ok {
			order = b.BoolValue()
		}
	}
	if order && !opt("eq") {
		return errors.NewValueError("eq must be true if order is true")
	}
	if order {
		for _, name := range []string{"__lt__", "__le__", "__gt__", "__ge__"} {
			if own(name) {
				return errors.NewTypeErrorTagged("Cannot overwrite attribute %s in class %s. Consider using functools.total_ordering", name, cls.Name)
			}
		}
	}

	// A dataclass subclass has its bases' fields first, as in Python; a
	// field it redeclares keeps its place.
	var allNames []string
	var chain []*object.Class
	for c := cls; c != nil; c = c.BaseClass {
		chain = append([]*object.Class{c}, chain...)
	}
	for _, c := range chain {
		if c != cls {
			if _, isDC := c.Methods["__dataclass_fields__"]; !isDC {
				continue
			}
		}
		for _, n := range c.FieldNames {
			if !slices.Contains(allNames, n) {
				allNames = append(allNames, n)
			}
		}
	}
	cls.FieldNames = allNames
	cls.Methods["__dataclass_fields__"] = object.NewBoolean(true)

	fields := make([]dcField, 0, len(cls.FieldNames))
	for _, name := range cls.FieldNames {
		f := dcField{name: name}
		if attr, ok := cls.LookupMember(name); ok {
			if spec, isSpec := attr.(*object.Instance); isSpec && spec.Class == fieldSpecClass {
				if d, isSent := spec.Field("default").(*object.Sentinel); isSent && d.Name == "MISSING" {
					// default_factory path: any zero-argument callable
					// (list, a lambda, a class).
					if fac := spec.Field("default_factory"); fac != nil && fac.Type() != object.NULL_OBJ {
						f.hasDef = true
						f.factory = fac
					}
				} else {
					f.hasDef = true
					f.def = spec.Field("default")
				}
				off := func(name string) bool {
					b, ok := spec.Field(name).(*object.Boolean)
					return ok && !b.BoolValue()
				}
				f.noInit, f.noRepr, f.noCmp = off("init"), off("repr"), off("compare")
			} else {
				f.hasDef = true
				f.def = attr
			}
		}
		fields = append(fields, f)
	}

	frozen := false
	if v, ok := kwargs.Lookup("frozen"); ok {
		if b, ok := v.(*object.Boolean); ok {
			frozen = b.BoolValue()
		}
	}
	if frozen {
		// Frozen dataclasses block field writes after construction.
		cls.Methods["__frozen__"] = object.NewBoolean(true)
	}
	// __hash__ follows Python's table: generated for eq and frozen (or
	// unsafe_hash), None for eq without frozen, and left alone when the
	// class body defines its own or eq is off.
	unsafeHash := false
	if v, ok := kwargs.Lookup("unsafe_hash"); ok {
		if b, ok := v.(*object.Boolean); ok {
			unsafeHash = b.BoolValue()
		}
	}
	explicitHash := false
	if h, ok := cls.Methods["__hash__"]; ok && (cls.BaseClass == nil || cls.BaseClass.Methods["__hash__"] != h) {
		_, isNone := h.(*object.Null)
		_, ownEq := cls.Methods["__eq__"]
		explicitHash = !isNone || !ownEq
	}
	switch {
	case unsafeHash && explicitHash:
		return errors.NewTypeErrorTagged("Cannot overwrite attribute __hash__ in class %s", cls.Name)
	case explicitHash || !opt("eq"):
	case unsafeHash || frozen:
		cls.Methods["__hash__"] = &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.NoKwargs("__hash__", kwargs); err != nil {
					return err
				}
				self, ok := args[0].(*object.Instance)
				if !ok {
					return errors.NewTypeErrorTagged("__hash__ requires an instance")
				}
				// Hash the tuple of the compared fields, as Python does.
				values := make([]object.Object, 0, len(fields))
				for _, f := range fields {
					if f.noCmp {
						continue
					}
					v, _ := self.GetField(f.name)
					if v == nil {
						v = &object.Null{}
					}
					values = append(values, v)
				}
				key, errObj := object.HashKey(ctx, &object.Tuple{Elements: values})
				if errObj != nil {
					return errObj
				}
				h := uint64(14695981039346656037)
				for i := 0; i < len(key); i++ {
					h ^= uint64(key[i])
					h *= 1099511628211
				}
				return object.NewInteger(int64(h))
			},
			HelpText: "__hash__ - value hash generated by @dataclass",
		}
	default:
		cls.Methods["__hash__"] = &object.Null{}
	}

	if opt("init") && !own("__init__") {
		var names []string
		var initFields []dcField
		for _, f := range fields {
			if !f.noInit {
				names = append(names, f.name)
				initFields = append(initFields, f)
			}
		}
		cls.Methods["__init__"] = &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) < 1 {
					return errors.NewTypeErrorTagged("__init__ requires self")
				}
				self, ok := args[0].(*object.Instance)
				if !ok {
					return errors.NewTypeErrorTagged("__init__ requires instance as first argument")
				}
				positional := args[1:]
				if len(positional) > len(initFields) {
					return errors.NewTypeErrorTagged("%s.__init__() takes %d positional arguments but %d were given", cls.Name, len(initFields)+1, len(positional)+1)
				}
				byName := map[string]object.Object{}
				for k, v := range kwargs.All() {
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
				for i, f := range initFields {
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
						res := callCallable(ctx, f.factory)
						if object.IsError(res) || res.Type() == object.EXCEPTION_OBJ {
							return res
						}
						self.SetField(f.name, res)
					} else {
						self.SetField(f.name, f.def)
					}
				}
				// init=False fields take their default, if any.
				for _, f := range fields {
					if !f.noInit || !f.hasDef {
						continue
					}
					if f.factory != nil {
						res := callCallable(ctx, f.factory)
						if object.IsError(res) || res.Type() == object.EXCEPTION_OBJ {
							return res
						}
						self.SetField(f.name, res)
					} else {
						self.SetField(f.name, f.def)
					}
				}
				// __post_init__ runs last, as in Python.
				if post, ok := cls.LookupMember("__post_init__"); ok {
					if res := callCallable(ctx, post, self); object.IsError(res) || res.Type() == object.EXCEPTION_OBJ {
						return res
					}
				}
				return &object.Null{}
			},
			HelpText: "__init__ - generated by @dataclass",
		}
	}

	if opt("repr") && !own("__repr__") {
		cls.Methods["__repr__"] = &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.NoKwargs("__repr__", kwargs); err != nil {
					return err
				}
				self, ok := args[0].(*object.Instance)
				if !ok {
					return errors.NewTypeErrorTagged("__repr__ requires an instance")
				}
				var names []string
				for _, f := range fields {
					if !f.noRepr {
						names = append(names, f.name)
					}
				}
				// A dataclass that holds itself renders the inner one as "...".
				rctx, entered, errObj := object.EnterRepr(ctx, self)
				if errObj != nil {
					return errObj
				}
				if !entered {
					return object.NewString("...")
				}
				defer object.LeaveRepr()
				display, errObj := fieldsRepr(rctx, cls.Name, names, self.GetField)
				if errObj != nil {
					return errObj
				}
				return object.NewString(display)
			},
			HelpText: "__repr__ - generated by @dataclass",
		}
	}

	if order {
		// Tuple-order over the fields, as Python's generated comparison
		// methods. compareFields returns -1/0/1 field-lexicographically.
		compareFields := func(ctx context.Context, a, b *object.Instance) (int, object.Object) {
			for _, f := range fields {
				if f.noCmp {
					continue
				}
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
				// Any other pair orders as the script's own < would: tuples,
				// lists, mixed numbers, and objects with their own __lt__.
				// Compared as one-element tuples, so values that are equal
				// but unordered (two equal instances) pass, as in the
				// tuple comparison Python generates.
				cmp, errObj := object.Compare(ctx, &object.Tuple{Elements: []object.Object{av}}, &object.Tuple{Elements: []object.Object{bv}})
				if errObj != nil {
					return 0, errObj
				}
				if cmp != 0 {
					return cmp, nil
				}
			}
			return 0, nil
		}
		orderMethod := func(name string, keep func(cmp int) bool) object.Object {
			return &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.NoKwargs(name, kwargs); err != nil {
						return err
					}
					if len(args) != 2 {
						return errors.NewArgumentError(len(args), 2)
					}
					a, okA := args[0].(*object.Instance)
					b, okB := args[1].(*object.Instance)
					if !okA || !okB || a.Class != b.Class {
						// Only instances of the same class order, as in Python.
						return errors.NewTypeErrorTagged("'%s' not supported between instances of '%s' and '%s'", opSymbol[name], object.PythonTypeName(args[0]), object.PythonTypeName(args[1]))
					}
					cmp, errObj := compareFields(ctx, a, b)
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

	if opt("eq") && !own("__eq__") {
		cls.Methods["__eq__"] = &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.NoKwargs("__eq__", kwargs); err != nil {
					return err
				}
				if len(args) != 2 {
					return errors.NewArgumentError(len(args), 2)
				}
				a, okA := args[0].(*object.Instance)
				b, okB := args[1].(*object.Instance)
				if !okA || !okB {
					return object.NewBoolean(false)
				}
				if a.Class != b.Class {
					return object.NewBoolean(false)
				}
				for _, f := range fields {
					if f.noCmp {
						continue
					}
					av, aok := a.GetField(f.name)
					bv, bok := b.GetField(f.name)
					if !aok || !bok {
						return object.NewBoolean(false)
					}
					// Fields compare as == does in a script, not by their repr.
					eq, errObj := object.Equal(ctx, av, bv)
					if errObj != nil {
						return errObj
					}
					if !eq {
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

// opSymbol maps an ordering dunder to its operator, for error messages.
var opSymbol = map[string]string{"__lt__": "<", "__le__": "<=", "__gt__": ">", "__ge__": ">="}

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
			d := object.NewDictSized(len(v.Class.FieldNames))
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
		object.TrackSlice(elems)
		return &object.List{Elements: elems}
	case *object.Tuple:
		elems := make([]object.Object, len(v.Elements))
		for i, e := range v.Elements {
			elems[i] = dataclassesAsDict(e)
		}
		object.TrackSlice(elems)
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

// dataclasses.fields, is_dataclass and replace.
func init() {
	fns := DataclassesLibrary.Functions()
	classOf := func(o object.Object) *object.Class {
		switch v := o.(type) {
		case *object.Class:
			return v
		case *object.Instance:
			return v.Class
		}
		return nil
	}
	isDC := func(o object.Object) bool {
		c := classOf(o)
		if c == nil {
			return false
		}
		_, ok := c.Methods["__dataclass_fields__"]
		return ok
	}
	fns["is_dataclass"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("is_dataclass", kwargs); err != nil {
				return err
			}
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			return object.NewBoolean(isDC(args[0]))
		},
		HelpText: "is_dataclass(obj) - Whether obj is a dataclass or an instance of one",
	}
	fns["fields"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("fields", kwargs); err != nil {
				return err
			}
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			if !isDC(args[0]) {
				return errors.NewTypeErrorTagged("must be called with a dataclass type or instance")
			}
			c := classOf(args[0])
			out := make([]object.Object, 0, len(c.FieldNames))
			for _, n := range c.FieldNames {
				f := object.NewInstance(fieldSpecClass)
				f.SetField("name", object.NewString(n))
				def := object.Object(dataclassesMissing)
				factory := object.Object(dataclassesMissing)
				if attr, ok := c.LookupMember(n); ok {
					if spec, isSpec := attr.(*object.Instance); isSpec && spec.Class == fieldSpecClass {
						def = spec.Field("default")
						if fac := spec.Field("default_factory"); fac != nil && fac.Type() != object.NULL_OBJ {
							factory = fac
						}
					} else {
						def = attr
					}
				}
				f.SetField("default", def)
				f.SetField("default_factory", factory)
				out = append(out, f)
			}
			return &object.Tuple{Elements: out}
		},
		HelpText: "fields(dataclass) - The dataclass's fields, each with .name, .default and .default_factory",
	}
	fns["replace"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			var inst *object.Instance
			ok := false
			if len(args) == 1 {
				inst, ok = args[0].(*object.Instance)
			}
			if !ok || !isDC(inst) {
				for k := range kwargs.All() {
					return errors.NewTypeErrorTagged("replace() got an unexpected keyword argument '%s'", k)
				}
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}
				return errors.NewTypeErrorTagged("replace() should be called on dataclass instances")
			}
			c := inst.Class
			vals := map[string]object.Object{}
			for _, n := range c.FieldNames {
				if v, has := inst.GetField(n); has {
					vals[n] = v
				}
			}
			for k, v := range kwargs.All() {
				if _, known := vals[k]; !known && !slices.Contains(c.FieldNames, k) {
					return errors.NewTypeErrorTagged("%s.__init__() got an unexpected keyword argument '%s'", c.Name, k)
				}
				vals[k] = v
			}
			// A new instance through the generated __init__ (init=False
			// fields are left to it).
			initFn, ok := c.Methods["__init__"].(*object.Builtin)
			if !ok {
				return errors.NewTypeErrorTagged("replace() needs a dataclass with a generated __init__")
			}
			fresh := object.NewInstance(c)
			var names []string
			var values []object.Object
			for _, n := range c.FieldNames {
				if attr, ok := c.LookupMember(n); ok {
					if spec, isSpec := attr.(*object.Instance); isSpec && spec.Class == fieldSpecClass {
						if b, ok := spec.Field("init").(*object.Boolean); ok && !b.BoolValue() {
							continue
						}
					}
				}
				if v, ok := vals[n]; ok {
					names = append(names, n)
					values = append(values, v)
				}
			}
			if r := initFn.Fn(ctx, object.NewKwargsFrom(names, values), fresh); object.IsError(r) || r.Type() == object.EXCEPTION_OBJ {
				return r
			}
			return fresh
		},
		HelpText: "replace(obj, **changes) - A copy of a dataclass instance with some fields changed",
	}
}
