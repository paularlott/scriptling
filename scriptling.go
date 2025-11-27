package scriptling

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/paularlott/scriptling/ast"
	"github.com/paularlott/scriptling/evaluator"
	"github.com/paularlott/scriptling/internal/cache"
	"github.com/paularlott/scriptling/lexer"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/parser"
	"github.com/paularlott/scriptling/stdlib"
)

type scriptLibrary struct {
	source string
	store  map[string]object.Object
}

type Scriptling struct {
	env                     *object.Environment
	registeredLibraries     map[string]*object.Library
	scriptLibraries         map[string]*scriptLibrary // Script-based libraries
	onDemandLibraryCallback func(*Scriptling, string) bool
}

var availableLibraries = map[string]*object.Library{
	"json":     stdlib.JSONLibrary,
	"re":       stdlib.ReLibrary,
	"time":     stdlib.TimeLibrary,
	"datetime": stdlib.DatetimeLibrary,
	"math":     stdlib.MathLibrary,
	"base64":   stdlib.Base64Library,
	"hashlib":  stdlib.HashlibLibrary,
	"random":   stdlib.RandomLibrary,
	"url":      stdlib.URLLibrary,
}

func New() *Scriptling {
	p := &Scriptling{
		env:                     object.NewEnvironment(),
		registeredLibraries:     make(map[string]*object.Library),
		scriptLibraries:         make(map[string]*scriptLibrary),
		onDemandLibraryCallback: nil,
	}

	// Register import builtin
	p.env.Set("import", evaluator.GetImportBuiltin())
	evaluator.SetImportCallback(func(libName string) error {
		return p.loadLibrary(libName)
	})

	// Register available libraries callback
	evaluator.SetAvailableLibrariesCallback(func() []evaluator.LibraryInfo {
		var libs []evaluator.LibraryInfo
		seen := make(map[string]bool)

		// Helper to check if imported
		isImported := func(name string) bool {
			_, ok := p.env.Get(name)
			return ok
		}

		// Standard libraries
		for name := range availableLibraries {
			if !seen[name] {
				libs = append(libs, evaluator.LibraryInfo{
					Name:       name,
					IsStandard: true,
					IsImported: isImported(name),
				})
				seen[name] = true
			}
		}

		// Registered libraries
		for name := range p.registeredLibraries {
			if !seen[name] {
				libs = append(libs, evaluator.LibraryInfo{
					Name:       name,
					IsStandard: false,
					IsImported: isImported(name),
				})
				seen[name] = true
			}
		}

		// Script libraries
		for name := range p.scriptLibraries {
			if !seen[name] {
				libs = append(libs, evaluator.LibraryInfo{
					Name:       name,
					IsStandard: false,
					IsImported: isImported(name),
				})
				seen[name] = true
			}
		}

		// Sort by name
		sort.Slice(libs, func(i, j int) bool {
			return libs[i].Name < libs[j].Name
		})

		return libs
	})

	return p
}

func (p *Scriptling) loadLibrary(name string) error {
	// Check if library is already imported
	if _, ok := p.env.Get(name); ok {
		return nil // Already imported, skip
	}

	for attempts := 0; attempts < 2; attempts++ {
		// Try from script libraries first
		if lib, ok := p.scriptLibraries[name]; ok {
			if lib.store == nil {
				store, err := p.evaluateScriptLibrary(name, lib.source)
				if err != nil {
					return err
				}
				lib.store = store
			}
			p.registerScriptLibrary(name, lib.store)
			return nil
		}

		// Try from registered libraries
		if lib, ok := p.registeredLibraries[name]; ok {
			p.registerLibrary(name, lib)
			return nil
		}

		// Try standard libraries
		if lib, ok := availableLibraries[name]; ok {
			p.registerLibrary(name, lib)
			return nil
		}

		// If first attempt and callback exists, call it
		if attempts == 0 && p.onDemandLibraryCallback != nil {
			if !p.onDemandLibraryCallback(p, name) {
				break // callback didn't register, stop
			}
			// else continue to second attempt
		} else {
			break
		}
	}

	return fmt.Errorf("unknown library: %s", name)
}

// Register library adds a new library to the script environment
func (p *Scriptling) registerLibrary(name string, lib *object.Library) {
	funcs := lib.Functions()
	consts := lib.Constants()
	dict := make(map[string]object.DictPair, len(funcs)+len(consts))
	for fname, fn := range funcs {
		dict[fname] = object.DictPair{
			Key:   &object.String{Value: fname},
			Value: fn,
		}
	}

	// Add constants
	for cname, val := range consts {
		dict[cname] = object.DictPair{
			Key:   &object.String{Value: cname},
			Value: val,
		}
	}

	// Add description if available
	if desc := lib.Description(); desc != "" {
		dict["__doc__"] = object.DictPair{
			Key:   &object.String{Value: "__doc__"},
			Value: &object.String{Value: desc},
		}
	}

	p.env.Set(name, &object.Dict{Pairs: dict})
}

// Eval executes script without timeout (backwards compatible)
func (p *Scriptling) Eval(input string) (object.Object, error) {
	return p.EvalWithContext(context.Background(), input)
}

// EvalWithTimeout executes script with timeout
func (p *Scriptling) EvalWithTimeout(timeout time.Duration, input string) (object.Object, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return p.EvalWithContext(ctx, input)
}

// EvalWithContext executes script with context for timeout/cancellation
func (p *Scriptling) EvalWithContext(ctx context.Context, input string) (object.Object, error) {
	// Try global cache first
	program, ok := cache.Get(input)
	if !ok {
		l := lexer.New(input)
		par := parser.New(l)
		program = par.ParseProgram()
		if len(par.Errors()) != 0 {
			return nil, fmt.Errorf("parser errors: %v", par.Errors())
		}
		// Store in global cache
		cache.Set(input, program)
	}

	result := evaluator.EvalWithContext(ctx, program, p.env)
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

// Convenience methods for type-safe variable access
func (p *Scriptling) GetVarAsString(name string) (string, bool) {
	obj, ok := p.env.Get(name)
	if !ok {
		return "", false
	}
	return obj.AsString()
}

func (p *Scriptling) GetVarAsInt(name string) (int64, bool) {
	obj, ok := p.env.Get(name)
	if !ok {
		return 0, false
	}
	return obj.AsInt()
}

func (p *Scriptling) GetVarAsFloat(name string) (float64, bool) {
	obj, ok := p.env.Get(name)
	if !ok {
		return 0, false
	}
	return obj.AsFloat()
}

func (p *Scriptling) GetVarAsBool(name string) (bool, bool) {
	obj, ok := p.env.Get(name)
	if !ok {
		return false, false
	}
	return obj.AsBool()
}

func (p *Scriptling) GetVarAsList(name string) ([]object.Object, bool) {
	obj, ok := p.env.Get(name)
	if !ok {
		return nil, false
	}
	return obj.AsList()
}

func (p *Scriptling) GetVarAsDict(name string) (map[string]object.Object, bool) {
	obj, ok := p.env.Get(name)
	if !ok {
		return nil, false
	}
	return obj.AsDict()
}

func (p *Scriptling) RegisterFunc(name string, fn func(ctx context.Context, args ...object.Object) object.Object, helpText ...string) {
	builtin := &object.Builtin{Fn: fn}
	if len(helpText) > 0 && helpText[0] != "" {
		builtin.HelpText = helpText[0]
	} else {
		// Auto-generate basic help
		builtin.HelpText = fmt.Sprintf("%s(...) - User-defined function", name)
	}
	p.env.Set(name, builtin)
}

// RegisterLibrary registers a new library that can be imported by scripts
func (p *Scriptling) RegisterLibrary(name string, lib *object.Library) {
	p.registeredLibraries[name] = lib
}

// RegisterScriptFunc registers a function written in Scriptling
// The script should define a function and this method will extract it and register it by name
func (p *Scriptling) RegisterScriptFunc(name string, script string) error {
	// Evaluate the script to get the function
	result, err := p.Eval(script)
	if err != nil {
		return fmt.Errorf("failed to evaluate script: %w", err)
	}

	// Check if result is a function
	switch fn := result.(type) {
	case *object.Function:
		p.env.Set(name, fn)
	case *object.LambdaFunction:
		p.env.Set(name, fn)
	default:
		return fmt.Errorf("script must evaluate to a function, got %s", result.Type())
	}

	return nil
}

// RegisterScriptLibrary registers a library written in Scriptling
// The script should define functions/values that will be available when the library is imported
func (p *Scriptling) RegisterScriptLibrary(name string, script string) error {
	p.scriptLibraries[name] = &scriptLibrary{
		source: script,
	}
	return nil
}

// SetOnDemandLibraryCallback sets a callback that is called when a library import fails
// The callback receives the Scriptling instance and the library name, and should return true
// if it successfully registered the library using RegisterLibrary or RegisterScriptLibrary
func (p *Scriptling) SetOnDemandLibraryCallback(callback func(*Scriptling, string) bool) {
	p.onDemandLibraryCallback = callback
}

func (p *Scriptling) evaluateScriptLibrary(name string, script string) (map[string]object.Object, error) {
	// Create a new environment for the library
	libEnv := object.NewEnvironment()

	// Inherit writer from main environment (for output capture)
	if w := p.env.GetWriter(); w != nil {
		if buf, ok := w.(*strings.Builder); ok {
			libEnv.SetOutput(buf)
		}
	}

	// Set up import builtin for nested imports
	libEnv.Set("import", evaluator.GetImportBuiltin())

	// Temporarily set up import callback to load into library environment
	oldCallback := evaluator.GetImportCallback()
	evaluator.SetImportCallback(func(libName string) error {
		// Try from script libraries first
		if lib, ok := p.scriptLibraries[libName]; ok {
			// Check if we need to evaluate it first (recursive lazy loading)
			if lib.store == nil {
				store, err := p.evaluateScriptLibrary(libName, lib.source)
				if err != nil {
					return err
				}
				lib.store = store
			}

			// Load script library into library environment
			libDict := make(map[string]object.DictPair, len(lib.store))
			for fname, obj := range lib.store {
				libDict[fname] = object.DictPair{
					Key:   &object.String{Value: fname},
					Value: obj,
				}
			}
			libEnv.Set(libName, &object.Dict{Pairs: libDict})
			return nil
		}

		// Try from registered libraries
		if lib, ok := p.registeredLibraries[libName]; ok {
			// Load Go library into library environment
			goLibDict := make(map[string]object.DictPair)
			for fname, fn := range lib.Functions() {
				goLibDict[fname] = object.DictPair{
					Key:   &object.String{Value: fname},
					Value: fn,
				}
			}
			for cname, val := range lib.Constants() {
				goLibDict[cname] = object.DictPair{
					Key:   &object.String{Value: cname},
					Value: val,
				}
			}
			libEnv.Set(libName, &object.Dict{Pairs: goLibDict})
			return nil
		}

		// Try standard libraries
		if lib, ok := availableLibraries[libName]; ok {
			stdLibDict := make(map[string]object.DictPair)
			for fname, fn := range lib.Functions() {
				stdLibDict[fname] = object.DictPair{
					Key:   &object.String{Value: fname},
					Value: fn,
				}
			}
			for cname, val := range lib.Constants() {
				stdLibDict[cname] = object.DictPair{
					Key:   &object.String{Value: cname},
					Value: val,
				}
			}
			libEnv.Set(libName, &object.Dict{Pairs: stdLibDict})
			return nil
		}

		return fmt.Errorf("unknown library: %s", libName)
	})
	defer evaluator.SetImportCallback(oldCallback)

	// Parse and evaluate the script in the library environment
	var program *ast.Program
	if cached, ok := cache.Get(script); ok {
		program = cached
	} else {
		l := lexer.New(script)
		par := parser.New(l)
		program = par.ParseProgram()
		if len(par.Errors()) != 0 {
			return nil, fmt.Errorf("parser errors: %v", par.Errors())
		}
		cache.Set(script, program)
	}

	// Check for module docstring (first statement is a string literal)
	var moduleDocstring *object.String
	if program != nil && len(program.Statements) > 0 {
		if exprStmt, ok := program.Statements[0].(*ast.ExpressionStatement); ok {
			if strLit, ok := exprStmt.Expression.(*ast.StringLiteral); ok {
				moduleDocstring = &object.String{Value: strLit.Value}
			}
		}
	}

	result := evaluator.Eval(program, libEnv)
	if err, ok := result.(*object.Error); ok {
		return nil, fmt.Errorf("%s", err.Message)
	}

	// Extract all defined names from the library environment
	store := libEnv.GetStore()

	// Filter out the import builtin and any imported libraries
	libStore := make(map[string]object.Object)
	for k, v := range store {
		// Skip import builtin and imported libraries (which are Dicts)
		if k == "import" {
			continue
		}
		// Include everything else (functions, constants, etc.)
		libStore[k] = v
	}

	// Add module docstring if found
	if moduleDocstring != nil {
		libStore["__doc__"] = moduleDocstring
	}

	return libStore, nil
}

// registerScriptLibrary loads a script library into the current environment as a dict
func (p *Scriptling) registerScriptLibrary(name string, store map[string]object.Object) {
	lib := make(map[string]object.DictPair, len(store))
	for fname, obj := range store {
		lib[fname] = object.DictPair{
			Key:   &object.String{Value: fname},
			Value: obj,
		}
	}
	p.env.Set(name, &object.Dict{Pairs: lib})
}

// EnableOutputCapture enables capturing print output instead of sending to stdout
func (p *Scriptling) EnableOutputCapture() {
	p.env.EnableOutputCapture()
}

// GetOutput returns captured output and clears the buffer
func (p *Scriptling) GetOutput() string {
	return p.env.GetOutput()
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
