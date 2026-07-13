import shlex

# quote
assert shlex.quote("safe") == "safe"
assert shlex.quote("has space") == "'has space'"
assert shlex.quote("") == "''"
assert shlex.quote("$HOME") == "'$HOME'"

# split
assert shlex.split("a b c") == ["a", "b", "c"]
assert shlex.split('x "hello world" y') == ["x", "hello world", "y"]
assert shlex.split("escaped\\ space") == ["escaped space"]

# join
assert shlex.join(["safe", "has space"]) == "safe 'has space'"

# round trip
original = ['cmd', '--flag="my value"', 'file with spaces.txt']
rt = shlex.split(shlex.join(original))
assert rt == original, f"round trip: {rt} != {original}"

True
