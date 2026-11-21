from urllib.parse import urlencode

# Values that are not strings are written as str() shows them, and pairs keep
# their order.
assert urlencode({"z": 1, "b": "x y", "c": 1.5, "d": True}) == "z=1&b=x+y&c=1.5&d=True"
assert urlencode([("k", 2), ("a", "b&c")]) == "k=2&a=b%26c"
assert urlencode({"f": [1, "a"]}, doseq=True) == "f=1&f=a"
assert urlencode({}) == ""
# Without doseq a sequence value is written as its str(); with it, expanded.
assert urlencode({"a": [1, 2]}) == "a=%5B1%2C+2%5D"
assert urlencode({"a": [1, 2]}, doseq=True) == "a=1&a=2"
assert urlencode({"a": (1, "x y")}, True) == "a=1&a=x+y"
assert urlencode([("a", ["x"])]) == "a=%5B%27x%27%5D"
assert urlencode({"a": "abc"}, doseq=True) == "a=abc"
assert urlencode({"a": None}) == "a=None"

print("OK")
