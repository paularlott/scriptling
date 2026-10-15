package conversion

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// maxJSONDepth bounds nesting so a hostile document cannot exhaust the stack.
const maxJSONDepth = 10000

// ParseJSON parses a JSON string and returns a Scriptling object.
// It uses UseNumber() to preserve large integers, and builds dicts in the
// document's key order (as Python's json.loads does) rather than through a Go
// map, whose iteration order is random.
func ParseJSON(jsonStr string) (object.Object, error) {
	decoder := json.NewDecoder(strings.NewReader(jsonStr))
	decoder.UseNumber()
	return decodeJSONValue(decoder, 0)
}

// decodeJSONValue reads one JSON value from the token stream, keeping object
// keys in document order. Duplicate keys keep the first position and the last
// value, as in Python.
func decodeJSONValue(dec *json.Decoder, depth int) (object.Object, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	delim, isDelim := tok.(json.Delim)
	if !isDelim {
		return FromGo(tok), nil
	}
	if depth >= maxJSONDepth {
		return nil, fmt.Errorf("maximum JSON nesting depth exceeded")
	}
	switch delim {
	case '{':
		d := object.NewDict()
		for dec.More() {
			keyTok, err := dec.Token()
			if err != nil {
				return nil, err
			}
			key, ok := keyTok.(string)
			if !ok {
				return nil, fmt.Errorf("invalid object key %v", keyTok)
			}
			val, err := decodeJSONValue(dec, depth+1)
			if err != nil {
				return nil, err
			}
			d.Store(object.DictStringKey(key), object.NewString(key), val)
		}
		if _, err := dec.Token(); err != nil { // closing }
			return nil, err
		}
		return d, nil
	case '[':
		elements := []object.Object{}
		for dec.More() {
			val, err := decodeJSONValue(dec, depth+1)
			if err != nil {
				return nil, err
			}
			elements = append(elements, val)
		}
		if _, err := dec.Token(); err != nil { // closing ]
			return nil, err
		}
		return &object.List{Elements: elements}, nil
	}
	return nil, fmt.Errorf("unexpected JSON delimiter %q", delim)
}

// MustParseJSON parses a JSON string and returns a Scriptling object,
// returning an Error object if parsing fails.
func MustParseJSON(jsonStr string) object.Object {
	result, err := ParseJSON(jsonStr)
	if err != nil {
		// Python's json.JSONDecodeError subclasses ValueError; the
		// exception hierarchy maps JSONDecodeError→ValueError, so both
		// `except ValueError` and `except json.JSONDecodeError` catch it.
		decodeErr := errors.NewError("JSONDecodeError: %s", err.Error())
		decodeErr.ExceptionType = "JSONDecodeError"
		return decodeErr
	}
	return result
}

// FromGo converts a Go interface{} value to a scriptling Object.
// It handles primitive types (nil, bool, int, float, string), nested structures
// (maps, slices), and falls back to JSON marshaling for unknown types.
func FromGo(v interface{}) object.Object {
	switch v := v.(type) {
	case nil:
		return &object.Null{}
	case bool:
		return object.NewBoolean(v)
	case int:
		return object.NewInteger(int64(v))
	case int8:
		return object.NewInteger(int64(v))
	case int16:
		return object.NewInteger(int64(v))
	case int32:
		return object.NewInteger(int64(v))
	case int64:
		return object.NewInteger(v)
	case uint:
		return object.NewInteger(int64(v))
	case uint8:
		return object.NewInteger(int64(v))
	case uint16:
		return object.NewInteger(int64(v))
	case uint32:
		return object.NewInteger(int64(v))
	case uint64:
		// Note: May overflow for very large uint64 values
		return object.NewInteger(int64(v))
	case float32:
		return object.NewFloat(float64(v))
	case float64:
		return object.NewFloat(v)
	case json.Number:
		// Try to parse as integer first, then fall back to float
		if intVal, err := v.Int64(); err == nil {
			return object.NewInteger(intVal)
		}
		if floatVal, err := v.Float64(); err == nil {
			return object.NewFloat(floatVal)
		}
		return object.NewString(string(v))
	case string:
		return object.NewString(v)
	case []byte:
		return object.NewBytes(v)
	case []interface{}:
		elements := make([]object.Object, len(v))
		for i, item := range v {
			elements[i] = FromGo(item)
		}
		return &object.List{Elements: elements}
	case map[string]interface{}:
		// A Go map has no order; insert in sorted key order so the dict is
		// deterministic (and ordered for later mutation).
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		d := object.NewDictSized(len(v))
		for _, key := range keys {
			d.Store(object.DictStringKey(key), object.NewString(key), FromGo(v[key]))
		}
		return d
	case map[interface{}]interface{}:
		type kv struct {
			key string
			val interface{}
		}
		items := make([]kv, 0, len(v))
		for key, val := range v {
			keyStr := ""
			switch k := key.(type) {
			case string:
				keyStr = k
			default:
				keyStr = fmt.Sprintf("%v", k)
			}
			items = append(items, kv{keyStr, val})
		}
		sort.Slice(items, func(i, j int) bool { return items[i].key < items[j].key })
		d := object.NewDictSized(len(items))
		for _, it := range items {
			d.Store(object.DictStringKey(it.key), object.NewString(it.key), FromGo(it.val))
		}
		return d
	default:
		// For unknown types, try to convert to JSON then parse
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			return object.NewString(fmt.Sprintf("%v", v))
		}
		var result interface{}
		if err := json.Unmarshal(jsonBytes, &result); err != nil {
			return object.NewString(fmt.Sprintf("%v", v))
		}
		return FromGo(result)
	}
}

// cyclicRefPlaceholder replaces a container that references itself (directly
// or through a chain) when converting to Go, mirroring how repr shows cyclic
// Python lists. Without the check a self-referential list built by a script
// recurses until the Go stack overflows, which no recover() can catch.
const cyclicRefPlaceholder = "<cyclic reference>"

// ToGo converts a scriptling Object to a Go interface{}.
// It returns the underlying Go value for the object type.
func ToGo(obj object.Object) interface{} {
	return toGo(obj, make(map[object.Object]struct{}), nil, 0)
}

// maxToGoDepth bounds how deeply nested a value may be converted: deeper
// than this the Go stack is at risk, so the rest is replaced by
// tooDeepPlaceholder (or, in a limited conversion, is an error).
const maxToGoDepth = 10000

const tooDeepPlaceholder = "<too deeply nested>"

// mapEntryBytes estimates a converted map's memory per entry: the key
// header, the value interface and the bucket's share.
const mapEntryBytes = 64

// ToGoLimited is ToGo held to the limits of the interpreter on the calling
// goroutine. A value shared by several containers converts once per
// reference, so a small script value can expand into a huge Go value; the
// conversion counts what it builds and stops with a MemoryError once that
// passes the value limit or the memory left. Use it for script data handed
// to libraries (templates, encoders, request bodies).
func ToGoLimited(obj object.Object) (interface{}, *object.Error) {
	var b conversionBudget
	v := toGo(obj, make(map[object.Object]struct{}), &b, 0)
	if b.err != nil {
		return nil, b.err
	}
	// The periodic check leaves a remainder: settle it.
	if errObj := b.check(b.nodes); errObj != nil {
		return nil, errObj
	}
	return v, nil
}

// Per converted value: the least it encodes to (a digit and a separator),
// which is what the value limit sees, and about what it holds in Go, which
// is what the memory left sees.
const (
	convertedValueBytes  = 2
	convertedMemoryBytes = 16
)

// check holds nodes converted values to the limits: as the encoded value
// they are about to become, and as the memory they hold meanwhile. The
// encoded value is a single value, so sharing cannot expand a small script
// value past the value limit, while data a script was allowed to build is
// not refused for its node count.
func (b *conversionBudget) check(nodes int64) *object.Error {
	if errObj := object.CheckAllocFor("converted value", nodes*convertedValueBytes); errObj != nil {
		return errObj.(*object.Error)
	}
	if errObj := object.CheckRoom("converted value", nodes*convertedMemoryBytes); errObj != nil {
		return errObj.(*object.Error)
	}
	return nil
}

// conversionBudget counts the Go values a conversion has built. Every
// budgetCheckEvery values it checks the total against the interpreter's
// limits, as a container of that many elements; once over, the conversion
// stops building.
type conversionBudget struct {
	nodes int64
	err   *object.Error
}

const budgetCheckEvery = 1024

// reserve checks a container about to be allocated for n values, when n is
// large enough to matter on its own, against the limits with what has been
// built so far.
func (b *conversionBudget) reserve(n int) bool {
	if b.err != nil {
		return false
	}
	if n < budgetCheckEvery {
		return true
	}
	// The container itself is a value of n slots; with what was built
	// before it, it is part of the one value being converted.
	if errObj := object.CheckElements(int64(n)); errObj != nil {
		b.err = errObj.(*object.Error)
		return false
	}
	if errObj := b.check(b.nodes + int64(n)); errObj != nil {
		b.err = errObj
		return false
	}
	return true
}

// ok counts one value and reports whether the conversion may go on.
func (b *conversionBudget) ok() bool {
	if b.err != nil {
		return false
	}
	b.nodes++
	if b.nodes&(budgetCheckEvery-1) != 0 {
		return true
	}
	if errObj := b.check(b.nodes); errObj != nil {
		b.err = errObj
		return false
	}
	return true
}

// toGo walks obj with the set of containers on the current path. A container
// already on the path is a cycle; entries are removed again on the way out so
// a substructure shared by two parents (a DAG, not a cycle) still converts
// fully. A nil budget converts without limit.
func toGo(obj object.Object, path map[object.Object]struct{}, b *conversionBudget, depth int) interface{} {
	if obj == nil {
		return nil
	}
	if b != nil && !b.ok() {
		return nil
	}
	if depth > maxToGoDepth {
		if b != nil {
			b.err = object.NewMemoryError("value too deeply nested to convert (more than %d levels)", maxToGoDepth)
			return nil
		}
		return tooDeepPlaceholder
	}

	switch o := obj.(type) {
	case *object.Null:
		return nil
	case *object.Boolean:
		return o.BoolValue()
	case *object.Integer:
		return o.IntValue()
	case *object.Float:
		return o.FloatValue()
	case *object.String:
		return o.StringValue()
	case *object.Bytes:
		return o.BytesValue()
	case *object.List:
		if _, cyclic := path[o]; cyclic {
			return cyclicRefPlaceholder
		}
		path[o] = struct{}{}
		if b != nil && !b.reserve(len(o.Elements)) {
			return nil
		}
		result := make([]interface{}, len(o.Elements))
		object.TrackSlice(result)
		for i, elem := range o.Elements {
			result[i] = toGo(elem, path, b, depth+1)
		}
		delete(path, o)
		return result
	case *object.Tuple:
		if _, cyclic := path[o]; cyclic {
			return cyclicRefPlaceholder
		}
		path[o] = struct{}{}
		if b != nil && !b.reserve(len(o.Elements)) {
			return nil
		}
		result := make([]interface{}, len(o.Elements))
		object.TrackSlice(result)
		for i, elem := range o.Elements {
			result[i] = toGo(elem, path, b, depth+1)
		}
		delete(path, o)
		return result
	case *object.Dict:
		if _, cyclic := path[o]; cyclic {
			return cyclicRefPlaceholder
		}
		path[o] = struct{}{}
		if b != nil && !b.reserve(len(o.Pairs)) {
			return nil
		}
		result := make(map[string]interface{}, len(o.Pairs))
		object.TrackMap(result, int64(len(o.Pairs))*mapEntryBytes)
		for _, pair := range o.Pairs {
			result[pair.StringKey()] = toGo(pair.Value, path, b, depth+1)
		}
		delete(path, o)
		return result
	case *object.Error:
		return o.Message
	case *object.Builtin:
		// Return help text if available, otherwise generic string
		if o.HelpText != "" {
			return o.HelpText
		}
		return "<builtin function>"
	case *object.Function:
		return o.Name
	case *object.FloatArray:
		if o.Is2D() {
			rows := o.Rows()
			cols := o.Cols()
			if b != nil && !b.reserve(rows*cols) {
				return nil
			}
			result := make([]interface{}, rows)
			object.TrackSlice(result)
			for i := 0; i < rows; i++ {
				row := make([]interface{}, cols)
				object.TrackSlice(row)
				off := i * cols
				for j := 0; j < cols; j++ {
					row[j] = o.Data[off+j]
				}
				result[i] = row
			}
			return result
		}
		if b != nil && !b.reserve(len(o.Data)) {
			return nil
		}
		result := make([]interface{}, len(o.Data))
		object.TrackSlice(result)
		for i, v := range o.Data {
			result[i] = v
		}
		return result
	default:
		// For other types (like ReturnValue, Break, Continue), return string representation
		return o.Inspect()
	}
}

// ToGoError converts a scriptling Object to a Go error.
// If the object is an Error type, it returns a Go error with the error message.
// Otherwise, it returns nil.
func ToGoError(obj object.Object) error {
	if obj == nil {
		return nil
	}
	if err, ok := obj.(*object.Error); ok {
		return fmt.Errorf("%s", err.Inspect())
	}
	return nil
}

// ToBytes accepts a Bytes or String input and returns the raw byte slice.
// Used by libraries that historically took strings but now also accept Bytes
// (file I/O, HTTP bodies, socket send, etc.). Returns a Scriptling *Error
// object for any other type so callers can return it directly.
//
// Bytes are returned as-is (no copy — treat the result as read-only).
// Strings are UTF-8 encoded into a fresh byte slice.
func ToBytes(obj object.Object) ([]byte, object.Object) {
	switch v := obj.(type) {
	case *object.Bytes:
		return v.BytesValue(), nil
	case *object.String:
		return []byte(v.StringValue()), nil
	default:
		return nil, errors.NewTypeError("BYTES or STRING", obj.Type().String())
	}
}

// ToGoWithError converts a Scriptling object to a Go value, returning error for complex types
func ToGoWithError(obj object.Object) (interface{}, *object.Error) {
	switch v := obj.(type) {
	case *object.String:
		return v.StringValue(), nil
	case *object.Bytes:
		return v.BytesValue(), nil
	case *object.Integer:
		return v.IntValue(), nil
	case *object.Float:
		return v.FloatValue(), nil
	case *object.Boolean:
		return v.BoolValue(), nil
	case *object.Null:
		return nil, nil
	case *object.List:
		result := make([]interface{}, len(v.Elements))
		for i, elem := range v.Elements {
			converted, err := ToGoWithError(elem)
			if err != nil {
				return nil, err
			}
			result[i] = converted
		}
		return result, nil
	case *object.Dict:
		result := make(map[string]interface{})
		for _, pair := range v.Pairs {
			converted, err := ToGoWithError(pair.Value)
			if err != nil {
				return nil, err
			}
			result[pair.StringKey()] = converted
		}
		return result, nil
	default:
		return nil, errors.NewError("cannot convert complex types to Go")
	}
}
