def test_func(a, b, c=3):
    return a + b + c

# Test **kwargs unpacking
kwargs = {"a": 1, "b": 2, "c": 10}
result = test_func(**kwargs)
print("Result:", result)  # Should be 13

# Test with partial kwargs
result2 = test_func(1, **{"b": 5, "c": 7})
print("Result2:", result2)  # Should be 13

# Test method call with **kwargs
class TestClass:
    def method(self, x, y):
        return x * y

obj = TestClass()
params = {"x": 3, "y": 4}
result3 = obj.method(**params)
print("Result3:", result3)  # Should be 12

print("All kwargs unpacking tests passed!")
