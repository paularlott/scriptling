# Test new string methods: expandtabs, casefold, maketrans, translate

# Test expandtabs
result = "a\tb\tc".expandtabs()
print("expandtabs result:")
print(repr(result))
print(len(result))
assert len(result) == 17, "expandtabs() length should be 17"
assert result == "a       b       c", "expandtabs() default"

result = "a\tb".expandtabs(4)
assert result == "a   b", "expandtabs(4)"

result = "01\t012\t0123\t01234".expandtabs(4)
assert result == "01  012 0123    01234", "expandtabs with varying column positions"

# Test casefold
result = "HELLO".casefold()
assert result == "hello", "casefold() basic"

result = "HeLLo WoRLD".casefold()
assert result == "hello world", "casefold() mixed case"

# Test maketrans and translate
# Two-argument form
trans = "".maketrans("abc", "xyz")
result = "abcdef".translate(trans)
assert result == "xyzdef", "translate() basic substitution"

# With delete characters (three arguments)
trans = "".maketrans("abc", "xyz", "def")
result = "abcdefghi".translate(trans)
assert result == "xyzghi", "translate() with deletion"

# Single dict argument
trans = {"a": "1", "b": "2", "c": "3"}
result = "abc".translate(trans)
assert result == "123", "translate() with dict"

# Test with None values (deletion)
trans = {"a": "X", "b": None}
result = "aabbcc".translate(trans)
assert result == "XXcc", "translate() with None deletion"

print("All new string method tests passed!")

# Return true for test framework
True
