import re

# Comprehensive regex tests - multiple tests counting failures
fails = 0

# Test 1: re.match at start of string
if not re.match("[0-9]+", "123abc"):
    fails = fails + 1

# Test 2: re.match returns false when pattern doesn't match at start
if re.match("[0-9]+", "abc123"):
    fails = fails + 1

# Test 3: re.search finds pattern anywhere
result = re.search("[0-9]+", "abc123def")
if result != "123":
    fails = fails + 1

# Test 4: re.findall finds all matches
matches = re.findall("[0-9]+", "a1b2c3")
if len(matches) != 3:
    fails = fails + 1

# Test 5: re.sub with Python signature (pattern, repl, string)
result = re.sub("[0-9]+", "X", "a1b2c3")
if result != "aXbXcX":
    fails = fails + 1

# Test 6: re.sub with count parameter
result = re.sub("[0-9]+", "X", "a1b2c3", 2)
if result != "aXbXc3":
    fails = fails + 1

# Test 7: re.split splits on pattern
parts = re.split("[,;]", "a,b;c")
if len(parts) != 3:
    fails = fails + 1

# Test 8: re.split with maxsplit
parts = re.split("[,;]", "a,b;c;d", 2)
if len(parts) != 2:
    fails = fails + 1

# Test 9: re.fullmatch matches entire string
if not re.fullmatch("[0-9]+", "123"):
    fails = fails + 1

# Test 10: re.fullmatch fails on partial match
if re.fullmatch("[0-9]+", "123abc"):
    fails = fails + 1

# Test 11: re.escape escapes special characters
escaped = re.escape("a.b+c*")
# The escaped string should contain backslashes before special chars
if len(escaped) <= len("a.b+c*"):
    fails = fails + 1

# Test 12: re.compile validates pattern
pattern = re.compile("[0-9]+")
if pattern != "[0-9]+":
    fails = fails + 1

# Test 13: Word boundary matching - use simpler pattern
words = re.findall("[A-Za-z]+", "Hello, World!")
if len(words) != 2:
    fails = fails + 1

# Test 14: Case insensitive with (?i) inline flag
result = re.match("(?i)hello", "HELLO world")
if not result:
    fails = fails + 1

# Test 15: re.search returns None on no match
result = re.search("[0-9]+", "no numbers here")
if result != None:
    fails = fails + 1

# Test 16: re.match with IGNORECASE flag
if not re.match("hello", "HELLO world", re.IGNORECASE):
    fails = fails + 1

# Test 17: re.match with I shorthand flag
if not re.match("hello", "HELLO world", re.I):
    fails = fails + 1

# Test 18: re.search with IGNORECASE flag
result = re.search("world", "HELLO WORLD", re.I)
if result != "WORLD":
    fails = fails + 1

# Test 19: re.findall with IGNORECASE flag
# "aAbBaAa" with case-insensitive 'a+' matches: "aA" and "aAa" = 2 matches
matches = re.findall("a+", "aAbBaAa", re.I)
if len(matches) != 2:
    fails = fails + 1

# Test 20: re.compile with flags
pattern = re.compile("hello", re.I)
if pattern != "(?i)hello":
    fails = fails + 1

# Test 21: Combined flags (IGNORECASE | MULTILINE)
pattern = re.compile("hello", re.I | re.M)
# Pattern should contain both i and m flags in (?...) format
# Go combines them as (?im) not (?i)(?m)
if "(?i" not in pattern or "m)" not in pattern:
    fails = fails + 1

# Test 22: MULTILINE flag for ^ and $ matching
if not re.match("^line", "line1\nline2", re.M):
    fails = fails + 1

# Test 23: DOTALL flag - dot matches newline
# Without DOTALL, .* won't match across newlines
# With DOTALL ((?s)), . matches everything including newlines
result = re.search("(?s)a.*b", "a\nb")
if result != "a\nb":
    fails = fails + 1

fails == 0
