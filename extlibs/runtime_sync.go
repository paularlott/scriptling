package extlibs

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/evaliface"
	"github.com/paularlott/scriptling/object"
)

// RuntimeWaitGroup is a named wait group
type RuntimeWaitGroup struct {
	wg sync.WaitGroup
}

// RuntimeQueue is a named thread-safe queue
type RuntimeQueue struct {
	mu      sync.Mutex
	items   []object.Object
	maxsize int
	closed  bool
	putCh   chan struct{} // signals space available for put
	getCh   chan struct{} // signals items available for get
}

func newRuntimeQueue(maxsize int) *RuntimeQueue {
	return &RuntimeQueue{
		items:   []object.Object{},
		maxsize: maxsize,
		putCh:   make(chan struct{}, 1),
		getCh:   make(chan struct{}, 1),
	}
}

// signalGet non-blocking send to getCh to wake a waiting get().
func (q *RuntimeQueue) signalGet() {
	select {
	case q.getCh <- struct{}{}:
	default:
	}
}

// signalPut non-blocking send to putCh to wake a waiting put().
func (q *RuntimeQueue) signalPut() {
	select {
	case q.putCh <- struct{}{}:
	default:
	}
}

func (q *RuntimeQueue) put(ctx context.Context, item object.Object) error {
	for {
		q.mu.Lock()
		if q.closed {
			q.mu.Unlock()
			return fmt.Errorf("queue is closed")
		}
		if q.maxsize <= 0 || len(q.items) < q.maxsize {
			q.items = append(q.items, item)
			q.signalGet()
			q.mu.Unlock()
			return nil
		}
		q.mu.Unlock()

		// Wait for space or context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-q.putCh:
			// Space may be available, retry
		}
	}
}

func (q *RuntimeQueue) get(ctx context.Context) (object.Object, error) {
	for {
		q.mu.Lock()
		if len(q.items) > 0 {
			item := q.items[0]
			q.items = q.items[1:]
			q.signalPut()
			q.mu.Unlock()
			return item, nil
		}
		if q.closed {
			q.mu.Unlock()
			return nil, fmt.Errorf("queue is closed")
		}
		q.mu.Unlock()

		// Wait for items or context cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-q.getCh:
			// Items may be available, retry
		}
	}
}

func (q *RuntimeQueue) size() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}

func (q *RuntimeQueue) close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.closed = true
	// Wake all waiters
	q.signalGet()
	q.signalPut()
}

// RuntimeAtomic is a named atomic counter
type RuntimeAtomic struct {
	value int64
}

func (a *RuntimeAtomic) add(delta int64) int64 {
	return atomic.AddInt64(&a.value, delta)
}

func (a *RuntimeAtomic) get() int64 {
	return atomic.LoadInt64(&a.value)
}

func (a *RuntimeAtomic) set(val int64) {
	atomic.StoreInt64(&a.value, val)
}

// RuntimeShared is a named shared value.
// Values stored should be treated as immutable. Use set() to replace.
// For atomic read-modify-write, use update() with a callback.
type RuntimeShared struct {
	mu    sync.RWMutex
	value object.Object
}

func (s *RuntimeShared) get() object.Object {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.value
}

func (s *RuntimeShared) set(val object.Object) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.value = val
}

// update atomically applies a function to the current value and stores the result.
// The callback receives the current value and must return the new value.
func (s *RuntimeShared) update(fn func(object.Object) object.Object) object.Object {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.value = fn(s.value)
	return s.value
}

var SyncSubLibrary = object.NewLibrary("sync", map[string]*object.Builtin{
	"WaitGroup": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.MinArgs(args, 1); err != nil {
				return err
			}

			name, err := args[0].AsString()
			if err != nil {
				return err
			}

			RuntimeState.Lock()
			wg, exists := RuntimeState.WaitGroups[name]
			if !exists {
				wg = &RuntimeWaitGroup{}
				RuntimeState.WaitGroups[name] = wg
			}
			RuntimeState.Unlock()

			return &object.Builtin{
				Attributes: map[string]object.Object{
					"add": &object.Builtin{
						Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
							delta := int64(1)
							if len(args) > 0 {
								if d, err := args[0].AsInt(); err == nil {
									delta = d
								}
							}
							wg.wg.Add(int(delta))
							return &object.Null{}
						},
						HelpText: "add(delta=1) - Add to the wait group counter",
					},
					"done": &object.Builtin{
						Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
							wg.wg.Done()
							return &object.Null{}
						},
						HelpText: "done() - Decrement the wait group counter",
					},
					"wait": &object.Builtin{
						Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
							wg.wg.Wait()
							return &object.Null{}
						},
						HelpText: "wait() - Block until counter reaches zero",
					},
				},
				HelpText: "WaitGroup - Go-style synchronization primitive",
			}
		},
		HelpText: `WaitGroup(name) - Get or create a named wait group

Parameters:
  name (string): Unique name for the wait group (shared across environments)

Example:
    wg = runtime.sync.WaitGroup("tasks")

    def worker(id):
        print(f"Worker {id}")
        wg.done()

    for i in range(10):
        wg.add(1)
        runtime.run(worker, i)

    wg.wait()`,
	},

	"Queue": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.MinArgs(args, 1); err != nil {
				return err
			}

			name, err := args[0].AsString()
			if err != nil {
				return err
			}

			maxsize := 0
			if len(args) > 1 {
				if m, err := args[1].AsInt(); err == nil {
					maxsize = int(m)
				}
			}
			if m, ok := kwargs.Kwargs["maxsize"]; ok {
				if mInt, err := m.AsInt(); err == nil {
					maxsize = int(mInt)
				}
			}

			RuntimeState.Lock()
			queue, exists := RuntimeState.Queues[name]
			if !exists {
				queue = newRuntimeQueue(maxsize)
				RuntimeState.Queues[name] = queue
			}
			RuntimeState.Unlock()

			return &object.Builtin{
				Attributes: map[string]object.Object{
					"put": &object.Builtin{
						Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
							if err := errors.ExactArgs(args, 1); err != nil {
								return err
							}
							if err := queue.put(ctx, args[0]); err != nil {
								return errors.NewError("queue error: %v", err)
							}
							return &object.Null{}
						},
						HelpText: "put(item) - Add item to queue (blocks if full, respects context timeout)",
					},
					"get": &object.Builtin{
						Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
							item, err := queue.get(ctx)
							if err != nil {
								return errors.NewError("queue error: %v", err)
							}
							return item
						},
						HelpText: "get() - Remove and return item from queue (blocks if empty, respects context timeout)",
					},
					"size": &object.Builtin{
						Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
							return object.NewInteger(int64(queue.size()))
						},
						HelpText: "size() - Return number of items in queue",
					},
					"close": &object.Builtin{
						Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
							queue.close()
							return &object.Null{}
						},
						HelpText: "close() - Close the queue",
					},
				},
				HelpText: "Queue - Thread-safe queue for producer-consumer patterns",
			}
		},
		HelpText: `Queue(name, maxsize=0) - Get or create a named queue

Parameters:
  name (string): Unique name for the queue (shared across environments)
  maxsize (int): Maximum queue size (0 = unbounded)

Example:
    queue = runtime.sync.Queue("jobs", maxsize=100)

    def producer():
        for i in range(10):
            queue.put(i)

    def consumer():
        for i in range(10):
            item = queue.get()
            print(item)

    runtime.run(producer)
    runtime.run(consumer)`,
	},

	"Atomic": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.MinArgs(args, 1); err != nil {
				return err
			}

			name, err := args[0].AsString()
			if err != nil {
				return err
			}

			initial := int64(0)
			if len(args) > 1 {
				if i, err := args[1].AsInt(); err == nil {
					initial = i
				}
			}
			if i := kwargs.Get("initial"); i != nil {
				if iVal, err := i.AsInt(); err == nil {
					initial = iVal
				}
			}

			RuntimeState.Lock()
			atomic, exists := RuntimeState.Atomics[name]
			if !exists {
				atomic = &RuntimeAtomic{value: initial}
				RuntimeState.Atomics[name] = atomic
			}
			RuntimeState.Unlock()

			return &object.Builtin{
				Attributes: map[string]object.Object{
					"add": &object.Builtin{
						Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
							delta := int64(1)
							if len(args) > 0 {
								if d, err := args[0].AsInt(); err == nil {
									delta = d
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
						Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
							return object.NewInteger(atomic.get())
						},
						HelpText: "get() - Atomically read the value",
					},
					"set": &object.Builtin{
						Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
							if err := errors.ExactArgs(args, 1); err != nil {
								return err
							}
							if val, err := args[0].AsInt(); err == nil {
								atomic.set(val)
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
		HelpText: `Atomic(name, initial=0) - Get or create a named atomic counter

Parameters:
  name (string): Unique name for the counter (shared across environments)
  initial (int): Initial value (only used if creating new counter)

Example:
    counter = runtime.sync.Atomic("requests", initial=0)
    counter.add(1)      # Atomic increment
    counter.add(-5)     # Atomic add
    counter.set(100)    # Atomic set
    value = counter.get()  # Atomic read`,
	},

	"Shared": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.MinArgs(args, 1); err != nil {
				return err
			}

			name, err := args[0].AsString()
			if err != nil {
				return err
			}

			var initial object.Object = &object.Null{}
			if len(args) > 1 {
				initial = args[1]
			}
			if i := kwargs.Get("initial"); i != nil {
				initial = i
			}

			RuntimeState.Lock()
			shared, exists := RuntimeState.Shareds[name]
			if !exists {
				shared = &RuntimeShared{value: initial}
				RuntimeState.Shareds[name] = shared
			}
			RuntimeState.Unlock()

			return &object.Builtin{
				Attributes: map[string]object.Object{
					"get": &object.Builtin{
						Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
							return shared.get()
						},
						HelpText: "get() - Get the current value (thread-safe read)",
					},
					"set": &object.Builtin{
						Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
							if err := errors.ExactArgs(args, 1); err != nil {
								return err
							}
							shared.set(args[0])
							return &object.Null{}
						},
						HelpText: "set(value) - Set the value (thread-safe write)",
					},
					"update": &object.Builtin{
						Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
							if err := errors.ExactArgs(args, 1); err != nil {
								return err
							}
							fn := args[0]
							result := shared.update(func(current object.Object) object.Object {
								eval := evaliface.FromContext(ctx)
								if eval == nil {
									return current
								}
								env := getEnvFromContext(ctx)
								return eval.CallObjectFunction(ctx, fn, []object.Object{current}, nil, env)
							})
							return result
						},
						HelpText: "update(fn) - Atomically read-modify-write: fn receives current value, returns new value",
					},
				},
				HelpText: "Shared variable - thread-safe access with get()/set()/update()",
			}
		},
		HelpText: `Shared(name, initial) - Get or create a named shared variable

Parameters:
  name (string): Unique name for the variable (shared across environments)
  initial: Initial value (only used if creating new variable)

Note: Values should be treated as immutable. Use set() to replace, or
update() for atomic read-modify-write operations.

Example:
    counter = runtime.sync.Shared("counter", 0)

    def increment(current):
        return current + 1

    # Atomic increment using update()
    counter.update(increment)

    # Simple get/set for immutable values
    counter.set(42)
    value = counter.get()`,
	},
}, nil, "Cross-environment named concurrency primitives")
