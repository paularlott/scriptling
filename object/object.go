package object

import (
	"fmt"
	"github.com/paularlott/scriptling/ast"
)

type ObjectType string

const (
	INTEGER_OBJ   = "INTEGER"
	FLOAT_OBJ     = "FLOAT"
	BOOLEAN_OBJ   = "BOOLEAN"
	STRING_OBJ    = "STRING"
	NULL_OBJ      = "NULL"
	RETURN_OBJ    = "RETURN"
	BREAK_OBJ     = "BREAK"
	CONTINUE_OBJ  = "CONTINUE"
	FUNCTION_OBJ  = "FUNCTION"
	LAMBDA_OBJ    = "LAMBDA"
	BUILTIN_OBJ   = "BUILTIN"
	LIST_OBJ      = "LIST"
	TUPLE_OBJ     = "TUPLE"
	DICT_OBJ      = "DICT"
	HTTP_RESP_OBJ = "HTTP_RESPONSE"
	ERROR_OBJ     = "ERROR"
	EXCEPTION_OBJ = "EXCEPTION"
)

type Object interface {
	Type() ObjectType
	Inspect() string
}

type Integer struct {
	Value int64
}

func (i *Integer) Type() ObjectType { return INTEGER_OBJ }
func (i *Integer) Inspect() string  { return fmt.Sprintf("%d", i.Value) }

type Float struct {
	Value float64
}

func (f *Float) Type() ObjectType { return FLOAT_OBJ }
func (f *Float) Inspect() string  { return fmt.Sprintf("%g", f.Value) }

type Boolean struct {
	Value bool
}

func (b *Boolean) Type() ObjectType { return BOOLEAN_OBJ }
func (b *Boolean) Inspect() string  { return fmt.Sprintf("%t", b.Value) }

type String struct {
	Value string
}

func (s *String) Type() ObjectType { return STRING_OBJ }
func (s *String) Inspect() string  { return s.Value }

type Null struct{}

func (n *Null) Type() ObjectType { return NULL_OBJ }
func (n *Null) Inspect() string  { return "None" }

type ReturnValue struct {
	Value Object
}

func (rv *ReturnValue) Type() ObjectType { return RETURN_OBJ }
func (rv *ReturnValue) Inspect() string  { return rv.Value.Inspect() }

type Break struct{}

func (b *Break) Type() ObjectType { return BREAK_OBJ }
func (b *Break) Inspect() string  { return "break" }

type Continue struct{}

func (c *Continue) Type() ObjectType { return CONTINUE_OBJ }
func (c *Continue) Inspect() string  { return "continue" }

type Function struct {
	Parameters    []*ast.Identifier
	DefaultValues map[string]ast.Expression
	Body          *ast.BlockStatement
	Env           *Environment
}

func (f *Function) Type() ObjectType { return FUNCTION_OBJ }
func (f *Function) Inspect() string  { return "<function>" }

type LambdaFunction struct {
	Parameters    []*ast.Identifier
	DefaultValues map[string]ast.Expression
	Body          ast.Expression
	Env           *Environment
}

func (lf *LambdaFunction) Type() ObjectType { return LAMBDA_OBJ }
func (lf *LambdaFunction) Inspect() string  { return "<lambda>" }

type BuiltinFunction func(args ...Object) Object

type Builtin struct {
	Fn BuiltinFunction
}

func (b *Builtin) Type() ObjectType { return BUILTIN_OBJ }
func (b *Builtin) Inspect() string  { return "<builtin function>" }

type Environment struct {
	store     map[string]Object
	outer     *Environment
	globals   map[string]bool
	nonlocals map[string]bool
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

type List struct {
	Elements []Object
}

func (l *List) Type() ObjectType { return LIST_OBJ }
func (l *List) Inspect() string {
	var out string
	out += "["
	for i, el := range l.Elements {
		if i > 0 {
			out += ", "
		}
		out += el.Inspect()
	}
	out += "]"
	return out
}

type Tuple struct {
	Elements []Object
}

func (t *Tuple) Type() ObjectType { return TUPLE_OBJ }
func (t *Tuple) Inspect() string {
	var out string
	out += "("
	for i, el := range t.Elements {
		if i > 0 {
			out += ", "
		}
		out += el.Inspect()
	}
	if len(t.Elements) == 1 {
		out += "," // Single element tuple needs trailing comma
	}
	out += ")"
	return out
}

type Dict struct {
	Pairs map[string]DictPair
}

type DictPair struct {
	Key   Object
	Value Object
}

func (d *Dict) Type() ObjectType { return DICT_OBJ }
func (d *Dict) Inspect() string {
	var out string
	out += "{"
	i := 0
	for _, pair := range d.Pairs {
		if i > 0 {
			out += ", "
		}
		out += pair.Key.Inspect() + ": " + pair.Value.Inspect()
		i++
	}
	out += "}"
	return out
}

type HttpResponse struct {
	StatusCode int
	Body       string
	Headers    map[string]string
}

func (h *HttpResponse) Type() ObjectType { return HTTP_RESP_OBJ }
func (h *HttpResponse) Inspect() string  { return h.Body }

type Error struct {
	Message string
}

func (e *Error) Type() ObjectType { return ERROR_OBJ }
func (e *Error) Inspect() string  { return "ERROR: " + e.Message }

type Exception struct {
	Message string
}

func (ex *Exception) Type() ObjectType { return EXCEPTION_OBJ }
func (ex *Exception) Inspect() string  { return "EXCEPTION: " + ex.Message }
