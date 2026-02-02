failures = 0

# Test 1: Basic **kwargs
def test_kwargs(**kwargs):
    return kwargs

result = test_kwargs(a=1, b=2, c=3)
if result['a'] != 1 or result['b'] != 2 or result['c'] != 3:
    failures += 1

# Test 2: Empty **kwargs
result = test_kwargs()
if len(result) != 0:
    failures += 1

# Test 3: **kwargs with regular parameters
def greet(name, **kwargs):
    greeting = kwargs.get('greeting', 'Hello')
    return greeting + ', ' + name

if greet('Alice') != 'Hello, Alice':
    failures += 1
if greet('Bob', greeting='Hi') != 'Hi, Bob':
    failures += 1

# Test 4: **kwargs with default parameters
def func_with_defaults(a, b=10, **kwargs):
    total = a + b
    for key, value in kwargs.items():
        total += value
    return total

if func_with_defaults(5) != 15:
    failures += 1
if func_with_defaults(5, b=20) != 25:
    failures += 1
if func_with_defaults(5, x=3, y=7) != 25:
    failures += 1
if func_with_defaults(5, b=20, x=3, y=7) != 35:
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
    failures += 1

# Test 6: Only **kwargs, no positional or *args
def only_kwargs(**options):
    return len(options)

if only_kwargs(a=1, b=2, c=3, d=4) != 4:
    failures += 1

# Test 7: Access kwargs like dict
def print_kwargs(**kwargs):
    if 'abc' in kwargs:
        return kwargs['abc']
    return None

if print_kwargs(abc=1) != 1:
    failures += 1
if print_kwargs(xyz=2) != None:
    failures += 1

# Test 8: Lambda with **kwargs
lambda_kwargs = lambda **kw: kw.get('value', 0)
if lambda_kwargs(value=42) != 42:
    failures += 1
if lambda_kwargs() != 0:
    failures += 1

# Test 9: Lambda with *args and **kwargs
lambda_both = lambda *a, **kw: [len(a), len(kw)]
result = lambda_both(1, 2, 3, x=1, y=2)
if result[0] != 3 or result[1] != 2:
    failures += 1

failures == 0
