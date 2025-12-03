package extlibs

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// RegisterThreadsLibrary registers the threads library with the given registrar
func RegisterThreadsLibrary(registrar interface{ RegisterLibrary(string, *object.Library) }) {
	registrar.RegisterLibrary(ThreadsLibraryName, ThreadsLibrary)
}

// ApplyFunctionFunc is set by the evaluator to allow calling user functions
// This avoids import cycles
var ApplyFunctionFunc func(ctx context.Context, fn object.Object, args []object.Object, kwargs map[string]object.Object, env *object.Environment) object.Object

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

// AtomicInt64 wraps an atomic int64
type AtomicInt64 struct {
	value int64
}

func newAtomicInt64(initial int64) *AtomicInt64 {
	return &AtomicInt64{value: initial}
}

func (a *AtomicInt64) add(delta int64) int64 {
	return atomic.AddInt64(&a.value, delta)
}

func (a *AtomicInt64) get() int64 {
	return atomic.LoadInt64(&a.value)
}

func (a *AtomicInt64) set(val int64) {
	atomic.StoreInt64(&a.value, val)
}

// SharedValue wraps a value with a mutex for thread-safe access
type SharedValue struct {
	mu    sync.RWMutex
	value object.Object
}

func newSharedValue(initial object.Object) *SharedValue {
	return &SharedValue{value: initial}
}

func (s *SharedValue) get() object.Object {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.value
}

func (s *SharedValue) set(val object.Object) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.value = val
}

// WaitGroup is a Go-style wait group
type WaitGroup struct {
	wg sync.WaitGroup
}

func newWaitGroup() *WaitGroup {
	return &WaitGroup{}
}

// Queue is a thread-safe queue
type Queue struct {
	mu      sync.Mutex
	items   []object.Object
	cond    *sync.Cond
	maxsize int
	closed  bool
}

func newQueue(maxsize int) *Queue {
	q := &Queue{
		items:   []object.Object{},
		maxsize: maxsize,
	}
	q.cond = sync.NewCond(&q.mu)
	return q
}

func (q *Queue) put(item object.Object) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return fmt.Errorf("queue is closed")
	}

	// Wait if queue is full (bounded queue)
	for q.maxsize > 0 && len(q.items) >= q.maxsize {
		q.cond.Wait()
	}

	q.items = append(q.items, item)
	q.cond.Signal()
	return nil
}

func (q *Queue) get() (object.Object, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	for len(q.items) == 0 {
		if q.closed {
			return nil, fmt.Errorf("queue is closed")
		}
		q.cond.Wait()
	}

	item := q.items[0]
	q.items = q.items[1:]

	// Signal that space is available
	q.cond.Signal()

	return item, nil
}

func (q *Queue) size() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}

func (q *Queue) close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.closed = true
	q.cond.Broadcast()
}

// Pool is a worker pool with a specific worker function
type Pool struct {
	worker     object.Object
	env        *object.Environment
	ctx        context.Context
	workers    int
	queueDepth int
	tasks      chan object.Object
	wg         sync.WaitGroup
	closeOnce  sync.Once
	closed     bool
	mu         sync.Mutex
}

func newPool(ctx context.Context, worker object.Object, env *object.Environment, workers, queueDepth int) *Pool {
	if queueDepth <= 0 {
		queueDepth = workers * 2 // Default queue depth
	}

	pool := &Pool{
		worker:     worker,
		env:        env,
		ctx:        ctx,
		workers:    workers,
		queueDepth: queueDepth,
		tasks:      make(chan object.Object, queueDepth),
	}

	// Start workers
	for i := 0; i < workers; i++ {
		pool.wg.Add(1)
		go func() {
			defer pool.wg.Done()

			for {
				select {
				case <-ctx.Done():
					return
				case task, ok := <-pool.tasks:
					if !ok {
						return
					}
					// Clone environment for this task
					clonedEnv := cloneEnvironment(env)

					// Call worker function with task data
					if ApplyFunctionFunc != nil {
						ApplyFunctionFunc(ctx, worker, []object.Object{task}, nil, clonedEnv)
					}
				}
			}
		}()
	}

	return pool
}

func (p *Pool) submit(data object.Object) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return fmt.Errorf("pool is closed")
	}

	select {
	case <-p.ctx.Done():
		return fmt.Errorf("context cancelled")
	case p.tasks <- data:
		return nil
	}
}

func (p *Pool) close() {
	p.closeOnce.Do(func() {
		p.mu.Lock()
		p.closed = true
		p.mu.Unlock()
		close(p.tasks)
		p.wg.Wait()
	})
}

// cloneEnvironment creates a deep copy of an environment using object.DeepCopy
// Note: Builtin objects (Atomic, Queue, WaitGroup, Shared, etc.) are NOT deep copied,
// which means the underlying Go data structures are shared between parent and threads.
// This is the desired behavior for thread-safe primitives.
func cloneEnvironment(env *object.Environment) *object.Environment {
	cloned := object.NewEnvironment()

	// Get the store and deep copy each value
	// Builtin objects are returned as-is by DeepCopy, ensuring shared state
	store := env.GetStore()
	for k, v := range store {
		cloned.Set(k, object.DeepCopy(v))
	}

	// Copy callbacks
	cloned.SetImportCallback(env.GetImportCallback())
	cloned.SetAvailableLibrariesCallback(env.GetAvailableLibrariesCallback())

	return cloned
}

// getEnvFromContext retrieves environment from context
func getEnvFromContext(ctx context.Context) *object.Environment {
	// Use the same key as evaluator
	if env, ok := ctx.Value("scriptling-env").(*object.Environment); ok {
		return env
	}
	return object.NewEnvironment() // fallback
}

// ThreadsLibrary provides async execution primitives
var ThreadsLibrary = object.NewLibrary(map[string]*object.Builtin{
	"run": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) < 1 {
				return errors.NewArgumentError(len(args), 1)
			}

			fn := args[0]
			fnArgs := args[1:]

			env := getEnvFromContext(ctx)
			if env == nil {
				return errors.NewError("async.run: no environment in context")
			}

			clonedEnv := cloneEnvironment(env)
			promise := newPromise()

			go func() {
				var result object.Object
				if ApplyFunctionFunc != nil {
					result = ApplyFunctionFunc(ctx, fn, fnArgs, kwargs, clonedEnv)
				} else {
					result = errors.NewError("async library not properly initialized")
				}

				if err, ok := result.(*object.Error); ok {
					promise.set(nil, fmt.Errorf("%s", err.Message))
				} else {
					promise.set(result, nil)
				}
			}()

			// Return Promise object
			return &object.Builtin{
				Attributes: map[string]object.Object{
					"get": &object.Builtin{
						Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
							result, err := promise.get()
							if err != nil {
								return errors.NewError("async error: %v", err)
							}
							return result
						},
						HelpText: "get() - Wait for and return the result",
					},
					"wait": &object.Builtin{
						Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
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
Supports both positional and keyword arguments.

Example:
    def worker(x, y=10):
        return x + y

    promise = async.run(worker, 5, y=3)
    result = promise.get()  # Returns 8
    # Or just wait for completion:
    promise.wait()  # Waits but discards result`,
	},

	"Atomic": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			initial := int64(0)
			if len(args) > 0 {
				if i, ok := args[0].(*object.Integer); ok {
					initial = i.Value
				} else {
					return errors.NewTypeError("INTEGER", args[0].Type().String())
				}
			}

			atomic := newAtomicInt64(initial)

			return &object.Builtin{
				Attributes: map[string]object.Object{
					"add": &object.Builtin{
						Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
							delta := int64(1)
							if len(args) > 0 {
								if d, ok := args[0].(*object.Integer); ok {
									delta = d.Value
								} else {
									return errors.NewTypeError("INTEGER", args[0].Type().String())
								}
							}
							newVal := atomic.add(delta)
							return object.NewInteger(newVal)
						},
						HelpText: "add(delta=1) - Atomically add delta and return new value",
					},
					"get": &object.Builtin{
						Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
							return object.NewInteger(atomic.get())
						},
						HelpText: "get() - Atomically read the value",
					},
					"set": &object.Builtin{
						Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
							if len(args) != 1 {
								return errors.NewArgumentError(len(args), 1)
							}
							if val, ok := args[0].(*object.Integer); ok {
								atomic.set(val.Value)
								return &object.Null{}
							}
							return errors.NewTypeError("INTEGER", args[0].Type().String())
						},
						HelpText: "set(value) - Atomically set the value",
					},
				},
				HelpText: "Atomic integer - lock-free operations",
			}
		},
		HelpText: `Atomic(initial=0) - Create an atomic integer counter

Lock-free atomic operations for high-performance counters.

Example:
    counter = async.Atomic(0)
    counter.add(1)      # Atomic increment
    counter.add(-5)     # Atomic add
    counter.set(100)    # Atomic set
    value = counter.get()  # Atomic read`,
	},

	"Shared": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			var initial object.Object = &object.Null{}
			if len(args) > 0 {
				initial = args[0]
			}

			shared := newSharedValue(initial)

			return &object.Builtin{
				Attributes: map[string]object.Object{
					"get": &object.Builtin{
						Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
							return shared.get()
						},
						HelpText: "get() - Get the current value (thread-safe)",
					},
					"set": &object.Builtin{
						Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
							if len(args) != 1 {
								return errors.NewArgumentError(len(args), 1)
							}
							shared.set(args[0])
							return &object.Null{}
						},
						HelpText: "set(value) - Set the value (thread-safe)",
					},
				},
				HelpText: "Shared variable - thread-safe access with get()/set()",
			}
		},
		HelpText: `Shared(initial_value) - Create a thread-safe shared variable

For complex types that need mutex protection.

Example:
    shared_list = async.Shared([])

    def append_item(item):
        current = shared_list.get()
        current.append(item)
        shared_list.set(current)

    promises = [async.run(append_item, i) for i in range(100)]
    for p in promises:
        p.get()`,
	},

	"WaitGroup": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			wg := newWaitGroup()

			return &object.Builtin{
				Attributes: map[string]object.Object{
					"add": &object.Builtin{
						Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
							delta := int64(1)
							if len(args) > 0 {
								if d, ok := args[0].(*object.Integer); ok {
									delta = d.Value
								}
							}
							wg.wg.Add(int(delta))
							return &object.Null{}
						},
						HelpText: "add(delta=1) - Add to the wait group counter",
					},
					"done": &object.Builtin{
						Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
							wg.wg.Done()
							return &object.Null{}
						},
						HelpText: "done() - Decrement the wait group counter",
					},
					"wait": &object.Builtin{
						Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
							wg.wg.Wait()
							return &object.Null{}
						},
						HelpText: "wait() - Block until counter reaches zero",
					},
				},
				HelpText: "WaitGroup - Go-style synchronization primitive",
			}
		},
		HelpText: `WaitGroup() - Create a wait group for synchronizing goroutines

Example:
    wg = async.WaitGroup()

    def worker(id):
        print(f"Worker {id}")
        wg.done()

    for i in range(10):
        wg.add(1)
        async.run(worker, i)

    wg.wait()`,
	},

	"Queue": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			maxsize := 0 // Unbounded by default
			if len(args) > 0 {
				if m, ok := args[0].(*object.Integer); ok {
					maxsize = int(m.Value)
				}
			}
			if m, ok := kwargs["maxsize"]; ok {
				if mInt, ok := m.(*object.Integer); ok {
					maxsize = int(mInt.Value)
				}
			}

			queue := newQueue(maxsize)

			return &object.Builtin{
				Attributes: map[string]object.Object{
					"put": &object.Builtin{
						Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
							if len(args) != 1 {
								return errors.NewArgumentError(len(args), 1)
							}
							if err := queue.put(args[0]); err != nil {
								return errors.NewError("queue error: %v", err)
							}
							return &object.Null{}
						},
						HelpText: "put(item) - Add item to queue (blocks if full)",
					},
					"get": &object.Builtin{
						Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
							item, err := queue.get()
							if err != nil {
								return errors.NewError("queue error: %v", err)
							}
							return item
						},
						HelpText: "get() - Remove and return item from queue (blocks if empty)",
					},
					"size": &object.Builtin{
						Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
							return object.NewInteger(int64(queue.size()))
						},
						HelpText: "size() - Return number of items in queue",
					},
					"close": &object.Builtin{
						Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
							queue.close()
							return &object.Null{}
						},
						HelpText: "close() - Close the queue",
					},
				},
				HelpText: "Queue - Thread-safe queue for producer-consumer patterns",
			}
		},
		HelpText: `Queue(maxsize=0) - Create a thread-safe queue

maxsize=0 creates unbounded queue, maxsize>0 creates bounded queue.

Example:
    queue = async.Queue(maxsize=100)

    def producer():
        for i in range(10):
            queue.put(i)

    def consumer():
        for i in range(10):
            item = queue.get()
            print(item)

    async.run(producer)
    async.run(consumer)`,
	},

	"Pool": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) < 1 {
				return errors.NewArgumentError(len(args), 1)
			}

			worker := args[0]
			workers := 4
			queueDepth := 0

			if len(args) > 1 {
				if w, ok := args[1].(*object.Integer); ok {
					workers = int(w.Value)
				}
			}
			if w, ok := kwargs["workers"]; ok {
				if wInt, ok := w.(*object.Integer); ok {
					workers = int(wInt.Value)
				}
			}
			if q, ok := kwargs["queue_depth"]; ok {
				if qInt, ok := q.(*object.Integer); ok {
					queueDepth = int(qInt.Value)
				}
			}

			env := getEnvFromContext(ctx)
			if env == nil {
				return errors.NewError("async.Pool: no environment in context")
			}

			pool := newPool(ctx, worker, env, workers, queueDepth)

			return &object.Builtin{
				Attributes: map[string]object.Object{
					"submit": &object.Builtin{
						Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
							if len(args) != 1 {
								return errors.NewArgumentError(len(args), 1)
							}
							if err := pool.submit(args[0]); err != nil {
								return errors.NewError("pool submit error: %v", err)
							}
							return &object.Null{}
						},
						HelpText: "submit(data) - Submit data to pool for processing",
					},
					"close": &object.Builtin{
						Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
							pool.close()
							return &object.Null{}
						},
						HelpText: "close() - Stop accepting work and wait for completion",
					},
				},
				HelpText: "Pool - Worker pool for processing data",
			}
		},
		HelpText: `Pool(worker_func, workers=4, queue_depth=workers*2) - Create a worker pool

worker_func is called with each submitted data item.

Example:
    def process_data(item):
        print(f"Processing {item}")

    pool = async.Pool(process_data, workers=4, queue_depth=1000)
    for i in range(100):
        pool.submit(i)
    pool.close()`,
	},
}, nil, "Asynchronous execution with isolated environments")
