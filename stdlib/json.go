package stdlib

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

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
			indent = strings.Repeat(" ", int(v.IntValue()))
		case *object.Float:
			if v.FloatValue() < 0 {
				return errors.NewError("json.dumps: indent must not be negative")
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

	raw, rerr := objectToJSONRaw(args[0], make(map[object.Object]struct{}), opts)
	if rerr != nil {
		return rerr
	}
	out := []byte(raw)
	// hasIndent distinguishes indent=0 (newline-separated, no spaces, as in
	// Python) from no indent argument at all (fully compact).
	if hasIndent {
		var ind bytes.Buffer
		if err := json.Indent(&ind, out, "", indent); err != nil {
			return errors.NewError("json serialize error: %s", err.Error())
		}
		out = ind.Bytes()
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

// pyFloat marshals with Python's json module float spellings: repr-style
// numbers ("2.0", "1e+20") and Infinity/-Infinity/NaN for non-finites, which
// encoding/json would reject (it prints "2" and cannot represent inf at all).
type pyFloat float64

func (p pyFloat) MarshalJSON() ([]byte, error) {
	f := float64(p)
	switch {
	case math.IsInf(f, 1):
		return []byte("Infinity"), nil
	case math.IsInf(f, -1):
		return []byte("-Infinity"), nil
	case math.IsNaN(f):
		return []byte("NaN"), nil
	}
	return []byte(object.FloatStr(f)), nil
}

// jsonOpts carries Python dumps() formatting: the item separator (between
// array elements and object members), the key separator, sort_keys and
// ensure_ascii.
type jsonOpts struct {
	itemSep     string
	keySep      string
	sortKeys    bool
	ensureAscii bool
}

// objectToJSONRaw renders obj as JSON text. Containers are assembled here —
// rather than via encoding/json's map/array marshaling — so dict keys keep
// the dict's insertion order and Python's separators apply at every level.
// Leaves go through encoding/json with HTML escaping off, matching Python.
func objectToJSONRaw(obj object.Object, seen map[object.Object]struct{}, opts jsonOpts) (json.RawMessage, object.Object) {
	encodeLeaf := func(v interface{}) (json.RawMessage, object.Object) {
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(v); err != nil {
			return nil, errors.NewError("json serialize error: %s", err.Error())
		}
		return json.RawMessage(bytes.TrimSuffix(buf.Bytes(), []byte("\n"))), nil
	}

	switch obj := obj.(type) {
	case *object.Integer:
		return encodeLeaf(obj.IntValue())
	case *object.Float:
		// Python writes non-finite floats as bare NaN/Infinity/-Infinity
		// tokens; emit them directly (encoding/json would reject a marshaler
		// that returns them as invalid JSON).
		f := obj.FloatValue()
		switch {
		case math.IsInf(f, 1):
			return json.RawMessage("Infinity"), nil
		case math.IsInf(f, -1):
			return json.RawMessage("-Infinity"), nil
		case math.IsNaN(f):
			return json.RawMessage("NaN"), nil
		}
		return encodeLeaf(pyFloat(f))
	case *object.String:
		return encodeJSONString(obj.StringValue(), opts.ensureAscii), nil
	case *object.Boolean:
		return encodeLeaf(obj.BoolValue())
	case *object.List:
		if _, cyclic := seen[obj]; cyclic {
			return encodeLeaf("<cyclic reference>")
		}
		seen[obj] = struct{}{}
		defer delete(seen, obj)
		return assembleJSONSeq(obj.Elements, seen, opts, encodeLeaf)
	case *object.Tuple:
		// Python serializes tuples as JSON arrays.
		if _, cyclic := seen[obj]; cyclic {
			return encodeLeaf("<cyclic reference>")
		}
		seen[obj] = struct{}{}
		defer delete(seen, obj)
		return assembleJSONSeq(obj.Elements, seen, opts, encodeLeaf)
	case *object.Dict:
		if _, cyclic := seen[obj]; cyclic {
			return encodeLeaf("<cyclic reference>")
		}
		seen[obj] = struct{}{}
		defer delete(seen, obj)
		pairs := obj.OrderedPairs()
		if opts.sortKeys {
			sorted := make([]object.DictPair, len(pairs))
			copy(sorted, pairs)
			sort.Slice(sorted, func(i, j int) bool {
				return sorted[i].StringKey() < sorted[j].StringKey()
			})
			pairs = sorted
		}
		var buf bytes.Buffer
		buf.WriteByte('{')
		for i, pair := range pairs {
			if i > 0 {
				buf.WriteString(opts.itemSep)
			}
			keyStr := pair.StringKey()
			keyJSON := encodeJSONString(keyStr, opts.ensureAscii)
			buf.Write(keyJSON)
			buf.WriteString(opts.keySep)
			valJSON, verr := objectToJSONRaw(pair.Value, seen, opts)
			if verr != nil {
				return nil, verr
			}
			buf.Write(valJSON)
		}
		buf.WriteByte('}')
		return json.RawMessage(buf.Bytes()), nil
	case *object.Null:
		return json.RawMessage("null"), nil
	default:
		return encodeLeaf(obj.Inspect())
	}
}

// encodeJSONString renders a Python-compatible JSON string literal: ", \\ and
// control characters escaped as in CPython's json module, and code points
// above U+007F written as \uXXXX (surrogate pairs beyond the BMP) when
// ensure_ascii is set — Python's default.
func encodeJSONString(s string, ensureAscii bool) json.RawMessage {
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '"':
			b.WriteString(`\"`)
		case r == '\\':
			b.WriteString(`\\`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == '\t':
			b.WriteString(`\t`)
		case r == '\b':
			b.WriteString(`\b`)
		case r == '\f':
			b.WriteString(`\f`)
		case r < 0x20:
			fmt.Fprintf(&b, `\u%04x`, r)
		case ensureAscii && r > 0x7F:
			if r > 0xFFFF {
				hi := 0xD800 + ((r - 0x10000) >> 10)
				lo := 0xDC00 + ((r - 0x10000) & 0x3FF)
				fmt.Fprintf(&b, `\u%04x\u%04x`, hi, lo)
			} else {
				fmt.Fprintf(&b, `\u%04x`, r)
			}
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return json.RawMessage(b.String())
}

// assembleJSONSeq renders a JSON array with the configured item separator.
func assembleJSONSeq(elements []object.Object, seen map[object.Object]struct{}, opts jsonOpts, encodeLeaf func(interface{}) (json.RawMessage, object.Object)) (json.RawMessage, object.Object) {
	var buf bytes.Buffer
	buf.WriteByte('[')
	for i, el := range elements {
		if i > 0 {
			buf.WriteString(opts.itemSep)
		}
		elJSON, err := objectToJSONRaw(el, seen, opts)
		if err != nil {
			return nil, err
		}
		buf.Write(elJSON)
	}
	buf.WriteByte(']')
	return json.RawMessage(buf.Bytes()), nil
}
