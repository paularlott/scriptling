# Test Python-style boolean literals
flag1 = True
flag2 = False

print("flag1 (true):", flag1)
print("flag2 (false):", flag2)

# Test in conditions
if flag1:
    print("flag1 is truthy")

if not flag2:
    print("flag2 is falsy")

# Test boolean operations
result1 = True and False
result2 = True or False
result3 = not True

print("True and False:", result1)
print("True or False:", result2)
print("not True:", result3)