from urllib.parse import urlencode

# Values that are not strings are written as str() shows them, and pairs keep
# their order.
assert urlencode({"z": 1, "b": "x y", "c": 1.5, "d": True}) == "z=1&b=x+y&c=1.5&d=True"
assert urlencode([("k", 2), ("a", "b&c")]) == "k=2&a=b%26c"
assert urlencode({"f": [1, "a"]}, doseq=True) == "f=1&f=a"
assert urlencode({}) == ""

print("OK")
