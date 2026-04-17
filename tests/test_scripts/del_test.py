# del statement coverage for list indexes, slices, dict keys, and attributes

# List index deletion
items = [10, 20, 30, 40]
del items[1]
assert items == [10, 30, 40]

# Negative list index deletion
del items[-1]
assert items == [10, 30]

# List slice deletion
values = [0, 1, 2, 3, 4, 5]
del values[1:5:2]
assert values == [0, 2, 4, 5]

# Slice deletion with reverse step
reverse_values = [0, 1, 2, 3, 4, 5]
del reverse_values[4:1:-2]
assert reverse_values == [0, 1, 3, 5]

# Dict key deletion
data = {"name": "Alice", "age": 30, "active": True}
del data["age"]
assert "age" not in data
assert data["name"] == "Alice"

# Dot access on dict
config = {"debug": True, "port": 8080}
del config.debug
assert "debug" not in config
assert config.port == 8080

# Attribute deletion
class User:
    def __init__(self):
        self.name = "Alice"
        self.email = "alice@example.com"

user = User()
del user.email
assert user.email == None
assert user.name == "Alice"

# Missing targets raise catchable exceptions
missing_index = False
try:
    del items[99]
except IndexError:
    missing_index = True
assert missing_index == True

missing_key = False
try:
    del data["missing"]
except KeyError:
    missing_key = True
assert missing_key == True

missing_attr = False
try:
    del user.email
except AttributeError:
    missing_attr = True
assert missing_attr == True

# Custom __delitem__ support
class Bucket:
    def __init__(self):
        self.values = ["a", "b", "c", "d"]

    def __delitem__(self, key):
        if type(key) == "SLICE":
            self.values = [self.values[0]]
        else:
            self.values = self.values[:key] + self.values[key + 1:]

bucket = Bucket()
del bucket[1]
assert bucket.values == ["a", "c", "d"]

del bucket[1:3]
assert bucket.values == ["a"]

True
