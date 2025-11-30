# Test copy library
import copy

# Test shallow copy of list
original = [1, 2, 3]
copied = copy.copy(original)
assert copied == [1, 2, 3]
assert len(copied) == 3

# Test deep copy of nested list
original = [1, [2, 3]]
copied = copy.deepcopy(original)
assert copied == [1, [2, 3]]
assert copied[0] == 1
assert copied[1] == [2, 3]

# Test shallow copy of dict
original = {"a": 1, "b": 2}
copied = copy.copy(original)
assert copied["a"] == 1
assert copied["b"] == 2

# Test deep copy of nested dict
original = {"a": 1, "b": {"c": 2}}
copied = copy.deepcopy(original)
assert copied["a"] == 1
assert copied["b"]["c"] == 2

# Test copy of tuple
original = (1, 2, 3)
copied = copy.copy(original)
assert copied == (1, 2, 3)

# Test copy of simple types
assert copy.copy(42) == 42
assert copy.copy(3.14) == 3.14
assert copy.copy("hello") == "hello"
assert copy.copy(True) == True
assert copy.copy(None) == None

# Test that copies are independent objects
original_list = [1, 2, 3]
shallow = copy.copy(original_list)
deep = copy.deepcopy(original_list)
original_list.append(4)
assert len(shallow) == 3  # Shallow copy not affected by append
assert len(deep) == 3     # Deep copy not affected by append
