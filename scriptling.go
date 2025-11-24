package scriptling

import (
	"fmt"
	"github.com/paularlott/scriptling/evaluator"
	"github.com/paularlott/scriptling/lexer"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/parser"
	"github.com/paularlott/scriptling/stdlib"
)

type Scriptling struct {
	env *object.Environment
}

var availableLibraries = map[string]func() map[string]*object.Builtin{
	"json":    stdlib.JSONLibrary,
	"re":      stdlib.ReLibrary,
	"time":    stdlib.GetTimeLibrary,
	"math":    stdlib.GetMathLibrary,
	"base64":  stdlib.GetBase64Library,
	"hashlib": stdlib.GetHashlibLibrary,
	"random":  stdlib.GetRandomLibrary,
	"url":     stdlib.GetURLLibrary,
}

func New() *Scriptling {
	p := &Scriptling{
		env: object.NewEnvironment(),
	}
	
	// Register import builtin
	p.env.Set("import", evaluator.GetImportBuiltin())
	evaluator.SetImportCallback(func(libName string) error {
		return p.loadLibrary(libName)
	})
	
	return p
}

func (p *Scriptling) loadLibrary(name string) error {
	if libFunc, ok := availableLibraries[name]; ok {
		p.RegisterLibrary(name, libFunc())
		return nil
	}
	return fmt.Errorf("unknown library: %s", name)
}

func (p *Scriptling) Eval(input string) (object.Object, error) {
	// Try global cache first
	program, ok := globalCache.get(input)
	if !ok {
		l := lexer.New(input)
		par := parser.New(l)
		program = par.ParseProgram()
		if len(par.Errors()) != 0 {
			return nil, fmt.Errorf("parser errors: %v", par.Errors())
		}
		// Store in global cache
		globalCache.set(input, program)
	}

	result := evaluator.Eval(program, p.env)
	if err, ok := result.(*object.Error); ok {
		return nil, fmt.Errorf("%s", err.Message)
	}

	return result, nil
}

func (p *Scriptling) SetVar(name string, value interface{}) error {
	obj := goToObject(value)
	if obj == nil {
		return fmt.Errorf("unsupported type: %T", value)
	}
	p.env.Set(name, obj)
	return nil
}

func (p *Scriptling) GetVar(name string) (interface{}, bool) {
	obj, ok := p.env.Get(name)
	if !ok {
		return nil, false
	}
	return objectToGo(obj), true
}

func (p *Scriptling) RegisterFunc(name string, fn func(args ...object.Object) object.Object) {
	p.env.Set(name, &object.Builtin{Fn: fn})
}

func (p *Scriptling) RegisterLibrary(name string, funcs map[string]*object.Builtin) {
	lib := make(map[string]object.DictPair, len(funcs))
	for fname, fn := range funcs {
		lib[fname] = object.DictPair{
			Key:   &object.String{Value: fname},
			Value: fn,
		}
	}
	p.env.Set(name, &object.Dict{Pairs: lib})
}



func goToObject(value interface{}) object.Object {
	switch v := value.(type) {
	case int:
		return &object.Integer{Value: int64(v)}
	case int64:
		return &object.Integer{Value: v}
	case float64:
		return &object.Float{Value: v}
	case float32:
		return &object.Float{Value: float64(v)}
	case string:
		return &object.String{Value: v}
	case bool:
		if v {
			return &object.Boolean{Value: true}
		}
		return &object.Boolean{Value: false}
	case nil:
		return &object.Null{}
	default:
		return nil
	}
}

func objectToGo(obj object.Object) interface{} {
	switch obj := obj.(type) {
	case *object.Integer:
		return obj.Value
	case *object.Float:
		return obj.Value
	case *object.String:
		return obj.Value
	case *object.Boolean:
		return obj.Value
	case *object.Null:
		return nil
	default:
		return obj.Inspect()
	}
}
