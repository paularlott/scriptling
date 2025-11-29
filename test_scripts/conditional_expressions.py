# Test conditional expressions (ternary operator: x if cond else y)

# Basic cases
x = 1 if True else 2
assert x == 1, f"Expected 1, got {x}"

y = 1 if False else 2
assert y == 2, f"Expected 2, got {y}"

# With variables as conditions
cond = True
a = "yes" if cond else "no"
assert a == "yes", f"Expected 'yes', got {a}"

cond = False
b = "yes" if cond else "no"
assert b == "no", f"Expected 'no', got {b}"

# Nested conditional expressions
val = 1 if False else 2 if True else 3
assert val == 2, f"Expected 2, got {val}"

val2 = 1 if True else 2 if True else 3
assert val2 == 1, f"Expected 1, got {val2}"

val3 = 1 if False else 2 if False else 3
assert val3 == 3, f"Expected 3, got {val3}"

# Conditional expressions with expressions, not just literals
num = 5
result = num * 2 if num > 3 else num + 1
assert result == 10, f"Expected 10, got {result}"

result2 = num * 2 if num > 10 else num + 1
assert result2 == 6, f"Expected 6, got {result2}"

# Conditional expressions in lists
items = [1 if True else 0, 2 if False else 20]
assert items[0] == 1, f"Expected 1, got {items[0]}"
assert items[1] == 20, f"Expected 20, got {items[1]}"

# Conditional expression followed by if statement (different constructs)
z = 100
if True:
    z = z + 1
assert z == 101, f"Expected 101, got {z}"

# Assignment with conditional, then if statement on next line
w = 50 if True else 25
if w > 40:
    w = w + 10
assert w == 60, f"Expected 60, got {w}"

# Multiple assignments with conditionals followed by if statements
p = 1 if True else 2
q = 3 if False else 4
if p == 1:
    p = p + q
assert p == 5, f"Expected 5, got {p}"
assert q == 4, f"Expected 4, got {q}"

# Conditional expression in function return
def get_sign(n):
    return "positive" if n > 0 else "non-positive"

assert get_sign(5) == "positive"
assert get_sign(-3) == "non-positive"
assert get_sign(0) == "non-positive"

# Conditional expression with string operations
name = "Alice"
greeting = "Hello, " + name if len(name) > 0 else "Hello, stranger"
assert greeting == "Hello, Alice", f"Expected 'Hello, Alice', got {greeting}"

# Empty string condition
empty = ""
msg = "has content" if empty else "empty"
assert msg == "empty", f"Expected 'empty', got {msg}"

# None condition
val_none = None
result_none = "has value" if val_none else "no value"
assert result_none == "no value", f"Expected 'no value', got {result_none}"

# List as condition
empty_list = []
non_empty = [1, 2, 3]
r1 = "not empty" if empty_list else "empty"
r2 = "not empty" if non_empty else "empty"
assert r1 == "empty", f"Expected 'empty', got {r1}"
assert r2 == "not empty", f"Expected 'not empty', got {r2}"

# Conditional expression in list comprehension (the expression part, not the filter)
nums = [1, 2, 3, 4, 5]
labels = ["even" if x % 2 == 0 else "odd" for x in nums]
assert labels == ["odd", "even", "odd", "even", "odd"], f"Got {labels}"

# List comprehension with filter (if is filter, not conditional)
evens = [x for x in nums if x % 2 == 0]
assert evens == [2, 4], f"Got {evens}"

# Both: conditional expression in expression AND filter
classified = ["EVEN" if x % 2 == 0 else "odd" for x in nums if x > 1]
assert classified == ["EVEN", "odd", "EVEN", "odd"], f"Got {classified}"

print("All conditional expression tests passed!")
True