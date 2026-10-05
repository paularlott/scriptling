package evaluator

import (
	"context"
	"fmt"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// buildEnumMembers turns a class deriving from enum.Enum into an enum: each
// plain top-level assignment (in source order) becomes a singleton member
// instance with name/value fields; auto() markers become sequential ints;
// duplicate values alias to the first member, as in Python. Members are
// exposed as class attributes, collected into EnumMembers (iteration order)
// and a __members__ dict.
func buildEnumMembers(class *object.Class) object.Object {
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
		if s, isSentinel := val.(*object.Sentinel); isSentinel && s.Name == "auto" {
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
	d := &object.Dict{Pairs: make(map[string]object.DictPair, len(members))}
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
				return errors.NewError("__hash__ requires an enum member")
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
					if !ok || !isInt {
						return object.NewBoolean(false)
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
	}
	// Python's str: the plain dotted name. repr (below) is the <Name:
	// value> form; __str__ wins in str() before the __repr__ method.
	class.Methods["__str__"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			self, ok := args[0].(*object.Instance)
			if !ok {
				return errors.NewError("__str__ requires an enum member")
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
				return errors.NewError("__repr__ requires an enum member")
			}
			name, _ := self.Field("name").(*object.String)
			value := self.Field("value")
			return object.NewString(fmt.Sprintf("<%s.%s: %s>", class.Name, name.StringValue(), value.Inspect()))
		},
		HelpText: "__repr__() - <Member: value> form",
	}
	return nil
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

// enumIterate materialises an enum class's members for iteration, `in`, and
// the list()/tuple() constructors.
func enumIterate(class *object.Class) []object.Object {
	out := make([]object.Object, len(class.EnumMembers))
	copy(out, class.EnumMembers)
	return out
}
