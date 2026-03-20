# Generator expressions (supported)
# yield-based generator functions are NOT supported in Scriptling

# Generator expression in join
text = "this is a test"
result = ' '.join(word.upper() for word in text.split())
assert result == "THIS IS A TEST"

# Generator expression with filter
numbers = [1, 2, 3, 4, 5, 6]
evens = list(x for x in numbers if x % 2 == 0)
assert evens == [2, 4, 6]

# Generator expression in sum/min/max
assert sum(x * x for x in range(5)) == 0 + 1 + 4 + 9 + 16
assert min(x * x for x in range(1, 5)) == 1
assert max(x * x for x in range(1, 5)) == 16

# Nested generator expression (multi-for)
pairs = list((x, y) for x in range(3) for y in range(3) if x != y)
assert len(pairs) == 6
assert (0, 1) in pairs
assert (1, 0) in pairs

# Generator expression consumed by any/all
assert any(x > 3 for x in [1, 2, 3, 4, 5])
assert not any(x > 10 for x in [1, 2, 3])
assert all(x > 0 for x in [1, 2, 3])
assert not all(x > 2 for x in [1, 2, 3])

print("Generator tests passed!")
