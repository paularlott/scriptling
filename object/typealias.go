package object

import (
	"context"
	"strings"
)

// Runtime forms of the type expressions annotations use, so code that
// evaluates them (list[int], dict[str, int], int | None, Optional[int])
// runs as in Python. A generic alias is callable as its origin type
// (list[int]([1]) is [1]); a union is accepted by isinstance and
// issubclass, which test each member.

// TypeExprName renders a type as it appears in a type expression: int,
// None, MyClass, list[int]. The evaluator installs it; nil until then.
var TypeExprName func(Object) string

const (
	typeUnionMarker  = "__union__"
	typeAliasMarker  = "__generic_alias__"
	typeArgsAttr     = "__args__"
	typeOriginAttr   = "__origin__"
	typeUnionNotCall = "'typing.Union' object is not callable"
)

func typeExprName(o Object) string {
	if TypeExprName != nil {
		return TypeExprName(o)
	}
	return o.Inspect()
}

// TypeUnionMembers returns the members of a union made by NewTypeUnion.
func TypeUnionMembers(o Object) ([]Object, bool) {
	b, ok := o.(*Builtin)
	if !ok || b.Attributes == nil || b.Attributes[typeUnionMarker] == nil {
		return nil, false
	}
	t, _ := b.Attributes[typeArgsAttr].(*Tuple)
	if t == nil {
		return nil, false
	}
	return t.Elements, true
}

// IsTypeExpr reports whether o is a union or generic alias.
func IsTypeExpr(o Object) bool {
	b, ok := o.(*Builtin)
	return ok && b.Attributes != nil && (b.Attributes[typeUnionMarker] != nil || b.Attributes[typeAliasMarker] != nil)
}

// NewTypeUnion is members joined with |: nested unions are flattened and
// repeats dropped, and a single distinct member is returned as itself.
func NewTypeUnion(members []Object) Object {
	var flat []Object
	add := func(m Object) {
		for _, f := range flat {
			if f == m {
				return
			}
			if _, fn := f.(*Null); fn {
				if _, mn := m.(*Null); mn {
					return
				}
			}
		}
		flat = append(flat, m)
	}
	for _, m := range members {
		if inner, ok := TypeUnionMembers(m); ok {
			for _, i := range inner {
				add(i)
			}
			continue
		}
		add(m)
	}
	if len(flat) == 1 {
		return flat[0]
	}
	names := make([]string, len(flat))
	for i, m := range flat {
		names[i] = typeExprName(m)
	}
	return &Builtin{
		Fn: func(ctx context.Context, kwargs Kwargs, args ...Object) Object {
			return &Exception{Message: typeUnionNotCall, ExceptionType: ExceptionTypeTypeError, Raised: true}
		},
		HelpText:   "A union of types, as X | Y makes",
		Repr:       strings.Join(names, " | "),
		Attributes: map[string]Object{typeUnionMarker: NewBoolean(true), typeArgsAttr: &Tuple{Elements: flat}},
	}
}

// NewGenericAlias is origin[args] for a builtin type: it calls as origin
// and displays as list[int].
func NewGenericAlias(origin *Builtin, originName string, args Object) Object {
	var elems []Object
	if t, ok := args.(*Tuple); ok {
		elems = t.Elements
	} else {
		elems = []Object{args}
	}
	names := make([]string, len(elems))
	for i, e := range elems {
		names[i] = typeExprName(e)
	}
	return &Builtin{
		Fn:       origin.Fn,
		HelpText: origin.HelpText,
		Repr:     originName + "[" + strings.Join(names, ", ") + "]",
		Attributes: map[string]Object{
			typeAliasMarker: NewBoolean(true),
			typeOriginAttr:  origin,
			typeArgsAttr:    &Tuple{Elements: elems},
		},
	}
}
