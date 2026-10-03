package object

import (
	"sort"
	"strings"
)

// ReprString renders a string the way Python's repr does: single quotes by
// default, switching to double quotes when the string contains a single
// quote and no double quote, with backslash, newline, carriage return and
// tab escaped.
func ReprString(v string) string {
	quote := byte('\'')
	if strings.ContainsRune(v, '\'') && !strings.ContainsRune(v, '"') {
		quote = '"'
	}
	var out strings.Builder
	out.Grow(len(v) + 2)
	out.WriteByte(quote)
	for i := 0; i < len(v); i++ {
		c := v[i]
		switch {
		case c == '\\':
			out.WriteString("\\\\")
		case c == '\n':
			out.WriteString("\\n")
		case c == '\r':
			out.WriteString("\\r")
		case c == '\t':
			out.WriteString("\\t")
		case c == quote:
			out.WriteByte('\\')
			out.WriteByte(c)
		case c < 0x20 || c == 0x7f:
			// Other control characters print as \xNN, as in Python.
			out.WriteString("\\x")
			out.WriteByte("0123456789abcdef"[c>>4])
			out.WriteByte("0123456789abcdef"[c&0xf])
		default:
			out.WriteByte(c)
		}
	}
	out.WriteByte(quote)
	return out.String()
}

// ReprException renders an exception as Python's repr does:
// ValueError('bad'), or ValueError() when it has no message.
func ReprException(e *Exception) string {
	name := e.ExceptionType
	if name == "" {
		name = "Exception"
	}
	if e.Message == "" {
		return name + "()"
	}
	return name + "(" + ReprString(e.Message) + ")"
}

// InstanceReprFunc renders an instance nested in a container, normally by
// calling its __repr__. A non-nil Object return is a raised error that aborts
// the rendering.
type InstanceReprFunc func(*Instance) (string, Object)

// InspectRepr renders a container as Python's repr() does: nested strings are
// quoted, and nested instances are rendered by instanceRepr (their default
// representation when it is nil). Non-containers render via Inspect.
func InspectRepr(obj Object, instanceRepr InstanceReprFunc) (string, Object) {
	r := reprRenderer{seen: make(map[Object]struct{}), instanceRepr: instanceRepr}
	r.container(obj)
	return r.out.String(), r.err
}

// IsReprContainer reports whether obj is a container whose display renders
// its elements with repr.
func IsReprContainer(obj Object) bool {
	switch obj.(type) {
	case *List, *Tuple, *Dict, *Set, *DictKeys, *DictValues, *DictItems:
		return true
	}
	return false
}

// cyclicInspectPlaceholder is rendered in place of a container that refers to
// itself (directly or through a chain). Without this guard a self-referential
// list/tuple/dict recurses until the Go stack overflows, which no recover()
// can catch and which aborts the whole host process.
const cyclicInspectPlaceholder = "<cyclic reference>"

type reprRenderer struct {
	out          strings.Builder
	seen         map[Object]struct{} // containers on the path from the root
	instanceRepr InstanceReprFunc
	err          Object
}

// element renders a value nested inside a container: repr semantics.
func (r *reprRenderer) element(obj Object) {
	if r.err != nil {
		return
	}
	switch o := obj.(type) {
	case *String:
		r.out.WriteString(ReprString(o.value))
	case *Exception:
		r.out.WriteString(ReprException(o))
	case *Instance:
		if r.instanceRepr == nil {
			r.out.WriteString(o.Inspect())
			return
		}
		s, err := r.instanceRepr(o)
		if err != nil {
			r.err = err
			return
		}
		r.out.WriteString(s)
	default:
		r.container(obj)
	}
}

// container renders obj itself, recursing into its elements.
func (r *reprRenderer) container(obj Object) {
	if !IsReprContainer(obj) {
		r.out.WriteString(obj.Inspect())
		return
	}
	if d, ok := obj.(*Dict); ok && d.Module != "" {
		r.out.WriteString("<module '" + d.Module + "'>")
		return
	}
	if _, cyclic := r.seen[obj]; cyclic {
		r.out.WriteString(cyclicInspectPlaceholder)
		return
	}
	r.seen[obj] = struct{}{}
	defer delete(r.seen, obj)

	switch o := obj.(type) {
	case *List:
		r.out.WriteByte('[')
		r.sequence(o.Elements)
		r.out.WriteByte(']')
	case *Tuple:
		r.out.WriteByte('(')
		r.sequence(o.Elements)
		if len(o.Elements) == 1 {
			r.out.WriteByte(',') // single element tuple needs a trailing comma
		}
		r.out.WriteByte(')')
	case *Dict:
		r.out.WriteByte('{')
		i := 0
		for _, pair := range o.Pairs {
			if i > 0 {
				r.out.WriteString(", ")
			}
			r.element(pair.Key)
			r.out.WriteString(": ")
			r.element(pair.Value)
			i++
		}
		r.out.WriteByte('}')
	case *Set:
		if len(o.Elements) == 0 {
			r.out.WriteString("set()")
			return
		}
		// Sorted for deterministic output.
		parts := make([]string, 0, len(o.Elements))
		for _, e := range o.Elements {
			sub := reprRenderer{seen: r.seen, instanceRepr: r.instanceRepr}
			sub.element(e)
			if sub.err != nil {
				r.err = sub.err
				return
			}
			parts = append(parts, sub.out.String())
		}
		sort.Strings(parts)
		r.out.WriteByte('{')
		r.out.WriteString(strings.Join(parts, ", "))
		r.out.WriteByte('}')
	case *DictKeys:
		r.view("dict_keys", o.Dict, func(p DictPair) { r.element(p.Key) })
	case *DictValues:
		r.view("dict_values", o.Dict, func(p DictPair) { r.element(p.Value) })
	case *DictItems:
		r.view("dict_items", o.Dict, func(p DictPair) {
			r.out.WriteByte('(')
			r.element(p.Key)
			r.out.WriteString(", ")
			r.element(p.Value)
			r.out.WriteByte(')')
		})
	}
}

func (r *reprRenderer) sequence(elements []Object) {
	for i, el := range elements {
		if i > 0 {
			r.out.WriteString(", ")
		}
		r.element(el)
	}
}

// view renders a dict view as Python does: dict_keys(['a', 'b']).
func (r *reprRenderer) view(name string, d *Dict, each func(DictPair)) {
	r.out.WriteString(name)
	r.out.WriteString("([")
	i := 0
	for _, pair := range d.Pairs {
		if i > 0 {
			r.out.WriteString(", ")
		}
		each(pair)
		i++
	}
	r.out.WriteString("])")
}

// inspectContainer is the Inspect of every container type: repr semantics
// with instances in their default representation.
func inspectContainer(obj Object) string {
	s, _ := InspectRepr(obj, nil)
	return s
}
