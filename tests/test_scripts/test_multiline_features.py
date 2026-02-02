
# Test Multiline Features

# 1. Multiline Function Definitions
def multiline_func(
    a,
    b,
    c
):
    return a + b + c

assert multiline_func(1, 2, 3) == 6, "Multiline function definition failed"

def multiline_func_defaults(
    a,
    b=2,
    c=3
):
    return a + b + c

assert multiline_func_defaults(1) == 6, "Multiline function definition with defaults failed"

# 2. Multiline Function Calls
result = multiline_func(
    1,
    2,
    3
)
assert result == 6, "Multiline function call failed"

result = multiline_func(
    1,
    b=2,
    c=3
)
assert result == 6, "Multiline function call with kwargs failed"

# 3. Multiline List Definitions
my_list = [
    1,
    2,
    3,
    4,
    5
]
assert len(my_list) == 5, "Multiline list definition failed"
assert my_list[0] == 1, "Multiline list element 0 failed"
assert my_list[4] == 5, "Multiline list element 4 failed"

# Trailing comma
my_list_trailing = [
    1,
    2,
    3,
]
assert len(my_list_trailing) == 3, "Multiline list with trailing comma failed"


# 4. Multiline Dict Definitions
my_dict = {
    "a": 1,
    "b": 2,
    "c": 3
}
assert my_dict["a"] == 1, "Multiline dict definition failed"
assert my_dict["c"] == 3, "Multiline dict element failed"

# Trailing comma
my_dict_trailing = {
    "a": 1,
    "b": 2,
}
assert my_dict_trailing["b"] == 2, "Multiline dict with trailing comma failed"

print("✓ All multiline tests passed")
