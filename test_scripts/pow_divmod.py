# Test pow() and divmod() builtins

# Test pow() with two arguments
r1 = pow(2, 3) == 8
r2 = pow(2, 10) == 1024
r3 = pow(3, 3) == 27
r4 = pow(10, 0) == 1
r5 = pow(5, -1) == 0.2

# Test pow() with three arguments (modular exponentiation)
r6 = pow(2, 10, 1000) == 24
r7 = pow(3, 4, 5) == 1
r8 = pow(7, 3, 11) == 2

# Test divmod() - compare elements individually since tuple comparison doesn't work
dm1 = divmod(17, 5)
r9 = dm1[0] == 3 and dm1[1] == 2
dm2 = divmod(10, 3)
r10 = dm2[0] == 3 and dm2[1] == 1
dm3 = divmod(20, 4)
r11 = dm3[0] == 5 and dm3[1] == 0

r1 and r2 and r3 and r4 and r5 and r6 and r7 and r8 and r9 and r10 and r11
