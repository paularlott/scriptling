package object

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	stderrors "errors"
	"fmt"
	"hash"
	"io"
	"maps"
	"math"
	"os"
	"reflect"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"unsafe"
	"weak"

	"github.com/paularlott/scriptling/ast"
)

// DictKey returns a canonical string key for use in Dict and Set maps.
// Matches Python 3 semantics where:
//   - int(1), float(1.0), and True all map to the same key
//   - str("1") maps to a different key
//   - None maps to its own unique key
func DictKey(obj Object) string {
	switch o := obj.(type) {
	case *Integer:
		return intDictKey(o.value)
	case *Float:
		if !math.IsInf(o.value, 0) && !math.IsNaN(o.value) && o.value == math.Trunc(o.value) && o.value >= math.MinInt64 && o.value <= math.MaxInt64 {
			return intDictKey(int64(o.value))
		}
		return "f:" + strconv.FormatFloat(o.value, 'g', -1, 64)
	case *Boolean:
		if o.value {
			return "n:1"
		}
		return "n:0"
	case *String:
		return stringDictKey(o.value)
	case *Bytes:
		// Equal bytes must produce equal keys regardless of pointer identity,
		// so encode the contents with a prefix that can't collide with strings.
		return "b:" + hex.EncodeToString(o.value)
	case *Null:
		return "null:"
	case *Set:
		// Frozen sets hash by content so equal frozensets land on the same
		// dict/set key, as in Python. Mutable sets keep the identity key of
		// the default case (they are unhashable at the language level).
		if o.Frozen {
			keys := o.Keys()
			sort.Strings(keys)
			return "fs:{" + strings.Join(keys, ",") + "}"
		}
		return fmt.Sprintf("%s:%p", obj.Type(), obj)
	case *Tuple:
		// Tuples are hashable in Python if all elements are hashable
		var k tupleKeyer
		return k.key(o)
	default:
		// Unhashable types - use type + pointer identity which will rarely collide
		// In Python, lists, dicts, sets are unhashable and raise TypeError
		return fmt.Sprintf("%s:%p", obj.Type(), obj)
	}
}

// tupleKeyHashAt is the length past which a tuple's key is replaced by a
// digest of it. A tuple can hold the same tuple many times over, so the
// spelled-out key of a small tuple can be exponentially long; a digest keeps
// every key short, and with each distinct tuple keyed once (tupleKeyer's
// memo) a key costs time in proportion to the tuple's own size.
const tupleKeyHashAt = 1024

// tupleKeyMemoAfter is how many nested tuples a key visits before it
// remembers their keys, so ordinary keys allocate no memo.
const tupleKeyMemoAfter = 64

// tupleKeyer builds tuple dict keys: "t:(" and the element keys, or, past
// tupleKeyHashAt, "th:" and the SHA-256 of that text. Equal tuples get equal
// keys either way, and the two forms never collide.
type tupleKeyer struct {
	visited int
	memo    map[*Tuple]string
}

func (k *tupleKeyer) key(t *Tuple) string {
	if k.memo != nil {
		if key, ok := k.memo[t]; ok {
			return key
		}
	}
	k.visited++
	var b strings.Builder
	var h hash.Hash
	b.WriteString("t:(")
	for i, e := range t.Elements {
		if i > 0 {
			b.WriteByte(',')
		}
		if et, ok := e.(*Tuple); ok {
			b.WriteString(k.key(et))
		} else {
			b.WriteString(DictKey(e))
		}
		if b.Len() > tupleKeyHashAt {
			if h == nil {
				h = sha256.New()
			}
			h.Write([]byte(b.String()))
			b.Reset()
		}
	}
	b.WriteByte(')')
	var key string
	if h != nil {
		h.Write([]byte(b.String()))
		key = "th:" + hex.EncodeToString(h.Sum(nil))
	} else {
		key = b.String()
	}
	if k.memo == nil && k.visited > tupleKeyMemoAfter {
		k.memo = make(map[*Tuple]string)
	}
	if k.memo != nil {
		k.memo[t] = key
	}
	return key
}

// DictStringKey returns the canonical dict key for a string key without
// requiring a temporary String object allocation. It must agree with the *String
// case of DictKey.
func DictStringKey(name string) string {
	return stringDictKey(name)
}

// stringDictKey encodes a string as a dict map key.
//
// Every other kind of key carries a type prefix, and every one of those prefixes
// contains a colon ("n:", "f:", "b:", "t:", "null:", "LIST:0x…"). A string with
// no colon therefore cannot be confused with any of them and is used verbatim —
// which makes the common case (identifiers, field names, words) allocation-free.
// A string that does contain a colon gets the "s:" prefix, and since that prefix
// itself contains a colon it can never collide with a verbatim key either.
func stringDictKey(s string) string {
	if strings.IndexByte(s, ':') < 0 {
		return s
	}
	return "s:" + s
}

// IsHashable reports whether obj can be used as a set element or dict key.
// Matches Python semantics: int, float, bool, string, bytes, None, and tuples
// of hashable elements are hashable; lists, dicts, sets, and instances are
// not unless the instance defines __hash__.
func IsHashable(obj Object) bool {
	switch o := obj.(type) {
	case *Integer, *Float, *Boolean, *String, *Bytes, *Null:
		return true
	case *Exception:
		// Exceptions hash by identity (like a default Python object). Dict keys
		// already accept them via evalHashKey; sets must agree.
		return true
	case *Sentinel:
		// Sentinels hash by identity (PEP 661): a sentinel's whole purpose is
		// uniqueness, so identity keys are exactly right.
		return true
	case *Tuple:
		var c hashableChecker
		return c.tuple(o)
	case *Set:
		// Frozen sets are immutable, so their content hash is stable (Python
		// frozenset); mutable sets are unhashable.
		return o.Frozen
	case *Instance:
		_, ok := o.Class.Methods["__hash__"]
		return ok
	default:
		return false
	}
}

// hashableChecker checks a tuple's elements, remembering (past
// tupleKeyMemoAfter tuples) the tuples it has found hashable, so a tuple that
// holds the same tuple many times over is checked in proportion to its own
// size rather than the size it spells out.
type hashableChecker struct {
	visited int
	ok      map[*Tuple]struct{}
}

func (c *hashableChecker) tuple(t *Tuple) bool {
	if c.ok != nil {
		if _, done := c.ok[t]; done {
			return true
		}
	}
	c.visited++
	for _, e := range t.Elements {
		if et, isTuple := e.(*Tuple); isTuple {
			if !c.tuple(et) {
				return false
			}
		} else if !IsHashable(e) {
			return false
		}
	}
	if c.ok == nil && c.visited > tupleKeyMemoAfter {
		c.ok = make(map[*Tuple]struct{})
	}
	if c.ok != nil {
		c.ok[t] = struct{}{}
	}
	return true
}

// Small integer cache for common values (-5 to 10000)
// This follows Python's approach and eliminates allocations for loop counters
// Extended range to 10000 for better loop performance
const (
	smallIntMin = -5
	smallIntMax = 10000

	// Type conversion error messages (exported for use by external packages)
	ErrMustBeString   = "must be a string"
	ErrMustBeInteger  = "must be an integer"
	ErrMustBeNumber   = "must be a number"
	ErrMustBeBoolean  = "must be a boolean"
	ErrMustBeList     = "must be a list"
	ErrMustBeDict     = "must be a dict"
	ErrMustBeIterable = "must be iterable"
	ErrMustBeBytes    = "must be bytes or string"
)

// smallIntegers holds the cached Integer values by value rather than by pointer.
// Stored inline, the array is pointer-free, so it lives in BSS instead of the
// heap: no per-entry allocation, no 80KB pointer table for the GC to rescan as a
// root on every cycle, and consecutive integers land next to each other in
// memory. NewInteger hands out interior pointers, which is safe because a
// package-level array never moves and Integer is immutable.
var smallIntegers [smallIntMax - smallIntMin + 1]Integer

// Pre-allocated error singletons for type accessor methods.
// These avoid allocating a new Error on every failed AsXxx() call.
var (
	errMustBeString  = &Error{Message: ErrMustBeString, ExceptionType: ExceptionTypeTypeError}
	errMustBeInteger = &Error{Message: ErrMustBeInteger, ExceptionType: ExceptionTypeTypeError}
	errMustBeNumber  = &Error{Message: ErrMustBeNumber, ExceptionType: ExceptionTypeTypeError}
	errMustBeBoolean = &Error{Message: ErrMustBeBoolean, ExceptionType: ExceptionTypeTypeError}
	errMustBeList    = &Error{Message: ErrMustBeList, ExceptionType: ExceptionTypeTypeError}
	errMustBeDict    = &Error{Message: ErrMustBeDict, ExceptionType: ExceptionTypeTypeError}
)

// Exception type constants
const (
	ExceptionTypeSystemExit         = "SystemExit"
	ExceptionTypePermissionError    = "PermissionError"
	ExceptionTypeException          = "Exception"
	ExceptionTypeValueError         = "ValueError"
	ExceptionTypeTypeError          = "TypeError"
	ExceptionTypeNameError          = "NameError"
	ExceptionTypeImportError        = "ImportError"
	ExceptionTypeStopIteration      = "StopIteration"
	ExceptionTypeRuntimeError       = "RuntimeError"
	ExceptionTypeRecursionError     = "RecursionError"
	ExceptionTypeZeroDivisionError  = "ZeroDivisionError"
	ExceptionTypeOverflowError      = "OverflowError"
	ExceptionTypeMemoryError        = "MemoryError"
	ExceptionTypeIndexError         = "IndexError"
	ExceptionTypeKeyError           = "KeyError"
	ExceptionTypeAttributeError     = "AttributeError"
	ExceptionTypeOSError            = "OSError"
	ExceptionTypeAssertionError     = "AssertionError"
	ExceptionTypeSyntaxError        = "SyntaxError"
	ExceptionTypeLookupError        = "LookupError"
	ExceptionTypeUnicodeEncodeError = "UnicodeEncodeError"
	ExceptionTypeUnicodeDecodeError = "UnicodeDecodeError"
	ExceptionTypeGeneric            = "" // Default for legacy compatibility
)

// Small integer cache for common values (-5 to 10000)

// Break and Continue singletons (like NULL, TRUE, FALSE)
var (
	BREAK    = &Break{}
	CONTINUE = &Continue{}
)

func init() {
	// Initialize small integer cache
	for i := smallIntMin; i <= smallIntMax; i++ {
		smallIntegers[i-smallIntMin].value = int64(i)
	}
	// Initialize the small integer dict-key cache ("n:0".."n:N").
	for i := range smallDictKeys {
		smallDictKeys[i] = "n:" + strconv.Itoa(i)
	}
}

// Small integer dict-key cache. Integer (and integral float) dict keys are
// extremely common (loop indices, ids); caching the canonical "n:N" string for
// small non-negative values makes those keys allocation-free, and AppendInt
// handles the rest in a single allocation instead of two.
const smallDictKeyMax = 1024

var smallDictKeys [smallDictKeyMax]string

// intDictKey returns the canonical dict key string for an integer value,
// equivalent to "n:" + strconv.FormatInt(v, 10) but allocation-free for small
// non-negative values and a single allocation otherwise.
func intDictKey(v int64) string {
	if v >= 0 && v < smallDictKeyMax {
		return smallDictKeys[v]
	}
	var buf [24]byte
	b := append(buf[:0], 'n', ':')
	b = strconv.AppendInt(b, v, 10)
	return string(b)
}

// NewInteger returns a cached integer for small values, or a new Integer for larger values
func NewInteger(val int64) *Integer {
	if val >= smallIntMin && val <= smallIntMax {
		return &smallIntegers[val-smallIntMin]
	}
	return &Integer{value: val}
}

type ObjectType int

type classLookupCacheEntry struct {
	value Object
	ok    bool
	epoch uint64
}

type boundMethodCacheEntry struct {
	bound  *BoundMethod
	method Object
	epoch  uint64
}

var classLookupEpoch atomic.Uint64

const (
	INTEGER_OBJ ObjectType = iota
	FLOAT_OBJ
	BOOLEAN_OBJ
	STRING_OBJ
	NULL_OBJ
	RETURN_OBJ
	BREAK_OBJ
	CONTINUE_OBJ
	FUNCTION_OBJ
	LAMBDA_OBJ
	BUILTIN_OBJ
	LIST_OBJ
	TUPLE_OBJ
	DICT_OBJ
	ERROR_OBJ
	EXCEPTION_OBJ
	CLASS_OBJ
	INSTANCE_OBJ
	SUPER_OBJ
	ITERATOR_OBJ
	DICT_KEYS_OBJ
	DICT_VALUES_OBJ
	DICT_ITEMS_OBJ
	SET_OBJ
	SLICE_OBJ
	PROPERTY_OBJ
	STATICMETHOD_OBJ
	CLASSMETHOD_OBJ
	FLOAT_ARRAY_OBJ
	BYTES_OBJ
	SENTINEL_OBJ
)

// String returns the string representation of the ObjectType
func (ot ObjectType) String() string {
	switch ot {
	case INTEGER_OBJ:
		return "INTEGER"
	case FLOAT_OBJ:
		return "FLOAT"
	case BOOLEAN_OBJ:
		return "BOOLEAN"
	case STRING_OBJ:
		return "STRING"
	case NULL_OBJ:
		return "NULL"
	case RETURN_OBJ:
		return "RETURN"
	case BREAK_OBJ:
		return "BREAK"
	case CONTINUE_OBJ:
		return "CONTINUE"
	case FUNCTION_OBJ:
		return "FUNCTION"
	case LAMBDA_OBJ:
		return "LAMBDA"
	case BUILTIN_OBJ:
		return "BUILTIN"
	case LIST_OBJ:
		return "LIST"
	case TUPLE_OBJ:
		return "TUPLE"
	case DICT_OBJ:
		return "DICT"
	case ERROR_OBJ:
		return "ERROR"
	case EXCEPTION_OBJ:
		return "EXCEPTION"
	case CLASS_OBJ:
		return "CLASS"
	case INSTANCE_OBJ:
		return "INSTANCE"
	case SUPER_OBJ:
		return "SUPER"
	case ITERATOR_OBJ:
		return "ITERATOR"
	case DICT_KEYS_OBJ:
		return "DICT_KEYS"
	case DICT_VALUES_OBJ:
		return "DICT_VALUES"
	case DICT_ITEMS_OBJ:
		return "DICT_ITEMS"
	case SET_OBJ:
		return "SET"
	case SLICE_OBJ:
		return "SLICE"
	case PROPERTY_OBJ:
		return "PROPERTY"
	case STATICMETHOD_OBJ:
		return "STATICMETHOD"
	case CLASSMETHOD_OBJ:
		return "CLASSMETHOD"
	case FLOAT_ARRAY_OBJ:
		return "FLOAT_ARRAY"
	case BYTES_OBJ:
		return "BYTES"
	case SENTINEL_OBJ:
		return "SENTINEL"
	default:
		return "UNKNOWN"
	}
}

type Object interface {
	Type() ObjectType
	Inspect() string

	// Type-safe accessor methods (strict type checking)
	AsString() (string, Object)
	AsInt() (int64, Object)
	AsFloat() (float64, Object)
	AsBool() (bool, Object)
	AsList() ([]Object, Object)
	AsDict() (map[string]Object, Object)

	// Coercion methods (loose type conversion with best effort)
	CoerceString() (string, Object)
	CoerceInt() (int64, Object)
	CoerceFloat() (float64, Object)
}

type Integer struct {
	value int64
}

func (i *Integer) IntValue() int64 { return i.value }

func (i *Integer) Type() ObjectType { return INTEGER_OBJ }
func (i *Integer) Inspect() string  { return strconv.FormatInt(i.value, 10) }

func (i *Integer) AsString() (string, Object)          { return "", errMustBeString }
func (i *Integer) AsInt() (int64, Object)              { return i.value, nil }
func (i *Integer) AsFloat() (float64, Object)          { return float64(i.value), nil }
func (i *Integer) AsBool() (bool, Object)              { return i.value != 0, nil }
func (i *Integer) AsList() ([]Object, Object)          { return nil, errMustBeList }
func (i *Integer) AsDict() (map[string]Object, Object) { return nil, errMustBeDict }

func (i *Integer) CoerceString() (string, Object) { return i.Inspect(), nil }
func (i *Integer) CoerceInt() (int64, Object)     { return i.value, nil }
func (i *Integer) CoerceFloat() (float64, Object) { return float64(i.value), nil }

type Float struct {
	value float64
}

func (f *Float) FloatValue() float64 { return f.value }

func (f *Float) Type() ObjectType { return FLOAT_OBJ }
func (f *Float) Inspect() string  { return FloatStr(f.value) }

func (f *Float) AsString() (string, Object)          { return "", errMustBeString }
func (f *Float) AsInt() (int64, Object)              { return int64(f.value), nil }
func (f *Float) AsFloat() (float64, Object)          { return f.value, nil }
func (f *Float) AsBool() (bool, Object)              { return f.value != 0, nil }
func (f *Float) AsList() ([]Object, Object)          { return nil, errMustBeList }
func (f *Float) AsDict() (map[string]Object, Object) { return nil, errMustBeDict }

func (f *Float) CoerceString() (string, Object) { return f.Inspect(), nil }
func (f *Float) CoerceInt() (int64, Object)     { return int64(f.value), nil }
func (f *Float) CoerceFloat() (float64, Object) { return f.value, nil }

var (
	boolTrue  = &Boolean{value: true}
	boolFalse = &Boolean{value: false}
)

func NewBoolean(v bool) *Boolean {
	if v {
		return boolTrue
	}
	return boolFalse
}

func NewFloat(v float64) *Float {
	return &Float{value: v}
}

func NewString(v string) *String {
	str := &String{value: v}
	if accountingOn.Load() {
		trackString(str)
	}
	return str
}

// asciiStrings holds one shared String for each ASCII character. Strings are
// immutable, so every one-character string of that character can be the same
// object, as with small integers.
var asciiStrings = func() (t [128]String) {
	for i := range t {
		t[i].value = ByteString(byte(i))
	}
	return t
}()

// NewRuneString returns the one-character string for r. ASCII characters come
// from a shared table, so iterating over or indexing into text allocates
// nothing for them.
func NewRuneString(r rune) *String {
	if r >= 0 && r < int32(len(asciiStrings)) {
		return &asciiStrings[r]
	}
	return NewString(string(r))
}

// stringObjectOverhead approximates the bytes a String costs beyond its text:
// the object itself and the string header it holds.
const stringObjectOverhead = int64(unsafe.Sizeof(String{})) + 16

type Boolean struct {
	value bool
}

func (b *Boolean) BoolValue() bool { return b.value }

func (b *Boolean) Type() ObjectType { return BOOLEAN_OBJ }
func (b *Boolean) Inspect() string {
	if b.value {
		return "True"
	}
	return "False"
}

func (b *Boolean) AsString() (string, Object) { return "", errMustBeString }

// A bool is an int in Python: wherever an int or a float is taken, True is 1
// and False is 0.
func (b *Boolean) AsInt() (int64, Object)              { return b.CoerceInt() }
func (b *Boolean) AsFloat() (float64, Object)          { return b.CoerceFloat() }
func (b *Boolean) AsBool() (bool, Object)              { return b.value, nil }
func (b *Boolean) AsList() ([]Object, Object)          { return nil, errMustBeList }
func (b *Boolean) AsDict() (map[string]Object, Object) { return nil, errMustBeDict }

func (b *Boolean) CoerceString() (string, Object) { return b.Inspect(), nil }
func (b *Boolean) CoerceInt() (int64, Object) {
	if b.value {
		return 1, nil
	}
	return 0, nil
}
func (b *Boolean) CoerceFloat() (float64, Object) {
	if b.value {
		return 1, nil
	}
	return 0, nil
}

// CoerceWireString coerces obj to its machine-facing string form, for wire
// formats and protocol payloads (query params, messages, tool responses).
// It behaves like CoerceString except booleans render in their lowercase
// wire form ("true"/"false") rather than the Python display form
// ("True"/"False").
func CoerceWireString(obj Object) (string, Object) {
	if b, ok := obj.(*Boolean); ok {
		return strconv.FormatBool(b.value), nil
	}
	return obj.CoerceString()
}

type String struct {
	value string
}

func (s *String) StringValue() string { return s.value }

func (s *String) Type() ObjectType { return STRING_OBJ }
func (s *String) Inspect() string  { return s.value }

func (s *String) AsString() (string, Object)          { return s.value, nil }
func (s *String) AsInt() (int64, Object)              { return 0, errMustBeInteger }
func (s *String) AsFloat() (float64, Object)          { return 0, errMustBeNumber }
func (s *String) AsBool() (bool, Object)              { return s.value != "", nil }
func (s *String) AsList() ([]Object, Object)          { return nil, errMustBeList }
func (s *String) AsDict() (map[string]Object, Object) { return nil, errMustBeDict }

func (s *String) CoerceString() (string, Object) { return s.value, nil }
func (s *String) CoerceInt() (int64, Object) {
	val, err := strconv.ParseInt(strings.TrimSpace(s.value), 10, 64)
	if err != nil {
		return 0, &Error{Message: fmt.Sprintf("cannot convert %s to int", s.value), ExceptionType: ExceptionTypeValueError}
	}
	return val, nil
}
func (s *String) CoerceFloat() (float64, Object) {
	val, err := strconv.ParseFloat(strings.TrimSpace(s.value), 64)
	if err != nil {
		return 0, &Error{Message: fmt.Sprintf("cannot convert %s to float", s.value), ExceptionType: ExceptionTypeValueError}
	}
	return val, nil
}

// Bytes is an immutable sequence of byte values (0-255), analogous to Python's
// bytes type. It is the canonical representation for binary data in Scriptling.
// Strict string operations raise a type error; use Decode() to obtain a string.
type Bytes struct {
	value []byte
}

// NewBytes returns a new Bytes object that owns a defensive copy of the given
// slice, so callers may freely mutate the original after the call.
func NewBytes(b []byte) *Bytes {
	if b == nil {
		b = []byte{}
	}
	cp := make([]byte, len(b))
	copy(cp, b)
	obj := &Bytes{value: cp}
	if accountingOn.Load() {
		trackBytes(obj)
	}
	return obj
}

// bytesObjectOverhead approximates the bytes a Bytes costs beyond its data.
const bytesObjectOverhead = int64(unsafe.Sizeof(Bytes{})) + 16

// NewBytesFromString returns a Bytes object whose contents are the UTF-8
// encoding of s. This is the equivalent of Python's str.encode("utf-8").
func NewBytesFromString(s string) *Bytes {
	obj := &Bytes{value: []byte(s)}
	if accountingOn.Load() {
		trackBytes(obj)
	}
	return obj
}

// BytesValue returns the underlying byte slice. Callers MUST NOT mutate the
// returned slice; treat it as read-only. If mutation is required, copy it.
func (b *Bytes) BytesValue() []byte { return b.value }

// Len returns the number of bytes.
func (b *Bytes) Len() int { return len(b.value) }

func (b *Bytes) Type() ObjectType { return BYTES_OBJ }

// Inspect returns a Python-style repr such as b'hello' or b'\x00\xff', suitable
// for debugging and print(). The default print() path uses CoerceString for a
// human-readable form; this is what shows up in the REPL and error messages.
func (b *Bytes) Inspect() string {
	var sb strings.Builder
	sb.Grow(len(b.value) + 3)
	sb.WriteString("b'")
	for _, c := range b.value {
		switch c {
		case '\'':
			sb.WriteString(`\'`)
		case '\\':
			sb.WriteString(`\\`)
		case '\n':
			sb.WriteString(`\n`)
		case '\r':
			sb.WriteString(`\r`)
		case '\t':
			sb.WriteString(`\t`)
		default:
			if c >= 0x20 && c < 0x7f {
				sb.WriteByte(c)
			} else {
				sb.WriteString(fmt.Sprintf(`\x%02x`, c))
			}
		}
	}
	sb.WriteByte('\'')
	return sb.String()
}

// AsString is strict: bytes are not strings. Callers that need a string must
// call Decode() explicitly (the .decode() method on Bytes). This mirrors
// Python's TypeError on mixing bytes and str.
func (b *Bytes) AsString() (string, Object)          { return "", errMustBeString }
func (b *Bytes) AsInt() (int64, Object)              { return 0, errMustBeInteger }
func (b *Bytes) AsFloat() (float64, Object)          { return 0, errMustBeNumber }
func (b *Bytes) AsBool() (bool, Object)              { return len(b.value) > 0, nil }
func (b *Bytes) AsList() ([]Object, Object)          { return nil, errMustBeList }
func (b *Bytes) AsDict() (map[string]Object, Object) { return nil, errMustBeDict }

// CoerceString returns a base64-encoded representation so that Bytes values can
// be printed, concatenated into error messages, or interpolated into f-strings
// without raising. Explicit decoding via .decode() should be preferred for real
// text content.
func (b *Bytes) CoerceString() (string, Object) {
	return base64.StdEncoding.EncodeToString(b.value), nil
}

// CoerceInt returns the length, mirroring the way Python treats bool/len
// coercion for truthiness only; numeric coercion of bytes is not meaningful.
func (b *Bytes) CoerceInt() (int64, Object)     { return 0, errMustBeInteger }
func (b *Bytes) CoerceFloat() (float64, Object) { return 0, errMustBeNumber }

// Equal reports whether two Bytes values have identical contents.
func (b *Bytes) Equal(other *Bytes) bool {
	return bytes.Equal(b.value, other.value)
}

type Slice struct {
	Start *Integer // nil means None (default start)
	End   *Integer // nil means None (default end)
	Step  *Integer // nil means None (default step = 1)
}

func (s *Slice) Type() ObjectType { return SLICE_OBJ }
func (s *Slice) Inspect() string {
	parts := []string{}
	if s.Start != nil {
		parts = append(parts, s.Start.Inspect())
	} else {
		parts = append(parts, "")
	}

	if s.End != nil {
		parts = append(parts, s.End.Inspect())
	} else {
		parts = append(parts, "")
	}

	if s.Step != nil && s.Step.value != 1 {
		parts = append(parts, s.Step.Inspect())
	}

	return fmt.Sprintf("slice(%s)", strings.Join(parts, ":"))
}

func (s *Slice) AsString() (string, Object)          { return "", errMustBeString }
func (s *Slice) AsInt() (int64, Object)              { return 0, errMustBeInteger }
func (s *Slice) AsFloat() (float64, Object)          { return 0, errMustBeNumber }
func (s *Slice) AsBool() (bool, Object)              { return false, errMustBeBoolean }
func (s *Slice) AsList() ([]Object, Object)          { return nil, errMustBeList }
func (s *Slice) AsDict() (map[string]Object, Object) { return nil, errMustBeDict }

func (s *Slice) CoerceString() (string, Object) { return s.Inspect(), nil }
func (s *Slice) CoerceInt() (int64, Object)     { return 0, errMustBeInteger }
func (s *Slice) CoerceFloat() (float64, Object) { return 0, errMustBeNumber }

type Null struct{}

func (n *Null) Type() ObjectType { return NULL_OBJ }
func (n *Null) Inspect() string  { return "None" }

func (n *Null) AsString() (string, Object)          { return "", errMustBeString }
func (n *Null) AsInt() (int64, Object)              { return 0, errMustBeInteger }
func (n *Null) AsFloat() (float64, Object)          { return 0, errMustBeNumber }
func (n *Null) AsBool() (bool, Object)              { return false, nil }
func (n *Null) AsList() ([]Object, Object)          { return nil, errMustBeList }
func (n *Null) AsDict() (map[string]Object, Object) { return nil, errMustBeDict }

func (n *Null) CoerceString() (string, Object) { return n.Inspect(), nil }
func (n *Null) CoerceInt() (int64, Object)     { return 0, nil }
func (n *Null) CoerceFloat() (float64, Object) { return 0, nil }

// Sentinel is a unique do-nothing value created by the sentinel() builtin
// (PEP 661). Each call returns a distinct object that is equal only to
// itself, so `value is MISSING` is the intended check. repr defaults to the
// name, which is the point of the feature: a readable marker in logs and the
// REPL instead of an opaque object address.
type Sentinel struct {
	Name string
	// Repr overrides Inspect when non-empty; empty means "use Name".
	Repr string
}

func NewSentinel(name, repr string) *Sentinel {
	return &Sentinel{Name: name, Repr: repr}
}

func (s *Sentinel) Type() ObjectType { return SENTINEL_OBJ }
func (s *Sentinel) Inspect() string {
	if s.Repr != "" {
		return s.Repr
	}
	return s.Name
}

func (s *Sentinel) AsString() (string, Object)          { return "", errMustBeString }
func (s *Sentinel) AsInt() (int64, Object)              { return 0, errMustBeInteger }
func (s *Sentinel) AsFloat() (float64, Object)          { return 0, errMustBeNumber }
func (s *Sentinel) AsBool() (bool, Object)              { return true, nil }
func (s *Sentinel) AsList() ([]Object, Object)          { return nil, errMustBeList }
func (s *Sentinel) AsDict() (map[string]Object, Object) { return nil, errMustBeDict }

func (s *Sentinel) CoerceString() (string, Object) { return s.Inspect(), nil }
func (s *Sentinel) CoerceInt() (int64, Object)     { return 0, errMustBeInteger }
func (s *Sentinel) CoerceFloat() (float64, Object) { return 0, errMustBeNumber }

type ReturnValue struct {
	Value Object
	// root points at the environment tree this frame was acquired from, so
	// ReleaseReturnValue can return it to the correct per-root free-list
	// without the caller threading the environment through. nil for values
	// constructed outside AcquireReturnValue (those are simply left to GC).
	root *rootState
}

func (rv *ReturnValue) Type() ObjectType { return RETURN_OBJ }
func (rv *ReturnValue) Inspect() string  { return rv.Value.Inspect() }

func (rv *ReturnValue) AsString() (string, Object) { return "", errMustBeString }
func (rv *ReturnValue) AsInt() (int64, Object)     { return 0, errMustBeInteger }
func (rv *ReturnValue) AsFloat() (float64, Object) { return 0, errMustBeNumber }
func (rv *ReturnValue) AsBool() (bool, Object)     { return false, errMustBeBoolean }
func (rv *ReturnValue) AsList() ([]Object, Object) { return nil, errMustBeList }
func (rv *ReturnValue) AsDict() (map[string]Object, Object) {
	return nil, errMustBeDict
}

func (rv *ReturnValue) CoerceString() (string, Object) { return rv.Inspect(), nil }
func (rv *ReturnValue) CoerceInt() (int64, Object)     { return 0, errMustBeInteger }
func (rv *ReturnValue) CoerceFloat() (float64, Object) { return 0, errMustBeNumber }

type Break struct{}

func (b *Break) Type() ObjectType { return BREAK_OBJ }
func (b *Break) Inspect() string  { return "break" }

func (b *Break) AsString() (string, Object)          { return "", errMustBeString }
func (b *Break) AsInt() (int64, Object)              { return 0, errMustBeInteger }
func (b *Break) AsFloat() (float64, Object)          { return 0, errMustBeNumber }
func (b *Break) AsBool() (bool, Object)              { return false, errMustBeBoolean }
func (b *Break) AsList() ([]Object, Object)          { return nil, errMustBeList }
func (b *Break) AsDict() (map[string]Object, Object) { return nil, errMustBeDict }

func (b *Break) CoerceString() (string, Object) { return b.Inspect(), nil }
func (b *Break) CoerceInt() (int64, Object)     { return 0, errMustBeInteger }
func (b *Break) CoerceFloat() (float64, Object) { return 0, errMustBeNumber }

type Continue struct{}

func (c *Continue) Type() ObjectType { return CONTINUE_OBJ }
func (c *Continue) Inspect() string  { return "continue" }

func (c *Continue) AsString() (string, Object)          { return "", errMustBeString }
func (c *Continue) AsInt() (int64, Object)              { return 0, errMustBeInteger }
func (c *Continue) AsFloat() (float64, Object)          { return 0, errMustBeNumber }
func (c *Continue) AsBool() (bool, Object)              { return false, errMustBeBoolean }
func (c *Continue) AsList() ([]Object, Object)          { return nil, errMustBeList }
func (c *Continue) AsDict() (map[string]Object, Object) { return nil, errMustBeDict }

func (c *Continue) CoerceString() (string, Object) { return c.Inspect(), nil }
func (c *Continue) CoerceInt() (int64, Object)     { return 0, errMustBeInteger }
func (c *Continue) CoerceFloat() (float64, Object) { return 0, errMustBeNumber }

// EvalFn is a compiled AST node: a closure that evaluates the node in env.
// Compiled closures are immutable after construction and safe to share
// between goroutines, exactly like the AST they are built from.
type EvalFn func(ctx context.Context, env *Environment) Object

type Function struct {
	Name             string
	Parameters       []*ast.Identifier
	DefaultValues    map[string]ast.Expression
	Variadic         *ast.Identifier // *args parameter
	Kwargs           *ast.Identifier // **kwargs parameter
	KeywordOnlyStart int             // 1-based index where keyword-only params start; 0 means none
	// PositionalOnly is the number of leading parameters that cannot be
	// passed by keyword (before a '/' marker); 0 means no marker.
	PositionalOnly   int
	Body             *ast.BlockStatement
	Env              *Environment
	LocalSlots       map[string]int
	LocalSlotNames   []string
	ParamSlotIndexes []int
	ReuseCallEnv     bool

	// CompiledDefaults holds the compiled form of each DefaultValues entry,
	// built once at definition time so calls that fill in a default do not
	// walk the AST.
	CompiledDefaults map[string]EvalFn
	// ResolvedDefaults holds the VALUE of each default, evaluated once at
	// definition time in the defining scope, as Python does. Index i aligns
	// with Parameters[i]; a nil slot means the parameter has no default. A
	// slice rather than a map: one allocation per definition, and the call
	// path already walks parameters by index.
	ResolvedDefaults []Object
	// CompiledBody memoises the body closure, which is compiled on the first
	// call and cached on Body, the AST node. The evaluator fills it in only
	// when CompilerOwned is set.
	CompiledBody EvalFn
	// CompilerOwned marks function objects the evaluator's compiler created.
	// Each belongs to one interpreter tree, whose execution is serialised, so
	// the evaluator may memoise into CompiledBody. Objects assembled elsewhere
	// may be shared between trees and are never written after construction.
	CompilerOwned bool
	// Abstract marks a method decorated with abc.abstractmethod.
	Abstract bool
	// GeneratorPlan is the evaluator's compiled resumable plan for a
	// generator function (body contains yield; opaque here, a
	// *evaluator.genPlan). Non-nil marks a generator: calling one
	// constructs a generator instead of running the body.
	GeneratorPlan any
}

// wrappedFunctions records, for a function decorated with functools.wraps,
// the callable it wraps. Few functions are wrappers, so the link lives here
// rather than in a field every function would carry. Entries are keyed weakly
// and removed when the wrapper is collected.
var wrappedFunctions sync.Map // weak.Pointer[Function] -> Object

// SetWrapped records the callable functools.wraps copied this function's
// identity from: __doc__ and __wrapped__ read through it.
func (f *Function) SetWrapped(wrapped Object) {
	key := weak.Make(f)
	if _, replaced := wrappedFunctions.Swap(key, wrapped); !replaced {
		runtime.AddCleanup(f, func(k weak.Pointer[Function]) { wrappedFunctions.Delete(k) }, key)
	}
}

// Wrapped returns the callable recorded by SetWrapped, or nil.
func (f *Function) Wrapped() Object {
	if v, ok := wrappedFunctions.Load(weak.Make(f)); ok {
		return v.(Object)
	}
	return nil
}

func (f *Function) Type() ObjectType { return FUNCTION_OBJ }
func (f *Function) Inspect() string  { return "<function>" }

func (f *Function) AsString() (string, Object)          { return "", errMustBeString }
func (f *Function) AsInt() (int64, Object)              { return 0, errMustBeInteger }
func (f *Function) AsFloat() (float64, Object)          { return 0, errMustBeNumber }
func (f *Function) AsBool() (bool, Object)              { return false, errMustBeBoolean }
func (f *Function) AsList() ([]Object, Object)          { return nil, errMustBeList }
func (f *Function) AsDict() (map[string]Object, Object) { return nil, errMustBeDict }

func (f *Function) CoerceString() (string, Object) { return f.Inspect(), nil }
func (f *Function) CoerceInt() (int64, Object)     { return 0, errMustBeInteger }
func (f *Function) CoerceFloat() (float64, Object) { return 0, errMustBeNumber }

type LambdaFunction struct {
	Parameters       []*ast.Identifier
	DefaultValues    map[string]ast.Expression
	Variadic         *ast.Identifier // *args parameter
	Kwargs           *ast.Identifier // **kwargs parameter
	KeywordOnlyStart int             // 1-based index where keyword-only params start; 0 means none
	// PositionalOnly is the number of leading parameters that cannot be
	// passed by keyword (before a '/' marker); 0 means no marker.
	PositionalOnly   int
	Body             ast.Expression
	Env              *Environment
	LocalSlots       map[string]int
	LocalSlotNames   []string
	ParamSlotIndexes []int
	// ReuseCallEnv allows a call's environment to return to the free-list
	// when the call ends. It must be false for a lambda that contains another
	// lambda, which can keep the environment alive after the call.
	ReuseCallEnv bool

	// CompiledBody is the compiled form of Body, set when the lambda is
	// compiled. The evaluator compiles and stores it on first call for
	// lambdas assembled elsewhere.
	CompiledBody EvalFn
	// CompiledDefaults holds the compiled form of each DefaultValues entry,
	// built once at definition time so calls that fill in a default do not
	// walk the AST.
	CompiledDefaults map[string]EvalFn
	// ResolvedDefaults holds the VALUE of each default, evaluated once at
	// definition time in the defining scope, as Python does. Index i aligns
	// with Parameters[i]; a nil slot means the parameter has no default.
	ResolvedDefaults []Object
}

func (lf *LambdaFunction) Type() ObjectType { return LAMBDA_OBJ }
func (lf *LambdaFunction) Inspect() string  { return "<lambda>" }

func (lf *LambdaFunction) AsString() (string, Object) { return "", errMustBeString }
func (lf *LambdaFunction) AsInt() (int64, Object)     { return 0, errMustBeInteger }
func (lf *LambdaFunction) AsFloat() (float64, Object) { return 0, errMustBeNumber }
func (lf *LambdaFunction) AsBool() (bool, Object)     { return false, errMustBeBoolean }
func (lf *LambdaFunction) AsList() ([]Object, Object) { return nil, errMustBeList }
func (lf *LambdaFunction) AsDict() (map[string]Object, Object) {
	return nil, errMustBeDict
}

func (lf *LambdaFunction) CoerceString() (string, Object) { return lf.Inspect(), nil }
func (lf *LambdaFunction) CoerceInt() (int64, Object)     { return 0, errMustBeInteger }
func (lf *LambdaFunction) CoerceFloat() (float64, Object) { return 0, errMustBeNumber }

// BuiltinFunction is the signature for all builtin functions
//   - ctx: Context with environment and runtime information
//   - kwargs: Keyword arguments passed to the function (wrapped with helper methods)
//   - args: Positional arguments passed to the function
//
// The args slice is BORROWED for the duration of the call: the evaluator reuses
// the backing array across calls (a per-root free-list, see AcquireArgs). A
// builtin must therefore not retain args — or any sub-slice such as args[1:] —
// past the point it returns. Reading elements (args[i], ranging over args,
// spreading args... into a synchronous call) is always safe; storing the slice
// in a struct field/map/global, capturing it in a goroutine or returned closure,
// or using it as a List's Elements requires copying first (make + copy). This
// matches the convention of CPython's tp_call and Go variadic APIs.
type BuiltinFunction func(ctx context.Context, kwargs Kwargs, args ...Object) Object

type Builtin struct {
	Fn         BuiltinFunction
	HelpText   string            // Optional help documentation for this builtin
	Attributes map[string]Object // Optional attributes for this builtin
	Repr       string            // Optional repr, e.g. for bound methods
}

func (b *Builtin) Type() ObjectType { return BUILTIN_OBJ }
func (b *Builtin) Inspect() string {
	if b.Repr != "" {
		return b.Repr
	}
	return "<builtin function>"
}

func (b *Builtin) AsString() (string, Object) { return "", errMustBeString }
func (b *Builtin) AsInt() (int64, Object)     { return 0, errMustBeInteger }
func (b *Builtin) AsFloat() (float64, Object) { return 0, errMustBeNumber }
func (b *Builtin) AsBool() (bool, Object)     { return false, errMustBeBoolean }
func (b *Builtin) AsList() ([]Object, Object) { return nil, errMustBeList }
func (b *Builtin) AsDict() (map[string]Object, Object) {
	if b.Attributes != nil {
		return b.Attributes, nil
	}
	return nil, errMustBeDict
}

func (b *Builtin) CoerceString() (string, Object) { return b.Inspect(), nil }
func (b *Builtin) CoerceInt() (int64, Object)     { return 0, errMustBeInteger }
func (b *Builtin) CoerceFloat() (float64, Object) { return 0, errMustBeNumber }

// Library represents a pre-built collection of builtin functions and constants
type Library struct {
	name         string
	functions    map[string]*Builtin
	constants    map[string]Object
	description  string
	instanceData any
}

// NewLibrary creates a new library with functions, optional constants, and optional description
func NewLibrary(name string, functions map[string]*Builtin, constants map[string]Object, description string) *Library {
	return &Library{
		name:        name,
		functions:   functions,
		constants:   constants,
		description: description,
	}
}

// Name returns the library's name
func (l *Library) Name() string {
	return l.name
}

// Functions returns the library's function map
func (l *Library) Functions() map[string]*Builtin {
	return l.functions
}

// Constants returns the library's constants map
func (l *Library) Constants() map[string]Object {
	return l.constants
}

// Description returns the library's description
func (l *Library) Description() string {
	return l.description
}

// GetDict builds and returns a fresh Dict representation of this library.
// Returns a new dict each time so callers (including concurrent goroutines)
// never share mutable state.
func (l *Library) GetDict() *Dict {
	// Members are stored in name order, so the module lists the same way
	// every time (a Go map has no order).
	names := make([]string, 0, len(l.functions)+len(l.constants)+1)
	for fname := range l.functions {
		names = append(names, fname)
	}
	for cname := range l.constants {
		if _, isFunc := l.functions[cname]; !isFunc {
			names = append(names, cname)
		}
	}
	if l.description != "" {
		if _, taken := l.functions["__doc__"]; !taken {
			if _, taken := l.constants["__doc__"]; !taken {
				names = append(names, "__doc__")
			}
		}
	}
	sort.Strings(names)

	dict := NewDictSized(len(names))
	dict.Module = l.name
	for _, name := range names {
		var value Object
		if name == "__doc__" && l.description != "" {
			value = &String{value: l.description}
		} else if c, ok := l.constants[name]; ok {
			value = c
		} else {
			value = l.functions[name]
		}
		dict.Store(DictKey(&String{value: name}), &String{value: name}, value)
	}
	return dict
}

// CachedDict returns nil (caching removed). Kept for test compatibility.
func (l *Library) CachedDict() *Dict {
	return nil
}

func (l *Library) Type() ObjectType { return BUILTIN_OBJ } // Libraries are like builtin objects
func (l *Library) Inspect() string  { return "<library>" }

func (l *Library) AsString() (string, Object)          { return "", errMustBeString }
func (l *Library) AsInt() (int64, Object)              { return 0, errMustBeInteger }
func (l *Library) AsFloat() (float64, Object)          { return 0, errMustBeNumber }
func (l *Library) AsBool() (bool, Object)              { return false, errMustBeBoolean }
func (l *Library) AsList() ([]Object, Object)          { return nil, errMustBeList }
func (l *Library) AsDict() (map[string]Object, Object) { return nil, errMustBeDict }

func (l *Library) CoerceString() (string, Object) { return l.Inspect(), nil }
func (l *Library) CoerceInt() (int64, Object)     { return 0, errMustBeInteger }
func (l *Library) CoerceFloat() (float64, Object) { return 0, errMustBeNumber }

// contextKey is used to store instance data in context
type contextKey string

const instanceDataKey contextKey = "library.instanceData"

// Instantiate creates a new library instance with instance-specific data
// Functions are wrapped to inject the instance data into the context
func (l *Library) Instantiate(instanceData any) *Library {
	newLib := &Library{
		name:         l.name,
		functions:    make(map[string]*Builtin, len(l.functions)),
		constants:    l.constants,
		description:  l.description,
		instanceData: instanceData,
	}

	// Wrap each function to inject instance data into context
	for name, builtin := range l.functions {
		originalFn := builtin.Fn
		newLib.functions[name] = &Builtin{
			Fn: func(ctx context.Context, kwargs Kwargs, args ...Object) Object {
				// Inject instance data into context
				ctx = context.WithValue(ctx, instanceDataKey, instanceData)
				return originalFn(ctx, kwargs, args...)
			},
			HelpText:   builtin.HelpText,
			Attributes: builtin.Attributes,
		}
	}

	return newLib
}

// InstanceData returns the instance-specific data for this library
func (l *Library) InstanceData() any {
	return l.instanceData
}

// InstanceDataFromContext retrieves instance data from the context
// Returns nil if no instance data is present
func InstanceDataFromContext(ctx context.Context) any {
	return ctx.Value(instanceDataKey)
}

// rootState is the part of an environment tree that exists once per tree
// rather than once per scope. Keeping it out of Environment halves the size
// of every scope and leaves a call frame one pointer to set up for it.
type rootState struct {
	output                     io.Writer
	errOutput                  io.Writer
	input                      io.Reader
	importCallback             func(context.Context, string) error
	availableLibrariesCallback func() []LibraryInfo
	currentModule              string // Current module path for relative import resolution
	// freeFrames is a free-list of reusable call-frame environments, populated
	// lazily. It is accessed without locking; the interpreter lock (gil)
	// guarantees only one goroutine runs script code in a tree at a time, so
	// the free-list never sees concurrent access.
	freeFrames *callFrameFreeList
	// freeReturns is a free-list of reusable *ReturnValue wrappers, under the
	// same contract as freeFrames. It avoids the per-call pin()/Get() overhead
	// of a global sync.Pool on the return path.
	freeReturns []*ReturnValue
	// freeArgBufs is a free-list of reusable []Object argument buffers used by
	// the evaluator's call-arg path, under the same contract as freeFrames.
	freeArgBufs [][]Object
	// gil is the tree's interpreter lock. The evaluator acquires it at every
	// Go->script boundary so that any number of goroutines may call into one
	// Environment tree, but only one runs script at a time.
	gil *gilLock
}

type Environment struct {
	store            map[string]Object
	slotIndex        map[string]int
	slotNames        []string
	slots            []Object
	callPoolSlots    uint8 // slot count + 1 when this is a pooled call frame, else 0
	ownSlots         bool  // slotIndex and slotNames are this environment's own copy
	outer            *Environment
	root             *Environment
	globals          map[string]bool
	nonlocals        map[string]bool
	importedBindings map[string]bool
	// rs is the state shared by every environment of a tree: the interpreter
	// lock, the free-lists and the tree's I/O and import settings. Every
	// environment points at its tree's one rootState.
	rs *rootState
	// ctxMemo caches this environment attached to a context; see
	// CachedEnvContext. Held as an atomic pointer to an immutable value so it is
	// safe to touch from any goroutine without relying on the interpreter lock —
	// background tasks pin their environment on a context off the hot path.
	ctxMemo atomic.Pointer[envContextMemo]
}

// envContextMemo records that attaching an environment to base produced derived.
// Immutable once published.
type envContextMemo struct {
	base    context.Context
	derived context.Context
}

// CachedEnvContext returns the context previously derived from base for this
// environment, if any.
//
// Builtins are invoked with their environment attached to the context, and
// deriving that context is a heap allocation. Since the base context is stable
// for the whole of one evaluation, memoising the derived context turns a
// per-builtin-call allocation into an atomic load and a comparison.
func (e *Environment) CachedEnvContext(base context.Context) (context.Context, bool) {
	if m := e.ctxMemo.Load(); m != nil && m.base == base {
		return m.derived, true
	}
	return nil, false
}

// StoreEnvContext memoises derived as the result of attaching this environment
// to base, so a later CachedEnvContext(base) can skip the allocation.
//
// Nothing is stored when base's dynamic type is not comparable, because the
// lookup compares contexts with ==. That guard is what makes the comparison
// safe: a stored base is always of a comparable type, so if an incoming context
// has a different (possibly non-comparable) type the comparison fails on the
// type word alone and never reaches the values.
func (e *Environment) StoreEnvContext(base, derived context.Context) {
	if base == nil || !reflect.TypeOf(base).Comparable() {
		return
	}
	e.ctxMemo.Store(&envContextMemo{base: base, derived: derived})
}

// gilLock is the interpreter lock for one environment tree. It tracks the
// owning goroutine so that re-entrant acquisitions (a builtin calling back into
// script on the same goroutine) are detected, and so that RunUnlocked only
// releases the lock for the goroutine that actually holds it.
type gilLock struct {
	mu    sync.Mutex
	owner atomic.Int64 // goroutine id of the current holder, 0 when free

	// guard is the interpreter's resource guard, nil when it has none. While
	// a goroutine holds the lock it is bound to guard (see bindGuard), and
	// boundPrev keeps the binding to restore on release. Only the holder
	// touches boundPrev.
	guard     *ResourceGuard
	boundPrev *ResourceGuard
	bound     bool // the holder's goroutine was bound by bind

	// Context-aware waiters. Unlock broadcasts by closing notify, so a
	// cancellable waiter wakes the moment the lock frees rather than on a
	// polling interval. waiters is only incremented along the contended
	// path, so the uncontended hot path pays one atomic load on unlock and
	// nothing extra on lock.
	waiters  atomic.Int64
	notifyMu sync.Mutex
	notify   chan struct{} // lazily created; closed to broadcast, then replaced

	// resources are what scripts in this lock domain opened and have not
	// closed (see TrackResource). A script library's domain shares its
	// interpreter's (see ShareResources).
	resources *resourceSet
}

// broadcastUnlock wakes every registered context-aware waiter. Callers must
// not hold mu (it runs after Unlock), so the lock orders notifyMu under mu
// trivially.
func (g *gilLock) broadcastUnlock() {
	if g.waiters.Load() == 0 {
		return
	}
	g.notifyMu.Lock()
	if g.notify != nil {
		close(g.notify)
		g.notify = nil
	}
	g.notifyMu.Unlock()
}

// waitChan returns the current generation's broadcast channel, creating it if
// the previous generation was already closed.
func (g *gilLock) waitChan() chan struct{} {
	g.notifyMu.Lock()
	defer g.notifyMu.Unlock()
	if g.notify == nil {
		g.notify = make(chan struct{})
	}
	return g.notify
}

const maxPooledCallEnvSlots = 16

// maxFreeFramesPerBucket caps how many idle frames a tree retains per slot-count
// bucket, bounding memory after deep recursion while still amortising the common
// depth across repeated calls.
const maxFreeFramesPerBucket = 256

// callFrameFreeList holds reusable call-frame environments bucketed by slot
// count. It lives on the root Environment of an evaluation tree and is accessed
// without synchronization.
//
// SAFETY: sound because the per-environment interpreter lock (GIL) serializes
// script execution on a tree — only one goroutine runs script code at a time,
// so the free-list never sees concurrent access. Multiple goroutines may share
// the environment (shared-env threads via runtime.background(shared=True),
// concurrent callers), and that is fine: they serialize on the GIL, never on
// the free-list concurrently. Independent environment trees have independent
// GILs and independent free-lists.
type callFrameFreeList struct {
	buckets [maxPooledCallEnvSlots + 1][]*Environment
}

// LibraryInfo contains information about available libraries
type LibraryInfo struct {
	Name       string
	IsImported bool
}

func NewEnvironment() *Environment {
	env := &Environment{
		store: make(map[string]Object, 4),
		// globals and nonlocals are nil by default - allocated on demand
	}
	env.root = env
	env.rs = &rootState{gil: &gilLock{resources: &resourceSet{}}}
	return env
}

// NewEnvironmentWithGILDomain creates an otherwise independent environment
// that serializes through the same interpreter lock as owner. It is useful for
// runtime callbacks that must retain a stable lock domain without retaining
// owner's store (which can create finalizer reachability cycles).
func NewEnvironmentWithGILDomain(owner *Environment) *Environment {
	env := NewEnvironment()
	if owner != nil && owner.rs != nil && owner.rs.gil != nil {
		env.rs.gil = owner.rs.gil
	}
	return env
}

func NewEnclosedEnvironment(outer *Environment) *Environment {
	env := &Environment{
		outer: outer,
	}
	if outer != nil {
		env.root = outer.root
		env.rs = outer.rs
	} else {
		env.root = env
		env.rs = &rootState{gil: &gilLock{resources: &resourceSet{}}}
	}
	return env
}

func NewEnclosedEnvironmentWithSlots(outer *Environment, slotIndex map[string]int, slotNames []string) *Environment {
	env := NewEnclosedEnvironment(outer)
	if len(slotIndex) > 0 {
		env.slotIndex = slotIndex
		env.slotNames = slotNames
		env.ownSlots = false
		env.slots = make([]Object, len(slotNames))
	}
	return env
}

// Root returns the root environment of this tree.
func (e *Environment) Root() *Environment { return e.root }

// envContextValueKey is the context key under which the evaluator stores the
// current environment for builtins. Kept in sync with the evaluator package so
// blocking builtins can release the interpreter lock via RunBlocking.
const envContextValueKey = "scriptling-env"

// RunBlocking runs fn (a blocking operation such as sleep or a socket read) with
// the interpreter lock released, so other goroutines can run script code in the
// same tree while this one is blocked. If no environment is available on the
// context it simply runs fn. Builtins that block should wrap the blocking call
// in RunBlocking.
func RunBlocking(ctx interface{ Value(any) any }, fn func()) {
	if env, ok := ctx.Value(envContextValueKey).(*Environment); ok && env != nil {
		env.RunUnlocked(fn)
		return
	}
	fn()
}

// EnterGIL acquires this tree's interpreter lock for the calling goroutine,
// unless this goroutine already holds it (a re-entrant call). It returns true if
// it actually acquired the lock (the caller must pair it with ExitGIL) and false
// if the call was re-entrant (caller does nothing on exit).
func (e *Environment) EnterGIL() bool {
	if e.rs == nil || e.rs.gil == nil {
		return false
	}
	g := e.rs.gil
	id := goid()
	if g.owner.Load() == id {
		return false // re-entrant: already held by this goroutine
	}
	g.mu.Lock()
	g.owner.Store(id)
	g.bind(id)
	return true
}

// bind attaches the holder's goroutine to the interpreter's guard. The caller
// must hold the lock.
func (g *gilLock) bind(id int64) {
	if g.guard != nil && g.guard.needsBinding() {
		g.boundPrev = bindGuard(id, g.guard)
		g.bound = true
	}
}

// unbind restores the binding bind replaced. The caller must hold the lock.
func (g *gilLock) unbind(id int64) {
	if g.bound {
		unbindGuard(id, g.boundPrev)
		g.boundPrev = nil
		g.bound = false
	}
}

// EnterGILWithContext acquires the interpreter lock while allowing a
// cancellable caller to abandon a contended wait. acquired reports whether the
// caller must invoke ExitGIL; entered is false only when ctx ended before entry.
// Contexts without cancellation use the regular blocking fast path.
func (e *Environment) EnterGILWithContext(ctx context.Context) (acquired, entered bool) {
	if ctx == nil || ctx.Done() == nil {
		return e.EnterGIL(), true
	}
	if ctx.Err() != nil {
		return false, false
	}
	if e.rs == nil || e.rs.gil == nil {
		return false, true
	}
	g := e.rs.gil
	id := goid()
	if g.owner.Load() == id {
		return false, true
	}
	if g.mu.TryLock() {
		g.owner.Store(id)
		g.bind(id)
		return true, true
	}

	// Contended: register for the next unlock broadcast. sync.Mutex has no
	// context-aware blocking acquire, so park on the generation channel and
	// re-try on every wake — unlock broadcasts immediately, so the wait ends
	// the moment the lock frees or the context does, never a tick later.
	//
	// Cancel-vs-unlock is deliberately best-effort: when ctx.Done() and the
	// broadcast become ready together, select may honor either. A caller can
	// therefore acquire despite concurrent cancellation (it must still
	// release), or abandon a lock that was just becoming free. Neither
	// outcome leaks the lock; preferring cancellation strictly would mean
	// re-checking ctx after a won TryLock and handing the lock back, which
	// buys nothing for the callers of this path.
	g.waiters.Add(1)
	defer g.waiters.Add(-1)
	for {
		notify := g.waitChan()
		// The unlock may have happened while registering: re-try before
		// parking so the wakeup cannot be lost.
		if g.mu.TryLock() {
			g.owner.Store(id)
			g.bind(id)
			return true, true
		}
		select {
		case <-ctx.Done():
			return false, false
		case <-notify:
			if g.mu.TryLock() {
				g.owner.Store(id)
				g.bind(id)
				return true, true
			}
			// Another waiter won this generation; re-register on the next.
		}
	}
}

// ExitGIL releases the interpreter lock previously acquired by EnterGIL.
func (e *Environment) ExitGIL() {
	if e.rs == nil || e.rs.gil == nil {
		return
	}
	g := e.rs.gil
	g.unbind(g.owner.Load())
	g.owner.Store(0)
	g.mu.Unlock()
	g.broadcastUnlock()
}

// RunUnlocked releases the interpreter lock, runs fn (typically a blocking call
// such as sleep or a socket read), then re-acquires it — but only if the calling
// goroutine actually holds the lock. Goroutines that do not hold it (e.g. raw
// worker goroutines doing pure-Go I/O) simply run fn. This lets other goroutines
// run script code in the same tree while the holder is blocked.
func (e *Environment) RunUnlocked(fn func()) {
	if e.rs == nil || e.rs.gil == nil {
		fn()
		return
	}
	g := e.rs.gil
	id := goid()
	if g.owner.Load() != id {
		fn() // not the holder — must not touch the lock
		return
	}
	// The goroutine stays bound to the guard while it blocks: what it reads
	// is for this interpreter. Another goroutine may take the lock meanwhile
	// and overwrite boundPrev, so keep this holder's copy.
	prev, bound := g.boundPrev, g.bound
	g.owner.Store(0)
	g.mu.Unlock()
	g.broadcastUnlock()
	defer func() {
		g.mu.Lock()
		g.owner.Store(id)
		g.boundPrev, g.bound = prev, bound
	}()
	fn()
}

// SetResourceGuard gives the interpreter that owns e's lock a resource guard.
// Set it before the interpreter runs; environments sharing the lock share the
// guard.
func (e *Environment) SetResourceGuard(guard *ResourceGuard) {
	if e.rs != nil && e.rs.gil != nil {
		e.rs.gil.guard = guard
	}
}

// ResourceGuard returns the guard of the interpreter that owns e's lock, nil
// when it has none.
func (e *Environment) ResourceGuard() *ResourceGuard {
	if e == nil || e.rs == nil || e.rs.gil == nil {
		return nil
	}
	return e.rs.gil.guard
}

// AcquireCallEnvironment returns a function-call environment, reusing an idle
// frame from the tree's per-root free-list when one is available.
//
// The free-list lives on the root environment and is accessed without locking;
// this is safe because an environment tree is only ever used by a single
// goroutine (see callFrameFreeList). Frames acquired here must be returned via
// ReleaseCallEnvironment.
func AcquireCallEnvironment(outer *Environment, slotIndex map[string]int, slotNames []string) *Environment {
	slotCount := len(slotNames)
	if slotCount <= maxPooledCallEnvSlots && outer != nil {
		if rs := outer.rs; rs != nil {
			if fl := rs.freeFrames; fl != nil {
				if bucket := fl.buckets[slotCount]; len(bucket) > 0 {
					n := len(bucket) - 1
					env := bucket[n]
					bucket[n] = nil
					fl.buckets[slotCount] = bucket[:n]
					env.slotIndex = slotIndex
					env.slotNames = slotNames
					env.ownSlots = false
					env.callPoolSlots = uint8(slotCount) + 1
					env.outer = outer
					env.root = outer.root
					env.rs = rs
					return env
				}
			}
		}
		env := NewEnclosedEnvironmentWithSlots(outer, slotIndex, slotNames)
		env.callPoolSlots = uint8(slotCount) + 1
		return env
	}
	return NewEnclosedEnvironmentWithSlots(outer, slotIndex, slotNames)
}

// ReleaseCallEnvironment clears a call frame and returns it to its tree's
// per-root free-list for reuse. Non-pooled environments are ignored. See
// AcquireCallEnvironment for the no-lock concurrency contract.
func ReleaseCallEnvironment(env *Environment) {
	if env == nil || env.callPoolSlots == 0 {
		return
	}
	rs := env.rs
	slotCount := env.callPoolSlots - 1
	for i := range env.slots {
		env.slots[i] = nil
	}
	if env.store != nil {
		clear(env.store)
	}
	if env.globals != nil {
		clear(env.globals)
	}
	if env.nonlocals != nil {
		clear(env.nonlocals)
	}
	if env.importedBindings != nil {
		clear(env.importedBindings)
	}
	env.callPoolSlots = 0
	env.outer = nil
	env.root = nil
	env.slotIndex = nil
	env.slotNames = nil
	env.ownSlots = false
	env.rs = nil

	if rs == nil {
		return
	}
	fl := rs.freeFrames
	if fl == nil {
		fl = &callFrameFreeList{}
		rs.freeFrames = fl
	}
	if len(fl.buckets[slotCount]) < maxFreeFramesPerBucket {
		fl.buckets[slotCount] = append(fl.buckets[slotCount], env)
	}
}

// maxFreeReturns caps how many idle *ReturnValue wrappers a tree retains,
// bounding memory after deep recursion.
const maxFreeReturns = 256

// AcquireReturnValue returns a *ReturnValue wrapping val, reusing one from the
// tree's per-root free-list when available. The wrapper remembers its root so
// ReleaseReturnValue can return it without the caller passing the environment.
// Uses the same lock-free, GIL-serialized contract as AcquireCallEnvironment.
func AcquireReturnValue(env *Environment, val Object) *ReturnValue {
	if env != nil {
		if root := env.rs; root != nil {
			if n := len(root.freeReturns); n > 0 {
				rv := root.freeReturns[n-1]
				root.freeReturns[n-1] = nil
				root.freeReturns = root.freeReturns[:n-1]
				rv.Value = val
				rv.root = root
				return rv
			}
			return &ReturnValue{Value: val, root: root}
		}
	}
	return &ReturnValue{Value: val}
}

// ReleaseReturnValue clears rv and returns it to its tree's free-list for reuse.
// Wrappers with no associated root (constructed outside AcquireReturnValue) are
// left for the garbage collector.
func ReleaseReturnValue(rv *ReturnValue) {
	if rv == nil {
		return
	}
	root := rv.root
	rv.Value = nil
	rv.root = nil
	if root == nil {
		return
	}
	if len(root.freeReturns) < maxFreeReturns {
		root.freeReturns = append(root.freeReturns, rv)
	}
}

// maxFreeArgBufs caps how many idle argument buffers a tree retains, bounding
// memory after deep call nesting.
const maxFreeArgBufs = 256

// AcquireArgs returns a []Object of length n for a call's argument list,
// reusing a buffer from the tree's per-root free-list when one of sufficient
// capacity is available. The returned slice is BORROWED: the caller must return
// it via ReleaseArgs once the callee has returned, and no callee may retain the
// slice past its own call (the interpreter's builtins, user-function binding,
// and varargs collection all satisfy this — varargs copies, params read by
// index). Uses the same lock-free, GIL-serialized contract as AcquireCallEnvironment.
func AcquireArgs(env *Environment, n int) []Object {
	if n <= 0 {
		return nil
	}
	if env != nil {
		if root := env.rs; root != nil {
			if s := root.freeArgBufs; len(s) > 0 {
				i := len(s) - 1
				if cap(s[i]) >= n {
					buf := s[i]
					s[i] = nil
					root.freeArgBufs = s[:i]
					return buf[:n]
				}
				// Top buffer is too small; fall through to allocate a fresh one,
				// leaving the small buffer pooled for a smaller call.
			}
		}
	}
	return make([]Object, n)
}

// ReleaseArgs clears args and returns its backing array to the tree's per-root
// free-list for reuse. Safe to call with nil/empty args (no-op) and on slices
// not obtained from AcquireArgs (they simply join the pool). The full backing
// capacity is cleared so no Object is pinned by an idle buffer.
func ReleaseArgs(env *Environment, args []Object) {
	if env == nil || len(args) == 0 {
		return
	}
	root := env.rs
	if root == nil {
		return
	}
	full := args[:cap(args)]
	for i := range full {
		full[i] = nil
	}
	if len(root.freeArgBufs) < maxFreeArgBufs {
		root.freeArgBufs = append(root.freeArgBufs, full)
	}
}

func (e *Environment) Get(name string) (Object, bool) {
	for env := e; env != nil; env = env.outer {
		if idx, ok := env.slotIndex[name]; ok {
			if idx >= 0 && idx < len(env.slots) && env.slots[idx] != nil {
				return env.slots[idx], true
			}
		}
		obj, ok := env.store[name]
		if ok {
			return obj, true
		}
	}
	return nil, false
}

// GetWithLocation resolves name like Get, additionally reporting where it was
// found in the environment chain: hops is the number of outer links traversed
// and slotIdx is the slot index if the value lives in a slot, or -1 if it came
// from a store map (not a stable, cacheable location). Used to seed call-site
// callee caches.
func (e *Environment) GetWithLocation(name string) (val Object, hops int, slotIdx int, ok bool) {
	hops = 0
	for env := e; env != nil; env = env.outer {
		if idx, found := env.slotIndex[name]; found {
			if idx >= 0 && idx < len(env.slots) && env.slots[idx] != nil {
				return env.slots[idx], hops, idx, true
			}
		}
		if obj, found := env.store[name]; found {
			return obj, hops, -1, true
		}
		hops++
	}
	return nil, 0, -1, false
}

// GetAtLocation reads the value at a cached (hops, slotIdx) location, validating
// that the slot still holds the expected name. Returns false if the cache is
// stale (chain too short or slot layout changed), so the caller can fall back to
// a full lookup.
func (e *Environment) GetAtLocation(hops, slotIdx int, name string) (Object, bool) {
	env := e
	for i := 0; i < hops; i++ {
		if env == nil {
			return nil, false
		}
		env = env.outer
	}
	if env == nil {
		return nil, false
	}
	if slotIdx >= 0 && slotIdx < len(env.slots) && slotIdx < len(env.slotNames) && env.slotNames[slotIdx] == name {
		if v := env.slots[slotIdx]; v != nil {
			return v, true
		}
	}
	return nil, false
}

// GetSlotByIndex returns the value at the given slot index.
// Returns (value, true) if the slot has a value, (nil, false) otherwise.
func (e *Environment) GetSlotByIndex(idx int) (Object, bool) {
	if idx >= 0 && idx < len(e.slots) && e.slots[idx] != nil {
		return e.slots[idx], true
	}
	return nil, false
}

// GetSlotIndex returns the slot index for the given variable name in this
// environment's local scope only. Returns (index, true) if found, (0, false)
// if not a local slot.
func (e *Environment) GetSlotIndex(name string) (int, bool) {
	if e.slotIndex == nil {
		return 0, false
	}
	idx, ok := e.slotIndex[name]
	if ok && idx >= 0 && idx < len(e.slots) {
		return idx, true
	}
	return 0, false
}

// HasSlots returns whether this environment has slot-based variable access configured.
func (e *Environment) HasSlots() bool {
	return e.slotIndex != nil
}

// SetupSlots configures slot-based variable access on this environment. The
// layout belongs to a parsed program, which the parse cache shares between
// interpreters, so it is used as is until ExtendSlots needs to grow it.
func (e *Environment) SetupSlots(slotIndex map[string]int, slotNames []string) {
	e.slotIndex = slotIndex
	e.slotNames = slotNames
	e.ownSlots = false
	e.slots = make([]Object, len(slotNames))
}

// ExtendSlots adds new variables to the existing slot layout. Variables
// already present keep their existing indices. New variables are appended.
// The first extension copies the layout: it may be a program's or a
// function's, shared with every other environment it was set up on, and
// growing it in place let one interpreter's variables move another's slots,
// so a name could read another variable's value.
func (e *Environment) ExtendSlots(slotIndex map[string]int, slotNames []string) {
	for _, name := range slotNames {
		if _, exists := e.slotIndex[name]; !exists {
			if !e.ownSlots {
				e.slotIndex = maps.Clone(e.slotIndex)
				e.slotNames = slices.Clip(slices.Clone(e.slotNames))
				e.ownSlots = true
			}
			idx := len(e.slotNames)
			e.slotIndex[name] = idx
			e.slotNames = append(e.slotNames, name)
			e.slots = append(e.slots, nil)
		}
	}
}

// SetSlotByIndex stores val in the given local slot index when valid.
func (e *Environment) SetSlotByIndex(idx int, val Object) bool {
	if idx >= 0 && idx < len(e.slots) {
		e.slots[idx] = val
		return true
	}
	return false
}

// GetCachedSlot returns the value at the given slot index after validating
// that the slot name matches. This prevents stale cached indices (from
// shared AST via the parse cache) from reading the wrong variable.
func (e *Environment) GetCachedSlot(idx int, name string) (Object, bool) {
	if idx >= 0 && idx < len(e.slots) && idx < len(e.slotNames) && sameName(e.slotNames[idx], name) {
		if e.slots[idx] != nil {
			return e.slots[idx], true
		}
	}
	return nil, false
}

// SetCachedSlot stores val at the given slot index after validating
// that the slot name matches. Returns false if the cache is stale,
// falling through to the full Set path.
func (e *Environment) SetCachedSlot(idx int, name string, val Object) bool {
	if idx >= 0 && idx < len(e.slots) && idx < len(e.slotNames) && sameName(e.slotNames[idx], name) {
		e.slots[idx] = val
		e.clearImportedBinding(name)
		return true
	}
	return false
}

// sameName reports whether two slot names are equal. Slot names and the
// identifier names the evaluator validates them against almost always share
// the same backing bytes, because both come from the parser's interned symbol
// table, so the pointer comparison settles the common case without the call
// into the runtime's string comparison; the full comparison remains as the
// fallback so semantics are unchanged.
func sameName(a, b string) bool {
	return len(a) == len(b) && (unsafe.StringData(a) == unsafe.StringData(b) || a == b)
}

// clearImportedBinding drops any import marker for name. The nil check matters:
// importedBindings is only allocated for scopes that actually bind an import,
// whereas this runs on every assignment, and delete() on a nil map is still a
// function call into the runtime.
func (e *Environment) clearImportedBinding(name string) {
	if e.importedBindings != nil {
		delete(e.importedBindings, name)
	}
}

func (e *Environment) Set(name string, val Object) Object {
	// Check if this variable is marked as global
	if e.globals != nil && e.globals[name] {
		return e.SetGlobal(name, val)
	}
	// Check if this variable is marked as nonlocal
	if e.nonlocals != nil && e.nonlocals[name] {
		if e.SetInParent(name, val) {
			return val
		}
	}
	if idx, ok := e.slotIndex[name]; ok && idx >= 0 && idx < len(e.slots) {
		e.slots[idx] = val
		e.clearImportedBinding(name)
		return val
	}
	if e.store == nil {
		e.store = make(map[string]Object, 4)
	}
	e.store[name] = val
	e.clearImportedBinding(name)
	return val
}

// EachLocal calls fn for every name bound directly in this environment (its own
// store and slots), not in any parent scope. Used to collect class-body
// assignments into class members after executing the body. Iteration order is
// unspecified.
func (e *Environment) EachLocal(fn func(name string, val Object)) {
	for name, val := range e.store {
		if val != nil {
			fn(name, val)
		}
	}
	for name, idx := range e.slotIndex {
		if idx >= 0 && idx < len(e.slots) {
			if val := e.slots[idx]; val != nil {
				fn(name, val)
			}
		}
	}
}

// Delete removes a variable from this environment (not parent scopes)
func (e *Environment) Delete(name string) {
	if idx, ok := e.slotIndex[name]; ok && idx >= 0 && idx < len(e.slots) {
		e.slots[idx] = nil
	}
	delete(e.store, name)
	e.clearImportedBinding(name)
}

// SetGlobal sets a variable in the global (outermost) environment
func (e *Environment) SetGlobal(name string, val Object) Object {
	root := e.root
	// Check if the root environment has a slot for this variable
	if root.slotIndex != nil {
		if idx, ok := root.slotIndex[name]; ok && idx >= 0 && idx < len(root.slots) {
			root.slots[idx] = val
			root.clearImportedBinding(name)
			return val
		}
	}
	if root.store == nil {
		root.store = make(map[string]Object, 4)
	}
	root.store[name] = val
	root.clearImportedBinding(name)
	return val
}

// GetGlobal gets the global (outermost) environment
func (e *Environment) GetGlobal() *Environment {
	return e.root
}

// SetInParent sets a variable in the parent environment (for nonlocal)
func (e *Environment) SetInParent(name string, val Object) bool {
	for env := e.outer; env != nil; env = env.outer {
		if idx, ok := env.slotIndex[name]; ok && idx >= 0 && idx < len(env.slots) {
			env.slots[idx] = val
			return true
		}
		_, ok := env.store[name]
		if ok {
			env.store[name] = val
			env.clearImportedBinding(name)
			return true
		}
	}
	return false
}

// MarkGlobal marks a variable name as global in this scope
func (e *Environment) MarkGlobal(name string) {
	if e.globals == nil {
		e.globals = make(map[string]bool, 2)
	}
	e.globals[name] = true
}

// MarkNonlocal marks a variable name as nonlocal in this scope
func (e *Environment) MarkNonlocal(name string) {
	if e.nonlocals == nil {
		e.nonlocals = make(map[string]bool, 2)
	}
	e.nonlocals[name] = true
}

// comprehensionScopeKey marks, in nonlocals, an environment that is a
// comprehension's scope (no name can be empty).
const comprehensionScopeKey = ""

// MarkWalrusTargets marks e, a comprehension's scope, as one whose
// assignment expressions (name := v) to names bind in the enclosing scope,
// as Python's do.
func (e *Environment) MarkWalrusTargets(names []string) {
	e.MarkNonlocal(comprehensionScopeKey)
	for _, n := range names {
		e.MarkNonlocal(n)
	}
}

// WalrusScope returns the environment an assignment expression binding name
// in e assigns to: e itself, or, from a comprehension's scope marked by
// MarkWalrusTargets, the first enclosing scope that is not one.
func (e *Environment) WalrusScope(name string) *Environment {
	t := e
	for t.nonlocals != nil && t.nonlocals[comprehensionScopeKey] && t.nonlocals[name] && t.outer != nil {
		t = t.outer
	}
	return t
}

// MarkImportedBinding marks a local binding as coming from an import.
func (e *Environment) MarkImportedBinding(name string) {
	if e.importedBindings == nil {
		e.importedBindings = make(map[string]bool, 2)
	}
	e.importedBindings[name] = true
}

// IsImportedBinding reports whether a local binding came from an import.
func (e *Environment) IsImportedBinding(name string) bool {
	return e.importedBindings != nil && e.importedBindings[name]
}

// IsGlobal checks if a variable is marked as global
func (e *Environment) IsGlobal(name string) bool {
	return e.globals != nil && e.globals[name]
}

// IsNonlocal checks if a variable is marked as nonlocal
func (e *Environment) IsNonlocal(name string) bool {
	return e.nonlocals != nil && e.nonlocals[name]
}

// shared returns the tree's root state, creating one for an Environment that
// was not made by a constructor.
func (e *Environment) shared() *rootState {
	if e.rs == nil {
		e.rs = &rootState{}
	}
	return e.rs
}

// EnableOutputCapture enables output capture for this environment. Captured
// output is one value handed back to the host, so it is held to the
// interpreter's value limit: a write that would take it over fails with
// ErrOutputLimitExceeded.
func (e *Environment) EnableOutputCapture() {
	e.shared().output = &captureBuffer{env: e}
}

// ErrOutputLimitExceeded is returned by a write to captured output that would
// take it over the interpreter's value limit.
var ErrOutputLimitExceeded = stderrors.New("captured output exceeds the value limit")

// ErrOutputNoRoom is returned by a write to captured output that would not
// fit in the interpreter's memory limit. It matches ErrOutputLimitExceeded.
var ErrOutputNoRoom = fmt.Errorf("%w: no room within the memory limit", ErrOutputLimitExceeded)

// captureBuffer collects captured output up to the interpreter's value limit.
type captureBuffer struct {
	strings.Builder
	env *Environment
}

func (b *captureBuffer) Write(p []byte) (int, error) {
	if err := b.reserve(len(p)); err != nil {
		return 0, err
	}
	oldCap := b.Cap()
	n, err := b.Builder.Write(p)
	b.track(oldCap)
	return n, err
}

func (b *captureBuffer) WriteString(s string) (int, error) {
	if err := b.reserve(len(s)); err != nil {
		return 0, err
	}
	oldCap := b.Cap()
	n, err := b.Builder.WriteString(s)
	b.track(oldCap)
	return n, err
}

// reserve checks that n more bytes keep the output within the interpreter's
// value limit and, once the buffer may grow, within its memory limit.
func (b *captureBuffer) reserve(n int) error {
	g := b.env.ResourceGuard()
	size := int64(b.Len() + n)
	if limit := g.MaxValueBytes(); limit > 0 && size > limit {
		return ErrOutputLimitExceeded
	}
	if size > int64(b.Cap()) && accountingOn.Load() && size >= sampleInterval.Load() && checkRoom(g, "captured output", GrowthRoom(b.Cap(), int(size))) != nil {
		return ErrOutputNoRoom
	}
	return nil
}

// track charges a backing array the last write allocated; its charge is
// credited back once a later growth or GetOutput lets it go.
func (b *captureBuffer) track(oldCap int) {
	if b.Cap() != oldCap && b.Len() > 0 && accountingOn.Load() {
		trackAlloc(unsafe.StringData(b.String()), int64(b.Cap()))
	}
}

// SetOutputWriter sets a custom writer for output
func (e *Environment) SetOutputWriter(w io.Writer) {
	e.shared().output = w
}

// SetErrorWriter sets a custom writer for error/diagnostic output (sys.stderr)
func (e *Environment) SetErrorWriter(w io.Writer) {
	e.shared().errOutput = w
}

// GetErrorWriter returns the appropriate writer for error output (sys.stderr)
func (e *Environment) GetErrorWriter() io.Writer {
	if e.shared().errOutput != nil {
		return e.shared().errOutput
	}
	return os.Stderr
}

// GetOutput returns captured output and clears the buffer
func (e *Environment) GetOutput() string {
	if e.shared().output == nil {
		return ""
	}
	switch builder := e.shared().output.(type) {
	case *captureBuffer:
		result := builder.String()
		builder.Reset()
		return result
	case *strings.Builder:
		result := builder.String()
		builder.Reset()
		return result
	}
	return ""
}

// GetWriter returns the appropriate writer for output
func (e *Environment) GetWriter() io.Writer {
	if e.shared().output != nil {
		return e.shared().output
	}
	return os.Stdout
}

// SetInputReader sets a custom reader for input
func (e *Environment) SetInputReader(r io.Reader) {
	e.shared().input = r
}

// GetReader returns the appropriate reader for input
func (e *Environment) GetReader() io.Reader {
	if e.shared().input != nil {
		return e.shared().input
	}
	return os.Stdin
}

// sizeHint returns a non-negative capacity hint for two combined lengths.
// len+len can overflow int when both operands are huge (e.g. an Environment
// store filled from untrusted input), and a wrapped negative hint panics
// make; on overflow the hint degrades to 0 and the container grows naturally.
func sizeHint(a, b int) int {
	if hint := a + b; hint >= a && hint >= b {
		return hint
	}
	return 0
}

// GetStore returns a copy of the environment's store (only local scope, not outer)
// Bindings returns the names bound in this environment and their values.
// Slot variables come first, in slot order (the order the names first appear
// in the source), followed by any others in name order.
func (e *Environment) Bindings() (names []string, values []Object) {
	names = make([]string, 0, len(e.store)+len(e.slotNames))
	values = make([]Object, 0, cap(names))
	for idx, name := range e.slotNames {
		if idx < len(e.slots) && e.slots[idx] != nil {
			names = append(names, name)
			values = append(values, e.slots[idx])
		}
	}
	slotted := len(names)
	for name := range e.store {
		if idx, ok := e.slotIndex[name]; ok && idx >= 0 && idx < len(e.slots) && e.slots[idx] != nil {
			continue // the slot holds this name's value
		}
		names = append(names, name)
	}
	sort.Strings(names[slotted:])
	for _, name := range names[slotted:] {
		values = append(values, e.store[name])
	}
	return names, values
}

func (e *Environment) GetStore() map[string]Object {
	store := make(map[string]Object, sizeHint(len(e.store), len(e.slotNames)))
	for k, v := range e.store {
		store[k] = v
	}
	for idx, name := range e.slotNames {
		if idx >= 0 && idx < len(e.slots) && e.slots[idx] != nil {
			store[name] = e.slots[idx]
		}
	}
	return store
}

// CallableSnapshot holds a snapshot of callable and scalar bindings safe to
// pass across goroutine boundaries. Use ApplySnapshot to write them into a new
// Environment.
type CallableSnapshot struct {
	functions map[string]*Function
	lambdas   map[string]*LambdaFunction
	dicts     map[string]*Dict
	scalars   map[string]Object
}

// SnapshotCallables reads callable and scalar bindings from this environment
// into a self-contained snapshot. No references to the source Environment's
// maps are retained, so it is safe to pass the snapshot to another goroutine.
// Module-level scalars (int, float, bool, str, None) are included because
// handler functions legitimately reference module constants; they are
// immutable value objects, so sharing them cannot create cross-task races.
// Mutable globals (lists, dicts, instances, classes) are deliberately excluded
// — data must be passed via task args.
func (e *Environment) SnapshotCallables() *CallableSnapshot {
	s := &CallableSnapshot{
		functions: make(map[string]*Function, sizeHint(len(e.store), len(e.slotNames))),
		lambdas:   make(map[string]*LambdaFunction, sizeHint(len(e.store), len(e.slotNames))),
		dicts:     make(map[string]*Dict, sizeHint(len(e.store), len(e.slotNames))),
		scalars:   make(map[string]Object, sizeHint(len(e.store), len(e.slotNames))),
	}
	snapshot := func(name string, value Object) {
		switch v := value.(type) {
		case *Function:
			s.functions[name] = v
		case *LambdaFunction:
			s.lambdas[name] = v
		case *Class:
			// Classes can't be safely shared across envs.
		case *Dict:
			if e.importedBindings != nil && e.importedBindings[name] {
				s.dicts[name] = v
			}
		case *Integer, *Float, *Boolean, *String, *Null:
			s.scalars[name] = v
		}
	}
	for name, value := range e.store {
		snapshot(name, value)
	}
	for idx, name := range e.slotNames {
		if idx >= 0 && idx < len(e.slots) && e.slots[idx] != nil {
			snapshot(name, e.slots[idx])
		}
	}
	return s
}

// ApplySnapshot writes the snapshot's bindings into target, rebound to target
// so closures resolve correctly. Dicts are deep-copied so concurrent tasks
// don't race when mutating intermediate dicts. Scalars are shared as-is — they
// are immutable value objects.
func (s *CallableSnapshot) ApplySnapshot(target *Environment) {
	for name, v := range s.functions {
		target.store[name] = &Function{
			Name:             v.Name,
			Parameters:       v.Parameters,
			DefaultValues:    v.DefaultValues,
			Variadic:         v.Variadic,
			Kwargs:           v.Kwargs,
			KeywordOnlyStart: v.KeywordOnlyStart,
			Body:             v.Body,
			Env:              target,
			LocalSlots:       v.LocalSlots,
			LocalSlotNames:   v.LocalSlotNames,
			ParamSlotIndexes: v.ParamSlotIndexes,
			ReuseCallEnv:     v.ReuseCallEnv,
			CompiledDefaults: v.CompiledDefaults,
			ResolvedDefaults: v.ResolvedDefaults,
			// CompiledBody is not copied: the source function may be running
			// on another goroutine that memoises it on first call, and this
			// can run without the GIL. The copy re-derives it from the
			// atomic AST cache on its own first call.
			CompilerOwned: v.CompilerOwned, // the copy belongs to the target tree alone
		}
	}
	for name, v := range s.lambdas {
		target.store[name] = &LambdaFunction{
			Parameters:       v.Parameters,
			DefaultValues:    v.DefaultValues,
			Variadic:         v.Variadic,
			Kwargs:           v.Kwargs,
			KeywordOnlyStart: v.KeywordOnlyStart,
			PositionalOnly:   v.PositionalOnly,
			Body:             v.Body,
			Env:              target,
			LocalSlots:       v.LocalSlots,
			LocalSlotNames:   v.LocalSlotNames,
			ParamSlotIndexes: v.ParamSlotIndexes,
			ReuseCallEnv:     v.ReuseCallEnv,
			CompiledBody:     v.CompiledBody,
			CompiledDefaults: v.CompiledDefaults,
			ResolvedDefaults: v.ResolvedDefaults,
		}
	}
	for name, v := range s.dicts {
		target.store[name] = deepCopyDict(v)
		if target.importedBindings == nil {
			target.importedBindings = make(map[string]bool, 2)
		}
		target.importedBindings[name] = true
	}
	for name, v := range s.scalars {
		target.store[name] = v
	}
}

// CopyCallableBindingsTo copies safe bindings into target for background task use.
// Functions and lambdas are copied and rebound to target. Dicts are copied so
// imported modules remain available. Other globals are intentionally skipped so
// background tasks cannot share caller-owned mutable or native-backed state.
func (e *Environment) CopyCallableBindingsTo(target *Environment) {
	e.SnapshotCallables().ApplySnapshot(target)
}

// deepCopyDict recursively copies a Dict so concurrent tasks don't race
// when mutating intermediate dicts.
func deepCopyDict(d *Dict) *Dict {
	if d == nil {
		return nil
	}
	out := NewDictSized(d.Len())
	out.SetFactory(d.Factory())
	for _, k := range d.OrderedKeys() {
		v, _ := d.Lookup(k)
		if nested, ok := v.Value.(*Dict); ok {
			v.Value = deepCopyDict(nested)
		}
		out.Store(k, v.Key, v.Value)
	}
	out.Module = d.Module
	return out
}

// ResetStore removes all keys from the environment store except those in keep.
func (e *Environment) ResetStore(keep map[string]bool) {
	for k := range e.store {
		if !keep[k] {
			delete(e.store, k)
		}
	}
	for idx, name := range e.slotNames {
		if !keep[name] && idx >= 0 && idx < len(e.slots) {
			e.slots[idx] = nil
		}
	}
}

// SetImportCallback sets a context-free import callback for backwards
// compatibility. New code should use SetImportCallbackWithContext so caller
// cancellation and values are preserved through imports.
func (e *Environment) SetImportCallback(fn func(string) error) {
	if fn == nil {
		e.shared().importCallback = nil
		return
	}
	e.shared().importCallback = func(_ context.Context, name string) error {
		return fn(name)
	}
}

// SetImportCallbackWithContext sets the context-aware import callback for this
// environment's tree: every environment in the tree sees it.
func (e *Environment) SetImportCallbackWithContext(fn func(context.Context, string) error) {
	e.shared().importCallback = fn
}

// GetImportCallback returns a context-free view of the import callback for
// backwards compatibility. Calls made through this wrapper use a background
// context; evaluator internals use GetImportCallbackWithContext.
func (e *Environment) GetImportCallback() func(string) error {
	fn := e.GetImportCallbackWithContext()
	if fn == nil {
		return nil
	}
	return func(name string) error {
		return fn(context.Background(), name)
	}
}

// GetImportCallbackWithContext gets the tree's context-aware import callback.
func (e *Environment) GetImportCallbackWithContext() func(context.Context, string) error {
	return e.shared().importCallback
}

// SetAvailableLibrariesCallback sets the available libraries callback for this
// environment's tree.
func (e *Environment) SetAvailableLibrariesCallback(fn func() []LibraryInfo) {
	e.shared().availableLibrariesCallback = fn
}

// GetAvailableLibrariesCallback gets the tree's available libraries callback
func (e *Environment) GetAvailableLibrariesCallback() func() []LibraryInfo {
	return e.shared().availableLibrariesCallback
}

// SetCurrentModule sets the current module path for relative import resolution
func (e *Environment) SetCurrentModule(module string) {
	e.shared().currentModule = module
}

// GetCurrentModule gets the tree's current module path
func (e *Environment) GetCurrentModule() string {
	return e.shared().currentModule
}

type List struct {
	Elements []Object
}

func (l *List) Type() ObjectType { return LIST_OBJ }
func (l *List) Inspect() string  { return inspectContainer(l) }

func (l *List) AsString() (string, Object) { return "", errMustBeString }
func (l *List) AsInt() (int64, Object)     { return 0, errMustBeInteger }
func (l *List) AsFloat() (float64, Object) { return 0, errMustBeNumber }
func (l *List) AsBool() (bool, Object)     { return len(l.Elements) > 0, nil }
func (l *List) AsList() ([]Object, Object) {
	result := make([]Object, len(l.Elements))
	copy(result, l.Elements)
	return result, nil
}
func (l *List) AsDict() (map[string]Object, Object) { return nil, errMustBeDict }

func (l *List) CoerceString() (string, Object) { return l.Inspect(), nil }
func (l *List) CoerceInt() (int64, Object)     { return 0, errMustBeInteger }
func (l *List) CoerceFloat() (float64, Object) { return 0, errMustBeNumber }

type Tuple struct {
	Elements []Object
}

// pairTuple is a two-element tuple together with its elements, so the pair
// is one allocation rather than a tuple and a separate backing array.
type pairTuple struct {
	Tuple
	elems [2]Object
}

// NewPair returns the tuple (a, b).
func NewPair(a, b Object) *Tuple {
	p := &pairTuple{elems: [2]Object{a, b}}
	p.Elements = p.elems[:]
	return &p.Tuple
}

func (t *Tuple) Type() ObjectType { return TUPLE_OBJ }
func (t *Tuple) Inspect() string  { return inspectContainer(t) }

func (t *Tuple) AsString() (string, Object) { return "", errMustBeString }
func (t *Tuple) AsInt() (int64, Object)     { return 0, errMustBeInteger }
func (t *Tuple) AsFloat() (float64, Object) { return 0, errMustBeNumber }
func (t *Tuple) AsBool() (bool, Object)     { return len(t.Elements) > 0, nil }
func (t *Tuple) AsList() ([]Object, Object) {
	result := make([]Object, len(t.Elements))
	copy(result, t.Elements)
	return result, nil
}
func (t *Tuple) AsDict() (map[string]Object, Object) { return nil, errMustBeDict }

func (t *Tuple) CoerceString() (string, Object) { return t.Inspect(), nil }
func (t *Tuple) CoerceInt() (int64, Object)     { return 0, errMustBeInteger }
func (t *Tuple) CoerceFloat() (float64, Object) { return 0, errMustBeNumber }

// StringKey returns the string representation of the key.
// For String keys, returns the actual string value; for other types, returns Inspect().
// This is the canonical way to extract a human-readable key from a DictPair.
func (p DictPair) StringKey() string {
	if s, ok := p.Key.(*String); ok {
		return s.value
	}
	return p.Key.Inspect()
}

// NewStringDict creates a Dict from string key-value pairs.
// Usage: NewStringDict(map[string]Object{"key": value, ...})
// A Go map has no order, so entries are inserted in sorted key order, which
// makes the resulting dict deterministic.
func NewStringDict(entries map[string]Object) *Dict {
	d := NewDictSized(len(entries))
	// The keys of a Go map are distinct, so the entries are appended
	// directly and sorted in place, with no lookup per key.
	for k, v := range entries {
		d.entries = append(d.entries, dictEntry{canonical: stringDictKey(k), DictPair: DictPair{Key: &String{value: k}, Value: v}})
	}
	d.live = int32(len(d.entries))
	slices.SortFunc(d.entries, func(a, b dictEntry) int {
		return strings.Compare(a.Key.(*String).value, b.Key.(*String).value)
	})
	if d.index != nil {
		for i := range d.entries {
			d.index[d.entries[i].canonical] = int32(i)
		}
	}
	return d
}

// NewModuleDict creates the dict for a module named name from its members,
// in the order given. The names must be distinct.
func NewModuleDict(name string, names []string, values []Object) *Dict {
	d := NewDictSized(len(names))
	d.Module = name
	for i, member := range names {
		d.push(dictEntry{canonical: stringDictKey(member), DictPair: DictPair{Key: &String{value: member}, Value: values[i]}})
	}
	return d
}

// dictExtra carries optional dict behaviour; nil for an ordinary dict.
type dictExtra struct {
	// factory makes the dict a defaultdict: reading a missing key calls it
	// (no arguments), stores the result and returns it. A *Null factory
	// (defaultdict(None)) behaves like a plain dict.
	factory Object
	// owner links an instance's __dict__ / vars() view to the instance:
	// writes to the dict write through to the instance's fields.
	owner *Instance
}

// Factory returns the defaultdict factory, or nil for an ordinary dict.
func (d *Dict) Factory() Object {
	if d.extra == nil {
		return nil
	}
	return d.extra.factory
}

// SetFactory makes the dict a defaultdict with the given factory (nil clears it).
func (d *Dict) SetFactory(f Object) {
	if f == nil && d.extra == nil {
		return
	}
	if d.extra == nil {
		d.extra = &dictExtra{}
	}
	d.extra.factory = f
}

// LinkOwner makes writes to this dict write through to inst's fields, as
// obj.__dict__ does in Python. Copies of the dict are not linked.
func (d *Dict) LinkOwner(inst *Instance) {
	if d.extra == nil {
		d.extra = &dictExtra{}
	}
	d.extra.owner = inst
}

// DefaultDictType is the collections.defaultdict constructor, registered by
// the stdlib so isinstance(d, defaultdict) can recognise it. FactoryRepr
// renders a default factory the way Python does (<class 'list'>); the
// evaluator installs it. Both are nil until then.
var (
	DefaultDictType *Builtin
	FactoryRepr     func(factory Object) string
)

// IsDefaultDict reports whether the dict was created by defaultdict().
func (d *Dict) IsDefaultDict() bool { return d.Factory() != nil }

func (d *Dict) Type() ObjectType { return DICT_OBJ }
func (d *Dict) Inspect() string  { return inspectContainer(d) }

func (d *Dict) AsString() (string, Object) { return "", errMustBeString }
func (d *Dict) AsInt() (int64, Object)     { return 0, errMustBeInteger }
func (d *Dict) AsFloat() (float64, Object) { return 0, errMustBeNumber }
func (d *Dict) AsBool() (bool, Object)     { return d.Len() > 0, nil }
func (d *Dict) AsList() ([]Object, Object) { return nil, errMustBeList }
func (d *Dict) AsDict() (map[string]Object, Object) {
	result := make(map[string]Object)
	for _, pair := range d.All() {
		result[pair.Key.Inspect()] = pair.Value
	}
	return result, nil
}

func (d *Dict) CoerceString() (string, Object) { return d.Inspect(), nil }
func (d *Dict) CoerceInt() (int64, Object)     { return 0, errMustBeInteger }
func (d *Dict) CoerceFloat() (float64, Object) { return 0, errMustBeNumber }

// GetByString retrieves a pair using a string key (convenience for attribute-style access).
func (d *Dict) GetByString(name string) (DictPair, bool) {
	pair, ok := d.Lookup(DictStringKey(name))
	return pair, ok
}

// SetByString sets a pair using a string key (convenience for attribute-style access).
func (d *Dict) SetByString(name string, value Object) {
	k := DictStringKey(name)
	d.Store(k, &String{value: name}, value)
}

// HasByString checks if a string key exists in the dict.
func (d *Dict) HasByString(name string) bool {
	_, ok := d.Lookup(DictStringKey(name))
	return ok
}

// DeleteByString deletes a string key from the dict. Returns true if key existed.
func (d *Dict) DeleteByString(name string) bool {
	k := DictStringKey(name)
	_, ok := d.Lookup(k)
	if ok {
		d.Delete(k)
	}
	return ok
}

type Error struct {
	Message  string
	Line     int
	File     string
	Function string
	// ExceptionType names the Python exception this error corresponds to, e.g.
	// "ZeroDivisionError". It lets `except <Type>:` match a runtime error
	// precisely instead of the try handler having to guess the type from the
	// message text. Empty means "unclassified": the handler falls back to
	// inferring a type, and the error is still caught by `except Exception`.
	//
	// Errors remain Errors rather than Exceptions on purpose — Error is the
	// evaluator's propagation signal, recognised by the object.IsError checks
	// threaded through expression evaluation — so setting this changes only how
	// an except clause matches, not how the value travels.
	ExceptionType string
}

func (e *Error) Type() ObjectType { return ERROR_OBJ }
func (e *Error) Inspect() string {
	msg := "ERROR: " + e.Message
	if e.Function != "" {
		msg += fmt.Sprintf(" in function '%s'", e.Function)
	}
	if e.File != "" {
		msg += fmt.Sprintf(" in %s", e.File)
	}
	if e.Line > 0 {
		msg += fmt.Sprintf(" at line %d", e.Line)
	}
	return msg
}

func (e *Error) AsString() (string, Object)          { return e.Message, nil }
func (e *Error) AsInt() (int64, Object)              { return 0, errMustBeInteger }
func (e *Error) AsFloat() (float64, Object)          { return 0, errMustBeNumber }
func (e *Error) AsBool() (bool, Object)              { return false, nil }
func (e *Error) AsList() ([]Object, Object)          { return nil, errMustBeList }
func (e *Error) AsDict() (map[string]Object, Object) { return nil, errMustBeDict }

func (e *Error) CoerceString() (string, Object) { return e.Inspect(), nil }
func (e *Error) CoerceInt() (int64, Object)     { return 0, errMustBeInteger }
func (e *Error) CoerceFloat() (float64, Object) { return 0, errMustBeNumber }

type Exception struct {
	Message       string
	ExceptionType string // Exception type for identification (e.g., "SystemExit", "ValueError", etc.)
	Code          int    // Exit code for SystemExit; ignored for other exception types
	// Args holds the constructor arguments for exceptions built from user
	// exception classes, so `e.args` matches Python.
	Args []Object
	// TypeChain names the user exception class hierarchy a custom exception
	// was raised through (["MyError", "ValueError"] for
	// class MyError(ValueError)), so except clauses and isinstance match
	// base classes of user exception classes.
	TypeChain []string
	// OriginInstance keeps the user exception class instance a raise
	// converted from, so custom attributes and methods (e.code) remain
	// reachable on the caught exception.
	OriginInstance *Instance
	// Cause is __cause__, set by `raise X from e`; nil is None (no cause,
	// or `raise X from None`).
	Cause *Exception
	// Raised distinguishes an exception that is actively propagating (produced
	// by a `raise` or by an operation that failed) from one that is merely a
	// value (constructed via `ValueError("x")`, bound by `except ... as e`, or
	// stored in a variable). Only a raised exception unwinds the call stack;
	// a value exception is passed around like any other object. This mirrors
	// Python, where constructing an exception does not raise it.
	Raised bool
}

func (ex *Exception) Type() ObjectType { return EXCEPTION_OBJ }
func (ex *Exception) Inspect() string {
	// repr(e) names the exception's class ("ValueError: bad input"), like
	// Python; the generic wrapper name only shows when no type was set.
	if ex.ExceptionType != "" {
		return ex.ExceptionType + ": " + ex.Message
	}
	return "EXCEPTION: " + ex.Message
}

func (ex *Exception) AsString() (string, Object)          { return ex.Message, nil }
func (ex *Exception) AsInt() (int64, Object)              { return 0, errMustBeInteger }
func (ex *Exception) AsFloat() (float64, Object)          { return 0, errMustBeNumber }
func (ex *Exception) AsBool() (bool, Object)              { return false, nil }
func (ex *Exception) AsList() ([]Object, Object)          { return nil, errMustBeList }
func (ex *Exception) AsDict() (map[string]Object, Object) { return nil, errMustBeDict }

func (ex *Exception) CoerceString() (string, Object) { return ex.Inspect(), nil }
func (ex *Exception) CoerceInt() (int64, Object)     { return 0, errMustBeInteger }
func (ex *Exception) CoerceFloat() (float64, Object) { return 0, errMustBeNumber }

// IsSystemExit returns true if this is a SystemExit exception
func (ex *Exception) IsSystemExit() bool {
	return ex.ExceptionType == ExceptionTypeSystemExit
}

func (ex *Exception) IsPermissionError() bool {
	return ex.ExceptionType == ExceptionTypePermissionError
}

// GetExitCode returns the exit code for SystemExit exceptions
// For non-SystemExit exceptions, returns 0 (the Code field is ignored)
func (ex *Exception) GetExitCode() int {
	return ex.Code
}

type Class struct {
	Name      string
	BaseClass *Class // optional parent class for inheritance
	Methods   map[string]Object
	Env       *Environment
	// ExceptionBase names the built-in exception type this class derives
	// from ("ValueError" for class MyError(ValueError)); "" means the class
	// is not an exception. Set when the base is an exception constructor;
	// inherited through BaseClass chains of user exception classes.
	ExceptionBase string
	// IsEnum marks classes deriving from enum.Enum: construction looks up
	// members by value and iteration yields the members, in EnumMembers
	// order. IntEnum additionally compares equal to plain values.
	IsEnum      bool
	IsIntEnum   bool
	EnumMembers []Object
	// IsABC marks abc.ABC and the classes deriving from it: only they
	// enforce @abstractmethod, as in Python.
	IsABC bool
	// AbstractMethods lists, sorted, the abstract methods an ABC class
	// leaves unimplemented; non-nil makes instantiation a TypeError.
	AbstractMethods []string
	// FieldNames lists the class body's annotated names in source order
	// (@dataclass fields); AssignNames the plain top-level assignment
	// targets (enum members, dataclass defaults). Compile-time metadata;
	// values live in Methods.
	FieldNames  []string
	AssignNames []string
	cacheMu     sync.RWMutex
	cache       map[string]classLookupCacheEntry
	// shapeRoot is the empty field layout of this class's instances, from
	// which the shared layouts they move through grow; see instShape.
	shapeRoot atomic.Pointer[instShape]
	// setattr caches whether the class defines __setattr__, so an attribute
	// assignment does not look the name up each time: the lookup epoch the
	// answer was computed under (plus one) shifted left, and the answer in
	// the low bit.
	setattr atomic.Uint64
}

func (c *Class) Type() ObjectType { return CLASS_OBJ }
func (c *Class) Inspect() string  { return fmt.Sprintf("<class '%s'>", c.Name) }

func (c *Class) AsString() (string, Object)          { return c.Name, nil }
func (c *Class) AsInt() (int64, Object)              { return 0, errMustBeInteger }
func (c *Class) AsFloat() (float64, Object)          { return 0, errMustBeNumber }
func (c *Class) AsBool() (bool, Object)              { return true, nil }
func (c *Class) AsList() ([]Object, Object)          { return nil, errMustBeList }
func (c *Class) AsDict() (map[string]Object, Object) { return c.Methods, nil }

func (c *Class) CoerceString() (string, Object) { return c.Inspect(), nil }
func (c *Class) CoerceInt() (int64, Object)     { return 0, errMustBeInteger }
func (c *Class) CoerceFloat() (float64, Object) { return 0, errMustBeNumber }

func (c *Class) LookupMember(name string) (Object, bool) {
	epoch := classLookupEpoch.Load()

	c.cacheMu.RLock()
	if entry, ok := c.cache[name]; ok && entry.epoch == epoch {
		c.cacheMu.RUnlock()
		return entry.value, entry.ok
	}
	c.cacheMu.RUnlock()

	currentClass := c
	var (
		value Object
		ok    bool
	)
	for currentClass != nil {
		if value, ok = currentClass.Methods[name]; ok {
			break
		}
		currentClass = currentClass.BaseClass
	}

	c.cacheMu.Lock()
	if c.cache == nil {
		c.cache = make(map[string]classLookupCacheEntry)
	}
	c.cache[name] = classLookupCacheEntry{
		value: value,
		ok:    ok,
		epoch: epoch,
	}
	c.cacheMu.Unlock()

	return value, ok
}

// HasSetattr reports whether the class defines __setattr__. The answer is
// cached until a class attribute is next assigned.
func (c *Class) HasSetattr() bool {
	epoch := classLookupEpoch.Load() + 1
	if v := c.setattr.Load(); v>>1 == epoch {
		return v&1 == 1
	}
	_, has := c.Methods["__setattr__"]
	v := epoch << 1
	if has {
		v |= 1
	}
	c.setattr.Store(v)
	return has
}

func (c *Class) InvalidateLookupCache() {
	classLookupEpoch.Add(1)
	c.cacheMu.Lock()
	c.cache = nil
	c.cacheMu.Unlock()
}

// instInlineVals is how many field values an Instance holds inline; values past
// it live in ext. Most instances have only a handful of fields.
const instInlineVals = 4

type Instance struct {
	Class *Class
	// Field storage: shape lists the field names in assignment order, and the
	// value of the field at position n is vals[n] for the first instInlineVals
	// fields and ext.more[n-instInlineVals] after that (an instance with a
	// private shape keeps them all in ext.more). Instances of a class that
	// assign the same fields in the same order share one shape, so the names
	// are not stored per instance. nil shape means no fields. Access only
	// through the field accessor methods — never touch these directly so the
	// representation can keep evolving.
	shape *instShape
	vals  [instInlineVals]Object

	NativeData       any
	boundMethodCache map[string]boundMethodCacheEntry
	ext              *instanceExt
}

// instanceExt holds the values of an instance's fields past the inline ones.
// The first few share its allocation.
type instanceExt struct {
	more []Object
	buf  [4]Object
}

// NewInstance creates an empty instance of the given class. Use the field
// accessor methods (SetField, GetField, …) to populate it. Every instance is
// created here so its memory is accounted: instances that reference only each
// other are held by no counted container.
func NewInstance(class *Class) *Instance {
	i := &Instance{Class: class}
	if accountingOn.Load() {
		trackInstance(i)
	}
	return i
}

// instanceObjectOverhead approximates the bytes an Instance holds before any
// fields past the inline ones: the object itself, inline values included.
const instanceObjectOverhead = int64(unsafe.Sizeof(Instance{})) + 16

// trackInstance keeps NewInstance small enough to inline: it only calls out
// when accounting is on.
//
//go:noinline
func trackInstance(i *Instance) {
	trackAlloc(i, instanceObjectOverhead)
}

// NewInstanceWithFields creates an instance of the given class pre-populated
// with the supplied fields. The map is consumed (copied into the instance's
// storage); a nil map yields an empty instance. Fields are set in name order:
// a Go map has none, and one order per class lets its instances share a field
// layout.
func NewInstanceWithFields(class *Class, fields map[string]Object) *Instance {
	i := NewInstance(class)
	var buf [16]string
	names := buf[:0]
	for k := range fields {
		names = append(names, k)
	}
	slices.Sort(names)
	for _, k := range names {
		i.SetField(k, fields[k])
	}
	return i
}

// NewInstanceWithData is like NewInstanceWithFields but also attaches an opaque
// native-data value to the instance.
func NewInstanceWithData(class *Class, fields map[string]Object, nativeData any) *Instance {
	i := NewInstanceWithFields(class, fields)
	i.NativeData = nativeData
	return i
}

func (i *Instance) Type() ObjectType { return INSTANCE_OBJ }
func (i *Instance) Inspect() string {
	// Check for __str_repr__ field (used by libraries to provide custom string representation)
	if strRepr, ok := i.GetField("__str_repr__"); ok {
		if s, ok := strRepr.(*String); ok {
			return s.value
		}
	}
	return fmt.Sprintf("<%s object at %p>", i.Class.Name, i)
}

// --- Instance field accessors ---
//
// These methods are the supported API for reading and writing instance fields.
// They exist so the underlying storage can change without affecting callers;
// do not access the backing store directly. Field access is single-goroutine
// per environment tree (serialized by the interpreter lock), so no locking is
// required here; shared shapes are immutable apart from their transitions,
// which are safe for concurrent use.

// slot returns the storage of the value at field position n: the first
// instInlineVals values of a shared shape live inline and the rest in
// ext.more; a private shape keeps all its values in ext.more.
func (i *Instance) slot(n int) *Object {
	if i.shape.private {
		return &i.ext.more[n]
	}
	if n < instInlineVals {
		return &i.vals[n]
	}
	return &i.ext.more[n-instInlineVals]
}

// GetField returns the value of the named field and whether it was set.
func (i *Instance) GetField(name string) (Object, bool) {
	s := i.shape
	if s == nil {
		return nil, false
	}
	if !s.private {
		for n, k := range s.names {
			if k == name {
				if n < instInlineVals {
					return i.vals[n], true
				}
				return i.ext.more[n-instInlineVals], true
			}
		}
		return nil, false
	}
	if n, ok := s.index[name]; ok {
		return i.ext.more[n], true
	}
	return nil, false
}

// Field returns the value of the named field, or nil if it is not set. It is a
// convenience for callers that immediately type-assert or coerce a field known
// to exist; use GetField when you need to distinguish "absent" from "nil".
func (i *Instance) Field(name string) Object {
	v, _ := i.GetField(name)
	return v
}

// SetField sets the named field, allocating backing storage on first use.
func (i *Instance) SetField(name string, val Object) {
	if s := i.shape; s != nil {
		if !s.private {
			for n, k := range s.names {
				if k == name {
					if n < instInlineVals {
						i.vals[n] = val
					} else {
						i.ext.more[n-instInlineVals] = val
					}
					return
				}
			}
		} else if n, ok := s.index[name]; ok {
			i.ext.more[n] = val
			return
		}
	}
	i.addField(name, val)
}

// addField appends a new field: the instance moves to the shared shape that
// extends its current one by name, or to a private shape when there is none.
func (i *Instance) addField(name string, val Object) {
	s := i.shape
	if s == nil {
		s = i.Class.rootShape()
		if s == nil {
			s = newPrivateShape(nil)
			i.shape = s
		}
	}
	if !s.private {
		if t := s.next(name); t != nil {
			n := len(s.names)
			i.shape = t
			if n < instInlineVals {
				i.vals[n] = val
				return
			}
			i.appendMore(val)
			return
		}
		i.privatize(s.names)
		s = i.shape
	}
	s.add(name)
	i.appendMore(val)
}

func (i *Instance) appendMore(val Object) {
	if i.ext == nil {
		i.ext = &instanceExt{}
		i.ext.more = i.ext.buf[:0]
	}
	oldCap := cap(i.ext.more)
	i.ext.more = append(i.ext.more, val)
	if cap(i.ext.more) != oldCap && accountingOn.Load() {
		trackAlloc(&i.ext.more[0], int64(cap(i.ext.more))*dictEntryWeight)
	}
}

// privatize gives the instance a private shape listing names, which are the
// names of its current values in order, and moves all values into ext.more.
func (i *Instance) privatize(names []string) {
	more := make([]Object, 0, len(names)+4)
	for n := range names {
		if n < instInlineVals {
			more = append(more, i.vals[n])
			i.vals[n] = nil
		} else {
			more = append(more, i.ext.more[n-instInlineVals])
		}
	}
	if i.ext == nil {
		i.ext = &instanceExt{}
	}
	i.ext.more = more
	if accountingOn.Load() && cap(more) > 0 {
		trackAlloc(&more[:1][0], int64(cap(more))*dictEntryWeight)
	}
	i.shape = newPrivateShape(names)
}

// DeleteField removes the named field if present. The remaining fields keep
// their order.
func (i *Instance) DeleteField(name string) {
	s := i.shape
	if s == nil {
		return
	}
	n := s.find(name)
	if n < 0 {
		return
	}
	last := len(s.names) - 1
	for k := n; k < last; k++ {
		*i.slot(k) = *i.slot(k + 1)
	}
	*i.slot(last) = nil
	if s.private {
		i.ext.more = i.ext.more[:last]
		s.remove(n)
		return
	}
	if last >= instInlineVals {
		i.ext.more = i.ext.more[:last-instInlineVals]
	}
	// Move to the shared shape holding the remaining names, reached from the
	// class's root as an instance assigning them in order would reach it.
	t := i.Class.rootShape()
	for k, nm := range s.names {
		if t == nil {
			break
		}
		if k != n {
			t = t.next(nm)
		}
	}
	if t != nil {
		i.shape = t
		return
	}
	rest := make([]string, 0, last)
	rest = append(rest, s.names[:n]...)
	rest = append(rest, s.names[n+1:]...)
	i.privatize(rest)
}

// HasField reports whether the named field is set.
func (i *Instance) HasField(name string) bool {
	_, ok := i.GetField(name)
	return ok
}

// FieldCount returns the number of set fields.
func (i *Instance) FieldCount() int {
	if i.shape == nil {
		return 0
	}
	return len(i.shape.names)
}

// RangeFields calls fn for each set field in insertion order (the order the
// fields were first assigned) and stops early if fn returns false.
func (i *Instance) RangeFields(fn func(name string, val Object) bool) {
	s := i.shape
	if s == nil {
		return
	}
	for n, k := range s.names {
		if !fn(k, *i.slot(n)) {
			return
		}
	}
}

// FieldsSnapshot returns a new map containing a copy of all set fields. Use it
// where a standalone map is needed (serialization, AsDict); mutating the result
// does not affect the instance.
func (i *Instance) FieldsSnapshot() map[string]Object {
	m := make(map[string]Object, i.FieldCount())
	i.RangeFields(func(k string, v Object) bool {
		m[k] = v
		return true
	})
	return m
}

func (i *Instance) AsString() (string, Object) { return i.Inspect(), nil }
func (i *Instance) AsInt() (int64, Object)     { return 0, errMustBeInteger }
func (i *Instance) AsFloat() (float64, Object) { return 0, errMustBeNumber }
func (i *Instance) AsBool() (bool, Object)     { return true, nil }

// Collector is implemented by the native state of an instance that Go code
// may take as a list by running it to its end: a generator, so a function
// that accepts a list through AsList accepts a generator expression too.
type Collector interface {
	Collect() ([]Object, Object)
}

// AsList collects a generator's remaining values; any other instance is not
// a list.
func (i *Instance) AsList() ([]Object, Object) {
	if c, ok := i.NativeData.(Collector); ok {
		return c.Collect()
	}
	return nil, errMustBeList
}
func (i *Instance) AsDict() (map[string]Object, Object) { return i.FieldsSnapshot(), nil }

func (i *Instance) CoerceString() (string, Object) { return i.Inspect(), nil }
func (i *Instance) CoerceInt() (int64, Object)     { return 0, errMustBeInteger }
func (i *Instance) CoerceFloat() (float64, Object) { return 0, errMustBeNumber }

func (i *Instance) GetBoundMethod(name string, method Object) *BoundMethod {
	epoch := classLookupEpoch.Load()
	if entry, ok := i.boundMethodCache[name]; ok && entry.epoch == epoch && entry.method == method {
		return entry.bound
	}
	if i.boundMethodCache == nil {
		i.boundMethodCache = make(map[string]boundMethodCacheEntry)
	}
	bound := &BoundMethod{Instance: i, Method: method}
	bound.selfBuf[0] = i
	i.boundMethodCache[name] = boundMethodCacheEntry{
		bound:  bound,
		method: method,
		epoch:  epoch,
	}
	return bound
}

func (i *Instance) InvalidateBoundMethod(name string) {
	if i.boundMethodCache == nil {
		return
	}
	delete(i.boundMethodCache, name)
}

type Super struct {
	Class    *Class
	Instance *Instance
}

func (s *Super) Type() ObjectType { return SUPER_OBJ }
func (s *Super) Inspect() string {
	return fmt.Sprintf("<super: <class '%s'>, <%s object>>", s.Class.Name, s.Instance.Class.Name)
}

func (s *Super) AsString() (string, Object)          { return s.Inspect(), nil }
func (s *Super) AsInt() (int64, Object)              { return 0, errMustBeInteger }
func (s *Super) AsFloat() (float64, Object)          { return 0, errMustBeNumber }
func (s *Super) AsBool() (bool, Object)              { return true, nil }
func (s *Super) AsList() ([]Object, Object)          { return nil, errMustBeList }
func (s *Super) AsDict() (map[string]Object, Object) { return nil, errMustBeDict }

func (s *Super) CoerceString() (string, Object) { return s.Inspect(), nil }
func (s *Super) CoerceInt() (int64, Object)     { return 0, errMustBeInteger }
func (s *Super) CoerceFloat() (float64, Object) { return 0, errMustBeNumber }

// Property wraps a getter (and optional setter) for use with @property.
type Property struct {
	Getter Object // Function to call when the attribute is accessed
	Setter Object // Function to call when the attribute is assigned (nil = read-only)
}

func (p *Property) Type() ObjectType { return PROPERTY_OBJ }
func (p *Property) Inspect() string  { return "<property>" }

func (p *Property) AsString() (string, Object)          { return "", errMustBeString }
func (p *Property) AsInt() (int64, Object)              { return 0, errMustBeInteger }
func (p *Property) AsFloat() (float64, Object)          { return 0, errMustBeNumber }
func (p *Property) AsBool() (bool, Object)              { return true, nil }
func (p *Property) AsList() ([]Object, Object)          { return nil, errMustBeList }
func (p *Property) AsDict() (map[string]Object, Object) { return nil, errMustBeDict }

func (p *Property) CoerceString() (string, Object) { return p.Inspect(), nil }
func (p *Property) CoerceInt() (int64, Object)     { return 0, errMustBeInteger }
func (p *Property) CoerceFloat() (float64, Object) { return 0, errMustBeNumber }

// StaticMethod wraps a function for use with @staticmethod.
// When called on an instance, self is not prepended.
type StaticMethod struct {
	Fn Object
}

func (s *StaticMethod) Type() ObjectType { return STATICMETHOD_OBJ }
func (s *StaticMethod) Inspect() string  { return "<staticmethod>" }

func (s *StaticMethod) AsString() (string, Object)          { return "", errMustBeString }
func (s *StaticMethod) AsInt() (int64, Object)              { return 0, errMustBeInteger }
func (s *StaticMethod) AsFloat() (float64, Object)          { return 0, errMustBeNumber }
func (s *StaticMethod) AsBool() (bool, Object)              { return true, nil }
func (s *StaticMethod) AsList() ([]Object, Object)          { return nil, errMustBeList }
func (s *StaticMethod) AsDict() (map[string]Object, Object) { return nil, errMustBeDict }

func (s *StaticMethod) CoerceString() (string, Object) { return s.Inspect(), nil }
func (s *StaticMethod) CoerceInt() (int64, Object)     { return 0, errMustBeInteger }
func (s *StaticMethod) CoerceFloat() (float64, Object) { return 0, errMustBeNumber }

// ClassMethod wraps a function for use with @classmethod.
// When called, the class is passed as the first argument instead of self.
type ClassMethod struct {
	Fn Object
}

func (c *ClassMethod) Type() ObjectType { return CLASSMETHOD_OBJ }
func (c *ClassMethod) Inspect() string  { return "<classmethod>" }

func (c *ClassMethod) AsString() (string, Object)          { return "", errMustBeString }
func (c *ClassMethod) AsInt() (int64, Object)              { return 0, errMustBeInteger }
func (c *ClassMethod) AsFloat() (float64, Object)          { return 0, errMustBeNumber }
func (c *ClassMethod) AsBool() (bool, Object)              { return true, nil }
func (c *ClassMethod) AsList() ([]Object, Object)          { return nil, errMustBeList }
func (c *ClassMethod) AsDict() (map[string]Object, Object) { return nil, errMustBeDict }

func (c *ClassMethod) CoerceString() (string, Object) { return c.Inspect(), nil }
func (c *ClassMethod) CoerceInt() (int64, Object)     { return 0, errMustBeInteger }
func (c *ClassMethod) CoerceFloat() (float64, Object) { return 0, errMustBeNumber }

// LibraryRegistrar is an interface for registering libraries.
// This allows external libraries to register themselves without circular imports.
type LibraryRegistrar interface {
	RegisterLibrary(lib *Library)
}

// AsErrorObj returns the object as an Error, or nil/false if not
func AsErrorObj(obj Object) (*Error, bool) {
	if err, ok := obj.(*Error); ok {
		return err, true
	}
	return nil, false
}

// ValidateTransferable checks whether obj contains only types that can be
// safely passed to a background task. Transferable types are scalars (Null,
// Boolean, Integer, Float, String) and recursively transferable containers
// (List, Dict, Set, Tuple). Instances, classes, functions, builtins, and
// other stateful objects are rejected. Circular references are also rejected.
func ValidateTransferable(obj Object) error {
	return validateTransferable(obj, make(map[any]struct{}))
}

func validateTransferable(obj Object, visited map[any]struct{}) error {
	switch v := obj.(type) {
	case *Null, *Boolean, *Integer, *Float, *String, *FloatArray:
		return nil
	case *List:
		if _, seen := visited[v]; seen {
			return fmt.Errorf("circular reference in list")
		}
		visited[v] = struct{}{}
		for i, e := range v.Elements {
			if err := validateTransferable(e, visited); err != nil {
				return fmt.Errorf("list[%d]: %w", i, err)
			}
		}
		delete(visited, v)
		return nil
	case *Tuple:
		for i, e := range v.Elements {
			if err := validateTransferable(e, visited); err != nil {
				return fmt.Errorf("tuple[%d]: %w", i, err)
			}
		}
		return nil
	case *Dict:
		if _, seen := visited[v]; seen {
			return fmt.Errorf("circular reference in dict")
		}
		visited[v] = struct{}{}
		for k, p := range v.All() {
			if err := validateTransferable(p.Key, visited); err != nil {
				return fmt.Errorf("dict key %q: %w", k, err)
			}
			if err := validateTransferable(p.Value, visited); err != nil {
				return fmt.Errorf("dict[%q]: %w", k, err)
			}
		}
		delete(visited, v)
		return nil
	case *Set:
		if _, seen := visited[v]; seen {
			return fmt.Errorf("circular reference in set")
		}
		visited[v] = struct{}{}
		for k, e := range v.All() {
			if err := validateTransferable(e, visited); err != nil {
				return fmt.Errorf("set element %q: %w", k, err)
			}
		}
		delete(visited, v)
		return nil
	default:
		return fmt.Errorf("%s is not transferable to background tasks (allowed: None, bool, int, float, str, list, dict, set, tuple)", v.Type())
	}
}

// CloneObject returns a deep copy of mutable Scriptling objects.
// Immutable/scalar types (Null, Boolean, Integer, Float, String, Builtin,
// Function, LambdaFunction, Class, Error, Exception) are returned as-is.
// Mutable containers (List, Dict, Set, Tuple, Instance) are recursively cloned.
// Opaque Instance.NativeData is not cloned because there is no generic way to
// duplicate native Go state safely.
func CloneObject(obj Object) Object {
	switch v := obj.(type) {
	case *Null, *Boolean, *Integer, *Float, *String:
		return obj
	case *FloatArray:
		data := make([]float64, len(v.Data))
		TrackSlice(data)
		copy(data, v.Data)
		shape := make([]int, len(v.Shape))
		copy(shape, v.Shape)
		return &FloatArray{Data: data, Shape: shape}
	case *List:
		elems := make([]Object, len(v.Elements))
		TrackSlice(elems)
		for i, e := range v.Elements {
			elems[i] = CloneObject(e)
		}
		return &List{Elements: elems}
	case *Tuple:
		elems := make([]Object, len(v.Elements))
		TrackSlice(elems)
		for i, e := range v.Elements {
			elems[i] = CloneObject(e)
		}
		return &Tuple{Elements: elems}
	case *Dict:
		out := NewDict()
		for _, k := range v.OrderedKeys() {
			p, _ := v.Lookup(k)
			out.Store(k, CloneObject(p.Key), CloneObject(p.Value))
		}
		return out
	case *Set:
		out := NewSetSized(v.Len())
		for k, e := range v.All() {
			out.AddKeyed(k, CloneObject(e))
		}
		out.Frozen = v.Frozen
		return out
	case *Instance:
		clone := NewInstance(v.Class)
		v.RangeFields(func(k string, val Object) bool {
			clone.SetField(k, CloneObject(val))
			return true
		})
		return clone
	default:
		// Builtins, Functions, Classes, Errors, etc. — immutable or singleton
		return obj
	}
}

// AsException returns the object as an Exception, or nil/false if not
func AsException(obj Object) (*Exception, bool) {
	if ex, ok := obj.(*Exception); ok {
		return ex, true
	}
	return nil, false
}

// NewSystemExit creates a new SystemExit exception with the given code and message
func NewSystemExit(code int, message string) *Exception {
	if message == "" {
		message = fmt.Sprintf("SystemExit: %d", code)
	}
	return &Exception{
		Message:       message,
		ExceptionType: ExceptionTypeSystemExit,
		Code:          code,
		// SystemExit always unwinds the stack; it is a raise, never a value.
		Raised: true,
	}
}
