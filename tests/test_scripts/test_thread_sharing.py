import scriptling.runtime as runtime

# Test Atomic sharing
counter = runtime.sync.Atomic("test_counter", 0)

def increment():
    for i in range(100):
        counter.add(1)

promises = []
for i in range(10):
    promises.append(runtime.run(increment))

for p in promises:
    p.get()

print("Counter value:", counter.get())
print("Expected: 1000")

# Test Queue sharing
queue = runtime.sync.Queue("test_queue")

def producer():
    for i in range(5):
        queue.put(i)

def consumer():
    items = []
    for i in range(5):
        items.append(queue.get())
    return items

p1 = runtime.run(producer)
p2 = runtime.run(consumer)

p1.get()
result = p2.get()
print("Queue items:", result)
