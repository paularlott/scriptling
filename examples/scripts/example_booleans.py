# Example: Boolean values and operations

print("Booleans")

# Boolean literals
t = True
f = False
print(f"True: {t}")
print(f"False: {f}")

# Boolean operations
print(f"True and True: {True and True}")
print(f"True and False: {True and False}")
print(f"False and False: {False and False}")

print(f"True or True: {True or True}")
print(f"True or False: {True or False}")
print(f"False or False: {False or False}")

print(f"not True: {not True}")
print(f"not False: {not False}")

# Comparison results
print(f"5 > 3: {5 > 3}")
print(f"5 < 3: {5 < 3}")
print(f"5 == 5: {5 == 5}")

# Truthiness
if True:
    print("True is truthy")

if not False:
    print("False is falsy")

# Boolean in variables
is_valid = True
if is_valid:
    print("Validation passed")

