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
	FUNCTION_OBJ  = "FUNCTION"
	BUILTIN_OBJ   = "BUILTIN"
	LIST_OBJ      = "LIST"
	DICT_OBJ      = "DICT"
	HTTP_RESP_OBJ = "HTTP_RESPONSE"
	ERROR_OBJ     = "ERROR"
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

type Function struct {
	Parameters []*ast.Identifier
	Body       *ast.BlockStatement
	Env        *Environment
}

func (f *Function) Type() ObjectType { return FUNCTION_OBJ }
func (f *Function) Inspect() string  { return "<function>" }

type BuiltinFunction func(args ...Object) Object

type Builtin struct {
	Fn BuiltinFunction
}

func (b *Builtin) Type() ObjectType { return BUILTIN_OBJ }
func (b *Builtin) Inspect() string  { return "<builtin function>" }

type Environment struct {
	store map[string]Object
	outer *Environment
}

func NewEnvironment() *Environment {
	return &Environment{store: make(map[string]Object, 16)}
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
	e.store[name] = val
	return val
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
