#!/usr/bin/env scriptling
"""Test async library functionality"""

import scriptling.runtime as runtime

print("=== Testing runtime.run ===")

def worker(x, y):
    return x + y

promise = runtime.run(worker, 5, 3)
result = promise.get()
print(f"runtime.run result: {result}")
assert result == 8, "runtime.run failed"

# Multiple async operations
promises = [runtime.run(worker, i, i+1) for i in range(5)]
results = [p.get() for p in promises]
print(f"Multiple async results: {results}")
assert results == [1, 3, 5, 7, 9], "Multiple async failed"

print("\n=== Testing runtime.sync.Atomic ===")

counter = runtime.sync.Atomic("test_counter", 0)
print(f"Initial counter: {counter.get()}")

def increment():
    counter.add(1)

promises = [runtime.run(increment) for _ in range(10)]
for p in promises:
    p.get()

final_count = counter.get()
print(f"Final counter after 10 increments: {final_count}")
assert final_count == 10, "Atomic counter failed"

# Test add with delta
counter.set(0)
counter.add(5)
counter.add(-2)
print(f"Counter after add(5) and add(-2): {counter.get()}")
assert counter.get() == 3, "Atomic add with delta failed"

print("\n=== Testing runtime.sync.Shared ===")

shared_counter = runtime.sync.Atomic("shared_counter", 0)

def increment_shared():
    shared_counter.add(1)

promises = [runtime.run(increment_shared) for i in range(5)]
for p in promises:
    p.get()

final_count = shared_counter.get()
print(f"Shared counter value: {final_count}")
assert final_count == 5, "Shared counter failed"

shared_value = runtime.sync.Shared("test_shared", "initial")

def set_value(val):
    shared_value.set(val)

p = runtime.run(set_value, "updated")
p.get()
print(f"Shared value: {shared_value.get()}")
assert shared_value.get() == "updated", "Shared value failed"

print("\n=== Testing runtime.sync.WaitGroup ===")

wg = runtime.sync.WaitGroup("test_wg")
wg_counter = runtime.sync.Atomic("wg_counter", 0)

def worker_wg(id):
    wg_counter.add(1)
    wg.done()

for i in range(5):
    wg.add(1)
    runtime.run(worker_wg, i)

wg.wait()
print(f"WaitGroup completed, counter: {wg_counter.get()}")
assert wg_counter.get() == 5, "WaitGroup failed"

print("\n=== Testing runtime.sync.Queue ===")

queue = runtime.sync.Queue("test_queue", maxsize=10)

def producer():
    for i in range(5):
        queue.put(i)
    queue.put(None)

consumed = []

def consumer():
    while True:
        item = queue.get()
        if item is None:
            break
        consumed.append(item)

runtime.run(producer)
p = runtime.run(consumer)
p.get()

print(f"Queue consumed items: {consumed}")
assert len(consumed) == 5, "Queue failed"

print("\n=== Testing promise.wait ===")

def worker_wait(x):
    return x * 2

promise1 = runtime.run(worker_wait, 5)
promise1.wait()
promise2 = runtime.run(worker_wait, 10)
promise2.wait()
print(f"promise.wait completed for both promises")

def worker_wait_kwargs(x, multiplier=1):
    return x * multiplier

promise3 = runtime.run(worker_wait_kwargs, 3, multiplier=4)
result = promise3.get()
print(f"promise.wait with kwargs result: {result}")
assert result == 12, "promise.wait with kwargs failed"

print("\n=== All async tests passed! ===")
