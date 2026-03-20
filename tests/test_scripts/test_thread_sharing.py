import scriptling.runtime as runtime

# Test Atomic sharing across goroutines
counter = runtime.sync.Atomic("test_counter", 0)

def increment():
    for i in range(100):
        counter.add(1)

promises = []
for i in range(10):
    promises.append(runtime.background(f"increment_{i}", "increment"))

for p in promises:
    p.get()

assert counter.get() == 1000, f"Expected 1000, got {counter.get()}"

# Test Queue sharing between producer and consumer
queue = runtime.sync.Queue("test_queue")

def producer():
    for i in range(5):
        queue.put(i)

def consumer():
    items = []
    for i in range(5):
        items.append(queue.get())
    return items

p1 = runtime.background("producer1", "producer")
p2 = runtime.background("consumer1", "consumer")

p1.get()
result = p2.get()
assert len(result) == 5
assert sorted(result) == [0, 1, 2, 3, 4]
