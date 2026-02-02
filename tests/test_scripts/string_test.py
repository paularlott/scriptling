# Test string methods

# find
s = "hello world"
assert s.find("world") == 6
assert s.find("xyz") == -1

# index
assert s.index("o") == 4

# count
assert s.count("o") == 2
assert s.count("l") == 3

# format
template = "Hello, {}!"
result = template.format("World")
assert result == "Hello, World!"

template2 = "{} + {} = {}"
result2 = template2.format(1, 2, 3)
assert result2 == "1 + 2 = 3"

# isdigit
assert "123".isdigit()
assert not "12a".isdigit()

# isalpha
assert "abc".isalpha()
assert not "ab1".isalpha()

# isalnum
assert "abc123".isalnum()
assert not "abc 123".isalnum()

# isspace
assert "   ".isspace()
assert not "  a  ".isspace()

# Test new string methods: expandtabs, casefold, maketrans, translate
result = "a\tb\tc".expandtabs()
assert len(result) == 17
assert result == "a       b       c"

result = "a\tb".expandtabs(4)
assert result == "a   b"

result = "01\t012\t0123\t01234".expandtabs(4)
assert result == "01  012 0123    01234"

# Test casefold
result = "HELLO".casefold()
assert result == "hello"

result = "HeLLo WoRLD".casefold()
assert result == "hello world"

# Test maketrans and translate
trans = "".maketrans("abc", "xyz")
result = "abcdef".translate(trans)
assert result == "xyzdef"

trans = "".maketrans("abc", "xyz", "def")
result = "abcdefghi".translate(trans)
assert result == "xyzghi"

trans = {"a": "1", "b": "2", "c": "3"}
result = "abc".translate(trans)
assert result == "123"

trans = {"a": "X", "b": None}
result = "aabbcc".translate(trans)
assert result == "XXcc"

# Test new string is* methods
assert "123".isnumeric()
assert not "12.3".isnumeric()
assert not "abc".isnumeric()
assert not "".isnumeric()
assert not "123abc".isnumeric()

assert "123".isdecimal()
assert not "12.3".isdecimal()
assert not "abc".isdecimal()
assert not "".isdecimal()

assert "Hello World".istitle()
assert not "Hello world".istitle()
assert not "HELLO WORLD".istitle()
assert not "hello world".istitle()
assert not "".istitle()
assert "Hello123World".istitle()

assert "hello".isidentifier()
assert "_hello".isidentifier()
assert "hello123".isidentifier()
assert not "123hello".isidentifier()
assert not "hello world".isidentifier()
assert not "".isidentifier()
assert "hello_world".isidentifier()

assert "Hello World".isprintable()
assert "".isprintable()
assert not "Hello\nWorld".isprintable()
assert not "Hello\tWorld".isprintable()

# Test string library constants
import string

assert len(string.ascii_letters) == 52
assert len(string.ascii_lowercase) == 26
assert len(string.ascii_uppercase) == 26
assert len(string.digits) == 10
assert len(string.hexdigits) == 22
assert len(string.octdigits) == 8
assert len(string.punctuation) > 0
assert len(string.whitespace) > 0
assert len(string.printable) > 0

assert "a" in string.ascii_lowercase
assert "Z" in string.ascii_uppercase
assert "5" in string.digits
assert "f" in string.hexdigits
assert "!" in string.punctuation

name = "Scriptling"
greeting = "Hello, " + name + "!"
assert greeting == "Hello, Scriptling!"

single = 'hello'
double = "hello"
triple = """hello"""
assert single == double and double == triple and triple == "hello"

# Test splitlines() - using a multi-line string with actual newlines
text = """hello
world"""
lines = text.splitlines()
assert len(lines) == 2 and lines[0] == 'hello' and lines[1] == 'world'
single_lines = "single".splitlines()
assert len(single_lines) == 1 and single_lines[0] == 'single'

# Test swapcase()
assert "Hello World".swapcase() == "hELLO wORLD"
assert "UPPER".swapcase() == "upper"
assert "lower".swapcase() == "LOWER"

# Test partition()
p1 = "hello-world".partition("-")
assert p1[0] == 'hello' and p1[1] == '-' and p1[2] == 'world'
p2 = "no sep here".partition("-")
assert p2[0] == 'no sep here' and p2[1] == '' and p2[2] == ''
p3 = "a-b-c".partition("-")
assert p3[0] == 'a' and p3[1] == '-' and p3[2] == 'b-c'

# Test rpartition()
p4 = "a-b-c".rpartition("-")
assert p4[0] == 'a-b' and p4[1] == '-' and p4[2] == 'c'
p5 = "hello".rpartition("-")
assert p5[0] == '' and p5[1] == '' and p5[2] == 'hello'
p6 = "one-two-three".rpartition("-")
assert p6[0] == 'one-two' and p6[1] == '-' and p6[2] == 'three'

# Test removeprefix()
assert "TestCase".removeprefix("Test") == "Case"
assert "TestCase".removeprefix("Foo") == "TestCase"
assert "Hello".removeprefix("He") == "llo"

# Test removesuffix()
assert "MyClass.py".removesuffix(".py") == "MyClass"
assert "MyClass.py".removesuffix(".txt") == "MyClass.py"
assert "filename.tar.gz".removesuffix(".gz") == "filename.tar"

# Test encode()
e1 = "ABC".encode()
assert len(e1) == 3 and e1[0] == 65 and e1[1] == 66 and e1[2] == 67
e2 = "hello".encode()
assert len(e2) == 5 and e2[0] == 104 and e2[4] == 111
e3 = "".encode()
assert len(e3) == 0