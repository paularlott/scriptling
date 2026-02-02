# Test for loop unpacking with tuples and lists
fails = 0

# Test 1: Unpacking tuples in for loop
result = []
for x, y in [(1, 2), (3, 4), (5, 6)]:
    result.append(x + y)
if result != [3, 7, 11]:
    fails = fails + 1

# Test 2: Unpacking lists in for loop
result = []
for a, b in [[1, 2], [3, 4]]:
    result.append(a * b)
if result != [2, 12]:
    fails = fails + 1

# Test 3: Using enumerate with unpacking
result = []
for i, val in enumerate(["a", "b", "c"]):
    result.append(str(i) + val)
if result != ["0a", "1b", "2c"]:
    fails = fails + 1

# Test 4: Using items() with unpacking
d = {"x": 1, "y": 2}
total = 0
for key, val in d.items():
    total = total + val
if total != 3:
    fails = fails + 1

# Test 5: Using zip with unpacking
result = []
for a, b in zip([1, 2, 3], [4, 5, 6]):
    result.append(a + b)
if result != [5, 7, 9]:
    fails = fails + 1

# Test 6: Triple unpacking
result = []
for x, y, z in [(1, 2, 3), (4, 5, 6)]:
    result.append(x + y + z)
if result != [6, 15]:
    fails = fails + 1

# Test 7: Single variable for loop (no unpacking)
result = []
for x in [1, 2, 3]:
    result.append(x * 2)
if result != [2, 4, 6]:
    fails = fails + 1

# Test 8: Nested for loop with unpacking
result = []
for i in range(2):
    for a, b in [(1, 2), (3, 4)]:
        result.append(i * 10 + a + b)
if result != [3, 7, 13, 17]:
    fails = fails + 1

fails == 0
