# Example: String methods in Scriptling
# This script demonstrates various string manipulation methods available in Scriptling

print("String Methods Demonstration")

# Basic case conversion
text = "hello"
result = text.upper()
print(f"'{text}'.upper(): {result}")

text = "WORLD"
result = text.lower()
print(f"'{text}'.lower(): {result}")

# Splitting strings
text = "one,two,three"
result = text.split(",")
print(f"'{text}'.split(','): {result}")

# Joining strings
words = ["hello", "world"]
result = " ".join(words)
print(f"' '.join(['hello', 'world']): {result}")

# Replacing substrings
text = "hello world"
result = text.replace("world", "Python")
print(f"'{text}'.replace('world', 'Python'): {result}")

# More string methods
text = "test string"
result = text.upper()
print(f"'{text}'.upper(): {result}")

result = text.lower()
print(f"'{text}'.lower(): {result}")

result = text.split(" ")
print(f"'{text}'.split(' '): {result}")

result = text.replace("test", "demo")
print(f"'{text}'.replace('test', 'demo'): {result}")

# Additional methods
text2 = "hello WORLD"
result = text2.capitalize()
print(f"'{text2}'.capitalize(): {result}")

text3 = "hello world"
result = text3.title()
print(f"'{text3}'.title(): {result}")

text4 = "  spaces  "
result = text4.strip()
print(f"'{text4}'.strip(): '{result}'")

text5 = "hello world"
if text5.startswith("hello"):
    print(f"'{text5}'.startswith('hello'): True")

if text5.endswith("world"):
    print(f"'{text5}'.endswith('world'): True")

# Split without arguments (splits on whitespace)
text6 = "one two three"
result = text6.split()
print(f"'{text6}'.split(): {result}")

# Join as method
result = " ".join(["a", "b", "c"])
print(f"' '.join(['a', 'b', 'c']): {result}")
print(f"'hello world'.title(): {result}")

text4 = "  spaces  "
result = text4.strip()
print(f"'  spaces  '.strip(): '{result}'")

text5 = "hello world"
if text5.startswith("hello"):
    print(f"'hello world'.startswith('hello'): True")

if text5.endswith("world"):
    print(f"'hello world'.endswith('world'): True")

# split with no args (whitespace)
text6 = "one two three"
result = text6.split()
print(f"'one two three'.split(): {result}")

# join as method
result = " ".join(["a", "b", "c"])
print(f"' '.join(['a', 'b', 'c']): {result}")
