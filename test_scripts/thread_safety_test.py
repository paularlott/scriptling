#!/usr/bin/env scriptling

# Test thread safety and edge cases for the threads library

import sl.threads as threads

print("=== Testing thread safety with shared variables ===")

# Test that shared variables work correctly with cloned environments
shared_counter = threads.Atomic(0)

def increment_worker():
    shared_counter.add(1)
    return shared_counter.get()

# Create multiple threads that increment the counter
promises = []
for i in range(10):
    promises.append(threads.run(increment_worker))

# Wait for all to complete and collect results
results = []
for p in promises:
    results.append(p.get())

print(f"Shared counter final value: {shared_counter.get()}")
print(f"Number of results collected: {len(results)}")
assert shared_counter.get() == 10, f"Expected 10, got {shared_counter.get()}"
assert len(results) == 10, f"Expected 10 results, got {len(results)}"

# Test that local variables don't interfere between threads
def worker_with_local(value):
    local = value * 2
    # This should NOT interfere with other threads
    return local

local_promises = []
for i in range(5):
    local_promises.append(threads.run(worker_with_local, i))

# Collect results from promises instead of using shared list
expected = []
for i in range(5):
    result = local_promises[i].get()
    expected.append(result)

print(f"Local thread results: {expected}")
assert expected == [0, 2, 4, 6, 8], f"Unexpected local results: {expected}"

print("\n=== Testing Pool with shared state ===")

pool_counter = threads.Atomic(0)

def pool_worker(x):
    pool_counter.add(1)
    return x * x

pool = threads.Pool(pool_worker, workers=3, queue_depth=10)

# Submit tasks
for i in range(7):
    pool.submit(i)

pool.close()

print(f"Pool counter: {pool_counter.get()}")
print(f"Pool processed {pool_counter.get()} items")
assert pool_counter.get() == 7, f"Expected 7, got {pool_counter.get()}"

print("\n=== Testing Queue with multiple consumers ===")

q = threads.Queue()
consumer_count = threads.Atomic(0)

def consumer():
    count = 0
    while True:
        item = q.get()
        if item is None:
            break
        count += 1
        consumer_count.add(1)
    return count  # Return how many items this consumer consumed

# Start multiple consumers
consumer_promises = []
for i in range(3):
    consumer_promises.append(threads.run(consumer))

# Producer adds items
for i in range(9):
    q.put(i)

# Signal consumers to stop
for i in range(3):
    q.put(None)

# Wait for all consumers and get their counts
total_consumed = 0
for p in consumer_promises:
    total_consumed += p.get()

print(f"Consumed {total_consumed} items")
print(f"Consumer count: {consumer_count.get()}")
assert total_consumed == 9, f"Expected 9 items, got {total_consumed}"
assert consumer_count.get() == 9, f"Expected count 9, got {consumer_count.get()}"

print("\n=== Testing WaitGroup edge cases ===")

wg = threads.WaitGroup()
completed_count = threads.Atomic(0)

def worker_with_delay(x):
    # Simple computation
    result = x * x
    completed_count.add(1)
    wg.done()  # Call done when work is actually complete
    return result

wg.add(5)
promises = []
for i in range(5):
    promises.append(threads.run(worker_with_delay, i))

# Wait for all to complete
wg.wait()

# Collect results
results = []
for p in promises:
    results.append(p.get())

print(f"WaitGroup completed with {len(results)} tasks")
assert len(results) == 5, f"Expected 5 tasks, got {len(results)}"
assert completed_count.get() == 5, f"Expected count 5, got {completed_count.get()}"

print("\n=== Testing Shared object ===")

shared_list = threads.Shared([])

def list_worker():
    current = shared_list.get()
    current.append(len(current))
    shared_list.set(current)

# Multiple threads modifying the shared list
list_promises = []
for i in range(5):
    list_promises.append(threads.run(list_worker))

for p in list_promises:
    p.get()

final_list = shared_list.get()
print(f"Final shared list: {final_list}")
assert len(final_list) == 5, f"Expected list of length 5, got {len(final_list)}"

print("\n=== All thread safety tests passed! ===")