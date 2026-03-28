import scriptling.runtime as runtime

# Test Atomic sharing across goroutines
counter = runtime.sync.Atomic("test_counter", initial=0)
runtime.sync.WaitGroup("thread_wg")

def increment():
    import scriptling.runtime as runtime
    runtime.sync.Atomic("test_counter").add(1)
    runtime.sync.WaitGroup("thread_wg").done()

for i in range(10):
    runtime.sync.WaitGroup("thread_wg").add(1)
    runtime.background(f"increment_{i}", "increment")

runtime.sync.WaitGroup("thread_wg").wait()
assert counter.get() == 10, f"Expected 10, got {counter.get()}"

# Test Shared state across goroutines
shared_value = runtime.sync.Shared("test_shared", "initial")
runtime.sync.WaitGroup("shared_wg")

def set_shared(val):
    import scriptling.runtime as runtime
    runtime.sync.Shared("test_shared").set(val)
    runtime.sync.WaitGroup("shared_wg").done()

runtime.sync.WaitGroup("shared_wg").add(1)
runtime.background("set_shared1", "set_shared", "updated")
runtime.sync.WaitGroup("shared_wg").wait()
assert shared_value.get() == "updated", f"Expected 'updated', got {shared_value.get()}"
