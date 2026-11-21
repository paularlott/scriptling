package object

import (
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
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
			writeHexEscape(&out, 'x', rune(c), 2)
		case c >= utf8.RuneSelf:
			r, size := utf8.DecodeRuneInString(v[i:])
			i += size - 1
			if r == utf8.RuneError && size == 1 || reprPrintable(r) {
				out.WriteString(v[i-size+1 : i+1])
			} else if r < 0x100 {
				writeHexEscape(&out, 'x', r, 2)
			} else if r < 0x10000 {
				writeHexEscape(&out, 'u', r, 4)
			} else {
				writeHexEscape(&out, 'U', r, 8)
			}
		default:
			out.WriteByte(c)
		}
	}
	out.WriteByte(quote)
	return out.String()
}

// reprPrintable reports whether repr shows a non-ASCII rune as is. Python
// escapes separators other than the ASCII space, control and format
// characters, surrogates, private-use and unassigned code points.
func reprPrintable(r rune) bool {
	return !unicode.In(r, unicode.Zs, unicode.Zl, unicode.Zp, unicode.Cc, unicode.Cf, unicode.Cs, unicode.Co) &&
		(unicode.IsPrint(r) || unicode.IsGraphic(r) || unicode.In(r, unicode.L, unicode.M, unicode.N, unicode.P, unicode.S))
}

func writeHexEscape(out *strings.Builder, kind byte, r rune, digits int) {
	out.WriteByte('\\')
	out.WriteByte(kind)
	for shift := (digits - 1) * 4; shift >= 0; shift -= 4 {
		out.WriteByte("0123456789abcdef"[(r>>uint(shift))&0xf])
	}
}

// ReprException renders an exception as Python's repr does:
// ValueError('bad'), or ValueError() when it has no message.
func ReprException(e *Exception) string {
	name := e.ExceptionType
	if name == "" {
		name = "Exception"
	}
	if len(e.Args) > 0 {
		// The constructor arguments, as Python shows them: KeyError('k'),
		// ValueError('bad', 2).
		parts := make([]string, len(e.Args))
		for i, a := range e.Args {
			parts[i] = ValueRepr(a)
		}
		return name + "(" + strings.Join(parts, ", ") + ")"
	}
	if e.Message == "" {
		return name + "()"
	}
	return name + "(" + ReprString(e.Message) + ")"
}

// ValueRepr renders a single value as Python's repr does, quoting strings
// and rendering containers with repr'd elements. Instances render via their
// default representation.
func ValueRepr(obj Object) string {
	r := reprRenderer{seen: make(map[Object]struct{}), limit: OutputLimit()}
	r.element(obj)
	return r.out.String()
}

// NewKeyError builds the KeyError Python raises for a missing key: its args
// hold the key itself and its message (what str(e) shows) is keyRepr, the
// key's repr.
func NewKeyError(key Object, keyRepr string) *Exception {
	return &Exception{Message: keyRepr, ExceptionType: ExceptionTypeKeyError, Args: []Object{key}, Raised: true}
}

// InstanceReprFunc renders an instance nested in a container, normally by
// calling its __repr__. A non-nil Object return is a raised error that aborts
// the rendering.
type InstanceReprFunc func(*Instance) (string, Object)

// InspectRepr renders a container as Python's repr() does: nested strings are
// quoted, and nested instances are rendered by instanceRepr (their default
// representation when it is nil). Non-containers render via Inspect.
//
// Under a value or memory limit the text is held to OutputLimit as it is
// written: a container can hold the same value many times over, so a small
// value can stand for an exponentially large text, and rendering stops with
// a MemoryError as soon as the text passes the limit rather than after it is
// built. Every element writes at least a byte, so this bounds the work too.
func InspectRepr(obj Object, instanceRepr InstanceReprFunc) (string, Object) {
	r := reprRenderer{seen: make(map[Object]struct{}), instanceRepr: instanceRepr, limit: OutputLimit()}
	r.container(obj)
	r.over()
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

// MaxReprDepth caps how deeply nested a container repr()/str() will render.
// Rendering recurses in Go, and a stack overflow is a fatal error that ends
// the whole process, so past this depth rendering raises RecursionError, as
// Python does. It matches the nesting depth the data decoders accept.
const MaxReprDepth = 10000

type reprRenderer struct {
	out          strings.Builder
	seen         map[Object]struct{} // containers on the path from the root
	instanceRepr InstanceReprFunc
	err          Object
	depth        int   // containers on the path from the root
	limit        int64 // longest text allowed, 0 for no limit (see OutputLimit)
	used         int64 // text written by enclosing renderers (set elements)
}

// over reports, once the text has passed the limit, that rendering must stop,
// recording the MemoryError.
func (r *reprRenderer) over() bool {
	if r.err != nil {
		return true
	}
	if r.limit > 0 && r.used+int64(r.out.Len()) > r.limit {
		r.err = errOutputTooLarge("repr", r.limit)
		return true
	}
	return false
}

// errReprTooDeep is the RecursionError raised past MaxReprDepth.
func errReprTooDeep() *Exception {
	return &Exception{
		Message:       "maximum recursion depth exceeded while getting the repr of an object",
		ExceptionType: "RecursionError",
		Raised:        true,
	}
}

// element renders a value nested inside a container: repr semantics.
func (r *reprRenderer) element(obj Object) {
	if r.over() {
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
	if r.depth >= MaxReprDepth {
		r.err = errReprTooDeep()
		return
	}
	r.depth++
	r.seen[obj] = struct{}{}
	defer func() {
		delete(r.seen, obj)
		r.depth--
	}()

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
		if o.Factory() != nil {
			r.out.WriteString("defaultdict(")
			if _, none := o.Factory().(*Null); none {
				r.out.WriteString("None")
			} else if FactoryRepr != nil {
				r.out.WriteString(FactoryRepr(o.Factory()))
			} else {
				r.out.WriteString(o.Factory().Inspect())
			}
			r.out.WriteString(", ")
		}
		r.out.WriteByte('{')
		i := 0
		for _, pair := range o.OrderedPairs() {
			if r.over() {
				return
			}
			if i > 0 {
				r.out.WriteString(", ")
			}
			r.element(pair.Key)
			r.out.WriteString(": ")
			r.element(pair.Value)
			i++
		}
		r.out.WriteByte('}')
		if o.Factory() != nil {
			r.out.WriteByte(')')
		}
	case *Set:
		if o.Len() == 0 {
			if o.Frozen {
				r.out.WriteString("frozenset()")
			} else {
				r.out.WriteString("set()")
			}
			return
		}
		// Sorted for deterministic output.
		parts := make([]string, 0, o.Len())
		used := r.used + int64(r.out.Len())
		for _, e := range o.All() {
			sub := reprRenderer{seen: r.seen, instanceRepr: r.instanceRepr, depth: r.depth, limit: r.limit, used: used}
			sub.element(e)
			if sub.err == nil {
				sub.over()
			}
			if sub.err != nil {
				r.err = sub.err
				return
			}
			used += int64(sub.out.Len()) + 2
			parts = append(parts, sub.out.String())
		}
		sort.Strings(parts)
		if o.Frozen {
			r.out.WriteString("frozenset({")
			r.out.WriteString(strings.Join(parts, ", "))
			r.out.WriteString("})")
			return
		}
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
		if r.err != nil {
			return
		}
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
	for _, pair := range d.OrderedPairs() {
		if r.over() {
			return
		}
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
//
// Inspect cannot fail, so under a limit a text that passes it is cut short
// just past the limit and ends in "...": the work stays bounded, and a
// caller that makes a value of the text still refuses it for its size.
func inspectContainer(obj Object) string {
	r := reprRenderer{seen: make(map[Object]struct{}), limit: OutputLimit()}
	r.container(obj)
	if r.err != nil && r.limit > 0 && r.used+int64(r.out.Len()) > r.limit {
		return r.out.String() + "..."
	}
	return r.out.String()
}
