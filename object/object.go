package object

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/paularlott/scriptling/ast"
)

// Small integer cache for common values (-5 to 256)
// This follows Python's approach and eliminates allocations for loop counters
const (
	smallIntMin = -5
	smallIntMax = 256
)

var smallIntegers [smallIntMax - smallIntMin + 1]*Integer

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
	DATETIME_OBJ
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
	case DATETIME_OBJ:
		return "DATETIME"
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
	default:
		return "UNKNOWN"
	}
}

type Object interface {
	Type() ObjectType
	Inspect() string

	// Type-safe accessor methods
	AsString() (string, bool)
	AsInt() (int64, bool)
	AsFloat() (float64, bool)
	AsBool() (bool, bool)
	AsList() ([]Object, bool)
	AsDict() (map[string]Object, bool)
}

type Integer struct {
	Value int64
}

func (i *Integer) Type() ObjectType { return INTEGER_OBJ }
func (i *Integer) Inspect() string  { return fmt.Sprintf("%d", i.Value) }

func (i *Integer) AsString() (string, bool)          { return "", false }
func (i *Integer) AsInt() (int64, bool)              { return i.Value, true }
func (i *Integer) AsFloat() (float64, bool)          { return float64(i.Value), true }
func (i *Integer) AsBool() (bool, bool)              { return i.Value != 0, true }
func (i *Integer) AsList() ([]Object, bool)          { return nil, false }
func (i *Integer) AsDict() (map[string]Object, bool) { return nil, false }

type Float struct {
	Value float64
}

func (f *Float) Type() ObjectType { return FLOAT_OBJ }
func (f *Float) Inspect() string  { return fmt.Sprintf("%g", f.Value) }

func (f *Float) AsString() (string, bool)          { return "", false }
func (f *Float) AsInt() (int64, bool)              { return 0, false }
func (f *Float) AsFloat() (float64, bool)          { return f.Value, true }
func (f *Float) AsBool() (bool, bool)              { return f.Value != 0, true }
func (f *Float) AsList() ([]Object, bool)          { return nil, false }
func (f *Float) AsDict() (map[string]Object, bool) { return nil, false }

type Datetime struct {
	Value time.Time
}

func (d *Datetime) Type() ObjectType { return DATETIME_OBJ }
func (d *Datetime) Inspect() string  { return d.Value.Format("2006-01-02 15:04:05") }

func (d *Datetime) AsString() (string, bool)          { return d.Inspect(), true }
func (d *Datetime) AsInt() (int64, bool)              { return 0, false }
func (d *Datetime) AsFloat() (float64, bool)          { return 0, false }
func (d *Datetime) AsBool() (bool, bool)              { return true, true }
func (d *Datetime) AsList() ([]Object, bool)          { return nil, false }
func (d *Datetime) AsDict() (map[string]Object, bool) { return nil, false }

type Boolean struct {
	Value bool
}

func (b *Boolean) Type() ObjectType { return BOOLEAN_OBJ }
func (b *Boolean) Inspect() string  { return fmt.Sprintf("%t", b.Value) }

func (b *Boolean) AsString() (string, bool)          { return "", false }
func (b *Boolean) AsInt() (int64, bool)              { return 0, false }
func (b *Boolean) AsFloat() (float64, bool)          { return 0, false }
func (b *Boolean) AsBool() (bool, bool)              { return b.Value, true }
func (b *Boolean) AsList() ([]Object, bool)          { return nil, false }
func (b *Boolean) AsDict() (map[string]Object, bool) { return nil, false }

type String struct {
	Value string
}

func (s *String) Type() ObjectType { return STRING_OBJ }
func (s *String) Inspect() string  { return s.Value }

func (s *String) AsString() (string, bool)          { return s.Value, true }
func (s *String) AsInt() (int64, bool)              { return 0, false }
func (s *String) AsFloat() (float64, bool)          { return 0, false }
func (s *String) AsBool() (bool, bool)              { return s.Value != "", true }
func (s *String) AsList() ([]Object, bool)          { return nil, false }
func (s *String) AsDict() (map[string]Object, bool) { return nil, false }

type Null struct{}

func (n *Null) Type() ObjectType { return NULL_OBJ }
func (n *Null) Inspect() string  { return "None" }

func (n *Null) AsString() (string, bool)          { return "", false }
func (n *Null) AsInt() (int64, bool)              { return 0, false }
func (n *Null) AsFloat() (float64, bool)          { return 0, false }
func (n *Null) AsBool() (bool, bool)              { return false, true }
func (n *Null) AsList() ([]Object, bool)          { return nil, false }
func (n *Null) AsDict() (map[string]Object, bool) { return nil, false }

type ReturnValue struct {
	Value Object
}

func (rv *ReturnValue) Type() ObjectType { return RETURN_OBJ }
func (rv *ReturnValue) Inspect() string  { return rv.Value.Inspect() }

func (rv *ReturnValue) AsString() (string, bool)          { return "", false }
func (rv *ReturnValue) AsInt() (int64, bool)              { return 0, false }
func (rv *ReturnValue) AsFloat() (float64, bool)          { return 0, false }
func (rv *ReturnValue) AsBool() (bool, bool)              { return false, false }
func (rv *ReturnValue) AsList() ([]Object, bool)          { return nil, false }
func (rv *ReturnValue) AsDict() (map[string]Object, bool) { return nil, false }

type Break struct{}

func (b *Break) Type() ObjectType { return BREAK_OBJ }
func (b *Break) Inspect() string  { return "break" }

func (b *Break) AsString() (string, bool)          { return "", false }
func (b *Break) AsInt() (int64, bool)              { return 0, false }
func (b *Break) AsFloat() (float64, bool)          { return 0, false }
func (b *Break) AsBool() (bool, bool)              { return false, false }
func (b *Break) AsList() ([]Object, bool)          { return nil, false }
func (b *Break) AsDict() (map[string]Object, bool) { return nil, false }

type Continue struct{}

func (c *Continue) Type() ObjectType { return CONTINUE_OBJ }
func (c *Continue) Inspect() string  { return "continue" }

func (c *Continue) AsString() (string, bool)          { return "", false }
func (c *Continue) AsInt() (int64, bool)              { return 0, false }
func (c *Continue) AsFloat() (float64, bool)          { return 0, false }
func (c *Continue) AsBool() (bool, bool)              { return false, false }
func (c *Continue) AsList() ([]Object, bool)          { return nil, false }
func (c *Continue) AsDict() (map[string]Object, bool) { return nil, false }

type Function struct {
	Name          string
	Parameters    []*ast.Identifier
	DefaultValues map[string]ast.Expression
	Variadic      *ast.Identifier // *args parameter
	Body          *ast.BlockStatement
	Env           *Environment
}

func (f *Function) Type() ObjectType { return FUNCTION_OBJ }
func (f *Function) Inspect() string  { return "<function>" }

func (f *Function) AsString() (string, bool)          { return "", false }
func (f *Function) AsInt() (int64, bool)              { return 0, false }
func (f *Function) AsFloat() (float64, bool)          { return 0, false }
func (f *Function) AsBool() (bool, bool)              { return false, false }
func (f *Function) AsList() ([]Object, bool)          { return nil, false }
func (f *Function) AsDict() (map[string]Object, bool) { return nil, false }

type LambdaFunction struct {
	Parameters    []*ast.Identifier
	DefaultValues map[string]ast.Expression
	Variadic      *ast.Identifier // *args parameter
	Body          ast.Expression
	Env           *Environment
}

func (lf *LambdaFunction) Type() ObjectType { return LAMBDA_OBJ }
func (lf *LambdaFunction) Inspect() string  { return "<lambda>" }

func (lf *LambdaFunction) AsString() (string, bool)          { return "", false }
func (lf *LambdaFunction) AsInt() (int64, bool)              { return 0, false }
func (lf *LambdaFunction) AsFloat() (float64, bool)          { return 0, false }
func (lf *LambdaFunction) AsBool() (bool, bool)              { return false, false }
func (lf *LambdaFunction) AsList() ([]Object, bool)          { return nil, false }
func (lf *LambdaFunction) AsDict() (map[string]Object, bool) { return nil, false }

// BuiltinFunction is the signature for all builtin functions
// - ctx: Context with environment and runtime information
// - kwargs: Keyword arguments passed to the function (may be nil or empty)
// - args: Positional arguments passed to the function
type BuiltinFunction func(ctx context.Context, kwargs map[string]Object, args ...Object) Object

type Builtin struct {
	Fn         BuiltinFunction
	HelpText   string            // Optional help documentation for this builtin
	Attributes map[string]Object // Optional attributes for this builtin
}

func (b *Builtin) Type() ObjectType { return BUILTIN_OBJ }
func (b *Builtin) Inspect() string  { return "<builtin function>" }

func (b *Builtin) AsString() (string, bool)          { return "", false }
func (b *Builtin) AsInt() (int64, bool)              { return 0, false }
func (b *Builtin) AsFloat() (float64, bool)          { return 0, false }
func (b *Builtin) AsBool() (bool, bool)              { return false, false }
func (b *Builtin) AsList() ([]Object, bool)          { return nil, false }
func (b *Builtin) AsDict() (map[string]Object, bool) { return b.Attributes, b.Attributes != nil }

// Library represents a pre-built collection of builtin functions and constants
// This eliminates the need for function wrappers and provides direct access
// Libraries can contain sub-libraries for nested module support (e.g., urllib.parse)
type Library struct {
	functions    map[string]*Builtin
	constants    map[string]Object
	subLibraries map[string]*Library
	description  string
}

// NewLibrary creates a new library with functions, optional constants, and optional description
// Pass nil for constants if there are none, and "" for description if not needed
func NewLibrary(functions map[string]*Builtin, constants map[string]Object, description string) *Library {
	return &Library{
		functions:    functions,
		constants:    constants,
		subLibraries: nil,
		description:  description,
	}
}

// NewLibraryWithSubs creates a new library with functions, constants, sub-libraries, and description
func NewLibraryWithSubs(functions map[string]*Builtin, constants map[string]Object, subLibraries map[string]*Library, description string) *Library {
	return &Library{
		functions:    functions,
		constants:    constants,
		subLibraries: subLibraries,
		description:  description,
	}
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

func (l *Library) Type() ObjectType { return BUILTIN_OBJ } // Libraries are like builtin objects
func (l *Library) Inspect() string  { return "<library>" }

func (l *Library) AsString() (string, bool)          { return "", false }
func (l *Library) AsInt() (int64, bool)              { return 0, false }
func (l *Library) AsFloat() (float64, bool)          { return 0, false }
func (l *Library) AsBool() (bool, bool)              { return false, false }
func (l *Library) AsList() ([]Object, bool)          { return nil, false }
func (l *Library) AsDict() (map[string]Object, bool) { return nil, false }

type Environment struct {
	store                      map[string]Object
	outer                      *Environment
	globals                    map[string]bool
	nonlocals                  map[string]bool
	output                     *strings.Builder
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
		store:     make(map[string]Object, 16),
		globals:   make(map[string]bool),
		nonlocals: make(map[string]bool),
	}
}

func NewEnclosedEnvironment(outer *Environment) *Environment {
	env := NewEnvironment()
	env.outer = outer
	return env
}

func (e *Environment) Get(name string) (Object, bool) {
	obj, ok := e.store[name]
	if !ok && e.outer != nil {
		obj, ok = e.outer.Get(name)
	}
	return obj, ok
}

func (e *Environment) Set(name string, val Object) Object {
	// Check if this variable is marked as global
	if e.globals[name] {
		return e.SetGlobal(name, val)
	}
	// Check if this variable is marked as nonlocal
	if e.nonlocals[name] {
		if e.SetInParent(name, val) {
			return val
		}
	}
	e.store[name] = val
	return val
}

// SetGlobal sets a variable in the global (outermost) environment
func (e *Environment) SetGlobal(name string, val Object) Object {
	if e.outer == nil {
		e.store[name] = val
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
	if _, ok := e.outer.store[name]; ok {
		e.outer.store[name] = val
		return true
	}
	if e.outer.outer != nil {
		return e.outer.SetInParent(name, val)
	}
	return false
}

// MarkGlobal marks a variable name as global in this scope
func (e *Environment) MarkGlobal(name string) {
	e.globals[name] = true
}

// MarkNonlocal marks a variable name as nonlocal in this scope
func (e *Environment) MarkNonlocal(name string) {
	e.nonlocals[name] = true
}

// IsGlobal checks if a variable is marked as global
func (e *Environment) IsGlobal(name string) bool {
	return e.globals[name]
}

// IsNonlocal checks if a variable is marked as nonlocal
func (e *Environment) IsNonlocal(name string) bool {
	return e.nonlocals[name]
}

// EnableOutputCapture enables output capture for this environment
func (e *Environment) EnableOutputCapture() {
	e.output = &strings.Builder{}
}

// SetOutput sets the output buffer for this environment
func (e *Environment) SetOutput(output *strings.Builder) {
	e.output = output
}

// GetOutput returns captured output and clears the buffer
func (e *Environment) GetOutput() string {
	if e.output == nil {
		return ""
	}
	result := e.output.String()
	e.output.Reset()
	return result
}

// GetWriter returns the appropriate writer for output
func (e *Environment) GetWriter() io.Writer {
	if e.output != nil {
		return e.output
	}
	return os.Stdout
}

// GetStore returns a copy of the environment's store (only local scope, not outer)
func (e *Environment) GetStore() map[string]Object {
	store := make(map[string]Object, len(e.store))
	for k, v := range e.store {
		store[k] = v
	}
	return store
}

// SetImportCallback sets the import callback for this environment
func (e *Environment) SetImportCallback(fn func(string) error) {
	e.importCallback = fn
	// Propagate to outer environments
	if e.outer != nil {
		e.outer.SetImportCallback(fn)
	}
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

// SetAvailableLibrariesCallback sets the available libraries callback for this environment
func (e *Environment) SetAvailableLibrariesCallback(fn func() []LibraryInfo) {
	e.availableLibrariesCallback = fn
	// Propagate to outer environments
	if e.outer != nil {
		e.outer.SetAvailableLibrariesCallback(fn)
	}
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

// Clone creates a deep copy of the environment for thread safety
func (e *Environment) Clone() *Environment {
	cloned := &Environment{
		store:     make(map[string]Object, len(e.store)),
		globals:   make(map[string]bool, len(e.globals)),
		nonlocals: make(map[string]bool, len(e.nonlocals)),
	}

	// Deep copy store, but avoid circular references with functions
	for k, v := range e.store {
		// Don't deep copy functions - they reference environments which would cause infinite recursion
		// Functions are safe to share across goroutines
		switch v.(type) {
		case *Function, *LambdaFunction, *Builtin, *Class, *Library:
			cloned.store[k] = v // Share these types
		default:
			cloned.store[k] = DeepCopy(v) // Deep copy data
		}
	}

	// Copy globals and nonlocals maps
	for k, v := range e.globals {
		cloned.globals[k] = v
	}
	for k, v := range e.nonlocals {
		cloned.nonlocals[k] = v
	}

	// Copy callbacks
	cloned.importCallback = e.importCallback
	cloned.availableLibrariesCallback = e.availableLibrariesCallback

	// Don't copy outer - this is a new root environment
	// Don't copy output - each thread gets its own

	return cloned
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

func (l *List) AsString() (string, bool)          { return "", false }
func (l *List) AsInt() (int64, bool)              { return 0, false }
func (l *List) AsFloat() (float64, bool)          { return 0, false }
func (l *List) AsBool() (bool, bool)              { return len(l.Elements) > 0, true }
func (l *List) AsList() ([]Object, bool)          { return l.Elements, true }
func (l *List) AsDict() (map[string]Object, bool) { return nil, false }

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

func (t *Tuple) AsString() (string, bool)          { return "", false }
func (t *Tuple) AsInt() (int64, bool)              { return 0, false }
func (t *Tuple) AsFloat() (float64, bool)          { return 0, false }
func (t *Tuple) AsBool() (bool, bool)              { return len(t.Elements) > 0, true }
func (t *Tuple) AsList() ([]Object, bool)          { return t.Elements, true }
func (t *Tuple) AsDict() (map[string]Object, bool) { return nil, false }

type Dict struct {
	Pairs map[string]DictPair
}

type DictPair struct {
	Key   Object
	Value Object
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

func (d *Dict) AsString() (string, bool) { return "", false }
func (d *Dict) AsInt() (int64, bool)     { return 0, false }
func (d *Dict) AsFloat() (float64, bool) { return 0, false }
func (d *Dict) AsBool() (bool, bool)     { return len(d.Pairs) > 0, true }
func (d *Dict) AsList() ([]Object, bool) { return nil, false }
func (d *Dict) AsDict() (map[string]Object, bool) {
	result := make(map[string]Object)
	for key, pair := range d.Pairs {
		result[key] = pair.Value
	}
	return result, true
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

func (e *Error) AsString() (string, bool)          { return e.Message, true }
func (e *Error) AsInt() (int64, bool)              { return 0, false }
func (e *Error) AsFloat() (float64, bool)          { return 0, false }
func (e *Error) AsBool() (bool, bool)              { return false, true }
func (e *Error) AsList() ([]Object, bool)          { return nil, false }
func (e *Error) AsDict() (map[string]Object, bool) { return nil, false }

type Exception struct {
	Message string
}

func (ex *Exception) Type() ObjectType { return EXCEPTION_OBJ }
func (ex *Exception) Inspect() string  { return "EXCEPTION: " + ex.Message }

func (ex *Exception) AsString() (string, bool)          { return ex.Message, true }
func (ex *Exception) AsInt() (int64, bool)              { return 0, false }
func (ex *Exception) AsFloat() (float64, bool)          { return 0, false }
func (ex *Exception) AsBool() (bool, bool)              { return false, true }
func (ex *Exception) AsList() ([]Object, bool)          { return nil, false }
func (ex *Exception) AsDict() (map[string]Object, bool) { return nil, false }

type Class struct {
	Name      string
	BaseClass *Class // optional parent class for inheritance
	Methods   map[string]Object
	Env       *Environment
}

func (c *Class) Type() ObjectType { return CLASS_OBJ }
func (c *Class) Inspect() string  { return fmt.Sprintf("<class '%s'>", c.Name) }

func (c *Class) AsString() (string, bool)          { return c.Name, true }
func (c *Class) AsInt() (int64, bool)              { return 0, false }
func (c *Class) AsFloat() (float64, bool)          { return 0, false }
func (c *Class) AsBool() (bool, bool)              { return true, true }
func (c *Class) AsList() ([]Object, bool)          { return nil, false }
func (c *Class) AsDict() (map[string]Object, bool) { return c.Methods, true }

type Instance struct {
	Class  *Class
	Fields map[string]Object
}

func (i *Instance) Type() ObjectType { return INSTANCE_OBJ }
func (i *Instance) Inspect() string {
	return fmt.Sprintf("<%s object at %p>", i.Class.Name, i)
}

func (i *Instance) AsString() (string, bool)          { return i.Inspect(), true }
func (i *Instance) AsInt() (int64, bool)              { return 0, false }
func (i *Instance) AsFloat() (float64, bool)          { return 0, false }
func (i *Instance) AsBool() (bool, bool)              { return true, true }
func (i *Instance) AsList() ([]Object, bool)          { return nil, false }
func (i *Instance) AsDict() (map[string]Object, bool) { return i.Fields, true }

type Super struct {
	Class    *Class
	Instance *Instance
}

func (s *Super) Type() ObjectType { return SUPER_OBJ }
func (s *Super) Inspect() string {
	return fmt.Sprintf("<super: <class '%s'>, <%s object>>", s.Class.Name, s.Instance.Class.Name)
}

func (s *Super) AsString() (string, bool)          { return s.Inspect(), true }
func (s *Super) AsInt() (int64, bool)              { return 0, false }
func (s *Super) AsFloat() (float64, bool)          { return 0, false }
func (s *Super) AsBool() (bool, bool)              { return true, true }
func (s *Super) AsList() ([]Object, bool)          { return nil, false }
func (s *Super) AsDict() (map[string]Object, bool) { return nil, false }

// LibraryRegistrar is an interface for registering libraries.
// This allows external libraries to register themselves without circular imports.
type LibraryRegistrar interface {
	RegisterLibrary(name string, lib *Library)
}
