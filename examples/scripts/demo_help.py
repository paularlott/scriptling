# Demo of the Scriptling Help System

print("=== 1. List all available libraries ===")
print("Command: help('modules')")
help("modules")
print("-" * 40)
print("")

print("=== 2. Help for a builtin function ===")
print("Command: help('print')")
help(print)
print("-" * 40)
print("")

print("=== 3. Help for a library ===")
# Note: Library must be imported to see detailed help,
# but help('libname') works if it's available even if not imported (shows basic info)
import math
print("Command: help('math')")
help("math")
print("-" * 40)
print("")

print("=== 4. Help for a library function ===")
print("Command: help('math.sqrt')")
help("math.sqrt")
print("-" * 40)
print("")

print("=== 5. Help for a user-defined function ===")
def calculate_hypotenuse(a, b):
    """Calculate the length of the hypotenuse of a right triangle.

    Args:
        a: Length of side a
        b: Length of side b

    Returns:
        The length of the hypotenuse (sqrt(a^2 + b^2))
    """
    return math.sqrt(a*a + b*b)

print("Command: help(calculate_hypotenuse)")
help(calculate_hypotenuse)
print("-" * 40)
print("")

print("=== 6. Help for a scripted library ===")
import testlib
help('testlib')

print('---')
help('testlib.greet')
