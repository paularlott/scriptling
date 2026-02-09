# Test f-string escape sequences
# This test ensures that escape sequences like \n, \t, etc. work correctly in f-strings

# Test newline escape
name = "Paul"
result = f"\nHello, {name}!"
assert result == "\nHello, Paul!", f"Expected newline at start, got: {repr(result)}"

# Test tab escape
value = 42
result = f"Value:\t{value}"
assert result == "Value:\t42", f"Expected tab, got: {repr(result)}"

# Test carriage return
result = f"Line1\rLine2"
assert result == "Line1\rLine2", f"Expected carriage return, got: {repr(result)}"

# Test backslash escape
result = f"Path: C:\\Users\\{name}"
assert result == "Path: C:\\Users\\Paul", f"Expected backslashes, got: {repr(result)}"

# Test double quote escape
result = f"He said: \"{name} is here\""
assert result == 'He said: "Paul is here"', f"Expected escaped quotes, got: {repr(result)}"

# Test single quote escape in double-quoted f-string
result = f"It's {name}'s book"
assert result == "It's Paul's book", f"Expected single quotes, got: {repr(result)}"

# Test multiple escape sequences in one f-string
result = f"\n\tName: {name}\n\tValue: {value}\n"
expected = "\n\tName: Paul\n\tValue: 42\n"
assert result == expected, f"Expected multiple escapes, got: {repr(result)}"

# Test escape sequences before expression
result = f"\nContent: {name}"
assert result[0] == "\n", f"Expected newline at position 0, got: {repr(result[0])}"
assert result == "\nContent: Paul", f"Expected newline before content, got: {repr(result)}"

# Test escape sequences after expression
result = f"{name}\nNext line"
assert "\n" in result, f"Expected newline in result, got: {repr(result)}"
assert result == "Paul\nNext line", f"Expected newline after name, got: {repr(result)}"

# Test escape sequences between expressions
x = 10
y = 20
result = f"{x}\n{y}"
assert result == "10\n20", f"Expected newline between values, got: {repr(result)}"

# Test null byte escape (edge case - may have display issues but should work internally)
result = f"Before\0After"
# Just verify it doesn't crash and has the right structure
assert "Before" in result and "After" in result, f"Expected Before and After in result"

# Test escaped braces with escape sequences
result = f"\n{{{name}}}\n"
assert result == "\n{Paul}\n", f"Expected newlines with braces, got: {repr(result)}"

# Test complex combination
result = f"\tUser: \"{name}\"\n\tScore: {value}\n"
expected = '\tUser: "Paul"\n\tScore: 42\n'
assert result == expected, f"Expected complex escape combo, got: {repr(result)}"

# Test that unknown escape sequences are preserved (backslash + char)
result = f"Test: \x{name}"
assert result == "Test: \\xPaul", f"Expected preserved unknown escape, got: {repr(result)}"

# Test empty f-string with just escape
result = f"\n"
assert result == "\n", f"Expected just newline, got: {repr(result)}"

# Test multiple tabs
result = f"\t\t{name}"
assert result == "\t\tPaul", f"Expected two tabs, got: {repr(result)}"

# Test escape at end
result = f"{name}\n"
assert result == "Paul\n", f"Expected newline at end, got: {repr(result)}"

# Test mixed quotes and escapes
result = f"Path: \"C:\\Users\\{name}\\Documents\""
assert result == 'Path: "C:\\Users\\Paul\\Documents"', f"Expected path with quotes and backslashes, got: {repr(result)}"

print("All f-string escape sequence tests passed!")
True
