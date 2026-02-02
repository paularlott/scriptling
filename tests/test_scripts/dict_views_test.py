# Test dict views

d = {"a": 1, "b": 2}
keys = d.keys()
values = d.values()
items = d.items()

# Test types
assert type(keys) == "DICT_KEYS"
assert type(values) == "DICT_VALUES"
assert type(items) == "DICT_ITEMS"

# Test len
assert len(keys) == 2
assert len(values) == 2
assert len(items) == 2

# Test iteration
k_list = []
for k in keys:
    k_list.append(k)
# Order is not guaranteed, so sort
k_list.sort()
assert k_list[0] == "a"
assert k_list[1] == "b"

# Test conversion to list
k_list2 = list(keys)
k_list2.sort()
assert k_list2 == k_list

# Test reflection of changes
d["c"] = 3
assert len(keys) == 3
assert len(values) == 3
assert len(items) == 3

# Test iteration after change
k_list3 = list(keys)
assert len(k_list3) == 3

# Test tuple conversion
t = tuple(keys)
assert len(t) == 3

# Test set conversion (returns list currently)
s = set(keys)
assert len(s) == 3

# Test 'in' operator
assert "a" in keys
assert "z" not in keys
assert 1 in values
assert 99 not in values
# Items check
t_item = ("a", 1)
assert t_item in items

print("Dict view tests passed!")
