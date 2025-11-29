# collections Library

The `collections` library provides Python-compatible specialized container datatypes.

## Import

```python
import collections
```

## Functions

### Counter

#### `Counter([iterable])`
Create a dict-like object that counts occurrences of elements.

```python
# Count list elements
c = collections.Counter([1, 1, 2, 3, 3, 3])
# {1: 2, 2: 1, 3: 3}

# Count characters in string
c = collections.Counter("hello")
# {"h": 1, "e": 1, "l": 2, "o": 1}

# Access counts (returns 0 for missing keys)
c["l"]  # 2
c["x"]  # 0 (not KeyError like regular dict)
```

#### Counter Methods

Counter objects support the following methods:

- `c[key]` - Get count for key (returns 0 if key not present)
- `c.most_common([n])` - Return n most common elements as (element, count) tuples
- `c.elements()` - Return iterator over elements (repeating each by its count)

#### `most_common(counter[, n])`
Return the n most common elements and their counts.

```python
c = collections.Counter([1, 1, 2, 3, 3, 3])
collections.most_common(c, 2)
# [(3, 3), (1, 2)]  - 3 appears 3 times, 1 appears 2 times
```

### OrderedDict

#### `OrderedDict([items])`
Create a dict that maintains insertion order.

```python
od = collections.OrderedDict([("a", 1), ("b", 2), ("c", 3)])
od["a"]  # 1

# Note: Regular dicts in Scriptling already maintain order
```

### deque (Double-Ended Queue)

#### `deque([iterable[, maxlen]])`
Create a double-ended queue.

```python
d = collections.deque([1, 2, 3])
```

#### `deque_appendleft(deque, elem)`
Add element to the left side.

```python
d = collections.deque([1, 2, 3])
collections.deque_appendleft(d, 0)
# d is now [0, 1, 2, 3]
```

#### `deque_popleft(deque)`
Remove and return element from the left side.

```python
d = collections.deque([1, 2, 3])
x = collections.deque_popleft(d)
# x = 1, d is now [2, 3]
```

#### `deque_extendleft(deque, iterable)`
Extend the left side with elements (in reverse order).

```python
d = collections.deque([1, 2, 3])
collections.deque_extendleft(d, [4, 5])
# d is now [5, 4, 1, 2, 3]
```

#### `deque_rotate(deque, n)`
Rotate the deque n steps to the right (negative for left).

```python
d = collections.deque([1, 2, 3, 4])
collections.deque_rotate(d, 1)
# d is now [4, 1, 2, 3]

d = collections.deque([1, 2, 3, 4])
collections.deque_rotate(d, -1)
# d is now [2, 3, 4, 1]
```

### namedtuple

#### `namedtuple(typename, field_names)`
Create a factory function for named tuple-like dicts.

```python
# Create a Point type
Point = collections.namedtuple("Point", ["x", "y"])

# Create instances
p = Point(1, 2)
p["x"]  # 1
p["y"]  # 2

# Field names can also be a string
Person = collections.namedtuple("Person", "name age")
```

### defaultdict

#### `defaultdict(default_factory)`
Create a dict with default values for missing keys.

```python
# Create with type name
d = collections.defaultdict("list")
d = collections.defaultdict("int")
d = collections.defaultdict("str")

# Use get_default to access with auto-creation
collections.get_default(d, "key")
```

#### `get_default(defaultdict, key)`
Get a value from a defaultdict, creating it if missing.

```python
d = collections.defaultdict("list")
lst = collections.get_default(d, "items")
# lst is [], and d["items"] is now []
```

### ChainMap

#### `ChainMap(*maps)`
Group multiple dicts for single lookup (first has priority).

```python
d1 = {"a": 1, "b": 2}
d2 = {"b": 20, "c": 3}
cm = collections.ChainMap(d1, d2)

cm["a"]  # 1 (from d1)
cm["b"]  # 2 (from d1 - first has priority)
cm["c"]  # 3 (from d2)
```

## Examples

### Word frequency counter
```python
import collections

text = "the quick brown fox jumps over the lazy dog"
words = text.split(" ")
word_counts = collections.Counter(words)

# Most common words
top_words = collections.most_common(word_counts, 3)
```

### Using deque as a queue
```python
import collections

# Create a task queue
queue = collections.deque()

# Add tasks
queue.append("task1")
queue.append("task2")
queue.append("task3")

# Process tasks (FIFO)
while len(queue) > 0:
    task = collections.deque_popleft(queue)
    print("Processing:", task)
```

### Using deque as a stack
```python
import collections

# Create a stack
stack = collections.deque()

# Push items
stack.append("first")
stack.append("second")
stack.append("third")

# Pop items (LIFO)
while len(stack) > 0:
    item = stack.pop()
    print("Popped:", item)
```

### Using namedtuple for data records
```python
import collections

# Define a record type
Employee = collections.namedtuple("Employee", ["name", "department", "salary"])

# Create records
emp1 = Employee("Alice", "Engineering", 75000)
emp2 = Employee("Bob", "Marketing", 65000)

print(emp1["name"], "works in", emp1["department"])
```

### Merging configurations with ChainMap
```python
import collections

# Default configuration
defaults = {"debug": False, "timeout": 30, "retries": 3}

# User overrides
user_config = {"debug": True, "timeout": 60}

# Merge (user config takes priority)
config = collections.ChainMap(user_config, defaults)

config["debug"]    # True (from user_config)
config["retries"]  # 3 (from defaults)
```
