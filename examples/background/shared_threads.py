#!/usr/bin/env scriptling
"""Shared-environment threads with runtime.background(shared=True).

By default runtime.background() runs a handler in an *isolated* copy of the
environment (see example.py) - safe, parallel, but no shared live state.

With shared=True the handler instead runs on a goroutine in the *same*
environment as the caller, so it can read and write the caller's variables
directly. The interpreter lock (GIL) serializes script execution, so concurrent
access to shared state is safe - you never need locks around it. Only one thread
runs script at a time; threads interleave when one blocks (sleep, queue, I/O).
"""

import scriptling.runtime as runtime

print("=== Shared-environment threads ===\n")

# Plain shared globals - no runtime.sync needed, the GIL protects them.
counter = 0
results = []


def worker(name, n):
    global counter
    i = 0
    while i < n:
        counter = counter + 1  # safe: serialized by the interpreter lock
        i = i + 1
    results.append(name + " done")
    return counter


# Spawn several threads that share this script's variables.
threads = []
i = 0
while i < 4:
    t = runtime.background("worker-" + str(i), "worker", "w" + str(i), 1000, shared=True)
    threads.append(t)
    i = i + 1

# wait() releases the lock so the workers run, then re-acquires it.
for t in threads:
    t.wait()

print(f"counter = {counter}  (expected {4 * 1000})")
print(f"results = {results}")
print("\n=== Done ===")
