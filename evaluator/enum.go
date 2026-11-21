package evaluator

import (
	"context"
	"fmt"
	"strings"

	"github.com/paularlott/scriptling/ast"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// buildEnumMembers turns a class deriving from enum.Enum into an enum: each
// plain top-level assignment (in source order) becomes a singleton member
// instance with name/value fields; auto() markers become sequential ints;
// duplicate values alias to the first member, as in Python. Members are
// exposed as class attributes, collected into EnumMembers (iteration order)
// and a __members__ dict.
func isEnumBaseName(name string) bool {
	switch name {
	case "Enum", "IntEnum", "StrEnum", "Flag", "IntFlag":
		return true
	}
	return false
}

func buildEnumMembers(class *object.Class) object.Object {
	kind := class.BaseClass.Name
	isFlag := kind == "Flag" || kind == "IntFlag"
	// auto() continues from the previous member's integer value (1 when it
	// is the first member); a non-integer previous value cannot increment.
	lastInt := int64(0)
	hasInt := false
	seen := 0
	members := make([]object.Object, 0, len(class.AssignNames))
	byKey := ""
	_ = byKey

	newMember := func(name string, value object.Object) *object.Instance {
		inst := object.NewInstance(class)
		inst.SetField("name", object.NewString(name))
		inst.SetField("value", value)
		inst.SetField("__str_repr__", object.NewString(class.Name+"."+name))
		return inst
	}

	for _, mname := range class.AssignNames {
		if len(mname) > 0 && mname[0] == '_' {
			continue
		}
		val, ok := class.Methods[mname]
		if !ok {
			continue
		}
		// auto() placeholders take the next sequential value.
		if s, isSentinel := val.(*object.Sentinel); isSentinel && s.Name == "auto" && kind == "StrEnum" {
			// StrEnum's auto() is the member name in lower case.
			val = object.NewString(strings.ToLower(mname))
		} else if s, isSentinel := val.(*object.Sentinel); isSentinel && s.Name == "auto" && isFlag {
			// Flag's auto() is the next power of two.
			next := int64(1)
			for next <= lastInt {
				next <<= 1
			}
			lastInt, hasInt = next, true
			val = object.NewInteger(next)
		} else if s, isSentinel := val.(*object.Sentinel); isSentinel && s.Name == "auto" {
			if hasInt {
				lastInt++
				val = object.NewInteger(lastInt)
			} else if seen == 0 {
				lastInt = 1
				val = object.NewInteger(1)
			} else {
				return &object.Exception{
					Message:       fmt.Sprintf("cannot generate the next value for %s.%s: unable to increment the previous value", class.Name, mname),
					ExceptionType: object.ExceptionTypeTypeError,
					Raised:        true,
				}
			}
			hasInt = true
		} else if iv, isInt := val.(*object.Integer); isInt {
			lastInt = iv.IntValue()
			hasInt = true
		}
		seen++
		// A value equal to an existing member's value aliases to it
		// (Python: Aliased.B is Aliased.A, one member in iteration).
		if alias := aliasEnumMember(members, val); alias != nil {
			class.Methods[mname] = alias
			continue
		}
		member := newMember(mname, val)
		class.Methods[mname] = member
		members = append(members, member)
	}

	class.EnumMembers = members

	// __members__ maps names to members, Python-style.
	d := object.NewDictSized(len(members))
	for _, m := range members {
		member := m.(*object.Instance)
		name, _ := member.Field("name").(*object.String)
		d.SetByString(name.StringValue(), member)
	}
	class.Methods["__members__"] = d

	// IntEnum members compare equal to their plain values; every enum
	// member hashes by value so members work as dict keys.
	eq := &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) != 2 {
				return errors.NewArgumentError(len(args), 2)
			}
			self, ok := args[0].(*object.Instance)
			if !ok {
				return object.NewBoolean(false)
			}
			other, isInst := args[1].(*object.Instance)
			if isInst && other.Class == self.Class {
				sn, _ := self.Field("name").(*object.String)
				on, _ := other.Field("name").(*object.String)
				return object.NewBoolean(sn.StringValue() == on.StringValue())
			}
			if class.IsIntEnum {
				// IntEnum members equal numbers of their value: 200,
				// 200.0, and members of other IntEnums.
				other := args[1]
				if oi, ok := other.(*object.Instance); ok && oi.Class.IsIntEnum {
					other = oi.Field("value")
				}
				if eq, numeric := mixedNumbersEqual(self.Field("value"), other); numeric {
					return object.NewBoolean(eq)
				}
			}
			return object.NewBoolean(false)
		},
		HelpText: "__eq__(other) - Compare members (IntEnum also matches plain values)",
	}
	hash := &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			self, ok := args[0].(*object.Instance)
			if !ok {
				return errors.NewTypeErrorTagged("__hash__ requires an enum member")
			}
			v := self.Field("value")
			h := uint64(14695981039346656037)
			for _, c := range v.Inspect() {
				h ^= uint64(c)
				h *= 1099511628211
			}
			return object.NewInteger(int64(h))
		},
		HelpText: "__hash__() - Hash by value",
	}
	class.Methods["__eq__"] = eq
	class.Methods["__hash__"] = hash

	// IntEnum members convert to their int values (operators: see
	// addValueOperators).
	if class.IsIntEnum {
		toInt := &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				return args[0].(*object.Instance).Field("value")
			},
			HelpText: "__int__() - The member's value",
		}
		class.Methods["__int__"] = toInt
		class.Methods["__index__"] = toInt
	}
	// Python's str: the plain dotted name. repr (below) is the <Name:
	// value> form; __str__ wins in str() before the __repr__ method.
	class.Methods["__str__"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			self, ok := args[0].(*object.Instance)
			if !ok {
				return errors.NewTypeErrorTagged("__str__ requires an enum member")
			}
			name, _ := self.Field("name").(*object.String)
			return object.NewString(class.Name + "." + name.StringValue())
		},
		HelpText: "__str__() - Color.RED form",
	}
	// Python's repr: <Color.RED: 1>.
	class.Methods["__repr__"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			self, ok := args[0].(*object.Instance)
			if !ok {
				return errors.NewTypeErrorTagged("__repr__ requires an enum member")
			}
			name, _ := self.Field("name").(*object.String)
			value := self.Field("value")
			return object.NewString(fmt.Sprintf("<%s.%s: %s>", class.Name, name.StringValue(), object.ValueRepr(value)))
		},
		HelpText: "__repr__() - <Member: value> form",
	}
	// format() of a member formats its str(), as in Python (the value
	// kinds replace this with their value's format below).
	class.Methods["__format__"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("__format__", kwargs); err != nil {
				return err
			}
			if len(args) != 2 {
				return errors.NewArgumentError(len(args), 2)
			}
			text, errObj := renderConvertedValue(ctx, args[0], "s", GetEnvFromContext(ctx))
			if errObj != nil {
				return errObj
			}
			spec, _ := args[1].AsString()
			out, err := formatWithSpec(object.NewString(text), spec)
			if err != nil {
				return err
			}
			return object.NewString(out)
		},
		HelpText: "__format__(spec) - Format the member's str()",
	}
	if kind == "StrEnum" || class.IsIntEnum {
		class.Methods["__hash__"] = enumValueHash
		addValueOperators(class, kind == "StrEnum", isFlag)
	}
	switch {
	case kind == "StrEnum":
		addValueStrMethods(class)
		addStrEnumMethods(class)
	case isFlag:
		addFlagMethods(class)
		if kind == "IntFlag" {
			addValueStrMethods(class)
		}
	case kind == "IntEnum":
		// str() and format() of an IntEnum member are its value's, as in
		// Python 3.11.
		addValueStrMethods(class)
	}
	return nil
}

// addValueStrMethods makes str() and format() of a member those of its
// value (IntEnum, IntFlag, StrEnum).
func addValueStrMethods(class *object.Class) {
	class.Methods["__str__"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			self, ok := args[0].(*object.Instance)
			if !ok {
				return errors.NewTypeErrorTagged("__str__ requires an enum member")
			}
			v := self.Field("value")
			if s, ok := v.(*object.String); ok {
				return s
			}
			return object.NewString(v.Inspect())
		},
		HelpText: "__str__() - The member's value as text",
	}
	class.Methods["__format__"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			self, ok := args[0].(*object.Instance)
			if !ok || len(args) < 2 {
				return errors.NewTypeErrorTagged("__format__ requires an enum member and a spec")
			}
			spec, _ := args[1].AsString()
			out, err := formatWithSpec(self.Field("value"), spec)
			if err != nil {
				return err
			}
			return object.NewString(out)
		},
		HelpText: "__format__(spec) - Format the member's value",
	}
}

// addStrEnumMethods makes StrEnum members equal to their string values.
func addStrEnumMethods(class *object.Class) {
	baseEq := class.Methods["__eq__"].(*object.Builtin)
	class.Methods["__eq__"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) == 2 {
				if s, ok := args[1].(*object.String); ok {
					if self, ok := args[0].(*object.Instance); ok {
						if v, ok := self.Field("value").(*object.String); ok {
							return object.NewBoolean(v.StringValue() == s.StringValue())
						}
					}
				}
			}
			return baseEq.Fn(ctx, kwargs, args...)
		},
		HelpText: "__eq__(other) - Members equal their string values",
	}
}

// flagValue reads a Flag member's (or an IntFlag operand's) integer value.
func flagValue(o object.Object, class *object.Class) (int64, bool) {
	switch v := o.(type) {
	case *object.Instance:
		if v.Class != class {
			return 0, false
		}
		n, ok := v.Field("value").(*object.Integer)
		if !ok {
			return 0, false
		}
		return n.IntValue(), true
	case *object.Integer:
		if class.IsIntEnum {
			return v.IntValue(), true
		}
	}
	return 0, false
}

// flagMember is the member, or the combination of members, with value v:
// a combination is named by its members joined with |, as in Python.
func flagMember(class *object.Class, v int64) object.Object {
	for _, m := range class.EnumMembers {
		if n, ok := flagValue(m, class); ok && n == v {
			return m
		}
	}
	var names []string
	rest := v
	for _, m := range class.EnumMembers {
		n, _ := flagValue(m, class)
		if n != 0 && n&(n-1) == 0 && v&n == n {
			name, _ := m.(*object.Instance).Field("name").(*object.String)
			names = append(names, name.StringValue())
			rest &^= n
		}
	}
	inst := object.NewInstance(class)
	inst.SetField("value", object.NewInteger(v))
	if len(names) == 0 || rest != 0 {
		inst.SetField("name", &object.Null{})
		inst.SetField("__str_repr__", object.NewString(fmt.Sprintf("%s(%d)", class.Name, v)))
		return inst
	}
	joined := strings.Join(names, "|")
	inst.SetField("name", object.NewString(joined))
	inst.SetField("__str_repr__", object.NewString(class.Name+"."+joined))
	return inst
}

// addFlagMethods gives Flag and IntFlag members the bitwise operators,
// membership, truth and iteration over the members a value holds.
func addFlagMethods(class *object.Class) {
	all := int64(0)
	for _, m := range class.EnumMembers {
		if n, ok := flagValue(m, class); ok {
			all |= n
		}
	}
	bin := func(name string, op func(a, b int64) int64) {
		class.Methods[name] = &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) != 2 {
					return errors.NewArgumentError(len(args), 2)
				}
				a, okA := flagValue(args[0], class)
				b, okB := flagValue(args[1], class)
				if !okA || !okB {
					return notImplementedValue
				}
				return flagMember(class, op(a, b))
			},
			HelpText: name + "(other) - Combine flags",
		}
	}
	bin("__or__", func(a, b int64) int64 { return a | b })
	bin("__and__", func(a, b int64) int64 { return a & b })
	bin("__xor__", func(a, b int64) int64 { return a ^ b })
	class.Methods["__ror__"] = class.Methods["__or__"]
	class.Methods["__rand__"] = class.Methods["__and__"]
	class.Methods["__rxor__"] = class.Methods["__xor__"]
	class.Methods["__invert__"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			a, _ := flagValue(args[0], class)
			return flagMember(class, all&^a)
		},
		HelpText: "__invert__() - The flags not set",
	}
	class.Methods["__contains__"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) != 2 {
				return errors.NewArgumentError(len(args), 2)
			}
			a, _ := flagValue(args[0], class)
			b, ok := flagValue(args[1], class)
			if !ok {
				return errors.NewTypeErrorTagged("unsupported operand type(s) for 'in': '%s' and '%s'", getTypeName(args[1]), class.Name)
			}
			return nativeBoolToBooleanObject(a&b == b)
		},
		HelpText: "__contains__(flag) - Whether every bit of flag is set",
	}
	class.Methods["__bool__"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			a, _ := flagValue(args[0], class)
			return nativeBoolToBooleanObject(a != 0)
		},
		HelpText: "__bool__() - False for no flags",
	}
	class.Methods["__len__"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			a, _ := flagValue(args[0], class)
			n := 0
			for _, m := range class.EnumMembers {
				if v, _ := flagValue(m, class); v != 0 && v&(v-1) == 0 && a&v == v {
					n++
				}
			}
			return object.NewInteger(int64(n))
		},
		HelpText: "__len__() - How many flags are set",
	}
	class.Methods["__iter__"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			a, _ := flagValue(args[0], class)
			var out []object.Object
			for _, m := range class.EnumMembers {
				if v, _ := flagValue(m, class); v != 0 && v&(v-1) == 0 && a&v == v {
					out = append(out, m)
				}
			}
			return object.NewIterator(sliceNext(out))
		},
		HelpText: "__iter__() - The single flags set",
	}
	class.Methods["__eq__"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) != 2 {
				return errors.NewArgumentError(len(args), 2)
			}
			a, _ := flagValue(args[0], class)
			b, ok := flagValue(args[1], class)
			return nativeBoolToBooleanObject(ok && a == b)
		},
		HelpText: "__eq__(other) - Equal flag values",
	}
	class.Methods["__repr__"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			self := args[0].(*object.Instance)
			v, _ := flagValue(self, class)
			if name, ok := self.Field("name").(*object.String); ok {
				return object.NewString(fmt.Sprintf("<%s.%s: %d>", class.Name, name.StringValue(), v))
			}
			return object.NewString(fmt.Sprintf("<%s: %d>", class.Name, v))
		},
		HelpText: "__repr__() - <Flag.A|B: 3> form",
	}
	class.Methods["__str__"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			return object.NewString(args[0].(*object.Instance).Inspect())
		},
		HelpText: "__str__() - Flag.A|B form",
	}
}

// sliceNext pulls the elements of a slice in turn.
func sliceNext(elems []object.Object) func() (object.Object, bool) {
	i := 0
	return func() (object.Object, bool) {
		if i >= len(elems) {
			return nil, false
		}
		i++
		return elems[i-1], true
	}
}

// enumValuesEqual reports whether two member values are equal, as == sees
// them: 1 and 1.0 are, "1" and 1 are not.
func enumValuesEqual(a, b object.Object) bool {
	if a == b {
		return true
	}
	eq, errObj := deepEqual(nil, a, b, 0)
	return errObj == nil && eq
}

// aliasEnumMember returns the existing member whose value equals v, if any.
func aliasEnumMember(members []object.Object, v object.Object) *object.Instance {
	for _, m := range members {
		member := m.(*object.Instance)
		if enumValuesEqual(member.Field("value"), v) {
			return member
		}
	}
	return nil
}

// enumValueLookup implements Color(value): members are searched by value
// equality; a miss is Python's ValueError.
func enumValueLookup(class *object.Class, value object.Object) object.Object {
	if n, ok := value.(*object.Integer); ok && class.BaseClass != nil && (class.BaseClass.Name == "Flag" || class.BaseClass.Name == "IntFlag") {
		return flagMember(class, n.IntValue())
	}
	for _, m := range class.EnumMembers {
		member := m.(*object.Instance)
		if enumValuesEqual(member.Field("value"), value) {
			return member
		}
	}
	return &object.Exception{
		Message:       fmt.Sprintf("%s is not a valid %s", object.ValueRepr(value), class.Name),
		ExceptionType: object.ExceptionTypeValueError,
		Raised:        true,
	}
}

// enumFunctional is Enum("Color", names, *, start=1, ...): names is a string
// of names separated by spaces or commas, a list of names, a list of
// (name, value) pairs, or a dict of name to value. Unvalued names take
// auto() values (start, start+1, ...; powers of two for flags; lower-case
// names for StrEnum).
func enumFunctional(base *object.Class, args []object.Object, keywords object.Kwargs) object.Object {
	if keywords.Len() > 0 {
		if err := errors.CheckKwargs(base.Name, keywords, "names", "module", "qualname", "type", "start", "boundary", "value"); err != nil {
			return err
		}
	}
	if v := keywords.Get("value"); v != nil {
		args = append([]object.Object{v}, args...)
	}
	if v := keywords.Get("names"); v != nil {
		args = append(args, v)
	}
	if len(args) != 2 {
		return errors.NewTypeErrorTagged("%s() takes 2 positional arguments (the class name and its member names) but %d were given", base.Name, len(args))
	}
	name, ok := args[0].(*object.String)
	if !ok {
		return errors.NewTypeErrorTagged("%s() argument 1 must be str, not %s", base.Name, object.PythonTypeName(args[0]))
	}
	start := int64(1)
	if s := keywords.Get("start"); s != nil {
		n, ok := s.(*object.Integer)
		if !ok {
			return errors.NewTypeErrorTagged("start must be an integer")
		}
		start = n.IntValue()
	}
	isFlag := base.Name == "Flag" || base.Name == "IntFlag"
	next := start
	autoValue := func() object.Object {
		if isFlag || base.Name == "StrEnum" {
			return object.NewSentinel("auto", "auto")
		}
		v := object.NewInteger(next)
		next++
		return v
	}
	var names []string
	values := map[string]object.Object{}
	add := func(n string, v object.Object) {
		if _, dup := values[n]; !dup {
			names = append(names, n)
		}
		values[n] = v
	}
	switch src := args[1].(type) {
	case *object.String:
		for _, f := range strings.FieldsFunc(src.StringValue(), func(r rune) bool { return r == ' ' || r == ',' }) {
			add(f, autoValue())
		}
	case *object.Dict:
		for _, p := range src.OrderedPairs() {
			k, ok := p.Key.(*object.String)
			if !ok {
				return errors.NewTypeErrorTagged("enum member names must be strings")
			}
			add(k.StringValue(), p.Value)
		}
	case *object.List, *object.Tuple:
		var elems []object.Object
		if l, ok := src.(*object.List); ok {
			elems = l.Elements
		} else {
			elems = src.(*object.Tuple).Elements
		}
		for _, e := range elems {
			switch item := e.(type) {
			case *object.String:
				add(item.StringValue(), autoValue())
			case *object.Tuple, *object.List:
				var pair []object.Object
				if t, ok := item.(*object.Tuple); ok {
					pair = t.Elements
				} else {
					pair = item.(*object.List).Elements
				}
				k, ok := pair[0].(*object.String)
				if len(pair) != 2 || !ok {
					return errors.NewTypeErrorTagged("enum members must be names or (name, value) pairs")
				}
				add(k.StringValue(), pair[1])
			default:
				return errors.NewTypeErrorTagged("enum members must be names or (name, value) pairs")
			}
		}
	default:
		return errors.NewTypeErrorTagged("enum member names must be a string, a sequence or a mapping, not %s", object.PythonTypeName(args[1]))
	}
	class := &object.Class{
		Name:        name.StringValue(),
		BaseClass:   base,
		Methods:     make(map[string]object.Object, len(names)+4),
		AssignNames: names,
		IsEnum:      true,
		IsIntEnum:   base.Name == "IntEnum" || base.Name == "IntFlag",
	}
	for mname, m := range base.Methods {
		class.Methods[mname] = m
	}
	for _, n := range names {
		class.Methods[n] = values[n]
	}
	if err := buildEnumMembers(class); err != nil {
		return err
	}
	return class
}

// enumMissing is Color(value) for a value no member has, on a class that
// defines _missing_: its result is the member, or None for the ValueError.
func enumMissing(ctx context.Context, class *object.Class, value, notFound object.Object, env *object.Environment) object.Object {
	fn, _ := class.LookupMember("_missing_")
	callArgs := []object.Object{value}
	switch m := fn.(type) {
	case *object.ClassMethod:
		fn, callArgs = m.Fn, []object.Object{class, value}
	case *object.StaticMethod:
		fn = m.Fn
	}
	r := applyFunctionWithContext(ctx, fn, callArgs, object.Kwargs{}, env)
	if propagates(r) {
		return r
	}
	if _, none := r.(*object.Null); none {
		return notFound
	}
	if inst, ok := r.(*object.Instance); ok && inst.Class == class {
		return r
	}
	return &object.Exception{
		Message:       fmt.Sprintf("error in %s._missing_: returned %s instead of None or a valid member", class.Name, r.Inspect()),
		ExceptionType: object.ExceptionTypeTypeError,
		Raised:        true,
	}
}

// valueMember returns the value of an IntEnum, IntFlag or StrEnum member,
// which acts as its int or str value; ok is false for anything else.
func valueMember(o object.Object) (object.Object, bool) {
	m, ok := o.(*object.Instance)
	if !ok || !m.Class.IsEnum {
		return nil, false
	}
	if fn, _ := object.InstanceHashMethod(m.Class); fn != enumValueHash {
		return nil, false
	}
	v, ok := m.GetField("value")
	return v, ok
}

// valueOf is o's value when o is a value member, else o.
func valueOf(o object.Object) object.Object {
	if v, ok := valueMember(o); ok {
		return v
	}
	return o
}

// addValueOperators gives the members of an IntEnum, IntFlag or StrEnum
// the operators of their values, as members of an int or str subclass:
// arithmetic and ordering (with plain values and other members), and for
// a StrEnum concatenation, len(), iteration, membership and indexing.
// Results are plain values, as in Python. A Flag keeps its own | & ^ ~.
func addValueOperators(class *object.Class, isStr, isFlag bool) {
	binary := func(name string, op ast.Op, reflected bool) {
		if isFlag {
			switch op {
			case ast.OpBitOr, ast.OpBitAnd, ast.OpBitXor:
				return
			}
		}
		class.Methods[name] = &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.NoKwargs(name, kwargs); err != nil {
					return err
				}
				if len(args) != 2 {
					return errors.NewArgumentError(len(args), 2)
				}
				a, b := valueOf(args[0]), valueOf(args[1])
				if reflected {
					a, b = b, a
				}
				return evalInfixExpression(ctx, op, a, b, GetEnvFromContext(ctx))
			},
			HelpText: name + "(other) - the operator on the member's value",
		}
	}
	for _, c := range []struct {
		name string
		op   ast.Op
	}{{"__lt__", ast.OpLt}, {"__le__", ast.OpLte}, {"__gt__", ast.OpGt}, {"__ge__", ast.OpGte}} {
		binary(c.name, c.op, false)
	}
	arith := []struct {
		name string
		op   ast.Op
	}{{"add", ast.OpAdd}, {"mul", ast.OpMul}, {"mod", ast.OpMod}}
	if !isStr {
		arith = append(arith, []struct {
			name string
			op   ast.Op
		}{{"sub", ast.OpSub}, {"truediv", ast.OpDiv}, {"floordiv", ast.OpFloorDiv}, {"pow", ast.OpPow},
			{"and", ast.OpBitAnd}, {"or", ast.OpBitOr}, {"xor", ast.OpBitXor}, {"lshift", ast.OpLShift}, {"rshift", ast.OpRShift}}...)
	}
	for _, a := range arith {
		binary("__"+a.name+"__", a.op, false)
		binary("__r"+a.name+"__", a.op, true)
	}
	unary := func(name string, op ast.Op) {
		class.Methods[name] = &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.NoKwargs(name, kwargs); err != nil {
					return err
				}
				if len(args) != 1 {
					return errors.NewArgumentError(len(args), 1)
				}
				return evalPrefixExpression(ctx, op, valueOf(args[0]), GetEnvFromContext(ctx))
			},
			HelpText: name + "() - the operator on the member's value",
		}
	}
	if !isStr {
		unary("__neg__", ast.OpSub)
		unary("__pos__", ast.OpPos)
		class.Methods["__abs__"] = &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.NoKwargs("__abs__", kwargs); err != nil {
					return err
				}
				return builtins["abs"].Fn(ctx, object.NewKwargs(nil), valueOf(args[0]))
			},
			HelpText: "__abs__() - abs() of the member's value",
		}
		if !isFlag {
			unary("__invert__", ast.OpBitNot)
		}
		return
	}
	// A StrEnum member is a str: len(), iteration, `in` and indexing.
	delegate := func(name, builtin string) {
		class.Methods[name] = &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.NoKwargs(name, kwargs); err != nil {
					return err
				}
				return builtins[builtin].Fn(ctx, object.NewKwargs(nil), valueOf(args[0]))
			},
			HelpText: name + "() - " + builtin + "() of the member's value",
		}
	}
	delegate("__len__", "len")
	delegate("__iter__", "iter")
	class.Methods["__contains__"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("__contains__", kwargs); err != nil {
				return err
			}
			if len(args) != 2 {
				return errors.NewArgumentError(len(args), 2)
			}
			return evalInOperator(ctx, valueOf(args[1]), valueOf(args[0]), GetEnvFromContext(ctx))
		},
		HelpText: "__contains__(x) - x in the member's value",
	}
	class.Methods["__getitem__"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("__getitem__", kwargs); err != nil {
				return err
			}
			if len(args) != 2 {
				return errors.NewArgumentError(len(args), 2)
			}
			return evalIndexExpression(ctx, valueOf(args[0]), args[1], false)
		},
		HelpText: "__getitem__(i) - indexing the member's value",
	}
}
