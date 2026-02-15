package extlibs

import (
	"context"
	"fmt"
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
	Backgrounds map[string]string // name -> "library.function"

	// KV store
	KVData map[string]*kvEntry

	// Sync primitives
	WaitGroups map[string]*RuntimeWaitGroup
	Queues     map[string]*RuntimeQueue
	Atomics    map[string]*RuntimeAtomic
	Shareds    map[string]*RuntimeShared
}{
	Routes:      make(map[string]*RouteInfo),
	Backgrounds: make(map[string]string),
	KVData:      make(map[string]*kvEntry),
	WaitGroups:  make(map[string]*RuntimeWaitGroup),
	Queues:      make(map[string]*RuntimeQueue),
	Atomics:     make(map[string]*RuntimeAtomic),
	Shareds:     make(map[string]*RuntimeShared),
}

// ResetRuntime clears all runtime state (for testing or re-initialization)
func ResetRuntime() {
	RuntimeState.Lock()
	defer RuntimeState.Unlock()

	RuntimeState.Routes = make(map[string]*RouteInfo)
	RuntimeState.Middleware = ""
	RuntimeState.Backgrounds = make(map[string]string)
	RuntimeState.KVData = make(map[string]*kvEntry)
	RuntimeState.WaitGroups = make(map[string]*RuntimeWaitGroup)
	RuntimeState.Queues = make(map[string]*RuntimeQueue)
	RuntimeState.Atomics = make(map[string]*RuntimeAtomic)
	RuntimeState.Shareds = make(map[string]*RuntimeShared)
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

// cloneEnvironment creates a lightweight environment for threading
func cloneEnvironment(env *object.Environment) *object.Environment {
	cloned := object.NewEnvironment()

	store := env.GetStore()
	for k, v := range store {
		if _, isLib := v.(*object.Library); isLib {
			cloned.Set(k, v)
		}
	}

	cloned.SetImportCallback(env.GetImportCallback())
	cloned.SetAvailableLibrariesCallback(env.GetAvailableLibrariesCallback())

	return cloned
}

func RegisterRuntimeLibrary(registrar interface{ RegisterLibrary(*object.Library) }) {
	registrar.RegisterLibrary(RuntimeLibrary)
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

var RuntimeLibrary = object.NewLibraryWithSubs(RuntimeLibraryName, map[string]*object.Builtin{
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

			RuntimeState.Lock()
			RuntimeState.Backgrounds[name] = handler
			RuntimeState.Unlock()

			return &object.Null{}
		},
		HelpText: `background(name, handler) - Register a background task

Parameters:
  name (string): Unique name for the background task
  handler (string): Handler function as "library.function" string

Example:
  runtime.background("telegram", "bot.start_telegram")
  runtime.background("cleanup", "workers.cleanup_expired")`,
	},

	"run": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.MinArgs(args, 1); err != nil {
				return err
			}

			fn := args[0]
			fnArgs := args[1:]

			env := getEnvFromContext(ctx)
			if env == nil {
				return errors.NewError("runtime.run: no environment in context")
			}

			clonedEnv := cloneEnvironment(env)
			promise := newPromise()

			go func() {
				var result object.Object
				eval := evaliface.FromContext(ctx)
				if eval != nil {
					result = eval.CallObjectFunction(ctx, fn, fnArgs, kwargs.Kwargs, clonedEnv)
				} else {
					result = errors.NewError("evaluator not available in context")
				}

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
		},
		HelpText: `run(func, *args, **kwargs) - Run function asynchronously

Executes function in a separate goroutine with isolated environment.
Returns a Promise object. Call .get() to retrieve the result or .wait() to wait without result.

Example:
    def worker(x, y=10):
        return x + y

    promise = runtime.run(worker, 5, y=3)
    result = promise.get()  # Returns 8`,
	},
}, nil, map[string]*object.Library{
	"http": HTTPSubLibrary,
	"kv":   KVSubLibrary,
	"sync": SyncSubLibrary,
}, "Runtime library for HTTP, KV store, and concurrency primitives")
