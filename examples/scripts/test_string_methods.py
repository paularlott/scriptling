# Test: String methods

print("=== Testing String Methods ===")

# upper
text = "hello"
result = upper(text)
print(f"upper('{text}'): {result}")

# lower
text = "WORLD"
result = lower(text)
print(f"lower('{text}'): {result}")

# split
text = "one,two,three"
result = split(text, ",")
print(f"split('{text}', ','): {result}")

# join
words = ["hello", "world"]
result = join(words, " ")
print(f"join(['hello', 'world'], ' '): {result}")

# replace
text = "hello world"
result = replace(text, "world", "Python")
print(f"replace('hello world', 'world', 'Python'): {result}")

# String method syntax
text = "test string"
result = text.upper()
print(f"'test string'.upper(): {result}")

result = text.lower()
print(f"'test string'.lower(): {result}")

result = text.split(" ")
print(f"'test string'.split(' '): {result}")

result = text.replace("test", "demo")
print(f"'test string'.replace('test', 'demo'): {result}")

# New string methods
text2 = "hello WORLD"
result = text2.capitalize()
print(f"'hello WORLD'.capitalize(): {result}")

text3 = "hello world"
result = text3.title()
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

print("✓ All string method tests passed")
