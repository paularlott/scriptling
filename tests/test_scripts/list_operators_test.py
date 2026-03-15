# list + list
a = [1, 2, 3]
b = [4, 5, 6]
c = a + b
assert c == [1, 2, 3, 4, 5, 6], f"list + list failed: {c}"

# list + empty
assert a + [] == [1, 2, 3], "list + [] failed"
assert [] + a == [1, 2, 3], "[] + list failed"

# list * int
assert a * 2 == [1, 2, 3, 1, 2, 3], f"list * 2 failed: {a * 2}"
assert a * 0 == [], f"list * 0 failed: {a * 0}"
assert a * 1 == [1, 2, 3], f"list * 1 failed: {a * 1}"

# int * list
assert 3 * [1, 2] == [1, 2, 1, 2, 1, 2], f"int * list failed: {3 * [1, 2]}"
assert 0 * [1, 2] == [], f"0 * list failed"

# list comprehension result + list (the original bug)
evens = [x for x in [1, 2, 3, 4] if x % 2 == 0]
odds  = [x for x in [1, 2, 3, 4] if x % 2 != 0]
assert evens + odds == [2, 4, 1, 3], f"comprehension + list failed: {evens + odds}"

print("all list operator tests passed")
