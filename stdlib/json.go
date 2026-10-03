package stdlib

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
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
	// means newline-separated with no spaces, matching Python.
	indent := ""
	hasIndent := false
	if iv := kwargs.Get("indent"); iv != nil {
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

	data := objectToJSON(args[0])
	// Encode without HTML escaping: Python's json never turns <, > and &
	// into \u003c etc. (encoding/json's Marshal does).
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(data); err != nil {
		return errors.NewError("json serialize error: %s", err.Error())
	}
	out := bytes.TrimSuffix(buf.Bytes(), []byte("\n"))
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
		HelpText: `dumps(obj, indent="") - Serialize object to JSON string

Converts a Scriptling object to its JSON string representation.
Optional indent parameter for pretty-printing: a string used verbatim, or a
number of spaces (indent=2 is the common idiom; indent=0 newline-separates
with no spaces). Object keys are always emitted in sorted order.`,
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
}, nil, "JSON encoding and decoding library")

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

func objectToJSON(obj object.Object) interface{} {
	return objectToJSONSeen(obj, make(map[object.Object]struct{}))
}

// objectToJSONSeen is objectToJSON with the set of containers on the current
// path. A container already on the path is a cycle; it is rendered as a
// string placeholder rather than recursed into, which otherwise overflows
// the Go stack and aborts the host process (unrecoverable).
func objectToJSONSeen(obj object.Object, seen map[object.Object]struct{}) interface{} {
	switch obj := obj.(type) {
	case *object.Integer:
		return obj.IntValue()
	case *object.Float:
		return pyFloat(obj.FloatValue())
	case *object.String:
		return obj.StringValue()
	case *object.Boolean:
		return obj.BoolValue()
	case *object.List:
		if _, cyclic := seen[obj]; cyclic {
			return "<cyclic reference>"
		}
		seen[obj] = struct{}{}
		defer delete(seen, obj)
		arr := make([]interface{}, len(obj.Elements))
		for i, el := range obj.Elements {
			arr[i] = objectToJSONSeen(el, seen)
		}
		return arr
	case *object.Tuple:
		// Python serializes tuples as JSON arrays.
		if _, cyclic := seen[obj]; cyclic {
			return "<cyclic reference>"
		}
		seen[obj] = struct{}{}
		defer delete(seen, obj)
		arr := make([]interface{}, len(obj.Elements))
		for i, el := range obj.Elements {
			arr[i] = objectToJSONSeen(el, seen)
		}
		return arr
	case *object.Dict:
		if _, cyclic := seen[obj]; cyclic {
			return "<cyclic reference>"
		}
		seen[obj] = struct{}{}
		defer delete(seen, obj)
		m := make(map[string]interface{}, len(obj.Pairs))
		for _, pair := range obj.Pairs {
			m[pair.StringKey()] = objectToJSONSeen(pair.Value, seen)
		}
		return m
	case *object.Null:
		return nil
	default:
		return obj.Inspect()
	}
}
