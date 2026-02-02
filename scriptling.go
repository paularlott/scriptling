package scriptling

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/paularlott/scriptling/ast"
	"github.com/paularlott/scriptling/evaluator"
	"github.com/paularlott/scriptling/extlibs"
	"github.com/paularlott/scriptling/lexer"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/parser"
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

func (p *Scriptling) loadLibrary(name string) error {
	// For dotted names like scriptling.ai.agent, check if we need to load parent first
	parts := strings.Split(name, ".")
	if len(parts) > 1 {
		// Check if parent exists, if not load it first
		parentName := strings.Join(parts[:len(parts)-1], ".")
		if _, ok := p.env.Get(parentName); !ok {
			// Parent doesn't exist, try to load it
			if err := p.loadLibrary(parentName); err != nil {
				// Parent doesn't exist as a library, that's ok - we'll create the structure
			}
		}
	}

	// Check if library is already imported
	if existingObj, ok := p.env.Get(name); ok {
		// For simple library names, check if it's a proper library dict
		// If it exists but is incomplete (e.g., only has sub-libraries from a dotted import),
		// we need to merge in the parent library
		if len(parts) == 1 {
			// Check if this is a registered library that should be loaded
			if _, isRegistered := p.registeredLibraries[name]; isRegistered {
				// Check if existing object is a dict
				if existingDict, ok := existingObj.(*object.Dict); ok {
					// Check if it has any functions from the library (not just sub-libraries)
					// If it only has sub-libraries, we need to merge the parent library
					hasLibraryFunctions := false
					for key := range existingDict.Pairs {
						// Check if any keys don't look like sub-library names (no dots in registered sub-libs)
						if key != "__doc__" && !strings.Contains(key, ".") {
							// Could be a function or a sub-library, check the registered library
							lib := p.registeredLibraries[name]
							if funcs := lib.Functions(); funcs != nil {
								if _, exists := funcs[key]; exists {
									hasLibraryFunctions = true
									break
								}
							}
						}
					}
					if !hasLibraryFunctions {
						// Existing dict only has sub-libraries, merge in the parent library
						lib := p.registeredLibraries[name]
						libDict := p.libraryToDict(lib)
						// Merge: add all entries from libDict to existingDict
						for k, v := range libDict.Pairs {
							if _, exists := existingDict.Pairs[k]; !exists {
								existingDict.Pairs[k] = v
							}
						}
						return nil
					}
				}
			}
		}
		return nil // Already imported, skip
	}

	// For dotted names like urllib.parse, also check if parent exists with the sub-library
	if len(parts) > 1 {
		// Check if parent is already imported with this sub-library
		if parentObj, ok := p.env.Get(parts[0]); ok {
			if parentDict, ok := parentObj.(*object.Dict); ok {
				// Navigate through the parts to see if it exists
				current := parentDict
				allExist := true
				for i := 1; i < len(parts); i++ {
					if pair, ok := current.Pairs[parts[i]]; ok {
						if subDict, ok := pair.Value.(*object.Dict); ok {
							current = subDict
						} else {
							allExist = false
							break
						}
					} else {
						allExist = false
						break
					}
				}
				if allExist {
					// Create an alias for the full path
					p.env.Set(name, current)
					return nil
				}
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
	// Convert library to dict
	libDict := p.libraryToDict(lib)

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
		if pair, ok := current.Pairs[partName]; ok {
			if d, ok := pair.Value.(*object.Dict); ok {
				current = d
			} else {
				// Exists but not a dict - replace with new dict
				newDict := &object.Dict{Pairs: make(map[string]object.DictPair)}
				current.Pairs[partName] = object.DictPair{
					Key:   &object.String{Value: partName},
					Value: newDict,
				}
				current = newDict
			}
		} else {
			// Doesn't exist - create
			newDict := &object.Dict{Pairs: make(map[string]object.DictPair)}
			current.Pairs[partName] = object.DictPair{
				Key:   &object.String{Value: partName},
				Value: newDict,
			}
			current = newDict
		}
	}

	// Set the final part
	finalName := parts[len(parts)-1]
	current.Pairs[finalName] = object.DictPair{
		Key:   &object.String{Value: finalName},
		Value: libDict,
	}

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

	// Add sub-libraries (recursive)
	for subName, subLib := range subs {
		dict[subName] = object.DictPair{
			Key:   &object.String{Value: subName},
			Value: p.libraryToDict(subLib),
		}
	}

	// Add description if available
	if desc := lib.Description(); desc != "" {
		dict["__doc__"] = object.DictPair{
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

// EvalWithContext executes script with context for timeout/cancellation
func (p *Scriptling) EvalWithContext(ctx context.Context, input string) (object.Object, error) {
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

	result := evaluator.EvalWithContext(ctx, program, p.env)
	if err, ok := result.(*object.Error); ok {
		return nil, fmt.Errorf("%s", err.Message)
	}

	// Check for SystemExit exception
	if ex, ok := result.(*object.Exception); ok && ex.ExceptionType == "SystemExit" {
		// Extract exit code from the message
		code := 0
		if strings.HasPrefix(ex.Message, "SystemExit: ") {
			codeStr := strings.TrimPrefix(ex.Message, "SystemExit: ")
			code = parseIntFromMessage(codeStr)
		}
		return nil, &extlibs.SysExitCode{Code: code}
	}

	return result, nil
}

// parseIntFromMessage extracts an integer from a message string
func parseIntFromMessage(msg string) int {
	var code int
	_, err := fmt.Sscanf(msg, "%d", &code)
	if err != nil {
		return 1 // Default to exit code 1 if parsing fails
	}
	return code
}

func (p *Scriptling) SetVar(name string, value interface{}) error {
	obj := FromGo(value)
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

func (p *Scriptling) GetVar(name string) (interface{}, object.Object) {
	obj, ok := p.env.Get(name)
	if !ok {
		return nil, &object.Error{Message: fmt.Sprintf("variable '%s' not found", name)}
	}
	return ToGo(obj), nil
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

// Kwargs is a wrapper type to explicitly pass keyword arguments to CallFunction.
// Use this to distinguish between a map being passed as a dict argument vs kwargs.
//
// Example:
//
//	// Pass a map as a dict argument:
//	result, err := p.CallFunction("process", map[string]interface{}{"key": "value"})
//
//	// Pass keyword arguments:
//	result, err := p.CallFunction("format", "text", Kwargs{"prefix": ">>"})
type Kwargs map[string]interface{}

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
	// 1. Look up function in environment
	fn, ok := p.env.Get(name)
	if !ok {
		return nil, fmt.Errorf("function '%s' not found", name)
	}

	// Convert Go args to Object args
	objArgs, objKwargs := convertArgsAndKwargs(args, nil)

	// 3. Call the function using evaluator
	result := evaluator.ApplyFunction(ctx, fn, objArgs, objKwargs, p.env)

	// 4. Handle errors
	if err, ok := result.(*object.Error); ok && err != nil {
		return nil, fmt.Errorf("function error: %s", err.Message)
	}

	// 5. Check for SystemExit exception
	if ex, ok := result.(*object.Exception); ok && ex.ExceptionType == "SystemExit" {
		code := 0
		if strings.HasPrefix(ex.Message, "SystemExit: ") {
			codeStr := strings.TrimPrefix(ex.Message, "SystemExit: ")
			code = parseIntFromMessage(codeStr)
		}
		return nil, &extlibs.SysExitCode{Code: code}
	}

	return result, nil
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

	// Check for errors
	if err, ok := instance.(*object.Error); ok && err != nil {
		return nil, fmt.Errorf("instance creation error: %s", err.Message)
	}

	// Check for SystemExit exception
	if ex, ok := instance.(*object.Exception); ok && ex.ExceptionType == "SystemExit" {
		code := 0
		if strings.HasPrefix(ex.Message, "SystemExit: ") {
			codeStr := strings.TrimPrefix(ex.Message, "SystemExit: ")
			code = parseIntFromMessage(codeStr)
		}
		return nil, &extlibs.SysExitCode{Code: code}
	}

	return instance, nil
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

	// Handle errors
	if err, ok := result.(*object.Error); ok && err != nil {
		return nil, fmt.Errorf("method error: %s", err.Message)
	}

	// Check for SystemExit exception
	if ex, ok := result.(*object.Exception); ok && ex.ExceptionType == "SystemExit" {
		code := 0
		if strings.HasPrefix(ex.Message, "SystemExit: ") {
			codeStr := strings.TrimPrefix(ex.Message, "SystemExit: ")
			code = parseIntFromMessage(codeStr)
		}
		return nil, &extlibs.SysExitCode{Code: code}
	}

	return result, nil
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

func (p *Scriptling) evaluateScriptLibrary(name string, script string) (map[string]object.Object, error) {
	// Create a new environment for the library
	libEnv := object.NewEnvironment()

	// Inherit writer from main environment (for output capture)
	writer := p.env.GetWriter()
	libEnv.SetOutputWriter(writer)

	// Set up import builtin for nested imports
	libEnv.Set("import", evaluator.GetImportBuiltin())

	// Create a custom import callback for this library environment
	// that loads libraries into libEnv instead of p.env
	libEnv.SetImportCallback(func(libName string) error {
		// Check if library is already imported in this environment
		if _, ok := libEnv.Get(libName); ok {
			return nil // Already imported, skip
		}

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
			// Convert library to dict and load into library environment
			libDict := p.libraryToDict(lib)
			libEnv.Set(libName, libDict)
			return nil
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
// Supports nested paths like "scriptling.ai.agent" - will create parent dicts as needed
func (p *Scriptling) registerScriptLibrary(name string, store map[string]object.Object) {
	lib := make(map[string]object.DictPair, len(store))
	for fname, obj := range store {
		lib[fname] = object.DictPair{
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
		if pair, ok := current.Pairs[partName]; ok {
			if d, ok := pair.Value.(*object.Dict); ok {
				current = d
			} else {
				// Exists but not a dict - replace with new dict
				newDict := &object.Dict{Pairs: make(map[string]object.DictPair)}
				current.Pairs[partName] = object.DictPair{
					Key:   &object.String{Value: partName},
					Value: newDict,
				}
				current = newDict
			}
		} else {
			// Doesn't exist - create
			newDict := &object.Dict{Pairs: make(map[string]object.DictPair)}
			current.Pairs[partName] = object.DictPair{
				Key:   &object.String{Value: partName},
				Value: newDict,
			}
			current = newDict
		}
	}

	// Set the final part
	finalName := parts[len(parts)-1]
	current.Pairs[finalName] = object.DictPair{
		Key:   &object.String{Value: finalName},
		Value: libDict,
	}

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
