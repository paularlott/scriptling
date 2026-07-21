package extlibs

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/evaliface"
	"github.com/paularlott/scriptling/evaluator"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/snapshotkv"
)

const RuntimeLibraryName = "scriptling.runtime"

// RuntimeState holds all runtime state
var RuntimeState = struct {
	sync.RWMutex

	// HTTP routes
	Routes          map[string]*RouteInfo
	Middleware      string
	NotFoundHandler string

	// JSON-RPC stdio methods and notifications (name -> "library.function")
	JSONRPCMethods       map[string]string
	JSONRPCNotifications map[string]string

	// WebSocket routes and connections
	WebSocketRoutes      map[string]*WebSocketRouteInfo
	WebSocketConnections map[string]*WebSocketServerConn

	// Background tasks
	Backgrounds       map[string]string                   // name -> "function_name"
	BackgroundArgs    map[string][]object.Object          // name -> args
	BackgroundKwargs  map[string]map[string]object.Object // name -> kwargs
	BackgroundEnvs    map[string]*object.Environment      // name -> environment
	BackgroundEvals   map[string]evaliface.Evaluator      // name -> evaluator
	BackgroundFactory SandboxFactory                      // Factory to create new Scriptling instances
	BackgroundCtxs    map[string]context.Context          // name -> context
	BackgroundReady   bool                                // If true, start tasks immediately

	// KV store
	KVDB *snapshotkv.DB

	// Sync primitives
	WaitGroups map[string]*RuntimeWaitGroup
	Queues     map[string]*RuntimeQueue
	Atomics    map[string]*RuntimeAtomic
	Shareds    map[string]*RuntimeShared

	// Server lifecycle channels (nil in script mode)
	ServerStartCh   chan struct{} // closed by start_server() to signal server is ready
	ServerRunningCh chan struct{} // closed by server on shutdown
	ServerStarted   bool          // prevents double-close of ServerStartCh
	ServerCollect   func()        // set by NewServer; called inside start_server() to snapshot routes atomically

	// Plugin server registration (set via runtime.plugin, agent variant only)
	PluginName        string
	PluginVersion     string
	PluginDescription string
	PluginFunctions   map[string]string        // function name → "library.function" handler
	PluginConstants   map[string]object.Object // constant name → value
	PluginClasses     map[string]string        // exposed class name → "library.ClassName" handler

	// Cleanup functions registered by libraries
	cleanupFuncs []func()
}{
	Routes:               make(map[string]*RouteInfo),
	NotFoundHandler:      "",
	JSONRPCMethods:       make(map[string]string),
	JSONRPCNotifications: make(map[string]string),
	WebSocketRoutes:      make(map[string]*WebSocketRouteInfo),
	WebSocketConnections: make(map[string]*WebSocketServerConn),
	Backgrounds:          make(map[string]string),
	BackgroundArgs:       make(map[string][]object.Object),
	BackgroundKwargs:     make(map[string]map[string]object.Object),
	BackgroundEnvs:       make(map[string]*object.Environment),
	BackgroundEvals:      make(map[string]evaliface.Evaluator),
	BackgroundFactory:    nil,
	BackgroundCtxs:       make(map[string]context.Context),
	BackgroundReady:      false,
	KVDB:                 nil,
	WaitGroups:           make(map[string]*RuntimeWaitGroup),
	Queues:               make(map[string]*RuntimeQueue),
	Atomics:              make(map[string]*RuntimeAtomic),
	Shareds:              make(map[string]*RuntimeShared),
	ServerStartCh:        nil,
	ServerRunningCh:      nil,
	ServerStarted:        false,
	PluginFunctions:      make(map[string]string),
	PluginConstants:      make(map[string]object.Object),
	PluginClasses:        make(map[string]string),
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
	RuntimeState.NotFoundHandler = ""

	RuntimeState.JSONRPCMethods = make(map[string]string)
	RuntimeState.JSONRPCNotifications = make(map[string]string)

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

	RuntimeState.ServerStartCh = nil
	RuntimeState.ServerRunningCh = nil
	RuntimeState.ServerStarted = false
	RuntimeState.ServerCollect = nil

	RuntimeState.PluginName = ""
	RuntimeState.PluginVersion = ""
	RuntimeState.PluginDescription = ""
	RuntimeState.PluginFunctions = make(map[string]string)
	RuntimeState.PluginConstants = make(map[string]object.Object)
	RuntimeState.PluginClasses = make(map[string]string)
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

// runtimeParentLibraries maps each registrar (one per evaluator) to the parent
// runtime library it created. RegisterRuntimePluginLibrary uses this to inject
// "plugin" into the per-evaluator runtime dict so that
// `import scriptling.runtime as rt; rt.plugin.*` works.
// sync.Map is used because multiple handler evaluators may call
// RegisterRuntimeLibraryAll concurrently. Each entry is removed by
// RegisterRuntimePluginLibrary (LoadAndDelete), so there is no leak.
var runtimeParentLibraries sync.Map // key: registrar interface value → *object.Library

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
	jsonrpcLib := JSONRPCSubLibrary
	mcpLib := MCPSubLibrary

	// Register each sub-library independently under its full name
	registrar.RegisterLibrary(httpLib)
	registrar.RegisterLibrary(kvLib)
	registrar.RegisterLibrary(syncLib)
	registrar.RegisterLibrary(sandboxLib)
	registrar.RegisterLibrary(jsonrpcLib)
	registrar.RegisterLibrary(mcpLib)

	// Register the parent with sub-library dicts as constants so
	// `import scriptling.runtime as rt; rt.kv.open(...)` keeps working.
	parent := object.NewLibrary(RuntimeLibraryName, RuntimeLibraryFunctions,
		map[string]object.Object{
			"http":    httpLib.GetDict(),
			"kv":      kvLib.GetDict(),
			"sync":    syncLib.GetDict(),
			"sandbox": sandboxLib.GetDict(),
			"jsonrpc": jsonrpcLib.GetDict(),
			"mcp":     mcpLib.GetDict(),
		},
		"Runtime library for HTTP, JSON-RPC, MCP, KV store, concurrency primitives, and sandboxed execution")
	runtimeParentLibraries.Store(registrar, parent)
	registrar.RegisterLibrary(parent)
}

func RegisterRuntimeHTTPLibrary(registrar interface{ RegisterLibrary(*object.Library) }) {
	registrar.RegisterLibrary(HTTPSubLibrary)
}

// RegisterRuntimeJSONRPCLibrary registers only the jsonrpc sub-library.
func RegisterRuntimeJSONRPCLibrary(registrar interface{ RegisterLibrary(*object.Library) }) {
	registrar.RegisterLibrary(JSONRPCSubLibrary)
}

// RegisterRuntimeMCPLibrary registers only the runtime.mcp sub-library.
func RegisterRuntimeMCPLibrary(registrar interface{ RegisterLibrary(*object.Library) }) {
	registrar.RegisterLibrary(MCPSubLibrary)
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

			env := getEnvFromContext(ctx)
			eval := evaliface.FromContext(ctx)

			// shared=True runs the handler in the caller's own environment (live
			// shared state, GIL-protected) instead of an isolated copy.
			shared := false
			if v := kwargs.Get("shared"); v != nil {
				if b, e := v.AsBool(); e == nil {
					shared = b
				}
			}
			if shared {
				// Pass args/kwargs live; the GIL serializes access. Strip the
				// "shared" control kwarg so it is not forwarded to the handler.
				sharedKwargs := make(map[string]object.Object, len(kwargs.Kwargs))
				for k, v := range kwargs.Kwargs {
					if k == "shared" {
						continue
					}
					sharedKwargs[k] = v
				}
				return startSharedTask(ctx, handler, args[2:], sharedKwargs, env, eval)
			}

			// Validate that all args/kwargs are transferable types
			// (scalars and recursively safe containers only).
			for i, a := range args[2:] {
				if err := object.ValidateTransferable(a); err != nil {
					return errors.NewError("background arg %d: %s", i, err)
				}
			}
			for k, v := range kwargs.Kwargs {
				if err := object.ValidateTransferable(v); err != nil {
					return errors.NewError("background kwarg '%s': %s", k, err)
				}
			}

			// Clone args and kwargs so the background task owns its own copy.
			fnArgs := make([]object.Object, len(args[2:]))
			for i, a := range args[2:] {
				fnArgs[i] = object.CloneObject(a)
			}
			fnKwargs := make(map[string]object.Object, len(kwargs.Kwargs))
			for k, v := range kwargs.Kwargs {
				fnKwargs[k] = object.CloneObject(v)
			}

			RuntimeState.Lock()
			backgroundReady := RuntimeState.BackgroundReady
			factory := RuntimeState.BackgroundFactory
			if !backgroundReady {
				RuntimeState.Backgrounds[name] = handler
				RuntimeState.BackgroundArgs[name] = fnArgs
				RuntimeState.BackgroundKwargs[name] = fnKwargs
				RuntimeState.BackgroundEnvs[name] = env
				RuntimeState.BackgroundEvals[name] = eval
				RuntimeState.BackgroundCtxs[name] = ctx
			}
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
  access to the calling script's data. Only sibling functions
  are copied; data must be passed via args or runtime.sync.

Parameters:
  name (string): Unique name for the background task
  handler (string): Function name or "library.function"
    "func_name" - runs in isolated env with import support and sibling functions
    "lib.func" - loads library.function in a new Scriptling instance
  *args: Positional arguments to pass to the function
  **kwargs: Keyword arguments to pass to the function

Arguments must be transferable types — only simple values and
recursively transferable containers are allowed:
  - Scalars: None, bool, int, float, str
  - Containers: list, dict, set, tuple (elements must also be
    transferable)
  - Not allowed: instances, classes, functions, builtins, or any
    other runtime-backed objects
Arguments are deep-copied before the task starts so the caller and
task cannot race on shared state.

Returns:
  null on success, error if handler validation fails

  Background tasks are fire-and-forget. For coordination between
  tasks use runtime.sync primitives (Shared, Atomic, Queue, WaitGroup).
  Access panels via console.Console().panel("name") from background tasks.

`,
	},

	"start_server": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			wait := true
			if v := kwargs.Get("wait"); v != nil {
				if b, e := v.AsBool(); e == nil {
					wait = b
				}
			}

			// Snapshot routes and close the start channel atomically so that
			// anything registered after start_server() returns is definitively
			// excluded. ServerCollect runs while the lock is held — collectRoutes
			// and collectJSONRPCMethods read RuntimeState fields directly without
			// re-acquiring the lock, so this is safe.
			RuntimeState.Lock()
			if !RuntimeState.ServerStarted && RuntimeState.ServerStartCh != nil {
				RuntimeState.ServerStarted = true
				if RuntimeState.ServerCollect != nil {
					RuntimeState.ServerCollect()
				}
				close(RuntimeState.ServerStartCh)
			}
			RuntimeState.Unlock()

			if wait {
				object.RunBlocking(ctx, func() {
					RuntimeState.RLock()
					ch := RuntimeState.ServerRunningCh
					RuntimeState.RUnlock()
					if ch != nil {
						<-ch
					}
				})
			}
			return &object.Null{}
		},
		HelpText: `start_server(wait=True) - Signal the server to start accepting requests

Signals the server to collect registered routes/methods and begin
listening for requests. Call this after all routes are registered.

Parameters:
  wait (bool, default True): If True, blocks until the server shuts
    down. If False, returns immediately so the script can continue
    running (e.g. to maintain gossip state or run a polling loop).

When wait=True the call blocks until the server receives a shutdown
signal (SIGTERM / Ctrl-C). Use wait=False combined with a
server_running() loop to stay alive while performing other work:

  runtime.start_server(wait=False)
  while runtime.server_running():
      yield_now()

Backward compatibility: scripts that exit without calling
start_server() continue to work — the server starts automatically
after the setup script finishes.

`,
	},

	"server_running": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			RuntimeState.RLock()
			ch := RuntimeState.ServerRunningCh
			RuntimeState.RUnlock()
			if ch == nil {
				return object.NewBoolean(false)
			}
			select {
			case <-ch:
				return object.NewBoolean(false)
			default:
				return object.NewBoolean(true)
			}
		},
		HelpText: `server_running() - Returns True while the server is running

Returns True as long as the server has not received a shutdown signal.
Returns False once the server is shutting down or if called outside
of server mode.

Typical usage with start_server(wait=False):

  runtime.start_server(wait=False)
  while runtime.server_running():
      yield_now()       # release GIL on each iteration

`,
	},
}

// RuntimeLibraryCore is the runtime library without sub-libraries
var RuntimeLibraryCore = object.NewLibrary(RuntimeLibraryName, RuntimeLibraryFunctions, nil, "Runtime library for background tasks")

// taskContext builds a fresh per-goroutine evaluation context for a background
// task. It gives the goroutine its own call-depth tracker (so concurrent tasks
// don't race the parent's recursion counter) and pins the task's environment on
// the context (so blocking builtins release the correct interpreter lock).
// Other parent-context values (evaluator, cancellation) are preserved.
func taskContext(ctx context.Context, env *object.Environment) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx = evaluator.SetEnvInContext(ctx, env)
	return evaluator.SetCallDepthInContext(ctx, evaluator.NewCallDepth(evaluator.DefaultMaxCallDepth))
}

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

	// Snapshot callable bindings before spawning the goroutine so we never
	// read the source Environment from another goroutine.
	var snapshot *object.CallableSnapshot
	if !isDotted {
		snapshot = env.GetGlobal().SnapshotCallables()
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

			result := eval.CallObjectFunction(taskContext(ctx, newEnv), fn, fnArgs, fnKwargs, newEnv)
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

			// Copy only sibling functions into the clean env, rebound to
			// newEnv so closures resolve correctly. No other globals are
			// shared — the task accesses data through validated args and
			// coordination via runtime.sync primitives.
			snapshot.ApplySnapshot(newEnv)

			fn, _ := newEnv.Get(handler)
			if fn == nil {
				promise.set(nil, fmt.Errorf("function not found: %s", handler))
				return
			}

			result := eval.CallObjectFunction(taskContext(ctx, newEnv), fn, fnArgs, fnKwargs, newEnv)
			if errObj, ok := result.(*object.Error); ok {
				promise.set(nil, fmt.Errorf("%s", errObj.Message))
			} else {
				promise.set(result, nil)
			}
		}
	}()

	return promiseObject(promise)
}

// promiseObject wraps a Promise as a script object exposing get()/wait().
func promiseObject(promise *Promise) object.Object {
	return &object.Builtin{
		Attributes: map[string]object.Object{
			"get": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					// Release the interpreter lock while waiting so the task (and
					// any shared-env threads) can run.
					var result object.Object
					var err error
					object.RunBlocking(ctx, func() { result, err = promise.get() })
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
					var err error
					object.RunBlocking(ctx, func() { _, err = promise.get() })
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

// startSharedTask runs handler on a new goroutine in the CALLER's environment,
// sharing its live state. The interpreter lock (GIL) serializes access, so this
// is memory-safe despite the sharing. Unlike background(), args are passed live
// (no transferable restriction, no cloning). A fresh context is used so the
// goroutine acquires the lock instead of inheriting the caller's hold.
func startSharedTask(ctx context.Context, handler string, fnArgs []object.Object, fnKwargs map[string]object.Object, env *object.Environment, eval evaliface.Evaluator) object.Object {
	if env == nil || eval == nil {
		return &object.Null{}
	}
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

	promise := newPromise()
	go func() {
		defer func() {
			if r := recover(); r != nil {
				promise.set(nil, fmt.Errorf("panic: %v", r))
			}
		}()
		result := eval.CallObjectFunction(taskContext(ctx, env), fn, fnArgs, fnKwargs, env)
		if errObj, ok := result.(*object.Error); ok {
			promise.set(nil, fmt.Errorf("%s", errObj.Message))
		} else {
			promise.set(result, nil)
		}
	}()
	return promiseObject(promise)
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
	// Drain the queues so ReleaseBackgroundTasks can't re-launch them
	RuntimeState.Backgrounds = make(map[string]string)
	RuntimeState.BackgroundArgs = make(map[string][]object.Object)
	RuntimeState.BackgroundKwargs = make(map[string]map[string]object.Object)
	RuntimeState.BackgroundEnvs = make(map[string]*object.Environment)
	RuntimeState.BackgroundEvals = make(map[string]evaliface.Evaluator)
	RuntimeState.BackgroundCtxs = make(map[string]context.Context)
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
