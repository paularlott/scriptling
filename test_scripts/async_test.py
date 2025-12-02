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

shared_list = threads.Shared([])

def append_item(item):
    current = shared_list.get()
    current.append(item)
    shared_list.set(current)

promises = [threads.run(append_item, i) for i in range(5)]
for p in promises:
    p.get()

final_list = shared_list.get()
print(f"Shared list length: {len(final_list)}")
assert len(final_list) == 5, "Shared list failed"

print("\n=== Testing threads.WaitGroup ===")

wg = threads.WaitGroup()
results = []

def worker_wg(id):
    results.append(id)
    wg.done()

for i in range(5):
    wg.add(1)
    threads.run(worker_wg, i)

wg.wait()
print(f"WaitGroup completed, results length: {len(results)}")
assert len(results) == 5, "WaitGroup failed"

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

processed = []

def process_data(item):
    processed.append(item * item)

pool = threads.Pool(process_data, workers=2, queue_depth=10)

for item in range(5):
    pool.submit(item)

pool.close()

print(f"Pool processed {len(processed)} items")
assert len(processed) == 5, "Pool failed"

print("\n=== All async tests passed! ===")
