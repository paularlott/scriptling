# Test regex compile and method calls
import re

fails = 0

# Test 1: Basic re.compile returns Regex object
pattern = re.compile(r"[0-9]+")
if type(pattern) != "Regex":
    fails = fails + 1

# Test 2: pattern.findall with no groups
pattern = re.compile(r"[0-9]+")
matches = pattern.findall("a1b2c3")
if len(matches) != 3:
    fails = fails + 1

# Test 3: pattern.findall with single group
pattern = re.compile(r"([0-9]+)")
matches = pattern.findall("a1b2c3")
if len(matches) != 3:
    fails = fails + 1

# Test 4: pattern.findall with multiple groups
pattern = re.compile(r"([a-z])([0-9])")
matches = pattern.findall("a1b2c3")
if len(matches) != 3:
    fails = fails + 1
# Each match should be a tuple of 2 elements
for m in matches:
    if len(m) != 2:
        fails = fails + 1

# Test 5: re.compile with IGNORECASE flag
pattern = re.compile(r"hello", re.I)
if type(pattern) != "Regex":
    fails = fails + 1

# Test 6: re.compile with DOTALL flag
pattern = re.compile(r"a.*b", re.S)
if type(pattern) != "Regex":
    fails = fails + 1

# Test 7: Combined flags
pattern = re.compile(r"hello", re.I | re.M)
if type(pattern) != "Regex":
    fails = fails + 1

# Test 8: Using compiled pattern in findall
pattern = re.compile(r'<a href="([^"]+)">([^<]+)</a>')
html = '<a href="link1.html">Link 1</a> and <a href="link2.html">Link 2</a>'
matches = pattern.findall(html)
if len(matches) != 2:
    fails = fails + 1

# Test 9: Unpack results from compiled pattern findall
pattern = re.compile(r'<a href="([^"]+)">([^<]+)</a>')
html = '<a href="link1.html">Link 1</a>'
for href, text in pattern.findall(html):
    if href != "link1.html":
        fails = fails + 1
    if text != "Link 1":
        fails = fails + 1

fails == 0
