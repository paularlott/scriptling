#!/usr/bin/env scriptling

# Test thread safety and edge cases for the runtime library

import scriptling.runtime as runtime

print("=== Testing thread safety with shared variables ===")

shared_counter = runtime.sync.Atomic("safety_counter", 0)

def increment_worker():
    shared_counter.add(1)
    return shared_counter.get()

promises = []
for i in range(10):
    promises.append(runtime.background(f"increment_worker_{i}", "increment_worker"))

results = []
for p in promises:
    results.append(p.get())

print(f"Shared counter final value: {shared_counter.get()}")
print(f"Number of results collected: {len(results)}")
assert shared_counter.get() == 10, f"Expected 10, got {shared_counter.get()}"
assert len(results) == 10, f"Expected 10 results, got {len(results)}"

def worker_with_local(value):
    local = value * 2
    return local

local_promises = []
for i in range(5):
    local_promises.append(runtime.background(f"worker_with_local_{i}", "worker_with_local", i))

expected = []
for i in range(5):
    result = local_promises[i].get()
    expected.append(result)

print(f"Local thread results: {expected}")
assert expected == [0, 2, 4, 6, 8], f"Unexpected local results: {expected}"

print("\n=== Testing Queue with multiple consumers ===")

q = runtime.sync.Queue("safety_queue")
consumer_count = runtime.sync.Atomic("consumer_count", 0)

def consumer():
    count = 0
    while True:
        item = q.get()
        if item is None:
            break
        count += 1
        consumer_count.add(1)
    return count

consumer_promises = []
for i in range(3):
    consumer_promises.append(runtime.background(f"consumer_{i}", "consumer"))

for i in range(9):
    q.put(i)

for i in range(3):
    q.put(None)

total_consumed = 0
for p in consumer_promises:
    total_consumed += p.get()

print(f"Consumed {total_consumed} items")
print(f"Consumer count: {consumer_count.get()}")
assert total_consumed == 9, f"Expected 9 items, got {total_consumed}"
assert consumer_count.get() == 9, f"Expected count 9, got {consumer_count.get()}"

print("\n=== Testing WaitGroup edge cases ===")

wg = runtime.sync.WaitGroup("safety_wg")
completed_count = runtime.sync.Atomic("completed_count", 0)

def worker_with_delay(x):
    result = x * x
    completed_count.add(1)
    wg.done()
    return result

wg.add(5)
promises = []
for i in range(5):
    promises.append(runtime.background(f"worker_with_delay_{i}", "worker_with_delay", i))

wg.wait()

results = []
for p in promises:
    results.append(p.get())

print(f"WaitGroup completed with {len(results)} tasks")
assert len(results) == 5, f"Expected 5 tasks, got {len(results)}"
assert completed_count.get() == 5, f"Expected count 5, got {completed_count.get()}"

print("\n=== Testing Shared object ===")

shared_list = runtime.sync.Shared("safety_list", [])

def list_worker():
    current = shared_list.get()
    current.append(len(current))
    shared_list.set(current)

list_promises = []
for i in range(5):
    list_promises.append(runtime.background(f"list_worker_{i}", "list_worker"))

for p in list_promises:
    p.get()

final_list = shared_list.get()
print(f"Final shared list: {final_list}")
assert len(final_list) == 5, f"Expected list of length 5, got {len(final_list)}"

print("\n=== All thread safety tests passed! ===")
