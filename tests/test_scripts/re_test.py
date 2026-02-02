import re

# Test re.match at start of string - returns Match object
m = re.match("[0-9]+", "123abc")
assert m != None and m.group(0) == "123"

# Test re.match returns None when pattern doesn't match at start
assert re.match("[0-9]+", "abc123") == None

# Test re.search finds pattern anywhere - returns Match object
result = re.search("[0-9]+", "abc123def")
assert result != None and result.group(0) == "123"

# Test re.findall finds all matches
matches = re.findall("[0-9]+", "a1b2c3")
assert len(matches) == 3

# Test re.sub with Python signature (pattern, repl, string)
result = re.sub("[0-9]+", "X", "a1b2c3")
assert result == "aXbXcX"

# Test re.sub with count parameter
result = re.sub("[0-9]+", "X", "a1b2c3", 2)
assert result == "aXbXc3"

# Test re.split splits on pattern
parts = re.split("[,;]", "a,b;c")
assert len(parts) == 3

# Test re.split with maxsplit
parts = re.split("[,;]", "a,b;c;d", 2)
assert len(parts) == 2

# Test re.fullmatch matches entire string
assert re.fullmatch("[0-9]+", "123")

# Test re.fullmatch fails on partial match
assert not re.fullmatch("[0-9]+", "123abc")

# Test re.escape escapes special characters
escaped = re.escape("a.b+c*")
assert len(escaped) > len("a.b+c*")

# Test re.compile returns a Regex object
pattern = re.compile("hello")
assert type(pattern) == "Regex"

# Test Word boundary matching - use simpler pattern
words = re.findall("[A-Za-z]+", "Hello, World!")
assert len(words) == 2

# Test Case insensitive with (?i) inline flag
result = re.match("(?i)hello", "HELLO world")
assert result != None and result.group(0) == "HELLO"

# Test re.search returns None on no match
result = re.search("[0-9]+", "no numbers here")
assert result == None

# Test re.match with IGNORECASE flag
result = re.match("hello", "HELLO world", re.IGNORECASE)
assert result != None and result.group(0) == "HELLO"

# Test re.match with I shorthand flag
result = re.match("hello", "HELLO world", re.I)
assert result != None and result.group(0) == "HELLO"

# Test re.search with IGNORECASE flag
result = re.search("world", "HELLO WORLD", re.I)
assert result != None and result.group(0) == "WORLD"

# Test re.findall with IGNORECASE flag
matches = re.findall("a+", "aAbBaAa", re.I)
assert len(matches) == 2

# Test re.compile with flags returns Regex object
pattern = re.compile("hello", re.I)
assert type(pattern) == "Regex"

# Test Combined flags (IGNORECASE | MULTILINE)
pattern = re.compile("hello", re.I | re.M)
assert type(pattern) == "Regex"

# Test MULTILINE flag for ^ and $ matching
result = re.match("^line", "line1\nline2", re.M)
assert result != None and result.group(0) == "line"

# Test DOTALL flag - dot matches newline
result = re.search("(?s)a.*b", "a\nb")
assert result != None and result.group(0) == "a\nb"

# Test re.finditer returns list of Match objects
matches = re.finditer("[0-9]+", "a1b2c3")
assert len(matches) == 3
assert type(matches[0]) == "Match" and matches[0].group(0) == "1"

# Additional simple tests
phones = re.findall("[0-9]{3}-[0-9]{4}", "Call 555-1234 or 555-5678")
assert len(phones) == 2