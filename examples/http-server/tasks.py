import scriptling.runtime as runtime
import time

def increment_counter():
    """Background task that increments a counter every second."""
    counter = runtime.sync.Atomic("request_counter", 0)
    
    while True:
        counter.add(1)
        print(f"Background counter: {counter.get()}")
        time.sleep(1)
