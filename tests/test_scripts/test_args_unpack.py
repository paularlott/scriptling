# Test *args unpacking in function calls
failures = 0

# Test 1: Basic list unpacking
def sum_three(a, b, c):
    return a + b + c

args_list = [1, 2, 3]
result = sum_three(*args_list)
if result != 6:
    print("FAIL: Test 1 - Basic list unpacking")
    print(f"  Expected: 6, Got: {result}")
    failures += 1

# Test 2: Tuple unpacking
args_tuple = (4, 5, 6)
result = sum_three(*args_tuple)
if result != 15:
    print("FAIL: Test 2 - Tuple unpacking")
    print(f"  Expected: 15, Got: {result}")
    failures += 1

# Test 3: Partial unpacking with positional args
result = sum_three(10, *args_list[1:])
if result != 15:
    print("FAIL: Test 3 - Partial unpacking")
    print(f"  Expected: 15, Got: {result}")
    failures += 1

# Test 4: Unpacking with *args parameter
def sum_all(*args):
    total = 0
    for arg in args:
        total += arg
    return total

numbers = [1, 2, 3, 4, 5]
result = sum_all(*numbers)
if result != 15:
    print("FAIL: Test 4 - Unpacking with *args parameter")
    print(f"  Expected: 15, Got: {result}")
    failures += 1

# Test 5: Mixing unpacked and regular args
def combine(a, b, c, d, e):
    return [a, b, c, d, e]

# Combine unpacked list with regular args
args_list = [1, 2, 3, 4]
result = combine(10, *args_list[:4])
# Note: This would error because we have 5 params but passing 5 args (10 + 4 from list)
# Let's test a simpler case
def sum_four(a, b, c, d):
    return a + b + c + d

args = [1, 2]
result = sum_four(5, 6, *args)
if result != 14:
    print("FAIL: Test 5a - Mixing unpacked and regular args")
    print(f"  Expected: 14, Got: {result}")
    failures += 1

# Test unpacking at different positions
result = sum_four(*args, 7, 8)
if result != 18:
    print("FAIL: Test 5b - Unpacking at start")
    print(f"  Expected: 18, Got: {result}")
    failures += 1

# Test 6: Empty list unpacking
def no_params():
    return "success"

result = no_params(*[])
if result != "success":
    print("FAIL: Test 6 - Empty list unpacking")
    print(f"  Expected: 'success', Got: {result}")
    failures += 1

# Test 7: Unpacking strings (each char becomes an arg)
def concat_three(a, b, c):
    return a + b + c

result = concat_three(*"abc")
if result != "abc":
    print("FAIL: Test 7 - String unpacking")
    print(f"  Expected: 'abc', Got: {result}")
    failures += 1

# Test 8: Unpacking with keyword arguments
def func_with_keywords(a, b, c=10, d=20):
    return a + b + c + d

args = [1, 2]
result = func_with_keywords(*args, c=100)
if result != 123:
    print("FAIL: Test 8 - Unpacking with keyword arguments")
    print(f"  Expected: 123, Got: {result}")
    failures += 1

# Test 9: Unpacking with **kwargs
def combined_func(a, b, *args, **kwargs):
    return {
        'a': a,
        'b': b,
        'args': args,
        'kwargs': kwargs
    }

args = [1, 2, 3, 4]
kwargs = {'x': 10, 'y': 20}
result = combined_func(*args, **kwargs)
if result['a'] != 1 or result['b'] != 2:
    print("FAIL: Test 9a - Unpacking with **kwargs (positional)")
    failures += 1
if len(result['args']) != 2 or result['args'][0] != 3 or result['args'][1] != 4:
    print("FAIL: Test 9b - Unpacking with **kwargs (args)")
    print(f"  Expected args: [3, 4], Got: {result['args']}")
    failures += 1
if result['kwargs']['x'] != 10 or result['kwargs']['y'] != 20:
    print("FAIL: Test 9c - Unpacking with **kwargs (kwargs)")
    failures += 1

# Test 10: Method call with *args
class TestClass:
    def method(self, a, b, c):
        return a * b + c

obj = TestClass()
method_args = [5, 3, 2]
result = obj.method(*method_args)
if result != 17:
    print("FAIL: Test 10 - Method call with *args")
    print(f"  Expected: 17, Got: {result}")
    failures += 1

# Test 11: Lambda with unpacked args
lambda_func = lambda x, y, z: x + y + z
lambda_args = [10, 20, 30]
result = lambda_func(*lambda_args)
if result != 60:
    print("FAIL: Test 11 - Lambda with unpacked args")
    print(f"  Expected: 60, Got: {result}")
    failures += 1

# Test 12: Nested list unpacking
def nested_test(a, b, c, d, e, f):
    return sum([a, b, c, d, e, f])

nested = [[1, 2], [3, 4], [5, 6]]
# Need to flatten first
flat = []
for n in nested:
    flat.extend(n)
result = nested_test(*flat)
if result != 21:
    print("FAIL: Test 12 - Nested list unpacking (after flatten)")
    failures += 1

# Test 13: Unpacking with default parameters
def defaults_func(a, b=5, c=10):
    return a + b + c

result = defaults_func(1)
if result != 16:
    print("FAIL: Test 13a - Unpacking with defaults (no unpack)")
    failures += 1

result = defaults_func(*[1])
if result != 16:
    print("FAIL: Test 13b - Unpacking with defaults (single unpack)")
    failures += 1

result = defaults_func(*[1, 2])
if result != 13:
    print("FAIL: Test 13c - Unpacking with defaults (two args)")
    failures += 1

result = defaults_func(*[1, 2, 3])
if result != 6:
    print("FAIL: Test 13d - Unpacking with defaults (three args)")
    failures += 1

# Test 14: Unpacking dict keys
def string_func(a, b, c):
    return a + b + c

d = {'x': 1, 'y': 2, 'z': 3}
# Dict keys order is not guaranteed, so we check if result contains all keys
result = string_func(*d.keys())
if not (result.count('x') == 1 and result.count('y') == 1 and result.count('z') == 1 and len(result) == 3):
    print("FAIL: Test 14 - Unpacking dict keys")
    print(f"  Expected keys x, y, z in any order, Got: {result}")
    failures += 1

# Test 15: Multiple *args unpacking
def sum_six(a, b, c, d, e, f):
    return a + b + c + d + e + f

list1 = [1, 2]
list2 = [3, 4]
list3 = [5, 6]
result = sum_six(*list1, *list2, *list3)
if result != 21:
    print("FAIL: Test 15 - Multiple *args unpacking")
    print(f"  Expected: 21, Got: {result}")
    failures += 1

# Test 16: Multiple *args with regular args
result = sum_six(1, *[2, 3], 4, *[5, 6])
if result != 21:
    print("FAIL: Test 16 - Multiple *args with regular args")
    print(f"  Expected: 21, Got: {result}")
    failures += 1

# Test 17: Multiple *args with different types
def concat_six(a, b, c, d, e, f):
    return str(a) + str(b) + str(c) + str(d) + str(e) + str(f)

result = concat_six(*[1, 2], *(3, 4), *"56")
if result != "123456":
    print("FAIL: Test 17 - Multiple *args with different types")
    print(f"  Expected: '123456', Got: {result}")
    failures += 1

# Test 18: Multiple *args with empty iterables
result = sum_six(*[], *[1, 2, 3], *[], *[4, 5, 6])
if result != 21:
    print("FAIL: Test 18 - Multiple *args with empty iterables")
    print(f"  Expected: 21, Got: {result}")
    failures += 1

# Summary
if failures == 0:
    print("All *args unpacking tests passed!")
else:
    print(f"FAILED: {failures} test(s)")

failures == 0
