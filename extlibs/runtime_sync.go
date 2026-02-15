package extlibs

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/paularlott/scriptling/errors"
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
	cond    *sync.Cond
	maxsize int
	closed  bool
}

func newRuntimeQueue(maxsize int) *RuntimeQueue {
	q := &RuntimeQueue{
		items:   []object.Object{},
		maxsize: maxsize,
	}
	q.cond = sync.NewCond(&q.mu)
	return q
}

func (q *RuntimeQueue) put(item object.Object) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return fmt.Errorf("queue is closed")
	}

	for q.maxsize > 0 && len(q.items) >= q.maxsize {
		q.cond.Wait()
	}

	q.items = append(q.items, item)
	q.cond.Signal()
	return nil
}

func (q *RuntimeQueue) get() (object.Object, error) {
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
	q.cond.Signal()

	return item, nil
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
	q.cond.Broadcast()
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

// RuntimeShared is a named shared value
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
							if err := queue.put(args[0]); err != nil {
								return errors.NewError("queue error: %v", err)
							}
							return &object.Null{}
						},
						HelpText: "put(item) - Add item to queue (blocks if full)",
					},
					"get": &object.Builtin{
						Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
							item, err := queue.get()
							if err != nil {
								return errors.NewError("queue error: %v", err)
							}
							return item
						},
						HelpText: "get() - Remove and return item from queue (blocks if empty)",
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
						HelpText: "get() - Get the current value (thread-safe)",
					},
					"set": &object.Builtin{
						Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
							if err := errors.ExactArgs(args, 1); err != nil {
								return err
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
		HelpText: `Shared(name, initial) - Get or create a named shared variable

Parameters:
  name (string): Unique name for the variable (shared across environments)
  initial: Initial value (only used if creating new variable)

Example:
    shared_list = runtime.sync.Shared("data", [])

    def append_item(item):
        current = shared_list.get()
        current.append(item)
        shared_list.set(current)

    promises = [runtime.run(append_item, i) for i in range(100)]
    for p in promises:
        p.get()`,
	},
}, nil, "Cross-environment named concurrency primitives")
