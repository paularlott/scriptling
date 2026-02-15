#!/usr/bin/env scriptling
"""Example demonstrating runtime library background tasks with Promises"""

import scriptling.runtime as runtime

print("=== Runtime Library Demo ===\n")

# Shared state for tracking task progress
task_status = runtime.sync.Shared("task_status", {
    "task1": "pending",
    "task2": "pending", 
    "task3": "pending"
})

# WaitGroup to coordinate task completion
wg = runtime.sync.WaitGroup("background_wg")

# Task 1: Counter that increments periodically
def counter_task():
    print("[Task 1] Counter task starting...")
    status = task_status.get()
    status["task1"] = "running"
    task_status.set(status)
    
    counter = runtime.sync.Atomic("counter", 0)
    for i in range(5):
        count = counter.add(1)
        print(f"[Task 1] Count: {count}")
    
    status = task_status.get()
    status["task1"] = "completed"
    task_status.set(status)
    print("[Task 1] Counter task finished!")
    wg.done()
    return "counter_done"

# Task 2: Data processor
def processor_task():
    print("[Task 2] Processor task starting...")
    status = task_status.get()
    status["task2"] = "running"
    task_status.set(status)
    
    queue = runtime.sync.Queue("work_queue", maxsize=10)
    
    # Process some items
    items = ["alpha", "beta", "gamma", "delta"]
    for item in items:
        processed = item.upper()
        print(f"[Task 2] Processed: {item} -> {processed}")
        queue.put(processed)
    
    status = task_status.get()
    status["task2"] = "completed"
    task_status.set(status)
    print("[Task 2] Processor task finished!")
    wg.done()
    return "processor_done"

# Task 3: Logger that monitors shared state
def logger_task():
    print("[Task 3] Logger task starting...")
    status = task_status.get()
    status["task3"] = "running"
    task_status.set(status)
    
    kv = runtime.kv
    kv.set("log_count", 0)
    
    for i in range(3):
        count = kv.incr("log_count", 1)
        print(f"[Task 3] Log entry #{count}")
    
    status = task_status.get()
    status["task3"] = "completed"
    task_status.set(status)
    print("[Task 3] Logger task finished!")
    wg.done()
    return "logger_done"

print("=== Background Tasks with Promises ===\n")
print("Starting background tasks...\n")
wg.add(3)

# Register and start background tasks - they return Promises
promise1 = runtime.background("counter", "counter_task")
promise2 = runtime.background("processor", "processor_task")
promise3 = runtime.background("logger", "logger_task")

print("Main: Waiting for all tasks to complete...\n")

# Wait for all tasks
wg.wait()

print("\nMain: All background tasks completed!")

# Get results from promises
if promise1:
    result1 = promise1.get()
    print(f"Task 1 result: {result1}")
if promise2:
    result2 = promise2.get()
    print(f"Task 2 result: {result2}")
if promise3:
    result3 = promise3.get()
    print(f"Task 3 result: {result3}")

# Check final status
final_status = task_status.get()
print(f"\nFinal task status:")
for task, status in final_status.items():
    print(f"  {task}: {status}")

# Demo concurrent calculations with background()
print("\n=== Concurrent Calculations ===\n")

def calculate(x, y, operation="add"):
    if operation == "add":
        return x + y
    elif operation == "multiply":
        return x * y
    else:
        return 0

# Use background() for concurrent calculations with arguments
print("Running concurrent calculations...")
p1 = runtime.background("calc1", "calculate", 10, 5, operation="add")
p2 = runtime.background("calc2", "calculate", 10, 5, operation="multiply")
p3 = runtime.background("calc3", "calculate", 100, 25, operation="add")

# Get results
result1 = p1.get()
result2 = p2.get()
result3 = p3.get()

print(f"  10 + 5 = {result1}")
print(f"  10 * 5 = {result2}")
print(f"  100 + 25 = {result3}")

# Demo KV store
print("\n=== KV Store ===\n")

kv = runtime.kv
kv.set("session_id", "abc123", ttl=60)
kv.set("user_name", "Alice")
kv.set("login_count", 0)

print(f"Session ID: {kv.get('session_id')}")
print(f"User name: {kv.get('user_name')}")
print(f"Login count: {kv.get('login_count')}")

# Increment counter
for i in range(3):
    count = kv.incr("login_count", 1)
    print(f"Login #{count}")

print(f"\nAll keys: {kv.keys()}")
print(f"Session exists: {kv.exists('session_id')}")

print("\n=== External Library Test ===\n")
# Spawn task from external library file (testlib.py in same folder)
# The CLI loads libraries from current folder, so testlib.worker will be auto-imported
promise = runtime.background("external_worker", "testlib.worker", 7, iterations=10)
print("Started external library task: testlib.worker")
result = promise.get()
print(f"External worker result: {result}")

print("\n=== Demo Complete ===")
