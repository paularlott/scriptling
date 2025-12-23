#!/usr/bin/env scriptling
"""Test async library functionality"""

import threads

print("=== Testing threads.run ===")

def worker(x, y):
    return x + y

promise = threads.run(worker, 5, 3)
result = promise.get()
print(f"threads.run result: {result}")
assert result == 8, "threads.run failed"

# Multiple async operations
promises = [threads.run(worker, i, i+1) for i in range(5)]
results = [p.get() for p in promises]
print(f"Multiple async results: {results}")
assert results == [1, 3, 5, 7, 9], "Multiple async failed"

print("\n=== Testing threads.Atomic ===")

counter = threads.Atomic(0)
print(f"Initial counter: {counter.get()}")

def increment():
    counter.add(1)

promises = [threads.run(increment) for _ in range(10)]
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

print("\n=== Testing threads.Shared ===")

# Use Atomic counter instead of Shared list for thread-safe counting
# The Shared type is for sharing values between threads, but read-modify-write
# operations need proper synchronization. Use Atomic for counting.
shared_counter = threads.Atomic(0)

def increment_shared():
    shared_counter.add(1)

promises = [threads.run(increment_shared) for i in range(5)]
for p in promises:
    p.get()

final_count = shared_counter.get()
print(f"Shared counter value: {final_count}")
assert final_count == 5, "Shared counter failed"

# Test Shared with a simple value (not read-modify-write)
shared_value = threads.Shared("initial")

def set_value(val):
    shared_value.set(val)

p = threads.run(set_value, "updated")
p.get()
print(f"Shared value: {shared_value.get()}")
assert shared_value.get() == "updated", "Shared value failed"

print("\n=== Testing threads.WaitGroup ===")

wg = threads.WaitGroup()
wg_counter = threads.Atomic(0)

def worker_wg(id):
    wg_counter.add(1)
    wg.done()

for i in range(5):
    wg.add(1)
    threads.run(worker_wg, i)

wg.wait()
print(f"WaitGroup completed, counter: {wg_counter.get()}")
assert wg_counter.get() == 5, "WaitGroup failed"

print("\n=== Testing threads.Queue ===")

queue = threads.Queue(maxsize=10)

def producer():
    for i in range(5):
        queue.put(i)
    queue.put(None)  # Sentinel

consumed = []

def consumer():
    while True:
        item = queue.get()
        if item is None:
            break
        consumed.append(item)

threads.run(producer)
p = threads.run(consumer)
p.get()

print(f"Queue consumed items: {consumed}")
assert len(consumed) == 5, "Queue failed"

print("\n=== Testing threads.Pool ===")

# Use Atomic counter for thread-safe counting instead of list
processed_count = threads.Atomic(0)

def process_data(item):
    processed_count.add(1)

pool = threads.Pool(process_data, workers=2, queue_depth=10)

for item in range(5):
    pool.submit(item)

pool.close()

print(f"Pool processed {processed_count.get()} items")
assert processed_count.get() == 5, "Pool failed"

print("\n=== Testing promise.wait ===")

# Test promise.wait - promises run sequentially with wait() between them
# Each promise completes before the next one starts
def worker_wait(x):
    return x * 2

promise1 = threads.run(worker_wait, 5)
promise1.wait()  # Wait for completion, discard result
promise2 = threads.run(worker_wait, 10)
promise2.wait()  # Wait for completion, discard result
print(f"promise.wait completed for both promises")

# Test promise.wait with keyword args
def worker_wait_kwargs(x, multiplier=1):
    return x * multiplier

promise3 = threads.run(worker_wait_kwargs, 3, multiplier=4)
result = promise3.get()  # Get result to verify
print(f"promise.wait with kwargs result: {result}")
assert result == 12, "promise.wait with kwargs failed"

print("\n=== All async tests passed! ===")
