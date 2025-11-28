# Test new string is* methods

# Test isnumeric
assert "123".isnumeric() == True
assert "12.3".isnumeric() == False
assert "abc".isnumeric() == False
assert "".isnumeric() == False
assert "123abc".isnumeric() == False

# Test isdecimal
assert "123".isdecimal() == True
assert "12.3".isdecimal() == False
assert "abc".isdecimal() == False
assert "".isdecimal() == False

# Test istitle
assert "Hello World".istitle() == True
assert "Hello world".istitle() == False
assert "HELLO WORLD".istitle() == False
assert "hello world".istitle() == False
assert "".istitle() == False
assert "Hello123World".istitle() == True

# Test isidentifier
assert "hello".isidentifier() == True
assert "_hello".isidentifier() == True
assert "hello123".isidentifier() == True
assert "123hello".isidentifier() == False
assert "hello world".isidentifier() == False
assert "".isidentifier() == False
assert "hello_world".isidentifier() == True

# Test isprintable
assert "Hello World".isprintable() == True
assert "".isprintable() == True
assert "Hello\nWorld".isprintable() == False
assert "Hello\tWorld".isprintable() == False

True
