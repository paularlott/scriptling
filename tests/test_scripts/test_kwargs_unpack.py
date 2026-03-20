def test_func(a, b, c=3):
    return a + b + c

# Basic **kwargs unpacking
kwargs = {"a": 1, "b": 2, "c": 10}
result = test_func(**kwargs)
assert result == 13

# Partial kwargs unpacking
result2 = test_func(1, **{"b": 5, "c": 7})
assert result2 == 13

# Method call with **kwargs
class TestClass:
    def method(self, x, y):
        return x * y

obj = TestClass()
params = {"x": 3, "y": 4}
result3 = obj.method(**params)
assert result3 == 12

# Mixed positional and **kwargs
def mixed(a, b, c=0, d=0):
    return a + b + c + d

assert mixed(1, 2, **{"c": 3, "d": 4}) == 10

# **kwargs overrides default
assert mixed(1, 2, **{"c": 10}) == 13
