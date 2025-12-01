# Test *args (variadic arguments) functionality

print("=== Basic *args")
def sum_all(*args):
    total = 0
    for num in args:
        total += num
    return total

print("sum_all(1, 2, 3) =", sum_all(1, 2, 3))  # 6
print("sum_all() =", sum_all())  # 0
print("sum_all(10) =", sum_all(10))  # 10

print("\n=== Mixed parameters with *args")
def prefix_print(prefix, *args):
    result = []
    for item in args:
        result.append(prefix + str(item))
    return result

print("prefix_print('>', 1, 2) =", prefix_print(">", 1, 2))  # [">1", ">2"]
print("prefix_print('#') =", prefix_print("#"))  # []

print("\n=== *args with default parameters")
def complex_args(a, b=10, *args):
    return [a, b, args]

print("complex_args(1) =", complex_args(1))  # [1, 10, []]
print("complex_args(1, 2) =", complex_args(1, 2))  # [1, 2, []]
print("complex_args(1, 2, 3, 4) =", complex_args(1, 2, 3, 4))  # [1, 2, [3, 4]]

print("\n=== Lambda with *args")
sum_lambda = lambda *args: sum(args)
print("sum_lambda(1, 2, 3) =", sum_lambda(1, 2, 3))  # 6

first_arg = lambda *args: args[0]
print("first_arg(10, 20) =", first_arg(10, 20))  # 10

print("\n=== *args type check")
def check_type(*args):
    print("type(args) =", type(args))  # LIST
    print("args.type() =", args.type())  # LIST
    return args

check_type(1, 2)

print("\n=== All tests completed")
