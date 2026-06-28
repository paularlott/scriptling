package object

import (
	"context"
	"fmt"
	"reflect"
)

const receiverField = "_receiver"

var (
	instanceType = reflect.TypeOf((*Instance)(nil))
)

// ClassBuilder provides a fluent API for creating scriptling classes.
// It allows registering typed Go methods that are automatically wrapped
// to handle conversion between Go types and scriptling Objects.
//
// Two styles are supported:
//
// 1. *Instance methods — manage fields via SetField/Field:
//
//	cb := NewClassBuilder("Person")
//	cb.Method("__init__", func(self *Instance, name string) {
//	    self.SetField("name", NewString(name))
//	})
//	cb.Method("greet", func(self *Instance) string {
//	    return self.Field("name").(*String).StringValue()
//	})
//
// 2. Typed receiver methods — Go struct is auto-managed:
//
//	type Config struct { values map[string]any }
//
//	cb := NewClassBuilder("Config")
//	cb.Constructor(func(name string) *Config {
//	    return &Config{values: map[string]any{"name": name}}
//	})
//	cb.Method("get", func(self *Config, key string) any {
//	    return self.values[key]
//	})
//
// When Constructor is used, the returned Go struct is automatically wrapped
// in a ClientWrapper and stored on the Instance. Methods whose first parameter
// matches the constructor's return type receive the unwrapped struct directly.
type ClassBuilder struct {
	name          string
	baseClass     *Class
	methods       map[string]*Builtin
	properties    map[string]*Builtin
	setters       map[string]*Builtin
	statics       map[string]*Builtin
	env           *Environment
	receiverType  reflect.Type
	constructor   *Builtin
	constructorFn reflect.Value
	constructorSig *FunctionSignature
}

// NewClassBuilder creates a new ClassBuilder with the given class name.
func NewClassBuilder(name string) *ClassBuilder {
	return &ClassBuilder{
		name:    name,
		methods: make(map[string]*Builtin),
	}
}

// BaseClass sets the base class for inheritance.
func (cb *ClassBuilder) BaseClass(base *Class) *ClassBuilder {
	cb.baseClass = base
	return cb
}

// Constructor registers a constructor function for typed receiver classes.
// The function must return a pointer type (e.g., *Config) which becomes the
// receiver type for subsequent Method calls. The returned struct is automatically
// wrapped in a ClientWrapper and stored on the Instance.
//
// Example:
//
//	type Config struct { values map[string]any }
//
//	cb := NewClassBuilder("Config")
//	cb.Constructor(func(name string) *Config {
//	    return &Config{values: map[string]any{"name": name}}
//	})
//	cb.Method("get", func(self *Config, key string) any {
//	    return self.values[key]
//	})
func (cb *ClassBuilder) Constructor(fn interface{}) *ClassBuilder {
	fnValue := reflect.ValueOf(fn)
	fnType := fnValue.Type()

	if fnType.Kind() != reflect.Func {
		panic(fmt.Sprintf("ClassBuilder.Constructor: must be a function, got %T", fn))
	}
	if fnType.NumOut() == 0 {
		panic("ClassBuilder.Constructor: function must return a pointer type")
	}

	retType := fnType.Out(0)
	if retType.Kind() != reflect.Ptr {
		panic(fmt.Sprintf("ClassBuilder.Constructor: must return a pointer type, got %v", retType))
	}

	if cb.receiverType != nil && cb.receiverType != retType {
		panic(fmt.Sprintf("ClassBuilder.Constructor: receiver type mismatch: already have %v, got %v", cb.receiverType, retType))
	}

	cb.receiverType = retType
	cb.constructorFn = fnValue
	cb.constructorSig = analyzeConstructorSignature(fnType)

	cb.constructor = &Builtin{
		Fn: func(ctx context.Context, kwargs Kwargs, args ...Object) Object {
			return cb.callConstructor(fnValue, cb.constructorSig, ctx, kwargs, args)
		},
	}

	return cb
}

// Method registers a typed Go method with the class.
// The first parameter is the receiver: either *Instance (manual Fields)
// or the typed pointer set by Constructor (e.g., *Config).
//
// Example with *Instance:
//
//	cb.Method("greet", func(self *Instance, name string) string {
//	    return "Hello, " + name
//	})
//
// Example with typed receiver:
//
//	cb.Method("get", func(self *Config, key string) any {
//	    return self.values[key]
//	})
func (cb *ClassBuilder) Method(name string, fn interface{}) *ClassBuilder {
	cb.MethodWithHelp(name, fn, "")
	return cb
}

// MethodWithHelp registers a method with help text.
// Help text is displayed when users call help() on the method.
func (cb *ClassBuilder) MethodWithHelp(name string, fn interface{}, helpText string) *ClassBuilder {
	wrapper := cb.createWrapper(fn, helpText)
	cb.methods[name] = wrapper
	return cb
}

// Property registers a getter function as a @property on the class.
// The getter receives self as its only argument.
//
// Example:
//
//	cb.Property("area", func(self *Instance) float64 {
//	    r, _ := self.Field("radius").AsFloat()
//	    return math.Pi * r * r
//	})
func (cb *ClassBuilder) Property(name string, fn interface{}) *ClassBuilder {
	if cb.properties == nil {
		cb.properties = make(map[string]*Builtin)
	}
	cb.properties[name] = cb.createWrapper(fn, "")
	return cb
}

// PropertyWithSetter registers a getter and setter as a @property on the class.
// The getter receives self only; the setter receives self and the new value.
//
// Example:
//
//	cb.PropertyWithSetter("radius",
//	    func(self *Instance) float64 { r, _ := self.Field("r").AsFloat(); return r },
//	    func(self *Instance, v float64) { self.SetField("r", NewFloat(v)) },
//	)
func (cb *ClassBuilder) PropertyWithSetter(name string, getter interface{}, setter interface{}) *ClassBuilder {
	if cb.properties == nil {
		cb.properties = make(map[string]*Builtin)
	}
	if cb.setters == nil {
		cb.setters = make(map[string]*Builtin)
	}
	cb.properties[name] = cb.createWrapper(getter, "")
	cb.setters[name] = cb.createWrapper(setter, "")
	return cb
}

// StaticMethod registers a function as a @staticmethod on the class.
// The function does NOT receive self — do not include *Instance as the first parameter.
//
// Example:
//
//	cb.StaticMethod("from_degrees", func(deg float64) float64 {
//	    return deg * math.Pi / 180
//	})
func (cb *ClassBuilder) StaticMethod(name string, fn interface{}) *ClassBuilder {
	if cb.statics == nil {
		cb.statics = make(map[string]*Builtin)
	}
	fb := NewFunctionBuilder()
	fb.Function(fn)
	cb.statics[name] = &Builtin{Fn: fb.Build()}
	return cb
}

// Environment sets the environment for the class.
// This is optional and usually not needed.
func (cb *ClassBuilder) Environment(env *Environment) *ClassBuilder {
	cb.env = env
	return cb
}

// Build creates and returns the Class from this builder.
// If a Constructor was registered, an __init__ method is auto-generated
// that calls the constructor and stores the result as a ClientWrapper.
func (cb *ClassBuilder) Build() *Class {
	methods := cb.convertMethodsToObjects()

	if cb.constructor != nil {
		if _, exists := methods["__init__"]; !exists {
			methods["__init__"] = cb.constructor
		}
	}

	return &Class{
		Name:      cb.name,
		BaseClass: cb.baseClass,
		Methods:   methods,
		Env:       cb.env,
	}
}

// convertMethodsToObjects converts the methods map to map[string]Object
func (cb *ClassBuilder) convertMethodsToObjects() map[string]Object {
	result := make(map[string]Object, len(cb.methods)+len(cb.properties)+len(cb.statics))
	for name, builtin := range cb.methods {
		result[name] = builtin
	}
	for name, getter := range cb.properties {
		var setter Object
		if cb.setters != nil {
			if s, ok := cb.setters[name]; ok {
				setter = s
			}
		}
		result[name] = &Property{Getter: getter, Setter: setter}
	}
	for name, fn := range cb.statics {
		result[name] = &StaticMethod{Fn: fn}
	}
	return result
}

// createWrapper creates a Builtin wrapper for a typed Go method.
func (cb *ClassBuilder) createWrapper(fn interface{}, helpText string) *Builtin {
	fnValue := reflect.ValueOf(fn)
	fnType := fnValue.Type()

	if fnType.Kind() != reflect.Func {
		panic(fmt.Sprintf("ClassBuilder: must be a function, got %T", fnValue.Interface()))
	}

	sig := analyzeClassMethodSignature(fnType, cb.receiverType)

	if sig.typedReceiver {
		return cb.createTypedReceiverWrapper(fnValue, sig, helpText)
	}

	if wrapper, ok := createFastMethodWrapper(fn, helpText); ok {
		return wrapper
	}

	return &Builtin{
		Fn: func(ctx context.Context, kwargs Kwargs, args ...Object) Object {
			return cb.callTypedMethod(fnValue, sig, ctx, kwargs, args)
		},
		HelpText: helpText,
	}
}

func createFastMethodWrapper(fn interface{}, helpText string) (*Builtin, bool) {
	switch typed := fn.(type) {
	case func(*Instance) string:
		return &Builtin{
			Fn: func(ctx context.Context, kwargs Kwargs, args ...Object) (result Object) {
				defer func() {
					if r := recover(); r != nil {
						result = newError("panic in method: %v", r)
					}
				}()
				if len(args) == 0 {
					return newError("method call requires at least one argument (instance)")
				}
				if len(args) != 1 {
					return newArgumentError(len(args), 1)
				}
				self, ok := args[0].(*Instance)
				if !ok {
					return newError("first argument must be an instance, got %T", args[0])
				}
				return &String{value: typed(self)}
			},
			HelpText: helpText,
		}, true
	case func(*Instance) Object:
		return &Builtin{
			Fn: func(ctx context.Context, kwargs Kwargs, args ...Object) (result Object) {
				defer func() {
					if r := recover(); r != nil {
						result = newError("panic in method: %v", r)
					}
				}()
				if len(args) == 0 {
					return newError("method call requires at least one argument (instance)")
				}
				if len(args) != 1 {
					return newArgumentError(len(args), 1)
				}
				self, ok := args[0].(*Instance)
				if !ok {
					return newError("first argument must be an instance, got %T", args[0])
				}
				out := typed(self)
				if out == nil {
					return &Null{}
				}
				return out
			},
			HelpText: helpText,
		}, true
	case func(*Instance, context.Context) Object:
		return &Builtin{
			Fn: func(ctx context.Context, kwargs Kwargs, args ...Object) (result Object) {
				defer func() {
					if r := recover(); r != nil {
						result = newError("panic in method: %v", r)
					}
				}()
				if len(args) == 0 {
					return newError("method call requires at least one argument (instance)")
				}
				if len(args) != 1 {
					return newArgumentError(len(args), 1)
				}
				self, ok := args[0].(*Instance)
				if !ok {
					return newError("first argument must be an instance, got %T", args[0])
				}
				out := typed(self, ctx)
				if out == nil {
					return &Null{}
				}
				return out
			},
			HelpText: helpText,
		}, true
	case func(*Instance, string) string:
		return &Builtin{
			Fn: func(ctx context.Context, kwargs Kwargs, args ...Object) (result Object) {
				defer func() {
					if r := recover(); r != nil {
						result = newError("panic in method: %v", r)
					}
				}()
				if len(args) == 0 {
					return newError("method call requires at least one argument (instance)")
				}
				if len(args) != 2 {
					return newArgumentError(len(args)-1, 1)
				}
				self, ok := args[0].(*Instance)
				if !ok {
					return newError("first argument must be an instance, got %T", args[0])
				}
				s, err := args[1].AsString()
				if err != nil {
					return err
				}
				return &String{value: typed(self, s)}
			},
			HelpText: helpText,
		}, true
	case func(*Instance, context.Context, string) Object:
		return &Builtin{
			Fn: func(ctx context.Context, kwargs Kwargs, args ...Object) (result Object) {
				defer func() {
					if r := recover(); r != nil {
						result = newError("panic in method: %v", r)
					}
				}()
				if len(args) == 0 {
					return newError("method call requires at least one argument (instance)")
				}
				if len(args) != 2 {
					return newArgumentError(len(args)-1, 1)
				}
				self, ok := args[0].(*Instance)
				if !ok {
					return newError("first argument must be an instance, got %T", args[0])
				}
				s, err := args[1].AsString()
				if err != nil {
					return err
				}
				out := typed(self, ctx, s)
				if out == nil {
					return &Null{}
				}
				return out
			},
			HelpText: helpText,
		}, true
	case func(*Instance, context.Context, string, string, map[string]string, Object) Object:
		return &Builtin{
			Fn: func(ctx context.Context, kwargs Kwargs, args ...Object) (result Object) {
				defer func() {
					if r := recover(); r != nil {
						result = newError("panic in method: %v", r)
					}
				}()
				if len(args) == 0 {
					return newError("method call requires at least one argument (instance)")
				}
				if len(args) != 5 {
					return newArgumentError(len(args)-1, 4)
				}
				self, ok := args[0].(*Instance)
				if !ok {
					return newError("first argument must be an instance, got %T", args[0])
				}
				name, err := args[1].AsString()
				if err != nil {
					return err
				}
				description, err := args[2].AsString()
				if err != nil {
					return err
				}
				paramsVal, convErr := convertObjectToValue(args[3], reflect.TypeOf(map[string]string{}))
				if convErr != nil {
					return convErr
				}
				params, _ := paramsVal.Interface().(map[string]string)
				out := typed(self, ctx, name, description, params, args[4])
				if out == nil {
					return &Null{}
				}
				return out
			},
			HelpText: helpText,
		}, true
	default:
		return nil, false
	}
}

// callConstructor calls the constructor function and wraps the result.
func (cb *ClassBuilder) callConstructor(fnValue reflect.Value, sig *FunctionSignature, ctx context.Context, kwargs Kwargs, args []Object) (result Object) {
	defer func() {
		if r := recover(); r != nil {
			result = newError("panic in constructor: %v", r)
		}
	}()

	if len(args) == 0 {
		return newError("constructor requires at least one argument (instance)")
	}

	instance, ok := args[0].(*Instance)
	if !ok {
		return newError("first argument must be an instance, got %T", args[0])
	}
	methodArgs := args[1:]

	argValuesPtr := getArgValueSlice(sig.numIn + 1)
	argValues := *argValuesPtr
	defer func() {
		putArgValueSlice(argValuesPtr, sig.numIn+1)
	}()

	if sig.hasContext {
		argValues = append(argValues, reflect.ValueOf(ctx))
	}
	if sig.hasKwargs {
		argValues = append(argValues, reflect.ValueOf(kwargs))
	}

	argIndex := 0
	expectedArgs := sig.maxPosArgs

	for i := 0; i < expectedArgs; i++ {
		fnParamIndex := i + sig.paramOffset

		if sig.isVariadic && fnParamIndex == sig.variadicIndex {
			elemType := sig.paramTypes[fnParamIndex].Elem()
			for j := argIndex; j < len(methodArgs); j++ {
				val, convErr := convertObjectToValue(methodArgs[j], elemType)
				if convErr != nil {
					return convErr
				}
				argValues = append(argValues, val)
			}
			break
		}

		if argIndex >= len(methodArgs) {
			return newArgumentError(len(methodArgs), expectedArgs)
		}

		val, convErr := convertObjectToValue(methodArgs[argIndex], sig.paramTypes[fnParamIndex])
		if convErr != nil {
			return convErr
		}
		argValues = append(argValues, val)
		argIndex++
	}

	if argIndex < len(methodArgs) && !sig.isVariadic {
		return newArgumentError(len(methodArgs), expectedArgs)
	}

	results := fnValue.Call(argValues)

	switch sig.numOut {
	case 0:
		return &Null{}
	case 1:
		receiverPtr := results[0]
		if !receiverPtr.IsValid() || receiverPtr.IsNil() {
			return newError("constructor returned nil")
		}
		instance.SetField(receiverField, &ClientWrapper{
			TypeName: cb.name,
			Client:   receiverPtr.Interface(),
		})
		return &Null{}
	case 2:
		if sig.returnIsError && !results[1].IsNil() {
			err, _ := results[1].Interface().(error)
			return newError("%s", err.Error())
		}
		receiverPtr := results[0]
		if !receiverPtr.IsValid() || receiverPtr.IsNil() {
			return newError("constructor returned nil")
		}
		instance.SetField(receiverField, &ClientWrapper{
			TypeName: cb.name,
			Client:   receiverPtr.Interface(),
		})
		return &Null{}
	default:
		return newError("constructor can return at most 2 values")
	}
}

// createTypedReceiverWrapper creates a wrapper for methods with a typed receiver.
func (cb *ClassBuilder) createTypedReceiverWrapper(fnValue reflect.Value, sig *FunctionSignature, helpText string) *Builtin {
	return &Builtin{
		Fn: func(ctx context.Context, kwargs Kwargs, args ...Object) (result Object) {
			return cb.callTypedReceiverMethod(fnValue, sig, ctx, kwargs, args)
		},
		HelpText: helpText,
	}
}

// callTypedReceiverMethod calls a method with a typed receiver (e.g., *Config).
func (cb *ClassBuilder) callTypedReceiverMethod(fnValue reflect.Value, sig *FunctionSignature, ctx context.Context, kwargs Kwargs, args []Object) (result Object) {
	defer func() {
		if r := recover(); r != nil {
			result = newError("panic in method: %v", r)
		}
	}()

	if len(args) == 0 {
		return newError("method call requires at least one argument (instance)")
	}

	instance, ok := args[0].(*Instance)
	if !ok {
		return newError("first argument must be an instance, got %T", args[0])
	}

	wrapper, ok := instance.Field(receiverField).(*ClientWrapper)
	if !ok {
		return newError("instance has no typed receiver")
	}

	receiverValue := reflect.ValueOf(wrapper.Client)
	if receiverValue.Type() != cb.receiverType {
		return newError("receiver type mismatch: expected %v, got %v", cb.receiverType, receiverValue.Type())
	}

	methodArgs := args[1:]

	argValuesPtr := getArgValueSlice(sig.numIn)
	argValues := *argValuesPtr
	defer func() {
		putArgValueSlice(argValuesPtr, sig.numIn)
	}()

	argValues = append(argValues, receiverValue)

	if sig.hasContext {
		argValues = append(argValues, reflect.ValueOf(ctx))
	}
	if sig.hasKwargs {
		argValues = append(argValues, reflect.ValueOf(kwargs))
	}

	argIndex := 0
	expectedArgs := sig.maxPosArgs

	for i := 0; i < expectedArgs; i++ {
		fnParamIndex := i + sig.paramOffset

		if sig.isVariadic && fnParamIndex == sig.variadicIndex {
			elemType := sig.paramTypes[fnParamIndex].Elem()
			for j := argIndex; j < len(methodArgs); j++ {
				val, convErr := convertObjectToValue(methodArgs[j], elemType)
				if convErr != nil {
					return convErr
				}
				argValues = append(argValues, val)
			}
			break
		}

		if argIndex >= len(methodArgs) {
			return newArgumentError(len(methodArgs), expectedArgs)
		}

		val, convErr := convertObjectToValue(methodArgs[argIndex], sig.paramTypes[fnParamIndex])
		if convErr != nil {
			return convErr
		}
		argValues = append(argValues, val)
		argIndex++
	}

	if argIndex < len(methodArgs) && !sig.isVariadic {
		return newArgumentError(len(methodArgs), expectedArgs)
	}

	results := fnValue.Call(argValues)

	switch sig.numOut {
	case 0:
		return &Null{}
	case 1:
		return convertReturnValue(results[0])
	case 2:
		if sig.returnIsError && !results[1].IsNil() {
			err, _ := results[1].Interface().(error)
			return newError("%s", err.Error())
		}
		return convertReturnValue(results[0])
	default:
		return newError("method can return at most 2 values")
	}
}

// callTypedMethod calls a typed Go method with cached signature info.
// For class methods, self is ALWAYS first, then: [context], [kwargs], ...args
func (cb *ClassBuilder) callTypedMethod(fnValue reflect.Value, sig *FunctionSignature, ctx context.Context, kwargs Kwargs, args []Object) (result Object) {
	if len(args) == 0 {
		return newError("method call requires at least one argument (instance)")
	}

	instance, ok := args[0].(*Instance)
	if !ok {
		return newError("first argument must be an instance, got %T", args[0])
	}
	methodArgs := args[1:]

	argValuesPtr := getArgValueSlice(sig.numIn)
	argValues := *argValuesPtr
	defer func() {
		putArgValueSlice(argValuesPtr, sig.numIn)
		if r := recover(); r != nil {
			result = newError("panic in method: %v", r)
		}
	}()

	argValues = append(argValues, reflect.ValueOf(instance))

	if sig.hasContext {
		argValues = append(argValues, reflect.ValueOf(ctx))
	}

	if sig.hasKwargs {
		argValues = append(argValues, reflect.ValueOf(kwargs))
	}

	argIndex := 0
	expectedArgs := sig.maxPosArgs

	for i := 0; i < expectedArgs; i++ {
		fnParamIndex := i + sig.paramOffset

		if sig.isVariadic && fnParamIndex == sig.variadicIndex {
			elemType := sig.paramTypes[fnParamIndex].Elem()
			for j := argIndex; j < len(methodArgs); j++ {
				val, convErr := convertObjectToValue(methodArgs[j], elemType)
				if convErr != nil {
					return convErr
				}
				argValues = append(argValues, val)
			}
			break
		}

		if argIndex >= len(methodArgs) {
			return newArgumentError(len(methodArgs), expectedArgs)
		}

		val, convErr := convertObjectToValue(methodArgs[argIndex], sig.paramTypes[fnParamIndex])
		if convErr != nil {
			return convErr
		}
		argValues = append(argValues, val)
		argIndex++
	}

	if argIndex < len(methodArgs) && !sig.isVariadic {
		return newArgumentError(len(methodArgs), expectedArgs)
	}

	results := fnValue.Call(argValues)

	switch sig.numOut {
	case 0:
		return &Null{}
	case 1:
		return convertReturnValue(results[0])
	case 2:
		if sig.returnIsError && !results[1].IsNil() {
			err, _ := results[1].Interface().(error)
			return newError("%s", err.Error())
		}
		return convertReturnValue(results[0])
	default:
		return newError("method can return at most 2 values")
	}
}

// analyzeConstructorSignature analyzes a constructor function signature.
// Constructors take: [context], [kwargs], ...args and return a pointer type.
func analyzeConstructorSignature(fnType reflect.Type) *FunctionSignature {
	numIn := fnType.NumIn()
	numOut := fnType.NumOut()
	isVariadic := fnType.IsVariadic()
	variadicIndex := -1
	if isVariadic {
		variadicIndex = numIn - 1
	}

	paramOffset := 0
	hasContext := numIn > paramOffset && fnType.In(paramOffset) == contextType
	if hasContext {
		paramOffset++
	}
	hasKwargs := numIn > paramOffset && fnType.In(paramOffset) == kwargsType
	if hasKwargs {
		paramOffset++
	}
	maxPosArgs := numIn - paramOffset

	paramTypes := make([]reflect.Type, numIn)
	for i := 0; i < numIn; i++ {
		paramTypes[i] = fnType.In(i)
	}

	returnIsError := numOut == 2 && fnType.Out(1).Implements(errorType)

	sig := &FunctionSignature{
		numIn:         numIn,
		numOut:        numOut,
		isVariadic:    isVariadic,
		variadicIndex: variadicIndex,
		hasContext:    hasContext,
		hasKwargs:     hasKwargs,
		paramOffset:   paramOffset,
		maxPosArgs:    maxPosArgs,
		paramTypes:    paramTypes,
		returnIsError: returnIsError,
	}

	return sig
}

// analyzeClassMethodSignature analyzes a function signature for class methods.
// For class methods: self (required), [context], [kwargs], ...args
// Valid signatures:
//   - func(self *Instance, ...args)        — manual Fields management
//   - func(self *Instance, ctx context.Context, ...args)
//   - func(self *Instance, kwargs Kwargs, ...args)
//   - func(self *Instance, ctx context.Context, kwargs Kwargs, ...args)
//   - func(self *T, ...args)              — typed receiver (T must match Constructor return)
//   - func(self *T, ctx context.Context, ...args)
//   - func(self *T, kwargs Kwargs, ...args)
//   - func(self *T, ctx context.Context, kwargs Kwargs, ...args)
func analyzeClassMethodSignature(fnType reflect.Type, receiverType reflect.Type) *FunctionSignature {
	if cached, ok := signatureCache.Load(fnType); ok {
		return cached.(*FunctionSignature)
	}

	numIn := fnType.NumIn()
	numOut := fnType.NumOut()
	isVariadic := fnType.IsVariadic()
	variadicIndex := -1
	if isVariadic {
		variadicIndex = numIn - 1
	}

	if numIn == 0 {
		panic("class method must have at least one parameter (receiver)")
	}

	firstParam := fnType.In(0)
	typedReceiver := firstParam != instanceType

	if !typedReceiver && firstParam != instanceType {
		panic(fmt.Sprintf("class method must have *Instance or typed pointer as first parameter, got %v", firstParam))
	}

	if typedReceiver && receiverType != nil && firstParam != receiverType {
		panic(fmt.Sprintf("typed receiver mismatch: constructor uses %v but method uses %v", receiverType, firstParam))
	}

	paramOffset := 1

	hasContext := numIn > paramOffset && fnType.In(paramOffset) == contextType
	if hasContext {
		paramOffset++
	}

	hasKwargs := numIn > paramOffset && fnType.In(paramOffset) == kwargsType
	if hasKwargs {
		paramOffset++
	}
	maxPosArgs := numIn - paramOffset

	paramTypes := make([]reflect.Type, numIn)
	for i := 0; i < numIn; i++ {
		paramTypes[i] = fnType.In(i)
	}

	returnIsError := numOut == 2 && fnType.Out(1).Implements(errorType)

	sig := &FunctionSignature{
		numIn:          numIn,
		numOut:         numOut,
		isVariadic:     isVariadic,
		variadicIndex:  variadicIndex,
		hasContext:     hasContext,
		hasKwargs:      hasKwargs,
		paramOffset:    paramOffset,
		maxPosArgs:     maxPosArgs,
		paramTypes:     paramTypes,
		returnIsError:  returnIsError,
		typedReceiver:  typedReceiver,
	}

	signatureCache.Store(fnType, sig)
	return sig
}
