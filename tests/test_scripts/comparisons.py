# Numeric comparisons
assert 10 > 5
assert 5 < 10
assert 10 >= 10
assert 10 <= 10
assert 10 == 10
assert 10 != 9

# Chained comparisons
x = 5
assert 1 < x < 10
assert 1 <= x <= 5
assert 10 > x > 0
assert 1 < x < 10 < 20

# Chained equality
a = 5
b = 5
assert a == b == 5

# String comparisons
assert "abc" < "abd"
assert "abc" == "abc"
assert "z" > "a"

# Mixed truthiness comparisons
assert (0 == False)
assert (1 == True)
assert (None == None)

# is / is not
lst = [1, 2, 3]
ref = lst
other = [1, 2, 3]
assert lst is ref
assert lst is not other

# in / not in
assert 3 in [1, 2, 3]
assert 4 not in [1, 2, 3]
assert "b" in "abc"
assert "x" not in "abc"
assert "key" in {"key": 1}
assert "missing" not in {"key": 1}
