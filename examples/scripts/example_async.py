#!/usr/bin/env scriptling
"""Example demonstrating async library features"""

import scriptling.runtime as runtime

print("=== Async Library Examples ===\n")

# Example 1: Basic async execution
print("1. Basic async execution:")

def calculate(x, y):
    result = x * y
    return result

promise = runtime.run(calculate, 6, 7)
result = promise.get()
print(f"   6 * 7 = {result}\n")

# Example 2: Multiple async operations
print("2. Multiple async operations:")

def square(n):
    return n * n

promises = [runtime.run(square, i) for i in range(1, 6)]
results = [p.get() for p in promises]
print(f"   Squares of 1-5: {results}\n")

# Example 3: Atomic counter
print("3. Atomic counter:")

counter = runtime.sync.Atomic("example_counter", 0)

def increment_counter():
    for _ in range(100):
        counter.add(1)

promises = [runtime.run(increment_counter) for _ in range(5)]
for p in promises:
    p.get()

print(f"   Counter after 5 workers * 100 increments: {counter.get()}\n")

# Example 4: Shared state
print("4. Shared state:")

shared_data = runtime.sync.Shared("example_shared", {"count": 0, "items": []})

def update_shared(id):
    data = shared_data.get()
    data["count"] = data["count"] + 1
    data["items"].append(id)
    shared_data.set(data)

promises = [runtime.run(update_shared, i) for i in range(5)]
for p in promises:
    p.get()

final_data = shared_data.get()
print(f"   Shared count: {final_data['count']}")
print(f"   Shared items: {final_data['items']}\n")

# Example 5: WaitGroup synchronization
print("5. WaitGroup synchronization:")

wg = runtime.sync.WaitGroup("example_wg")
completed = []

def worker(id):
    completed.append(f"Worker-{id}")
    wg.done()

print("   Starting 5 workers...")
for i in range(5):
    wg.add(1)
    runtime.run(worker, i)

wg.wait()
print(f"   All workers completed: {len(completed)} workers\n")

# Example 6: Producer-Consumer with Queue
print("6. Producer-Consumer pattern:")

queue = runtime.sync.Queue("example_queue", maxsize=10)
consumed_items = []

def producer():
    for i in range(10):
        queue.put(i)
    queue.put(None)

def consumer():
    while True:
        item = queue.get()
        if item is None:
            break
        consumed_items.append(item)

runtime.run(producer)
consumer_promise = runtime.run(consumer)
consumer_promise.get()

print(f"   Produced and consumed {len(consumed_items)} items\n")

print("=== All examples completed successfully! ===")
