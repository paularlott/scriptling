# Test itertools library
import itertools

# Test chain
result = itertools.chain([1, 2], [3, 4])
assert result == [1, 2, 3, 4]

result = itertools.chain([1], [2], [3])
assert result == [1, 2, 3]

# Test repeat
result = itertools.repeat("x", 3)
assert result == ["x", "x", "x"]

result = itertools.repeat(0, 5)
assert result == [0, 0, 0, 0, 0]

# Test cycle
result = itertools.cycle([1, 2], 3)
assert result == [1, 2, 1, 2, 1, 2]

# Test count
result = itertools.count(0, 5)
assert result == [0, 1, 2, 3, 4]

result = itertools.count(0, 10, 2)
assert result == [0, 2, 4, 6, 8]

result = itertools.count(5, 0, -1)
assert result == [5, 4, 3, 2, 1]

# Test islice
result = itertools.islice([0, 1, 2, 3, 4], 3)
assert result == [0, 1, 2]

result = itertools.islice([0, 1, 2, 3, 4], 1, 4)
assert result == [1, 2, 3]

result = itertools.islice([0, 1, 2, 3, 4], 0, 5, 2)
assert result == [0, 2, 4]

# Test zip_longest
result = itertools.zip_longest([1, 2, 3], ["a", "b"])
assert len(result) == 3
assert result[0] == (1, "a")
assert result[1] == (2, "b")
assert result[2][0] == 3

# Test product
result = itertools.product([1, 2], ["a", "b"])
assert len(result) == 4
assert result[0] == (1, "a")
assert result[1] == (1, "b")
assert result[2] == (2, "a")
assert result[3] == (2, "b")

# Test permutations
result = itertools.permutations([1, 2, 3], 2)
assert len(result) == 6
assert (1, 2) in result
assert (2, 1) in result
assert (1, 3) in result
assert (3, 1) in result
assert (2, 3) in result
assert (3, 2) in result

# Test combinations
result = itertools.combinations([1, 2, 3], 2)
assert len(result) == 3
assert (1, 2) in result
assert (1, 3) in result
assert (2, 3) in result

# Test combinations_with_replacement
result = itertools.combinations_with_replacement([1, 2], 2)
assert len(result) == 3
assert (1, 1) in result
assert (1, 2) in result
assert (2, 2) in result

# Test accumulate
result = itertools.accumulate([1, 2, 3, 4])
assert result == [1, 3, 6, 10]

# Test compress
result = itertools.compress([1, 2, 3, 4], [True, False, True, False])
assert result == [1, 3]

# Test pairwise
result = itertools.pairwise([1, 2, 3, 4])
assert len(result) == 3
assert result[0] == (1, 2)
assert result[1] == (2, 3)
assert result[2] == (3, 4)

# Test batched
result = itertools.batched([1, 2, 3, 4, 5], 2)
assert len(result) == 3
assert result[0] == (1, 2)
assert result[1] == (3, 4)
assert result[2] == (5,)

# Test groupby (simple case)
result = itertools.groupby([1, 1, 2, 2, 3])
assert len(result) == 3
assert result[0][0] == 1
assert result[0][1] == [1, 1]
assert result[1][0] == 2
assert result[1][1] == [2, 2]

True
