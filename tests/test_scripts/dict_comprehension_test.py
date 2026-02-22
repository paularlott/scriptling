# Basic dict comprehension
squares = {x: x * x for x in range(5)}
assert squares[0] == 0
assert squares[1] == 1
assert squares[2] == 4
assert squares[3] == 9
assert squares[4] == 16

# Dict comprehension with condition
even_squares = {x: x * x for x in range(10) if x % 2 == 0}
assert even_squares[0] == 0
assert even_squares[2] == 4
assert even_squares[4] == 16
assert 1 not in even_squares
assert 3 not in even_squares

# Dict comprehension from items()
original = {"a": 1, "b": 2, "c": 3}
doubled = {k: v * 2 for k, v in original.items()}
assert doubled["a"] == 2
assert doubled["b"] == 4
assert doubled["c"] == 6

# Dict comprehension inverting keys/values
inverted = {v: k for k, v in original.items()}
assert inverted[1] == "a"
assert inverted[2] == "b"
assert inverted[3] == "c"

# Dict comprehension from list of tuples
pairs = [("x", 10), ("y", 20), ("z", 30)]
d = {k: v for k, v in pairs}
assert d["x"] == 10
assert d["y"] == 20
assert d["z"] == 30

# Dict comprehension with string transformation
words = ["hello", "world", "foo"]
lengths = {w: len(w) for w in words}
assert lengths["hello"] == 5
assert lengths["world"] == 5
assert lengths["foo"] == 3

print("All dict comprehension tests passed!")
