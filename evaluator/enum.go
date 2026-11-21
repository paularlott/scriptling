package evaluator

import (
	"context"
	"fmt"
	"strings"

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
				sv, _ := self.Field("value").(*object.Integer)
				if ov, ok := args[1].(*object.Integer); ok && sv != nil {
					return object.NewBoolean(sv.IntValue() == ov.IntValue())
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

	// IntEnum members behave as ints in arithmetic and comparisons.
	if class.IsIntEnum {
		memberInt := func(self *object.Instance) (int64, bool) {
			v, isInt := self.Field("value").(*object.Integer)
			if !isInt {
				return 0, false
			}
			return v.IntValue(), true
		}
		binop := func(name string, f func(a, b int64) int64) {
			class.Methods[name] = &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if len(args) != 2 {
						return errors.NewArgumentError(len(args), 2)
					}
					self, ok := args[0].(*object.Instance)
					other, isInt := args[1].(*object.Integer)
					if !ok || !isInt {
						return errors.NewTypeErrorTagged("unsupported operand types for %s", name)
					}
					a, okA := memberInt(self)
					if !okA {
						return errors.NewTypeErrorTagged("unsupported operand types for %s", name)
					}
					return object.NewInteger(f(a, other.IntValue()))
				},
				HelpText: name + "(other) - integer arithmetic on the member value",
			}
		}
		binop("__add__", func(a, b int64) int64 { return a + b })
		binop("__sub__", func(a, b int64) int64 { return a - b })
		binop("__mul__", func(a, b int64) int64 { return a * b })
		binop("__floordiv__", func(a, b int64) int64 { return a / b })
		binop("__mod__", func(a, b int64) int64 { return a % b })
		binop("__truediv__", func(a, b int64) int64 { return a / b })
		cmp := func(name string, f func(a, b int64) bool) {
			class.Methods[name] = &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if len(args) != 2 {
						return errors.NewArgumentError(len(args), 2)
					}
					self, ok := args[0].(*object.Instance)
					other, isInt := args[1].(*object.Integer)
					if o, isMember := args[1].(*object.Instance); isMember && o.Class == class {
						// Two members compare by value: sorted() and max().
						other, isInt = o.Field("value").(*object.Integer)
					}
					if !ok || !isInt {
						return notImplementedValue
					}
					a, okA := memberInt(self)
					if !okA {
						return object.NewBoolean(false)
					}
					return object.NewBoolean(f(a, other.IntValue()))
				},
				HelpText: name + "(other) - integer comparison on the member value",
			}
		}
		cmp("__lt__", func(a, b int64) bool { return a < b })
		cmp("__le__", func(a, b int64) bool { return a <= b })
		cmp("__gt__", func(a, b int64) bool { return a > b })
		cmp("__ge__", func(a, b int64) bool { return a >= b })
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

// aliasEnumMember returns the existing member whose value equals v, if any.
func aliasEnumMember(members []object.Object, v object.Object) *object.Instance {
	for _, m := range members {
		member := m.(*object.Instance)
		if member.Field("value").Inspect() == v.Inspect() {
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
		mv := member.Field("value")
		if mv.Inspect() == value.Inspect() {
			return member
		}
	}
	return &object.Exception{
		Message:       fmt.Sprintf("%s is not a valid %s", value.Inspect(), class.Name),
		ExceptionType: object.ExceptionTypeValueError,
		Raised:        true,
	}
}
