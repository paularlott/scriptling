# Comprehensive **kwargs test
failures = 0

# Test 1: Basic **kwargs
def test_basic(**kwargs):
    return kwargs

result = test_basic(a=1, b=2, c=3)
if result['a'] != 1 or result['b'] != 2 or result['c'] != 3:
    print("FAIL: Test 1 - Basic **kwargs")
    failures += 1

# Test 2: Empty **kwargs
result = test_basic()
if len(result) != 0:
    print("FAIL: Test 2 - Empty **kwargs")
    failures += 1

# Test 3: **kwargs with regular parameters
def greet(name, **kwargs):
    greeting = kwargs.get('greeting', 'Hello')
    return greeting + ', ' + name

if greet('Alice') != 'Hello, Alice':
    print("FAIL: Test 3a - **kwargs with regular parameters (default)")
    failures += 1
if greet('Bob', greeting='Hi') != 'Hi, Bob':
    print("FAIL: Test 3b - **kwargs with regular parameters (custom)")
    failures += 1

# Test 4: **kwargs with default parameters
def func_with_defaults(a, b=10, **kwargs):
    total = a + b
    for key, value in kwargs.items():
        total += value
    return total

if func_with_defaults(5) != 15:
    print("FAIL: Test 4a - **kwargs with default parameters (no kwargs)")
    failures += 1
if func_with_defaults(5, b=20) != 25:
    print("FAIL: Test 4b - **kwargs with default parameters (override default)")
    failures += 1
if func_with_defaults(5, x=3, y=7) != 25:
    print("FAIL: Test 4c - **kwargs with default parameters (with kwargs)")
    failures += 1
if func_with_defaults(5, b=20, x=3, y=7) != 35:
    print("FAIL: Test 4d - **kwargs with default parameters (all)")
    failures += 1

# Test 5: *args and **kwargs together
def combined(*args, **kwargs):
    args_sum = 0
    for arg in args:
        args_sum += arg
    kwargs_sum = 0
    for key, value in kwargs.items():
        kwargs_sum += value
    return [args_sum, kwargs_sum]

result = combined(1, 2, 3, x=10, y=20)
if result[0] != 6 or result[1] != 30:
    print("FAIL: Test 5 - *args and **kwargs together")
    failures += 1

# Test 6: Only **kwargs, no positional or *args
def only_kwargs(**options):
    return len(options)

if only_kwargs(a=1, b=2, c=3, d=4) != 4:
    print("FAIL: Test 6 - Only **kwargs")
    failures += 1

# Test 7: Access kwargs like dict
def print_kwargs(**kwargs):
    if 'abc' in kwargs:
        return kwargs['abc']
    return None

if print_kwargs(abc=1) != 1:
    print("FAIL: Test 7a - Access kwargs like dict (found)")
    failures += 1
if print_kwargs(xyz=2) != None:
    print("FAIL: Test 7b - Access kwargs like dict (not found)")
    failures += 1

# Test 8: Lambda with **kwargs
lambda_kwargs = lambda **kw: kw.get('value', 0)
if lambda_kwargs(value=42) != 42:
    print("FAIL: Test 8a - Lambda with **kwargs (with value)")
    failures += 1
if lambda_kwargs() != 0:
    print("FAIL: Test 8b - Lambda with **kwargs (default)")
    failures += 1

# Test 9: Lambda with *args and **kwargs
lambda_both = lambda *a, **kw: [len(a), len(kw)]
result = lambda_both(1, 2, 3, x=1, y=2)
if result[0] != 3 or result[1] != 2:
    print("FAIL: Test 9 - Lambda with *args and **kwargs")
    failures += 1

# Test 10: All parameter types together
def all_params(a, b=10, *args, **kwargs):
    return {
        'a': a,
        'b': b,
        'args': args,
        'kwargs': kwargs
    }

result = all_params(1, 2, 3, 4, x=5, y=6)
if result['a'] != 1 or result['b'] != 2:
    print("FAIL: Test 10a - All parameter types (a, b)")
    failures += 1
if len(result['args']) != 2 or result['args'][0] != 3 or result['args'][1] != 4:
    print("FAIL: Test 10b - All parameter types (args)")
    failures += 1
if result['kwargs']['x'] != 5 or result['kwargs']['y'] != 6:
    print("FAIL: Test 10c - All parameter types (kwargs)")
    failures += 1

# Summary
if failures == 0:
    print("All **kwargs tests passed!")
else:
    print("FAILED: " + str(failures) + " test(s)")

failures == 0
