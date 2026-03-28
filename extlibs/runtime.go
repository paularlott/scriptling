package extlibs

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/evaliface"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/snapshotkv"
)

const RuntimeLibraryName = "scriptling.runtime"

// RuntimeState holds all runtime state
var RuntimeState = struct {
	sync.RWMutex

	// HTTP routes
	Routes     map[string]*RouteInfo
	Middleware string

	// WebSocket routes and connections
	WebSocketRoutes      map[string]*WebSocketRouteInfo
	WebSocketConnections map[string]*WebSocketServerConn

	// Background tasks
	Backgrounds       map[string]string                   // name -> "function_name"
	BackgroundArgs    map[string][]object.Object          // name -> args
	BackgroundKwargs  map[string]map[string]object.Object // name -> kwargs
	BackgroundEnvs    map[string]*object.Environment      // name -> environment
	BackgroundEvals   map[string]evaliface.Evaluator      // name -> evaluator
	BackgroundFactory SandboxFactory // Factory to create new Scriptling instances
	BackgroundCtxs  map[string]context.Context // name -> context
	BackgroundReady bool                       // If true, start tasks immediately

	// KV store
	KVDB *snapshotkv.DB

	// Sync primitives
	WaitGroups map[string]*RuntimeWaitGroup
	Queues     map[string]*RuntimeQueue
	Atomics    map[string]*RuntimeAtomic
	Shareds    map[string]*RuntimeShared

	// Cleanup functions registered by libraries
	cleanupFuncs []func()
}{
	Routes:               make(map[string]*RouteInfo),
	WebSocketRoutes:      make(map[string]*WebSocketRouteInfo),
	WebSocketConnections: make(map[string]*WebSocketServerConn),
	Backgrounds:       make(map[string]string),
	BackgroundArgs:    make(map[string][]object.Object),
	BackgroundKwargs:  make(map[string]map[string]object.Object),
	BackgroundEnvs:    make(map[string]*object.Environment),
	BackgroundEvals:   make(map[string]evaliface.Evaluator),
	BackgroundFactory: nil,
	BackgroundCtxs:    make(map[string]context.Context),
	BackgroundReady:   false,
	KVDB:              nil,
	WaitGroups:        make(map[string]*RuntimeWaitGroup),
	Queues:            make(map[string]*RuntimeQueue),
	Atomics:           make(map[string]*RuntimeAtomic),
	Shareds:           make(map[string]*RuntimeShared),
}

// RegisterCleanup registers a function to be called during ResetRuntime.
// Libraries use this to clean up their own state without creating
// dependencies between packages.
func RegisterCleanup(fn func()) {
	RuntimeState.Lock()
	RuntimeState.cleanupFuncs = append(RuntimeState.cleanupFuncs, fn)
	RuntimeState.Unlock()
}

// ResetRuntime clears all runtime state (for testing or re-initialization)
func ResetRuntime() {
	// Run library cleanup functions before acquiring the lock, as they
	// manage their own synchronisation.
	RuntimeState.Lock()
	cleanups := RuntimeState.cleanupFuncs
	RuntimeState.Unlock()
	for _, fn := range cleanups {
		fn()
	}

	RuntimeState.Lock()
	defer RuntimeState.Unlock()

	RuntimeState.Routes = make(map[string]*RouteInfo)
	RuntimeState.Middleware = ""

	// Close all WebSocket connections
	for _, conn := range RuntimeState.WebSocketConnections {
		conn.Close()
	}
	RuntimeState.WebSocketRoutes = make(map[string]*WebSocketRouteInfo)
	RuntimeState.WebSocketConnections = make(map[string]*WebSocketServerConn)

	RuntimeState.Backgrounds = make(map[string]string)
	RuntimeState.BackgroundArgs = make(map[string][]object.Object)
	RuntimeState.BackgroundKwargs = make(map[string]map[string]object.Object)
	RuntimeState.BackgroundEnvs = make(map[string]*object.Environment)
	RuntimeState.BackgroundEvals = make(map[string]evaliface.Evaluator)
	RuntimeState.BackgroundFactory = nil
	RuntimeState.BackgroundCtxs = make(map[string]context.Context)
	RuntimeState.BackgroundReady = false

	// Close and reset KV store, then reinitialize in-memory
	if RuntimeState.KVDB != nil {
		RuntimeState.KVDB.Close()
		RuntimeState.KVDB = nil
	}
	if db, err := snapshotkv.Open("", nil); err == nil {
		RuntimeState.KVDB = db
	}

	RuntimeState.WaitGroups = make(map[string]*RuntimeWaitGroup)
	RuntimeState.Queues = make(map[string]*RuntimeQueue)
	RuntimeState.Atomics = make(map[string]*RuntimeAtomic)
	RuntimeState.Shareds = make(map[string]*RuntimeShared)
	RuntimeState.cleanupFuncs = nil
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

// SetBackgroundFactory sets the factory function for creating Scriptling instances in background tasks.
// Deprecated: Use SetSandboxFactory instead, which sets the factory for both sandbox and background use.
func SetBackgroundFactory(factory SandboxFactory) {
	RuntimeState.Lock()
	RuntimeState.BackgroundFactory = factory
	RuntimeState.Unlock()
}

// RegisterRuntimeLibrary registers only the core runtime library (background function).
// Sub-libraries (http, kv, sync) must be registered separately if needed.
func RegisterRuntimeLibrary(registrar interface{ RegisterLibrary(*object.Library) }) {
	registrar.RegisterLibrary(RuntimeLibraryCore)
}

// RegisterRuntimeLibraryAll registers the runtime library with all sub-libraries,
// including sandbox with the specified allowed paths for exec_file restrictions.
// If allowedPaths is nil, all paths are allowed (no restrictions).
// If allowedPaths is empty slice, no paths are allowed (deny all).
func RegisterRuntimeLibraryAll(registrar interface{ RegisterLibrary(*object.Library) }, allowedPaths []string) {
	httpLib := HTTPSubLibrary
	kvLib := NewKVSubLibrary()
	syncLib := SyncSubLibrary
	sandboxLib := NewSandboxLibrary(allowedPaths)

	// Register each sub-library independently under its full name
	registrar.RegisterLibrary(httpLib)
	registrar.RegisterLibrary(kvLib)
	registrar.RegisterLibrary(syncLib)
	registrar.RegisterLibrary(sandboxLib)

	// Register the parent with sub-library dicts as constants so
	// `import scriptling.runtime as rt; rt.kv.open(...)` keeps working.
	parent := object.NewLibrary(RuntimeLibraryName, RuntimeLibraryFunctions,
		map[string]object.Object{
			"http":    httpLib.GetDict(),
			"kv":      kvLib.GetDict(),
			"sync":    syncLib.GetDict(),
			"sandbox": sandboxLib.GetDict(),
		},
		"Runtime library for HTTP, KV store, concurrency primitives, and sandboxed execution")
	registrar.RegisterLibrary(parent)
}

func RegisterRuntimeHTTPLibrary(registrar interface{ RegisterLibrary(*object.Library) }) {
	registrar.RegisterLibrary(HTTPSubLibrary)
}

func RegisterRuntimeKVLibrary(registrar interface{ RegisterLibrary(*object.Library) }) {
	registrar.RegisterLibrary(NewKVSubLibrary())
}

// RegisterRuntimeKVLibraryWithSecurity registers the kv library restricted to allowedPaths.
// In-memory stores are always permitted regardless of allowedPaths.
// If allowedPaths is nil, all paths are allowed. If empty slice, all filesystem paths are denied.
func RegisterRuntimeKVLibraryWithSecurity(registrar interface{ RegisterLibrary(*object.Library) }, allowedPaths []string) {
	registrar.RegisterLibrary(NewKVSubLibraryWithSecurity(allowedPaths))
}

func RegisterRuntimeSyncLibrary(registrar interface{ RegisterLibrary(*object.Library) }) {
	registrar.RegisterLibrary(SyncSubLibrary)
}

// RegisterRuntimeSandboxLibrary registers the sandbox library with the specified allowed paths.
// If allowedPaths is nil, all paths are allowed (no restrictions).
// If allowedPaths is empty slice, no paths are allowed (deny all).
func RegisterRuntimeSandboxLibrary(registrar interface{ RegisterLibrary(*object.Library) }, allowedPaths []string) {
	registrar.RegisterLibrary(NewSandboxLibrary(allowedPaths))
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
				return startBackgroundTask(handler, fnArgs, fnKwargs, env, eval, factory, ctx)
			}

			return &object.Null{}
		},
		HelpText: `background(name, handler, *args, **kwargs) - Start a fire-and-forget background task

Starts a background task in a goroutine and returns immediately.
Returns null on success, or an error if the handler is not found.

  Both handler patterns run in isolated environments with no
  access to the calling script's data.

Parameters:
  name (string): Unique name for the background task
  handler (string): Function name or "library.function"
    "func_name" - runs in isolated env with import support and sibling functions
    "lib.func" - loads library.function in a new Scriptling instance
  *args: Positional arguments to pass to the function
  **kwargs: Keyword arguments to pass to the function

Returns:
  null on success, error if handler validation fails

  Background tasks are fire-and-forget. Pass data via
  *args/**kwargs or use runtime.sync primitives
  (Shared, Atomic, Queue) for coordination. Access panels
  via console.Console().panel("name") from background tasks.

`,
	},
}

// RuntimeLibraryCore is the runtime library without sub-libraries
var RuntimeLibraryCore = object.NewLibrary(RuntimeLibraryName, RuntimeLibraryFunctions, nil, "Runtime library for background tasks")

// startBackgroundTask starts a background task in a goroutine and returns a Promise.
func startBackgroundTask(handler string, fnArgs []object.Object, fnKwargs map[string]object.Object, env *object.Environment, eval evaliface.Evaluator, factory SandboxFactory, ctx context.Context) object.Object {
	if env == nil || eval == nil {
		return &object.Null{}
	}

	promise := newPromise()

	// For simple (non-dotted) handlers, validate the function exists synchronously.
	isDotted := strings.Contains(handler, ".")
	if !isDotted {
		fn, _ := env.Get(handler)
		if fn == nil {
			return errors.NewError("function not found: %s", handler)
		}
		switch fn.(type) {
		case *object.Function, *object.LambdaFunction:
			// ok
		default:
			return errors.NewError("handler is not a function: %s (%T)", handler, fn)
		}
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				promise.set(nil, fmt.Errorf("panic: %v", r))
			}
		}()

		if isDotted {
			// Handler is "library.function" — load into new instance
			parts := strings.SplitN(handler, ".", 2)
			libName := parts[0]
			funcName := parts[1]

			if factory == nil {
				promise.set(nil, fmt.Errorf("cannot load library: no factory configured"))
				return
			}
			scriptling := factory()
			if scriptling == nil {
				promise.set(nil, fmt.Errorf("factory returned nil"))
				return
			}

			newEnv := object.NewEnvironment()
			if err := scriptling.LoadLibraryIntoEnv(libName, newEnv); err != nil {
				promise.set(nil, fmt.Errorf("failed to load library %s: %v", libName, err))
				return
			}

			var fn object.Object
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

			result := eval.CallObjectFunction(ctx, fn, fnArgs, fnKwargs, newEnv)
			if errObj, ok := result.(*object.Error); ok {
				promise.set(nil, fmt.Errorf("%s", errObj.Message))
			} else {
				promise.set(result, nil)
			}
		} else {
			// Simple function name — create clean environment via factory
			if factory == nil {
				promise.set(nil, fmt.Errorf("no factory configured"))
				return
			}
			scriptling := factory()
			if scriptling == nil {
				promise.set(nil, fmt.Errorf("factory returned nil"))
				return
			}

			newEnv := object.NewEnvironment()

			// Set up import callback so the function can import libraries
			newEnv.SetImportCallback(func(libName string) error {
				return scriptling.LoadLibraryIntoEnv(libName, newEnv)
			})

			// Copy the caller's global scope into the clean env:
			// - Functions get fresh copies bound to newEnv so closures work
			// - Everything else (Builtins, Dicts, primitives, lists, etc.) is
			//   shared directly — builtins are stateless or manage their own
			//   synchronisation, and sharing lets tasks use wg/task_status etc.
			globalEnv := env.GetGlobal()
			store := globalEnv.GetStore()
			for k, v := range store {
				switch origFn := v.(type) {
				case *object.Function:
					newEnv.Set(k, &object.Function{
						Name:          origFn.Name,
						Parameters:    origFn.Parameters,
						DefaultValues: origFn.DefaultValues,
						Variadic:      origFn.Variadic,
						Kwargs:        origFn.Kwargs,
						Body:          origFn.Body,
						Env:           newEnv,
					})
				case *object.LambdaFunction:
					newEnv.Set(k, &object.LambdaFunction{
						Parameters:    origFn.Parameters,
						DefaultValues: origFn.DefaultValues,
						Variadic:      origFn.Variadic,
						Kwargs:        origFn.Kwargs,
						Body:          origFn.Body,
						Env:           newEnv,
					})
				case *object.Class:
					// skip — classes can't be safely shared across envs
				default:
					newEnv.Set(k, origFn)
				}
			}

			fn, _ := newEnv.Get(handler)
			if fn == nil {
				promise.set(nil, fmt.Errorf("function not found: %s", handler))
				return
			}

			result := eval.CallObjectFunction(ctx, fn, fnArgs, fnKwargs, newEnv)
			if errObj, ok := result.(*object.Error); ok {
				promise.set(nil, fmt.Errorf("%s", errObj.Message))
			} else {
				promise.set(result, nil)
			}
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
					if result == nil {
						return &object.Null{}
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
		HelpText: "Promise - call .get() to retrieve result or .wait() to wait without result",
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
			startBackgroundTask(t.handler, t.args, t.kwargs, t.env, t.eval, factory, t.ctx)
		}(name, task)
	}
}
