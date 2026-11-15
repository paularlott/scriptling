package stdlib

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/paularlott/scriptling/conversion"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

func jsonLoads(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	if len(args) != 1 {
		return errors.NewError("wrong number of arguments. got=%d, want=1", len(args))
	}
	if args[0].Type() != object.STRING_OBJ {
		return errors.NewError("argument to loads/parse must be STRING")
	}
	str, _ := args[0].AsString()
	return conversion.MustParseJSON(str)
}

func jsonDumps(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	if len(args) != 1 {
		return errors.NewError("wrong number of arguments. got=%d, want=1", len(args))
	}

	// indent accepts a string (used verbatim, like Python) or a number of
	// spaces (the common json.dumps(x, indent=2) idiom). An integer of 0
	// means newline-separated with no spaces, matching Python. indent=None
	// is the default (no indent), as in Python.
	indent := ""
	hasIndent := false
	if iv := kwargs.Get("indent"); iv != nil && iv.Type() != object.NULL_OBJ {
		hasIndent = true
		switch v := iv.(type) {
		case *object.String:
			indent = v.StringValue()
		case *object.Integer:
			if v.IntValue() < 0 {
				return errors.NewError("json.dumps: indent must not be negative")
			}
			if errObj := object.CheckAllocFor("JSON indent", v.IntValue()); errObj != nil {
				return errObj
			}
			indent = strings.Repeat(" ", int(v.IntValue()))
		case *object.Float:
			if v.FloatValue() < 0 {
				return errors.NewError("json.dumps: indent must not be negative")
			}
			if errObj := object.CheckAllocFor("JSON indent", int64(v.FloatValue())); errObj != nil {
				return errObj
			}
			indent = strings.Repeat(" ", int(v.FloatValue()))
		default:
			return errors.NewError("json.dumps: indent must be a string or a number of spaces")
		}
	}

	// Default separators follow Python: (", ", ": "), or (",", ": ") when
	// indenting (the indent reflow below normalizes item separators either
	// way, but the key separator always applies). Non-ASCII is escaped as
	// \uXXXX unless ensure_ascii=False, matching Python's default.
	opts := jsonOpts{itemSep: ", ", keySep: ": ", ensureAscii: true}
	if hasIndent {
		opts.itemSep = ","
	}
	if sep := kwargs.Get("separators"); sep != nil {
		var elems []object.Object
		switch s := sep.(type) {
		case *object.List:
			elems = s.Elements
		case *object.Tuple:
			elems = s.Elements
		default:
			return errors.NewError("json.dumps: separators must be a (item, key) pair")
		}
		if len(elems) != 2 {
			return errors.NewError("json.dumps: separators must be a (item, key) pair")
		}
		item, err := elems[0].AsString()
		if err != nil {
			return err
		}
		key, err := elems[1].AsString()
		if err != nil {
			return err
		}
		opts.itemSep, opts.keySep = item, key
	}
	if sk := kwargs.Get("sort_keys"); sk != nil {
		b, err := sk.AsBool()
		if err != nil {
			return errors.NewError("json.dumps: sort_keys must be a boolean")
		}
		opts.sortKeys = b
	}
	if ea := kwargs.Get("ensure_ascii"); ea != nil {
		b, err := ea.AsBool()
		if err != nil {
			return errors.NewError("json.dumps: ensure_ascii must be a boolean")
		}
		opts.ensureAscii = b
	}

	if errObj := object.CheckAllocFor("JSON indent", int64(len(indent))); errObj != nil {
		return errObj
	}
	enc := jsonEncoder{opts: opts}
	if errObj := enc.value(args[0], make(map[object.Object]struct{}), 0); errObj != nil {
		return errObj
	}
	out := enc.buf.Bytes()
	// hasIndent distinguishes indent=0 (newline-separated, no spaces, as in
	// Python) from no indent argument at all (fully compact).
	if hasIndent {
		ind, errObj := indentJSON(out, indent)
		if errObj != nil {
			return errObj
		}
		out = ind
	}
	return object.NewString(string(out))
}

var JSONLibrary = object.NewLibrary(JSONLibraryName, map[string]*object.Builtin{
	"loads": {
		Fn: jsonLoads,
		HelpText: `loads(json_string) - Parse JSON string

Parses a JSON string and returns the corresponding Scriptling object.`,
	},
	"dumps": {
		Fn: jsonDumps,
		HelpText: `dumps(obj, indent="", separators=None, sort_keys=False, ensure_ascii=True) - Serialize object to JSON string

Converts a Scriptling object to its JSON string representation. Dict keys are
emitted in the dict's insertion order, as in Python; sort_keys=True sorts them.
Default separators are (", ", ": "); pass separators=(",", ":") for compact
output. Non-ASCII characters are escaped as \uXXXX unless ensure_ascii=False.
Optional indent parameter for pretty-printing: a string used verbatim,
or a number of spaces (indent=2 is the common idiom; indent=0
newline-separates with no spaces).`,
	},
	"parse": {
		Fn: jsonLoads,
		HelpText: `parse(json_string) - Parse JSON string (alias for loads)

Parses a JSON string and returns the corresponding Scriptling object.`,
	},
	"stringify": {
		Fn: jsonDumps,
		HelpText: `stringify(obj, indent="") - Serialize object to JSON string (alias for dumps)

Converts a Scriptling object to its JSON string representation.
Optional indent parameter for pretty-printing.`,
	},
}, map[string]object.Object{
	// json.JSONDecodeError: a ValueError subclass, as in Python, so
	// `except json.JSONDecodeError:` matches parse failures.
	"JSONDecodeError": &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			message := ""
			if len(args) > 0 {
				if str, err := args[0].AsString(); err == nil {
					message = str
				} else {
					message = args[0].Inspect()
				}
			}
			return &object.Exception{Message: message, ExceptionType: "JSONDecodeError"}
		},
		HelpText: `JSONDecodeError([message]) - Create a JSON decode error`,
	},
}, "JSON encoding and decoding library")

// jsonOpts carries Python dumps() formatting: the item separator (between
// array elements and object members), the key separator, sort_keys and
// ensure_ascii.
type jsonOpts struct {
	itemSep     string
	keySep      string
	sortKeys    bool
	ensureAscii bool
}

// maxJSONDumpDepth bounds nesting when rendering, as maxJSONDepth does when
// parsing.
const maxJSONDumpDepth = 10000

// jsonEncoder renders a value as JSON text into one buffer. Containers are
// assembled here, rather than via encoding/json's map and array marshaling,
// so dict keys keep the dict's insertion order and Python's separators apply
// at every level.
//
// The whole text is one value in the making, so after every element its
// length is held to the value limit and the memory left: a value shared by
// several containers renders once per reference, and a small script value
// can stand for an exponentially large text, so the encoder stops as soon as
// the text outgrows the limits instead of finishing it first. The work done
// is bounded by the text written, since every value writes at least a byte.
type jsonEncoder struct {
	buf  bytes.Buffer
	opts jsonOpts
}

// check holds the text written so far to the limits.
func (e *jsonEncoder) check() object.Object {
	return object.CheckAllocFor("JSON", int64(e.buf.Len()))
}

func (e *jsonEncoder) value(obj object.Object, seen map[object.Object]struct{}, depth int) object.Object {
	depth++
	if depth > maxJSONDumpDepth {
		return object.NewMemoryError("value too deeply nested to serialize (more than %d levels)", maxJSONDumpDepth)
	}
	switch obj := obj.(type) {
	case *object.Integer:
		e.buf.Write(strconv.AppendInt(e.buf.AvailableBuffer(), obj.IntValue(), 10))
	case *object.Float:
		// Python writes non-finite floats as bare NaN/Infinity/-Infinity
		// tokens.
		f := obj.FloatValue()
		switch {
		case math.IsInf(f, 1):
			e.buf.WriteString("Infinity")
		case math.IsInf(f, -1):
			e.buf.WriteString("-Infinity")
		case math.IsNaN(f):
			e.buf.WriteString("NaN")
		default:
			e.buf.WriteString(object.FloatStr(f))
		}
	case *object.String:
		writeJSONString(&e.buf, obj.StringValue(), e.opts.ensureAscii)
	case *object.Boolean:
		if obj.BoolValue() {
			e.buf.WriteString("true")
		} else {
			e.buf.WriteString("false")
		}
	case *object.Null:
		e.buf.WriteString("null")
	case *object.List:
		return e.sequence(obj, obj.Elements, seen, depth)
	case *object.Tuple:
		// Python serializes tuples as JSON arrays.
		return e.sequence(obj, obj.Elements, seen, depth)
	case *object.Dict:
		if _, cyclic := seen[obj]; cyclic {
			return e.goString("<cyclic reference>")
		}
		seen[obj] = struct{}{}
		defer delete(seen, obj)
		pairs := obj.OrderedPairs()
		if e.opts.sortKeys {
			sorted := make([]object.DictPair, len(pairs))
			copy(sorted, pairs)
			sort.Slice(sorted, func(i, j int) bool {
				return sorted[i].StringKey() < sorted[j].StringKey()
			})
			pairs = sorted
		}
		e.buf.WriteByte('{')
		for i, pair := range pairs {
			if i > 0 {
				e.buf.WriteString(e.opts.itemSep)
			}
			writeJSONString(&e.buf, pair.StringKey(), e.opts.ensureAscii)
			e.buf.WriteString(e.opts.keySep)
			if errObj := e.value(pair.Value, seen, depth); errObj != nil {
				return errObj
			}
			if errObj := e.check(); errObj != nil {
				return errObj
			}
		}
		e.buf.WriteByte('}')
	default:
		return e.goString(obj.Inspect())
	}
	return e.check()
}

func (e *jsonEncoder) sequence(obj object.Object, elements []object.Object, seen map[object.Object]struct{}, depth int) object.Object {
	if _, cyclic := seen[obj]; cyclic {
		return e.goString("<cyclic reference>")
	}
	seen[obj] = struct{}{}
	defer delete(seen, obj)
	e.buf.WriteByte('[')
	for i, el := range elements {
		if i > 0 {
			e.buf.WriteString(e.opts.itemSep)
		}
		if errObj := e.value(el, seen, depth); errObj != nil {
			return errObj
		}
		if errObj := e.check(); errObj != nil {
			return errObj
		}
	}
	e.buf.WriteByte(']')
	return nil
}

// goString writes s as encoding/json does (with HTML escaping off), the
// spelling used for values JSON has no type for, rendered by Inspect.
func (e *jsonEncoder) goString(s string) object.Object {
	enc := json.NewEncoder(&e.buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(s); err != nil {
		return errors.NewError("json serialize error: %s", err.Error())
	}
	e.buf.Truncate(e.buf.Len() - 1) // Encode ends with a newline
	return e.check()
}

// writeJSONString renders a Python-compatible JSON string literal: ", \\ and
// control characters escaped as in CPython's json module, and code points
// above U+007F written as \uXXXX (surrogate pairs beyond the BMP) when
// ensure_ascii is set, Python's default. Escaping can grow a string sixfold;
// the encoder checks the text after each string, which exists already, so
// one string overshoots the limit by at most that.
func writeJSONString(b *bytes.Buffer, s string, ensureAscii bool) {
	b.Grow(len(s) + 2)
	b.WriteByte('"')
	start := 0
	for i := 0; i < len(s); {
		c := s[i]
		if c >= 0x20 && c != '"' && c != '\\' && c < utf8.RuneSelf {
			i++
			continue
		}
		if c >= utf8.RuneSelf && !ensureAscii {
			// Copied as is, as before (invalid UTF-8 included).
			r, size := utf8.DecodeRuneInString(s[i:])
			if r != utf8.RuneError || size != 1 {
				i += size
				continue
			}
		}
		b.WriteString(s[start:i])
		if c < utf8.RuneSelf {
			switch c {
			case '"':
				b.WriteString(`\"`)
			case '\\':
				b.WriteString(`\\`)
			case '\n':
				b.WriteString(`\n`)
			case '\r':
				b.WriteString(`\r`)
			case '\t':
				b.WriteString(`\t`)
			case '\b':
				b.WriteString(`\b`)
			case '\f':
				b.WriteString(`\f`)
			default:
				writeUnicodeEscape(b, rune(c))
			}
			i++
			start = i
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		i += size
		start = i
		switch {
		case !ensureAscii:
			// An invalid byte: WriteRune writes U+FFFD, as the range loop did.
			b.WriteRune(r)
		case r > 0xFFFF:
			writeUnicodeEscape(b, 0xD800+((r-0x10000)>>10))
			writeUnicodeEscape(b, 0xDC00+((r-0x10000)&0x3FF))
		default:
			writeUnicodeEscape(b, r)
		}
	}
	b.WriteString(s[start:])
	b.WriteByte('"')
}

func writeUnicodeEscape(b *bytes.Buffer, r rune) {
	const hex = "0123456789abcdef"
	b.WriteString(`\u`)
	b.WriteByte(hex[(r>>12)&0xf])
	b.WriteByte(hex[(r>>8)&0xf])
	b.WriteByte(hex[(r>>4)&0xf])
	b.WriteByte(hex[r&0xf])
}

// indentJSON lays compact JSON text out as encoding/json's Indent does: a
// newline and one indent per level after every opening bracket and item
// separator, ": " after keys, empty containers kept as [] and {}, and other
// whitespace between tokens dropped. Unlike Indent it is held to the value
// limit and the memory left as it goes: the indentation grows with nesting
// depth as well as size, so deeply nested text can indent to many times its
// compact length.
func indentJSON(src []byte, indent string) ([]byte, object.Object) {
	var dst bytes.Buffer
	if errObj := object.CheckAllocFor("JSON", int64(len(src))); errObj != nil {
		return nil, errObj
	}
	dst.Grow(len(src) + len(src)/4)
	depth := 0
	needIndent := false
	inString := false
	newline := func() object.Object {
		if errObj := object.CheckAllocFor("JSON", int64(dst.Len())+1+int64(depth)*int64(len(indent))); errObj != nil {
			return errObj
		}
		dst.WriteByte('\n')
		for range depth {
			dst.WriteString(indent)
		}
		return nil
	}
	start := 0 // start of the pending run of plain bytes
	for i := 0; i < len(src); i++ {
		c := src[i]
		if inString {
			if c == '\\' {
				i++
			} else if c == '"' {
				inString = false
			}
			continue
		}
		switch c {
		case ' ', '\t', '\n', '\r':
			dst.Write(src[start:i])
			start = i + 1
			continue
		}
		if needIndent && c != '}' && c != ']' {
			dst.Write(src[start:i])
			start = i
			needIndent = false
			depth++
			if errObj := newline(); errObj != nil {
				return nil, errObj
			}
		}
		switch c {
		case '"':
			inString = true
		case '{', '[':
			needIndent = true
		case ',':
			dst.Write(src[start : i+1])
			start = i + 1
			if errObj := newline(); errObj != nil {
				return nil, errObj
			}
		case ':':
			dst.Write(src[start : i+1])
			dst.WriteByte(' ')
			start = i + 1
		case '}', ']':
			if needIndent {
				needIndent = false
			} else {
				dst.Write(src[start:i])
				start = i
				depth--
				if errObj := newline(); errObj != nil {
					return nil, errObj
				}
			}
		}
	}
	dst.Write(src[start:])
	if errObj := object.CheckAllocFor("JSON", int64(dst.Len())); errObj != nil {
		return nil, errObj
	}
	return dst.Bytes(), nil
}
