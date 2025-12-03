import threads

# Test Atomic sharing
counter = threads.Atomic(0)

def increment():
    for i in range(100):
        counter.add(1)

promises = []
for i in range(10):
    promises.append(threads.run(increment))

for p in promises:
    p.get()

print("Counter value:", counter.get())
print("Expected: 1000")

# Test Queue sharing
queue = threads.Queue()

def producer():
    for i in range(5):
        queue.put(i)

def consumer():
    items = []
    for i in range(5):
        items.append(queue.get())
    return items

p1 = threads.run(producer)
p2 = threads.run(consumer)

p1.get()
result = p2.get()
print("Queue items:", result)
