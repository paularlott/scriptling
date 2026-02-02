# Test callable() and isinstance() builtins

# Test callable()
def my_func():
    return 42

r1 = callable(my_func)
r2 = callable(len)
r3 = callable(print)
r4 = callable(42) == False
r5 = callable("hello") == False
r6 = callable([1, 2, 3]) == False
r7 = callable(lambda x: x * 2)

# Test isinstance()
r8 = isinstance(42, "int")
r9 = isinstance(3.14, "float")
r10 = isinstance("hello", "str")
r11 = isinstance([1, 2], "list")
r12 = isinstance({"a": 1}, "dict")
r13 = isinstance(True, "bool")
r14 = isinstance(None, "NoneType")
r15 = isinstance((1, 2), "tuple")

# Negative tests
r16 = isinstance(42, "str") == False
r17 = isinstance("hello", "int") == False
r18 = isinstance([1, 2], "dict") == False

r1 and r2 and r3 and r4 and r5 and r6 and r7 and r8 and r9 and r10 and r11 and r12 and r13 and r14 and r15 and r16 and r17 and r18
