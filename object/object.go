package object

import (
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"

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
		return fmt.Sprintf("n:%d", o.Value)
	case *Float:
		// If float is exactly representable as int64, use integer key (Python: hash(1.0) == hash(1))
		if !math.IsInf(o.Value, 0) && !math.IsNaN(o.Value) && o.Value == math.Trunc(o.Value) && o.Value >= math.MinInt64 && o.Value <= math.MaxInt64 {
			return fmt.Sprintf("n:%d", int64(o.Value))
		}
		return fmt.Sprintf("f:%v", o.Value)
	case *Boolean:
		if o.Value {
			return "n:1" // True == 1
		}
		return "n:0" // False == 0
	case *String:
		return "s:" + o.Value
	case *Null:
		return "null:"
	case *Tuple:
		// Tuples are hashable in Python if all elements are hashable
		var b strings.Builder
		b.WriteString("t:(")
		for i, e := range o.Elements {
			if i > 0 {
				b.WriteString(",")
			}
			b.WriteString(DictKey(e))
		}
		b.WriteString(")")
		return b.String()
	default:
		// Unhashable types - use type + pointer identity which will rarely collide
		// In Python, lists, dicts, sets are unhashable and raise TypeError
		return fmt.Sprintf("%s:%p", obj.Type(), obj)
	}
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
)

var smallIntegers [smallIntMax - smallIntMin + 1]*Integer

// Pre-allocated error singletons for type accessor methods.
// These avoid allocating a new Error on every failed AsXxx() call.
var (
	errMustBeString  = &Error{Message: ErrMustBeString}
	errMustBeInteger = &Error{Message: ErrMustBeInteger}
	errMustBeNumber  = &Error{Message: ErrMustBeNumber}
	errMustBeBoolean = &Error{Message: ErrMustBeBoolean}
	errMustBeList    = &Error{Message: ErrMustBeList}
	errMustBeDict    = &Error{Message: ErrMustBeDict}
)

// Exception type constants
const (
	ExceptionTypeSystemExit      = "SystemExit"
	ExceptionTypeException       = "Exception"
	ExceptionTypeValueError      = "ValueError"
	ExceptionTypeTypeError       = "TypeError"
	ExceptionTypeNameError       = "NameError"
	ExceptionTypeStopIteration   = "StopIteration"
	ExceptionTypeGeneric         = "" // Default for legacy compatibility
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
		smallIntegers[i-smallIntMin] = &Integer{Value: int64(i)}
	}
}

// NewInteger returns a cached integer for small values, or a new Integer for larger values
func NewInteger(val int64) *Integer {
	if val >= smallIntMin && val <= smallIntMax {
		return smallIntegers[val-smallIntMin]
	}
	return &Integer{Value: val}
}

type ObjectType int

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
	Value int64
}

func (i *Integer) Type() ObjectType { return INTEGER_OBJ }
func (i *Integer) Inspect() string  { return fmt.Sprintf("%d", i.Value) }

func (i *Integer) AsString() (string, Object)          { return "", errMustBeString }
func (i *Integer) AsInt() (int64, Object)              { return i.Value, nil }
func (i *Integer) AsFloat() (float64, Object)          { return float64(i.Value), nil }
func (i *Integer) AsBool() (bool, Object)              { return i.Value != 0, nil }
func (i *Integer) AsList() ([]Object, Object)          { return nil, errMustBeList }
func (i *Integer) AsDict() (map[string]Object, Object) { return nil, errMustBeDict }

func (i *Integer) CoerceString() (string, Object) { return i.Inspect(), nil }
func (i *Integer) CoerceInt() (int64, Object)     { return i.Value, nil }
func (i *Integer) CoerceFloat() (float64, Object) { return float64(i.Value), nil }

type Float struct {
	Value float64
}

func (f *Float) Type() ObjectType { return FLOAT_OBJ }
func (f *Float) Inspect() string  { return fmt.Sprintf("%g", f.Value) }

func (f *Float) AsString() (string, Object)          { return "", errMustBeString }
func (f *Float) AsInt() (int64, Object)              { return int64(f.Value), nil }
func (f *Float) AsFloat() (float64, Object)          { return f.Value, nil }
func (f *Float) AsBool() (bool, Object)              { return f.Value != 0, nil }
func (f *Float) AsList() ([]Object, Object)          { return nil, errMustBeList }
func (f *Float) AsDict() (map[string]Object, Object) { return nil, errMustBeDict }

func (f *Float) CoerceString() (string, Object) { return f.Inspect(), nil }
func (f *Float) CoerceInt() (int64, Object)     { return int64(f.Value), nil }
func (f *Float) CoerceFloat() (float64, Object) { return f.Value, nil }

type Boolean struct {
	Value bool
}

func (b *Boolean) Type() ObjectType { return BOOLEAN_OBJ }
func (b *Boolean) Inspect() string  { return fmt.Sprintf("%t", b.Value) }

func (b *Boolean) AsString() (string, Object)          { return "", errMustBeString }
func (b *Boolean) AsInt() (int64, Object)              { return 0, errMustBeInteger }
func (b *Boolean) AsFloat() (float64, Object)          { return 0, errMustBeNumber }
func (b *Boolean) AsBool() (bool, Object)              { return b.Value, nil }
func (b *Boolean) AsList() ([]Object, Object)          { return nil, errMustBeList }
func (b *Boolean) AsDict() (map[string]Object, Object) { return nil, errMustBeDict }

func (b *Boolean) CoerceString() (string, Object) { return b.Inspect(), nil }
func (b *Boolean) CoerceInt() (int64, Object) {
	if b.Value {
		return 1, nil
	}
	return 0, nil
}
func (b *Boolean) CoerceFloat() (float64, Object) {
	if b.Value {
		return 1, nil
	}
	return 0, nil
}

type String struct {
	Value string
}

func (s *String) Type() ObjectType { return STRING_OBJ }
func (s *String) Inspect() string  { return s.Value }

func (s *String) AsString() (string, Object)          { return s.Value, nil }
func (s *String) AsInt() (int64, Object)              { return 0, errMustBeInteger }
func (s *String) AsFloat() (float64, Object)          { return 0, errMustBeNumber }
func (s *String) AsBool() (bool, Object)              { return s.Value != "", nil }
func (s *String) AsList() ([]Object, Object)          { return nil, errMustBeList }
func (s *String) AsDict() (map[string]Object, Object) { return nil, errMustBeDict }

func (s *String) CoerceString() (string, Object) { return s.Value, nil }
func (s *String) CoerceInt() (int64, Object) {
	val, err := strconv.ParseInt(strings.TrimSpace(s.Value), 10, 64)
	if err != nil {
		return 0, &Error{Message: fmt.Sprintf("cannot convert %s to int", s.Value)}
	}
	return val, nil
}
func (s *String) CoerceFloat() (float64, Object) {
	val, err := strconv.ParseFloat(strings.TrimSpace(s.Value), 64)
	if err != nil {
		return 0, &Error{Message: fmt.Sprintf("cannot convert %s to float", s.Value)}
	}
	return val, nil
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

	if s.Step != nil && s.Step.Value != 1 {
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

type ReturnValue struct {
	Value Object
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

type Function struct {
	Name          string
	Parameters    []*ast.Identifier
	DefaultValues map[string]ast.Expression
	Variadic      *ast.Identifier // *args parameter
	Kwargs        *ast.Identifier // **kwargs parameter
	Body          *ast.BlockStatement
	Env           *Environment
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
	Parameters    []*ast.Identifier
	DefaultValues map[string]ast.Expression
	Variadic      *ast.Identifier // *args parameter
	Kwargs        *ast.Identifier // **kwargs parameter
	Body          ast.Expression
	Env           *Environment
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
// - ctx: Context with environment and runtime information
// - kwargs: Keyword arguments passed to the function (wrapped with helper methods)
// - args: Positional arguments passed to the function
type BuiltinFunction func(ctx context.Context, kwargs Kwargs, args ...Object) Object

type Builtin struct {
	Fn         BuiltinFunction
	HelpText   string            // Optional help documentation for this builtin
	Attributes map[string]Object // Optional attributes for this builtin
}

func (b *Builtin) Type() ObjectType { return BUILTIN_OBJ }
func (b *Builtin) Inspect() string  { return "<builtin function>" }

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
// This eliminates the need for function wrappers and provides direct access
// Libraries can contain sub-libraries for nested module support (e.g., urllib.parse)
type Library struct {
	name         string
	functions    map[string]*Builtin
	constants    map[string]Object
	subLibraries map[string]*Library
	description  string
	instanceData any        // Instance-specific data for this library
	cachedDict   *Dict      // Cached dict representation (built once)
	cachedDictMu sync.Mutex // Protects cachedDict for concurrent access
}

// NewLibrary creates a new library with functions, optional constants, and optional description
// Pass nil for constants if there are none, and "" for description if not needed
func NewLibrary(name string, functions map[string]*Builtin, constants map[string]Object, description string) *Library {
	return &Library{
		name:         name,
		functions:    functions,
		constants:    constants,
		subLibraries: nil,
		description:  description,
	}
}

// NewLibraryWithSubs creates a new library with functions, constants, sub-libraries, and description
func NewLibraryWithSubs(name string, functions map[string]*Builtin, constants map[string]Object, subLibraries map[string]*Library, description string) *Library {
	return &Library{
		name:         name,
		functions:    functions,
		constants:    constants,
		subLibraries: subLibraries,
		description:  description,
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

// SubLibraries returns the library's sub-libraries map
func (l *Library) SubLibraries() map[string]*Library {
	return l.subLibraries
}

// Description returns the library's description
func (l *Library) Description() string {
	return l.description
}

// GetDict returns the cached Dict representation of this library, building it if necessary
// This caching avoids rebuilding the dict every time a library is imported
func (l *Library) GetDict() *Dict {
	l.cachedDictMu.Lock()
	defer l.cachedDictMu.Unlock()

	if l.cachedDict != nil {
		return l.cachedDict
	}

	// Build dict from library contents
	funcs := l.functions
	consts := l.constants
	subs := l.subLibraries

	dict := make(map[string]DictPair, len(funcs)+len(consts)+len(subs))

	for fname, fn := range funcs {
		dict[DictKey(&String{Value: fname})] = DictPair{
			Key:   &String{Value: fname},
			Value: fn,
		}
	}

	// Add constants
	if consts != nil {
		for cname, val := range consts {
			dict[DictKey(&String{Value: cname})] = DictPair{
				Key:   &String{Value: cname},
				Value: val,
			}
		}
	}

	// Add sub-libraries (recursive)
	if subs != nil {
		for subName, subLib := range subs {
			dict[DictKey(&String{Value: subName})] = DictPair{
				Key:   &String{Value: subName},
				Value: subLib.GetDict(),
			}
		}
	}

	// Add description if available
	if l.description != "" {
		dict[DictKey(&String{Value: "__doc__"})] = DictPair{
			Key:   &String{Value: "__doc__"},
			Value: &String{Value: l.description},
		}
	}

	l.cachedDict = &Dict{Pairs: dict}
	return l.cachedDict
}

// CachedDict returns the cached dict for testing purposes
// This is exported only for library_instantiate_test.go to verify caching behavior
func (l *Library) CachedDict() *Dict {
	return l.cachedDict
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
		constants:    l.constants,    // Constants are shared (immutable)
		subLibraries: l.subLibraries, // Sub-libraries are shared
		description:  l.description,
		instanceData: instanceData,
		cachedDict:   nil, // New instance needs fresh cache
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

type Environment struct {
	mu                         sync.RWMutex
	store                      map[string]Object
	outer                      *Environment
	globals                    map[string]bool
	nonlocals                  map[string]bool
	output                     io.Writer
	input                      io.Reader
	importCallback             func(string) error
	availableLibrariesCallback func() []LibraryInfo
}

// LibraryInfo contains information about available libraries
type LibraryInfo struct {
	Name       string
	IsImported bool
}

func NewEnvironment() *Environment {
	return &Environment{
		store: make(map[string]Object, 4),
		// globals and nonlocals are nil by default - allocated on demand
	}
}

func NewEnclosedEnvironment(outer *Environment) *Environment {
	env := &Environment{
		store: make(map[string]Object, 4),
		outer: outer,
	}

	if outer != nil {
		if outer.output != nil {
			env.output = outer.output
		}
		if outer.input != nil {
			env.input = outer.input
		}
	}

	return env
}

func (e *Environment) Get(name string) (Object, bool) {
	e.mu.RLock()
	obj, ok := e.store[name]
	e.mu.RUnlock()
	if !ok && e.outer != nil {
		obj, ok = e.outer.Get(name)
	}
	return obj, ok
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
	e.mu.Lock()
	e.store[name] = val
	e.mu.Unlock()
	return val
}

// Delete removes a variable from this environment (not parent scopes)
func (e *Environment) Delete(name string) {
	e.mu.Lock()
	delete(e.store, name)
	e.mu.Unlock()
}

// SetGlobal sets a variable in the global (outermost) environment
func (e *Environment) SetGlobal(name string, val Object) Object {
	if e.outer == nil {
		e.mu.Lock()
		e.store[name] = val
		e.mu.Unlock()
		return val
	}
	return e.outer.SetGlobal(name, val)
}

// GetGlobal gets the global (outermost) environment
func (e *Environment) GetGlobal() *Environment {
	if e.outer == nil {
		return e
	}
	return e.outer.GetGlobal()
}

// SetInParent sets a variable in the parent environment (for nonlocal)
func (e *Environment) SetInParent(name string, val Object) bool {
	if e.outer == nil {
		return false
	}
	e.outer.mu.Lock()
	_, ok := e.outer.store[name]
	if ok {
		e.outer.store[name] = val
		e.outer.mu.Unlock()
		return true
	}
	e.outer.mu.Unlock()
	if e.outer.outer != nil {
		return e.outer.SetInParent(name, val)
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

// IsGlobal checks if a variable is marked as global
func (e *Environment) IsGlobal(name string) bool {
	return e.globals != nil && e.globals[name]
}

// IsNonlocal checks if a variable is marked as nonlocal
func (e *Environment) IsNonlocal(name string) bool {
	return e.nonlocals != nil && e.nonlocals[name]
}

// EnableOutputCapture enables output capture for this environment
func (e *Environment) EnableOutputCapture() {
	e.output = &strings.Builder{}
}

// SetOutputWriter sets a custom writer for output
func (e *Environment) SetOutputWriter(w io.Writer) {
	e.output = w
}

// GetOutput returns captured output and clears the buffer
func (e *Environment) GetOutput() string {
	if e.output == nil {
		return ""
	}
	if builder, ok := e.output.(*strings.Builder); ok {
		result := builder.String()
		builder.Reset()
		return result
	}
	return ""
}

// GetWriter returns the appropriate writer for output
func (e *Environment) GetWriter() io.Writer {
	if e.output != nil {
		return e.output
	}
	return os.Stdout
}

// SetInputReader sets a custom reader for input
func (e *Environment) SetInputReader(r io.Reader) {
	e.input = r
}

// GetReader returns the appropriate reader for input
func (e *Environment) GetReader() io.Reader {
	if e.input != nil {
		return e.input
	}
	return os.Stdin
}

// GetStore returns a copy of the environment's store (only local scope, not outer)
func (e *Environment) GetStore() map[string]Object {
	e.mu.RLock()
	store := make(map[string]Object, len(e.store))
	for k, v := range e.store {
		store[k] = v
	}
	e.mu.RUnlock()
	return store
}

// SetImportCallback sets the import callback for this environment.
// GetImportCallback walks up the scope chain, so setting on any env
// makes it available to that env and all enclosed children.
func (e *Environment) SetImportCallback(fn func(string) error) {
	e.importCallback = fn
}

// GetImportCallback gets the import callback from this environment or outer
func (e *Environment) GetImportCallback() func(string) error {
	if e.importCallback != nil {
		return e.importCallback
	}
	if e.outer != nil {
		return e.outer.GetImportCallback()
	}
	return nil
}

// SetAvailableLibrariesCallback sets the available libraries callback for this environment.
// GetAvailableLibrariesCallback walks up the scope chain, so setting on any env
// makes it available to that env and all enclosed children.
func (e *Environment) SetAvailableLibrariesCallback(fn func() []LibraryInfo) {
	e.availableLibrariesCallback = fn
}

// GetAvailableLibrariesCallback gets the available libraries callback from this environment or outer
func (e *Environment) GetAvailableLibrariesCallback() func() []LibraryInfo {
	if e.availableLibrariesCallback != nil {
		return e.availableLibrariesCallback
	}
	if e.outer != nil {
		return e.outer.GetAvailableLibrariesCallback()
	}
	return nil
}

type List struct {
	Elements []Object
}

func (l *List) Type() ObjectType { return LIST_OBJ }
func (l *List) Inspect() string {
	var out strings.Builder
	out.WriteString("[")
	for i, el := range l.Elements {
		if i > 0 {
			out.WriteString(", ")
		}
		out.WriteString(el.Inspect())
	}
	out.WriteString("]")
	return out.String()
}

func (l *List) AsString() (string, Object)          { return "", errMustBeString }
func (l *List) AsInt() (int64, Object)              { return 0, errMustBeInteger }
func (l *List) AsFloat() (float64, Object)          { return 0, errMustBeNumber }
func (l *List) AsBool() (bool, Object)              { return len(l.Elements) > 0, nil }
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

func (t *Tuple) Type() ObjectType { return TUPLE_OBJ }
func (t *Tuple) Inspect() string {
	var out strings.Builder
	out.WriteString("(")
	for i, el := range t.Elements {
		if i > 0 {
			out.WriteString(", ")
		}
		out.WriteString(el.Inspect())
	}
	if len(t.Elements) == 1 {
		out.WriteString(",") // Single element tuple needs trailing comma
	}
	out.WriteString(")")
	return out.String()
}

func (t *Tuple) AsString() (string, Object)          { return "", errMustBeString }
func (t *Tuple) AsInt() (int64, Object)              { return 0, errMustBeInteger }
func (t *Tuple) AsFloat() (float64, Object)          { return 0, errMustBeNumber }
func (t *Tuple) AsBool() (bool, Object)              { return len(t.Elements) > 0, nil }
func (t *Tuple) AsList() ([]Object, Object) {
	result := make([]Object, len(t.Elements))
	copy(result, t.Elements)
	return result, nil
}
func (t *Tuple) AsDict() (map[string]Object, Object) { return nil, errMustBeDict }

func (t *Tuple) CoerceString() (string, Object) { return t.Inspect(), nil }
func (t *Tuple) CoerceInt() (int64, Object)     { return 0, errMustBeInteger }
func (t *Tuple) CoerceFloat() (float64, Object) { return 0, errMustBeNumber }

type Dict struct {
	Pairs map[string]DictPair
}

type DictPair struct {
	Key   Object
	Value Object
}

// StringKey returns the string representation of the key.
// For String keys, returns the actual string value; for other types, returns Inspect().
// This is the canonical way to extract a human-readable key from a DictPair.
func (p DictPair) StringKey() string {
	if s, ok := p.Key.(*String); ok {
		return s.Value
	}
	return p.Key.Inspect()
}

// NewStringDict creates a Dict from string key-value pairs.
// Usage: NewStringDict(map[string]Object{"key": value, ...})
func NewStringDict(entries map[string]Object) *Dict {
	pairs := make(map[string]DictPair, len(entries))
	for k, v := range entries {
		pairs[DictKey(&String{Value: k})] = DictPair{Key: &String{Value: k}, Value: v}
	}
	return &Dict{Pairs: pairs}
}

func (d *Dict) Type() ObjectType { return DICT_OBJ }
func (d *Dict) Inspect() string {
	var out strings.Builder
	out.WriteString("{")
	i := 0
	for _, pair := range d.Pairs {
		if i > 0 {
			out.WriteString(", ")
		}
		out.WriteString(pair.Key.Inspect())
		out.WriteString(": ")
		out.WriteString(pair.Value.Inspect())
		i++
	}
	out.WriteString("}")
	return out.String()
}

func (d *Dict) AsString() (string, Object) { return "", errMustBeString }
func (d *Dict) AsInt() (int64, Object)     { return 0, errMustBeInteger }
func (d *Dict) AsFloat() (float64, Object) { return 0, errMustBeNumber }
func (d *Dict) AsBool() (bool, Object)     { return len(d.Pairs) > 0, nil }
func (d *Dict) AsList() ([]Object, Object) { return nil, errMustBeList }
func (d *Dict) AsDict() (map[string]Object, Object) {
	result := make(map[string]Object)
	for _, pair := range d.Pairs {
		result[pair.Key.Inspect()] = pair.Value
	}
	return result, nil
}

func (d *Dict) CoerceString() (string, Object) { return d.Inspect(), nil }
func (d *Dict) CoerceInt() (int64, Object)     { return 0, errMustBeInteger }
func (d *Dict) CoerceFloat() (float64, Object) { return 0, errMustBeNumber }

// GetPair retrieves a pair from the dict using proper DictKey lookup.
func (d *Dict) GetPair(key Object) (DictPair, bool) {
	pair, ok := d.Pairs[DictKey(key)]
	return pair, ok
}

// SetPair sets a key-value pair in the dict using proper DictKey.
func (d *Dict) SetPair(key, value Object) {
	d.Pairs[DictKey(key)] = DictPair{Key: key, Value: value}
}

// HasKey checks if a key exists in the dict.
func (d *Dict) HasKey(key Object) bool {
	_, ok := d.Pairs[DictKey(key)]
	return ok
}

// DeleteKey removes a key from the dict. Returns true if the key existed.
func (d *Dict) DeleteKey(key Object) bool {
	k := DictKey(key)
	_, ok := d.Pairs[k]
	if ok {
		delete(d.Pairs, k)
	}
	return ok
}

// GetByString retrieves a pair using a string key (convenience for attribute-style access).
func (d *Dict) GetByString(name string) (DictPair, bool) {
	pair, ok := d.Pairs[DictKey(&String{Value: name})]
	return pair, ok
}

// SetByString sets a pair using a string key (convenience for attribute-style access).
func (d *Dict) SetByString(name string, value Object) {
	d.Pairs[DictKey(&String{Value: name})] = DictPair{Key: &String{Value: name}, Value: value}
}

// HasByString checks if a string key exists in the dict.
func (d *Dict) HasByString(name string) bool {
	_, ok := d.Pairs[DictKey(&String{Value: name})]
	return ok
}

// DeleteByString deletes a string key from the dict. Returns true if key existed.
func (d *Dict) DeleteByString(name string) bool {
	k := DictKey(&String{Value: name})
	_, ok := d.Pairs[k]
	if ok {
		delete(d.Pairs, k)
	}
	return ok
}

type Error struct {
	Message  string
	Line     int
	File     string
	Function string
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
}

func (ex *Exception) Type() ObjectType { return EXCEPTION_OBJ }
func (ex *Exception) Inspect() string  { return "EXCEPTION: " + ex.Message }

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

type Instance struct {
	Class  *Class
	Fields map[string]Object
}

func (i *Instance) Type() ObjectType { return INSTANCE_OBJ }
func (i *Instance) Inspect() string {
	// Check for __str_repr__ field (used by libraries to provide custom string representation)
	if strRepr, ok := i.Fields["__str_repr__"]; ok {
		if s, ok := strRepr.(*String); ok {
			return s.Value
		}
	}
	return fmt.Sprintf("<%s object at %p>", i.Class.Name, i)
}

func (i *Instance) AsString() (string, Object)          { return i.Inspect(), nil }
func (i *Instance) AsInt() (int64, Object)              { return 0, errMustBeInteger }
func (i *Instance) AsFloat() (float64, Object)          { return 0, errMustBeNumber }
func (i *Instance) AsBool() (bool, Object)              { return true, nil }
func (i *Instance) AsList() ([]Object, Object)          { return nil, errMustBeList }
func (i *Instance) AsDict() (map[string]Object, Object) { return i.Fields, nil }

func (i *Instance) CoerceString() (string, Object) { return i.Inspect(), nil }
func (i *Instance) CoerceInt() (int64, Object)     { return 0, errMustBeInteger }
func (i *Instance) CoerceFloat() (float64, Object) { return 0, errMustBeNumber }

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
	}
}
