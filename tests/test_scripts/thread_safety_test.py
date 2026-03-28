#!/usr/bin/env scriptling

# Test thread safety and edge cases for the runtime library

import scriptling.runtime as runtime

print("=== Testing thread safety with shared variables ===")

shared_counter = runtime.sync.Atomic("safety_counter", initial=0)
runtime.sync.WaitGroup("safety_wg")

def increment_worker():
    import scriptling.runtime as runtime
    runtime.sync.Atomic("safety_counter").add(1)
    runtime.sync.WaitGroup("safety_wg").done()

for i in range(10):
    runtime.sync.WaitGroup("safety_wg").add(1)
    runtime.background(f"increment_worker_{i}", "increment_worker")

runtime.sync.WaitGroup("safety_wg").wait()
print(f"Shared counter final value: {shared_counter.get()}")
assert shared_counter.get() == 10, f"Expected 10, got {shared_counter.get()}"

print("\n=== Testing WaitGroup edge cases ===")

wg = runtime.sync.WaitGroup("safety_wg2")
completed_count = runtime.sync.Atomic("completed_count", initial=0)

def worker_with_delay(x):
    import scriptling.runtime as runtime
    result = x * x
    runtime.sync.Atomic("completed_count").add(1)
    runtime.sync.WaitGroup("safety_wg2").done()

wg.add(5)
for i in range(5):
    runtime.background(f"worker_with_delay_{i}", "worker_with_delay", i)

wg.wait()
print(f"WaitGroup completed with 5 tasks")
assert completed_count.get() == 5, f"Expected count 5, got {completed_count.get()}"

print("\n=== All thread safety tests passed! ===")
