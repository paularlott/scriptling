package extlibs

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/evaliface"
	"github.com/paularlott/scriptling/object"
)

const RuntimeLibraryName = "scriptling.runtime"

// RuntimeState holds all runtime state
var RuntimeState = struct {
	sync.RWMutex

	// HTTP routes
	Routes     map[string]*RouteInfo
	Middleware string

	// Background tasks
	Backgrounds       map[string]string                   // name -> "function_name"
	BackgroundArgs    map[string][]object.Object          // name -> args
	BackgroundKwargs  map[string]map[string]object.Object // name -> kwargs
	BackgroundEnvs    map[string]*object.Environment      // name -> environment
	BackgroundEvals   map[string]evaliface.Evaluator      // name -> evaluator
	BackgroundFactory func() interface {
		LoadLibraryIntoEnv(string, *object.Environment) error
	} // Factory to create new Scriptling instances
	BackgroundCtxs  map[string]context.Context // name -> context
	BackgroundReady bool                       // If true, start tasks immediately

	// KV store
	KVData map[string]*kvEntry

	// Sync primitives
	WaitGroups map[string]*RuntimeWaitGroup
	Queues     map[string]*RuntimeQueue
	Atomics    map[string]*RuntimeAtomic
	Shareds    map[string]*RuntimeShared
}{
	Routes:            make(map[string]*RouteInfo),
	Backgrounds:       make(map[string]string),
	BackgroundArgs:    make(map[string][]object.Object),
	BackgroundKwargs:  make(map[string]map[string]object.Object),
	BackgroundEnvs:    make(map[string]*object.Environment),
	BackgroundEvals:   make(map[string]evaliface.Evaluator),
	BackgroundFactory: nil,
	BackgroundCtxs:    make(map[string]context.Context),
	BackgroundReady:   false,
	KVData:            make(map[string]*kvEntry),
	WaitGroups:        make(map[string]*RuntimeWaitGroup),
	Queues:            make(map[string]*RuntimeQueue),
	Atomics:           make(map[string]*RuntimeAtomic),
	Shareds:           make(map[string]*RuntimeShared),
}

// ResetRuntime clears all runtime state (for testing or re-initialization)
func ResetRuntime() {
	RuntimeState.Lock()
	defer RuntimeState.Unlock()

	RuntimeState.Routes = make(map[string]*RouteInfo)
	RuntimeState.Middleware = ""
	RuntimeState.Backgrounds = make(map[string]string)
	RuntimeState.BackgroundArgs = make(map[string][]object.Object)
	RuntimeState.BackgroundKwargs = make(map[string]map[string]object.Object)
	RuntimeState.BackgroundEnvs = make(map[string]*object.Environment)
	RuntimeState.BackgroundEvals = make(map[string]evaliface.Evaluator)
	RuntimeState.BackgroundFactory = nil
	RuntimeState.BackgroundCtxs = make(map[string]context.Context)
	RuntimeState.BackgroundReady = false
	RuntimeState.KVData = make(map[string]*kvEntry)
	RuntimeState.WaitGroups = make(map[string]*RuntimeWaitGroup)
	RuntimeState.Queues = make(map[string]*RuntimeQueue)
	RuntimeState.Atomics = make(map[string]*RuntimeAtomic)
	RuntimeState.Shareds = make(map[string]*RuntimeShared)

	// Restart the KV cleanup goroutine
	startKVCleanup()
}

// Promise represents an async operation result
type Promise struct {
	mu     sync.Mutex
	done   chan struct{}
	result object.Object
	err    error
}

func newPromise() *Promise {
	return &Promise{done: make(chan struct{})}
}

func (p *Promise) set(result object.Object, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.result = result
	p.err = err
	close(p.done)
}

func (p *Promise) get() (object.Object, error) {
	<-p.done
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.result, p.err
}

// getEnvFromContext retrieves environment from context
func getEnvFromContext(ctx context.Context) *object.Environment {
	if env, ok := ctx.Value("scriptling-env").(*object.Environment); ok {
		return env
	}
	return object.NewEnvironment()
}

// SetBackgroundFactory sets the factory function for creating Scriptling instances in background tasks
func SetBackgroundFactory(factory func() interface {
	LoadLibraryIntoEnv(string, *object.Environment) error
}) {
	RuntimeState.Lock()
	RuntimeState.BackgroundFactory = factory
	RuntimeState.Unlock()
}

// RegisterRuntimeLibrary registers only the core runtime library (background function).
// Sub-libraries (http, kv, sync) must be registered separately if needed.
func RegisterRuntimeLibrary(registrar interface{ RegisterLibrary(*object.Library) }) {
	registrar.RegisterLibrary(RuntimeLibraryCore)
}

// RegisterRuntimeLibraryAll registers the runtime library with all sub-libraries (http, kv, sync).
func RegisterRuntimeLibraryAll(registrar interface{ RegisterLibrary(*object.Library) }) {
	registrar.RegisterLibrary(RuntimeLibraryWithSubs)
}

func RegisterRuntimeHTTPLibrary(registrar interface{ RegisterLibrary(*object.Library) }) {
	registrar.RegisterLibrary(HTTPSubLibrary)
}

func RegisterRuntimeKVLibrary(registrar interface{ RegisterLibrary(*object.Library) }) {
	registrar.RegisterLibrary(KVSubLibrary)
}

func RegisterRuntimeSyncLibrary(registrar interface{ RegisterLibrary(*object.Library) }) {
	registrar.RegisterLibrary(SyncSubLibrary)
}

// RuntimeLibraryFunctions contains the core runtime functions (background)
var RuntimeLibraryFunctions = map[string]*object.Builtin{
	"background": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.MinArgs(args, 2); err != nil {
				return err
			}

			name, err := args[0].AsString()
			if err != nil {
				return err
			}

			handler, err := args[1].AsString()
			if err != nil {
				return err
			}

			// Capture additional args and kwargs
			fnArgs := args[2:]
			fnKwargs := kwargs.Kwargs
			env := getEnvFromContext(ctx)
			eval := evaliface.FromContext(ctx)

			RuntimeState.Lock()
			RuntimeState.Backgrounds[name] = handler
			RuntimeState.BackgroundArgs[name] = fnArgs
			RuntimeState.BackgroundKwargs[name] = fnKwargs
			RuntimeState.BackgroundEnvs[name] = env
			RuntimeState.BackgroundEvals[name] = eval
			RuntimeState.BackgroundCtxs[name] = ctx
			backgroundReady := RuntimeState.BackgroundReady
			factory := RuntimeState.BackgroundFactory
			RuntimeState.Unlock()

			// Start immediately if BackgroundReady is true
			if backgroundReady {
				return startBackgroundTask(name, handler, fnArgs, fnKwargs, env, eval, factory, ctx)
			}

			return &object.Null{}
		},
		HelpText: `background(name, handler, *args, **kwargs) - Register and start a background task

Registers a background task and starts it immediately in a goroutine (unless in server mode).
Returns a Promise object that can be used to wait for completion or get the result.

Parameters:
  name (string): Unique name for the background task
  handler (string): Function name to execute
  *args: Positional arguments to pass to the function
  **kwargs: Keyword arguments to pass to the function

Returns:
  Promise object (in script mode) or null (in server mode)

Example:
  def my_task(x, y, operation="add"):
      if operation == "add":
          return x + y
      return x * y

  promise = runtime.background("calc", "my_task", 10, 5, operation="multiply")
  if promise:
      result = promise.get()  # Returns 50`,
	},
}

// RuntimeLibraryCore is the runtime library without sub-libraries
var RuntimeLibraryCore = object.NewLibrary(RuntimeLibraryName, RuntimeLibraryFunctions, nil, "Runtime library for background tasks")

// RuntimeLibraryWithSubs is the runtime library with all sub-libraries (http, kv, sync)
var RuntimeLibraryWithSubs = object.NewLibraryWithSubs(RuntimeLibraryName, RuntimeLibraryFunctions, nil, map[string]*object.Library{
	"http": HTTPSubLibrary,
	"kv":   KVSubLibrary,
	"sync": SyncSubLibrary,
}, "Runtime library for HTTP, KV store, and concurrency primitives")

// RuntimeLibrary is an alias for RuntimeLibraryWithSubs for backward compatibility
var RuntimeLibrary = RuntimeLibraryWithSubs

// startBackgroundTask starts a single background task with its own isolated Scriptling instance
func startBackgroundTask(name, handler string, fnArgs []object.Object, fnKwargs map[string]object.Object, env *object.Environment, eval evaliface.Evaluator, factory func() interface {
	LoadLibraryIntoEnv(string, *object.Environment) error
}, ctx context.Context) object.Object {
	if env == nil || eval == nil {
		return &object.Null{}
	}

	// For simple (non-dotted) handlers, resolve the function from the environment
	// on the calling goroutine to avoid concurrent map access in the background goroutine.
	var prefetchedFn object.Object
	isDotted := strings.Contains(handler, ".")
	if !isDotted {
		prefetchedFn, _ = env.Get(handler)
		if prefetchedFn == nil {
			return errors.NewError("function not found: %s", handler)
		}
	}

	promise := newPromise()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				promise.set(nil, fmt.Errorf("panic: %v", r))
			}
		}()

		// If handler contains a dot, create new Scriptling instance and import library
		var fn object.Object
		if strings.Contains(handler, ".") {
			// Handler is "library.function" - need to import library
			parts := strings.SplitN(handler, ".", 2)
			libName := parts[0]
			funcName := parts[1]

			// Create new Scriptling instance for this task
			if factory == nil {
				promise.set(nil, fmt.Errorf("cannot load library: no factory configured"))
				return
			}

			scriptling := factory()
			if scriptling == nil {
				promise.set(nil, fmt.Errorf("factory returned nil"))
				return
			}

			// Create new environment and load library into it
			newEnv := object.NewEnvironment()
			if err := scriptling.LoadLibraryIntoEnv(libName, newEnv); err != nil {
				promise.set(nil, fmt.Errorf("failed to load library %s: %v", libName, err))
				return
			}

			// Get the library and function from the new environment
			libObj, ok := newEnv.Get(libName)
			if !ok {
				promise.set(nil, fmt.Errorf("library not found: %s", libName))
				return
			}

			if libDict, ok := libObj.(*object.Dict); ok {
				if pair, exists := libDict.GetByString(funcName); exists {
					fn = pair.Value
				}
			}

			if fn == nil {
				promise.set(nil, fmt.Errorf("function not found: %s.%s", libName, funcName))
				return
			}

			// Call the function with the new environment
			result := eval.CallObjectFunction(ctx, fn, fnArgs, fnKwargs, newEnv)
			if err, ok := result.(*object.Error); ok {
				promise.set(nil, fmt.Errorf("%s", err.Message))
			} else {
				promise.set(result, nil)
			}
			return
		} else {
			// Simple function name - already resolved before goroutine launch
			fn = prefetchedFn
		}

		if fn == nil {
			promise.set(nil, fmt.Errorf("function not found: %s", handler))
			return
		}

		// Create a new isolated environment for the background task
		newEnv := object.NewEnvironment()
		result := eval.CallObjectFunction(ctx, fn, fnArgs, fnKwargs, newEnv)
		if err, ok := result.(*object.Error); ok {
			promise.set(nil, fmt.Errorf("%s", err.Message))
		} else {
			promise.set(result, nil)
		}
	}()

	return &object.Builtin{
		Attributes: map[string]object.Object{
			"get": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					result, err := promise.get()
					if err != nil {
						return errors.NewError("async error: %v", err)
					}
					return result
				},
				HelpText: "get() - Wait for and return the result",
			},
			"wait": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					_, err := promise.get()
					if err != nil {
						return errors.NewError("async error: %v", err)
					}
					return &object.Null{}
				},
				HelpText: "wait() - Wait for completion and discard the result",
			},
		},
		HelpText: "Promise object - call .get() to retrieve result or .wait() to wait without result",
	}
}

// ReleaseBackgroundTasks sets BackgroundReady=true and starts all queued tasks
func ReleaseBackgroundTasks() {
	RuntimeState.Lock()
	RuntimeState.BackgroundReady = true
	factory := RuntimeState.BackgroundFactory
	tasks := make(map[string]struct {
		handler string
		args    []object.Object
		kwargs  map[string]object.Object
		env     *object.Environment
		eval    evaliface.Evaluator
		ctx     context.Context
	})
	for name := range RuntimeState.Backgrounds {
		tasks[name] = struct {
			handler string
			args    []object.Object
			kwargs  map[string]object.Object
			env     *object.Environment
			eval    evaliface.Evaluator
			ctx     context.Context
		}{
			handler: RuntimeState.Backgrounds[name],
			args:    RuntimeState.BackgroundArgs[name],
			kwargs:  RuntimeState.BackgroundKwargs[name],
			env:     RuntimeState.BackgroundEnvs[name],
			eval:    RuntimeState.BackgroundEvals[name],
			ctx:     RuntimeState.BackgroundCtxs[name],
		}
	}
	RuntimeState.Unlock()

	// Start all queued tasks
	for name, task := range tasks {
		go func(n string, t struct {
			handler string
			args    []object.Object
			kwargs  map[string]object.Object
			env     *object.Environment
			eval    evaliface.Evaluator
			ctx     context.Context
		}) {
			startBackgroundTask(n, t.handler, t.args, t.kwargs, t.env, t.eval, factory, t.ctx)
		}(name, task)
	}
}
