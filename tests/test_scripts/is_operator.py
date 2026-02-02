# Test is / is not identity operators

# Test None identity
x = None
y = None
assert x is None, "x is None"
assert y is None, "y is None"
assert x is y, "x is y (both None)"
assert not (x is not None), "x is not not None"

# Test boolean identity
a = True
b = True
assert a is True, "a is True"
assert b is True, "b is True"
assert a is b, "a is b (both True)"

c = False
d = False
assert c is False, "c is False"
assert d is False, "d is False"
assert c is d, "c is d (both False)"

# Test is not
assert True is not False, "True is not False"
assert False is not True, "False is not True"
assert None is not True, "None is not True"
assert None is not False, "None is not False"

# Test with integers (Python caches small ints -5 to 256)
x = 100
y = 100
assert x is y, "small integers should be cached"

# Test with strings (Python interns short strings)
s1 = "hello"
s2 = "hello"
assert s1 is s2, "short strings should be interned"

# Test list identity
list1 = [1, 2, 3]
list2 = [1, 2, 3]
list3 = list1

# Lists with same content are equal but not identical
assert list1 == list2, "list1 == list2"
assert list1 is not list2, "list1 is not list2 (different objects)"
assert list1 is list3, "list1 is list3 (same object)"

# Test dict identity
dict1 = {"a": 1}
dict2 = {"a": 1}
dict3 = dict1

assert dict1 == dict2, "dict1 == dict2"
assert dict1 is not dict2, "dict1 is not dict2 (different objects)"
assert dict1 is dict3, "dict1 is dict3 (same object)"

print("All is/is not tests passed!")

# Return true for test framework
True
