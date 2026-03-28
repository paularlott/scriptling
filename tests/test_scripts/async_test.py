#!/usr/bin/env scriptling
"""Test async library functionality"""

import scriptling.runtime as runtime

print("=== Testing runtime.background ===")

# Test 1: Basic background task writes to shared Atomic
runtime.sync.Atomic("bg_result", initial=0)
runtime.sync.WaitGroup("bg_wg1")

def worker(x, y):
    import scriptling.runtime as runtime
    runtime.sync.Atomic("bg_result").set(x + y)
    runtime.sync.WaitGroup("bg_wg1").done()

runtime.sync.WaitGroup("bg_wg1").add(1)
runtime.background("worker1", "worker", 5, 3)
runtime.sync.WaitGroup("bg_wg1").wait()
result = runtime.sync.Atomic("bg_result").get()
print(f"runtime.background result: {result}")
assert result == 8, f"runtime.background failed, got {result}"

# Test 2: Multiple background tasks with individual results
runtime.sync.WaitGroup("bg_wg2")

def worker_multi(idx, x, y):
    import scriptling.runtime as runtime
    runtime.sync.Atomic(f"multi_{idx}").set(x + y)
    runtime.sync.WaitGroup("bg_wg2").done()

for i in range(5):
    runtime.sync.Atomic(f"multi_{i}", initial=0)
    runtime.sync.WaitGroup("bg_wg2").add(1)
    runtime.background(f"worker_multi_{i}", "worker_multi", i, i, i+1)

runtime.sync.WaitGroup("bg_wg2").wait()
results = [runtime.sync.Atomic(f"multi_{i}").get() for i in range(5)]
print(f"Multiple async results: {results}")
assert results == [1, 3, 5, 7, 9], f"Multiple async failed, got {results}"

print("\n=== Testing runtime.sync.Atomic ===")

counter = runtime.sync.Atomic("test_counter", initial=0)
print(f"Initial counter: {counter.get()}")

# Test concurrent increments
runtime.sync.WaitGroup("bg_wg3")

def increment():
    import scriptling.runtime as runtime
    runtime.sync.Atomic("test_counter").add(1)
    runtime.sync.WaitGroup("bg_wg3").done()

for i in range(10):
    runtime.sync.WaitGroup("bg_wg3").add(1)
    runtime.background(f"increment_{i}", "increment")

runtime.sync.WaitGroup("bg_wg3").wait()
final_count = counter.get()
print(f"Final counter after 10 increments: {final_count}")
assert final_count == 10, f"Atomic counter failed, got {final_count}"

# Test add with delta
counter.set(0)
counter.add(5)
counter.add(-2)
print(f"Counter after add(5) and add(-2): {counter.get()}")
assert counter.get() == 3, "Atomic add with delta failed"

print("\n=== Testing runtime.sync.Shared ===")

shared_value = runtime.sync.Shared("test_shared", "initial")
runtime.sync.WaitGroup("bg_wg4")

def set_value(val):
    import scriptling.runtime as runtime
    runtime.sync.Shared("test_shared").set(val)
    runtime.sync.WaitGroup("bg_wg4").done()

runtime.sync.WaitGroup("bg_wg4").add(1)
runtime.background("set_value1", "set_value", "updated")
runtime.sync.WaitGroup("bg_wg4").wait()
print(f"Shared value: {shared_value.get()}")
assert shared_value.get() == "updated", "Shared value failed"

print("\n=== Testing runtime.sync.WaitGroup ===")

wg = runtime.sync.WaitGroup("test_wg")
wg_counter = runtime.sync.Atomic("wg_counter", initial=0)

def worker_wg(id):
    import scriptling.runtime as runtime
    runtime.sync.Atomic("wg_counter").add(1)
    runtime.sync.WaitGroup("test_wg").done()

for i in range(5):
    wg.add(1)
    runtime.background(f"worker_wg_{i}", "worker_wg", i)

wg.wait()
print(f"WaitGroup completed, counter: {wg_counter.get()}")
assert wg_counter.get() == 5, "WaitGroup failed"

print("\n=== Testing runtime.sync.Queue ===")

queue = runtime.sync.Queue("test_queue", maxsize=10)
runtime.sync.Atomic("queue_total", initial=0)
runtime.sync.WaitGroup("queue_wg")

def queue_consumer():
    import scriptling.runtime as runtime
    q = runtime.sync.Queue("test_queue")
    total = 0
    total = total + q.get()
    total = total + q.get()
    total = total + q.get()
    total = total + q.get()
    total = total + q.get()
    runtime.sync.Atomic("queue_total").set(total)
    runtime.sync.WaitGroup("queue_wg").done()

runtime.sync.WaitGroup("queue_wg").add(1)
runtime.background("consumer1", "queue_consumer")
queue.put(10)
queue.put(20)
queue.put(30)
queue.put(40)
queue.put(50)
runtime.sync.WaitGroup("queue_wg").wait()

total = runtime.sync.Atomic("queue_total").get()
print(f"Queue total: {total}")
assert total == 150, f"Queue failed, got {total}"

print("\n=== All async tests passed! ===")
