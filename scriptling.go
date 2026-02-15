package scriptling

import (
	"context"
	"fmt"
	"io"
	"runtime/debug"
	"sort"
	"strings"
	"time"

	"github.com/paularlott/scriptling/ast"
	"github.com/paularlott/scriptling/conversion"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/evaluator"
	"github.com/paularlott/scriptling/lexer"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/parser"
)

// Kwargs is a wrapper type to explicitly pass keyword arguments to CallFunction.
// Use this to distinguish between a map being passed as a dict argument vs kwargs.
type Kwargs map[string]interface{}

// convertArgsAndKwargs converts Go arguments to Object arguments and separates kwargs.
func convertArgsAndKwargs(args []interface{}, prependSelf object.Object) ([]object.Object, map[string]object.Object) {
	var objArgs []object.Object
	var objKwargs map[string]object.Object

	if len(args) > 0 {
		lastIdx := len(args) - 1
		if kwargsMap, ok := args[lastIdx].(Kwargs); ok {
			objKwargs = make(map[string]object.Object, len(kwargsMap))
			for key, val := range kwargsMap {
				objKwargs[key] = conversion.FromGo(val)
			}
			if prependSelf != nil {
				objArgs = make([]object.Object, lastIdx+1)
				objArgs[0] = prependSelf
				for i, arg := range args[:lastIdx] {
					objArgs[i+1] = conversion.FromGo(arg)
				}
			} else {
				objArgs = make([]object.Object, lastIdx)
				for i, arg := range args[:lastIdx] {
					objArgs[i] = conversion.FromGo(arg)
				}
			}
		} else {
			if prependSelf != nil {
				objArgs = make([]object.Object, len(args)+1)
				objArgs[0] = prependSelf
				for i, arg := range args {
					objArgs[i+1] = conversion.FromGo(arg)
				}
			} else {
				objArgs = make([]object.Object, len(args))
				for i, arg := range args {
					objArgs[i] = conversion.FromGo(arg)
				}
			}
		}
	} else if prependSelf != nil {
		objArgs = []object.Object{prependSelf}
	}

	return objArgs, objKwargs
}

type scriptLibrary struct {
	source string
	store  map[string]object.Object
}

const (
	maxLibraryNestingDepth = 5  // Max depth for library imports (e.g., a.b.c.d.e)
	maxDottedPathDepth     = 10 // Max depth for dotted paths (e.g., a.b.c.d.e.f.g.h.i.j)
)

type Scriptling struct {
	env                     *object.Environment
	registeredLibraries     map[string]*object.Library
	scriptLibraries         map[string]*scriptLibrary // Script-based libraries
	onDemandLibraryCallback func(*Scriptling, string) bool
	sourceFile              string // optional source file name for error reporting
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

	// Set import callback on environment
	p.env.SetImportCallback(func(libName string) error {
		return p.loadLibrary(libName)
	})

	// Set available libraries callback on environment
	p.env.SetAvailableLibrariesCallback(func() []object.LibraryInfo {
		var libs []object.LibraryInfo
		seen := make(map[string]bool)

		// Helper to check if imported
		isImported := func(name string) bool {
			_, ok := p.env.Get(name)
			return ok
		}

		// Registered libraries (includes selected standard libraries)
		for name := range p.registeredLibraries {
			if !seen[name] {
				libs = append(libs, object.LibraryInfo{
					Name:       name,
					IsImported: isImported(name),
				})
				seen[name] = true
			}
		}

		// Script libraries
		for name := range p.scriptLibraries {
			if !seen[name] {
				libs = append(libs, object.LibraryInfo{
					Name:       name,
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

// splitPath splits a dotted path into parts
func splitPath(path string) []string {
	return strings.Split(path, ".")
}

// traverseDictPath navigates a dotted path through Dict objects
func traverseDictPath(root object.Object, parts []string, maxDepth int) (object.Object, error) {
	if len(parts) > maxDepth {
		return nil, fmt.Errorf("path too deep (max %d levels): %s", maxDepth, strings.Join(parts, "."))
	}

	current := root
	for i, part := range parts {
		dict, ok := current.(*object.Dict)
		if !ok {
			return nil, fmt.Errorf("'%s' is not a module", strings.Join(parts[:i+1], "."))
		}
		pair, exists := dict.GetByString(part)
		if !exists {
			return nil, fmt.Errorf("'%s' not found", strings.Join(parts[:i+1], "."))
		}
		current = pair.Value
	}
	return current, nil
}

// needsParentMerge checks if an existing dict only has sub-libraries and needs parent functions merged
func (p *Scriptling) needsParentMerge(name string, existingDict *object.Dict) bool {
	lib, ok := p.registeredLibraries[name]
	if !ok {
		return false
	}

	funcs := lib.Functions()
	if funcs == nil {
		return false
	}

	// Check if dict has any library functions (not just sub-libraries)
	for _, pair := range existingDict.Pairs {
		keyStr := pair.StringKey()
		if keyStr != "__doc__" && funcs[keyStr] != nil {
			return false // Already has functions
		}
	}
	return true // Only has sub-libraries, needs merge
}

func (p *Scriptling) loadLibrary(name string) error {
	return p.loadLibraryWithDepth(name, 0)
}

func (p *Scriptling) loadLibraryWithDepth(name string, depth int) error {
	parts := splitPath(name)

	if len(parts)-1 > maxLibraryNestingDepth {
		return fmt.Errorf("library nesting too deep (max %d levels): %s", maxLibraryNestingDepth, name)
	}

	if depth > maxLibraryNestingDepth {
		return fmt.Errorf("library nesting too deep (max %d levels): %s", maxLibraryNestingDepth, name)
	}

	// Lazy parent loading: only load parent if this library actually needs it
	// Check if library exists first before loading parent
	if len(parts) > 1 {
		// Check if we have this library registered
		_, hasScript := p.scriptLibraries[name]
		_, hasRegistered := p.registeredLibraries[name]

		if hasScript || hasRegistered {
			// Library exists, check if parent is needed
			parentName := strings.Join(parts[:len(parts)-1], ".")
			if _, ok := p.env.Get(parentName); !ok {
				// Parent doesn't exist, try to load it
				if err := p.loadLibraryWithDepth(parentName, depth+1); err != nil {
					// Parent doesn't exist as a library, that's ok - we'll create the structure
				}
			}
		}
	}

	// Check if library is already imported
	if existingObj, ok := p.env.Get(name); ok {
		// For simple library names, check if it needs parent merge
		if len(parts) == 1 {
			if existingDict, ok := existingObj.(*object.Dict); ok {
				if p.needsParentMerge(name, existingDict) {
					// Merge parent library functions
					lib := p.registeredLibraries[name]
					libDict := p.libraryToDict(lib)
					for k, v := range libDict.Pairs {
						if _, exists := existingDict.Pairs[k]; !exists {
							existingDict.Pairs[k] = v
						}
					}
					return nil
				}
			}
		}
		return nil // Already imported, skip
	}

	// For dotted names like urllib.parse, check if parent exists with the sub-library
	if len(parts) > 1 {
		if parentObj, ok := p.env.Get(parts[0]); ok {
			if result, err := traverseDictPath(parentObj, parts[1:], maxLibraryNestingDepth); err == nil {
				// Create an alias for the full path
				p.env.Set(name, result)
				return nil
			}
		}
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

// registerLibrary adds a library to the script environment
// Supports nested paths like "urllib.parse" - will create parent dicts as needed
func (p *Scriptling) registerLibrary(name string, lib *object.Library) {
	// Convert library to dict (using cached version)
	libDict := lib.GetDict()

	// Check if this is a dotted path
	parts := strings.Split(name, ".")
	if len(parts) == 1 {
		// Simple case - just set directly
		p.env.Set(name, libDict)
		return
	}

	// Nested case - need to create/update parent dicts
	// First, get or create the root dict
	rootName := parts[0]
	var rootDict *object.Dict
	if existing, ok := p.env.Get(rootName); ok {
		if d, ok := existing.(*object.Dict); ok {
			rootDict = d
		} else {
			// Exists but not a dict - replace with new dict
			rootDict = &object.Dict{Pairs: make(map[string]object.DictPair)}
			p.env.Set(rootName, rootDict)
		}
	} else {
		rootDict = &object.Dict{Pairs: make(map[string]object.DictPair)}
		p.env.Set(rootName, rootDict)
	}

	// Navigate/create the path
	current := rootDict
	for i := 1; i < len(parts)-1; i++ {
		partName := parts[i]
		if pair, ok := current.GetByString(partName); ok {
			if d, ok := pair.Value.(*object.Dict); ok {
				current = d
			} else {
				// Exists but not a dict - replace with new dict
				newDict := &object.Dict{Pairs: make(map[string]object.DictPair)}
				current.SetByString(partName, newDict)
				current = newDict
			}
		} else {
			// Doesn't exist - create
			newDict := &object.Dict{Pairs: make(map[string]object.DictPair)}
			current.SetByString(partName, newDict)
			current = newDict
		}
	}

	// Set the final part
	finalName := parts[len(parts)-1]
	current.SetByString(finalName, libDict)

	// Also set the full path as an alias for convenience
	p.env.Set(name, libDict)
}

// libraryToDict converts a Library to a Dict object
func (p *Scriptling) libraryToDict(lib *object.Library) *object.Dict {
	funcs := lib.Functions()
	consts := lib.Constants()
	subs := lib.SubLibraries()

	dict := make(map[string]object.DictPair, len(funcs)+len(consts)+len(subs))

	for fname, fn := range funcs {
		dict[object.DictKey(&object.String{Value: fname})] = object.DictPair{
			Key:   &object.String{Value: fname},
			Value: fn,
		}
	}

	// Add constants
	for cname, val := range consts {
		dict[object.DictKey(&object.String{Value: cname})] = object.DictPair{
			Key:   &object.String{Value: cname},
			Value: val,
		}
	}

	// Add sub-libraries (recursive)
	for subName, subLib := range subs {
		dict[object.DictKey(&object.String{Value: subName})] = object.DictPair{
			Key:   &object.String{Value: subName},
			Value: p.libraryToDict(subLib),
		}
	}

	// Add description if available
	if desc := lib.Description(); desc != "" {
		dict[object.DictKey(&object.String{Value: "__doc__"})] = object.DictPair{
			Key:   &object.String{Value: "__doc__"},
			Value: &object.String{Value: desc},
		}
	}

	return &object.Dict{Pairs: dict}
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

// EvalWithContext executes script with context for timeout/cancellation.
// This method is safe against deep recursion (via call depth tracking) and
// recovers from panics during script execution.
func (p *Scriptling) EvalWithContext(ctx context.Context, input string) (result object.Object, err error) {
	// Add call depth tracking to prevent stack overflow from deep recursion
	// Only add if not already present (allows callers to customize max depth)
	if evaluator.GetCallDepthFromContext(ctx) == nil {
		ctx = evaluator.ContextWithCallDepth(ctx, evaluator.DefaultMaxCallDepth)
	}

	// Try global cache first
	program, ok := Get(input)
	if !ok {
		l := lexer.New(input)
		par := parser.New(l)
		program = par.ParseProgram()
		if len(par.Errors()) != 0 {
			return nil, fmt.Errorf("parser errors: %v", par.Errors())
		}
		// Store in global cache
		Set(input, program)
	}

	// Recover from any panics during execution (e.g., bugs in interpreter or builtins)
	defer func() {
		if r := recover(); r != nil {
			stackTrace := string(debug.Stack())
			result = errors.NewPanicError(r)
			err = fmt.Errorf("script panic: %v\n%s", r, stackTrace)
		}
	}()

	// Add source file info to context for error reporting
	if p.sourceFile != "" {
		ctx = evaluator.ContextWithSourceFile(ctx, p.sourceFile)
	}

	result = evaluator.EvalWithContext(ctx, program, p.env)
	return p.handleResult(result, "")
}

func (p *Scriptling) SetVar(name string, value interface{}) error {
	obj := conversion.FromGo(value)
	p.env.Set(name, obj)
	return nil
}

// SetObjectVar sets a variable in the environment from a scriptling Object.
// This is useful when you already have a scriptling object (like an Instance)
// and want to set it directly without converting from Go types.
func (p *Scriptling) SetObjectVar(name string, obj object.Object) error {
	p.env.Set(name, obj)
	return nil
}

// GetVarAsObject retrieves a variable from the environment as a scriptling Object.
func (p *Scriptling) GetVarAsObject(name string) (object.Object, error) {
	obj, ok := p.env.Get(name)
	if !ok {
		return nil, fmt.Errorf("variable '%s' not found", name)
	}
	return obj, nil
}

func (p *Scriptling) GetVar(name string) (interface{}, object.Object) {
	obj, ok := p.env.Get(name)
	if !ok {
		return nil, &object.Error{Message: fmt.Sprintf("variable '%s' not found", name)}
	}
	return conversion.ToGo(obj), nil
}

// Convenience methods for type-safe variable access
func (p *Scriptling) GetVarAsString(name string) (string, object.Object) {
	obj, ok := p.env.Get(name)
	if !ok {
		return "", &object.Error{Message: fmt.Sprintf("variable '%s' not found", name)}
	}
	return obj.AsString()
}

func (p *Scriptling) GetVarAsInt(name string) (int64, object.Object) {
	obj, ok := p.env.Get(name)
	if !ok {
		return 0, &object.Error{Message: fmt.Sprintf("variable '%s' not found", name)}
	}
	return obj.AsInt()
}

func (p *Scriptling) GetVarAsFloat(name string) (float64, object.Object) {
	obj, ok := p.env.Get(name)
	if !ok {
		return 0, &object.Error{Message: fmt.Sprintf("variable '%s' not found", name)}
	}
	return obj.AsFloat()
}

func (p *Scriptling) GetVarAsBool(name string) (bool, object.Object) {
	obj, ok := p.env.Get(name)
	if !ok {
		return false, &object.Error{Message: fmt.Sprintf("variable '%s' not found", name)}
	}
	return obj.AsBool()
}

func (p *Scriptling) GetVarAsList(name string) ([]object.Object, object.Object) {
	obj, ok := p.env.Get(name)
	if !ok {
		return nil, &object.Error{Message: fmt.Sprintf("variable '%s' not found", name)}
	}
	return obj.AsList()
}

func (p *Scriptling) GetVarAsDict(name string) (map[string]object.Object, object.Object) {
	obj, ok := p.env.Get(name)
	if !ok {
		return nil, &object.Error{Message: fmt.Sprintf("variable '%s' not found", name)}
	}
	return obj.AsDict()
}

func (p *Scriptling) RegisterFunc(name string, fn func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object, helpText ...string) {
	builtin := &object.Builtin{Fn: fn}
	if len(helpText) > 0 && helpText[0] != "" {
		builtin.HelpText = helpText[0]
	} else {
		// Auto-generate basic help
		builtin.HelpText = fmt.Sprintf("%s(...) - User-defined function", name)
	}
	p.env.Set(name, builtin)
}

// CallFunction calls a registered function by name with Go arguments.
// Args are Go types (int, string, etc.) that will be converted to Object.
// Returns object.Object - use .AsInt(), .AsString(), etc. to extract value.
//
// Works with both Go-registered functions (via RegisterFunc) and script-defined functions.
//
// To pass a map as a dict argument, use map[string]interface{} directly.
// To pass keyword arguments, wrap the map in Kwargs{}.
//
// Example:
//
//	p.RegisterFunc("add", addFunc)
//	result, err := p.CallFunction("add", 10, 32)
//	sum, _ := result.AsInt()
//
//	// Pass a map as a dict argument
//	dataMap := map[string]interface{}{"key": "value"}
//	result, err := p.CallFunction("process", dataMap)
//
//	// With keyword arguments (use Kwargs wrapper)
//	result, err := p.CallFunction("format", "value", Kwargs{"prefix": ">>"})
func (p *Scriptling) CallFunction(name string, args ...interface{}) (object.Object, error) {
	return p.CallFunctionWithContext(context.Background(), name, args...)
}

// CallFunctionWithContext calls a registered function by name with Go arguments and a context.
// The context can be used for cancellation or timeouts.
// Args are Go types (int, string, etc.) that will be converted to Object.
// Returns object.Object - use .AsInt(), .AsString(), etc. to extract value.
//
// Works with both Go-registered functions (via RegisterFunc) and script-defined functions.
//
// To pass a map as a dict argument, use map[string]interface{} directly.
// To pass keyword arguments, wrap the map in Kwargs{}.
//
// Example:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
//	defer cancel()
//	result, err := p.CallFunctionWithContext(ctx, "add", 10, 32)
//	sum, _ := result.AsInt()
//
//	// Pass a map as a dict argument
//	dataMap := map[string]interface{}{"key": "value"}
//	result, err := p.CallFunctionWithContext(ctx, "process", dataMap)
//
//	// With keyword arguments (use Kwargs wrapper)
//	result, err := p.CallFunctionWithContext(ctx, "format", "value", Kwargs{"prefix": ">>"})
func (p *Scriptling) CallFunctionWithContext(ctx context.Context, name string, args ...interface{}) (object.Object, error) {
	// 1. Look up function in environment, supporting dotted paths like "mylib.testHandler"
	var fn object.Object
	var ok bool

	if strings.Contains(name, ".") {
		// Handle dotted path: split and traverse
		parts := splitPath(name)

		fn, ok = p.env.Get(parts[0])
		if !ok {
			return nil, fmt.Errorf("function '%s' not found", name)
		}

		if len(parts) > 1 {
			var err error
			fn, err = traverseDictPath(fn, parts[1:], maxDottedPathDepth)
			if err != nil {
				return nil, fmt.Errorf("function '%s' not found: %v", name, err)
			}
		}
	} else {
		fn, ok = p.env.Get(name)
		if !ok {
			return nil, fmt.Errorf("function '%s' not found", name)
		}
	}

	// Convert Go args to Object args
	objArgs, objKwargs := convertArgsAndKwargs(args, nil)

	// 3. Call the function using evaluator
	result := evaluator.ApplyFunction(ctx, fn, objArgs, objKwargs, p.env)
	return p.handleResult(result, fmt.Sprintf("function '%s'", name))
}

// CreateInstance creates an instance of a Scriptling class and returns it as an object.Object.
// The className should be the name of a class defined in the script or registered via a library.
// Args are Go types that will be converted to Object and passed to __init__.
//
// Example:
//
//	p.Eval("class Counter:\n    def __init__(self, start=0):\n        self.value = start")
//	instance, err := p.CreateInstance("Counter", 10)
//	if err != nil {
//	    // handle error
//	}
//	// Now you can use CallMethod on this instance
func (p *Scriptling) CreateInstance(className string, args ...interface{}) (object.Object, error) {
	return p.CreateInstanceWithContext(context.Background(), className, args...)
}

// CreateInstanceWithContext creates an instance of a Scriptling class with a context.
// The context can be used for cancellation or timeouts.
//
// Example:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
//	defer cancel()
//	instance, err := p.CreateInstanceWithContext(ctx, "Counter", 10)
func (p *Scriptling) CreateInstanceWithContext(ctx context.Context, className string, args ...interface{}) (object.Object, error) {
	// Look up the class in environment
	classObj, ok := p.env.Get(className)
	if !ok {
		return nil, fmt.Errorf("class '%s' not found", className)
	}

	// Verify it's a class
	class, ok := classObj.(*object.Class)
	if !ok {
		return nil, fmt.Errorf("'%s' is not a class, got %s", className, classObj.Type())
	}

	// Convert Go args to Object args
	objArgs, objKwargs := convertArgsAndKwargs(args, nil)

	// Create the instance using evaluator
	instance := evaluator.ApplyFunction(ctx, class, objArgs, objKwargs, p.env)
	return p.handleResult(instance, fmt.Sprintf("class '%s'", className))
}

// CallMethod calls a method on a Scriptling object (typically an Instance).
// The obj should be an object.Object (usually obtained from CreateInstance or script evaluation).
// Args are Go types that will be converted to Object.
// Returns object.Object - use .AsInt(), .AsString(), etc. to extract value.
//
// Example:
//
//	instance, _ := p.CreateInstance("Counter", 10)
//	result, err := p.CallMethod(instance, "increment")
//	value, _ := result.AsInt()
func (p *Scriptling) CallMethod(obj object.Object, methodName string, args ...interface{}) (object.Object, error) {
	return p.CallMethodWithContext(context.Background(), obj, methodName, args...)
}

// CallMethodWithContext calls a method on a Scriptling object with a context.
// The context can be used for cancellation or timeouts.
//
// Example:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
//	defer cancel()
//	instance, _ := p.CreateInstance("Counter", 10)
//	result, err := p.CallMethodWithContext(ctx, instance, "increment")
func (p *Scriptling) CallMethodWithContext(ctx context.Context, obj object.Object, methodName string, args ...interface{}) (object.Object, error) {
	// Verify obj is an Instance
	instance, ok := obj.(*object.Instance)
	if !ok {
		return nil, fmt.Errorf("object is not an instance, got %s", obj.Type())
	}

	// Look up the method in the instance's class
	method, ok := instance.Class.Methods[methodName]
	if !ok {
		return nil, fmt.Errorf("method '%s' not found in class '%s'", methodName, instance.Class.Name)
	}

	// Convert Go args to Object args (prepend self)
	objArgs, objKwargs := convertArgsAndKwargs(args, instance)

	// Call the method using evaluator
	result := evaluator.ApplyFunction(ctx, method, objArgs, objKwargs, p.env)
	return p.handleResult(result, fmt.Sprintf("method '%s' on class '%s'", methodName, instance.Class.Name))
}

// RegisterLibrary registers a new library that can be imported by scripts
// The library name is extracted from the library itself
func (p *Scriptling) RegisterLibrary(lib *object.Library) {
	p.registeredLibraries[lib.Name()] = lib
}

// Import imports a library into the current environment, making it available for use without needing an import statement in scripts
func (p *Scriptling) Import(names interface{}) error {
	switch v := names.(type) {
	case string:
		// Single library name
		return p.loadLibrary(v)
	case []string:
		// Go slice of strings
		for _, name := range v {
			if err := p.loadLibrary(name); err != nil {
				return err
			}
		}
		return nil
	case *object.List:
		// Scriptling list of strings
		for _, elem := range v.Elements {
			if str, ok := elem.(*object.String); ok {
				if err := p.loadLibrary(str.Value); err != nil {
					return err
				}
			} else {
				return fmt.Errorf("Import: all elements must be strings, got %T", elem)
			}
		}
		return nil
	default:
		return fmt.Errorf("Import: expected string, []string, or list of strings, got %T", names)
	}
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

// LoadLibraryIntoEnv loads a library into the specified environment.
// This is useful for loading libraries into cloned environments for background tasks.
// Returns an error if the library cannot be loaded.
func (p *Scriptling) LoadLibraryIntoEnv(name string, env *object.Environment) error {
	loaded, err := p.loadLibraryIntoEnv(name, env)
	if err != nil {
		return err
	}
	if !loaded {
		// Try on-demand callback
		if p.onDemandLibraryCallback != nil && p.onDemandLibraryCallback(p, name) {
			// Retry after callback
			loaded, err = p.loadLibraryIntoEnv(name, env)
			if err != nil {
				return err
			}
			if loaded {
				return nil
			}
		}
		return fmt.Errorf("library not found: %s", name)
	}
	return nil
}

// SetSourceFile sets the source file name used in error messages.
// When set, errors will include the file name and line number for better debugging.
func (p *Scriptling) SetSourceFile(name string) {
	p.sourceFile = name
}

// loadLibraryIntoEnv loads a script or registered library into the given environment as a dict.
// Returns true if the library was found and loaded, false otherwise.
func (p *Scriptling) loadLibraryIntoEnv(name string, env *object.Environment) (bool, error) {
	var libDict *object.Dict

	// Try from script libraries
	if lib, ok := p.scriptLibraries[name]; ok {
		if lib.store == nil {
			store, err := p.evaluateScriptLibrary(name, lib.source)
			if err != nil {
				return false, err
			}
			lib.store = store
		}
		pairs := make(map[string]object.DictPair, len(lib.store))
		for fname, obj := range lib.store {
			pairs[object.DictKey(&object.String{Value: fname})] = object.DictPair{
				Key:   &object.String{Value: fname},
				Value: obj,
			}
		}
		libDict = &object.Dict{Pairs: pairs}
	} else if lib, ok := p.registeredLibraries[name]; ok {
		// Try from registered libraries
		libDict = p.libraryToDict(lib)
	} else {
		return false, nil
	}

	// Handle dotted paths - create parent dicts as needed
	parts := strings.Split(name, ".")
	if len(parts) == 1 {
		// Simple case - just set directly
		env.Set(name, libDict)
		return true, nil
	}

	// Nested case - create/update parent dicts
	rootName := parts[0]
	var rootDict *object.Dict
	if existing, ok := env.Get(rootName); ok {
		if d, ok := existing.(*object.Dict); ok {
			rootDict = d
		} else {
			rootDict = &object.Dict{Pairs: make(map[string]object.DictPair)}
			env.Set(rootName, rootDict)
		}
	} else {
		rootDict = &object.Dict{Pairs: make(map[string]object.DictPair)}
		env.Set(rootName, rootDict)
	}

	// Navigate/create the path
	current := rootDict
	for i := 1; i < len(parts)-1; i++ {
		partName := parts[i]
		if pair, ok := current.GetByString(partName); ok {
			if d, ok := pair.Value.(*object.Dict); ok {
				current = d
			} else {
				newDict := &object.Dict{Pairs: make(map[string]object.DictPair)}
				current.SetByString(partName, newDict)
				current = newDict
			}
		} else {
			newDict := &object.Dict{Pairs: make(map[string]object.DictPair)}
			current.SetByString(partName, newDict)
			current = newDict
		}
	}

	// Set the final part, merging any existing sub-module entries
	// This preserves sub-library registrations when a parent library is loaded
	// after its children (e.g., importing scriptling.ai after scriptling.ai.agent)
	finalPart := parts[len(parts)-1]
	if existingPair, ok := current.GetByString(finalPart); ok {
		if existingDict, ok := existingPair.Value.(*object.Dict); ok {
			for k, v := range existingDict.Pairs {
				if _, exists := libDict.Pairs[k]; !exists {
					libDict.Pairs[k] = v
				}
			}
		}
	}
	current.SetByString(finalPart, libDict)

	// Also store the full dotted name for reliable "already imported" checks
	env.Set(name, libDict)

	return true, nil
}

func (p *Scriptling) evaluateScriptLibrary(name string, script string) (map[string]object.Object, error) {
	// Create a new environment for the library
	libEnv := object.NewEnvironment()

	// Inherit writer from main environment (for output capture)
	writer := p.env.GetWriter()
	libEnv.SetOutputWriter(writer)

	// Set up import builtin for nested imports
	libEnv.Set("import", evaluator.GetImportBuiltin())

	// Create a custom import callback for this library environment
	libEnv.SetImportCallback(func(libName string) error {
		// Check if library is already imported using direct lookup.
		// We use env.Get(libName) rather than path traversal because intermediate
		// dicts created for child libraries (e.g., scriptling.ai created as a
		// placeholder when loading scriptling.ai.agent) can produce false positives.
		if _, ok := libEnv.Get(libName); ok {
			return nil // Already imported
		}

		for attempts := 0; attempts < 2; attempts++ {
			loaded, err := p.loadLibraryIntoEnv(libName, libEnv)
			if err != nil {
				return err
			}
			if loaded {
				return nil
			}

			// If first attempt and callback exists, try on-demand loading
			if attempts == 0 && p.onDemandLibraryCallback != nil {
				if !p.onDemandLibraryCallback(p, libName) {
					break
				}
			} else {
				break
			}
		}

		return fmt.Errorf("unknown library: %s", libName)
	})

	// Copy available libraries callback from parent environment
	libEnv.SetAvailableLibrariesCallback(p.env.GetAvailableLibrariesCallback())

	// Parse and evaluate the script in the library environment
	var program *ast.Program
	if cached, ok := Get(script); ok {
		program = cached
	} else {
		l := lexer.New(script)
		par := parser.New(l)
		program = par.ParseProgram()
		if len(par.Errors()) != 0 {
			return nil, fmt.Errorf("parser errors: %v", par.Errors())
		}
		Set(script, program)
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

	result := evaluator.EvalWithContext(
		evaluator.ContextWithSourceFile(context.Background(), name),
		program, libEnv,
	)
	if err, ok := result.(*object.Error); ok {
		// Include location info in the error message
		msg := err.Message
		if err.Line > 0 || err.File != "" {
			loc := ""
			if err.File != "" {
				loc = err.File
			}
			if err.Line > 0 {
				if loc != "" {
					loc += fmt.Sprintf(":%d", err.Line)
				} else {
					loc = fmt.Sprintf("line %d", err.Line)
				}
			}
			msg = fmt.Sprintf("%s (%s)", msg, loc)
		}
		return nil, fmt.Errorf("%s", msg)
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
// Supports nested paths like "scriptling.ai.agent" - will create parent dicts as needed
func (p *Scriptling) registerScriptLibrary(name string, store map[string]object.Object) {
	lib := make(map[string]object.DictPair, len(store))
	for fname, obj := range store {
		lib[object.DictKey(&object.String{Value: fname})] = object.DictPair{
			Key:   &object.String{Value: fname},
			Value: obj,
		}
	}
	libDict := &object.Dict{Pairs: lib}

	// Check if this is a dotted path
	parts := strings.Split(name, ".")
	if len(parts) == 1 {
		// Simple case - just set directly
		p.env.Set(name, libDict)
		return
	}

	// Nested case - need to create/update parent dicts
	// First, get or create the root dict
	rootName := parts[0]
	var rootDict *object.Dict
	if existing, ok := p.env.Get(rootName); ok {
		if d, ok := existing.(*object.Dict); ok {
			rootDict = d
		} else {
			// Exists but not a dict - replace with new dict
			rootDict = &object.Dict{Pairs: make(map[string]object.DictPair)}
			p.env.Set(rootName, rootDict)
		}
	} else {
		rootDict = &object.Dict{Pairs: make(map[string]object.DictPair)}
		p.env.Set(rootName, rootDict)
	}

	// Navigate/create the path
	current := rootDict
	for i := 1; i < len(parts)-1; i++ {
		partName := parts[i]
		if pair, ok := current.GetByString(partName); ok {
			if d, ok := pair.Value.(*object.Dict); ok {
				current = d
			} else {
				// Exists but not a dict - replace with new dict
				newDict := &object.Dict{Pairs: make(map[string]object.DictPair)}
				current.SetByString(partName, newDict)
				current = newDict
			}
		} else {
			// Doesn't exist - create
			newDict := &object.Dict{Pairs: make(map[string]object.DictPair)}
			current.SetByString(partName, newDict)
			current = newDict
		}
	}

	// Set the final part
	finalName := parts[len(parts)-1]
	current.SetByString(finalName, libDict)

	// Also set the full path as an alias for convenience
	p.env.Set(name, libDict)
}

// EnableOutputCapture enables capturing print output instead of sending to stdout
func (p *Scriptling) EnableOutputCapture() {
	p.env.EnableOutputCapture()
}

// SetOutputWriter sets a custom writer for output (e.g., for streaming to a websocket or logger)
func (p *Scriptling) SetOutputWriter(w io.Writer) {
	p.env.SetOutputWriter(w)
}

// SetInputReader sets a custom reader for input (e.g., for reading from a websocket)
func (p *Scriptling) SetInputReader(r io.Reader) {
	p.env.SetInputReader(r)
}

// GetOutput returns captured output and clears the buffer
func (p *Scriptling) GetOutput() string {
	return p.env.GetOutput()
}

// handleResult converts an Object result to (Object, error)
//
// Return behavior:
//   - Normal value: (value, nil)
//   - Error object: (Error object, error with message)
//   - SystemExit(0): (Exception, nil) - clean success but object available for inspection
//   - SystemExit(!=0): (Exception, error with message)
//   - Other exceptions: (Exception, error with message)
//
// The contextMsg parameter provides context for error messages, e.g.:
//   - "function 'add'"
//   - "method 'increment' on class 'Counter'"
//   - "class 'Counter'"
//   - "" (for Eval, no additional context)
func (p *Scriptling) handleResult(result object.Object, contextMsg string) (object.Object, error) {
	switch obj := result.(type) {
	case *object.Error:
		// Build error message with location info
		msg := obj.Message
		if obj.Line > 0 || obj.File != "" {
			loc := ""
			if obj.File != "" {
				loc = obj.File
			}
			if obj.Line > 0 {
				if loc != "" {
					loc += fmt.Sprintf(":%d", obj.Line)
				} else {
					loc = fmt.Sprintf("line %d", obj.Line)
				}
			}
			msg = fmt.Sprintf("%s (%s)", msg, loc)
		}
		if contextMsg != "" {
			return obj, fmt.Errorf("%s: %s", contextMsg, msg)
		}
		return obj, fmt.Errorf("%s", msg)

	case *object.Exception:
		if obj.IsSystemExit() {
			// Always return the Exception object for consistency
			// Error return indicates whether it's an error condition
			if obj.GetExitCode() == 0 {
				return obj, nil // clean exit
			}
			return obj, fmt.Errorf("%s", obj.Message)
		}
		// Other exceptions
		return obj, fmt.Errorf("%s", obj.Message)

	default:
		return result, nil
	}
}
